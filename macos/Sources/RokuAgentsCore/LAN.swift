import Darwin

/// A network interface with an IPv4 address.
public struct NetworkInterface: Equatable, Hashable, Sendable {
    public let name: String
    public let ipv4: String

    public init(name: String, ipv4: String) {
        self.name = name
        self.ipv4 = ipv4
    }
}

/// Finds the address the server should listen on: the Mac's address on the
/// home network, which is also the one the Roku channel is built with.
public enum LAN {
    /// The interfaces that are up and have an IPv4 address, loopback excluded.
    public static func interfaces() -> [NetworkInterface] {
        var list: UnsafeMutablePointer<ifaddrs>?
        guard getifaddrs(&list) == 0, let first = list else { return [] }
        defer { freeifaddrs(list) }

        var result: [NetworkInterface] = []
        for entry in sequence(first: first, next: { $0.pointee.ifa_next }) {
            let flags = Int32(entry.pointee.ifa_flags)
            guard let addr = entry.pointee.ifa_addr,
                  addr.pointee.sa_family == UInt8(AF_INET),
                  flags & IFF_UP != 0, flags & IFF_LOOPBACK == 0
            else { continue }
            var host = [CChar](repeating: 0, count: Int(NI_MAXHOST))
            guard getnameinfo(addr, socklen_t(addr.pointee.sa_len), &host, socklen_t(host.count),
                              nil, 0, NI_NUMERICHOST) == 0
            else { continue }
            result.append(NetworkInterface(name: cString(entry.pointee.ifa_name), ipv4: cString(host)))
        }
        return result
    }

    /// Whether ip is in a private (home or office network) IPv4 range.
    public static func isPrivate(_ ip: String) -> Bool {
        let octets = ip.split(separator: ".").compactMap { Int($0) }
        guard octets.count == 4, octets.allSatisfy({ (0...255).contains($0) }) else { return false }
        switch (octets[0], octets[1]) {
        case (10, _), (192, 168): return true
        case (172, 16...31): return true
        default: return false
        }
    }

    /// The interface to listen on. A pinned interface is used only while it
    /// has a private address. Automatic picks the lowest-numbered Wi-Fi or
    /// Ethernet interface (en0 before en1), never a VPN or virtual machine
    /// bridge, whose addresses the Roku cannot reach.
    public static func pick(from interfaces: [NetworkInterface], pinned: String?) -> NetworkInterface? {
        let usable = interfaces.filter { isPrivate($0.ipv4) }
        if let pinned {
            return usable.first { $0.name == pinned }
        }
        return usable
            .filter { $0.name.hasPrefix("en") && Int($0.name.dropFirst(2)) != nil }
            .min { Int($0.name.dropFirst(2))! < Int($1.name.dropFirst(2))! }
    }

    private static func cString(_ pointer: UnsafePointer<CChar>) -> String {
        String(cString: pointer)
    }

    private static func cString(_ chars: [CChar]) -> String {
        String(decoding: chars.prefix { $0 != 0 }.map { UInt8(bitPattern: $0) }, as: UTF8.self)
    }
}
