#include <unity.h>

#include <cstdio>
#include <initializer_list>

#include "managents/core/scene.hpp"

using namespace managents::core;

namespace {

constexpr ScreenGeometry kGeometry = screenGeometry(480, 320);

Agent makeAgent(const char* id, const char* name, AgentStatus status, std::uint32_t age) {
    Agent agent;
    agent.id.assign(id);
    agent.name.assign(name);
    agent.kind = AgentKind::Claude;
    agent.status = status;
    agent.ageSeconds = age;
    return agent;
}

HostState hostWith(std::initializer_list<Agent> agents) {
    HostState host;
    host.hostEpochSeconds = 12 * 3600 + 30 * 60;  // 12:30 UTC
    host.utcOffsetSeconds = 2 * 3600;
    for (const Agent& agent : agents) {
        host.agents[host.agentCount++] = agent;
    }
    return host;
}

void shows_the_link_state_without_host() {
    struct Case {
        LinkState link;
        SceneKind kind;
    };
    const Case cases[] = {
        {LinkState::Connecting, SceneKind::Connecting},
        {LinkState::SetupNeeded, SceneKind::SetupNeeded},
        {LinkState::Lost, SceneKind::Reconnecting},
    };
    for (const Case& c : cases) {
        SceneInput input;
        input.link = c.link;
        const Scene scene = buildScene(input, kGeometry);
        TEST_ASSERT_EQUAL(c.kind, scene.kind);
        TEST_ASSERT_EQUAL_UINT8(0, scene.cardCount);
    }
}

void shows_empty_state_with_clock() {
    const HostState host = hostWith({});
    const Scene scene = buildScene({&host, 0, true}, kGeometry);
    TEST_ASSERT_EQUAL(SceneKind::NoAgents, scene.kind);
    TEST_ASSERT_EQUAL_STRING("14:30", scene.header.clock.c_str());
}

void builds_one_card_per_agent_in_order() {
    const HostState host = hostWith(
        {makeAgent("c:1", "first", AgentStatus::Working, 5), makeAgent("c:2", "second", AgentStatus::Waiting, 120)});
    const Scene scene = buildScene({&host, 0, true}, kGeometry);
    TEST_ASSERT_EQUAL(SceneKind::Agents, scene.kind);
    TEST_ASSERT_EQUAL_UINT8(2, scene.cardCount);
    TEST_ASSERT_EQUAL_STRING("first", scene.cards[0].name.c_str());
    TEST_ASSERT_EQUAL_STRING("5s", scene.cards[0].age.c_str());
    TEST_ASSERT_EQUAL_STRING("2m", scene.cards[1].age.c_str());
    TEST_ASSERT_TRUE(scene.cards[0].bounds.y >= kGeometry.headerHeight);
}

void advances_ages_and_clock_between_frames() {
    const HostState host = hostWith({makeAgent("c:1", "a", AgentStatus::Working, 58)});
    const Scene scene = buildScene({&host, 61500, true}, kGeometry);
    TEST_ASSERT_EQUAL_STRING("1m", scene.cards[0].age.c_str());
    TEST_ASSERT_EQUAL_STRING("14:31", scene.header.clock.c_str());
}

void blinks_only_error_cards() {
    const HostState host =
        hostWith({makeAgent("c:1", "ok", AgentStatus::Working, 1), makeAgent("c:2", "bad", AgentStatus::Error, 1)});
    const Scene bright = buildScene({&host, 0, true}, kGeometry);
    const Scene dark = buildScene({&host, 0, false}, kGeometry);
    TEST_ASSERT_FALSE(bright.cards[1].alertPhase);
    TEST_ASSERT_TRUE(dark.cards[1].alertPhase);
    TEST_ASSERT_FALSE(dark.cards[0].alertPhase);
    TEST_ASSERT_TRUE(bright != dark);
}

void stops_blinking_after_the_first_minute() {
    const HostState host = hostWith({makeAgent("c:1", "bad", AgentStatus::Error, 50)});
    TEST_ASSERT_TRUE(buildScene({&host, 9999, false}, kGeometry).cards[0].alertPhase);    // 59 s old
    TEST_ASSERT_FALSE(buildScene({&host, 10000, false}, kGeometry).cards[0].alertPhase);  // 60 s old: steady
}

void saturates_ages_instead_of_wrapping() {
    const HostState host = hostWith({makeAgent("c:1", "old", AgentStatus::Idle, 0xFFFFFFFF)});
    TEST_ASSERT_EQUAL_STRING("49710d 6h", buildScene({&host, 5000, true}, kGeometry).cards[0].age.c_str());
}

void is_stable_when_nothing_visible_changes() {
    const HostState host = hostWith({makeAgent("c:1", "a", AgentStatus::Working, 1)});
    TEST_ASSERT_TRUE(buildScene({&host, 100, true}, kGeometry) == buildScene({&host, 900, false}, kGeometry));
}

void shows_overflow_badge_and_context_bar() {
    HostState host =
        hostWith({makeAgent("c:1", "a", AgentStatus::Working, 1), makeAgent("c:2", "b", AgentStatus::Working, 1)});
    host.moreCount = 3;
    host.agents[0].context = {true, 150000, 200000};
    host.agents[1].context = {true, 150000, 0};  // limit unknown: no bar
    const Scene scene = buildScene({&host, 0, true}, kGeometry);
    TEST_ASSERT_EQUAL_STRING("+3", scene.header.overflowBadge.c_str());
    TEST_ASSERT_TRUE(scene.cards[0].context.visible);
    TEST_ASSERT_EQUAL_UINT8(75, scene.cards[0].context.percent);
    TEST_ASSERT_FALSE(scene.cards[1].context.visible);
}

void folds_names_to_what_the_fonts_can_draw() {
    const HostState host = hostWith({makeAgent("c:1", "caf\303\251-\346\227\245", AgentStatus::Working, 1)});
    TEST_ASSERT_EQUAL_STRING("cafe-?", buildScene({&host, 0, true}, kGeometry).cards[0].name.c_str());
}

HostState hostWithAgents(std::size_t count) {
    HostState host = hostWith({});
    for (std::size_t i = 0; i < count; ++i) {
        char id[24];
        std::snprintf(id, sizeof id, "c:%zu", i);
        host.agents[host.agentCount++] = makeAgent(id, id, AgentStatus::Waiting, 1);
    }
    return host;
}

void shows_nine_cards_per_page() {
    const HostState host = hostWithAgents(11);
    SceneInput input{&host, 0, true, 0};
    const Scene first = buildScene(input, kGeometry);
    TEST_ASSERT_EQUAL_UINT8(9, first.cardCount);
    TEST_ASSERT_EQUAL_UINT8(0, first.header.page);
    TEST_ASSERT_EQUAL_UINT8(2, first.header.pageCount);
    TEST_ASSERT_EQUAL_STRING("c:0", first.cards[0].name.c_str());

    input.page = 1;
    const Scene second = buildScene(input, kGeometry);
    TEST_ASSERT_EQUAL_UINT8(2, second.cardCount);
    TEST_ASSERT_EQUAL_STRING("c:9", second.cards[0].name.c_str());
    TEST_ASSERT_TRUE(second.cards[0].bounds.w > first.cards[0].bounds.w);  // re-laid out for 2

    input.page = 7;  // stale page index after the list shrank
    TEST_ASSERT_EQUAL_UINT8(1, buildScene(input, kGeometry).header.page);
}

void marks_pages_that_need_attention() {
    HostState host = hostWithAgents(20);  // all waiting
    for (std::size_t i = 0; i < 9; ++i) {
        host.agents[i].status = AgentStatus::Working;
    }
    host.agents[0].status = AgentStatus::Idle;
    host.agents[19].status = AgentStatus::Error;
    const Scene scene = buildScene({&host, 0, true, 0}, kGeometry);
    TEST_ASSERT_EQUAL_UINT8(3, scene.header.pageCount);
    TEST_ASSERT_EQUAL(Attention::Working, scene.header.pageAttention[0]);
    TEST_ASSERT_EQUAL(Attention::Waiting, scene.header.pageAttention[1]);
    TEST_ASSERT_EQUAL(Attention::Error, scene.header.pageAttention[2]);

    host.agents[19].status = AgentStatus::Waiting;  // off screen, but the dots change
    TEST_ASSERT_TRUE(scene != buildScene({&host, 0, true, 0}, kGeometry));
}

}  // namespace

void setUp() {}
void tearDown() {}

int main() {
    UNITY_BEGIN();
    RUN_TEST(shows_the_link_state_without_host);
    RUN_TEST(shows_empty_state_with_clock);
    RUN_TEST(builds_one_card_per_agent_in_order);
    RUN_TEST(advances_ages_and_clock_between_frames);
    RUN_TEST(blinks_only_error_cards);
    RUN_TEST(stops_blinking_after_the_first_minute);
    RUN_TEST(saturates_ages_instead_of_wrapping);
    RUN_TEST(is_stable_when_nothing_visible_changes);
    RUN_TEST(shows_overflow_badge_and_context_bar);
    RUN_TEST(folds_names_to_what_the_fonts_can_draw);
    RUN_TEST(shows_nine_cards_per_page);
    RUN_TEST(marks_pages_that_need_attention);
    return UNITY_END();
}
