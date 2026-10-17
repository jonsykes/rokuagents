import Foundation
import Observation

/// Runs `managents serve` (the copy inside the app) and restarts it when it
/// exits. Its output goes to the log file; the last line is kept for the menu.
@MainActor @Observable
final class ServerProcess {
    enum State: Equatable {
        case stopped
        case running(address: String)
        case failed(String)
    }

    struct Options: Equatable {
        var address: String
        var token: String
        var titles: Bool
        var activity: Bool
    }

    private(set) var state: State = .stopped
    @ObservationIgnored private var process: Process?
    @ObservationIgnored private var options: Options?
    @ObservationIgnored private var restart: Task<Void, Never>?
    @ObservationIgnored private var lastLine = ""

    let binary: URL
    let logURL: URL

    init(binary: URL, logURL: URL) {
        self.binary = binary
        self.logURL = logURL
        stopLeftovers()
    }

    /// Starts the server with options, replacing a running one whose options differ.
    func start(_ options: Options, force: Bool = false) {
        if !force, options == self.options, process != nil { return }
        self.options = options
        launch()
    }

    func stop() {
        options = nil
        restart?.cancel()
        terminate()
        state = .stopped
    }

    /// Stops the server before the app quits, waiting briefly for it to exit.
    func shutdown() {
        let running = process
        stop()
        running?.waitUntilExit()
    }

    private func launch() {
        restart?.cancel()
        terminate()
        guard let options else { return }

        let server = Process()
        server.executableURL = binary
        server.arguments = ["serve", "--addr", options.address, "--titles=\(options.titles)",
                            "--activity=\(options.activity)"]
        var environment = ProcessInfo.processInfo.environment
        environment["MANAGENTS_TOKEN"] = options.token
        server.environment = environment

        let output = Pipe()
        server.standardOutput = output
        server.standardError = output
        let log = openLog()
        output.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty else {
                handle.readabilityHandler = nil
                try? log?.close()
                return
            }
            try? log?.write(contentsOf: data)
            let text = String(decoding: data, as: UTF8.self)
            Task { @MainActor in self?.noteOutput(text) }
        }
        server.terminationHandler = { [weak self] ended in
            Task { @MainActor in self?.exited(ended) }
        }

        lastLine = ""
        do {
            try server.run()
            process = server
            state = .running(address: options.address)
        } catch {
            state = .failed(error.localizedDescription)
            scheduleRestart()
        }
    }

    private func terminate() {
        guard let running = process else { return }
        process = nil // so its exit does not schedule a restart
        if running.isRunning { running.terminate() }
    }

    private func exited(_ ended: Process) {
        guard ended === process else { return } // replaced or stopped on purpose
        process = nil
        guard options != nil else {
            state = .stopped
            return
        }
        state = .failed(lastLine.isEmpty ? "The server stopped (exit \(ended.terminationStatus))." : lastLine)
        scheduleRestart()
    }

    private func scheduleRestart() {
        restart?.cancel()
        restart = Task { [weak self] in
            try? await Task.sleep(for: .seconds(5))
            guard !Task.isCancelled else { return }
            self?.launch()
        }
    }

    private func noteOutput(_ text: String) {
        if let line = text.split(whereSeparator: \.isNewline).last {
            lastLine = Self.readable(String(line))
        }
    }

    /// "managents: listen tcp ...: bind: ..." reads better without the prefix,
    /// and a structured log line by its msg.
    static func readable(_ line: String) -> String {
        if let range = line.range(of: "msg=\"") {
            let rest = line[range.upperBound...]
            return String(rest.prefix { $0 != "\"" })
        }
        return line.hasPrefix("managents: ") ? String(line.dropFirst("managents: ".count)) : line
    }

    private func openLog() -> FileHandle? {
        let files = FileManager.default
        try? files.createDirectory(at: logURL.deletingLastPathComponent(), withIntermediateDirectories: true)
        if !files.fileExists(atPath: logURL.path) {
            files.createFile(atPath: logURL.path, contents: nil)
        }
        let handle = try? FileHandle(forWritingTo: logURL)
        _ = try? handle?.seekToEnd()
        return handle
    }

    /// A server left behind by a crash of the app would hold the port.
    private func stopLeftovers() {
        let pkill = Process()
        pkill.executableURL = URL(fileURLWithPath: "/usr/bin/pkill")
        pkill.arguments = ["-f", "^\(binary.path) serve"]
        try? pkill.run()
        pkill.waitUntilExit()
    }
}
