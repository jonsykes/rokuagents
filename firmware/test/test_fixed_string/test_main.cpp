#include <unity.h>

#include <string>

#include "managents/core/fixed_string.hpp"

using namespace managents::core;

namespace {

void keeps_text_that_fits() {
    const FixedString<8> text("agents");
    TEST_ASSERT_EQUAL_STRING("agents", text.c_str());
    TEST_ASSERT_EQUAL(6, text.size());
    TEST_ASSERT_FALSE(text.empty());
}

void fills_the_capacity_exactly() {
    TEST_ASSERT_EQUAL_STRING("12345678", FixedString<8>("12345678").c_str());
    TEST_ASSERT_EQUAL_STRING("12345678", FixedString<8>("123456789").c_str());
    // A multi-byte character that ends exactly at the capacity is kept whole.
    TEST_ASSERT_EQUAL_STRING("1234\xF0\x9F\x98\x80", FixedString<8>("1234\xF0\x9F\x98\x80").c_str());
}

void copies_only_the_given_length() {
    FixedString<8> text;
    text.assign("managents", 3);
    TEST_ASSERT_EQUAL_STRING("man", text.c_str());
    TEST_ASSERT_EQUAL(3, text.size());
}

void truncates_inside_multibyte_characters_on_a_boundary() {
    // Every character width after 0-3 ASCII bytes, so the cut at the capacity
    // lands on the first, second, third and fourth byte of a character in turn.
    const std::string characters[] = {"\xC3\xA9", "\xE6\x97\xA5", "\xF0\x9F\x98\x80"};  // é, 日, 😀
    for (const std::string& character : characters) {
        for (std::size_t prefix = 0; prefix < 4; ++prefix) {
            std::string text(prefix, 'a');
            while (text.size() <= 64) {
                text += character;
            }
            const FixedString<64> stored(text.c_str());

            const std::size_t width = character.size();
            const std::size_t whole = prefix + (64 - prefix) / width * width;  // whole characters that fit
            const std::string label =
                std::to_string(width) + "-byte characters after " + std::to_string(prefix) + " ASCII bytes";
            TEST_ASSERT_EQUAL_MESSAGE(whole, stored.size(), label.c_str());
            TEST_ASSERT_EQUAL_STRING_MESSAGE(text.substr(0, whole).c_str(), stored.c_str(), label.c_str());
        }
    }
}

void treats_null_as_empty() {
    FixedString<8> text("agents");
    text.assign(nullptr);
    TEST_ASSERT_TRUE(text.empty());
    TEST_ASSERT_EQUAL_STRING("", text.c_str());

    text.assign("agents");
    text.assign(nullptr, 4);
    TEST_ASSERT_TRUE(text.empty());
    TEST_ASSERT_TRUE(FixedString<8>(nullptr).empty());
}

void compares_by_content() {
    const FixedString<8> text("abc");
    TEST_ASSERT_TRUE(text == FixedString<8>("abc"));
    TEST_ASSERT_TRUE(text != FixedString<8>("abd"));
    TEST_ASSERT_TRUE(text != FixedString<8>("ab"));
    TEST_ASSERT_TRUE(text == "abc");
    TEST_ASSERT_FALSE(text == "abcd");
    TEST_ASSERT_FALSE(text == nullptr);
}

}  // namespace

void setUp() {}
void tearDown() {}

int main() {
    UNITY_BEGIN();
    RUN_TEST(keeps_text_that_fits);
    RUN_TEST(fills_the_capacity_exactly);
    RUN_TEST(copies_only_the_given_length);
    RUN_TEST(truncates_inside_multibyte_characters_on_a_boundary);
    RUN_TEST(treats_null_as_empty);
    RUN_TEST(compares_by_content);
    return UNITY_END();
}
