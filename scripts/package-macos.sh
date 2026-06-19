#!/usr/bin/env bash
set -euo pipefail

ARCH="${1:?usage: scripts/package-macos.sh <amd64|arm64>}"
APP_NAME="SS14 URL Patcher"
BIN_NAME="ss14-url-patcher"
DIST_DIR="dist"
APP_DIR="$DIST_DIR/$APP_NAME.app"
MACOS_DIR="$APP_DIR/Contents/MacOS"
RES_DIR="$APP_DIR/Contents/Resources"

rm -rf "$APP_DIR"
mkdir -p "$MACOS_DIR" "$RES_DIR"

go build -trimpath -ldflags='-s -w' -o "$MACOS_DIR/$BIN_NAME" .
chmod +x "$MACOS_DIR/$BIN_NAME"

cat > "$APP_DIR/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key>
  <string>$APP_NAME</string>
  <key>CFBundleDisplayName</key>
  <string>$APP_NAME</string>
  <key>CFBundleIdentifier</key>
  <string>club.ss220.ss14-url-patcher</string>
  <key>CFBundleExecutable</key>
  <string>$BIN_NAME</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>1.0.0</string>
  <key>CFBundleVersion</key>
  <string>1</string>
  <key>LSMinimumSystemVersion</key>
  <string>11.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
PLIST

# Ad-hoc signature. This is not a Developer ID signature and will not replace notarization.
codesign --force --deep --sign - "$APP_DIR"

ZIP="$DIST_DIR/ss14-url-patcher-darwin-$ARCH.zip"
ditto -c -k --keepParent "$APP_DIR" "$ZIP"
shasum -a 256 "$ZIP" > "$ZIP.sha256"
