package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// MediaMTXConfig controls the optional MediaMTX sidecar for RTMP / RTSP / WebRTC / HLS ingest & output.
type MediaMTXConfig struct {
	Enabled    bool   `json:"enabled"`
	Path       string `json:"path"`       // optional custom path; "" auto-detects mediamtx next to binary
	RTMPPort   int    `json:"rtmpPort"`   // default 1935
	RTSPPort   int    `json:"rtspPort"`   // default 8554
	WebRTCPort int    `json:"webrtcPort"` // default 8889
	HLSPort    int    `json:"hlsPort"`    // default 8888
	SRTPort    int    `json:"srtPort"`    // default 8890 (separate from LiveLink Hub primary SRT port)
}

type MediaMTXStatus struct {
	Enabled    bool   `json:"enabled"`
	Found      bool   `json:"found"`
	Running    bool   `json:"running"`
	Path       string `json:"path"`
	RTMPPort   int    `json:"rtmpPort"`
	RTSPPort   int    `json:"rtspPort"`
	WebRTCPort int    `json:"webrtcPort"`
	HLSPort    int    `json:"hlsPort"`
	SRTPort    int    `json:"srtPort"`
	Restarts   int    `json:"restarts"`
	Error      string `json:"error,omitempty"`
}

func defaultMediaMTXConfig() MediaMTXConfig {
	return MediaMTXConfig{
		Enabled:    false,
		RTMPPort:   1935,
		RTSPPort:   8554,
		WebRTCPort: 8889,
		HLSPort:    8888,
		SRTPort:    8890,
	}
}

func (c *MediaMTXConfig) normalize() {
	if c.RTMPPort < 1 || c.RTMPPort > 65535 {
		c.RTMPPort = 1935
	}
	if c.RTSPPort < 1 || c.RTSPPort > 65535 {
		c.RTSPPort = 8554
	}
	if c.WebRTCPort < 1 || c.WebRTCPort > 65535 {
		c.WebRTCPort = 8889
	}
	if c.HLSPort < 1 || c.HLSPort > 65535 {
		c.HLSPort = 8888
	}
	if c.SRTPort < 1 || c.SRTPort > 65535 {
		c.SRTPort = 8890
	}
}

// MediaMTXSidecar manages mediamtx as an isolated, auto-restarting watchdog child process.
type MediaMTXSidecar struct {
	hub      *Hub
	mu       sync.Mutex
	cmd      *exec.Cmd
	running  bool
	restarts int
	lastErr  string
	stopCh   chan struct{}
	wakeCh   chan struct{}
	stopped  bool
}

func newMediaMTXSidecar(h *Hub) *MediaMTXSidecar {
	return &MediaMTXSidecar{
		hub:    h,
		stopCh: make(chan struct{}),
		wakeCh: make(chan struct{}, 1),
	}
}

func (m *MediaMTXSidecar) resolveBinary(cfg MediaMTXConfig) string {
	if p := strings.TrimSpace(cfg.Path); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		return ""
	}
	binName := "mediamtx"
	if runtime.GOOS == "windows" {
		binName = "mediamtx.exe"
	}
	candidates := []string{
		filepath.Join(exeDir(), binName),
		filepath.Join(exeDir(), "mediamtx", binName),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if lp, err := exec.LookPath(binName); err == nil {
		return lp
	}
	return ""
}

func (m *MediaMTXSidecar) notify() {
	select {
	case m.wakeCh <- struct{}{}:
	default:
	}
}

func (m *MediaMTXSidecar) status() *MediaMTXStatus {
	m.hub.mu.Lock()
	cfg := m.hub.cfg.MediaMTX
	m.hub.mu.Unlock()

	bin := m.resolveBinary(cfg)
	m.mu.Lock()
	defer m.mu.Unlock()
	return &MediaMTXStatus{
		Enabled:    cfg.Enabled,
		Found:      bin != "",
		Running:    m.running,
		Path:       bin,
		RTMPPort:   cfg.RTMPPort,
		RTSPPort:   cfg.RTSPPort,
		WebRTCPort: cfg.WebRTCPort,
		HLSPort:    cfg.HLSPort,
		SRTPort:    cfg.SRTPort,
		Restarts:   m.restarts,
		Error:      m.lastErr,
	}
}

func (m *MediaMTXSidecar) writeConfig(cfg MediaMTXConfig, slots int) (string, error) {
	var b strings.Builder
	b.WriteString("# LiveLink Hub — MediaMTX Bridge (auto-generated)\n")
	b.WriteString("logLevel: info\n")
	b.WriteString("logDestinations: [stdout]\n\n")
	b.WriteString("rtsp: yes\nrtmp: yes\nhls: yes\nsrt: yes\nwebrtc: yes\n")
	b.WriteString("api: no\nmetrics: no\npprof: no\nplayback: no\n\n")
	fmt.Fprintf(&b, "rtspAddress: :%d\n", cfg.RTSPPort)
	fmt.Fprintf(&b, "rtmpAddress: :%d\n", cfg.RTMPPort)
	fmt.Fprintf(&b, "hlsAddress: :%d\n", cfg.HLSPort)
	fmt.Fprintf(&b, "srtAddress: :%d\n", cfg.SRTPort)
	fmt.Fprintf(&b, "webrtcAddress: :%d\n\n", cfg.WebRTCPort)
	b.WriteString("paths:\n")
	b.WriteString("  all_others:\n")
	for i := 1; i <= slots; i++ {
		fmt.Fprintf(&b, "  cam%d:\n", i)
	}
	ymlPath := filepath.Join(filepath.Dir(configFile), "mediamtx_livelink.yml")
	if err := os.WriteFile(ymlPath, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return ymlPath, nil
}

func (m *MediaMTXSidecar) loop() {
	backoff := time.Second
	for {
		select {
		case <-m.stopCh:
			m.killProcess()
			return
		default:
		}

		m.hub.mu.Lock()
		cfg := m.hub.cfg.MediaMTX
		slots := m.hub.cfg.Slots
		m.hub.mu.Unlock()

		if !cfg.Enabled {
			m.killProcess()
			m.mu.Lock()
			m.lastErr = ""
			m.mu.Unlock()
			select {
			case <-m.stopCh:
				return
			case <-m.wakeCh:
				backoff = time.Second
				continue
			case <-time.After(2 * time.Second):
				continue
			}
		}

		bin := m.resolveBinary(cfg)
		if bin == "" {
			m.mu.Lock()
			m.running = false
			m.lastErr = "mediamtx binary not found next to LiveLink Hub"
			m.mu.Unlock()
			select {
			case <-m.stopCh:
				return
			case <-m.wakeCh:
				continue
			case <-time.After(3 * time.Second):
				continue
			}
		}

		ymlPath, err := m.writeConfig(cfg, slots)
		if err != nil {
			m.mu.Lock()
			m.lastErr = "config write failed: " + err.Error()
			m.mu.Unlock()
			time.Sleep(2 * time.Second)
			continue
		}

		cmd := exec.Command(bin, ymlPath)
		cmd.Dir = filepath.Dir(bin)
		setHideWindow(cmd)
		if err := cmd.Start(); err != nil {
			m.mu.Lock()
			m.running = false
			m.lastErr = "start failed: " + err.Error()
			m.mu.Unlock()
			time.Sleep(backoff)
			if backoff < 5*time.Second {
				backoff *= 2
			}
			continue
		}

		m.mu.Lock()
		m.cmd = cmd
		m.running = true
		m.lastErr = ""
		m.mu.Unlock()
		m.hub.note("MediaMTX sidecar started (RTMP :%d, RTSP :%d, WebRTC :%d)", cfg.RTMPPort, cfg.RTSPPort, cfg.WebRTCPort)
		backoff = time.Second

		waitCh := make(chan error, 1)
		go func() { waitCh <- cmd.Wait() }()

		select {
		case <-m.stopCh:
			m.killProcess()
			return
		case <-m.wakeCh:
			// Settings changed: restart cleanly
			m.killProcess()
			<-waitCh
			continue
		case err := <-waitCh:
			m.mu.Lock()
			m.cmd = nil
			m.running = false
			if err != nil {
				m.lastErr = "exited: " + err.Error()
			}
			m.restarts++
			m.mu.Unlock()
			log.Printf("MediaMTX watchdog: process exited (%v), restarting in %s", err, backoff)
			select {
			case <-m.stopCh:
				return
			case <-time.After(backoff):
				if backoff < 5*time.Second {
					backoff *= 2
				}
			}
		}
	}
}

func (m *MediaMTXSidecar) killProcess() {
	m.mu.Lock()
	cmd := m.cmd
	m.cmd = nil
	m.running = false
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (m *MediaMTXSidecar) stop() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	close(m.stopCh)
	m.mu.Unlock()
	m.killProcess()
}
