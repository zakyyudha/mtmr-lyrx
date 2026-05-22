import Foundation
import AppKit
import Combine

/// Observable state model for the mtmr-lyrx menu bar controller.
@MainActor
final class AppState: ObservableObject {
    // MARK: - Published state

    @Published var binaryPath: String = ""
    @Published var daemonRunning: Bool = false
    @Published var externalDaemonDetected: Bool = false
    @Published var statusSummary: String = "Not connected"
    @Published var lastError: String? = nil
    @Published var loggedIn: Bool = false
    @Published var tokenValid: Bool = false
    @Published var trackName: String = ""
    @Published var artistName: String = ""
    @Published var isPlaying: Bool = false
    @Published var currentOffsetMS: Int = 0
    @Published var displayWidth: Int = 30
    @Published var scrollSpeedMS: Int = 200
    @Published var lastAction: String = ""
    @Published var updateAvailable: Bool = false
    @Published var latestVersion: String = ""
    @Published var updateStatus: String = "Updates not checked"
    @Published var updateInFlight: Bool = false

    // MARK: - Private

    private var managedProcess: Process? = nil
    private var refreshTimer: Timer? = nil
    private var lastStateFileMTime: Date? = nil
    private var statusRefreshInFlight: Bool = false
    private let runner: CommandRunning = SystemCommandRunner()

    private let configFilePath: String = {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        return "\(home)/.config/mtmr-lyrx/config.yaml"
    }()

    private let stateFilePath: String = {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        return "\(home)/.config/mtmr-lyrx/cache/current.txt"
    }()

    // PID file — written on daemon start, read on app launch to reattach
    private let pidFilePath: String = {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        return "\(home)/.config/mtmr-lyrx/daemon.pid"
    }()

    init() {
        binaryPath = resolveBinaryPath()
        refreshConfigValues()
        reattachIfRunning()
        startRefreshTimer()
    }

    // MARK: - Binary resolution

    private func resolveBinaryPath() -> String {
        if let saved = UserDefaults.standard.string(forKey: "binaryPath"), !saved.isEmpty,
           FileManager.default.fileExists(atPath: saved) {
            return saved
        }
        for path in ["./mtmr-lyrx", "/usr/local/bin/mtmr-lyrx", "/opt/homebrew/bin/mtmr-lyrx"] {
            if FileManager.default.fileExists(atPath: path) { return path }
        }
        return ""
    }

    // MARK: - PID file helpers

    private func writePID(_ pid: Int32) {
        try? "\(pid)".write(toFile: pidFilePath, atomically: true, encoding: .utf8)
    }

    private func readPID() -> Int32? {
        guard let s = try? String(contentsOfFile: pidFilePath, encoding: .utf8),
              let pid = Int32(s.trimmingCharacters(in: .whitespacesAndNewlines)) else { return nil }
        return pid
    }

    private func clearPID() {
        try? FileManager.default.removeItem(atPath: pidFilePath)
    }

    /// Returns true if a process with this PID is alive.
    private func isProcessAlive(_ pid: Int32) -> Bool {
        kill(pid, 0) == 0
    }

    /// Find an already-running mtmr-lyrx daemon not started by this app session.
    /// This prevents duplicate daemons after force-quit or manual terminal starts.
    private func findExistingDaemonPID() -> Int32? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/pgrep")
        process.arguments = ["-f", "mtmr-lyrx run"]

        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = Pipe()

        do {
            try process.run()
            process.waitUntilExit()
        } catch {
            return nil
        }

        guard process.terminationStatus == 0 else { return nil }
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        let output = String(data: data, encoding: .utf8) ?? ""
        let pids = output
            .split(whereSeparator: \.isNewline)
            .compactMap { Int32($0.trimmingCharacters(in: .whitespaces)) }
            .filter { isProcessAlive($0) }

        return pids.first
    }

    // MARK: - Reattach on launch

    /// On app launch, check if a daemon PID is recorded and still alive.
    /// If no PID file exists, detect an existing `mtmr-lyrx run` process and attach to it.
    private func reattachIfRunning() {
        if let pid = readPID(), isProcessAlive(pid) {
            daemonRunning = true
            return
        }

        clearPID()

        if let pid = findExistingDaemonPID() {
            writePID(pid)
            daemonRunning = true
            return
        }

        daemonRunning = false
    }

    // MARK: - Timer

    private func startRefreshTimer() {
        refreshTimer = Timer.scheduledTimer(withTimeInterval: 2.0, repeats: true) { [weak self] _ in
            Task { @MainActor [weak self] in
                await self?.refreshStatusAsync()
            }
        }
    }

    // MARK: - Daemon control

    func startDaemon() {
        guard !binaryPath.isEmpty else {
            lastError = "mtmr-lyrx binary not found."
            return
        }

        // If a PID-tracked daemon is already alive, don't spawn another.
        if let pid = readPID(), isProcessAlive(pid) {
            daemonRunning = true
            return
        }

        // If a daemon exists but no PID file exists (old app version, manual start,
        // or force-quit before PID write), attach instead of spawning duplicate.
        if let pid = findExistingDaemonPID() {
            writePID(pid)
            daemonRunning = true
            return
        }

        do {
            let proc = try runner.start(binaryPath, ["run"])
            managedProcess = proc
            daemonRunning = true
            lastError = nil
            writePID(proc.processIdentifier)

            proc.terminationHandler = { [weak self] p in
                Task { @MainActor [weak self] in
                    self?.daemonRunning = false
                    self?.managedProcess = nil
                    self?.clearPID()
                }
            }
        } catch {
            lastError = "Failed to start daemon: \(error.localizedDescription)"
        }
    }

    func stopDaemon() {
        // Try managed Process first
        if let proc = managedProcess, proc.isRunning {
            proc.terminate()
            managedProcess = nil
            daemonRunning = false
            clearPID()
            return
        }

        // Fall back to PID-based kill (reattached session)
        if let pid = readPID() {
            kill(pid, SIGTERM)
            clearPID()
        }
        daemonRunning = false
        managedProcess = nil
    }

    // MARK: - Status refresh

    func refreshStatus() {
        Task { await refreshStatusAsync() }
    }

    private func refreshStatusAsync() async {
        refreshConfigValues()

        if statusRefreshInFlight {
            return
        }
        statusRefreshInFlight = true
        defer { statusRefreshInFlight = false }

        // Sync daemonRunning with PID file reality
        if let pid = readPID(), isProcessAlive(pid) {
            daemonRunning = true
        } else if managedProcess == nil || !(managedProcess?.isRunning ?? false) {
            if daemonRunning {
                daemonRunning = false
                clearPID()
            }
        }

        // External daemon detection via state file mtime
        if let attrs = try? FileManager.default.attributesOfItem(atPath: stateFilePath),
           let mtime = attrs[.modificationDate] as? Date {
            let noManaged = managedProcess == nil || !(managedProcess?.isRunning ?? false)
            let noPID = readPID() == nil
            if noManaged && noPID, let last = lastStateFileMTime, mtime > last {
                externalDaemonDetected = true
            } else if !noManaged || !noPID {
                externalDaemonDetected = false
            }
            lastStateFileMTime = mtime
        }

        guard !binaryPath.isEmpty else { return }

        let bin = binaryPath
        let output = await Task.detached(priority: .background) {
            try? SystemCommandRunner().run(bin, ["status", "--json"])
        }.value

        guard let output,
              let data = output.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return }

        loggedIn = json["logged_in"] as? Bool ?? false
        tokenValid = json["token_valid"] as? Bool ?? false
        trackName = json["track_name"] as? String ?? ""
        artistName = json["artist_name"] as? String ?? ""
        isPlaying = json["is_playing"] as? Bool ?? false

        if let errMsg = json["error"] as? String, !errMsg.isEmpty {
            statusSummary = errMsg
        } else if !trackName.isEmpty {
            statusSummary = "\(artistName) — \(trackName)"
        } else if loggedIn {
            statusSummary = "Logged in, no track playing"
        } else {
            statusSummary = "Not logged in"
        }
    }

    // MARK: - Menu actions

    func openConfig() {
        NSWorkspace.shared.open(URL(fileURLWithPath: configFilePath))
    }

    func login() {
        guard !binaryPath.isEmpty else {
            lastError = "mtmr-lyrx binary not found."
            return
        }
        statusSummary = "Opening Spotify login..."
        lastError = nil
        let bin = binaryPath
        Task.detached(priority: .userInitiated) {
            do {
                // Run login — the Go binary opens the browser and waits for OAuth callback.
                // PATH must include /usr/bin so `open` works inside the subprocess.
                let process = Process()
                process.executableURL = URL(fileURLWithPath: bin)
                process.arguments = ["login"]
                // Inherit a sane PATH so exec.Command("open", ...) works inside Go
                var env = ProcessInfo.processInfo.environment
                if let path = env["PATH"], !path.contains("/usr/bin") {
                    env["PATH"] = "/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin:" + path
                } else if env["PATH"] == nil {
                    env["PATH"] = "/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin"
                }
                process.environment = env

                let outPipe = Pipe()
                let errPipe = Pipe()
                process.standardOutput = outPipe
                process.standardError = errPipe

                try process.run()
                process.waitUntilExit()

                let outData = outPipe.fileHandleForReading.readDataToEndOfFile()
                let errData = errPipe.fileHandleForReading.readDataToEndOfFile()
                let output = String(data: outData, encoding: .utf8) ?? ""
                let errOutput = String(data: errData, encoding: .utf8) ?? ""

                await MainActor.run {
                    if process.terminationStatus == 0 {
                        self.statusSummary = "Login successful"
                        self.lastError = nil
                    } else {
                        let msg = (errOutput + output).trimmingCharacters(in: .whitespacesAndNewlines)
                        self.lastError = msg.isEmpty ? "Login failed (exit \(process.terminationStatus))" : msg
                        self.statusSummary = "Login failed"
                    }
                }
            } catch {
                await MainActor.run {
                    self.lastError = "Login error: \(error.localizedDescription)"
                    self.statusSummary = "Login failed"
                }
            }
        }
    }

    func setConfigValue(key: String, value: String) {
        guard !binaryPath.isEmpty else { lastError = "mtmr-lyrx binary not found."; return }
        applyLocalConfigValue(key: key, value: value)
        lastAction = "Set \(displayLabel(for: key)) = \(value)"
        let bin = binaryPath
        Task.detached(priority: .background) {
            _ = try? SystemCommandRunner().run(bin, ["config", "set", key, value])
        }
    }

    func adjustOffset(_ deltaMs: Int) {
        guard !binaryPath.isEmpty else { return }
        currentOffsetMS += deltaMs
        lastAction = "Offset \(currentOffsetMS)ms"
        let bin = binaryPath
        let value = "\(currentOffsetMS)"
        Task.detached(priority: .background) {
            _ = try? SystemCommandRunner().run(bin, ["config", "set", "lyrics.offset_ms", value])
        }
    }

    func resetOffset() {
        guard !binaryPath.isEmpty else { return }
        currentOffsetMS = 0
        lastAction = "Offset 0ms"
        let bin = binaryPath
        Task.detached(priority: .background) {
            _ = try? SystemCommandRunner().run(bin, ["config", "set", "lyrics.offset_ms", "0"])
        }
    }

    private func applyLocalConfigValue(key: String, value: String) {
        switch key {
        case "display.width":
            if let n = Int(value) { displayWidth = n }
        case "display.scroll_speed_ms":
            if let n = Int(value) { scrollSpeedMS = n }
        case "lyrics.offset_ms":
            if let n = Int(value) { currentOffsetMS = n }
        default:
            break
        }
    }

    private func displayLabel(for key: String) -> String {
        switch key {
        case "display.width": return "width"
        case "display.scroll_speed_ms": return "speed"
        case "lyrics.offset_ms": return "offset"
        default: return key
        }
    }

    private func refreshConfigValues() {
        guard let content = try? String(contentsOfFile: configFilePath, encoding: .utf8) else { return }
        if let n = yamlInt(content, key: "offset_ms") { currentOffsetMS = n }
        if let n = yamlInt(content, key: "width") { displayWidth = n }
        if let n = yamlInt(content, key: "scroll_speed_ms") { scrollSpeedMS = n }
    }

    private func yamlInt(_ content: String, key: String) -> Int? {
        for line in content.split(whereSeparator: \.isNewline) {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if trimmed.hasPrefix("\(key):") {
                let parts = trimmed.split(separator: ":", maxSplits: 1)
                if parts.count == 2 {
                    return Int(parts[1].trimmingCharacters(in: .whitespaces))
                }
            }
        }
        return nil
    }

    // MARK: - Update

    func checkForUpdates() {
        guard !binaryPath.isEmpty else {
            updateStatus = "Binary not found"
            return
        }
        guard !updateInFlight else { return }
        updateInFlight = true
        let bin = binaryPath
        Task.detached(priority: .background) {
            let output = try? SystemCommandRunner().run(bin, ["update", "check", "--json"])
            await MainActor.run {
                self.updateInFlight = false
                guard let output,
                      let data = output.data(using: .utf8),
                      let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
                    self.updateStatus = "Update check failed"
                    return
                }
                if let errMsg = json["error"] as? String, !errMsg.isEmpty {
                    self.updateStatus = "Update check failed: \(errMsg)"
                    self.updateAvailable = false
                    return
                }
                let available = json["update_available"] as? Bool ?? false
                let latest = json["latest_version"] as? String ?? ""
                self.updateAvailable = available
                self.latestVersion = latest
                self.updateStatus = available ? "Update available: \(latest)" : "Up to date"
            }
        }
    }

    func installUpdate() {
        guard !binaryPath.isEmpty else {
            updateStatus = "Binary not found"
            return
        }
        guard updateAvailable, !updateInFlight else { return }
        updateInFlight = true
        let bin = binaryPath
        Task.detached(priority: .background) {
            let output = try? SystemCommandRunner().run(bin, ["update", "install", "--yes", "--json"])
            await MainActor.run {
                self.updateInFlight = false
                guard let output,
                      let data = output.data(using: .utf8),
                      let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
                    self.updateStatus = "Install failed"
                    return
                }
                if let errMsg = json["error"] as? String, !errMsg.isEmpty {
                    self.updateStatus = "Install failed: \(errMsg)"
                    return
                }
                let installed = json["installed"] as? Bool ?? false
                if installed {
                    self.updateAvailable = false
                    self.updateStatus = "Updated. Restart daemon/menu bar."
                } else {
                    let msg = json["message"] as? String ?? "Install did not complete"
                    self.updateStatus = msg
                }
            }
        }
    }
}
