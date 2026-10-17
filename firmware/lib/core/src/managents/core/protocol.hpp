#pragma once

#include <cstddef>
#include <cstdint>

#include "managents/core/model.hpp"

namespace managents::core {

/// Version of the wire protocol implemented by this firmware (docs/protocol.md).
inline constexpr int kProtocolVersion = 1;

enum class MessageType : std::uint8_t {
    Invalid,  ///< Not JSON, wrong shape, or failed validation. Must be ignored.
    Ignored,  ///< Well-formed but not for us (unknown `t`, other protocol version).
    State,    ///< A `state` frame; the output HostState was replaced.
    Hello,    ///< A `hello` probe from the host.
};

/// Identity reported to the host in our `hello` message.
struct DeviceInfo {
    const char* device;
    const char* firmwareVersion;
    const char* board;
    std::uint16_t width;
    std::uint16_t height;
};

/// Decodes one host->device line. `state` is only written when the result is
/// MessageType::State, so a rejected frame never corrupts the last good one.
MessageType decodeHostMessage(const char* line, std::size_t length, HostState& state);

/// Writes our `hello` message (without the trailing newline) into `out`.
/// Returns the number of characters written, or 0 if `capacity` is too small.
std::size_t encodeHello(const DeviceInfo& info, char* out, std::size_t capacity);

}  // namespace managents::core
