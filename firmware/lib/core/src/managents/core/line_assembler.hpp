#pragma once

#include <cstddef>

namespace managents::core {

/// Splits a byte stream into `\n`-terminated lines.
///
/// A trailing `\r` is dropped. Lines longer than the buffer are discarded as a
/// whole and the assembler resynchronises at the next `\n`, as required by the
/// protocol ("receivers discard longer lines").
class LineAssembler {
public:
    static constexpr std::size_t kMaxLineLength = 4096;

    /// Feeds one byte. Returns true when a complete line is available via `line()`;
    /// the line stays valid until the next call to `push`.
    bool push(char byte);

    const char* line() const { return buffer_; }
    std::size_t length() const { return length_; }

private:
    char buffer_[kMaxLineLength + 1] = {};
    std::size_t length_ = 0;
    bool lineReady_ = false;
    bool overflowed_ = false;
};

}  // namespace managents::core
