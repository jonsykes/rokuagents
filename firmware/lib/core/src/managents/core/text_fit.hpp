#pragma once

#include <cstddef>
#include <cstdint>

namespace managents::core {

/// Measures text in a family of fonts ordered largest first. A port: the UI
/// measures with the panel's fonts, the tests with a fake.
class TextMeasure {
public:
    virtual ~TextMeasure() = default;
    virtual std::size_t fontCount() const = 0;
    /// Width of `text` in `font`, in pixels.
    virtual std::int32_t width(std::size_t font, const char* text) const = 0;
    /// Line height of `font`, in pixels.
    virtual std::int32_t height(std::size_t font) const = 0;
};

/// The largest font in which `text` fits in `maxWidth` x `maxHeight`, or the
/// smallest if none does.
std::size_t largestFitting(const TextMeasure& measure, const char* text, std::int32_t maxWidth, std::int32_t maxHeight);

/// A name fitted into a box, ready to draw line by line in `font`.
struct FittedName {
    /// A card name (64 bytes) shortened with "..." still fits.
    static constexpr std::size_t kLineCapacity = 72;
    std::size_t font = 0;
    char lines[2][kLineCapacity] = {};
    std::size_t lineCount = 0;
};

/// Fits an ASCII name in `maxWidth` x `maxHeight`, keeping it whole whenever it
/// can: one line in the largest font that holds it, else two lines in the
/// largest font that holds them, preferably broken after - _ / . or a space.
/// Only if two lines in the smallest font are not enough is the second one
/// shortened in the middle, keeping the end that often tells similar names
/// apart ("infra #2").
FittedName fitName(const TextMeasure& measure, const char* text, std::int32_t maxWidth, std::int32_t maxHeight);

/// Copies ASCII `text` into `out`, shortened in the middle with "..." if it is
/// wider than `maxWidth` in `font`: "tonylook/infra #2" -> "tonylo...nfra #2".
/// Always NUL-terminates `out`.
void ellipsizeMiddle(const TextMeasure& measure, std::size_t font, const char* text, std::int32_t maxWidth, char* out,
                     std::size_t capacity);

}  // namespace managents::core
