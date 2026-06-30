# AI Remote Execution System

Raspberry Pi kontrolise vise Windows masina iza NAT-a preko HTTPS long-polling protokola.

## Arhitektura

```
[Windows Agent PC-01]──outbound HTTPS──┐
[Windows Agent PC-02]──outbound HTTPS──┤
...                                    ├── Cloudflare Tunnel ── [Pi: FastAPI Server :9990]
[Windows Agent PC-N]──outbound HTTPS──┘               ↑
                                              [OpenCode AI]
```

- **Pi** hostuje FastAPI server + SQLite bazu
- **Cloudflare Tunnel** (besplatan `cloudflared tunnel`) izlaze Pi server na internet
- **Windows agent** (Go) polluje `GET /next-task?wait=60`, izvrsava komande lokalno
- **Nema SSH, port forwardinga, VPN-a, VPS-a**

## Folder struktura

```
server/                     # Pi: FastAPI
  main.py                   # Entry point
  config.py                 # Settings
  database.py               # SQLite (machines, tasks)
  models.py                 # Pydantic
  auth.py                   # API key auth
  router_agent.py           # Agent endpoints (poll, result, register)
  router_control.py         # Control endpoints (create task, kill, list)
  command_validator.py      # Allowlist/denylist
  requirements.txt          # Python deps
  static/
    agent.exe               # Windows agent binary (pre-built)
    install.ps1             # One-line bootstrap installer
    index.html              # Landing page

agent/                      # Windows: Go agent
  go.mod
  main.go                   # Entry (service/console mode)
  config.go                 # Config loader
  poller.go                 # Long-polling loop
  executor.go               # Interface
  executor_windows.go       # PowerShell, CMD execution
  service.go                # Windows service

deploy/
  install-pi.sh             # Pi full setup (server + systemd + tunnel)
  deploy-agent.ps1          # Manual Windows deploy
  generate-install-url.sh   # Print one-line install command
```

## Komande

| Akcija | Payload | Validacija |
|--------|---------|------------|
| `ping` | - | health check |
| `run_powershell` | PS skripta | denylist: Remove-Item, Format-Volume, Shutdown |
| `run_cmd` | CMD komanda | denylist: format, diskpart, shutdown |
| `restart_service` | Ime servisa | allowlist: hermes, spooler, wuauserv... |
| `file_read` | Path | Samo u dozvoljenim direktorijumima |
| `file_write` | Path | Path validacija, max 10MB |
| `file_delete` | Path | U temp direktorijumu |
| `install_package` | Package name | allowlist |
| `kill` | - | Gasi agenta |

## Security

- HTTPS + Cloudflare TLS
- API key auth (X-API-Key, X-Control-Key header)
- Command validation (allowlist/denylist)
- Timeout po tasku
- Kill switch
- Svi taskovi logovani u SQLite

## Deployment

### Pi (server)

```bash
cd deploy
sudo ./install-pi.sh
```

Ovo radi:
1. Kopira server Python fajlove u /opt/remote-exec
2. Instalira Python dependencies (fastapi, uvicorn, aiosqlite, pydantic)
3. Kreira systemd servis `remote-exec-server`
4. Kreira systemd servis `remote-exec-tunnel` (cloudflared)
5. Generise CONTROL_KEY

**Cloudflare tunnel URL:**
```bash
systemctl status remote-exec-tunnel | grep trycloudflare.com
# ili:
./generate-install-url.sh
```

### Windows (agent) — One-line install

**Najjednostavnije — jedan PowerShell komand:**
```powershell
powershell -c "$u='https://TUNNEL_URL'; irm $u/install.ps1 | iex"
```

Ovo instalira agent kao Windows Service, pravi config, registruje na serveru.

**Manual deploy:**
```powershell
.\deploy-agent.ps1 -ServerUrl "https://TUNNEL_URL" -MachineId "pc-living-room"
```

### Build agent-a (potrebno samo kad menjas Go kod)

```bash
cd agent
GOOS=windows GOARCH=amd64 go build -o ../server/static/agent.exe .
```

## API

### Agent routes (auth: X-Machine-Id + X-API-Key)

| Metod | Path | Opis |
|-------|------|------|
| POST | /api/v1/agent/register | Registracija |
| GET | /api/v1/agent/next-task?wait=60 | Long poll za sledeci task |
| POST | /api/v1/agent/result | Submit task rezultata |
| GET | /api/v1/agent/ping | Health check |

### Control routes (auth: X-Control-Key)

| Metod | Path | Opis |
|-------|------|------|
| GET | /api/v1/control/health | Server health |
| GET | /api/v1/control/machines | List masina |
| POST | /api/v1/control/tasks | Kreiraj task |
| GET | /api/v1/control/tasks | List taskova |
| POST | /api/v1/control/kill/{id} | Kill switch |

### Protokol

**Komanda:**
```json
{"machine_id":"pc-01","action":"run_powershell","payload":"Get-Process","timeout":30}
```

**Odgovor:**
```json
{"task_id":"uuid","status":"success","output":"...","error":null,"exit_code":0}
```

## Quick test

```bash
# Start server
cd server
pip3 install -r requirements.txt
REMOTE_EXEC_PORT=9990 python3 main.py &

# Register machine
curl -X POST http://127.0.0.1:9990/api/v1/agent/register \
  -H "Content-Type: application/json" \
  -d '{"name":"test","api_key":"mysecretkey1234567890abcdeffedcba","hostname":"TEST"}'

# Create task
curl -X POST http://127.0.0.1:9990/api/v1/control/tasks \
  -H "X-Control-Key: control-key-change-me" \
  -H "Content-Type: application/json" \
  -d '{"machine_id":"<MID>","action":"ping"}'

# Poll as agent
curl -H "X-Machine-Id: <MID>" -H "X-API-Key: mysecret..." \
  "http://127.0.0.1:9990/api/v1/agent/next-task?wait=5"
```
