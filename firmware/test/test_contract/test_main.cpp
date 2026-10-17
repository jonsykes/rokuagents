// Contract tests: the firmware must accept every line in protocol/fixtures/valid
// and reject every line in protocol/fixtures/invalid — the same files the helper
// validates against the JSON Schema.

#include <unity.h>

#include <dirent.h>

#include <fstream>
#include <string>
#include <vector>

#include "managents/core/protocol.hpp"

using namespace managents::core;

namespace {

// `pio test` runs from the firmware directory.
const std::string kFixtures = "../protocol/fixtures/";

void appendLines(const std::string& path, std::vector<std::string>& lines) {
    std::ifstream file(kFixtures + path);
    TEST_ASSERT_TRUE_MESSAGE(file.is_open(), path.c_str());
    for (std::string line; std::getline(file, line);) {
        if (!line.empty()) {
            lines.push_back(line);
        }
    }
}

std::vector<std::string> linesIn(const std::string& directory) {
    std::vector<std::string> lines;
    DIR* dir = opendir((kFixtures + directory).c_str());
    TEST_ASSERT_NOT_NULL_MESSAGE(dir, "fixture directory not found");
    while (const dirent* entry = readdir(dir)) {
        const std::string name = entry->d_name;
        if (name.size() >= 6 && name.substr(name.size() - 6) == ".jsonl") {
            appendLines(directory + "/" + name, lines);
        }
    }
    closedir(dir);
    TEST_ASSERT_FALSE_MESSAGE(lines.empty(), "no fixture lines");
    return lines;
}

void accepts_valid_fixtures() {
    for (const std::string& line : linesIn("valid")) {
        HostState state;
        const MessageType type = decodeHostMessage(line.c_str(), line.size(), state);
        TEST_ASSERT_TRUE_MESSAGE(type == MessageType::State || type == MessageType::Hello, line.c_str());
    }
}

void rejects_invalid_fixtures_and_keeps_the_last_frame() {
    // A frame is applied entirely or not at all: no rejected line may leave a
    // trace of its partly decoded agents in the last good state.
    std::vector<std::string> frame;
    appendLines("valid/mixed-four.jsonl", frame);
    HostState state;
    TEST_ASSERT_EQUAL(MessageType::State, decodeHostMessage(frame[0].c_str(), frame[0].size(), state));
    const HostState lastGood = state;

    for (const std::string& line : linesIn("invalid")) {
        TEST_ASSERT_EQUAL_MESSAGE(MessageType::Invalid, decodeHostMessage(line.c_str(), line.size(), state),
                                  line.c_str());
        TEST_ASSERT_EQUAL_UINT8_MESSAGE(lastGood.agentCount, state.agentCount, line.c_str());
        for (std::uint8_t i = 0; i < lastGood.agentCount; ++i) {
            TEST_ASSERT_EQUAL_STRING_MESSAGE(lastGood.agents[i].name.c_str(), state.agents[i].name.c_str(),
                                             line.c_str());
        }
    }
}

}  // namespace

void setUp() {}
void tearDown() {}

int main() {
    UNITY_BEGIN();
    RUN_TEST(accepts_valid_fixtures);
    RUN_TEST(rejects_invalid_fixtures_and_keeps_the_last_frame);
    return UNITY_END();
}
