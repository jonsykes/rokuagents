#pragma once

#include <cstdint>

#include <LovyanGFX.hpp>

#include "managents/core/scene.hpp"

namespace managents::ui {

/// Texts fixed at build time that the link screens show.
struct LinkScreenText {
    const char* setupUrl;         ///< where the setup screen's QR code leads
    const char* firmwareVersion;  ///< the footer reads "fw <version>"
};

/// Paints a Scene onto any LovyanGFX canvas. The canvas may be a horizontal
/// strip of the screen starting at `originY`: everything is drawn shifted by
/// that amount and clipped by the canvas, so the same code renders the whole
/// screen strip by strip without a full-frame buffer.
class ScenePainter {
public:
    ScenePainter(lgfx::LovyanGFX& canvas, std::int32_t originY, std::int32_t screenWidth, std::int32_t screenHeight,
                 const LinkScreenText& text);

    void paint(const core::Scene& scene);

private:
    void paintHeader(const core::HeaderView& header);
    void paintPageDots(const core::HeaderView& header, std::int32_t middle);
    void paintCard(const core::CardView& card);
    void paintContextBar(const core::ContextView& context, std::uint32_t ink, std::uint32_t background,
                         std::int32_t left, std::int32_t top, std::int32_t width);
    void paintNoAgents();
    void paintMessage(const char* title, const char* subtitle);
    void paintSetup();
    void paintFooter();

    bool intersects(std::int32_t top, std::int32_t height) const;
    std::int32_t y(std::int32_t sceneY) const { return sceneY - originY_; }

    lgfx::LovyanGFX& canvas_;
    std::int32_t originY_;
    std::int32_t width_;
    std::int32_t height_;
    LinkScreenText text_;
};

}  // namespace managents::ui
