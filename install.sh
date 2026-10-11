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

echo -e "${BLUE}[1/5] Dizin yapisi hazirlaniyor...${NC}"
mkdir -p "$INSTALL_DIR/data"
mkdir -p "$INSTALL_DIR/backups"
mkdir -p "$INSTALL_DIR/logs"
chmod 750 "$INSTALL_DIR"
chmod 700 "$INSTALL_DIR/data"
chmod 700 "$INSTALL_DIR/backups"

# 2. İkili Dosyayı (Binary) Hazırla
echo -e "${BLUE}[2/5] GNNwatchtower ikili dosyasi yerlestiriliyor...${NC}"
if [ -f "./bin/gnnwatchtower" ]; then
    cp "./bin/gnnwatchtower" "$BIN_TARGET"
elif [ -f "./gnnwatchtower" ]; then
    cp "./gnnwatchtower" "$BIN_TARGET"
elif command -v go >/dev/null 2>&1; then
    echo -e "${CYAN}Yerel derleme baslatiliyor (Go CGO_ENABLED=0)...${NC}"
    (cd backend && (go mod download 2>/dev/null || go mod tidy) && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BIN_TARGET" ./cmd/server)
else
    echo -e "${RED}[HATA] Derlenmis binary (./bin/gnnwatchtower) bulunamadi ve Go yuklu degil.${NC}"
    echo -e "Lutfen 'apt-get install -y golang-go' kurarak veya onceden derlenmis binary ile calistirin."
    exit 1
fi

chmod +x "$BIN_TARGET"

# 3. Ortam Değişkeni (.env) Dosyası
if [ ! -f "$INSTALL_DIR/.env" ]; then
    echo -e "${BLUE}[3/5] .env yapilandirma dosyasi olusturuluyor...${NC}"
    JWT_SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
    cat <<EOF > "$INSTALL_DIR/.env"
SERVER_PORT=3800
GIN_MODE=release
JWT_SECRET=${JWT_SECRET}
DEFAULT_SCAN_SUBNET=192.168.1.0/24
DEFAULT_CHECK_INTERVAL=15
EOF
    chmod 600 "$INSTALL_DIR/.env"
fi

# 4. Systemd Servisini Oluştur
echo -e "${BLUE}[4/5] Systemd servisi tanimlaniyor...${NC}"
cat <<EOF > "$SERVICE_FILE"
[Unit]
Description=GNNwatchtower Network Monitoring Appliance
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=-${INSTALL_DIR}/.env
ExecStart=${BIN_TARGET}
Restart=always
RestartSec=5s
LimitNOFILE=65535

# Guvenlik İzolasyon Sertlestirmesi
ProtectHome=true
ProtectSystem=full
ReadWritePaths=${INSTALL_DIR}

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable gnnwatchtower
systemctl restart gnnwatchtower

# 5. Güvenlik Duvarı (UFW)
if command -v ufw >/dev/null 2>&1; then
    ufw allow 3800/tcp comment 'GNNwatchtower Monitoring Appliance' >/dev/null 2>&1 || true
fi

# 6. Bilgilendirme
SERVER_IP=$(hostname -I | awk '{print $1}' || echo "SUNUCU_IP")

echo ""
echo -e "${GREEN}=================================================================${NC}"
echo -e "${GREEN}  ✓ GNNwatchtower Kurulumu Basariyla Tamamlandi!                 ${NC}"
echo -e "${GREEN}=================================================================${NC}"
echo -e "Yonetim Paneli Adresi: ${CYAN}http://${SERVER_IP}:3800${NC}"
echo -e "Ilk Giris Bilgileri  : Kullanici Adi: ${CYAN}admin${NC} | Parola: ${CYAN}admin${NC}"
echo -e "Durum Kontrolu       : ${BLUE}systemctl status gnnwatchtower${NC}"
echo -e "Canli Gunluk Kayit   : ${BLUE}journalctl -u gnnwatchtower -f${NC}"
echo -e "${GREEN}=================================================================${NC}"
