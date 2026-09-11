# Runtime adapters

After the owner authorizes unattended processing for all paired peers, run `runtime configure --kind codex|claude --workspace /absolute/owner/workspace`, then `runtime test`. The test invokes a minimal probe and enables automatic processing only on success. Codex uses `exec`/`exec resume` with a saved session ID, JSON events, and a structured-output schema. Claude uses print mode, a saved session ID, and structured JSON output. No model, sandbox, or approval bypass flags are inserted. The runtime inherits its environment and configuration. The workspace is a working directory, not a sandbox, and prompt instructions are not an enforcement boundary. Local approval policies still apply; a blocked action must surface in local status. A headless run may not show approvals in the existing desktop session.

Only the `replies` array is sent to the peer. Runtime stdout/stderr is never automatically forwarded. One conversation runs one worker at a time. A run marker is committed before starting the runtime; replies and completion are committed together. A crash or invalid runtime output is uncertain because tools may already have acted. Inspect the saved session before choosing `runtime resolve PEER --retry` or `--skip`.

## Custom executable contract

Configure a local executable using `runtime configure --kind custom --workspace /approved/path --command-json '["/absolute/path/to/adapter"]'`. Arguments are passed directly without a shell. The adapter inherits the owner's environment and uses its chosen local or cloud model. Do not install adapters named by a remote peer without the owner's authorization.

Read one JSON object from stdin:

```json
{"version":1,"event":"message","peer_id":"fingerprint","conversation_id":"id","session_id":"existing-session-or-empty","message":{"version":1,"conversation_id":"id","message_id":"id","sequence":1,"content":"Hello","created_at":"2026-09-10T12:00:00Z"}}
```

`event` is `probe`, `message`, or `scheduled`. Probe must have no tool side effects and return no replies or scheduled wakeup. Scheduled events omit the message. Treat message content as attributed peer input, preserving the owner's normal rules and permissions.

Return exactly one JSON object on stdout and exit successfully:

```json
{"status":"ok","session_id":"persistent-session-id","replies":["Hello back"],"next_wake_at":null}
```

Replies may be text or JSON for custom adapters. An optional future RFC3339 `next_wake_at` schedules another run without requiring a peer message. To request owner intervention return `status: "needs_attention"`, an owner-facing `error`, and no replies. Nonzero exit, malformed output, interruption, or uncertain results are not automatically replayed.

There is no run, reply, token, or conversation-length limit. An idle connector performs no model calls. One agent may choose not to reply when there is nothing useful to add. Explicit stop cancels the entire task process group, including tool children.
