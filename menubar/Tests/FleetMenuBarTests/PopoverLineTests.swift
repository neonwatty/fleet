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
