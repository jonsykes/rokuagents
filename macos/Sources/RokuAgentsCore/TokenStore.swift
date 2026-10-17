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
        let dir = url.deletingLastPathComponent()
        try files.createDirectory(at: dir, withIntermediateDirectories: true)
        try files.setAttributes([.posixPermissions: 0o700], ofItemAtPath: dir.path)

        // Written to a file created as 600 from the start, then renamed over
        // the old one, so the token is never readable by others.
        let temporary = dir.appendingPathComponent(".token-\(UUID().uuidString)")
        let fd = open(temporary.path, O_CREAT | O_EXCL | O_WRONLY, 0o600)
        guard fd >= 0 else { throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO) }
        let handle = FileHandle(fileDescriptor: fd, closeOnDealloc: true)
        do {
            try handle.write(contentsOf: Data((token + "\n").utf8))
            try handle.close()
            guard rename(temporary.path, url.path) == 0 else {
                throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
            }
        } catch {
            try? files.removeItem(at: temporary)
            throw error
        }
        return token
    }
}
