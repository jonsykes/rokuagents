#include "managents/core/format.hpp"

#include <cstdio>
#include <cstring>

namespace managents::core {
namespace {

constexpr std::uint32_t kSecondsPerMinute = 60;
constexpr std::uint32_t kSecondsPerHour = 3600;
constexpr std::int64_t kSecondsPerDay = 86400;

/// ASCII for U+00C0..U+00FF, one entry per code point. Each fold is at most as
/// long as the character's two UTF-8 bytes.
constexpr const char* kLatin1Letters[] = {
    "A", "A", "A", "A", "A", "A", "AE", "C", "E", "E", "E", "E", "I", "I", "I",  "I",   // U+00C0
    "D", "N", "O", "O", "O", "O", "O",  "x", "O", "U", "U", "U", "U", "Y", "Th", "ss",  // U+00D0
    "a", "a", "a", "a", "a", "a", "ae", "c", "e", "e", "e", "e", "i", "i", "i",  "i",   // U+00E0
    "d", "n", "o", "o", "o", "o", "o",  "/", "o", "u", "u", "u", "u", "y", "th", "y",   // U+00F0
};
static_assert(sizeof kLatin1Letters / sizeof kLatin1Letters[0] == 0x40, "one entry per code point");

bool isContinuation(unsigned char byte) {
    return (byte & 0xC0) == 0x80;
}

/// Bytes after a lead byte, or -1 if `lead` cannot start a sequence.
int continuationsAfter(unsigned char lead) {
    if (lead >= 0xC2 && lead <= 0xDF) {
        return 1;
    }
    if (lead >= 0xE0 && lead <= 0xEF) {
        return 2;
    }
    if (lead >= 0xF0 && lead <= 0xF4) {
        return 3;
    }
    return -1;
}

/// Folds the character that starts at `text` into `folded` and returns how many
/// bytes it spans.
std::size_t foldCharacter(const unsigned char* text, char (&folded)[3]) {
    folded[0] = '?';
    folded[1] = '\0';
    if (text[0] < 0x80) {
        if (text[0] >= 0x20 && text[0] != 0x7F) {
            folded[0] = static_cast<char>(text[0]);
        }
        return 1;
    }
    const int expected = continuationsAfter(text[0]);
    if (expected < 0) {
        return 1;
    }
    std::uint32_t codePoint = text[0] & (0x3F >> expected);
    for (int i = 1; i <= expected; ++i) {
        if (!isContinuation(text[i])) {
            return static_cast<std::size_t>(i);  // truncated: what was read is one malformed character
        }
        codePoint = (codePoint << 6) | (text[i] & 0x3F);
    }
    // Only a two-byte sequence encodes U+00C0..U+00FF; a longer one is overlong.
    if (expected == 1 && codePoint >= 0xC0 && codePoint <= 0xFF) {
        std::strcpy(folded, kLatin1Letters[codePoint - 0xC0]);
    }
    return static_cast<std::size_t>(expected) + 1;
}

}  // namespace

void formatAge(std::uint32_t seconds, char* out, std::size_t capacity) {
    const auto days = static_cast<unsigned long>(seconds / kSecondsPerDay);
    const auto hours = static_cast<unsigned long>(seconds % kSecondsPerDay / kSecondsPerHour);
    const auto minutes = static_cast<unsigned long>(seconds % kSecondsPerHour / kSecondsPerMinute);
    if (seconds < kSecondsPerMinute) {
        std::snprintf(out, capacity, "%lus", static_cast<unsigned long>(seconds));
    } else if (seconds < kSecondsPerHour) {
        std::snprintf(out, capacity, "%lum", minutes);
    } else if (days > 0 && hours > 0) {
        std::snprintf(out, capacity, "%lud %luh", days, hours);
    } else if (days > 0) {
        std::snprintf(out, capacity, "%lud", days);
    } else if (minutes > 0) {
        std::snprintf(out, capacity, "%luh %lum", hours, minutes);
    } else {
        std::snprintf(out, capacity, "%luh", hours);
    }
}

void formatClock(std::int64_t localEpochSeconds, char* out, std::size_t capacity) {
    std::int64_t secondOfDay = localEpochSeconds % kSecondsPerDay;
    if (secondOfDay < 0) {
        secondOfDay += kSecondsPerDay;
    }
    const int hours = static_cast<int>(secondOfDay / kSecondsPerHour);
    const int minutes = static_cast<int>((secondOfDay % kSecondsPerHour) / kSecondsPerMinute);
    std::snprintf(out, capacity, "%02d:%02d", hours, minutes);
}

std::uint8_t percentOf(std::uint32_t used, std::uint32_t limit) {
    if (limit == 0 || used >= limit) {
        return limit == 0 ? 0 : 100;
    }
    return static_cast<std::uint8_t>((static_cast<std::uint64_t>(used) * 100 + limit / 2) / limit);
}

void foldToAscii(const char* text, char* out, std::size_t capacity) {
    if (capacity == 0) {
        return;
    }
    const auto* in = reinterpret_cast<const unsigned char*>(text);
    std::size_t written = 0;
    while (*in != '\0') {
        char folded[3];
        const std::size_t length = foldCharacter(in, folded);
        const std::size_t size = std::strlen(folded);
        if (written + size >= capacity) {
            break;
        }
        std::memcpy(out + written, folded, size);
        written += size;
        in += length;
    }
    out[written] = '\0';
}

}  // namespace managents::core
