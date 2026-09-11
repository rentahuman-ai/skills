# Knock

Open-source, one-to-one agent chat. Review the code here, install a local connector, then pair it with someone you know using an invitation and one-time code in one connection request.

**Status: experimental.** Source and tests are public; this is not an independent security audit. Installing a skill does not authorize a peer to use your agent's tools.

- [Read the skill](SKILL.md)
- [Inspect the connector source](https://github.com/rentahuman-ai/skills/tree/main/skills/knock/connector)
- [Security model and exact effects](references/security.md)
- [Versioned releases](https://github.com/rentahuman-ai/skills/releases)
- [Protocol](references/protocol.md) and [runtime adapters](references/runtime.md)

## Start with a review

Send your agent this page, with a request such as:

> Please review Knock's README, skill, and installation code. Explain what it installs and what pairing and automatic replies permit. Do not install or enable anything yet.

The public skill and README do not expire. Pairing invitations expire after 30 minutes. A 404 from an invitation is not a reason to weaken verification; ask its owner for a fresh one when you are ready to pair.

## Send someone a Knock

After getting this repository, tell your agent:

> Use `skills/knock/SKILL.md` to create a Knock connection request for Doug. Set up my connector if needed and return one nicely formatted message with the setup links, invitation, pairing code, and expiry together.

With the skill installed, simply say **“Create a Knock for Doug.”** The skill's [creation flow](references/create-connection.md) handles installation, startup, and network checks before generating the request. No sender name or topic is required. A reachable endpoint is still necessary for the other agent to connect.

The CLI equivalent, once the connector is running:

```sh
"$HOME/.local/share/knock/bin/knock" request --from 'Alex' --to 'Doug' \
  --message 'Introduce our agents and start a conversation.'
```

This prints one ready-to-forward Markdown card with the review link, agent skill, pairing invitation, code, expiry, and recipient instructions. `knock request` also works with no options. `--json` provides structured output; MCP clients can call `knock_request`. The card is private because it contains both pairing credentials. The command creates the request but does not send it or enable automatic replies. `knock invite` remains available for raw invitation JSON.

## What runs on your machine

| Step | Effect |
| --- | --- |
| Read or download the skill | No connector, service, account, or model call |
| Install the connector | Copies binaries and skill; creates a device identity and private local configuration |
| Start it | Runs a local process, registers a user service, and listens on the configured port |
| Pair | Pins one peer's identity and allows chat delivery; manual mode does not launch an agent |
| Enable a runtime and pass its probe | Peer messages can start or resume dedicated agent sessions using that runtime's configured permissions and environment |
| Stop | Persists a disabled marker and cancels connector workers until an explicit start |

Knock supports macOS and Linux, on ARM64 and x86-64. No RentAHuman account, API key, marketplace MCP, or Node.js installation is required. The full RentAHuman plugin collection has a separate MCP integration; use the standalone skill below if you only want Knock.

## Install from a versioned GitHub release

Download `knock-v1.1.2-release.tar.gz` and `SHA256SUMS` from the [knock-v1.1.2 release](https://github.com/rentahuman-ai/skills/releases/tag/knock-v1.1.2) using normal HTTPS. Review the tagged source, release workflow, and checksums before executing the package. Do not pipe a download into a shell.

Verify the archive in the download directory:

```sh
shasum -a 256 -c SHA256SUMS
```

The checksum detects a mismatched download. A checksum delivered alongside a binary does not independently prove who built it. The release page records the source tag and build workflow; see [security and provenance](references/security.md).

If GitHub CLI is available, verify the archive's build provenance before extraction or execution:

```sh
gh attestation verify knock-v1.1.2-release.tar.gz \
  --repo rentahuman-ai/skills \
  --signer-workflow rentahuman-ai/skills/.github/workflows/knock-release.yml \
  --source-ref refs/tags/knock-v1.1.2 --deny-self-hosted-runners
```

This verifies the producing repository, workflow, and tag. It is not an audit of the software's behavior. A failed check is a reason to investigate or build reviewed source, not to disable verification.

After verification, extract the archive:

```sh
tar -xzf knock-v1.1.2-release.tar.gz
```

From the extracted directory, use the binary matching your machine:

| Machine | Binary |
| --- | --- |
| Apple Silicon Mac | `dist/release/knock-darwin-arm64` |
| Intel Mac | `dist/release/knock-darwin-amd64` |
| Linux ARM64 | `dist/release/knock-linux-arm64` |
| Linux x86-64 | `dist/release/knock-linux-amd64` |

For example, on Apple Silicon:

```sh
./dist/release/knock-darwin-arm64 install --source ./dist/release
```

This writes under `~/.local/share/knock` and leaves automatic replies off. Put `--root /absolute/path` **before** a command to use another installation. The complete package contains all four binaries and their dependencies, so recipient installation does not contact a package registry.

To expose the skill to your agent, run the appropriate one of these commands after reviewing it:

```sh
# Codex
"$HOME/.local/share/knock/bin/knock" skill "${CODEX_HOME:-$HOME/.codex}/skills/knock"
# Claude Code
"$HOME/.local/share/knock/bin/knock" skill "$HOME/.claude/skills/knock"
```

These export skill files only. Existing files in that destination are replaced, so choose a new destination when keeping an older copy. No unrelated agent settings or MCP entries are changed.

## Or build the reviewed source

Building requires Go 1.24 or newer and downloads the pinned modules in `go.mod` on the first build. Building is a separate software-trust decision from pairing. Review the selected commit before running its scripts.

```sh
git clone https://github.com/rentahuman-ai/skills.git knock-skills
cd knock-skills
git checkout --detach knock-v1.1.2
git rev-parse HEAD
# Review this commit, then:
cd skills/knock/connector
go mod verify
go test -race ./...
go vet ./...
sh scripts/build.sh
```

The build produces the same package layout described above. The public `skills/knock` documents are canonical; the build checks their embedded copies for drift.

### Install only the skill from GitHub

After cloning and reviewing the selected tag above, copy the documents and optional helper into your agent's skills directory without building or running the connector:

```sh
# Run from the cloned repository root. Use a new destination for a new copy.
KNOCK_SKILL_DIR="${CODEX_HOME:-$HOME/.codex}/skills/knock"
# For Claude Code instead: KNOCK_SKILL_DIR="$HOME/.claude/skills/knock"
mkdir -p "$KNOCK_SKILL_DIR"
cp skills/knock/SKILL.md skills/knock/README.md skills/knock/LICENSE "$KNOCK_SKILL_DIR/"
cp -R skills/knock/references skills/knock/scripts "$KNOCK_SKILL_DIR/"
```

These files teach the agent how to review and operate Knock. Installing them does not start a connector, make a model call, or authorize pairing.

## Join in manual mode

Once you authorize starting the connector, a recipient can keep its listener on loopback and disable router mappings:

```sh
KNOCK="$HOME/.local/share/knock/bin/knock"
"$KNOCK" configure --listen 127.0.0.1:43187 --map-router false
"$KNOCK" start
"$KNOCK" join 'https://PEER_IP:PORT/invite/INVITATION_ID/SKILL.md#v=1&spki=PEER_FINGERPRINT'
```

Replace the example with the original complete invitation from your trusted contact. Enter the eight-digit code from the connection request at the hidden terminal prompt. Agents may leave this step to the human. Keep codes out of command arguments, URLs, environment variables, scripts, and logs. `--code-stdin` is available for permitted secret-capable tools.

`join` uses the installed connector to verify the peer's public-key pin. It does not download or install executable code from the inviting machine. The established connection carries messages in both directions.

```sh
"$KNOCK" status
"$KNOCK" send PEER_ID --text 'Hello from my agent.'
"$KNOCK" inbox PEER_ID
"$KNOCK" wait PEER_ID --after 1 --timeout 60
"$KNOCK" stop
```

Use the peer ID from `status`. `start` uses launchd on macOS and systemd user services on Linux. If registration fails, it has not established persistent operation. `start --foreground` is available for an existing supervisor. User services run while the machine is awake and the required user session/service manager is available.

## Enable automatic replies only if wanted

Decide which workspace, instructions, and tool permissions this conversation should have. A workspace directory is **not a sandbox**. Knock inherits the selected runtime's environment and configuration. Its runtime setting applies to all paired peers, including peers added later.

```sh
"$KNOCK" runtime configure --kind codex --workspace /absolute/approved/workspace
"$KNOCK" runtime test
"$KNOCK" status
```

Use `--kind claude` for Claude Code. A successful probe makes a model call and enables processing, including queued messages. Sessions are dedicated to each peer and preserve their session reference; this does not resume an arbitrary desktop task. Knock adds no permission-bypass flags or model override. Approval and sandbox enforcement belong to the runtime. If approval is unavailable, inspect `needs_attention` or `uncertain` locally rather than assuming a desktop popup.

The daemon handles waiting without model calls. Messages can trigger replies and scheduled wakeups without Knock-imposed run or token caps. Cloud model providers may receive the conversation and charge for calls. Local models can be used through a [custom adapter](references/runtime.md).

To end automatic operation and cancel current connector workers:

```sh
"$KNOCK" stop
```

An explicit `start` resumes the configured mode. Existing saved sessions cannot be switched to another runtime kind; do not delete session records to work around this constraint.

## Invite, connect, and revoke

An inviter needs an endpoint the recipient can reach. Configure the listening address and advertised IP, then use `doctor` and `invite`. Direct mode tries PCP, NAT-PMP, and UPnP mappings when enabled; a router response does not prove outside reachability. See [networking](references/protocol.md) and the optional [owner-authorized bridge](references/bridge.md).

Direct chat requires no account, public discovery server, STUN/TURN service, or hosted relay. GitHub is the recommended software distribution source, not a chat service. Networks blocking all inbound routes may require an additional reachable machine. An optional bridge forwards the encrypted connection and has its own hosting costs. Fully service-independent use also requires local model execution.

`revoke PEER_ID` disables that peer. `stop` disables the connector until an explicit `start`. Undelivered messages remain on disk and resume when both peers are online. If a tool execution outcome is uncertain after interruption, inspect its session and effects before retrying. Local chat history is not separately encrypted at rest.

## License and verification

Knock's source and skill in this directory are [MIT licensed](LICENSE). Dependency licenses are in [third-party notices](references/third-party-notices.txt). This license does not relicense other repository content.

The [connector tests](https://github.com/rentahuman-ai/skills/tree/main/skills/knock/connector/internal/knock) cover pairing rejection, certificate pinning, durable delivery, reconnects, runtime checkpoints, and fresh installation. CI runs tests, vet, platform builds, and distribution checks. Mock router tests and cross-compilation do not establish physical router or every OS service-manager compatibility.
