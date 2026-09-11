#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
knock_stage=$(mktemp -d)
trap 'rm -rf "$knock_stage"' EXIT HUP INT TERM
cp ../SKILL.md ../README.md ../LICENSE "$knock_stage/"
cp -R ../references "$knock_stage/references"
mkdir "$knock_stage/scripts"
cp ../scripts/bootstrap.sh "$knock_stage/scripts/"
case "${1:---check}" in
  --check) diff -ru "$knock_stage" internal/knock/assets ;;
  --write)
    rm -rf internal/knock/assets
    mkdir -p internal/knock/assets
    cp -R "$knock_stage/." internal/knock/assets/
    ;;
  *) echo 'Usage: sync-skill.sh [--check|--write]' >&2; exit 2 ;;
esac
