# 11. Render the display UI on the host

Date: 2026-10-09 · Status: Accepted

## Context

The UI could only be seen on the board: there was no picture for the README, a UI change could not be reviewed in a
pull request, and the core tests stop at the `Scene`, so nothing checked the pixels. Strip rendering (ADR 0006) adds
a failure the core tests cannot see: a drawing that does not line up across strip boundaries. Two logos did exactly
that.

LovyanGFX builds on a desktop only with SDL2, OpenCV or the Linux frame buffer; without one its platform header stops
with `#error unknown platform`.

## Decision

- `pio run -e sim` builds `firmware/sim`, a command-line program that runs the real `ui/` code and `LgfxDisplay` on
  the host: protocol fixture → `core::buildScene` → 40-pixel strips → a panel-sized RGB565 sprite → PNG.
- No SDL. `sim/headless/SDL2/SDL.h` comes first on the include path and declares the few platform hooks LovyanGFX needs
  to draw into memory; it does not define `SDL_h_`, so the SDL window and event code stays compiled out.
  `sim/headless/platform.cpp` implements the hooks (time from the host; pins and buses that do nothing).
- Every screen is also painted in one full-frame pass, and any difference fails the run. The renderer paints each
  page of every valid fixture and the waiting screen, in both orientations and both blink phases.
- `make screens` writes the landscape screens, a few variants and a pixel-doubled `showcase@2x.png` to
  `docs/screens`, which is committed. The firmware CI job renders them again and fails if `docs/screens` changes, so
  the committed images are golden images.
- The env is built with `-O2` and `-ffp-contract=off`, and uses the same LovyanGFX pin as the board.

## Consequences

- The README shows the device's own pixels, and a UI change shows up as an image diff in its pull request.
- Strip seams fail CI instead of showing on the panel.
- The images must be byte-identical across compilers. They are for Apple clang 21 (arm64) and GCC 13 on Ubuntu 24.04
  (x86_64, as on CI). Floating point is used by the logos, LovyanGFX's anti-aliasing and the grid choice in
  `core::chooseGridShape` (a whole-number result), and fused multiply-add is off.
- LovyanGFX 1.2.30 compiles `lgfx_qrcode_getModule` as C, where it returns an `unsigned char` that can be 128, while
  C++ callers expect a `bool` of 0 or 1. Whether the QR code is drawn depends on how the compiler tests that value:
  right on the board, with clang at `-O2` and with GCC on x86_64; nearly blank at `-O0` and with GCC on arm64 Linux.
  The golden diff shows it when it happens.
- The shim relies on how LovyanGFX 1.2.30 selects its platform. A LovyanGFX upgrade that changes it fails to compile
  or link instead of drawing differently.
