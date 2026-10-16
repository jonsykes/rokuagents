# Serial protocol, version 1

The contract between the **helper** (computer) and the **display** (firmware). Machine-readable versions:
[`protocol/schema/`](../protocol/schema). Shared test cases: [`protocol/fixtures/`](../protocol/fixtures) — the helper
validates them against the schema and the firmware decodes them, so both sides are tested against the same files.

## Transport

- USB serial, **115200 8N1**. The helper keeps **DTR and RTS low**: on ESP32 boards they drive the auto-reset circuit.
- UTF-8 text, **one JSON object per line**, terminated by `\n` (a preceding `\r` is ignored).
- Maximum line length **4096 bytes**, not counting the terminating `\n`. Receivers discard longer lines and
  resynchronise at the next `\n`.
- Every message has `"v"` (protocol version) and `"t"` (type). Receivers ignore unknown types, unknown fields, and
  messages with another `v`.
- Anything that is not a valid message (boot log, noise, partial lines) is ignored silently.

## Host → device

### `state`

Sent every 2 seconds, and at the next poll (≤ 1 s) when anything visible changes.

```json
{
  "v": 1,
  "t": "state",
  "now": 1791560000,
  "tz": 7200,
  "agents": [
    {
      "id": "claude:48211",
      "kind": "claude",
      "name": "managents",
      "status": "working",
      "age": 42,
      "ctx": { "used": 88000, "limit": 200000 }
    }
  ],
  "more": 0
}
```

| Field | Type | Meaning |
|---|---|---|
| `now` | int | Host UNIX time in seconds. The display shows the clock from `now + tz` and advances it locally. |
| `tz` | int, optional | Host UTC offset in seconds, DST applied. Default 0. |
| `agents` | array, ≤ 24 | **Already filtered, named and ordered** by activity: working sessions first, then the most recent status change first. The display never sorts, renames or derives status; it shows them nine per page. |
| `more` | int ≥ 0, optional | Agents left out of `agents`. Shown as a `+N` badge (capped at 65535 on the display). Default 0. |

Agent fields:

| Field | Type | Meaning |
|---|---|---|
| `id` | string, 1 to 48 bytes | Stable for the life of the session: `<kind>:<pid>`, or `<kind>:<session id>`. Empty ids are rejected; the helper truncates longer ones at a UTF-8 boundary. |
| `kind` | string | `claude`, `opencode` or `codex`. Other values get a generic logo. |
| `name` | string, ≤ 64 bytes | Display name, already unique (`name`, `parent/name`, `name #2`). See [Names](#names). |
| `path` | string, optional | **Reserved and ignored.** The helper does not send directory paths. The schema still accepts the field, so older helpers' frames stay valid. |
| `status` | enum | `working`, `waiting`, `error`, `idle` (see below). |
| `age` | int ≥ 0 | Seconds in the current status. The display advances it locally and formats it (`42s`, `5m`, `1h 12m`, `2h`, `1d 2h`, `3d`). |
| `ctx` | object or null, optional | Context-window usage: `used` tokens (int ≥ 0) and `limit` (int ≥ 1, or null when unknown). The card shows a fill bar along its bottom edge when the limit is known, and nothing otherwise. The bar is the only display of the context, never numbers. |

`age`, `ctx.used` and `ctx.limit` above 4294967295 saturate at that value on the display.

### Names

The helper sends the folder name, made unique. The display makes it fit:

1. **Folded to ASCII.** The fonts have printable ASCII only. Latin-1 letters lose their accent (`é` becomes `e`,
   `ß` becomes `ss`, `æ` becomes `ae`); every other character becomes `?`.
2. **Fitted.** The name is kept whole when it can be: on one line in the largest font that holds it, else on two
   lines in the largest font that holds them, preferably broken after `-`, `_`, `/`, `.` or a space.
3. **Middle ellipsis, last resort.** Only a name that does not fit on two lines in the smallest font is shortened
   with `...` in the middle of its second line, keeping the end, which often tells similar names apart.

Status meanings:

| Status | Meaning | Card |
|---|---|---|
| `working` | the agent is running a turn | green |
| `waiting` | blocked on the user, or finished and waiting for the next prompt | yellow |
| `error` | the last turn ended in an API/model error | red; blinks at 1 Hz during the first minute of the error, then stays red |
| `idle` | open but no activity for 2 hours | dark, dimmed |

A frame is validated as a whole: if any part is invalid (wrong type, unknown status, negative age, empty `id`,
`ctx.limit` below 1…) the display ignores the entire frame and keeps the previous one.

### `hello`

```json
{ "v": 1, "t": "hello" }
```

A probe. The display answers with its own `hello`.

## Device → host

### `hello`

```json
{ "v": 1, "t": "hello", "device": "managents", "fw": "0.2.0", "board": "e32r40t", "w": 480, "h": 320, "proto": 1 }
```

Sent at boot and in reply to a probe. `fw` is the firmware version, taken from the git tag it was built from
(`0.0.0-dev` for a build outside a release). `w`/`h` are the screen size in the current rotation. The helper treats a
port as a display only after receiving this message (within 3 s, probing once per second).

## Behaviour

| Situation | Display | Helper |
|---|---|---|
| Normal operation | Draws the agents received, in order, nine per page. A tap anywhere shows the next page; after 30 s without a tap it returns to the first. Without touch (E32N40T build), pages turn by themselves. | Sends `state` every 2 s and on change. |
| No agents | Shows "No agents open" and the clock. | Sends `state` with `"agents": []`, never silence. |
| Boot, nothing heard yet | **Connecting**: "Connecting to your computer...". | — |
| Nothing heard for 15 s since boot (no `state`, no `hello` probe) | **Setup needed**: "Waiting for the helper" with a QR code to [the setup steps](https://github.com/tonylook/managents#get-started). Any `state` or `hello` replaces it at once. | Not running, or not installed. |
| Frames arrived once, then none for 6 s | **Reconnecting**: "Reconnecting... Is the computer asleep?". The next valid `state` replaces it. | Asleep, stopped, or the cable is out. |
| Display unplugged | — | Keeps running; rediscovers ports every 2 s. |
| Several displays | — | Sends to all of them. |
| A detector fails | — | Skips that source, logs once, keeps sending the rest. |
| Nobody needs the screen | The backlight dims (`MANAGENTS_BACKLIGHT_DIM`) after 1 minute of Reconnecting, or 5 minutes with no agents or only idle ones. A tap wakes it. Connecting and Setup needed never dim. | — |

The RGB LED follows the most urgent status: red (blinking during an error's first minute) for `error`, orange for
`waiting`, green for `working`, off when everything is idle, dim blue while the host is gone.

## Versioning

- Backwards-compatible additions (optional fields, new message types) keep `v: 1` and are documented here.
- A breaking change bumps `v`. The helper must keep speaking the version a display announces in `proto`.
- **Product versions move in lockstep.** The helper embeds the firmware of its own release, so one release number
  names both. The helper's `MinFirmware` (in `helper/internal/protocol/version.go`) is the oldest firmware it fully
  supports; it warns, in the log and in `managents devices`, when the display is older and says to run
  `managents flash`. Raise it only when the helper starts relying on something newer firmware does. A display that
  announces a newer `proto` than the helper speaks makes the helper log that it should be updated.

Planned additions for touch and sound are listed in the [roadmap](roadmap.md).
