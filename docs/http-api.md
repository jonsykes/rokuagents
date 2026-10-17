# HTTP API, version 1

The contract between `managents serve` (the helper, on your computer) and any display that pulls state over the
network instead of listening on USB serial, such as the Roku channel in `roku/`.

The body of `/state` is exactly the serial protocol's `state` frame (see [protocol.md](protocol.md#state)), without the
trailing newline. Everything protocol.md says about fields, ordering, names and status meanings holds here too.
A display that can render a serial frame can render this.

## What the helper promises

- It serves status metadata: folder names, session titles, activity lines, statuses, ages and context usage. No
  paths, prompts or tool output. Titles are the ones the Claude app shows; `--titles=false` leaves them out. An
  activity line (`activity`, at most 100 bytes) says what a working session is doing or what a waiting one needs;
  `--activity=false` leaves it out. A display without room for these fields can ignore them.
- It answers within milliseconds; detection runs in the background every second, never inside a request.
- The frame it serves is at most about 2 seconds old while the helper is healthy.
- It never writes anything on behalf of a client. Every route is read-only.

## Authentication

When the helper was started with `MANAGENTS_TOKEN` set, every route except `/healthz` requires

```
Authorization: Bearer <token>
```

The helper refuses to listen on a non-loopback address without a token.

## Routes

| Method and path | Auth | Success | Failure |
|---|---|---|---|
| `GET /state` | yes | `200`, `application/json`, the latest `state` frame | `401` wrong or missing token; `503` with `Retry-After: 1` before the first frame exists |
| `GET /healthz` | no | `200`, `text/plain`, `ok` | none |

Any other method on these paths answers `405`. Unknown paths answer `404`.

Responses carry `Cache-Control: no-store`.

## What a client should do

- Poll `/state` about once a second, with a timeout of a few seconds.
- Advance `age` and the clock locally between polls, as the serial display does.
- Treat 6 seconds without a successful poll as "Reconnecting", like the serial display.
- Treat `401` as a configuration error to show the user, not something to retry quickly.
- Ignore unknown fields, and frames whose `v` is not `1`.
