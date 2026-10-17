# Troubleshooting

Start with two commands. They tell you most of what you need:

```sh
managents service status     # is the helper running, its warnings, the latest log lines
managents status             # the agent sessions the helper detects right now, display or not
```

The helper's log is `~/Library/Logs/managents.log` on macOS (also in Console.app) and
`journalctl --user -u managents` on Linux. Without the service, `managents run` logs to the terminal; add
`--log-level debug` for more.

## Symptom, check, fix

| Symptom | Check | Fix |
|---|---|---|
| The display shows **Setup needed** (a QR code) | The helper is not running or not installed. `managents service status` | Not installed: run the one-liner from the [README](../README.md#get-started). Installed but stopped: `managents service start`. Not running after a login: see "The service does not start" below. |
| The display shows **Connecting** for longer than a few seconds, then Setup needed | The helper cannot talk to it. `managents devices` | Use a USB-C **data** cable (many are charge-only). See "No display found" below. |
| **Reconnecting... Is the computer asleep?** | The helper stopped sending: sleep, the service stopped, the cable came out. | Wake the computer. If the computer is awake, `managents service status`; `managents service start` if it is stopped. The display recovers by itself within a second or two. |
| No display found: `managents devices` lists nothing | The cable, then the driver. | Try another cable and another port. On Windows install the CH340 driver. On Linux see "Permission denied". |
| `managents devices` says **not a managents display (new board? run: managents flash)** | The board has no managents firmware, or other firmware. | `managents flash`. |
| `managents devices` says **in use by another program** | The service holds the port, which is exactly right while it runs. | Nothing to fix. To use the port yourself (`demo`, a serial monitor, PlatformIO): `managents service stop`, then `managents service start` afterwards. Only one program can open the port at a time. |
| The display is lit but shows **No agents open** | `managents status` | Nothing is detected: see "No agents shown". |
| A card shows the wrong state | `managents status` shows what the helper sees; the display only draws it. | OpenCode cannot report a permission prompt, so "waiting" is inferred when a tool stays at "running" for 6 seconds: a long silent command looks the same. Codex does not record its approval prompts either: "waiting" is inferred when a tool call has had no output for 6 seconds and no command started during the turn runs in the folder, so a long call to an MCP tool looks the same. Sessions turn idle after 2 hours. Report anything else with the output of `managents status`. |
| A card name shows `?` | The display's fonts have ASCII only. | Accents are dropped (`é` shows `e`); other characters show `?`. Known limitation. |
| The backlight is dim | Nobody needs the screen: a minute of Reconnecting, or five minutes with no or only idle agents. | Tap it, or start an agent. |

## No agents shown

`managents status` prints what the helper detects. If it is empty while an agent is open:

- **Claude Code** is read from `~/.claude/sessions/<pid>.json` (the folder moves with `$CLAUDE_CONFIG_DIR`). Check
  the folder exists and has files for your running sessions. Records of sessions that are not running are ignored.
- **OpenCode** is read from its running `opencode` processes and `~/.local/share/opencode/opencode.db`
  (`$XDG_DATA_HOME/opencode/opencode.db` when set). Both have to be there. OpenCode 2 and 1 are both read; a helper
  older than v0.2.0 only reads OpenCode 1 and shows every OpenCode 2 session as idle: upgrade it.
- **Codex** is read from its running `codex` processes and `~/.codex/state_<N>.sqlite` (`$CODEX_HOME` when set),
  then the thread's log under `~/.codex/sessions`. Sessions of the IDE extension and the desktop app, and
  `codex exec` runs, are not shown. A session started with `--cd` in another folder is matched by the folder the
  `codex` process runs in, so start it from the project folder.
- The service runs as **you**, with your home folder. A display attached to a Mac where another user's service holds
  the port shows that user's agents (see "Several users").

## The service does not start

`managents service status` says why: the program path, the state and the last exit status.

- **macOS** may need you to allow it: System Settings, General, Login Items & Extensions, "Allow in the Background",
  switch managents on, then `managents service start`. `service status` mentions this when it is the cause.
- A binary in `~/Desktop`, `~/Documents` or `~/Downloads` is refused by `service install`, and macOS will not run
  background programs from there. Install with the one-liner (it uses `~/.local/bin`).
- After moving or deleting the binary, run `managents service install` from where it lives now.
- **Linux**: a user service needs a login session (`systemctl --user status managents`). `journalctl --user -u
  managents` shows the log.

## macOS says the program cannot be verified (Gatekeeper)

Files fetched with `curl` (the one-liner) carry no quarantine mark and the binary is ad-hoc signed, so this should not
happen. If you downloaded the archive in a browser, clear the mark and install again:

```sh
xattr -d com.apple.quarantine ~/.local/bin/managents
```

## Permission denied on Linux

Opening `/dev/ttyUSB0` or `/dev/ttyACM0` needs the `dialout` group (`uucp` on Arch and some others):

```sh
sudo usermod -aG dialout "$USER"     # then log out and back in
```

The helper retries every pass but does not log a permission error by default: `managents run --log-level debug` (stop
the service first) shows it. `ls -l /dev/ttyUSB0` shows the owner group.

## Flash failures

`managents flash` finds the board by itself; read the message it prints.

| Message | Meaning and fix |
|---|---|
| `no board found on a USB serial port: plug it in with a data cable...` | Cable, driver or port: see "No display found". |
| `several boards found, choose one with --port: ...` | `managents flash --port /dev/cu.usbserial-10` (the name `managents devices` prints; `COM3` on Windows). |
| `close any serial monitor or other flashing tool that uses ...` | Another program has the port. The service is stopped for you; close PlatformIO's monitor, `screen`, Arduino IDE. |
| It stalls or fails during the transfer | The bridge could not keep up. Use `managents flash --baud 115200`, a shorter cable, a different port. It already falls back to 115200 if the board does not follow 460800. |
| `interrupted: the display has no firmware until you run managents flash again` | You pressed Ctrl-C mid-write. Run it again: the chip's bootloader cannot be damaged. |
| It wrote and verified, but the display does not answer | Unplug and replug it, then `managents devices`. |
| `this helper was built without firmware` | You built the helper with plain `go build`. Use a release, `make helper` after `make firmware`, or `--image FILE`. |

A board whose USB bridge has no auto-reset circuit has to be put into its bootloader by hand (hold BOOT while
resetting); `managents flash` cannot do that for it. The E32R40T does not need it.

## Several users on one Mac

One display is held by one program: the first user's service opens the port exclusively and every other helper
(another user's service, `managents run`, `demo`) logs "serial port in use by another program" once and retries. To
hand the display to another user, run `managents service stop` as the first user; the second user's helper takes the
port on its next 2-second pass.

## Still stuck

Open an [issue](https://github.com/tonylook/managents/issues/new/choose) with the output of `managents version`,
`managents service status` and the last lines of the log. `managents status` lists the folder of each session, so
read it before pasting. Security problems: [SECURITY.md](../SECURITY.md).
