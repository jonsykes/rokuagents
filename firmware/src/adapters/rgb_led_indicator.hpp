#pragma once

#include <cstdint>

#include "managents/core/ports.hpp"

namespace managents::adapters {

/// core::StatusIndicator on the board's common-anode RGB LED (PWM, inverted).
/// The LED is on the back of the board: it cannot be seen from the front, and
/// in a closed case perhaps not at all. The screen is the primary signal.
///   Error: red, blinking during the error's first minute · Waiting: amber ·
///   Working: green · Quiet: off · Disconnected: dim blue.
class RgbLedIndicator : public core::StatusIndicator {
public:
    RgbLedIndicator(int redPin, int greenPin, int bluePin, std::uint8_t level);

    void begin();
    void show(core::Attention attention, bool blinkOn) override;

private:
    struct Rgb {
        std::uint8_t red, green, blue;
        bool operator==(const Rgb& other) const {
            return red == other.red && green == other.green && blue == other.blue;
        }
    };

    void write(const Rgb& color);
    std::uint8_t scale(std::uint8_t value) const;

    int pins_[3];
    std::uint8_t level_;
    Rgb current_{1, 1, 1};  // forces the first write
};

}  // namespace managents::adapters
