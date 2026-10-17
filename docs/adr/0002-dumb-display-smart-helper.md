# 2. Dumb display, smart helper

Date: 2026-10-09 · Status: Accepted

## Context

Agent status lives on the computer: Claude Code's session registry and transcripts, OpenCode's processes and SQLite
database. These formats are internal to the tools and change between releases. The display could read nothing of it
directly anyway.

## Decision

All detection, naming and ordering happen in the helper. The display receives display-ready data and only draws it.
It has no Wi-Fi, no Bluetooth and no configuration. Layout stays on the display, because it depends on the screen.

## Consequences

- A change in an agent's storage format means a helper release, never a reflash.
- The firmware is small and fully testable against fixtures.
- New agent products are added on the host only.
- The display is useless without the helper running. It says so: a Connecting screen at first, a setup screen with a
  QR code to the setup steps when the helper stays silent, and a Reconnecting screen when it stops talking
  ([ADR-0009](0009-display-ui-rules.md), [protocol](../protocol.md#behaviour)).
