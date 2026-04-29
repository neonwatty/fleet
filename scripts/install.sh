#!/bin/bash
set -euo pipefail

PREFIX="${PREFIX:-/opt/homebrew}"
BIN_DIR="${BIN_DIR:-$PREFIX/bin}"
APP_DIR="${APP_DIR:-$HOME/Applications}"
INSTALL_MENUBAR="${INSTALL_MENUBAR:-1}"
INSTALL_LOGIN_ITEM="${INSTALL_LOGIN_ITEM:-0}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [ -x "$ROOT/fleet" ]; then
  FLEET_BIN="$ROOT/fleet"
else
  FLEET_BIN="$ROOT/bin/fleet"
fi
MENUBAR_APP="$ROOT/menubar/build/Build/Products/Release/FleetMenuBar.app"

if [ ! -x "$FLEET_BIN" ] && [ -f "$ROOT/go.mod" ]; then
  echo "Building fleet CLI..."
  make -C "$ROOT" build
  FLEET_BIN="$ROOT/bin/fleet"
fi

if [ ! -x "$FLEET_BIN" ]; then
  echo "fleet binary not found. Expected $ROOT/fleet or $ROOT/bin/fleet" >&2
  exit 1
fi

mkdir -p "$BIN_DIR"
cp "$FLEET_BIN" "$BIN_DIR/fleet"
echo "Installed fleet to $BIN_DIR/fleet"

if [ "$INSTALL_MENUBAR" = "1" ]; then
  if [ -d "$ROOT/FleetMenuBar.app" ]; then
    MENUBAR_APP="$ROOT/FleetMenuBar.app"
  fi
  if [ ! -d "$MENUBAR_APP" ] && [ -f "$ROOT/menubar/project.yml" ]; then
    echo "Building FleetMenuBar.app..."
    make -C "$ROOT" menubar-build
  fi
  if [ -d "$MENUBAR_APP" ]; then
    mkdir -p "$APP_DIR"
    rm -rf "$APP_DIR/FleetMenuBar.app"
    cp -R "$MENUBAR_APP" "$APP_DIR/"
    echo "Installed FleetMenuBar.app to $APP_DIR"
  else
    echo "FleetMenuBar.app not found; skipping menu bar install."
  fi

  if [ "$INSTALL_LOGIN_ITEM" = "1" ] && [ -x "$ROOT/menubar/scripts/install-login-item.sh" ]; then
    "$ROOT/menubar/scripts/install-login-item.sh"
  fi
fi

CONFIG_CREATED=0
if [ ! -f "$HOME/.fleet/config.toml" ]; then
  "$BIN_DIR/fleet" init || true
  CONFIG_CREATED=1
fi

if [ "$CONFIG_CREATED" = "1" ]; then
  "$BIN_DIR/fleet" doctor --fix --machine local || true
else
  "$BIN_DIR/fleet" doctor || true
fi
