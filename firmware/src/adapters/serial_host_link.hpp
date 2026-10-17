#pragma once

#include <Arduino.h>

#include "managents/core/ports.hpp"

namespace managents::adapters {

/// core::HostLink over the USB serial port (the board's CH340C bridge).
class SerialHostLink : public core::HostLink {
public:
    explicit SerialHostLink(HardwareSerial& serial) : serial_(serial) {}

    void sendLine(const char* line) override {
        serial_.print(line);
        serial_.print('\n');
    }

private:
    HardwareSerial& serial_;
};

}  // namespace managents::adapters
