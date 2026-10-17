#include <unity.h>

#include <cstring>
#include <string>
#include <vector>

#include "managents/core/line_assembler.hpp"

using managents::core::LineAssembler;

namespace {

std::vector<std::string> feed(LineAssembler& assembler, const std::string& bytes) {
    std::vector<std::string> lines;
    for (char byte : bytes) {
        if (assembler.push(byte)) {
            lines.emplace_back(assembler.line(), assembler.length());
        }
    }
    return lines;
}

void splits_on_newline() {
    LineAssembler assembler;
    const auto lines = feed(assembler, "first\nsecond\n");
    TEST_ASSERT_EQUAL(2, lines.size());
    TEST_ASSERT_EQUAL_STRING("first", lines[0].c_str());
    TEST_ASSERT_EQUAL_STRING("second", lines[1].c_str());
}

void strips_carriage_return() {
    LineAssembler assembler;
    const auto lines = feed(assembler, "crlf\r\n");
    TEST_ASSERT_EQUAL(1, lines.size());
    TEST_ASSERT_EQUAL_STRING("crlf", lines[0].c_str());
}

void holds_partial_line_until_newline() {
    LineAssembler assembler;
    TEST_ASSERT_EQUAL(0, feed(assembler, "par").size());
    const auto lines = feed(assembler, "tial\n");
    TEST_ASSERT_EQUAL(1, lines.size());
    TEST_ASSERT_EQUAL_STRING("partial", lines[0].c_str());
}

void discards_oversize_line_and_resynchronises() {
    LineAssembler assembler;
    const std::string oversize(LineAssembler::kMaxLineLength + 10, 'x');
    const auto lines = feed(assembler, oversize + "\nnext\n");
    TEST_ASSERT_EQUAL(1, lines.size());
    TEST_ASSERT_EQUAL_STRING("next", lines[0].c_str());
}

void accepts_line_of_exactly_max_length() {
    LineAssembler assembler;
    const std::string exact(LineAssembler::kMaxLineLength, 'y');
    const auto lines = feed(assembler, exact + "\n");
    TEST_ASSERT_EQUAL(1, lines.size());
    TEST_ASSERT_EQUAL(LineAssembler::kMaxLineLength, lines[0].size());
}

void reports_empty_lines() {
    LineAssembler assembler;
    const auto lines = feed(assembler, "\n\n");
    TEST_ASSERT_EQUAL(2, lines.size());
    TEST_ASSERT_EQUAL(0, lines[0].size());
}

}  // namespace

void setUp() {}
void tearDown() {}

int main() {
    UNITY_BEGIN();
    RUN_TEST(splits_on_newline);
    RUN_TEST(strips_carriage_return);
    RUN_TEST(holds_partial_line_until_newline);
    RUN_TEST(discards_oversize_line_and_resynchronises);
    RUN_TEST(accepts_line_of_exactly_max_length);
    RUN_TEST(reports_empty_lines);
    return UNITY_END();
}
