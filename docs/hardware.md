# Hardware

## The board

**LCDWiki 4.0" ESP32-32E display**, SKU **E32R40T** (resistive touch, the tested board) or **E32N40T** (no touch; untested, build it with `-DMANAGENTS_TOUCH=0`). Vendor page:
<https://www.lcdwiki.com/4.0inch_ESP32-32E_Display> — schematic, outline drawing (`E32R40T_Size.pdf`), STEP model,
and sample code.

| Item | Value |
|---|---|
| Module | ESP32-WROOM-32E: ESP32-D0WD-V3, dual core 240 MHz, 520 KB SRAM, 4 MB flash, **no PSRAM** |
| Panel | 4.0" TN TFT, 320 × 480, **ST7796S**, 4-wire SPI, RGB565 used; best viewing direction "12 o'clock" (portrait top) |
| Touch | Resistive, **XPT2046** on the LCD's SPI bus (E32R40T only) |
| USB | USB-C through a **CH340C** bridge (VID:PID `1A86:7523`), with auto-reset to the download mode |
| Extras | RGB LED (common anode), microSD, speaker connector (SC8002B amp on the DAC), Li-Po connector with TP4054 charger, I²C/SPI/UART/input JST 1.25 mm headers |
| Power | 5 V over USB-C; ~230 mA with the display on |

### Pin map

| Function | GPIO | Notes |
|---|---|---|
| LCD CS / DC | 15 / 2 | DC high = data |
| SPI SCLK / MOSI / MISO | 14 / 13 / 12 | shared by LCD and touch (HSPI) |
| LCD reset | EN | resets together with the ESP32 |
| Backlight | 27 | high = on, PWM-dimmable |
| Touch CS / IRQ | 33 / 36 | IRQ low while touched |
| RGB LED R / G / B | 22 / 16 / 17 | common anode: **low = on** |
| microSD CS / MOSI / SCK / MISO | 5 / 23 / 18 / 19 | shared with the SPI header (VSPI) |
| Audio enable / DAC | 4 / 26 | enable is active low |
| Battery voltage | 34 | ADC through a 100 kΩ / 100 kΩ divider |
| BOOT button | 0 | |
| UART0 RX / TX | 3 / 1 | also the USB serial port |
| I²C header SCL / SDA | 25 / 32 | |
| SPI header CS | 21 | |
| Input-only header | 35, 39 | |

The firmware's copy of this table is [`firmware/src/board/e32r40t.hpp`](../firmware/src/board/e32r40t.hpp).

### Bring-up findings (2026-10-09, on a real E32R40T)

| Finding | Consequence |
|---|---|
| Chip reports ESP32-D0WD-V3 rev 3.1; esptool warns the crystal reads 41.01 MHz | Harmless; runs at 40 MHz. |
| Uploading at 921600 baud fails through the CH340C on macOS ("serial data stream stopped") | `upload_speed = 460800`. |
| First frames were garbled with SPI at 40 MHz, touch CS unconfigured and no wait on the strip DMA | Now 27 MHz, touch configured (keeps its CS high), `waitDMA()` after each strip. Output is clean. |
| Colours correct with `invert = false`, BGR order | Matches the vendor's init (no INVON, MADCTL BGR). |
| LovyanGFX rotation 1 = landscape with **USB-C on the right** | Default `MANAGENTS_ROTATION=1`; the enclosure is built around it. |
| Opening the port with DTR/RTS low does **not** reset the board | The helper connects in ~50 ms without a reboot. |

### Placement

The panel is TN, and its best viewing direction (portrait top, the antenna end) lies on the **left** in rotation 1.
Put the display to the right of your line of sight, so you look at it from its left; seen from the right, dark greys
invert. Looking down at the 45° face from a seat moves the eye along the panel's symmetric axis, which TN panels
tolerate. This follows from the drawing and is still to be confirmed on the real unit.

## Mechanical data

From the vendor outline drawing (V1.0, 2025-04-15), tolerance ±0.2 mm. Portrait, seen from the front, USB-C edge
at the bottom:

| Item | Value |
|---|---|
| PCB | 60.88 × 111.11 × 1.6 mm, corner radius 3.5 |
| Total thickness | 5.65 mm (PCB 1.6 + tape 0.5 + LCD 2.5 + touch 1.05); back components up to 5.09 mm |
| Mounting holes | 4 × Ø3.2, pad Ø5.6; centres 53.28 × 104.11 apart, 3.80 from the long edges, 3.50 from the short edges |
| LCD outline | 60.88 × 94.57, 8.27 from the top edge |
| Touch view area | 56.88 × 85.22, 10.27 from the top, 2.00 from the sides |
| LCD active area | 55.68 × 83.52, 10.92 from the top, 2.60 from the sides |
| Antenna notch | 20.00 wide × 6.70 deep in the top edge, 21.14 from the right edge |
| USB-C | top-mount receptacle on the back, centred on the bottom edge, about 0.5 mm past it; only its shell's solder tabs show on the front |
| Back connectors | 1.25 mm JST, side entry at the board edge. Right edge (front view): I²C, SPI, speaker, IO35/39 at 18.01, 41.01, 62.94, 81.67 from the top; left edge: microSD, UART, battery at 67.50, 44.92, 25.48 from the bottom |
| RGB LED | back, near the centre: about 30.8 from the left edge and 52.5 from the bottom (measured on the vendor render, ±1 mm), facing away from the screen |
| BOOT / RESET | push buttons on the back, 3.26 from the bottom edge; BOOT 15.38 from the left edge, RESET 15.38 from the right |

The active area is **not centred**: it is closer to the antenna end. The enclosure's window follows the drawing. These
values live in [`enclosure/lib/e32r40t.scad`](../enclosure/lib/e32r40t.scad).

The front shows only the LCD and the USB-C shell's tabs, so any case hides everything else:

- **RGB LED**: the LCD covers it from the front. In the [enclosure](../enclosure/README.md) its light only gets out
  through the back grille (or through a translucent body), so it is a glow behind the display, not a front indicator.
- **BOOT and RESET**: not needed. The CH340C's auto-reset restarts the board and enters the download mode for
  flashing.
- **microSD**: unused, and unreachable inside the enclosure.

The enclosure needs its four rubber feet (without them a tap slides it) and takes USB-C plugs with overmolds up to
about 13 × 8.5 mm. Print and assembly notes are in [`enclosure/README.md`](../enclosure/README.md).

## Other boards

Everything board-specific is in `firmware/src/board/` and `firmware/platformio.ini`. The core and the UI adapt to any
resolution and orientation (the layout is computed from the screen size reported by the panel). Supporting another
LovyanGFX-compatible display means a new board file and environment; see the roadmap.
