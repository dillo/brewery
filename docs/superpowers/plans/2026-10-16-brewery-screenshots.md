# brewery: README screenshots — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Four screenshots of the real brewery UI in the README, rendered from a made-up machine. They are Home, Installed, Discover and an upgrade in the drawer. `make screenshots` regenerates them.

**Architecture:**
- **Demo data:** a made-up machine in brew's own formats, in `internal/ui/testdata/demo/`.
- **Renderer:** a test behind the `screenshots` build tag (`internal/ui/screenshots_test.go`). It drives the real `App` on that data and writes each screen's coloured render to `docs/screenshots/<name>.ansi`.
- **Images:** Charm's `freeze` turns each capture into a PNG.
- **README:** shows the PNGs.

**Tech Stack:** Go; Bubble Tea v2 (as before). Tools: Charm's freeze v0.2.2 (`go install github.com/charmbracelet/freeze@latest`) and JetBrains Mono Nerd Font (its NL cut).

**Spec:** `docs/superpowers/specs/2026-10-16-brewery-screenshots-design.md`, all of it.

**Found while building this:**
- **brew's `freeze` is a different program.** `brew install freeze` installs a cask for an Amazon Glacier client. Charm's freeze comes from `go install github.com/charmbracelet/freeze@latest` (or `charmbracelet/tap/freeze`).
- **freeze ignores `--font.ligatures=false` when it rasterises the PNG.** So plain JetBrains Mono turns brew's `==>` and `->` into arrows. The **NL** cut (`JetBrainsMonoNL Nerd Font`), which the Nerd Font cask installs alongside the regular one, has no ligatures.
- **freeze reads stdin whenever stdin isn't a terminal, even when given a file, and hangs.** Every call redirects `</dev/null`.
- **`App.render()` already emits true colour** (`38;2;…`), because Lip Gloss v2 only downsamples at output. No colour forcing is needed.
- **brew expands cask targets with the local home** (`/Users/<you>/Library/Fonts/…` for fonts), so the demo generator rewrote the home path to `/Users/demo`. No username or personal path is in the data.
- **The All view opens on libraries** (abseil, ada-url, brotli…), so the Installed shot uses Top-level.
- **Discover uses pipx** (#7 in the real 30-day list, not installed on the demo machine). Its preview shows "Would also install python@3.15" (its real `brew deps`). `uv` would have said "nothing else".

**Verified:**
- The prototype (branch `proto/screenshots`, commits 358697a and 252d97e) renders all four screens. `make screenshots` writes the PNGs at 4892×2684 (412–882 KB each, 2.6 MB total). Each was looked at: icons render, colours are brewery's, `==>` shows as typed, and no screen is loading or empty.
- `go test ./...` doesn't build the tagged file.
- `go vet` passes with and without the tag, and golangci-lint shows 0 issues with and without `--build-tags screenshots`.
- No file contains the local username.

## Global Constraints

- **Nothing personal in the repo:** no username (`locutus`), no `/Users/<real user>` path, no real package list. The demo machine's user is `demo`, its home `/Users/demo`.
- **The renderer never runs in `go test ./...` or CI:** the first line is `//go:build screenshots`.
- **Screens and file names:** `docs/screenshots/{home,installed,discover,upgrade}.png`, rendered at 140×36 with icons on. Captures are `docs/screenshots/*.ansi`, which are git-ignored.
- **freeze flags (exact):**
  - `--language ansi --window --background '#24273a'`
  - `--padding 20,24 --margin 0 --border.radius 10`
  - `--font.family 'JetBrainsMonoNL Nerd Font' --font.file "$(FONT)"`
  - `--font.size 14 --line-height 1.2`
  - stdin from `/dev/null`
- **Tools for implementers:**
  - Charm's freeze is already built at `/private/tmp/claude-501/-Users-locutus-Dev-tools-brewery/12a9a9c1-001a-4784-bd8a-fd8ddfef5515/scratchpad/bin/freeze`. Pass it as `make screenshots FREEZE=<that path>`.
  - The font is installed at `~/Library/Fonts/JetBrainsMonoNLNerdFont-Regular.ttf`.
  - Install nothing.
- Never install or upgrade anything with brew while testing.

## File Structure

```
internal/ui/testdata/demo/           the demo machine (new): info.json, outdated.json, services.json,
                                     taps.json, doctor.txt, catalogue.txt, five analytics lists
internal/ui/screenshots_test.go      the renderer (new, build tag screenshots)
Makefile                             screenshots target
.gitignore                           /docs/screenshots/*.ansi
docs/screenshots/*.png               the four images (new, generated)
README.md                            hero image, "A look around", make screenshots
```

---

### Task 1: The demo machine

**Files:**
- Create: `internal/ui/testdata/demo/` (11 files)

**Interfaces:**
- **Produces:**
  - brew-format files that `brew.ParseInfo`, `ParseOutdated`, `ParseServices`, `ParseTaps`, `ParseDoctor` and `ParseCatalogue`, plus `analytics.Parse`, accept;
  - the five analytics files, named as `strings.ReplaceAll(list, "/", "-") + ".json"` for each `analytics.Lists` entry.

The data was generated from brew's public package data and formulae.brew.sh's public analytics. The generator scripts aren't part of the repo. Take the files verbatim from the prototype.

- [ ] **Step 1: Copy the demo machine from the prototype**

```bash
git checkout 358697a -- internal/ui/testdata/demo
ls internal/ui/testdata/demo
```

Expected: `cask-install-30d.json cask-install-90d.json catalogue.txt doctor.txt info.json install-on-request-30d.json install-on-request-365d.json install-on-request-90d.json outdated.json services.json taps.json`.

(358697a is on the branch `proto/screenshots`. If git can't find it, ask the controller.)

- [ ] **Step 2: Check it**

```bash
grep -rli 'locutus' internal/ui/testdata/demo; echo "personal: $?"   # 1 = nothing found
grep -o '"/Users/[a-z]*' -h internal/ui/testdata/demo/*.json | sort -u     # only "/Users/demo
for f in internal/ui/testdata/demo/*.json; do python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$f" || echo "BAD $f"; done
python3 -c "import json; d=json.load(open('internal/ui/testdata/demo/info.json')); print(len(d['formulae']), 'formulae', len(d['casks']), 'casks', sum(f['installed'][0]['installed_on_request'] for f in d['formulae']), 'on request')"
cat internal/ui/testdata/demo/outdated.json
```

Expected:
- `personal: 1`;
- only `"/Users/demo`;
- no `BAD`;
- `63 formulae 6 casks 18 on request`;
- outdated lists node 26.11.1 → 26.12.0 (pinned), gh 2.101.0 → 2.102.0, and firefox 157.0.2 → 158.0.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/testdata/demo
git commit -m "test: a made-up machine for the README screenshots"
```

---

### Task 2: `make screenshots` renders brewery on the demo machine

**Files:**
- Create: `internal/ui/screenshots_test.go`
- Modify: `Makefile`
- Modify: `.gitignore`

**Interfaces:**
- **Consumes:**
  - the demo files (Task 1);
  - test helpers in package `ui`: `fakeStarter`, `newFakeProc`, `fakeProc.exit`, `fakeCatalog{entries, added, details}`, `fakeLogs{text}`, `keys`, `press`, `settle`, `resume`, `selectInstalled` (it switches to All), `selectDiscover`, `stripANSI`;
  - `App` fields `now`, `tickEvery`, `readLog`, `detailsDelay`, `snap`, `outdated`;
  - `App.UseHistory(jobs.History, []jobs.Record)`.
- **Produces:** `make screenshots` writes `docs/screenshots/{home,installed,discover,upgrade}.{ansi,png}`.

- [ ] **Step 1: Create `internal/ui/screenshots_test.go`**

```go
//go:build screenshots

package ui

import (
	"context"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/analytics"
	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/jobs"
	"github.com/dillo/brewery/internal/state"
)

// The README's screenshots: `make screenshots` runs this, then freeze turns
// each docs/screenshots/<name>.ansi into a PNG. The machine is made up
// (testdata/demo), so nothing about the machine running it shows.

// demoNow is the demo machine's clock.
var demoNow = time.Date(2026, 10, 16, 14, 32, 0, 0, time.UTC)

// demoFile reads one of the demo machine's files.
func demoFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "demo", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// demoSize is a package's size on disk: the big ones as they really are,
// the rest spread between 0.1 and 40MB.
func demoSize(key string) int64 {
	big := map[string]int64{
		"cask/docker-desktop": 2_362_232_012, "cask/visual-studio-code": 712_000_000, "cask/firefox": 431_000_000,
		"cask/raycast": 398_000_000, "cask/font-jetbrains-mono-nerd-font": 241_000_000, "cask/iterm2": 94_000_000,
		"formula/go": 309_000_000, "formula/node": 112_000_000, "formula/python@3.13": 87_000_000,
		"formula/icu4c@78": 83_000_000, "formula/postgresql@16": 77_000_000, "formula/git": 63_200_000,
	}
	if n, ok := big[key]; ok {
		return n
	}
	h := fnv.New32a()
	h.Write([]byte(key))
	return 100_000 + int64(h.Sum32()%40_000_000)
}

// demoSnapshot is everything brew would say about the demo machine.
func demoSnapshot(t *testing.T) state.Snapshot {
	t.Helper()
	var s state.Snapshot
	var err error
	if s.Packages, err = brew.ParseInfo(demoFile(t, "info.json")); err != nil {
		t.Fatal(err)
	}
	if s.Outdated, err = brew.ParseOutdated(demoFile(t, "outdated.json")); err != nil {
		t.Fatal(err)
	}
	if s.Services, err = brew.ParseServices(demoFile(t, "services.json")); err != nil {
		t.Fatal(err)
	}
	if s.Taps, err = brew.ParseTaps(demoFile(t, "taps.json")); err != nil {
		t.Fatal(err)
	}
	s.Doctor, s.DoctorRan = brew.ParseDoctor(string(demoFile(t, "doctor.txt"))), true
	s.Cleanup = brew.Cleanup{
		Standard: brew.CleanupEstimate{Items: 14, Size: "1.2GB"},
		All:      brew.CleanupEstimate{Items: 38, Size: "2.7GB"},
	}
	s.CleanupRan = true
	s.Sizes = map[string]int64{}
	for _, p := range s.Packages {
		s.Sizes[p.Key()] = demoSize(p.Key())
	}
	s.FetchedAt, s.DataAt = demoNow.Add(-2*time.Minute), demoNow.Add(-3*time.Hour)
	return s
}

// demoSource answers every query from the demo snapshot.
type demoSource struct{ snap state.Snapshot }

func (d demoSource) Installed(context.Context) ([]brew.Package, error) { return d.snap.Packages, nil }
func (d demoSource) Outdated(context.Context) ([]brew.Outdated, error) { return d.snap.Outdated, nil }
func (d demoSource) Services(context.Context) ([]brew.Service, error)  { return d.snap.Services, nil }
func (d demoSource) Taps(context.Context) ([]brew.Tap, error)          { return d.snap.Taps, nil }
func (d demoSource) Doctor(context.Context) ([]brew.DoctorWarning, error) {
	return d.snap.Doctor, nil
}
func (d demoSource) Cleanup(context.Context) (brew.Cleanup, error) { return d.snap.Cleanup, nil }
func (d demoSource) Sizes(context.Context, []brew.Package) (map[string]int64, error) {
	return d.snap.Sizes, nil
}
func (d demoSource) DataUpdated(context.Context) (time.Time, error) { return d.snap.DataAt, nil }

// demoStats serves Homebrew's analytics as of the demo machine's day.
type demoStats struct{ lists map[string]analytics.List }

func (d demoStats) Cached(list string) (analytics.List, time.Time, error) {
	return d.lists[list], demoNow.Add(-time.Hour), nil
}
func (d demoStats) Fetch(_ context.Context, list string) (analytics.List, error) {
	return d.lists[list], nil
}
func (d demoStats) Tried() string             { return demoNow.Format(time.DateOnly) }
func (d demoStats) SetTried(day string) error { return nil }

// pipx is what brew says about pipx (brew info and brew deps, Oct 2026).
var pipx = brew.Details{
	Package: brew.Package{
		Name: "pipx", FullName: "pipx", Kind: brew.KindFormula, Tap: "homebrew/core",
		Desc: "Execute binaries from Python packages in isolated environments", Homepage: "https://pipx.pypa.io",
		License: "MIT", Latest: "1.18.1",
	},
	Needs: []string{"ca-certificates", "lz4", "mpdecimal", "openssl@4", "python@3.15", "readline", "sqlite", "xz", "zstd"},
}

// upgradeGh is brew's output while it upgrades gh, as the drawer shows it.
const upgradeGh = "==> Upgrading 1 outdated package:\r\n" +
	"gh 2.101.0 -> 2.102.0\r\n" +
	"==> Fetching downloads for: gh\r\n" +
	"==> Downloading https://ghcr.io/v2/homebrew/core/gh/manifests/2.102.0\r\n" +
	"######################################################################### 100.0%\r\n" +
	"==> Fetching gh\r\n" +
	"==> Downloading https://ghcr.io/v2/homebrew/core/gh/blobs/sha256:4f1c2b…\r\n" +
	"######################################################################### 100.0%\r\n" +
	"==> Upgrading gh\r\n" +
	"  2.101.0 -> 2.102.0\r\n" +
	"==> Pouring gh--2.102.0.arm64_tahoe.bottle.tar.gz\r\n"

// demoApp is brewery on the demo machine at 140×36, icons on.
func demoApp(t *testing.T, fs *fakeStarter) *App {
	t.Helper()
	t.Setenv("HOME", "/Users/demo")
	local := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = local })

	snap := demoSnapshot(t)
	a := New(demoSource{snap}, fs.start, snap, filepath.Join(t.TempDir(), "snapshot.json"))
	a.now = func() time.Time { return demoNow }
	a.tickEvery = 0
	a.UseIcons(true)
	a.readLog = (&fakeLogs{text: map[string]string{"/opt/homebrew/var/log/postgresql@16.log": "" +
		"2026-10-16 09:12:03.418 UTC [4821] LOG:  starting PostgreSQL 16.10 on aarch64-apple-darwin25.0.0\n" +
		"2026-10-16 09:12:03.420 UTC [4821] LOG:  listening on IPv6 address \"::1\", port 5432\n" +
		"2026-10-16 09:12:03.420 UTC [4821] LOG:  listening on IPv4 address \"127.0.0.1\", port 5432\n" +
		"2026-10-16 09:12:03.431 UTC [4821] LOG:  database system is ready to accept connections\n"}}).read

	catalog := &fakeCatalog{
		entries: brew.ParseCatalogue(string(demoFile(t, "catalogue.txt"))),
		added:   []brew.Entry{{Name: "python@3.15", Kind: brew.KindFormula}, {Name: "ghostty", Kind: brew.KindCask}},
		details: map[string]brew.Details{"pipx": pipx},
	}
	a.UseCatalog(catalog)
	lists := map[string]analytics.List{}
	for _, name := range analytics.Lists {
		l, err := analytics.Parse(demoFile(t, strings.ReplaceAll(name, "/", "-")+".json"))
		if err != nil {
			t.Fatal(err)
		}
		lists[name] = l
	}
	a.UseAnalytics(demoStats{lists})
	a.detailsDelay = 0

	recs := []jobs.Record{
		{Title: "upgrade starship", Args: []string{"upgrade", "starship"}, State: jobs.Done,
			Started: demoNow.Add(-26 * time.Hour), Ended: demoNow.Add(-26*time.Hour + 9*time.Second)},
		{Title: "install lazygit", Args: []string{"install", "lazygit"}, State: jobs.Done,
			Started: demoNow.Add(-50 * time.Minute), Ended: demoNow.Add(-50*time.Minute + 6*time.Second)},
		{Title: "update", Args: []string{"update"}, State: jobs.Done,
			Started: demoNow.Add(-3 * time.Hour), Ended: demoNow.Add(-3*time.Hour + 4*time.Second)},
	}
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{recs[1], recs[2], recs[0]}) // newest first
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	return a
}

// shoot writes the screen as docs/screenshots/<name>.ansi, colours and all.
func shoot(t *testing.T, a *App, name string) {
	t.Helper()
	screen := a.render()
	plain := stripANSI(screen)
	for _, bad := range []string{"loading…", "starting brewery…", "asking brew…"} {
		if strings.Contains(plain, bad) {
			t.Fatalf("%s is still loading (%q):\n%s", name, bad, plain)
		}
	}
	if strings.TrimSpace(plain) == "" {
		t.Fatalf("%s is empty", name)
	}
	dir := filepath.Join("..", "..", "docs", "screenshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(screen+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScreenshots(t *testing.T) {
	a := demoApp(t, &fakeStarter{})
	keys(t, a, "1")
	shoot(t, a, "home")

	selectInstalled(t, a, "git") // on All
	keys(t, a, "F")              // back to Top-level: the tools, not their libraries
	shoot(t, a, "installed")

	keys(t, a, "6") // Popular tools
	selectDiscover(t, a, "pipx")
	shoot(t, a, "discover")

	p := newFakeProc(0, upgradeGh) // still running when the picture is taken
	fs := &fakeStarter{procs: []*fakeProc{p}}
	a = demoApp(t, fs)
	keys(t, a, "3")
	for range len(a.snap.Outdated) {
		if i := a.outdated.cursor(); i >= 0 && a.snap.Outdated[i].Name == "gh" {
			break
		}
		keys(t, a, "j")
	}
	waiting := settle(t, a, a.handleKey(press("u")))
	shoot(t, a, "upgrade")
	p.exit()
	resume(t, a, waiting)
}
```

- [ ] **Step 2: Run it**

Run: `go test -tags screenshots -count=1 -run TestScreenshots ./internal/ui && ls docs/screenshots`
Expected: `ok`, and `discover.ansi home.ansi installed.ansi upgrade.ansi`. Each starts with an escape sequence such as `\x1b[38;2;…`; check with `head -c 40 docs/screenshots/home.ansi | od -c | head -2`.

Then check that the tag keeps it out of normal runs. `go test ./internal/ui -run TestScreenshots -v 2>&1 | grep -c 'RUN.*TestScreenshots'` should print `0`.

- [ ] **Step 3: Add `make screenshots`, in `Makefile`**

Replace:

```makefile
.PHONY: build test test-live lint snapshot run
```

with:

```makefile
.PHONY: build test test-live lint snapshot screenshots run
```

and insert above `run: build`:

```makefile
# The README's screenshots, from a made-up machine (internal/ui/testdata/demo).
# Needs Charm's freeze (go install github.com/charmbracelet/freeze@latest; brew's
# "freeze" is a different app) and the JetBrains Mono Nerd Font, whose NL
# (no ligatures) cut shows brew's ==> as typed:
# brew install --cask font-jetbrains-mono-nerd-font
FREEZE ?= freeze
FONT ?= $(HOME)/Library/Fonts/JetBrainsMonoNLNerdFont-Regular.ttf
screenshots:
	go test -tags screenshots -count=1 -run TestScreenshots ./internal/ui
	for f in docs/screenshots/*.ansi; do \
		$(FREEZE) "$$f" --language ansi --window --background '#24273a' \
			--padding 20,24 --margin 0 --border.radius 10 \
			--font.family 'JetBrainsMonoNL Nerd Font' --font.file "$(FONT)" \
			--font.size 14 --line-height 1.2 -o "$${f%.ansi}.png" </dev/null || exit 1; \
	done

```

Recipe lines start with a tab, not spaces.

- [ ] **Step 4: Ignore the captures, in `.gitignore`**

Append the line:

```text
/docs/screenshots/*.ansi
```

- [ ] **Step 5: Run `make screenshots`**

Run: `make screenshots FREEZE=/private/tmp/claude-501/-Users-locutus-Dev-tools-brewery/12a9a9c1-001a-4784-bd8a-fd8ddfef5515/scratchpad/bin/freeze && file docs/screenshots/*.png && git status --short`
Expected:
- four `WROTE docs/screenshots/<name>.png` lines;
- `file` reports `PNG image data, 4892 x 2684` for each;
- `git status` shows the new PNGs as untracked, and no `.ansi` files.

- [ ] **Step 6: Checks, and commit (not the PNGs: Task 3 commits them)**

```bash
gofmt -l .
go vet ./... && go vet -tags screenshots ./internal/ui
/private/tmp/claude-501/-Users-locutus-Dev-tools-brewery/12a9a9c1-001a-4784-bd8a-fd8ddfef5515/scratchpad/bin/golangci-lint run --build-tags screenshots ./internal/ui
go test ./...
git add internal/ui/screenshots_test.go Makefile .gitignore
git commit -m "build: make screenshots renders brewery on a made-up machine"
```

Expected: nothing from gofmt or vet, `0 issues.`, all tests `ok`.

---

### Task 3: Screenshots in the README

**Files:**
- Create: `docs/screenshots/{home,installed,discover,upgrade}.png` (generated by `make screenshots`)
- Modify: `README.md`

**Interfaces:**
- **Consumes:** `make screenshots` (Task 2).

- [ ] **Step 1: Generate the images, and look at each one**

Run: `make screenshots FREEZE=/private/tmp/claude-501/-Users-locutus-Dev-tools-brewery/12a9a9c1-001a-4784-bd8a-fd8ddfef5515/scratchpad/bin/freeze`

Open each PNG and check for:
- **home:** five tiles (Installed 69, Outdated 3 with "1 pinned · gh, fire…", Services 1 running, Doctor 1 warning, Cleanup 2.7GB), the Command Line Tools warning, and three recent jobs (install lazygit 50m ago, update 3h ago, upgrade starship yesterday);
- **installed:** Top-level 24 selected, git highlighted, and git's preview with its tree (gettext, json-c, libunistring, pcre2);
- **discover:** Tools with pipx (#7) highlighted, and its preview with "Would also install python@3.15" and installs 93,902 / 225,614 / 640,029;
- **upgrade:** Outdated with gh highlighted, and the drawer with "brew upgrade gh" and brew's output. `==>` must appear as typed, not as an arrow.

All four should show Nerd Font icons and brewery's colours on `#24273a`, with no text cut off mid-glyph.

- [ ] **Step 2: Edit `README.md`**

Replace:

```markdown
live in a drawer, and any brew command a `:` away.

## Install
```

with:

```markdown
live in a drawer, and any brew command a `:` away.

![brewery's Home tab: counts of what's installed, outdated and running, doctor's warnings and recent jobs](docs/screenshots/home.png)

## Install
```

Replace:

```markdown
Then run `brewery`.

## Requirements
```

with:

```markdown
Then run `brewery`.

## A look around

Everything installed, with what it needs and what needs it.

![The Installed tab: a list of packages with versions, sizes and status, and a preview of git with its dependency tree](docs/screenshots/installed.png)

Find popular and rising packages, and install them.

![The Discover tab: Homebrew's most-installed tools, with a preview of pipx showing what installing it would add and its install counts](docs/screenshots/discover.png)

brew's own output, live — answer its questions right in the drawer.

![An upgrade of gh running, with brew's output in the drawer below the Outdated list](docs/screenshots/upgrade.png)

## Requirements
```

Replace:

````markdown
```sh
make build   # ./brewery
make test
```
````

with:

````markdown
```sh
make build   # ./brewery
make test
```

`make screenshots` regenerates the images above from a made-up machine
(`internal/ui/testdata/demo`). It needs Charm's
[freeze](https://github.com/charmbracelet/freeze)
(`go install github.com/charmbracelet/freeze@latest`) and the JetBrains Mono
Nerd Font (`brew install --cask font-jetbrains-mono-nerd-font`).
````

- [ ] **Step 3: Commit**

```bash
git add README.md docs/screenshots/home.png docs/screenshots/installed.png docs/screenshots/discover.png docs/screenshots/upgrade.png
git status --short   # nothing left, and no .ansi
git commit -m "docs: screenshots in the README"
```

---

### Task 4: Final checks

**Files:** none (verification only)

- [ ] **Step 1:**

```bash
go test -race -count=1 ./...
go vet ./... && go vet -tags screenshots ./internal/ui
gofmt -l .
go mod tidy -diff
golangci-lint run ./... && golangci-lint run --build-tags screenshots ./internal/ui
grep -rli 'locutus' README.md docs/screenshots internal/ui/testdata/demo internal/ui/screenshots_test.go; echo "personal: $?"
du -sh docs/screenshots
```

Expected:
- all tests `ok`, and `0 issues.` twice;
- nothing from gofmt, vet or tidy;
- `personal: 1`;
- about 2.6M of images.

(golangci-lint and freeze are at the scratchpad bin path; see Global Constraints.)
