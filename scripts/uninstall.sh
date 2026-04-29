#!/bin/bash
set -euo pipefail

PREFIX="${PREFIX:-/opt/homebrew}"
BIN_DIR="${BIN_DIR:-$PREFIX/bin}"
APP_DIR="${APP_DIR:-$HOME/Applications}"
PURGE="${PURGE:-0}"
PLIST="$HOME/Library/LaunchAgents/com.neonwatty.FleetMenuBar.plist"

if [ -f "$PLIST" ]; then
  launchctl unload "$PLIST" 2>/dev/null || true
  rm -f "$PLIST"
  echo "Removed FleetMenuBar LaunchAgent"
fi

pkill -x FleetMenuBar 2>/dev/null || true

rm -rf "$APP_DIR/FleetMenuBar.app"
echo "Removed $APP_DIR/FleetMenuBar.app"

rm -f "$BIN_DIR/fleet"
echo "Removed $BIN_DIR/fleet"

if [ "$PURGE" = "1" ]; then
  rm -rf "$HOME/.fleet"
  echo "Removed $HOME/.fleet"
else
  echo "Left $HOME/.fleet intact. Re-run with PURGE=1 to remove config and state."
fi
