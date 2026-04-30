#!/bin/bash
set -euo pipefail

APP_PATH="${APP_PATH:-menubar/build/Build/Products/Release/FleetMenuBar.app}"
ZIP_PATH="${ZIP_PATH:-dist/FleetMenuBar_signed.zip}"

: "${APPLE_DEVELOPER_ID:?Set APPLE_DEVELOPER_ID to your Developer ID Application identity}"
: "${APPLE_ID:?Set APPLE_ID to your Apple ID email}"
: "${APPLE_TEAM_ID:?Set APPLE_TEAM_ID to your Apple Developer Team ID}"
: "${APPLE_APP_PASSWORD:?Set APPLE_APP_PASSWORD to an app-specific password or notarytool password}"

if [ ! -d "$APP_PATH" ]; then
  echo "App not found: $APP_PATH" >&2
  exit 1
fi

if [ -n "${APPLE_CERTIFICATE_P12_BASE64:-}" ]; then
  : "${APPLE_CERTIFICATE_PASSWORD:?Set APPLE_CERTIFICATE_PASSWORD when APPLE_CERTIFICATE_P12_BASE64 is provided}"
  KEYCHAIN_PATH="$RUNNER_TEMP/fleet-signing.keychain-db"
  KEYCHAIN_PASSWORD="$(openssl rand -hex 16)"
  CERT_PATH="$RUNNER_TEMP/fleet-signing.p12"

  echo "$APPLE_CERTIFICATE_P12_BASE64" | base64 --decode > "$CERT_PATH"
  security create-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
  security set-keychain-settings -lut 21600 "$KEYCHAIN_PATH"
  security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
  security import "$CERT_PATH" -P "$APPLE_CERTIFICATE_PASSWORD" -A -t cert -f pkcs12 -k "$KEYCHAIN_PATH"
  security list-keychain -d user -s "$KEYCHAIN_PATH" login.keychain-db
  security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
fi

codesign --force --deep --options runtime --timestamp --sign "$APPLE_DEVELOPER_ID" "$APP_PATH"
codesign --verify --deep --strict --verbose=2 "$APP_PATH"

mkdir -p "$(dirname "$ZIP_PATH")"
ditto -c -k --keepParent "$APP_PATH" "$ZIP_PATH"

xcrun notarytool submit "$ZIP_PATH" \
  --apple-id "$APPLE_ID" \
  --team-id "$APPLE_TEAM_ID" \
  --password "$APPLE_APP_PASSWORD" \
  --wait

xcrun stapler staple "$APP_PATH"
ditto -c -k --keepParent "$APP_PATH" "$ZIP_PATH"
ZIP_DIR="$(dirname "$ZIP_PATH")"
ZIP_FILE="$(basename "$ZIP_PATH")"
(cd "$ZIP_DIR" && shasum -a 256 "$ZIP_FILE" > "$ZIP_FILE.sha256")
