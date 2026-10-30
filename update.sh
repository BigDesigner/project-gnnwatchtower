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

# Refuse an unprepared HTTP-to-HTTPS migration before stopping the service.
if ! grep -q '^BIND_ADDRESS=.' "$INSTALL_DIR/.env"; then
    echo "[HATA] Once mevcut yonetim adresini BIND_ADDRESS olarak acikca tanimlayin."
    exit 1
fi
if ! grep -Eq '^TLS_ENABLED=(true|1)$' "$INSTALL_DIR/.env"; then
    echo "[HATA] Once docs/security-upgrade-0.4.12.md HTTPS gecisini tamamlayin."
    exit 1
fi
# Backup after a clean service stop, including WAL sidecars if present.
systemctl stop gnnwatchtower

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="$INSTALL_DIR/backups"
mkdir -p "$BACKUP_DIR"

if [ -f "$INSTALL_DIR/data/gnnwatchtower.db" ]; then
    echo -e "${BLUE}[1/4] Mevcut veritabani guvenle yedekleniyor: pre-update-${TIMESTAMP}.db...${NC}"
    cp "$INSTALL_DIR/data/gnnwatchtower.db" "$BACKUP_DIR/pre-update-${TIMESTAMP}.db"
    for suffix in -wal -shm; do
        if [ -f "$INSTALL_DIR/data/gnnwatchtower.db$suffix" ]; then cp "$INSTALL_DIR/data/gnnwatchtower.db$suffix" "$BACKUP_DIR/pre-update-${TIMESTAMP}.db$suffix"; fi
    done
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

APP_VERSION=$(cat "$BUILD_DIR/VERSION" 2>/dev/null || cat VERSION 2>/dev/null || echo "0.4.12")
cp "$BUILD_DIR/VERSION" "$INSTALL_DIR/VERSION" 2>/dev/null || true

TEMP_BIN="$BUILD_DIR/gnnwatchtower"
(cd "$BUILD_DIR/backend" && go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w -X 'gnnwatchtower/internal/version.Version=${APP_VERSION}'" -o "$TEMP_BIN" ./cmd/server)

# Ajan ikili dosyalarini da guncelle (v${APP_VERSION})
mkdir -p "$INSTALL_DIR/bin"
if command -v x86_64-w64-mingw32-windres >/dev/null 2>&1; then
    (cd "$BUILD_DIR/backend/cmd/agent" && x86_64-w64-mingw32-windres versioninfo.rc -O coff -o rsrc_windows_amd64.syso 2>/dev/null || true)
fi
(cd "$BUILD_DIR/backend" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X 'gnnwatchtower/internal/version.Version=${APP_VERSION}' -X 'main.Version=${APP_VERSION}'" -o "$INSTALL_DIR/bin/gnnwatchtower-agent" ./cmd/agent)
(cd "$BUILD_DIR/backend" && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X 'gnnwatchtower/internal/version.Version=${APP_VERSION}' -X 'main.Version=${APP_VERSION}'" -o "$INSTALL_DIR/bin/gnnwatchtower-agent.exe" ./cmd/agent)
chmod +x "$INSTALL_DIR/bin/gnnwatchtower-agent"

# Binary'yi guvenle yerlestir
install -m 755 "$TEMP_BIN" "$BIN_TARGET"
bash "$BUILD_DIR/scripts/harden-collector.sh"
rm -rf "$BUILD_DIR"

# .env icinde DB_PATH tanimini garantiye al
if [ -f "$INSTALL_DIR/.env" ] && ! grep -q "DB_PATH=" "$INSTALL_DIR/.env"; then
    echo "DB_PATH=$INSTALL_DIR/data/gnnwatchtower.db" >> "$INSTALL_DIR/.env"
fi

# Eger yerel sunucu uzerinde ajan kuruluysa, ikili dosyayi ve signer.pub anahtarini otomatik guncelle
if [ -d "/opt/gnnwatchtower-agent" ]; then
    systemctl stop gnnwatchtower-agent 2>/dev/null || true
    install -m 755 "$INSTALL_DIR/bin/gnnwatchtower-agent" "/opt/gnnwatchtower-agent/gnnwatchtower-agent"
    systemctl start gnnwatchtower-agent 2>/dev/null || true
fi

# 5. Servisi Yeniden Baslat ve Kontrol Et
echo -e "${BLUE}[4/4] GNNwatchtower servisi yeniden baslatiliyor...${NC}"
systemctl daemon-reload
systemctl restart gnnwatchtower

sleep 2
if systemctl is-active --quiet gnnwatchtower; then
    SERVER_IP=$(hostname -I | awk '{print $1}' || echo "SUNUCU_IP")
    echo ""
    echo -e "${GREEN}=================================================================${NC}"
    echo -e "${GREEN}  ✓ GNNwatchtower Basariyla Guncellendi! (v${APP_VERSION})      ${NC}"
    echo -e "${GREEN}  ✓ Tum ayarlar, cihazlar ve veritabani korundu.                ${NC}"
    echo -e "${GREEN}=================================================================${NC}"
    echo -e "Yonetim Paneli: ${CYAN}https://${SERVER_IP}:3800${NC}"
    echo -e "Yedek Konumu  : ${BLUE}$BACKUP_DIR/pre-update-${TIMESTAMP}.db${NC}"
    echo -e "Servis Durumu : ${GREEN}Aktif (Running)${NC}"
    echo -e "${GREEN}=================================================================${NC}"
else
    echo -e "${RED}[UYARI] Servis baslatilamadi. Loglar: journalctl -u gnnwatchtower -n 20${NC}"
    exit 1
fi