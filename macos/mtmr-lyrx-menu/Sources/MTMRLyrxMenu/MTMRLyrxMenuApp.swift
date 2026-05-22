import SwiftUI
import AppKit

@main
struct MTMRLyrxMenuApp: App {
    @StateObject private var appState = AppState()

    var body: some Scene {
        MenuBarExtra {
            MenuView()
                .environmentObject(appState)
        } label: {
            Image(nsImage: menuBarIcon())
        }
        .menuBarExtraStyle(.window)
    }
}

private func menuBarIcon() -> NSImage {
    if let url = Bundle.main.url(forResource: "MenuBarIcon", withExtension: "png"),
       let image = NSImage(contentsOf: url) {
        image.size = NSSize(width: 18, height: 18)
        image.isTemplate = false
        return image
    }

    if let fallback = NSImage(systemSymbolName: "music.note", accessibilityDescription: "mtmr-lyrx") {
        fallback.size = NSSize(width: 18, height: 18)
        fallback.isTemplate = true
        return fallback
    }

    return NSImage(size: NSSize(width: 18, height: 18))
}
