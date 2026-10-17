#include "ui/scene_painter.hpp"

#include <algorithm>
#include <cstdio>

#include "ui/logos.hpp"
#include "ui/text.hpp"
#include "ui/theme.hpp"

namespace managents::ui {
namespace {

using core::CardView;
using core::Rect;

/// Space around the link screens' text.
constexpr std::int32_t kMargin = 24;
constexpr std::int32_t kCardRadius = 10;
constexpr std::int32_t kBarHeight = 4;
/// Cards shorter than this (the 3 x 3 grid of 7 to 9 agents) use the compact
/// layout: a small logo and status label, so the name gets two lines.
constexpr std::int32_t kCompactBelowHeight = 100;

/// Card metrics derived from its size, like the POC's CSS container queries.
struct CardMetrics {
    bool compact;
    std::int32_t padding;
    std::int32_t gap;
    std::int32_t logo;
    const lgfx::IFont* ageFont;
};

CardMetrics metricsFor(const Rect& bounds) {
    const std::int32_t height = bounds.h;
    CardMetrics metrics{};
    metrics.compact = height < kCompactBelowHeight;
    metrics.padding = metrics.compact ? 7 : std::clamp<std::int32_t>(height / 10, 6, 14);
    metrics.gap = metrics.compact ? 3 : std::max<std::int32_t>(metrics.padding / 2, 3);
    metrics.logo = metrics.compact ? 16 : std::clamp<std::int32_t>(height * 22 / 100, 16, 40);
    metrics.ageFont = height >= 150 ? &fonts::FreeSansBold12pt7b : &fonts::FreeSansBold9pt7b;
    return metrics;
}

}  // namespace

ScenePainter::ScenePainter(lgfx::LovyanGFX& canvas, std::int32_t originY, std::int32_t screenWidth,
                           std::int32_t screenHeight, const LinkScreenText& text)
    : canvas_(canvas), originY_(originY), width_(screenWidth), height_(screenHeight), text_(text) {}

void ScenePainter::paint(const core::Scene& scene) {
    canvas_.fillScreen(color::kBackground);
    switch (scene.kind) {
        case core::SceneKind::Connecting:
            paintMessage("managents", "Connecting to your computer...");
            paintFooter();
            return;
        case core::SceneKind::SetupNeeded:
            paintSetup();
            paintFooter();
            return;
        case core::SceneKind::Reconnecting:
            paintMessage("Reconnecting...", "Is the computer asleep?");
            paintFooter();
            return;
        case core::SceneKind::NoAgents:
            paintHeader(scene.header);
            paintNoAgents();
            return;
        case core::SceneKind::Agents:
            paintHeader(scene.header);
            for (std::uint8_t i = 0; i < scene.cardCount; ++i) {
                if (intersects(scene.cards[i].bounds.y, scene.cards[i].bounds.h)) {
                    paintCard(scene.cards[i]);
                }
            }
            return;
    }
}

bool ScenePainter::intersects(std::int32_t top, std::int32_t height) const {
    return top < originY_ + canvas_.height() && top + height > originY_;
}

void ScenePainter::paintHeader(const core::HeaderView& header) {
    const Rect& bounds = header.bounds;
    if (!intersects(bounds.y, bounds.h)) {
        return;
    }
    const std::int32_t middle = y(bounds.y + bounds.h / 2);
    canvas_.setFont(&fonts::FreeSans9pt7b);
    canvas_.setTextColor(color::kFaintText);
    canvas_.setTextDatum(textdatum_t::middle_left);
    canvas_.drawString("managents", bounds.x + 10, middle);

    canvas_.setTextColor(color::kMutedText);
    canvas_.setTextDatum(textdatum_t::middle_right);
    const std::int32_t clockRight = bounds.x + bounds.w - 10;
    canvas_.drawString(header.clock.c_str(), clockRight, middle);

    if (!header.overflowBadge.empty()) {
        const std::int32_t badgeRight = clockRight - canvas_.textWidth(header.clock.c_str()) - 12;
        canvas_.setTextColor(color::kBrightText);
        canvas_.drawString(header.overflowBadge.c_str(), badgeRight, middle);
    }
    paintPageDots(header, middle);
}

// The current page is bright; the others are red if they hold an error,
// yellow if an agent waits there, so nothing that needs you hides on page 2.
void ScenePainter::paintPageDots(const core::HeaderView& header, std::int32_t middle) {
    if (header.pageCount < 2) {
        return;
    }
    constexpr std::int32_t kRadius = 4;
    constexpr std::int32_t kSpacing = 16;
    const std::int32_t first = width_ / 2 - (header.pageCount - 1) * kSpacing / 2;
    for (std::int32_t i = 0; i < header.pageCount; ++i) {
        std::uint32_t ink = color::kFaintText;
        if (i == header.page) {
            ink = color::kBrightText;
        } else if (header.pageAttention[i] == core::Attention::Error) {
            ink = color::kError;
        } else if (header.pageAttention[i] == core::Attention::Waiting) {
            ink = color::kWaiting;
        }
        canvas_.fillCircle(first + i * kSpacing, middle, kRadius, ink);
    }
}

// Card layout:
//   [logo]                 age
//   name, full width, one or two lines
//   STATUS
//   ==== context bar ====
void ScenePainter::paintCard(const CardView& card) {
    const Rect& bounds = card.bounds;
    const StatusStyle& style = styleFor(card.status);
    const std::uint32_t background = card.alertPhase ? style.alertBackground : style.background;
    const CardMetrics metrics = metricsFor(bounds);
    const std::int32_t top = y(bounds.y);
    const std::int32_t bottom = top + bounds.h;
    const std::int32_t left = bounds.x + metrics.padding;
    const std::int32_t right = bounds.x + bounds.w - metrics.padding;
    const std::int32_t innerWidth = right - left;

    canvas_.fillRoundRect(bounds.x, top, bounds.w, bounds.h, kCardRadius, background);

    const std::int32_t logoTop = top + metrics.padding;
    drawLogo(canvas_, card.kind, left, logoTop, metrics.logo, style.dimLogo, background);
    canvas_.setFont(metrics.ageFont);
    canvas_.setTextColor(blend(style.text, background, 15));  // a step below the name
    canvas_.setTextDatum(textdatum_t::middle_right);
    canvas_.drawString(card.age.c_str(), right, logoTop + metrics.logo / 2);

    std::int32_t baseline = bottom - metrics.padding;
    if (card.context.visible) {
        const std::int32_t barTop = bottom - metrics.gap - kBarHeight;
        paintContextBar(card.context, style.text, background, left, barTop, innerWidth);
        baseline = barTop - metrics.gap;
    }

    // The status may take up to 60% of the height under the logo row, the name
    // the rest. The widest label sets the font, so equal cards match.
    const std::int32_t nameTop = logoTop + metrics.logo + metrics.gap;
    const LgfxTextMeasure statusFonts(canvas_, kStatusFonts);
    const std::size_t statusFont =
        metrics.compact ? statusFonts.fontCount() - 1
                        : core::largestFitting(statusFonts, kWidestLabel, innerWidth, (baseline - nameTop) * 60 / 100);
    canvas_.setFont(statusFonts.font(statusFont));
    canvas_.setTextColor(style.text);
    canvas_.setTextDatum(textdatum_t::baseline_left);
    canvas_.drawString(style.label, left, baseline);
    const std::int32_t labelTop = baseline - ascentOf(statusFonts.font(statusFont));

    const LgfxTextMeasure nameFonts(canvas_, kNameFonts);
    const core::FittedName name =
        core::fitName(nameFonts, card.name.c_str(), innerWidth, labelTop - metrics.gap - nameTop);
    canvas_.setFont(nameFonts.font(name.font));
    canvas_.setTextDatum(textdatum_t::top_left);
    for (std::size_t line = 0; line < name.lineCount; ++line) {
        canvas_.drawString(name.lines[line], left, nameTop + static_cast<std::int32_t>(line) * canvas_.fontHeight());
    }
}

// The fill is the card's text colour on a track of the card colour, darkened:
// readable on every status, the blink's dark phase included.
void ScenePainter::paintContextBar(const core::ContextView& context, std::uint32_t ink, std::uint32_t background,
                                   std::int32_t left, std::int32_t top, std::int32_t width) {
    canvas_.fillRoundRect(left, top, width, kBarHeight, kBarHeight / 2, blend(background, color::kBlack, 35));
    const std::int32_t filled = width * context.percent / 100;
    if (filled > 0) {
        canvas_.fillRoundRect(left, top, filled, kBarHeight, kBarHeight / 2, ink);
    }
}

void ScenePainter::paintNoAgents() {
    canvas_.setFont(&fonts::FreeSansBold18pt7b);
    canvas_.setTextColor(color::kMutedText);
    canvas_.setTextDatum(textdatum_t::middle_center);
    canvas_.drawString("No agents open", width_ / 2, y(height_ / 2));
}

void ScenePainter::paintMessage(const char* title, const char* subtitle) {
    canvas_.setTextDatum(textdatum_t::middle_center);
    canvas_.setFont(&fonts::FreeSansBold18pt7b);
    canvas_.setTextColor(color::kBrightText);
    canvas_.drawString(title, width_ / 2, y(height_ / 2 - 16));
    canvas_.setFont(&fonts::FreeSans9pt7b);
    canvas_.setTextColor(color::kMutedText);
    canvas_.drawString(subtitle, width_ / 2, y(height_ / 2 + 22));
}

// Shown when the helper has not been heard from since boot. It may simply not
// run yet (the computer is at the login screen, or the service is stopped), so
// the text does not claim it is missing.
void ScenePainter::paintSetup() {
    const bool landscape = width_ > height_;
    const std::int32_t qrSize = landscape ? 132 : 150;

    // QR code to the setup guide: right column in landscape, bottom in portrait.
    const std::int32_t qrX = landscape ? width_ - kMargin - qrSize : (width_ - qrSize) / 2;
    const std::int32_t qrY = landscape ? (height_ - qrSize) / 2 - 10 : height_ - kMargin - qrSize - 56;
    if (intersects(qrY - 6, qrSize + 40)) {
        canvas_.fillRoundRect(qrX - 6, y(qrY - 6), qrSize + 12, qrSize + 12, 8, color::kQrBackdrop);
        canvas_.qrcode(text_.setupUrl, qrX, y(qrY), qrSize);
        canvas_.setFont(&fonts::FreeSans9pt7b);
        canvas_.setTextColor(color::kMutedText);
        canvas_.setTextDatum(textdatum_t::top_center);
        canvas_.drawString("setup guide", qrX + qrSize / 2, y(qrY + qrSize + 12));
    }

    const std::int32_t textTop = landscape ? qrY - 6 : kMargin + 16;  // level with the QR backdrop
    canvas_.setTextDatum(textdatum_t::top_left);
    canvas_.setFont(&fonts::FreeSansBold18pt7b);
    canvas_.setTextColor(color::kBrightText);
    canvas_.drawString("Waiting for", kMargin, y(textTop));
    canvas_.drawString("the helper", kMargin, y(textTop + 36));
    canvas_.setFont(&fonts::FreeSans9pt7b);
    canvas_.setTextColor(color::kMutedText);
    canvas_.drawString("managents shows your agents", kMargin, y(textTop + 86));
    canvas_.drawString("once its helper runs on the", kMargin, y(textTop + 108));
    canvas_.drawString("computer. Not installed yet?", kMargin, y(textTop + 130));
    canvas_.drawString("Scan to set it up.", kMargin, y(textTop + 152));
}

void ScenePainter::paintFooter() {
    canvas_.setFont(&fonts::FreeSans9pt7b);
    canvas_.setTextColor(color::kFaintText);
    canvas_.setTextDatum(textdatum_t::bottom_left);
    char footer[48];
    std::snprintf(footer, sizeof footer, "fw %s", text_.firmwareVersion);
    canvas_.drawString(footer, kMargin, y(height_ - 10));
}

}  // namespace managents::ui
