# Install and flash

Two things go on the system: the **helper** on your computer, and the **firmware** on the display. Both come from the
same release, and the firmware is built into the helper, so you install only the helper.

## Install the helper

On macOS and Linux:

```sh
curl -fsSL https://github.com/tonylook/managents/releases/latest/download/install.sh | sh
```

It downloads the helper for your computer, checks its SHA-256, puts it in `~/.local/bin/managents` and runs
`managents service install`, so it starts at login and restarts if it stops. Nothing needs `sudo`. If
`~/.local/bin` is not on your `PATH`, the installer says how to add it.

Check it:

```sh
managents version
managents service status      # running (pid N), plus the latest log lines
```

On Windows, take `managents_windows_amd64.zip` from the
[latest release](https://github.com/tonylook/managents/releases/latest): the binary only, untested, with no service.
Start `managents.exe run` yourself, and install the CH340 driver if the display is not found.

Settings for the installer: `MANAGENTS_VERSION=v0.2.0` for a specific release, `MANAGENTS_BIN_DIR` for
another folder, `MANAGENTS_NO_SERVICE=1` to skip the service.

## Flash the display

Plug the display in with a USB-C **data** cable and run:

```sh
managents flash
```

A board straight from the shop has no managents firmware, so the helper does not recognise it (its log says "new board?
run: managents flash"). The command takes about 13 seconds, checks the result against the chip's own checksum, and
stops and restarts the service around the transfer. It overwrites whatever firmware the board had. With several boards
connected, choose one with `--port`; `managents devices` lists the names.

Within a couple of seconds every open Claude Code and OpenCode session appears as a card. Unplug and replug whenever
you like; the helper finds the display again.

## Upgrade

Run the one-liner again. If the log or `managents devices` says the display's firmware is out of date, run
`managents flash`.

## Uninstall

```sh
managents service uninstall
```

Then delete `~/.local/bin/managents`. That removes everything the helper put on your computer.

## Supported systems

| Component | Status | Notes |
|---|---|---|
| macOS 26, Apple silicon | tested | The helper is one universal binary. |
| macOS, Intel | built, not tested | The universal binary includes it. |
| Linux, amd64 and arm64 | best effort | Unit tests run in CI. systemd user unit. Your user needs the `dialout` (or `uucp`) group to open serial ports. |
| Windows 11 | binary only, untested | Unit tests run in CI. No service. |
| Claude Code (terminal and desktop app) | supported | Read from its session files. |
| OpenCode | supported | Read from its process list and database. |
| E32R40T display | tested | The board this project is made for. |
| E32N40T display (no touch) | untested | Build the firmware with `-DMANAGENTS_TOUCH=0`. |

Something not working? [Troubleshooting](troubleshooting.md).
