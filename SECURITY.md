# Security policy

## Supported versions

Only the latest release gets security fixes. Upgrade with
`brew upgrade --cask brewery`.

## Reporting a vulnerability

Please don't open a public issue. Report it privately instead, from the
repository's **Security** tab: **Report a vulnerability**
(<https://github.com/dillo/brewery/security/advisories/new>).

Include the brewery version (`brewery --version`), your macOS and Homebrew
versions, and the steps that show the problem. You should hear back within a
week. Once a fix is released, the advisory is published with credit to you,
unless you'd rather not be named.

## Verifying a release

Each release's `checksums.txt` is signed with Sigstore keyless signing, by
the Release workflow. With [cosign](https://github.com/sigstore/cosign):

```sh
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/dillo/brewery/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
shasum -a 256 --check --ignore-missing checksums.txt
```
