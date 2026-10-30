#!/usr/bin/env bash
# Fresh Ubuntu/Debian installation. Existing installations use the update workflow.
set -euo pipefail
if [ "$EUID" -ne 0 ]; then echo '[HATA] sudo bash ile calistirin.'; exit 1; fi
INSTALL_DIR=/opt/gnnwatchtower
if [ -f "$INSTALL_DIR/gnnwatchtower" ]; then
    echo 'Mevcut kurulum bulundu; verileri koruyan guncelleme kullaniliyor.'
    curl --proto '=https' --proto-redir '=https' -fsSL https://raw.githubusercontent.com/BigDesigner/project-gnnwatchtower/main/update.sh | bash
    exit 0
fi
BUILD_DIR=$(mktemp -d -t watchtower-install-XXXXXX)
trap 'rm -rf "$BUILD_DIR"' EXIT
MISSING=()
command -v git >/dev/null 2>&1 || MISSING+=(git)
command -v curl >/dev/null 2>&1 || MISSING+=(curl)
command -v go >/dev/null 2>&1 || MISSING+=(golang-go)
command -v x86_64-w64-mingw32-windres >/dev/null 2>&1 || MISSING+=(binutils-mingw-w64-x86-64)
command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1 || MISSING+=(gcc-mingw-w64-x86-64)
if [ ${#MISSING[@]} -gt 0 ]; then apt-get update -qq; apt-get install -y -qq "${MISSING[@]}"; fi
SOURCE_DIR="$BUILD_DIR/source"
echo '[1/4] Kaynak ve derleyici hazirlaniyor.'
git clone --depth 1 -b main https://github.com/BigDesigner/project-gnnwatchtower.git "$SOURCE_DIR"
source "$SOURCE_DIR/scripts/ensure-go.sh" "$BUILD_DIR/toolchain"
APP_VERSION=$(tr -d '\r\n' < "$SOURCE_DIR/VERSION")
mkdir -p "$BUILD_DIR/bin"
echo '[2/4] Sunucu ve Windows/Linux ajanlari derleniyor.'
(cd "$SOURCE_DIR/backend/cmd/agent" && x86_64-w64-mingw32-windres versioninfo.rc -O coff -o rsrc_windows_amd64.syso)
(cd "$SOURCE_DIR/backend" && CGO_ENABLED=0 go build -ldflags="-s -w -X gnnwatchtower/internal/version.Version=$APP_VERSION" -o "$BUILD_DIR/bin/gnnwatchtower" ./cmd/server)
(cd "$SOURCE_DIR/backend" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X gnnwatchtower/internal/version.Version=$APP_VERSION -X main.Version=$APP_VERSION" -o "$BUILD_DIR/bin/gnnwatchtower-agent" ./cmd/agent)
(cd "$SOURCE_DIR/backend" && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X gnnwatchtower/internal/version.Version=$APP_VERSION -X main.Version=$APP_VERSION" -o "$BUILD_DIR/bin/gnnwatchtower-agent.exe" ./cmd/agent)
echo '[3/4] Dosyalar, HTTPS ve servis hesabi hazirlaniyor.'
mkdir -p "$INSTALL_DIR/bin" "$INSTALL_DIR/data" "$INSTALL_DIR/backups" "$INSTALL_DIR/logs"
install -m 755 "$BUILD_DIR/bin/gnnwatchtower" "$INSTALL_DIR/gnnwatchtower"
install -m 755 "$BUILD_DIR/bin/gnnwatchtower-agent" "$INSTALL_DIR/bin/gnnwatchtower-agent"
install -m 755 "$BUILD_DIR/bin/gnnwatchtower-agent.exe" "$INSTALL_DIR/bin/gnnwatchtower-agent.exe"
cp "$SOURCE_DIR/VERSION" "$INSTALL_DIR/VERSION"
bash "$SOURCE_DIR/scripts/configure-tls.sh"
bash "$SOURCE_DIR/scripts/harden-collector.sh"
cat > /etc/systemd/system/gnnwatchtower.service <<'EOF'
[Unit]
Description=GNNwatchtower Network Monitoring Appliance
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
User=gnnwatchtower
Group=gnnwatchtower
WorkingDirectory=/opt/gnnwatchtower
EnvironmentFile=/opt/gnnwatchtower/.env
ExecStart=/opt/gnnwatchtower/gnnwatchtower
Restart=on-failure
RestartSec=5
[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable gnnwatchtower
systemctl start gnnwatchtower
echo '[4/4] HTTPS hazirligi kontrol ediliyor.'
BIND_ADDRESS=$(sed -n 's/^BIND_ADDRESS=//p' "$INSTALL_DIR/.env" | tail -1)
PORT=$(sed -n 's/^SERVER_PORT=//p' "$INSTALL_DIR/.env" | tail -1); PORT=${PORT:-3800}
CERT=$(sed -n 's/^TLS_CERT_FILE=//p' "$INSTALL_DIR/.env" | tail -1); CERT=${CERT:-$INSTALL_DIR/data/cert.pem}
HEALTH_ADDRESS=$BIND_ADDRESS
case "$HEALTH_ADDRESS" in 0.0.0.0|::) HEALTH_ADDRESS=127.0.0.1;; esac
case "$HEALTH_ADDRESS" in *:*) HEALTH_ADDRESS="[$HEALTH_ADDRESS]";; esac
READY=0
for attempt in $(seq 1 15); do
    if curl --noproxy '*' --cacert "$CERT" --connect-timeout 2 --max-time 3 -fsS "https://$HEALTH_ADDRESS:$PORT/api/health" >/dev/null 2>&1; then READY=1;break;fi
    sleep 1
done
if [ "$READY" != 1 ]; then echo '[HATA] HTTPS hazir olmadi. journalctl -u gnnwatchtower -n 30';exit 1;fi
printf '\nKuruldu: %s\nPanel: https://%s:%s\nKullanici: admin\nIlk parola: sudo cat /opt/gnnwatchtower/data/initial_admin_password.txt\n' "$APP_VERSION" "$BIND_ADDRESS" "$PORT"
echo 'Ajan kurulumu: Ajan Ozeti > Ajan Dagitimi > Komutu Kopyala.'
if command -v openssl >/dev/null 2>&1; then openssl x509 -in "$CERT" -noout -fingerprint -sha256;fi
