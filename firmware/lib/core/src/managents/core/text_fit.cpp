#include "managents/core/text_fit.hpp"

#include <cstring>

namespace managents::core {
namespace {

constexpr char kEllipsis[] = "...";
constexpr std::size_t kEllipsisLength = sizeof kEllipsis - 1;

bool isBreakAfter(char c) {
    return c == '-' || c == '_' || c == '/' || c == '.' || c == ' ';
}

/// Copies the first `length` bytes of `text` into `out`, cut to fit `capacity`.
void copyPrefix(const char* text, std::size_t length, char* out, std::size_t capacity) {
    length = length < capacity - 1 ? length : capacity - 1;
    std::memcpy(out, text, length);
    out[length] = '\0';
}

std::int32_t prefixWidth(const TextMeasure& measure, std::size_t font, const char* text, std::size_t length) {
    char prefix[FittedName::kLineCapacity];
    copyPrefix(text, length, prefix, sizeof prefix);
    return measure.width(font, prefix);
}

/// Length of the longest prefix of `text` that fits in `maxWidth`.
std::size_t longestFitting(const TextMeasure& measure, std::size_t font, const char* text, std::int32_t maxWidth) {
    const std::size_t length = std::strlen(text);
    std::size_t fits = 0;
    while (fits < length && fits + 1 < FittedName::kLineCapacity &&
           prefixWidth(measure, font, text, fits + 1) <= maxWidth) {
        ++fits;
    }
    return fits;
}

/// Where to break `text` into two lines that both fit in `maxWidth`: after the
/// last separator that leaves a second line that fits, else as late as
/// possible. 0 if the text does not fit on two lines.
std::size_t twoLineSplit(const TextMeasure& measure, std::size_t font, const char* text, std::int32_t maxWidth) {
    const std::size_t fits = longestFitting(measure, font, text, maxWidth);
    for (std::size_t end = fits; end > 0; --end) {
        if (isBreakAfter(text[end - 1]) && measure.width(font, text + end) <= maxWidth) {
            return end;
        }
    }
    return fits > 0 && measure.width(font, text + fits) <= maxWidth ? fits : 0;
}

/// Where to end a first line followed by a shortened one: as late as possible,
/// or after a separator if there is one in the second half of the line.
std::size_t firstLineEnd(const TextMeasure& measure, std::size_t font, const char* text, std::int32_t maxWidth) {
    const std::size_t fits = longestFitting(measure, font, text, maxWidth);
    for (std::size_t end = fits; end > fits / 2; --end) {
        if (isBreakAfter(text[end - 1])) {
            return end;
        }
    }
    return fits;
}

FittedName oneLine(std::size_t font, const char* text) {
    FittedName name;
    name.font = font;
    copyPrefix(text, std::strlen(text), name.lines[0], FittedName::kLineCapacity);
    name.lineCount = 1;
    return name;
}

FittedName twoLines(std::size_t font, const char* text, std::size_t split) {
    FittedName name;
    name.font = font;
    copyPrefix(text, split, name.lines[0], FittedName::kLineCapacity);
    copyPrefix(text + split, std::strlen(text + split), name.lines[1], FittedName::kLineCapacity);
    name.lineCount = 2;
    return name;
}

}  // namespace

std::size_t largestFitting(const TextMeasure& measure, const char* text, std::int32_t maxWidth,
                           std::int32_t maxHeight) {
    const std::size_t smallest = measure.fontCount() - 1;
    for (std::size_t font = 0; font < smallest; ++font) {
        if (measure.height(font) <= maxHeight && measure.width(font, text) <= maxWidth) {
            return font;
        }
    }
    return smallest;
}

FittedName fitName(const TextMeasure& measure, const char* text, std::int32_t maxWidth, std::int32_t maxHeight) {
    const std::size_t fonts = measure.fontCount();
    for (std::size_t font = 0; font < fonts; ++font) {
        if (measure.height(font) <= maxHeight && measure.width(font, text) <= maxWidth) {
            return oneLine(font, text);
        }
    }
    for (std::size_t font = 0; font < fonts; ++font) {
        if (2 * measure.height(font) > maxHeight) {
            continue;
        }
        const std::size_t split = twoLineSplit(measure, font, text, maxWidth);
        if (split > 0) {
            return twoLines(font, text, split);
        }
    }

    const std::size_t smallest = fonts - 1;
    FittedName name;
    name.font = smallest;
    if (2 * measure.height(smallest) > maxHeight) {
        ellipsizeMiddle(measure, smallest, text, maxWidth, name.lines[0], FittedName::kLineCapacity);
        name.lineCount = 1;
        return name;
    }
    const std::size_t split = firstLineEnd(measure, smallest, text, maxWidth);
    copyPrefix(text, split, name.lines[0], FittedName::kLineCapacity);
    ellipsizeMiddle(measure, smallest, text + split, maxWidth, name.lines[1], FittedName::kLineCapacity);
    name.lineCount = 2;
    return name;
}

void ellipsizeMiddle(const TextMeasure& measure, std::size_t font, const char* text, std::int32_t maxWidth, char* out,
                     std::size_t capacity) {
    const std::size_t length = std::strlen(text);
    copyPrefix(text, length, out, capacity);
    if (length < capacity && measure.width(font, out) <= maxWidth) {
        return;
    }
    // Drop characters from the middle, one at a time from the longer side, so
    // the start and the end stay about equally long, the end never shorter.
    std::size_t head = length / 2;
    std::size_t tail = length - head;
    while (head + tail > 0) {
        if (head >= tail) {
            --head;
        } else {
            --tail;
        }
        if (head + kEllipsisLength + tail >= capacity) {
            continue;
        }
        std::memcpy(out, text, head);
        std::memcpy(out + head, kEllipsis, kEllipsisLength);
        std::memcpy(out + head + kEllipsisLength, text + length - tail, tail);
        out[head + kEllipsisLength + tail] = '\0';
        if (measure.width(font, out) <= maxWidth) {
            return;
        }
    }
    copyPrefix(kEllipsis, kEllipsisLength, out, capacity);
}

}  // namespace managents::core
