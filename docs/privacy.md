# Privacy

The helper reads Claude Code's session files (`~/.claude/sessions`), the **end** of each open session's transcript,
OpenCode's database (read-only) and the list of running processes. From the transcripts it decodes only status
fields, timestamps and token counts; conversation text is never decoded, stored, logged or sent.

The only thing that leaves the computer is what goes down the USB cable: each session's folder name, status, age and
context-window usage. There is no network access and no telemetry, and the display has no radio. Tests enforce the
text part.

The full list of what is read and sent, how releases are built and how to report a problem: [SECURITY.md](../SECURITY.md).
