# managents firmware

Firmware for the LCDWiki 4.0" ESP32-32E display (E32R40T; the E32N40T without touch is untested, build it with
`-DMANAGENTS_TOUCH=0`). It listens on the USB serial port, decodes protocol v1 frames and draws one card per agent.
No Wi-Fi, no Bluetooth, no settings.

Users never build it: the helper embeds a release image and `managents flash` writes it. This page is for
development.

## Build, test, flash

Requires [PlatformIO Core](https://docs.platformio.org/en/latest/core/installation/index.html).

```bash
pio test -e native                         # unit + contract tests of lib/core on this computer (89 cases, ~12 s)
pio run -e e32r40t                         # build; also writes the merged image to flash at 0x0
pio run -e e32r40t -t upload               # flash (add --upload-port /dev/cu.usbserial-XXXX if needed)
pio device monitor                         # see the hello message; type {"v":1,"t":"hello"} to probe
pio run -e sim && .pio/build/sim/program ../docs/screens   # render the UI into PNGs (`make screens`)
```

`pio` may not be on your `PATH` (it is `~/.local/bin/pio` after `pipx install platformio`): from the repository root
use `make PIO=~/.local/bin/pio ...`. `make flash` and `make test-firmware` wrap the commands above, and `make flash`
stops the helper's service around the upload, because the service holds the serial port.

The USB-C port has auto-reset: no buttons needed to flash. Upload runs at 460800 baud; 921600 fails through the
CH340C.

**Versions.** The firmware's version is not written by hand: it is `MANAGENTS_VERSION` from the environment (a
leading `v` dropped), else `git describe --tags --match 'v*'`, else `0.0.0-dev`. The helper treats `0.0.0-dev` as an
unknown version, not an old one.

**Image.** `.pio/build/e32r40t/managents-firmware-e32r40t.bin` (about 484 KB) is bootloader, partition table,
application and boot app in one file, for flash offset 0x0. `managents flash` writes it; plain esptool does too:
`esptool.py --chip esp32 write_flash 0x0 managents-firmware-e32r40t.bin`. `make helper` embeds the image from the
last build into the helper.

**Runtime budget** (`pio run -e e32r40t`): flash 418,505 B, 31.9% of the 1.25 MB application partition; RAM 32,636 B,
10.0%. The strip sprite is 37.5 KiB of heap and each logo briefly uses at most 3.2 KB while drawn. The loop runs under
the 5 s task watchdog: any new blocking work longer than that (OTA, calibration) must call `feedLoopWDT()`.

Build-time options (add to `build_flags` in `platformio.ini`):

| Flag | Default | Meaning |
|---|---|---|
| `MANAGENTS_ROTATION` | `1` | `1` = landscape with USB-C on the right (the enclosure's orientation), `3` = flipped, `0`/`2` = portrait. The layout adapts. |
| `MANAGENTS_BACKLIGHT` | `200` | Backlight level 0–255. |
| `MANAGENTS_BACKLIGHT_DIM` | `40` | Backlight level after a minute of "Reconnecting" or five minutes of no or only idle agents; a tap wakes it. |
| `MANAGENTS_LED_LEVEL` | `40` | RGB LED brightness 0–255. |
| `MANAGENTS_TOUCH` | `1` | `1`: a tap anywhere turns the page. `0` (E32N40T, no touch): pages turn by themselves, counting one full interval from the moment a second page appears. |
| `MANAGENTS_PAGE_INTERVAL_MS` | `10000` | Page interval without touch. |

## Structure

```
lib/core/            hardware-independent core (no Arduino) — all behaviour, unit-tested on the host
  application        bytes in → scene + LED + backlight out; link screens (Connecting, Setup needed, Reconnecting), timeouts, blink, dimming, redraw-on-change
  pager              nine cards per page, tap to turn, back to page one after 30 s; tap debouncing
  protocol           decode/validate host messages, encode hello
  scene              pure function: host state + time → everything the screen shows
  grid_layout        card grid for any count, screen size and orientation
  line_assembler     newline framing with oversize-line recovery
  format             ages, clock, percentages, UTF-8 → ASCII folding of names
  text_fit           fits a name into a box (font, line breaks, middle ellipsis) behind a text-measuring port
  ports              Display, StatusIndicator, HostLink interfaces
src/
  board/e32r40t.hpp  pin map and LovyanGFX device (ST7796S, XPT2046, backlight)
  ui/                ScenePainter (draws a Scene on any canvas), theme, vector logos, fonts
  adapters/          LgfxDisplay (strip renderer), RgbLedIndicator, SerialHostLink, TouchSensor
  main.cpp           composition root and loop
sim/                 host renderer: the real ui/ and LgfxDisplay on a headless LovyanGFX; fixtures -> docs/screens/*.png
scripts/             version.py (version from the tag), merge_image.py (the one image to flash at 0x0)
test/                Unity tests for lib/core, including the shared protocol fixtures (11 suites)
```

Design notes: [architecture](../docs/architecture.md#firmware), [strip rendering](../docs/adr/0006-strip-rendering.md),
[hardware findings](../docs/hardware.md#bring-up-findings-2026-10-09-on-a-real-e32r40t).
