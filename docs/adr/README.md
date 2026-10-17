# Architecture decision records

Short records of decisions that shape the project, in the order they were made. A decision is changed by adding a new
record that supersedes the old one, never by editing history. That rule starts with the first tagged release: until
then, records may still be corrected in place.

| ADR | Decision | Status |
|---|---|---|
| [0001](0001-record-architecture-decisions.md) | Record architecture decisions | Accepted |
| [0002](0002-dumb-display-smart-helper.md) | Dumb display, smart helper | Accepted |
| [0003](0003-json-lines-over-usb-serial.md) | JSON lines over USB serial | Accepted |
| [0004](0004-firmware-stack.md) | PlatformIO, Arduino and LovyanGFX, with a host-tested core | Accepted |
| [0005](0005-helper-in-go.md) | Helper written in Go | Accepted |
| [0006](0006-strip-rendering.md) | Render in strips instead of a frame buffer | Accepted |
| [0007](0007-parametric-45-degree-enclosure.md) | Parametric OpenSCAD enclosure, inclined 45° | Accepted |
| [0008](0008-passive-first.md) | Passive display first, touch later | Accepted, paging amended by 0009 |
| [0009](0009-display-ui-rules.md) | Display UI rules | Accepted |
| [0010](0010-always-on-user-service.md) | Run the helper as an always-on per-user service | Accepted |
| [0011](0011-host-renderer.md) | Render the display UI on the host | Accepted |
| [0012](0012-built-in-flasher.md) | Flash the display from the helper | Accepted |

Template: Context → Decision → Consequences.
