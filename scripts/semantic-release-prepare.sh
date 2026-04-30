#!/bin/bash
set -euo pipefail

VERSION="${1:?usage: scripts/semantic-release-prepare.sh VERSION}"

make release-local VERSION="$VERSION"

if [ -z "${APPLE_DEVELOPER_ID:-}" ] || [ -z "${APPLE_ID:-}" ] || [ -z "${APPLE_TEAM_ID:-}" ] || [ -z "${APPLE_APP_PASSWORD:-}" ]; then
  echo "Apple signing secrets are not configured; keeping unsigned menu bar artifact."
  exit 0
fi

ZIP_PATH="dist/FleetMenuBar_${VERSION}_darwin_arm64_signed.zip" scripts/sign-notarize-menubar.sh
