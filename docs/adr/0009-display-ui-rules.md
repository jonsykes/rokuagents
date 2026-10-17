# 9. Display UI rules

Date: 2026-10-09 · Status: Accepted · Amends the paging part of [0008](0008-passive-first.md)

## Context

The display is read at arm's length, in a glance, from a desk. After the first hardware review the owner fixed a
handful of rules for what it shows and how it reacts. They are visible product behaviour that features, roadmap items
and refactors tend to bump into, so they are recorded here instead of being rediscovered each time.

## Decision

1. **At most nine cards per page** (`Pager::kPerPage`). More sessions go on further pages, and a `+N` badge counts
   those the helper left out. The grid is chosen from the card count, down to a compact 3 × 3 for seven to nine.
2. **A tap anywhere shows the next page.** There is no hit-testing and no calibration (the display only knows
   "pressed or not"). After 30 s without a tap it returns to page one. A tap on a dimmed screen only wakes it. Boards
   without touch turn pages by themselves.
3. **Order is decided by the helper:** working sessions first, in start order, so they do not swap places, then the
   most recent status change first. The display never sorts.
4. **Context is a bar along the bottom of the card, never numbers.** The bar is shown when the limit is known.
5. **Names are the full folder name in a small font**: `name`, `parent/name` when two folders share it, `name #2` for
   several sessions in one folder. The display folds them to ASCII and fits them (one line, else two, a middle
   ellipsis only as a last resort).
6. **Link screens say what to do.** Connecting while the first frame is awaited; Setup needed, with a QR code to the
   README's Get started section, when the helper has not been heard from for 15 s since boot; Reconnecting when
   frames stop. An error card, like the LED, blinks only during the first minute of the error.
7. **The backlight dims when nobody needs it**: after a minute of Reconnecting, or five minutes with no agents or
   only idle ones. Connecting and Setup needed never dim.

## Consequences

- Touch features that need a position must not take the plain tap. The roadmap's detail view is a long-press.
- Anything that changes these rules needs a new ADR that supersedes this one, and the owner's agreement.
- The decisions live in `Pager`, `chooseGridShape`, `buildScene` and the painter, with tests; the rendered screens in
  `docs/screens` show them.
