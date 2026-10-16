# brewery: README screenshots

Date: 2026-10-16. Status: approved in conversation; branch `feat/screenshots`.

## Goal

Show people what brewery looks like before they install it. The README gets four screenshots of the real UI, generated from a made-up demo machine, so nothing personal is published. `make screenshots` regenerates them identically whenever the UI changes.

## 1. Demo data (`internal/ui/testdata/demo/`)

The demo data is in brew's own formats and goes through brewery's real parsers (`brew.ParseInfo`, `ParseOutdated`, `ParseServices`, `ParseTaps`, `ParseCatalogue`, `analytics.Parse`). That keeps every screen authentic.

- **`info.json`** (`brew info --json=v2 --installed`): about 40 well-known packages, each with a description, homepage, tap, versions, install time and runtime dependencies.
  - **Formulae:** git, node, python@3.13, go, ripgrep, fd, fzf, bat, jq, gh, neovim, tmux, wget, htop, postgresql@16, redis, plus their usual dependencies (pcre2, openssl@3, ca-certificates, libuv, icu4c, sqlite, xz, readline, oniguruma, …).
  - **Casks:** firefox, visual-studio-code, iterm2, docker-desktop, raycast, font-jetbrains-mono-nerd-font.
  - About 30 formulae and 6 casks, so the view counts look like a real machine.
- **`outdated.json`:** three outdated (node, gh, firefox), one of them pinned (node). Those packages are also marked outdated and pinned in `info.json`, so every tab agrees.
- **`services.json`:** postgresql@16 started and redis stopped. `user` is `demo`, and paths are under `/Users/demo` and `/opt/homebrew`.
- **`taps.json`:** `homebrew/core`, `homebrew/cask` and one third-party tap.
- **Sizes:** a realistic size per package (git 63 MB, docker-desktop 2.2 GB, …).
- **Discover:**
  - `catalogue.txt` is a few hundred entries in `brew search --desc ""` format, including everything installed plus popular packages that aren't.
  - The analytics lists use formulae.brew.sh's JSON format: `install-on-request/30d`, `90d` and `365d`, and `cask-install/30d` and `90d`.
- Nothing comes from the user's machine. No real username, path or package list.

## 2. Renderer (`internal/ui/screenshots_test.go`)

- **Build tag:** `//go:build screenshots`, so `go test ./...`, CI and lint skip it.
- **`TestScreenshots`** builds an `App` from the demo data:
  - a demo `Source`, `Catalog` and `Analytics`, in the style of the existing test fakes;
  - a fixed clock (`a.now`);
  - `HOME=/Users/demo` and `time.Local = time.UTC`;
  - window size 120×36, icons on (`UseIcons(true)`), and Lip Gloss colour forced to true colour so the capture carries real colours.
- **Screens**, each written to `docs/screenshots/<name>.ansi` as `App.render()` output, with ANSI codes kept:
  1. `home`: the Home tab, with doctor's warnings and two recent jobs from a demo history.
  2. `installed`: Installed, All view, with git selected and its preview showing the dependency tree.
  3. `discover`: Discover, Popular tools, with a not-installed package selected and its preview showing version, licence, "Would also install" and installs.
  4. `upgrade`: `u` on an outdated package, unpinned, with the drawer open showing brew's upgrade output, written by a fake process from a recorded demo transcript.
- **Checks:** the test fails if a capture is empty or contains `loading…`, `starting brewery…` or `asking brew…` (no screen captured mid-load).

## 3. Images

- **`freeze`** (`github.com/charmbracelet/freeze`) turns each `.ansi` capture into `docs/screenshots/<name>.png`:
  - a terminal window frame (`--window`);
  - JetBrains Mono Nerd Font (`--font.family`, or `--font.file` pointing at the installed TTF), so icons render;
  - background `#24273a` (Catppuccin Macchiato Base, brewery's palette);
  - modest padding, rounded corners, and the output scaled for a retina-sharp PNG.

  The prototype settles the exact flags.
- **`make screenshots`** runs the tagged test, then `freeze` on each capture. It needs `freeze` (`brew install freeze`) and the font (`brew install --cask font-jetbrains-mono-nerd-font`). The Makefile comment says so.
- The `.ansi` captures are intermediate files, and `.gitignore` excludes them. Only the PNGs are committed. Each should be a few hundred KB at most.

## 4. README

- Under the intro paragraph, the Home screenshot is the hero image: `![brewery's Home tab](docs/screenshots/home.png)`.
- A new section "A look around" comes after Install. It has the other three images, each with a one-line caption:
  - Installed: "Everything installed, with what it needs and what needs it."
  - Discover: "Find popular and rising packages, and install them."
  - upgrade: "brew's own output, live — answer its questions right in the drawer."
- "Building from source" gains `make screenshots` and what it needs.

## Testing

- The tagged test's own checks.
- A human look at each PNG before committing, for clipped text, wrong colours or missing icons.
- `go test ./...`, lint and CI are unaffected. Running `go test ./...` without the tag must not see the new file.

## Out of scope

Animated GIFs, VHS, a demo mode in the shipped binary, and screenshots of every tab.
