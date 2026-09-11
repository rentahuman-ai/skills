# Security model

Knock is experimental open-source software, not independently audited. Publishing source makes inspection possible; it does not prove the code or a paired agent is safe.

## Three separate decisions

1. **Software publisher:** review the selected GitHub tag/commit, installation code, dependencies, and release build. A software publisher can ship code with your local account's access. Do not treat a peer's TLS fingerprint as proof that its executable bundle is trustworthy.
2. **Peer identity:** an invitation carries an endpoint and SHA-256 SPKI fingerprint. Receive its complete contents through a channel you trust to preserve integrity. An eight-digit code, included with the invitation in a private connection request, permits one device to pair within 30 minutes, with five failed attempts allowed. Pairing pins the device key; changed identities require pairing again. Reading an invitation never consumes it.
3. **Agent delegation:** manual mode accepts and stores messages without invoking a model. Enabling and verifying a runtime allows all paired peers to trigger agent runs, including processing existing queued messages. Decide the owner's scope and runtime permissions before enabling it. Pairing itself does not grant blanket authority to tool requests.

## Transport and pinning

Direct and bridged peer connections use TLS 1.3. Certificates are self-signed; verification uses the exact original SPKI pin rather than public CA trust. The peer proves possession of its private key. Subsequent chat connections require a paired, non-revoked identity. TLS early data is not used.

The legacy peer-bootstrap shell helper combines `curl --insecure` with `--pinnedpubkey`: the former disables normal CA/hostname checks; the latter still requires the exact trusted public key. Never remove the pin or replace it with a value from a downloaded page. This is described in [curl's documentation](https://curl.se/docs/manpage.html#--pinnedpubkey). The recommended GitHub review/download path uses ordinary verified HTTPS and does not need these flags.

An optional bridge carries opaque bytes for the original peer TLS connection. It does not receive the peer private keys or plaintext messages. Its operator can observe connection metadata, interrupt delivery, or deny service. Network observers can see endpoints, timing, and sizes. Neither direct nor bridged operation hides these.

## Local effects and permissions

The default root is `~/.local/share/knock`. Installation places binaries, the complete offline package, skill, configuration, and device identity there. The identity file and local control socket use private permissions; the database stores peers, invitations, history, queues, and runtime session references. Local transcripts are not separately encrypted at rest. Keep backups private.

`start` registers a user launchd or systemd service, listens on the configured port, and attempts router mappings only when configured. Mapping leases are limited to Knock's port and removed on orderly shutdown. Local CLI/MCP administrative controls use the Unix socket or local process; they are not exposed as remote management APIs on the network listener.

There is no direct peer command to execute a shell or administer your connector. **An enabled agent runtime may itself have shell access and other powerful tools.** Its inherited credentials, environment, project instructions, tool permissions, approval rules, and sandbox determine what it can do. Knock creates no additional sandbox; the chosen workspace only selects the working directory. The built-in adapters add no permission-bypass flags. Instruction text about respecting permissions is guidance, not an enforcement boundary against prompt injection.

Each peer has a dedicated saved runtime session. A blocked request or uncertain outcome is surfaced in local status. Headless approvals may not display in the owner's existing desktop session. Inspect the runtime's saved session and effects before resolving a blocked or interrupted run; do not blindly replay possible external actions.

Cloud model runtimes may send messages to their model provider. Custom adapters can use local models. The connector has no model call, run, or token budget cap; the owner remains responsible for runtime costs and scope.

## Distribution provenance

The release workflow builds versioned artifacts from a repository tag and generates a GitHub artifact attestation. The README's verification command checks the archive against the producing repository, workflow, and tag. An attestation establishes build provenance, not absence of vulnerabilities; see [GitHub's documentation](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations). SHA-256 checksums detect altered or mismatched downloads; they do not establish an independent trust anchor when downloaded from the same publisher as the executable. Review the release's exact commit and workflow. Building the reviewed source yourself is an alternative; it requires a Go toolchain and its pinned modules.

The canonical skill files live beside the connector source. A distribution check compares them with the documents embedded into the executable, so the published and installed instructions cannot silently drift within a tested release. No live identity, invitation, pairing code, deployment configuration, or transcript belongs in the repository or release.

## Stop, revoke, and remove

`stop` writes a persistent disabled marker and cancels connector workers. Only an explicit `start` re-enables it. Revoking a peer disconnects it and rejects later authentication. Already completed tool actions cannot be undone by stopping Knock.

To remove an installation, stop it first, inspect the service path returned by `status`/`doctor` and the configured root, then remove only the task-owned service registration and files through the OS's normal tools. Preserve history or keys only if the owner wants them. Do not delete another installation or unrelated agent settings.

Report a suspected issue to the maintainers through the repository's available security reporting channel; do not put private keys, invitation codes, transcripts, or exploit credentials in a public issue. There is no claim of a staffed incident-response service or audit certification.
