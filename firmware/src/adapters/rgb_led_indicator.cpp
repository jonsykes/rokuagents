#include "adapters/rgb_led_indicator.hpp"

#include <Arduino.h>

namespace managents::adapters {
namespace {

constexpr std::uint32_t kPwmFrequency = 5000;
constexpr std::uint8_t kPwmBits = 8;
constexpr std::uint8_t kFirstChannel = 0;  // channel 7 belongs to the backlight

}  // namespace

RgbLedIndicator::RgbLedIndicator(int redPin, int greenPin, int bluePin, std::uint8_t level)
    : pins_{redPin, greenPin, bluePin}, level_(level) {}

void RgbLedIndicator::begin() {
    for (std::uint8_t i = 0; i < 3; ++i) {
        ledcSetup(kFirstChannel + i, kPwmFrequency, kPwmBits);
        ledcAttachPin(pins_[i], kFirstChannel + i);
    }
    write({0, 0, 0});
}

void RgbLedIndicator::show(core::Attention attention, bool blinkOn) {
    switch (attention) {
        case core::Attention::Error:
            write(blinkOn ? Rgb{255, 0, 0} : Rgb{0, 0, 0});
            return;
        case core::Attention::Waiting:
            write({255, 110, 0});
            return;
        case core::Attention::Working:
            write({0, 255, 0});
            return;
        case core::Attention::Disconnected:
            write({0, 0, 60});
            return;
        case core::Attention::Quiet:
            break;
    }
    write({0, 0, 0});
}

void RgbLedIndicator::write(const Rgb& color) {
    if (color == current_) {
        return;
    }
    const std::uint8_t values[3] = {color.red, color.green, color.blue};
    for (std::uint8_t i = 0; i < 3; ++i) {
        // Common anode: 255 duty keeps the pin high, i.e. the colour off.
        ledcWrite(kFirstChannel + i, 255 - scale(values[i]));
    }
    current_ = color;
}

std::uint8_t RgbLedIndicator::scale(std::uint8_t value) const {
    return static_cast<std::uint8_t>(static_cast<std::uint16_t>(value) * level_ / 255);
}

}  // namespace managents::adapters
