#include <unity.h>

#include <cstring>

#include "managents/core/text_fit.hpp"

using namespace managents::core;

namespace {

/// Monospace fonts: font 0 is 12 px per character and 23 px tall, font 1 is
/// 9 px per character and 18 px tall (about FreeSansBold12pt and 9pt).
class FakeMeasure : public TextMeasure {
public:
    static constexpr std::int32_t kAdvance[] = {12, 9};
    static constexpr std::int32_t kHeight[] = {23, 18};

    std::size_t fontCount() const override { return 2; }
    std::int32_t width(std::size_t font, const char* text) const override {
        return kAdvance[font] * static_cast<std::int32_t>(std::strlen(text));
    }
    std::int32_t height(std::size_t font) const override { return kHeight[font]; }
};

const FakeMeasure kMeasure;

void assertFitted(const FittedName& name, std::size_t font, const char* first, const char* second) {
    TEST_ASSERT_EQUAL_UINT(font, name.font);
    TEST_ASSERT_EQUAL_UINT(second == nullptr ? 1 : 2, name.lineCount);
    TEST_ASSERT_EQUAL_STRING(first, name.lines[0]);
    if (second != nullptr) {
        TEST_ASSERT_EQUAL_STRING(second, name.lines[1]);
    }
}

void picks_the_largest_font_that_fits() {
    TEST_ASSERT_EQUAL_UINT(0, largestFitting(kMeasure, "WORKING", 84, 23));
    TEST_ASSERT_EQUAL_UINT(1, largestFitting(kMeasure, "WORKING", 83, 23));  // too wide for font 0
    TEST_ASSERT_EQUAL_UINT(1, largestFitting(kMeasure, "WORKING", 84, 22));  // too tall for font 0
    TEST_ASSERT_EQUAL_UINT(1, largestFitting(kMeasure, "WORKING", 10, 10));  // nothing fits: the smallest
}

void keeps_a_short_name_on_one_line_in_the_largest_font() {
    assertFitted(fitName(kMeasure, "web", 100, 46), 0, "web", nullptr);
    assertFitted(fitName(kMeasure, "", 100, 46), 0, "", nullptr);
}

void prefers_a_smaller_font_to_a_second_line() {
    assertFitted(fitName(kMeasure, "alpha-gamma", 100, 46), 1, "alpha-gamma", nullptr);
}

void wraps_in_the_largest_font_that_holds_both_lines() {
    assertFitted(fitName(kMeasure, "alpha-bravo", 80, 46), 0, "alpha-", "bravo");
    // Font 0 is too wide for the second line, font 1 is not.
    assertFitted(fitName(kMeasure, "alpha-beta-gamma-delta", 100, 46), 1, "alpha-beta-", "gamma-delta");
    // Two lines of font 0 are too tall.
    assertFitted(fitName(kMeasure, "alpha-bravo", 80, 45), 1, "alpha-", "bravo");
}

void breaks_after_the_last_separator_that_lets_the_rest_fit() {
    assertFitted(fitName(kMeasure, "infra-terraform", 124, 46), 0, "infra-", "terraform");
    assertFitted(fitName(kMeasure, "web-app-frontend", 100, 46), 0, "web-app-", "frontend");
}

void breaks_anywhere_when_no_separator_helps() {
    assertFitted(fitName(kMeasure, "a-b-cdefghijklmnop", 100, 36), 1, "a-b-cdefghi", "jklmnop");
}

void shortens_the_second_line_in_the_middle_as_a_last_resort() {
    assertFitted(fitName(kMeasure, "first-part-second-very-long-tail #2", 100, 36), 1, "first-part-", "seco...l #2");
}

void shortens_a_single_line_when_two_do_not_fit() {
    assertFitted(fitName(kMeasure, "tonylook/infra #2", 144, 20), 1, "tonylo...nfra #2", nullptr);
}

void ellipsizes_in_the_middle_keeping_the_end() {
    char out[FittedName::kLineCapacity];
    ellipsizeMiddle(kMeasure, 1, "tonylook/infra #2", 153, out, sizeof out);
    TEST_ASSERT_EQUAL_STRING("tonylook/infra #2", out);  // fits: unchanged
    ellipsizeMiddle(kMeasure, 1, "tonylook/infra #2", 144, out, sizeof out);
    TEST_ASSERT_EQUAL_STRING("tonylo...nfra #2", out);
    ellipsizeMiddle(kMeasure, 1, "abcdefgh", 9, out, sizeof out);
    TEST_ASSERT_EQUAL_STRING("...", out);  // not even one character on each side fits
}

void ellipsizing_never_overruns_the_output() {
    char out[8];
    ellipsizeMiddle(kMeasure, 1, "abcdefghijkl", 1000, out, sizeof out);
    TEST_ASSERT_EQUAL_STRING("ab...kl", out);  // fits the width, not the buffer
}

}  // namespace

void setUp() {}
void tearDown() {}

int main() {
    UNITY_BEGIN();
    RUN_TEST(picks_the_largest_font_that_fits);
    RUN_TEST(keeps_a_short_name_on_one_line_in_the_largest_font);
    RUN_TEST(prefers_a_smaller_font_to_a_second_line);
    RUN_TEST(wraps_in_the_largest_font_that_holds_both_lines);
    RUN_TEST(breaks_after_the_last_separator_that_lets_the_rest_fit);
    RUN_TEST(breaks_anywhere_when_no_separator_helps);
    RUN_TEST(shortens_the_second_line_in_the_middle_as_a_last_resort);
    RUN_TEST(shortens_a_single_line_when_two_do_not_fit);
    RUN_TEST(ellipsizes_in_the_middle_keeping_the_end);
    RUN_TEST(ellipsizing_never_overruns_the_output);
    return UNITY_END();
}
