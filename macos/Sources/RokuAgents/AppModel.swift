import AppKit
import Foundation
import Network
import Observation
import RokuAgentsCore
import ServiceManagement

/// Everything the menu and the settings window show and do. It keeps the
/// server on the Mac's current LAN address and, when that address changes,
/// reinstalls the Roku channel so it follows.
@MainActor @Observable
final class AppModel {
    static let shared = AppModel()

    enum InstallState: Equatable {
        case idle
        case installing
        case done(String)
        case failed(String)
    }

    // Settings, saved in UserDefaults.
    private(set) var port: Int
    private(set) var pinnedInterface: String? // nil: automatic
    private(set) var titles: Bool
    private(set) var rokuHost: String
    private(set) var autoReinstall: Bool

    private(set) var token = ""
    private(set) var tokenError: String?
    private(set) var interfaces: [NetworkInterface] = []
    /// ip:port the server listens on, nil while there is no usable address.
    private(set) var address: String?
    private(set) var frame: StateFrame?
    private(set) var pollError: String?
    private(set) var install: InstallState = .idle
    private(set) var legacyAgentPresent = false
    private(set) var hasRokuPassword = false

    let server: ServerProcess
    @ObservationIgnored private let defaults = UserDefaults.standard
    @ObservationIgnored private let tokens = TokenStore.standard
    @ObservationIgnored private let monitor = NWPathMonitor()
    @ObservationIgnored private var poller: Task<Void, Never>?
    @ObservationIgnored private var watchdog: Task<Void, Never>?

    static let rokudevAccount = "rokudev"
    static let legacyAgent = FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent("Library/LaunchAgents/com.managents.serve.plist")
    static let logURL = FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent("Library/Logs/rokuagents-serve.log")

    private init() {
        defaults.register(defaults: ["port": 8787, "titles": true, "autoReinstall": true])
        port = defaults.integer(forKey: "port")
        pinnedInterface = defaults.string(forKey: "pinnedInterface")
        titles = defaults.bool(forKey: "titles")
        rokuHost = defaults.string(forKey: "rokuHost") ?? ""
        autoReinstall = defaults.bool(forKey: "autoReinstall")

        let binary = Bundle.main.url(forAuxiliaryExecutable: "managents")
            ?? URL(fileURLWithPath: "/usr/local/bin/managents")
        server = ServerProcess(binary: binary, logURL: Self.logURL)

        do {
            token = try tokens.loadOrCreate()
        } catch {
            tokenError = "Cannot read or create \(tokens.url.path): \(error.localizedDescription)"
        }
        checkLegacyAgent()
        hasRokuPassword = rokuPassword != nil

        monitor.pathUpdateHandler = { [weak self] _ in
            Task { @MainActor in
                self?.refreshNetwork()
                // Addresses can arrive a moment after the path changes.
                try? await Task.sleep(for: .seconds(3))
                self?.refreshNetwork()
            }
        }
        monitor.start(queue: .main)
        watchdog = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(30))
                self?.refreshNetwork()
            }
        }
        startPolling()
        refreshNetwork()
    }

    // MARK: Derived state

    var chosenInterface: NetworkInterface? { LAN.pick(from: interfaces, pinned: pinnedInterface) }
    var serverURL: String? { address.map { "http://\($0)" } }
    var rokuConfigured: Bool { !rokuHost.isEmpty && hasRokuPassword }
    var rokuPassword: String? { Keychain.password(for: Self.rokudevAccount) }
    var counts: Counts { frame?.counts ?? Counts() }

    var serverRunning: Bool {
        if case .running = server.state, pollError == nil { return true }
        return false
    }

    var statusLine: String {
        if legacyAgentPresent { return "The old LaunchAgent is running: disable it in Settings" }
        if tokenError != nil { return "No token: see Settings" }
        guard address != nil else {
            return pinnedInterface.map { "No address on \($0)" } ?? "No home network address"
        }
        switch server.state {
        case .stopped: return "Server stopped"
        case .failed(let reason): return "Server failed: \(reason)"
        case .running(let address):
            if let pollError { return "Server on \(address): \(pollError)" }
            return "Serving on \(address)"
        }
    }

    // MARK: Network and server

    func refreshNetwork() {
        interfaces = LAN.interfaces()
        applyAddress()
    }

    private func applyAddress(force: Bool = false) {
        let previous = address
        guard !legacyAgentPresent, tokenError == nil, let chosen = chosenInterface else {
            address = nil
            frame = nil
            server.stop()
            return
        }
        let next = "\(chosen.ipv4):\(port)"
        address = next
        server.start(.init(address: next, token: token, titles: titles), force: force)
        if let previous, previous != next, autoReinstall, rokuConfigured {
            Task {
                try? await Task.sleep(for: .seconds(2))
                await installOnRoku(reason: "your Mac moved to \(chosen.ipv4)")
            }
        }
    }

    func restartServer() { applyAddress(force: true) }

    private func startPolling() {
        poller = Task { [weak self] in
            let session = URLSession(configuration: .ephemeral)
            while !Task.isCancelled {
                await self?.poll(session)
                try? await Task.sleep(for: .seconds(2))
            }
        }
    }

    private func poll(_ session: URLSession) async {
        guard case .running = server.state, let serverURL, let url = URL(string: serverURL + "/state") else {
            pollError = nil
            return
        }
        var request = URLRequest(url: url, timeoutInterval: 3)
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        do {
            let (data, response) = try await session.data(for: request)
            switch (response as? HTTPURLResponse)?.statusCode {
            case 200:
                frame = try JSONDecoder().decode(StateFrame.self, from: data)
                pollError = nil
            case 503:
                pollError = nil // starting up
            case let code:
                pollError = "HTTP \(code ?? 0)"
            }
        } catch {
            pollError = "not answering yet"
        }
    }

    // MARK: Roku

    func installOnRoku(reason: String? = nil) async {
        guard let serverURL, let password = rokuPassword, !rokuHost.isEmpty else {
            install = .failed("Set the Roku's address and developer password first.")
            return
        }
        guard let channel = Bundle.main.url(forResource: "roku", withExtension: nil) else {
            install = .failed("The app has no copy of the channel. Rebuild it with macos/build.sh.")
            return
        }
        install = .installing
        do {
            let zip = try ChannelPackage.build(channelDir: channel, url: serverURL, token: token)
            let outcome = try await RokuInstaller(host: rokuHost, password: password).install(zip: zip)
            let suffix = reason.map { " because \($0)" } ?? ""
            switch outcome {
            case .installed: install = .done("Installed on \(rokuHost)\(suffix).")
            case .unchanged: install = .done("Already up to date on \(rokuHost); opened it.")
            case .failed(let message): install = .failed(message)
            }
        } catch {
            install = .failed(error.localizedDescription)
        }
    }

    // MARK: Settings

    func setPort(_ value: Int) {
        guard (1024...65535).contains(value), value != port else { return }
        port = value
        defaults.set(value, forKey: "port")
        applyAddress()
    }

    func setPinnedInterface(_ name: String?) {
        pinnedInterface = name
        defaults.set(name, forKey: "pinnedInterface")
        applyAddress()
    }

    func setTitles(_ value: Bool) {
        titles = value
        defaults.set(value, forKey: "titles")
        applyAddress()
    }

    func setRokuHost(_ value: String) {
        rokuHost = value.trimmingCharacters(in: .whitespaces)
        defaults.set(rokuHost, forKey: "rokuHost")
    }

    func setRokuPassword(_ value: String) {
        Keychain.setPassword(value, for: Self.rokudevAccount)
        hasRokuPassword = !value.isEmpty
    }

    func setAutoReinstall(_ value: Bool) {
        autoReinstall = value
        defaults.set(value, forKey: "autoReinstall")
    }

    func regenerateToken() {
        do {
            token = try tokens.regenerate()
            tokenError = nil
        } catch {
            tokenError = error.localizedDescription
            return
        }
        applyAddress(force: true)
        if rokuConfigured {
            Task { await installOnRoku(reason: "the token changed") }
        }
    }

    func copyToken() {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(token, forType: .string)
    }

    var launchAtLogin: Bool { SMAppService.mainApp.status == .enabled }

    func setLaunchAtLogin(_ enabled: Bool) {
        do {
            if enabled { try SMAppService.mainApp.register() } else { try SMAppService.mainApp.unregister() }
        } catch {
            install = .failed("Launch at login: \(error.localizedDescription)")
        }
    }

    // MARK: The LaunchAgent from the command-line setup

    func checkLegacyAgent() {
        legacyAgentPresent = FileManager.default.fileExists(atPath: Self.legacyAgent.path)
    }

    /// Unloads the README's LaunchAgent and renames its plist, so the app's
    /// server can have the port. Renaming keeps it easy to bring back.
    func disableLegacyAgent() {
        let bootout = Process()
        bootout.executableURL = URL(fileURLWithPath: "/bin/launchctl")
        bootout.arguments = ["bootout", "gui/\(getuid())/com.managents.serve"]
        try? bootout.run()
        bootout.waitUntilExit()
        let disabled = Self.legacyAgent.appendingPathExtension("disabled")
        try? FileManager.default.removeItem(at: disabled)
        try? FileManager.default.moveItem(at: Self.legacyAgent, to: disabled)
        checkLegacyAgent()
        applyAddress(force: true)
    }

    func shutdown() {
        monitor.cancel()
        poller?.cancel()
        watchdog?.cancel()
        server.shutdown()
    }
}
