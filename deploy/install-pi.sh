#!/bin/bash
set -e

INSTALL_DIR="${INSTALL_DIR:-/opt/remote-exec}"
SERVICE_NAME="remote-exec-server"
GENERATE_KEY="${GENERATE_KEY:-}"

echo "=== AI Remote Execution — Pi Server Install ==="
echo "Install dir: $INSTALL_DIR"
echo ""

if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: run as root (sudo ./install-pi.sh)"
    exit 1
fi

check_cmd() {
    if ! command -v "$1" &>/dev/null; then
        echo "ERROR: $1 not installed. Install it first."
        exit 1
    fi
}

echo "[1/5] Checking dependencies..."
check_cmd python3
check_cmd pip3
check_cmd cloudflared

echo "[2/6] Creating directories..."
mkdir -p "$INSTALL_DIR/data"
cp -r server/{*.py,requirements.txt,static} "$INSTALL_DIR/"

echo "[3/6] Creating Python virtual environment..."
python3 -m venv "$INSTALL_DIR/venv"
"$INSTALL_DIR/venv/bin/pip" install -r "$INSTALL_DIR/requirements.txt"

echo "[4/6] Setting up systemd service..."
if [ -z "$GENERATE_KEY" ]; then
    GENERATE_KEY=$(python3 -c "import secrets; print(secrets.token_hex(16))")
fi

mkdir -p /etc/remote-exec

cat > /etc/remote-exec/server.env <<EOF
REMOTE_EXEC_PORT=9990
REMOTE_EXEC_DB=$INSTALL_DIR/data/remote_exec.db
REMOTE_EXEC_CONTROL_KEY=$GENERATE_KEY
REMOTE_EXEC_LOG_LEVEL=INFO
EOF

cat > /etc/systemd/system/$SERVICE_NAME.service <<EOF
[Unit]
Description=AI Remote Execution Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
EnvironmentFile=/etc/remote-exec/server.env
ExecStart=$INSTALL_DIR/venv/bin/python -m uvicorn main:app --host 127.0.0.1 --port 9990
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

echo "[5/6] Setting up cloudflared auto-tunnel..."
cat > /etc/systemd/system/remote-exec-tunnel.service <<EOF
[Unit]
Description=Cloudflare Tunnel for Remote Exec
After=remote-exec-server.service
Requires=remote-exec-server.service

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/cloudflared tunnel --url http://127.0.0.1:9990 --protocol http2 --no-autoupdate
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable remote-exec-tunnel
systemctl restart remote-exec-tunnel

echo "[6/6] Copying helper scripts..."
cp deploy/generate-install-url.sh "$INSTALL_DIR/"
cp deploy/deploy-agent.ps1 "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/generate-install-url.sh"

echo ""
echo "=== INSTALL COMPLETE ==="

CONTROL_KEY=$(grep REMOTE_EXEC_CONTROL_KEY /etc/remote-exec/server.env | cut -d= -f2)
echo "Control API Key: $CONTROL_KEY"
echo ""
echo "Waiting for tunnel URL..."
sleep 5
echo ""

# Try to get tunnel URL
TUNNEL_URL=$(journalctl -u remote-exec-tunnel --no-pager -n 200 2>/dev/null | grep -oP 'https://[^\s]+\.trycloudflare\.com' | tail -1)

if [ -n "$TUNNEL_URL" ]; then
    echo "Tunnel URL: $TUNNEL_URL"
    echo ""
    echo "=== ONE-LINE INSTALL (run on Windows) ==="
    echo "powershell -c \"\$u='$TUNNEL_URL'; irm \$u/install.ps1 | iex\""
else
    echo "Tunnel URL not found yet. Check:"
    echo "  journalctl -u remote-exec-tunnel --no-pager -n 50 | grep trycloudflare"
fi

echo ""
echo "Services:"
echo "  systemctl status $SERVICE_NAME"
echo "  systemctl status remote-exec-tunnel"
echo ""
echo "Health check:"
echo "  curl -H 'X-Control-Key: $CONTROL_KEY' http://127.0.0.1:9990/api/v1/control/health"
echo ""
echo "One-line install command (when tunnel is up):"
echo "  cd $INSTALL_DIR && ./generate-install-url.sh"
