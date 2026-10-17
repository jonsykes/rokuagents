#include "ui/text.hpp"

namespace managents::ui {

const lgfx::IFont* const kStatusFonts[4] = {
    &fonts::FreeSansBold24pt7b,
    &fonts::FreeSansBold18pt7b,
    &fonts::FreeSansBold12pt7b,
    &fonts::FreeSansBold9pt7b,
};

const lgfx::IFont* const kNameFonts[2] = {
    &fonts::FreeSansBold12pt7b,
    &fonts::FreeSansBold9pt7b,
};

std::int32_t LgfxTextMeasure::width(std::size_t font, const char* text) const {
    canvas_.setFont(fonts_[font]);
    return canvas_.textWidth(text);
}

std::int32_t LgfxTextMeasure::height(std::size_t font) const {
    canvas_.setFont(fonts_[font]);
    return canvas_.fontHeight();
}

std::int32_t ascentOf(const lgfx::IFont* font) {
    lgfx::FontMetrics metrics{};
    font->getDefaultMetric(&metrics);
    return metrics.baseline;
}

}  // namespace managents::ui
