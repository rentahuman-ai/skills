# Optional installation from the inviting peer

Prefer the reviewed GitHub release or source in the [README](../README.md). Use this alternative only when the owner explicitly chooses to trust the inviter as the software publisher, for example for installation without a package registry or GitHub access. Pinning proves which peer supplied the files; it does not prove the supplied code is safe.

Review the received instructions and bootstrap script before execution. The original human-delivered invitation fingerprint is the trust anchor. Never replace it with one from the server or silently follow redirects.

For `https://IP:port/invite/ID/SKILL.md#v=1&spki=PIN`, translate the 43-character base64url PIN by replacing `_` with `/` and `-` with `+`, append `=`, and prefix `sha256//`. With `CURL_PIN`, `INVITE_BASE`, `TEMP_SCRIPT`, `ORIGINAL_INVITE_URL`, and `INSTALL_ROOT` set from the trusted invitation and owner's chosen local paths:

```sh
curl --noproxy '*' --fail --silent --show-error --insecure \
  --pinnedpubkey "$CURL_PIN" --proto '=https' \
  "$INVITE_BASE/scripts/bootstrap.sh" -o "$TEMP_SCRIPT"
# Inspect the downloaded script before the authorized installation:
sh "$TEMP_SCRIPT" "$ORIGINAL_INVITE_URL" "$INSTALL_ROOT"
```

Do not use `--insecure` alone. The paired pin check remains mandatory. The helper verifies download checksums, installs the native connector and complete package, and prints the executable path. It does not start the service or enable an agent runtime. Continue with the README's separately authorized pairing steps.
