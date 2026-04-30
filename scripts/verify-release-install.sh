#!/bin/bash
set -euo pipefail

VERSION="${VERSION:-latest}"
REPO="${REPO:-${GITHUB_REPOSITORY:-}}"

fail() {
  echo "release install verification failed: $*" >&2
  exit 1
}

if ! command -v gh >/dev/null 2>&1; then
  fail "gh is required"
fi

gh_release_view() {
  if [ -n "$REPO" ]; then
    gh release view --repo "$REPO" "$@"
  else
    gh release view "$@"
  fi
}

gh_release_download() {
  if [ -n "$REPO" ]; then
    gh release download --repo "$REPO" "$@"
  else
    gh release download "$@"
  fi
}

if [ "$VERSION" = "latest" ]; then
  TAG="$(gh_release_view --json tagName --jq .tagName)"
else
  TAG="$VERSION"
  if [[ "$TAG" != v* ]]; then
    TAG="v$TAG"
  fi
fi

ASSET_VERSION="${TAG#v}"
DIST_NAME="fleet_${ASSET_VERSION}_darwin_arm64"
CLI_ARCHIVE="${DIST_NAME}.tar.gz"
CLI_SUM="${CLI_ARCHIVE}.sha256"
MENUBAR_ZIP="FleetMenuBar_${ASSET_VERSION}_darwin_arm64.zip"
MENUBAR_SUM="${MENUBAR_ZIP}.sha256"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

DOWNLOAD_DIR="$TMP_DIR/download"
HOME_DIR="$TMP_DIR/home"
PREFIX_DIR="$TMP_DIR/prefix"
APP_DIR="$TMP_DIR/apps"
UNPACK_DIR="$TMP_DIR/unpack"

mkdir -p "$DOWNLOAD_DIR" "$HOME_DIR" "$PREFIX_DIR" "$APP_DIR" "$UNPACK_DIR"

echo "Downloading $TAG release assets..."
gh_release_download "$TAG" \
  --dir "$DOWNLOAD_DIR" \
  --pattern "$CLI_ARCHIVE" \
  --pattern "$CLI_SUM" \
  --pattern "$MENUBAR_ZIP" \
  --pattern "$MENUBAR_SUM"

(
  cd "$DOWNLOAD_DIR"
  shasum -c "$CLI_SUM"
  shasum -c "$MENUBAR_SUM"
)

tar -xzf "$DOWNLOAD_DIR/$CLI_ARCHIVE" -C "$UNPACK_DIR"
PACKAGE_DIR="$UNPACK_DIR/$DIST_NAME"
INSTALL_SCRIPT="$PACKAGE_DIR/scripts/install.sh"

[ -x "$PACKAGE_DIR/fleet" ] || fail "expected packaged fleet binary"
[ -x "$INSTALL_SCRIPT" ] || fail "expected executable install script"

HOME="$HOME_DIR" \
PREFIX="$PREFIX_DIR" \
APP_DIR="$APP_DIR" \
MENUBAR_ZIP="$DOWNLOAD_DIR/$MENUBAR_ZIP" \
INSTALL_MENUBAR=1 \
"$INSTALL_SCRIPT"

FLEET_BIN="$PREFIX_DIR/bin/fleet"
[ -x "$FLEET_BIN" ] || fail "expected installed fleet binary"
[ -d "$APP_DIR/FleetMenuBar.app" ] || fail "expected installed FleetMenuBar.app"

HOME="$HOME_DIR" "$FLEET_BIN" --version | grep -q "$ASSET_VERSION" || fail "fleet --version did not include $ASSET_VERSION"
HOME="$HOME_DIR" "$FLEET_BIN" doctor

echo "Release install verification passed for $TAG."
