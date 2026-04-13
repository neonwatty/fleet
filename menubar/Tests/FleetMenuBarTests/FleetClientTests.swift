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

    @MainActor
    func testRefreshDropsOverlappingCalls() {
        // Use a path that's guaranteed missing so `Process.run()` throws fast.
        // The failure still hops through DispatchQueue.main.async, which won't
        // run until after this test method returns — so `isRefreshing` stays
        // true for the duration of the test.
        let client = FleetClient(binaryPath: "/tmp/definitely-nonexistent-fleet-binary-xyz")
        XCTAssertFalse(client.isRefreshing)

        client.refresh()
        XCTAssertTrue(
            client.isRefreshing,
            "refresh() should set the in-flight flag synchronously before dispatching"
        )

        // A second call while the first is still in flight must be a no-op
        // (dropped, not queued).
        client.refresh()
        XCTAssertTrue(
            client.isRefreshing,
            "overlapping refresh() should leave the flag set without stacking work"
        )
    }
}
