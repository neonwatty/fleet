import SwiftUI

enum HealthBand: Equatable {
    case normal      // default foreground
    case free        // green
    case ok          // muted green / secondary
    case warn        // orange
    case high        // red
    case offline     // gray
}

struct HealthBadge: View {
    let health: String
    let swapMB: Double
    let thresholds: Thresholds

    var body: some View {
        Text(health)
            .font(.system(size: 11, weight: .medium))
            .foregroundColor(Self.color(for: Self.healthBand(health: health)))
    }

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
