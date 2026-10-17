#pragma once

#include <cstdint>

#include "managents/core/model.hpp"
#include "managents/core/scene.hpp"

namespace managents::core {

// Ports: the boundaries between the hardware-independent core and the board.
// Adapters in firmware/src implement them; tests implement them with fakes.

enum class Brightness : std::uint8_t { Full, Dimmed };

/// Paints a scene on the panel and drives its backlight.
class Display {
public:
    virtual ~Display() = default;
    virtual void present(const Scene& scene) = 0;
    /// Called on every tick; adapters touch the backlight only when it changes.
    virtual void setBrightness(Brightness brightness) = 0;
};

/// A secondary indicator (the board's RGB LED).
class StatusIndicator {
public:
    virtual ~StatusIndicator() = default;
    /// Called on every tick. `blinkOn` is the blink phase for an Error that
    /// should blink; it stays true once the error is past its first minute.
    virtual void show(Attention attention, bool blinkOn) = 0;
};

/// Writes protocol lines back to the host.
class HostLink {
public:
    virtual ~HostLink() = default;
    /// `line` is a complete message without the trailing newline.
    virtual void sendLine(const char* line) = 0;
};

}  // namespace managents::core
