// Headless LovyanGFX platform for the host renderer (env:sim). NOT the SDL library.
//
// LovyanGFX picks its desktop back end by probing for <SDL2/SDL.h>. This file is
// found first on the include path and supplies the few platform hooks the
// library needs to draw into memory (sprites), so the real ScenePainter runs on
// macOS and Linux with no SDL, display or framebuffer. It deliberately does not
// define SDL_h_, which keeps LovyanGFX's SDL window and event code compiled out.
// The hooks are implemented in ../platform.cpp.

#pragma once

#include <cstdint>
#include <cstdlib>

namespace lgfx {
inline namespace v1 {

unsigned long millis();
unsigned long micros();
void delay(unsigned long milliseconds);
void delayMicroseconds(unsigned int microseconds);

static inline void* heap_alloc(std::size_t length) {
    return std::malloc(length);
}
static inline void* heap_alloc_psram(std::size_t length) {
    return std::malloc(length);
}
static inline void* heap_alloc_dma(std::size_t length) {
    return std::malloc(length);
}
static inline void heap_free(void* buffer) {
    std::free(buffer);
}
static inline bool heap_capable_dma(const void*) {
    return false;
}

enum pin_mode_t { output, input, input_pullup, input_pulldown };

void gpio_hi(std::uint32_t pin);
void gpio_lo(std::uint32_t pin);
bool gpio_in(std::uint32_t pin);
void pinMode(std::int_fast16_t pin, pin_mode_t mode);
void lgfxPinMode(std::int_fast16_t pin, pin_mode_t mode);

}  // namespace v1
}  // namespace lgfx
