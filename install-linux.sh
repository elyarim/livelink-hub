#!/usr/bin/env bash
set -euo pipefail

echo "=== LiveLink Hub Linux Installer (Ubuntu/Debian) ==="
echo "1. Tuning kernel UDP buffers for stable multi-channel SRT..."
sudo tee /etc/sysctl.d/99-livelink-hub.conf >/dev/null <<'EOF'
net.core.rmem_max=26214400
net.core.wmem_max=26214400
net.core.rmem_default=4194304
net.core.wmem_default=4194304
EOF
sudo sysctl --system >/dev/null

echo "2. Installing systemd service..."
BIN_DIR="$(cd "$(dirname "$0")" && pwd)"
sudo tee /etc/systemd/system/livelink-hub.service >/dev/null <<EOF
[Unit]
Description=LiveLink Hub Broadcast SRT/MediaMTX Router
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${BIN_DIR}
ExecStart=${BIN_DIR}/livelink-hub -headless -bind 0.0.0.0:8790 -config ${BIN_DIR}/config.json -log ${BIN_DIR}/livelink-hub.log
Restart=always
RestartSec=2
LimitNOFILE=65536
Nice=-10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now livelink-hub.service
echo "Done! Open http://$(hostname -I | awk '{print $1}'):8790 in your browser."
