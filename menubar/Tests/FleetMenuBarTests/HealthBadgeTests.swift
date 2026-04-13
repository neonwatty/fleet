import XCTest
import SwiftUI
@testable import FleetMenuBar

final class HealthBadgeTests: XCTestCase {

    private let thresholds = Thresholds(swapWarnMB: 1024, swapHighMB: 4096)

    func testSwapColorBelowWarnIsPrimary() {
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 0, thresholds: thresholds), .normal)
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 1023, thresholds: thresholds), .normal)
    }

    func testSwapColorAtWarnIsOrange() {
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 1024, thresholds: thresholds), .warn)
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 4095, thresholds: thresholds), .warn)
    }

    func testSwapColorAtHighIsRed() {
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 4096, thresholds: thresholds), .high)
        XCTAssertEqual(HealthBadge.swapBand(swapMB: 99999, thresholds: thresholds), .high)
    }

    func testHealthBandForEachLabel() {
        XCTAssertEqual(HealthBadge.healthBand(health: "free"), .free)
        XCTAssertEqual(HealthBadge.healthBand(health: "ok"), .ok)
        XCTAssertEqual(HealthBadge.healthBand(health: "busy"), .warn)
        XCTAssertEqual(HealthBadge.healthBand(health: "stressed"), .high)
        XCTAssertEqual(HealthBadge.healthBand(health: "offline"), .offline)
        XCTAssertEqual(HealthBadge.healthBand(health: "??"), .normal) // unknown fallback
    }
}

// Thresholds already Codable, but the test builds one directly.
extension Thresholds {
    init(swapWarnMB: Int, swapHighMB: Int) {
        let json = """
        {"swap_warn_mb":\(swapWarnMB),"swap_high_mb":\(swapHighMB)}
        """
        self = try! JSONDecoder().decode(Thresholds.self, from: Data(json.utf8))
    }
}
