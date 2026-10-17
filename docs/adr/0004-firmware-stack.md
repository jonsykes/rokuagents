# 4. PlatformIO, Arduino and LovyanGFX, with a host-tested core

Date: 2026-10-09 · Status: Accepted

## Context

The ESP32 can be programmed with ESP-IDF or Arduino. The vendor ships TFT_eSPI examples that require editing the
library's own files to configure it. We want reproducible builds, unit tests and a board definition in our code.

## Decision

- PlatformIO pins the platform and library versions in `platformio.ini`; builds and tests run the same locally and in CI.
- Arduino-ESP32 for the runtime, LovyanGFX for the panel: it supports the ST7796S and the XPT2046 on a shared bus and
  is configured by a C++ class in our repository (`src/board/e32r40t.hpp`).
- All behaviour lives in `lib/core`, which uses neither Arduino nor LovyanGFX and runs under `pio test -e native`.
  Hardware is reached only through ports (`Display`, `StatusIndicator`, `HostLink`).
- C++17 (the Arduino-ESP32 2.x toolchain is GCC 8), no exceptions or heap use in the model (`FixedString`, fixed arrays).

## Consequences

- Most of the firmware is tested on any computer without the board.
- Swapping the display library or the board touches only `src/`.
