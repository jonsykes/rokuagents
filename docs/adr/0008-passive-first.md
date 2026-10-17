# 8. Passive display first, touch later

Date: 2026-10-09 · Status: Accepted; paging amended by [0009](0009-display-ui-rules.md)

## Context

The E32R40T has a resistive touch panel. Touch would allow details on demand, paging and settings, but needs
per-unit calibration, a device-to-host event protocol and a UI that works at arm's length.

## Decision

The MVP is a passive display: nothing requires touching it. The touch controller is already configured in the board
definition (it shares the SPI bus and its chip select must be driven), but no input is read. Touch is the first
milestone of the roadmap; the protocol reserves room for it.

## Consequences

- The MVP works the same on the E32N40T (no touch).
- The enclosure leaves the touch panel free (air gap above the glass) so touch can be enabled later without a reprint.
