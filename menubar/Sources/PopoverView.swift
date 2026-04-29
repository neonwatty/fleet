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
                ScrollView {
                    machineList(snap)
                }
                .frame(maxHeight: 420)
            } else {
                Text("Loading…")
                    .foregroundStyle(.secondary)
                    .font(.system(size: 12))
            }

            Divider()
            footer
        }
        .padding(12)
        .frame(width: 320)
        .fixedSize(horizontal: false, vertical: true)
    }

    private var header: some View {
        HStack(alignment: .firstTextBaseline) {
            Text("FLEET")
                .font(.system(size: 11, weight: .bold))
                .tracking(0.8)
                .foregroundStyle(.secondary)
            Spacer()
            VStack(alignment: .trailing, spacing: 1) {
                Text("v0.1.0")
                    .font(.system(size: 10))
                    .foregroundStyle(.tertiary)
                if let snap = client.snapshot {
                    Text(Self.lastUpdatedText(timestamp: snap.timestamp))
                        .font(.system(size: 10))
                        .foregroundStyle(.tertiary)
                }
            }
        }
    }

    private func errorBlock(_ error: String) -> some View {
        ScrollView {
            Text(error)
                .font(.system(size: 11, design: .monospaced))
                .foregroundStyle(.red)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .frame(maxHeight: 220)
    }

    private func machineList(_ snap: FleetSnapshot) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            ForEach(snap.machines, id: \.name) { m in
                machineCard(m, thresholds: snap.thresholds)
            }
        }
    }

    private func machineCard(_ m: MachineStatus, thresholds: Thresholds) -> some View {
        VStack(alignment: .leading, spacing: 3) {
            HStack(alignment: .firstTextBaseline) {
                Text(m.name)
                    .font(.system(size: 13, weight: .semibold))
                    .lineLimit(1)
                    .truncationMode(.middle)
                if !m.accounts.isEmpty {
                    Text("[\(m.accounts.joined(separator: ","))]")
                        .font(.system(size: 10))
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                        .truncationMode(.middle)
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
                    Text("\(Self.formatSwap(m.swapGB))GB swap")
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
                        .lineLimit(1)
                        .truncationMode(.middle)
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
            Button("Preferences") { Self.openPreferences() }
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
        let (path, args) = Self.openFullDashboardCommand()
        let proc = Process()
        proc.launchPath = path
        proc.arguments = args
        try? proc.run()
    }

    private static func openPreferences() {
        NSApp.sendAction(Selector(("showSettingsWindow:")), to: nil, from: nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    /// Returns the command used to open the full dashboard in Terminal.
    /// Separated so tests can assert the exact invocation without spawning a process.
    static func openFullDashboardCommand(
        defaults: UserDefaults = .standard,
        env: [String: String] = ProcessInfo.processInfo.environment
    ) -> (path: String, args: [String]) {
        let binary = FleetClient.resolveBinaryPath(defaults: defaults, env: env)
        let statusArgs = Array(FleetClient.statusArguments(
            configPath: FleetClient.resolveConfigPath(defaults: defaults),
            statePath: FleetClient.resolveStatePath(defaults: defaults)
        ).dropLast())
        let command = ([binary] + statusArgs).map(shellQuote).joined(separator: " ")
        return (
            "/usr/bin/osascript",
            ["-e", "tell application \"Terminal\" to do script \(appleScriptString(command))",
             "-e", "tell application \"Terminal\" to activate"]
        )
    }

    private static func shellQuote(_ value: String) -> String {
        if value.rangeOfCharacter(from: CharacterSet(charactersIn: " \t\n'\"\\$;&|()<>*?[]{}!")) == nil {
            return value
        }
        return "'" + value.replacingOccurrences(of: "'", with: "'\\''") + "'"
    }

    private static func appleScriptString(_ value: String) -> String {
        "\"" + value
            .replacingOccurrences(of: "\\", with: "\\\\")
            .replacingOccurrences(of: "\"", with: "\\\"") + "\""
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

    static func lastUpdatedText(timestamp: String) -> String {
        let parser = ISO8601DateFormatter()
        guard let date = parser.date(from: timestamp) else {
            return "Updated --"
        }
        let formatter = DateFormatter()
        formatter.dateFormat = "HH:mm:ss"
        formatter.timeZone = .current
        return "Updated \(formatter.string(from: date))"
    }
}
