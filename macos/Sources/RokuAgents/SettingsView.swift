import RokuAgentsCore
import SwiftUI

struct SettingsView: View {
    let model: AppModel
    @State private var portText = ""
    @State private var rokuHost = ""
    @State private var password = ""
    @State private var showToken = false

    var body: some View {
        Form {
            if model.legacyAgentPresent {
                Section {
                    Text("The LaunchAgent from the command-line setup (com.managents.serve) is installed. It runs its "
                        + "own server on the same port, so the app waits until it is disabled.")
                    Button("Disable the LaunchAgent") { model.disableLegacyAgent() }
                } header: {
                    Text("Command-line setup found")
                }
            }

            Section("Server") {
                LabeledContent("Status", value: model.statusLine)
                Picker("Network", selection: Binding(
                    get: { model.pinnedInterface ?? "" },
                    set: { model.setPinnedInterface($0.isEmpty ? nil : $0) }
                )) {
                    Text(automaticLabel).tag("")
                    ForEach(model.interfaces.filter { LAN.isPrivate($0.ipv4) }, id: \.name) { interface in
                        Text("\(interface.name): \(interface.ipv4)").tag(interface.name)
                    }
                    if let pinned = model.pinnedInterface,
                       !model.interfaces.contains(where: { $0.name == pinned }) {
                        Text("\(pinned) (not connected)").tag(pinned)
                    }
                }
                TextField("Port", text: $portText)
                    .onSubmit { model.setPort(Int(portText) ?? model.port) }
                Toggle("Send session titles", isOn: Binding(get: { model.titles }, set: { model.setTitles($0) }))
                Text("Titles are the ones the Claude app shows. Off, the TV shows folder names only.")
                    .font(.caption).foregroundStyle(.secondary)
                LabeledContent("Token") {
                    HStack {
                        Text(showToken ? model.token : String(repeating: "•", count: 12))
                            .font(.system(.body, design: .monospaced))
                            .textSelection(.enabled)
                        Button(showToken ? "Hide" : "Show") { showToken.toggle() }
                        Button("Copy") { model.copyToken() }
                        Button("New Token") { model.regenerateToken() }
                    }
                }
                if let error = model.tokenError {
                    Text(error).foregroundStyle(.red)
                }
            }

            Section("Roku") {
                TextField("Roku address", text: $rokuHost, prompt: Text("192.168.1.50"))
                    .onSubmit { model.setRokuHost(rokuHost) }
                SecureField("Developer password", text: $password,
                            prompt: Text(model.hasRokuPassword ? "Saved in the keychain" : "Required"))
                    .onSubmit(savePassword)
                Toggle("Reinstall when this Mac's address changes", isOn: Binding(
                    get: { model.autoReinstall }, set: { model.setAutoReinstall($0) }))
                HStack {
                    Button(model.install == .installing ? "Installing…" : "Install on Roku") {
                        model.setRokuHost(rokuHost)
                        savePassword()
                        Task { await model.installOnRoku() }
                    }
                    .disabled(model.install == .installing || rokuHost.isEmpty
                        || (password.isEmpty && !model.hasRokuPassword) || model.address == nil)
                    installMessage
                }
                Text("Developer mode: on the remote press Home 3 times, Up 2 times, then Right, Left, Right, Left, "
                    + "Right. The Roku shows its address and asks you to set the password.")
                    .font(.caption).foregroundStyle(.secondary)
            }

            Section("General") {
                Toggle("Open at login", isOn: Binding(
                    get: { model.launchAtLogin }, set: { model.setLaunchAtLogin($0) }))
                LabeledContent("Log") {
                    Button("Open") { NSWorkspace.shared.open(AppModel.logURL) }
                }
            }
        }
        .formStyle(.grouped)
        .frame(width: 520)
        .fixedSize(horizontal: false, vertical: true)
        .onAppear {
            portText = String(model.port)
            rokuHost = model.rokuHost
            model.checkLegacyAgent()
            model.refreshNetwork()
        }
    }

    private var automaticLabel: String {
        if model.pinnedInterface == nil, let chosen = model.chosenInterface {
            return "Automatic (\(chosen.name): \(chosen.ipv4))"
        }
        return "Automatic"
    }

    @ViewBuilder private var installMessage: some View {
        switch model.install {
        case .idle, .installing: EmptyView()
        case .done(let message): Text(message).foregroundStyle(.secondary)
        case .failed(let message): Text(message).foregroundStyle(.red)
        }
    }

    private func savePassword() {
        guard !password.isEmpty else { return }
        model.setRokuPassword(password)
        password = ""
    }
}
