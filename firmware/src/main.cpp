// managents firmware — composition root.
//
// Wires the board adapters to the hardware-independent Application and pumps
// the serial port. All behaviour lives in lib/core and is unit-tested there.

#include <Arduino.h>

#include "adapters/lgfx_display.hpp"
#include "adapters/rgb_led_indicator.hpp"
#include "adapters/serial_host_link.hpp"
#include "adapters/touch_sensor.hpp"
#include "board/e32r40t.hpp"
#include "config.hpp"
#include "managents/core/application.hpp"

// Frames of up to 24 agents are decoded on the stack; give the loop task room.
SET_LOOP_TASK_STACK_SIZE(16 * 1024);

namespace {

using namespace managents;

board::Panel panel;
adapters::LgfxDisplay display(panel, {config::kSetupUrl, config::kFirmwareVersion},
                              {config::kBacklight, config::kBacklightDimmed});
adapters::RgbLedIndicator indicator(board::pins::kLedRed, board::pins::kLedGreen, board::pins::kLedBlue,
                                    config::kLedLevel);
adapters::SerialHostLink hostLink(Serial);
adapters::TouchSensor touch(panel);

// Created in setup(), once the panel knows its rotated size.
core::Application* application = nullptr;

void pumpSerial() {
    char buffer[256];
    while (Serial.available() > 0) {
        const std::size_t length = Serial.readBytes(buffer, sizeof buffer);
        application->onReceive(buffer, length, millis());
    }
}

}  // namespace

void setup() {
    Serial.setRxBufferSize(config::kSerialRxBuffer);
    Serial.begin(config::kSerialBaud);
    Serial.setTimeout(0);

    panel.init();
    panel.setRotation(config::kRotation);
    display.begin();
    indicator.begin();

    const auto width = static_cast<std::int16_t>(panel.width());
    const auto height = static_cast<std::int16_t>(panel.height());
    const core::DeviceInfo device{config::kDeviceName, config::kFirmwareVersion, board::kBoardId,
                                  static_cast<std::uint16_t>(width), static_cast<std::uint16_t>(height)};
    const core::Pager pager(config::kTouch ? 0 : config::kPageIntervalMs);
    static core::Application app(display, indicator, hostLink, device, core::screenGeometry(width, height), pager);
    application = &app;
    application->begin(millis());

    // Reboot if loop() ever stalls: Arduino's loop task feeds the 5 s task watchdog
    // before each pass, and the slowest pass (a full repaint) takes about 90 ms.
    enableLoopWDT();
}

void loop() {
    pumpSerial();
    if constexpr (config::kTouch) {
        application->onTouch(touch.pressed(), millis());
    }
    application->tick(millis());
    delay(10);
}
