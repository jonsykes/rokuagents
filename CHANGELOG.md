# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.0] - 2026-10-14

The first release: from a prototype you build from source to something you install and plug in.

### Added

- **One-line install.** `install.sh` (the release asset behind the `curl | sh` line in the README) installs the helper
  into `~/.local/bin`, checks its SHA-256, registers the service and can be run again to upgrade.
- **Background service.** `managents service install|uninstall|start|stop|status`: a LaunchAgent on macOS, a systemd
  user unit on Linux, started at login and restarted if it exits; logs to `~/Library/Logs/managents.log` on macOS and
  to the journal on Linux. Plugging in a display then just works: ports are scanned every 2 s.
- **Built-in flasher.** `managents flash` writes the firmware embedded in the helper over the ESP32's ROM bootloader,
  at 460800 baud with a fallback to 115200 and an MD5 check, stopping and restarting the service around the
  transfer. No PlatformIO or esptool needed.
- **Releases.** A `v*` tag builds the helper for macOS (universal), Linux (amd64, arm64) and Windows (amd64, not
  tested), the merged firmware image, `install.sh` and checksums; the version comes from the tag, for the helper and
  the firmware. `make dist`, `dist-check`, `check-version`, `release-notes`.
- **Firmware version check.** The helper warns, in its log and in `managents devices`, when a display's firmware is
  older than the one it needs, or when the display speaks a newer protocol; development builds count as unknown.
- **Command line.** `managents` alone prints help; commands have exit codes and `-h`; `run --log-file`; `devices`
  lists every USB serial port, shows firmware versions and tells a busy port from a board that is not a display.
- **Link screens.** Connecting, Setup needed (a QR code to the README's Get started section) and Reconnecting replace
  the single waiting screen.
- **Page dots coloured by attention**: they point at errors and waiting agents on other pages.
- **Backlight dimming** (`MANAGENTS_BACKLIGHT_DIM`) when nobody needs the screen: a minute of Reconnecting, or five
  minutes with no or only idle agents. A tap wakes it.
- **Host renderer** (`make screens`): the real UI painted into `docs/screens`, with a check that strip painting
  equals a full-frame paint; CI fails when the committed images are out of date ([ADR-0011](docs/adr/0011-host-renderer.md)).
- **Documentation.** A README for newcomers, `AGENTS.md` for AI coding agents, `docs/troubleshooting.md`,
  `SECURITY.md`, ADRs 0009 to 0012, third-party notices.
- **Enclosure v1.2.** No heat-set inserts: four ST2.9 × 9.5 self-tapping screws cut their own thread in Ø2.5 pilot
  holes in the corner supports (4.9 mm of thread, 1.5 mm of plastic below the hole; the supports are 1 mm taller).
  The bezel and the fit-test coupon are unchanged; only the body STL is regenerated.
- **Enclosure v1.1.** Bezel posts make the bezel one solid and clamp the board; the USB-C cut is centred on the
  receptacle and fits overmolds up to about 13 x 8.5 mm; an even 0.4 mm board clearance and a 0.5 mm glass gap;
  required rubber feet at the corners (tip threshold about 9.4 N instead of about 5 N); rounded corner supports with a
  0.4 mm insert lead-in; fit-test pins that reach the board, used face down; 1 mm chamfers; a speaker mount that is
  off by default; a warning when `face_angle` is not 45. Binary STLs (0.45 to 2.77 MB), `make enclosure-check` (each
  part is one solid) in CI, `make enclosure-images` for reproducible renders; `make enclosure` works on OpenSCAD 2021.01.
- CI: job timeouts, a weekly run, `govulncheck`, `shellcheck`, `go mod tidy` and sanitizer builds of the host tests;
  the `-Werror -Wshadow` build of `src/`.

### Changed

- **Frames no longer carry the working directory.** `path` is reserved and ignored; frames hold folder names,
  statuses, ages and context token counts only. Agent ids are limited to 48 bytes.
- Names are folded to ASCII (accents dropped, other characters shown as `?`), wrapped onto two lines, and shortened in
  the middle only as a last resort. Long ages read `1d 2h`. Card text is uniformly bold and the context bar is
  readable on every card colour.
- Error cards and the LED blink only during the first minute of an error.
- Detection: a Claude error counts only when it is the last main-thread message, and old errors turn idle; errors and waiting states turn idle after 2 hours; Claude's config directory follows
  `$CLAUDE_CONFIG_DIR` and OpenCode's database `$XDG_DATA_HOME`; the transcript reader scans the last 256 KiB, up to
  4 MiB when it finds no token usage there; macOS processes are listed with one `sysctl` (about 3.5 ms of CPU per poll instead of 25 ms); same-folder
  cards are numbered in start order; OpenCode turn ages count from the prompt.
- Discovery probes only USB serial ports behind ESP32 bridges (CH340, CP210x, FTDI, Espressif), retries a silent
  port 10 s and 70 s later and then leaves it alone, and never delays frames to connected displays.
- The display decoder is stricter: an empty `id` or a `ctx.limit` below 1 rejects the frame; integers above 32 bits
  saturate.
- The Go toolchain is pinned (`go1.26.9`); `go test -cover` works without the covdata error.
- `make flash` stops and restarts the service around the upload.

### Fixed

- A short write to a serial port with a full output buffer is retried instead of dropping part of a frame.
- A closed device manager stays closed.
- **OpenCode 2 is detected again.** OpenCode 2 writes its sessions to `session_v2` and `session_message` and no longer
  touches the `message` and `part` tables the helper read, so every card stayed idle. The helper now reads the new
  tables (turn end and outcome, interrupted and failed replies, compaction) and falls back to the old ones for
  OpenCode 1. Its background service (`opencode serve`) is no longer shown as a session in the home folder.
- OpenCode: errors left by a previous run are ignored; a reply of a dead run is not "working"; a store error no longer
  hides the sessions.
- Logos are drawn the same in every strip; auto-advance on boards without touch waits a full interval after a second
  page appears; a null string no longer reaches `memcpy`; the firmware reboots when its loop stalls (5 s watchdog).

### Removed

- The single "Waiting for computer" screen, `helper/init` (the hand-edited plist and unit templates the service
  replaces), `process.Table` and the `LGFX_USE_V1` defines. The firmware's hand-written version number: it comes from
  the tag now.

## [0.1.0] - 2026-10-09 (never tagged)

The minimum viable product. It was never tagged or published as a release; its contents ship with the first release.

### Added

- Firmware for the LCDWiki 4.0" ESP32-32E display (E32R40T): protocol v1, adaptive card grid with up to nine cards
  per page and a tap anywhere to turn the page, clock, overflow badge, context bar, blinking error cards,
  "waiting for computer" screen with QR code, RGB LED status summary, flicker-free strip rendering.
- Helper (`managents`): Claude Code and OpenCode detection, ordering by activity (working first, long-idle last),
  Claude context size and limit, serial auto-discovery with handshake, hot-plug and multiple displays; `run`,
  `status`, `devices`, `demo` commands.
- Protocol v1 JSON Schemas and shared fixtures tested by both the helper and the firmware.
- 45° desk enclosure in OpenSCAD with ready-to-print STLs and a fit-test coupon.
- Documentation: architecture, protocol, hardware notes with bring-up findings, ADRs, roadmap.
- CI: helper tests on Linux, macOS and Windows; lint; firmware host tests and build; enclosure renders.

[Unreleased]: https://github.com/tonylook/managents/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/tonylook/managents/commits/main
[0.1.0]: https://github.com/tonylook/managents/commit/b9edbd2
