import XCTest
import SwiftUI
@testable import FleetMenuBar

@MainActor
final class PopoverLayoutTests: XCTestCase {

    private func snapshot(machineCount: Int) -> FleetSnapshot {
        var machines: [MachineStatus] = []
        for i in 0..<machineCount {
            machines.append(machine(index: i))
        }
        return FleetSnapshot(
            version: "1",
            timestamp: "2026-04-30T12:00:00Z",
            thresholds: Thresholds(swapWarnMB: 1024, swapHighMB: 4096),
            machines: machines,
            sessions: []
        )
    }

    private func machine(index i: Int) -> MachineStatus {
        MachineStatus(
            name: "very-long-machine-name-\(i)-with-extra-text",
            sshTarget: "user@very-long-machine-name-\(i).local",
            status: i % 3 == 2 ? "offline" : "online",
            health: i % 3 == 0 ? "free" : "busy",
            memAvailablePct: 55,
            swapGB: Double(i) / 10,
            ccCount: i % 2,
            score: 20,
            accounts: ["personal-max"],
            labels: [label(name: "feature-\(i)", live: i % 2 == 0)],
            agentProcesses: [AgentProcessStatus(kind: "codex", count: 1, rssMB: 256, pids: [1000 + i])]
        )
    }

    private func label(name: String, live: Bool) -> LabelStatus {
        let json = #"{"name":"\#(name)","live":\#(live),"session_id":"s1"}"#
        return try! JSONDecoder().decode(LabelStatus.self, from: Data(json.utf8))
    }

    func testPopoverWithManyMachinesKeepsExpectedWidthAndFiniteHeight() {
        let client = FleetClient(binaryPath: "/tmp/fleet", initialSnapshot: snapshot(machineCount: 18))
        let controller = NSHostingController(rootView: PopoverView(client: client))
        let size = controller.sizeThatFits(in: NSSize(width: 320, height: 1000))

        XCTAssertEqual(size.width, 320, accuracy: 1)
        XCTAssertGreaterThan(size.height, 100)
        XCTAssertLessThanOrEqual(size.height, 520)
    }

    func testPopoverErrorStateUsesExpectedWidth() {
        let client = FleetClient(binaryPath: "/tmp/fleet", initialError: String(repeating: "error ", count: 80))
        let controller = NSHostingController(rootView: PopoverView(client: client))
        let size = controller.sizeThatFits(in: NSSize(width: 320, height: 1000))

        XCTAssertEqual(size.width, 320, accuracy: 1)
        XCTAssertGreaterThan(size.height, 80)
        XCTAssertLessThanOrEqual(size.height, 340)
    }
}
