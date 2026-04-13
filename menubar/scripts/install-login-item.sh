#!/bin/bash
# Install FleetMenuBar as a login item via LaunchAgent.
# Ad-hoc signed apps can't use SMAppService.mainApp.register(), so we install
# a plist directly. See memory/smappservice_adhoc_signing.md for context.

set -euo pipefail

LABEL="com.neonwatty.FleetMenuBar"
APP_PATH="${HOME}/Applications/FleetMenuBar.app"
EXEC_PATH="${APP_PATH}/Contents/MacOS/FleetMenuBar"
PLIST_PATH="${HOME}/Library/LaunchAgents/${LABEL}.plist"

if [ ! -x "${EXEC_PATH}" ]; then
  echo "error: ${EXEC_PATH} not found or not executable" >&2
  echo "run 'make menubar-install' first" >&2
  exit 1
fi

mkdir -p "${HOME}/Library/LaunchAgents"

cat > "${PLIST_PATH}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${EXEC_PATH}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
EOF

# Unload if already loaded, then load fresh.
launchctl unload "${PLIST_PATH}" 2>/dev/null || true
launchctl load "${PLIST_PATH}"

echo "installed LaunchAgent: ${PLIST_PATH}"
echo "FleetMenuBar will launch at login."
echo "to uninstall: launchctl unload \"${PLIST_PATH}\" && rm \"${PLIST_PATH}\""
