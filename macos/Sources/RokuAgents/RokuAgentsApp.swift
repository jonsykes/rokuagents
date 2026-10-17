import AppKit
import SwiftUI

@main
struct RokuAgentsApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @State private var model = AppModel.shared

    var body: some Scene {
        MenuBarExtra {
            MenuContent(model: model)
        } label: {
            MenuLabel(model: model)
        }
        .menuBarExtraStyle(.menu)

        Window("rokuagents Settings", id: "settings") {
            SettingsView(model: model)
        }
        .windowResizability(.contentSize)
        .defaultLaunchBehavior(.suppressed)
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationWillTerminate(_ notification: Notification) {
        MainActor.assumeIsolated { AppModel.shared.shutdown() }
    }
}

/// The menu bar item: a TV, crossed out while the server is down, with the
/// number of sessions that need you.
struct MenuLabel: View {
    let model: AppModel

    var body: some View {
        let waiting = model.counts.waiting + model.counts.error
        HStack(spacing: 3) {
            Image(systemName: model.serverRunning ? "tv" : "tv.slash")
            if model.serverRunning, waiting > 0 {
                Text("\(waiting)")
            }
        }
    }
}
