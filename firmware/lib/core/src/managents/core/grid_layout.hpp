#pragma once

#include <cstddef>
#include <cstdint>

namespace managents::core {

struct Rect {
    std::int16_t x = 0;
    std::int16_t y = 0;
    std::int16_t w = 0;
    std::int16_t h = 0;

    bool operator==(const Rect& other) const { return x == other.x && y == other.y && w == other.w && h == other.h; }
    bool operator!=(const Rect& other) const { return !(*this == other); }
};

struct GridShape {
    std::uint8_t columns = 0;
    std::uint8_t rows = 0;
};

/// Picks the column/row split for `count` cards in `area`: cards should be close
/// to a comfortable landscape aspect ratio, with as few empty slots as possible.
/// Works for any screen orientation.
GridShape chooseGridShape(std::size_t count, const Rect& area, std::int16_t gap);

/// Fills `out[0..count)` with card rectangles, row by row, left to right.
/// Pixels lost to integer division go to the last row/column so cards tile `area` exactly.
void layoutGrid(std::size_t count, const Rect& area, std::int16_t gap, Rect* out);

}  // namespace managents::core
