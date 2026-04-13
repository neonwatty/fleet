import AppKit

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var client: FleetClient!
    private var controller: StatusItemController!

    func applicationDidFinishLaunching(_ notification: Notification) {
        let binaryPath = FleetClient.resolveBinaryPath(
            defaults: UserDefaults.standard,
            env: ProcessInfo.processInfo.environment
        )
        let interval = UserDefaults.standard.double(forKey: "refreshInterval")
        client = FleetClient(
            binaryPath: binaryPath,
            refreshInterval: interval > 0 ? interval : 10
        )
        controller = StatusItemController(client: client)
        client.start()
    }
}
