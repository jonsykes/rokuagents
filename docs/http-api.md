# HTTP API, version 1

The contract between `managents serve` (the helper, on your computer) and any display that pulls state over the
network instead of listening on USB serial, such as the Roku channel in `roku/`.

The body of `/state` is exactly the serial protocol's `state` frame (see [protocol.md](protocol.md#state)), without the
trailing newline. Everything protocol.md says about fields, ordering, names and status meanings holds here too.
A display that can render a serial frame can render this.

## What the helper promises

- It serves only status metadata: folder names, session titles, statuses, ages and context usage. No paths, prompts
  or messages. Titles are the ones the Claude app shows; `--titles=false` leaves them out.
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
- When polls keep failing, look for the helper with [discovery](#discovery), since the computer's address may have
  changed.

## Discovery

A display that cannot reach the helper at its configured address can find it on the LAN. The helper listens for UDP
on the port with the same number as its HTTP port (8787 by default), on every interface, so it receives broadcasts.
`--discovery=false` turns this off. It is always off without a token or with a token shorter than 32 characters,
because every device on the network sees the token's id, and a reply is keyed with the token: both would let someone
test guesses at a short token offline. `openssl rand -hex 16` and the menu bar app make 32-character random tokens.

The exchange is one datagram each way, plain ASCII, fields separated by single spaces.

**Request**, sent by the display to `255.255.255.255:<port>` (and, if it likes, its subnet's broadcast address):

```
rokuagents-discover v1 <token id> <nonce>
```

- `<token id>` is the first 16 hex characters of SHA-256 of the token.
- `<nonce>` is 16 to 64 random hex characters, new for every request.

**Reply**, sent by the helper unicast to the request's source address and port:

```
rokuagents v1 <base url> <mac>
```

- `<base url>` is where to poll, such as `http://192.168.1.20:8787`. Its host is the address `serve` listens on, or,
  when it listens on every interface, this computer's address on the route back to the display.
- `<mac>` is the first 32 hex characters of HMAC-SHA256, keyed with the token, of `<nonce> <base url>` (the nonce,
  one space, the base url).

The helper replies only when the token id matches its own token and the request is well formed, and at most 10 times
a second. Anything else gets no reply at all, so a scanner learns nothing about the port. The display must check
`<mac>` before using `<base url>`: the token is what proves the reply came from its own helper, so another device on
the network cannot redirect the display, and its bearer token, to itself.

Worked example with the token `00112233445566778899aabbccddeeff` and the nonce `0a1b2c3d4e5f6789`:

```
rokuagents-discover v1 5947d7c33d783f94 0a1b2c3d4e5f6789
rokuagents v1 http://192.168.1.20:8787 74fe2a399359ebb7cea3878bb17d357c
```

The Roku channel broadcasts after 3 seconds without a good poll, at most every 5 seconds, waits 1.5 seconds for a
reply, and remembers the address it found for its next start.
