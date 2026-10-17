// Robustness test: mutated protocol fixtures, delivered the way a serial port
// does (in arbitrary chunks, between touches and ticks, across the millis()
// wrap), must never crash the application or break what the screen relies on.
// The generator has a fixed seed, so every run feeds the same bytes and a
// failure always reproduces.

#include <unity.h>

#include <dirent.h>

#include <algorithm>
#include <cstdint>
#include <fstream>
#include <string>
#include <vector>

#include "managents/core/application.hpp"

using namespace managents::core;

namespace {

// `pio test` runs from the firmware directory.
const std::string kFixtures = "../protocol/fixtures/";

constexpr int kIterations = 20000;

/// Marsaglia's xorshift32: small, fast and deterministic.
class Random {
public:
    /// A value in [0, bound).
    std::size_t below(std::size_t bound) {
        state_ ^= state_ << 13;
        state_ ^= state_ >> 17;
        state_ ^= state_ << 5;
        return state_ % bound;
    }

private:
    std::uint32_t state_ = 2463534242u;
};

class CheckingDisplay : public Display {
public:
    void present(const Scene& scene) override {
        TEST_ASSERT_LESS_OR_EQUAL_UINT8(Pager::kPerPage, scene.cardCount);
        TEST_ASSERT_LESS_THAN_UINT8(scene.header.pageCount, scene.header.page);
        ++presented;
    }
    void setBrightness(Brightness) override {}
    std::size_t presented = 0;
};

class NullIndicator : public StatusIndicator {
public:
    void show(Attention, bool) override {}
};

class NullLink : public HostLink {
public:
    void sendLine(const char*) override {}
};

const DeviceInfo kDevice{"managents", "0.1.0", "e32r40t", 480, 320};
const ScreenGeometry kGeometry{480, 320, 28, 6, 6};

/// Every line of the valid and invalid fixtures: the seeds for the mutations.
std::vector<std::string> fixtureLines() {
    std::vector<std::string> lines;
    for (const char* directory : {"valid/", "invalid/"}) {
        DIR* dir = opendir((kFixtures + directory).c_str());
        TEST_ASSERT_NOT_NULL_MESSAGE(dir, "fixture directory not found");
        while (const dirent* entry = readdir(dir)) {
            const std::string name = entry->d_name;
            if (name.size() < 6 || name.substr(name.size() - 6) != ".jsonl") {
                continue;
            }
            std::ifstream file(kFixtures + directory + name);
            for (std::string line; std::getline(file, line);) {
                if (!line.empty()) {
                    lines.push_back(line);
                }
            }
        }
        closedir(dir);
    }
    TEST_ASSERT_FALSE_MESSAGE(lines.empty(), "no fixture lines");
    return lines;
}

/// Applies 1-6 random edits to `line` (replace, delete or insert bytes, or
/// duplicate a slice), sometimes makes it too long to keep, and frames it.
std::string mutate(std::string line, Random& random) {
    // JSON punctuation and digits, line framing, a NUL, UTF-8 lead and
    // continuation bytes, and a byte that is never valid UTF-8.
    static const std::string kBytes = std::string("{}[]\":,\\-.e09\r\n") + '\0' + "\xC3\xA9\xF0\x80\xFF";
    const std::size_t edits = 1 + random.below(6);
    for (std::size_t i = 0; i < edits && !line.empty(); ++i) {
        const std::size_t at = random.below(line.size());
        switch (random.below(4)) {
            case 0:
                line[at] = kBytes[random.below(kBytes.size())];
                break;
            case 1:
                line.erase(at, 1 + random.below(8));
                break;
            case 2:
                line.insert(at, 1, kBytes[random.below(kBytes.size())]);
                break;
            default:
                line.insert(at, line.substr(random.below(line.size()), random.below(64)));
                break;
        }
    }
    if (random.below(100) == 0) {
        line.append(LineAssembler::kMaxLineLength, 'x');
    }
    return line + '\n';
}

void survives_mutated_frames_in_random_chunks() {
    const std::vector<std::string> seeds = fixtureLines();
    CheckingDisplay display;
    NullIndicator indicator;
    NullLink link;
    Application app(display, indicator, link, kDevice, kGeometry);
    Random random;

    std::uint32_t nowMs = UINT32_MAX - 600000;  // millis() wraps ten minutes in
    app.begin(nowMs);
    int connectedTicks = 0;
    for (int i = 0; i < kIterations; ++i) {
        const std::string bytes = mutate(seeds[random.below(seeds.size())], random);
        for (std::size_t sent = 0; sent < bytes.size();) {
            const std::size_t chunk = std::min(bytes.size() - sent, 1 + random.below(300));
            app.onReceive(bytes.data() + sent, chunk, nowMs);
            sent += chunk;
        }
        nowMs += static_cast<std::uint32_t>(random.below(400));
        app.onTouch(random.below(3) == 0, nowMs);
        app.tick(nowMs);
        connectedTicks += app.hostConnected(nowMs) ? 1 : 0;
    }

    // Not vacuous: some mutated frames were accepted and some timed out.
    TEST_ASSERT_GREATER_THAN_INT(0, connectedTicks);
    TEST_ASSERT_LESS_THAN_INT(kIterations, connectedTicks);
    TEST_ASSERT_GREATER_THAN_size_t(1, display.presented);
}

}  // namespace

void setUp() {}
void tearDown() {}

int main() {
    UNITY_BEGIN();
    RUN_TEST(survives_mutated_frames_in_random_chunks);
    return UNITY_END();
}
