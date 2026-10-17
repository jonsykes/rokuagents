#pragma once

#include <cstddef>
#include <cstring>

namespace managents::core {

/// Bounded, heap-free string. Assignments that do not fit are truncated on a
/// UTF-8 character boundary, so the content is always valid UTF-8 if the input was.
/// A null `text` assigns the empty string.
template <std::size_t Capacity>
class FixedString {
public:
    FixedString() = default;

    explicit FixedString(const char* text) { assign(text); }

    void assign(const char* text) { assign(text, text == nullptr ? 0 : std::strlen(text)); }

    void assign(const char* text, std::size_t length) {
        if (text == nullptr) {
            // memcpy from a null pointer is undefined, even for zero bytes.
            data_[0] = '\0';
            size_ = 0;
            return;
        }
        if (length > Capacity) {
            length = Capacity;
            while (length > 0 && isUtf8Continuation(text[length])) {
                --length;
            }
        }
        std::memcpy(data_, text, length);
        data_[length] = '\0';
        size_ = length;
    }

    void clear() { assign("", 0); }

    const char* c_str() const { return data_; }
    std::size_t size() const { return size_; }
    bool empty() const { return size_ == 0; }
    static constexpr std::size_t capacity() { return Capacity; }

    bool operator==(const FixedString& other) const {
        return size_ == other.size_ && std::memcmp(data_, other.data_, size_) == 0;
    }

    bool operator!=(const FixedString& other) const { return !(*this == other); }

    bool operator==(const char* other) const { return other != nullptr && std::strcmp(data_, other) == 0; }

private:
    static bool isUtf8Continuation(char c) { return (static_cast<unsigned char>(c) & 0xC0) == 0x80; }

    char data_[Capacity + 1] = {};
    std::size_t size_ = 0;
};

}  // namespace managents::core
