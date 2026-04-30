import XCTest
@testable import FleetMenuBar

final class StatusItemControllerIconStateTests: XCTestCase {

    private let thresholds = Thresholds(swapWarnMB: 1024, swapHighMB: 4096)

    private func snapshot(_ machines: [MachineStatus]) -> FleetSnapshot {
        FleetSnapshot(
            version: "1",
            timestamp: "2026-04-13T00:00:00Z",
            thresholds: thresholds,
            machines: machines,
            sessions: []
        )
    }

    private func machine(health: String = "free", status: String = "online") -> MachineStatus {
        MachineStatus(
            name: "m",
            status: status,
            health: health,
            memAvailablePct: 50,
            swapGB: 0,
            ccCount: 0,
            score: 0,
            accounts: [],
            labels: []
        )
    }

    func testLoadingWhenNoSnapshotAndNoError() {
        XCTAssertEqual(StatusItemController.iconState(snapshot: nil, error: nil), .loading)
    }

    func testErrorWinsEvenWithSnapshot() {
        XCTAssertEqual(StatusItemController.iconState(snapshot: nil, error: "boom"), .error)
        let snap = snapshot([machine()])
        XCTAssertEqual(StatusItemController.iconState(snapshot: snap, error: "boom"), .error)
    }

    func testStressedWinsOverBusy() {
        let snap = snapshot([
            machine(health: "busy"),
            machine(health: "stressed"),
            machine(health: "free"),
        ])
        XCTAssertEqual(StatusItemController.iconState(snapshot: snap, error: nil), .stressed)
    }

    func testBusy() {
        let snap = snapshot([machine(health: "busy"), machine(health: "free")])
        XCTAssertEqual(StatusItemController.iconState(snapshot: snap, error: nil), .busy)
    }

    func testNormal() {
        let snap = snapshot([machine(health: "free"), machine(health: "ok")])
        XCTAssertEqual(StatusItemController.iconState(snapshot: snap, error: nil), .normal)
    }

    func testOfflineMachineDoesNotOverrideHealthyOnlineMachines() {
        let snap = snapshot([
            machine(health: "free"),
            machine(health: "ok"),
            machine(health: "", status: "offline"),
        ])
        XCTAssertEqual(StatusItemController.iconState(snapshot: snap, error: nil), .normal)
    }

    func testAllOffline() {
        let snap = snapshot([
            machine(health: "offline", status: "offline"),
            machine(health: "offline", status: "offline"),
        ])
        XCTAssertEqual(StatusItemController.iconState(snapshot: snap, error: nil), .allOffline)
    }

    func testEmptyMachinesIsNormal() {
        let snap = snapshot([])
        XCTAssertEqual(StatusItemController.iconState(snapshot: snap, error: nil), .normal)
    }

    func testStatusTitleSummarizesOnlineAndCCCounts() {
        let snap = snapshot([
            machine(health: "free", status: "online"),
            machine(health: "busy", status: "online"),
            machine(health: "offline", status: "offline"),
        ])
        let withCounts = FleetSnapshot(
            version: snap.version,
            timestamp: snap.timestamp,
            thresholds: snap.thresholds,
            machines: [
                MachineStatus(name: "mm1", status: "online", health: "free", memAvailablePct: 50, swapGB: 0, ccCount: 2, score: 10, accounts: [], labels: []),
                MachineStatus(name: "mm2", status: "online", health: "busy", memAvailablePct: 25, swapGB: 1, ccCount: 1, score: 5, accounts: [], labels: []),
                MachineStatus(name: "mm3", status: "offline", health: "", memAvailablePct: 0, swapGB: 0, ccCount: 0, score: 0, accounts: [], labels: []),
            ],
            sessions: []
        )
        XCTAssertEqual(StatusItemController.statusTitle(snapshot: withCounts), " 2/3 · 3 CC")
    }

    func testStatusTitleIsBlankBeforeFirstSnapshot() {
        XCTAssertEqual(StatusItemController.statusTitle(snapshot: nil), "")
    }
}
