#!/bin/sh
set -eu
[ "$#" -eq 2 ] || { echo 'Usage: bootstrap.sh ORIGINAL_INVITE_URL INSTALL_ROOT' >&2; exit 2; }
knock_link=$1
knock_root=$2
case "$knock_link" in https://*/invite/*/SKILL.md\#v=1\&spki=*) ;; *) echo 'Invalid invitation format' >&2; exit 1;; esac
knock_pin=${knock_link##*spki=}
case "$knock_pin" in *[!A-Za-z0-9_-]*) echo 'Invalid public key fingerprint' >&2; exit 1;; esac
[ "${#knock_pin}" -eq 43 ] || { echo 'Invalid fingerprint length' >&2; exit 1; }
knock_base=${knock_link%/SKILL.md#*}
knock_curl_pin="sha256//$(printf '%s=' "$knock_pin" | tr '_-' '/+')"
knock_os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) knock_arch=arm64;; x86_64|amd64) knock_arch=amd64;; *) echo 'Unsupported architecture' >&2; exit 1;; esac
case "$knock_os" in darwin|linux) ;; *) echo 'Only macOS and Linux are supported' >&2; exit 1;; esac
umask 077
knock_tmp=$(mktemp -d)
trap 'rm -rf "$knock_tmp"' EXIT HUP INT TERM
knock_name="knock-$knock_os-$knock_arch"
knock_fetch() { curl --noproxy '*' --fail --silent --show-error --insecure --pinnedpubkey "$knock_curl_pin" --proto '=https' --connect-timeout 10 --max-time 300 "$knock_base/$1" -o "$2"; }
knock_fetch checksums.txt "$knock_tmp/checksums"
knock_fetch "$knock_name" "$knock_tmp/knock"
knock_expected=$(awk -v n="$knock_name" '$2 == n { print $1 }' "$knock_tmp/checksums")
case "$knock_expected" in *[!a-f0-9]*|'') echo 'Invalid expected checksum' >&2; exit 1;; esac
[ "${#knock_expected}" -eq 64 ] || { echo 'Invalid checksum length' >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then knock_actual=$(sha256sum "$knock_tmp/knock" | awk '{print $1}'); else knock_actual=$(shasum -a 256 "$knock_tmp/knock" | awk '{print $1}'); fi
[ "$knock_actual" = "$knock_expected" ] || { echo 'Downloaded binary checksum mismatch' >&2; exit 1; }
chmod 700 "$knock_tmp/knock"
"$knock_tmp/knock" --root "$knock_root" bootstrap "$knock_link"
