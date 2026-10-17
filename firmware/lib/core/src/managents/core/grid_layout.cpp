#include "managents/core/grid_layout.hpp"

#include <cmath>

namespace managents::core {
namespace {

/// Width/height ratio cards look best at (a short, wide tile like the POC's).
constexpr float kPreferredAspect = 1.6F;
/// Cost of one empty slot, in units of |log(aspect error)|.
constexpr float kEmptySlotPenalty = 0.3F;

std::int16_t cellSize(std::int16_t total, std::uint8_t cells, std::int16_t gap) {
    return static_cast<std::int16_t>((total - gap * (cells - 1)) / cells);
}

}  // namespace

GridShape chooseGridShape(std::size_t count, const Rect& area, std::int16_t gap) {
    if (count == 0) {
        return {};
    }
    GridShape best{1, static_cast<std::uint8_t>(count)};
    float bestScore = INFINITY;
    for (std::size_t columns = 1; columns <= count; ++columns) {
        const std::size_t rows = (count + columns - 1) / columns;
        if ((columns - 1) * rows >= count) {
            continue;  // a whole column would stay empty
        }
        const float width = cellSize(area.w, static_cast<std::uint8_t>(columns), gap);
        const float height = cellSize(area.h, static_cast<std::uint8_t>(rows), gap);
        if (width <= 0 || height <= 0) {
            continue;
        }
        const float aspectError = std::fabs(std::log((width / height) / kPreferredAspect));
        const float score = aspectError + kEmptySlotPenalty * static_cast<float>(columns * rows - count);
        if (score < bestScore) {
            bestScore = score;
            best = {static_cast<std::uint8_t>(columns), static_cast<std::uint8_t>(rows)};
        }
    }
    return best;
}

void layoutGrid(std::size_t count, const Rect& area, std::int16_t gap, Rect* out) {
    const GridShape shape = chooseGridShape(count, area, gap);
    if (shape.columns == 0) {
        return;
    }
    const std::int16_t width = cellSize(area.w, shape.columns, gap);
    const std::int16_t height = cellSize(area.h, shape.rows, gap);
    for (std::size_t i = 0; i < count; ++i) {
        const auto column = static_cast<std::int16_t>(i % shape.columns);
        const auto row = static_cast<std::int16_t>(i / shape.columns);
        Rect& rect = out[i];
        rect.x = static_cast<std::int16_t>(area.x + column * (width + gap));
        rect.y = static_cast<std::int16_t>(area.y + row * (height + gap));
        const bool lastColumn = column == shape.columns - 1;
        const bool lastRow = row == shape.rows - 1;
        rect.w = lastColumn ? static_cast<std::int16_t>(area.x + area.w - rect.x) : width;
        rect.h = lastRow ? static_cast<std::int16_t>(area.y + area.h - rect.y) : height;
    }
}

}  // namespace managents::core
