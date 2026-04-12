# Fleet SwiftBar plugin

Compact menu bar indicator for fleet. Shows online/total machines, live CC
count, and per-machine details in the dropdown.

## Install

1. Install [SwiftBar](https://github.com/swiftbar/SwiftBar): `brew install --cask swiftbar`.
2. Install `jq`: `brew install jq`.
3. Make sure `fleet` is on your `PATH` (e.g. `cp bin/fleet /opt/homebrew/bin/`).
4. Copy the plugin to your SwiftBar plugin directory:

   ```bash
   mkdir -p ~/Documents/SwiftBar
   cp fleet.10s.sh ~/Documents/SwiftBar/
   ```

5. Open SwiftBar and accept the default plugin folder (`~/Documents/SwiftBar`). The `fleet` plugin should appear in the menu bar within 10s.

**Note:** SwiftBar 2 defaults to `~/Documents/SwiftBar`. Older versions used `~/Library/Application Support/SwiftBar/Plugins` — that path still works if you point SwiftBar at it explicitly in the first-launch folder picker.

The `.10s.` in the filename controls the refresh cadence. Rename to
`fleet.30s.sh` for a slower refresh or `fleet.5s.sh` for a faster one.

## Customizing

Set `FLEET_BIN` if `fleet` is not on `PATH`:

```bash
export FLEET_BIN=/opt/homebrew/bin/fleet
```

## Testing

Run the fixture test from the repo root:

```bash
make test-swiftbar
```

This diffs the script's output against a committed golden file so regressions
are caught in CI without needing SwiftBar installed.
