import Foundation
import Combine

@MainActor
final class FleetClient: ObservableObject {
    @Published private(set) var snapshot: FleetSnapshot?
    @Published private(set) var lastError: String?

    private let binaryPath: String
    private let refreshInterval: TimeInterval
    private var timer: Timer?
    private var refreshInFlight = false
    private let queue = DispatchQueue(label: "com.neonwatty.FleetMenuBar.refresh", qos: .utility)

    /// Read-only view of the in-flight flag, for tests.
    var isRefreshing: Bool { refreshInFlight }

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
        // Drop overlapping calls. If `fleet status --json` takes longer than
        // `refreshInterval` (e.g. an SSH probe stalls), the timer keeps ticking
        // but we don't stack work on the background queue.
        guard !refreshInFlight else { return }
        refreshInFlight = true
        let path = binaryPath
        queue.async { [weak self] in
            let result = Self.runStatus(binaryPath: path)
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

    nonisolated static func decode(_ data: Data) throws -> FleetSnapshot {
        return try JSONDecoder().decode(FleetSnapshot.self, from: data)
    }

    nonisolated private enum RunResult {
        case success(FleetSnapshot)
        case failure(String)
    }

    nonisolated private static func runStatus(binaryPath: String) -> RunResult {
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
