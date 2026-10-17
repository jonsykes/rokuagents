import Foundation
import XCTest

@testable import RokuAgentsCore

final class LANTests: XCTestCase {
    func testPrivateRanges() {
        for ip in ["10.0.0.5", "172.16.1.1", "172.31.255.255", "192.168.1.20"] {
            XCTAssertTrue(LAN.isPrivate(ip), ip)
        }
        for ip in ["8.8.8.8", "172.32.0.1", "169.254.1.1", "192.169.0.1", "not.an.ip", "1.2.3"] {
            XCTAssertFalse(LAN.isPrivate(ip), ip)
        }
    }

    func testAutomaticPrefersLowestEthernetOrWiFi() {
        let interfaces = [
            NetworkInterface(name: "utun4", ipv4: "10.8.0.2"),
            NetworkInterface(name: "bridge100", ipv4: "192.168.64.1"),
            NetworkInterface(name: "en1", ipv4: "192.168.1.20"),
            NetworkInterface(name: "en10", ipv4: "192.168.1.30"),
        ]
        XCTAssertEqual(LAN.pick(from: interfaces, pinned: nil)?.name, "en1")
        XCTAssertEqual(LAN.pick(from: interfaces + [.init(name: "en0", ipv4: "192.168.1.10")], pinned: nil)?.name,
                       "en0")
    }

    func testAutomaticIgnoresVPNsAndPublicAddresses() {
        let interfaces = [
            NetworkInterface(name: "utun4", ipv4: "10.8.0.2"),
            NetworkInterface(name: "en0", ipv4: "203.0.113.7"),
        ]
        XCTAssertNil(LAN.pick(from: interfaces, pinned: nil))
    }

    func testPinnedInterface() {
        let interfaces = [
            NetworkInterface(name: "en0", ipv4: "192.168.1.10"),
            NetworkInterface(name: "en1", ipv4: "192.168.1.20"),
        ]
        XCTAssertEqual(LAN.pick(from: interfaces, pinned: "en1")?.ipv4, "192.168.1.20")
        XCTAssertNil(LAN.pick(from: interfaces, pinned: "en5"), "a pinned interface that is down gives no address")
    }

    func testListsThisMachinesInterfacesWithoutLoopback() {
        XCTAssertFalse(LAN.interfaces().contains { $0.ipv4 == "127.0.0.1" })
    }
}

final class StateFrameTests: XCTestCase {
    let json = """
        {"v":1,"t":"state","now":1792247293,"tz":-14400,"future":true,"agents":[
          {"id":"claude:1","kind":"claude","name":"payments-api","status":"working","age":12,
           "title":"Retry failed webhooks","ctx":{"used":1,"limit":2}},
          {"id":"claude:2","kind":"claude","name":"website","status":"waiting","age":90},
          {"id":"claude:3","kind":"claude","name":"docs","status":"waiting","age":5,"title":""},
          {"id":"claude:4","kind":"claude","name":"infra-terraform","status":"error","age":30},
          {"id":"claude:5","kind":"claude","name":"search","status":"idle","age":9000}]}
        """

    func testDecodesAndCounts() throws {
        let frame = try JSONDecoder().decode(StateFrame.self, from: Data(json.utf8))
        XCTAssertEqual(frame.agents.count, 5)
        XCTAssertEqual(frame.agents[0].headline, "Retry failed webhooks")
        XCTAssertEqual(frame.agents[1].headline, "website")
        XCTAssertEqual(frame.agents[2].headline, "docs", "an empty title falls back to the folder")
        XCTAssertEqual(frame.counts.summary, "1 working, 2 need you, 1 failed, 1 idle")
    }

    func testSummaryWording() {
        var counts = Counts()
        XCTAssertEqual(counts.summary, "")
        counts.waiting = 1
        XCTAssertEqual(counts.summary, "1 needs you")
    }
}

final class MultipartTests: XCTestCase {
    func testBody() {
        let form = MultipartForm(boundary: "B")
        let body = form.body(
            fields: [("mysubmit", "Install")],
            file: .init(field: "archive", filename: "c.zip", contentType: "application/zip", data: Data("ZIP".utf8)))
        let expected = "--B\r\nContent-Disposition: form-data; name=\"mysubmit\"\r\n\r\nInstall\r\n"
            + "--B\r\nContent-Disposition: form-data; name=\"archive\"; filename=\"c.zip\"\r\n"
            + "Content-Type: application/zip\r\n\r\nZIP\r\n--B--\r\n"
        XCTAssertEqual(String(decoding: body, as: UTF8.self), expected)
        XCTAssertEqual(form.contentType, "multipart/form-data; boundary=B")
    }
}

final class InstallOutcomeTests: XCTestCase {
    func testParse() {
        XCTAssertEqual(InstallOutcome.parse(#"{"text":"Install Success.","type":"success"}"#), .installed)
        XCTAssertEqual(InstallOutcome.parse("<font>Application Received: Identical to previous version -- not "
                + "replacing.</font>"), .unchanged)
        XCTAssertEqual(InstallOutcome.parse(#"{"text":"Install Failure: Compilation Failed.","type":"error"}"#),
                       .failed("Install Failure: Compilation Failed."))
        XCTAssertEqual(InstallOutcome.parse("<html></html>"), .failed("The Roku gave an unexpected answer."))
    }
}

final class ChannelPackageTests: XCTestCase {
    func testConfigEscapesValues() throws {
        let data = try ChannelPackage.config(url: "http://192.168.1.20:8787", token: #"a"b"#)
        let decoded = try JSONSerialization.jsonObject(with: data) as? [String: String]
        XCTAssertEqual(decoded, ["url": "http://192.168.1.20:8787", "token": #"a"b"#])
    }

    func testBuildZipsTheChannelWithItsConfig() throws {
        let channel = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: channel) }
        for dir in ["source", "components", "images"] {
            try FileManager.default.createDirectory(at: channel.appendingPathComponent(dir),
                                                    withIntermediateDirectories: true)
            try Data("x".utf8).write(to: channel.appendingPathComponent("\(dir)/file"))
        }
        try Data("title=Test".utf8).write(to: channel.appendingPathComponent("manifest"))

        let zip = try ChannelPackage.build(channelDir: channel, url: "http://10.0.0.2:8787", token: "t")
        XCTAssertEqual(zip.prefix(2), Data("PK".utf8))
        let listing = try unzipListing(zip)
        for entry in ["manifest", "config.json", "source/file", "components/file", "images/file"] {
            XCTAssertTrue(listing.contains(entry), "\(entry) missing from \(listing)")
        }
    }

    func testBuildFailsWithoutAManifest() {
        let empty = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        XCTAssertThrowsError(try ChannelPackage.build(channelDir: empty, url: "http://x", token: "t"))
    }

    private func unzipListing(_ zip: Data) throws -> String {
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("\(UUID().uuidString).zip")
        defer { try? FileManager.default.removeItem(at: file) }
        try zip.write(to: file)
        let unzip = Process()
        let output = Pipe()
        unzip.executableURL = URL(fileURLWithPath: "/usr/bin/unzip")
        unzip.arguments = ["-Z1", file.path]
        unzip.standardOutput = output
        try unzip.run()
        unzip.waitUntilExit()
        return String(decoding: output.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
    }
}

final class TokenStoreTests: XCTestCase {
    func testCreatesAPrivateTokenOnceAndRegenerates() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: dir) }
        let store = TokenStore(url: dir.appendingPathComponent("managents/token"))

        let first = try store.loadOrCreate()
        XCTAssertEqual(first.count, 32)
        XCTAssertEqual(try store.loadOrCreate(), first, "an existing token is kept")
        let mode = try FileManager.default.attributesOfItem(atPath: store.url.path)[.posixPermissions] as? Int
        XCTAssertEqual(mode, 0o600)

        let second = try store.regenerate()
        XCTAssertNotEqual(second, first)
        XCTAssertEqual(try store.loadOrCreate(), second)
    }
}

/// Installs the real channel on a real Roku. Runs only when asked:
/// ROKU_IT_HOST=<roku ip> ROKU_IT_PASSWORD=<dev password> ROKU_IT_SERVER=http://<mac ip>:8787 \
/// ROKU_IT_TOKEN=<token> swift test --filter RokuIntegrationTests
final class RokuIntegrationTests: XCTestCase {
    func testInstallsTheChannel() async throws {
        let env = ProcessInfo.processInfo.environment
        guard let host = env["ROKU_IT_HOST"], let password = env["ROKU_IT_PASSWORD"],
              let server = env["ROKU_IT_SERVER"], let token = env["ROKU_IT_TOKEN"]
        else { throw XCTSkip("set ROKU_IT_HOST, ROKU_IT_PASSWORD, ROKU_IT_SERVER and ROKU_IT_TOKEN to run") }
        let channel = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent().appendingPathComponent("../../../roku").standardized
        let zip = try ChannelPackage.build(channelDir: channel, url: server, token: token)

        let installer = RokuInstaller(host: host, password: password)
        let serial = try await installer.serialNumber()
        XCTAssertFalse(serial.isEmpty)
        let outcome = try await installer.install(zip: zip)
        XCTAssertTrue([.installed, .unchanged].contains(outcome), "\(outcome)")

        do {
            _ = try await RokuInstaller(host: host, password: password + "-wrong").install(zip: zip)
            XCTFail("a wrong password must fail")
        } catch RokuInstaller.Failure.wrongPassword {
        }
    }
}

final class DeviceInfoTests: XCTestCase {
    func testSerialNumber() {
        let xml = "<device-info>\n\t<udn>x</udn>\n\t<serial-number>X0123456789A</serial-number>\n</device-info>"
        XCTAssertEqual(RokuInstaller.serialNumber(inDeviceInfo: xml), "X0123456789A")
        XCTAssertNil(RokuInstaller.serialNumber(inDeviceInfo: "<html>router login</html>"))
        XCTAssertNil(RokuInstaller.serialNumber(inDeviceInfo: "<serial-number> </serial-number>"))
    }
}
