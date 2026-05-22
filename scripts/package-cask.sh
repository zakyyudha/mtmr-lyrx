#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-0.1.0}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"
APP_NAME="MTMRLyrx"
APP_DIR="$DIST_DIR/${APP_NAME}.app"
BUNDLE_ID="dev.zakyyudha.mtmr-lyrx"
CLI_BIN="$DIST_DIR/mtmr-lyrx"
SWIFT_BIN="$ROOT_DIR/macos/mtmr-lyrx-menu/.build/release/${APP_NAME}"
ZIP_PATH="$DIST_DIR/${APP_NAME}-${VERSION}-macos.zip"

mkdir -p "$DIST_DIR"

GOOS=darwin GOARCH="$(go env GOARCH)" go build \
  -ldflags "-X github.com/zakyyudha/mtmr-lyrx/internal/cli.version=${VERSION}" \
  -o "$CLI_BIN" \
  "$ROOT_DIR/cmd/mtmr-lyrx"

swift build --package-path "$ROOT_DIR/macos/mtmr-lyrx-menu" -c release

rm -rf "$APP_DIR"
mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"
cp "$SWIFT_BIN" "$APP_DIR/Contents/MacOS/${APP_NAME}"
cp "$ROOT_DIR/macos/mtmr-lyrx-menu/Sources/MTMRLyrxMenu/Resources/AppIcon.icns" "$APP_DIR/Contents/Resources/AppIcon.icns"
cp "$ROOT_DIR/macos/mtmr-lyrx-menu/Sources/MTMRLyrxMenu/Resources/MenuBarIcon.png" "$APP_DIR/Contents/Resources/MenuBarIcon.png"
chmod 0755 "$APP_DIR/Contents/MacOS/${APP_NAME}"

cat > "$APP_DIR/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>${APP_NAME}</string>
  <key>CFBundleIdentifier</key>
  <string>${BUNDLE_ID}</string>
  <key>CFBundleName</key>
  <string>${APP_NAME}</string>
  <key>CFBundleDisplayName</key>
  <string>${APP_NAME}</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleIconFile</key>
  <string>AppIcon</string>
  <key>CFBundleShortVersionString</key>
  <string>${VERSION}</string>
  <key>CFBundleVersion</key>
  <string>${VERSION}</string>
  <key>LSMinimumSystemVersion</key>
  <string>13.0</string>
  <key>LSUIElement</key>
  <true/>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
PLIST

# Ad-hoc sign the unsigned test build so macOS recognizes the app bundle as signed.
# Public releases should use Developer ID signing + notarization instead.
/usr/bin/codesign --force --deep --sign - "$APP_DIR"

rm -f "$ZIP_PATH"
(
  cd "$DIST_DIR"
  /usr/bin/zip -qry "$(basename "$ZIP_PATH")" "${APP_NAME}.app" "mtmr-lyrx"
)

shasum -a 256 "$ZIP_PATH" | tee "$DIST_DIR/${APP_NAME}-${VERSION}-macos.zip.sha256"
printf '\nCreated: %s\n' "$ZIP_PATH"
