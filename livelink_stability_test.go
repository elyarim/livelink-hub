package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFanoutNonBlocking(t *testing.T) {
	f := newTSFanout()
	sub := f.subscribe()
	defer f.unsubscribe(sub)

	pkt := make([]byte, 1316)
	pkt[0] = 0x47 // MPEG-TS sync byte

	start := time.Now()
	// Push 1000 packets while subscriber reads nothing; must NEVER block.
	for i := 0; i < 1000; i++ {
		f.push(pkt)
	}
	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Fatalf("fanout blocked for %v on slow subscriber", elapsed)
	}
	if sub.dropped.Load() == 0 {
		t.Fatalf("expected dropped packets on unread subscriber, got 0")
	}
}

func TestRecorderWritesAndFlushes(t *testing.T) {
	dir := t.TempDir()
	f := newTSFanout()
	if err := f.startRec(1, dir, nil); err != nil {
		t.Fatalf("startRec failed: %v", err)
	}
	pkt := make([]byte, 188)
	pkt[0] = 0x47
	for i := 0; i < 50; i++ {
		f.push(pkt)
	}
	time.Sleep(50 * time.Millisecond)
	st := f.recStatus()
	if st == nil || !st.Active {
		t.Fatalf("expected active recording status, got %+v", st)
	}
	file := filepath.Join(dir, st.File)
	f.stopRec()

	info, err := os.Stat(file)
	if err != nil {
		t.Fatalf("recording file missing: %v", err)
	}
	if info.Size() != int64(50*188) {
		t.Fatalf("expected %d bytes written, got %d", 50*188, info.Size())
	}
}

func TestMediaMTXConfigNormalize(t *testing.T) {
	c := MediaMTXConfig{}
	c.normalize()
	if c.RTMPPort != 1935 || c.RTSPPort != 8554 || c.WebRTCPort != 8889 || c.HLSPort != 8888 || c.SRTPort != 8890 {
		t.Fatalf("unexpected normalized defaults: %+v", c)
	}
}
