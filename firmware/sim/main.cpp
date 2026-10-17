// Host renderer: paints the firmware's real UI into PNG files, for the README,
// the golden images in docs/screens and reviewing UI changes.
//
// Every screen takes the board's path: a protocol frame, core::buildScene, then
// LgfxDisplay painting 40-pixel strips with ScenePainter, into an RGB565 sprite
// the size of the panel instead of the panel itself. Each one is compared with a
// single full-frame paint, so a drawing that does not line up across strip
// boundaries fails here instead of on the device.
//
// The first frame of each fixture in protocol/fixtures/valid, and the screens
// shown without a live link (connecting, setup needed, reconnecting), are
// painted on every page, in both orientations and both blink phases. The
// landscape ones are written, plus a few variants worth reviewing (kAlsoWritten)
// and showcase@2x.png, pixel-doubled for the README.
//
//   make screens, or: cd firmware && pio run -e sim && .pio/build/sim/program ../docs/screens
//
// Exits with 1 if a fixture does not decode, a file cannot be written or a
// strip check fails.

#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <map>
#include <memory>
#include <set>
#include <string>
#include <system_error>

#include <LovyanGFX.hpp>

#include "adapters/lgfx_display.hpp"
#include "config.hpp"
#include "managents/core/pager.hpp"
#include "managents/core/protocol.hpp"
#include "managents/core/scene.hpp"
#include "ui/scene_painter.hpp"

namespace {

using namespace managents;
namespace fs = std::filesystem;

/// Relative to firmware/, where PlatformIO builds and runs.
const fs::path kFixtureDir = "../protocol/fixtures/valid";

/// The README's picture; it is also written at twice the size.
const std::string kShowcase = "showcase";

/// Variants written besides the landscape screens in the bright blink phase.
const std::set<std::string> kAlsoWritten = {"showcase-portrait", "mixed-four-blink-dark", "setup-needed-portrait"};

/// What the link screens show besides the scene.
const ui::LinkScreenText kLinkScreenText{config::kSetupUrl, config::kFirmwareVersion};

/// The E32R40T panel; the enclosure holds it in landscape.
constexpr std::int16_t kLongSide = 480;
constexpr std::int16_t kShortSide = 320;

bool createCanvas(lgfx::LGFX_Sprite& canvas, std::int32_t width, std::int32_t height) {
    canvas.setColorDepth(16);  // RGB565, as on the panel
    return canvas.createSprite(width, height) != nullptr;
}

std::size_t countDifferentPixels(lgfx::LGFX_Sprite& a, lgfx::LGFX_Sprite& b) {
    const auto* left = static_cast<const std::uint16_t*>(a.getBuffer());
    const auto* right = static_cast<const std::uint16_t*>(b.getBuffer());
    std::size_t count = 0;
    for (std::size_t i = 0; i < a.bufferLength() / sizeof *left; ++i) {
        count += left[i] != right[i] ? 1 : 0;
    }
    return count;
}

bool writePng(lgfx::LGFX_Sprite& canvas, const fs::path& path) {
    // createPng() encodes nothing unless the size is given explicitly.
    std::size_t length = 0;
    const std::unique_ptr<void, decltype(&std::free)> png(
        canvas.createPng(&length, 0, 0, canvas.width(), canvas.height()), &std::free);
    std::ofstream file(path, std::ios::binary);
    return png != nullptr && file.write(static_cast<const char*>(png.get()), static_cast<std::streamsize>(length));
}

/// Writes `canvas` with every pixel drawn as 2 x 2, so it stays sharp on high-density screens.
bool writeDoubledPng(lgfx::LGFX_Sprite& canvas, const fs::path& path) {
    lgfx::LGFX_Sprite doubled;
    if (!createCanvas(doubled, canvas.width() * 2, canvas.height() * 2)) {
        return false;
    }
    const auto* source = static_cast<const std::uint16_t*>(canvas.getBuffer());
    auto* target = static_cast<std::uint16_t*>(doubled.getBuffer());
    for (std::int32_t y = 0; y < doubled.height(); ++y) {
        for (std::int32_t x = 0; x < doubled.width(); ++x) {
            target[y * doubled.width() + x] = source[(y / 2) * canvas.width() + x / 2];
        }
    }
    return writePng(doubled, path);
}

/// The first frame of every fixture that holds a state message, by file name
/// without the extension. Fixtures with a hello probe are skipped.
bool loadFrames(std::map<std::string, core::HostState>& frames) {
    std::error_code error;
    for (const fs::directory_entry& entry : fs::directory_iterator(kFixtureDir, error)) {
        if (entry.path().extension() != ".jsonl") {
            continue;
        }
        std::ifstream file(entry.path());
        std::string line;
        std::getline(file, line);
        core::HostState host;
        const core::MessageType type = core::decodeHostMessage(line.c_str(), line.size(), host);
        if (type == core::MessageType::State) {
            frames.emplace(entry.path().stem().string(), host);
        } else if (type != core::MessageType::Hello) {
            std::fprintf(stderr, "%s: the first line is not a host message\n", entry.path().c_str());
            return false;
        }
    }
    if (error) {
        std::fprintf(stderr, "%s: %s (run from firmware/)\n", kFixtureDir.c_str(), error.message().c_str());
        return false;
    }
    return true;
}

/// Paints one screen through the board's strip pipeline, checks it against a
/// full-frame paint and, if `write`, saves it as <name>.png in `directory`.
bool render(const std::string& name, const core::SceneInput& input, bool portrait, bool write,
            const fs::path& directory) {
    const std::int16_t width = portrait ? kShortSide : kLongSide;
    const std::int16_t height = portrait ? kLongSide : kShortSide;
    const core::Scene scene = core::buildScene(input, core::screenGeometry(width, height));

    lgfx::LGFX_Sprite screen;
    lgfx::LGFX_Sprite reference;
    if (!createCanvas(screen, width, height) || !createCanvas(reference, width, height)) {
        std::fprintf(stderr, "%s: out of memory\n", name.c_str());
        return false;
    }
    adapters::LgfxDisplay display(screen, kLinkScreenText);
    display.begin();
    display.present(scene);
    ui::ScenePainter(reference, 0, width, height, kLinkScreenText).paint(scene);

    const std::size_t different = countDifferentPixels(screen, reference);
    if (different > 0) {
        std::fprintf(stderr, "%s: %zu pixels differ between strip and full-frame painting\n", name.c_str(), different);
    }
    if (!write) {
        return different == 0;
    }
    const bool written = writePng(screen, directory / (name + ".png")) &&
                         (name != kShowcase || writeDoubledPng(screen, directory / (name + "@2x.png")));
    if (!written) {
        std::fprintf(stderr, "%s: cannot write to %s\n", name.c_str(), directory.c_str());
    }
    return different == 0 && written;
}

/// Renders every page of `host` in both orientations and blink phases; without
/// a host, the screen for `link`.
bool renderAll(const std::string& subject, const core::HostState* host, core::LinkState link,
               const fs::path& directory) {
    const std::size_t pages = core::Pager::pageCount(host != nullptr ? host->agentCount : 0);
    bool ok = true;
    for (std::size_t page = 0; page < pages; ++page) {
        for (const bool portrait : {false, true}) {
            for (const bool blinkOn : {true, false}) {
                std::string name = subject;
                name += page > 0 ? "-page-" + std::to_string(page + 1) : "";
                name += portrait ? "-portrait" : "";
                name += blinkOn ? "" : "-blink-dark";
                const bool write = (!portrait && blinkOn) || kAlsoWritten.count(name) > 0;
                ok = render(name, {host, 0, blinkOn, page, link}, portrait, write, directory) && ok;
            }
        }
    }
    return ok;
}

}  // namespace

int main(int argc, char** argv) {
    if (argc != 2) {
        std::fprintf(stderr, "usage: %s <output-directory>\n", argv[0]);
        return 2;
    }
    const fs::path directory = argv[1];
    std::error_code error;
    fs::create_directories(directory, error);
    if (error) {
        std::fprintf(stderr, "%s: %s\n", directory.c_str(), error.message().c_str());
        return 1;
    }
    std::map<std::string, core::HostState> frames;
    if (!loadFrames(frames)) {
        return 1;
    }

    bool ok = renderAll("connecting", nullptr, core::LinkState::Connecting, directory);
    ok = renderAll("setup-needed", nullptr, core::LinkState::SetupNeeded, directory) && ok;
    ok = renderAll("reconnecting", nullptr, core::LinkState::Lost, directory) && ok;
    for (const auto& [name, host] : frames) {
        ok = renderAll(name, &host, core::LinkState::Connecting, directory) && ok;
    }
    return ok ? 0 : 1;
}
