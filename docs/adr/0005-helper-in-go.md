# 5. Helper written in Go

Date: 2026-10-09 · Status: Accepted

## Context

The proof of concept was a Python script. The helper must run on macOS, Linux and Windows for people who do not have
a Python environment, start at login, and talk to serial ports.

## Decision

Go, as one statically linked binary per platform. Libraries: `go.bug.st/serial` (ports, USB VID/PID), `gopsutil`
(process start times and working directories on every OS), `modernc.org/sqlite` (pure Go, no cgo). Standard library
for everything else (`flag`, `log/slog`). Tools are pinned in `go.mod` (`go tool staticcheck`).

## Consequences

- Trivial installation: download one file.
- Cross-compilation works for everything except serial port enumeration on macOS, which uses IOKit through cgo; macOS
  binaries are built on macOS runners.
