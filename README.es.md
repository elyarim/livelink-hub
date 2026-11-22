# LiveLink Hub

**Central Broadcast Multi-Canal SRT con Matriz N×N, Multiviewer GPU en Vivo, Grabación DVR `.ts` y Módulo Opcional MediaMTX (RTMP / RTSP / WebRTC / HLS).**

> 🇺🇸 [Read in English (README.md)](README.md)

## Características Principales
- **Arquitectura Broadcast Sin Bloqueos (Zero-Blocking):** El enrutamiento SRT en vivo corre totalmente aislado. Un preview lento o un disco lleno jamás congelan la señal hacia OBS o vMix.
- **Multiviewer en Vivo por GPU (Cero FFmpeg):** Reproductor `mpegts.js` integrado offline que decodifica en la GPU del navegador sin cargar el CPU del servidor.
- **Grabación Directa `.ts` (DVR):** Botón `REC` por canal resistente a cortes de luz, con guardia automática de espacio en disco (< 1 GB) y cierre seguro.
- **Matriz de Video N×N + Stream Deck:** Conmuta cualquier cámara hacia cualquier salida de estudio desde la web o vía protocolo Blackmagic Smart Videohub (TCP `9990`).
- **Contador de Datos Acumulados:** Monitoreo en tiempo real de MB/GB de subida y bajada (ideal para enlaces celulares 4G/5G).
- **Códigos QR Universales para Móviles:** Compatible con Larix Broadcaster, Haivision Play Pro, cámara nativa (texto) y SRT puro.
- **Módulo Opcional MediaMTX con Watchdog:** Coloca `mediamtx.exe` (Windows) o `mediamtx` (Linux) junto al ejecutable para habilitar ingesta **RTMP** (drones DJI, GoPro, ATEM Mini), **RTSP** (cámaras IP/PTZ), **WebRTC** y **HLS** con reinicio automático ante caídas.
- **Interfaz Dark Studio Bilingüe (`ES | EN`):** Cambio instantáneo entre Español e Inglés.
- **Multiplataforma:** Binario nativo ligero para **Windows**, **Linux (`amd64` / `arm64`)** y **Docker**.

## Inicio Rápido

### Windows
Ejecuta `livelink-hub.exe` con doble clic (o compila con `.\build.ps1`).

### Linux / Servidor VPS
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o livelink-hub .
./livelink-hub -headless -bind 0.0.0.0:8790
```
O instálalo como servicio `systemd` 24/7 con optimización de buffers UDP:
```bash
chmod +x install-linux.sh && ./install-linux.sh
```

### Docker
```bash
docker compose up -d --build
```

## Autores y Créditos
- **Autor:** Alan de Santiago ([@elyarim](https://github.com/elyarim))
- **Coautor:** CPAGallardo ([@CPAGallardo](https://github.com/CPAGallardo))
- Basado en los fundamentos originales de enrutamiento SRT de [Peter Aellig](https://github.com/peteraellig/srt-bridge).

## Licencia
Licencia MIT

