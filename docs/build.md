# Build guide

Everything on this page is needed to replicate the project. Four parts to get, then the software, which is one
command each: [Install and flash](install.md).

| Part | How many |
|---|---|
| LCDWiki 4.0" ESP32-32E display, model E32R40T | 1 |
| USB-C data cable | 1 |
| 3D-printed stand: body and bezel | 1 of each |
| Self-tapping screws ST2.9 x 9.5 | 4 |
| Self-adhesive rubber feet, 10 mm round | 4 |

You also need a Mac or Linux computer that runs Claude Code or OpenCode, and a Phillips or Torx screwdriver that fits
your screws.

<p align="center">
  <img src="../enclosure/images/assembly.png" alt="The assembled stand with the display (render)" width="560">
</p>

## 1. The display

The project is made for one board: the **LCDWiki 4.0" ESP32-32E display, model E32R40T**. The firmware drives its
exact pins and panel, and the stand is cut to its dimensions. Buy that board, not a similar one.

What it is: a 4.0" 480 x 320 colour screen (ST7796S) with resistive touch, an ESP32-WROOM-32E, a USB-C port and an RGB
LED, on one 61 x 111 mm board.

**Where to buy it.** Any electronics shop that sells it. It is a common board on Amazon, AliExpress and Alibaba;
AliExpress and Amazon tend to sell single boards, Alibaba tends to sell in bulk. Other shops work as well. Type one of
these in the search box:

- `E32R40T`
- `ESP32-32E 4.0 inch display`
- `LCDWiki ESP32 4.0 inch 480x320 touch`

**Check the listing before you order:**

- It says **4.0 inch**, **480x320** (or 320x480), **ST7796**, **resistive touch** and **ESP32-32E** (or
  ESP32-WROOM-32E). The model name **E32R40T** is the best sign. The letter R is the touch version; the E32N40T has no
  touch and is not what this project was built and tested on.
- The photos match the board on the [vendor page](https://www.lcdwiki.com/4.0inch_ESP32-32E_Display): a portrait board
  with a USB-C port in the middle of the bottom edge and the ESP32 antenna at the top.
- It is the 4.0 inch size. The same shops sell 2.8 inch and 3.5 inch ESP32 displays and look-alikes under other names;
  their pins differ and the firmware will not work on them.
- A listing may bundle a cable, a speaker or a battery. Only the board is needed.

The board arrives with no managents firmware. [`managents flash`](install.md#flash-the-display) puts it on.

## 2. The cable

A USB-C cable that carries **data**, not only power; many cheap cables are charge-only and are the most common reason a
board is not found. Search `USB-C data cable`, or `USB-C right angle cable` to keep it flat on the desk, which is
what the stand is made for: its side opening fits plugs with an overmold up to about 13 x 8.5 mm.

## 3. The stand

A 45 degree wedge that holds the screen in landscape, USB-C on the right. Two printed parts, no supports.

**Print.** The files are in [`enclosure/stl/`](../enclosure/stl/): `managents-body.stl` and `managents-bezel.stl`.
PLA or PETG, 0.2 mm layers, 4 walls, 15 % infill, no supports. If you have no printer, upload the STLs to an online
3D printing service or ask a local makerspace or library. The design is printed and tested: the board fits as it
comes.

**Screws.** Four self-tapping pan head screws, **ST2.9 x 9.5** (DIN 7981 type C, stainless or zinc). Search
`ST2.9 x 9.5 self tapping screws pan head`. A thread-forming 3.0 x 10 mm pan head screw for plastic also works. No
inserts, no nuts.

**Feet.** Four self-adhesive rubber feet, 10 mm diameter, at least 2 mm thick. Search `self adhesive rubber feet 10mm
round`. Without them a tap on the screen slides the stand across the desk.

**Assemble.** Drop the board into the body, screen up, USB-C towards the side opening. Put the bezel on and drive the
four screws into the corner holes in two passes, stopping as soon as the heads touch the bezel. Stick the feet under
the base. Step by step, with print notes and a picture of every part: [enclosure/README.md](../enclosure/README.md).

## 4. The computer

macOS (tested on Apple silicon) or Linux, with Claude Code and/or OpenCode. The helper has no other requirements and
needs no `sudo`. Supported systems: [Install and flash](install.md#supported-systems).

## 5. Software

Install the helper, plug the display in, flash it once: [Install and flash](install.md). Or tell your AI to do it, as
the [README](../README.md#or-let-your-ai-do-it) explains.
