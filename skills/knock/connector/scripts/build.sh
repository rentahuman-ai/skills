#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
sh scripts/sync-skill.sh --check
mkdir -p dist/release
for target in darwin-arm64 darwin-amd64 linux-arm64 linux-amd64; do
  GOOS=${target%-*} GOARCH=${target#*-} CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o "dist/release/knock-$target" ./cmd/knock
done
knock_os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) knock_arch=arm64;; *) knock_arch=amd64;; esac
"dist/release/knock-$knock_os-$knock_arch" release dist/release
"dist/release/knock-$knock_os-$knock_arch" skill dist/skill/knock
knock_version=$("dist/release/knock-$knock_os-$knock_arch" --version | sed -n 's/.*"version": "\([^"]*\)".*/\1/p')
knock_stage=$(mktemp -d)
trap 'rm -rf "$knock_stage"' EXIT HUP INT TERM
cp ../SKILL.md ../README.md ../LICENSE "$knock_stage/"
cp -R ../references "$knock_stage/references"
mkdir -p "$knock_stage/dist"
cp -R dist/release dist/skill "$knock_stage/dist/"
COPYFILE_DISABLE=1 tar -czf "dist/knock-v$knock_version-release.tar.gz" -C "$knock_stage" SKILL.md README.md LICENSE references dist
(
  cd dist
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "knock-v$knock_version-release.tar.gz" > SHA256SUMS
  else
    shasum -a 256 "knock-v$knock_version-release.tar.gz" > SHA256SUMS
  fi
)
printf 'Release bundle: %s/dist/release/bundle.tar.gz\n' "$(pwd)"
printf 'Standalone installation package: %s/dist/knock-v%s-release.tar.gz\n' "$(pwd)" "$knock_version"
