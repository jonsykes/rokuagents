#pragma once

#include <cstdint>

#include <LovyanGFX.hpp>

#include "managents/core/ports.hpp"
#include "ui/scene_painter.hpp"

namespace managents::adapters {

/// Backlight levels, 0..255, for core::Brightness.
struct BacklightLevels {
    std::uint8_t full;
    std::uint8_t dimmed;
};

/// core::Display on a LovyanGFX target: the panel on the board, a full-screen
/// sprite in the host renderer. Each scene is painted into a strip sprite one
/// band at a time and pushed to the target: no flicker, and no full-screen
/// buffer (the ESP32-WROOM-32E has no PSRAM).
class LgfxDisplay : public core::Display {
public:
    static constexpr std::int32_t kStripHeight = 40;

    /// A target without a backlight (the host renderer's sprite).
    LgfxDisplay(lgfx::LovyanGFX& target, const ui::LinkScreenText& text);
    /// The board's panel, whose backlight follows setBrightness().
    LgfxDisplay(lgfx::LGFX_Device& panel, const ui::LinkScreenText& text, BacklightLevels backlight);

    /// Allocates the strip buffer and turns the backlight on. Without the
    /// buffer, scenes are drawn straight to the target.
    void begin();

    void present(const core::Scene& scene) override;
    void setBrightness(core::Brightness brightness) override;

private:
    lgfx::LovyanGFX& target_;
    lgfx::LGFX_Sprite strip_;
    ui::LinkScreenText text_;
    bool buffered_ = false;

    lgfx::LGFX_Device* panel_ = nullptr;
    BacklightLevels backlight_{};
    std::uint8_t level_ = 0;
};

}  // namespace managents::adapters
