# Roadmap

What is done, and what might come. The UI rules in [ADR-0009](adr/0009-display-ui-rules.md) hold for everything
below. Protocol changes stay backwards compatible ([protocol.md § Versioning](protocol.md#versioning)).

## v0.1: MVP (never tagged)

- [x] Firmware: protocol v1 decoder, adaptive card grid, tap anywhere for the next page, clock, `+N` badge, context
      bar, blinking error cards, RGB LED summary.
- [x] Helper: Claude Code and OpenCode detection, activity ordering, hot-plug discovery with a `hello` handshake,
      multiple displays, `run` / `status` / `devices` / `demo`.
- [x] Shared contract: JSON Schemas and fixtures tested on both sides.
- [x] Enclosure v1: 45° wedge, support-free parts, fit-test coupon.
- [x] CI: tests, lint, firmware build, STL render.

## v0.2: Plug and play

Goal: someone else can install it in five minutes. The whole round is in the [changelog](../CHANGELOG.md).

- [x] `managents service install|uninstall|start|stop|status`: start at login as a LaunchAgent (macOS) or a systemd
      user unit (Linux), always on ([ADR-0010](adr/0010-always-on-user-service.md)).
- [x] `install.sh`, the one-line installer, and a release workflow on `v*` tags: the helper for macOS (universal),
      Linux (amd64, arm64) and Windows (amd64, untested), the merged firmware image, checksums.
- [x] `managents flash`: a built-in ESP32 flasher with the firmware embedded in the helper, instead of a web flasher
      ([ADR-0012](adr/0012-built-in-flasher.md)). One version for helper and firmware, from the tag.
- [x] Firmware version check: the helper warns when the display's `fw` is older than `MinFirmware`.
- [x] Display UI: Connecting, Setup needed (QR code to the README) and Reconnecting screens, error cards that blink
      only for a minute, page dots that point at errors and waiting agents, a backlight that dims when nobody needs
      it, names folded to ASCII and fitted.
- [x] Host renderer: the real UI painted into `docs/screens`, checked in CI ([ADR-0011](adr/0011-host-renderer.md)).
- [x] Privacy: frames carry no paths; canary tests keep conversation text out of the helper's output.
- [x] Enclosure v1.1: bezel posts that clamp the board, a centred USB-C cut, even board clearance, required feet,
      chamfers, a solid-check in CI.
- [ ] Hardware sign-off before tagging: see [Releasing](../CONTRIBUTING.md#releasing).

## Later

No order, no dates. Each item would be one issue.

- **Touch beyond paging.** Long-press a card for a detail view (a plain tap stays "next page"). Anything that needs
  to know where the tap was needs calibration, which must stay lazy: never at first boot.
- **Sound.** A chime when an agent starts waiting or fails. The enclosure has the back grille; the speaker mount
  (`speaker_mount`, off by default; its underside needs supports) and a relief in the face shell for the side-entry
  SPEAKER plug come with this feature.
- **More agents.** OpenAI Codex CLI, Gemini CLI, Aider, Cursor's agent CLI: one detector package each.
- **Richer status.** The model's real context window instead of inferring 200k or 1M from usage; a subagent badge;
  `managents status --watch`. Open question: use Claude Code's session name instead of the folder name?
- **Platforms.** Verify Windows end to end (process detection, COM ports, the CH340 driver) and add a service there;
  a Homebrew tap.
- **Display.** LED patterns and an off switch; settings (brightness, LED, sound) set from the computer.
- **Other boards.** One directory per board under `firmware/src/board/` and one PlatformIO environment each, selected
  at build time; the E32N40T (no touch) is untested.
- **Enclosure.** Fold measured corrections from the first print into `lib/e32r40t.scad`; a cable strain relief; print
  profile notes from real prints; the LED position (back of the PCB, hidden).
- **Robustness.** Fuzz the firmware decoder and the helper's transcript reader.
