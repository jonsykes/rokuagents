#include "managents/core/scene.hpp"

#include <cstdio>

#include "managents/core/format.hpp"

namespace managents::core {
namespace {

ContextView buildContextView(const ContextUsage& usage) {
    ContextView view;
    view.visible = usage.known && usage.limit > 0;
    view.percent = view.visible ? percentOf(usage.used, usage.limit) : 0;
    return view;
}

CardView buildCard(const Agent& agent, const Rect& bounds, std::uint32_t elapsedSeconds, bool blinkOn) {
    CardView card;
    card.bounds = bounds;
    card.kind = agent.kind;
    card.status = agent.status;
    card.alertPhase = blinks(agent, elapsedSeconds) && !blinkOn;
    char name[decltype(card.name)::capacity() + 1];
    foldToAscii(agent.name.c_str(), name, sizeof name);  // the fonts only draw ASCII
    card.name.assign(name);
    char age[decltype(card.age)::capacity() + 1];
    formatAge(ageAt(agent, elapsedSeconds), age, sizeof age);
    card.age.assign(age);
    card.context = buildContextView(agent.context);
    return card;
}

HeaderView buildHeader(const HostState& host, std::uint32_t elapsedSeconds, const ScreenGeometry& geometry) {
    HeaderView header;
    header.bounds = {0, 0, geometry.width, geometry.headerHeight};
    char text[8];
    formatClock(host.hostEpochSeconds + host.utcOffsetSeconds + elapsedSeconds, text, sizeof text);
    header.clock.assign(text);
    if (host.moreCount > 0) {
        std::snprintf(text, sizeof text, "+%u", static_cast<unsigned>(host.moreCount));
        header.overflowBadge.assign(text);
    }
    header.pageCount = static_cast<std::uint8_t>(Pager::pageCount(host.agentCount));
    for (std::size_t page = 0; page < header.pageCount; ++page) {
        const std::size_t first = page * Pager::kPerPage;
        const std::size_t remaining = host.agentCount - first;
        header.pageAttention[page] =
            summarize(host.agents + first, remaining < Pager::kPerPage ? remaining : Pager::kPerPage);
    }
    return header;
}

Rect gridArea(const ScreenGeometry& geometry) {
    const std::int16_t top = geometry.headerHeight;
    return {geometry.margin, top, static_cast<std::int16_t>(geometry.width - 2 * geometry.margin),
            static_cast<std::int16_t>(geometry.height - top - geometry.margin)};
}

SceneKind linkScene(LinkState link) {
    switch (link) {
        case LinkState::SetupNeeded:
            return SceneKind::SetupNeeded;
        case LinkState::Lost:
            return SceneKind::Reconnecting;
        case LinkState::Connecting:
            break;
    }
    return SceneKind::Connecting;
}

}  // namespace

Scene buildScene(const SceneInput& input, const ScreenGeometry& geometry) {
    Scene scene;
    if (input.host == nullptr) {
        scene.kind = linkScene(input.link);
        scene.header.bounds = {0, 0, geometry.width, geometry.headerHeight};
        return scene;
    }

    const HostState& host = *input.host;
    const std::uint32_t elapsedSeconds = input.msSinceFrame / 1000;
    scene.header = buildHeader(host, elapsedSeconds, geometry);
    if (host.agentCount == 0) {
        scene.kind = SceneKind::NoAgents;
        return scene;
    }

    const std::size_t pages = scene.header.pageCount;
    const std::size_t page = input.page < pages ? input.page : pages - 1;
    const std::size_t first = page * Pager::kPerPage;
    const std::size_t remaining = host.agentCount - first;
    const std::size_t count = remaining < Pager::kPerPage ? remaining : Pager::kPerPage;
    scene.header.page = static_cast<std::uint8_t>(page);

    scene.kind = SceneKind::Agents;
    scene.cardCount = static_cast<std::uint8_t>(count);
    Rect bounds[Pager::kPerPage];
    layoutGrid(count, gridArea(geometry), geometry.gap, bounds);
    for (std::size_t i = 0; i < count; ++i) {
        scene.cards[i] = buildCard(host.agents[first + i], bounds[i], elapsedSeconds, input.blinkOn);
    }
    return scene;
}

std::uint32_t ageAt(const Agent& agent, std::uint32_t elapsedSeconds) {
    const std::uint64_t age = static_cast<std::uint64_t>(agent.ageSeconds) + elapsedSeconds;
    return age > UINT32_MAX ? UINT32_MAX : static_cast<std::uint32_t>(age);
}

bool blinks(const Agent& agent, std::uint32_t elapsedSeconds) {
    return agent.status == AgentStatus::Error && ageAt(agent, elapsedSeconds) < kBlinkWindowSeconds;
}

}  // namespace managents::core
