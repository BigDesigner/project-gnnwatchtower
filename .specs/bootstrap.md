# Bootstrap Specification (GNNwatchtower)

## 1. Environment & Prerequisites
- **Go**: 1.22+ (Verified via `backend/go.mod`)
- **Target OS**: Linux (Ubuntu 22.04 / 24.04 LTS target appliance), development supported on Windows/macOS.
- **Node.js & npm** (Frontend build phase only): Node 20 LTS, npm/pnpm for Vite frontend compilation.
- **CGO Requirement**: None (CGO_ENABLED=0 pure Go SQLite).

## 2. Directory Layout & Storage
- **Binary Location**: `/opt/gnnwatchtower/gnnwatchtower` (Verified)
- **Data Directory**: `/opt/gnnwatchtower/data/` (Contains `gnnwatchtower.db` SQLite database) (Verified)
- **Backup Directory**: `/opt/gnnwatchtower/backups/` (Nightly 03:00 backups) (Verified)
- **Logs Directory**: `/opt/gnnwatchtower/logs/` or Systemd journal (`journalctl -u gnnwatchtower -f`) (Verified)

## 3. Installation & Build Workflow
```bash
# 1. Clone repository
git clone https://github.com/BigDesigner/project-gnnwatchtower.git
cd project-gnnwatchtower

# 2. Build Frontend (when ready)
cd frontend
npm install
npm run build
cd ..

# 3. Build Single Go Binary with embedded assets
cd backend
go mod download
CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/gnnwatchtower ./cmd/server
cd ..
```

## 4. Environment Variables (.env)
Copy template:
```bash
cp .env.example .env
```
Key variables:
- `SERVER_PORT`: Default `3800`
- `GIN_MODE`: `release`
- `JWT_SECRET`: Random 32+ character secret
- `DEFAULT_SCAN_SUBNET`: Local CIDR (e.g. `192.168.1.0/24`)

## 5. Systemd Service Definition
`/etc/systemd/system/gnnwatchtower.service`:
```ini
[Unit]
Description=GNNwatchtower Network Monitoring Appliance
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/gnnwatchtower
ExecStart=/opt/gnnwatchtower/gnnwatchtower
Restart=always
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```
