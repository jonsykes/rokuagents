#include "managents/core/protocol.hpp"

#include <ArduinoJson.h>

#include <cstring>

namespace managents::core {
namespace {

bool parseKind(const char* text, AgentKind& kind) {
    if (text == nullptr) {
        return false;
    }
    if (std::strcmp(text, "claude") == 0) {
        kind = AgentKind::Claude;
    } else if (std::strcmp(text, "opencode") == 0) {
        kind = AgentKind::OpenCode;
    } else if (std::strcmp(text, "codex") == 0) {
        kind = AgentKind::Codex;
    } else {
        kind = AgentKind::Unknown;
    }
    return true;
}

bool parseStatus(const char* text, AgentStatus& status) {
    if (text == nullptr) {
        return false;
    }
    struct Entry {
        const char* name;
        AgentStatus status;
    };
    static constexpr Entry kStatuses[] = {
        {"working", AgentStatus::Working},
        {"waiting", AgentStatus::Waiting},
        {"error", AgentStatus::Error},
        {"idle", AgentStatus::Idle},
    };
    for (const Entry& entry : kStatuses) {
        if (std::strcmp(text, entry.name) == 0) {
            status = entry.status;
            return true;
        }
    }
    return false;
}

/// An integer of at least `minimum`, like the schema's "minimum" keyword.
bool isIntegerAtLeast(JsonVariantConst value, std::int64_t minimum) {
    return value.is<std::int64_t>() && value.as<std::int64_t>() >= minimum;
}

/// Reads a non-negative integer, saturating at the largest 32-bit value:
/// `as<std::uint32_t>()` would turn anything above it into 0.
std::uint32_t saturatedU32(JsonVariantConst value) {
    const std::int64_t number = value.as<std::int64_t>();
    return number > INT64_C(0xFFFFFFFF) ? UINT32_MAX : static_cast<std::uint32_t>(number);
}

bool parseContext(JsonVariantConst value, ContextUsage& context) {
    context = ContextUsage{};
    if (value.isNull()) {
        return true;
    }
    if (!value.is<JsonObjectConst>() || !isIntegerAtLeast(value["used"], 0)) {
        return false;
    }
    JsonVariantConst limit = value["limit"];
    if (!limit.isNull() && !isIntegerAtLeast(limit, 1)) {
        return false;
    }
    context.known = true;
    context.used = saturatedU32(value["used"]);
    context.limit = limit.isNull() ? 0 : saturatedU32(limit);
    return true;
}

bool parseAgent(JsonVariantConst value, Agent& agent) {
    if (!value.is<JsonObjectConst>()) {
        return false;
    }
    const char* id = value["id"];
    const char* name = value["name"];
    if (id == nullptr || *id == '\0' || name == nullptr || !isIntegerAtLeast(value["age"], 0)) {
        return false;
    }
    if (!parseKind(value["kind"], agent.kind) || !parseStatus(value["status"], agent.status)) {
        return false;
    }
    agent.id.assign(id);
    agent.name.assign(name);
    agent.ageSeconds = saturatedU32(value["age"]);
    return parseContext(value["ctx"], agent.context);
}

MessageType decodeState(JsonVariantConst root, HostState& state) {
    JsonVariantConst agents = root["agents"];
    if (!root["now"].is<std::int64_t>() || !agents.is<JsonArrayConst>()) {
        return MessageType::Invalid;
    }
    JsonVariantConst tz = root["tz"];
    JsonVariantConst more = root["more"];
    if ((!tz.isNull() && !tz.is<std::int32_t>()) || (!more.isNull() && !isIntegerAtLeast(more, 0))) {
        return MessageType::Invalid;
    }

    // Decode into a scratch copy first: a frame is applied entirely or not at all.
    HostState decoded;
    decoded.hostEpochSeconds = root["now"].as<std::int64_t>();
    decoded.utcOffsetSeconds = tz.isNull() ? 0 : tz.as<std::int32_t>();
    std::size_t overflow = 0;
    for (JsonVariantConst item : agents.as<JsonArrayConst>()) {
        if (decoded.agentCount == kMaxAgents) {
            ++overflow;
            continue;
        }
        if (!parseAgent(item, decoded.agents[decoded.agentCount])) {
            return MessageType::Invalid;
        }
        ++decoded.agentCount;
    }
    // 64 bits: `more` may not fit in size_t, which is 32 bits wide on the ESP32.
    const std::uint64_t reportedMore = more.isNull() ? 0 : more.as<std::uint64_t>();
    const std::uint64_t totalMore = reportedMore + overflow;
    decoded.moreCount = static_cast<std::uint16_t>(totalMore > 0xFFFF ? 0xFFFF : totalMore);

    state = decoded;
    return MessageType::State;
}

}  // namespace

MessageType decodeHostMessage(const char* line, std::size_t length, HostState& state) {
    JsonDocument document;
    if (deserializeJson(document, line, length) != DeserializationError::Ok) {
        return MessageType::Invalid;
    }
    JsonVariantConst root = document.as<JsonVariantConst>();
    if (!root.is<JsonObjectConst>() || !root["v"].is<int>() || !root["t"].is<const char*>()) {
        return MessageType::Invalid;
    }
    if (root["v"].as<int>() != kProtocolVersion) {
        return MessageType::Ignored;
    }

    const char* type = root["t"];
    if (std::strcmp(type, "state") == 0) {
        return decodeState(root, state);
    }
    if (std::strcmp(type, "hello") == 0) {
        return MessageType::Hello;
    }
    return MessageType::Ignored;
}

std::size_t encodeHello(const DeviceInfo& info, char* out, std::size_t capacity) {
    JsonDocument document;
    document["v"] = kProtocolVersion;
    document["t"] = "hello";
    document["device"] = info.device;
    document["fw"] = info.firmwareVersion;
    document["board"] = info.board;
    document["w"] = info.width;
    document["h"] = info.height;
    document["proto"] = kProtocolVersion;

    if (measureJson(document) >= capacity) {
        return 0;
    }
    return serializeJson(document, out, capacity);
}

}  // namespace managents::core
