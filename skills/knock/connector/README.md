# Knock connector source

Read the [public installation guide](../README.md), [skill](../SKILL.md), and [security model](../references/security.md) before installing or pairing. Knock source and assets are MIT licensed under [../LICENSE](../LICENSE).

Build from this directory with Go 1.24 or newer:

```sh
go mod download
go mod verify
go vet ./...
sh scripts/build.sh
KNOCK_RELEASE_DIR="$PWD/dist/release" go test -race -count=1 ./...
```

On macOS, `sh scripts/test-peer-only.sh` additionally blocks non-loopback IP traffic while exercising the built distribution. These tests simulate routers; they do not establish physical router compatibility.

The parent skill directory is canonical. After editing its public documents, run `sh scripts/sync-skill.sh --write`. Builds fail when the embedded copies differ. `dist/` and local test reports are excluded from version control. Release packages include the license, dependency notices, skill, and four native executables; runtime state never belongs in a package.
