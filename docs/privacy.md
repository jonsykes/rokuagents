# Privacy

The helper reads Claude Code's session files (`~/.claude/sessions`), the **end** of each open session's transcript,
OpenCode's database (read-only), Codex's thread list (read-only) and the end of each open Codex thread's log, and
the list of running processes. From the transcripts and logs it decodes only status fields, timestamps and token
counts; conversation text is never decoded, stored, logged or sent.

The only thing that leaves the computer is what goes down the USB cable: each session's folder name, status, age and
context-window usage. There is no network access and no telemetry, and the display has no radio. Tests enforce the
text part.

## Network displays (rokuagents)

`managents serve` is the one exception to the above, and it only runs if you start it. It listens on the address you
give it and answers HTTP requests carrying its bearer token with each Claude Code session's folder name, title,
status, age and context usage. The title is the one the Claude app shows in its sidebar: the one you set, otherwise
the one Claude Code generated from the conversation. To read it, `serve` scans each open transcript for title entries
and decodes only their title fields. Leave titles out with `--titles=false`.

The traffic is plain HTTP, so anyone on your network who has the token, or can watch the traffic, can read it, and
the titles are visible to anyone who can see the TV. `managents run` and the USB display never read titles.

`serve` also answers LAN discovery requests on the UDP port with the same number, so the Roku channel can find it
after your computer's address changes ([the exchange](http-api.md#discovery)). It answers only requests that name its
token (by a hash of it), and the reply holds only the address to poll and a signature made with the token. No session
data travels over UDP. Anyone who can watch your network can see the token's hash in the TV's broadcasts and your
computer's address in the reply. For a random 32-character token, as the setup makes, neither helps them guess the
token, so they cannot read session state. With a shorter token, discovery stays off on both ends, since the hash
would let them test guesses offline. `--discovery=false` turns it off.

The full list of what is read and sent, how releases are built and how to report a problem: [SECURITY.md](../SECURITY.md).
