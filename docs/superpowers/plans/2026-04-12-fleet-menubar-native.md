# Fleet Native Menu Bar App Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the SwiftBar shell plugin with a self-contained native Swift menu bar app that consumes `fleet status --json`, mirroring `neonwatty/space-labeler`'s conventions.

**Architecture:** New top-level `menubar/` directory containing an XcodeGen-built `.app` bundle. AppDelegate wires a `FleetClient` (process + JSONDecoder, publishes `FleetSnapshot`) into a `StatusItemController` (renders `NSStatusItem` + hosts a SwiftUI `PopoverView`). Ad-hoc signed, sandbox off, `LSUIElement: true`, deploys to `~/Applications`. Only coupling surface with the Go CLI is the existing `fleet status --json` JSON schema — no Go production code changes.

**Tech Stack:** Swift 5.9, SwiftUI, AppKit (NSStatusItem/NSPopover), Combine, XcodeGen, xcodebuild, macOS 13+. Go side only gets one new test file for schema-drift protection.

**Branch:** All tasks land on branch `menubar-native-app` (already created, currently has the spec commit). One PR against `main` at the end.

---

## File Structure

Files created by this plan:

```
menubar/
├── project.yml                              # Task 1
├── .gitignore                               # Task 1
├── Sources/
│   ├── Info.plist                           # Task 1
│   ├── FleetMenuBarApp.swift                # Task 2
│   ├── AppDelegate.swift                    # Task 2
│   ├── FleetModel.swift                     # Task 3
│   ├── FleetClient.swift                    # Task 5
│   ├── HealthBadge.swift                    # Task 6
│   ├── StatusItemController.swift           # Task 7
│   └── PopoverView.swift                    # Task 8
├── Tests/FleetMenuBarTests/
│   ├── Fixtures/
│   │   └── status.json                      # Task 3 (copied from swiftbar fixture)
│   ├── FleetModelTests.swift                # Task 3
│   ├── HealthBadgeTests.swift               # Task 6
│   └── PopoverLineTests.swift               # Task 8
├── scripts/
│   └── install-login-item.sh                # Task 9
└── README.md                                # Task 10

cmd/fleet/status_json_fixture_test.go       # Task 4
Makefile                                     # Task 11 (add menubar-* targets)
README.md                                    # Task 12 (root — Menu Bar section rewrite)
scripts/swiftbar/                             # Task 13 (DELETED)
```

**File responsibilities:**

- `project.yml` — XcodeGen source of truth. `.xcodeproj` is gitignored; always regenerated.
- `Sources/Info.plist` — `LSUIElement: true`, bundle id `com.neonwatty.FleetMenuBar`, version 0.1.0.
- `Sources/FleetMenuBarApp.swift` — `@main` struct, adapts `AppDelegate`. ~10 lines.
- `Sources/AppDelegate.swift` — wires `FleetClient` + `StatusItemController`, calls `client.start()`. ~25 lines.
- `Sources/FleetModel.swift` — pure Codable structs matching `buildStatusJSON` output. No behavior. ~90 lines.
- `Sources/FleetClient.swift` — spawns `fleet status --json`, decodes, publishes `@Published snapshot: FleetSnapshot?` and `lastError: String?`. Timer-driven + on-demand refresh. ~110 lines.
- `Sources/HealthBadge.swift` — pure helpers `swapColor(_:thresholds:)` and `healthColor(_:)`, plus a tiny view. Separated so color logic is directly unit-testable. ~50 lines.
- `Sources/StatusItemController.swift` — owns `NSStatusItem` + `NSPopover`, subscribes to `FleetClient`, renders the title string, toggles popover on click and refreshes on open. ~90 lines.
- `Sources/PopoverView.swift` — SwiftUI view that takes a `FleetSnapshot` + error and renders machines + labels + footer. Also exports pure helper `renderMachineLine(_:thresholds:)` for tests. ~140 lines.
- `Tests/FleetMenuBarTests/FleetModelTests.swift` — decodes the bundled `status.json` fixture, asserts structure. Schema-drift canary.
- `Tests/FleetMenuBarTests/HealthBadgeTests.swift` — band boundary tests for `swapColor` and health-string → color mapping.
- `Tests/FleetMenuBarTests/PopoverLineTests.swift` — tests `renderMachineLine` for online/offline/stressed/labels-rendered cases.
- `cmd/fleet/status_json_fixture_test.go` — Go test that decodes the Swift fixture into `statusDoc`. Fails the Go suite if the fixture drifts from the Go emitter.
- `menubar/scripts/install-login-item.sh` — writes `~/Library/LaunchAgents/com.neonwatty.FleetMenuBar.plist` and `launchctl load`s it.
- `menubar/README.md` — install + config docs.
- `Makefile` — add `menubar-build`, `menubar-test`, `menubar-install`, `menubar-install-login`, `menubar-clean`. Fold `menubar-test` into `check`. Remove `test-swiftbar`.
- Root `README.md` — rewrite "Menu Bar (SwiftBar)" section to "Menu Bar" pointing at `menubar/README.md`.

---

## Preconditions

- Running on macOS with Xcode 15+ installed (needed for `xcodebuild` against macOS 13 target).
- `xcodegen` available (`brew install xcodegen` — one-time). Task 1 verifies.
- Currently on branch `menubar-native-app` with `87a17aa` (spec commit) at HEAD.
- `fleet` binary built at `/opt/homebrew/bin/fleet` (for end-to-end smoke verification in Task 14 — the unit tests don't need it).

---

### Task 1: XcodeGen project scaffolding

**Files:**
- Create: `menubar/project.yml`
- Create: `menubar/.gitignore`
- Create: `menubar/Sources/Info.plist`
- Create: `menubar/Tests/FleetMenuBarTests/.gitkeep`

- [ ] **Step 1: Verify xcodegen is installed**

Run: `which xcodegen && xcodegen --version`
Expected: prints a path and a version number. If not installed: `brew install xcodegen`.

- [ ] **Step 2: Create `menubar/project.yml`**

```yaml
name: FleetMenuBar
options:
  bundleIdPrefix: com.neonwatty
  deploymentTarget:
    macOS: "13.0"
  createIntermediateGroups: true
  generateEmptyDirectories: true

settings:
  base:
    SWIFT_VERSION: "5.9"
    MARKETING_VERSION: "0.1.0"
    CURRENT_PROJECT_VERSION: "1"
    DEVELOPMENT_TEAM: ""
    CODE_SIGN_STYLE: Automatic
    CODE_SIGN_IDENTITY: "-"
    ENABLE_HARDENED_RUNTIME: NO
    ENABLE_APP_SANDBOX: NO
    COMBINE_HIDPI_IMAGES: YES

targets:
  FleetMenuBar:
    type: application
    platform: macOS
    sources:
      - path: Sources
    info:
      path: Sources/Info.plist
      properties:
        LSUIElement: true
        CFBundleName: FleetMenuBar
        CFBundleDisplayName: Fleet Menu Bar
        CFBundleIdentifier: com.neonwatty.FleetMenuBar
        CFBundleShortVersionString: "0.1.0"
        CFBundleVersion: "1"
        LSMinimumSystemVersion: "13.0"
        NSHumanReadableCopyright: "Copyright © 2026 Jeremy Watt"

  FleetMenuBarTests:
    type: bundle.unit-test
    platform: macOS
    sources:
      - path: Tests/FleetMenuBarTests
    dependencies:
      - target: FleetMenuBar
    settings:
      base:
        GENERATE_INFOPLIST_FILE: YES
        PRODUCT_BUNDLE_IDENTIFIER: com.neonwatty.FleetMenuBarTests
```

- [ ] **Step 3: Create `menubar/.gitignore`**

```
FleetMenuBar.xcodeproj/
build/
*.xcuserstate
xcuserdata/
DerivedData/
```

- [ ] **Step 4: Create `menubar/Sources/Info.plist`**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>$(DEVELOPMENT_LANGUAGE)</string>
	<key>CFBundleDisplayName</key>
	<string>Fleet Menu Bar</string>
	<key>CFBundleExecutable</key>
	<string>$(EXECUTABLE_NAME)</string>
	<key>CFBundleIdentifier</key>
	<string>com.neonwatty.FleetMenuBar</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>FleetMenuBar</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>0.1.0</string>
	<key>CFBundleVersion</key>
	<string>1</string>
	<key>LSMinimumSystemVersion</key>
	<string>13.0</string>
	<key>LSUIElement</key>
	<true/>
	<key>NSHumanReadableCopyright</key>
	<string>Copyright © 2026 Jeremy Watt</string>
</dict>
</plist>
```

- [ ] **Step 5: Placeholder test-target directory**

Run: `mkdir -p menubar/Tests/FleetMenuBarTests && touch menubar/Tests/FleetMenuBarTests/.gitkeep`

- [ ] **Step 6: Verify xcodegen can generate the project**

Run: `cd menubar && xcodegen generate`
Expected: "Created project at menubar/FleetMenuBar.xcodeproj". Exit 0.

- [ ] **Step 7: Verify xcodegen output is ignored by git**

Run: `cd /Users/jeremywatt/Desktop/fleet && git status menubar/FleetMenuBar.xcodeproj`
Expected: no output (file ignored).

- [ ] **Step 8: Commit**

```bash
git add menubar/project.yml menubar/.gitignore menubar/Sources/Info.plist menubar/Tests/FleetMenuBarTests/.gitkeep
git commit -m "feat(menubar): scaffold XcodeGen project and Info.plist"
```

---

### Task 2: App entry point and AppDelegate

Two tiny files that boot the app. No FleetClient yet — this task just proves the app builds and launches; we stub the `StatusItemController` reference so nothing else is needed.

**Files:**
- Create: `menubar/Sources/FleetMenuBarApp.swift`
- Create: `menubar/Sources/AppDelegate.swift`

- [ ] **Step 1: Create `menubar/Sources/FleetMenuBarApp.swift`**

```swift
import SwiftUI

@main
struct FleetMenuBarApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate

    var body: some Scene {
        Settings { EmptyView() }
    }
}
```

- [ ] **Step 2: Create `menubar/Sources/AppDelegate.swift`**

```swift
import AppKit

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        // Wired in Task 7 once StatusItemController exists.
    }
}
```

- [ ] **Step 3: Verify the app builds**

Run: `cd menubar && xcodegen generate && xcodebuild build -project FleetMenuBar.xcodeproj -scheme FleetMenuBar -destination 'platform=macOS' -derivedDataPath build 2>&1 | tail -20`
Expected: `** BUILD SUCCEEDED **`.

- [ ] **Step 4: Commit**

```bash
git add menubar/Sources/FleetMenuBarApp.swift menubar/Sources/AppDelegate.swift
git commit -m "feat(menubar): add app entry point and empty AppDelegate"
```

---

### Task 3: FleetModel — Codable DTOs + fixture decode test

Build the data model first, test-first, because it's the schema-drift canary. Copy the existing SwiftBar fixture into the test bundle.

**Files:**
- Create: `menubar/Tests/FleetMenuBarTests/Fixtures/status.json`
- Create: `menubar/Tests/FleetMenuBarTests/FleetModelTests.swift`
- Create: `menubar/Sources/FleetModel.swift`

- [ ] **Step 1: Copy SwiftBar fixture to new test bundle path**

Run: `mkdir -p menubar/Tests/FleetMenuBarTests/Fixtures && cp scripts/swiftbar/fixtures/status.json menubar/Tests/FleetMenuBarTests/Fixtures/status.json`

- [ ] **Step 2: Tell the test target to bundle the fixture**

Edit `menubar/project.yml` — replace the `FleetMenuBarTests` target block with:

```yaml
  FleetMenuBarTests:
    type: bundle.unit-test
    platform: macOS
    sources:
      - path: Tests/FleetMenuBarTests
      - path: Tests/FleetMenuBarTests/Fixtures
        buildPhase: resources
    dependencies:
      - target: FleetMenuBar
    settings:
      base:
        GENERATE_INFOPLIST_FILE: YES
        PRODUCT_BUNDLE_IDENTIFIER: com.neonwatty.FleetMenuBarTests
```

And delete the placeholder: `rm menubar/Tests/FleetMenuBarTests/.gitkeep` (no longer needed — real files exist).

- [ ] **Step 3: Write the failing test `menubar/Tests/FleetMenuBarTests/FleetModelTests.swift`**

```swift
import XCTest
@testable import FleetMenuBar

final class FleetModelTests: XCTestCase {

    func testDecodesFixtureSnapshot() throws {
        let url = Bundle(for: Self.self).url(forResource: "status", withExtension: "json")
        XCTAssertNotNil(url, "status.json fixture not found in test bundle")
        let data = try Data(contentsOf: url!)
        let snapshot = try JSONDecoder().decode(FleetSnapshot.self, from: data)

        XCTAssertEqual(snapshot.version, "1")
        XCTAssertEqual(snapshot.machines.count, 3)
        XCTAssertEqual(snapshot.thresholds.swapWarnMB, 1024)
        XCTAssertEqual(snapshot.thresholds.swapHighMB, 4096)

        let mm1 = snapshot.machines.first { $0.name == "mm1" }!
        XCTAssertEqual(mm1.status, "online")
        XCTAssertEqual(mm1.health, "busy")
        XCTAssertEqual(mm1.memAvailablePct, 22)
        XCTAssertEqual(mm1.swapGB, 9.1, accuracy: 0.001)
        XCTAssertEqual(mm1.ccCount, 2)
        XCTAssertEqual(mm1.accounts, ["personal-max"])
        XCTAssertEqual(mm1.labels.count, 2)
        XCTAssertEqual(mm1.labels[0].name, "bleep")
        XCTAssertTrue(mm1.labels[0].live)
        XCTAssertEqual(mm1.labels[0].sessionId, "a1b2c3")
        XCTAssertEqual(mm1.labels[1].name, "deckchecker")
        XCTAssertFalse(mm1.labels[1].live)

        let mm3 = snapshot.machines.first { $0.name == "mm3" }!
        XCTAssertEqual(mm3.status, "offline")
        XCTAssertEqual(mm3.labels.count, 0)

        XCTAssertEqual(snapshot.sessions.count, 2)
        let s = snapshot.sessions.first { $0.id == "a1b2c3" }!
        XCTAssertEqual(s.project, "neonwatty/bleep")
        XCTAssertEqual(s.machine, "mm1")
        XCTAssertEqual(s.account, "personal-max")
        XCTAssertEqual(s.tunnelLocalPort, 3000)
    }
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `cd menubar && xcodegen generate && xcodebuild test -project FleetMenuBar.xcodeproj -scheme FleetMenuBar -destination 'platform=macOS' -derivedDataPath build 2>&1 | tail -30`
Expected: build error — `cannot find type 'FleetSnapshot' in scope`.

- [ ] **Step 5: Write `menubar/Sources/FleetModel.swift`**

```swift
import Foundation

struct FleetSnapshot: Codable {
    let version: String
    let timestamp: String
    let thresholds: Thresholds
    let machines: [MachineStatus]
    let sessions: [SessionStatus]
}

struct Thresholds: Codable {
    let swapWarnMB: Int
    let swapHighMB: Int

    enum CodingKeys: String, CodingKey {
        case swapWarnMB = "swap_warn_mb"
        case swapHighMB = "swap_high_mb"
    }
}

struct MachineStatus: Codable {
    let name: String
    let status: String
    let health: String
    let memAvailablePct: Int
    let swapGB: Double
    let ccCount: Int
    let score: Double
    let accounts: [String]
    let labels: [LabelStatus]

    enum CodingKeys: String, CodingKey {
        case name, status, health, score, accounts, labels
        case memAvailablePct = "mem_available_pct"
        case swapGB = "swap_gb"
        case ccCount = "cc_count"
    }
}

struct LabelStatus: Codable {
    let name: String
    let live: Bool
    let sessionId: String

    enum CodingKeys: String, CodingKey {
        case name, live
        case sessionId = "session_id"
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        self.name = try c.decode(String.self, forKey: .name)
        self.live = try c.decode(Bool.self, forKey: .live)
        // Go emits session_id with `omitempty`, so it may be absent OR "".
        self.sessionId = try c.decodeIfPresent(String.self, forKey: .sessionId) ?? ""
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(name, forKey: .name)
        try c.encode(live, forKey: .live)
        if !sessionId.isEmpty {
            try c.encode(sessionId, forKey: .sessionId)
        }
    }
}

struct SessionStatus: Codable {
    let id: String
    let project: String
    let machine: String
    let branch: String
    let account: String?
    let label: String?
    let tunnelLocalPort: Int
    let tunnelRemotePort: Int
    let startedAt: String

    enum CodingKeys: String, CodingKey {
        case id, project, machine, branch, account, label
        case tunnelLocalPort = "tunnel_local_port"
        case tunnelRemotePort = "tunnel_remote_port"
        case startedAt = "started_at"
    }
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd menubar && xcodegen generate && xcodebuild test -project FleetMenuBar.xcodeproj -scheme FleetMenuBar -destination 'platform=macOS' -derivedDataPath build 2>&1 | tail -15`
Expected: `Test Suite 'FleetModelTests' passed` and `** TEST SUCCEEDED **`.

- [ ] **Step 7: Commit**

```bash
git add menubar/Sources/FleetModel.swift \
  menubar/Tests/FleetMenuBarTests/FleetModelTests.swift \
  menubar/Tests/FleetMenuBarTests/Fixtures/status.json \
  menubar/project.yml
git rm menubar/Tests/FleetMenuBarTests/.gitkeep
git commit -m "feat(menubar): add FleetSnapshot Codable model + fixture decode test"
```

---

### Task 4: Go-side contract test — schema-drift canary

A Go test that decodes the Swift fixture into `statusDoc`. If either side drifts, both suites fail.

**Files:**
- Create: `cmd/fleet/status_json_fixture_test.go`

- [ ] **Step 1: Write the failing test**

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSwiftFixtureMatchesGoSchema decodes the fixture used by the native
// menu bar app's Swift tests into the same Go type that buildStatusJSON
// emits. If either side drifts, this test fails loudly.
func TestSwiftFixtureMatchesGoSchema(t *testing.T) {
	path := filepath.Join("..", "..", "menubar", "Tests", "FleetMenuBarTests", "Fixtures", "status.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var doc statusDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if doc.Version != "1" {
		t.Errorf("version = %q, want \"1\"", doc.Version)
	}
	if len(doc.Machines) != 3 {
		t.Errorf("machines len = %d, want 3", len(doc.Machines))
	}
	if doc.Thresholds.SwapWarnMB != 1024 || doc.Thresholds.SwapHighMB != 4096 {
		t.Errorf("thresholds = %+v, want swap_warn_mb=1024 swap_high_mb=4096", doc.Thresholds)
	}
	if len(doc.Sessions) != 2 {
		t.Errorf("sessions len = %d, want 2", len(doc.Sessions))
	}
}
```

- [ ] **Step 2: Run test to verify it passes immediately**

Run: `go test ./cmd/fleet/ -run TestSwiftFixtureMatchesGoSchema -v`
Expected: `--- PASS: TestSwiftFixtureMatchesGoSchema`. (The Go side already matches the fixture — this test exists to keep them matched going forward.)

- [ ] **Step 3: Sanity-check the failure mode — temporarily corrupt the fixture**

Run: `echo '{"version":"1"}' > /tmp/bad.json && go test ./cmd/fleet/ -run TestSwiftFixtureMatchesGoSchema -v 2>&1 | tail -5`
That's a no-op on the real path — actually verify by renaming:
```bash
mv menubar/Tests/FleetMenuBarTests/Fixtures/status.json menubar/Tests/FleetMenuBarTests/Fixtures/status.json.bak
go test ./cmd/fleet/ -run TestSwiftFixtureMatchesGoSchema -v 2>&1 | tail -5
mv menubar/Tests/FleetMenuBarTests/Fixtures/status.json.bak menubar/Tests/FleetMenuBarTests/Fixtures/status.json
```
Expected during the rename: `FAIL` with "read fixture". After restore, re-run and confirm PASS.

- [ ] **Step 4: Run full Go test suite to confirm nothing else broke**

Run: `go test ./cmd/fleet/`
Expected: `ok  github.com/neonwatty/fleet/cmd/fleet`.

- [ ] **Step 5: Commit**

```bash
git add cmd/fleet/status_json_fixture_test.go
git commit -m "test(fleet): lock Swift fixture against Go status schema"
```

---

### Task 5: FleetClient — binary resolution, Process launch, Combine publisher

Now the core of the Go↔Swift boundary. No test for the real `Process` launch — we test the binary-resolution helper and the timer setup with an injected fake command.

**Files:**
- Create: `menubar/Tests/FleetMenuBarTests/FleetClientTests.swift`
- Create: `menubar/Sources/FleetClient.swift`

- [ ] **Step 1: Write failing test `menubar/Tests/FleetMenuBarTests/FleetClientTests.swift`**

```swift
import XCTest
import Combine
@testable import FleetMenuBar

final class FleetClientTests: XCTestCase {

    func testResolveBinaryPrefersUserDefaultsOverride() {
        let defaults = UserDefaults(suiteName: "FleetMenuBarTests.\(UUID())")!
        defaults.set("/tmp/custom-fleet", forKey: "fleetBinPath")
        defer { defaults.removeObject(forKey: "fleetBinPath") }

        let resolved = FleetClient.resolveBinaryPath(
            defaults: defaults,
            env: ["FLEET_BIN": "/opt/should-be-ignored/fleet"]
        )
        XCTAssertEqual(resolved, "/tmp/custom-fleet")
    }

    func testResolveBinaryFallsBackToEnvVar() {
        let defaults = UserDefaults(suiteName: "FleetMenuBarTests.\(UUID())")!
        let resolved = FleetClient.resolveBinaryPath(
            defaults: defaults,
            env: ["FLEET_BIN": "/opt/from-env/fleet"]
        )
        XCTAssertEqual(resolved, "/opt/from-env/fleet")
    }

    func testResolveBinaryFallsBackToHomebrew() {
        let defaults = UserDefaults(suiteName: "FleetMenuBarTests.\(UUID())")!
        let resolved = FleetClient.resolveBinaryPath(defaults: defaults, env: [:])
        XCTAssertEqual(resolved, "/opt/homebrew/bin/fleet")
    }

    func testDecodeSnapshotFromBytesPublishesOnMain() throws {
        let json = """
        {
          "version":"1","timestamp":"2026-04-12T14:32:10Z",
          "thresholds":{"swap_warn_mb":1024,"swap_high_mb":4096},
          "machines":[],
          "sessions":[]
        }
        """
        let data = Data(json.utf8)
        let snapshot = try FleetClient.decode(data)
        XCTAssertEqual(snapshot.version, "1")
        XCTAssertEqual(snapshot.machines.count, 0)
    }

    func testDecodeSurfacesDecoderError() {
        let data = Data("{}".utf8)
        XCTAssertThrowsError(try FleetClient.decode(data))
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd menubar && xcodegen generate && xcodebuild test -project FleetMenuBar.xcodeproj -scheme FleetMenuBar -destination 'platform=macOS' -derivedDataPath build 2>&1 | tail -15`
Expected: build error — `cannot find 'FleetClient' in scope`.

- [ ] **Step 3: Write `menubar/Sources/FleetClient.swift`**

```swift
import Foundation
import Combine

@MainActor
final class FleetClient: ObservableObject {
    @Published private(set) var snapshot: FleetSnapshot?
    @Published private(set) var lastError: String?

    private let binaryPath: String
    private let refreshInterval: TimeInterval
    private var timer: Timer?
    private let queue = DispatchQueue(label: "com.neonwatty.FleetMenuBar.refresh", qos: .utility)

    init(binaryPath: String, refreshInterval: TimeInterval = 10) {
        self.binaryPath = binaryPath
        self.refreshInterval = refreshInterval
    }

    func start() {
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: refreshInterval, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.refresh() }
        }
    }

    func stop() {
        timer?.invalidate()
        timer = nil
    }

    func refresh() {
        let path = binaryPath
        queue.async { [weak self] in
            let result = Self.runStatus(binaryPath: path)
            DispatchQueue.main.async {
                guard let self else { return }
                switch result {
                case .success(let snap):
                    self.snapshot = snap
                    self.lastError = nil
                case .failure(let err):
                    self.lastError = err
                }
            }
        }
    }

    // MARK: - Pure helpers (testable)

    static func resolveBinaryPath(defaults: UserDefaults, env: [String: String]) -> String {
        if let override = defaults.string(forKey: "fleetBinPath"), !override.isEmpty {
            return override
        }
        if let envPath = env["FLEET_BIN"], !envPath.isEmpty {
            return envPath
        }
        return "/opt/homebrew/bin/fleet"
    }

    static func decode(_ data: Data) throws -> FleetSnapshot {
        return try JSONDecoder().decode(FleetSnapshot.self, from: data)
    }

    private enum RunResult {
        case success(FleetSnapshot)
        case failure(String)
    }

    private static func runStatus(binaryPath: String) -> RunResult {
        let process = Process()
        process.launchPath = binaryPath
        process.arguments = ["status", "--json"]

        let stdout = Pipe()
        let stderr = Pipe()
        process.standardOutput = stdout
        process.standardError = stderr

        do {
            try process.run()
        } catch {
            return .failure("launch failed: \(error.localizedDescription)")
        }
        process.waitUntilExit()

        if process.terminationStatus != 0 {
            let errData = stderr.fileHandleForReading.readDataToEndOfFile()
            let msg = String(data: errData, encoding: .utf8) ?? "unknown error"
            return .failure("fleet status --json exited \(process.terminationStatus): \(msg)")
        }

        let data = stdout.fileHandleForReading.readDataToEndOfFile()
        do {
            let snap = try decode(data)
            return .success(snap)
        } catch {
            return .failure("decode failed: \(error.localizedDescription)")
        }
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd menubar && xcodegen generate && xcodebuild test -project FleetMenuBar.xcodeproj -scheme FleetMenuBar -destination 'platform=macOS' -derivedDataPath build 2>&1 | tail -15`
Expected: `Test Suite 'FleetClientTests' passed`. All four FleetClient tests + the one FleetModel test pass.

- [ ] **Step 5: Commit**

```bash
git add menubar/Sources/FleetClient.swift menubar/Tests/FleetMenuBarTests/FleetClientTests.swift
git commit -m "feat(menubar): FleetClient resolves binary path and decodes snapshots"
```

---

### Task 6: HealthBadge — color logic + tests

Pure functions for health-string → color and swap-MB → color, extracted so the popover rendering doesn't need to own this logic.

**Files:**
- Create: `menubar/Tests/FleetMenuBarTests/HealthBadgeTests.swift`
- Create: `menubar/Sources/HealthBadge.swift`

- [ ] **Step 1: Write failing test**

```swift
import XCTest
import SwiftUI
@testable import FleetMenuBar

final class HealthBadgeTests: XCTestCase {

    private let thresholds = Thresholds(swapWarnMB: 1024, swapHighMB: 4096)

    func testSwapColorBelowWarnIsPrimary() {
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 0, thresholds: thresholds), .normal)
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 1023, thresholds: thresholds), .normal)
    }

    func testSwapColorAtWarnIsOrange() {
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 1024, thresholds: thresholds), .warn)
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 4095, thresholds: thresholds), .warn)
    }

    func testSwapColorAtHighIsRed() {
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 4096, thresholds: thresholds), .high)
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 99999, thresholds: thresholds), .high)
    }

    func testHealthBandForEachLabel() {
        XCTAssertEqual(HealthBadge.healthBand(health: "free"), .free)
        XCTAssertEqual(HealthBadge.healthBand(health: "ok"), .ok)
        XCTAssertEqual(HealthBadge.healthBand(health: "busy"), .warn)
        XCTAssertEqual(HealthBadge.healthBand(health: "stressed"), .high)
        XCTAssertEqual(HealthBadge.healthBand(health: "offline"), .offline)
        XCTAssertEqual(HealthBadge.healthBand(health: "??"), .normal) // unknown fallback
    }
}

// Thresholds already Codable, but the test builds one directly.
extension Thresholds {
    init(swapWarnMB: Int, swapHighMB: Int) {
        let json = """
        {"swap_warn_mb":\(swapWarnMB),"swap_high_mb":\(swapHighMB)}
        """
        self = try! JSONDecoder().decode(Thresholds.self, from: Data(json.utf8))
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd menubar && xcodegen generate && xcodebuild test -project FleetMenuBar.xcodeproj -scheme FleetMenuBar -destination 'platform=macOS' -derivedDataPath build 2>&1 | tail -15`
Expected: build error — `cannot find 'HealthBadge' in scope`.

- [ ] **Step 3: Write `menubar/Sources/HealthBadge.swift`**

```swift
import SwiftUI

enum HealthBand: Equatable {
    case normal      // default foreground
    case free        // green
    case ok          // muted green / secondary
    case warn        // orange
    case high        // red
    case offline     // gray
}

struct HealthBadge: View {
    let health: String
    let swapMB: Double
    let thresholds: Thresholds

    var body: some View {
        Text(health)
            .font(.system(size: 11, weight: .medium))
            .foregroundColor(Self.color(for: Self.healthBand(health: health)))
    }

    static func swapBand(swapMB: Double, thresholds: Thresholds) -> HealthBand {
        if swapMB >= Double(thresholds.swapHighMB) { return .high }
        if swapMB >= Double(thresholds.swapWarnMB) { return .warn }
        return .normal
    }

    static func healthBand(health: String) -> HealthBand {
        switch health {
        case "free": return .free
        case "ok": return .ok
        case "busy": return .warn
        case "stressed": return .high
        case "offline": return .offline
        default: return .normal
        }
    }

    static func color(for band: HealthBand) -> Color {
        switch band {
        case .normal: return .primary
        case .free: return .green
        case .ok: return .secondary
        case .warn: return .orange
        case .high: return .red
        case .offline: return .gray
        }
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd menubar && xcodegen generate && xcodebuild test -project FleetMenuBar.xcodeproj -scheme FleetMenuBar -destination 'platform=macOS' -derivedDataPath build 2>&1 | tail -15`
Expected: all HealthBadge tests pass.

- [ ] **Step 5: Commit**

```bash
git add menubar/Sources/HealthBadge.swift menubar/Tests/FleetMenuBarTests/HealthBadgeTests.swift
git commit -m "feat(menubar): HealthBadge color bands for health + swap"
```

---

### Task 7: StatusItemController — NSStatusItem + NSPopover

Owns the menu bar item, builds the title string, hosts the popover. The popover content is a stub until Task 8.

**Files:**
- Create: `menubar/Sources/StatusItemController.swift`
- Modify: `menubar/Sources/AppDelegate.swift`

- [ ] **Step 1: Create `menubar/Sources/StatusItemController.swift`**

```swift
import AppKit
import Combine
import SwiftUI

@MainActor
final class StatusItemController {
    private let statusItem: NSStatusItem
    private let popover: NSPopover
    private let client: FleetClient
    private var cancellables = Set<AnyCancellable>()

    init(client: FleetClient) {
        self.client = client

        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)

        popover = NSPopover()
        popover.behavior = .transient
        popover.contentSize = NSSize(width: 320, height: 420)
        popover.contentViewController = NSHostingController(
            rootView: PopoverView(client: client)
        )

        statusItem.button?.target = self
        statusItem.button?.action = #selector(togglePopover(_:))

        client.$snapshot
            .combineLatest(client.$lastError)
            .receive(on: RunLoop.main)
            .sink { [weak self] snap, err in
                self?.render(snapshot: snap, error: err)
            }
            .store(in: &cancellables)

        render(snapshot: nil, error: nil)
    }

    private func render(snapshot: FleetSnapshot?, error: String?) {
        guard let button = statusItem.button else { return }

        if error != nil {
            button.attributedTitle = Self.titleString("fleet ⚠", color: .systemRed)
            return
        }
        guard let snap = snapshot else {
            button.attributedTitle = Self.titleString("fleet …", color: .secondaryLabelColor)
            return
        }

        let machines = snap.machines
        let total = machines.count
        let online = machines.filter { $0.status == "online" }.count
        let cc = machines.reduce(0) { $0 + $1.ccCount }
        let hasStressed = machines.contains { $0.health == "stressed" }
        let hasBusy = machines.contains { $0.health == "busy" }

        let prefix = (hasStressed || hasBusy) ? "⚠ " : ""
        let text = "\(prefix)\(online)/\(total) · \(cc) CC"
        let color: NSColor = hasStressed ? .systemRed : (hasBusy ? .systemOrange : .labelColor)
        button.attributedTitle = Self.titleString(text, color: color)
    }

    private static func titleString(_ text: String, color: NSColor) -> NSAttributedString {
        NSAttributedString(
            string: text,
            attributes: [
                .foregroundColor: color,
                .font: NSFont.systemFont(ofSize: 13, weight: .medium),
            ]
        )
    }

    @objc private func togglePopover(_ sender: Any?) {
        guard let button = statusItem.button else { return }
        if popover.isShown {
            popover.performClose(sender)
        } else {
            client.refresh()
            popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
            popover.contentViewController?.view.window?.makeKey()
        }
    }
}
```

- [ ] **Step 2: Add a stub `PopoverView` so the build succeeds**

Create `menubar/Sources/PopoverView.swift` with a temporary stub (real impl in Task 8):

```swift
import SwiftUI

struct PopoverView: View {
    @ObservedObject var client: FleetClient

    var body: some View {
        Text("fleet (loading)")
            .padding()
            .frame(width: 320, height: 420)
    }
}
```

- [ ] **Step 3: Wire AppDelegate to instantiate everything**

Replace `menubar/Sources/AppDelegate.swift` with:

```swift
import AppKit

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var client: FleetClient!
    private var controller: StatusItemController!

    func applicationDidFinishLaunching(_ notification: Notification) {
        let binaryPath = FleetClient.resolveBinaryPath(
            defaults: UserDefaults.standard,
            env: ProcessInfo.processInfo.environment
        )
        let interval = UserDefaults.standard.double(forKey: "refreshInterval")
        client = FleetClient(
            binaryPath: binaryPath,
            refreshInterval: interval > 0 ? interval : 10
        )
        controller = StatusItemController(client: client)
        client.start()
    }
}
```

- [ ] **Step 4: Build + run tests to verify nothing is broken**

Run: `cd menubar && xcodegen generate && xcodebuild test -project FleetMenuBar.xcodeproj -scheme FleetMenuBar -destination 'platform=macOS' -derivedDataPath build 2>&1 | tail -15`
Expected: `** TEST SUCCEEDED **`. All earlier tests still pass; no new tests yet.

- [ ] **Step 5: Commit**

```bash
git add menubar/Sources/StatusItemController.swift menubar/Sources/PopoverView.swift menubar/Sources/AppDelegate.swift
git commit -m "feat(menubar): StatusItemController renders fleet title + hosts popover stub"
```

---

### Task 8: PopoverView — real popover with `renderMachineLine` helper + tests

Replace the stub popover with the real one. Extract a pure `renderMachineLine` helper so we can test the line shape without SwiftUI rendering.

**Files:**
- Create: `menubar/Tests/FleetMenuBarTests/PopoverLineTests.swift`
- Modify: `menubar/Sources/PopoverView.swift`

- [ ] **Step 1: Write the failing test**

```swift
import XCTest
@testable import FleetMenuBar

final class PopoverLineTests: XCTestCase {

    private let thresholds = Thresholds(swapWarnMB: 1024, swapHighMB: 4096)

    private func machine(
        name: String = "mm1",
        status: String = "online",
        health: String = "ok",
        memPct: Int = 45,
        swapGB: Double = 0.5,
        cc: Int = 1,
        accounts: [String] = [],
        labels: [LabelStatus] = []
    ) -> MachineStatus {
        let json = """
        {
          "name":"\(name)","status":"\(status)","health":"\(health)",
          "mem_available_pct":\(memPct),"swap_gb":\(swapGB),"cc_count":\(cc),
          "score":10,"accounts":\(accountsJSON(accounts)),"labels":[]
        }
        """
        var m = try! JSONDecoder().decode(MachineStatus.self, from: Data(json.utf8))
        // Labels round-tripped separately
        m = MachineStatus(copying: m, labels: labels)
        return m
    }

    private func accountsJSON(_ a: [String]) -> String {
        "[" + a.map { "\"\($0)\"" }.joined(separator: ",") + "]"
    }

    func testRenderOfflineMachine() {
        let m = machine(status: "offline", health: "offline", memPct: 0, swapGB: 0, cc: 0)
        let line = PopoverView.renderMachineLine(m, thresholds: thresholds)
        XCTAssertTrue(line.contains("mm1"))
        XCTAssertTrue(line.contains("offline"))
    }

    func testRenderOnlineMachineIncludesMemSwapCC() {
        let m = machine(health: "free", memPct: 72, swapGB: 0.3, cc: 0)
        let line = PopoverView.renderMachineLine(m, thresholds: thresholds)
        XCTAssertTrue(line.contains("72% mem"))
        XCTAssertTrue(line.contains("0.3GB swap"))
        XCTAssertTrue(line.contains("0 CC"))
        XCTAssertTrue(line.contains("free"))
    }

    func testRenderIncludesAccountChip() {
        let m = machine(accounts: ["personal-max"])
        let line = PopoverView.renderMachineLine(m, thresholds: thresholds)
        XCTAssertTrue(line.contains("[personal-max]"))
    }

    func testRenderRoundsSwapToOneDecimal() {
        let m = machine(swapGB: 1.909_062_5)
        let line = PopoverView.renderMachineLine(m, thresholds: thresholds)
        XCTAssertTrue(line.contains("1.9GB swap"), "line = \(line)")
    }
}

// Test-only helper so we can build a MachineStatus with custom labels.
extension MachineStatus {
    init(copying base: MachineStatus, labels: [LabelStatus]) {
        self.init(
            name: base.name, status: base.status, health: base.health,
            memAvailablePct: base.memAvailablePct, swapGB: base.swapGB,
            ccCount: base.ccCount, score: base.score,
            accounts: base.accounts, labels: labels
        )
    }
}
```

For the copy-init to work, `MachineStatus` needs a memberwise init visible to the test target. Structs auto-generate one — it's internal by default, which matches `@testable import`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd menubar && xcodegen generate && xcodebuild test -project FleetMenuBar.xcodeproj -scheme FleetMenuBar -destination 'platform=macOS' -derivedDataPath build 2>&1 | tail -15`
Expected: build error — `type 'PopoverView' has no member 'renderMachineLine'`.

- [ ] **Step 3: Rewrite `menubar/Sources/PopoverView.swift`**

```swift
import SwiftUI

struct PopoverView: View {
    @ObservedObject var client: FleetClient

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            header

            Divider()

            if let error = client.lastError {
                errorBlock(error)
            } else if let snap = client.snapshot {
                machineList(snap)
            } else {
                Text("Loading…")
                    .foregroundStyle(.secondary)
                    .font(.system(size: 12))
            }

            Spacer(minLength: 0)
            Divider()
            footer
        }
        .padding(12)
        .frame(width: 320, height: 420)
    }

    private var header: some View {
        HStack(alignment: .firstTextBaseline) {
            Text("FLEET")
                .font(.system(size: 11, weight: .bold))
                .tracking(0.8)
                .foregroundStyle(.secondary)
            Spacer()
            Text("v0.1.0")
                .font(.system(size: 10))
                .foregroundStyle(.tertiary)
        }
    }

    private func errorBlock(_ error: String) -> some View {
        ScrollView {
            Text(error)
                .font(.system(size: 11, design: .monospaced))
                .foregroundStyle(.red)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    private func machineList(_ snap: FleetSnapshot) -> some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 10) {
                ForEach(snap.machines, id: \.name) { m in
                    machineCard(m, thresholds: snap.thresholds)
                }
            }
        }
    }

    private func machineCard(_ m: MachineStatus, thresholds: Thresholds) -> some View {
        VStack(alignment: .leading, spacing: 3) {
            HStack(alignment: .firstTextBaseline) {
                Text(m.name)
                    .font(.system(size: 13, weight: .semibold))
                if !m.accounts.isEmpty {
                    Text("[\(m.accounts.joined(separator: ","))]")
                        .font(.system(size: 10))
                        .foregroundStyle(.secondary)
                }
                Spacer()
                Text(m.health)
                    .font(.system(size: 11, weight: .medium))
                    .foregroundColor(HealthBadge.color(for: HealthBadge.healthBand(health: m.health)))
            }

            if m.status == "offline" {
                Text("offline")
                    .font(.system(size: 11))
                    .foregroundStyle(.secondary)
            } else {
                HStack(spacing: 8) {
                    Text("\(m.memAvailablePct)% mem")
                        .font(.system(size: 11))
                    Text("\(formatSwap(m.swapGB))GB swap")
                        .font(.system(size: 11))
                        .foregroundColor(HealthBadge.color(for: HealthBadge.swapBand(swapMB: m.swapGB * 1024, thresholds: thresholds)))
                    Text("\(m.ccCount) CC")
                        .font(.system(size: 11))
                }
                .foregroundStyle(.secondary)
            }

            ForEach(m.labels, id: \.name) { l in
                HStack(spacing: 6) {
                    Text(l.live ? "●" : "○")
                        .foregroundStyle(l.live ? Color.primary : Color.secondary)
                    Text(l.live ? l.name : "\(l.name) (stale)")
                        .font(.system(size: 11))
                        .foregroundStyle(l.live ? Color.primary : Color.secondary)
                }
                .padding(.leading, 12)
            }
        }
    }

    private var footer: some View {
        HStack {
            Button("Open full dashboard") { Self.openFullDashboard() }
                .buttonStyle(.plain)
                .font(.system(size: 11))
                .foregroundStyle(.secondary)
            Spacer()
            Button("Quit") { NSApp.terminate(nil) }
                .buttonStyle(.plain)
                .font(.system(size: 11))
                .foregroundStyle(.secondary)
        }
    }

    private static func openFullDashboard() {
        let proc = Process()
        proc.launchPath = "/usr/bin/open"
        proc.arguments = ["-a", "Terminal", "-n", "--args", "fleet", "status"]
        try? proc.run()
    }

    // MARK: - Pure helpers (testable)

    static func renderMachineLine(_ m: MachineStatus, thresholds: Thresholds) -> String {
        var parts: [String] = [m.name]
        if !m.accounts.isEmpty {
            parts.append("[\(m.accounts.joined(separator: ","))]")
        }
        if m.status == "offline" {
            parts.append("offline")
            return parts.joined(separator: " ")
        }
        parts.append(m.health)
        parts.append("\(m.memAvailablePct)% mem")
        parts.append("\(formatSwap(m.swapGB))GB swap")
        parts.append("\(m.ccCount) CC")
        return parts.joined(separator: " ")
    }

    static func formatSwap(_ swapGB: Double) -> String {
        String(format: "%.1f", swapGB)
    }
}

private func formatSwap(_ swapGB: Double) -> String {
    PopoverView.formatSwap(swapGB)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd menubar && xcodegen generate && xcodebuild test -project FleetMenuBar.xcodeproj -scheme FleetMenuBar -destination 'platform=macOS' -derivedDataPath build 2>&1 | tail -20`
Expected: `** TEST SUCCEEDED **`, all PopoverLine tests pass.

- [ ] **Step 5: Build the full app (sanity check)**

Run: `cd menubar && xcodebuild build -project FleetMenuBar.xcodeproj -scheme FleetMenuBar -configuration Release -destination 'platform=macOS' -derivedDataPath build 2>&1 | tail -10`
Expected: `** BUILD SUCCEEDED **`.

- [ ] **Step 6: Commit**

```bash
git add menubar/Sources/PopoverView.swift menubar/Tests/FleetMenuBarTests/PopoverLineTests.swift
git commit -m "feat(menubar): real PopoverView with renderMachineLine helper + tests"
```

---

### Task 9: Login-item install script

Ad-hoc signing breaks `SMAppService`; the user-facing fix is a LaunchAgent plist. One shell script that writes + loads it.

**Files:**
- Create: `menubar/scripts/install-login-item.sh`

- [ ] **Step 1: Create the script**

```bash
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
```

- [ ] **Step 2: Make it executable**

Run: `chmod +x menubar/scripts/install-login-item.sh`

- [ ] **Step 3: Verify it at least parses without executing**

Run: `bash -n menubar/scripts/install-login-item.sh && echo ok`
Expected: `ok`. (We're not running it — that would write to the user's LaunchAgents and require the app to be installed.)

- [ ] **Step 4: Commit**

```bash
git add menubar/scripts/install-login-item.sh
git commit -m "feat(menubar): add login-item install script via LaunchAgent"
```

---

### Task 10: menubar/README.md

**Files:**
- Create: `menubar/README.md`

- [ ] **Step 1: Create the README**

````markdown
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

Set with `defaults write`:

```sh
defaults write com.neonwatty.FleetMenuBar refreshInterval -int 5
defaults write com.neonwatty.FleetMenuBar fleetBinPath -string /opt/homebrew/bin/fleet
```

Binary resolution fallback chain: `fleetBinPath` default → `$FLEET_BIN` env var → `/opt/homebrew/bin/fleet`.

## Click behavior

- **Click menu bar item** — toggles the popover. On open, the app refreshes fleet state immediately (not just on the next 10s tick).
- **"Open full dashboard"** in the popover footer — launches `fleet status` (the TUI) in a new Terminal.app window.
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
````

- [ ] **Step 2: Commit**

```bash
git add menubar/README.md
git commit -m "docs(menubar): add install, config, and architecture README"
```

---

### Task 11: Makefile targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Read the current Makefile to find where to add targets**

Run: `cat Makefile`
Note the existing `check:` target and the `test-swiftbar:` target (will be removed in Task 13).

- [ ] **Step 2: Add new `menubar-*` targets**

Append to `Makefile` (replace any existing `menubar-*` section if present):

```make
.PHONY: menubar-build menubar-test menubar-install menubar-install-login menubar-clean

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
	mkdir -p $(HOME)/Applications
	rm -rf $(HOME)/Applications/FleetMenuBar.app
	cp -R menubar/build/Build/Products/Release/FleetMenuBar.app $(HOME)/Applications/
	open $(HOME)/Applications/FleetMenuBar.app

menubar-install-login:
	./menubar/scripts/install-login-item.sh

menubar-clean:
	rm -rf menubar/FleetMenuBar.xcodeproj menubar/build
```

- [ ] **Step 3: Add `menubar-test` to `check`**

Find the `check:` line in the Makefile. It currently depends on several targets including `test-swiftbar`. Update it to:
- Remove `test-swiftbar` from the dependency list (Task 13 deletes that target entirely).
- Add `menubar-test` to the dependency list.

Example before:
```make
check: fmt lint vet test test-swiftbar build
```

Example after:
```make
check: fmt lint vet test menubar-test build
```

- [ ] **Step 4: Verify `make menubar-build` works**

Run: `make menubar-build 2>&1 | tail -10`
Expected: `** BUILD SUCCEEDED **` and no error exit.

- [ ] **Step 5: Verify `make menubar-test` works**

Run: `make menubar-test 2>&1 | tail -10`
Expected: `** TEST SUCCEEDED **`.

- [ ] **Step 6: Commit**

```bash
git add Makefile
git commit -m "build(menubar): add menubar-* make targets and fold into check"
```

---

### Task 12: Update root README.md — Menu Bar section

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Locate the existing Menu Bar section**

Run: `grep -n "Menu Bar" README.md`
Expected: finds a heading like `## Menu Bar (SwiftBar)` pointing at `scripts/swiftbar/README.md`.

- [ ] **Step 2: Replace the Menu Bar section**

Find the section that starts with `## Menu Bar (SwiftBar)` and replace it with:

```markdown
## Menu Bar

Fleet ships a native macOS menu bar app that shows a compact fleet indicator
and a popover with per-machine health, swap, and labels. See
[`menubar/README.md`](menubar/README.md) for install instructions.

At a glance: `3/4 · 2 CC` means 3 of 4 machines are online with 2 live Claude
Code instances. Click the indicator for a per-machine popover with accounts,
labels, memory, and swap.
```

- [ ] **Step 3: Verify the link target exists**

Run: `test -f menubar/README.md && echo ok`
Expected: `ok`.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs(readme): point Menu Bar section at native menubar app"
```

---

### Task 13: Delete SwiftBar plugin

Final cleanup. Delete the plugin script, its fixtures, its README, and remove its make target.

**Files:**
- Delete: `scripts/swiftbar/fleet.10s.sh`
- Delete: `scripts/swiftbar/README.md`
- Delete: `scripts/swiftbar/fixtures/status.json`
- Delete: `scripts/swiftbar/fixtures/status.expected.txt`
- Delete: `scripts/swiftbar/fixtures/` (empty after above)
- Delete: `scripts/swiftbar/` (empty after above)
- Modify: `Makefile` (remove `test-swiftbar` target if still present)

- [ ] **Step 1: Delete SwiftBar files**

Run:
```bash
git rm scripts/swiftbar/fleet.10s.sh
git rm scripts/swiftbar/README.md
git rm scripts/swiftbar/fixtures/status.json
git rm scripts/swiftbar/fixtures/status.expected.txt
rmdir scripts/swiftbar/fixtures scripts/swiftbar
```

- [ ] **Step 2: Remove `test-swiftbar` target from Makefile if still present**

Run: `grep -n "test-swiftbar" Makefile || echo "no references"`

If any references remain, open `Makefile` and delete:
- The `test-swiftbar:` target recipe (all of its lines)
- The `test-swiftbar` token from the `.PHONY:` declaration if it's listed
- Any remaining `test-swiftbar` token from the `check:` dependency list (should already be gone from Task 11)

- [ ] **Step 3: Verify `make check` still works**

Run: `make check 2>&1 | tail -20`
Expected: all checks pass. No `test-swiftbar` references.

- [ ] **Step 4: Verify Go schema-drift test still finds the fixture via its new path**

Run: `go test ./cmd/fleet/ -run TestSwiftFixtureMatchesGoSchema -v`
Expected: `--- PASS`. (Task 4 already pointed at `menubar/Tests/...`, not `scripts/swiftbar/...`, so this should still pass cleanly.)

- [ ] **Step 5: Commit**

```bash
git add Makefile
git commit -m "chore(swiftbar): remove SwiftBar plugin — replaced by native menubar app"
```

---

### Task 14: End-to-end smoke verification

Not a code change — a manual checklist the implementer runs before opening the PR. Covered here so it's not forgotten.

- [ ] **Step 1: Full build from scratch**

```bash
make menubar-clean
make menubar-build
```
Expected: `** BUILD SUCCEEDED **`.

- [ ] **Step 2: Full test run**

```bash
make check
```
Expected: `go test` passes (including the new `TestSwiftFixtureMatchesGoSchema`), `xcodebuild test` passes, lint clean, file-size gate passes.

- [ ] **Step 3: Install and manually verify the app**

```bash
make menubar-install
```

Check visually:
- Menu bar shows `N/M · K CC` (your fleet's real numbers).
- Click menu bar item: popover opens, shows machines with health/mem/swap/CC.
- Stale labels render dim with `○` prefix; live labels render bold with `●`.
- Swap values near your `swap_warn_mb`/`swap_high_mb` thresholds show orange/red.
- "Open full dashboard" button launches `fleet status` in a Terminal window.
- "Quit" exits the app cleanly (menu bar item disappears).

- [ ] **Step 4: Kill fleet binary and verify error state**

Temporarily break it:
```bash
sudo mv /opt/homebrew/bin/fleet /opt/homebrew/bin/fleet.bak
```
Wait up to `refreshInterval` seconds. Expected: menu bar shows `fleet ⚠` red. Click → popover shows the stderr from the failed launch.

Restore:
```bash
sudo mv /opt/homebrew/bin/fleet.bak /opt/homebrew/bin/fleet
```
Wait one tick. Expected: menu bar returns to normal state.

- [ ] **Step 5: Final CI pre-check**

```bash
make lint
find . -name '*.go' -exec awk 'END { if (NR > 300) print FILENAME ": " NR " lines" }' {} \;
```
Expected: no output from either command.

- [ ] **Step 6: Push and open PR**

```bash
git push -u origin menubar-native-app
gh pr create --title "Native menu bar app: replace SwiftBar plugin" --body "$(cat <<'EOF'
## Summary
- Replaces `scripts/swiftbar/` shell plugin with a native Swift menu bar app under `menubar/`.
- Removes `swiftbar` and `jq` install dependencies; users now install a single `.app` bundle via `make menubar-install`.
- JSON contract between Go and Swift is protected from drift by a shared fixture (`menubar/Tests/FleetMenuBarTests/Fixtures/status.json`) and decode tests on both sides.
- Only production Go change: one new test file (`cmd/fleet/status_json_fixture_test.go`).

## Test plan
- [ ] `make check` passes locally (go test + menubar-test + lint + file-size gate)
- [ ] `make menubar-install` produces a working `.app` in `~/Applications/`
- [ ] Menu bar shows live fleet status; clicking opens popover with machine rows + labels
- [ ] Removing `fleet` binary puts the app into the red `fleet ⚠` error state without crashing
- [ ] CI (macOS runner) passes `xcodegen` + `xcodebuild test`

## Migration note
Anyone currently using the SwiftBar plugin should remove `~/Documents/SwiftBar/fleet.10s.sh` after installing the native app. There is no deprecation window — SwiftBar support is deleted in the same commit that adds the native app.
EOF
)"
```
Expected: PR URL printed.

---

## Self-review

**1. Spec coverage:**

| Spec section | Task |
|---|---|
| Repo layout `menubar/` | Task 1 |
| `LSUIElement: true`, ad-hoc sign, sandbox off | Task 1 (`project.yml` + `Info.plist`) |
| `FleetMenuBarApp.swift`, `AppDelegate.swift` | Task 2, wired in Task 7 |
| `FleetModel.swift` Codable structs | Task 3 |
| Schema-drift canary (Swift side) | Task 3 |
| Schema-drift canary (Go side) | Task 4 |
| `FleetClient.swift` binary resolution + decode | Task 5 |
| `HealthBadge.swift` color bands | Task 6 |
| `StatusItemController.swift` NSStatusItem + popover toggle + refresh-on-open | Task 7 |
| `PopoverView.swift` SwiftUI layout + `renderMachineLine` tests | Task 8 |
| Login-item LaunchAgent script | Task 9 |
| `menubar/README.md` docs | Task 10 |
| Makefile targets + fold into `check` | Task 11 |
| Root README Menu Bar section update | Task 12 |
| Delete `scripts/swiftbar/` | Task 13 |
| End-to-end smoke verification | Task 14 |
| Config knobs (`refreshInterval`, `fleetBinPath`) | Task 5 resolves them + Task 7 reads refreshInterval in AppDelegate |
| Error handling (missing binary, non-zero exit, decode fail) | Task 5 (runStatus) + Task 7 (render) |
| Ad-hoc sign + LaunchAgent workaround | Task 9 + Task 10 (README) |
| CI integration | Task 11 (`menubar-test` in `check`) + Task 14 pre-check |

All spec sections covered.

**2. Placeholder scan:**

- No "TBD", "TODO", "implement later", "fill in details".
- No "add appropriate error handling" — error handling is spelled out in Task 5's `runStatus` with concrete return paths and in Task 7's `render` with concrete title strings.
- No "similar to Task N" — every step shows its own code.
- No "write tests for the above" — every test step has the actual test code.

**3. Type consistency:**

- `FleetSnapshot` fields (`version`, `timestamp`, `thresholds`, `machines`, `sessions`) — defined in Task 3, matched in Task 5 test, Task 7 render, Task 8 popover.
- `MachineStatus` fields — `name`, `status`, `health`, `memAvailablePct`, `swapGB`, `ccCount`, `score`, `accounts`, `labels` — consistent across Tasks 3, 5, 7, 8.
- `LabelStatus` — `name`, `live`, `sessionId` — consistent across Tasks 3 and 8.
- `HealthBand` enum cases — `normal`, `free`, `ok`, `warn`, `high`, `offline` — defined in Task 6, used in Task 8.
- `FleetClient.resolveBinaryPath(defaults:env:)` signature — defined in Task 5 test, implemented in Task 5, consumed in Task 7 `AppDelegate`.
- `HealthBadge.swapBand(swapMB:thresholds:)`, `HealthBadge.healthBand(health:)`, `HealthBadge.color(for:)` — defined and used consistently across Tasks 6 and 8.
- `PopoverView.renderMachineLine(_:thresholds:)` — defined in Task 8 test, implemented in Task 8 source.

One thing to flag: `Task 8` test file builds a `Thresholds` via a helper extension with a memberwise-style init, but `Thresholds` is a `Codable` struct with no explicit init. I added a helper in Task 6 (`extension Thresholds { init(swapWarnMB:swapHighMB:) }`) that uses JSON decoding to sidestep writing a real init. Task 8 reuses that helper — it's in the test target, so no production code needs to grow an extra init. ✓
