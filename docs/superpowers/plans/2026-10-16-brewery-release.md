# brewery Plan 8: Ready to ship — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make brewery ready for its first public release:
- a Homebrew 7.0 floor, enforced at startup;
- the module path `github.com/dillo/brewery`, an MIT licence and a README;
- golangci-lint and golden screen snapshots;
- CI, and a GoReleaser pipeline that writes a cask to `dillo/homebrew-tap`;
- `RELEASING.md` for the first publish.

This plan publishes nothing.

**Architecture:**
- **`internal/brew`:** gains `Version`, `ParseVersion` and `(*Client) BrewVersion`. `main` checks the version through a `brewVersion` seam, as it already finds brew through `locateBrew`.
- **Rename:** one mechanical commit.
- **Lint:** golangci-lint v2 with the `std-error-handling` preset. The remaining findings are fixed in code.
- **Snapshots:** a small golden-file helper over `App.render()`.
- **Release:** GoReleaser 2.18 builds darwin arm64/amd64 and writes a cask through `homebrew_casks`. A `custom_block` holds Homebrew's current `postflight_steps` stanza, which clears quarantine.

**Tech Stack:** Go ≥ 1.25; Bubble Tea v2.0.10. Tools: golangci-lint v2.14.0 and GoReleaser v2.18.3 (`brew install golangci-lint goreleaser`, or `go install` them anywhere), plus actionlint for checking workflows. GitHub Actions use `actions/checkout@v7`, `actions/setup-go@v7`, `golangci/golangci-lint-action@v9` and `goreleaser/goreleaser-action@v7`.

**Spec:** `docs/superpowers/specs/2026-10-16-brewery-release-design.md`, all of it.

**Found while building this:**
- **brew --version takes about 50 ms** (`Homebrew 7.0.9-108-gf63eb29` here), so checking it before the TUI costs nothing noticeable.
- **teatest isn't used.** Its v2 module exists only as an untagged pseudo-version (`v2.0.0-20261012…`) built against Bubble Tea `v2.0.0-rc.1`. It also captures the renderer's raw terminal output, not the screen. brewery's tests already drive the model directly, so a ten-line golden helper over `render()` is deterministic and has no dependencies. Five runs in a row gave identical snapshots.
- **golangci-lint found 55 issues; 53 were unchecked errors.** Most were `Close`, `os.Remove` and `fmt.Fprint*`, which golangci-lint's own `std-error-handling` preset exempts (the v1 default). The other 23 are fixed:
  - test helpers now check their errors;
  - deliberate best-effort calls to brew's pseudo-terminal and the drawer's emulator use an explicit `_ =`;
  - an `errorlint` finding: `%v` becomes `%w`;
  - a deprecated call: `ansi.Style.DefaultForegroundColor()` becomes `ForegroundColor(nil)`, which writes the same `\x1b[39m` (`TestTintResetsOnlyTheForeground` still passes).
- **GoReleaser 2.18.3's cask hooks write Homebrew's deprecated `postflight` block.** Brew 7.0.9 warns `` `postflight` is deprecated, use `postflight_steps` `` (`cask/dsl.rb:863`). GoReleaser's `install_steps` arrive in v2.19, which isn't released. So the stanza goes in `custom_block`. Brew 7.0.9's own loader (`Cask::CaskLoader::FromContentLoader`) parsed the generated cask as one `Binary` and one `PostflightSteps` running `/usr/bin/xattr -dr com.apple.quarantine brewery` from the staged path.
- **brew refuses casks outside a tap** (`brew info --cask ./file.rb` errors), so a real install test waits for the tap (RELEASING.md step 5).
- **The tap commits are authored by GoReleaser's bot.** `commit_author` is left unset so no personal email goes into public commits.

**Verified:** every code block was compiled and tested in a prototype (branch `proto/release`) before the plan was written. Checks:
- race tests, live tests, vet, gofmt, `go mod tidy -diff` and golangci-lint (0 issues) are all clean;
- `goreleaser check` validates, and `goreleaser release --snapshot --clean` builds both Mach-O binaries (`brewery --version` prints the stamped version), archives them with LICENSE and README, and writes the cask;
- actionlint v1.7.12 passes both workflows.

The real binary:
- with a fake `brew` printing `Homebrew 6.4.2` (via `HOMEBREW_PREFIX`), it exits 1 with the floor message;
- with the real brew 7.0.9, it opens Home as before.

## Global Constraints

From Plans 1–7¾:
- Only `internal/brew` and `main` run brew. Views use `theme.Styles` only.
- Real-binary runs use absolute scratch paths for `XDG_STATE_HOME` and `XDG_CACHE_HOME`.
- **Never install anything for real while testing**, including brewery itself from a cask.
- **Publish nothing:**
  - no `git remote add`, no push, no tag pushes;
  - no GitHub API calls;
  - `goreleaser` only with `check` or `release --snapshot`.

Added by this plan:
- **Floor:** Homebrew `7.0` (`brew.MinMajor = 7`, `brew.MinMinor = 0`).
- **Floor message:** `` brewery needs Homebrew 7.0 or newer (you have 6.4.2). Update it with `brew update`. `` The version is as brew printed it, up to any `-` suffix. If the version can't be read, brewery starts anyway.
- **Identity:**
  - module: `github.com/dillo/brewery`
  - tap: `dillo/homebrew-tap`; install with `brew install dillo/tap/brewery`
  - licence: MIT, `Copyright (c) 2026 Dillo Raju`
- **Release:**
  - builds: darwin arm64 and amd64 only, with `CGO_ENABLED=0`, `-trimpath` and `-s -w -X main.version={{ .Version }}`
  - archives: tar.gz with LICENSE and README.md
  - the cask: `homebrew_casks`, with a `postflight_steps` block that runs xattr from the staged path
  - the tap token: the secret `HOMEBREW_TAP_GITHUB_TOKEN`
- **Snapshots:** `internal/ui/testdata/golden/{home,installed,outdated,services,discover-all,help}.txt`, ANSI-stripped, at 120×40 with icons off. `go test ./internal/ui -run TestScreens -update` rewrites them.

## File Structure

```
internal/brew/version.go            Version, MinMajor/MinMinor, ParseVersion, BrewVersion (new)
internal/brew/version_test.go       (new)
internal/brew/live_test.go          TestLiveBrewVersion
cmd/brewery/main.go                 brewVersion seam, readBrewVersion, versionProblem
cmd/brewery/main_test.go            TestBrewTooOld, TestVersionProblem
go.mod, every .go file              module github.com/dillo/brewery
LICENSE, README.md                  (new)
.golangci.yml                       (new); fixes across 13 files
Makefile                            lint, snapshot
internal/ui/golden_test.go          golden, TestScreens (new)
internal/ui/testdata/golden/*.txt   generated snapshots (new)
.goreleaser.yaml                    (new)
.github/workflows/ci.yml, release.yml (new)
RELEASING.md                        (new)
```

---

### Task 1: brewery needs Homebrew 7.0 or newer

**Files:**
- Create: `internal/brew/version.go`
- Create: `internal/brew/version_test.go`
- Modify: `internal/brew/live_test.go`
- Modify: `cmd/brewery/main.go`
- Modify: `cmd/brewery/main_test.go`

**Interfaces:**
- **Consumes:** `Client.query`, `NewClient`, `ExecRunner`, `QueryEnv`; the test runner `fakeRunner{out, err map[string]…}` in `client_test.go`.
- **Produces:**
  - `brew.MinMajor`, `brew.MinMinor` (7, 0)
  - `brew.Version{Major, Minor int; Text string}` and `(Version) TooOld() bool`
  - `brew.ParseVersion(out string) (Version, bool)`
  - `(*Client) BrewVersion(ctx) (Version, error)`
  - in `main`: `var brewVersion = readBrewVersion`, `readBrewVersion(bin string) (brew.Version, error)`, `versionProblem(bin string) string`

- [ ] **Step 1: Create `internal/brew/version_test.go`**

```go
package brew

import (
	"context"
	"errors"
	"testing"
)

func TestParseVersion(t *testing.T) {
	for _, tt := range []struct {
		out  string
		want Version
		ok   bool
	}{
		{"Homebrew 7.0.9-108-gf63eb29\n", Version{7, 0, "7.0.9"}, true},
		{"Homebrew 7.1.0\nHomebrew/homebrew-core (git revision abc)\n", Version{7, 1, "7.1.0"}, true},
		{"Homebrew 6.4.2\n", Version{6, 4, "6.4.2"}, true},
		{"Homebrew 10.0.0\n", Version{10, 0, "10.0.0"}, true},
		{"Homebrew 7.2\n", Version{7, 2, "7.2"}, true},
		{"Warning: something\nHomebrew 7.0.1\n", Version{7, 0, "7.0.1"}, true},
		{"Homebrew >=4.3.0 (shallow or no git repository)\n", Version{}, false},
		{"garbage", Version{}, false},
		{"", Version{}, false},
	} {
		got, ok := ParseVersion(tt.out)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ParseVersion(%q) = %+v, %v; want %+v, %v", tt.out, got, ok, tt.want, tt.ok)
		}
	}
}

func TestTooOld(t *testing.T) {
	for _, tt := range []struct {
		v    Version
		want bool
	}{
		{Version{6, 9, "6.9.0"}, true},
		{Version{7, 0, "7.0.0"}, false},
		{Version{7, 3, "7.3.1"}, false},
		{Version{8, 0, "8.0.0"}, false},
	} {
		if got := tt.v.TooOld(); got != tt.want {
			t.Errorf("%+v.TooOld() = %v, want %v", tt.v, got, tt.want)
		}
	}
}

func TestClientBrewVersion(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{"--version": []byte("Homebrew 7.0.9-108-gf63eb29\n")}})
	if v, err := c.BrewVersion(context.Background()); err != nil || v != (Version{7, 0, "7.0.9"}) {
		t.Errorf("got %+v, %v", v, err)
	}
	c = NewClient(fakeRunner{out: map[string][]byte{"--version": []byte("odd\n")}})
	if _, err := c.BrewVersion(context.Background()); err == nil {
		t.Error("unparseable output: want an error")
	}
	broke := errors.New("exit status 1")
	c = NewClient(fakeRunner{err: map[string]error{"--version": broke}})
	if _, err := c.BrewVersion(context.Background()); !errors.Is(err, broke) {
		t.Errorf("got %v, want brew's failure", err)
	}
}
```

- [ ] **Step 2: Append to `cmd/brewery/main_test.go`**

Add `"errors"` to its imports:

```go
import (
	"bytes"
	"errors"
	"strings"
	"testing"
```

Append:

```go

func TestBrewTooOld(t *testing.T) {
	locateBrew = func() (string, error) { return "/opt/homebrew/bin/brew", nil }
	brewVersion = func(string) (brew.Version, error) { return brew.Version{Major: 6, Minor: 4, Text: "6.4.2"}, nil }
	t.Cleanup(func() { locateBrew, brewVersion = brew.Locate, readBrewVersion })
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if want := "brewery needs Homebrew 7.0 or newer (you have 6.4.2). Update it with `brew update`.\n"; errb.String() != want {
		t.Errorf("stderr %q, want %q", errb.String(), want)
	}
}

func TestVersionProblem(t *testing.T) {
	t.Cleanup(func() { brewVersion = readBrewVersion })
	for _, tt := range []struct {
		v    brew.Version
		err  error
		want string
	}{
		{brew.Version{Major: 7, Minor: 0, Text: "7.0.9"}, nil, ""},
		{brew.Version{Major: 8, Minor: 1, Text: "8.1.0"}, nil, ""},
		{brew.Version{}, errors.New("can't read the version"), ""}, // unknown: start anyway
		{brew.Version{Major: 6, Minor: 9, Text: "6.9.1"}, nil, "brewery needs Homebrew 7.0 or newer (you have 6.9.1). Update it with `brew update`."},
	} {
		brewVersion = func(string) (brew.Version, error) { return tt.v, tt.err }
		if got := versionProblem("/opt/homebrew/bin/brew"); got != tt.want {
			t.Errorf("%+v, %v: got %q, want %q", tt.v, tt.err, got, tt.want)
		}
	}
}
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `go test ./internal/brew/ ./cmd/brewery/`
Expected: FAIL to compile (`undefined: Version`, `undefined: brewVersion`).

- [ ] **Step 4: Create `internal/brew/version.go`**

```go
package brew

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
)

// The oldest Homebrew brewery supports: the version it was built and tested
// against.
const (
	MinMajor = 7
	MinMinor = 0
)

// Version is a Homebrew version: Major and Minor to compare, and Text as
// brew printed it, up to any "-" suffix ("7.0.9").
type Version struct {
	Major, Minor int
	Text         string
}

// TooOld reports whether v is older than MinMajor.MinMinor.
func (v Version) TooOld() bool {
	return v.Major < MinMajor || (v.Major == MinMajor && v.Minor < MinMinor)
}

var versionRe = regexp.MustCompile(`(?m)^Homebrew (\d+)\.(\d+)(\.\d+)?`)

// ParseVersion reads the first "Homebrew <major>.<minor>[.<patch>]" line of
// brew --version's output.
func ParseVersion(out string) (Version, bool) {
	m := versionRe.FindStringSubmatch(out)
	if m == nil {
		return Version{}, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return Version{Major: major, Minor: minor, Text: m[1] + "." + m[2] + m[3]}, true
}

// BrewVersion runs brew --version and parses it.
func (c *Client) BrewVersion(ctx context.Context) (Version, error) {
	out, err := c.query(ctx, "--version")
	if err != nil {
		return Version{}, err
	}
	v, ok := ParseVersion(string(out))
	if !ok {
		return Version{}, fmt.Errorf("can't read the version in %q", out)
	}
	return v, nil
}
```

- [ ] **Step 5: Check the version in `cmd/brewery/main.go`**

Add `"context"` to the imports (`"time"` is already there):

```go
import (
	"context"
	"errors"
```

Replace:

```go
// locateBrew is swapped out in tests.
var locateBrew = brew.Locate
```

with:

```go
// locateBrew and brewVersion are swapped out in tests.
var (
	locateBrew  = brew.Locate
	brewVersion = readBrewVersion
)

// readBrewVersion asks the brew at bin for its version. It takes about
// 50ms; a brew that hangs is given 5 seconds.
func readBrewVersion(bin string) (brew.Version, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return brew.NewClient(brew.ExecRunner{Bin: bin, Env: brew.QueryEnv}).BrewVersion(ctx)
}

// versionProblem says why the brew at bin is too old for brewery, or
// returns "". A version brewery can't read doesn't stop it: anything really
// broken shows in the tabs.
func versionProblem(bin string) string {
	v, err := brewVersion(bin)
	if err != nil || !v.TooOld() {
		return ""
	}
	return fmt.Sprintf("brewery needs Homebrew %d.%d or newer (you have %s). Update it with `brew update`.", brew.MinMajor, brew.MinMinor, v.Text)
}
```

In `run`, replace:

```go
		fmt.Fprintln(stderr, "Install it from https://brew.sh, then run brewery again.")
		return 1
	}
```

with:

```go
		fmt.Fprintln(stderr, "Install it from https://brew.sh, then run brewery again.")
		return 1
	}
	if msg := versionProblem(bin); msg != "" {
		fmt.Fprintln(stderr, msg)
		return 1
	}
```

- [ ] **Step 6: Append a live test to `internal/brew/live_test.go`**

```go

func TestLiveBrewVersion(t *testing.T) {
	bin, err := Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	v, err := NewClient(ExecRunner{Bin: bin, Env: QueryEnv}).BrewVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.TooOld() {
		t.Errorf("this machine's brew %s is older than brewery's floor", v.Text)
	}
	t.Logf("brew %s", v.Text)
}
```

- [ ] **Step 7: Run the tests**

Run: `go test -race ./... && go test -tags live -run LiveBrewVersion -v ./internal/brew/ && go vet ./... && go vet -tags live ./...`
Expected: all `ok`; the live test logs `brew 7.0.9` (or whatever is installed, at least 7.0).

- [ ] **Step 8: Commit**

```bash
gofmt -l .
git add internal/brew/version.go internal/brew/version_test.go internal/brew/live_test.go cmd/brewery/main.go cmd/brewery/main_test.go
git commit -m "feat: brewery needs Homebrew 7.0 or newer"
```

---

### Task 2: The module is github.com/dillo/brewery; MIT licence; README

**Files:**
- Modify: `go.mod` and every `.go` file that imports `brewery/…`
- Create: `LICENSE`
- Create: `README.md`

**Interfaces:**
- **Produces:** the module path `github.com/dillo/brewery`. Every internal import becomes `github.com/dillo/brewery/internal/…`.

- [ ] **Step 1: Rename the module**

```bash
go mod edit -module github.com/dillo/brewery
grep -rl --include='*.go' '"brewery/' . | xargs sed -i '' 's#"brewery/#"github.com/dillo/brewery/#g'
```

- [ ] **Step 2: Check the rename**

Run: `grep -rn --include='*.go' '"brewery/' . ; gofmt -l . ; go list -m ; go build ./... && go vet ./... && go test -race ./... && go mod tidy -diff`
Expected: no `"brewery/` imports left, gofmt prints nothing, `go list -m` prints `github.com/dillo/brewery`, all tests `ok`, tidy prints nothing.

- [ ] **Step 3: Create `LICENSE`**

```text
MIT License

Copyright (c) 2026 Dillo Raju

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 4: Create `README.md`**

The keys come from the help overlay (`internal/ui/help.go`); the paths from `state.DefaultCachePath` and `jobs.DefaultStateDir`.

````markdown
# brewery

A terminal dashboard for Homebrew. See what's installed, what's outdated and
what's running; upgrade, uninstall, pin, start and stop services, clean up,
and find new formulae, apps and fonts to install — with brew's own output
live in a drawer, and any brew command a `:` away.

## Install

```sh
brew install dillo/tap/brewery
```

Then run `brewery`.

## Requirements

- macOS (Apple Silicon or Intel)
- Homebrew 7.0 or newer
- A [Nerd Font](https://www.nerdfonts.com) for icons — optional. Without one,
  run `brewery --no-icons`, or set `BREWERY_NO_ICONS` to any value.

## Tabs

1. **Home** — counts, doctor's warnings, recent jobs
2. **Installed** — everything installed, with a preview of each package
3. **Outdated** — what has a newer version
4. **Services** — `brew services`, with each service's log
5. **Taps**
6. **Discover** — popular and rising packages from Homebrew's analytics, what's
   new, and the whole catalogue to search

## Keys

Press `?` anywhere for the keys that work there.

| Key | What it does |
|---|---|
| `1`–`6`, `[` `]` | switch tabs |
| `j` `k`, `gg` `G`, `ctrl+d` `ctrl+u` | move in a list |
| `/` | filter the list |
| `u` | upgrade (Installed, Outdated) |
| `A` | upgrade all (Outdated) |
| `x` | uninstall (Installed), untap (Taps) |
| `p` | pin or unpin (Installed) |
| `space` | mark several for `u` or `x` |
| `s` `S` `R` | start, stop, restart a service; `l` follows its log |
| `a` | install (Discover) |
| `tab` `enter` | preview, then `j` `k` scroll |
| `y` | copy the name |
| `gx` | open the homepage |
| `:` | run any brew command |
| `r` | refresh |
| `U` | `brew update` |
| `c` | clean up |
| `o` | show or hide brew's output |
| `i` | type into the running job (passwords, questions) |
| `ctrl+c` | stop the running job |
| `q` | quit |

## Files

- `$XDG_CACHE_HOME/brewery` (default `~/.cache/brewery`): the last snapshot
  of your packages, so brewery opens instantly, and Homebrew's analytics.
- `$XDG_STATE_HOME/brewery` (default `~/.local/state/brewery`): the history
  of jobs brewery ran, and crash logs.

## Building from source

```sh
make build   # ./brewery
make test
```

## Licence

MIT. See [LICENSE](LICENSE).
````

- [ ] **Step 5: Commit**

```bash
git add -A go.mod cmd internal LICENSE README.md
git status --short   # only renamed imports, go.mod, LICENSE and README.md; nothing under .superpowers/
git commit -m "chore: module github.com/dillo/brewery; MIT licence; README"
```

---

### Task 3: golangci-lint, and what it finds

**Files:**
- Create: `.golangci.yml`
- Modify: `Makefile`
- Modify (fixes): `cmd/brewery/crash.go`, `cmd/brewery/crash_test.go`, `internal/analytics/store.go`, `internal/analytics/store_test.go`, `internal/brew/completions_test.go`, `internal/jobs/history_test.go`, `internal/state/snapshot_test.go`, `internal/theme/theme.go`, `internal/ui/app.go`, `internal/ui/drawer.go`, `internal/ui/history_test.go`, `internal/ui/run.go`, `internal/ui/run_test.go`

**Interfaces:** none change. `make lint` runs golangci-lint.

- [ ] **Step 1: Create `.golangci.yml`**

```yaml
version: "2"

linters:
  # The standard set (errcheck, govet, ineffassign, staticcheck, unused),
  # plus a few cheap checks.
  enable:
    - errorlint
    - misspell
    - unconvert
  exclusions:
    # Closing, removing and printing best-effort, as Go code conventionally does.
    presets:
      - std-error-handling

formatters:
  enable:
    - gofmt
```

- [ ] **Step 2: Run it to see the findings**

Run: `golangci-lint run --max-same-issues=0 --max-issues-per-linter=0 ./...`
Expected: `23 issues: errcheck: 21, errorlint: 1, staticcheck: 1`, at the lines fixed below.

- [ ] **Step 3: Fix the production code**

In `cmd/brewery/crash.go`, replace:

```go
	unix.Dup2(int(c.terminal.Fd()), 2)
```

with:

```go
	_ = unix.Dup2(int(c.terminal.Fd()), 2) // best effort: brewery is on its way out
```

In `internal/ui/app.go`, make five replacements. Replace:

```go
	a.proc.Interrupt()
```

with:

```go
	_ = a.proc.Interrupt()
```

Replace:

```go
		a.proc.Kill()
```

with:

```go
		_ = a.proc.Kill()
```

Replace:

```go
			a.proc.Write([]byte(msg.Content)) // raw: sudo wants no bracketed-paste wrapping
```

with:

```go
			_, _ = a.proc.Write([]byte(msg.Content)) // raw: sudo wants no bracketed-paste wrapping
```

Replace:

```go
		a.proc.Resize(a.drawer.cols, a.drawer.rows)
```

with:

```go
		_ = a.proc.Resize(a.drawer.cols, a.drawer.rows)
```

Replace:

```go
			a.proc.Write(keyBytes(msg))
```

with:

```go
			_, _ = a.proc.Write(keyBytes(msg))
```

In `internal/ui/drawer.go`, make four replacements. Replace:

```go
	go io.Copy(d.reply, d.term)
```

with:

```go
	go func() { _, _ = io.Copy(d.reply, d.term) }()
```

Replace:

```go
		r.w.Write(b) // a write error only means the reply is lost
```

with:

```go
		_, _ = r.w.Write(b) // a write error only means the reply is lost
```

Replace:

```go
	d.term.Write([]byte("\x1bc")) // full terminal reset: screen, cursor, colors
```

with:

```go
	_, _ = d.term.Write([]byte("\x1bc")) // full terminal reset: screen, cursor, colors
```

Replace:

```go
	d.term.Write(b)
```

with:

```go
	_, _ = d.term.Write(b)
```

In `internal/ui/run.go`, replace:

```go
		a.proc.Interrupt() // the exit message follows
```

with:

```go
		_ = a.proc.Interrupt() // the exit message follows
```

In `internal/analytics/store.go`, replace:

```go
		return List{}, fmt.Errorf("%s: %w: %v", list, ErrBadList, err)
```

with:

```go
		return List{}, fmt.Errorf("%s: %w: %w", list, ErrBadList, err)
```

In `internal/theme/theme.go`, replace:

```go
	return ansi.NewStyle().ForegroundColor(c).String() + s + ansi.NewStyle().DefaultForegroundColor().String()
```

with:

```go
	return ansi.NewStyle().ForegroundColor(c).String() + s + ansi.NewStyle().ForegroundColor(nil).String()
```

- [ ] **Step 4: Fix the tests**

In `cmd/brewery/crash_test.go`, replace:

```go
	unix.Write(2, []byte("fatal error: concurrent map writes\n"))
```

with:

```go
	if _, err := unix.Write(2, []byte("fatal error: concurrent map writes\n")); err != nil {
		t.Fatal(err)
	}
```

In `internal/analytics/store_test.go`, replace:

```go
		w.Write(body)
```

with:

```go
		_, _ = w.Write(body)
```

In `internal/brew/completions_test.go`, replace:

```go
	os.MkdirAll(dir, 0o755)
```

with:

```go
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
```

and:

```go
	os.WriteFile(filepath.Join(dir, "brew.fish"), b, 0o644)
```

with:

```go
	if err := os.WriteFile(filepath.Join(dir, "brew.fish"), b, 0o644); err != nil {
		t.Fatal(err)
	}
```

In `internal/jobs/history_test.go`, replace:

```go
	h.Save(record("upgrade fd", t0))
	os.WriteFile(filepath.Join(h.Dir, "20990101T000000.000000000.json"), []byte("{not json"), 0o644)
```

with:

```go
	if err := h.Save(record("upgrade fd", t0)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Dir, "20990101T000000.000000000.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
```

In `internal/state/snapshot_test.go`, replace:

```go
	os.WriteFile(path, []byte(`{"version": 999, "packages": [{"name": "fd"}]}`), 0o644)
```

with:

```go
	if err := os.WriteFile(path, []byte(`{"version": 999, "packages": [{"name": "fd"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
```

In `internal/ui/history_test.go`, replace:

```go
	os.WriteFile(file, nil, 0o644)
```

with:

```go
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
```

In `internal/ui/run_test.go`, replace:

```go
	p.Resize(cols, rows)
```

with:

```go
	_ = p.Resize(cols, rows)
```

and:

```go
	p.Interrupt() // brew dies of the 0x03, exit 130
```

with:

```go
	if err := p.Interrupt(); err != nil { // brew dies of the 0x03, exit 130
		t.Fatal(err)
	}
```

- [ ] **Step 5: Add `make lint`, in `Makefile`**

Replace:

```makefile
.PHONY: build test test-live run
```

with:

```makefile
.PHONY: build test test-live lint run
```

and insert above `run: build`:

```makefile
# golangci-lint v2: brew install golangci-lint
lint:
	golangci-lint run ./...

```

- [ ] **Step 6: Run everything**

Run: `golangci-lint run ./... && go test -race -count=1 ./... && go vet ./... && gofmt -l .`
Expected: `0 issues.`, all tests `ok`, nothing from vet or gofmt.

- [ ] **Step 7: Commit**

```bash
git add .golangci.yml Makefile cmd internal
git commit -m "chore: golangci-lint config and fixes; make lint"
```

---

### Task 4: Golden snapshots of the main screens

**Files:**
- Create: `internal/ui/golden_test.go`
- Create (generated): `internal/ui/testdata/golden/{home,installed,outdated,services,discover-all,help}.txt`

**Interfaces:**
- **Consumes:** `newJobApp`, `fakeStarter`, `fakeLogs`, `newFakeCatalog`, `newFakeStats`, `keys`, `stripANSI`, `App.render`, `App.help`.
- **Produces:** `golden(t, name, got string)` and the `-update` flag.

- [ ] **Step 1: Create `internal/ui/golden_test.go`**

```go
package ui

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update rewrites the golden snapshots: go test ./internal/ui -run TestScreens -update
var update = flag.Bool("update", false, "rewrite the golden snapshots in testdata/golden")

// golden compares got with testdata/golden/<name>.txt, or rewrites the file
// with -update.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".txt")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (go test ./internal/ui -run TestScreens -update writes it)", err)
	}
	if got != string(want) {
		t.Errorf("%s changed (if that's intended, rewrite it with -update); got:\n%s", path, got)
	}
}

// TestScreens snapshots brewery's main screens at 120×40, icons off, from
// the test fixtures.
func TestScreens(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.UseCatalog(newFakeCatalog(t))
	a.UseAnalytics(newFakeStats(t))
	a.detailsDelay = 0
	for _, s := range []struct {
		name string
		keys []string
	}{
		{"home", []string{"1"}},
		{"installed", []string{"2", "j", "j", "j"}}, // git, with its preview
		{"outdated", []string{"3"}},
		{"services", []string{"4"}},
		{"discover-all", []string{"6", "F"}},
		{"help", []string{"2", "?"}},
	} {
		keys(t, a, s.keys...)
		golden(t, s.name, stripANSI(a.render()))
		if a.help {
			keys(t, a, "esc") // any key closes help
		}
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/ui/ -run TestScreens`
Expected: FAIL: `open testdata/golden/home.txt: no such file or directory (go test ./internal/ui -run TestScreens -update writes it)`.

- [ ] **Step 3: Write the snapshots, and read them**

Run: `go test ./internal/ui/ -run TestScreens -update && ls internal/ui/testdata/golden`
Expected: six files. Open each and check it shows its screen with fixture data. For example:
- `installed.txt` has `Top-level 5   All 7   Apps 2   Dependencies 2`, and the git preview with `└─ pcre2`;
- `home.txt` has the five tiles (`7`, `2`, `1 error`, `2 warnings`, `86.7MB`);
- `discover-all.txt` has the All view starting at `abcl`;
- `help.txt` has the help box over Installed.

None should say `loading…` or `starting brewery…`.

- [ ] **Step 4: Run it again, several times**

Run: `go test -race -count=5 ./internal/ui/ -run TestScreens && golangci-lint run ./...`
Expected: `ok` (the snapshots are stable) and `0 issues.`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/golden_test.go internal/ui/testdata/golden
git commit -m "test: golden snapshots of brewery's main screens"
```

---

### Task 5: GoReleaser cask for dillo/homebrew-tap; CI and release workflows; RELEASING.md

**Files:**
- Create: `.goreleaser.yaml`
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`
- Create: `RELEASING.md`
- Modify: `Makefile`

**Interfaces:** `make snapshot` builds the release into `dist/`, which `.gitignore` already ignores (`/dist/`).

- [ ] **Step 1: Create `.goreleaser.yaml`**

```yaml
# GoReleaser builds brewery's release and writes its cask to dillo/homebrew-tap.
# Check it with `goreleaser check`; build it locally with `make snapshot`.
version: 2

project_name: brewery

before:
  hooks:
    - go mod tidy -diff

builds:
  - main: ./cmd/brewery
    binary: brewery
    env:
      - CGO_ENABLED=0
    goos: [darwin]
    goarch: [arm64, amd64]
    flags: [-trimpath]
    ldflags:
      - -s -w -X main.version={{ .Version }}

archives:
  - formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - LICENSE
      - README.md

checksum:
  name_template: checksums.txt

release:
  github:
    owner: dillo
    name: brewery

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"

homebrew_casks:
  - name: brewery
    binaries: [brewery]
    homepage: https://github.com/dillo/brewery
    description: Terminal dashboard for Homebrew
    repository:
      owner: dillo
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    # brewery isn't signed by Apple: clear the quarantine flag macOS puts on
    # downloads, or Gatekeeper blocks the binary. postflight_steps is
    # Homebrew's current form; GoReleaser's hooks still write the deprecated
    # postflight block (until v2.19), so it's written here directly.
    custom_block: |
      postflight_steps do
        run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "brewery"], chdir: "."
      end
```

- [ ] **Step 2: Check it and build a snapshot**

Run: `HOMEBREW_TAP_GITHUB_TOKEN=x goreleaser check && HOMEBREW_TAP_GITHUB_TOKEN=x goreleaser release --snapshot --clean`
Expected: `1 configuration file(s) validated`. The snapshot builds `darwin_amd64_v1` and `darwin_arm64_v8.0`, archives both, writes `dist/homebrew/Casks/brewery.rb` and ends `release succeeded`. It also logs "ignoring errors because this is a snapshot … No remote configured", which is fine: nothing is published.

- [ ] **Step 3: Inspect what it built**

Run:

```bash
file dist/brewery_darwin_*/brewery
./dist/brewery_darwin_arm64_v8.0/brewery --version
tar tzf dist/brewery_*_darwin_arm64.tar.gz
cat dist/homebrew/Casks/brewery.rb
ruby -c dist/homebrew/Casks/brewery.rb
HOMEBREW_NO_AUTO_UPDATE=1 brew ruby -e 'c = Cask::CaskLoader::FromContentLoader.new(File.read(ARGV.first)).load(config: nil); c.artifacts.each { |a| puts a.class.name }; p c.artifacts.find { |x| x.is_a?(Cask::Artifact::PostflightSteps) }.steps' dist/homebrew/Casks/brewery.rb
```

Expected:
- two Mach-O executables (x86_64, arm64);
- `brewery 0.0.0-SNAPSHOT-none`;
- the archive holds `LICENSE`, `README.md`, `brewery`;
- the cask has `postflight_steps`, `on_arm`/`on_intel` urls under `https://github.com/dillo/brewery/releases/download/`, `binary "brewery"`;
- `Syntax OK`;
- brew's loader lists `Cask::Artifact::Binary` and `Cask::Artifact::PostflightSteps`, with steps `[{"command" => {"path" => "/usr/bin/xattr"}, "chdir" => {"base" => "staged_path", "path" => "."}, "type" => "run", "args" => ["-dr", "com.apple.quarantine", "brewery"]}]`, and no deprecation warning.

Don't install the cask: brew only installs casks from a tap, and testing installs nothing.

- [ ] **Step 4: Create `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  push:
  pull_request:

permissions:
  contents: read

jobs:
  test:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - name: Test
        run: go test -race ./...
      - name: Vet
        run: go vet ./...
      - name: Format
        run: test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }
      - name: Tidy
        run: go mod tidy -diff
      - uses: golangci/golangci-lint-action@v9
        with:
          version: v2.14.0
```

- [ ] **Step 5: Create `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write # the GitHub release

jobs:
  release:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0 # the changelog needs the history
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - name: Test
        run: go test -race ./...
      - uses: goreleaser/goreleaser-action@v7
        with:
          version: "~> v2.18"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
```

- [ ] **Step 6: Check the workflows**

Run: `actionlint .github/workflows/*.yml` (install it with `go install github.com/rhysd/actionlint/cmd/actionlint@latest`, or `brew install actionlint`).
Expected: no output.

- [ ] **Step 7: Add `make snapshot`, in `Makefile`**

Replace:

```makefile
.PHONY: build test test-live lint run
```

with:

```makefile
.PHONY: build test test-live lint snapshot run
```

and insert above `run: build`:

```makefile
# Builds the release locally into dist/ without publishing: brew install goreleaser
snapshot:
	goreleaser release --snapshot --clean

```

- [ ] **Step 8: Create `RELEASING.md`**

````markdown
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
   Actions secret `HOMEBREW_TAP_GITHUB_TOKEN`.
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
````

- [ ] **Step 9: Commit**

```bash
git status --short   # no dist/
git add .goreleaser.yaml .github RELEASING.md Makefile
git commit -m "build: GoReleaser cask for dillo/homebrew-tap; CI and release workflows; RELEASING.md"
```

---

### Task 6: Final checks

**Files:** none (verification only)

- [ ] **Step 1: Everything**

```bash
go test -race -count=1 ./...
go test -tags live -count=1 ./internal/...
go vet ./... && go vet -tags live ./...
gofmt -l .
go mod tidy -diff
golangci-lint run ./...
HOMEBREW_TAP_GITHUB_TOKEN=x goreleaser check
actionlint .github/workflows/*.yml
```

Expected: all `ok`, `0 issues.`, the config validated, and nothing else printed.

- [ ] **Step 2: The real binary** (absolute scratch XDG dirs)
  - Build it: `go build -o <scratch>/brewery ./cmd/brewery`.
  - **With a fake old brew:** make `<scratch>/oldbrew/bin/brew` a script that prints `Homebrew 6.4.2`, then run `HOMEBREW_PREFIX=<scratch>/oldbrew <scratch>/brewery`. It exits 1 with ``brewery needs Homebrew 7.0 or newer (you have 6.4.2). Update it with `brew update`.``
  - **With the real brew:** brewery opens Home as before, and `q` quits.
