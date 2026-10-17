#include <unity.h>

#include <cstdio>

#include "managents/core/grid_layout.hpp"
#include "managents/core/model.hpp"

using namespace managents::core;

namespace {

const Rect kLandscape{6, 28, 468, 286};
const Rect kPortrait{6, 28, 308, 446};
constexpr std::int16_t kGap = 6;

void assertShape(std::size_t count, const Rect& area, int columns, int rows) {
    const GridShape shape = chooseGridShape(count, area, kGap);
    char message[48];
    std::snprintf(message, sizeof message, "count=%zu", count);
    TEST_ASSERT_EQUAL_MESSAGE(columns, shape.columns, message);
    TEST_ASSERT_EQUAL_MESSAGE(rows, shape.rows, message);
}

void picks_landscape_grids() {
    assertShape(1, kLandscape, 1, 1);
    assertShape(2, kLandscape, 2, 1);
    assertShape(4, kLandscape, 2, 2);
    assertShape(6, kLandscape, 3, 2);
    assertShape(9, kLandscape, 3, 3);
    assertShape(12, kLandscape, 4, 3);
}

void picks_portrait_grids() {
    assertShape(1, kPortrait, 1, 1);
    assertShape(2, kPortrait, 1, 2);
    assertShape(6, kPortrait, 2, 3);
}

void never_leaves_a_whole_column_empty() {
    for (std::size_t count = 1; count <= kMaxAgents; ++count) {
        const GridShape shape = chooseGridShape(count, kLandscape, kGap);
        TEST_ASSERT_TRUE(shape.columns * shape.rows >= count);
        TEST_ASSERT_TRUE((shape.columns - 1u) * shape.rows < count);
        TEST_ASSERT_TRUE(shape.columns * (shape.rows - 1u) < count);
    }
}

void tiles_the_area_exactly() {
    for (std::size_t count = 1; count <= kMaxAgents; ++count) {
        Rect cards[kMaxAgents];
        layoutGrid(count, kLandscape, kGap, cards);
        const GridShape shape = chooseGridShape(count, kLandscape, kGap);
        const Rect& first = cards[0];
        TEST_ASSERT_EQUAL(kLandscape.x, first.x);
        TEST_ASSERT_EQUAL(kLandscape.y, first.y);
        const Rect& lastOfFirstRow = cards[shape.columns - 1];
        TEST_ASSERT_EQUAL(kLandscape.x + kLandscape.w, lastOfFirstRow.x + lastOfFirstRow.w);
        const Rect& last = cards[count - 1];
        TEST_ASSERT_EQUAL(kLandscape.y + kLandscape.h, last.y + last.h);
    }
}

void lays_out_rows_left_to_right() {
    Rect cards[4];
    layoutGrid(4, kLandscape, kGap, cards);
    TEST_ASSERT_TRUE(cards[1].x > cards[0].x);
    TEST_ASSERT_EQUAL(cards[0].y, cards[1].y);
    TEST_ASSERT_EQUAL(cards[0].x, cards[2].x);
    TEST_ASSERT_TRUE(cards[2].y > cards[0].y);
    TEST_ASSERT_EQUAL(cards[0].w + kGap, cards[1].x - cards[0].x);
}

}  // namespace

void setUp() {}
void tearDown() {}

int main() {
    UNITY_BEGIN();
    RUN_TEST(picks_landscape_grids);
    RUN_TEST(picks_portrait_grids);
    RUN_TEST(never_leaves_a_whole_column_empty);
    RUN_TEST(tiles_the_area_exactly);
    RUN_TEST(lays_out_rows_left_to_right);
    return UNITY_END();
}
