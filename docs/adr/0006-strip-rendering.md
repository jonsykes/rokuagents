# 6. Render in strips instead of a frame buffer

Date: 2026-10-09 · Status: Accepted

## Context

A full 480 × 320 RGB565 frame buffer needs 300 KB. The ESP32-WROOM-32E has 520 KB of SRAM in total, no PSRAM, and
its largest free block is far smaller. Drawing directly on the panel flickers (background first, then text).

## Decision

The application builds an immutable `Scene` and the display adapter paints it into a 480 × 40 sprite (38 KB), one
band at a time, pushing each band to the panel. The painter takes the band's vertical offset and lets the canvas
clip, so the drawing code is written for the whole screen. The scene is repainted only when it differs from the last
one shown.

## Consequences

- No flicker, modest memory.
- A full repaint costs about 90 ms of SPI time, fine for a screen that changes at most a few times per second.
- Animations (smooth transitions) would need partial updates or PSRAM; out of scope for a status display.
