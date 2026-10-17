/// The body of `GET /state` (docs/http-api.md). Unknown fields are ignored.
public struct StateFrame: Decodable, Equatable, Sendable {
    public let v: Int
    public let agents: [Agent]
    public let more: Int?

    public struct Agent: Decodable, Equatable, Identifiable, Sendable {
        public let id: String
        public let name: String
        public let status: String
        public let age: Int
        public let title: String?

        /// What a card shows as its headline: the title, or the folder name.
        public var headline: String {
            if let title, !title.isEmpty { return title }
            return name
        }
    }

    /// How many sessions are in each status.
    public var counts: Counts {
        var counts = Counts()
        for agent in agents {
            switch agent.status {
            case "working": counts.working += 1
            case "waiting": counts.waiting += 1
            case "error": counts.error += 1
            default: counts.idle += 1
            }
        }
        return counts
    }
}

public struct Counts: Equatable, Sendable {
    public var working = 0
    public var waiting = 0
    public var error = 0
    public var idle = 0

    public init() {}

    /// "2 working, 1 needs you", as in the channel's header. Empty when
    /// nothing is open.
    public var summary: String {
        var parts: [String] = []
        if working > 0 { parts.append("\(working) working") }
        if waiting > 0 { parts.append("\(waiting) \(waiting == 1 ? "needs" : "need") you") }
        if error > 0 { parts.append("\(error) failed") }
        if idle > 0 { parts.append("\(idle) idle") }
        return parts.joined(separator: ", ")
    }
}
