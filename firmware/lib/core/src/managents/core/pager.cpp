#include "managents/core/pager.hpp"

namespace managents::core {

void Pager::next(std::size_t items, std::uint32_t nowMs) {
    page_ = (page_ + 1) % pageCount(items);
    lastChangeMs_ = nowMs;
}

void Pager::update(std::size_t items, std::uint32_t nowMs) {
    const std::size_t pages = pageCount(items);
    if (pages == 1) {
        // Nothing to page through: the timers start when a second page appears.
        page_ = 0;
        lastChangeMs_ = nowMs;
        return;
    }
    if (page_ >= pages) {
        page_ = pages - 1;  // the list shrank under the current page
    }
    const std::uint32_t idleMs = nowMs - lastChangeMs_;
    if (autoAdvanceMs_ > 0) {
        if (idleMs >= autoAdvanceMs_) {
            next(items, nowMs);
        }
    } else if (page_ != 0 && idleMs >= kReturnToFirstMs) {
        page_ = 0;
        lastChangeMs_ = nowMs;
    }
}

bool TapDetector::update(bool pressed, std::uint32_t nowMs) {
    if (pressed) {
        const bool isNewPress = !held_;
        held_ = true;
        lastPressedMs_ = nowMs;
        return isNewPress;
    }
    if (held_ && nowMs - lastPressedMs_ >= kReleaseMs) {
        held_ = false;
    }
    return false;
}

}  // namespace managents::core
