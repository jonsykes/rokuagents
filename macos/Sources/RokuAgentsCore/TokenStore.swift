import Foundation

/// The bearer token the server requires and the channel sends. It lives in
/// ~/.config/managents/token (mode 600), the same file the README's
/// command-line setup uses, so curl and roku/build.sh keep working.
public struct TokenStore: Sendable {
    public let url: URL

    public init(url: URL) { self.url = url }

    public static var standard: TokenStore {
        TokenStore(url: FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent(".config/managents/token"))
    }

    /// The saved token, or a new one saved first.
    public func loadOrCreate() throws -> String {
        if let data = try? Data(contentsOf: url) {
            let token = String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
            if !token.isEmpty { return token }
        }
        return try regenerate()
    }

    /// Replaces the token with a new random one (16 bytes, hex) and returns it.
    @discardableResult
    public func regenerate() throws -> String {
        var generator = SystemRandomNumberGenerator()
        let token = (0..<16).map { _ in String(format: "%02x", UInt8.random(in: 0...255, using: &generator)) }
            .joined()
        let files = FileManager.default
        try files.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true,
                                  attributes: [.posixPermissions: 0o700])
        try? files.removeItem(at: url)
        guard files.createFile(atPath: url.path, contents: Data((token + "\n").utf8),
                               attributes: [.posixPermissions: 0o600])
        else { throw CocoaError(.fileWriteUnknown, userInfo: [NSFilePathErrorKey: url.path]) }
        return token
    }
}
