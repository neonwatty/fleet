#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-install-smoke}"
RELEASE_TARGET="${RELEASE_TARGET:-release-cli}"
DIST_NAME="fleet_${VERSION}_darwin_arm64"
ARCHIVE="$ROOT/dist/${DIST_NAME}.tar.gz"

fail() {
  echo "install smoke failed: $*" >&2
  exit 1
}

assert_exists() {
  local path="$1"
  [ -e "$path" ] || fail "expected $path to exist"
}

assert_executable() {
  local path="$1"
  [ -x "$path" ] || fail "expected $path to be executable"
}

assert_missing() {
  local path="$1"
  [ ! -e "$path" ] || fail "expected $path to be removed"
}

echo "Building $RELEASE_TARGET artifacts..."
make -C "$ROOT" "$RELEASE_TARGET" VERSION="$VERSION"

assert_exists "$ARCHIVE"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

HOME_DIR="$TMP_DIR/home"
PREFIX_DIR="$TMP_DIR/prefix"
APP_DIR="$TMP_DIR/apps"
UNPACK_DIR="$TMP_DIR/unpack"

mkdir -p "$HOME_DIR" "$PREFIX_DIR" "$APP_DIR" "$UNPACK_DIR"
tar -xzf "$ARCHIVE" -C "$UNPACK_DIR"

PACKAGE_DIR="$UNPACK_DIR/$DIST_NAME"
INSTALL_SCRIPT="$PACKAGE_DIR/scripts/install.sh"
UNINSTALL_SCRIPT="$PACKAGE_DIR/scripts/uninstall.sh"

assert_executable "$PACKAGE_DIR/fleet"
assert_exists "$INSTALL_SCRIPT"
assert_exists "$UNINSTALL_SCRIPT"

echo "Installing from packaged archive..."
HOME="$HOME_DIR" \
PREFIX="$PREFIX_DIR" \
APP_DIR="$APP_DIR" \
INSTALL_MENUBAR=0 \
"$INSTALL_SCRIPT"

FLEET_BIN="$PREFIX_DIR/bin/fleet"
assert_executable "$FLEET_BIN"

HOME="$HOME_DIR" "$FLEET_BIN" --version | grep -q "$VERSION" || fail "fleet --version did not include $VERSION"
assert_exists "$HOME_DIR/.fleet/config.toml"
assert_exists "$HOME_DIR/fleet-work"
assert_exists "$HOME_DIR/fleet-repos"

# release-cli does not include the app bundle, but uninstall should still remove
# an app installed at the configured APP_DIR when present.
mkdir -p "$APP_DIR/FleetMenuBar.app"

echo "Uninstalling with PURGE=1..."
HOME="$HOME_DIR" \
PREFIX="$PREFIX_DIR" \
APP_DIR="$APP_DIR" \
PURGE=1 \
"$UNINSTALL_SCRIPT"

assert_missing "$FLEET_BIN"
assert_missing "$APP_DIR/FleetMenuBar.app"
assert_missing "$HOME_DIR/.fleet"

echo "Install smoke passed."
