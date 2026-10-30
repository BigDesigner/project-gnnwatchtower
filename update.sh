#!/usr/bin/env bash
# Build while the existing service runs. Cut over only after every binary is ready.
set -euo pipefail
if [ "$EUID" -ne 0 ]; then echo '[HATA] sudo bash ile calistirin.'; exit 1; fi
INSTALL_DIR=/opt/gnnwatchtower
if [ ! -f "$INSTALL_DIR/gnnwatchtower" ]; then echo '[HATA] Once install.sh ile kurulum yapin.'; exit 1; fi
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BUILD_DIR=$(mktemp -d -t watchtower-update-XXXXXX)
BACKUP_DIR="$INSTALL_DIR/backups/pre-update-$TIMESTAMP"
STOPPED=0
SWAPPED=0
cleanup() {
    result=$?
    if [ "$result" -ne 0 ] && [ "$STOPPED" = 1 ]; then
        cp "$BACKUP_DIR/environment" "$INSTALL_DIR/.env"
        if [ "$SWAPPED" = 1 ]; then
            cp "$BACKUP_DIR/gnnwatchtower" "$INSTALL_DIR/gnnwatchtower.rollback"
            chmod 755 "$INSTALL_DIR/gnnwatchtower.rollback"
            mv -f "$INSTALL_DIR/gnnwatchtower.rollback" "$INSTALL_DIR/gnnwatchtower"
            cp "$BACKUP_DIR/environment" "$INSTALL_DIR/.env"
            cp "$BACKUP_DIR/VERSION" "$INSTALL_DIR/VERSION" 2>/dev/null || true
            if [ -f "$BACKUP_DIR/security.conf" ]; then
                cp "$BACKUP_DIR/security.conf" /etc/systemd/system/gnnwatchtower.service.d/security.conf
            else
                rm -f /etc/systemd/system/gnnwatchtower.service.d/security.conf
            fi
        fi
        systemctl daemon-reload
        systemctl start gnnwatchtower || true
        echo '[HATA] Guncelleme tamamlanamadi; onceki sunucu yeniden baslatildi. Gunluk: journalctl -u gnnwatchtower -n 30'
    fi
    rm -rf "$BUILD_DIR"
    exit "$result"
}
trap cleanup EXIT
MISSING=()
command -v git >/dev/null 2>&1 || MISSING+=(git)
command -v go >/dev/null 2>&1 || MISSING+=(golang-go)
command -v curl >/dev/null 2>&1 || MISSING+=(curl)
command -v x86_64-w64-mingw32-windres >/dev/null 2>&1 || MISSING+=(binutils-mingw-w64-x86-64)
command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1 || MISSING+=(gcc-mingw-w64-x86-64)
if [ ${#MISSING[@]} -gt 0 ]; then apt-get update -qq; apt-get install -y -qq "${MISSING[@]}"; fi
SOURCE_DIR="$BUILD_DIR/source"
echo '[1/5] Kaynak ve ikili dosyalar hazirlaniyor; mevcut servis calismaya devam ediyor.'
git clone --depth 1 -b main https://github.com/BigDesigner/project-gnnwatchtower.git "$SOURCE_DIR"
source "$SOURCE_DIR/scripts/ensure-go.sh" "$BUILD_DIR/toolchain"
APP_VERSION=$(tr -d '\r\n' < "$SOURCE_DIR/VERSION")
mkdir -p "$BUILD_DIR/bin"
if command -v x86_64-w64-mingw32-windres >/dev/null 2>&1; then
    (cd "$SOURCE_DIR/backend/cmd/agent" && x86_64-w64-mingw32-windres versioninfo.rc -O coff -o rsrc_windows_amd64.syso)
fi
(cd "$SOURCE_DIR/backend" && CGO_ENABLED=0 go build -ldflags="-s -w -X gnnwatchtower/internal/version.Version=$APP_VERSION" -o "$BUILD_DIR/bin/gnnwatchtower" ./cmd/server)
(cd "$SOURCE_DIR/backend" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X gnnwatchtower/internal/version.Version=$APP_VERSION -X main.Version=$APP_VERSION" -o "$BUILD_DIR/bin/gnnwatchtower-agent" ./cmd/agent)
(cd "$SOURCE_DIR/backend" && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X gnnwatchtower/internal/version.Version=$APP_VERSION -X main.Version=$APP_VERSION" -o "$BUILD_DIR/bin/gnnwatchtower-agent.exe" ./cmd/agent)
mkdir -p "$INSTALL_DIR/bin"
# Publish atomically so the old server can offer its existing signed upgrade mechanism.
for binary in gnnwatchtower-agent gnnwatchtower-agent.exe; do
    install -m 755 "$BUILD_DIR/bin/$binary" "$INSTALL_DIR/bin/$binary.new"
    mv -f "$INSTALL_DIR/bin/$binary.new" "$INSTALL_DIR/bin/$binary"
done
echo '[2/5] Mevcut ajanlarin imzali gecis guncellemesi hazirlaniyor.'
AGENT_COUNT=0
# Only an already TLS-enabled fleet can receive an automatic remote trust transition.
# Unknown legacy HTTP trust must be repaired through the authenticated installer.
if grep -Eq '^TLS_ENABLED=(true|1)$' "$INSTALL_DIR/.env"; then
    AGENT_COUNT=$("$BUILD_DIR/bin/gnnwatchtower" -prepare-agent-migration -migration-db "$INSTALL_DIR/data/gnnwatchtower.db")
else
    echo 'Eski HTTP ajanlari: sunucu guncellemesinden sonra paneldeki sertifika iceren kurulum komutunu yeniden calistirin.'
fi
if [ "$AGENT_COUNT" -gt 0 ]; then
    echo "$AGENT_COUNT aktif ajana guncelleme sunuldu; 45 saniyelik gecis araligi bekleniyor."
    sleep 45
fi
# Snapshot only after clean stop; never copy a live SQLite main file in isolation.
echo '[3/5] Servis durduruluyor ve tutarli yedek aliniyor.'
mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"
cp "$INSTALL_DIR/.env" "$BACKUP_DIR/environment"
cp "$INSTALL_DIR/gnnwatchtower" "$BACKUP_DIR/gnnwatchtower"
cp "$INSTALL_DIR/VERSION" "$BACKUP_DIR/VERSION" 2>/dev/null || true
if [ -f /etc/systemd/system/gnnwatchtower.service.d/security.conf ]; then cp /etc/systemd/system/gnnwatchtower.service.d/security.conf "$BACKUP_DIR/security.conf"; fi
bash "$SOURCE_DIR/scripts/configure-tls.sh"
systemctl stop gnnwatchtower
STOPPED=1
cp -a "$INSTALL_DIR/data" "$BACKUP_DIR/data"
echo '[4/5] Sunucu ve yetkiler guncelleniyor.'
install -m 755 "$BUILD_DIR/bin/gnnwatchtower" "$INSTALL_DIR/gnnwatchtower.new"
mv -f "$INSTALL_DIR/gnnwatchtower.new" "$INSTALL_DIR/gnnwatchtower"
SWAPPED=1
cp "$SOURCE_DIR/VERSION" "$INSTALL_DIR/VERSION"
bash "$SOURCE_DIR/scripts/harden-collector.sh"
if [ -d /opt/gnnwatchtower-agent ]; then
    systemctl stop gnnwatchtower-agent 2>/dev/null || true
    install -m 755 "$BUILD_DIR/bin/gnnwatchtower-agent" /opt/gnnwatchtower-agent/gnnwatchtower-agent
fi
systemctl daemon-reload
systemctl restart gnnwatchtower
# Check actual HTTPS readiness using this appliance's certificate; never disable verification.
echo '[5/5] HTTPS hazirligi kontrol ediliyor.'
BIND_ADDRESS=$(sed -n 's/^BIND_ADDRESS=//p' "$INSTALL_DIR/.env" | tail -1)
PORT=$(sed -n 's/^SERVER_PORT=//p' "$INSTALL_DIR/.env" | tail -1); PORT=${PORT:-3800}
CERT=$(sed -n 's/^TLS_CERT_FILE=//p' "$INSTALL_DIR/.env" | tail -1); CERT=${CERT:-$INSTALL_DIR/data/cert.pem}
HEALTH_ADDRESS=$BIND_ADDRESS
case "$HEALTH_ADDRESS" in 0.0.0.0|::) HEALTH_ADDRESS=127.0.0.1;; esac
case "$HEALTH_ADDRESS" in *:*) HEALTH_ADDRESS="[$HEALTH_ADDRESS]";; esac
READY=0
for attempt in $(seq 1 15); do
    if curl --noproxy '*' --cacert "$CERT" --connect-timeout 2 --max-time 3 -fsS "https://$HEALTH_ADDRESS:$PORT/api/health" >/dev/null 2>&1; then READY=1; break; fi
    sleep 1
done
if [ "$READY" != 1 ]; then echo '[HATA] HTTPS hazir olmadi.'; exit 1; fi
STOPPED=0
if [ -d /opt/gnnwatchtower-agent ]; then
    install -m 600 "$CERT" /opt/gnnwatchtower-agent/server-cert.pem
    systemctl start gnnwatchtower-agent 2>/dev/null || true
fi
PANEL_ADDRESS=$BIND_ADDRESS
case "$PANEL_ADDRESS" in 0.0.0.0|::) PANEL_ADDRESS=$(hostname -I | awk '{print $1}');; esac
printf '\nGuncellendi: %s\nPanel: https://%s:%s\nYedek: %s\n' "$APP_VERSION" "$PANEL_ADDRESS" "$PORT" "$BACKUP_DIR"
echo 'Ajan komutlari: Ajan Ozeti > Ajan Dagitimi. Cevrimdisi veya imza anahtari eksik eski ajanlarda paneldeki kurulum komutunu yeniden calistirin.'
if command -v openssl >/dev/null 2>&1; then openssl x509 -in "$CERT" -noout -fingerprint -sha256; fi
