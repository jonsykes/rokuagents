# AGENTS.md

Handbook for AI coding agents working on, or installing, **managents**. Humans start at [README.md](README.md) and
[CONTRIBUTING.md](CONTRIBUTING.md). What the user tells you in the session overrides this file. Keep this file under
32 KiB (Codex's default limit) and link to `docs/` instead of copying prose into it. Update it in the same pull
request as any change it describes.

## 1. What this is

managents is a USB desk display (LCDWiki 4.0" ESP32-32E, SKU E32R40T, 480x320) that shows what AI coding agents
(Claude Code, OpenCode) are doing, one card per session, plus a Go **helper** on the computer, the ESP32 **firmware**
(C++), the JSON-lines **protocol** between them, and a 3D-printable 45 degree **enclosure** (OpenSCAD).

Product goal: **plug and play and minimal.** The user installs one thing (the helper, with `install.sh`), plugs in the
display, and it works. No configuration, no Wi-Fi, no settings on the device. A new board is flashed with
`managents flash`. Prefer deleting code to adding options. Everything is small, production quality and tested.

## 2. Repository map

| Path | Role | Test it with |
|---|---|---|
| `helper/cmd/managents` | CLI: `run`, `status`, `devices`, `demo`, `flash`, `service`, `version`, `help`; flags, exit codes, log setup; the only place that knows concrete types (`commands.go`, `flash.go`, `service.go`, `log.go`, `main.go`) | `cd helper && go test -race ./cmd/...` |
| `helper/internal/agent` | Domain: `Session`, `Status`, `Kind`, `Arrange` (order and unique names). Imports no internal package. | `go test ./internal/agent` |
| `helper/internal/detect` | `Source` interface, `All` (merge, tolerate a failing source) | `go test ./internal/detect` |
| `helper/internal/detect/claude` | Claude Code: session registry, stale-record filter, transcript tail (errors, context) | `go test ./internal/detect/claude` |
| `helper/internal/detect/opencode` | OpenCode: processes plus `opencode.db` through a `Store` (SQLite, read-only) | `go test ./internal/detect/opencode` |
| `helper/internal/process` | `System`: process list (one `sysctl` on macOS, gopsutil elsewhere) | `go test ./internal/process` |
| `helper/internal/protocol` | Wire types, `NewState`, `Encode`, `ParseDeviceHello`, `MinFirmware`, version helpers | `go test ./internal/protocol` |
| `helper/internal/device` | Port enumeration, `hello` handshake, `Manager` (hot-plug, retry schedule, broadcast) | `go test ./internal/device` |
| `helper/internal/app` | `Runner`: poll, diff, send; discovery without delaying frames | `go test ./internal/app` |
| `helper/internal/service` | LaunchAgent and systemd user unit install/uninstall/start/stop/status (templates `launchd.plist.tmpl`, `systemd.service.tmpl`) | `go test ./internal/service` |
| `helper/internal/flash` | ESP32 ROM bootloader protocol (SLIP, sync, compressed write, MD5, reset), tested against a fake ROM | `go test ./internal/flash` |
| `helper/internal/firmware` | The display firmware image, embedded with `go:embed` (`images/` is gitignored, filled by `make helper`) | `go test ./internal/firmware` |
| `helper/internal/demo` | Canned scenes for `managents demo` (made-up names) | `go test ./internal/demo` |
| `firmware/lib/core` | Hardware-independent core: protocol decode, `buildScene`, grid, pager, `Application`, `format`, `text_fit`. No Arduino. | `cd firmware && pio test -e native` |
| `firmware/src/{board,ui,adapters}`, `main.cpp`, `config.hpp` | E32R40T pins and device; `ScenePainter`, theme, logos; strip display, LED, serial, touch; the loop | `pio run -e e32r40t` (build), `make screens` (pixels) |
| `firmware/sim` | Host renderer: the real UI on a headless LovyanGFX, fixtures to `docs/screens/*.png` | `make screens` |
| `firmware/test/test_*` | 11 Unity suites (88 cases): application, contract, fixed_string, format, grid_layout, line_assembler, pager, protocol, robustness, scene, text_fit | `pio test -e native -f test_scene` |
| `firmware/scripts` | `version.py` (version from the tag), `merge_image.py` (one image for 0x0) | built by `pio run` |
| `protocol/schema`, `protocol/fixtures/{valid,invalid}` | The contract: JSON Schemas and shared test lines, read by both test suites | both of the above |
| `enclosure/managents_case.scad`, `lib/e32r40t.scad`, `stl/`, `images/` | Parametric case, board dimensions, printable STLs, renders | `make enclosure enclosure-check` |
| `docs/` | `build.md` (what to buy, print, assemble), `install.md`, `using.md`, `privacy.md`, `architecture.md`, `protocol.md`, `hardware.md`, `troubleshooting.md`, `roadmap.md`, `adr/` (decisions), `screens/` (golden UI images) | link check, see section 4 |
| `install.sh` | The one-line installer, a release asset | `shellcheck install.sh` (in `make lint`) |
| `.github/` | `workflows/ci.yml`, `workflows/release.yml`, issue and PR templates, dependabot | |
| `Makefile` | Every developer entry point: `make help` | |

## 3. Architecture invariants

Read [docs/architecture.md](docs/architecture.md) and ADRs 0002, 0003, 0006, 0009, 0010, 0011, 0012 before structural
changes.

- **Dumb display, smart helper (ADR-0002).** Only the helper detects, names, orders and derives status. The firmware
  draws what it receives and owns layout. No Wi-Fi, Bluetooth, settings or sorting on the device.
- **The protocol is the contract.** It is the only thing the helper and firmware share. Its docs, schema, fixtures
  and both implementations change together.
- **Ports and adapters.** Behaviour lives in a core behind small interfaces; adapters stay thin. Helper dependency
  rules (check with `cd helper && go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...`): `agent` imports no
  internal package; detectors never import `protocol` or `device`; only `cmd/managents` knows every concrete type.
- **Firmware core:** `lib/core` includes no Arduino or LovyanGFX header and uses no heap in the model
  (`FixedString`, fixed arrays). Every `Scene`, `CardView` and `HeaderView` field must take part in `operator==`, or
  redraw-on-change misses it. `ScenePainter` must draw correctly per 40-pixel strip (use `y()` and `intersects()`);
  `make screens` fails when a strip differs from a full-frame paint.
- **Privacy: no content, no network.** See section 10.
- **One version.** The git tag is the version of the helper and of the firmware; the helper embeds the firmware of its
  own release. `MinFirmware` (`helper/internal/protocol/version.go`) is the oldest firmware the helper fully supports.

## 4. Build, test, lint

Run from the repository root. `pio` may not be on `PATH`: use `make PIO=~/.local/bin/pio ...`.

| Command | What it does | Success looks like |
|---|---|---|
| `make test` | helper `go test -race ./...` plus `pio test -e native` | every helper package `ok`; `88 test cases: 88 succeeded` (about 12 s) |
| `make lint` | gofmt, go vet, staticcheck, `go mod tidy -diff`, govulncheck, shellcheck, clang-format | exits 0; ends with `No vulnerabilities found.` and no clang-format output |
| `make format` | gofmt and clang-format in place | rewrites files |
| `make build` | firmware image, then `bin/managents` with it embedded | RAM about 10% (32.6 KB), flash about 32% (419 KB) of 1.25 MB |
| `make helper` | `bin/managents` only; embeds the image of the last firmware build if there is one | prints `embedded firmware <version>`; `bin/managents version` prints `managents <git describe>` |
| `make screens` | rebuild `docs/screens/*.png` with `pio run -e sim` and check strips | `git status docs/screens` shows no change unless the UI changed |
| `make enclosure` / `enclosure-check` / `enclosure-images` | STLs, one-solid check (CI runs it), renders (needs an OpenSCAD snapshot) | `enclosure-check: body is one solid` x3 |
| `make install` | build and install `~/.local/bin/managents`, then `service install` | the service runs |
| `make dist`, `dist-check`, `check-version`, `release-notes` | release assets (macOS only); see section 13 | |

Focused runs: `cd helper && go test -race ./internal/agent/`, `cd firmware && pio test -e native -f test_scene`.
`bin/managents status [--json]` needs no display. Firmware tests under sanitizers are already the default (`env:native`
runs AddressSanitizer and UBSan). Docs-heavy changes: check every relative link and anchor (a tiny script is enough;
there is no link-check job in CI).

## 5. Toolchain quirks (verified on macOS 26, arm64)

- **Go.** `helper/go.mod` says `go 1.26.0` and `toolchain go1.26.9`; an older `go` downloads it. Do not set
  `GOTOOLCHAIN=local`. staticcheck and govulncheck are `go tool` dependencies (`go tool staticcheck ./...`).
- **PlatformIO 6.2.0** at `~/.local/bin/pio`, perhaps not on `PATH`. espressif32@7.1.3 means Arduino-ESP32 2.0.17 and
  GCC 8.4: C++17 and the 2.x `ledc` API. Do not move to Arduino 3.x casually. Env `e32r40t` is the board, `native`
  the host tests, `sim` the host renderer.
- **clang-format must be 23.1.3**, the CI pin (`.clang-format`: Google style, 4 spaces, 120 columns). Other versions
  reformat differently and fail `firmware format`.
- **OpenSCAD:** the Makefile adds `--backend=manifold` when the binary supports it (snapshots); stable 2021.01 works
  for `make enclosure` and `make enclosure-check` through CGAL. `make enclosure-images` needs a snapshot with OpenGL.
  Never run `openscad` without `-o` in automation: it opens the GUI and waits forever. macOS has no `timeout`.
- **No SDL2.** The host renderer uses a headless shim (ADR-0011). On Linux arm64 with GCC the QR code in the sim draws
  nearly blank (a LovyanGFX 1.2.30 bug); macOS and x86_64 Linux, which CI uses, are exact.
- **macOS `sed -i`** needs an empty suffix argument (`sed -i ''`), and the shell is zsh.

## 6. Verified hardware facts

The board is an E32R40T behind a CH340C (1A86:7523). The port is `/dev/cu.usbserial-10` on the owner's Mac; use
`cu.*`, never `tty.*`. Pin map and bring-up notes: [docs/hardware.md](docs/hardware.md).

- **SPI write 27 MHz.** 40 MHz garbles frames on the shared bus. `waitDMA()` after each strip; touch CS configured.
- **Upload at 460800 baud.** 921600 fails through the CH340C ("serial data stream stopped").
- **Rotation 1** = landscape with USB-C on the right; `invert = false`, BGR. The enclosure is built around it.
- **DTR/RTS low means no reset.** The helper opens the port with both low and the board keeps running. The flasher
  drives them on purpose to enter the bootloader.
- **No PSRAM.** A 480x40 strip sprite (37.5 KiB) is the biggest allocation; never a full framebuffer. Loop stack 16 KiB;
  the loop runs under the 5 s task watchdog (`enableLoopWDT`), so blocking work above 5 s must call `feedLoopWDT()`.
- **The serial port is exclusive.** The helper (`TIOCEXCL`) and the service hold it, so a second program gets "busy".
  Stop the service (`managents service stop`) before `make monitor`, `pio device monitor`, `make flash`; start it
  after. `make flash` and `managents flash` do both for you.
- **The RGB LED is on the back of the PCB**, hidden behind the LCD and the case. It can only glow through the back
  grille or a translucent body. Never describe it as a front indicator.
- **Agents must not open, flash or monitor the port unless the user asked**, and must never assume a board is
  attached. CI has none.
- TN panel: its best viewing direction lies on the left in rotation 1 (not yet confirmed on the unit).

## 7. Owner decisions: do not change without the owner

These are product rules, recorded in [ADR-0009](docs/adr/0009-display-ui-rules.md). A change needs the owner's say-so
and a new ADR.

- At most **9 cards per page** (`Pager::kPerPage`); compact 3x3 layout for 7 to 9 cards.
- **A tap anywhere is "next page"**, no hit-testing, back to page 1 after 30 s. Position-based touch features use
  long-press (the roadmap's detail view), never the plain tap.
- **Order:** working sessions first (start order), then the most recent status change. The helper decides.
- **Context only as a bottom bar**, never as text or numbers.
- **Small full folder names** (`name`, `parent/name`, `name #2`), folded to ASCII and fitted (never cut mid-name unless
  nothing else fits).
- Error cards and the LED blink only during the first minute; the backlight dims when nobody needs it; Setup needed
  points to `https://github.com/tonylook/managents#get-started`, so the README keeps a `## Get started` heading, and it stays short: details go in `docs/`.

## 8. How-tos

**Add an agent detector.** Full checklist in [CONTRIBUTING.md](CONTRIBUTING.md#adding-an-agent): `agent.Kind`
(`helper/internal/agent/agent.go`); a package `helper/internal/detect/<name>` implementing `detect.Source` with its own
small interfaces, fakes and `t.TempDir()` in tests (copy `opencode_test.go`); the canary test that conversation text
never leaks; register in `defaultSources` (`helper/cmd/managents/commands.go`); `helper/README.md`; the compatibility
table in `docs/install.md`; `internal/demo`. A logo is optional (unknown kinds draw a generic one). If it gets one: `AgentKind`
in `core/model.hpp`, `parseKind` in `core/protocol.cpp` with a `test_protocol` case, `drawLogo` in `ui/logos.cpp`,
colours in `ui/theme.hpp`, the `kind` list in `docs/protocol.md` and the schema description, a fixture card, then
`make screens`. Finish with `make test lint` and a CHANGELOG line.

**Change the protocol.** Decide first: additive (optional field or new message type, stays `v: 1`) or breaking (bump
`v`; the helper keeps speaking the `proto` the display announces). In one PR: `docs/protocol.md`,
`protocol/schema/*.json`, `protocol/fixtures/{valid,invalid}` (at least one line each), `helper/internal/protocol`,
`firmware/lib/core/src/managents/core/{model.hpp,protocol.cpp}`, tests on both sides. Old firmware must ignore unknown
fields and new firmware must accept old frames. Field limits live in three places that must agree: the doc, the schema
(`id` maxLength 48; `name` 64 bytes is enforced by the helper, since JSON Schema counts characters), and the firmware's
`FixedString` sizes. `path` is reserved and ignored; never send directory paths. Add a CHANGELOG entry.

**Change the UI.** Data and logic in `core/scene.*` (pure: state, time, page, link state to a `Scene`) with
`operator==` and a `test_scene` case; drawing in `ui/scene_painter.cpp`; colours in `ui/theme.hpp` (typed RGB888
constants: LovyanGFX reads a plain int as RGB565); fonts are ASCII-only. Then `make screens`, look at the PNGs
(`docs/screens/showcase@2x.png` is the README hero, rendered from `protocol/fixtures/valid/showcase.jsonl`), and commit
them: CI re-renders and fails on any difference. State whether it was tested on hardware. Do not weaken section 7.

**Change the enclosure.** Parameters at the top of `enclosure/managents_case.scad`; board dimensions in
`lib/e32r40t.scad` with the source cited (drawing, datasheet, caliper). Keep overhangs at 45 degrees or steeper (only
45 degrees prints without supports). Check clearances with the `section` part (see `enclosure/README.md`; the preview is interactive, so
render with `-o` in automation), then `make enclosure enclosure-check`; run `make enclosure-images` for new renders; commit the STLs and PNGs; update the
size line and bill of materials in `enclosure/README.md` (120 x 88 x 57 mm plus the 6 mm bezel). Do not hand-edit STLs.

**Add a board.** `firmware/src/board/<id>.hpp` (pins, LovyanGFX device), a PlatformIO env with its `MANAGENTS_*`
flags, `docs/hardware.md`, the compatibility table in `docs/install.md`, and the shopping notes in `docs/build.md`. The core and UI adapt to any resolution; no helper change.

**Add a CLI command or flag.** `helper/cmd/managents/commands.go` (or its own file), the usage text in `main.go`, tests
in `commands_test.go`, `helper/README.md`, and `docs/install.md` or `docs/using.md` if users see it. Commands return errors; only `cmd` prints.

**Change the service or the flasher.** `helper/internal/service` and `helper/internal/flash` hide the OS and the
serial port behind interfaces (`Commander`, `Port`); extend the fakes, never the tests' reach into the real system.
`launchd_integration_test.go` is the only test that touches launchd, and only with `MANAGENTS_LAUNCHD_IT=1`; never run it
unasked. Read ADR-0010 and ADR-0012 first.

## 9. Installing or troubleshooting managents on a user's machine

When the user asks you to **install** managents:

1. Supported: macOS (tested), Linux (best effort). Windows: binary only, no installer, no service; point to the
   release page. Check `uname -sm`.
2. Run `curl -fsSL https://github.com/tonylook/managents/releases/latest/download/install.sh | sh`. It installs
   `~/.local/bin/managents` (checksum verified), runs `managents service install` and `managents service status`.
   Settings: `MANAGENTS_VERSION=v0.2.0`, `MANAGENTS_BIN_DIR`, `MANAGENTS_NO_SERVICE=1`. If `~/.local/bin` is not on
   `PATH` it says so; use the full path meanwhile. Rerun it to upgrade. Show the user the command before running it.
3. Verify: `managents version`, then `managents service status` shows `running (pid N)` and log lines. Plugging in a
   display produces `display connected` in that log. `managents devices` is **not** the check while the service runs:
   it correctly says the port is in use by another program.
4. New or blank board: `managents flash` (about 13 s; it stops and restarts the service itself). It overwrites the
   board's firmware: only run it when the user asked for it. With several boards pass `--port`.
5. Uninstall: `managents service uninstall`, delete `~/.local/bin/managents`.

When the user asks you to **troubleshoot**, run `managents service status`, then `managents status` (what is detected;
it prints each session's folder, so do not paste it elsewhere without asking), and read the log
(`~/Library/Logs/managents.log`, or `journalctl --user -u managents`). The full table is
[docs/troubleshooting.md](docs/troubleshooting.md). The common failures:

| Symptom | Cause and fix |
|---|---|
| Display shows "Setup needed" (QR code) | The helper is not running or not installed: `managents service start`, or install |
| "Reconnecting..." | The computer slept, the service is stopped, or the cable is out |
| `devices` says "in use by another program" | Normal while the service runs. `managents service stop` first if you need the port, `start` after |
| `devices` says "not a managents display (new board? run: managents flash)" | Blank or foreign firmware: `managents flash` |
| Nothing detected | `managents status` empty: check `~/.claude/sessions` (or `$CLAUDE_CONFIG_DIR`), `~/.local/share/opencode/opencode.db` |
| Linux: nothing connects, no error | Permission denied on `/dev/ttyUSB*` is logged only at debug: add the user to `dialout` (or `uucp`), log in again |
| macOS service will not start | Login Items & Extensions, allow managents; binaries in Desktop, Documents, Downloads are refused |
| Flash fails | Cable (data, not charge-only), another program holds the port, `--baud 115200`, `--port` for several boards |
| Two Mac users | One service holds the display; the other user's helper logs "in use" once |

Never open the serial port yourself to "test", never delete `~/Library/Logs/managents.log` by hand
(`service uninstall` does), and never run `sudo` for managents: it installs and runs as the user.

## 10. Privacy: hard rules

- Read only: `<config dir>/sessions/<pid>.json` (`~/.claude`, or `$CLAUDE_CONFIG_DIR`); the **last 256 KiB** of the
  session transcript (growing to 4 MiB only if no token usage is found), decoding only type, sidechain flag, API-error
  flag and token counts (**never add a content field to the transcript `entry` type**); OpenCode's
  `$XDG_DATA_HOME/opencode/opencode.db` read-only, extracting JSON fields inside SQL; process name, cwd, start time.
- Send only protocol fields over USB: id, kind, folder name, status, age, context used and limit. No directory paths.
- No network calls and no telemetry. `net/http` is not linked; keep it that way. The display has no radio.
- Canary tests (`TestConversationContentNeverLeaks`) enforce it; extend them, never weaken them.
- **Synthetic names only** in fixtures, screens, demos, docs and tests (`payments-api`, `website`, `infra-terraform`,
  home `/Users/ann`). Never a real project, customer or employer name, and never real session content.

## 11. Conventions

- **Go:** gofmt, vet and staticcheck clean; small interfaces defined where they are used; wrap errors with `%w`;
  detectors return errors and only `app`, `device` and `cmd` log (`slog`); inject a clock, no sleeps in tests;
  table-driven tests; tests using `t.Setenv` cannot be parallel. Exit codes and usage text are tested in `cmd`.
- **C++:** C++17, `.clang-format`; namespaces `managents::{core,ui,adapters,board,config}`; `kConstant` names;
  `-Wall -Wextra -Werror` on native and `-Werror -Wshadow` on `src/` for the board.
- **Docs** change in the same PR as the behaviour. ADRs are immutable once the first release is tagged: add a new
  one that supersedes. Plain prose, no emojis, no fluff; claims in docs are checked against the code.
- Fixtures: `protocol/fixtures/valid` lines must pass the schema and the decoder; `invalid` lines must fail both.

## 12. Git, PR and CI

- Never commit to `main` and never force-push. Branch `feat/...` or `fix/...`, commit small logical steps in
  Conventional Commits (`feat(helper): ...`; scopes `helper`, `firmware`, `protocol`, `enclosure`, `docs`, `ci`),
  open a PR from the template. The repo uses the owner's GitHub noreply address: never change `user.email`.
- `main` is protected: PR required (no approvals), branch up to date, conversations resolved, enforced for admins. The
  **seven required check names** must stay exactly: `helper tests (ubuntu-latest)`, `helper tests (macos-latest)`,
  `helper tests (windows-latest)`, `helper lint`, `firmware tests and build`, `firmware format`, `enclosure renders`.
  `firmware tests and build` also renders `docs/screens` and fails if they changed. A weekly run reports new
  advisories. Do not add jobs that become required without telling the owner.
- Do not push, tag or touch GitHub settings unless asked.

## 13. Release process

The owner tags; agents prepare. [CONTRIBUTING.md](CONTRIBUTING.md#releasing) has the steps and the hardware gate. In
short: write `CHANGELOG.md` (`## [X.Y.Z] - date`; `make check-version VERSION=X.Y.Z` must pass), merge through a PR,
run the hardware gate, `git tag -a vX.Y.Z`, push the tag. `release.yml` builds on macOS: a universal macOS helper
(ad-hoc signed), Linux amd64 and arm64, Windows amd64 (untested), the merged firmware image, `install.sh`,
`checksums.txt`; the version comes from the tag. A tag with `-` is a pre-release. Release asset names carry no
version, so `releases/latest/download/<asset>` stays a stable URL: do not rename them.

## 14. Do not

- push to `main`, force-push, tag, or change repository settings;
- raise the SPI speed or the upload speed, or touch DTR/RTS handling casually;
- add radios, settings or sorting to the device, or derive status on it;
- change section 7, or the privacy rules in section 10;
- read conversation content, send paths, or add a network call or telemetry;
- use real project or employer names anywhere in the repo;
- hand-edit STLs, generated screens or the embedded firmware image;
- rewrite an accepted ADR after the first release; add a new one instead;
- add a dependency without need (the helper is one static binary; the core has no Arduino);
- open, flash or monitor the serial port unprompted, or stop the user's service without saying so;
- create files outside the repository, or leave build outputs (`bin/`, `dist/`, `.pio/`) in a commit.
