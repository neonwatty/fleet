import SwiftUI

struct PreferencesView: View {
    @AppStorage("fleetBinPath") private var fleetBinPath: String = ""
    @AppStorage("refreshInterval") private var refreshInterval: Double = 10

    var body: some View {
        Form {
            Section {
                TextField("Fleet binary path", text: $fleetBinPath)
                    .textFieldStyle(.roundedBorder)
                    .frame(minWidth: 360)
                Stepper(value: $refreshInterval, in: 2...120, step: 1) {
                    Text("Refresh every \(Int(refreshInterval)) seconds")
                }
            } footer: {
                Text("Leave the binary path empty to use FLEET_BIN or /opt/homebrew/bin/fleet. Restart Fleet Menu Bar after changing these settings.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(20)
        .frame(width: 460)
    }
}
