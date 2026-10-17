import Foundation

/// What the Roku's developer installer said about an upload.
public enum InstallOutcome: Equatable, Sendable {
    case installed
    case unchanged // the same zip was already installed
    case failed(String)

    /// Reads the installer's HTML answer, which reports "Install Success.",
    /// "Identical to previous version -- not replacing." or "Install Failure: ...".
    public static func parse(_ html: String) -> InstallOutcome {
        if let range = html.range(of: "Install Failure") {
            let rest = html[range.lowerBound...]
            let end = rest.firstIndex { "\"<\n".contains($0) } ?? rest.endIndex
            return .failed(String(rest[..<end]).trimmingCharacters(in: .whitespaces))
        }
        if html.contains("Identical to previous version") { return .unchanged }
        if html.contains("Install Success") { return .installed }
        return .failed("The Roku gave an unexpected answer.")
    }
}

/// Uploads a channel zip to a Roku in developer mode, as roku/build.sh does
/// with curl: a form POST to /plugin_install with digest authentication as
/// user rokudev.
public struct RokuInstaller: Sendable {
    public let host: String
    public let password: String

    public init(host: String, password: String) {
        self.host = host
        self.password = password
    }

    public enum Failure: Error, LocalizedError {
        case badHost
        case wrongPassword
        case notARoku
        case http(Int)

        public var errorDescription: String? {
            switch self {
            case .badHost: "The Roku address is not valid."
            case .wrongPassword: "The Roku rejected the developer password."
            case .notARoku: "The device at that address did not answer as a Roku."
            case .http(let code): "The Roku answered HTTP \(code)."
            }
        }
    }

    public func install(zip: Data) async throws -> InstallOutcome {
        guard let url = URL(string: "http://\(host)/plugin_install") else { throw Failure.badHost }
        let form = MultipartForm()
        var request = URLRequest(url: url, timeoutInterval: 60)
        request.httpMethod = "POST"
        request.setValue(form.contentType, forHTTPHeaderField: "Content-Type")
        let body = form.body(
            fields: [("mysubmit", "Install")],
            file: .init(field: "archive", filename: "rokuagents.zip", contentType: "application/zip", data: zip))

        let (data, response) = try await upload(request, body: body)
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        if status == 401 { throw Failure.wrongPassword }
        guard status == 200 else { throw Failure.http(status) }
        let outcome = InstallOutcome.parse(String(decoding: data, as: UTF8.self))
        if outcome == .unchanged {
            try? await launch()
        }
        return outcome
    }

    /// The installer's web server sometimes drops the connection, after its
    /// digest challenge or while the previous install is still restarting the
    /// channel, so a lost connection is retried up to three times.
    private func upload(_ request: URLRequest, body: Data) async throws -> (Data, URLResponse) {
        for attempt in 1... {
            let session = URLSession(configuration: .ephemeral)
            defer { session.finishTasksAndInvalidate() }
            do {
                return try await session.upload(for: request, from: body,
                                                delegate: DigestCredentials(password: password))
            } catch let error as URLError where error.code == .networkConnectionLost && attempt < 4 {
                try await Task.sleep(for: .seconds(2))
            }
        }
        fatalError("unreachable")
    }

    /// The Roku's serial number, from ECP's device-info (no password needed).
    /// The app checks it before reinstalling by itself, so a different device
    /// at the same address on another network never gets the channel and token.
    public func serialNumber() async throws -> String {
        guard let url = URL(string: "http://\(host):8060/query/device-info") else { throw Failure.badHost }
        let (data, _) = try await URLSession.shared.data(for: URLRequest(url: url, timeoutInterval: 5))
        guard let serial = Self.serialNumber(inDeviceInfo: String(decoding: data, as: UTF8.self)) else {
            throw Failure.notARoku
        }
        return serial
    }

    static func serialNumber(inDeviceInfo xml: String) -> String? {
        guard let start = xml.range(of: "<serial-number>"),
              let end = xml.range(of: "</serial-number>", range: start.upperBound..<xml.endIndex)
        else { return nil }
        let serial = xml[start.upperBound..<end.lowerBound].trimmingCharacters(in: .whitespaces)
        return serial.isEmpty ? nil : serial
    }

    /// Opens the sideloaded channel, which the Roku only does by itself after
    /// a changed install.
    public func launch() async throws {
        guard let url = URL(string: "http://\(host):8060/launch/dev") else { throw Failure.badHost }
        var request = URLRequest(url: url, timeoutInterval: 10)
        request.httpMethod = "POST"
        _ = try await URLSession.shared.data(for: request)
    }
}

/// Answers the installer's digest challenge once; a second challenge means the
/// password is wrong, and the request then fails with 401. Only Digest is
/// answered: Basic would send the password in clear text over plain HTTP.
private final class DigestCredentials: NSObject, URLSessionTaskDelegate, @unchecked Sendable {
    let password: String

    init(password: String) { self.password = password }

    func urlSession(_ session: URLSession, task: URLSessionTask, didReceive challenge: URLAuthenticationChallenge)
        async -> (URLSession.AuthChallengeDisposition, URLCredential?)
    {
        guard challenge.protectionSpace.authenticationMethod == NSURLAuthenticationMethodHTTPDigest else {
            return (.cancelAuthenticationChallenge, nil)
        }
        guard challenge.previousFailureCount == 0 else { return (.rejectProtectionSpace, nil) }
        return (.useCredential, URLCredential(user: "rokudev", password: password, persistence: .none))
    }
}
