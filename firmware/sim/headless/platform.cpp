// Platform hooks for the headless LovyanGFX build (see SDL2/SDL.h).
//
// The renderer only draws into sprites. Time is real; the pin and bus functions
// exist because LovyanGFX's panel and touch drivers are compiled in and refer to
// them, but nothing calls them on the host: they do nothing and report failure.

#include <chrono>
#include <cstdio>  // glibc's <chrono> declares FILE, which turns on LovyanGFX's file reader: it needs fopen
#include <thread>

#include <LovyanGFX.hpp>

namespace lgfx {
inline namespace v1 {
namespace {

const auto kStart = std::chrono::steady_clock::now();

template <typename Unit>
unsigned long elapsed() {
    return static_cast<unsigned long>(
        std::chrono::duration_cast<Unit>(std::chrono::steady_clock::now() - kStart).count());
}

auto unsupported() {
    return cpp::fail(error_t::periph_device_err);
}

}  // namespace

unsigned long millis() {
    return elapsed<std::chrono::milliseconds>();
}
unsigned long micros() {
    return elapsed<std::chrono::microseconds>();
}
void delay(unsigned long milliseconds) {
    std::this_thread::sleep_for(std::chrono::milliseconds(milliseconds));
}
void delayMicroseconds(unsigned int microseconds) {
    std::this_thread::sleep_for(std::chrono::microseconds(microseconds));
}

void gpio_hi(std::uint32_t) {}
void gpio_lo(std::uint32_t) {}
bool gpio_in(std::uint32_t) {
    return false;
}
void pinMode(std::int_fast16_t, pin_mode_t) {}
void lgfxPinMode(std::int_fast16_t, pin_mode_t) {}

namespace spi {
cpp::result<void, error_t> init(int, int, int, int) {
    return unsupported();
}
void release(int) {}
void beginTransaction(int, std::uint32_t, int) {}
void endTransaction(int) {}
void writeBytes(int, const std::uint8_t*, std::size_t) {}
void readBytes(int, std::uint8_t*, std::size_t) {}
}  // namespace spi

namespace i2c {
cpp::result<void, error_t> init(int, int, int) {
    return unsupported();
}
cpp::result<void, error_t> release(int) {
    return unsupported();
}
cpp::result<void, error_t> restart(int, int, std::uint32_t, bool) {
    return unsupported();
}
cpp::result<void, error_t> beginTransaction(int, int, std::uint32_t, bool) {
    return unsupported();
}
cpp::result<void, error_t> endTransaction(int) {
    return unsupported();
}
cpp::result<void, error_t> writeBytes(int, const std::uint8_t*, std::size_t) {
    return unsupported();
}
cpp::result<void, error_t> readBytes(int, std::uint8_t*, std::size_t, bool) {
    return unsupported();
}
cpp::result<void, error_t> transactionWrite(int, int, const std::uint8_t*, std::uint8_t, std::uint32_t) {
    return unsupported();
}
cpp::result<void, error_t> transactionRead(int, int, std::uint8_t*, std::uint8_t, std::uint32_t) {
    return unsupported();
}
cpp::result<void, error_t> transactionWriteRead(int, int, const std::uint8_t*, std::uint8_t, std::uint8_t*, std::size_t,
                                                std::uint32_t) {
    return unsupported();
}
cpp::result<std::uint8_t, error_t> readRegister8(int, int, std::uint8_t, std::uint32_t) {
    return unsupported();
}
cpp::result<void, error_t> writeRegister8(int, int, std::uint8_t, std::uint8_t, std::uint8_t, std::uint32_t) {
    return unsupported();
}
}  // namespace i2c

}  // namespace v1
}  // namespace lgfx
