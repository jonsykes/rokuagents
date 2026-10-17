# 3. JSON lines over USB serial

Date: 2026-10-09 · Status: Accepted

## Context

The board connects over USB through a CH340C bridge, so the transport is a serial port. Frames are small (a dozen
agents), sent every two seconds. The agent-lights proof of concept had already designed a JSON-lines protocol.

## Decision

One JSON object per line at 115200 baud, versioned with `v`, typed with `t`, at most 4096 bytes per line; receivers
ignore what they do not understand. The schema in `protocol/schema` and the fixtures in `protocol/fixtures` are the
contract, and both the helper and the firmware test against them.

## Consequences

- Human-readable: any serial monitor shows what is going on.
- Boot logs and noise on the same port are harmless (non-JSON lines are ignored).
- JSON is larger than a binary encoding, which does not matter at this rate.
- The protocol can grow additively (touch events, settings) without breaking older displays.
