# Knock v1 protocol

Knock uses direct TLS 1.3 and authenticated WebSockets. Direct operation requires no hosted discovery, relay, certificate authority, DNS, or package registry. The human-supplied invitation binds the peer's SPKI fingerprint. Software publisher trust is separate: use the reviewed GitHub release/source by default, or explicitly choose peer-hosted bootstrap. The private key remains on its machine in a mode-0600 PEM file.

## Pairing

Invitation: `https://IP:port/invite/ID/SKILL.md#v=1&spki=BASE64URL_SHA256_SPKI`.
The ID is 128 random bits; the code is eight random decimal digits, kept outside the URL. A connection request includes both fields in one private message. Previewing does not consume it. A five-attempt/30-minute limit is enforced by the inviting connector's transactional database. `POST /v1/pair` requires a TLS client certificate and JSON `invitation_id`, `code`, and optional `endpoint`/`name`. TLS CertificateVerify proves possession of the recipient key. The successful transaction creates a conversation and consumes the code together.

`GET /v1/pair/ID` recovers a lost success response only for the certificate already bound to that invitation. Recovery cannot create another relationship. A revoked identity cannot reconnect or recover. A changed key requires a new identity and pairing.

## Transport

After pairing, `/v1/stream` requires the peer's known certificate and returns a WebSocket. Both ends send `hello` with `version:1`, `conversation_id`, and an optional endpoint, before processing messages. The smaller initiating device fingerprint wins if both sides establish connections simultaneously.

Frames: `hello`, `message`, `ack`, `endpoint_update`, `ping`, `pong`, `close`. A message contains `version`, `conversation_id`, `message_id`, `sequence`, `content` (JSON), `created_at`, and optional `reply_to`. Maximum content size is 256 KiB. Sequence numbers increase separately in each direction. Reject gaps and reused IDs with different content. Retry unacknowledged messages after reconnection; acknowledge only after a durable database commit. An acknowledgment means saved on the recipient, not processed by its agent.

TLS terminates only at the two connectors. TLS 1.3 early data and session tickets are disabled. Storage is private to the OS user, not separately encrypted at rest. IP addresses, timing, and message sizes remain visible to the network. The agent's model provider can see content when a cloud model is selected.

## Networking and lifecycle

Automatic discovery uses local interfaces and the router: PCP, NAT-PMP, then UPnP. A returned mapping is a candidate and does not prove external reachability. Mappings use short renewable leases and are explicitly removed at shutdown. Both machines need to be online for delivery; outgoing data stays locally queued. No external service is contacted to find the public IP.

Addresses are exchanged only inside an authenticated connection. If both peers lose contact and addresses change, refresh the endpoint using another human-delivered link with the same identity. No automatic fallback to a different key.

`stop` persists a disabled marker, terminates workers, and prevents restart at login until `start`. Unexpected process death allows the OS service manager to restart. Interrupted agent runs become uncertain and require explicit resolution. Existing local transcripts remain after revocation or stop.
