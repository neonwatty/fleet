import Foundation
import Combine

@MainActor
final class FleetClient: ObservableObject {
    @Published private(set) var snapshot: FleetSnapshot?
    @Published private(set) var lastError: String?

    private let binaryPath: String
    private let configPath: String
    private let statePath: String
    private let refreshInterval: TimeInterval
    private var timer: Timer?
    private var refreshInFlight = false
    private let queue = DispatchQueue(label: "com.neonwatty.FleetMenuBar.refresh", qos: .utility)

    /// Read-only view of the in-flight flag, for tests.
    var isRefreshing: Bool { refreshInFlight }

    init(
        binaryPath: String,
        configPath: String = "",
        statePath: String = "",
        refreshInterval: TimeInterval = 10
    ) {
        self.binaryPath = binaryPath
        self.configPath = configPath
        self.statePath = statePath
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
        // Drop overlapping calls. If `fleet status --json` takes longer than
        // `refreshInterval` (e.g. an SSH probe stalls), the timer keeps ticking
        // but we don't stack work on the background queue.
        guard !refreshInFlight else { return }
        refreshInFlight = true
        let path = binaryPath
        let config = configPath
        let state = statePath
        queue.async { [weak self] in
            let result = Self.runStatus(binaryPath: path, configPath: config, statePath: state)
            DispatchQueue.main.async {
                guard let self else { return }
                self.refreshInFlight = false
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

    nonisolated static func resolveBinaryPath(defaults: UserDefaults, env: [String: String]) -> String {
        if let override = defaults.string(forKey: "fleetBinPath"), !override.isEmpty {
            return override
        }
        if let envPath = env["FLEET_BIN"], !envPath.isEmpty {
            return envPath
        }
        return "/opt/homebrew/bin/fleet"
    }

    nonisolated static func resolveConfigPath(defaults: UserDefaults) -> String {
        defaults.string(forKey: "fleetConfigPath")?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    }

    nonisolated static func resolveStatePath(defaults: UserDefaults) -> String {
        defaults.string(forKey: "fleetStatePath")?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    }

    nonisolated static func statusArguments(configPath: String = "", statePath: String = "") -> [String] {
        var args: [String] = []
        let config = configPath.trimmingCharacters(in: .whitespacesAndNewlines)
        let state = statePath.trimmingCharacters(in: .whitespacesAndNewlines)
        if !config.isEmpty {
            args.append(contentsOf: ["--config", config])
        }
        if !state.isEmpty {
            args.append(contentsOf: ["--state", state])
        }
        args.append(contentsOf: ["status", "--json"])
        return args
    }

    nonisolated static func decode(_ data: Data) throws -> FleetSnapshot {
        return try JSONDecoder().decode(FleetSnapshot.self, from: data)
    }

    nonisolated enum RunResult {
        case success(FleetSnapshot)
        case failure(String)
    }

    nonisolated static func runStatus(
        binaryPath: String,
        configPath: String = "",
        statePath: String = "",
        timeout: TimeInterval = 20
    ) -> RunResult {
        let process = Process()
        process.launchPath = binaryPath
        process.arguments = statusArguments(configPath: configPath, statePath: statePath)

        let stdout = Pipe()
        let stderr = Pipe()
        process.standardOutput = stdout
        process.standardError = stderr
        let finished = DispatchSemaphore(value: 0)
        process.terminationHandler = { _ in finished.signal() }

        do {
            try process.run()
        } catch {
            return .failure("launch failed: \(error.localizedDescription)")
        }
        if finished.wait(timeout: .now() + timeout) == .timedOut {
            process.terminate()
            _ = finished.wait(timeout: .now() + 1)
            return .failure("fleet status --json timed out after \(Int(timeout))s")
        }

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
