#!/bin/bash
# <bitbar.title>Fleet</bitbar.title>
# <bitbar.version>v1.0</bitbar.version>
# <bitbar.author>neonwatty</bitbar.author>
# <bitbar.desc>Fleet status in the macOS menu bar (reads fleet status --json).</bitbar.desc>
# <bitbar.dependencies>jq, fleet</bitbar.dependencies>
#
# SwiftBar plugin for fleet. Install: copy this file to your SwiftBar plugin
# directory and enable it. Requires `fleet` on $PATH and `jq` installed.
#
# The script accepts FLEET_BIN and FLEET_STATUS_FIXTURE env vars for testing.

set -eu

FLEET_BIN="${FLEET_BIN:-fleet}"

if ! command -v jq >/dev/null 2>&1; then
  echo "fleet: install jq"
  echo "---"
  echo "brew install jq | bash=/opt/homebrew/bin/brew param1=install param2=jq terminal=true"
  exit 0
fi

if [ -n "${FLEET_STATUS_FIXTURE:-}" ]; then
  JSON=$(cat "$FLEET_STATUS_FIXTURE")
elif ! JSON=$("$FLEET_BIN" status --json 2>/dev/null); then
  echo "fleet ⚠ | color=red"
  echo "---"
  echo "fleet status --json failed"
  echo "Check fleet binary on PATH | bash=which param1=fleet terminal=true"
  exit 0
fi

total=$(echo "$JSON" | jq '.machines | length')
online=$(echo "$JSON" | jq '[.machines[] | select(.status == "online")] | length')
cc=$(echo "$JSON" | jq '[.machines[].cc_count] | add // 0')
stressed=$(echo "$JSON" | jq '[.machines[] | select(.health == "stressed")] | length')
busy=$(echo "$JSON" | jq '[.machines[] | select(.health == "busy")] | length')

prefix=""
color=""
if [ "$stressed" -gt 0 ]; then
  prefix="⚠ "
  color=" | color=red"
elif [ "$busy" -gt 0 ]; then
  prefix="⚠ "
  color=" | color=orange"
fi

echo "${prefix}${online}/${total} · ${cc} CC${color}"
echo "---"
echo "FLEET | size=11 color=gray"
echo "---"

echo "$JSON" | jq -r '
  .machines[] |
  (if .status == "offline" then
    "\(.name) — offline | color=gray"
  else
    "\(.name) \(if (.accounts | length) > 0 then "[" + (.accounts | join(",")) + "]" else "" end)  \(.health) · \(.mem_available_pct)% mem · \(.swap_gb)GB swap · \(.cc_count) CC"
  end),
  (.labels[] |
    (if .live then "  ● " + .name + " | color=white"
    else "  ○ " + .name + "(stale) | color=gray"
    end))
'

echo "---"
echo "Open full dashboard | bash=$FLEET_BIN param1=status terminal=true"
echo "Refresh | refresh=true"
