# Claude Sessions for Roku

Your open Claude Code sessions as cards on the TV: green while a session works, yellow when it needs you, red when it
failed, dimmed when it has been idle for hours. Same data and colors as the managents desk display, pulled over your
home network instead of USB.

Two parts: `managents serve` on your computer publishes session status (see [docs/http-api.md](../docs/http-api.md)),
and this channel on the Roku polls it once a second.

## 1. Run the helper on your computer

Build the fork (Go 1.26):

```sh
cd helper && go build -o ~/bin/managents ./cmd/managents
```

Pick a token and start serving. Use your computer's LAN address so it only listens there:

```sh
export MANAGENTS_TOKEN=$(openssl rand -hex 16)
echo "$MANAGENTS_TOKEN"          # keep it for step 3
managents serve --addr 192.168.1.20:8787
```

Check it from another terminal:

```sh
curl -H "Authorization: Bearer $MANAGENTS_TOKEN" http://192.168.1.20:8787/state
```

Give your computer a reserved DHCP address in your router, or the channel will lose it when the address changes.
On macOS the firewall may ask once whether `managents` may accept incoming connections: allow it.

### Start it at login (macOS)

Save as `~/Library/LaunchAgents/com.managents.serve.plist`, fill in the path, address and token, then run
`launchctl load ~/Library/LaunchAgents/com.managents.serve.plist`.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.managents.serve</string>
  <key>ProgramArguments</key>
  <array>
    <string>/Users/YOU/bin/managents</string>
    <string>serve</string>
    <string>--addr</string><string>192.168.1.20:8787</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict><key>MANAGENTS_TOKEN</key><string>YOUR_TOKEN</string></dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardErrorPath</key><string>/tmp/managents-serve.log</string>
</dict>
</plist>
```

## 2. Put the Roku in developer mode (once)

On the Roku remote press **Home ×3, Up ×2, Right, Left, Right, Left, Right**. Enable the installer, accept the
agreement, and set a password. The Roku restarts and shows its IP address.

## 3. Build and install the channel

```sh
cd roku
ROKU_IP=192.168.1.50 ROKU_PASS=your-dev-password ./build.sh http://192.168.1.20:8787 "$MANAGENTS_TOKEN"
```

Without `ROKU_IP` and `ROKU_PASS` it only builds `dist/claude-sessions.zip`, which you can upload yourself at
`http://ROKU_IP` in a browser (user `rokudev`).

The address and token are baked into the package, so rerun this whenever either changes.

## Using it

- Nine sessions per page. Left/right or up/down turn pages; after 30 seconds it returns to the first page.
- The header shows counts, your computer's clock, and `+N more` beyond 24 sessions.
- The bar along the bottom of a card is context-window usage, shown only when the limit is known.
- "Reconnecting..." after 6 seconds without an answer, with the reason when there is one.
- **Screensaver:** Settings > Theme > Screensavers > Claude Sessions. Pages turn every 10 seconds and the layout drifts
  a few pixels each minute so nothing burns in.

A sideloaded channel stays installed until you sideload another one or turn developer mode off.

## Files

| Path | What it does |
|---|---|
| `source/main.brs` | Entry points for the channel and the screensaver |
| `components/PollTask.*` | Background poll of `/state`; publishes frames and failure reasons |
| `components/SessionsScene.*` | Layout, paging, clock, connection messages, screensaver behavior |
| `components/SessionCard.*` | One card: color, name, status and age, context bar, error blink |
| `components/format.brs` | Status styles, age and clock formatting |
| `build.sh` | Writes `config.json`, zips, optionally sideloads |
