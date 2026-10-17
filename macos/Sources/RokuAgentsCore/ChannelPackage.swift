import Foundation

/// Builds the channel zip that roku/build.sh builds: the channel files plus a
/// config.json with the server address and token baked in.
public enum ChannelPackage {
    /// What goes in the zip, at its root, besides config.json.
    public static let contents = ["manifest", "source", "components", "images"]

    public enum Failure: Error, LocalizedError {
        case missing(String)
        case zip(Int32)

        public var errorDescription: String? {
            switch self {
            case .missing(let name): "The channel is missing \(name)."
            case .zip(let status): "zip failed (exit \(status))."
            }
        }
    }

    /// config.json for the channel (roku/components/SessionsScene.brs reads it).
    public static func config(url: String, token: String) throws -> Data {
        try JSONSerialization.data(withJSONObject: ["url": url, "token": token], options: [.sortedKeys])
    }

    /// Zips the channel in channelDir with a config.json for url and token.
    public static func build(channelDir: URL, url: String, token: String) throws -> Data {
        let files = FileManager.default
        let staging = files.temporaryDirectory.appendingPathComponent("rokuagents-\(UUID().uuidString)")
        try files.createDirectory(at: staging, withIntermediateDirectories: true)
        defer { try? files.removeItem(at: staging) }

        for name in contents {
            let source = channelDir.appendingPathComponent(name)
            guard files.fileExists(atPath: source.path) else { throw Failure.missing(name) }
            try files.copyItem(at: source, to: staging.appendingPathComponent(name))
        }
        try config(url: url, token: token).write(to: staging.appendingPathComponent("config.json"))

        let zip = Process()
        zip.executableURL = URL(fileURLWithPath: "/usr/bin/zip")
        zip.currentDirectoryURL = staging
        zip.arguments = ["-q", "-r", "channel.zip", "config.json"] + contents
        try zip.run()
        zip.waitUntilExit()
        guard zip.terminationStatus == 0 else { throw Failure.zip(zip.terminationStatus) }
        return try Data(contentsOf: staging.appendingPathComponent("channel.zip"))
    }
}
