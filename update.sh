#!/usr/bin/env bash
# ==============================================================================
# GNNwatchtower - Safe In-Place Updater (Ubuntu 22.04 / 24.04 LTS)
# Port: 3800 | Preserves Data & Settings • Creates Pre-Update Backup
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
echo -e "${CYAN}    GNNwatchtower Guvenli Guncelleme (Update) Baslatiliyor       ${NC}"
echo -e "${CYAN}=================================================================${NC}"

# 1. Root Yetkisi Kontrolü
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[HATA] Bu guncelleme betigi root yetkisiyle calistirilmalidir (sudo bash update.sh)${NC}"
    exit 1
fi

INSTALL_DIR="/opt/gnnwatchtower"
BIN_TARGET="$INSTALL_DIR/gnnwatchtower"
REPO_URL="https://github.com/BigDesigner/project-gnnwatchtower.git"
BRANCH="main"

if [ ! -d "$INSTALL_DIR" ]; then
    echo -e "${RED}[HATA] $INSTALL_DIR kurulu degil. Lutfen once 'install.sh' ile kurulum yapiniz.${NC}"
    exit 1
fi

# 2. Otomatik Guvenlik Yedegi (Veritabanı ve .env asla kaybolmaz)
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="$INSTALL_DIR/backups"
mkdir -p "$BACKUP_DIR"

if [ -f "$INSTALL_DIR/data/gnnwatchtower.db" ]; then
    echo -e "${BLUE}[1/4] Mevcut veritabani guvenle yedekleniyor: pre-update-${TIMESTAMP}.db...${NC}"
    cp "$INSTALL_DIR/data/gnnwatchtower.db" "$BACKUP_DIR/pre-update-${TIMESTAMP}.db"
fi

if [ -f "$INSTALL_DIR/.env" ]; then
    cp "$INSTALL_DIR/.env" "$BACKUP_DIR/pre-update-${TIMESTAMP}.env"
fi

# 3. Servisi Gecici Olarak Durdur (Text file busy onlemi ve temiz SQLite WAL checkpoint)
echo -e "${BLUE}[2/4] GNNwatchtower servisi gecici olarak durduruluyor...${NC}"
systemctl stop gnnwatchtower 2>/dev/null || true

# 4. En Guncel Surumu Cek ve Derle
echo -e "${BLUE}[3/4] En guncel kodlar GitHub'dan indiriliyor ve derleniyor...${NC}"

MISSING_PKGS=()
if ! command -v git >/dev/null 2>&1; then MISSING_PKGS+=("git"); fi
if ! command -v go >/dev/null 2>&1; then MISSING_PKGS+=("golang-go"); fi
if ! command -v x86_64-w64-mingw32-windres >/dev/null 2>&1; then MISSING_PKGS+=("binutils-mingw-w64-x86-64"); fi
if [ ${#MISSING_PKGS[@]} -gt 0 ]; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq || true
    apt-get install -y -qq "${MISSING_PKGS[@]}"
fi

BUILD_DIR=$(mktemp -d -t gnnwatchtower-update-XXXXXX)
git clone --depth 1 -b "$BRANCH" "$REPO_URL" "$BUILD_DIR"

TEMP_BIN="$BUILD_DIR/gnnwatchtower"
(cd "$BUILD_DIR/backend" && go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$TEMP_BIN" ./cmd/server)

# Ajan ikili dosyalarini da guncelle (v0.4.1)
mkdir -p "$INSTALL_DIR/bin"
if command -v x86_64-w64-mingw32-windres >/dev/null 2>&1; then
    (cd "$BUILD_DIR/backend/cmd/agent" && x86_64-w64-mingw32-windres versioninfo.rc -O coff -o rsrc_windows_amd64.syso 2>/dev/null || true)
fi
(cd "$BUILD_DIR/backend" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X 'main.Version=0.4.1'" -o "$INSTALL_DIR/bin/gnnwatchtower-agent" ./cmd/agent)
(cd "$BUILD_DIR/backend" && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X 'main.Version=0.4.1'" -o "$INSTALL_DIR/bin/gnnwatchtower-agent.exe" ./cmd/agent)
chmod +x "$INSTALL_DIR/bin/gnnwatchtower-agent"

# Binary'yi guvenle yerlestir
install -m 755 "$TEMP_BIN" "$BIN_TARGET"
rm -rf "$BUILD_DIR"

# 5. Servisi Yeniden Baslat ve Kontrol Et
echo -e "${BLUE}[4/4] GNNwatchtower servisi yeniden baslatiliyor...${NC}"
systemctl daemon-reload
systemctl restart gnnwatchtower

sleep 2
if systemctl is-active --quiet gnnwatchtower; then
    SERVER_IP=$(hostname -I | awk '{print $1}' || echo "SUNUCU_IP")
    echo ""
    echo -e "${GREEN}=================================================================${NC}"
    echo -e "${GREEN}  ✓ GNNwatchtower Basariyla Guncellendi! (v0.4.1)               ${NC}"
    echo -e "${GREEN}  ✓ Tum ayarlar, cihazlar ve veritabani korundu.                ${NC}"
    echo -e "${GREEN}=================================================================${NC}"
    echo -e "Yonetim Paneli: ${CYAN}http://${SERVER_IP}:3800${NC}"
    echo -e "Yedek Konumu  : ${BLUE}$BACKUP_DIR/pre-update-${TIMESTAMP}.db${NC}"
    echo -e "Servis Durumu : ${GREEN}Aktif (Running)${NC}"
    echo -e "${GREEN}=================================================================${NC}"
else
    echo -e "${RED}[UYARI] Servis baslatilamadi. Loglar: journalctl -u gnnwatchtower -n 20${NC}"
    exit 1
fi
