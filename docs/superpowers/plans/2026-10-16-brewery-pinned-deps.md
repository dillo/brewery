# brewery Plan 7¾: Pinned dependencies — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Say when a pinned, outdated dependency will make brew refuse an install or upgrade, before a job starts and fails.
- **Installed preview:** pinned dependencies are marked in the tree, with their versions when they're behind, and a warning line follows.
- **Upgrades:** `u` refuses with a reason; marked upgrades leave blocked formulae out and say so; the upgrade-all dialog names what brew will refuse.
- **Discover:** the install dialog becomes "Can't install …" with only `n close`.

**Architecture:**
- **`internal/state`:** `Index.PinnedBehind(names...)`, and `Index.Tree` takes a label function for dependency names.
- **`internal/ui`:** `App.pinnedBehind(name, kind)` finds an installed formula's blockers; the preview, `u`, marked upgrades, the upgrade-all dialog and the install dialog use it or `PinnedBehind` directly.

**Tech Stack:** Go ≥ 1.25; Bubble Tea v2, Lip Gloss v2. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-16-brewery-pinned-deps-design.md`, all of it.

**Found while building this:**
- **Upgrade all is `A` on Outdated**, not `U` (`U` is `brew update`). The spec is corrected.
- **Test refreshes reload the fake source.** Once a job finishes, brewery refreshes from its Source, so a test that only edits `a.snap` loses its setup. `pinnedApp` applies the same changes to the fake source.
- **A flash after starting a job shows while it runs.** The skipped-upgrade note is set after the job starts, and stays until the job's own ✓ or ✗ replaces it.

**Verified:** every code block was compiled and tested in a prototype (branch `proto/pinned-deps`) before the plan was written. Each task's new tests fail before its code (Task 1 by compile error: `PinnedBehind` and the new `Tree` signature don't exist). `go test -race ./...`, `go vet`, `gofmt -l` are clean at the end. Rendered with icons, git's preview reads `└─ pcre2 󰐃 pinned · 10.49 → 10.50` then `⚠ upgrading needs pcre2 unpinned`; `u` flashes `git needs pcre2 unpinned first`; the upgrade-all dialog ends `brew will refuse git: it needs pcre2 unpinned`.

## Global Constraints

These come from Plans 1–7½ and still hold:
- Module path `brewery`; Go ≥ 1.25.
- **Only `internal/brew` and `main` run brew.** The UI decides from data it already has.
- Views use `theme.Styles` only. Table cells have no color of their own; `theme.Tint` colors text in a cell.
- One job at a time; lock retries; pane actions get `brew.JobEnv`.
- Key routing is by mode first. A confirmation gets every key.
- **Never install or upgrade anything for real while testing.**

Added by this plan:
- **Blocker:** an installed package in a formula's dependencies (recursive, `Package.Deps`) that is pinned and outdated. Casks are never checked.
- **Texts** (names joined with ", "):
  - preview warning: `⚠ upgrading needs <blockers> unpinned` (Warn style)
  - preview tree node: `<name> <pin icon> pinned`, plus ` · <version> → <latest>` in Warn when outdated
  - `u`: `<name> needs <blockers> unpinned first`
  - marked: `skipped <names>: needs <blockers> unpinned` (Warn)
  - upgrade all: `brew will refuse <names>: it needs <blockers> unpinned` (`they need` for several names)
  - install dialog: title `Can't install <name>`, body `It needs <blockers>, which is pinned and outdated.` / `Unpin it on Installed first, or brew will refuse.` (`are` / `them` for several blockers), choices `n close` only

## File Structure

```
internal/state/index.go       PinnedBehind; Tree(fullName, label)
internal/state/index_test.go  Tree callers pass nil; TestTreeLabelsDependencies, TestPinnedBehind
internal/ui/installed.go      depLabel; the tree uses it; the warning line
internal/ui/actions.go        App.pinnedBehind; upgradeSelected; askUpgradeAll
internal/ui/marks.go          upgradeMarked leaves blocked formulae out
internal/ui/install.go        installBox: Can't install
internal/ui/pinned_test.go    pinBehind, pinnedApp, selectOutdated and the UI tests (new)
```

---

### Task 1: `PinnedBehind`, and `Tree` labels dependencies

**Files:**
- Modify: `internal/state/index.go`
- Modify: `internal/state/index_test.go`
- Modify: `internal/ui/installed.go` (one call site)

**Interfaces:**
- **Produces:**
  - `(*Index) PinnedBehind(names ...string) []string`: those of `names` (full names) that are installed, pinned and outdated; sorted, no repeats; nil when none.
  - `(*Index) Tree(fullName string, label func(fullName string) string) []string`: as before, with each dependency's name written by `label` (nil = the name itself). The root line stays the plain name.

- [ ] **Step 1: Update `internal/state/index_test.go`**

Replace the three `Tree` calls:

```go
	if got := ix.Tree("git"); !reflect.DeepEqual(got, want) {
```

with

```go
	if got := ix.Tree("git", nil); !reflect.DeepEqual(got, want) {
```

```go
	if got := ix.Tree("fd"); !reflect.DeepEqual(got, []string{"fd"}) {
```

with

```go
	if got := ix.Tree("fd", nil); !reflect.DeepEqual(got, []string{"fd"}) {
```

and

```go
	if got := NewIndex(pkgs).Tree("a"); !reflect.DeepEqual(got, want) {
```

with

```go
	if got := NewIndex(pkgs).Tree("a", nil); !reflect.DeepEqual(got, want) {
```

Append:

```go

func TestTreeLabelsDependencies(t *testing.T) {
	ix := NewIndex(testPkgs)
	label := func(name string) string { return name + "!" }
	want := []string{"git", "└─ pcre2!", "   └─ json-c!"}
	if got := ix.Tree("git", label); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPinnedBehind(t *testing.T) {
	pkgs := slices.Clone(testPkgs)
	pkgs[1].Pinned, pkgs[1].Outdated = true, true // pcre2: pinned and behind
	pkgs[2].Pinned = true                         // json-c: pinned but current
	pkgs[3].Outdated = true                       // fd: behind but not pinned
	ix := NewIndex(pkgs)
	if got := ix.PinnedBehind("json-c", "pcre2", "fd", "pcre2", "nope"); !reflect.DeepEqual(got, []string{"pcre2"}) {
		t.Errorf("PinnedBehind = %v, want [pcre2]", got)
	}
	if got := ix.PinnedBehind(); got != nil {
		t.Errorf("PinnedBehind() = %v, want nil", got)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/state/`
Expected: FAIL to compile (`too many arguments in call to ix.Tree`, `ix.PinnedBehind undefined`).

- [ ] **Step 3: Update `internal/state/index.go`**

Replace:

```go
// Tree renders p's direct dependencies recursively, one line per node,
// using box-drawing prefixes. Repeated subtrees are shown once.
func (ix *Index) Tree(fullName string) []string {
```

with:

```go
// Tree renders p's direct dependencies recursively, one line per node,
// using box-drawing prefixes. Repeated subtrees are shown once. label, if
// not nil, writes each dependency's name.
func (ix *Index) Tree(fullName string, label func(fullName string) string) []string {
	if label == nil {
		label = func(name string) string { return name }
	}
```

Replace:

```go
				lines = append(lines, prefix+branch+d+" …")
```

with:

```go
				lines = append(lines, prefix+branch+label(d)+" …")
```

and:

```go
			lines = append(lines, prefix+branch+d)
```

with:

```go
			lines = append(lines, prefix+branch+label(d))
```

Append to the file:

```go

// PinnedBehind lists those of names that are installed, pinned and
// outdated, sorted and without repeats. brew refuses to install or upgrade
// a formula while one of its dependencies is pinned behind its latest
// version (FormulaInstaller#check_install_sanity).
func (ix *Index) PinnedBehind(names ...string) []string {
	var behind []string
	for _, n := range names {
		if p, ok := ix.byFull[n]; ok && p.Pinned && p.Outdated && !slices.Contains(behind, n) {
			behind = append(behind, n)
		}
	}
	slices.Sort(behind)
	return behind
}
```

- [ ] **Step 4: Keep the one caller building, in `internal/ui/installed.go`**

Replace:

```go
			b.WriteString(strings.Join(t.ix.Tree(p.FullName), "\n") + "\n")
```

with:

```go
			b.WriteString(strings.Join(t.ix.Tree(p.FullName, nil), "\n") + "\n")
```

- [ ] **Step 5: Run the tests**

Run: `go test -race ./internal/state/ ./internal/ui/ && go vet ./...`
Expected: `ok` for both; vet prints nothing.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal
git add internal/state/index.go internal/state/index_test.go internal/ui/installed.go
git commit -m "feat(state): PinnedBehind; Tree labels dependencies"
```

---

### Task 2: The Installed preview marks pinned dependencies

**Files:**
- Create: `internal/ui/pinned_test.go`
- Modify: `internal/ui/installed.go`

**Interfaces:**
- **Consumes:** `Index.PinnedBehind`, `Index.Tree(fullName, label)`, `Index.Get`, `theme.With`, `Styles.Icons.Pinned`; test helpers `newFakeSource`, `fixtureSnapshot`, `fakeStarter`, `testNow`, `selectInstalled` (app_test.go: opens Installed, presses `f` once for All, then finds the row — call it once per app).
- **Produces:**
  - `(*installedTab) depLabel(name string) string`.
  - Test helpers `pinBehind(*state.Snapshot)` and `pinnedApp(t) (*App, *fakeStarter)`, used by Tasks 3 and 4.

- [ ] **Step 1: Create `internal/ui/pinned_test.go`**

```go
package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
)

// pinBehind pins pcre2 at 10.49 while 10.50 is out, and makes git, which
// needs pcre2, outdated and not pinned: brew would refuse to upgrade git.
func pinBehind(snap *state.Snapshot) {
	for i := range snap.Packages {
		p := &snap.Packages[i]
		switch p.Name {
		case "pcre2":
			p.Pinned, p.Outdated, p.Latest = true, true, "10.50"
		case "git":
			p.Pinned, p.Outdated, p.Latest = false, true, "2.52.0"
		}
	}
	snap.Outdated = append(snap.Outdated,
		brew.Outdated{Name: "git", Kind: brew.KindFormula, Installed: "2.51.0", Current: "2.52.0"},
		brew.Outdated{Name: "pcre2", Kind: brew.KindFormula, Installed: "10.49", Current: "10.50", Pinned: true})
}

// pinnedApp is a job app whose packages, and brew's later answers, are
// as pinBehind leaves them.
func pinnedApp(t *testing.T) (*App, *fakeStarter) {
	t.Helper()
	fs, src := &fakeStarter{}, newFakeSource(t)
	pinBehind(&src.snap)
	snap := fixtureSnapshot(t)
	pinBehind(&snap)
	a := New(src, fs.start, snap, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return a, fs
}

func TestPreviewMarksPinnedDependencies(t *testing.T) {
	a, _ := pinnedApp(t)
	selectInstalled(t, a, "git")
	out := stripANSI(a.render())
	for _, want := range []string{"└─ pcre2 pinned · 10.49 → 10.50", "⚠ upgrading needs pcre2 unpinned"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	a, _ = pinnedApp(t)
	selectInstalled(t, a, "fd")
	if out := stripANSI(a.render()); strings.Contains(out, "upgrading needs") {
		t.Errorf("fd needs nothing pinned:\n%s", out)
	}
}
```

- [ ] **Step 2: Run the test to see it fail**

Run: `go test ./internal/ui/ -run TestPreviewMarksPinnedDependencies`
Expected: FAIL: `missing "└─ pcre2 pinned · 10.49 → 10.50"` and `missing "⚠ upgrading needs pcre2 unpinned"`.

- [ ] **Step 3: Label and warn, in `internal/ui/installed.go`**

In `preview`, replace:

```go
			b.WriteString(strings.Join(t.ix.Tree(p.FullName, nil), "\n") + "\n")
		}
```

with:

```go
			b.WriteString(strings.Join(t.ix.Tree(p.FullName, t.depLabel), "\n") + "\n")
		}
		if behind := t.ix.PinnedBehind(p.Deps...); p.Kind == brew.KindFormula && len(behind) > 0 {
			b.WriteString(st.Warn.Render("⚠ upgrading needs "+strings.Join(behind, ", ")+" unpinned") + "\n")
		}
```

Append to the file:

```go

// depLabel names a dependency in the preview's tree, marking it if it's
// pinned, and with its versions if it's pinned behind: brew won't install
// or upgrade what needs it until it's unpinned.
func (t *installedTab) depLabel(name string) string {
	d, ok := t.ix.Get(name)
	if !ok || !d.Pinned {
		return name
	}
	s := name + " " + theme.With(t.st.Icons.Pinned, "pinned")
	if d.Outdated {
		s += t.st.Warn.Render(" · " + d.Version + " → " + d.Latest)
	}
	return s
}
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./internal/ui/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
gofmt -l internal
git add internal/ui/installed.go internal/ui/pinned_test.go
git commit -m "feat(ui): the Installed preview marks pinned dependencies and warns when one is behind"
```

---

### Task 3: Upgrades say when a pinned dependency is behind

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/marks.go`
- Modify: `internal/ui/pinned_test.go`

**Interfaces:**
- **Consumes:** `Index.Get`, `Index.PinnedBehind`, `App.note`, `App.byKind`, `target`, `pinnedApp` (Task 2), `confirmText` (install_test.go), `lastCall`, `settle`, `resume`, `newFakeProc`.
- **Produces:** `(*App) pinnedBehind(name string, kind brew.Kind) []string`: the blockers of an installed formula, found by full name or, for Outdated rows, by short name and kind; nil for casks and unknown names.

- [ ] **Step 1: Update `internal/ui/pinned_test.go`**

Add `"slices"` to the imports:

```go
import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
```

Insert above `func TestPreviewMarksPinnedDependencies(t *testing.T) {`:

```go
// selectOutdated moves Outdated's cursor to name.
func selectOutdated(t *testing.T, a *App, name string) {
	t.Helper()
	keys(t, a, "3", "g", "g")
	for range len(a.snap.Outdated) {
		if i := a.outdated.cursor(); i >= 0 && a.snap.Outdated[i].Name == name {
			return
		}
		keys(t, a, "j")
	}
	t.Fatalf("%s not found on Outdated", name)
}

```

Append:

```go

func TestUpgradeRefusesWhileADependencyIsPinnedBehind(t *testing.T) {
	a, fs := pinnedApp(t)
	selectInstalled(t, a, "git")
	keys(t, a, "u")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "git needs pcre2 unpinned first") {
		t.Errorf("calls %v, status %q", fs.calls, stripANSI(a.statusBar()))
	}
	selectOutdated(t, a, "git")
	keys(t, a, "u")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "git needs pcre2 unpinned first") {
		t.Errorf("Outdated: calls %v, status %q", fs.calls, stripANSI(a.statusBar()))
	}
}

func TestMarkedUpgradeSkipsBlockedFormulae(t *testing.T) {
	a, fs := pinnedApp(t)
	selectOutdated(t, a, "git")
	keys(t, a, "space")
	keys(t, a, "u")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "skipped git: needs pcre2 unpinned") {
		t.Errorf("only git marked: calls %v, status %q", fs.calls, stripANSI(a.statusBar()))
	}
	p := newFakeProc(0) // the upgrade keeps running while the note shows
	fs.procs = []*fakeProc{p}
	if a.marks() != 1 {
		t.Fatalf("git should still be marked: %d marks", a.marks())
	}
	selectOutdated(t, a, "lua-language-server")
	keys(t, a, "space")
	waiting := settle(t, a, a.handleKey(press("u")))
	if want := []string{"upgrade", "lua-language-server"}; !slices.Equal(lastCall(fs), want) || len(fs.calls) != 1 {
		t.Errorf("ran %v, want %v", fs.calls, want)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "skipped git: needs pcre2 unpinned") {
		t.Errorf("status %q", stripANSI(a.statusBar()))
	}
	p.exit()
	resume(t, a, waiting)
}

func TestUpgradeAllNamesWhatBrewWillRefuse(t *testing.T) {
	a, _ := pinnedApp(t)
	keys(t, a, "3", "A")
	if got := confirmText(a); !strings.Contains(got, "brew will refuse git: it needs pcre2 unpinned") {
		t.Errorf("dialog:\n%s", got)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/ui/ -run 'TestUpgradeRefuses|TestMarkedUpgradeSkips|TestUpgradeAllNames'`
Expected: FAIL: `u` runs `upgrade git`; the marked upgrade runs `upgrade lua-language-server git`; the dialog has no "brew will refuse" line.

- [ ] **Step 3: `pinnedBehind` and the checks, in `internal/ui/actions.go`**

In `upgradeSelected`, replace:

```go
		if p.Pinned {
			return a.note(p.Name + " is pinned · p unpins it")
		}
		name, kind = p.FullName, p.Kind
```

with:

```go
		if p.Pinned {
			return a.note(p.Name + " is pinned · p unpins it")
		}
		if behind := a.pinnedBehind(p.FullName, p.Kind); len(behind) > 0 {
			return a.note(p.Name + " needs " + strings.Join(behind, ", ") + " unpinned first")
		}
		name, kind = p.FullName, p.Kind
```

and replace:

```go
		if o.Pinned {
			return a.note(o.Name + " is pinned · unpin it on Installed")
		}
		name, kind = o.Name, o.Kind
```

with:

```go
		if o.Pinned {
			return a.note(o.Name + " is pinned · unpin it on Installed")
		}
		if behind := a.pinnedBehind(o.Name, o.Kind); len(behind) > 0 {
			return a.note(o.Name + " needs " + strings.Join(behind, ", ") + " unpinned first")
		}
		name, kind = o.Name, o.Kind
```

In `askUpgradeAll`, replace:

```go
	body := []string{strings.Join(names, ", ")}
	if pinned > 0 {
		body = append(body, fmt.Sprintf("%d pinned will be skipped.", pinned))
	}
```

with:

```go
	body := []string{strings.Join(names, ", ")}
	if pinned > 0 {
		body = append(body, fmt.Sprintf("%d pinned will be skipped.", pinned))
	}
	var refused, behind []string
	for _, o := range a.snap.Outdated {
		if b := a.pinnedBehind(o.Name, o.Kind); !o.Pinned && len(b) > 0 {
			refused, behind = append(refused, o.Name), append(behind, b...)
		}
	}
	if len(refused) > 0 {
		they := "they need"
		if len(refused) == 1 {
			they = "it needs"
		}
		behind = a.ix.PinnedBehind(behind...) // sorted, once each
		body = append(body, fmt.Sprintf("brew will refuse %s: %s %s unpinned", strings.Join(refused, ", "), they, strings.Join(behind, ", ")))
	}
```

Append to the file:

```go

// pinnedBehind lists the pinned, outdated dependencies of the installed
// formula name (a full name, or the short name Outdated rows carry). brew
// refuses to upgrade it until they're unpinned. It goes by the installed
// version's dependencies, which an upgrade can change.
func (a *App) pinnedBehind(name string, kind brew.Kind) []string {
	if kind != brew.KindFormula {
		return nil // brew checks formulae only
	}
	p, ok := a.ix.Get(name)
	if !ok {
		i := slices.IndexFunc(a.snap.Packages, func(p brew.Package) bool { return p.Name == name && p.Kind == kind })
		if i < 0 {
			return nil
		}
		p = a.snap.Packages[i]
	}
	return a.ix.PinnedBehind(p.Deps...)
}
```

- [ ] **Step 4: Leave blocked formulae out of marked upgrades, in `internal/ui/marks.go`**

In `upgradeMarked`, replace:

```go
	if len(ts) == 0 {
		return a.note("nothing marked needs upgrading (pinned or up to date)")
	}
	a.clearMarks()
	return a.byKind(ts, func(kind brew.Kind, names []string) action {
		return action{title: "upgrade " + strings.Join(names, " "), args: kindArgs("upgrade", kind, names...), refresh: diskQueries}
	})
}
```

with:

```go
	// brew would refuse those with a dependency pinned behind: leave them out.
	var skipped, behind []string
	ts = slices.DeleteFunc(ts, func(t target) bool {
		b := a.pinnedBehind(t.name, t.kind)
		if len(b) > 0 {
			skipped, behind = append(skipped, t.name), append(behind, b...)
		}
		return len(b) > 0
	})
	why := ""
	if len(skipped) > 0 {
		why = "skipped " + strings.Join(skipped, ", ") + ": needs " + strings.Join(a.ix.PinnedBehind(behind...), ", ") + " unpinned"
	}
	if len(ts) == 0 {
		if why != "" {
			return a.note(why)
		}
		return a.note("nothing marked needs upgrading (pinned or up to date)")
	}
	a.clearMarks()
	cmd := a.byKind(ts, func(kind brew.Kind, names []string) action {
		return action{title: "upgrade " + strings.Join(names, " "), args: kindArgs("upgrade", kind, names...), refresh: diskQueries}
	})
	if why != "" {
		a.flash = a.st.Warn.Render(why)
	}
	return cmd
}
```

`marks.go` already imports `slices`; if `go build` says it doesn't, add it.

- [ ] **Step 5: Run the tests**

Run: `go test -race ./internal/ui/ && go vet ./...`
Expected: `ok`; vet prints nothing.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal
git add internal/ui/actions.go internal/ui/marks.go internal/ui/pinned_test.go
git commit -m "feat(ui): upgrades say when a pinned dependency is behind instead of failing"
```

---

### Task 4: Discover won't offer an install brew would refuse

**Files:**
- Modify: `internal/ui/install.go`
- Modify: `internal/ui/pinned_test.go`

**Interfaces:**
- **Consumes:** `Index.PinnedBehind`, `discoverTab.details`, `installTag`, `pinBehind` (Task 2); test helpers `newInstallApp`, `ripgrep`, `selectDiscover`, `confirmText`, `detailsDueMsg`, `drain`.
- **Produces:** `installBox` returns the "Can't install" box (same `tag`, so it still updates when brew answers) for a formula whose `Needs` include a blocker.

- [ ] **Step 1: Append to `internal/ui/pinned_test.go`**

```go

func TestInstallRefusesWhileADependencyIsPinnedBehind(t *testing.T) {
	a, fs, _, cat := newInstallApp(t)
	pinBehind(&a.snap)
	a.sync()
	rg := ripgrep
	rg.Needs = []string{"pcre2", "oniguruma"}
	cat.details["ripgrep"] = rg
	selectDiscover(t, a, "ripgrep")
	keys(t, a, "a")
	want := "Can't install ripgrep\nIt needs pcre2, which is pinned and outdated.\nUnpin it on Installed first, or brew will refuse."
	if got := confirmText(a); got != want {
		t.Errorf("asked\n%s\nwant\n%s", got, want)
	}
	keys(t, a, "y")
	if len(fs.calls) != 0 {
		t.Errorf("y ran %v", fs.calls)
	}
}

func TestInstallDialogLearnsOfAPinnedDependency(t *testing.T) {
	a, fs, _, cat := newInstallApp(t)
	pinBehind(&a.snap)
	a.sync()
	rg := ripgrep
	rg.Needs = []string{"pcre2"}
	cat.details["ripgrep"] = rg
	selectDiscover(t, a, "python@3.15")
	a.Update(press("j")) // to ripgrep; brew hasn't been asked yet
	a.Update(press("a"))
	if got := confirmText(a); !strings.Contains(got, "checking dependencies…") {
		t.Fatalf("asked\n%s", got)
	}
	drain(t, a, a.detailsDue(detailsDueMsg{key: entryKey("ripgrep", brew.KindFormula)}))
	if got := confirmText(a); !strings.HasPrefix(got, "Can't install ripgrep\n") {
		t.Errorf("the open dialog didn't learn of pcre2:\n%s", got)
	}
	keys(t, a, "y")
	if len(fs.calls) != 0 {
		t.Errorf("y ran %v", fs.calls)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/ui/ -run 'TestInstallRefusesWhile|TestInstallDialogLearns'`
Expected: FAIL: the dialog asks "Install ripgrep 15.2.0?", and `y` runs `install ripgrep`.

- [ ] **Step 3: Say "Can't install", in `internal/ui/install.go`**

In `installBox`, replace:

```go
	key := entryKey(r.Name, r.Kind)
	about := []string{r.Desc}
```

with:

```go
	key := entryKey(r.Name, r.Kind)
	if det, ok := a.discover.details[key]; ok && r.Kind == brew.KindFormula {
		if behind := a.ix.PinnedBehind(det.Needs...); len(behind) > 0 {
			it, is := "it", "is"
			if len(behind) > 1 {
				it, is = "them", "are"
			}
			return &confirmBox{
				title:   "Can't install " + r.Name,
				body:    []string{fmt.Sprintf("It needs %s, which %s pinned and outdated.", strings.Join(behind, ", "), is), "Unpin " + it + " on Installed first, or brew will refuse."},
				choices: []choice{{"n", "close", nil}},
				tag:     installTag(key),
			}
		}
	}
	about := []string{r.Desc}
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./... && go vet ./...`
Expected: all `ok`; vet prints nothing.

- [ ] **Step 5: Commit**

```bash
gofmt -l internal
git add internal/ui/install.go internal/ui/pinned_test.go
git commit -m "feat(ui): Discover won't offer an install brew would refuse over a pinned dependency"
```

---

### Task 5: Final checks

**Files:** none (verification only)

- [ ] **Step 1:**

```bash
go test -race -count=1 ./...
go vet ./...
gofmt -l .
go mod tidy -diff
```

Expected: all `ok`, no output from the rest.

- [ ] **Step 2: Real binary.** Nothing on the machine is pinned and outdated, so the warnings can't be seen live without a real install. Run `make run` and check only that nothing regressed: Installed's preview for a package with dependencies still shows its tree (a pinned dependency that's up to date shows `<pin icon> pinned` and no warning), `u` and `A` behave as before.
