#!/bin/sh
set -eu
[ "$#" -eq 2 ] || { echo 'Usage: install-bridge-server.sh NATIVE_BINARY INVITER_PUBLIC_KEY_PIN' >&2; exit 2; }
knock_binary=$1
knock_owner_pin=$2
case "$knock_owner_pin" in *[!A-Za-z0-9_-]*) echo 'Invalid inviter pin' >&2; exit 1;; esac
[ "${#knock_owner_pin}" -eq 43 ] || { echo 'Invalid inviter pin length' >&2; exit 1; }
if ! id knockbridge >/dev/null 2>&1; then useradd --system --home /var/lib/knock-bridge --shell /usr/sbin/nologin knockbridge; fi
install -m 0755 "$knock_binary" /usr/local/bin/knock
install -d -o knockbridge -g knockbridge -m 0700 /var/lib/knock-bridge
runuser -u knockbridge -- /usr/local/bin/knock --root /var/lib/knock-bridge bridge identity
cat > /etc/systemd/system/knock-bridge.service <<EOF
[Unit]
Description=Knock opaque encrypted connection bridge
After=network-online.target
Wants=network-online.target

[Service]
User=knockbridge
Group=knockbridge
ExecStart=/usr/local/bin/knock --root /var/lib/knock-bridge bridge serve --owner-pin $knock_owner_pin --listen :443 --public-listen :8443
Restart=always
RestartSec=5
UMask=0077
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/knock-bridge
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
LimitNOFILE=1024
MemoryMax=256M

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now knock-bridge
systemctl is-active knock-bridge
