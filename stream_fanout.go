package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const minFreeDiskBytes uint64 = 1 << 30 // 1 GB safety floor for recordings

var tsPktPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 4096)
		return &b
	},
}

func clonePkt(src []byte) *[]byte {
	bp := tsPktPool.Get().(*[]byte)
	*bp = append((*bp)[:0], src...)
	return bp
}

func releasePkt(bp *[]byte) {
	if bp == nil || cap(*bp) > 16384 {
		return
	}
	tsPktPool.Put(bp)
}

// tsSub is a non-blocking subscriber for HTTP-TS / Multiviewer preview.
type tsSub struct {
	ch      chan *[]byte
	dropped atomic.Uint64
}

// RecStatus describes an active or recently stopped recording on a slot.
type RecStatus struct {
	Active    bool   `json:"active"`
	File      string `json:"file,omitempty"`
	Bytes     uint64 `json:"bytes"`
	StartedAt int64  `json:"startedAt,omitempty"`
	Dropped   uint64 `json:"dropped"`
	Error     string `json:"error,omitempty"`
}

type tsRecorder struct {
	slot      int
	path      string
	file      string
	dir       string
	startedAt time.Time
	ch        chan *[]byte
	done      chan struct{}
	stopOnce  sync.Once
	bytes     atomic.Uint64
	dropped   atomic.Uint64
	active    atomic.Bool
	errMsg    atomic.Pointer[string]
	onStop    func(reason string)
}

func newTSRecorder(slot int, dir string, onStop func(string)) (*tsRecorder, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create recordings dir: %w", err)
	}
	if free, err := freeDiskBytes(dir); err == nil && free < minFreeDiskBytes {
		return nil, fmt.Errorf("low disk space (%.2f GB free, need at least 1.0 GB)", float64(free)/(1<<30))
	}
	name := fmt.Sprintf("LiveLink_CH%02d_%s.ts", slot, time.Now().Format("20060102_150405"))
	full := filepath.Join(dir, name)
	f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open recording file: %w", err)
	}
	r := &tsRecorder{
		slot:      slot,
		path:      full,
		file:      name,
		dir:       dir,
		startedAt: time.Now(),
		ch:        make(chan *[]byte, 256),
		done:      make(chan struct{}),
		onStop:    onStop,
	}
	r.active.Store(true)
	go r.writeLoop(f)
	return r, nil
}

func (r *tsRecorder) push(pkt []byte) {
	if !r.active.Load() {
		return
	}
	bp := clonePkt(pkt)
	select {
	case r.ch <- bp:
	default:
		releasePkt(bp)
		r.dropped.Add(1)
	}
}

func (r *tsRecorder) stop() {
	r.stopOnce.Do(func() {
		r.active.Store(false)
		close(r.ch)
		<-r.done
	})
}

func (r *tsRecorder) status() *RecStatus {
	if r == nil {
		return nil
	}
	st := &RecStatus{
		Active:    r.active.Load(),
		File:      r.file,
		Bytes:     r.bytes.Load(),
		StartedAt: r.startedAt.Unix(),
		Dropped:   r.dropped.Load(),
	}
	if p := r.errMsg.Load(); p != nil {
		st.Error = *p
	}
	return st
}

func (r *tsRecorder) writeLoop(f *os.File) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("recorder slot %d panic recovered: %v", r.slot, rec)
		}
		r.active.Store(false)
		close(r.done)
	}()
	bw := bufio.NewWriterSize(f, 64*1024)
	defer func() {
		_ = bw.Flush()
		_ = f.Close()
	}()
	lastDiskCheck := time.Now()
	for bp := range r.ch {
		if bp == nil {
			continue
		}
		n, err := bw.Write(*bp)
		releasePkt(bp)
		if n > 0 {
			r.bytes.Add(uint64(n))
		}
		if err != nil {
			msg := "write error: " + err.Error()
			r.errMsg.Store(&msg)
			r.active.Store(false)
			if r.onStop != nil {
				r.onStop(msg)
			}
			// Drain remaining queued packets without blocking sender
			for rem := range r.ch {
				releasePkt(rem)
			}
			return
		}
		if time.Since(lastDiskCheck) >= 2*time.Second {
			lastDiskCheck = time.Now()
			_ = bw.Flush()
			if free, err := freeDiskBytes(r.dir); err == nil && free < minFreeDiskBytes {
				msg := "auto-stopped: disk space below 1 GB"
				r.errMsg.Store(&msg)
				r.active.Store(false)
				if r.onStop != nil {
					r.onStop(msg)
				}
				for rem := range r.ch {
					releasePkt(rem)
				}
				return
			}
		}
	}
}

// tsFanout distributes MPEG-TS packets to live previews and optional DVR recording
// with zero blocking on the primary SRT forwarding hot-path.
type tsFanout struct {
	mu      sync.Mutex
	subsMap map[*tsSub]struct{}
	subs    atomic.Pointer[[]*tsSub]
	rec     atomic.Pointer[tsRecorder]
	lastErr atomic.Pointer[string]
}

func newTSFanout() *tsFanout {
	f := &tsFanout{
		subsMap: make(map[*tsSub]struct{}),
	}
	empty := []*tsSub{}
	f.subs.Store(&empty)
	return f
}

func (f *tsFanout) subscribe() *tsSub {
	s := &tsSub{ch: make(chan *[]byte, 128)}
	f.mu.Lock()
	f.subsMap[s] = struct{}{}
	f.rebuildSubsLocked()
	f.mu.Unlock()
	return s
}

func (f *tsFanout) unsubscribe(s *tsSub) {
	f.mu.Lock()
	delete(f.subsMap, s)
	f.rebuildSubsLocked()
	f.mu.Unlock()
	for {
		select {
		case bp := <-s.ch:
			releasePkt(bp)
		default:
			return
		}
	}
}

func (f *tsFanout) rebuildSubsLocked() {
	list := make([]*tsSub, 0, len(f.subsMap))
	for s := range f.subsMap {
		list = append(list, s)
	}
	f.subs.Store(&list)
}

// push is called on the SRT hot-path. Zero allocations when no preview or recording is active.
func (f *tsFanout) push(pkt []byte) {
	if len(pkt) == 0 {
		return
	}
	subs := *f.subs.Load()
	rec := f.rec.Load()
	if len(subs) == 0 && (rec == nil || !rec.active.Load()) {
		return
	}
	if rec != nil && rec.active.Load() {
		rec.push(pkt)
	}
	for _, s := range subs {
		bp := clonePkt(pkt)
		select {
		case s.ch <- bp:
		default:
			releasePkt(bp)
			s.dropped.Add(1)
		}
	}
}

func (f *tsFanout) startRec(slot int, dir string, onAutoStop func(string)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if cur := f.rec.Load(); cur != nil && cur.active.Load() {
		return nil
	}
	rec, err := newTSRecorder(slot, dir, func(reason string) {
		f.lastErr.Store(&reason)
		if onAutoStop != nil {
			onAutoStop(reason)
		}
	})
	if err != nil {
		return err
	}
	f.lastErr.Store(nil)
	f.rec.Store(rec)
	return nil
}

func (f *tsFanout) stopRec() {
	f.mu.Lock()
	cur := f.rec.Swap(nil)
	f.mu.Unlock()
	if cur != nil {
		cur.stop()
	}
}

func (f *tsFanout) recStatus() *RecStatus {
	if cur := f.rec.Load(); cur != nil {
		return cur.status()
	}
	if e := f.lastErr.Load(); e != nil && *e != "" {
		return &RecStatus{Active: false, Error: *e}
	}
	return nil
}

func (h *Hub) recordingsDir() string {
	return filepath.Join(filepath.Dir(configFile), "recordings")
}

func (h *Hub) startRecording(slot int) error {
	h.mu.Lock()
	if slot < 1 || slot > len(h.rt) {
		h.mu.Unlock()
		return errors.New("unknown signal")
	}
	rt := h.rt[slot-1]
	h.mu.Unlock()
	if err := rt.fanout.startRec(slot, h.recordingsDir(), func(reason string) {
		h.note("recording signal %d stopped: %s", slot, reason)
	}); err != nil {
		return err
	}
	h.note("recording started on signal %d", slot)
	return nil
}

func (h *Hub) stopRecording(slot int) error {
	h.mu.Lock()
	if slot < 1 || slot > len(h.rt) {
		h.mu.Unlock()
		return errors.New("unknown signal")
	}
	rt := h.rt[slot-1]
	h.mu.Unlock()
	rt.fanout.stopRec()
	h.note("recording stopped on signal %d", slot)
	return nil
}

func (h *Hub) stopAllRecordings() {
	h.mu.Lock()
	rts := append([]*placeRT{}, h.rt...)
	h.mu.Unlock()
	for _, rt := range rts {
		if rt != nil && rt.fanout != nil {
			rt.fanout.stopRec()
		}
	}
}

// handlePreviewStream serves live MPEG-TS over HTTP for browser mpegts.js Multiviewer and VLC/OBS.
func handlePreviewStream(h *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("preview handler panic recovered: %v", rec)
			}
		}()
		raw := strings.TrimSuffix(r.PathValue("slot"), ".ts")
		slot, err := strconv.Atoi(raw)
		if err != nil || slot < 1 {
			http.Error(w, "invalid channel", http.StatusBadRequest)
			return
		}
		h.mu.Lock()
		if slot > len(h.rt) {
			h.mu.Unlock()
			http.Error(w, "channel not found", http.StatusNotFound)
			return
		}
		rt := h.rt[slot-1]
		h.mu.Unlock()

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "video/mp2t")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		sub := rt.fanout.subscribe()
		defer rt.fanout.unsubscribe(sub)

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case bp := <-sub.ch:
				if bp == nil {
					continue
				}
				_, werr := w.Write(*bp)
				releasePkt(bp)
				if werr != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}
