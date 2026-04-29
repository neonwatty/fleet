import SwiftUI

struct PreferencesView: View {
    @AppStorage("fleetBinPath") private var fleetBinPath: String = ""
    @AppStorage("fleetConfigPath") private var fleetConfigPath: String = ""
    @AppStorage("fleetStatePath") private var fleetStatePath: String = ""
    @AppStorage("refreshInterval") private var refreshInterval: Double = 10

    var body: some View {
        Form {
            Section {
                TextField("Fleet binary path", text: $fleetBinPath)
                    .textFieldStyle(.roundedBorder)
                    .frame(minWidth: 360)
                TextField("Config path", text: $fleetConfigPath)
                    .textFieldStyle(.roundedBorder)
                TextField("State path", text: $fleetStatePath)
                    .textFieldStyle(.roundedBorder)
                Stepper(value: $refreshInterval, in: 2...120, step: 1) {
                    Text("Refresh every \(Int(refreshInterval)) seconds")
                }
            } footer: {
                Text("Leave paths empty to use fleet defaults. Restart Fleet Menu Bar after changing these settings.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(20)
        .frame(width: 460)
    }
}
