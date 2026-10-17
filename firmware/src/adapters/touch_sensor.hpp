#pragma once

#include <LovyanGFX.hpp>

namespace managents::adapters {

/// Reports whether the resistive panel is pressed anywhere. Position is not
/// needed (a tap anywhere turns the page), so no calibration is required.
class TouchSensor {
public:
    explicit TouchSensor(lgfx::LGFX_Device& panel) : panel_(panel) {}

    bool pressed() {
        lgfx::touch_point_t point;
        return panel_.getTouchRaw(&point, 1) > 0;
    }

private:
    lgfx::LGFX_Device& panel_;
};

}  // namespace managents::adapters
