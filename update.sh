#!/usr/bin/env bash
# ==============================================================================
# GNNwatchtower - Turnkey Appliance Updater (Ubuntu 22.04 / 24.04 LTS)
# Port: 3800 | Architecture: Single Binary • Pure Go SQLite WAL
# License: GNU Affero General Public License v3 (AGPL-3.0)
# ==============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${CYAN}=================================================================${NC}"
echo -e "${CYAN}    GNNwatchtower Appliance Guncellemesi Baslatiliyor           ${NC}"
echo -e "${CYAN}=================================================================${NC}"

# 1. Root Yetkisi Kontrolü
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[HATA] Bu guncelleme betigi root yetkisiyle calistirilmalidir (sudo bash update.sh)${NC}"
    exit 1
fi

INSTALL_DIR="/opt/gnnwatchtower"
BIN_TARGET="$INSTALL_DIR/gnnwatchtower"
SERVICE_NAME="gnnwatchtower"
REPO_URL="https://github.com/BigDesigner/project-gnnwatchtower.git"
BRANCH="main"

# 2. Mevcut Kurulum Kontrolü
if [ ! -d "$INSTALL_DIR" ] || [ ! -f "$BIN_TARGET" ]; then
    echo -e "${RED}[HATA] GNNwatchtower kurulumu bulunamadi ($INSTALL_DIR).${NC}"
    echo -e "${YELLOW}Lutfen once install.sh betigi ile kurulum yapiniz.${NC}"
    exit 1
fi

# 3. Otomatik Veritabanı Yedeği (Güncelleme Öncesi Emniyet)
BACKUP_DIR="$INSTALL_DIR/backups"
DB_FILE="$INSTALL_DIR/data/gnnwatchtower.db"
mkdir -p "$BACKUP_DIR"
mkdir -p "$INSTALL_DIR/bin"

if [ -f "$DB_FILE" ]; then
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    BACKUP_FILE="$BACKUP_DIR/gnnwatchtower_pre_update_${TIMESTAMP}.db"
    echo -e "${BLUE}[1/4] Guncelleme oncesi veritabani yedegi aliniyor...${NC}"
    cp "$DB_FILE" "$BACKUP_FILE"
    chmod 600 "$BACKUP_FILE"
    echo -e "${GREEN}  ✓ Guvenlik yedegi alindi: $BACKUP_FILE${NC}"
fi

# 4. En Son Sürümün Alınması ve Derlenmesi
echo -e "${BLUE}[2/4] Guncel kaynak kodlar indiriliyor ve derleniyor...${NC}"

MISSING_PKGS=()
if ! command -v git >/dev/null 2>&1; then MISSING_PKGS+=("git"); fi
if ! command -v go >/dev/null 2>&1; then MISSING_PKGS+=("golang-go"); fi

if [ ${#MISSING_PKGS[@]} -gt 0 ]; then
    echo -e "${BLUE}Gerekli sistem paketleri yukleniyor: ${MISSING_PKGS[*]}...${NC}"
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq || true
    apt-get install -y -qq "${MISSING_PKGS[@]}"
fi

BUILD_DIR=$(mktemp -d -t gnnwatchtower-update-XXXXXX)
git clone --depth 1 -b "$BRANCH" "$REPO_URL" "$BUILD_DIR"

echo -e "${CYAN}Gömülü Web UI ile birlikte tek ikili dosya derleniyor...${NC}"
(cd "$BUILD_DIR/backend" && go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BUILD_DIR/gnnwatchtower" ./cmd/server)

echo -e "${CYAN}Mini-Ajan ikili dosyalari derleniyor (Linux & Windows)...${NC}"
(cd "$BUILD_DIR/backend" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$INSTALL_DIR/bin/gnnwatchtower-agent" ./cmd/agent)
(cd "$BUILD_DIR/backend" && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$INSTALL_DIR/bin/gnnwatchtower-agent.exe" ./cmd/agent)
chmod +x "$INSTALL_DIR/bin/gnnwatchtower-agent"

# 5. İkili Dosyanın Değiştirilmesi ve Servisin Yeniden Başlatılması
echo -e "${BLUE}[3/4] Yeni ikili dosya sisteme yerlestiriliyor...${NC}"
install -m 755 "$BUILD_DIR/gnnwatchtower" "$BIN_TARGET"
rm -rf "$BUILD_DIR"

echo -e "${BLUE}[4/4] Servis yeniden baslatiliyor...${NC}"
systemctl daemon-reload
systemctl restart "$SERVICE_NAME"
sleep 2

# 6. Sağlık Kontrolü (Health Check)
if systemctl is-active --quiet "$SERVICE_NAME"; then
    SERVER_IP=$(hostname -I | awk '{print $1}' || echo "SUNUCU_IP")
    echo ""
    echo -e "${GREEN}=================================================================${NC}"
    echo -e "${GREEN}  ✓ GNNwatchtower Basariyla Guncellendi ve Calisiyor!             ${NC}"
    echo -e "${GREEN}=================================================================${NC}"
    echo -e "Yonetim Paneli Adresi: ${CYAN}http://${SERVER_IP}:3800${NC}"
    echo -e "Servis Durumu        : ${BLUE}systemctl status gnnwatchtower${NC}"
    echo -e "${GREEN}=================================================================${NC}"
else
    echo -e "${RED}[HATA] Guncelleme sonrasi servis baslatilamadi!${NC}"
    journalctl -u "$SERVICE_NAME" -n 25 --no-pager
    exit 1
fi
