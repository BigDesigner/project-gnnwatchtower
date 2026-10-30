#!/usr/bin/env bash
set -euo pipefail
INSTALL_DIR=/opt/gnnwatchtower
id -u gnnwatchtower >/dev/null 2>&1 || useradd --system --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin gnnwatchtower
chown root:gnnwatchtower "$INSTALL_DIR"
chmod 750 "$INSTALL_DIR"
for path in data backups logs; do
    mkdir -p "$INSTALL_DIR/$path"
    chown -R gnnwatchtower:gnnwatchtower "$INSTALL_DIR/$path"
    chmod 700 "$INSTALL_DIR/$path"
done
mkdir -p /etc/systemd/system/gnnwatchtower.service.d
cat > /etc/systemd/system/gnnwatchtower.service.d/security.conf <<'EOF'
[Service]
User=gnnwatchtower
Group=gnnwatchtower
NoNewPrivileges=true
CapabilityBoundingSet=
AmbientCapabilities=
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=
ReadWritePaths=/opt/gnnwatchtower/data /opt/gnnwatchtower/backups /opt/gnnwatchtower/logs
EOF
# Existing network policy remains operator-managed; do not add firewall rules/capabilities.
