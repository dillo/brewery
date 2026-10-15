# brewery Plan 7: Discover — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A sixth tab, Discover, for finding formulae, apps and fonts you didn't know to look for, and installing them.
- **Views:** Popular tools, Popular apps, Rising, New, Fonts, All.
- **Preview:** what brew says about the package, what installing it would add, and its installs.
- **Installing:** `a`, after a dialog that names what comes along.
- **Completion:** commands that take any package complete from the whole catalogue.

**Architecture:**
- **`internal/brew`** (read-only additions):
  - the catalogue, from `brew search --desc ""`;
  - what brew's last update added, from its own name lists;
  - details for one package, installed or not: `brew info --json=v2` and `brew deps`, run at once.
- **`internal/analytics`** (new): Homebrew's public install counts from formulae.brew.sh.
  - Parsing, and Rising.
  - A `Store` that downloads each list and keeps the last copy.
- **`internal/ui`:**
  - the Discover tab, its views and its preview;
  - the analytics fetched once a day, one list after another;
  - the install dialog.
  The catalogue, analytics and details come in as Discover's own messages, through two interfaces `main` injects: `Catalog` (the brew client) and `Analytics` (an `analytics.Store`).
- **The UI still never runs brew or uses the network itself.**

**Tech Stack:**
- Go ≥ 1.25
- Bubble Tea v2, Bubbles v2, Lip Gloss v2
- `net/http` for the analytics. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-15-brewery-discover-design.md`, all of it.

**Roadmap:** the release (goreleaser, the tap, the module rename, CI, golangci-lint, teatest) is now Plan 8, and it waits until the user asks.

**Found while building this:**
- **`brew search --desc ""` lists the whole catalogue** in about 0.5s: 8,663 formulae and 7,783 casks on brew 7.
  - A cask line carries its display names in brackets, and they can nest: `ape: (ApE (A Plasmid Editor)) Software for…`.
  - Fonts have no description (`(Fira Code) [no description]`), so they keep their display name.
- **Rising needs a floor below as well as above.** A package released this month often has a handful of installs before release, which would make it ×5000. Fewer than 10 installs a month before counts as `new`. Packages that didn't grow are left out.
- **brew keeps the name lists from before an update** beside the current ones (`api/formula_names.before.txt`), so New needs no network.
- **Each preview costs about 0.45s of brew,** with info and deps run at once. brew is asked only once the cursor rests on a package for 250ms, and answers are kept until the catalogue is read again.
- **It's fast:** with the real catalogue and analytics, opening Discover takes 34ms of work in the UI, and filtering 16,443 packages takes about 5ms a keystroke.

**Verified:** every code block in this plan was compiled and tested before the plan was written.
- **The edits are exact.** A checker applied this plan's edits to `master` (d37af76) and confirmed that:
  - each task reproduces the tested code byte for byte;
  - each task's tests fail before its code and pass after (Task 10 adds only wiring and live tests);
  - `go mod tidy` stays clean.
  Every task's commit passes `go test -race` on its own.
- **Live, read-only:** the live tests read the real catalogue, ask about `wget` and `firefox`, and download one analytics list.
- **The real binary** ran at 120×34 in a PTY, with its cache and state in a scratch directory:
  - Discover opened with "fetching analytics…", then filled Popular tools (gh 217k first, installed packages marked ✓), Rising and the preview (uv: licence, homepage, installs over 30, 90 and 365 days).
  - `/cowsay` on All found cowsay and ponysay.
  - `a` asked "Install cowsay 3.8.4?", and `n` cancelled it. Nothing was installed.
  - `:install ripgr` suggested ripgrep and ripgrep-all.
  - The analytics landed in the scratch cache.

## Global Constraints

These come from Plans 1–6 and still hold:
- Module path `brewery`; Go ≥ 1.25.
- **Only `internal/brew` and `main` run brew.** Only `internal/analytics` uses the network.
- Views use `theme.Styles` only. Table cells have no color of their own; `theme.Tint` colors text in a cell.
- One job at a time; lock retries; pane actions get `brew.JobEnv`.
- Key routing is by mode first (Plan 5's order). A confirmation gets every key.
- Refreshes are never dropped; the cache is saved before doctor runs.
- `splitFor`: side by side from 100 columns, the preview under the list from 60, otherwise one at a time.
- Real-binary runs must use **absolute** scratch paths for `XDG_STATE_HOME` and `XDG_CACHE_HOME`. brewery ignores relative ones and would use the real folders.
- **Never install anything for real while testing.** Open the install dialog and answer `n`.

Added by this plan:
- **The tab:** Discover is tab 6. `1`–`6`, `]` and `[` reach it, and help says `1-6 [ ]`.
- **Views,** in the order `f` steps through them:

  | View | Rows | Columns |
  |---|---|---|
  | Popular tools | the top 500 formulae from `install-on-request/30d` | #, NAME, DESCRIPTION, INSTALLS |
  | Popular apps | the top 500 casks from `cask-install/30d`, without `font-…` | #, NAME, DESCRIPTION, INSTALLS |
  | Rising | the top 50 risers, formulae and casks together | #, NAME, TYPE, GROWTH, DESCRIPTION |
  | New | added since brew last updated | NAME, TYPE, DESCRIPTION |
  | Fonts | `font-…` casks, most installed first, then the rest by name | #, NAME, DESCRIPTION, INSTALLS |
  | All | the whole catalogue | NAME, TYPE, DESCRIPTION |

- **Rows and cells:**
  - Packages the catalogue doesn't have (from untapped taps) are left out, and ranks count what's shown.
  - Installed packages show `✓ ` before the name, with every cell tinted Overlay0.
  - INSTALLS is `shortCount`: `249`, `2.8k`, `54k`, `217k`, `3.0M`, and blank for 0.
  - GROWTH is `new`, `×4.1` below 10, and `×24` from 10.
  - Column sizes: `#` 3; NAME 20 (floor 12); TYPE 9 (floor 4); GROWTH 6; INSTALLS 8; DESCRIPTION flexible, keeping 20.
- **Rising** (in `analytics`):
  - growth = 30-day installs ÷ ((90-day − 30-day) ÷ 2);
  - at least `MinInstalls = 300` in the last 30 days;
  - `new` when the month before averages under `NewBelow = 10`;
  - growth must be over 1;
  - order (`Compare`): new first, then growth, then installs, then name.
- **Analytics fetching:**
  - Lists in fetch order: `install-on-request/30d`, `cask-install/30d`, `install-on-request/90d`, `cask-install/90d`, `install-on-request/365d`, from `https://formulae.brew.sh/api/analytics/<list>.json`.
  - User agent `brewery/<version>`; a 30-second timeout.
  - Each copy is kept as `<cache dir>/analytics/<list with / as ->.json`, written atomically. Its fetch time is the file's modification time.
  - The kept copies are read the first time Discover opens. Then all five lists are fetched, one after another, unless every copy is from today or a fetch already started today. `r` on Discover fetches anyway.
  - The first failure stops the fetch, and the copies stay.
- **Discover's messages:** with no rows, an analytics view says:
  - "fetching analytics from formulae.brew.sh…" while fetching;
  - "couldn't reach formulae.brew.sh · r retries" after a failure;
  - "no analytics yet · r fetches them" with none.
  New says "nothing new since brew last updated". The title adds "fetching analytics…", or "analytics from Jan 2" when the oldest copy isn't from today, on the analytics views only.
- **The catalogue** is read the first time Discover or the `:` line opens. After that, every refresh (`r`, and the one after `U`) reads it again. A failed read shows the error in the tab, and `r` retries it.
- **Preview:**
  - brew is asked after the cursor rests for `detailsDelay = 250ms`, and only about the package it's still on.
  - It shows the name and version, the description, then type, installed (`yes` or `no`), tap, licence and homepage.
  - "Would also install": what the package needs that isn't installed, or "nothing else", "asking brew…" or "couldn't ask brew: …".
  - "Installs": 30, 90 and 365 days for formulae, 30 and 90 for casks, with thousands separators.
  - `tab`/`enter` focus it and `j`/`k` scroll; `esc` returns to the list.
- **Install dialog:**
  - Title "Install <name> <version>?" (the version once brew has said it).
  - Then the description, after "app · " or "font · " for casks.
  - Then "also installs N dependency/dependencies: a, b, c, d, e +N more", or "checking dependencies…" until brew answers, when the open dialog updates.
  - `y` runs `brew install [--cask] <name>` with `HOMEBREW_NO_ASK=1` and refreshes `diskQueries`.
  - `a` on an installed package only says "<name> is already installed".
- **Completion:** the kinds `formulae_all` and `casks_all` complete from the catalogue.

## File Structure

```
internal/brew/types.go           Entry; Package.License
internal/brew/catalogue.go       Catalogue, ParseCatalogue, dropDisplayNames, NewSinceUpdate, added (new)
internal/brew/details.go         Details, (*Client) Details (new)
internal/brew/parse.go           a formula's licence
internal/brew/testdata/          catalogue.txt, details-ripgrep.json (new)
internal/analytics/analytics.go  List, Item, Parse, Counts, Rising, Compare (new)
internal/analytics/store.go      the list names, Store, NewStore, Cached, Fetch, save (new)
internal/analytics/testdata/     five lists cut from the real ones (new)
internal/ui/viewbar.go           viewBar, shared by Installed and Discover (new)
internal/ui/discover.go          Catalog, the tab, its views, rows and cells (new)
internal/ui/stats.go             Analytics, reading and fetching, statsAge, shortCount, growthCell (new)
internal/ui/details.go           asking brew where the cursor rests; the preview; withCommas (new)
internal/ui/install.go           askInstall, installBox, alsoInstalls (new)
internal/ui/app.go               tab 6, showTab, messages, r on Discover, esc from its preview
internal/ui/installed.go         viewBar moved out
internal/ui/filter.go, links.go, actions.go, help.go, confirm.go, cmdline.go, complete.go
cmd/brewery/main.go              UseCatalog, UseAnalytics
```

---

### Task 1: brew's catalogue, and what its last update added

**Files:**
- Create: `internal/brew/catalogue.go`
- Create: `internal/brew/catalogue_test.go`
- Create: `internal/brew/testdata/catalogue.txt`
- Modify: `internal/brew/types.go`

**Interfaces:**
- **Consumes:** `Client.query`, `fakeRunner` (tests).
- **Produces:**
  - `brew.Entry{Name, Desc string; Kind Kind}`.
  - `(*Client) Catalogue(ctx) ([]Entry, error)` (runs `search --desc ""`) and `ParseCatalogue(out string) []Entry`.
  - `(*Client) NewSinceUpdate(ctx) ([]Entry, error)`: names only, no descriptions. Missing lists mean nothing new, not an error.

- [ ] **Step 1: Create `internal/brew/catalogue_test.go`**

```go
package brew

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseCatalogue(t *testing.T) {
	es := ParseCatalogue(string(fixture(t, "catalogue.txt")))
	if len(es) != 17 {
		t.Fatalf("got %d entries, want 17", len(es))
	}
	want := map[string]Entry{
		"abcl":           {"abcl", "Armed Bear Common Lisp: a full implementation of Common Lisp", KindFormula},
		"wget":           {"wget", "Internet file retriever", KindFormula},
		"0-ad":           {"0-ad", "Real-time strategy game", KindCask},
		"ape":            {"ape", "Software for DNA sequence analysis and annotation", KindCask},
		"font-fira-code": {"font-fira-code", "Fira Code", KindCask}, // no description: the display name says what it is
	}
	for _, e := range es {
		if w, ok := want[e.Name]; ok && e != w {
			t.Errorf("got %+v, want %+v", e, w)
		}
	}
}

func TestParseCatalogueSkipsNoise(t *testing.T) {
	out := "Warning: something\nno-colon line\n==> Formulae\nfd: Find\n\nbad name: x\n"
	if got, want := ParseCatalogue(out), []Entry{{"fd", "Find", KindFormula}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestClientCatalogue(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{"search --desc ": fixture(t, "catalogue.txt")}})
	es, err := c.Catalogue(context.Background())
	if err != nil || len(es) != 17 {
		t.Errorf("got %d entries, %v", len(es), err)
	}
}

func TestNewSinceUpdate(t *testing.T) {
	cache := t.TempDir()
	write := func(name, body string) {
		if err := os.MkdirAll(filepath.Join(cache, "api"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cache, "api", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("formula_names.before.txt", "fd\ngit\nopenssl@3.5\n")
	write("formula_names.txt", "fd\ngit\nopenssl@3.6\nportop\n")
	write("cask_names.before.txt", "firefox\nold-app\n")
	write("cask_names.txt", "firefox\nnew-app\n")
	c := NewClient(fakeRunner{out: map[string][]byte{"--cache": []byte(cache + "\n")}})
	got, err := c.NewSinceUpdate(context.Background())
	want := []Entry{{Name: "openssl@3.6", Kind: KindFormula}, {Name: "portop", Kind: KindFormula}, {Name: "new-app", Kind: KindCask}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, %v; want %+v", got, err, want)
	}

	// Before brew's first update with the lists, there is nothing to compare.
	c = NewClient(fakeRunner{out: map[string][]byte{"--cache": []byte(t.TempDir())}})
	if got, err := c.NewSinceUpdate(context.Background()); err != nil || len(got) != 0 {
		t.Errorf("without the lists: got %+v, %v", got, err)
	}
}
```

- [ ] **Step 2: Create `internal/brew/testdata/catalogue.txt`**

```text
==> Formulae
abcl: Armed Bear Common Lisp: a full implementation of Common Lisp
docker: Pack, ship and run any application as a lightweight container
fd: Simple, fast and user-friendly alternative to find
gh: GitHub command-line tool
git: Distributed revision control system
pcre2: Perl compatible regular expressions library with a new API
python@3.15: Interpreted, interactive, object-oriented programming language
ripgrep: Search tool like grep and The Silver Searcher
wget: Internet file retriever

==> Casks
0-ad: (0 A.D.) Real-time strategy game
ape: (ApE (A Plasmid Editor)) Software for DNA sequence analysis and annotation
codex: (Codex) OpenAI's coding agent that runs in your terminal
docker-desktop: (Docker Desktop, Docker Community Edition, Docker CE) App to build and share containerised applications and microservices
firefox: (Mozilla Firefox) Web browser
font-fira-code: (Fira Code) [no description]
font-jetbrains-mono: (JetBrains Mono) [no description]
fontbase: (FontBase) Font manager
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/brew/catalogue_test.go:12:8: undefined: ParseCatalogue
    internal/brew/catalogue_test.go:16:21: undefined: Entry
    internal/brew/catalogue_test.go:32:18: undefined: ParseCatalogue
```

- [ ] **Step 4: Create `internal/brew/catalogue.go`**

```go
package brew

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Catalogue is every formula and cask brew can install, with their
// descriptions, from brew's local package data.
func (c *Client) Catalogue(ctx context.Context) ([]Entry, error) {
	out, err := c.query(ctx, "search", "--desc", "")
	if err != nil {
		return nil, err
	}
	return ParseCatalogue(string(out)), nil
}

// ParseCatalogue reads `brew search --desc ""`: "name: description" lines
// under "==> Formulae" and "==> Casks".
func ParseCatalogue(out string) []Entry {
	var es []Entry
	kind, in := KindFormula, false
	for _, line := range strings.Split(out, "\n") {
		switch line = strings.TrimSpace(line); line {
		case "==> Formulae":
			kind, in = KindFormula, true
			continue
		case "==> Casks":
			kind, in = KindCask, true
			continue
		}
		name, desc, ok := strings.Cut(line, ":")
		if !in || !ok || name == "" || strings.Contains(name, " ") {
			continue
		}
		desc = strings.TrimSpace(desc)
		if kind == KindCask {
			desc = dropDisplayNames(desc)
		}
		es = append(es, Entry{Name: name, Desc: desc, Kind: kind})
	}
	return es
}

// dropDisplayNames drops the display names in brackets that start a cask's
// description, which can nest: "(ApE (A Plasmid Editor)) Software for…".
// A cask with no description keeps its display names instead.
func dropDisplayNames(desc string) string {
	if !strings.HasPrefix(desc, "(") {
		return desc
	}
	depth := 0
	for i, r := range desc {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth > 0 {
				continue
			}
			if rest := strings.TrimSpace(desc[i+1:]); rest != "" && rest != "[no description]" {
				return rest
			}
			return desc[1:i]
		}
	}
	return desc
}

// NewSinceUpdate lists what brew's last update added. brew keeps the name
// lists from before an update beside the current ones; without them (brew
// hasn't updated since it started keeping them) there is nothing new.
func (c *Client) NewSinceUpdate(ctx context.Context) ([]Entry, error) {
	out, err := c.query(ctx, "--cache")
	if err != nil {
		return nil, err
	}
	api := filepath.Join(strings.TrimSpace(string(out)), "api")
	var es []Entry
	for _, l := range []struct {
		file string
		kind Kind
	}{{"formula_names", KindFormula}, {"cask_names", KindCask}} {
		before, err := os.ReadFile(filepath.Join(api, l.file+".before.txt"))
		if err != nil {
			continue
		}
		now, err := os.ReadFile(filepath.Join(api, l.file+".txt"))
		if err != nil {
			continue
		}
		for _, name := range added(string(before), string(now)) {
			es = append(es, Entry{Name: name, Kind: l.kind})
		}
	}
	return es, nil
}

// added lists the names in now that aren't in before, one name a line.
func added(before, now string) []string {
	had := map[string]bool{}
	for _, n := range strings.Fields(before) {
		had[n] = true
	}
	var out []string
	for _, n := range strings.Fields(now) {
		if !had[n] {
			out = append(out, n)
		}
	}
	return out
}
```

- [ ] **Step 5: Edit `internal/brew/types.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
// Key identifies a package: a formula and a cask can share a name.
func (p Package) Key() string { return p.Kind.String() + "/" + p.FullName }

// Outdated is one row of `brew outdated --json=v2`.
type Outdated struct {
	Name      string `json:"name"`
```

   with:

```go
// Key identifies a package: a formula and a cask can share a name.
func (p Package) Key() string { return p.Kind.String() + "/" + p.FullName }

// Entry is one package in brew's catalogue: anything it can install.
type Entry struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
	Kind Kind   `json:"kind"`
}

// Outdated is one row of `brew outdated --json=v2`.
type Outdated struct {
	Name      string `json:"name"`
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(brew): the catalogue, and what brew's last update added"
```

---

### Task 2: Details for one package, installed or not

**Files:**
- Create: `internal/brew/details.go`
- Create: `internal/brew/details_test.go`
- Modify: `internal/brew/parse.go`
- Create: `internal/brew/testdata/details-ripgrep.json`
- Modify: `internal/brew/types.go`

**Interfaces:**
- **Consumes:** `ParseInfo`, `Client.query`.
- **Produces:**
  - `Package.License` (a formula's `license`, parsed by `ParseInfo`).
  - `brew.Details{Package; Needs []string}` and `(*Client) Details(ctx, name string, kind Kind) (Details, error)`, which runs `deps --formula|--cask <name>` beside `info --json=v2 --formula|--cask <name>`.
  - When brew knows no such package, it's an error.

- [ ] **Step 1: Create `internal/brew/details_test.go`**

```go
package brew

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestClientDetails(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{
		"info --json=v2 --formula ripgrep": fixture(t, "details-ripgrep.json"),
		"deps --formula ripgrep":           []byte("pcre2\n"),
	}})
	d, err := c.Details(context.Background(), "ripgrep", KindFormula)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "ripgrep" || d.Latest != "15.2.0" || d.Version != "" || d.License != "Unlicense" || d.Homepage != "https://github.com/BurntSushi/ripgrep" {
		t.Errorf("got %+v", d.Package)
	}
	if !reflect.DeepEqual(d.Needs, []string{"pcre2"}) {
		t.Errorf("needs %v", d.Needs)
	}
}

func TestClientDetailsCask(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{
		"info --json=v2 --cask firefox": []byte(`{"formulae":[],"casks":[{"token":"firefox","full_token":"firefox","tap":"homebrew/cask","name":["Mozilla Firefox"],"desc":"Web browser","homepage":"https://www.mozilla.org/firefox/","version":"158.0","installed":null}]}`),
	}})
	d, err := c.Details(context.Background(), "firefox", KindCask)
	if err != nil || d.Kind != KindCask || d.Latest != "158.0" || len(d.Needs) != 0 {
		t.Errorf("got %+v, %v", d, err)
	}
}

func TestClientDetailsFails(t *testing.T) {
	broke := errors.New("exit status 1")
	c := NewClient(fakeRunner{
		out: map[string][]byte{"info --json=v2 --formula nope": []byte(`{"formulae":[],"casks":[]}`)},
		err: map[string]error{"deps --formula gone": broke, "info --json=v2 --formula gone": broke},
	})
	if _, err := c.Details(context.Background(), "nope", KindFormula); err == nil {
		t.Error("brew found nothing: want an error")
	}
	if _, err := c.Details(context.Background(), "gone", KindFormula); !errors.Is(err, broke) {
		t.Errorf("got %v, want brew's failure", err)
	}
}
```

- [ ] **Step 2: Create `internal/brew/testdata/details-ripgrep.json`**

```json
{
  "formulae": [
    {
      "name": "ripgrep",
      "full_name": "ripgrep",
      "tap": "homebrew/core",
      "desc": "Search tool like grep and The Silver Searcher",
      "license": "Unlicense",
      "homepage": "https://github.com/BurntSushi/ripgrep",
      "versions": {
        "stable": "15.2.0",
        "head": "HEAD",
        "bottle": true
      },
      "dependencies": [
        "pcre2"
      ],
      "installed": [],
      "pinned": false,
      "outdated": false,
      "deprecated": false,
      "caveats": null
    }
  ],
  "casks": []
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/brew/details_test.go:15:14: c.Details undefined (type *Client has no field or method Details)
    internal/brew/details_test.go:31:14: c.Details undefined (type *Client has no field or method Details)
    internal/brew/details_test.go:43:17: c.Details undefined (type *Client has no field or method Details)
```

- [ ] **Step 4: Create `internal/brew/details.go`**

```go
package brew

import (
	"context"
	"fmt"
	"strings"
)

// Details is what brew says about one package, installed or not: its info,
// and every formula it needs to run.
type Details struct {
	Package
	Needs []string // its dependencies at any depth, by name
}

// Details asks brew about one package, running its info and its
// dependencies at once.
func (c *Client) Details(ctx context.Context, name string, kind Kind) (Details, error) {
	flag := "--formula"
	if kind == KindCask {
		flag = "--cask"
	}
	type result struct {
		out []byte
		err error
	}
	deps := make(chan result, 1)
	go func() {
		out, err := c.query(ctx, "deps", flag, name)
		deps <- result{out, err}
	}()
	info, err := c.query(ctx, "info", "--json=v2", flag, name)
	d := <-deps
	if err != nil {
		return Details{}, err
	}
	if d.err != nil {
		return Details{}, d.err
	}
	pkgs, err := ParseInfo(info)
	if err != nil {
		return Details{}, err
	}
	if len(pkgs) == 0 {
		return Details{}, fmt.Errorf("brew has no %s named %s", kind, name)
	}
	return Details{Package: pkgs[0], Needs: strings.Fields(string(d.out))}, nil
}
```

- [ ] **Step 5: Edit `internal/brew/parse.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
		Tap      string  `json:"tap"`
		Desc     *string `json:"desc"`
		Homepage string  `json:"homepage"`
		Versions struct {
			Stable string `json:"stable"`
		} `json:"versions"`
```

   with:

```go
		Tap      string  `json:"tap"`
		Desc     *string `json:"desc"`
		Homepage string  `json:"homepage"`
		License  *string `json:"license"`
		Versions struct {
			Stable string `json:"stable"`
		} `json:"versions"`
```

2. Replace:

```go
	for _, f := range in.Formulae {
		p := Package{
			Name: f.Name, FullName: f.FullName, Kind: KindFormula, Tap: f.Tap,
			Desc: deref(f.Desc), Homepage: f.Homepage, Latest: f.Versions.Stable,
			Pinned: f.Pinned, Outdated: f.Outdated, Deprecated: f.Deprecated,
			Caveats: strings.TrimSpace(deref(f.Caveats)),
		}
```

   with:

```go
	for _, f := range in.Formulae {
		p := Package{
			Name: f.Name, FullName: f.FullName, Kind: KindFormula, Tap: f.Tap,
			Desc: deref(f.Desc), Homepage: f.Homepage, License: deref(f.License), Latest: f.Versions.Stable,
			Pinned: f.Pinned, Outdated: f.Outdated, Deprecated: f.Deprecated,
			Caveats: strings.TrimSpace(deref(f.Caveats)),
		}
```

- [ ] **Step 6: Edit `internal/brew/types.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	Tap         string    `json:"tap"`
	Desc        string    `json:"desc"`
	Homepage    string    `json:"homepage"`
	Version     string    `json:"version"` // installed version
	Latest      string    `json:"latest"`  // newest version known to brew
	OnRequest   bool      `json:"on_request"`
```

   with:

```go
	Tap         string    `json:"tap"`
	Desc        string    `json:"desc"`
	Homepage    string    `json:"homepage"`
	License     string    `json:"license"` // a formula's SPDX licence
	Version     string    `json:"version"` // installed version
	Latest      string    `json:"latest"`  // newest version known to brew
	OnRequest   bool      `json:"on_request"`
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(brew): details for one package, installed or not, with its licence and what it needs"
```

---

### Task 3: Parse Homebrew's install counts and find what's rising

**Files:**
- Create: `internal/analytics/analytics.go`
- Create: `internal/analytics/analytics_test.go`
- Create: `internal/analytics/testdata/cask-install-30d.json`
- Create: `internal/analytics/testdata/cask-install-90d.json`
- Create: `internal/analytics/testdata/install-on-request-30d.json`
- Create: `internal/analytics/testdata/install-on-request-365d.json`
- Create: `internal/analytics/testdata/install-on-request-90d.json`

**Interfaces:**
- **Consumes:** nothing; this is a new package.
- **Produces:**
  - `analytics.List{Start, End string; Items []Item}`, `Item{Name string; Rank, Count int}`.
  - `Parse([]byte) (List, error)`, which rejects JSON without an `end_date`; `(List) Counts() map[string]int`.
  - `MinInstalls = 300`, `NewBelow = 10`.
  - `Riser{Name string; Count int; Growth float64; New bool}`, `Rising(d30, d90 List) []Riser`, `Compare(a, b Riser) int`.

- [ ] **Step 1: Create `internal/analytics/analytics_test.go`**

```go
package analytics

import (
	"os"
	"reflect"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParse(t *testing.T) {
	l, err := Parse(fixture(t, "install-on-request-30d"))
	if err != nil {
		t.Fatal(err)
	}
	if l.Start != "2026-09-15" || l.End != "2026-10-15" || len(l.Items) != 7 {
		t.Fatalf("got %s to %s, %d items", l.Start, l.End, len(l.Items))
	}
	if got, want := l.Items[0], (Item{Name: "gh", Rank: 1, Count: 217104}); got != want {
		t.Errorf("first item %+v, want %+v", got, want)
	}
	if got := l.Items[6]; got.Name != "dagsommer/boks/boks" || got.Count != 1 {
		t.Errorf("last item %+v", got)
	}

	casks, err := Parse(fixture(t, "cask-install-30d"))
	if err != nil || casks.Items[0] != (Item{Name: "codex", Rank: 1, Count: 135078}) {
		t.Errorf("casks: %+v, %v", casks.Items, err)
	}
}

func TestParseRejects(t *testing.T) {
	for _, b := range []string{`not json`, `{"error":"rate limited"}`, `<html>`} {
		if _, err := Parse([]byte(b)); err == nil {
			t.Errorf("%q: want an error", b)
		}
	}
	// A broken item is skipped, not fatal.
	l, err := Parse([]byte(`{"start_date":"a","end_date":"b","items":[{"number":1,"formula":"x","count":"n/a"},{"number":2,"formula":"y","count":"1,000"}]}`))
	if err != nil || !reflect.DeepEqual(l.Items, []Item{{Name: "y", Rank: 2, Count: 1000}}) {
		t.Errorf("got %+v, %v", l.Items, err)
	}
}

func list(items ...Item) List { return List{Start: "s", End: "e", Items: items} }

func TestRising(t *testing.T) {
	d30 := list(
		Item{"steady", 1, 3000},  // 3000 a month before too: ×1, so not rising
		Item{"doubled", 2, 2000}, // 1000 a month before: ×2
		Item{"fresh", 3, 900},    // 9 a month before: too few to count
		Item{"newer", 4, 1500},   // nothing before either, but more installs
		Item{"tied-b", 5, 600},   // ×2, like doubled, with fewer installs
		Item{"falling", 8, 1000}, // 2000 a month before: ×0.5
		Item{"tied-a", 6, 600},   // same again: then by name
		Item{"tiny", 7, 299},     // under the floor
	)
	d90 := list(
		Item{"steady", 1, 9000}, Item{"doubled", 2, 4000}, Item{"fresh", 3, 918},
		Item{"tied-b", 4, 1200}, Item{"tied-a", 5, 1200}, Item{"tiny", 6, 300}, Item{"falling", 7, 5000},
	)
	got := Rising(d30, d90)
	want := []Riser{
		{Name: "newer", Count: 1500, New: true},
		{Name: "fresh", Count: 900, New: true},
		{Name: "doubled", Count: 2000, Growth: 2},
		{Name: "tied-a", Count: 600, Growth: 2},
		{Name: "tied-b", Count: 600, Growth: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestRisingFromFixtures(t *testing.T) {
	d30, _ := Parse(fixture(t, "install-on-request-30d"))
	d90, _ := Parse(fixture(t, "install-on-request-90d"))
	rs := Rising(d30, d90)
	if rs[0].Name != "python@3.15" || !rs[0].New {
		t.Errorf("first %+v, want python@3.15, new", rs[0])
	}
	if rs[1].Name != "wget" || rs[1].Growth < 4 || rs[1].Growth > 4.01 {
		t.Errorf("second %+v, want wget at ×4.0", rs[1])
	}
	if len(rs) != 3 || rs[2].Name != "git" {
		t.Errorf("got %+v, want python@3.15, wget and git: the rest didn't grow", rs)
	}
}

func TestCounts(t *testing.T) {
	l := list(Item{"a", 1, 10}, Item{"b", 2, 5})
	if got := l.Counts(); !reflect.DeepEqual(got, map[string]int{"a": 10, "b": 5}) {
		t.Errorf("got %v", got)
	}
}
```

- [ ] **Step 2: Create `internal/analytics/testdata/cask-install-30d.json`**

```json
{
 "category": "cask_install",
 "total_items": 12057,
 "start_date": "2026-09-15",
 "end_date": "2026-10-15",
 "total_count": 1813169,
 "items": [
  {
   "number": 1,
   "cask": "codex",
   "count": "135,078",
   "percent": "7.45"
  },
  {
   "number": 7,
   "cask": "docker-desktop",
   "count": "32,443",
   "percent": "1.79"
  },
  {
   "number": 30,
   "cask": "firefox",
   "count": "10,902",
   "percent": "0.60"
  },
  {
   "number": 116,
   "cask": "font-jetbrains-mono",
   "count": "2,810",
   "percent": "0.15"
  },
  {
   "number": 228,
   "cask": "font-fira-code",
   "count": "1,145",
   "percent": "0.06"
  },
  {
   "number": 677,
   "cask": "0-ad",
   "count": "249",
   "percent": "0.01"
  }
 ]
}
```

- [ ] **Step 3: Create `internal/analytics/testdata/cask-install-90d.json`**

```json
{
 "category": "cask_install",
 "total_items": 18102,
 "start_date": "2026-07-17",
 "end_date": "2026-10-15",
 "total_count": 5670700,
 "items": [
  {
   "number": 1,
   "cask": "codex",
   "count": "345,161",
   "percent": "6.09"
  },
  {
   "number": 6,
   "cask": "docker-desktop",
   "count": "113,628",
   "percent": "2"
  },
  {
   "number": 27,
   "cask": "firefox",
   "count": "34,608",
   "percent": "0.61"
  },
  {
   "number": 115,
   "cask": "font-jetbrains-mono",
   "count": "9,220",
   "percent": "0.16"
  },
  {
   "number": 211,
   "cask": "font-fira-code",
   "count": "4,078",
   "percent": "0.07"
  },
  {
   "number": 725,
   "cask": "0-ad",
   "count": "743",
   "percent": "0.01"
  }
 ]
}
```

- [ ] **Step 4: Create `internal/analytics/testdata/install-on-request-30d.json`**

```json
{
 "category": "formula_install_on_request",
 "total_items": 22824,
 "start_date": "2026-09-15",
 "end_date": "2026-10-15",
 "total_count": 6978526,
 "items": [
  {
   "number": 1,
   "formula": "gh",
   "count": "217,104",
   "percent": "3.11"
  },
  {
   "number": 6,
   "formula": "git",
   "count": "105,334",
   "percent": "1.51"
  },
  {
   "number": 14,
   "formula": "wget",
   "count": "54,269",
   "percent": "0.78"
  },
  {
   "number": 39,
   "formula": "ripgrep",
   "count": "30,895",
   "percent": "0.44"
  },
  {
   "number": 85,
   "formula": "fd",
   "count": "16,374",
   "percent": "0.23"
  },
  {
   "number": 228,
   "formula": "python@3.15",
   "count": "5,554",
   "percent": "0.08"
  },
  {
   "number": 18485,
   "formula": "dagsommer/boks/boks",
   "count": "1",
   "percent": "0"
  }
 ]
}
```

- [ ] **Step 5: Create `internal/analytics/testdata/install-on-request-365d.json`**

```json
{
 "category": "formula_install_on_request",
 "total_items": 88711,
 "start_date": "2025-10-15",
 "end_date": "2026-10-15",
 "total_count": 100951216,
 "items": [
  {
   "number": 1,
   "formula": "gh",
   "count": "2,994,048",
   "percent": "2.97"
  },
  {
   "number": 6,
   "formula": "git",
   "count": "1,440,736",
   "percent": "1.43"
  },
  {
   "number": 32,
   "formula": "ripgrep",
   "count": "530,224",
   "percent": "0.53"
  },
  {
   "number": 60,
   "formula": "wget",
   "count": "320,412",
   "percent": "0.32"
  },
  {
   "number": 91,
   "formula": "fd",
   "count": "222,114",
   "percent": "0.22"
  },
  {
   "number": 1353,
   "formula": "python@3.15",
   "count": "5,554",
   "percent": "0.01"
  },
  {
   "number": 48852,
   "formula": "dagsommer/boks/boks",
   "count": "1",
   "percent": "0"
  }
 ]
}
```

- [ ] **Step 6: Create `internal/analytics/testdata/install-on-request-90d.json`**

```json
{
 "category": "formula_install_on_request",
 "total_items": 41764,
 "start_date": "2026-07-17",
 "end_date": "2026-10-15",
 "total_count": 21911676,
 "items": [
  {
   "number": 1,
   "formula": "gh",
   "count": "710,524",
   "percent": "3.24"
  },
  {
   "number": 7,
   "formula": "git",
   "count": "256,356",
   "percent": "1.17"
  },
  {
   "number": 23,
   "formula": "ripgrep",
   "count": "143,527",
   "percent": "0.66"
  },
  {
   "number": 49,
   "formula": "wget",
   "count": "81,374",
   "percent": "0.37"
  },
  {
   "number": 90,
   "formula": "fd",
   "count": "52,510",
   "percent": "0.24"
  },
  {
   "number": 524,
   "formula": "python@3.15",
   "count": "5,556",
   "percent": "0.03"
  },
  {
   "number": 29439,
   "formula": "dagsommer/boks/boks",
   "count": "1",
   "percent": "0"
  }
 ]
}
```

- [ ] **Step 7: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/analytics/analytics_test.go:19:12: undefined: Parse
    internal/analytics/analytics_test.go:26:31: undefined: Item
    internal/analytics/analytics_test.go:33:16: undefined: Parse
```

- [ ] **Step 8: Create `internal/analytics/analytics.go`**

```go
// Package analytics reads Homebrew's public install counts from
// formulae.brew.sh: how often each formula and cask was installed.
package analytics

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// List is one of Homebrew's analytics lists, most installed first.
type List struct {
	Start, End string // the days it covers, as "2026-09-15"
	Items      []Item
}

// Item is one package's installs over the list's days.
type Item struct {
	Name  string // "user/tap/name" for packages outside Homebrew's own taps
	Rank  int
	Count int
}

type listJSON struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Items     []struct {
		Number  int    `json:"number"`
		Formula string `json:"formula"`
		Cask    string `json:"cask"`
		Count   string `json:"count"`
	} `json:"items"`
}

// Parse reads one list. Counts come as text with thousands separators,
// "217,104"; an item whose count isn't a number is skipped.
func Parse(b []byte) (List, error) {
	var in listJSON
	if err := json.Unmarshal(b, &in); err != nil {
		return List{}, fmt.Errorf("parse analytics: %w", err)
	}
	if in.EndDate == "" {
		return List{}, errors.New("parse analytics: not an analytics list")
	}
	l := List{Start: in.StartDate, End: in.EndDate, Items: make([]Item, 0, len(in.Items))}
	for _, it := range in.Items {
		name := cmp.Or(it.Formula, it.Cask)
		n, err := strconv.Atoi(strings.ReplaceAll(it.Count, ",", ""))
		if name == "" || err != nil {
			continue
		}
		l.Items = append(l.Items, Item{Name: name, Rank: it.Number, Count: n})
	}
	return l, nil
}

// Counts maps each package in the list to its installs.
func (l List) Counts() map[string]int {
	m := make(map[string]int, len(l.Items))
	for _, it := range l.Items {
		m[it.Name] = it.Count
	}
	return m
}

// MinInstalls is how many installs in the last 30 days a package needs to
// count as rising, so a jump from 2 to 10 doesn't top the list.
const MinInstalls = 300

// NewBelow is the most installs a month a package can have averaged before
// the last 30 days and still count as new: a handful of installs before a
// release doesn't make it ×5000.
const NewBelow = 10

// Riser is a package whose installs grew.
type Riser struct {
	Name   string
	Count  int     // installs in the last 30 days
	Growth float64 // installs now over a month's installs before; 0 when New
	New    bool    // (almost) no installs before the last 30 days
}

// Rising compares each package's last 30 days with the 60 days before
// them, which the 90-day list covers too: growth is the last 30 days'
// installs over the average 30 days before. Packages that didn't grow are
// left out. The result is in Compare's order.
func Rising(d30, d90 List) []Riser {
	before := d90.Counts()
	var rs []Riser
	for _, it := range d30.Items {
		if it.Count < MinInstalls {
			continue
		}
		r := Riser{Name: it.Name, Count: it.Count}
		if month := float64(before[it.Name]-it.Count) / 2; month >= NewBelow {
			r.Growth = float64(it.Count) / month
		} else {
			r.New = true
		}
		if r.New || r.Growth > 1 {
			rs = append(rs, r)
		}
	}
	slices.SortFunc(rs, Compare)
	return rs
}

// Compare orders risers: new packages first, then by growth, then by
// installs, then by name.
func Compare(a, b Riser) int {
	if a.New != b.New {
		if a.New {
			return -1
		}
		return 1
	}
	return cmp.Or(cmp.Compare(b.Growth, a.Growth), cmp.Compare(b.Count, a.Count), strings.Compare(a.Name, b.Name))
}
```

- [ ] **Step 9: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat(analytics): parse Homebrew's install counts and find what's rising"
```

---

### Task 4: Download the lists and keep the last copy of each

**Files:**
- Create: `internal/analytics/store.go`
- Create: `internal/analytics/store_test.go`

**Interfaces:**
- **Consumes:** Task 3's `Parse`.
- **Produces:**
  - `Tools30`, `Tools90`, `Tools365`, `Apps30`, `Apps90`; `Lists` (fetch order); `BaseURL`.
  - `Store{Dir string; Client *http.Client; UserAgent, Base string}`, `NewStore(dir, version string) Store`.
  - `(Store) Cached(list) (List, time.Time, error)` and `(Store) Fetch(ctx, list) (List, error)`. Only a good list is saved; failing to save it doesn't fail the fetch.

- [ ] **Step 1: Create `internal/analytics/store_test.go`**

```go
package analytics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// server serves body for every request with status, recording the last
// request's path and user agent.
func server(t *testing.T, status int, body []byte, delay time.Duration) (*httptest.Server, *[2]string) {
	t.Helper()
	var got [2]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = [2]string{r.URL.Path, r.UserAgent()}
		time.Sleep(delay)
		w.WriteHeader(status)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestFetchKeepsACopy(t *testing.T) {
	srv, got := server(t, http.StatusOK, fixture(t, "install-on-request-30d"), 0)
	s := NewStore(t.TempDir(), "1.2.3")
	s.Base = srv.URL + "/api/analytics/"
	l, err := s.Fetch(context.Background(), Tools30)
	if err != nil || len(l.Items) != 7 {
		t.Fatalf("got %d items, %v", len(l.Items), err)
	}
	if *got != [2]string{"/api/analytics/install-on-request/30d.json", "brewery/1.2.3"} {
		t.Errorf("asked for %q", *got)
	}
	cached, at, err := s.Cached(Tools30)
	if err != nil || len(cached.Items) != 7 || time.Since(at) > time.Minute {
		t.Errorf("cached copy: %d items from %v, %v", len(cached.Items), at, err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "install-on-request-30d.json")); err != nil {
		t.Error(err)
	}
}

func TestFetchFailureKeepsTheCache(t *testing.T) {
	dir := t.TempDir()
	good, _ := server(t, http.StatusOK, fixture(t, "cask-install-30d"), 0)
	s := NewStore(dir, "dev")
	s.Base = good.URL + "/"
	if _, err := s.Fetch(context.Background(), Apps30); err != nil {
		t.Fatal(err)
	}

	broken, _ := server(t, http.StatusOK, []byte("<html>oops</html>"), 0)
	missing, _ := server(t, http.StatusNotFound, nil, 0)
	slow, _ := server(t, http.StatusOK, fixture(t, "cask-install-30d"), 200*time.Millisecond)
	for name, base := range map[string]string{"broken": broken.URL, "missing": missing.URL, "slow": slow.URL} {
		s := NewStore(dir, "dev")
		s.Base = base + "/"
		s.Client.Timeout = 50 * time.Millisecond
		_, err := s.Fetch(context.Background(), Apps30)
		if err == nil {
			t.Errorf("%s: want an error", name)
		}
		if name == "missing" && !strings.Contains(err.Error(), "404") {
			t.Errorf("missing: %v should give the status", err)
		}
		if l, _, err := s.Cached(Apps30); err != nil || len(l.Items) != 6 {
			t.Errorf("%s: the cache has %d items, %v", name, len(l.Items), err)
		}
	}
}

func TestCachedWithoutACopy(t *testing.T) {
	if _, _, err := NewStore(t.TempDir(), "dev").Cached(Tools90); err == nil {
		t.Error("want an error when nothing was fetched yet")
	}
}

func TestNewStore(t *testing.T) {
	s := NewStore("/x", "dev")
	if s.Base != BaseURL || s.Client.Timeout != 30*time.Second || s.UserAgent != "brewery/dev" {
		t.Errorf("got %+v", s)
	}
	if len(Lists) != 5 || Lists[0] != Tools30 || Lists[1] != Apps30 {
		t.Errorf("Lists = %v: the 30-day lists come first, for Popular", Lists)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/analytics/store_test.go:31:7: undefined: NewStore
    internal/analytics/store_test.go:33:42: undefined: Tools30
    internal/analytics/store_test.go:40:30: undefined: Tools30
```

- [ ] **Step 3: Create `internal/analytics/store.go`**

```go
package analytics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The lists Discover reads, named by their paths on formulae.brew.sh:
// tools are formulae people asked for (not ones pulled in as dependencies),
// apps are casks.
const (
	Tools30  = "install-on-request/30d"
	Tools90  = "install-on-request/90d"
	Tools365 = "install-on-request/365d"
	Apps30   = "cask-install/30d"
	Apps90   = "cask-install/90d"
)

// Lists is every list, in the order they're fetched: Popular's first.
var Lists = []string{Tools30, Apps30, Tools90, Apps90, Tools365}

// BaseURL is where Homebrew publishes its analytics.
const BaseURL = "https://formulae.brew.sh/api/analytics/"

// maxSize bounds a download; the largest list is about 4MB.
const maxSize = 64 << 20

// Store downloads analytics lists and keeps the last copy of each in Dir.
type Store struct {
	Dir       string
	Client    *http.Client
	UserAgent string
	Base      string // BaseURL; tests point it at a local server
}

// NewStore keeps copies in dir and names brewery's version in each request.
// A download gives up after 30 seconds.
func NewStore(dir, version string) Store {
	return Store{Dir: dir, Client: &http.Client{Timeout: 30 * time.Second}, UserAgent: "brewery/" + version, Base: BaseURL}
}

// path is where the copy of list is kept.
func (s Store) path(list string) string {
	return filepath.Join(s.Dir, strings.ReplaceAll(list, "/", "-")+".json")
}

// Cached is the last copy of list, and when it was fetched.
func (s Store) Cached(list string) (List, time.Time, error) {
	path := s.path(list)
	b, err := os.ReadFile(path)
	if err != nil {
		return List{}, time.Time{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return List{}, time.Time{}, err
	}
	l, err := Parse(b)
	return l, info.ModTime(), err
}

// Fetch downloads list and keeps a copy. A failed download leaves the
// copy as it was.
func (s Store) Fetch(ctx context.Context, list string) (List, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Base+list+".json", nil)
	if err != nil {
		return List{}, err
	}
	req.Header.Set("User-Agent", s.UserAgent)
	resp, err := s.Client.Do(req)
	if err != nil {
		return List{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return List{}, fmt.Errorf("%s: %s", list, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return List{}, err
	}
	l, err := Parse(b)
	if err != nil {
		return List{}, err
	}
	_ = s.save(list, b) // without a copy, the next start only shows the lists once they're fetched again
	return l, nil
}

// save writes the copy of list atomically, so a crash midway never leaves
// half a file.
func (s Store) save(list string, b []byte) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Dir, ".fetch-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // fails harmlessly once renamed
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.path(list))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(analytics): download the lists and keep the last copy of each"
```

---

### Task 5: A Discover tab with the whole catalogue and what's new

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`
- Create: `internal/ui/discover.go`
- Create: `internal/ui/discover_test.go`
- Modify: `internal/ui/filter.go`
- Modify: `internal/ui/help.go`
- Modify: `internal/ui/installed.go`
- Modify: `internal/ui/links.go`
- Create: `internal/ui/viewbar.go`

**Interfaces:**
- **Consumes:** Task 1's `Entry`, `Catalogue`, `NewSinceUpdate`; `kindCell`, `sizeTable`, `setRows`, `skeleton`, `errorBody`, `refreshBanner`, `matches`.
- **Produces:**
  - `viewSeg{name, short string; n int}` and `viewBar(st, segs, active, about, w)`, moved out of Installed, plus `viewBarHeight`.
  - `ui.Catalog` (Catalogue, NewSinceUpdate) and `(*App) UseCatalog`.
  - `tabDiscover`; `(*App) showTab(t int) tea.Cmd`, used by `1`–`6`, `]` and `[`.
  - `catalogueMsg`, `readCatalogue`, `reloadCatalogue` (called by `refresh`), `gotCatalogue`.
  - `discoverTab`: views `discNew`, `discAll`; `entryKey(name, kind)`; `discRow`; `setCatalogue`, `setHave`, `apply`, `cells`, `selected`, `selectedKey`, `matchText`/`setMatch`, `update`, `size`, `resize`, `view`, `empty`, `viewBar`.
  - The field holding the view is `showing`, because `view` is the render method.

`TestTabKeys` gains Discover.

- [ ] **Step 1: Edit `internal/ui/app_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		key  string
		want int
	}{
		{"3", tabOutdated}, {"]", tabServices}, {"]", tabTaps}, {"]", tabHome}, {"[", tabTaps}, {"1", tabHome},
	}
	for _, s := range steps {
		a.Update(press(s.key))
```

   with:

```go
		key  string
		want int
	}{
		{"3", tabOutdated}, {"]", tabServices}, {"]", tabTaps}, {"]", tabDiscover}, {"]", tabHome}, {"[", tabDiscover},
		{"5", tabTaps}, {"6", tabDiscover}, {"1", tabHome},
	}
	for _, s := range steps {
		a.Update(press(s.key))
```

- [ ] **Step 2: Create `internal/ui/discover_test.go`**

```go
package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"brewery/internal/brew"
)

// fakeCatalog serves the catalogue fixture; added is what brew's last
// update brought in. calls counts catalogue reads.
type fakeCatalog struct {
	entries, added []brew.Entry
	err            error
	calls          int
}

func newFakeCatalog(t *testing.T) *fakeCatalog {
	return &fakeCatalog{
		entries: brew.ParseCatalogue(string(readFixture(t, "catalogue.txt"))),
		added:   []brew.Entry{{Name: "python@3.15", Kind: brew.KindFormula}, {Name: "codex", Kind: brew.KindCask}},
	}
}

func (f *fakeCatalog) Catalogue(context.Context) ([]brew.Entry, error) {
	f.calls++
	return f.entries, f.err
}

func (f *fakeCatalog) NewSinceUpdate(context.Context) ([]brew.Entry, error) { return f.added, nil }

// newDiscoverApp is an app with packages loaded and a catalogue to read,
// still on Home.
func newDiscoverApp(t *testing.T) (*App, *fakeCatalog) {
	t.Helper()
	a := newTestApp(t, newFakeSource(t))
	cat := newFakeCatalog(t)
	a.UseCatalog(cat)
	drain(t, a, a.Init())
	return a, cat
}

// discoverRows are the names in the Discover table, as shown.
func discoverRows(a *App) []string {
	var names []string
	for _, r := range a.discover.table.Rows() {
		names = append(names, stripANSI(r[0]))
	}
	return names
}

func TestDiscoverReadsTheCatalogueOnce(t *testing.T) {
	a, cat := newDiscoverApp(t)
	if cat.calls != 0 {
		t.Fatalf("read the catalogue %d times before Discover opened", cat.calls)
	}
	keys(t, a, "6")
	if a.tab != tabDiscover || cat.calls != 1 || !a.discover.loaded {
		t.Fatalf("tab %d, %d reads, loaded %v", a.tab, cat.calls, a.discover.loaded)
	}
	keys(t, a, "1", "6", "]", "[")
	if cat.calls != 1 {
		t.Errorf("read the catalogue %d times, want once", cat.calls)
	}
	// brew update can change the catalogue: a refresh reads it again.
	keys(t, a, "r")
	if cat.calls != 2 {
		t.Errorf("after r: %d reads, want 2", cat.calls)
	}
}

func TestDiscoverViews(t *testing.T) {
	a, _ := newDiscoverApp(t)
	keys(t, a, "6")
	out := stripANSI(a.render())
	for _, want := range []string{"6 Discover", "New 2", "All 17", "Added to Homebrew since brew last updated."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	// New takes descriptions from the catalogue.
	if got := strings.Join(discoverRows(a), ","); got != "python@3.15,codex" {
		t.Errorf("New shows %s", got)
	}
	if !strings.Contains(out, "OpenAI's coding agent") {
		t.Errorf("codex's description is missing:\n%s", out)
	}

	keys(t, a, "f")
	rows := discoverRows(a)
	if len(rows) != 17 || rows[0] != "abcl" {
		t.Fatalf("All shows %v", rows)
	}
	// Installed packages stay in the list, marked.
	if !strings.Contains(strings.Join(rows, ","), "✓ git") || !strings.Contains(strings.Join(rows, ","), "✓ docker-desktop") {
		t.Errorf("installed packages aren't marked: %v", rows)
	}
	if strings.Contains(strings.Join(rows, ","), "✓ docker,") {
		t.Errorf("the docker formula isn't installed, only the cask: %v", rows)
	}
	keys(t, a, "F")
	if a.discover.showing != discNew {
		t.Errorf("F goes back to New, got %d", a.discover.showing)
	}
}

func TestDiscoverFilterSearchesDescriptions(t *testing.T) {
	a, _ := newDiscoverApp(t)
	keys(t, a, "6", "f", "/", "r", "e", "t", "r", "i", "e", "v", "e", "r", "enter")
	if got := discoverRows(a); len(got) != 1 || got[0] != "wget" {
		t.Errorf("/retriever shows %v", got)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Discover · /retriever") || !strings.Contains(out, "Discover › wget") {
		t.Errorf("the title or status bar doesn't say:\n%s", out)
	}
	keys(t, a, "/", "x", "x", "x", "enter")
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing matches /retrieverxxx") {
		t.Errorf("no match isn't explained:\n%s", out)
	}
}

func TestDiscoverEmptyAndFailed(t *testing.T) {
	a, cat := newDiscoverApp(t)
	cat.added = nil
	keys(t, a, "6")
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing new since brew last updated") {
		t.Errorf("an empty New isn't explained:\n%s", out)
	}

	a, cat = newDiscoverApp(t)
	cat.err = errors.New("brew search --desc : exit status 1")
	keys(t, a, "6")
	if out := stripANSI(a.render()); !strings.Contains(out, "couldn't load this view") || !strings.Contains(out, "exit status 1") {
		t.Errorf("the failure isn't shown:\n%s", out)
	}
	cat.err = nil
	keys(t, a, "r")
	if !a.discover.loaded || a.discover.err != nil {
		t.Errorf("r didn't retry: loaded %v, err %v", a.discover.loaded, a.discover.err)
	}
}

func TestDiscoverBeforeTheCatalogue(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	a.UseCatalog(newFakeCatalog(t))
	a.Update(press("6")) // the read hasn't come back
	if out := stripANSI(a.render()); !strings.Contains(out, "loading…") {
		t.Errorf("want a loading placeholder:\n%s", out)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/app_test.go:278:65: undefined: tabDiscover
    internal/ui/app_test.go:279:25: undefined: tabDiscover
    internal/ui/discover_test.go:40:4: a.UseCatalog undefined (type *App has no field or method UseCatalog)
```

- [ ] **Step 4: Edit `internal/ui/actions.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		return "s start · S stop · R restart · l log"
	case tabTaps:
		return "x untap"
	}
	return "1-5 tabs · r refresh · U update · c clean up"
}
```

   with:

```go
		return "s start · S stop · R restart · l log"
	case tabTaps:
		return "x untap"
	case tabDiscover:
		return "f/F view · / search"
	}
	return "1-6 tabs · r refresh · U update · c clean up"
}
```

- [ ] **Step 5: Edit `internal/ui/app.go`** (13 replacements; each old block appears exactly once)

1. Replace:

```go
	tabOutdated
	tabServices
	tabTaps
)

var tabNames = [...]string{"Home", "Installed", "Outdated", "Services", "Taps"}

// tabQueries is the query each tab's data comes from. Home shows doctor.
var tabQueries = map[int]query{
```

   with:

```go
	tabOutdated
	tabServices
	tabTaps
	tabDiscover
)

var tabNames = [...]string{"Home", "Installed", "Outdated", "Services", "Taps", "Discover"}

// tabQueries is the query each tab's data comes from. Home shows doctor.
var tabQueries = map[int]query{
```

2. Replace:

```go
// App is the root Bubble Tea model.
type App struct {
	src       Source
	start     Starter
	cachePath string
	st        theme.Styles
```

   with:

```go
// App is the root Bubble Tea model.
type App struct {
	src       Source
	catalog   Catalog // Discover's catalogue, or nil
	start     Starter
	cachePath string
	st        theme.Styles
```

3. Replace:

```go
	outdated  listTab
	services  listTab
	taps      listTab
}

// New builds the App. src answers queries and start runs brew commands.
```

   with:

```go
	outdated  listTab
	services  listTab
	taps      listTab
	discover  discoverTab
}

// New builds the App. src answers queries and start runs brew commands.
```

4. Replace:

```go
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
		taps:      newListTab(st, "Taps", tapCols, firstCell, "no taps installed"),
		jobs:      newListTab(st, "Jobs", jobCols, jobKey, "no jobs yet"),
	}
	// The history is read before the app starts, so the jobs view is never
	// waiting for it.
```

   with:

```go
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
		taps:      newListTab(st, "Taps", tapCols, firstCell, "no taps installed"),
		jobs:      newListTab(st, "Jobs", jobCols, jobKey, "no jobs yet"),
		discover:  newDiscoverTab(st),
	}
	// The history is read before the app starts, so the jobs view is never
	// waiting for it.
```

5. Replace:

```go
		ic = theme.Nerd
	}
	// Every part keeps its own copy of the styles.
	for _, st := range []*theme.Styles{&a.st, &a.installed.st, &a.outdated.st, &a.services.st, &a.taps.st, &a.jobs.st, &a.home.st, &a.drawer.st} {
		st.Icons = ic
	}
	a.sync()
```

   with:

```go
		ic = theme.Nerd
	}
	// Every part keeps its own copy of the styles.
	for _, st := range []*theme.Styles{&a.st, &a.installed.st, &a.outdated.st, &a.services.st, &a.taps.st, &a.jobs.st, &a.home.st, &a.drawer.st, &a.discover.st} {
		st.Icons = ic
	}
	a.sync()
```

6. Replace:

```go
}

// refresh re-runs every query: the JSON ones in parallel, then doctor and
// cleanup's dry-runs.
func (a *App) refresh() tea.Cmd {
	a.doctorAfter = true
	return a.request(jsonQueries...)
}

// request starts the given queries. A query that is already running is not
```

   with:

```go
}

// refresh re-runs every query: the JSON ones in parallel, then doctor and
// cleanup's dry-runs. Discover's catalogue is read again if it was read.
func (a *App) refresh() tea.Cmd {
	a.doctorAfter = true
	return tea.Batch(a.request(jsonQueries...), a.reloadCatalogue())
}

// request starts the given queries. A query that is already running is not
```

7. Replace:

```go
	a.outdated.setRows(outdatedRows(a.st.Icons, a.snap.Outdated))
	a.services.setRows(serviceRows(a.st, a.snap.Services))
	a.taps.setRows(tapRows(a.snap.Taps))
}

func (a *App) apply(m resultMsg) {
```

   with:

```go
	a.outdated.setRows(outdatedRows(a.st.Icons, a.snap.Outdated))
	a.services.setRows(serviceRows(a.st, a.snap.Services))
	a.taps.setRows(tapRows(a.snap.Taps))
	a.discover.setHave(a.snap.Packages)
}

func (a *App) apply(m resultMsg) {
```

8. Replace:

```go
		a.width, a.height = msg.Width, msg.Height
	case resultMsg:
		return a.handleResult(msg)
	case savedMsg:
		a.saveErr = msg.err
	case historySavedMsg:
```

   with:

```go
		a.width, a.height = msg.Width, msg.Height
	case resultMsg:
		return a.handleResult(msg)
	case catalogueMsg:
		a.gotCatalogue(msg)
	case savedMsg:
		a.saveErr = msg.err
	case historySavedMsg:
```

9. Replace:

```go
	for _, t := range []*listTab{&a.outdated, &a.services, &a.taps, &a.jobs} {
		t.resize(a.width, a.tabH)
	}
}

// handleResult applies one query result and decides what runs next: a
```

   with:

```go
	for _, t := range []*listTab{&a.outdated, &a.services, &a.taps, &a.jobs} {
		t.resize(a.width, a.tabH)
	}
	a.discover.resize(a.width, a.tabH)
}

// handleResult applies one query result and decides what runs next: a
```

10. Replace:

```go
			a.typeIntoJob()
		}
		return nil
	case "1", "2", "3", "4", "5":
		a.tab = int(k[0] - '1')
		return nil
	case "]":
		a.tab = (a.tab + 1) % len(tabNames)
		return nil
	case "[":
		a.tab = (a.tab + len(tabNames) - 1) % len(tabNames)
		return nil
	case "r":
		return a.refresh()
	}
```

   with:

```go
			a.typeIntoJob()
		}
		return nil
	case "1", "2", "3", "4", "5", "6":
		return a.showTab(int(k[0] - '1'))
	case "]":
		return a.showTab((a.tab + 1) % len(tabNames))
	case "[":
		return a.showTab((a.tab + len(tabNames) - 1) % len(tabNames))
	case "r":
		return a.refresh()
	}
```

11. Replace:

```go
		return a.services.update(msg)
	case tabTaps:
		return a.taps.update(msg)
	}
	return nil
}
```

   with:

```go
		return a.services.update(msg)
	case tabTaps:
		return a.taps.update(msg)
	case tabDiscover:
		return a.discover.update(msg)
	}
	return nil
}
```

12. Replace:

```go
			body = a.services.view(a.errs[qServices])
		case tabTaps:
			body = a.taps.view(a.errs[qTaps])
		}
	}
	drawer := a.drawerShown()
```

   with:

```go
			body = a.services.view(a.errs[qServices])
		case tabTaps:
			body = a.taps.view(a.errs[qTaps])
		case tabDiscover:
			body = a.discover.view()
		}
	}
	drawer := a.drawerShown()
```

13. Replace:

```go
		if p, ok := a.installed.selected(); ok {
			left += " › " + p.Name
		}
	}
	if n := a.marks(); n > 0 && !a.jobsOpen {
		left += fmt.Sprintf(" · %d marked", n)
```

   with:

```go
		if p, ok := a.installed.selected(); ok {
			left += " › " + p.Name
		}
	case a.tab == tabDiscover:
		if r, ok := a.discover.selected(); ok {
			left += " › " + r.Name
		}
	}
	if n := a.marks(); n > 0 && !a.jobsOpen {
		left += fmt.Sprintf(" · %d marked", n)
```

- [ ] **Step 6: Create `internal/ui/discover.go`**

```go
package ui

import (
	"context"
	"slices"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/theme"
)

// Catalog is brew's catalogue of everything it can install, for Discover.
// *brew.Client satisfies it; tests use a fake.
type Catalog interface {
	Catalogue(ctx context.Context) ([]brew.Entry, error)
	NewSinceUpdate(ctx context.Context) ([]brew.Entry, error)
}

var _ Catalog = (*brew.Client)(nil)

// UseCatalog sets where Discover's catalogue comes from.
func (a *App) UseCatalog(c Catalog) { a.catalog = c }

// catalogueMsg carries the catalogue and what brew's last update added.
type catalogueMsg struct {
	entries, added []brew.Entry
	err            error
}

func loadCatalogue(ctx context.Context, c Catalog) tea.Cmd {
	return func() tea.Msg {
		es, err := c.Catalogue(ctx)
		if err != nil {
			return catalogueMsg{err: err}
		}
		added, _ := c.NewSinceUpdate(ctx) // only a hint: without it, New is empty
		return catalogueMsg{entries: es, added: added}
	}
}

// showTab switches to tab t. Discover reads the catalogue the first time
// it opens.
func (a *App) showTab(t int) tea.Cmd {
	a.tab = t
	if t != tabDiscover || a.discover.loaded || a.discover.err != nil {
		return nil
	}
	return a.readCatalogue()
}

// reloadCatalogue reads the catalogue again if Discover has read it (or
// tried to): brew update can change it.
func (a *App) reloadCatalogue() tea.Cmd {
	if !a.discover.loaded && a.discover.err == nil {
		return nil
	}
	return a.readCatalogue()
}

func (a *App) readCatalogue() tea.Cmd {
	if a.catalog == nil || a.discover.loading {
		return nil
	}
	a.discover.loading = true
	return loadCatalogue(a.ctx, a.catalog)
}

func (a *App) gotCatalogue(m catalogueMsg) {
	a.discover.loading = false
	if m.err != nil && a.ctx.Err() != nil {
		return // quitting: the read was cancelled, not broken
	}
	a.discover.err = m.err
	if m.err == nil {
		a.discover.setCatalogue(m.entries, m.added)
	}
}

type discoverView int

const (
	discNew discoverView = iota
	discAll
)

// discoverViews are Discover's views, in the order f steps through them.
var discoverViews = [...]struct{ name, short, about string }{
	discNew: {"New", "New", "Added to Homebrew since brew last updated."},
	discAll: {"All", "All", "Everything brew can install. / searches names and descriptions."},
}

// discoverCols are each view's columns.
var discoverCols = [...][]col{
	discNew: {{"NAME", 24, 12}, {"TYPE", 9, 4}, {"DESCRIPTION", 0, 0}},
	discAll: {{"NAME", 24, 12}, {"TYPE", 9, 4}, {"DESCRIPTION", 0, 0}},
}

// entryKey identifies a package by kind and name: a formula and a cask
// can share a name.
func entryKey(name string, k brew.Kind) string { return k.String() + "/" + name }

// discRow is one package in a Discover view.
type discRow struct {
	brew.Entry
}

// discoverTab lists packages you could install.
type discoverTab struct {
	st      theme.Styles
	showing discoverView    // the view on screen
	cat     []brew.Entry    // everything brew can install
	added   []brew.Entry    // what brew's last update added, with descriptions
	have    map[string]bool // installed packages, by entryKey
	rows    []discRow       // the view's rows that pass the filter, in table order
	match   string          // the "/" filter on names and descriptions
	loaded  bool            // the catalogue has been read
	loading bool
	err     error // the last read failed
	table   table.Model
	w, h    int
}

func newDiscoverTab(st theme.Styles) discoverTab {
	return discoverTab{st: st, table: newTable(st, discoverCols[discNew])}
}

// setCatalogue takes a fresh catalogue. What the last update added comes
// as bare names: their descriptions are in the catalogue.
func (t *discoverTab) setCatalogue(entries, added []brew.Entry) {
	desc := make(map[string]string, len(entries))
	for _, e := range entries {
		desc[entryKey(e.Name, e.Kind)] = e.Desc
	}
	t.cat, t.added, t.loaded = entries, make([]brew.Entry, len(added)), true
	for i, e := range added {
		e.Desc = desc[entryKey(e.Name, e.Kind)]
		t.added[i] = e
	}
	t.apply()
}

// setHave notes which packages are installed.
func (t *discoverTab) setHave(pkgs []brew.Package) {
	t.have = make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		t.have[entryKey(p.Name, p.Kind)] = true
	}
	t.apply()
}

// entries is every package view v lists, before the filter.
func (t *discoverTab) entries(v discoverView) []brew.Entry {
	if v == discNew {
		return t.added
	}
	return t.cat
}

// source is every row view v lists, before the filter.
func (t *discoverTab) source(v discoverView) []discRow {
	es := t.entries(v)
	rows := make([]discRow, len(es))
	for i, e := range es {
		rows[i] = discRow{Entry: e}
	}
	return rows
}

func (t *discoverTab) apply() {
	before := t.selectedKey()
	t.rows = t.rows[:0]
	for _, r := range t.source(t.showing) {
		if matches(r.Name+" "+r.Desc, t.match) {
			t.rows = append(t.rows, r)
		}
	}
	rows := make([]table.Row, len(t.rows))
	for i, r := range t.rows {
		rows[i] = t.cells(r)
	}
	t.table.SetRows(nil) // the columns change with the view: old rows mustn't meet new columns
	t.size()
	setRows(&t.table, rows)
	if i := slices.IndexFunc(t.rows, func(r discRow) bool { return entryKey(r.Name, r.Kind) == before }); i >= 0 {
		t.table.SetCursor(i)
	}
}

// cells is r's row in the current view. Installed packages are marked
// and dimmed.
func (t *discoverTab) cells(r discRow) table.Row {
	name := r.Name
	have := t.have[entryKey(r.Name, r.Kind)]
	if have {
		name = "✓ " + name
	}
	row := table.Row{name, kindCell(t.st.Icons, r.Kind), r.Desc}
	if have {
		for i := range row {
			row[i] = theme.Tint(t.st.P.Overlay0, row[i])
		}
	}
	return row
}

// selected is the package under the cursor.
func (t *discoverTab) selected() (discRow, bool) {
	c := t.table.Cursor()
	if c < 0 || c >= len(t.rows) {
		return discRow{}, false
	}
	return t.rows[c], true
}

// selectedKey identifies the selected package, or is "" for none.
func (t *discoverTab) selectedKey() string {
	r, ok := t.selected()
	if !ok {
		return ""
	}
	return entryKey(r.Name, r.Kind)
}

func (t *discoverTab) matchText() string { return t.match }

func (t *discoverTab) setMatch(m string) {
	t.match = m
	t.apply()
}

func (t *discoverTab) update(msg tea.KeyPressMsg) tea.Cmd {
	if k := msg.String(); k == "f" || k == "F" {
		step := discoverView(1)
		if k == "F" {
			step = discoverView(len(discoverViews) - 1)
		}
		t.showing = (t.showing + step) % discoverView(len(discoverViews))
		t.apply()
		return nil
	}
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}

// size fits the table to the pane, under the bar of views.
func (t *discoverTab) size() {
	iw, ih := paneInner(t.w, t.h)
	sizeTable(&t.table, discoverCols[t.showing], iw, max(ih-viewBarHeight, 0))
}

// resize must run before key handling: the table ignores navigation
// until it has been sized.
func (t *discoverTab) resize(w, h int) {
	t.w, t.h = w, h
	t.size()
}

func (t *discoverTab) view() string {
	w, h := t.w, t.h
	if t.err != nil && !t.loaded {
		return pane(t.st, "Discover", errorBody(t.st, t.err), w, h, true)
	}
	iw, ih := paneInner(w, h)
	body := t.viewBar(iw) + "\n"
	ih -= viewBarHeight
	if t.err != nil {
		body += refreshBanner(t.st, t.err, iw) + "\n"
		ih -= bannerHeight
	}
	title := "Discover"
	if t.match != "" {
		title += " · /" + t.match
	}
	switch {
	case !t.loaded:
		body += t.st.Muted.Render("loading…") + "\n" + skeleton(t.st, iw, ih-1)
	case len(t.rows) == 0 && t.match != "":
		body += t.st.Muted.Render("nothing matches /" + t.match + " · / then esc clears it")
	case len(t.rows) == 0:
		body += t.st.Muted.Render(t.empty())
	default:
		t.table.SetHeight(max(ih, 0))
		body += t.table.View()
	}
	return pane(t.st, title, body, w, h, true)
}

// empty says why the current view has no rows.
func (t *discoverTab) empty() string {
	if t.showing == discNew {
		return "nothing new since brew last updated"
	}
	return "brew's catalogue is empty"
}

// viewBar is the bar of Discover's views with their counts.
func (t *discoverTab) viewBar(w int) string {
	segs := make([]viewSeg, len(discoverViews))
	for v, d := range discoverViews {
		segs[v] = viewSeg{name: d.name, short: d.short, n: len(t.entries(discoverView(v)))}
	}
	return viewBar(t.st, segs, int(t.showing), discoverViews[t.showing].about, w)
}
```

- [ ] **Step 7: Edit `internal/ui/filter.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		return &a.services
	case tabTaps:
		return &a.taps
	}
	return nil
}
```

   with:

```go
		return &a.services
	case tabTaps:
		return &a.taps
	case tabDiscover:
		return &a.discover
	}
	return nil
}
```

- [ ] **Step 8: Edit `internal/ui/help.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
		here = helpGroup{"Services", [][2]string{{"s", "start"}, {"S", "stop"}, {"R", "restart"}, {"l", "follow the log full screen"}}}
	case a.tab == tabTaps:
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	}
	groups := []helpGroup{here}
	switch {
```

   with:

```go
		here = helpGroup{"Services", [][2]string{{"s", "start"}, {"S", "stop"}, {"R", "restart"}, {"l", "follow the log full screen"}}}
	case a.tab == tabTaps:
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	case a.tab == tabDiscover:
		here = helpGroup{"Discover", [][2]string{{"f F", "next or previous view"}, {"/", "search names and descriptions"}}}
	}
	groups := []helpGroup{here}
	switch {
```

2. Replace:

```go
		}})
	}
	return append(groups, helpGroup{"Everywhere", [][2]string{
		{"1-5 [ ]", "switch tabs"}, {":", "run any brew command"}, {"r", "refresh"}, {"U", "brew update"},
		{"c", "clean up"}, {"o", "show or hide output"}, {"i", "type into the running job"},
		{"ctrl+c", "stop the running job"}, {"q", "quit"},
	}})
```

   with:

```go
		}})
	}
	return append(groups, helpGroup{"Everywhere", [][2]string{
		{"1-6 [ ]", "switch tabs"}, {":", "run any brew command"}, {"r", "refresh"}, {"U", "brew update"},
		{"c", "clean up"}, {"o", "show or hide output"}, {"i", "type into the running job"},
		{"ctrl+c", "stop the running job"}, {"q", "quit"},
	}})
```

- [ ] **Step 9: Edit `internal/ui/installed.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/state"
```

   with:

```go
package ui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
```

2. Replace:

```go
	filterDeps: {"Dependencies", "Deps", "Installed because something else needed them."},
}

// viewBarHeight is the bar of views plus the line about the active one.
const viewBarHeight = 2

var installedCols = []col{{"NAME", 0, 20}, {"VERSION", 16, 8}, {"TYPE", 9, 4}, {"STATUS", 10, 6}, {"SIZE", 8, -1}}

// installedTab lists installed packages with a details preview.
```

   with:

```go
	filterDeps: {"Dependencies", "Deps", "Installed because something else needed them."},
}

var installedCols = []col{{"NAME", 0, 20}, {"VERSION", 16, 8}, {"TYPE", 9, 4}, {"STATUS", 10, 6}, {"SIZE", 8, -1}}

// installedTab lists installed packages with a details preview.
```

3. Replace:

```go
	return sp.join(list, pane(t.st, previewTitle, body, sp.pvW, sp.pvH, t.pv.focus))
}

// viewBar lists every view with its count, the active one highlighted,
// and under it a line saying what the active view shows. Narrow panes get
// the short names.
func (t *installedTab) viewBar(w int) string {
	bar := func(short bool) string {
		var segs []string
		for f, v := range views {
			n := 0
			for _, p := range t.all {
				if t.shows(installedFilter(f), p) {
					n++
				}
			}
			name, style := v.name, t.st.View
			if short {
				name = v.short
			}
			if installedFilter(f) == t.filter {
				style = t.st.ViewActive
			}
			segs = append(segs, style.Render(fmt.Sprintf("%s %d", name, n)))
		}
		return strings.Join(segs, " ")
	}
	b := bar(false)
	if lipgloss.Width(b) > w {
		b = bar(true)
	}
	// The keys always show, shortened before the description is: on narrow
	// terminals the status bar has no room for hints.
	about, keys := views[t.filter].about, "f/F to switch"
	if len(about)+2+len(keys) > w {
		keys = "f/F"
		about = ansi.Truncate(about, max(w-2-len(keys), 0), "…")
	}
	gap := max(w-ansi.StringWidth(about)-len(keys), 1)
	return b + "\n" + t.st.Muted.Render(about+strings.Repeat(" ", gap)+keys)
}

func (t *installedTab) preview(p brew.Package) string {
```

   with:

```go
	return sp.join(list, pane(t.st, previewTitle, body, sp.pvW, sp.pvH, t.pv.focus))
}

// viewBar is the bar of Installed's views with their counts.
func (t *installedTab) viewBar(w int) string {
	segs := make([]viewSeg, len(views))
	for f, v := range views {
		segs[f] = viewSeg{name: v.name, short: v.short}
		for _, p := range t.all {
			if t.shows(installedFilter(f), p) {
				segs[f].n++
			}
		}
	}
	return viewBar(t.st, segs, int(t.filter), views[t.filter].about, w)
}

func (t *installedTab) preview(p brew.Package) string {
```

- [ ] **Step 10: Edit `internal/ui/links.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
			return a.services.update(home), true
		case tabTaps:
			return a.taps.update(home), true
		}
		return nil, true
	case "x":
```

   with:

```go
			return a.services.update(home), true
		case tabTaps:
			return a.taps.update(home), true
		case tabDiscover:
			return a.discover.update(home), true
		}
		return nil, true
	case "x":
```

2. Replace:

```go
			}
			return t.Name, url, true
		}
	}
	return "", "", false
}
```

   with:

```go
			}
			return t.Name, url, true
		}
	case tabDiscover:
		if r, ok := a.discover.selected(); ok {
			return r.Name, "", true
		}
	}
	return "", "", false
}
```

- [ ] **Step 11: Create `internal/ui/viewbar.go`**

```go
package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/theme"
)

// viewBarHeight is the bar of views plus the line about the active one.
const viewBarHeight = 2

// viewSeg is one view in a tab's bar of views: its name, a shorter name
// for narrow panes, and how many rows it has.
type viewSeg struct {
	name, short string
	n           int
}

// viewBar lists every view with its count, the active one highlighted,
// and under it about, a line saying what the active view shows. Narrow
// panes get the short names.
func viewBar(st theme.Styles, segs []viewSeg, active int, about string, w int) string {
	bar := func(short bool) string {
		parts := make([]string, len(segs))
		for i, v := range segs {
			name, style := v.name, st.View
			if short {
				name = v.short
			}
			if i == active {
				style = st.ViewActive
			}
			parts[i] = style.Render(fmt.Sprintf("%s %d", name, v.n))
		}
		return strings.Join(parts, " ")
	}
	b := bar(false)
	if lipgloss.Width(b) > w {
		b = bar(true)
	}
	// The keys always show, shortened before the description is: on narrow
	// terminals the status bar has no room for hints.
	keys := "f/F to switch"
	if len(about)+2+len(keys) > w {
		keys = "f/F"
		about = ansi.Truncate(about, max(w-2-len(keys), 0), "…")
	}
	gap := max(w-ansi.StringWidth(about)-len(keys), 1)
	return b + "\n" + st.Muted.Render(about+strings.Repeat(" ", gap)+keys)
}
```

- [ ] **Step 12: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 13: Commit**

```bash
git add -A
git commit -m "feat(ui): a Discover tab with the whole catalogue and what's new"
```

---

### Task 6: Popular tools and apps, Rising and Fonts from Homebrew's analytics

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/discover.go`
- Modify: `internal/ui/discover_test.go`
- Modify: `internal/ui/help.go`
- Create: `internal/ui/stats.go`
- Create: `internal/ui/stats_test.go`

**Interfaces:**
- **Consumes:** Task 3's `Rising`, `Compare`, `Counts`; Task 4's list names; Task 5's tab.
- **Produces:**
  - `ui.Analytics` (Cached, Fetch) and `(*App) UseAnalytics`.
  - `cachedStatsMsg`, `fetchedStatsMsg`; `wantStats`, `gotCachedStats`, `fetchStats(force bool)`, `fetchList(i)`, `gotStats`, `statsAge`.
  - `shortCount`, `growthCell`, `rankCell`.
  - The views `discTools`, `discApps`, `discRising`, `discNew`, `discFonts`, `discAll` and `(discoverView) ranked()`; `rankedCols`, `plainCols`; `popularRows = 500`, `risingRows = 50`.
  - `discRow` gains `rank`, `installs`, `growth`, `fresh`. `discoverTab` gains `byView`, `stats`, `statsAt`, `statsRead`, `fetching`, `fetchErr`, `triedOn`, `note`, `statsFrom(day)`, `rebuild()` and `rising(byKey)`.
  - `r` on Discover also runs `fetchStats(true)`.

Task 5's tests change where they reach New and All: Discover now opens on Popular tools.

- [ ] **Step 1: Edit `internal/ui/discover_test.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go

func TestDiscoverViews(t *testing.T) {
	a, _ := newDiscoverApp(t)
	keys(t, a, "6")
	out := stripANSI(a.render())
	for _, want := range []string{"6 Discover", "New 2", "All 17", "Added to Homebrew since brew last updated."} {
		if !strings.Contains(out, want) {
```

   with:

```go

func TestDiscoverViews(t *testing.T) {
	a, _ := newDiscoverApp(t)
	keys(t, a, "6", "f", "f", "f") // to New
	out := stripANSI(a.render())
	for _, want := range []string{"6 Discover", "New 2", "All 17", "Added to Homebrew since brew last updated."} {
		if !strings.Contains(out, want) {
```

2. Replace:

```go
		t.Errorf("codex's description is missing:\n%s", out)
	}

	keys(t, a, "f")
	rows := discoverRows(a)
	if len(rows) != 17 || rows[0] != "abcl" {
		t.Fatalf("All shows %v", rows)
```

   with:

```go
		t.Errorf("codex's description is missing:\n%s", out)
	}

	keys(t, a, "f", "f")
	rows := discoverRows(a)
	if len(rows) != 17 || rows[0] != "abcl" {
		t.Fatalf("All shows %v", rows)
```

3. Replace:

```go
		t.Errorf("the docker formula isn't installed, only the cask: %v", rows)
	}
	keys(t, a, "F")
	if a.discover.showing != discNew {
		t.Errorf("F goes back to New, got %d", a.discover.showing)
	}
}

func TestDiscoverFilterSearchesDescriptions(t *testing.T) {
	a, _ := newDiscoverApp(t)
	keys(t, a, "6", "f", "/", "r", "e", "t", "r", "i", "e", "v", "e", "r", "enter")
	if got := discoverRows(a); len(got) != 1 || got[0] != "wget" {
		t.Errorf("/retriever shows %v", got)
	}
```

   with:

```go
		t.Errorf("the docker formula isn't installed, only the cask: %v", rows)
	}
	keys(t, a, "F")
	if a.discover.showing != discFonts {
		t.Errorf("F goes back to Fonts, got %d", a.discover.showing)
	}
}

func TestDiscoverFilterSearchesDescriptions(t *testing.T) {
	a, _ := newDiscoverApp(t)
	keys(t, a, "6", "F", "/", "r", "e", "t", "r", "i", "e", "v", "e", "r", "enter")
	if got := discoverRows(a); len(got) != 1 || got[0] != "wget" {
		t.Errorf("/retriever shows %v", got)
	}
```

4. Replace:

```go
func TestDiscoverEmptyAndFailed(t *testing.T) {
	a, cat := newDiscoverApp(t)
	cat.added = nil
	keys(t, a, "6")
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing new since brew last updated") {
		t.Errorf("an empty New isn't explained:\n%s", out)
	}
```

   with:

```go
func TestDiscoverEmptyAndFailed(t *testing.T) {
	a, cat := newDiscoverApp(t)
	cat.added = nil
	keys(t, a, "6", "f", "f", "f")
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing new since brew last updated") {
		t.Errorf("an empty New isn't explained:\n%s", out)
	}
```

- [ ] **Step 2: Create `internal/ui/stats_test.go`**

```go
package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"brewery/internal/analytics"
)

// fakeStats serves the analytics fixtures. cached are the copies kept from
// earlier downloads, fetched at cachedAt; err makes downloads fail.
type fakeStats struct {
	fresh    map[string]analytics.List
	cached   map[string]analytics.List
	cachedAt time.Time
	err      error
	fetched  []string // lists downloaded, in order
}

func statsFixture(t *testing.T, list string) analytics.List {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "analytics", "testdata", strings.ReplaceAll(list, "/", "-")+".json"))
	if err != nil {
		t.Fatal(err)
	}
	l, err := analytics.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func newFakeStats(t *testing.T) *fakeStats {
	f := &fakeStats{fresh: map[string]analytics.List{}, cached: map[string]analytics.List{}}
	for _, l := range analytics.Lists {
		f.fresh[l] = statsFixture(t, l)
	}
	return f
}

func (f *fakeStats) Cached(list string) (analytics.List, time.Time, error) {
	if l, ok := f.cached[list]; ok {
		return l, f.cachedAt, nil
	}
	return analytics.List{}, time.Time{}, os.ErrNotExist
}

func (f *fakeStats) Fetch(_ context.Context, list string) (analytics.List, error) {
	f.fetched = append(f.fetched, list)
	if f.err != nil {
		return analytics.List{}, f.err
	}
	return f.fresh[list], nil
}

// newStatsApp is a Discover app with analytics to read, still on Home.
func newStatsApp(t *testing.T) (*App, *fakeStats) {
	t.Helper()
	a, _ := newDiscoverApp(t)
	st := newFakeStats(t)
	a.UseAnalytics(st)
	return a, st
}

// discoverCells are the Discover table's rows, as shown.
func discoverCells(a *App) []string {
	var rows []string
	for _, r := range a.discover.table.Rows() {
		cells := make([]string, len(r))
		for i, c := range r {
			cells[i] = stripANSI(c)
		}
		rows = append(rows, strings.Join(cells, "|"))
	}
	return rows
}

func TestDiscoverPopular(t *testing.T) {
	a, st := newStatsApp(t)
	keys(t, a, "6")
	if !slices.Equal(st.fetched, analytics.Lists) {
		t.Errorf("fetched %v, want every list in order", st.fetched)
	}
	// Packages brew doesn't know (from other taps) are left out, and the
	// ranks count what's shown.
	want := []string{
		"1|gh|GitHub command-line tool|217k",
		"2|✓ git|Distributed revision control system|105k",
		"3|wget|Internet file retriever|54k",
		"4|ripgrep|Search tool like grep and The Silver Searcher|30k",
		"5|✓ fd|Simple, fast and user-friendly alternative to find|16k",
		"6|python@3.15|Interpreted, interactive, object-oriented programming language|5.6k",
	}
	if got := discoverCells(a); !slices.Equal(got, want) {
		t.Errorf("Popular tools:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	out := stripANSI(a.render())
	for _, s := range []string{"Popular tools 6", "Popular apps 4", "Rising 4", "New 2", "Fonts 2", "All 17", "The most-installed tools of the last 30 days."} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q in\n%s", s, out)
		}
	}
	if strings.Contains(out, "analytics from") {
		t.Errorf("today's analytics need no date:\n%s", out)
	}

	keys(t, a, "f") // apps, without fonts
	want = []string{
		"1|codex|OpenAI's coding agent that runs in your terminal|135k",
		"2|✓ docker-desktop|App to build and share containerised applications and microservices|32k",
		"3|firefox|Web browser|10k",
		"4|0-ad|Real-time strategy game|249",
	}
	if got := discoverCells(a); !slices.Equal(got, want) {
		t.Errorf("Popular apps:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDiscoverRising(t *testing.T) {
	a, _ := newStatsApp(t)
	keys(t, a, "6", "f", "f")
	want := []string{
		"1|python@3.15|formula|new|Interpreted, interactive, object-oriented programming language",
		"2|wget|formula|×4.0|Internet file retriever",
		"3|✓ git|formula|×1.4|Distributed revision control system",
		"4|codex|cask|×1.3|OpenAI's coding agent that runs in your terminal",
	}
	if got := discoverCells(a); !slices.Equal(got, want) {
		t.Errorf("Rising:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDiscoverFonts(t *testing.T) {
	a, st := newStatsApp(t)
	// font-fira-code has no installs: it follows the ranked fonts, by name.
	apps := st.fresh[analytics.Apps30]
	apps.Items = slices.DeleteFunc(slices.Clone(apps.Items), func(it analytics.Item) bool { return it.Name == "font-fira-code" })
	st.fresh[analytics.Apps30] = apps
	keys(t, a, "6", "f", "f", "f", "f")
	want := []string{"1|font-jetbrains-mono|JetBrains Mono|2.8k", "|font-fira-code|Fira Code|"}
	if got := discoverCells(a); !slices.Equal(got, want) {
		t.Errorf("Fonts:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDiscoverUsesTheCacheOffline(t *testing.T) {
	a, st := newStatsApp(t)
	st.cached[analytics.Tools30] = st.fresh[analytics.Tools30]
	st.cachedAt = testNow.Add(-24 * time.Hour)
	st.err = errors.New("dial tcp: no route to host")
	keys(t, a, "6")
	if len(st.fetched) != 1 {
		t.Errorf("fetched %v: a failure stops the rest", st.fetched)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "gh") || !strings.Contains(out, "analytics from Oct 12") {
		t.Errorf("want yesterday's copy, dated:\n%s", out)
	}
	keys(t, a, "f") // nothing kept for apps
	if out := stripANSI(a.render()); !strings.Contains(out, "couldn't reach formulae.brew.sh · r retries") {
		t.Errorf("the failure isn't explained:\n%s", out)
	}
	// New and All don't need the network.
	keys(t, a, "f", "f")
	if got := discoverRows(a); len(got) != 2 {
		t.Errorf("New shows %v", got)
	}

	st.err = nil
	keys(t, a, "F", "F", "r")
	if out := stripANSI(a.render()); !strings.Contains(out, "codex") || strings.Contains(out, "analytics from") {
		t.Errorf("after r, want today's apps:\n%s", out)
	}
}

func TestDiscoverFetchesOnceADay(t *testing.T) {
	a, st := newStatsApp(t)
	for _, l := range analytics.Lists {
		st.cached[l] = st.fresh[l]
	}
	st.cachedAt = testNow.Add(-time.Hour) // already today
	keys(t, a, "6", "1", "6")
	if len(st.fetched) != 0 {
		t.Errorf("fetched %v, though the copies are from today", st.fetched)
	}
	keys(t, a, "r")
	if len(st.fetched) != 5 {
		t.Errorf("r fetched %v, want every list", st.fetched)
	}

	a, st = newStatsApp(t)
	st.err = errors.New("offline")
	keys(t, a, "6", "1", "6")
	if len(st.fetched) != 1 {
		t.Errorf("fetched %v: a failed fetch isn't retried by opening Discover again the same day", st.fetched)
	}
}

func TestDiscoverWhileFetching(t *testing.T) {
	a, _ := newStatsApp(t)
	a.Update(press("6")) // nothing comes back yet
	if a.fetchStats(true) == nil {
		t.Fatal("the first fetch didn't start")
	}
	if a.fetchStats(true) != nil {
		t.Error("a second fetch started while one is running")
	}
	if !strings.Contains(stripANSI(a.render()), "fetching analytics") {
		t.Errorf("want a note that analytics are coming:\n%s", stripANSI(a.render()))
	}
}

func TestShortCount(t *testing.T) {
	for n, want := range map[int]string{0: "", 249: "249", 2810: "2.8k", 54269: "54k", 217104: "217k", 2994048: "3.0M"} {
		if got := shortCount(n); got != want {
			t.Errorf("shortCount(%d) = %q, want %q", n, got, want)
		}
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/discover_test.go:104:27: undefined: discFonts
    internal/ui/stats_test.go:67:4: a.UseAnalytics undefined (type *App has no field or method UseAnalytics)
    internal/ui/stats_test.go:208:7: a.fetchStats undefined (type *App has no field or method fetchStats)
```

- [ ] **Step 4: Edit `internal/ui/actions.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	case tabTaps:
		return "x untap"
	case tabDiscover:
		return "f/F view · / search"
	}
	return "1-6 tabs · r refresh · U update · c clean up"
}
```

   with:

```go
	case tabTaps:
		return "x untap"
	case tabDiscover:
		return "f/F view · / search · r refetch"
	}
	return "1-6 tabs · r refresh · U update · c clean up"
}
```

- [ ] **Step 5: Edit `internal/ui/app.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go
// App is the root Bubble Tea model.
type App struct {
	src       Source
	catalog   Catalog // Discover's catalogue, or nil
	start     Starter
	cachePath string
	st        theme.Styles
```

   with:

```go
// App is the root Bubble Tea model.
type App struct {
	src       Source
	catalog   Catalog   // Discover's catalogue, or nil
	analytics Analytics // Discover's install counts, or nil
	start     Starter
	cachePath string
	st        theme.Styles
```

2. Replace:

```go
	a.jobs.matchCol = 1 // jobs are found by their command
	a.services.preview = a.servicePreview
	a.outdated.note = a.dataAge
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
		for _, q := range []query{qInstalled, qOutdated, qServices, qTaps} {
```

   with:

```go
	a.jobs.matchCol = 1 // jobs are found by their command
	a.services.preview = a.servicePreview
	a.outdated.note = a.dataAge
	a.discover.note = a.statsAge
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
		for _, q := range []query{qInstalled, qOutdated, qServices, qTaps} {
```

3. Replace:

```go
		return a.handleResult(msg)
	case catalogueMsg:
		a.gotCatalogue(msg)
	case savedMsg:
		a.saveErr = msg.err
	case historySavedMsg:
```

   with:

```go
		return a.handleResult(msg)
	case catalogueMsg:
		a.gotCatalogue(msg)
	case cachedStatsMsg:
		return a.gotCachedStats(msg)
	case fetchedStatsMsg:
		return a.gotStats(msg)
	case savedMsg:
		a.saveErr = msg.err
	case historySavedMsg:
```

4. Replace:

```go
	case "[":
		return a.showTab((a.tab + len(tabNames) - 1) % len(tabNames))
	case "r":
		return a.refresh()
	}
	if cmd, ok := a.actionKey(k); ok {
```

   with:

```go
	case "[":
		return a.showTab((a.tab + len(tabNames) - 1) % len(tabNames))
	case "r":
		if a.tab == tabDiscover {
			return tea.Batch(a.refresh(), a.fetchStats(true))
		}
		return a.refresh()
	}
	if cmd, ok := a.actionKey(k); ok {
```

- [ ] **Step 6: Replace the whole of `internal/ui/discover.go` with**

```go
package ui

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/analytics"
	"brewery/internal/brew"
	"brewery/internal/theme"
)

// Catalog is brew's catalogue of everything it can install, for Discover.
// *brew.Client satisfies it; tests use a fake.
type Catalog interface {
	Catalogue(ctx context.Context) ([]brew.Entry, error)
	NewSinceUpdate(ctx context.Context) ([]brew.Entry, error)
}

var _ Catalog = (*brew.Client)(nil)

// UseCatalog sets where Discover's catalogue comes from.
func (a *App) UseCatalog(c Catalog) { a.catalog = c }

// catalogueMsg carries the catalogue and what brew's last update added.
type catalogueMsg struct {
	entries, added []brew.Entry
	err            error
}

func loadCatalogue(ctx context.Context, c Catalog) tea.Cmd {
	return func() tea.Msg {
		es, err := c.Catalogue(ctx)
		if err != nil {
			return catalogueMsg{err: err}
		}
		added, _ := c.NewSinceUpdate(ctx) // only a hint: without it, New is empty
		return catalogueMsg{entries: es, added: added}
	}
}

// showTab switches to tab t. Discover reads the catalogue the first time
// it opens, and the analytics as wantStats says.
func (a *App) showTab(t int) tea.Cmd {
	a.tab = t
	if t != tabDiscover {
		return nil
	}
	var cat tea.Cmd
	if !a.discover.loaded && a.discover.err == nil {
		cat = a.readCatalogue()
	}
	return tea.Batch(cat, a.wantStats())
}

// reloadCatalogue reads the catalogue again if Discover has read it (or
// tried to): brew update can change it.
func (a *App) reloadCatalogue() tea.Cmd {
	if !a.discover.loaded && a.discover.err == nil {
		return nil
	}
	return a.readCatalogue()
}

func (a *App) readCatalogue() tea.Cmd {
	if a.catalog == nil || a.discover.loading {
		return nil
	}
	a.discover.loading = true
	return loadCatalogue(a.ctx, a.catalog)
}

func (a *App) gotCatalogue(m catalogueMsg) {
	a.discover.loading = false
	if m.err != nil && a.ctx.Err() != nil {
		return // quitting: the read was cancelled, not broken
	}
	a.discover.err = m.err
	if m.err == nil {
		a.discover.setCatalogue(m.entries, m.added)
	}
}

type discoverView int

const (
	discTools discoverView = iota
	discApps
	discRising
	discNew
	discFonts
	discAll
)

// discoverViews are Discover's views, in the order f steps through them.
var discoverViews = [...]struct{ name, short, about string }{
	discTools:  {"Popular tools", "Tools", "The most-installed tools of the last 30 days."},
	discApps:   {"Popular apps", "Apps", "The most-installed apps of the last 30 days."},
	discRising: {"Rising", "Rising", "Growing fastest: the last 30 days against the 60 before."},
	discNew:    {"New", "New", "Added to Homebrew since brew last updated."},
	discFonts:  {"Fonts", "Fonts", "Fonts, the most installed first."},
	discAll:    {"All", "All", "Everything brew can install. / searches names and descriptions."},
}

// ranked reports whether view v is ordered by the analytics.
func (v discoverView) ranked() bool { return v != discNew && v != discAll }

// Descriptions keep 20 columns while names and the rest give way.
var (
	rankedCols = []col{{"#", 3, 0}, {"NAME", 20, 12}, {"DESCRIPTION", 0, 20}, {"INSTALLS", 8, 0}}
	plainCols  = []col{{"NAME", 20, 12}, {"TYPE", 9, 4}, {"DESCRIPTION", 0, 20}}
)

// discoverCols are each view's columns.
var discoverCols = [...][]col{
	discTools:  rankedCols,
	discApps:   rankedCols,
	discRising: {{"#", 3, 0}, {"NAME", 20, 12}, {"TYPE", 9, 4}, {"GROWTH", 6, 0}, {"DESCRIPTION", 0, 20}},
	discNew:    plainCols,
	discFonts:  rankedCols,
	discAll:    plainCols,
}

// How many rows the Popular views and Rising show.
const (
	popularRows = 500
	risingRows  = 50
)

// entryKey identifies a package by kind and name: a formula and a cask
// can share a name.
func entryKey(name string, k brew.Kind) string { return k.String() + "/" + name }

// discRow is one package in a Discover view.
type discRow struct {
	brew.Entry
	rank     int     // its place in a ranked view, from 1; 0 for none
	installs int     // in the last 30 days; 0 if not known
	growth   float64 // Rising: how many times its installs grew
	fresh    bool    // Rising: (almost) no installs before
}

// discoverTab lists packages you could install.
type discoverTab struct {
	st      theme.Styles
	showing discoverView    // the view on screen
	cat     []brew.Entry    // everything brew can install
	added   []brew.Entry    // what brew's last update added, with descriptions
	have    map[string]bool // installed packages, by entryKey
	byView  [len(discoverViews)][]discRow
	rows    []discRow // the view's rows that pass the filter, in table order
	match   string    // the "/" filter on names and descriptions
	loaded  bool      // the catalogue has been read
	loading bool
	err     error // the last read failed
	table   table.Model
	w, h    int

	stats     map[string]analytics.List // by list name
	statsAt   map[string]time.Time      // when each list was fetched
	statsRead bool                      // the kept copies have been read
	fetching  bool
	fetchErr  error  // the last fetch failed
	triedOn   string // the day a fetch last started, as 2006-01-02
	note      func() string
}

func newDiscoverTab(st theme.Styles) discoverTab {
	return discoverTab{
		st: st, table: newTable(st, discoverCols[discTools]),
		stats: map[string]analytics.List{}, statsAt: map[string]time.Time{},
	}
}

// statsFrom reports whether every list was fetched on day.
func (t *discoverTab) statsFrom(day string) bool {
	for _, l := range analytics.Lists {
		if at, ok := t.statsAt[l]; !ok || at.Format(time.DateOnly) != day {
			return false
		}
	}
	return true
}

// setCatalogue takes a fresh catalogue. What the last update added comes
// as bare names: their descriptions are in the catalogue.
func (t *discoverTab) setCatalogue(entries, added []brew.Entry) {
	desc := make(map[string]string, len(entries))
	for _, e := range entries {
		desc[entryKey(e.Name, e.Kind)] = e.Desc
	}
	t.cat, t.added, t.loaded = entries, make([]brew.Entry, len(added)), true
	for i, e := range added {
		e.Desc = desc[entryKey(e.Name, e.Kind)]
		t.added[i] = e
	}
	t.rebuild()
}

// rebuild works out every view's rows from the catalogue and the
// analytics. Packages the catalogue doesn't have, such as ones from taps
// you haven't tapped, are left out.
func (t *discoverTab) rebuild() {
	byKey := make(map[string]brew.Entry, len(t.cat))
	all := make([]discRow, len(t.cat))
	for i, e := range t.cat {
		byKey[entryKey(e.Name, e.Kind)] = e
		all[i] = discRow{Entry: e}
	}
	// top is list's packages of kind that keep accepts, at most n (all if
	// n < 0), ranked by installs.
	top := func(list string, kind brew.Kind, keep func(name string) bool, n int) []discRow {
		var rows []discRow
		for _, it := range t.stats[list].Items {
			if e, ok := byKey[entryKey(it.Name, kind)]; ok && keep(it.Name) && len(rows) != n {
				rows = append(rows, discRow{Entry: e, rank: len(rows) + 1, installs: it.Count})
			}
		}
		return rows
	}
	font := func(name string) bool { return strings.HasPrefix(name, "font-") }
	t.byView[discTools] = top(analytics.Tools30, brew.KindFormula, func(string) bool { return true }, popularRows)
	t.byView[discApps] = top(analytics.Apps30, brew.KindCask, func(n string) bool { return !font(n) }, popularRows)
	fonts := top(analytics.Apps30, brew.KindCask, font, -1)
	ranked := make(map[string]bool, len(fonts))
	for _, r := range fonts {
		ranked[r.Name] = true
	}
	for _, e := range t.cat {
		if e.Kind == brew.KindCask && font(e.Name) && !ranked[e.Name] {
			fonts = append(fonts, discRow{Entry: e}) // no installs: after the ranked ones, by name
		}
	}
	t.byView[discFonts] = fonts
	t.byView[discRising] = t.rising(byKey)
	t.byView[discNew] = make([]discRow, len(t.added))
	for i, e := range t.added {
		t.byView[discNew][i] = discRow{Entry: e}
	}
	t.byView[discAll] = all
	t.apply()
}

// rising is the top risers among formulae and casks together. It needs
// both the 30-day and the 90-day lists.
func (t *discoverTab) rising(byKey map[string]brew.Entry) []discRow {
	type riser struct {
		analytics.Riser
		e brew.Entry
	}
	var rs []riser
	for _, src := range []struct {
		d30, d90 string
		kind     brew.Kind
	}{{analytics.Tools30, analytics.Tools90, brew.KindFormula}, {analytics.Apps30, analytics.Apps90, brew.KindCask}} {
		d30, ok30 := t.stats[src.d30]
		d90, ok90 := t.stats[src.d90]
		if !ok30 || !ok90 {
			continue
		}
		for _, r := range analytics.Rising(d30, d90) {
			if e, ok := byKey[entryKey(r.Name, src.kind)]; ok {
				rs = append(rs, riser{r, e})
			}
		}
	}
	slices.SortStableFunc(rs, func(a, b riser) int { return analytics.Compare(a.Riser, b.Riser) })
	rows := make([]discRow, min(len(rs), risingRows))
	for i := range rows {
		r := rs[i]
		rows[i] = discRow{Entry: r.e, rank: i + 1, installs: r.Count, growth: r.Growth, fresh: r.New}
	}
	return rows
}

// setHave notes which packages are installed.
func (t *discoverTab) setHave(pkgs []brew.Package) {
	t.have = make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		t.have[entryKey(p.Name, p.Kind)] = true
	}
	t.apply()
}

func (t *discoverTab) apply() {
	before := t.selectedKey()
	t.rows = t.rows[:0]
	for _, r := range t.byView[t.showing] {
		if matches(r.Name+" "+r.Desc, t.match) {
			t.rows = append(t.rows, r)
		}
	}
	rows := make([]table.Row, len(t.rows))
	for i, r := range t.rows {
		rows[i] = t.cells(r)
	}
	t.table.SetRows(nil) // the columns change with the view: old rows mustn't meet new columns
	t.size()
	setRows(&t.table, rows)
	if i := slices.IndexFunc(t.rows, func(r discRow) bool { return entryKey(r.Name, r.Kind) == before }); i >= 0 {
		t.table.SetCursor(i)
	}
}

// cells is r's row in the current view. Installed packages are marked
// and dimmed.
func (t *discoverTab) cells(r discRow) table.Row {
	name := r.Name
	have := t.have[entryKey(r.Name, r.Kind)]
	if have {
		name = "✓ " + name
	}
	var row table.Row
	switch t.showing {
	case discRising:
		row = table.Row{rankCell(r.rank), name, kindCell(t.st.Icons, r.Kind), growthCell(r), r.Desc}
	case discNew, discAll:
		row = table.Row{name, kindCell(t.st.Icons, r.Kind), r.Desc}
	default:
		row = table.Row{rankCell(r.rank), name, r.Desc, shortCount(r.installs)}
	}
	if have {
		for i := range row {
			row[i] = theme.Tint(t.st.P.Overlay0, row[i])
		}
	}
	return row
}

// selected is the package under the cursor.
func (t *discoverTab) selected() (discRow, bool) {
	c := t.table.Cursor()
	if c < 0 || c >= len(t.rows) {
		return discRow{}, false
	}
	return t.rows[c], true
}

// selectedKey identifies the selected package, or is "" for none.
func (t *discoverTab) selectedKey() string {
	r, ok := t.selected()
	if !ok {
		return ""
	}
	return entryKey(r.Name, r.Kind)
}

func (t *discoverTab) matchText() string { return t.match }

func (t *discoverTab) setMatch(m string) {
	t.match = m
	t.apply()
}

func (t *discoverTab) update(msg tea.KeyPressMsg) tea.Cmd {
	if k := msg.String(); k == "f" || k == "F" {
		step := discoverView(1)
		if k == "F" {
			step = discoverView(len(discoverViews) - 1)
		}
		t.showing = (t.showing + step) % discoverView(len(discoverViews))
		t.apply()
		return nil
	}
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}

// size fits the table to the pane, under the bar of views.
func (t *discoverTab) size() {
	iw, ih := paneInner(t.w, t.h)
	sizeTable(&t.table, discoverCols[t.showing], iw, max(ih-viewBarHeight, 0))
}

// resize must run before key handling: the table ignores navigation
// until it has been sized.
func (t *discoverTab) resize(w, h int) {
	t.w, t.h = w, h
	t.size()
}

func (t *discoverTab) view() string {
	w, h := t.w, t.h
	if t.err != nil && !t.loaded {
		return pane(t.st, "Discover", errorBody(t.st, t.err), w, h, true)
	}
	iw, ih := paneInner(w, h)
	body := t.viewBar(iw) + "\n"
	ih -= viewBarHeight
	if t.err != nil {
		body += refreshBanner(t.st, t.err, iw) + "\n"
		ih -= bannerHeight
	}
	title := "Discover"
	if t.match != "" {
		title += " · /" + t.match
	}
	if t.note != nil && t.showing.ranked() {
		if n := t.note(); n != "" {
			title += " · " + n
		}
	}
	switch {
	case !t.loaded:
		body += t.st.Muted.Render("loading…") + "\n" + skeleton(t.st, iw, ih-1)
	case len(t.rows) == 0 && t.match != "":
		body += t.st.Muted.Render("nothing matches /" + t.match + " · / then esc clears it")
	case len(t.rows) == 0:
		body += t.st.Muted.Render(t.empty())
	default:
		t.table.SetHeight(max(ih, 0))
		body += t.table.View()
	}
	return pane(t.st, title, body, w, h, true)
}

// empty says why the current view has no rows.
func (t *discoverTab) empty() string {
	switch {
	case t.showing == discNew:
		return "nothing new since brew last updated"
	case t.showing == discAll:
		return "brew's catalogue is empty"
	case t.fetching:
		return "fetching analytics from formulae.brew.sh…"
	case t.fetchErr != nil:
		return "couldn't reach formulae.brew.sh · r retries"
	case len(t.stats) == 0:
		return "no analytics yet · r fetches them"
	}
	return "nothing here yet"
}

// rankCell is a row's place in a ranked view, blank for none.
func rankCell(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// viewBar is the bar of Discover's views with their counts.
func (t *discoverTab) viewBar(w int) string {
	segs := make([]viewSeg, len(discoverViews))
	for v, d := range discoverViews {
		segs[v] = viewSeg{name: d.name, short: d.short, n: len(t.byView[v])}
	}
	return viewBar(t.st, segs, int(t.showing), discoverViews[t.showing].about, w)
}
```

- [ ] **Step 7: Edit `internal/ui/help.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	case a.tab == tabTaps:
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	case a.tab == tabDiscover:
		here = helpGroup{"Discover", [][2]string{{"f F", "next or previous view"}, {"/", "search names and descriptions"}}}
	}
	groups := []helpGroup{here}
	switch {
```

   with:

```go
	case a.tab == tabTaps:
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	case a.tab == tabDiscover:
		here = helpGroup{"Discover", [][2]string{
			{"f F", "next or previous view"}, {"/", "search names and descriptions"}, {"r", "fetch the analytics again"},
		}}
	}
	groups := []helpGroup{here}
	switch {
```

- [ ] **Step 8: Create `internal/ui/stats.go`**

```go
package ui

import (
	"context"
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/analytics"
)

// Analytics are Homebrew's install counts, downloaded and kept between
// runs. analytics.Store satisfies it; tests use a fake.
type Analytics interface {
	Cached(list string) (analytics.List, time.Time, error)
	Fetch(ctx context.Context, list string) (analytics.List, error)
}

var _ Analytics = analytics.Store{}

// UseAnalytics sets where Discover's install counts come from.
func (a *App) UseAnalytics(an Analytics) { a.analytics = an }

// Messages from reading the kept analytics and downloading them.
type (
	cachedStatsMsg struct {
		lists map[string]analytics.List
		at    map[string]time.Time
	}
	fetchedStatsMsg struct {
		i   int // the list's index in analytics.Lists
		l   analytics.List
		err error
	}
)

// wantStats reads the kept analytics the first time Discover opens; after
// that it fetches them if they aren't from today.
func (a *App) wantStats() tea.Cmd {
	if a.analytics == nil {
		return nil
	}
	if !a.discover.statsRead {
		a.discover.statsRead = true
		an := a.analytics
		return func() tea.Msg {
			m := cachedStatsMsg{lists: map[string]analytics.List{}, at: map[string]time.Time{}}
			for _, name := range analytics.Lists {
				if l, at, err := an.Cached(name); err == nil {
					m.lists[name], m.at[name] = l, at
				}
			}
			return m
		}
	}
	return a.fetchStats(false)
}

func (a *App) gotCachedStats(m cachedStatsMsg) tea.Cmd {
	d := &a.discover
	for name, l := range m.lists {
		if _, ok := d.stats[name]; !ok { // a download that finished first is newer
			d.stats[name], d.statsAt[name] = l, m.at[name]
		}
	}
	d.rebuild()
	return a.fetchStats(false)
}

// fetchStats downloads every list, one after another, unless the copies
// are all from today or a fetch already started today. force (r) fetches
// anyway. Only one fetch runs at a time.
func (a *App) fetchStats(force bool) tea.Cmd {
	d := &a.discover
	today := a.now().Format(time.DateOnly)
	if a.analytics == nil || d.fetching || (!force && (d.triedOn == today || d.statsFrom(today))) {
		return nil
	}
	d.fetching, d.triedOn, d.fetchErr = true, today, nil
	return a.fetchList(0)
}

func (a *App) fetchList(i int) tea.Cmd {
	an, ctx := a.analytics, a.ctx
	return func() tea.Msg {
		l, err := an.Fetch(ctx, analytics.Lists[i])
		return fetchedStatsMsg{i: i, l: l, err: err}
	}
}

// gotStats takes one downloaded list and fetches the next. A failure
// stops the fetch: offline, every list would fail the same way.
func (a *App) gotStats(m fetchedStatsMsg) tea.Cmd {
	d := &a.discover
	if m.err != nil {
		d.fetching = false
		if a.ctx.Err() == nil { // quitting cancels a download: that's no failure
			d.fetchErr = m.err
		}
		return nil
	}
	name := analytics.Lists[m.i]
	d.stats[name], d.statsAt[name] = m.l, a.now()
	d.rebuild()
	if m.i+1 < len(analytics.Lists) {
		return a.fetchList(m.i + 1)
	}
	d.fetching = false
	return nil
}

// statsAge notes a fetch in progress, or how old the analytics are when
// they aren't from today.
func (a *App) statsAge() string {
	d := &a.discover
	if d.fetching {
		return "fetching analytics…"
	}
	var oldest time.Time
	for _, at := range d.statsAt {
		if oldest.IsZero() || at.Before(oldest) {
			oldest = at
		}
	}
	if oldest.IsZero() || oldest.Format(time.DateOnly) == a.now().Format(time.DateOnly) {
		return ""
	}
	return "analytics from " + oldest.Format("Jan 2")
}

// shortCount writes an install count in a few characters: 249, 2.8k,
// 217k, 1.2M. Zero, not known, is blank.
func shortCount(n int) string {
	switch {
	case n == 0:
		return ""
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return strconv.Itoa(n/1000) + "k"
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return strconv.Itoa(n)
}

// growthCell is a riser's GROWTH: "new", or how many times its installs
// grew, "×4.1".
func growthCell(r discRow) string {
	switch {
	case r.fresh:
		return "new"
	case r.growth < 10:
		return fmt.Sprintf("×%.1f", r.growth)
	}
	return fmt.Sprintf("×%.0f", r.growth)
}
```

- [ ] **Step 9: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat(ui): Popular tools and apps, Rising and Fonts from Homebrew's analytics"
```

---

### Task 7: Discover's preview asks brew about the package the cursor rests on

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/app.go`
- Create: `internal/ui/details.go`
- Create: `internal/ui/details_test.go`
- Modify: `internal/ui/discover.go`
- Modify: `internal/ui/discover_test.go`
- Modify: `internal/ui/help.go`
- Modify: `internal/ui/links.go`

**Interfaces:**
- **Consumes:** Task 2's `Details`; Task 6's counts; `scroller`, `splitFor`, `kv`.
- **Produces:**
  - `Catalog` gains `Details`.
  - `detailsDelay` and `App.detailsDelay` (tests set it to 0).
  - `detailsDueMsg`, `detailsMsg`; `wantDetails` (run after every Update on Discover), `detailsDue`, `gotDetails`.
  - `discoverTab` gains `pv`, `details`, `detailsErr`, `waiting`, `asking`, `counts`, `missing(key)`, `preview(r)` and `followSelection`. `withCommas`.
  - `gx` opens the homepage brew gave; `esc` leaves the preview.

`newDiscoverApp` sets `detailsDelay` to 0.

- [ ] **Step 1: Create `internal/ui/details_test.go`**

```go
package ui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
)

var ripgrep = brew.Details{
	Package: brew.Package{
		Name: "ripgrep", FullName: "ripgrep", Kind: brew.KindFormula, Tap: "homebrew/core",
		Desc: "Search tool like grep and The Silver Searcher", Homepage: "https://github.com/BurntSushi/ripgrep",
		License: "Unlicense", Latest: "15.2.0",
	},
	Needs: []string{"pcre2", "oniguruma"},
}

// selectDiscover moves the Discover cursor to name, from the top.
func selectDiscover(t *testing.T, a *App, name string) {
	t.Helper()
	keys(t, a, "g", "g")
	for range len(a.discover.rows) {
		if r, ok := a.discover.selected(); ok && r.Name == name {
			return
		}
		keys(t, a, "j")
	}
	t.Fatalf("%s not found on Discover", name)
}

func TestDiscoverPreview(t *testing.T) {
	a, cat := newDiscoverApp(t)
	cat.details = map[string]brew.Details{"ripgrep": ripgrep}
	keys(t, a, "6", "F") // All
	selectDiscover(t, a, "ripgrep")
	out := stripANSI(a.render())
	for _, want := range []string{
		"ripgrep 15.2.0", "Search tool like grep", "formula", "homebrew/core", "Unlicense",
		"https://github.com/BurntSushi/ripgrep", "Would also install", "oniguruma",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("preview is missing %q:\n%s", want, out)
		}
	}
	// pcre2 is installed already.
	if r, _ := a.discover.selected(); strings.Contains(stripANSI(a.discover.preview(r)), "pcre2") {
		t.Errorf("pcre2 is installed, so it isn't extra:\n%s", out)
	}

	selectDiscover(t, a, "git")
	if out := stripANSI(a.render()); !strings.Contains(out, "installed  yes") {
		t.Errorf("git is installed:\n%s", out)
	}
}

func TestDiscoverPreviewInstalls(t *testing.T) {
	a, _ := newStatsApp(t)
	keys(t, a, "6") // Popular tools, gh first
	out := stripANSI(a.render())
	for _, want := range []string{"Installs", "30 days    217,104", "90 days    710,524", "365 days   2,994,048"} {
		if !strings.Contains(out, want) {
			t.Errorf("preview is missing %q:\n%s", want, out)
		}
	}
	keys(t, a, "f") // codex: apps have no 365-day list
	if out := stripANSI(a.render()); !strings.Contains(out, "90 days    345,161") || strings.Contains(out, "365 days") {
		t.Errorf("cask installs wrong:\n%s", out)
	}
}

func TestDiscoverAsksBrewOnlyWhereTheCursorRests(t *testing.T) {
	a, cat := newDiscoverApp(t)
	keys(t, a, "6", "F") // asks about abcl, the first row
	cat.asked = nil
	var cmds []func()
	for range 3 {
		_, cmd := a.Update(press("j"))
		cmds = append(cmds, func() { drain(t, a, cmd) })
	}
	for _, run := range cmds { // the waits end after the cursor has moved on
		run()
	}
	if !slices.Equal(cat.asked, []string{"gh"}) {
		t.Errorf("asked about %v, want only gh, where the cursor stopped", cat.asked)
	}
	keys(t, a, "k", "j") // fd, then gh again: gh is known
	if !slices.Equal(cat.asked, []string{"gh", "fd"}) {
		t.Errorf("asked about %v", cat.asked)
	}
}

func TestDiscoverPreviewFailsAndWaits(t *testing.T) {
	a, cat := newDiscoverApp(t)
	cat.detailsErr = errors.New("brew info: exit status 1")
	keys(t, a, "6", "F")
	if out := stripANSI(a.render()); !strings.Contains(out, "couldn't ask brew") {
		t.Errorf("the failure isn't shown:\n%s", out)
	}

	a, _ = newDiscoverApp(t)
	keys(t, a, "6")
	a.Update(press("F")) // to All, and brew's answer about abcl hasn't come
	if out := stripANSI(a.render()); !strings.Contains(out, "asking brew…") {
		t.Errorf("want a note while brew is asked:\n%s", out)
	}
}

func TestDiscoverPreviewFocusAndLinks(t *testing.T) {
	a, cat := newDiscoverApp(t)
	cat.details = map[string]brew.Details{"ripgrep": ripgrep}
	var opened, copied string
	a.UseOpener(func(url string) error { opened = url; return nil })
	a.UseClipboard(func(text string) error { copied = text; return nil })
	keys(t, a, "6", "F")
	selectDiscover(t, a, "ripgrep")
	keys(t, a, "g", "x", "y")
	if opened != "https://github.com/BurntSushi/ripgrep" || copied != "ripgrep" {
		t.Errorf("opened %q, copied %q", opened, copied)
	}
	keys(t, a, "tab")
	if !a.discover.pv.focus {
		t.Fatal("tab should focus the preview")
	}
	keys(t, a, "j")
	if r, _ := a.discover.selected(); r.Name != "ripgrep" {
		t.Errorf("j scrolls the focused preview, not the list: on %s", r.Name)
	}
	keys(t, a, "esc")
	if a.discover.pv.focus {
		t.Error("esc goes back to the list")
	}
}

func TestDiscoverFitsSmallTerminals(t *testing.T) {
	for _, size := range []struct{ w, h int }{{140, 40}, {100, 30}, {80, 24}, {60, 20}, {50, 16}} {
		a, cat := newDiscoverApp(t)
		cat.details = map[string]brew.Details{"ripgrep": ripgrep}
		a.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		keys(t, a, "6", "F")
		selectDiscover(t, a, "ripgrep")
		for _, focus := range []bool{false, true} {
			a.discover.pv.focus = focus
			out := a.render()
			if h := lipgloss.Height(out); h != size.h {
				t.Errorf("%dx%d focus=%v: %d lines\n%s", size.w, size.h, focus, h, stripANSI(out))
			}
			if w := lipgloss.Width(out); w > size.w {
				t.Errorf("%dx%d focus=%v: %d columns", size.w, size.h, focus, w)
			}
		}
	}
}
```

- [ ] **Step 2: Edit `internal/ui/discover_test.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
)

// fakeCatalog serves the catalogue fixture; added is what brew's last
// update brought in. calls counts catalogue reads.
type fakeCatalog struct {
	entries, added []brew.Entry
	err            error
	calls          int
}

func newFakeCatalog(t *testing.T) *fakeCatalog {
```

   with:

```go
)

// fakeCatalog serves the catalogue fixture; added is what brew's last
// update brought in. calls counts catalogue reads. details answers
// questions about one package by name (a bare entry if it isn't there),
// detailsErr makes them fail, and asked records each package asked about.
type fakeCatalog struct {
	entries, added []brew.Entry
	err            error
	calls          int
	details        map[string]brew.Details
	detailsErr     error
	asked          []string
}

func newFakeCatalog(t *testing.T) *fakeCatalog {
```

2. Replace:

```go

func (f *fakeCatalog) NewSinceUpdate(context.Context) ([]brew.Entry, error) { return f.added, nil }

// newDiscoverApp is an app with packages loaded and a catalogue to read,
// still on Home.
func newDiscoverApp(t *testing.T) (*App, *fakeCatalog) {
```

   with:

```go

func (f *fakeCatalog) NewSinceUpdate(context.Context) ([]brew.Entry, error) { return f.added, nil }

func (f *fakeCatalog) Details(_ context.Context, name string, kind brew.Kind) (brew.Details, error) {
	f.asked = append(f.asked, name)
	if f.detailsErr != nil {
		return brew.Details{}, f.detailsErr
	}
	if d, ok := f.details[name]; ok {
		return d, nil
	}
	return brew.Details{Package: brew.Package{Name: name, Kind: kind}}, nil
}

// newDiscoverApp is an app with packages loaded and a catalogue to read,
// still on Home.
func newDiscoverApp(t *testing.T) (*App, *fakeCatalog) {
```

3. Replace:

```go
	a := newTestApp(t, newFakeSource(t))
	cat := newFakeCatalog(t)
	a.UseCatalog(cat)
	drain(t, a, a.Init())
	return a, cat
}
```

   with:

```go
	a := newTestApp(t, newFakeSource(t))
	cat := newFakeCatalog(t)
	a.UseCatalog(cat)
	a.detailsDelay = 0
	drain(t, a, a.Init())
	return a, cat
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/details_test.go:52:74: a.discover.preview undefined (type discoverTab has no field or method preview)
    internal/ui/details_test.go:127:17: a.discover.pv undefined (type discoverTab has no field or method pv)
    internal/ui/details_test.go:135:16: a.discover.pv undefined (type discoverTab has no field or method pv)
```

- [ ] **Step 4: Edit `internal/ui/actions.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	case tabTaps:
		return "x untap"
	case tabDiscover:
		return "f/F view · / search · r refetch"
	}
	return "1-6 tabs · r refresh · U update · c clean up"
}
```

   with:

```go
	case tabTaps:
		return "x untap"
	case tabDiscover:
		if a.discover.pv.focus {
			return "j/k scroll · tab back to the list"
		}
		return "f/F view · / search · tab preview · r refetch"
	}
	return "1-6 tabs · r refresh · U update · c clean up"
}
```

- [ ] **Step 5: Edit `internal/ui/app.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
	jobs     listTab       // the ":jobs" view
	jobsOpen bool

	mode     mode
	cmd      cmdline         // the ":" prompt
	filterIn textinput.Model // the "/" prompt
```

   with:

```go
	jobs     listTab       // the ":jobs" view
	jobsOpen bool

	detailsDelay time.Duration // how long the Discover cursor rests before brew is asked

	mode     mode
	cmd      cmdline         // the ":" prompt
	filterIn textinput.Model // the "/" prompt
```

2. Replace:

```go
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		acts: map[int]action{}, backoff: jobs.Backoff, tickEvery: 100 * time.Millisecond, quitGrace: quitGrace,
		readLog:   readLogTail,
		drawer:    newDrawer(st),
		cmd:       newCmdline(st),
		filterIn:  newFilterInput(st),
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
		taps:      newListTab(st, "Taps", tapCols, firstCell, "no taps installed"),
		jobs:      newListTab(st, "Jobs", jobCols, jobKey, "no jobs yet"),
		discover:  newDiscoverTab(st),
	}
	// The history is read before the app starts, so the jobs view is never
	// waiting for it.
```

   with:

```go
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		acts: map[int]action{}, backoff: jobs.Backoff, tickEvery: 100 * time.Millisecond, quitGrace: quitGrace,
		detailsDelay: detailsDelay,
		readLog:      readLogTail,
		drawer:       newDrawer(st),
		cmd:          newCmdline(st),
		filterIn:     newFilterInput(st),
		installed:    newInstalledTab(st),
		outdated:     newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:     newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
		taps:         newListTab(st, "Taps", tapCols, firstCell, "no taps installed"),
		jobs:         newListTab(st, "Jobs", jobCols, jobKey, "no jobs yet"),
		discover:     newDiscoverTab(st),
	}
	// The history is read before the app starts, so the jobs view is never
	// waiting for it.
```

3. Replace:

```go
	if a.tab == tabServices && !a.jobsOpen {
		cmd = tea.Batch(cmd, a.wantLog())
	}
	if a.promptPending && a.mode == modeNormal {
		a.promptPending = false
		if a.proc != nil && a.drawer.prompt() {
```

   with:

```go
	if a.tab == tabServices && !a.jobsOpen {
		cmd = tea.Batch(cmd, a.wantLog())
	}
	if a.tab == tabDiscover && !a.jobsOpen {
		cmd = tea.Batch(cmd, a.wantDetails())
	}
	if a.promptPending && a.mode == modeNormal {
		a.promptPending = false
		if a.proc != nil && a.drawer.prompt() {
```

4. Replace:

```go
		return a.gotCachedStats(msg)
	case fetchedStatsMsg:
		return a.gotStats(msg)
	case savedMsg:
		a.saveErr = msg.err
	case historySavedMsg:
```

   with:

```go
		return a.gotCachedStats(msg)
	case fetchedStatsMsg:
		return a.gotStats(msg)
	case detailsDueMsg:
		return a.detailsDue(msg)
	case detailsMsg:
		a.gotDetails(msg)
	case savedMsg:
		a.saveErr = msg.err
	case historySavedMsg:
```

5. Replace:

```go
		switch {
		case a.tab == tabInstalled && a.installed.pv.focus:
			a.installed.pv.focus = false // back to the list first
		case a.tab == tabServices && a.services.pvOpen && a.services.previewAlone():
			a.services.pvOpen = false
		case a.marks() > 0 && !a.drawerShown():
```

   with:

```go
		switch {
		case a.tab == tabInstalled && a.installed.pv.focus:
			a.installed.pv.focus = false // back to the list first
		case a.tab == tabDiscover && a.discover.pv.focus:
			a.discover.pv.focus = false
		case a.tab == tabServices && a.services.pvOpen && a.services.previewAlone():
			a.services.pvOpen = false
		case a.marks() > 0 && !a.drawerShown():
```

- [ ] **Step 6: Create `internal/ui/details.go`**

```go
package ui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/analytics"
	"brewery/internal/brew"
)

// detailsDelay is how long the cursor rests on a package before Discover
// asks brew about it, so holding j doesn't start a brew for every row.
const detailsDelay = 250 * time.Millisecond

// Messages from waiting on the cursor and asking brew about a package.
type (
	detailsDueMsg struct{ key string }
	detailsMsg    struct {
		key string
		d   brew.Details
		err error
	}
)

// wantDetails waits for the cursor to rest, then asks brew about the
// selected package if the preview doesn't know it yet.
func (a *App) wantDetails() tea.Cmd {
	d := &a.discover
	key := d.selectedKey()
	if a.catalog == nil || key == "" || key == d.waiting || key == d.asking {
		return nil
	}
	if _, ok := d.details[key]; ok {
		return nil
	}
	if _, ok := d.detailsErr[key]; ok {
		return nil
	}
	d.waiting = key
	return tea.Tick(a.detailsDelay, func(time.Time) tea.Msg { return detailsDueMsg{key} })
}

// detailsDue asks brew about the package once the cursor has rested on
// it, unless it has since moved on.
func (a *App) detailsDue(m detailsDueMsg) tea.Cmd {
	d := &a.discover
	if m.key != d.waiting {
		return nil // the cursor moved on, and a newer wait is running
	}
	d.waiting = ""
	r, ok := d.selected()
	if !ok || entryKey(r.Name, r.Kind) != m.key {
		return nil
	}
	d.asking = m.key
	cat, ctx := a.catalog, a.ctx
	return func() tea.Msg {
		det, err := cat.Details(ctx, r.Name, r.Kind)
		return detailsMsg{m.key, det, err}
	}
}

func (a *App) gotDetails(m detailsMsg) {
	d := &a.discover
	if d.asking == m.key {
		d.asking = ""
	}
	switch {
	case m.err == nil:
		d.details[m.key] = m.d
	case a.ctx.Err() == nil: // quitting cancels the question: that's no failure
		d.detailsErr[m.key] = m.err
	}
}

// missing is what installing the package with key would add: what it
// needs that isn't installed. ok is false until brew has said.
func (t *discoverTab) missing(key string) (names []string, ok bool) {
	det, ok := t.details[key]
	if !ok {
		return nil, false
	}
	for _, n := range det.Needs {
		if !t.have[entryKey(n, brew.KindFormula)] && !t.have[entryKey(n, brew.KindCask)] {
			names = append(names, n)
		}
	}
	return names, true
}

func (t *discoverTab) preview(r discRow) string {
	st := t.st
	key := entryKey(r.Name, r.Kind)
	det, known := t.details[key]
	var b strings.Builder
	b.WriteString(st.Title.Render(strings.TrimSpace(r.Name+" "+det.Latest)) + "\n")
	if r.Desc != "" {
		b.WriteString(st.Muted.Render(r.Desc) + "\n")
	}
	b.WriteString("\n")
	installed := "no"
	if t.have[key] {
		installed = st.OK.Render("yes")
	}
	b.WriteString(kv(st, "type", r.Kind.String(), "installed", installed,
		"tap", det.Tap, "licence", det.License, "homepage", det.Homepage))

	b.WriteString("\n" + st.Accent.Render("Would also install") + "\n")
	switch missing, ok := t.missing(key); {
	case ok && len(missing) == 0:
		b.WriteString(st.Muted.Render("nothing else") + "\n")
	case ok:
		b.WriteString(strings.Join(missing, ", ") + "\n")
	case t.detailsErr[key] != nil:
		first, _, _ := strings.Cut(t.detailsErr[key].Error(), "\n")
		b.WriteString(st.Err.Render("couldn't ask brew: "+first) + "\n")
	case !known:
		b.WriteString(st.Muted.Render("asking brew…") + "\n")
	}

	lists := []struct{ label, list string }{{"30 days", analytics.Tools30}, {"90 days", analytics.Tools90}, {"365 days", analytics.Tools365}}
	if r.Kind == brew.KindCask {
		lists = []struct{ label, list string }{{"30 days", analytics.Apps30}, {"90 days", analytics.Apps90}}
	}
	var pairs []string
	for _, l := range lists {
		if n, ok := t.counts[l.list][r.Name]; ok {
			pairs = append(pairs, l.label, withCommas(n))
		}
	}
	if len(pairs) > 0 {
		b.WriteString("\n" + st.Accent.Render("Installs") + "\n" + kv(st, pairs...))
	}
	return b.String()
}

// withCommas writes n with thousands separators, as Homebrew does:
// 2,994,048.
func withCommas(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
```

- [ ] **Step 7: Edit `internal/ui/discover.go`** (10 replacements; each old block appears exactly once)

1. Replace:

```go
type Catalog interface {
	Catalogue(ctx context.Context) ([]brew.Entry, error)
	NewSinceUpdate(ctx context.Context) ([]brew.Entry, error)
}

var _ Catalog = (*brew.Client)(nil)
```

   with:

```go
type Catalog interface {
	Catalogue(ctx context.Context) ([]brew.Entry, error)
	NewSinceUpdate(ctx context.Context) ([]brew.Entry, error)
	Details(ctx context.Context, name string, kind brew.Kind) (brew.Details, error)
}

var _ Catalog = (*brew.Client)(nil)
```

2. Replace:

```go
	table   table.Model
	w, h    int

	stats     map[string]analytics.List // by list name
	statsAt   map[string]time.Time      // when each list was fetched
	statsRead bool                      // the kept copies have been read
	fetching  bool
```

   with:

```go
	table   table.Model
	w, h    int

	pv         scroller                // the preview, which tab or enter focuses
	details    map[string]brew.Details // what brew said about each package, by entryKey
	detailsErr map[string]error        // why brew couldn't say, by entryKey
	waiting    string                  // the package the cursor is resting on, before brew is asked
	asking     string                  // the package brew is being asked about

	stats     map[string]analytics.List // by list name
	counts    map[string]map[string]int // each list's installs by name
	statsAt   map[string]time.Time      // when each list was fetched
	statsRead bool                      // the kept copies have been read
	fetching  bool
```

3. Replace:

```go
	return discoverTab{
		st: st, table: newTable(st, discoverCols[discTools]),
		stats: map[string]analytics.List{}, statsAt: map[string]time.Time{},
	}
}
```

   with:

```go
	return discoverTab{
		st: st, table: newTable(st, discoverCols[discTools]),
		stats: map[string]analytics.List{}, statsAt: map[string]time.Time{},
		details: map[string]brew.Details{}, detailsErr: map[string]error{},
	}
}
```

4. Replace:

```go
	for _, e := range entries {
		desc[entryKey(e.Name, e.Kind)] = e.Desc
	}
	t.cat, t.added, t.loaded = entries, make([]brew.Entry, len(added)), true
	for i, e := range added {
		e.Desc = desc[entryKey(e.Name, e.Kind)]
```

   with:

```go
	for _, e := range entries {
		desc[entryKey(e.Name, e.Kind)] = e.Desc
	}
	// brew update can bring new versions: ask brew again.
	t.details, t.detailsErr = map[string]brew.Details{}, map[string]error{}
	t.cat, t.added, t.loaded = entries, make([]brew.Entry, len(added)), true
	for i, e := range added {
		e.Desc = desc[entryKey(e.Name, e.Kind)]
```

5. Replace:

```go
		t.byView[discNew][i] = discRow{Entry: e}
	}
	t.byView[discAll] = all
	t.apply()
}
```

   with:

```go
		t.byView[discNew][i] = discRow{Entry: e}
	}
	t.byView[discAll] = all
	t.counts = make(map[string]map[string]int, len(t.stats))
	for name, l := range t.stats {
		t.counts[name] = l.Counts()
	}
	t.apply()
}
```

6. Replace:

```go
	if i := slices.IndexFunc(t.rows, func(r discRow) bool { return entryKey(r.Name, r.Kind) == before }); i >= 0 {
		t.table.SetCursor(i)
	}
}

// cells is r's row in the current view. Installed packages are marked
```

   with:

```go
	if i := slices.IndexFunc(t.rows, func(r discRow) bool { return entryKey(r.Name, r.Kind) == before }); i >= 0 {
		t.table.SetCursor(i)
	}
	t.followSelection(before)
}

// followSelection starts the preview at the top when the selection has
// moved on from before.
func (t *discoverTab) followSelection(before string) {
	if t.selectedKey() != before {
		t.pv.off = 0
	}
}

// cells is r's row in the current view. Installed packages are marked
```

7. Replace:

```go
}

func (t *discoverTab) update(msg tea.KeyPressMsg) tea.Cmd {
	if k := msg.String(); k == "f" || k == "F" {
		step := discoverView(1)
		if k == "F" {
			step = discoverView(len(discoverViews) - 1)
```

   with:

```go
}

func (t *discoverTab) update(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	switch {
	case k == "tab" || (k == "enter" && !t.pv.focus):
		t.pv.focus = !t.pv.focus
		return nil
	case t.pv.focus && k != "f" && k != "F":
		sp := splitFor(t.w, t.h, true)
		_, ih := paneInner(sp.pvW, sp.pvH)
		t.pv.scroll(k, ih/2)
		return nil
	}
	if k == "f" || k == "F" {
		step := discoverView(1)
		if k == "F" {
			step = discoverView(len(discoverViews) - 1)
```

8. Replace:

```go
		t.apply()
		return nil
	}
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}

// size fits the table to the pane, under the bar of views.
func (t *discoverTab) size() {
	iw, ih := paneInner(t.w, t.h)
	sizeTable(&t.table, discoverCols[t.showing], iw, max(ih-viewBarHeight, 0))
}
```

   with:

```go
		t.apply()
		return nil
	}
	before := t.selectedKey()
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	t.followSelection(before)
	return cmd
}

// size fits the table to the list pane, under the bar of views.
func (t *discoverTab) size() {
	sp := splitFor(t.w, t.h, false)
	iw, ih := paneInner(sp.listW, sp.listH)
	sizeTable(&t.table, discoverCols[t.showing], iw, max(ih-viewBarHeight, 0))
}
```

9. Replace:

```go
	if t.err != nil && !t.loaded {
		return pane(t.st, "Discover", errorBody(t.st, t.err), w, h, true)
	}
	iw, ih := paneInner(w, h)
	body := t.viewBar(iw) + "\n"
	ih -= viewBarHeight
	if t.err != nil {
```

   with:

```go
	if t.err != nil && !t.loaded {
		return pane(t.st, "Discover", errorBody(t.st, t.err), w, h, true)
	}
	sp := splitFor(w, h, t.pv.focus)
	if !t.loaded {
		sp = split{listW: w, listH: h} // nothing to preview yet
	}
	iw, ih := paneInner(sp.listW, sp.listH)
	body := t.viewBar(iw) + "\n"
	ih -= viewBarHeight
	if t.err != nil {
```

10. Replace:

```go
		t.table.SetHeight(max(ih, 0))
		body += t.table.View()
	}
	return pane(t.st, title, body, w, h, true)
}

// empty says why the current view has no rows.
```

   with:

```go
		t.table.SetHeight(max(ih, 0))
		body += t.table.View()
	}
	list := pane(t.st, title, body, sp.listW, sp.listH, !t.pv.focus)
	if sp.pvW == 0 {
		return list
	}
	previewTitle, preview := "Preview", t.st.Muted.Render("nothing in this view")
	if r, ok := t.selected(); ok {
		_, ph := paneInner(sp.pvW, sp.pvH)
		previewTitle, preview = r.Name, t.pv.window(t.preview(r), ph)
	}
	return sp.join(list, pane(t.st, previewTitle, preview, sp.pvW, sp.pvH, t.pv.focus))
}

// empty says why the current view has no rows.
```

- [ ] **Step 8: Edit `internal/ui/help.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	case a.tab == tabDiscover:
		here = helpGroup{"Discover", [][2]string{
			{"f F", "next or previous view"}, {"/", "search names and descriptions"}, {"r", "fetch the analytics again"},
		}}
	}
	groups := []helpGroup{here}
```

   with:

```go
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	case a.tab == tabDiscover:
		here = helpGroup{"Discover", [][2]string{
			{"f F", "next or previous view"}, {"/", "search names and descriptions"},
			{"tab enter", "preview, then j/k scroll"}, {"r", "fetch the analytics again"},
		}}
	}
	groups := []helpGroup{here}
```

- [ ] **Step 9: Edit `internal/ui/links.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		}
	case tabDiscover:
		if r, ok := a.discover.selected(); ok {
			return r.Name, "", true
		}
	}
	return "", "", false
```

   with:

```go
		}
	case tabDiscover:
		if r, ok := a.discover.selected(); ok {
			return r.Name, a.discover.details[entryKey(r.Name, r.Kind)].Homepage, true
		}
	}
	return "", "", false
```

- [ ] **Step 10: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "feat(ui): Discover's preview asks brew about the package the cursor rests on"
```

---

### Task 8: a installs from Discover, after a dialog naming what comes along

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/confirm.go`
- Modify: `internal/ui/details.go`
- Modify: `internal/ui/help.go`
- Create: `internal/ui/install.go`
- Create: `internal/ui/install_test.go`

**Interfaces:**
- **Consumes:** Task 7's `missing` and details; `action`, `kindArgs`, `diskQueries`, `confirmBox`, `run`.
- **Produces:**
  - `confirmBox.tag`.
  - `askInstall`, `installTag(key)`, `installBox(r)`, `alsoInstalls(names)`.
  - `gotDetails` rebuilds an install dialog that is open for that package.

- [ ] **Step 1: Create `internal/ui/install_test.go`**

```go
package ui

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

// newInstallApp is a Discover app that runs jobs, on the All view.
func newInstallApp(t *testing.T) (*App, *fakeStarter, *fakeSource, *fakeCatalog) {
	t.Helper()
	fs, src := &fakeStarter{}, newFakeSource(t)
	a := New(src, fs.start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	cat := newFakeCatalog(t)
	cat.details = map[string]brew.Details{"ripgrep": ripgrep}
	a.UseCatalog(cat)
	a.detailsDelay = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	keys(t, a, "6", "F")
	return a, fs, src, cat
}

// confirmText is the open confirmation's title and body, or "".
func confirmText(a *App) string {
	if a.confirm == nil {
		return ""
	}
	return a.confirm.title + "\n" + strings.Join(a.confirm.body, "\n")
}

func TestInstallAsksFirst(t *testing.T) {
	a, fs, src, _ := newInstallApp(t)
	selectDiscover(t, a, "ripgrep")
	keys(t, a, "a")
	want := "Install ripgrep 15.2.0?\nSearch tool like grep and The Silver Searcher\nalso installs 1 dependency: oniguruma"
	if got := confirmText(a); got != want {
		t.Errorf("asked\n%s\nwant\n%s", got, want)
	}
	keys(t, a, "n")
	if len(fs.calls) != 0 {
		t.Fatalf("n ran %v", fs.calls)
	}

	src.snap.Packages = append(src.snap.Packages, brew.Package{Name: "ripgrep", FullName: "ripgrep", Kind: brew.KindFormula, Version: "15.2.0"})
	keys(t, a, "a", "y")
	if want := []string{"install", "ripgrep"}; !slices.Equal(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	// The dialog asked: brew needn't ask again.
	if env := fs.envs[len(fs.envs)-1]; !slices.Contains(env, "HOMEBREW_NO_ASK=1") || !slices.Contains(env, "HOMEBREW_NO_AUTO_UPDATE=1") {
		t.Errorf("env %v", env)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "✓ install ripgrep") || !strings.Contains(out, "✓ ripgrep") {
		t.Errorf("want the job done and ripgrep marked installed:\n%s", out)
	}
}

func TestInstallSaysWhenInstalled(t *testing.T) {
	a, fs, _, _ := newInstallApp(t)
	selectDiscover(t, a, "git")
	keys(t, a, "a")
	if a.confirm != nil || len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "git is already installed") {
		t.Errorf("confirm %v, calls %v, status %q", a.confirm, fs.calls, stripANSI(a.statusBar()))
	}
}

func TestInstallCasks(t *testing.T) {
	a, fs, _, _ := newInstallApp(t)
	selectDiscover(t, a, "firefox")
	keys(t, a, "a")
	if got, want := confirmText(a), "Install firefox?\napp · Web browser"; got != want {
		t.Errorf("asked\n%s\nwant\n%s", got, want)
	}
	keys(t, a, "y")
	if want := []string{"install", "--cask", "firefox"}; !slices.Equal(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}

	selectDiscover(t, a, "font-fira-code")
	keys(t, a, "a")
	if got := confirmText(a); !strings.Contains(got, "\nfont · Fira Code") {
		t.Errorf("asked\n%s", got)
	}
}

func TestInstallBeforeBrewHasSaid(t *testing.T) {
	a, fs, _, _ := newInstallApp(t)
	selectDiscover(t, a, "python@3.15")
	a.Update(press("j")) // to ripgrep; brew hasn't been asked yet
	a.Update(press("a"))
	if got := confirmText(a); !strings.Contains(got, "checking dependencies…") {
		t.Fatalf("asked\n%s", got)
	}
	drain(t, a, a.detailsDue(detailsDueMsg{key: entryKey("ripgrep", brew.KindFormula)}))
	if got := confirmText(a); !strings.Contains(got, "also installs 1 dependency: oniguruma") {
		t.Errorf("the dialog didn't catch up:\n%s", got)
	}
	keys(t, a, "y")
	if want := []string{"install", "ripgrep"}; !slices.Equal(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestAlsoInstalls(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string
	}{
		{[]string{"pcre2"}, "also installs 1 dependency: pcre2"},
		{[]string{"a", "b", "c", "d", "e", "f", "g"}, "also installs 7 dependencies: a, b, c, d, e +2 more"},
	} {
		if got := alsoInstalls(c.names); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/install_test.go:121:13: undefined: alsoInstalls
```

- [ ] **Step 3: Edit `internal/ui/actions.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
		return a.openLog(), true
	case a.tab == tabTaps && k == "x":
		return a.askUntap(), true
	}
	return nil, false
}
```

   with:

```go
		return a.openLog(), true
	case a.tab == tabTaps && k == "x":
		return a.askUntap(), true
	case a.tab == tabDiscover && k == "a":
		return a.askInstall(), true
	}
	return nil, false
}
```

2. Replace:

```go
		if a.discover.pv.focus {
			return "j/k scroll · tab back to the list"
		}
		return "f/F view · / search · tab preview · r refetch"
	}
	return "1-6 tabs · r refresh · U update · c clean up"
}
```

   with:

```go
		if a.discover.pv.focus {
			return "j/k scroll · tab back to the list"
		}
		return "a install · f/F view · / search · tab preview · r refetch"
	}
	return "1-6 tabs · r refresh · U update · c clean up"
}
```

- [ ] **Step 4: Edit `internal/ui/confirm.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	title   string
	body    []string
	choices []choice
}

// answer maps a key press to a choice. esc always cancels; keys that are
```

   with:

```go
	title   string
	body    []string
	choices []choice
	tag     string // what the box is about, so news can update it while it's open
}

// answer maps a key press to a choice. esc always cancels; keys that are
```

- [ ] **Step 5: Edit `internal/ui/details.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	case a.ctx.Err() == nil: // quitting cancels the question: that's no failure
		d.detailsErr[m.key] = m.err
	}
}

// missing is what installing the package with key would add: what it
```

   with:

```go
	case a.ctx.Err() == nil: // quitting cancels the question: that's no failure
		d.detailsErr[m.key] = m.err
	}
	// An install dialog open for the package can now say what comes along.
	if r, ok := d.selected(); ok && a.confirm != nil && a.confirm.tag == installTag(m.key) {
		a.confirm = a.installBox(r)
	}
}

// missing is what installing the package with key would add: what it
```

- [ ] **Step 6: Edit `internal/ui/help.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	case a.tab == tabDiscover:
		here = helpGroup{"Discover", [][2]string{
			{"f F", "next or previous view"}, {"/", "search names and descriptions"},
			{"tab enter", "preview, then j/k scroll"}, {"r", "fetch the analytics again"},
		}}
	}
```

   with:

```go
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	case a.tab == tabDiscover:
		here = helpGroup{"Discover", [][2]string{
			{"a", "install"}, {"f F", "next or previous view"}, {"/", "search names and descriptions"},
			{"tab enter", "preview, then j/k scroll"}, {"r", "fetch the analytics again"},
		}}
	}
```

- [ ] **Step 7: Create `internal/ui/install.go`**

```go
package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

// askInstall confirms installing the package selected on Discover, naming
// what else it would add.
func (a *App) askInstall() tea.Cmd {
	r, ok := a.discover.selected()
	if !ok {
		return nil
	}
	if a.discover.have[entryKey(r.Name, r.Kind)] {
		return a.note(r.Name + " is already installed")
	}
	a.confirm = a.installBox(r)
	return nil
}

// installTag marks the install dialog for the package with key, so brew's
// answer about its dependencies can update it.
func installTag(key string) string { return "install " + key }

// installBox is the dialog for installing r. Until brew has said what r
// needs, it says it's checking; y works either way.
func (a *App) installBox(r discRow) *confirmBox {
	key := entryKey(r.Name, r.Kind)
	about := []string{r.Desc}
	if r.Kind == brew.KindCask {
		what := "app"
		if strings.HasPrefix(r.Name, "font-") {
			what = "font"
		}
		about = []string{what, r.Desc}
	}
	var body []string
	if line := strings.Join(slices.DeleteFunc(about, func(s string) bool { return s == "" }), " · "); line != "" {
		body = append(body, line)
	}
	switch missing, ok := a.discover.missing(key); {
	case !ok:
		body = append(body, "checking dependencies…")
	case len(missing) > 0:
		body = append(body, alsoInstalls(missing))
	}
	// This dialog is the confirmation, so brew 7 needn't ask again.
	install := action{title: "install " + r.Name, args: kindArgs("install", r.Kind, r.Name), env: []string{"HOMEBREW_NO_ASK=1"}, refresh: diskQueries}
	return &confirmBox{
		title: "Install " + strings.TrimSpace(r.Name+" "+a.discover.details[key].Latest) + "?",
		body:  body,
		choices: []choice{
			{"y", "install", func() tea.Cmd { return a.run(install) }},
			{"n", "cancel", nil},
		},
		tag: installTag(key),
	}
}

// alsoInstalls says which dependencies come along, naming at most five.
func alsoInstalls(names []string) string {
	noun := "dependency"
	if len(names) > 1 {
		noun = "dependencies"
	}
	s := fmt.Sprintf("also installs %d %s: %s", len(names), noun, strings.Join(names[:min(len(names), 5)], ", "))
	if len(names) > 5 {
		s += fmt.Sprintf(" +%d more", len(names)-5)
	}
	return s
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(ui): a installs from Discover, after a dialog naming what comes along"
```

---

### Task 9: Commands that take any package complete from the catalogue

**Files:**
- Modify: `internal/ui/cmdline.go`
- Modify: `internal/ui/complete.go`
- Modify: `internal/ui/complete_test.go`
- Modify: `internal/ui/discover.go`

**Interfaces:**
- **Consumes:** Task 5's catalogue; `argValues`, `suggest`.
- **Produces:**
  - `wantCatalogue()`, used by `showTab` and `openCmdline`.
  - `gotCatalogue` refreshes the suggestions while the `:` line is open.
  - The kinds `formulae_all` and `casks_all` complete from the catalogue.

`testCompletions` gains `install`.

- [ ] **Step 1: Edit `internal/ui/complete_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
// testCompletions is a small slice of what brew's fish script describes.
var testCompletions = brew.Completions{Commands: []brew.Command{
	{Name: "info", Desc: "Display brief statistics", Args: []string{"formulae_all", "casks_all"}},
	{Name: "pin", Desc: "Pin the specified package", Args: []string{"formulae_installed"}},
	{Name: "services", Desc: "Manage background services", Subs: []brew.Command{
		{Name: "start", Desc: "Start the service", Args: []string{"services"}, Flags: []brew.Flag{{Name: "--all", Desc: "Start all services"}}},
```

   with:

```go
// testCompletions is a small slice of what brew's fish script describes.
var testCompletions = brew.Completions{Commands: []brew.Command{
	{Name: "info", Desc: "Display brief statistics", Args: []string{"formulae_all", "casks_all"}},
	{Name: "install", Desc: "Install a formula or cask", Args: []string{"formulae_all", "casks_all"}},
	{Name: "pin", Desc: "Pin the specified package", Args: []string{"formulae_installed"}},
	{Name: "services", Desc: "Manage background services", Subs: []brew.Command{
		{Name: "start", Desc: "Start the service", Args: []string{"services"}, Flags: []brew.Flag{{Name: "--all", Desc: "Start all services"}}},
```

2. Replace:

```go
		}
	}
}
```

   with:

```go
		}
	}
}

func TestCompletesFromTheCatalogue(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	a.UseCatalog(newFakeCatalog(t))
	_, load := a.Update(press(":")) // the catalogue is read when the line opens
	for _, k := range "install rip" {
		a.Update(press(string(k)))
	}
	if len(a.cmd.items) != 0 {
		t.Fatalf("suggested %v before the catalogue came", candidateTexts(a.cmd.items))
	}
	drain(t, a, load)
	if got := candidateTexts(a.cmd.items); got != "ripgrep" {
		t.Errorf("once the catalogue came: %q, want ripgrep", got)
	}
	if _, cs := a.candidates("info fo"); candidateTexts(cs) != "font-fira-code font-jetbrains-mono fontbase" {
		t.Errorf("info fo: %q", candidateTexts(cs))
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    --- FAIL: TestCompletesFromTheCatalogue (0.00s)
```

- [ ] **Step 3: Edit `internal/ui/cmdline.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
func (a *App) openCmdline() tea.Cmd {
	a.mode = modeCommand
	a.cmd.open()
	return nil
}

// cmdKey handles a key in command mode: enter runs the line, esc cancels,
```

   with:

```go
func (a *App) openCmdline() tea.Cmd {
	a.mode = modeCommand
	a.cmd.open()
	return a.wantCatalogue() // for commands that take any package
}

// cmdKey handles a key in command mode: enter runs the line, esc cancels,
```

- [ ] **Step 4: Edit `internal/ui/complete.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
}

// argValues lists the values brewery knows for an argument kind from
// brew's completions. Kinds that need brew's full catalogue (every
// formula, not just installed ones) aren't known yet.
func (a *App) argValues(kind string) []candidate {
	var cs []candidate
	switch kind {
	case "formulae_installed", "casks_installed":
		want := brew.KindFormula
		if kind == "casks_installed" {
```

   with:

```go
}

// argValues lists the values brewery knows for an argument kind from
// brew's completions. Any formula or cask comes from Discover's catalogue,
// once it has been read.
func (a *App) argValues(kind string) []candidate {
	var cs []candidate
	switch kind {
	case "formulae_all", "casks_all":
		want := brew.KindFormula
		if kind == "casks_all" {
			want = brew.KindCask
		}
		for _, e := range a.discover.cat {
			if e.Kind == want {
				cs = append(cs, candidate{e.Name, e.Desc})
			}
		}
	case "formulae_installed", "casks_installed":
		want := brew.KindFormula
		if kind == "casks_installed" {
```

- [ ] **Step 5: Edit `internal/ui/discover.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	if t != tabDiscover {
		return nil
	}
	var cat tea.Cmd
	if !a.discover.loaded && a.discover.err == nil {
		cat = a.readCatalogue()
	}
	return tea.Batch(cat, a.wantStats())
}

// reloadCatalogue reads the catalogue again if Discover has read it (or
```

   with:

```go
	if t != tabDiscover {
		return nil
	}
	return tea.Batch(a.wantCatalogue(), a.wantStats())
}

// wantCatalogue reads the catalogue unless it has been read (or tried).
func (a *App) wantCatalogue() tea.Cmd {
	if a.discover.loaded || a.discover.err != nil {
		return nil
	}
	return a.readCatalogue()
}

// reloadCatalogue reads the catalogue again if Discover has read it (or
```

2. Replace:

```go
	if m.err == nil {
		a.discover.setCatalogue(m.entries, m.added)
	}
}

type discoverView int
```

   with:

```go
	if m.err == nil {
		a.discover.setCatalogue(m.entries, m.added)
	}
	if a.mode == modeCommand {
		a.suggest(false) // the line can now complete any package
	}
}

type discoverView int
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(ui): commands that take any package complete from the catalogue"
```

---

### Task 10: brewery reads the catalogue from brew and analytics from formulae.brew.sh

**Files:**
- Modify: `cmd/brewery/main.go`
- Create: `internal/analytics/live_test.go`
- Modify: `internal/brew/live_test.go`

**Interfaces:**
- **Consumes:** everything above.
- **Produces:**
  - `main` calls `app.UseCatalog(client)` and `app.UseAnalytics(analytics.NewStore(<dir of the snapshot>/analytics, version))`.
  - Live tests (build tag `live`) for the catalogue and details, and for one real download.

There is no failing unit test for this task: it is wiring. The live tests are the check. They run read-only brew queries and download one public list (`cask-install/30d`, about 0.9MB).

- [ ] **Step 1: Create `internal/analytics/live_test.go`**

```go
//go:build live

package analytics

import (
	"context"
	"testing"
)

// Downloads one real list from formulae.brew.sh, to catch format drift.
// Run with: go test -tags live ./internal/analytics/
func TestLiveFetch(t *testing.T) {
	s := NewStore(t.TempDir(), "live-test")
	l, err := s.Fetch(context.Background(), Apps30)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Items) < 1000 || l.Items[0].Rank != 1 || l.Items[0].Count < 1000 || l.End == "" {
		t.Errorf("got %d items from %s to %s, first %+v", len(l.Items), l.Start, l.End, l.Items[0])
	}
	if _, _, err := s.Cached(Apps30); err != nil {
		t.Error(err)
	}
}
```

- [ ] **Step 2: Edit `internal/brew/live_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	}
}

// Runs a read-only brew command through the pseudo-terminal runner.
func TestLivePTY(t *testing.T) {
	bin, err := Locate()
```

   with:

```go
	}
}

// Reads brew's catalogue and asks about packages that may not be installed.
func TestLiveCatalogue(t *testing.T) {
	bin, err := Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	c := NewClient(ExecRunner{Bin: bin, Env: QueryEnv})
	ctx := context.Background()
	es, err := c.Catalogue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	casks := 0
	for _, e := range es {
		if e.Kind == KindCask {
			casks++
			if strings.HasPrefix(e.Desc, "(") {
				t.Errorf("display names left in %+v", e)
			}
		}
	}
	if len(es) < 5000 || casks < 2000 {
		t.Errorf("only %d entries, %d casks", len(es), casks)
	}
	if _, err := c.NewSinceUpdate(ctx); err != nil {
		t.Error(err)
	}
	wget, err := c.Details(ctx, "wget", KindFormula)
	if err != nil || wget.Latest == "" || wget.License == "" || len(wget.Needs) == 0 {
		t.Errorf("wget: %+v, %v", wget, err)
	}
	firefox, err := c.Details(ctx, "firefox", KindCask)
	if err != nil || firefox.Kind != KindCask || firefox.Latest == "" {
		t.Errorf("firefox: %+v, %v", firefox, err)
	}
}

// Runs a read-only brew command through the pseudo-terminal runner.
func TestLivePTY(t *testing.T) {
	bin, err := Locate()
```

- [ ] **Step 3: Edit `cmd/brewery/main.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"brewery/internal/brew"
	"brewery/internal/jobs"
	"brewery/internal/state"
```

   with:

```go
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"brewery/internal/analytics"
	"brewery/internal/brew"
	"brewery/internal/jobs"
	"brewery/internal/state"
```

2. Replace:

```go
	app.UseHistory(hist, recs)
	app.UseIcons(wantIcons(*noIcons, os.Getenv("BREWERY_NO_ICONS")))
	app.UseCompletions(client.Completions)
	// Full-screen commands get the real stderr: while the TUI runs,
	// os.Stderr is the crash log.
	realStderr := os.Stderr
```

   with:

```go
	app.UseHistory(hist, recs)
	app.UseIcons(wantIcons(*noIcons, os.Getenv("BREWERY_NO_ICONS")))
	app.UseCompletions(client.Completions)
	app.UseCatalog(client)
	// Discover's analytics are kept beside the snapshot.
	app.UseAnalytics(analytics.NewStore(filepath.Join(filepath.Dir(cachePath), "analytics"), version))
	// Full-screen commands get the real stderr: while the TUI runs,
	// os.Stderr is the crash log.
	realStderr := os.Stderr
```

- [ ] **Step 4: Run the live tests (read-only brew queries and one public download)**

```bash
go test -tags live -count=1 -run 'LiveCatalogue|LiveFetch' ./internal/brew/ ./internal/analytics/
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: brewery reads the catalogue from brew and analytics from formulae.brew.sh"
```

---

### Task 11: End-to-end check in a real terminal

**Files:** none (verification only)

- [ ] **Step 1: Build and run**

Run: `make run`

- [ ] **Step 2: Check each of these by hand.** None of these change anything, except Step 2.6, which you cancel.
  1. **Opening:** press `6`. The title says "fetching analytics…", then Popular tools fills in, with your installed packages marked ✓ and dimmed.
  2. **Views:** `f` steps through Popular apps, Rising (`new` and `×` growth), New, Fonts and All; `F` goes back.
  3. **Preview:** rest on a row. The preview shows the version, licence and homepage, what it would also install, and its installs. `tab` focuses it, `j`/`k` scroll, and `esc` returns.
  4. **Search:** on All, `/` then a word filters names and descriptions. `gx` opens the homepage and `y` copies the name.
  5. **Installed:** `a` on a ✓ row only says it's already installed.
  6. **Dialog:** `a` on something not installed asks "Install <name> <version>?" and names any dependencies. **Press `n`.**
  7. **Completion:** `:install rip` then `tab` suggests ripgrep.
  8. **Offline:** quit, turn off the network, and start again. Discover shows the kept analytics; `r` says it couldn't reach formulae.brew.sh, and New and All still work.

- [ ] **Step 3: Final checks**

```bash
go test -race -count=1 ./...
go test -tags live -count=1 ./internal/brew/ ./internal/analytics/
go vet ./... && go vet -tags live ./...
gofmt -l .
go mod tidy -diff
```

Expected: all `ok`, with no output from vet, gofmt, or `go mod tidy -diff`.
