# Action Menu, Reinstall, Link and Fetch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `enter` opens an action menu on Installed, Outdated, Services, Taps, Discover and History. Reinstall (`R`), link/unlink (`L`) and fetch (`f`) are added as actions, reachable from the menu or directly.

**Architecture:**
- **Data:** brew's `linked_keg` and `keg_only` become `Package.Linked` and `Package.KegOnly`.
- **Actions:** the three new actions live in a new `internal/ui/reinstall.go`, with direct keys added to the existing per-tab key helpers.
- **The menu:** a small modal box in a new `internal/ui/menu.go`. Each item runs its key through `actionKey` (or `copyName`/`openHomepage`), so the menu and the keys can't drift apart.

**Tech Stack:** Go 1.26, Bubble Tea v2, Lip Gloss v2, golangci-lint v2.15.0.

**Spec:** `docs/superpowers/specs/2026-10-19-brewery-reinstall-link-fetch-design.md`

## Global Constraints

- **Lint:**
  - `.golangci.yml` is fixed: gocognit and gocyclo `min-complexity: 20`, with both excluded for `_test.go`. Don't edit it, and don't add `//nolint`.
  - `golangci-lint run ./...` must report `0 issues.` at the end of every task.
  - If a function you touch goes over a limit, split it into a named helper.
- **Coverage:** `make cover` stays at 90% or above in total and per package.
- **Goldens:** update them only in the tasks that say so, with `go test ./internal/ui -run TestScreens -update -count=1`. Read the diff (`git diff internal/ui/testdata/golden`) before committing; only the changes the task names may appear.
- **Style:**
  - Match the surrounding code: its comment density, naming and idioms.
  - Every function gets a short doc comment in plain sentences, as the rest of `internal/ui` does.
- **Exact texts:** user-facing strings are exactly as written in this plan.
- **Commits:** one commit per task, conventional style, with the message given in the task.

## Commands

| Check | Command |
|---|---|
| Tests | `go test ./... -count=1` |
| One package | `go test ./internal/ui -count=1` |
| Goldens | `go test ./internal/ui -run TestScreens -count=1` |
| Lint | `golangci-lint run ./...` |
| Coverage | `make cover` |

---

### Task 1: Linked and keg-only, and the details' `linked` row

**Files:**
- Modify: `internal/brew/types.go` (`Package`)
- Modify: `internal/brew/parse.go` (`formulaJSON`, `formulaJSON.toPackage`)
- Modify: `internal/brew/testdata/info.json`
- Modify: `internal/state/snapshot.go` (`snapshotVersion`)
- Modify: `internal/ui/installed.go` (the details, near line 281)
- Test: `internal/brew/parse_test.go`, `internal/ui/installed_test.go`
- Goldens: `internal/ui/testdata/golden/installed.txt` gains a `linked` row

**Interfaces:**
- Produces: `brew.Package.Linked bool` and `brew.Package.KegOnly bool`, which Tasks 2 and 4 use. In the fixture, every formula is linked except json-c, which is keg-only and unlinked. Casks are neither.

- [ ] **Step 1: Write the failing parse test.** Add to `internal/brew/parse_test.go`:

```go
func TestParseInfoLinks(t *testing.T) {
	pkgs, err := ParseInfo(fixture(t, "info.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name            string
		linked, kegOnly bool
	}{
		{"fd", true, false},
		{"json-c", false, true},
		{"docker-desktop", false, false}, // casks are never linked
	} {
		p := byName(pkgs, tt.name)
		if p.Linked != tt.linked || p.KegOnly != tt.kegOnly {
			t.Errorf("%s: linked %v, keg-only %v; want %v, %v", tt.name, p.Linked, p.KegOnly, tt.linked, tt.kegOnly)
		}
	}
}
```

- [ ] **Step 2: Run it.** `go test ./internal/brew -run TestParseInfoLinks -count=1`. Expected: it fails to compile, since `p.Linked` is undefined.

- [ ] **Step 3: Add the fields.**

In `internal/brew/types.go`, add to `Package` after `Pinned`:

```go
	Linked      bool      `json:"linked"`   // a formula's keg is linked into brew's prefix
	KegOnly     bool      `json:"keg_only"` // brew doesn't link the formula unless forced
```

(Let gofmt align the struct.)

In `internal/brew/parse.go`, add to `formulaJSON` after `Pinned`:

```go
	LinkedKeg  *string `json:"linked_keg"` // the linked version, or null
	KegOnly    bool    `json:"keg_only"`
```

In `formulaJSON.toPackage`, add to the `Package` literal:

```go
		Linked: deref(f.LinkedKeg) != "", KegOnly: f.KegOnly,
```

- [ ] **Step 4: Update the fixture.** In `internal/brew/testdata/info.json`, add these keys to each formula, before its `"pinned"`:
  - fd: `"linked_keg": "10.5.0", "keg_only": false,`
  - git: `"linked_keg": "2.51.0", "keg_only": false,`
  - pcre2: `"linked_keg": "10.49", "keg_only": false,`
  - json-c: `"linked_keg": null, "keg_only": true,`
  - lua-language-server: `"linked_keg": "3.19.1", "keg_only": false,`

  Casks get nothing.

- [ ] **Step 5: Run the brew tests.** `go test ./internal/brew -count=1`. Expected: PASS, including the existing `TestParseInfo`.

- [ ] **Step 6: Bump the snapshot version.** In `internal/state/snapshot.go`, set `const snapshotVersion = 5`.

- [ ] **Step 7: Write the failing details test.** Add to `internal/ui/installed_test.go` (add `"regexp"` to its imports if missing):

```go
func TestInstalledDetailsSayWhetherLinked(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		unlink     bool
	}{
		{"fd", `linked\s+yes`, false},
		{"fd", `linked\s+no\n`, true},
		{"json-c", `linked\s+no · keg-only`, false},
	} {
		a := newJobApp(t, &fakeStarter{})
		if tt.unlink {
			for i := range a.snap.Packages {
				if a.snap.Packages[i].Name == tt.name {
					a.snap.Packages[i].Linked = false
				}
			}
			a.sync()
		}
		selectInstalled(t, a, tt.name)
		if out := stripANSI(a.render()); !regexp.MustCompile(tt.want).MatchString(out) {
			t.Errorf("%s (unlinked %v): details lack %q:\n%s", tt.name, tt.unlink, tt.want, out)
		}
	}
	a := newJobApp(t, &fakeStarter{})
	selectInstalled(t, a, "fontbase")
	if out := stripANSI(a.render()); regexp.MustCompile(`linked\s+(yes|no)`).MatchString(out) {
		t.Errorf("a cask has no linked row:\n%s", out)
	}
}
```

- [ ] **Step 8: Run it.** `go test ./internal/ui -run TestInstalledDetailsSayWhetherLinked -count=1`. Expected: FAIL, as the details lack "linked".

- [ ] **Step 9: Add the row.** In `internal/ui/installed.go`, add a helper below the details function:

```go
// linkedText is the details' linked row for p: empty for a cask, which
// brew doesn't link.
func linkedText(p brew.Package) string {
	switch {
	case p.Kind == brew.KindCask:
		return ""
	case p.Linked:
		return "yes"
	case p.KegOnly:
		return "no · keg-only"
	}
	return "no"
}
```

and put it in the `kv` call after `"pinned", pinned,`:

```go
		"latest", latest, "pinned", pinned, "linked", linkedText(p), "deprecated", deprecated, "homepage", p.Homepage))
```

- [ ] **Step 10: Run the UI tests.** Run `go test ./internal/ui -count=1`. Expected: the new test passes, and `TestScreens` fails only on `installed.txt`.

- [ ] **Step 11: Update the goldens.**
  - Run `go test ./internal/ui -run TestScreens -update -count=1`.
  - Then run `git diff internal/ui/testdata/golden`. The only change must be one new `linked     yes` row in `installed.txt` (git's details).

- [ ] **Step 12: Check.** `go test ./... -count=1` and `golangci-lint run ./...` (`0 issues.`).

- [ ] **Step 13: Commit.** `git commit -am "feat(brew): read whether a formula is linked or keg-only"`. The fixture and golden are already tracked, so `-am` includes them.

---

### Task 2: Reinstall, link/unlink and fetch, with their keys

**Files:**
- Create: `internal/ui/reinstall.go`
- Modify: `internal/ui/actions.go` (`installedKey`, `outdatedKey`)
- Modify: `internal/ui/help.go` (the Installed and Outdated groups)
- Test: create `internal/ui/reinstall_test.go`; adjust `internal/ui/help_test.go` only if it asserts the old `space` texts

**Interfaces:**
- Consumes: `Package.Linked`, `Package.KegOnly` (Task 1).
- Consumes, existing in `internal/ui`:
  - `kindArgs(verb string, kind brew.Kind, names ...string) []string`;
  - `action{title, args, refresh}`, `a.run(action) tea.Cmd`, `a.note(string) tea.Cmd`;
  - `confirmBox{title, body, choices}`, `choice{key, label, run}`;
  - `target{name, kind}`, `a.byKind([]target, func(brew.Kind, []string) action) tea.Cmd`;
  - `a.installed.markedPackages()`, `a.outdated.markedIdx()`, `a.outdated.cursor()`, `a.marks()`, `a.clearMarks()`;
  - `diskQueries`, `qInstalled`, `qCleanup`.
- Produces, for Task 4: `R`, `L` and `f` on Installed and `f` on Outdated, all through `actionKey`.

- [ ] **Step 1: Write the failing tests.** Create `internal/ui/reinstall_test.go`:

```go
package ui

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestReinstallAsksThenRuns(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	selectInstalled(t, a, "fd")
	keys(t, a, "R")
	if a.confirm == nil || a.confirm.title != "Reinstall fd 10.5.0?" ||
		!reflect.DeepEqual(a.confirm.body, []string{"brew removes it and installs it again."}) {
		t.Fatalf("R should ask: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"reinstall", "fd"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestReinstallMarkedRunsOneJobPerKind(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	selectInstalled(t, a, "fd")
	keys(t, a, "space")
	selectInstalled(t, a, "docker-desktop")
	keys(t, a, "space", "R")
	if a.confirm == nil || a.confirm.title != "Reinstall 2 packages?" ||
		!reflect.DeepEqual(a.confirm.body, []string{"docker-desktop, fd"}) {
		t.Fatalf("R with marks should ask once: %+v", a.confirm)
	}
	keys(t, a, "y")
	want := [][]string{{"reinstall", "fd"}, {"reinstall", "--cask", "docker-desktop"}}
	if !reflect.DeepEqual(fs.calls, want) {
		t.Errorf("ran %v, want %v", fs.calls, want)
	}
	if a.marks() != 0 {
		t.Errorf("marks should clear: %d", a.marks())
	}
}

func TestLinkKey(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	selectInstalled(t, a, "fontbase")
	keys(t, a, "L")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "casks aren't linked") {
		t.Errorf("L on a cask: %v %q", fs.calls, stripANSI(a.statusBar()))
	}

	selectInstalled(t, a, "fd") // linked
	keys(t, a, "L")
	if a.confirm == nil || a.confirm.title != "Unlink fd?" {
		t.Fatalf("L on a linked formula should ask: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"unlink", "fd"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}

	selectInstalled(t, a, "json-c") // keg-only, unlinked
	keys(t, a, "L")
	if a.confirm == nil || a.confirm.title != "json-c is keg-only" {
		t.Fatalf("L on a keg-only formula should ask: %+v", a.confirm)
	}
	keys(t, a, "y") // not a choice here: nothing happens
	if a.confirm == nil {
		t.Fatal("y isn't a choice on the keg-only box")
	}
	keys(t, a, "f")
	if want := []string{"link", "--force", "json-c"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}

	for i := range a.snap.Packages {
		if a.snap.Packages[i].Name == "pcre2" {
			a.snap.Packages[i].Linked = false
		}
	}
	a.sync()
	selectInstalled(t, a, "pcre2") // unlinked, not keg-only
	keys(t, a, "L")
	if a.confirm != nil {
		t.Fatalf("linking shouldn't ask: %+v", a.confirm)
	}
	if want := []string{"link", "pcre2"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestFetchKey(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	selectInstalled(t, a, "fd")
	keys(t, a, "f")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "fd is up to date") {
		t.Errorf("f on an up-to-date package: %v %q", fs.calls, stripANSI(a.statusBar()))
	}
	selectInstalled(t, a, "lua-language-server")
	keys(t, a, "f")
	if want := []string{"fetch", "lua-language-server"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}

	selectInstalled(t, a, "fd")
	keys(t, a, "space", "f")
	if !strings.Contains(stripANSI(a.statusBar()), "nothing marked is outdated") {
		t.Errorf("f with only up-to-date marks: %q", stripANSI(a.statusBar()))
	}
	selectInstalled(t, a, "docker-desktop")
	keys(t, a, "space")
	selectInstalled(t, a, "lua-language-server")
	keys(t, a, "space")
	fs.calls = nil
	keys(t, a, "f")
	want := [][]string{{"fetch", "lua-language-server"}, {"fetch", "--cask", "docker-desktop"}}
	if !reflect.DeepEqual(fs.calls, want) {
		t.Errorf("ran %v, want %v", fs.calls, want)
	}
	if a.marks() != 3 {
		t.Errorf("fetch keeps the marks: %d", a.marks())
	}

	fs.calls = nil
	selectOutdated(t, a, "docker-desktop")
	keys(t, a, "f")
	if want := []string{"fetch", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("Outdated: ran %v, want %v", lastCall(fs), want)
	}
}

// refreshedBy runs keys on Installed, with name selected, against a
// counting source, and reports how often each query ran afterwards.
func refreshedBy(t *testing.T, name string, ks ...string) map[string]int {
	t.Helper()
	src := newCountingSource(t)
	fs := &fakeStarter{}
	a := New(src, fs.start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	selectInstalled(t, a, name)
	clear(src.calls)
	keys(t, a, ks...)
	return src.calls
}

func TestNewActionsRefreshWhatTheyChange(t *testing.T) {
	if got := refreshedBy(t, "lua-language-server", "f"); got["cleanup"] == 0 || got["installed"] != 0 {
		t.Errorf("fetch should measure cleanup again, and only that: %v", got)
	}
	if got := refreshedBy(t, "fd", "L", "y"); got["installed"] == 0 || got["outdated"] != 0 {
		t.Errorf("unlink should read Installed again, and only that: %v", got)
	}
	if got := refreshedBy(t, "fd", "R", "y"); got["installed"] == 0 || got["outdated"] == 0 || got["sizes"] == 0 {
		t.Errorf("reinstall should refresh what's on disk: %v", got)
	}
}
```

If `selectOutdated` is unexported in another test file, it's already in package `ui` (`pinned_test.go`).

- [ ] **Step 2: Run them.** `go test ./internal/ui -run 'Reinstall|LinkKey|FetchKey|NewActionsRefresh' -count=1`. Expected: FAIL, as R, L and f do nothing yet.

- [ ] **Step 3: Write `internal/ui/reinstall.go`:**

```go
package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
)

// askReinstall confirms reinstalling the marked packages, or else the
// selected one.
func (a *App) askReinstall() {
	if a.marks() > 0 {
		a.askReinstallMarked()
		return
	}
	p, ok := a.installed.selected()
	if !ok {
		return
	}
	act := action{title: "reinstall " + p.Name, args: kindArgs("reinstall", p.Kind, p.FullName), refresh: diskQueries}
	a.confirm = &confirmBox{
		title: fmt.Sprintf("Reinstall %s %s?", p.Name, p.Version),
		body:  []string{"brew removes it and installs it again."},
		choices: []choice{
			{"y", "reinstall", func() tea.Cmd { return a.run(act) }},
			{"n", "cancel", nil},
		},
	}
}

// askReinstallMarked confirms reinstalling the marked packages: one job
// per kind, formulae first. The marks clear once it starts.
func (a *App) askReinstallMarked() {
	pkgs := a.installed.markedPackages()
	names := make([]string, len(pkgs))
	ts := make([]target, len(pkgs))
	for i, p := range pkgs {
		names[i], ts[i] = p.Name, target{p.FullName, p.Kind}
	}
	run := func() tea.Cmd {
		a.clearMarks()
		return a.byKind(ts, func(kind brew.Kind, names []string) action {
			return action{title: "reinstall " + strings.Join(names, " "), args: kindArgs("reinstall", kind, names...), refresh: diskQueries}
		})
	}
	a.confirm = &confirmBox{
		title:   fmt.Sprintf("Reinstall %d packages?", len(pkgs)),
		body:    []string{strings.Join(names, ", ")},
		choices: []choice{{"y", "reinstall", run}, {"n", "cancel", nil}},
	}
}

// linkKey links the selected formula, or unlinks it if it's linked.
// Unlinking asks first, and so does forcing a keg-only formula's link.
func (a *App) linkKey() tea.Cmd {
	p, ok := a.installed.selected()
	switch {
	case !ok:
		return nil
	case p.Kind == brew.KindCask:
		return a.note("casks aren't linked")
	case p.Linked:
		act := action{title: "unlink " + p.Name, args: []string{"unlink", p.FullName}, refresh: []query{qInstalled}}
		a.confirm = &confirmBox{
			title: "Unlink " + p.Name + "?",
			body:  []string{"Its commands leave your PATH until you link it again."},
			choices: []choice{
				{"y", "unlink", func() tea.Cmd { return a.run(act) }},
				{"n", "cancel", nil},
			},
		}
		return nil
	case p.KegOnly:
		act := action{title: "link --force " + p.Name, args: []string{"link", "--force", p.FullName}, refresh: []query{qInstalled}}
		a.confirm = &confirmBox{
			title: p.Name + " is keg-only",
			body:  []string{"brew won't link it unless forced, as it", "can clash with macOS or another formula."},
			choices: []choice{
				{"f", "link --force", func() tea.Cmd { return a.run(act) }},
				{"n", "cancel", nil},
			},
		}
		return nil
	}
	return a.run(action{title: "link " + p.Name, args: []string{"link", p.FullName}, refresh: []query{qInstalled}})
}

// fetchAction downloads the newer version of names, all of kind. The
// download cache grows, so Cleanup's dry run is measured again.
func fetchAction(kind brew.Kind, names []string) action {
	return action{title: "fetch " + strings.Join(names, " "), args: kindArgs("fetch", kind, names...), refresh: []query{qCleanup}}
}

// fetchKey downloads the newer version of the marked packages, or else of
// the selected one, so a later upgrade needn't wait for it.
func (a *App) fetchKey() tea.Cmd {
	if a.marks() > 0 {
		return a.fetchMarked()
	}
	switch a.tab {
	case tabInstalled:
		p, ok := a.installed.selected()
		if !ok {
			return nil
		}
		if !p.Outdated {
			return a.note(p.Name + " is up to date")
		}
		return a.run(fetchAction(p.Kind, []string{p.FullName}))
	case tabOutdated:
		if i := a.outdated.cursor(); i >= 0 {
			o := a.snap.Outdated[i]
			return a.run(fetchAction(o.Kind, []string{o.Name}))
		}
	}
	return nil
}

// fetchMarked downloads the newer version of every marked package that is
// outdated, pinned or not, as a download upgrades nothing. The marks stay,
// so u can follow on the same set.
func (a *App) fetchMarked() tea.Cmd {
	var ts []target
	if a.tab == tabInstalled {
		for _, p := range a.installed.markedPackages() {
			if p.Outdated {
				ts = append(ts, target{p.FullName, p.Kind})
			}
		}
	} else {
		for _, i := range a.outdated.markedIdx() {
			o := a.snap.Outdated[i]
			ts = append(ts, target{o.Name, o.Kind})
		}
	}
	if len(ts) == 0 {
		return a.note("nothing marked is outdated")
	}
	return a.byKind(ts, fetchAction)
}
```

The keg-only body is split over two lines because the confirm box truncates lines at 56 columns; the spec's sentence is unchanged.

- [ ] **Step 4: Route the keys.** In `internal/ui/actions.go`:

In `installedKey`, add these cases to its switch, after `p`:

```go
	case "R":
		a.askReinstall()
		return nil, true
	case "L":
		return a.linkKey(), true
	case "f":
		return a.fetchKey(), true
```

Replace `outdatedKey`'s body:

```go
	switch k {
	case "A":
		return a.askUpgradeAll(), true
	case "f":
		return a.fetchKey(), true
	}
	return a.markKey(k)
```

- [ ] **Step 5: Update help.** In `internal/ui/help.go`, set the Installed and Outdated groups to:

```go
	case a.tab == tabInstalled:
		here = helpGroup{"Installed", [][2]string{
			{"u", "upgrade"}, {"x", "uninstall"}, {"p", "pin or unpin"},
			{"R", "reinstall"}, {"L", "link or unlink"}, {"f", "fetch the newer version"},
			{"space", "mark for several"}, {"l tab enter", "details, then j/k scroll"},
		}}
	case a.tab == tabOutdated:
		here = helpGroup{"Outdated", [][2]string{
			{"u", "upgrade"}, {"f", "fetch the newer version"}, {"A", "upgrade all"}, {"space", "mark for several"},
		}}
```

(`l tab enter` changes in Task 4.)

- [ ] **Step 6: Run the tests.**
  - `go test ./internal/ui -count=1`. Expected: PASS.
  - If `help_test.go` asserts the old `mark for u/x on several` text, update it to `mark for several`.
  - If the help golden (`help.txt`) changes, check that the diff is only these rows. Then update it with `go test ./internal/ui -run TestScreens -update -count=1`.
  - If `TestNewActionsRefreshWhatTheyChange` fails only because a follow-up query also ran, look at what the refresh set triggers (`handleResult`) before changing an assertion. The spec's refresh sets are what must hold: `fetch` → `[qCleanup]`, link/unlink → `[qInstalled]`, reinstall → `diskQueries`.

- [ ] **Step 7: Check.** `go test ./... -count=1`, `golangci-lint run ./...` (`0 issues.`), `make cover`.

- [ ] **Step 8: Commit.** `git add -A internal/ui && git commit -m "feat(ui): reinstall, link or unlink, and fetch from Installed and Outdated"`

---

### Task 3: The menu box

**Files:**
- Create: `internal/ui/menu.go`
- Test: create `internal/ui/menu_test.go`

**Interfaces:**
- Produces, for Task 4:
  - `type menuItem struct{ key, label string; run func() tea.Cmd }`
  - `type menuBox struct{ title string; items []menuItem; cursor int; gPending bool }`
  - `func (m *menuBox) key(k string) (run func() tea.Cmd, done bool)`: done means close the menu; run is what to do, or nil.
  - `func (m *menuBox) view(st theme.Styles, maxW int) string`
  - `const menuFooter = "j/k move · enter run · esc close"`

- [ ] **Step 1: Write the failing tests.** Create `internal/ui/menu_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// testMenu is a menu whose items record which one ran.
func testMenu(ran *string) *menuBox {
	item := func(key, label string) menuItem {
		return menuItem{key, label, func() tea.Cmd { *ran = key; return nil }}
	}
	return &menuBox{title: "fd 10.5.0", items: []menuItem{
		item("u", "Upgrade to 10.6.0"), item("x", "Uninstall…"), item("gx", "Open the homepage"),
	}}
}

func TestMenuKeys(t *testing.T) {
	var ran string
	pick := func(m *menuBox, ks ...string) (done bool) {
		for _, k := range ks {
			run, d := m.key(k)
			if run != nil {
				run()
			}
			done = d
		}
		return done
	}

	m := testMenu(&ran)
	if done := pick(m, "k"); done || m.cursor != 2 {
		t.Errorf("k from the top wraps to the last: cursor %d, done %v", m.cursor, done)
	}
	if done := pick(m, "j", "down"); done || m.cursor != 1 {
		t.Errorf("j and down move down, wrapping: cursor %d", m.cursor)
	}
	if done := pick(m, "up"); done || m.cursor != 0 {
		t.Errorf("up moves up: cursor %d", m.cursor)
	}
	if done := pick(m, "j", "enter"); !done || ran != "x" {
		t.Errorf("enter runs the item under the cursor: ran %q, done %v", ran, done)
	}

	for _, tt := range []struct {
		keys []string
		ran  string
		done bool
	}{
		{[]string{"u"}, "u", true},
		{[]string{"x"}, "x", true},
		{[]string{"g", "x"}, "gx", true},
		{[]string{"g", "u"}, "", false}, // g then anything but x does nothing
		{[]string{"esc"}, "", true},
		{[]string{"q"}, "", true},
		{[]string{"z"}, "", false},
	} {
		ran = ""
		if done := pick(testMenu(&ran), tt.keys...); done != tt.done || ran != tt.ran {
			t.Errorf("%v: ran %q, done %v; want %q, %v", tt.keys, ran, done, tt.ran, tt.done)
		}
	}
}

func TestMenuView(t *testing.T) {
	var ran string
	m := testMenu(&ran)
	m.cursor = 1
	out := stripANSI(m.view(testStyles, 140))
	for _, want := range []string{"fd 10.5.0", "  Upgrade to 10.6.0", "› Uninstall…", "Open the homepage", "gx", menuFooter} {
		if !strings.Contains(out, want) {
			t.Errorf("menu lacks %q:\n%s", want, out)
		}
	}
	lines := strings.Split(out, "\n")
	for _, l := range lines[1 : len(lines)-1] {
		if w := len([]rune(l)); w != len([]rune(lines[0])) {
			t.Errorf("ragged box, line %q is %d wide, want %d:\n%s", l, w, len([]rune(lines[0])), out)
		}
	}
	for _, l := range lines {
		if strings.Contains(l, "Uninstall…") && !strings.HasSuffix(strings.TrimRight(l, " "), "x │") {
			t.Errorf("the key sits at the right of its item: %q", l)
		}
	}
	if narrow := stripANSI(m.view(testStyles, 24)); strings.Contains(narrow, "Upgrade to 10.6.0") {
		t.Errorf("a narrow menu truncates its labels:\n%s", narrow)
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/ui -run TestMenu -count=1`. Expected: it fails to compile (`menuBox` undefined).

- [ ] **Step 3: Write `internal/ui/menu.go`:**

```go
package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/theme"
)

// menuItem is one thing the action menu offers: the key that does the
// same outside the menu, a label, and what it does.
type menuItem struct {
	key   string
	label string
	run   func() tea.Cmd
}

// menuBox lists what can be done to the selected row, or to the marked
// rows. enter, or an item's key, runs it.
type menuBox struct {
	title    string
	items    []menuItem
	cursor   int
	gPending bool // g was pressed: x next picks gx
}

// menuFooter says how to use the menu.
const menuFooter = "j/k move · enter run · esc close"

// key handles a key press while the menu is open. done is true when the
// menu should close; run is what was picked, or nil.
func (m *menuBox) key(k string) (run func() tea.Cmd, done bool) {
	if m.gPending {
		m.gPending = false
		if k == "x" {
			return m.pick("gx")
		}
		return nil, false
	}
	switch k {
	case "esc", "q":
		return nil, true
	case "j", "down":
		m.cursor = (m.cursor + 1) % len(m.items)
	case "k", "up":
		m.cursor = (m.cursor + len(m.items) - 1) % len(m.items)
	case "enter":
		return m.items[m.cursor].run, true
	case "g":
		m.gPending = true
	default:
		return m.pick(k)
	}
	return nil, false
}

// pick finds the item whose key is k; done is false when there is none.
func (m *menuBox) pick(k string) (run func() tea.Cmd, done bool) {
	for _, it := range m.items {
		if it.key == k {
			return it.run, true
		}
	}
	return nil, false
}

// view renders the menu, at most maxW columns wide: the title, the items
// with their keys at the right, and how to use it.
func (m *menuBox) view(st theme.Styles, maxW int) string {
	labelW, keyW := 0, 0
	for _, it := range m.items {
		labelW = max(labelW, lipgloss.Width(it.label))
		keyW = max(keyW, lipgloss.Width(it.key))
	}
	inner := min(max(2+labelW+2+keyW, lipgloss.Width(menuFooter)), 56, max(maxW-4, 10))
	lines := []string{st.Title.Render(ansi.Truncate(m.title, inner, "…")), ""}
	for i, it := range m.items {
		label := ansi.Truncate(it.label, max(inner-2-2-keyW, 1), "…")
		gap := strings.Repeat(" ", max(inner-2-lipgloss.Width(label)-lipgloss.Width(it.key), 1))
		mark := "  "
		if i == m.cursor {
			mark, label = st.Accent.Render("› "), st.Accent.Bold(true).Render(label)
		}
		lines = append(lines, mark+label+gap+st.Muted.Render(it.key))
	}
	lines = append(lines, "", st.Muted.Render(ansi.Truncate(menuFooter, inner, "…")))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(st.P.Mauve).
		Padding(0, 1).Render(strings.Join(lines, "\n"))
}
```

The title is the box's first line, as in the confirm box, rather than set into the border as the spec's sketch draws it.

- [ ] **Step 4: Run them.** `go test ./internal/ui -run TestMenu -count=1`. Expected: PASS. If the box-width assertion trips only on the title or footer line, check `inner` rather than loosening the test: every line must end at the border.

- [ ] **Step 5: Check.** `go test ./internal/ui -count=1`, `golangci-lint run ./...` (`0 issues.`). The `unused` linter may flag `menuBox` until Task 4 uses it. If it does, note it in the report and commit anyway; Task 4 clears it.

- [ ] **Step 6: Commit.** `git add internal/ui/menu.go internal/ui/menu_test.go && git commit -m "feat(ui): an action menu box"`

---

### Task 4: `enter` opens the menu on every tab with actions

**Files:**
- Modify: `internal/ui/menu.go` (add `openMenu`, the per-tab item builders, `keyItem`, `copyItems`, `linkLabel`, `hasMenu`)
- Modify: `internal/ui/app.go`:
  - add `menu *menuBox` to `App`, next to `confirm`;
  - `modalKey`;
  - `tabKey` and its doc comment;
  - `render`.
- Modify: `internal/ui/actions.go` (`actionHints`)
- Modify: `internal/ui/help.go`
- Modify: `internal/ui/golden_test.go` (one new screen)
- Modify: tests that press `enter` to reach the details (see Step 7)
- Test: `internal/ui/menu_test.go`

**Interfaces:**
- Consumes:
  - from Task 3, `menuBox`, `menuItem` and `menuFooter`;
  - from Task 2, `R`, `L` and `f` in `actionKey`;
  - existing: `a.actionKey(k)`, `a.copyName()`, `a.openHomepage()`, `a.selection() (name, url string, ok bool)`, `webPage(url)`, `a.selectedService()`, `a.selectedTap()`, `a.discover.selected()`, `a.jobs.cursor()`, `a.history`, `a.outdated.cursor()`, `a.snap.Outdated` (`brew.Outdated{Name, Kind, Installed, Current, Pinned}`).

- [ ] **Step 1: Write the failing tests.** Append to `internal/ui/menu_test.go` (merge the imports: it needs `"reflect"` and `"github.com/dillo/brewery/internal/jobs"` too):

```go
// menuLabels opens the menu with enter and lists its title and labels;
// it closes the menu again.
func menuLabels(t *testing.T, a *App) (title string, labels []string) {
	t.Helper()
	keys(t, a, "enter")
	if a.menu == nil {
		t.Fatal("enter didn't open the menu")
	}
	for _, it := range a.menu.items {
		labels = append(labels, it.label)
	}
	title = a.menu.title
	keys(t, a, "esc")
	return title, labels
}

func TestMenuItemsOnInstalled(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	for _, tt := range []struct {
		name, title string
		want        []string
	}{
		{"lua-language-server", "lua-language-server 3.19.1", []string{"Upgrade to 3.19.2", "Fetch 3.19.2", "Reinstall…", "Unlink…", "Pin", "Uninstall…", "Copy the name", "Open the homepage"}},
		{"fd", "fd 10.5.0", []string{"Reinstall…", "Unlink…", "Pin", "Uninstall…", "Copy the name", "Open the homepage"}},
		{"git", "git 2.51.0", []string{"Reinstall…", "Unlink…", "Unpin", "Uninstall…", "Copy the name", "Open the homepage"}},
		{"json-c", "json-c 0.19", []string{"Reinstall…", "Link…", "Pin", "Uninstall…", "Copy the name", "Open the homepage"}},
		{"docker-desktop", "docker-desktop 4.39.0,184744", []string{"Upgrade to 4.95.0,242831", "Fetch 4.95.0,242831", "Reinstall…", "Pin", "Uninstall…", "Copy the name", "Open the homepage"}},
	} {
		selectInstalled(t, a, tt.name)
		if title, got := menuLabels(t, a); title != tt.title || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: %q %q, want %q %q", tt.name, title, got, tt.title, tt.want)
		}
	}
	selectInstalled(t, a, "fd")
	keys(t, a, "space")
	selectInstalled(t, a, "git")
	keys(t, a, "space")
	if title, got := menuLabels(t, a); title != "2 marked" || !reflect.DeepEqual(got, []string{"Upgrade", "Fetch", "Reinstall…", "Uninstall…"}) {
		t.Errorf("marked: %q %q", title, got)
	}
}

func TestMenuItemsOnOtherTabs(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	selectOutdated(t, a, "lua-language-server")
	if title, got := menuLabels(t, a); title != "lua-language-server 3.19.1" ||
		!reflect.DeepEqual(got, []string{"Upgrade to 3.19.2", "Fetch 3.19.2", "Upgrade all…", "Copy the name", "Open the homepage"}) {
		t.Errorf("Outdated: %q %q", title, got)
	}
	keys(t, a, "space")
	if title, got := menuLabels(t, a); title != "1 marked" || !reflect.DeepEqual(got, []string{"Upgrade", "Fetch"}) {
		t.Errorf("Outdated, marked: %q %q", title, got)
	}

	show(t, a, "Services")
	keys(t, a, "g", "g") // postgresql@16, in error
	if title, got := menuLabels(t, a); title != "postgresql@16" || !reflect.DeepEqual(got, []string{"Start", "Follow the log", "Copy the name"}) {
		t.Errorf("Services, stopped: %q %q", title, got)
	}
	keys(t, a, "j") // redis, started
	if _, got := menuLabels(t, a); !reflect.DeepEqual(got, []string{"Stop…", "Restart", "Follow the log", "Copy the name"}) {
		t.Errorf("Services, started: %q", got)
	}

	show(t, a, "Taps")
	keys(t, a, "g", "g") // anomalyco/tap, on GitHub
	if title, got := menuLabels(t, a); title != "anomalyco/tap" || !reflect.DeepEqual(got, []string{"Untap…", "Copy the name", "Open the homepage"}) {
		t.Errorf("Taps: %q %q", title, got)
	}

	a.UseHistory(jobs.History{Dir: t.TempDir()}, recentJobs(a.now()))
	show(t, a, "History")
	keys(t, a, "g", "g")
	if title, got := menuLabels(t, a); title != "brew upgrade fd" || !reflect.DeepEqual(got, []string{"Run again…", "Copy the command"}) {
		t.Errorf("History: %q %q", title, got)
	}

	a.UseCatalog(newFakeCatalog(t))
	show(t, a, "Search all")
	if _, got := menuLabels(t, a); len(got) < 2 || got[0] != "Install…" || got[1] != "Copy the name" {
		t.Errorf("Discover: %q", got)
	}
}

func TestEnterOpensTheMenuNotTheDetails(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	selectInstalled(t, a, "fd")
	keys(t, a, "enter")
	if a.menu == nil || a.focus != colList {
		t.Fatalf("enter should open the menu and leave the focus: menu %v, focus %v", a.menu, a.focus)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "› Reinstall…") || !strings.Contains(stripANSI(a.statusBar()), menuFooter) {
		t.Errorf("the menu should show, with its hint:\n%s", out)
	}
	keys(t, a, "x") // any key goes to the menu while it's open: x picks Uninstall…
	if a.menu != nil || a.confirm == nil || a.confirm.title != "Uninstall fd 10.5.0?" {
		t.Fatalf("x in the menu should close it and ask: menu %v, confirm %+v", a.menu, a.confirm)
	}
	keys(t, a, "n")

	keys(t, a, "l", "enter") // from the details too
	if a.menu == nil || a.focus != colDetails {
		t.Errorf("enter in the details should open the menu: menu %v, focus %v", a.menu, a.focus)
	}
	keys(t, a, "esc", "esc")
	keys(t, a, "tab")
	if a.focus != colDetails {
		t.Errorf("tab still opens the details: focus %v", a.focus)
	}

	show(t, a, "Maintenance")
	keys(t, a, "enter")
	if a.menu != nil {
		t.Errorf("Maintenance's enter runs its row, with no menu: %+v", a.menu)
	}
}

func TestMenuNeedsARow(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.snap.Outdated = nil
	a.sync()
	show(t, a, "Outdated")
	keys(t, a, "enter")
	if a.menu != nil {
		t.Errorf("an empty list has no menu: %+v", a.menu)
	}
}
```

If "Search all" doesn't load the catalogue in this app, copy how `TestScreens` reaches the `discover-all` screen (it shows "Popular tools" and then "Search all"). 

- [ ] **Step 2: Run them.** `go test ./internal/ui -run 'TestMenuItems|TestEnterOpens|TestMenuNeedsARow' -count=1`. Expected: it fails to compile (`a.menu` undefined).

- [ ] **Step 3: Build the menus.** Append to `internal/ui/menu.go`. It now imports `fmt` and `github.com/dillo/brewery/internal/brew` too.

```go
// hasMenu reports whether tab's rows have an action menu. Maintenance's
// rows are each a single action, so enter runs them instead.
func hasMenu(tab int) bool {
	switch tab {
	case tabInstalled, tabOutdated, tabServices, tabTaps, tabDiscover, tabHistory:
		return true
	}
	return false
}

// openMenu opens the action menu for the current tab's selection, or for
// its marked rows. It does nothing when there's no row to act on.
func (a *App) openMenu() {
	var m *menuBox
	switch a.tab {
	case tabInstalled:
		m = a.installedMenu()
	case tabOutdated:
		m = a.outdatedMenu()
	case tabServices:
		m = a.servicesMenu()
	case tabTaps:
		m = a.tapsMenu()
	case tabDiscover:
		m = a.discoverMenu()
	case tabHistory:
		m = a.historyMenu()
	}
	if m != nil && len(m.items) > 0 {
		a.menu = m
	}
}

// keyItem is a menu item that does what pressing k does on this tab, so
// the menu and the keys always agree.
func (a *App) keyItem(k, label string) menuItem {
	run := func() tea.Cmd {
		cmd, _ := a.actionKey(k)
		return cmd
	}
	switch k {
	case "y":
		run = a.copyName
	case "gx":
		run = a.openHomepage
	}
	return menuItem{k, label, run}
}

// copyItems end every menu: copying the name (or what copyLabel says),
// and opening the web page when there is one.
func (a *App) copyItems(copyLabel string) []menuItem {
	items := []menuItem{a.keyItem("y", copyLabel)}
	if _, url, ok := a.selection(); ok && webPage(url) {
		items = append(items, a.keyItem("gx", "Open the homepage"))
	}
	return items
}

// markedMenu is the menu for n marked rows: only what works on several.
func (a *App) markedMenu(n int, items ...menuItem) *menuBox {
	return &menuBox{title: fmt.Sprintf("%d marked", n), items: items}
}

// installedMenu is the menu for the selected installed package.
func (a *App) installedMenu() *menuBox {
	if n := a.marks(); n > 0 {
		return a.markedMenu(n, a.keyItem("u", "Upgrade"), a.keyItem("f", "Fetch"),
			a.keyItem("R", "Reinstall…"), a.keyItem("x", "Uninstall…"))
	}
	p, ok := a.installed.selected()
	if !ok {
		return nil
	}
	var items []menuItem
	if p.Outdated && !p.Pinned {
		items = append(items, a.keyItem("u", "Upgrade to "+p.Latest))
	}
	if p.Outdated {
		items = append(items, a.keyItem("f", "Fetch "+p.Latest))
	}
	items = append(items, a.keyItem("R", "Reinstall…"))
	if p.Kind == brew.KindFormula {
		items = append(items, a.keyItem("L", linkLabel(p)))
	}
	pin := "Pin"
	if p.Pinned {
		pin = "Unpin"
	}
	items = append(items, a.keyItem("p", pin), a.keyItem("x", "Uninstall…"))
	return &menuBox{title: p.Name + " " + p.Version, items: append(items, a.copyItems("Copy the name")...)}
}

// linkLabel is what L does to the formula p; … where it asks first.
func linkLabel(p brew.Package) string {
	switch {
	case p.Linked:
		return "Unlink…"
	case p.KegOnly:
		return "Link…"
	}
	return "Link"
}

// outdatedMenu is the menu for the selected outdated package.
func (a *App) outdatedMenu() *menuBox {
	if n := a.marks(); n > 0 {
		return a.markedMenu(n, a.keyItem("u", "Upgrade"), a.keyItem("f", "Fetch"))
	}
	i := a.outdated.cursor()
	if i < 0 {
		return nil
	}
	o := a.snap.Outdated[i]
	var items []menuItem
	if !o.Pinned {
		items = append(items, a.keyItem("u", "Upgrade to "+o.Current))
	}
	items = append(items, a.keyItem("f", "Fetch "+o.Current), a.keyItem("A", "Upgrade all…"))
	return &menuBox{title: o.Name + " " + o.Installed, items: append(items, a.copyItems("Copy the name")...)}
}

// servicesMenu is the menu for the selected service.
func (a *App) servicesMenu() *menuBox {
	s, ok := a.selectedService()
	if !ok {
		return nil
	}
	var items []menuItem
	if s.Status == "started" {
		items = append(items, a.keyItem("S", "Stop…"), a.keyItem("R", "Restart"))
	} else {
		items = append(items, a.keyItem("s", "Start"))
	}
	if s.Log != "" || s.ErrorLog != "" {
		items = append(items, a.keyItem("L", "Follow the log"))
	}
	return &menuBox{title: s.Name, items: append(items, a.copyItems("Copy the name")...)}
}

// tapsMenu is the menu for the selected tap.
func (a *App) tapsMenu() *menuBox {
	t, ok := a.selectedTap()
	if !ok {
		return nil
	}
	return &menuBox{title: t.Name, items: append([]menuItem{a.keyItem("x", "Untap…")}, a.copyItems("Copy the name")...)}
}

// discoverMenu is the menu for the selected catalogue entry.
func (a *App) discoverMenu() *menuBox {
	r, ok := a.discover.selected()
	if !ok {
		return nil
	}
	return &menuBox{title: r.Name, items: append([]menuItem{a.keyItem("a", "Install…")}, a.copyItems("Copy the name")...)}
}

// historyMenu is the menu for the selected job.
func (a *App) historyMenu() *menuBox {
	i := a.jobs.cursor()
	if i < 0 {
		return nil
	}
	r := a.history[i]
	var items []menuItem
	if len(r.Args) > 0 {
		items = append(items, a.keyItem("R", "Run again…"))
	}
	return &menuBox{title: "brew " + strings.Join(r.Args, " "), items: append(items, a.copyItems("Copy the command")...)}
}
```

Notes:
- If `brew.Outdated.Installed` isn't a string in `internal/brew/types.go`, use whatever field holds the installed version as text.
- `markedMenu` doesn't use its receiver. If `unparam` or `revive` complains, make it a plain function.

- [ ] **Step 4: Wire it into the App.** In `internal/ui/app.go`:

Add to `App`, after `confirm`:

```go
	menu       *menuBox        // open action menu, or nil
```

In `modalKey`, add a case right after the `a.confirm != nil` case:

```go
	case a.menu != nil:
		run, done := a.menu.key(k)
		if done {
			a.menu = nil // closed before run, which may open a confirmation
		}
		if run == nil {
			return nil, true
		}
		return run(), true
```

Replace `tabKey`'s doc comment and its first two cases:

```go
// tabKey gives the key to the current tab: the list's keys move its
// selection, and once the details have the focus, keys scroll them. tab
// goes from the list to the details and back. enter opens the action
// menu, from either column; on Maintenance it runs the selected row.
func (a *App) tabKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	switch {
	case a.tab == tabMaintenance && k == "enter":
		return a.maintRun()
	case k == "enter" && hasMenu(a.tab):
		a.openMenu()
		return nil
	case a.focus == colList && k == "tab" && a.hasDetails():
		a.focus = colDetails
		return nil
```

(The remaining cases stay as they are.)

In `render`:
- Make the small-window guard `if bodyH == 0 && (a.confirm != nil || a.menu != nil || a.help) {`.
- Right after the `a.confirm` overlay, add:

```go
	if a.menu != nil {
		body = overlay(body, a.menu.view(a.st, a.width), a.width, bodyH)
	}
```

If `render` goes over a complexity limit, move the overlays into a helper `overlays(body string, bodyH int) string`.

- [ ] **Step 5: Update the hints.** In `internal/ui/actions.go`:
  - Start `actionHints` with this, before the sidebar check:

```go
	if a.menu != nil {
		return menuFooter
	}
```

  - Set these cases' texts exactly:

| Case | Text |
|---|---|
| Installed, details | `"enter actions · j/k scroll · h back to the list"` |
| Installed, list | `"enter actions · space mark · l details"` |
| Outdated | `"enter actions · space mark · A upgrade all"` |
| Services, details (`hint :=`) | `"enter actions · h back to the list"`, still prefixed with `"j/k scroll the log · "` when the log scrolls |
| Services, list | `"enter actions · l details"` |
| Taps | `"enter actions"` |
| Discover, details | `"enter actions · j/k scroll · h back to the list"` |
| Discover, list | `"enter actions · / search · l details · r refetch"` |
| History, details | `"enter actions · j/k scroll · h back to the list"` |
| History, list | `"enter actions · l details · / filter by command"` |

  - Maintenance, Home and the sidebar are unchanged.
  - If `actionHints` goes over a complexity limit, split the per-tab texts into a helper such as `listHints()`.

- [ ] **Step 6: Update help.** In `internal/ui/help.go`:
  - Each of the six tabs' groups starts with `{"enter", "actions"}`.
  - Every `{"l tab enter", …}` becomes `{"l tab", …}`.
  - Taps becomes `{{"enter", "actions"}, {"x", "untap"}}`.
  - Outdated, for example:

```go
		here = helpGroup{"Outdated", [][2]string{
			{"enter", "actions"}, {"u", "upgrade"}, {"f", "fetch the newer version"}, {"A", "upgrade all"}, {"space", "mark for several"},
		}}
```

  - Maintenance, Home and the sidebar groups are unchanged.

- [ ] **Step 7: Fix the tests that used `enter` for the details.**
  - Run `go test ./internal/ui -count=1`. Tests that pressed `enter` to focus the details on the six menu tabs now fail.
  - Candidates, from `grep -n '"enter"' internal/ui/*_test.go`: `app_test.go:309`, `columns_test.go:139,158`, `discover_test.go:218`, `historytab_test.go:86,168`, `scroll_test.go:61`, `marks_test.go:108,142,178`, `help_test.go:57,87,153`, `links_test.go:110`, `filter_test.go`, `install_test.go:204` and `sidebar_test.go:187`.
  - Many of these `enter`s close a `/` filter, a `:` line, a confirm box or the sidebar. Those don't change; leave them.
  - Change only an `enter` that was meant to move from a list to its details, and change it to `l`. Keep each test's intent. Where a test checked that `enter` opens the details, make it check that `l` or `tab` does.
  - Update `TestStatusHintsFollowTheTab` and `help_test.go` to the new texts where they assert old ones.

- [ ] **Step 8: Add the menu golden.** In `internal/ui/golden_test.go`'s screen list, add this as the **last** entry:

```go
		{"installed-menu", []string{"All"}, []string{"g", "g", "j", "j", "j", "j", "enter"}}, // lua-language-server's menu
```

  - Adjust the `j` count so the menu's title is `lua-language-server 3.19.1`; check the rendered file.
  - Then run `go test ./internal/ui -run TestScreens -update -count=1`.
  - Read `git diff internal/ui/testdata/golden`. Expected changes:
    - the new `installed-menu.txt`;
    - the hint line on the existing screens;
    - the help screen.
  - Nothing else may change.

- [ ] **Step 9: Check.** `go test ./... -count=1`, `golangci-lint run ./...` (`0 issues.`), `make cover` (≥ 90% total and per package).

- [ ] **Step 10: Screenshots.** If `make screenshots` exists and its outputs are tracked, run `make screenshots FREEZE="go run github.com/charmbracelet/freeze@v0.2.2"` and include changed images. If it fails for want of a tool, say so in the report and skip it.

- [ ] **Step 11: Commit.** `git add -A internal/ui docs && git commit -m "feat(ui): enter opens an action menu on every list with actions"`
