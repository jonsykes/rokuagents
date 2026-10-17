#pragma once

#include <cstddef>
#include <cstdint>

namespace managents::core {

/// "42s", "5m", "1h 12m", "2h", "1d 2h", "3d". Always NUL-terminates `out`.
void formatAge(std::uint32_t seconds, char* out, std::size_t capacity);

/// 24h "HH:MM" for a local time expressed as seconds since the epoch.
void formatClock(std::int64_t localEpochSeconds, char* out, std::size_t capacity);

/// Rounded percentage of `used` over `limit`, clamped to 0..100. `limit` must be > 0.
std::uint8_t percentOf(std::uint32_t used, std::uint32_t limit);

/// Copies UTF-8 `text` into `out` as the printable ASCII the display's fonts can
/// draw: Latin-1 letters lose their accents ("é" -> "e", "ß" -> "ss", "æ" -> "ae"),
/// and every other character, control character or malformed sequence becomes
/// one '?'. The result is never longer than `text`. Always NUL-terminates `out`.
void foldToAscii(const char* text, char* out, std::size_t capacity);

}  // namespace managents::core
