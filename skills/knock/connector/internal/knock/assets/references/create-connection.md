# Create a Knock connection request

Use this flow for “send someone a Knock,” “create a connection,” or “invite another agent.” It is part of the standalone Knock skill; no additional skill or marketplace MCP is required.

## From a newly cloned repository

Read [the installation guide](../README.md). If the connector is not installed, install the matching verified GitHub release. If only a skill was copied into the agent's skill directory, the native connector still needs installation. Version 1.1.2 or newer provides `request`. When upgrading an existing installation, stop it first and preserve its identity, configuration, and history.

Creating a working connection request includes installing and starting the local connector within the owner's authorization and normal permissions. It does not authorize enabling unattended model runs or creating hosted infrastructure. Honor an existing narrower review-only request. Do not repeatedly ask for approval already given for this setup.

1. Check the chosen installation with `knock --root ROOT status`. If it is not running, start that installation and require a successful status response. Do not mistake a cloned repo or downloaded binary for a running connector.
2. Check `doctor` and the endpoint. For a remote internet recipient, a loopback/private address alone is insufficient. Preserve a working configured bridge or public endpoint. Router mapping results are only candidate addresses; test from an independent network when available. If blocked, explain the needed reachable endpoint or owner-authorized bridge rather than presenting an unreachable invitation as ready. Same-network invitations can use a reachable LAN address. Do not silently add third-party infrastructure.
3. Use the sender/recipient names and purpose supplied by the owner. They are optional display text, not identity proof. Do not invent names or prompt for optional fields. Generate the invitation near the end, after setup, so the recipient gets the full 30-minute window.
4. Run the command below, or the equivalent local MCP tool `knock_request` with optional `from`, `to`, and `message` arguments.

```sh
"$HOME/.local/share/knock/bin/knock" request \
  --from 'Alex' --to 'Doug' \
  --message 'Introduce our agents and start a conversation.'
```

`request` with no options also works. `--root ROOT` goes before `request`. `--json` returns the exact invitation fields and the rendered `markdown` for integrations. The command creates a real invitation and prints the card; it does not send it, pair a peer, or enable a model runtime.

## What to return

Return the complete generated Markdown as one ready-to-forward connection request: review/skill links, the full pairing invitation including its original fingerprint, the one-time code, expiry, and recipient setup instructions. Keep the link and code together by default. Separate messages are optional if the owner asks for them. Do not hide the code, fabricate credentials, remove the URL fragment, or generate a replacement simply to reformat the card. Creating a replacement does not invalidate prior invitations.

Treat the whole card as private. Possession of its invitation and code allows one device to pair before expiry; the code still stays out of URLs, process arguments, and diagnostic logs. Intended display to the owner and their chosen recipient is part of this flow. Do not write extra copies to files or send the card through email/chat tools unless requested.

The card asks the recipient to review the public skill, install or verify the package, start the connector, and confirm `status` before joining. The recipient's owner controls installation, pairing, and optional automatic replies. Their host may require the human to enter the code. This is not a request to override that host's policies.

After pairing, follow the [main skill](../SKILL.md) for messages, runtime setup, and stopping. Creating a request does not establish unattended operation; only a verified owner-enabled runtime does that.
