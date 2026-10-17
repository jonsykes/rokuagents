# Protocol contract

The machine-readable half of [docs/protocol.md](../docs/protocol.md).

| Path | Content | Checked by |
|---|---|---|
| `schema/host-message.schema.json` | JSON Schema of every helper → display line | helper tests |
| `schema/device-message.schema.json` | JSON Schema of the display's `hello` | helper tests |
| `fixtures/valid/*.jsonl` | Lines the display must accept: every status, empty, 12 + `more`, long and non-ASCII names, unknown kinds and fields, the `showcase` frame the README picture is rendered from, a probe | helper (schema) and firmware (decoder) |
| `fixtures/invalid/*.jsonl` | Lines the display must ignore: broken JSON, missing `t`, `kind`, `status` or `name`, wrong types (a string `v` or `now`, a numeric `t` or `kind`, an `agent` or `ctx` that is not an object, a fractional `age`), values out of range (a negative `more`, `age` or `limit`, `limit` 0, an empty `id`), an unknown status | helper (schema) and firmware (decoder) |

Adding a case here tests both implementations at once. Change the schema and the fixtures together with
`docs/protocol.md`.

Things to know when you edit the contract:

- The schema bounds `id` to 48 bytes and `ctx.limit` to 1 or more (or null). `name` is limited to 64 bytes by the
  helper, not by the schema: JSON Schema counts characters, not bytes. `path` is reserved: the schema accepts it and
  everyone ignores it.
- The firmware test `test_contract` decodes every fixture and also checks that a rejected frame leaves the last good
  one untouched.
- Fixtures and screens use synthetic names only (`payments-api`, `website`, ...), never real project or employer
  names. The README's screenshots are rendered from `valid/showcase.jsonl`.
