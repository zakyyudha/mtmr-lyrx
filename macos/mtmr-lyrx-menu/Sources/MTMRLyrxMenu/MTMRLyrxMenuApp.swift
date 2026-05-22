import SwiftUI

@main
struct MTMRLyrxMenuApp: App {
    @StateObject private var appState = AppState()

    var body: some Scene {
        MenuBarExtra("mtmr-lyrx", systemImage: "music.note") {
            MenuView()
                .environmentObject(appState)
        }
        .menuBarExtraStyle(.window)
    }
}
