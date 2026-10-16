#include "ui/logos.hpp"

#include <algorithm>
#include <cmath>

#include "ui/codex_logo.hpp"
#include "ui/theme.hpp"

namespace managents::ui {
namespace {

/// How far a dimmed logo fades into the card, in percent.
constexpr std::uint32_t kDimPercent = 55;

/// The Claude mark: a warm starburst of tapered rays.
void drawClaude(lgfx::LovyanGFX& canvas, float cx, float cy, float size, std::uint32_t ink) {
    constexpr int kRays = 12;
    constexpr float kPi = 3.14159265F;
    const float baseRadius = size * 0.07F;
    const float tipRadius = size * 0.035F;
    for (int i = 0; i < kRays; ++i) {
        const float angle = 2.0F * kPi * static_cast<float>(i) / kRays + 0.13F;
        const float length = size * (i % 2 == 0 ? 0.36F : 0.29F);
        const float tipX = cx + std::cos(angle) * length;
        const float tipY = cy + std::sin(angle) * length;
        canvas.drawWedgeLine(cx, cy, tipX, tipY, baseRadius, tipRadius, ink);
    }
}

/// The OpenCode mark: a white frame around a grey block (from its SVG logo),
/// laid out on a 16-unit grid so every edge falls on a whole pixel.
void drawOpenCode(lgfx::LovyanGFX& canvas, std::int32_t x, std::int32_t y, std::int32_t size, std::uint32_t outer,
                  std::uint32_t inner, std::uint32_t tile) {
    auto rect = [&](std::int32_t left, std::int32_t top, std::int32_t right, std::int32_t bottom, std::uint32_t ink) {
        const std::int32_t x0 = x + size * left / 16;
        const std::int32_t y0 = y + size * top / 16;
        canvas.fillRect(x0, y0, x + size * right / 16 - x0, y + size * bottom / 16 - y0, ink);
    };
    rect(4, 3, 12, 13, outer);
    rect(6, 5, 10, 11, tile);
    rect(6, 7, 10, 11, inner);
}

/// The Codex mark: its blob in the blue-violet gradient with the `>_` prompt
/// in white, as on its colour icon. The coverage maps of codex_logo.hpp are
/// box-filtered down to the logo size; the blob spans three quarters of the
/// tile.
void drawCodex(lgfx::LovyanGFX& canvas, std::int32_t x, std::int32_t y, std::int32_t size, std::uint32_t tile,
               const std::uint32_t (&gradient)[3], std::uint32_t prompt) {
    const std::int32_t mark = std::max<std::int32_t>(size * 3 / 4, 1);
    const std::int32_t left = x + (size - mark) / 2;
    const std::int32_t top = y + (size - mark) / 2;
    for (std::int32_t row = 0; row < mark; ++row) {
        const std::int32_t r0 = row * kCodexMapSize / mark;
        const std::int32_t r1 = std::max((row + 1) * kCodexMapSize / mark, r0 + 1);
        const auto along = static_cast<std::uint32_t>(row * 200 / std::max<std::int32_t>(mark - 1, 1));
        const std::uint32_t fill =
            along <= 100 ? blend(gradient[0], gradient[1], along) : blend(gradient[1], gradient[2], along - 100);
        for (std::int32_t column = 0; column < mark; ++column) {
            const std::int32_t c0 = column * kCodexMapSize / mark;
            const std::int32_t c1 = std::max((column + 1) * kCodexMapSize / mark, c0 + 1);
            std::uint32_t blob = 0;
            std::uint32_t cut = 0;
            for (std::int32_t r = r0; r < r1; ++r) {
                for (std::int32_t c = c0; c < c1; ++c) {
                    blob += kCodexBlob[r * kCodexMapSize + c];
                    cut += kCodexPrompt[r * kCodexMapSize + c];
                }
            }
            const auto cells = static_cast<std::uint32_t>((r1 - r0) * (c1 - c0));
            if (blob == 0) {
                continue;
            }
            std::uint32_t ink = blend(tile, fill, blob * 100 / (255 * cells));
            ink = blend(ink, prompt, cut * 100 / (255 * cells));
            canvas.drawPixel(left + column, top + row, ink);
        }
    }
}

/// The rounded tile with the agent's mark, drawn straight onto `canvas`.
void drawTile(lgfx::LovyanGFX& canvas, core::AgentKind kind, std::int32_t x, std::int32_t y, std::int32_t size,
              bool dimmed, std::uint32_t background) {
    auto ink = [&](std::uint32_t color) { return dimmed ? blend(color, background, kDimPercent) : color; };
    const std::uint32_t tile = ink(color::kLogoTile);
    canvas.fillRoundRect(x, y, size, size, size * 22 / 100, tile);

    const float side = static_cast<float>(size);
    switch (kind) {
        case core::AgentKind::Claude:
            drawClaude(canvas, x + side / 2, y + side / 2, side, ink(color::kClaude));
            break;
        case core::AgentKind::OpenCode:
            drawOpenCode(canvas, x, y, size, ink(color::kOpenCodeOuter), ink(color::kOpenCodeInner), tile);
            break;
        case core::AgentKind::Codex:
            drawCodex(canvas, x, y, size, tile,
                      {ink(color::kCodexTop), ink(color::kCodexMiddle), ink(color::kCodexBottom)},
                      ink(color::kCodexPrompt));
            break;
        case core::AgentKind::Unknown:
            canvas.fillCircle(x + size / 2, y + size / 2, size / 4, ink(color::kMutedText));
            break;
    }
}

}  // namespace

void drawLogo(lgfx::LovyanGFX& canvas, core::AgentKind kind, std::int32_t x, std::int32_t y, std::int32_t size,
              bool dimmed, std::uint32_t background) {
    // LovyanGFX's anti-aliased lines round negative coordinates differently from
    // positive ones, so a logo cut by a strip boundary would not line up. Drawn at
    // the origin of its own small sprite, it gets the same pixels in every strip.
    lgfx::LGFX_Sprite tile(&canvas);
    tile.setColorDepth(16);
    if (tile.createSprite(size, size) == nullptr) {
        drawTile(canvas, kind, x, y, size, dimmed, background);
        return;
    }
    tile.fillScreen(background);
    drawTile(tile, kind, 0, 0, size, dimmed, background);
    tile.pushSprite(x, y);
}

}  // namespace managents::ui
