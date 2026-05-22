import Foundation

/// Protocol for running CLI commands — allows faking in tests.
protocol CommandRunning {
    /// Run a command synchronously and return stdout as a string.
    func run(_ executable: String, _ arguments: [String]) throws -> String
    /// Start a command as a background process and return the Process object.
    func start(_ executable: String, _ arguments: [String]) throws -> Process
}

enum CommandRunnerError: Error {
    case timeout
}

/// Default implementation using Foundation Process.
final class SystemCommandRunner: CommandRunning {
    func run(_ executable: String, _ arguments: [String]) throws -> String {
        try run(executable, arguments, timeoutSeconds: 8)
    }

    func run(_ executable: String, _ arguments: [String], timeoutSeconds: TimeInterval) throws -> String {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: executable)
        process.arguments = arguments

        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = pipe

        try process.run()

        let deadline = Date().addingTimeInterval(timeoutSeconds)
        while process.isRunning {
            if Date() >= deadline {
                process.terminate()
                // Give it a brief chance to exit, then force kill.
                Thread.sleep(forTimeInterval: 0.2)
                if process.isRunning {
                    kill(process.processIdentifier, SIGKILL)
                }
                throw CommandRunnerError.timeout
            }
            Thread.sleep(forTimeInterval: 0.05)
        }

        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        return String(data: data, encoding: .utf8) ?? ""
    }

    func start(_ executable: String, _ arguments: [String]) throws -> Process {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: executable)
        process.arguments = arguments

        let outPipe = Pipe()
        let errPipe = Pipe()
        process.standardOutput = outPipe
        process.standardError = errPipe

        try process.run()
        return process
    }
}
