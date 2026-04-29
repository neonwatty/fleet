# Fleet Menu Bar

Native macOS menu bar app for fleet. Shows `{online}/{total} · {cc} CC` in the menu bar and opens a popover with per-machine health, swap, and session labels.

## Requirements

- macOS 13 (Ventura) or later
- Xcode 15 or later
- [XcodeGen](https://github.com/yonaskolb/XcodeGen) — `brew install xcodegen`
- `fleet` binary on `PATH` (see the repo root `README.md` for fleet install)

## Install

```sh
cd fleet
make menubar-install
```

What that runs:
1. `xcodegen generate` — regenerates `FleetMenuBar.xcodeproj` from `project.yml`
2. `xcodebuild build -configuration Release` — builds the .app bundle
3. Copies the built app to `~/Applications/FleetMenuBar.app`
4. `open`s the app so the menu bar item appears immediately

## Run at login

The app is ad-hoc signed, so `SMAppService.mainApp.register()` doesn't persist across reboots. Use a LaunchAgent plist instead:

```sh
make menubar-install-login
```

This writes `~/Library/LaunchAgents/com.neonwatty.FleetMenuBar.plist` and `launchctl load`s it. After reboot, the menu bar item comes back automatically. To uninstall:

```sh
launchctl unload ~/Library/LaunchAgents/com.neonwatty.FleetMenuBar.plist
rm ~/Library/LaunchAgents/com.neonwatty.FleetMenuBar.plist
```

## Configuration

Two knobs, both in `UserDefaults` domain `com.neonwatty.FleetMenuBar`:

| Key                | Type   | Default  | Notes                                            |
|--------------------|--------|----------|--------------------------------------------------|
| `refreshInterval`  | Double | `10.0`   | Seconds between background polls                 |
| `fleetBinPath`     | String | (empty)  | Override for `fleet` binary path                  |

Set these in the native Preferences window from the popover, or with
`defaults write`:

```sh
defaults write com.neonwatty.FleetMenuBar refreshInterval -int 5
defaults write com.neonwatty.FleetMenuBar fleetBinPath -string /opt/homebrew/bin/fleet
```

Binary resolution fallback chain: `fleetBinPath` default → `$FLEET_BIN` env var → `/opt/homebrew/bin/fleet`.

## Click behavior

- **Click menu bar item** — toggles the popover. On open, the app refreshes fleet state immediately (not just on the next 10s tick).
- **"Open full dashboard"** in the popover footer — launches `fleet status` (the TUI) in a new Terminal.app window.
- **"Preferences"** in the popover footer — opens settings for the fleet binary path and refresh interval.
- **"Quit"** — `NSApp.terminate(nil)`.

## Development

```sh
# Regenerate the Xcode project after editing project.yml
cd menubar && xcodegen generate

# Build only
make menubar-build

# Run the test suite (no fleet binary required — uses fixture)
make menubar-test

# Clean build artifacts + generated project
make menubar-clean
```

The `.xcodeproj` is gitignored. `project.yml` is the source of truth — always run `xcodegen generate` after cloning or editing it.

## Architecture

```
FleetMenuBarApp → AppDelegate → FleetClient (spawns `fleet status --json`)
                             → StatusItemController → NSStatusItem
                                                   → NSPopover → PopoverView
```

- `FleetModel.swift` — `Codable` mirror of `buildStatusJSON` in `cmd/fleet/status_json.go`. Schema-drift protection comes from `FleetModelTests` (Swift side) + `status_json_fixture_test.go` (Go side) sharing the same `status.json` fixture.
- `FleetClient.swift` — `Process` launch, `JSONDecoder`, Combine `@Published` snapshot. `FleetClientTests` cover the pure helpers (`resolveBinaryPath`, `decode`).
- `HealthBadge.swift` — pure color-band helpers shared between the status bar title and popover body. `HealthBadgeTests` cover band boundaries.
- `StatusItemController.swift` — owns `NSStatusItem`, subscribes to the client, toggles the popover, refreshes on click.
- `PopoverView.swift` — SwiftUI layout. Pure helper `renderMachineLine(_:thresholds:)` is tested directly without SwiftUI rendering.
