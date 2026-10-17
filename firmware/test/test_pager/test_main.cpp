#include <unity.h>

#include "managents/core/pager.hpp"

using namespace managents::core;

namespace {

void counts_pages() {
    TEST_ASSERT_EQUAL(1, Pager::pageCount(0));
    TEST_ASSERT_EQUAL(1, Pager::pageCount(9));
    TEST_ASSERT_EQUAL(2, Pager::pageCount(10));
    TEST_ASSERT_EQUAL(3, Pager::pageCount(24));
}

void next_wraps_around() {
    Pager pager;
    pager.next(20, 0);
    TEST_ASSERT_EQUAL(1, pager.page());
    TEST_ASSERT_EQUAL(9, pager.firstItem());
    pager.next(20, 10);
    pager.next(20, 20);
    TEST_ASSERT_EQUAL(0, pager.page());
}

void single_page_stays_put() {
    Pager pager;
    pager.next(5, 0);
    TEST_ASSERT_EQUAL(0, pager.page());
}

void clamps_when_the_list_shrinks() {
    Pager pager;
    pager.next(20, 0);
    pager.next(20, 0);
    pager.update(12, 10);
    TEST_ASSERT_EQUAL(1, pager.page());
}

void returns_to_the_first_page_when_left_alone() {
    Pager pager;
    pager.next(20, 1000);
    pager.update(20, 1000 + Pager::kReturnToFirstMs - 1);
    TEST_ASSERT_EQUAL(1, pager.page());
    pager.update(20, 1000 + Pager::kReturnToFirstMs);
    TEST_ASSERT_EQUAL(0, pager.page());
}

void auto_advances_without_touch() {
    Pager pager(10000);
    pager.update(20, 9999);
    TEST_ASSERT_EQUAL(0, pager.page());
    pager.update(20, 10000);
    TEST_ASSERT_EQUAL(1, pager.page());
    pager.update(20, 20000);
    TEST_ASSERT_EQUAL(2, pager.page());
}

void auto_advance_waits_a_full_interval_after_a_second_page_appears() {
    Pager pager(10000);
    for (std::uint32_t t = 0; t <= 60000; t += 100) {
        pager.update(5, t);  // a minute with a single page
    }
    pager.update(10, 60000);  // a tenth agent opens page 2
    TEST_ASSERT_EQUAL(0, pager.page());
    pager.update(10, 69999);
    TEST_ASSERT_EQUAL(0, pager.page());
    pager.update(10, 70000);
    TEST_ASSERT_EQUAL(1, pager.page());
}

void taps_once_per_press() {
    TapDetector tap;
    TEST_ASSERT_TRUE(tap.update(true, 0));
    TEST_ASSERT_FALSE(tap.update(true, 10));
    TEST_ASSERT_FALSE(tap.update(false, 20));
    TEST_ASSERT_FALSE(tap.update(true, 50));  // bounce: released too briefly
    TEST_ASSERT_FALSE(tap.update(false, 60));
    TEST_ASSERT_FALSE(tap.update(false, 60 + TapDetector::kReleaseMs));
    TEST_ASSERT_TRUE(tap.update(true, 400));
}

}  // namespace

void setUp() {}
void tearDown() {}

int main() {
    UNITY_BEGIN();
    RUN_TEST(counts_pages);
    RUN_TEST(next_wraps_around);
    RUN_TEST(single_page_stays_put);
    RUN_TEST(clamps_when_the_list_shrinks);
    RUN_TEST(returns_to_the_first_page_when_left_alone);
    RUN_TEST(auto_advances_without_touch);
    RUN_TEST(auto_advance_waits_a_full_interval_after_a_second_page_appears);
    RUN_TEST(taps_once_per_press);
    return UNITY_END();
}
