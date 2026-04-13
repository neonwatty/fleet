import SwiftUI

struct PopoverView: View {
    @ObservedObject var client: FleetClient

    var body: some View {
        Text("fleet (loading)")
            .padding()
            .frame(width: 320, height: 420)
    }
}
