import SwiftUI

enum HealthBand: Equatable {
    case normal
    case free
    case ok
    case warn
    case high
    case offline
}

enum HealthBadge {
    static func swapBand(swapMB: Double, thresholds: Thresholds) -> HealthBand {
        if swapMB >= Double(thresholds.swapHighMB) { return .high }
        if swapMB >= Double(thresholds.swapWarnMB) { return .warn }
        return .normal
    }

    static func healthBand(health: String) -> HealthBand {
        switch health {
        case "free": return .free
        case "ok": return .ok
        case "busy": return .warn
        case "stressed": return .high
        case "offline": return .offline
        default: return .normal
        }
    }

    static func color(for band: HealthBand) -> Color {
        switch band {
        case .normal: return .primary
        case .free: return .green
        case .ok: return .secondary
        case .warn: return .orange
        case .high: return .red
        case .offline: return .gray
        }
    }
}
