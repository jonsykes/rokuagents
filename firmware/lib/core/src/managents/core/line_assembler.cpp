#include "managents/core/line_assembler.hpp"

namespace managents::core {

bool LineAssembler::push(char byte) {
    if (lineReady_) {
        length_ = 0;
        lineReady_ = false;
    }

    if (byte == '\n') {
        if (overflowed_) {
            overflowed_ = false;
            length_ = 0;
            return false;
        }
        if (length_ > 0 && buffer_[length_ - 1] == '\r') {
            --length_;
        }
        buffer_[length_] = '\0';
        lineReady_ = true;
        return true;
    }

    if (overflowed_) {
        return false;
    }
    if (length_ == kMaxLineLength) {
        overflowed_ = true;
        length_ = 0;
        return false;
    }
    buffer_[length_++] = byte;
    return false;
}

}  // namespace managents::core
