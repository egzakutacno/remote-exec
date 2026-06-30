#!/bin/bash
TOKEN=$(cat /home/ruter/.github-token)
GIST_ID="0c3de11a3381ae878b09626b306d04d1"
CACHE=/var/lib/remote-exec/tunnel-url.txt

URL=$(journalctl -u remote-exec-tunnel --no-pager -n 100 2>/dev/null | grep -oP 'https://[^\s]+\.trycloudflare\.com' | tail -1)

if [ -z "$URL" ]; then
    exit 0
fi

if [ -f "$CACHE" ] && [ "$(cat $CACHE)" = "$URL" ]; then
    exit 0
fi

echo "$URL" > "$CACHE"

PAYLOAD=$(python3 -c "
import json
print(json.dumps({
    'description': 'AI Remote Exec tunnel registry',
    'files': {'tunnel-url.txt': {'content': '$URL'}}
}))
")

curl -s -X PATCH -H "Authorization: token $TOKEN" -H "Content-Type: application/json" \
    -d "$PAYLOAD" "https://api.github.com/gists/$GIST_ID" > /dev/null
