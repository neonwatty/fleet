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
    let sshTarget: String?
    let status: String
    let health: String
    let memAvailablePct: Int
    let swapGB: Double
    let ccCount: Int
    let score: Double
    let accounts: [String]
    let labels: [LabelStatus]
    let agentProcesses: [AgentProcessStatus]?

    // Explicit memberwise init so test helpers can build instances by field,
    // even if a custom init(from:) is added later and removes the synthesized one.
    init(
        name: String,
        sshTarget: String? = nil,
        status: String,
        health: String,
        memAvailablePct: Int,
        swapGB: Double,
        ccCount: Int,
        score: Double,
        accounts: [String],
        labels: [LabelStatus],
        agentProcesses: [AgentProcessStatus]? = nil
    ) {
        self.name = name
        self.sshTarget = sshTarget
        self.status = status
        self.health = health
        self.memAvailablePct = memAvailablePct
        self.swapGB = swapGB
        self.ccCount = ccCount
        self.score = score
        self.accounts = accounts
        self.labels = labels
        self.agentProcesses = agentProcesses
    }

    enum CodingKeys: String, CodingKey {
        case name, status, health, score, accounts, labels
        case sshTarget = "ssh_target"
        case memAvailablePct = "mem_available_pct"
        case swapGB = "swap_gb"
        case ccCount = "cc_count"
        case agentProcesses = "agent_processes"
    }
}

struct AgentProcessStatus: Codable {
    let kind: String
    let count: Int
    let rssMB: Int
    let pids: [Int]

    enum CodingKeys: String, CodingKey {
        case kind, count, pids
        case rssMB = "rss_mb"
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
    let launchCommand: String?
    let label: String?
    let tunnelLocalPort: Int
    let tunnelRemotePort: Int
    let startedAt: String

    enum CodingKeys: String, CodingKey {
        case id, project, machine, branch, account, label
        case launchCommand = "launch_command"
        case tunnelLocalPort = "tunnel_local_port"
        case tunnelRemotePort = "tunnel_remote_port"
        case startedAt = "started_at"
    }
}
