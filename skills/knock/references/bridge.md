# Optional bridge

Use this mode only when the owner authorizes a separate internet-reachable server. It adds infrastructure; the default direct mode still uses only the two peers.

The bridge accepts opaque TCP connections and forwards them over an outgoing mutual-TLS WebSocket from the inviter. The original connector TLS stays intact through the bridge. Its public-key pin, installation verification, private keys, pairing codes, and peer authentication remain on the connectors. The bridge can observe connection timing, size, and addresses and can interrupt traffic, but cannot decrypt chat or replace a pinned connector. It must never receive the connector's private key.

On the owner's server, install the matching native binary and run under its service manager:

```sh
knock --root /var/lib/knock-bridge bridge identity
knock --root /var/lib/knock-bridge bridge serve --owner-pin INVITER_DEVICE_PIN --listen :443 --public-listen :8443
```

Obtain the bridge identity pin through the server's authenticated administrative channel. The bridge permits only the configured inviter certificate on its control listener. Port 443 is the inviter's outgoing control/data path; 8443 is the incoming peer path. Both ports must be reachable. Administration remains on the server's separately authenticated channel.

On the inviting connector:

```sh
knock stop
knock bridge configure --endpoint https://BRIDGE_IP:443 --pin BRIDGE_PIN --public https://BRIDGE_IP:8443
knock start
knock doctor
```

Wait for `bridge_connected: true`. Verify the public endpoint from another machine using the original connector pin, then create the invitation. A connected control channel alone does not prove outside reachability. Recipients use the existing skill download, bootstrap and join commands unchanged. They do not need bridge credentials or another account.

The connector supervises the outgoing bridge and reconnects with backoff capped at 60 seconds. `knock stop` cancels bridge connections and agent workers and remains stopped across service restarts. The cloud bridge server remains a separately managed resource, potentially incurring charges until the owner removes it. Both connectors and the bridge must be online for delivery; local outboxes retain disconnected messages.

The server accepts at most 128 concurrent TCP connections to bound resources. This is not a conversation, message, or model-run cap. Normal chat keepalives keep streams active; idle unpaired streams time out.
