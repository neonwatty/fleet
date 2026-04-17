import AppKit
import Combine
import SwiftUI

enum IconState: Equatable {
    case loading
    case error
    case normal
    case busy
    case stressed
    case allOffline
}

@MainActor
final class StatusItemController {
    private let statusItem: NSStatusItem
    private let popover: NSPopover
    private let client: FleetClient
    private var cancellables = Set<AnyCancellable>()

    init(client: FleetClient) {
        self.client = client

        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)

        popover = NSPopover()
        popover.behavior = .transient
        let hosting = NSHostingController(rootView: PopoverView(client: client))
        hosting.sizingOptions = [.preferredContentSize]
        popover.contentViewController = hosting

        statusItem.button?.target = self
        statusItem.button?.action = #selector(togglePopover(_:))

        client.$snapshot
            .combineLatest(client.$lastError)
            .receive(on: RunLoop.main)
            .sink { [weak self] snap, err in
                self?.render(snapshot: snap, error: err)
            }
            .store(in: &cancellables)

        render(snapshot: nil, error: nil)
    }

    private func render(snapshot: FleetSnapshot?, error: String?) {
        guard let button = statusItem.button else { return }
        let state = Self.iconState(snapshot: snapshot, error: error)
        let config = NSImage.SymbolConfiguration(paletteColors: [Self.tintColor(for: state)])
        let image = NSImage(
            systemSymbolName: Self.symbolName(for: state),
            accessibilityDescription: Self.accessibilityLabel(snapshot: snapshot, error: error)
        )?.withSymbolConfiguration(config)
        image?.isTemplate = false
        button.image = image
        button.contentTintColor = nil
        button.attributedTitle = NSAttributedString()
    }

    nonisolated static func iconState(snapshot: FleetSnapshot?, error: String?) -> IconState {
        if error != nil { return .error }
        guard let snap = snapshot else { return .loading }
        if snap.machines.contains(where: { $0.health == "stressed" }) { return .stressed }
        if snap.machines.contains(where: { $0.health == "busy" }) { return .busy }
        if !snap.machines.isEmpty && snap.machines.allSatisfy({ $0.status == "offline" }) {
            return .allOffline
        }
        return .normal
    }

    private static func symbolName(for state: IconState) -> String {
        switch state {
        case .error: return "exclamationmark.triangle.fill"
        default: return "rectangle.stack.fill"
        }
    }

    private static func tintColor(for state: IconState) -> NSColor {
        switch state {
        case .loading, .allOffline: return .secondaryLabelColor
        case .error, .stressed: return .systemRed
        case .busy: return .systemOrange
        case .normal: return .systemGreen
        }
    }

    private static func accessibilityLabel(snapshot: FleetSnapshot?, error: String?) -> String {
        if error != nil { return "Fleet: error" }
        guard let snap = snapshot else { return "Fleet: loading" }
        let online = snap.machines.filter { $0.status == "online" }.count
        let total = snap.machines.count
        let cc = snap.machines.reduce(0) { $0 + $1.ccCount }
        return "Fleet: \(online) of \(total) online, \(cc) CC"
    }

    @objc private func togglePopover(_ sender: Any?) {
        guard let button = statusItem.button else { return }
        if popover.isShown {
            popover.performClose(sender)
        } else {
            client.refresh()
            popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
            popover.contentViewController?.view.window?.makeKey()
        }
    }
}
