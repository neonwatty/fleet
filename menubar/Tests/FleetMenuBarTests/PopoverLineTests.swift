import XCTest
@testable import FleetMenuBar

final class PopoverLineTests: XCTestCase {

    private let thresholds = Thresholds(swapWarnMB: 1024, swapHighMB: 4096)

    private func machine(
        name: String = "mm1",
        sshTarget: String? = nil,
        status: String = "online",
        health: String = "ok",
        memPct: Int = 45,
        swapGB: Double = 0.5,
        cc: Int = 1,
        accounts: [String] = [],
        labels: [LabelStatus] = [],
        agentProcessesJSON: String = "[]"
    ) -> MachineStatus {
        let json = """
        {
          "name":"\(name)",\(sshTargetJSON(sshTarget))"status":"\(status)","health":"\(health)",
          "mem_available_pct":\(memPct),"swap_gb":\(swapGB),"cc_count":\(cc),
          "score":10,"accounts":\(accountsJSON(accounts)),
          "agent_processes":\(agentProcessesJSON),"labels":[]
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

    private func sshTargetJSON(_ sshTarget: String?) -> String {
        guard let sshTarget else {
            return ""
        }
        return "\"ssh_target\":\"\(sshTarget)\","
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

    func testRenderOnlineMachineIncludesSSHTarget() {
        let m = machine(sshTarget: "jeremywatt@mm0", health: "free")
        let line = PopoverView.renderMachineLine(m, thresholds: thresholds)
        XCTAssertTrue(line.contains("jeremywatt@mm0"), "line = \(line)")
    }

    func testSSHCommandPrefixesTarget() {
        XCTAssertEqual(PopoverView.sshCommand("jeremywatt@mm0"), "ssh jeremywatt@mm0")
    }

    func testRenderOnlineMachineIncludesAgentProcesses() {
        let m = machine(
            agentProcessesJSON: """
            [
              {"kind":"codex","count":1,"rss_mb":164,"pids":[40623]},
              {"kind":"claude","count":2,"rss_mb":512,"pids":[100,101]}
            ]
            """
        )
        let line = PopoverView.renderMachineLine(m, thresholds: thresholds)
        XCTAssertTrue(line.contains("Claude: 2"), "line = \(line)")
        XCTAssertTrue(line.contains("Codex: 1"), "line = \(line)")
    }

    func testAgentProcessSummaryOmitsEmptyProcesses() {
        let m = machine(agentProcessesJSON: """
        [{"kind":"codex","count":0,"rss_mb":0,"pids":[]}]
        """)
        XCTAssertNil(PopoverView.agentProcessSummary(m.agentProcesses))
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

    func testLastUpdatedTextFormatsValidTimestamp() {
        let text = PopoverView.lastUpdatedText(timestamp: "2026-04-29T12:17:52Z")
        XCTAssertTrue(text.hasPrefix("Updated "))
        XCTAssertFalse(text.contains("--"))
    }

    func testLastUpdatedTextHandlesInvalidTimestamp() {
        XCTAssertEqual(PopoverView.lastUpdatedText(timestamp: "not-a-date"), "Updated --")
    }

    func testOpenFullDashboardCommandUsesOsascript() {
        let (path, args) = PopoverView.openFullDashboardCommand()
        XCTAssertEqual(path, "/usr/bin/osascript")
        XCTAssertTrue(args.contains("-e"))
        XCTAssertTrue(
            args.contains(where: { $0.contains("do script \"/opt/homebrew/bin/fleet status\"") }),
            "args should tell Terminal to run fleet status, got: \(args)"
        )
        XCTAssertTrue(
            args.contains(where: { $0.contains("activate") }),
            "args should activate Terminal to bring it to front, got: \(args)"
        )
    }

    func testOpenFullDashboardCommandUsesConfiguredPaths() {
        let defaults = UserDefaults(suiteName: "FleetMenuBarTests.\(UUID())")!
        defaults.set("/tmp/fleet bin/fleet", forKey: "fleetBinPath")
        defaults.set("/tmp/fleet config.toml", forKey: "fleetConfigPath")
        defaults.set("/tmp/state.json", forKey: "fleetStatePath")

        let (path, args) = PopoverView.openFullDashboardCommand(defaults: defaults, env: [:])
        XCTAssertEqual(path, "/usr/bin/osascript")
        XCTAssertTrue(
            args.contains(where: {
                $0.contains("'/tmp/fleet bin/fleet' --config '/tmp/fleet config.toml' --state /tmp/state.json status")
            }),
            "args should include configured binary/config/state paths, got: \(args)"
        )
        XCTAssertFalse(args.contains(where: { $0.contains("--json") }))
    }
}

// Test-only helper so we can build a MachineStatus with custom labels.
extension MachineStatus {
    init(copying base: MachineStatus, labels: [LabelStatus]) {
        self.init(
            name: base.name, sshTarget: base.sshTarget,
            status: base.status, health: base.health,
            memAvailablePct: base.memAvailablePct, swapGB: base.swapGB,
            ccCount: base.ccCount, score: base.score,
            accounts: base.accounts, labels: labels,
            agentProcesses: base.agentProcesses
        )
    }
}
