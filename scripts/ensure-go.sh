#!/usr/bin/env bash
# Source this file with a task-owned build directory argument.
# Obtain the current stable compiler using official TLS metadata and its SHA-256.
set -euo pipefail
TOOLCHAIN_DIR=$1
# This helper is sourced from the freshly cloned release, even by a cached entry script.
if ! command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
    echo "Windows kaynak derleyicisi icin eksik MinGW GCC kuruluyor."
    apt-get update -qq
    apt-get install -y -qq gcc-mingw-w64-x86-64
    hash -r
    command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1 || { echo "[HATA] MinGW GCC kurulumu dogrulanamadi."; return 1; }
fi
if command -v go >/dev/null 2>&1; then
    GO_MINOR=$(go version | sed -n 's/.*go1\.\([0-9]*\).*/\1/p')
    if [ -n "$GO_MINOR" ] && [ "$GO_MINOR" -ge 22 ]; then return 0; fi
fi
command -v python3 >/dev/null 2>&1 || { apt-get update -qq; apt-get install -y -qq python3; }
command -v curl >/dev/null 2>&1 || { apt-get update -qq; apt-get install -y -qq curl; }
ARCH=$(dpkg --print-architecture)
case "$ARCH" in amd64|arm64) ;; *) echo '[HATA] Desteklenmeyen derleme mimarisi.'; return 1;; esac
mkdir -p "$TOOLCHAIN_DIR"
curl --proto '=https' --proto-redir '=https' -fsSL 'https://go.dev/dl/?mode=json' -o "$TOOLCHAIN_DIR/releases.json"
read -r GO_ARCHIVE GO_SHA < <(python3 - "$TOOLCHAIN_DIR/releases.json" "$ARCH" <<'PY'
import json, sys
for release in json.load(open(sys.argv[1])):
    if not release['stable']:
        continue
    for entry in release['files']:
        if entry['os']=='linux' and entry['arch']==sys.argv[2] and entry['kind']=='archive':
            print(entry['filename'], entry['sha256'])
            sys.exit(0)
raise SystemExit('No stable Go toolchain found')
PY
)
case "$GO_ARCHIVE" in go[0-9]*.linux-*.tar.gz) ;; *) return 1;; esac
curl --proto '=https' --proto-redir '=https' -fsSL "https://go.dev/dl/$GO_ARCHIVE" -o "$TOOLCHAIN_DIR/go.tar.gz"
printf '%s  %s\n' "$GO_SHA" "$TOOLCHAIN_DIR/go.tar.gz" | sha256sum -c -
tar -xzf "$TOOLCHAIN_DIR/go.tar.gz" -C "$TOOLCHAIN_DIR"
export PATH="$TOOLCHAIN_DIR/go/bin:$PATH"
