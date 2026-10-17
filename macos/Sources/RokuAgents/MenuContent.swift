import AppKit
import RokuAgentsCore
import SwiftUI

struct MenuContent: View {
    let model: AppModel
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        Text(model.statusLine)
        if let frame = model.frame, model.serverRunning {
            let summary = frame.counts.summary
            Text(summary.isEmpty ? "No open Claude Code sessions" : summary)
            if !frame.agents.isEmpty {
                Divider()
                ForEach(frame.agents.prefix(9)) { agent in
                    Text("\(Self.dot(agent.status))  \(agent.headline)")
                }
                if frame.agents.count > 9 {
                    Text("and \(frame.agents.count - 9) more")
                }
            }
        }

        Divider()
        if model.rokuConfigured {
            Button(installLabel) { Task { await model.installOnRoku() } }
                .disabled(model.install == .installing || model.address == nil)
        } else {
            Button("Set Up Roku…") { showSettings() }
        }
        if case .failed(let message) = model.install {
            Text(message)
        }
        Button("Restart Server") { model.restartServer() }
            .disabled(model.address == nil)
        Button("Copy Token") { model.copyToken() }
        Button("Open Log") { NSWorkspace.shared.open(AppModel.logURL) }

        Divider()
        Button("Settings…") { showSettings() }
            .keyboardShortcut(",")
        Button("Quit rokuagents") { NSApp.terminate(nil) }
            .keyboardShortcut("q")
    }

    private var installLabel: String {
        model.install == .installing ? "Installing on Roku…" : "Install on Roku (\(model.rokuHost))"
    }

    private func showSettings() {
        openWindow(id: "settings")
        NSApp.activate()
    }

    static func dot(_ status: String) -> String {
        switch status {
        case "working": "🟢"
        case "waiting": "🟡"
        case "error": "🔴"
        default: "⚪️"
        }
    }
}
