#!/usr/bin/env bash
set -euo pipefail
INSTALL_DIR=/opt/gnnwatchtower
mkdir -p "$INSTALL_DIR/data"
touch "$INSTALL_DIR/.env"
chmod 600 "$INSTALL_DIR/.env"
if ! grep -q '^BIND_ADDRESS=.' "$INSTALL_DIR/.env"; then
    # Preserve an existing listener's interface scope; otherwise choose the host's first IPv4 address.
    LISTEN_ADDRESS=$(ss -H -ltn 'sport = :3800' 2>/dev/null | awk 'NR==1 {print $4}' || true)
    case "$LISTEN_ADDRESS" in
        \[*\]:*) BIND_ADDRESS=${LISTEN_ADDRESS%]:*}; BIND_ADDRESS=${BIND_ADDRESS#\[} ;;
        *:*) BIND_ADDRESS=${LISTEN_ADDRESS%:*} ;;
        *) BIND_ADDRESS=$(hostname -I | awk '{for(i=1;i<=NF;i++) if($i ~ /^[0-9]+\./) {print $i;exit}}') ;;
    esac
    if [ -z "$BIND_ADDRESS" ] || [ "$BIND_ADDRESS" = '*' ]; then BIND_ADDRESS=127.0.0.1; fi
    printf '\nBIND_ADDRESS=%s\n' "$BIND_ADDRESS" >> "$INSTALL_DIR/.env"
fi
# The server creates/preserves its own certificate and signing key. No firewall or OS trust changes.
sed -i '/^TLS_ENABLED=/d' "$INSTALL_DIR/.env"
printf '\nTLS_ENABLED=true\n' >> "$INSTALL_DIR/.env"
