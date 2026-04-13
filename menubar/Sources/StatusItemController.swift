import AppKit
import Combine
import SwiftUI

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

        if error != nil {
            button.attributedTitle = Self.titleString("fleet ⚠", color: .systemRed)
            return
        }
        guard let snap = snapshot else {
            button.attributedTitle = Self.titleString("fleet …", color: .secondaryLabelColor)
            return
        }

        let machines = snap.machines
        let total = machines.count
        let online = machines.filter { $0.status == "online" }.count
        let cc = machines.reduce(0) { $0 + $1.ccCount }
        let hasStressed = machines.contains { $0.health == "stressed" }
        let hasBusy = machines.contains { $0.health == "busy" }

        let prefix = (hasStressed || hasBusy) ? "⚠ " : ""
        let text = "\(prefix)\(online)/\(total) · \(cc) CC"
        let color: NSColor = hasStressed ? .systemRed : (hasBusy ? .systemOrange : .labelColor)
        button.attributedTitle = Self.titleString(text, color: color)
    }

    private static func titleString(_ text: String, color: NSColor) -> NSAttributedString {
        NSAttributedString(
            string: text,
            attributes: [
                .foregroundColor: color,
                .font: NSFont.systemFont(ofSize: 13, weight: .medium),
            ]
        )
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
