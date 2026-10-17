# 10. Run the helper as an always-on per-user service

Date: 2026-10-09 · Status: Accepted

## Context

The display shows nothing unless the helper runs, so plug and play needs the helper started at login without the
user thinking about it. Until now that meant copying a template from `helper/init/`, editing the binary path by hand
and loading it with the deprecated `launchctl load`; the log went to `/tmp`.

The helper only needs to work while a display is plugged in. launchd can start a job when a USB device appears
(`LaunchEvents` with `com.apple.iokit.matching` on the CH340's 1A86:7523), which looks cheaper than a program that
runs all day.

Measured on an Apple M3 with macOS 26:

- Idle, with no display: about 0.05% of one core and 9-15 MB of memory. Listing the USB serial ports takes 0.21 ms,
  every 2 s; nothing else runs until a display answers.
- With a display and three agent sessions: about 1% of one core, after the process scan moved to one `sysctl` call.
- From plugging the display in to the first frame: the next discovery pass (at most 2 s) plus about 50 ms of handshake,
  and up to about 1 s more while a freshly powered board boots.

## Decision

- `managents service install` installs a per-user service that runs `managents run` from login, always on: a
  LaunchAgent (`com.github.tonylook.managents`, in `~/Library/LaunchAgents`) on macOS, a systemd user unit
  (`managents.service`) on Linux. `uninstall`, `start`, `stop` and `status` complete the command. Windows has no
  service support yet.
- No launch on USB attach. A job started by an IOKit event must consume it through XPC
  (`xpc_set_event_stream_handler`, so cgo and libxpc), or launchd starts it again in a loop. The event fires for every
  CH340 device, not only displays, and Linux would need a udev rule installed as root. All of it would save less than
  0.1% of a core.
- The LaunchAgent sets `RunAtLoad` and `KeepAlive`, and no `ProcessType`. The default (Standard) is kept because the
  Background policy also throttles disk I/O, and the helper reads transcripts and the OpenCode database every second:
  during a heavy build, exactly when the status matters, throttled reads could exceed the display's 2 s keep-alive.
  At about 1% of a core, a lower priority saves nothing worth having.
- The program in the service file is the path the binary was run from, made absolute but not resolved through
  symlinks, so a package manager's link keeps working after an upgrade. `install` refuses binaries that would not
  start at the next login: `go run` builds, temporary folders, `~/Desktop`, `~/Documents` and `~/Downloads` (macOS
  keeps background services out of them), and installs as root.
- Logs: on macOS `run --log-file ~/Library/Logs/managents.log`, where Console.app finds it, with launchd's capture of
  stderr in the same file so a panic lands next to the lines before it. Past 1 MiB the file moves to
  `managents.log.1` at the next start, which bounds a crash loop (one restart every 10 s) to about 2 MiB. On Linux the
  journal keeps and rotates the log. `uninstall` removes the logs with the service file.
- One helper per display, without a lock file: the serial port is opened exclusively, so a second helper (`run`,
  `demo`, a second user's service) gets "busy". The helper logs that once with the hint `managents service stop`, and
  `managents devices` reports the port as in use by another program.

## Consequences

- Installing is one command, and running it again after an update replaces the old installation. On macOS 13 and
  later the user sees a "Background Items Added" notification, and the service can be switched off under Login Items &
  Extensions; `managents service status` mentions that switch when the service is not running.
- `managents` alone prints help instead of starting a second helper next to the service.
- Flashing a display that the service holds needs `managents service stop` first (`start` afterwards).
- On a Mac with several users logged in, the first user's service takes the display and the others log "busy" once.
- The service runs whether or not a display is plugged in, at the idle cost above.
