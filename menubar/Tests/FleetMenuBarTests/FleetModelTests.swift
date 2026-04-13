import XCTest
@testable import FleetMenuBar

final class FleetModelTests: XCTestCase {

    func testDecodesFixtureSnapshot() throws {
        let url = try XCTUnwrap(
            Bundle(for: Self.self).url(forResource: "status", withExtension: "json"),
            "status.json fixture not found in test bundle"
        )
        let data = try Data(contentsOf: url)
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
        XCTAssertEqual(mm1.labels[1].sessionId, "")

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
