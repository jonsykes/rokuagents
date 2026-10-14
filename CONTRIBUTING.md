# Contributing

Thanks for helping! Issues and pull requests are welcome. For anything bigger than a fix, open an issue first so we
can agree on the approach. **AI coding agents: read [AGENTS.md](AGENTS.md)**, which has every command, quirk and rule
in one place.

## Workflow

`main` is protected: no direct pushes, not even for admins.

1. Branch from `main`: `feat/<topic>` or `fix/<topic>`.
2. Commit in small, logical steps with [Conventional Commits](https://www.conventionalcommits.org/): `feat(helper):`,
   `fix(firmware):`, `docs:`, `ci:`, `build:`, `test(protocol):`, `refactor(enclosure):`. Scopes: `helper`,
   `firmware`, `protocol`, `enclosure`, `docs`, `ci`.
3. Open a pull request and fill in its template. CI has seven required checks, all of which must pass with the branch
   up to date with `main`, and conversations must be resolved: `helper tests (ubuntu-latest)`,
   `helper tests (macos-latest)`, `helper tests (windows-latest)`, `helper lint`, `firmware tests and build`,
   `firmware format`, `enclosure renders`. Do not rename those jobs.
4. Docs change in the same pull request as the behaviour they describe, and so does `CHANGELOG.md` (under
   `[Unreleased]`).

## Setup

- Go: `helper/go.mod` pins `go 1.26.0` and `toolchain go1.26.9`; an older `go` downloads the right one by itself
  (do not set `GOTOOLCHAIN=local`).
- PlatformIO Core 6.2.0, the CI version (`pipx install platformio==6.2.0`). `pio` may not be on your `PATH`: it is
  `~/.local/bin/pio` with pipx, so run `make PIO=~/.local/bin/pio test`.
- clang-format **23.1.3** exactly, the CI pin (`pipx install clang-format==23.1.3`), and `shellcheck`.
- Optional, for the enclosure: OpenSCAD. The stable 2021.01 or a snapshot both work for `make enclosure` and
  `make enclosure-check` (snapshots are faster); `make enclosure-images` needs a snapshot.
- Hardware, optional: an E32R40T. **The helper's service holds the serial port exclusively**, so run
  `managents service stop` before `make flash`, `make monitor` or any other program that opens the port (`make flash`
  does it for you; `managents service start` brings the service back). CI has no board; nothing in the test suites
  needs one.

```bash
make test     # helper tests with -race, firmware core tests: everything that runs without hardware
make lint     # gofmt, go vet, staticcheck, go mod tidy, govulncheck, shellcheck, clang-format
make help     # every target
```

## Ground rules

- **The protocol is the contract.** Change `docs/protocol.md`, `protocol/schema` and `protocol/fixtures` in the same
  pull request, keep it backwards compatible, and let both test suites prove it.
- **Behaviour goes in the core.** Firmware logic belongs in `firmware/lib/core` (no Arduino headers) with a native
  test; helper logic belongs in `internal/` packages behind small interfaces, tested with fakes. Adapters stay thin.
- **No conversation content.** Detectors read session metadata only. Never log, store or send prompt or reply text,
  and never send directory paths to the display.
- **Synthetic names only** in fixtures, screens, demos and tests (`payments-api`, `website`, ...): never a real
  project, customer or employer name.
- **A UI change means `make screens`**, and committing the regenerated `docs/screens`: CI fails when they are stale.
- **Record decisions.** If a change makes a design choice others will wonder about, add an ADR in `docs/adr/`. The
  owner's UI rules are in [ADR-0009](docs/adr/0009-display-ui-rules.md); changing them needs a new ADR.

## Repository layout

| Path | What |
|---|---|
| [`firmware/`](firmware/README.md) | ESP32 firmware (PlatformIO, Arduino, LovyanGFX). Hardware-independent core in `lib/core`, unit-tested on the host; `sim/` renders the UI on the host. |
| [`helper/`](helper/README.md) | Host helper (Go): session detection, protocol encoding, serial transport, the background service, the flasher. |
| [`protocol/`](protocol/README.md) | The contract: JSON Schemas and shared fixtures, tested by **both** sides. |
| [`enclosure/`](enclosure/README.md) | Parametric OpenSCAD enclosure, inclined 45°, with ready-to-print STLs. |
| [`docs/`](docs/) | [Build guide](docs/build.md), [install and flash](docs/install.md), [using the display](docs/using.md), [privacy](docs/privacy.md), [architecture](docs/architecture.md), [protocol](docs/protocol.md), [hardware](docs/hardware.md), [troubleshooting](docs/troubleshooting.md), [decisions](docs/adr/README.md), [roadmap](docs/roadmap.md), `screens/`. |

## Adding an agent

1. **Helper kind.** Add `Kind<Name>` in `helper/internal/agent/agent.go`.
2. **Detector.** Create `helper/internal/detect/<name>/` implementing `detect.Source`. Declare the small interfaces it
   needs (process list, a store of its state) in the package itself, so tests use fakes and `t.TempDir()`; copy the
   shape of `detect/opencode`. Map the agent's states onto `working` / `waiting` / `error` / `idle`. Add a canary test
   that conversation text never leaks (`TestConversationContentNeverLeaks`).
3. **Register** it in `defaultSources()` in `helper/cmd/managents/commands.go`.
4. **Document** the mapping in `helper/README.md` ("What is detected") and a row in the compatibility table in `docs/install.md`.
5. **A new logo is optional**: an unknown `kind` draws a generic logo, and older firmware ignores it. If it gets one:
   `AgentKind` in `firmware/lib/core/src/managents/core/model.hpp`; `parseKind` in `core/protocol.cpp` with a
   `test_protocol` case; the vector logo in `firmware/src/ui/logos.cpp` (`drawLogo`, drawn the same in every 40-pixel
   strip) and its colours in `ui/theme.hpp`; the `kind` list in `docs/protocol.md`; the `kind` description in
   `protocol/schema/host-message.schema.json`; a card of the new kind in a fixture (`protocol/fixtures/valid/`); then
   `make screens` and commit the images. Add the kind to `helper/internal/demo` so `managents demo` shows it.
6. `CHANGELOG.md` entry; run `make test lint`.

## Hardware changes

Board pin maps live in `firmware/src/board/`; enclosure dimensions in `enclosure/lib/`. Note the source of every
dimension (drawing, datasheet, caliper) in a comment. After changing `enclosure/managents_case.scad`, run
`make enclosure enclosure-check` and commit the regenerated STLs; run `make enclosure-images` (needs an OpenSCAD
snapshot) when the look changes.

## Releasing

Releases are cut by the owner. The version is the git tag, for the helper and the firmware alike.

1. **Changelog.** In `CHANGELOG.md`, rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD`, add a fresh empty
   `## [Unreleased]` above it, and update the link references at the bottom. `make check-version VERSION=X.Y.Z` must
   pass, and so must `make release-notes VERSION=X.Y.Z` (it prints the text of the GitHub release). A
   test keeps the helper's `MinFirmware` at or below the newest changelog heading.
2. **Merge** to `main` through a pull request, with all seven checks green.
3. **Hardware gate**, below, on a real board with a release build: `make dist` on macOS, or the artifacts of a pre-release.
4. **Tag and push:** `git tag -a vX.Y.Z -m vX.Y.Z && git push origin vX.Y.Z`. `.github/workflows/release.yml`
   runs `make check-version dist` and `make dist-check` on macOS and publishes the release: a universal macOS
   binary, Linux amd64 and arm64, Windows amd64 (untested), the merged firmware image, `install.sh` and
   `checksums.txt`. A tag with a `-` in it (`v0.3.0-rc1`) is published as a pre-release.
5. **Check the published release**: run the one-liner from the README on a clean user account and see the display
   come up.

### Hardware gate

Run by the owner; agents never open the serial port.

- **Fresh user path.** On a computer without managents: run the `curl | sh` one-liner; plug in a board that has no
  firmware; `managents flash` (about 13 s, verified); the cards appear within a few seconds. Unplug and replug.
- **Sleep and wake.** Put the computer to sleep: the display shows Reconnecting; wake it: the cards come back.
- **Login.** Reboot and log in: the service starts by itself. Before login the display shows Connecting, then Setup
  needed after 15 s.
- **Port contention.** `managents service stop`, `make flash`, `managents service start`. While the service runs,
  `managents devices` and `managents demo` say the port is in use. A second CH340 device is probed at most three times
  (at once, +10 s, +70 s).
- **Load.** Run a heavy build while agents work: `managents run --log-level debug` shows no gap over 3 s between
  frames; taps are not missed while the ages tick.
- **Visual.** The QR code scans and opens the README's Get started section. The TN panel looks best from its
  left in rotation 1. Names and the context bar read well at 70 cm. Decide whether the LED is visible in the printed
  case.
- **Mechanical.** Print the body and bezel: the board drops in, the window frames the picture, and the bezel posts
  clamp the board without touching the glass. The USB-C receptacle height (3.26 mm assumed) and a plug with an
  overmold of about 8 mm fit. The corner feet keep the case still when tapped.
- **Owner settings** (once): enable private vulnerability reporting in the repository settings.
