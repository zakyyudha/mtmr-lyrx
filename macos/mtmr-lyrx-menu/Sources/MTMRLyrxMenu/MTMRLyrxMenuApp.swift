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
                .resizable()
                .scaledToFit()
                .frame(width: 18, height: 18)
        }
        .menuBarExtraStyle(.window)
    }
}

private func menuBarIcon() -> NSImage {
    if let url = Bundle.main.url(forResource: "AppIcon", withExtension: "icns"),
       let image = NSImage(contentsOf: url) {
        image.isTemplate = false
        return image
    }

    if let fallback = NSImage(systemSymbolName: "music.note", accessibilityDescription: "mtmr-lyrx") {
        fallback.isTemplate = true
        return fallback
    }

    return NSImage()
}
