# 12. Flash the display from the helper

Date: 2026-10-09 · Status: Accepted

## Context

The owner wants one thing to install: the helper. Flashing a board with PlatformIO or esptool needs Python, a
toolchain or a download of its own, and a user who only wants the display would have to learn them first. The board
is a classic ESP32 behind a CH340C USB bridge, and the bridge's DTR and RTS lines already drive the chip's boot and
reset pins, so the helper can start the chip's bootloader itself.

## Decision

- `managents flash [--port P] [--image FILE] [--baud N]` writes the firmware that is embedded in the helper
  (`go:embed`; release builds copy the merged image into `helper/internal/firmware/images`). `--image` writes another merged image, for development.
- `internal/flash` speaks the serial protocol of the ESP32's ROM bootloader, the one esptool uses without its
  flasher stub: SLIP framing, sync, chip check, SPI attach, compressed write, MD5 check, reset. Nothing but the
  helper and the ROM is involved. The port is found as for the other commands; if several boards match, `--port`
  is required.
- The transfer runs at 460800 baud and falls back to 115200 when the chip or the bridge does not follow. 921600
  fails on the CH340C.
- The background service holds the port, so a running service is stopped for the transfer and started again
  afterwards, also when the flash fails.
- Afterwards the helper reads the display's handshake and prints its board and firmware version.

## Rejected alternatives

- **Depend on esptool.** Python and a pip install on every machine, and a second program whose version must match.
  It would not be one install.
- **A web flasher** (ESP Web Tools). It needs a Chromium browser, a hosted page and a manifest, works only online,
  and the firmware would no longer come with the helper version that speaks its protocol.
- **esptool's flasher stub.** Faster, but a per-chip binary blob to carry and keep in step. The ROM protocol is
  fast enough for one image.

## Consequences

- A release helper installs, updates and checks the display with no other tool. The firmware it embeds is the one
  it was tested with: `managents devices` and the service log say when the board is older.
- Flashing is only ever explicit. The service and `managents run` never write to a board, however old its firmware.
- The ROM bootloader is in mask ROM and cannot be overwritten, so an interrupted or failed flash leaves a board
  that can be flashed again: the old firmware is erased first, but the chip always starts its bootloader on
  request. The written image is checked against the chip's own MD5 before the board restarts into it.
- Only the classic ESP32 is accepted: the chip is identified before anything is erased, and other chips are
  refused.
- A helper built without images (a plain `go build`) has no firmware; `managents flash` then needs `--image`.
- The reset sequence assumes the usual DTR/RTS auto-reset circuit. A board without it has to be put into its
  bootloader by hand (hold BOOT while resetting), which `managents flash` cannot do for it.
