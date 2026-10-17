# 1. Record architecture decisions

Date: 2026-10-09 · Status: Accepted

## Context

The project spans firmware, a host program, a wire protocol and a printed enclosure. Decisions in one part constrain
the others, and contributors need to know why things are the way they are.

## Decision

Significant decisions are recorded as short ADRs in `docs/adr/`, numbered, never rewritten; a later ADR supersedes an
earlier one.

## Consequences

Reviewers can point to an ADR instead of re-arguing a choice. Changing a recorded decision takes a new ADR.
