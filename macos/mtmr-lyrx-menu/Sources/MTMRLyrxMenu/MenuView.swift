import SwiftUI

struct MenuView: View {
    @EnvironmentObject var appState: AppState

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {

            HStack {
                Circle()
                    .fill(appState.daemonRunning ? Color.green : Color.gray)
                    .frame(width: 8, height: 8)
                Text(appState.statusSummary)
                    .font(.system(size: 12))
                    .lineLimit(1)
                    .truncationMode(.tail)
            }
            .padding(.horizontal, 8)
            .padding(.top, 6)
            .padding(.bottom, 2)

            if appState.externalDaemonDetected {
                Text("External daemon detected")
                    .font(.system(size: 10))
                    .foregroundColor(.orange)
                    .padding(.horizontal, 8)
            }

            if let err = appState.lastError {
                Text(err)
                    .font(.system(size: 10))
                    .foregroundColor(.red)
                    .lineLimit(2)
                    .padding(.horizontal, 8)
            }

            Divider().padding(.vertical, 2)

            HoverMenuButton(
                label: appState.daemonRunning ? "Stop Daemon" : "Start Daemon",
                icon: appState.daemonRunning ? "stop.circle" : "play.circle",
                disabled: appState.binaryPath.isEmpty
            ) {
                if appState.daemonRunning {
                    appState.stopDaemon()
                } else {
                    appState.startDaemon()
                }
            }

            Divider().padding(.vertical, 2)

            HoverMenuButton(label: "Spotify Login", icon: "person.badge.key") {
                appState.login()
            }

            HoverMenuButton(label: "Open Config", icon: "gear") {
                appState.openConfig()
            }

            Divider().padding(.vertical, 2)

            Text("Timing Offset: \(appState.currentOffsetMS)ms")
                .font(.system(size: 10, weight: .semibold))
                .foregroundColor(.secondary)
                .padding(.horizontal, 8)

            HStack(spacing: 4) {
                HoverPillButton("-500") { appState.adjustOffset(-500) }
                HoverPillButton("-100") { appState.adjustOffset(-100) }
                HoverPillButton("0") { appState.resetOffset() }
                HoverPillButton("+100") { appState.adjustOffset(100) }
                HoverPillButton("+500") { appState.adjustOffset(500) }
            }
            .padding(.horizontal, 8)

            Divider().padding(.vertical, 2)

            Text("Display Width: \(appState.displayWidth)")
                .font(.system(size: 10, weight: .semibold))
                .foregroundColor(.secondary)
                .padding(.horizontal, 8)

            HStack(spacing: 4) {
                HoverPillButton("30", selected: appState.displayWidth == 30) { appState.setConfigValue(key: "display.width", value: "30") }
                HoverPillButton("45", selected: appState.displayWidth == 45) { appState.setConfigValue(key: "display.width", value: "45") }
                HoverPillButton("60", selected: appState.displayWidth == 60) { appState.setConfigValue(key: "display.width", value: "60") }
            }
            .padding(.horizontal, 8)

            Text("Scroll Speed: \(appState.scrollSpeedMS)ms")
                .font(.system(size: 10, weight: .semibold))
                .foregroundColor(.secondary)
                .padding(.horizontal, 8)
                .padding(.top, 2)

            HStack(spacing: 4) {
                HoverPillButton("150", selected: appState.scrollSpeedMS == 150) { appState.setConfigValue(key: "display.scroll_speed_ms", value: "150") }
                HoverPillButton("200", selected: appState.scrollSpeedMS == 200) { appState.setConfigValue(key: "display.scroll_speed_ms", value: "200") }
                HoverPillButton("300", selected: appState.scrollSpeedMS == 300) { appState.setConfigValue(key: "display.scroll_speed_ms", value: "300") }
            }
            .padding(.horizontal, 8)

            Divider().padding(.vertical, 2)

            Text("Updates: \(appState.updateStatus)")
                .font(.system(size: 10, weight: .semibold))
                .foregroundColor(.secondary)
                .lineLimit(2)
                .padding(.horizontal, 8)

            HStack(spacing: 4) {
                HoverPillButton("Check", selected: false) {
                    appState.checkForUpdates()
                }
                HoverPillButton("Install", selected: appState.updateAvailable) {
                    appState.installUpdate()
                }
            }
            .padding(.horizontal, 8)

            Divider().padding(.vertical, 2)

            HoverMenuButton(label: "Quit", icon: "xmark.circle") {
                NSApplication.shared.terminate(nil)
            }

            Spacer(minLength: 4)
        }
        .frame(width: 240)
        .padding(.bottom, 2)
    }
}

struct HoverMenuButton: View {
    let label: String
    let icon: String
    var disabled: Bool = false
    let action: () -> Void

    @State private var hovered = false

    var body: some View {
        Button(action: action) {
            Label(label, systemImage: icon)
                .font(.system(size: 12))
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 8)
                .padding(.vertical, 4)
                .foregroundColor(disabled ? .secondary : (hovered ? .white : .primary))
                .background(
                    RoundedRectangle(cornerRadius: 6)
                        .fill(hovered && !disabled ? Color.accentColor : Color.clear)
                )
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .disabled(disabled)
        .padding(.horizontal, 4)
        .onHover { inside in
            hovered = inside
        }
    }
}

struct HoverPillButton: View {
    let label: String
    var selected: Bool = false
    let action: () -> Void

    @State private var hovered = false

    init(_ label: String, selected: Bool = false, action: @escaping () -> Void) {
        self.label = label
        self.selected = selected
        self.action = action
    }

    var body: some View {
        Button(action: action) {
            Text(label)
                .font(.system(size: 10, weight: .medium))
                .foregroundColor((selected || hovered) ? .white : .primary)
                .padding(.horizontal, 9)
                .padding(.vertical, 4)
                .background(
                    RoundedRectangle(cornerRadius: 6)
                        .fill(selected ? Color.accentColor : (hovered ? Color.accentColor.opacity(0.85) : Color.gray.opacity(0.35)))
                )
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .onHover { inside in
            hovered = inside
        }
    }
}
