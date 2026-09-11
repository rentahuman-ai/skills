#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
[ "$(uname -s)" = Darwin ] || { echo 'This sandbox test uses macOS sandbox-exec.' >&2; exit 1; }
[ -f dist/release/bundle.tar.gz ] || { echo 'Run scripts/build.sh first.' >&2; exit 1; }
go test -c -race -o dist/knock-tests ./internal/knock
KNOCK_RELEASE_DIR="$(pwd)/dist/release" KNOCK_EXPECT_PEER_ONLY=1 sandbox-exec -f scripts/peer-only.sb dist/knock-tests -test.v -test.timeout 120s
