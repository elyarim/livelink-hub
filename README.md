# LiveLink Hub

**Broadcast-grade Multi-Channel SRT Matrix, GPU Multiviewer, Direct `.ts` DVR Recorder & Optional MediaMTX Sidecar (RTMP / RTSP / WebRTC / HLS).**

> 🇪🇸 [Leer en Español (README.es.md)](README.es.md)

## Key Features
- **Zero-Blocking Broadcast Architecture:** Live SRT forwarding runs on an isolated hot-path. Slow browser previews or full disks never drop or stall live studio feeds.
- **Live GPU Multiviewer (Zero FFmpeg):** Embedded `mpegts.js` player decodes live MPEG-TS streams directly on the browser GPU with sub-second latency.
- **Direct `.ts` DVR Recording:** 1-click crash-proof MPEG-TS recording per channel with automatic < 1 GB disk space protection and graceful shutdown flush.
- **Full N×N Routing Matrix + Stream Deck:** Route any incoming camera to any studio output, controllable via Web UI or Blackmagic Smart Videohub protocol (TCP `9990`).
- **Cumulative Data Monitor:** Real-time MB/GB inbound and outbound traffic tracking per channel and globally.
- **Universal Mobile QR Codes:** Instant QR generation for Larix Broadcaster, Haivision Play Pro, native phone camera text, and Raw SRT.
- **Optional MediaMTX Sidecar with Watchdog:** Drop `mediamtx.exe` (Windows) or `mediamtx` (Linux) next to LiveLink Hub to unlock RTMP (DJI drones, GoPro, ATEM Mini), RTSP (IP/PTZ cameras), WebRTC, and HLS with automatic crash recovery.
- **Bilingual Dark Studio UI (`ES | EN`):** Instant 1-click language switch.
- **Cross-Platform:** Native single binary for **Windows** (WebView2 desktop app), **Linux (`amd64` / `arm64`)**, and **Docker**.

## Quick Start

### Windows
Double-click `livelink-hub.exe` (or build from source with `.\build.ps1`).

### Linux / VPS
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o livelink-hub .
./livelink-hub -headless -bind 0.0.0.0:8790
```
Or install as a 24/7 `systemd` service with UDP buffer tuning:
```bash
chmod +x install-linux.sh && ./install-linux.sh
```

### Docker
```bash
docker compose up -d --build
```

## Authors & Credits
- **Author:** Alan de Santiago ([@elyarim](https://github.com/elyarim))
- **Co-Author:** CPAGallardo ([@CPAGallardo](https://github.com/CPAGallardo))
- Built upon original SRT routing foundations by [Peter Aellig](https://github.com/peteraellig/srt-bridge).

## License
MIT License

