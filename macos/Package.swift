// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "RokuAgents",
    platforms: [.macOS(.v15)],
    targets: [
        // Logic with no UI, so it can be unit tested.
        .target(name: "RokuAgentsCore"),
        // The menu bar app. build.sh wraps it in rokuagents.app.
        .executableTarget(name: "RokuAgents", dependencies: ["RokuAgentsCore"]),
        .testTarget(name: "RokuAgentsCoreTests", dependencies: ["RokuAgentsCore"]),
    ],
    swiftLanguageModes: [.v5]
)
