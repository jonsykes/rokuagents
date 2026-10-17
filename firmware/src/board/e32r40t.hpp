#pragma once

// Board support for the LCDWiki 4.0" ESP32-32E display (E32R40T with resistive
// touch, E32N40T without). Pin map from the vendor manual, see docs/hardware.md.

#include <LovyanGFX.hpp>

namespace managents::board {

namespace pins {
// LCD (ST7796S) and touch (XPT2046) share one SPI bus.
constexpr int kSpiSclk = 14;
constexpr int kSpiMosi = 13;
constexpr int kSpiMiso = 12;
constexpr int kLcdCs = 15;
constexpr int kLcdDc = 2;
constexpr int kLcdBacklight = 27;
constexpr int kTouchCs = 33;
constexpr int kTouchIrq = 36;

// RGB status LED, common anode: a pin driven low lights its colour.
constexpr int kLedRed = 22;
constexpr int kLedGreen = 16;
constexpr int kLedBlue = 17;

// Reserved for later features (docs/roadmap.md).
constexpr int kAudioEnable = 4;  // active low
constexpr int kAudioDac = 26;
constexpr int kBatteryAdc = 34;  // through a 100k/100k divider
}  // namespace pins

constexpr const char* kBoardId = "e32r40t";
constexpr int kPanelWidth = 320;
constexpr int kPanelHeight = 480;

/// LovyanGFX device for the ST7796S panel and its backlight.
class Panel : public lgfx::LGFX_Device {
public:
    Panel() {
        configureBus();
        configurePanel();
        configureBacklight();
        configureTouch();
        setPanel(&panel_);
    }

private:
    void configureBus() {
        auto config = bus_.config();
        config.spi_host = HSPI_HOST;
        config.spi_mode = 0;
        config.freq_write = 27000000;  // conservative: the shared, unterminated bus garbled frames at 40 MHz
        config.freq_read = 16000000;
        config.spi_3wire = false;
        config.use_lock = true;
        config.dma_channel = SPI_DMA_CH_AUTO;
        config.pin_sclk = pins::kSpiSclk;
        config.pin_mosi = pins::kSpiMosi;
        config.pin_miso = pins::kSpiMiso;
        config.pin_dc = pins::kLcdDc;
        bus_.config(config);
        panel_.setBus(&bus_);
    }

    void configurePanel() {
        auto config = panel_.config();
        config.pin_cs = pins::kLcdCs;
        config.pin_rst = -1;  // tied to EN, reset together with the ESP32
        config.pin_busy = -1;
        config.panel_width = kPanelWidth;
        config.panel_height = kPanelHeight;
        config.memory_width = kPanelWidth;
        config.memory_height = kPanelHeight;
        config.offset_x = 0;
        config.offset_y = 0;
        config.offset_rotation = 0;
        config.readable = true;
        config.invert = false;
        config.rgb_order = false;
        config.dlen_16bit = false;
        config.bus_shared = true;  // the touch controller sits on the same bus
        panel_.config(config);
    }

    void configureBacklight() {
        auto config = light_.config();
        config.pin_bl = pins::kLcdBacklight;
        config.invert = false;
        config.freq = 12000;
        config.pwm_channel = 7;
        light_.config(config);
        panel_.setLight(&light_);
    }

    // Configuring the touch controller also keeps its chip select high, which the
    // shared bus needs even while touch input is unused. Raw 12-bit range below is
    // the XPT2046 default; per-unit calibration is a roadmap item.
    void configureTouch() {
        auto config = touch_.config();
        config.spi_host = HSPI_HOST;
        config.freq = 1000000;
        config.pin_sclk = pins::kSpiSclk;
        config.pin_mosi = pins::kSpiMosi;
        config.pin_miso = pins::kSpiMiso;
        config.pin_cs = pins::kTouchCs;
        config.pin_int = pins::kTouchIrq;
        config.bus_shared = true;
        config.offset_rotation = 0;
        config.x_min = 300;
        config.x_max = 3900;
        config.y_min = 200;
        config.y_max = 3800;
        touch_.config(config);
        panel_.setTouch(&touch_);
    }

    lgfx::Bus_SPI bus_;
    lgfx::Panel_ST7796 panel_;
    lgfx::Light_PWM light_;
    lgfx::Touch_XPT2046 touch_;
};

}  // namespace managents::board
