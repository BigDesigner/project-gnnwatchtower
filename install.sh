#!/usr/bin/env bash
# ==============================================================================
# GNNwatchtower - Turnkey Appliance Installer (Ubuntu 22.04 / 24.04 LTS)
# Port: 3800 | Architecture: Single Binary • Pure Go SQLite WAL
# License: GNU Affero General Public License v3 (AGPL-3.0)
# ==============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}=================================================================${NC}"
echo -e "${CYAN}    GNNwatchtower Appliance Kurulumu Baslatiliyor (Port 3800)    ${NC}"
echo -e "${CYAN}=================================================================${NC}"

# 1. Root Yetkisi Kontrolü
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[HATA] Bu kurulum betigi root yetkisiyle calistirilmalidir (sudo bash install.sh)${NC}"
    exit 1
fi

INSTALL_DIR="/opt/gnnwatchtower"
BIN_TARGET="$INSTALL_DIR/gnnwatchtower"
SERVICE_FILE="/etc/systemd/system/gnnwatchtower.service"

echo -e "${BLUE}[1/5] Dizin yapisi ve guvenlik hazirlaniyor...${NC}"
mkdir -p "$INSTALL_DIR/data"
mkdir -p "$INSTALL_DIR/backups"
mkdir -p "$INSTALL_DIR/logs"
chmod 750 "$INSTALL_DIR"
chmod 700 "$INSTALL_DIR/data"
chmod 700 "$INSTALL_DIR/backups"

# Mevcut kurulum varsa otomatik yedek al ve servisi durdur
if [ -f "$INSTALL_DIR/data/gnnwatchtower.db" ]; then
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    echo -e "${CYAN}Mevcut veritabani koruma altina aliniyor: pre-install-${TIMESTAMP}.db...${NC}"
    cp "$INSTALL_DIR/data/gnnwatchtower.db" "$INSTALL_DIR/backups/pre-install-${TIMESTAMP}.db"
elif [ -f "./data/gnnwatchtower.db" ]; then
    echo -e "${CYAN}Yerel test veritabani tespit edildi, /opt/gnnwatchtower altina aktariliyor...${NC}"
    cp "./data/gnnwatchtower.db" "$INSTALL_DIR/data/gnnwatchtower.db"
fi

if systemctl is-active --quiet gnnwatchtower 2>/dev/null; then
    echo -e "${CYAN}Calisan servis gecici olarak durduruluyor...${NC}"
    systemctl stop gnnwatchtower 2>/dev/null || true
fi

# 2. İkili Dosyayı (Binary) Hazırla
echo -e "${BLUE}[2/5] GNNwatchtower ikili dosyasi hazirlaniyor...${NC}"
REPO_URL="https://github.com/BigDesigner/project-gnnwatchtower.git"
BRANCH="main"

if [ -f "./bin/gnnwatchtower" ]; then
    echo -e "${CYAN}Mevcut ikili dosya (./bin/gnnwatchtower) kopyalaniyor...${NC}"
    cp "./bin/gnnwatchtower" "$BIN_TARGET"
elif [ -f "./gnnwatchtower" ]; then
    echo -e "${CYAN}Mevcut ikili dosya (./gnnwatchtower) kopyalaniyor...${NC}"
    cp "./gnnwatchtower" "$BIN_TARGET"
elif [ -d "./backend" ] && command -v go >/dev/null 2>&1; then
    echo -e "${CYAN}Yerel kaynak koddan derleme baslatiliyor (Go CGO_ENABLED=0)...${NC}"
    (cd backend && go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BIN_TARGET" ./cmd/server)
    mkdir -p "$INSTALL_DIR/bin"
    (cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$INSTALL_DIR/bin/gnnwatchtower-agent" ./cmd/agent)
    (cd backend && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$INSTALL_DIR/bin/gnnwatchtower-agent.exe" ./cmd/agent)
    chmod +x "$INSTALL_DIR/bin/gnnwatchtower-agent"
else
    echo -e "${CYAN}GitHub uzerinden bagimsiz tek satir kurulum modu algilandi...${NC}"
    
    # Gerekli sistem araclarini kontrol et ve eksikse kur (git, curl, golang-go)
    MISSING_PKGS=()
    if ! command -v git >/dev/null 2>&1; then MISSING_PKGS+=("git"); fi
    if ! command -v curl >/dev/null 2>&1; then MISSING_PKGS+=("curl"); fi
    if ! command -v go >/dev/null 2>&1; then MISSING_PKGS+=("golang-go"); fi
    if ! command -v x86_64-w64-mingw32-windres >/dev/null 2>&1; then MISSING_PKGS+=("binutils-mingw-w64-x86-64"); fi
    
    if [ ${#MISSING_PKGS[@]} -gt 0 ]; then
        echo -e "${BLUE}Gerekli sistem paketleri yukleniyor: ${MISSING_PKGS[*]}...${NC}"
        export DEBIAN_FRONTEND=noninteractive
        apt-get update -qq || true
        apt-get install -y -qq "${MISSING_PKGS[@]}"
    fi

    BUILD_DIR=$(mktemp -d -t gnnwatchtower-build-XXXXXX)
    echo -e "${CYAN}Gecici derleme dizini olusturuldu: $BUILD_DIR${NC}"
    
    echo -e "${BLUE}GNNwatchtower kaynak kodlari GitHub'dan klonlaniyor ($BRANCH)...${NC}"
    git clone --depth 1 -b "$BRANCH" "$REPO_URL" "$BUILD_DIR"
    
    APP_VERSION=$(cat "$BUILD_DIR/VERSION" 2>/dev/null || cat VERSION 2>/dev/null || echo "0.4.12")
    cp "$BUILD_DIR/VERSION" "$INSTALL_DIR/VERSION" 2>/dev/null || true

    echo -e "${CYAN}Gömülü Web UI ile birlikte tek ikili dosya derleniyor (v${APP_VERSION})...${NC}"
    (cd "$BUILD_DIR/backend" && go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w -X 'gnnwatchtower/internal/version.Version=${APP_VERSION}'" -o "$BUILD_DIR/gnnwatchtower" ./cmd/server)
    install -m 755 "$BUILD_DIR/gnnwatchtower" "$BIN_TARGET"
    
    echo -e "${CYAN}Kurumsal Telemetri Ajan ikili dosyalari derleniyor (Linux & Windows - v${APP_VERSION})...${NC}"
    mkdir -p "$INSTALL_DIR/bin"
    if command -v x86_64-w64-mingw32-windres >/dev/null 2>&1; then
        (cd "$BUILD_DIR/backend/cmd/agent" && x86_64-w64-mingw32-windres versioninfo.rc -O coff -o rsrc_windows_amd64.syso 2>/dev/null || true)
    fi
    (cd "$BUILD_DIR/backend" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X 'gnnwatchtower/internal/version.Version=${APP_VERSION}' -X 'main.Version=${APP_VERSION}'" -o "$INSTALL_DIR/bin/gnnwatchtower-agent" ./cmd/agent)
    (cd "$BUILD_DIR/backend" && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X 'gnnwatchtower/internal/version.Version=${APP_VERSION}' -X 'main.Version=${APP_VERSION}'" -o "$INSTALL_DIR/bin/gnnwatchtower-agent.exe" ./cmd/agent)
    chmod +x "$INSTALL_DIR/bin/gnnwatchtower-agent"
    
    rm -rf "$BUILD_DIR"
fi

chmod +x "$BIN_TARGET"

# 3. Ortam Değişkeni (.env) Dosyası
if [ ! -f "$INSTALL_DIR/.env" ]; then
    echo -e "${BLUE}[3/5] .env yapilandirma dosyasi olusturuluyor...${NC}"
    cat <<EOF > "$INSTALL_DIR/.env"
SERVER_PORT=3800
DB_PATH=/opt/gnnwatchtower/data/gnnwatchtower.db
GIN_MODE=release
TLS_ENABLED=true
BIND_ADDRESS=127.0.0.1
DEFAULT_CHECK_INTERVAL=15
EOF
    chmod 600 "$INSTALL_DIR/.env"
fi

# Dedicated collector identity; only mutable directories are writable.
id -u gnnwatchtower >/dev/null 2>&1 || useradd --system --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin gnnwatchtower
chown root:gnnwatchtower "$INSTALL_DIR"
chown -R gnnwatchtower:gnnwatchtower "$INSTALL_DIR/data" "$INSTALL_DIR/backups" "$INSTALL_DIR/logs"
# 4. Systemd Servisini Oluştur
echo -e "${BLUE}[4/5] Systemd servisi tanimlaniyor...${NC}"
cat <<EOF > "$SERVICE_FILE"
[Unit]
Description=GNNwatchtower Network Monitoring Appliance
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=gnnwatchtower
Group=gnnwatchtower
NoNewPrivileges=true
CapabilityBoundingSet=
PrivateTmp=true
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=-${INSTALL_DIR}/.env
ExecStart=${BIN_TARGET}
Restart=always
RestartSec=5s
LimitNOFILE=65535

# Guvenlik İzolasyon Sertlestirmesi
ProtectHome=true
ProtectSystem=full
ReadWritePaths=${INSTALL_DIR}/data ${INSTALL_DIR}/backups ${INSTALL_DIR}/logs

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable gnnwatchtower
systemctl restart gnnwatchtower

# No firewall rules are added.
# 6. Bilgilendirme
SERVER_IP=$(hostname -I | awk '{print $1}' || echo "SUNUCU_IP")

echo ""
echo -e "${GREEN}=================================================================${NC}"
echo -e "${GREEN}  ✓ GNNwatchtower Kurulumu Basariyla Tamamlandi!                 ${NC}"
echo -e "${GREEN}=================================================================${NC}"
echo -e "Yonetim Paneli Adresi: ${CYAN}https://localhost:3800${NC}"
echo "Ilk parola: sudo cat /opt/gnnwatchtower/data/initial_admin_password.txt"
echo -e "Durum Kontrolu       : ${BLUE}systemctl status gnnwatchtower${NC}"
echo -e "Canli Gunluk Kayit   : ${BLUE}journalctl -u gnnwatchtower -f${NC}"
echo -e "${GREEN}=================================================================${NC}"
