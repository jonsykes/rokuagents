#pragma once

#include <cstddef>
#include <cstdint>

#include "managents/core/fixed_string.hpp"

namespace managents::core {

/// Maximum number of agents a single `state` frame may carry (see docs/protocol.md).
inline constexpr std::size_t kMaxAgents = 24;

enum class AgentKind : std::uint8_t { Unknown, Claude, OpenCode, Codex };

/// The UI indexes per-status tables in this order.
enum class AgentStatus : std::uint8_t { Working, Waiting, Error, Idle };

/// Context-window usage reported by the host. `limit == 0` means "limit unknown".
struct ContextUsage {
    bool known = false;
    std::uint32_t used = 0;
    std::uint32_t limit = 0;
};

struct Agent {
    FixedString<48> id;
    FixedString<64> name;
    AgentKind kind = AgentKind::Unknown;
    AgentStatus status = AgentStatus::Waiting;
    std::uint32_t ageSeconds = 0;
    ContextUsage context;
};

/// The last valid `state` frame received from the host.
struct HostState {
    std::int64_t hostEpochSeconds = 0;
    std::int32_t utcOffsetSeconds = 0;
    Agent agents[kMaxAgents];
    std::uint8_t agentCount = 0;
    std::uint16_t moreCount = 0;
};

/// What a group of agents needs from the user, most urgent first.
/// Disconnected stands for "no host", never for an agent.
enum class Attention : std::uint8_t { Error, Waiting, Working, Quiet, Disconnected };

inline Attention attentionOf(AgentStatus status) {
    switch (status) {
        case AgentStatus::Error:
            return Attention::Error;
        case AgentStatus::Waiting:
            return Attention::Waiting;
        case AgentStatus::Working:
            return Attention::Working;
        case AgentStatus::Idle:
            break;
    }
    return Attention::Quiet;
}

/// The most urgent attention among `count` agents; Quiet for none.
inline Attention summarize(const Agent* agents, std::size_t count) {
    Attention most = Attention::Quiet;
    for (std::size_t i = 0; i < count; ++i) {
        const Attention attention = attentionOf(agents[i].status);
        most = attention < most ? attention : most;
    }
    return most;
}

inline Attention summarize(const HostState& state) {
    return summarize(state.agents, state.agentCount);
}

}  // namespace managents::core
