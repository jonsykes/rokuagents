#pragma once

#include <cstddef>
#include <cstdint>

#include <LovyanGFX.hpp>

#include "managents/core/text_fit.hpp"

namespace managents::ui {

/// Bold sans fonts for the status label, largest first.
extern const lgfx::IFont* const kStatusFonts[4];
/// Fonts for card names, largest first. Names stay modest so long folder names
/// fit whole (the status is the big text), and bold, because thin strokes wash
/// out on the coloured cards of a TN panel.
extern const lgfx::IFont* const kNameFonts[2];

/// core::TextMeasure for a family of LovyanGFX fonts, measured on `canvas`
/// (which is left with the last measured font selected).
class LgfxTextMeasure : public core::TextMeasure {
public:
    template <std::size_t Count>
    LgfxTextMeasure(lgfx::LovyanGFX& canvas, const lgfx::IFont* const (&fonts)[Count])
        : canvas_(canvas), fonts_(fonts), count_(Count) {}

    std::size_t fontCount() const override { return count_; }
    std::int32_t width(std::size_t font, const char* text) const override;
    std::int32_t height(std::size_t font) const override;

    const lgfx::IFont* font(std::size_t index) const { return fonts_[index]; }

private:
    lgfx::LovyanGFX& canvas_;
    const lgfx::IFont* const* fonts_;
    std::size_t count_;
};

/// Pixels the font draws above the baseline.
std::int32_t ascentOf(const lgfx::IFont* font);

}  // namespace managents::ui
