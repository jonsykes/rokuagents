#pragma once

#include <cstddef>
#include <cstdint>

namespace managents::core {

/// Splits the agent list into pages and tracks which one is shown.
///
/// A tap anywhere goes to the next page (wrapping to the first). Without
/// touches the view returns to the first page — where the most active agents
/// are — after `kReturnToFirstMs`. Boards without touch can auto-advance instead.
class Pager {
public:
    static constexpr std::size_t kPerPage = 9;
    static constexpr std::uint32_t kReturnToFirstMs = 30000;

    /// `autoAdvanceMs` > 0 flips pages by itself at that interval (no-touch boards).
    explicit Pager(std::uint32_t autoAdvanceMs = 0) : autoAdvanceMs_(autoAdvanceMs) {}

    static std::size_t pageCount(std::size_t items) { return items == 0 ? 1 : (items + kPerPage - 1) / kPerPage; }

    /// Shows the next page (a tap).
    void next(std::size_t items, std::uint32_t nowMs);

    /// Keeps the page valid for `items` and applies the timers, which only run
    /// while there is more than one page. Call every tick.
    void update(std::size_t items, std::uint32_t nowMs);

    std::size_t page() const { return page_; }
    std::size_t firstItem() const { return page_ * kPerPage; }

private:
    std::uint32_t autoAdvanceMs_;
    std::size_t page_ = 0;
    std::uint32_t lastChangeMs_ = 0;
};

/// Turns raw "is the panel pressed" samples into single tap events: one per
/// press, ignoring bounces and the noise of a resistive panel.
class TapDetector {
public:
    /// A press must be released this long before the next one counts.
    static constexpr std::uint32_t kReleaseMs = 120;

    /// Returns true exactly once per distinct press.
    bool update(bool pressed, std::uint32_t nowMs);

private:
    bool held_ = false;
    std::uint32_t lastPressedMs_ = 0;
};

}  // namespace managents::core
