#!/usr/bin/env bash
set -euo pipefail

APP="ss14-url-patcher"
GOOS_VALUE="$(go env GOOS)"
GOARCH_VALUE="$(go env GOARCH)"
OUT_DIR="dist/${APP}-${GOOS_VALUE}-${GOARCH_VALUE}"
mkdir -p "$OUT_DIR"

case "$GOOS_VALUE" in
  windows)
    go build -trimpath -ldflags='-s -w -H windowsgui' -o "$OUT_DIR/${APP}.exe" .
    ;;
  darwin)
    ./scripts/package-macos.sh "$GOARCH_VALUE"
    ;;
  linux)
    go build -trimpath -ldflags='-s -w' -o "$OUT_DIR/${APP}" .
    chmod +x "$OUT_DIR/${APP}"
    tar -C dist -czf "dist/${APP}-linux-${GOARCH_VALUE}.tar.gz" "${APP}-linux-${GOARCH_VALUE}"
    sha256sum "dist/${APP}-linux-${GOARCH_VALUE}.tar.gz" > "dist/${APP}-linux-${GOARCH_VALUE}.tar.gz.sha256"
    ;;
  *)
    echo "Unsupported GOOS: $GOOS_VALUE" >&2
    exit 1
    ;;
esac
