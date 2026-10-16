# Releasing brewery

Releases are built by GoReleaser in GitHub Actions when a `v*` tag is pushed
(`.github/workflows/release.yml`). It publishes darwin arm64 and amd64
binaries as a GitHub release, and writes the cask to `dillo/homebrew-tap`.

Before tagging, `make snapshot` builds the same release into `dist/` without
publishing anything.

## The first release

1. On GitHub, create two public repositories: `dillo/brewery` and
   `dillo/homebrew-tap`. The tap can start empty.
2. Create a fine-grained personal access token with **Contents: read and
   write** on `dillo/homebrew-tap` only. In `dillo/brewery`, add it as the
   Actions secret `HOMEBREW_TAP_GITHUB_TOKEN`. Fine-grained tokens expire:
   when it does, a tag still makes the GitHub release but no cask, so renew
   the token and re-run the Release workflow.
3. Push the code, and check that the CI workflow passes:

   ```sh
   git remote add origin git@github.com:dillo/brewery.git
   git push -u origin master
   ```

4. Tag the release, and check that the Release workflow passes and
   `Casks/brewery.rb` appears in `dillo/homebrew-tap`:

   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```

5. In a new shell, install it as a user would:

   ```sh
   brew install dillo/tap/brewery
   brewery --version   # brewery 0.1.0
   brewery
   ```

## Later releases

Tag `vX.Y.Z` on `master` and push the tag.
