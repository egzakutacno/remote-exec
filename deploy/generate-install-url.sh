#!/bin/bash

TUNNEL_LOG="${TUNNEL_LOG:-/var/log/remote-exec-tunnel.log}"
JOURNAL_METHOD="${JOURNAL_METHOD:-false}"

if [ "$JOURNAL_METHOD" = "true" ] || [ ! -f "$TUNNEL_LOG" ]; then
    URL=$(journalctl -u remote-exec-tunnel --no-pager -n 200 2>/dev/null | grep -oP 'https://[^\s]+\.trycloudflare\.com' | tail -1)
else
    URL=$(grep -oP 'https://[^\s]+\.trycloudflare\.com' "$TUNNEL_LOG" 2>/dev/null | tail -1)
fi

if [ -z "$URL" ]; then
    echo "ERROR: Could not find tunnel URL."
    echo "Make sure cloudflared is running: systemctl status remote-exec-tunnel"
    exit 1
fi

echo "# One-line install command for Windows:"
echo ""
echo "powershell -c \"\$u='$URL'; irm \$u/install.ps1 | iex\""
echo ""
echo "# Or with custom machine name:"
echo "# powershell -c \"\$u='$URL'; \$n='PC-LIVING-ROOM'; irm \$u/install.ps1 | iex\""
