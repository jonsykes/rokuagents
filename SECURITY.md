# Security policy

## Supported versions

Only the latest release gets fixes. managents is a small project with one maintainer: please update first
(`curl -fsSL https://github.com/tonylook/managents/releases/latest/download/install.sh | sh`).

## Reporting a vulnerability

Please report privately, not in a public issue: use **Report a vulnerability** on the repository's
[Security tab](https://github.com/tonylook/managents/security/advisories/new). Include what you found, how to
reproduce it and the version (`managents version`). You will get a reply within 7 days.

## Design notes

What the software does and does not do, so you can judge a report against it.

- **The helper runs as you**, never as root, and `managents service install` refuses to install as root. It needs no
  privileges.
- **What it reads:** Claude Code's session files, the end of each open session's transcript (decoded for status,
  timestamps and token counts only), OpenCode's database read-only, Codex's thread list read-only with the end of
  each open thread's log (decoded the same way), and the process list.
- **What it sends:** only to a USB serial port, only after the device answered a `hello` handshake as a managents
  display: folder names, statuses, ages and context token counts. No directory paths, no conversation text.
- **No network.** The helper makes no network connections of its own. `install.sh` downloads from GitHub releases and
  checks the SHA-256 of the archive against `checksums.txt` before installing.
- **The display has no radio** (no Wi-Fi, no Bluetooth) and a bounded parser: lines of at most 4096 bytes, frames
  validated as a whole, the loop under a watchdog.
- **The flasher** writes only to a classic ESP32 that identifies itself over the ROM bootloader, and only when you
  run `managents flash`. The service never writes firmware.
- Releases are built by a GitHub Actions workflow from a tag and ship SHA-256 checksums. The macOS binary is ad-hoc
  signed, not notarised.
