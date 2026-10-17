# managents helper

The program that runs on your computer: it finds open AI agent sessions and streams their status to every connected
managents display. One static binary, no runtime, no configuration. It also installs itself as a background service
and flashes the display's firmware, which is embedded in it.

## Install

Most people use the one-liner from the [README](../README.md#get-started). From a clone:

```bash
make install     # builds bin/managents, copies it to ~/.local/bin and runs `managents service install`
```

Or build only: `make helper` writes `bin/managents` (`go build ./cmd/managents` in this directory works too, but
then the binary has no firmware: `managents flash` needs `--image`).

## Commands

Bare `managents` prints the help. Every command takes `-h`.

| Command | What it does |
|---|---|
| `managents run [--port P] [--log-level L] [--log-file F] [--claude-context N]` | Stream to displays until Ctrl-C. This is what the service runs. Without `--port`, USB serial ports are scanned every 2 s and each candidate is identified with a `hello` handshake. `--log-file` appends the log to a file (past 1 MiB it moves to `F.1` at the next start). |
| `managents status [--json] [--claude-context N]` | Print the detected sessions, or the exact frame that would be sent. Needs no display. The table has a FOLDER column with each session's working directory (`--json` has no paths). |
| `managents devices` | List the USB serial ports and say which are managents displays, with board and firmware version; a port in use by another program (the service) or a board that is not a display (new board? `managents flash`) says so. |
| `managents service install\|uninstall\|start\|stop\|status` | The background service, see [below](#the-background-service). |
| `managents flash [--port P] [--image F] [--baud N]` | Write the embedded firmware to a board over USB, about 13 s at 460800 baud, checked against the chip's MD5. Stops the service for the transfer and starts it again. See [ADR-0012](../docs/adr/0012-built-in-flasher.md). |
| `managents demo [--port P] [--hold 6s]` | Cycle through demo screens (every layout, status and edge case, with made-up project names) on the connected displays. |
| `managents version` | Print the version. |

`run`, `demo` and `devices` need the serial port, which the service holds: stop it first
(`managents service stop`, `managents service start` afterwards), or `devices` just reports the port as in use.

## What is detected

| Agent | Source | Status |
|---|---|---|
| Claude Code (terminal and desktop app) | `<config dir>/sessions/<pid>.json`, the live session registry. The config dir is `~/.claude`, or `$CLAUDE_CONFIG_DIR` when set. Only files named `<digits>.json` are read. A record counts only if its pid is running **and** started when the record says (stale files from crashed sessions are ignored). | `busy` → working; a pending permission prompt or question → waiting, and it stays waiting; idle at the prompt → waiting, or error when the last main-thread message is an API error, and either turns idle after 2 h. Context size comes from the last main-thread model call that reports usage (API-error entries have none and are skipped); the limit is 200k, or 1M once a session goes past 200k (force it with `--claude-context`). |
| OpenCode | Running `opencode` processes (one card per working directory, owned by the oldest process there; the background `opencode serve` is not a session) and `$XDG_DATA_HOME/opencode/opencode.db`, default `~/.local/share/opencode/opencode.db`, opened read-only: `session_message` of OpenCode 2, or `message` and `part` of OpenCode 1. | Newest step of the folder unfinished (or finished with tool calls, so the turn goes on), or a prompt sent < 90 s ago → working (its age counts from the prompt; a reply written before the process started is not working); the turn's `idle` row ends it; assistant error or a failed turn → error (an interruption is not an error); a tool frozen at `running` for > 6 s → waiting; error and waiting turn idle after 2 h. OpenCode does not store pending permissions, so the 6 s rule cannot tell a long silent command from a permission prompt. A store error is logged and the sessions are still shown. |
| Codex CLI | Running `codex` processes (one card per working directory, owned by the oldest process there; `codex exec`, the background `codex app-server` and the other service subcommands are not sessions). The folder's thread is the newest one in `$CODEX_HOME/state_<N>.sqlite`, default `~/.codex`, opened read-only (not archived, not a subagent's, not run by `codex exec` or `codex mcp-server`); its status comes from the end of the thread's rollout (the same tail as for Claude Code). | `task_started` without its `task_complete` → working (its age counts from the start of the turn; a turn written before the process started is not working); `task_complete` → waiting, or error when it carries an error; `turn_aborted` (an interruption) → waiting; a tool call left without output for > 6 s while no command started during the turn runs in the folder (a child of a `codex` process, the TUI or its app server; a long command is polled by later calls, so the turn is what counts) → waiting, which is how an approval prompt or a question shows; error and waiting turn idle after 2 h. Context size and limit come from the last token count (`last_token_usage` of the model's `model_context_window`). Codex sessions of the IDE extension or the desktop app have no `codex` process of their own and are not shown. |

Cards are ordered by activity: working sessions first (in start order, so they don't swap places), then everything
else with the most recent status change first. Agents left waiting or idle for a long time sink to the later pages.
Names are the folder name, `parent/name` when two folders share a name, and `name #1`, `name #2` for several
sessions in the same folder.

## Discovery

- Only USB serial ports behind these bridges are probed: WCH CH340 and CH9102 (VID `1A86`), Silicon Labs CP210x
  (`10C4`), Espressif native USB (`303A`) and FTDI (`0403`). Anything else needs `--port`. On macOS only `/dev/cu.*`
  is used.
- A port that does not answer the handshake is probed when it appears, again 10 s later, and 60 s after that; then
  it is left alone until it is replugged. A `--port` port is retried every 60 s. A port in use by another program is
  retried on every 2 s pass.
- Discovery runs in the background: connected displays get a frame at least every 2 s whatever the probing does (the
  display's link timeout is 6 s).
- One helper per display: the port is opened exclusively, so a second helper (or `demo`, or a second user's service)
  gets "busy".

## The background service

`managents service install` registers `managents run` to start at login and restart if it exits
([ADR-0010](../docs/adr/0010-always-on-user-service.md)). Run it again after an update to replace the old one.

| | macOS | Linux |
|---|---|---|
| Unit | LaunchAgent `com.github.tonylook.managents`, `~/Library/LaunchAgents/com.github.tonylook.managents.plist` | systemd user unit `managents.service`, in `~/.config/systemd/user/` |
| Log | `~/Library/Logs/managents.log` (and `.1`), also readable in Console.app | `journalctl --user -u managents` |
| Windows | no service yet: Task Scheduler → "At log on" → `managents.exe run` | |

`managents service status` shows whether it runs, any warnings and the latest log lines. `uninstall` stops the
service and removes its file and logs. Your user must be allowed to open serial ports on Linux (group `dialout` or
`uucp`). The binary must live somewhere permanent (`~/.local/bin`): `install` refuses temporary folders, `go run`
builds, and on macOS `~/Desktop`, `~/Documents` and `~/Downloads`.

### What the log says

Lines worth knowing (the exact text):

| Level | Message |
|---|---|
| info | `display connected` (port, board, firmware, size) and `display disconnected` |
| info, once per busy episode | `serial port in use by another program; if it is the managents service: managents service stop` |
| info, once per plug-in | `serial port did not answer as a managents display; new board? run: managents flash` |
| warn | `display firmware is out of date; update it with: managents flash` (with the firmware and the minimum) |
| info | `display speaks a newer protocol than this helper; update the helper` |
| warn | `listing serial ports failed` (once per distinct error) |

## Privacy

The helper reads the session registry, the **end** of each open session's transcript (the last 256 KiB, growing to
at most 4 MiB only when no token usage is found there), OpenCode's database read-only, Codex's thread list read-only
(folder and rollout path only) with the same tail of each thread's rollout, and the process list. A Claude Code
transcript is decoded for four things only: entry type, sidechain flag, API-error flag and token counts; a Codex
rollout for entry and event types, timestamps, whether a turn ended in an error, and token counts. Conversation
text is never decoded, stored, logged or sent. The frame on the USB cable carries the folder name, status, age and
context token counts, nothing else (no directory paths). No network connections. Canary tests
(`TestConversationContentNeverLeaks` in every detector package) enforce it.

## Layout

See [docs/architecture.md](../docs/architecture.md#helper). Tests: `go test -race ./...` (or `make test-helper`): no
agents, ports or displays needed; the protocol tests check the shared fixtures in [`../protocol`](../protocol)
against the schema. `go tool staticcheck ./...` and `go tool govulncheck ./...` are pinned in `go.mod`.
