# 7. Parametric OpenSCAD enclosure, inclined 45°

Date: 2026-10-09 · Status: Accepted

## Context

The display sits on a desk and should be readable from a seated position and reachable for touch. The vendor
publishes a dimensioned outline drawing (and a STEP model), so the case can be designed from real numbers.

## Decision

- OpenSCAD, text-based and diff-able; every board dimension comes from the vendor drawing and lives in one file,
  `enclosure/lib/e32r40t.scad`.
- A wedge with the screen face at 45°, on a deep base with rubber feet at its corners, so that pressing the screen
  neither tips nor slides it.
- Two parts that print without supports: the body standing on its base (nothing overhangs more than 45° from
  vertical except a few short bridges) and the bezel face down. 4 self-tapping screws (ST2.9 × 9.5) into Ø2.5 pilot
  holes in the printed corner supports clamp the board at its mounting pads, with no inserts or nuts to buy or heat;
  the bezel never touches the resistive panel.
- A small fit-test coupon to verify hole positions on the real board before the full print.
- STLs are committed so that people without OpenSCAD can print; CI renders them and checks that each part is one
  solid.

## Consequences

- Other boards are parameter changes. The angle stays at 45°, the only one at which both the face's underside and the
  features built along its normal print without supports.
- Dimensions not in the drawing (exact USB-C height, button actuators) are covered by generous clearances until a
  test print confirms them.
