#pragma once

#include <cstddef>
#include <cstdint>

// Build-time settings. Override any of them from platformio.ini with -D flags.

#ifndef MANAGENTS_FW_VERSION
#define MANAGENTS_FW_VERSION "0.0.0-dev"
#endif

/// Panel rotation (LovyanGFX convention). 1 and 3 are the two landscape
/// orientations, 0 and 2 the portrait ones; the layout adapts to any of them.
/// 1 = landscape with the USB-C port on the right (what the enclosure expects).
#ifndef MANAGENTS_ROTATION
#define MANAGENTS_ROTATION 1
#endif

/// Backlight level, 0..255.
#ifndef MANAGENTS_BACKLIGHT
#define MANAGENTS_BACKLIGHT 200
#endif

/// Backlight level, 0..255, while nobody needs the screen: after a minute of a
/// lost link (the computer is asleep), or five minutes of no or only idle agents.
#ifndef MANAGENTS_BACKLIGHT_DIM
#define MANAGENTS_BACKLIGHT_DIM 40
#endif

/// RGB LED level, 0..255. The LED sits on the back of the board; keep it soft.
#ifndef MANAGENTS_LED_LEVEL
#define MANAGENTS_LED_LEVEL 40
#endif

/// 1 on boards with a touch panel (E32R40T): a tap turns the page.
/// 0 (E32N40T): pages turn by themselves every MANAGENTS_PAGE_INTERVAL_MS.
#ifndef MANAGENTS_TOUCH
#define MANAGENTS_TOUCH 1
#endif

#ifndef MANAGENTS_PAGE_INTERVAL_MS
#define MANAGENTS_PAGE_INTERVAL_MS 10000
#endif

static_assert(MANAGENTS_ROTATION >= 0 && MANAGENTS_ROTATION <= 3, "MANAGENTS_ROTATION must be 0..3");
static_assert(MANAGENTS_BACKLIGHT >= 0 && MANAGENTS_BACKLIGHT <= 255, "MANAGENTS_BACKLIGHT must be 0..255");
static_assert(MANAGENTS_BACKLIGHT_DIM >= 0 && MANAGENTS_BACKLIGHT_DIM <= MANAGENTS_BACKLIGHT,
              "MANAGENTS_BACKLIGHT_DIM must be 0..MANAGENTS_BACKLIGHT");
static_assert(MANAGENTS_LED_LEVEL >= 0 && MANAGENTS_LED_LEVEL <= 255, "MANAGENTS_LED_LEVEL must be 0..255");
static_assert(MANAGENTS_PAGE_INTERVAL_MS > 0, "MANAGENTS_PAGE_INTERVAL_MS must be positive");

namespace managents::config {

constexpr const char* kDeviceName = "managents";
constexpr const char* kFirmwareVersion = MANAGENTS_FW_VERSION;
/// Where the setup screen's QR code leads: the setup steps in the README.
constexpr const char* kSetupUrl = "https://github.com/tonylook/managents#get-started";
constexpr std::uint8_t kRotation = MANAGENTS_ROTATION;
constexpr std::uint8_t kBacklight = MANAGENTS_BACKLIGHT;
constexpr std::uint8_t kBacklightDimmed = MANAGENTS_BACKLIGHT_DIM;
constexpr std::uint8_t kLedLevel = MANAGENTS_LED_LEVEL;
constexpr bool kTouch = MANAGENTS_TOUCH != 0;
constexpr std::uint32_t kPageIntervalMs = MANAGENTS_PAGE_INTERVAL_MS;

constexpr std::uint32_t kSerialBaud = 115200;
/// Big enough for a whole maximum-size line while a frame is being drawn.
constexpr std::size_t kSerialRxBuffer = 8192;

}  // namespace managents::config
