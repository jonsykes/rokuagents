#include "adapters/lgfx_display.hpp"

namespace managents::adapters {

LgfxDisplay::LgfxDisplay(lgfx::LovyanGFX& target, const ui::LinkScreenText& text)
    : target_(target), strip_(&target), text_(text) {}

LgfxDisplay::LgfxDisplay(lgfx::LGFX_Device& panel, const ui::LinkScreenText& text, BacklightLevels backlight)
    : target_(panel), strip_(&panel), text_(text), panel_(&panel), backlight_(backlight) {}

void LgfxDisplay::begin() {
    strip_.setColorDepth(16);
    buffered_ = strip_.createSprite(target_.width(), kStripHeight) != nullptr;
    if (panel_ != nullptr) {
        level_ = backlight_.full;
        panel_->setBrightness(level_);
    }
}

void LgfxDisplay::present(const core::Scene& scene) {
    const std::int32_t width = target_.width();
    const std::int32_t height = target_.height();

    if (!buffered_) {
        ui::ScenePainter(target_, 0, width, height, text_).paint(scene);
        return;
    }

    target_.startWrite();
    for (std::int32_t top = 0; top < height; top += kStripHeight) {
        ui::ScenePainter(strip_, top, width, height, text_).paint(scene);
        strip_.pushSprite(0, top);
        target_.waitDMA();  // the strip buffer is reused for the next band
    }
    target_.endWrite();
}

void LgfxDisplay::setBrightness(core::Brightness brightness) {
    const std::uint8_t level = brightness == core::Brightness::Dimmed ? backlight_.dimmed : backlight_.full;
    if (panel_ != nullptr && level != level_) {
        panel_->setBrightness(level);
        level_ = level;
    }
}

}  // namespace managents::adapters
