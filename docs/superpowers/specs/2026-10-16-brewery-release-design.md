# brewery: ready to ship (Plan 8)

Date: 2026-10-16. Status: approved in conversation; becomes Plan 8 on `feat/release`.

## Goal

Make brewery ready for its first public release: a stated Homebrew floor that brewery enforces, a real module path, a licence and README, lint and snapshot tests, CI, and a release pipeline that builds the binaries and writes a cask to the user's tap. This plan publishes nothing. It ends with a checklist for the first release, which the user runs, or Claude runs later on an explicit go-ahead.

## Decisions

- **GitHub owner:** `dillo` (a personal account). Module `github.com/dillo/brewery`; tap repo `dillo/homebrew-tap`; users run `brew install dillo/tap/brewery`.
- **Licence:** MIT, © 2026 Dillo Raju.
- **Packaging:** a Homebrew **cask**, written by GoReleaser's `homebrew_casks`. GoReleaser has deprecated `brews` (formulae), and `goreleaser check` fails on it. The binary isn't signed by Apple, so the cask clears macOS's quarantine attribute when it installs. Homebrew removed `--no-quarantine`, and the official cask tap rejects unsigned casks, but third-party taps may clear quarantine in an install step.
- **Platforms:** darwin arm64 and amd64 only.

## 1. Homebrew version check

- **When:** after `locateBrew` finds brew, before the TUI starts, `main` runs `brew --version` with a 5-second timeout. It takes about 50 ms.
- **Parsing:** the first line looks like `Homebrew 7.0.9-108-gf63eb29`. `brew.ParseVersion(out string) (v Version, ok bool)` reads the first `Homebrew <major>.<minor>[.<patch>]` it finds; `Version{Major, Minor int; Text string}` keeps what brew printed up to the first `-` (`7.0.9`).
- **Floor:** `brew.MinMajor, brew.MinMinor = 7, 0`.
- **Below the floor:** brewery prints `brewery needs Homebrew 7.0 or newer (you have 6.4.2). Update it with ` + "`brew update`" + `.` to stderr and exits 1. The version shown is `Version.Text`.
- **Fails, times out, or doesn't parse:** brewery starts anyway. A check that can't read the version mustn't block brewery, and anything that's really broken shows up in the tabs.
- **Where:** the parsing and comparison live in `internal/brew` (`Version`, `ParseVersion`, `(*Client) BrewVersion(ctx) (Version, error)` that runs `brew --version` and parses it, and `(Version) TooOld() bool` against the floor), with table tests: `7.0.9-108-gf63eb29`, `7.1.0`, `6.4.2`, `10.0.0`, garbage, empty. `main` gets a `checkBrew` seam like `locateBrew`, with tests for "too old exits 1 with the message" and "unknown starts".

## 2. Identity and docs

- **Module rename:** `brewery` becomes `github.com/dillo/brewery` in `go.mod` and every import. `go build`, the tests and `go mod tidy -diff` stay clean.
- **`LICENSE`:** MIT text, `Copyright (c) 2026 Dillo Raju`.
- **`README.md`:**
  - what brewery is (one paragraph)
  - install: `brew install dillo/tap/brewery`
  - requirements: macOS; Homebrew 7.0 or newer; a Nerd Font is optional, or use `--no-icons` / `BREWERY_NO_ICONS`
  - the tabs and the main keys (taken from the help overlay, not invented)
  - where state and cache live (`$XDG_STATE_HOME/brewery`, `$XDG_CACHE_HOME/brewery`, with their defaults)
  - building from source (`make build`, `make test`)

## 3. Quality gates

- **golangci-lint:** `.golangci.yml` (config version 2) with the default linters plus `gofmt`, `misspell`, `errorlint` and `unconvert`. Every finding is fixed, not suppressed, unless a fix would make the code worse. In that case it gets a `//nolint:<linter> // <reason>` on the line. The prototype measures the findings before the plan is written.
- **Snapshot tests:** golden snapshots of the main screens, from the existing fakes, at 120×40 with icons off:
  - Home
  - Installed with the preview
  - Outdated
  - Services
  - Discover (All view)
  - the help overlay

  Snapshots are ANSI-stripped text in `internal/ui/testdata/golden/*.txt`. `go test ./internal/ui -update` rewrites them. They use `teatest` if a v2 module works with Bubble Tea v2.0.10; otherwise a small `golden(t, name, got)` helper. The prototype decides which, and the plan says why.
- **Makefile:** gains `lint` (`golangci-lint run`) and `snapshot` (`goreleaser release --snapshot --clean`).

## 4. CI and release

- **`.github/workflows/ci.yml`:** on push and pull request, on `macos-latest`. Steps:
  - checkout
  - setup-go (version from `go.mod`)
  - `go test -race ./...`
  - `go vet ./...`
  - golangci-lint (official action)
  - `gofmt -l .` must be empty
  - `go mod tidy -diff`

  Live tests aren't run there.
- **`.github/workflows/release.yml`:** on tags `v*`, on `macos-latest`. It runs GoReleaser with `GITHUB_TOKEN` and `HOMEBREW_TAP_GITHUB_TOKEN`.
- **`.goreleaser.yaml`:**
  - builds `./cmd/brewery` for darwin arm64 and amd64 with `CGO_ENABLED=0` and `-s -w -X main.version={{.Version}}`
  - archives the binary as tar.gz with `LICENSE` and `README.md`
  - writes checksums
  - writes a changelog from commits
  - `homebrew_casks` writes a cask to `dillo/homebrew-tap` with the binary, a description and homepage, and the quarantine-clearing install step

  The exact stanza for that step follows the GoReleaser version current when the plan is written. Homebrew deprecated `postflight` for `postflight_steps`, and GoReleaser's fix was due in v2.19. Clearing must happen before anything runs the binary, so use preflight if completion generation would run it first.
- **Local verification:** `goreleaser check` passes, and `goreleaser release --snapshot --clean` builds both binaries and renders the cask, with nothing published. `dist/` is git-ignored.

## 5. First-publish checklist (`RELEASING.md`)

1. Create the GitHub repos `dillo/brewery` (public) and `dillo/homebrew-tap` (public, can start empty).
2. Create a fine-grained token with Contents read and write on `dillo/homebrew-tap`. Add it to `dillo/brewery` as the Actions secret `HOMEBREW_TAP_GITHUB_TOKEN`.
3. `git remote add origin git@github.com:dillo/brewery.git`, then `git push -u origin master`. Check that CI passes.
4. `git tag v0.1.0`, then `git push origin v0.1.0`. Check that the release workflow passes and the cask appears in the tap.
5. In a clean shell: `brew install dillo/tap/brewery`, then `brewery --version` prints `0.1.0`, and brewery opens.

Later releases: tag `vX.Y.Z` and push the tag.

## Out of scope

Linux builds, Apple signing and notarization, homebrew/core submission, and announcing the release.

## Testing

Unit tests for the version check (`internal/brew`) and its wiring (`cmd/brewery`). The golden snapshots. golangci-lint clean. `goreleaser check` and a snapshot build locally. The existing race, vet, gofmt and tidy checks. Nothing is published while testing.
