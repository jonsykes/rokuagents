#include <unity.h>

#include <cstdint>
#include <cstring>

#include "managents/core/format.hpp"

using namespace managents::core;

namespace {

void formats_ages_compactly() {
    struct Case {
        std::uint32_t seconds;
        const char* expected;
    };
    const Case cases[] = {
        {0, "0s"},
        {59, "59s"},
        {60, "1m"},
        {3599, "59m"},
        {3600 + 12 * 60 + 5, "1h 12m"},
        {2 * 3600 + 30, "2h"},
        {86399, "23h 59m"},
        {86400 + 2 * 3600 + 59 * 60, "1d 2h"},
        {3 * 86400 + 59 * 60, "3d"},
        {0xFFFFFFFF, "49710d 6h"},  // a saturated age still fits the card's buffer
    };
    for (const Case& c : cases) {
        char text[12];
        formatAge(c.seconds, text, sizeof text);
        TEST_ASSERT_EQUAL_STRING(c.expected, text);
    }
}

void formats_clock_and_wraps_days() {
    char text[8];
    formatClock(0, text, sizeof text);
    TEST_ASSERT_EQUAL_STRING("00:00", text);
    formatClock(86400 * 3 + 13 * 3600 + 7 * 60 + 59, text, sizeof text);
    TEST_ASSERT_EQUAL_STRING("13:07", text);
    formatClock(-60, text, sizeof text);
    TEST_ASSERT_EQUAL_STRING("23:59", text);
}

void computes_clamped_percentages() {
    TEST_ASSERT_EQUAL_UINT8(44, percentOf(88000, 200000));
    TEST_ASSERT_EQUAL_UINT8(100, percentOf(300, 200));
    TEST_ASSERT_EQUAL_UINT8(0, percentOf(5, 0));
}

void folds_names_to_drawable_ascii() {
    struct Case {
        const char* text;
        const char* expected;
    };
    // Octal escapes, because a hex escape would swallow the letters after it.
    const Case cases[] = {
        {"plain-ascii_1.0 #2", "plain-ascii_1.0 #2"},
        {"caf\303\251-m\303\274ller", "cafe-muller"},                     // café-müller
        {"\303\234bung-Stra\303\237e-\303\246on", "Ubung-Strasse-aeon"},  // Übung-Straße-æon
        {"caf\303\251-\303\251-\346\227\245\346\234\254", "cafe-e-??"},   // café-é-日本 (the fixture)
        {"rocket-\360\237\232\200", "rocket-?"},                          // an emoji
        {"tab\there", "tab?here"},                                        // a control character
        {"a\200b\303", "a?b?"},                                           // stray and truncated bytes
        {"\346\227-x", "?-x"},                                            // a cut 3-byte sequence
        {"\300\257", "??"},                                               // an overlong '/'
        {"\340\203\200", "?"},                                            // an overlong 'A' with grave
        {"", ""},
    };
    for (const Case& c : cases) {
        char out[64];
        foldToAscii(c.text, out, sizeof out);
        TEST_ASSERT_EQUAL_STRING_MESSAGE(c.expected, out, c.text);
        TEST_ASSERT_TRUE(std::strlen(out) <= std::strlen(c.text));
    }
}

void fold_never_overruns_the_output() {
    char out[4] = {'x', 'x', 'x', 'x'};
    foldToAscii("abcdef", out, sizeof out);
    TEST_ASSERT_EQUAL_STRING("abc", out);

    foldToAscii("ab\303\237", out, 3);  // "ss" does not fit after "ab": dropped whole
    TEST_ASSERT_EQUAL_STRING("ab", out);

    foldToAscii("abc", out, 1);
    TEST_ASSERT_EQUAL_STRING("", out);
}

}  // namespace

void setUp() {}
void tearDown() {}

int main() {
    UNITY_BEGIN();
    RUN_TEST(formats_ages_compactly);
    RUN_TEST(formats_clock_and_wraps_days);
    RUN_TEST(computes_clamped_percentages);
    RUN_TEST(folds_names_to_drawable_ascii);
    RUN_TEST(fold_never_overruns_the_output);
    return UNITY_END();
}
