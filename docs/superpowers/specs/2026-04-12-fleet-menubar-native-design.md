# Fleet Menu Bar — Native Swift App Design

**Date:** 2026-04-12
**Status:** Draft for review
**Supersedes:** `scripts/swiftbar/` (shell plugin + golden-file test)

## Goal

Replace the SwiftBar plugin with a self-contained native macOS menu bar app, mirroring the conventions used in `neonwatty/space-labeler`. The native app consumes the existing `fleet status --json` contract, removes the `swiftbar` + `jq` install dependencies, and opens the door to richer UI without changing any Go production code.

## Non-Goals (v1)

- Click-to-rename labels, click-to-kill sessions, click-to-cycle accounts. Popover is read-only.
- Menu-bar icons beyond text + emoji (matches current SwiftBar fidelity).
- Settings UI — knobs live in `UserDefaults`, set via `defaults write`.
- Auto-update, code signing, notarization, Mac App Store. App is ad-hoc signed, built from source.
- A backwards-compat path for SwiftBar users — the single known user (Jeremy) is on the hub that ships this PR, so no deprecation window.

## Architecture

Fleet gains a new top-level `menubar/` directory containing a self-contained Swift app built with XcodeGen. The `.xcodeproj` is gitignored; `project.yml` is the source of truth. The existing `fleet status --json` contract is the **only** coupling surface between the Go CLI and the Swift app.

```
fleet/
├── menubar/
│   ├── project.yml
│   ├── Sources/
│   │   ├── FleetMenuBarApp.swift
│   │   ├── AppDelegate.swift
│   │   ├── FleetClient.swift
│   │   ├── FleetModel.swift
│   │   ├── StatusItemController.swift
│   │   ├── PopoverView.swift
│   │   ├── HealthBadge.swift
│   │   └── Info.plist
│   ├── Tests/
│   │   └── FleetMenuBarTests/
│   │       ├── Fixtures/
│   │       │   └── status.json
│   │       ├── FleetClientTests.swift
│   │       ├── HealthBadgeTests.swift
│   │       └── PopoverViewTests.swift
│   └── README.md
├── scripts/swiftbar/           # DELETED
└── Makefile                    # + menubar-build / menubar-test / menubar-install / menubar-install-login / menubar-clean
```

**Xcode project settings** (from `project.yml`, mirroring space-labeler):
- macOS 13 deployment target, Swift 5.9
- `CODE_SIGN_IDENTITY: "-"` (ad-hoc), `ENABLE_HARDENED_RUNTIME: NO`, `ENABLE_APP_SANDBOX: NO`
- `LSUIElement: true` — no Dock icon
- Bundle identifier: `com.neonwatty.FleetMenuBar`

## Components

Each Swift file has one responsibility; internals can change without touching siblings.

### `FleetMenuBarApp.swift` (~10 lines)
`@main` struct. Uses `NSApplicationDelegateAdaptor(AppDelegate.self)`. Nothing else.

### `AppDelegate.swift` (~20 lines)
`applicationDidFinishLaunching` instantiates `FleetClient` (passing resolved binary path) and `StatusItemController(client:)`. Calls `client.start()` to kick off the refresh timer.

### `FleetClient.swift` (~80 lines) — Go-to-Swift boundary
```swift
@MainActor
final class FleetClient: ObservableObject {
    @Published private(set) var snapshot: FleetSnapshot?
    @Published private(set) var lastError: String?

    private let binaryPath: String
    private let refreshInterval: TimeInterval
    private var timer: Timer?
    private let env: [String: String]?

    init(binaryPath: String, refreshInterval: TimeInterval = 10, env: [String: String]? = nil)
    func start()
    func refresh()   // fire-and-forget, dispatched to background queue
    func stop()
}
```

Responsibilities:
- Resolve `fleet` binary: `UserDefaults` key `fleetBinPath` → `$FLEET_BIN` env var → `/opt/homebrew/bin/fleet` → `fleet` via `/usr/bin/env`.
- Schedule a repeating `Timer` at `refreshInterval` (default 10.0s, from `UserDefaults` key `refreshInterval`).
- On each tick: run `Process()` → capture stdout + stderr → decode JSON → assign `snapshot` on success, `lastError` on failure. Stale snapshot is **not cleared** on failure; a single transient miss flashes the error state for one tick.
- `env` parameter exists so tests can inject fixtures without a real binary.

### `FleetModel.swift` (~100 lines) — pure data, no behavior
Plain Codable structs mirroring `buildStatusJSON` in `cmd/fleet/status_json.go`. snake_case CodingKeys.

```swift
struct FleetSnapshot: Codable {
    let version: String
    let machines: [MachineStatus]
    let sessions: [SessionStatus]
    let tunnels: [TunnelStatus]
    let thresholds: Thresholds
}

struct MachineStatus: Codable {
    let name: String
    let status: String           // "online" | "offline"
    let health: String           // "free" | "ok" | "busy" | "stressed"
    let memAvailablePct: Int
    let swapGB: Double
    let ccCount: Int
    let accounts: [String]
    let labels: [LabelStatus]
}

struct LabelStatus: Codable {
    let name: String
    let sessionId: String?
    let live: Bool
}

struct Thresholds: Codable {
    let swapWarnMB: Int
    let swapHighMB: Int
}
```

`SessionStatus` and `TunnelStatus` are included as Codable structs so decode doesn't fail when the JSON contains those fields, but `PopoverView` does not render them in v1.

### `StatusItemController.swift` (~80 lines) — presentation layer
- Owns `NSStatusItem` (variable length) and `NSPopover` (transient behavior, 320×420 content size, `NSHostingController(rootView: PopoverView(client: client))`).
- Subscribes to `client.$snapshot` and `client.$lastError` via Combine.
- `render(snapshot:error:)` computes the status title:
  - `lastError != nil` → `fleet ⚠` red
  - `snapshot == nil` → `fleet …` dim gray
  - Normal → `{online}/{total} · {cc} CC`, prefixed with `⚠ ` and colored orange if any machine is `busy`, red if any is `stressed`
- `togglePopover` — on open: calls `client.refresh()` (fire-and-forget), shows popover, makes window key.

### `PopoverView.swift` (~120 lines) — SwiftUI, read-only
Top-down layout:
- Header: bold `FLEET` title, small gray `v0.1.0` to its right.
- Divider.
- `ForEach(snapshot.machines)` machine rows, each:
  - Row 1: name, account chip (`[personal-max]` dim), spacer, `HealthBadge`
  - Row 2: `{mem}% mem · {swap}GB swap · {cc} CC` — swap colored by band via `HealthBadge`
  - Indented `ForEach(machine.labels)`: `● {name}` live / `○ {name} (stale)` dim
- Divider.
- Footer: "Open full dashboard" button → launches Terminal with `fleet status`. "Quit" button → `NSApp.terminate(nil)`.

Depends only on a `FleetSnapshot` value, not on `FleetClient` (easier to test).

### `HealthBadge.swift` (~30 lines)
Pure-function style view:
```swift
struct HealthBadge: View {
    let health: String
    let swapMB: Double
    let thresholds: Thresholds
    var body: some View { ... }
}
```
Also exposes `static func swapColor(swapMB: Double, thresholds: Thresholds) -> Color` so `HealthBadgeTests` can assert band boundaries without rendering.

## Data Flow

### Routine tick (every 10s)
1. `Timer` fires on main run loop.
2. `FleetClient.refresh()` dispatches to a background `DispatchQueue`.
3. `Process` runs `{binaryPath} status --json`, capturing stdout + stderr.
4. `JSONDecoder` → `FleetSnapshot`.
5. Hop to main: `self.snapshot = decoded; self.lastError = nil`.
6. Combine fires. `StatusItemController.render(...)` updates `NSStatusItem.button.attributedTitle`. If the popover is visible, SwiftUI re-renders the bindings automatically.

### User-click refresh
1. User clicks status item → `togglePopover`.
2. If popover hidden: `client.refresh()` fires in the background (not awaited), then popover shows with current snapshot still visible.
3. When the new snapshot lands (~200ms local, ~1-2s with remote probes), `PopoverView` re-renders in place via Combine.

### First launch
`AppDelegate.applicationDidFinishLaunching` → `FleetClient.start()` schedules the timer and fires an immediate `refresh()`. Status item shows `fleet …` gray until the first snapshot resolves.

## Configuration

Two knobs read from `UserDefaults` domain `com.neonwatty.FleetMenuBar`:

| Key                | Type    | Default                      | Notes                                                          |
|--------------------|---------|------------------------------|----------------------------------------------------------------|
| `refreshInterval`  | Double  | `10.0`                       | Seconds between background polls                               |
| `fleetBinPath`     | String  | `""` (uses fallback chain)   | Override for `fleet` binary path                               |

Fallback chain for binary resolution: `fleetBinPath` default → `$FLEET_BIN` env var → `/opt/homebrew/bin/fleet` → `fleet` via `PATH`.

No settings UI. Set via `defaults write com.neonwatty.FleetMenuBar refreshInterval -int 5`. Documented in `menubar/README.md`.

## Error Handling

| Failure                              | Detection                                 | Title              | Popover behavior                                                                                   |
|--------------------------------------|-------------------------------------------|--------------------|----------------------------------------------------------------------------------------------------|
| `fleet` binary not found             | `Process.run()` throws `NSPOSIXErrorDomain` | `fleet ⚠` red      | Header shows "`fleet` binary not found — set `fleetBinPath` default or add to PATH"                |
| `fleet status --json` exits non-zero | Termination status != 0                   | `fleet ⚠` red      | Header shows captured stderr in a scrollable text block                                            |
| JSON decode fails (schema drift)     | `JSONDecoder` throws                      | `fleet ?` orange   | Header shows "Unexpected status schema — fleet CLI may be newer than menubar app. Run `make menubar-build`" |
| All machines offline                 | `snapshot.machines.allSatisfy { .status == "offline" }` | `0/N` dim gray     | Machine list still renders, all dimmed                                                             |
| First-launch loading                 | `snapshot == nil && lastError == nil`     | `fleet …` dim gray | Single "Loading…" row                                                                              |

**Key principle:** the old snapshot is not overwritten on failure. One flaky tick flashes the error state for ~10s but doesn't blank the UI.

## Testing

### Swift unit tests (`menubar/Tests/FleetMenuBarTests/`)

1. **`FleetClientTests.swift`** — pure decode tests. Loads `Fixtures/status.json` from the test bundle, asserts `FleetSnapshot` machine count, threshold values, label liveness. This is the **schema-drift canary**: if `cmd/fleet/status_json.go` changes field names without updating the Swift `CodingKeys`, this test fails loudly. No `Process`, no real fleet binary.

2. **`HealthBadgeTests.swift`** — three band boundary tests:
   - `swapMB = 512, warn = 1024, high = 4096` → `.primary`
   - `swapMB = 1024` → `.orange`
   - `swapMB = 4096` → `.red`

   Plus one test per health value: `free`/`ok`/`busy`/`stressed` → expected color. Pure function, no SwiftUI rendering.

3. **`PopoverViewTests.swift`** — renders a pure helper:
   ```swift
   func renderMachineLine(_ m: MachineStatus, thresholds: Thresholds) -> String
   ```
   Fed a fixture with one offline machine, one `free` machine, and one `stressed` machine with two labels (one live, one stale). Asserts substring contents of the output line. No `ViewInspector` dependency.

Run via `xcodebuild test -scheme FleetMenuBar -destination 'platform=macOS'`. Wrapped by `make menubar-test`.

### Go-side contract test

4. **`cmd/fleet/status_json_fixture_test.go`** (new, ~30 lines) — reads `../../menubar/Tests/FleetMenuBarTests/Fixtures/status.json` via relative path, asserts it decodes cleanly into the Go `statusDoc` type that `buildStatusJSON` emits. Fails the Go suite if the Swift fixture drifts from the Go emitter's schema. This is the only production Go change (test-only, actually).

### CI

CI already runs on macOS (needed for process-probing tests) and has Xcode available. The CI workflow adds:
- `brew install xcodegen` step
- `make menubar-test` added to the `check` make target

Expected additional CI time: 60-90s for the `xcodegen generate` + `xcodebuild test` pass.

### Out of scope

- No end-to-end UI automation that launches the .app and clicks the status item. Too flaky, low ROI.
- No test that exercises the real `fleet` binary from Swift. That's an integration concern; `FleetClientTests`' decode test is the part that catches schema drift.

## Makefile targets

```make
MENUBAR_DERIVED := menubar/build

menubar-build:
	cd menubar && xcodegen generate && xcodebuild build \
	  -project FleetMenuBar.xcodeproj -scheme FleetMenuBar \
	  -configuration Release -destination 'platform=macOS' \
	  -derivedDataPath build

menubar-test:
	cd menubar && xcodegen generate && xcodebuild test \
	  -project FleetMenuBar.xcodeproj -scheme FleetMenuBar \
	  -destination 'platform=macOS' \
	  -derivedDataPath build

menubar-install: menubar-build
	mkdir -p ~/Applications
	cp -R menubar/build/Build/Products/Release/FleetMenuBar.app ~/Applications/
	open ~/Applications/FleetMenuBar.app

menubar-install-login:
	./menubar/scripts/install-login-item.sh

menubar-clean:
	rm -rf menubar/FleetMenuBar.xcodeproj menubar/build
```

`menubar-test` joins `make check`. `menubar-build` / `menubar-install*` stay out of `check` — they touch the user's `~/Applications`.

## Login-item persistence

Ad-hoc signing silently breaks `SMAppService.mainApp.register()` (see memory `smappservice_adhoc_signing.md`). We do **not** attempt `SMAppService`.

Instead, `menubar/scripts/install-login-item.sh` writes `~/Library/LaunchAgents/com.neonwatty.FleetMenuBar.plist` pointing launchd at `~/Applications/FleetMenuBar.app/Contents/MacOS/FleetMenuBar` with `RunAtLoad: true`, `KeepAlive: false`, then runs `launchctl load`. Standard pattern for ad-hoc signed apps, no mysterious failures.

## Install flow (documented in `menubar/README.md`)

```bash
# one-time
brew install xcodegen

# build + install
cd fleet
make menubar-install

# optional: run at login
make menubar-install-login
```

After install, the user manually removes any existing `~/Documents/SwiftBar/fleet.10s.sh` since it's no longer in the repo.

## Migration & rollout

One PR deletes `scripts/swiftbar/` (plugin script, fixtures, README, golden file, `make test-swiftbar` target, and its inclusion in `make check`) and adds `menubar/` with the native app. Root `README.md` "Menu Bar (SwiftBar)" section is replaced with "Menu Bar" pointing at `menubar/README.md`.

No deprecation window. Commit message explicitly tells anyone on SwiftBar to remove `~/Documents/SwiftBar/fleet.10s.sh` after installing the native app.

## Out-of-scope follow-ups (candidates for later PRs)

- Click-to-rename labels from the popover
- Click-to-kill sessions from the popover
- Click-to-cycle per-session account
- Custom menu-bar icon (image instead of emoji)
- Preferences window (GUI over `UserDefaults`)
- Proper signed + notarized distribution

## Risks & mitigations

| Risk                                                       | Mitigation                                                                                    |
|------------------------------------------------------------|-----------------------------------------------------------------------------------------------|
| JSON schema drift between Go and Swift                     | Shared fixture + Go-side decode test (`status_json_fixture_test.go`) + Swift decode test      |
| `xcodegen` not installed in CI                             | CI step explicitly runs `brew install xcodegen`                                               |
| `xcodebuild test` slow in CI (60-90s)                      | Accepted cost; CI already runs on macOS                                                       |
| `LaunchAgent` plist path hardcoded                         | `install-login-item.sh` uses `${HOME}` and the install path, not hardcoded                    |
| User on SwiftBar gets stranded after PR merge              | Single known user (Jeremy), coordinated by hand. Commit message + README call out the cleanup |
| Ad-hoc sign + login-item footgun surprises a new contributor | Both the README and `smappservice_adhoc_signing.md` memory document the workaround            |

## Success criteria

- `make menubar-test` passes locally and in CI.
- `make menubar-install` produces a working `.app` that shows live fleet state in the menu bar after first run.
- The popover visually matches the information density of the current SwiftBar plugin (machines, health, mem%, swap with three-band coloring, CC count, live/stale labels).
- Click on status item refreshes data on open.
- Removing the `fleet` binary from PATH produces the `fleet ⚠` error state without crashing.
- `scripts/swiftbar/` is gone; root `README.md` references `menubar/README.md`.
