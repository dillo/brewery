# brewery sidebar (step 1 of 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace brewery's tab bar with a sidebar of entries (Home, PACKAGES, MANAGE, DISCOVER) that the focus moves through with `h`/`l`, as step 1 of `docs/superpowers/specs/2026-10-17-sidebar-layout-design.md`.

**Architecture:** The sidebar is a layer over today's tabs. A fixed `entries` table maps each sidebar line to an existing tab and one of its views (`installedFilter` or `discoverView`). `App.selectEntry` sets the tab and view, which is what number keys and `f` did before. The tabs keep their lists, previews and keys. App gains `sel` (the selected entry), `sideFocus` (the sidebar has the focus) and `lastIn` (each section's last-used entry). Steps 2 (sliding columns) and 3 (History and Maintenance) get their own plans once this lands.

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2, bubbles `table`.

## Global Constraints

- Branch: `feat/sidebar` (already created; the spec is committed on it).
- Commits are authored as `Dillo Raju <4197+dillo@users.noreply.github.com>`. The repo's local git config already sets this, so plain `git commit` is right.
- Sidebar width: `sidebarW = 22`, border included. It sits beside the tab from `sidebarBeside = sidebarW + 60` (82) columns.
- Sidebar labels, in order: Home · PACKAGES: All, Top-level, Formulae, Casks, Dependencies, Outdated, Pinned · MANAGE: Services, Taps · DISCOVER: Popular tools, Popular apps, Rising, New, Fonts, Search all.
- `1`–`4` jump to Home, Packages, Manage, Discover. `f`/`F` are removed. Services' log moves from `l` to `L`. Home's tiles move with `j`/`k`.
- Code style: match the package. Comments are full sentences saying why. Names are short. Tests are table-driven where the package's tests are.
- Verify with `go test ./...`, `go vet ./...`, `go vet -tags screenshots ./internal/ui` and `golangci-lint run ./...` (`make lint`).

---

## File map

| File | Change |
|---|---|
| `internal/ui/installed.go` | Formulae and Pinned views; Apps renamed Casks; `count`; `f`/`F` removed (Task 3); view bar replaced by an about line (Task 4) |
| `internal/ui/sidebar.go` | **new**: entries, sections, selection, badges, the sidebar's view, its keys |
| `internal/ui/sidebar_test.go` | **new** |
| `internal/ui/app.go` | App fields; key routing; tab bar removed; render with the sidebar; status trail |
| `internal/ui/home.go` | `j`/`k` for tiles; `idle` |
| `internal/ui/discover.go` | `f`/`F` removed; view bar replaced; title is the view's name; `idle` |
| `internal/ui/list.go` | `idle` |
| `internal/ui/actions.go` | `L` for the log; hints |
| `internal/ui/links.go` | `gg`/`gx` in the sidebar |
| `internal/ui/help.go` | new keys |
| `internal/ui/components.go` | `aboutHeight`, `about` |
| `internal/ui/viewbar.go` | **deleted** (Task 4) |
| `internal/ui/helpers_test.go` | `show` helper |
| `internal/ui/*_test.go` | number keys and `f`/`F` migrated |
| `README.md`, `docs/screenshots/*` | Sidebar section, keys, screenshots |

---

### Task 1: Installed's six views

**Files:**
- Modify: `internal/ui/installed.go:15-31` (filters, views), `:67-80` (`shows`), and add `count`
- Modify: `internal/ui/installed.go:241-252` (`viewBar` uses `count`)
- Test: `internal/ui/installed_test.go:31-56` (`TestInstalledViews`), and the view-bar tests at `:58-85`

**Interfaces:**
- Produces: `filterFormulae`, `filterCasks` (was `filterApps`), `filterPinned` of type `installedFilter`; `func (t *installedTab) count(f installedFilter) int`.

- [ ] **Step 1: Write the failing test.** Replace `TestInstalledViews` in `internal/ui/installed_test.go` with:

```go
func TestInstalledViews(t *testing.T) {
	it := newFixtureInstalled(t)
	for _, w := range []struct {
		f     installedFilter
		names string
	}{
		{filterTop, "docker-desktop fd fontbase git lua-language-server"},
		{filterAll, "docker-desktop fd fontbase git json-c lua-language-server pcre2"},
		{filterFormulae, "fd git json-c lua-language-server pcre2"},
		{filterCasks, "docker-desktop fontbase"},
		{filterDeps, "json-c pcre2"},
		{filterPinned, "git"},
	} {
		it.filter = w.f
		it.apply()
		if got := rowNames(it); got != w.names {
			t.Errorf("view %s: got %q, want %q", views[w.f].name, got, w.names)
		}
		if got, want := it.count(w.f), len(strings.Fields(w.names)); got != want {
			t.Errorf("view %s: count %d, want %d", views[w.f].name, got, want)
		}
	}
}
```

In `TestInstalledViewBarShowsEveryViewAndExplainsTheActiveOne` and the narrow view-bar test after it (`:58-85`), replace `"Apps 2"` with `"Casks 2"`, and `"Apps"` in the error message with `"Casks"`. Add `"Formulae 5"` and `"Pinned 1"` to the wide test's list. If the narrow test's expected short names no longer fit, drop its expectation for the last segments rather than widen the pane, since Task 4 deletes the view bar.

- [ ] **Step 2: Run the test to verify it fails.**

Run: `go test ./internal/ui -run 'TestInstalledView' -count=1`
Expected: a build failure: `undefined: filterFormulae` (and `filterCasks`, `filterPinned`, `count`).

- [ ] **Step 3: Implement.** In `internal/ui/installed.go`, replace the filter constants and `views`:

```go
const (
	filterTop installedFilter = iota
	filterAll
	filterCasks
	filterDeps
	filterFormulae
	filterPinned
)

// views are the Installed tab's views, with a short name for narrow panes
// and a line saying what each shows.
var views = [...]struct{ name, short, about string }{
	filterTop:      {"Top-level", "Top", "Things you installed that nothing else needs."},
	filterAll:      {"All", "All", "Everything installed."},
	filterCasks:    {"Casks", "Casks", "macOS apps and fonts (casks)."},
	filterDeps:     {"Dependencies", "Deps", "Installed because something else needed them."},
	filterFormulae: {"Formulae", "Form", "Command-line tools and libraries (formulae)."},
	filterPinned:   {"Pinned", "Pin", "Held at their version: brew upgrade skips them."},
}
```

In `shows`, replace `case filterApps:` and add the two new views:

```go
	case filterCasks:
		return p.Kind == brew.KindCask
	case filterFormulae:
		return p.Kind == brew.KindFormula
	case filterPinned:
		return p.Pinned
```

Add, after `shows`:

```go
// count is how many installed packages view f lists, before the "/" filter.
func (t *installedTab) count(f installedFilter) int {
	n := 0
	for _, p := range t.all {
		if t.shows(f, p) {
			n++
		}
	}
	return n
}
```

In `viewBar`, replace the inner counting loop with `segs[f] = viewSeg{name: v.name, short: v.short, n: t.count(installedFilter(f))}`. Then rename any remaining `filterApps` in the package (`grep -rn filterApps internal/ui`) to `filterCasks`.

- [ ] **Step 4: Run the tests to verify they pass.**

Run: `go test ./internal/ui -count=1`
Expected: PASS, except `TestScreens`: the Installed golden's view bar now lists six views. Regenerate it with `go test ./internal/ui -run TestScreens -update -count=1` and check in `git diff internal/ui/testdata/golden` that only the view bar changed. Tests that step through views with `f` still pass, because `f` steps Top → All → Casks → Deps as before and then on to Formulae and Pinned. Fix any test that counted on `f` wrapping after Dependencies by giving it one more `f`, or two.

- [ ] **Step 5: Commit.**

```bash
git add internal/ui/installed.go internal/ui/installed_test.go internal/ui/testdata/golden
git commit -m "feat(ui): Installed gains Formulae and Pinned views; Apps becomes Casks"
```

---

### Task 2: The sidebar's entries, selection and drawing

This task adds the model and its view. Nothing calls them from keys or `render` yet; Tasks 3 and 4 wire them in.

**Files:**
- Create: `internal/ui/sidebar.go`
- Create: `internal/ui/sidebar_test.go`
- Modify: `internal/ui/app.go` (App fields, `New`)
- Modify: `internal/ui/helpers_test.go` (`show`)

**Interfaces:**
- Consumes: `filterAll`, `filterTop`, `filterFormulae`, `filterCasks`, `filterDeps`, `filterPinned`, `(*installedTab).count` (Task 1); `discTools` … `discAll`; `(*App).showTab(int) tea.Cmd`; `tabQueries`; `paneInner`; `pane`.
- Produces:
  - `type section int` with `secHome`, `secPackages`, `secManage`, `secDiscover`
  - `type entry struct{ label string; sec section; tab, view int }`; `var entries []entry`
  - `func entryIndex(label string) int`
  - `func (a *App) selectEntry(i int) tea.Cmd`
  - `func (a *App) selectSection(s section) tea.Cmd`
  - `func (a *App) stepEntry(d int) tea.Cmd`
  - `func (a *App) badge(e entry) string`
  - `func (a *App) sidebarView(w, h int) string`
  - `const sidebarW = 22`
  - App fields `sel int`, `sideFocus bool`, `lastIn map[section]int`
  - test helper `show(t *testing.T, a *App, label string)`

- [ ] **Step 1: Write the failing tests.** Create `internal/ui/sidebar_test.go`:

```go
package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestSidebarHasEveryViewOnce(t *testing.T) {
	seen := map[[2]int]int{}
	for _, e := range entries {
		seen[[2]int{e.tab, e.view}]++
	}
	for f := range views {
		if n := seen[[2]int{tabInstalled, f}]; n != 1 {
			t.Errorf("Installed view %s is in the sidebar %d times", views[f].name, n)
		}
	}
	for v := range discoverViews {
		if n := seen[[2]int{tabDiscover, v}]; n != 1 {
			t.Errorf("Discover view %s is in the sidebar %d times", discoverViews[v].name, n)
		}
	}
	for _, tab := range []int{tabHome, tabOutdated, tabServices, tabTaps} {
		if n := seen[[2]int{tab, 0}]; n != 1 {
			t.Errorf("tab %d is in the sidebar %d times", tab, n)
		}
	}
}

func TestSelectEntryShowsItsTabAndView(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	for _, tt := range []struct {
		label string
		tab   int
		check func() bool
	}{
		{"Casks", tabInstalled, func() bool { return a.installed.filter == filterCasks }},
		{"Pinned", tabInstalled, func() bool { return a.installed.filter == filterPinned }},
		{"Rising", tabDiscover, func() bool { return a.discover.showing == discRising }},
		{"Search all", tabDiscover, func() bool { return a.discover.showing == discAll }},
		{"Taps", tabTaps, func() bool { return true }},
	} {
		settle(t, a, a.selectEntry(entryIndex(tt.label)))
		if a.tab != tt.tab || !tt.check() || entries[a.sel].label != tt.label {
			t.Errorf("%s: tab %d, sel %s", tt.label, a.tab, entries[a.sel].label)
		}
	}
}

func TestSelectSectionReturnsToItsLastEntry(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	settle(t, a, a.selectSection(secDiscover))
	if got := entries[a.sel].label; got != "Popular tools" {
		t.Errorf("first visit to Discover: %s, want its first entry", got)
	}
	settle(t, a, a.selectEntry(entryIndex("Taps")))
	settle(t, a, a.selectSection(secHome))
	settle(t, a, a.selectSection(secManage))
	if got := entries[a.sel].label; got != "Taps" {
		t.Errorf("back to Manage: %s, want Taps", got)
	}
}

func TestStepEntryWraps(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	settle(t, a, a.stepEntry(-1))
	if got := entries[a.sel].label; got != "Search all" {
		t.Errorf("before Home: %s, want the last entry", got)
	}
	settle(t, a, a.stepEntry(1))
	if got := entries[a.sel].label; got != "Home" {
		t.Errorf("after the last entry: %s, want Home", got)
	}
}

func TestSidebarShowsCountsAndChips(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	out := stripANSI(a.sidebarView(sidebarW, 40))
	for _, want := range []string{
		"All 7", "Top-level 5", "Formulae 5", "Casks 2", "Dependencies 2", "Pinned 1",
		fmt.Sprintf("Outdated ↑%d", len(a.snap.Outdated)),
		fmt.Sprintf("Taps %d", len(a.snap.Taps)),
		"PACKAGES", "MANAGE", "DISCOVER", "Search all",
	} {
		if !strings.Contains(squash(out), want) {
			t.Errorf("sidebar lacks %q:\n%s", want, out)
		}
	}
	for i, l := range strings.Split(out, "\n") {
		if w := lipgloss.Width(l); w > sidebarW {
			t.Errorf("line %d is %d wide, want <= %d: %q", i, w, sidebarW, l)
		}
	}
	a.errs[qTaps] = errors.New("offline")
	if out := squash(stripANSI(a.sidebarView(sidebarW, 40))); !strings.Contains(out, "Taps ⚠") {
		t.Errorf("a failed refresh should mark its entry: %s", out)
	}
}
```

Before relying on `"Services ✗"` or a Services count, check the fixture: `TestTabBarBadges` shows that `newFakeSource`'s data has one failing service. The `newJobApp` fixture (`fixtureSnapshot`) may differ, so the test above leaves Services out.

- [ ] **Step 2: Run the tests to verify they fail.**

Run: `go test ./internal/ui -run 'Sidebar|SelectEntry|SelectSection|StepEntry' -count=1`
Expected: a build failure: `undefined: entries`.

- [ ] **Step 3: Implement.** Create `internal/ui/sidebar.go`:

```go
package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// section is a group of sidebar entries; 1-4 jump to them.
type section int

const (
	secHome section = iota
	secPackages
	secManage
	secDiscover
)

// sectionNames head each section in the sidebar. Home has no heading.
var sectionNames = [...]string{secPackages: "PACKAGES", secManage: "MANAGE", secDiscover: "DISCOVER"}

// entry is one line of the sidebar: the tab it shows and, for Installed
// and Discover, which of the tab's views (an installedFilter or a
// discoverView).
type entry struct {
	label string
	sec   section
	tab   int
	view  int
}

var entries = []entry{
	{"Home", secHome, tabHome, 0},
	{"All", secPackages, tabInstalled, int(filterAll)},
	{"Top-level", secPackages, tabInstalled, int(filterTop)},
	{"Formulae", secPackages, tabInstalled, int(filterFormulae)},
	{"Casks", secPackages, tabInstalled, int(filterCasks)},
	{"Dependencies", secPackages, tabInstalled, int(filterDeps)},
	{"Outdated", secPackages, tabOutdated, 0},
	{"Pinned", secPackages, tabInstalled, int(filterPinned)},
	{"Services", secManage, tabServices, 0},
	{"Taps", secManage, tabTaps, 0},
	{"Popular tools", secDiscover, tabDiscover, int(discTools)},
	{"Popular apps", secDiscover, tabDiscover, int(discApps)},
	{"Rising", secDiscover, tabDiscover, int(discRising)},
	{"New", secDiscover, tabDiscover, int(discNew)},
	{"Fonts", secDiscover, tabDiscover, int(discFonts)},
	{"Search all", secDiscover, tabDiscover, int(discAll)},
}

// entryIndex is the index of the entry labelled label. The labels are
// fixed, so an unknown one is a bug.
func entryIndex(label string) int {
	for i, e := range entries {
		if e.label == label {
			return i
		}
	}
	panic("no sidebar entry " + label)
}

// selectEntry shows entry i: its tab, and its view of that tab. The focus
// stays in its column.
func (a *App) selectEntry(i int) tea.Cmd {
	e := entries[i]
	a.sel = i
	a.lastIn[e.sec] = i
	switch e.tab {
	case tabInstalled:
		if f := installedFilter(e.view); a.installed.filter != f {
			a.installed.filter = f
			a.installed.apply() // keeps the selected package if the new view shows it
		}
	case tabDiscover:
		if v := discoverView(e.view); a.discover.showing != v {
			a.discover.showing = v
			a.discover.apply()
		}
	}
	return a.showTab(e.tab)
}

// selectSection shows the entry last used in section s, or its first.
func (a *App) selectSection(s section) tea.Cmd {
	if i, ok := a.lastIn[s]; ok {
		return a.selectEntry(i)
	}
	for i, e := range entries {
		if e.sec == s {
			return a.selectEntry(i)
		}
	}
	return nil
}

// stepEntry shows the entry d places on, wrapping around.
func (a *App) stepEntry(d int) tea.Cmd {
	n := len(entries)
	return a.selectEntry(((a.sel+d)%n + n) % n)
}

// badge is what an entry shows on its right: a chip when something needs
// attention, otherwise a muted count, or nothing before the data is in.
func (a *App) badge(e entry) string {
	st := a.st
	if q, ok := tabQueries[e.tab]; ok && e.tab != tabHome && a.errs[q] != nil {
		return st.ChipStale.Render("⚠") // stale data outranks any count
	}
	n := -1
	switch e.tab {
	case tabInstalled:
		if a.loaded[qInstalled] {
			n = a.installed.count(installedFilter(e.view))
		}
	case tabOutdated:
		if k := len(a.snap.Outdated); k > 0 {
			return st.ChipWarn.Render(fmt.Sprintf("↑%d", k))
		}
		if a.loaded[qOutdated] {
			n = 0
		}
	case tabServices:
		failing := 0
		for _, s := range a.snap.Services {
			if s.Status == "error" {
				failing++
			}
		}
		if failing > 0 {
			return st.ChipErr.Render(fmt.Sprintf("✗%d", failing))
		}
		if a.loaded[qServices] {
			n = len(a.snap.Services)
		}
	case tabTaps:
		if a.loaded[qTaps] {
			n = len(a.snap.Taps)
		}
	}
	if n < 0 {
		return ""
	}
	return st.Muted.Render(fmt.Sprint(n))
}

// sidebarW is the sidebar's width, border included: room for
// " Dependencies  145 " inside it.
const sidebarW = 22

// sidebarView draws the sidebar in a w×h pane. The selected entry is a
// solid block, as the active tab was.
func (a *App) sidebarView(w, h int) string {
	st := a.st
	iw, _ := paneInner(w, h)
	var lines []string
	for i, e := range entries {
		if i > 0 && e.sec != entries[i-1].sec {
			lines = append(lines, "", st.Muted.Render(sectionNames[e.sec]))
		}
		b := a.badge(e)
		if i == a.sel {
			b = ansi.Strip(b) // the block's colours carry it
		}
		label := ansi.Truncate(e.label, max(iw-3-ansi.StringWidth(b), 1), "…")
		gap := max(iw-2-ansi.StringWidth(label)-ansi.StringWidth(b), 1)
		line := " " + label + strings.Repeat(" ", gap) + b + " "
		if i == a.sel {
			line = st.TabActive.Render(line)
		}
		lines = append(lines, line)
	}
	return pane(st, "brewery", strings.Join(lines, "\n"), w, h, a.sideFocus)
}
```

In `internal/ui/app.go`, add to `App` after `tab int`:

```go
	sel       int             // the sidebar entry shown
	sideFocus bool            // keys go to the sidebar, not the tab
	lastIn    map[section]int // each section's last-used entry, for 1-4
```

In `New`, add `sideFocus: true, lastIn: map[section]int{},` to the struct literal. brewery opens on Home with the sidebar focused.

In `internal/ui/helpers_test.go`, add:

```go
// show selects the sidebar entry labelled label and focuses its list, as
// moving there in the sidebar and pressing l would.
func show(t *testing.T, a *App, label string) {
	t.Helper()
	settle(t, a, a.selectEntry(entryIndex(label)))
	a.sideFocus = false
	_, cmd := a.Update(nil) // what follows any key: Services' log, Discover's details, the layout
	settle(t, a, cmd)
}
```

- [ ] **Step 4: Run the tests to verify they pass.**

Run: `go test ./internal/ui -count=1`
Expected: PASS. If `TestSidebarShowsCountsAndChips`'s counts disagree with the fixture, print the sidebar and correct the expected numbers. The Installed counts are the ones Task 1's test established.

- [ ] **Step 5: Commit.**

```bash
git add internal/ui/sidebar.go internal/ui/sidebar_test.go internal/ui/app.go internal/ui/helpers_test.go
git commit -m "feat(ui): sidebar entries, selection and drawing"
```

---

### Task 3: Keys and focus

The sidebar takes the keys: `j`/`k`/`gg`/`G` move through entries, `h`/`l` move the focus across sidebar, list and details, `1`–`4` jump to sections and `[`/`]` step through entries. `f`/`F` go away, Home's tiles use `j`/`k`, and Services' log moves to `L`. The tab bar is still drawn until Task 4. It highlights `a.tab`, which `selectEntry` keeps right.

**Files:**
- Modify: `internal/ui/sidebar.go` (add `sidebarKey`, `focusDetails`, `unfocusDetails`, `tileEntries`)
- Modify: `internal/ui/app.go` (`handleKey`)
- Modify: `internal/ui/home.go` (`update`)
- Modify: `internal/ui/installed.go`, `internal/ui/discover.go` (`update`: drop `f`/`F`)
- Modify: `internal/ui/actions.go` (`L`, `actionHints`)
- Modify: `internal/ui/links.go` (`gKey`)
- Modify: `internal/ui/jobsview.go` (`openJobs`)
- Modify: `internal/ui/help.go`
- Test: `internal/ui/sidebar_test.go`, plus every test file that presses `1`–`6`, `f`, `F`, `l` on Services, or `h`/`l` on Home

**Interfaces:**
- Consumes: everything Task 2 produced.
- Produces: `func (a *App) sidebarKey(k string) (tea.Cmd, bool)`, `func (a *App) focusDetails()`, `func (a *App) unfocusDetails() bool`, `var tileEntries map[int]string`.

- [ ] **Step 1: Write the failing tests.** Append to `internal/ui/sidebar_test.go`:

```go
func label(a *App) string { return entries[a.sel].label }

func TestSidebarKeysMoveThroughEntries(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	if label(a) != "Home" || !a.sideFocus {
		t.Fatalf("brewery should open on Home with the sidebar focused: %s %v", label(a), a.sideFocus)
	}
	keys(t, a, "k")
	if label(a) != "Home" {
		t.Errorf("k at the top: %s", label(a))
	}
	keys(t, a, "j", "j", "j", "j")
	if label(a) != "Casks" || a.tab != tabInstalled || a.installed.filter != filterCasks || !a.sideFocus {
		t.Errorf("4×j: %s tab %d filter %d focus %v", label(a), a.tab, a.installed.filter, a.sideFocus)
	}
	keys(t, a, "G")
	if label(a) != "Search all" {
		t.Errorf("G: %s", label(a))
	}
	keys(t, a, "g", "g")
	if label(a) != "Home" {
		t.Errorf("gg: %s", label(a))
	}
}

func TestFocusMovesAcrossColumns(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "j", "l")
	if a.sideFocus || a.installed.pv.focus {
		t.Fatalf("l from the sidebar should focus the list")
	}
	keys(t, a, "l")
	if !a.installed.pv.focus {
		t.Fatalf("l from the list should focus the details")
	}
	keys(t, a, "h")
	if a.installed.pv.focus || a.sideFocus {
		t.Fatalf("h from the details should focus the list")
	}
	keys(t, a, "h")
	if !a.sideFocus {
		t.Fatalf("h from the list should focus the sidebar")
	}
	keys(t, a, "enter")
	if a.sideFocus {
		t.Errorf("enter in the sidebar should focus the list")
	}
}

func TestNumberKeysJumpToSections(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "4", "j")
	if label(a) != "Popular apps" || !a.sideFocus {
		t.Fatalf("4 then j: %s, focus %v", label(a), a.sideFocus)
	}
	keys(t, a, "1", "4")
	if label(a) != "Popular apps" {
		t.Errorf("4 should return to Discover's last entry: %s", label(a))
	}
	keys(t, a, "3")
	if label(a) != "Services" {
		t.Errorf("3: %s", label(a))
	}
}

func TestBracketsStepEntriesAndKeepTheFocus(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Taps")
	keys(t, a, "]")
	if label(a) != "Popular tools" || a.sideFocus {
		t.Errorf("] from Taps: %s, focus %v", label(a), a.sideFocus)
	}
	keys(t, a, "[")
	if label(a) != "Taps" {
		t.Errorf("[: %s", label(a))
	}
}

func TestRowKeysDoNothingInTheSidebar(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "j", "u", "x", "p", "space", "y")
	if len(fs.calls) != 0 || a.confirm != nil || a.marks() != 0 {
		t.Errorf("row keys in the sidebar acted: %v %+v %d", fs.calls, a.confirm, a.marks())
	}
	keys(t, a, "U")
	if got := lastCall(fs); len(got) == 0 || got[0] != "update" {
		t.Errorf("U should work from the sidebar: %v", got)
	}
}

func TestSlashFromTheSidebarFiltersTheList(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "j", "/")
	if a.mode != modeFilter || a.sideFocus {
		t.Errorf("/ should filter the list: mode %d, focus %v", a.mode, a.sideFocus)
	}
}

func TestHomeTileOpensItsEntry(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "l", "j", "enter") // Home's content, then the Outdated tile
	if label(a) != "Outdated" || a.sideFocus {
		t.Errorf("enter on the Outdated tile: %s, focus %v", label(a), a.sideFocus)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail.**

Run: `go test ./internal/ui -run 'SidebarKeys|FocusMoves|NumberKeys|Brackets|RowKeys|SlashFrom|HomeTile' -count=1`
Expected: FAIL. For example, `4×j: Home …`, because no key reaches the sidebar yet.

- [ ] **Step 3: Implement the sidebar's keys.** Append to `internal/ui/sidebar.go`:

```go
// sidebarKey handles a key while the sidebar has the focus. ok is false
// for the keys that work the same from any column.
func (a *App) sidebarKey(k string) (cmd tea.Cmd, ok bool) {
	switch k {
	case "j", "down":
		return a.selectEntry(min(a.sel+1, len(entries)-1)), true
	case "k", "up":
		return a.selectEntry(max(a.sel-1, 0)), true
	case "G", "end":
		return a.selectEntry(len(entries)-1), true
	case "home":
		return a.selectEntry(0), true
	case "l", "right", "enter", "tab":
		a.sideFocus = false
		return nil, true
	case "/":
		a.sideFocus = false // the filter is the list's: go there to type it
		return nil, false
	case "A":
		return nil, a.tab != tabOutdated // upgrade all needs no row
	case "q", ":", "?", "ctrl+c", "esc", "o", "i", "r", "U", "c", "[", "]", "1", "2", "3", "4":
		return nil, false
	}
	return nil, true // the rest act on a row, and the sidebar has none
}

// focusDetails moves the focus from the list to the details, on tabs
// that have them.
func (a *App) focusDetails() {
	switch a.tab {
	case tabInstalled:
		a.installed.pv.focus = true
	case tabDiscover:
		a.discover.pv.focus = true
	case tabServices:
		if a.services.previewAlone() {
			a.services.pvOpen = true
		}
	}
}

// unfocusDetails moves the focus from the details back to the list. ok is
// false when the list had it already.
func (a *App) unfocusDetails() (ok bool) {
	switch {
	case a.tab == tabInstalled && a.installed.pv.focus:
		a.installed.pv.focus = false
	case a.tab == tabDiscover && a.discover.pv.focus:
		a.discover.pv.focus = false
	case a.servicesPreviewAlone():
		a.services.pvOpen = false
	default:
		return false
	}
	return true
}

// tileEntries are the entries Home's tiles open, by the tab they name.
var tileEntries = map[int]string{tabInstalled: "All", tabOutdated: "Outdated", tabServices: "Services"}
```

- [ ] **Step 4: Route the keys.** In `internal/ui/app.go` `handleKey`, after the `if a.jobsOpen { … }` block, add:

```go
	if a.sideFocus {
		if cmd, ok := a.sidebarKey(k); ok {
			return cmd
		}
	}
```

In the `switch k` that follows, replace the `"1"…"6"`, `"]"` and `"["` cases with:

```go
	case "1", "2", "3", "4":
		return a.selectSection(section(k[0] - '1'))
	case "]":
		return a.stepEntry(1)
	case "[":
		return a.stepEntry(-1)
	case "h", "left":
		if !a.unfocusDetails() {
			a.sideFocus = true
		}
		return nil
	case "l", "right":
		a.focusDetails()
		return nil
```

In the same function's `case tabHome:`, replace `a.tab = t` with:

```go
		case t >= 0:
			return a.selectEntry(entryIndex(tileEntries[t]))
```

In `internal/ui/home.go` `update`, replace the `"l", "right"` and `"h", "left"` cases with:

```go
	case "j", "down":
		h.sel = min(h.sel+1, n-1)
	case "k", "up":
		h.sel = max(h.sel-1, 0)
```

In `internal/ui/installed.go` and `internal/ui/discover.go` `update`, delete the `if k == "f" || k == "F" { … }` block. Change `case t.pv.focus && k != "f" && k != "F":` to `case t.pv.focus:`. Update the `views` and `discoverViews` comments that say "in the order f steps through them".

In `internal/ui/actions.go` `actionKey`, change `a.tab == tabServices && k == "l"` to `a.tab == tabServices && k == "L"`. Replace `actionHints` with:

```go
// actionHints are the status-bar hints for where the focus is.
func (a *App) actionHints() string {
	if a.jobsOpen {
		return "enter show output · esc close"
	}
	if a.sideFocus {
		return "j/k choose · l open · 1-4 sections"
	}
	switch a.tab {
	case tabInstalled:
		if a.installed.pv.focus {
			return "j/k scroll · h back to the list · u upgrade · x uninstall"
		}
		return "u upgrade · x uninstall · p pin · l details"
	case tabOutdated:
		return "u upgrade · A upgrade all"
	case tabServices:
		return "s start · S stop · R restart · L log"
	case tabTaps:
		return "x untap"
	case tabDiscover:
		if a.discover.pv.focus {
			return "j/k scroll · h back to the list"
		}
		return "a install · / search · l details · r refetch"
	}
	return "j/k tiles · h sidebar · r refresh · U update · c clean up"
}
```

In `internal/ui/links.go` `gKey`, make the first lines of `case "g":` and `case "x":`:

```go
	case "g":
		if a.sideFocus {
			return a.selectEntry(0), true
		}
```

```go
	case "x":
		if a.sideFocus {
			return nil, true // no row, so no homepage
		}
		return a.openHomepage(), true
```

In `internal/ui/jobsview.go` `openJobs`, add `a.sideFocus = false` so the jobs view's keys aren't mistaken for the sidebar's.

In `internal/ui/help.go` `helpGroups`:
- Add a case after `a.jobsOpen`: `case a.sideFocus: here = helpGroup{"Sidebar", [][2]string{{"j k", "choose"}, {"gg G", "first, last"}, {"l enter", "go to the list"}}}`
- Home: `{"j k", "move between tiles"}, {"enter", "open the tile's entry (Cleanup: clean up)"}`
- Installed: remove `{"f F", …}`; replace the `tab enter` row with `{"l tab enter", "details, then j/k scroll"}`
- Services: `{"L", "follow the log full screen"}`
- Discover: remove `{"f F", …}`; replace the `tab enter` row with `{"l tab enter", "details, then j/k scroll"}`
- The Lists group's condition: `case a.tab != tabHome && !a.sideFocus:`
- Everywhere: replace `{"1-6 [ ]", "switch tabs"}` with `{"h l", "sidebar, list, details"}, {"1-4", "Home, Packages, Manage, Discover"}, {"[ ]", "previous or next entry"}`

- [ ] **Step 5: Run the new tests.**

Run: `go test ./internal/ui -run 'SidebarKeys|FocusMoves|NumberKeys|Brackets|RowKeys|SlashFrom|HomeTile' -count=1`
Expected: PASS.

- [ ] **Step 6: Migrate the old tests' number keys.** Old number keys landed on a tab with its list focused. `show` does the same through an entry. Map each old key to the entry that keeps the view the test expects:

```bash
perl -pi -e 'my %m = (1 => "Home", 2 => "Top-level", 3 => "Outdated", 4 => "Services", 5 => "Taps", 6 => "Popular tools");
  s/\bkeys\(t, a, "([1-6])"\)/show(t, a, "$m{$1}")/g;
  s/\bkeys\(t, a, "([1-6])", /show(t, a, "$m{$1}"); keys(t, a, /g' internal/ui/*_test.go
gofmt -w internal/ui
```

Then fix by hand what the script can't reach. Each fix keeps the test's intent:

| Place | Was | Becomes |
|---|---|---|
| `actions_test.go:130` | `"2", "f", "f", "g", "g", "x"` | `show(t, a, "Casks")`, then `keys(… "g", "g", "x")` |
| `actions_test.go:149` | `"2", "f", "j", "x"` | `show(t, a, "All")`, then `keys(… "j", "x")` |
| `actions_test.go:161` | `"2", "f", "f", "f", "g", "g", "x"` | `show(t, a, "Dependencies")`, then `keys(… "g", "g", "x")` |
| `actions_test.go:189` | `"f", "f", "g", "g", "p"` (from Top-level) | `show(t, a, "Casks")`, then `keys(… "g", "g", "p")` |
| `actions_test.go:236` | `"3", "x", "p", "l", "4", "u", "A", "x", "5", "s", "l"` | `show(t, a, "Outdated")`; `keys(… "x", "p", "L")`; `show(t, a, "Services")`; `keys(… "u", "A", "x")`; `show(t, a, "Taps")`; `keys(… "s", "L")` |
| `actions_test.go:246-250` | hint table keyed by number keys | key each row by entry label and call `show`; the expected hints are the new `actionHints` strings (Installed: `u upgrade · x uninstall · p pin · l details`; Services: `s start · S stop · R restart · L log`; Home: `U update · c clean up`) |
| `app_test.go` `a.Update(press("N"))` at `:266, 310, 323, 331, 436, 521, 554` | direct presses | `show(t, a, <mapped label>)`; `:437`'s `press("f")` with the `"2"` before it becomes `show(t, a, "All")` |
| `app_test.go:305` | `press("l")` to reach the Outdated tile | `press("j")` |
| `cleanup_test.go:69` | `"l", "l", "l", "l", "enter"` | `"j", "j", "j", "j", "enter"` |
| `home_test.go:111-125` | `press("l")` / `press("h")` | `press("j")` / `press("k")` |
| `services_test.go:177, 195, 199, 220, 270` | `"l"` for the log | `"L"`; `:181`'s `"l log"` becomes `"L log"` |
| `details_test.go:40, 79, 101, 120, 145` | `"6", "F"` | `show(t, a, "Search all")` |
| `details_test.go:71` | `"f"` (from Popular tools) | `show(t, a, "Popular apps")` |
| `details_test.go:108` | `a.Update(press("F"))` | `a.selectEntry(entryIndex("Search all"))`, keeping the test's point that the answer hasn't come |
| `stats_test.go:131, 183` | `"f"` | `show(t, a, "Popular apps")` |
| `stats_test.go:145` | `"6", "f", "f"` | `show(t, a, "Rising")` |
| `stats_test.go:163` | `"6", "f", "f", "f", "f"` | `show(t, a, "Fonts")` |
| `marks_test.go:60, 81, 114` | `"2", "f", …` | `show(t, a, "All")`, then the rest |
| `marks_test.go:120` | a loop pressing `"f"` through views | loop over `[]string{"All", "Top-level", "Formulae", "Casks", "Dependencies", "Pinned"}` calling `show` |
| `installed_test.go:66-67, 114, 238, 242` | `it.update(press("f"))` on the tab | set `it.filter` to the view the comment names (`filterAll`, `filterCasks`, …) and call `it.apply()` |
| `golden_test.go:55-62` | `{"installed", []string{"2", "j", "j", "j"}}` etc. | give the table an `entry string` field and call `show(t, a, s.entry)` before `keys`: home→`Home`, installed→`Top-level` + `j j j`, outdated→`Outdated`, services→`Services`, discover-all→`Search all`, help→`Top-level` + `?` |
| `screenshots_test.go` | number keys, and `"F"` at `:215` | the same mapping; `:215` becomes `show(t, a, "Top-level")` |

Find anything left:

```bash
grep -nE 'press\("[1-6fF]"\)|keys\(t, [a-z]+, "[1-6fF]"|"[1-6]", "' internal/ui/*_test.go
```

Expected: no output. Lines from `complete_test.go` that type `f` into the command line are fine.

- [ ] **Step 7: Run everything.**

Run: `go test ./internal/ui -count=1`
Expected: only `TestScreens` fails, because the help text and the status bar hints changed. Regenerate the goldens and read the diff: only help lines and status bar hints should change.

```bash
go test ./internal/ui -run TestScreens -update -count=1
git diff --stat internal/ui/testdata/golden
go test ./... && go vet ./... && go vet -tags screenshots ./internal/ui && golangci-lint run ./...
```

Expected: all pass.

- [ ] **Step 8: Commit.**

```bash
git add internal/ui
git commit -m "feat(ui): the sidebar takes the keys: h/l focus, 1-4 sections, [ ] entries; f/F go"
```

---

### Task 4: Draw the sidebar in place of the tab bar

**Files:**
- Modify: `internal/ui/app.go` (`render`, `layout`, `bodyHeight`, `statusBar`; delete `tabBar`, `tabNames`)
- Modify: `internal/ui/installed.go`, `internal/ui/discover.go` (about line instead of the view bar; titles; `idle`)
- Modify: `internal/ui/list.go`, `internal/ui/home.go` (`idle`)
- Modify: `internal/ui/components.go` (`aboutHeight`, `about`)
- Delete: `internal/ui/viewbar.go` and its tests, if it has any (`grep -ln viewBar internal/ui/*_test.go`)
- Test: `internal/ui/app_test.go:387-535` (tab bar and fit tests), `internal/ui/installed_test.go:58-85`, goldens

**Interfaces:**
- Consumes: `sidebarView`, `sidebarW`, `entries`, `a.sel`, `a.sideFocus`.
- Produces: `const sidebarBeside = sidebarW + 60`; `func (a *App) contentWidth() int`; `idle bool` on `installedTab`, `listTab`, `discoverTab`, `homeTab`; `const aboutHeight = 1`; `func about(st theme.Styles, s string, w int) string`.

- [ ] **Step 1: Write the failing tests.** In `internal/ui/app_test.go`, replace `TestTabBarBadges` and `TestTabBarCountsEveryFailingService` with:

```go
func TestSidebarBadges(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	got := squash(stripANSI(a.render()))
	if !strings.Contains(got, "Outdated ↑2") || !strings.Contains(got, "Services ✗1") {
		t.Errorf("sidebar badges missing: %q", got)
	}
}

func TestSidebarCountsEveryFailingService(t *testing.T) {
	src := newFakeSource(t)
	src.snap.Services = append(src.snap.Services, brew.Service{Name: "mysql", Status: "error"})
	a := newTestApp(t, src)
	drain(t, a, a.Init())
	if got := squash(stripANSI(a.render())); !strings.Contains(got, "Services ✗2") {
		t.Errorf("want ✗2 for two failing services: %q", got)
	}
}

func TestNarrowWindowShowsTheSidebarOrTheTab(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 70, Height: 30})
	keys(t, a, "j") // All, the sidebar still focused
	if out := stripANSI(a.render()); !strings.Contains(out, "PACKAGES") || strings.Contains(out, "NAME") {
		t.Errorf("narrow, sidebar focused: the sidebar alone\n%s", out)
	}
	keys(t, a, "l")
	if out := stripANSI(a.render()); strings.Contains(out, "PACKAGES") || !strings.Contains(out, "NAME") {
		t.Errorf("narrow, list focused: the list alone\n%s", out)
	}
}
```

In `TestViewFitsTerminal` (`:414`) and the size-sweep test (`:455-470`), replace `for tab := range tabNames { a.tab = tab` with `for i, e := range entries { settle(t, a, a.selectEntry(i))`, and `tabNames[tab]` in messages with `e.label`. Replace the narrow test at `:505` that measured `a.tabBar()` with one checking that `lipgloss.Width` of every line of `a.render()` is `<= 40`. In the stale-data tests at `:518` and `:531`, change `squash(stripANSI(a.tabBar()))` to `squash(stripANSI(a.render()))` and `"4 Services ⚠"` to `"Services ⚠"`.

In `internal/ui/installed_test.go`, replace the two view-bar tests (`:58-85`) with:

```go
func TestInstalledTitleNamesTheViewAndSaysWhatItShows(t *testing.T) {
	it := newFixtureInstalled(t)
	it.filter = filterCasks
	it.apply()
	it.resize(120, 30)
	out := stripANSI(it.view(nil))
	for _, want := range []string{"Casks", "macOS apps and fonts (casks)."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "f/F") {
		t.Errorf("no view keys any more:\n%s", out)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail.**

Run: `go test ./internal/ui -run 'SidebarBadges|SidebarCounts|NarrowWindow|InstalledTitle' -count=1`
Expected: FAIL. The render still has the tab bar ("3 Outdated ↑2"), the narrow window shows the tab, and the Installed pane still shows "f/F to switch".

- [ ] **Step 3: Implement the layout.** In `internal/ui/app.go`:

Delete `tabNames` and `tabBar()`. Replace `bodyHeight`:

```go
// bodyHeight is the space above the status bar.
func (a *App) bodyHeight() int { return max(a.height-1, 3) }
```

Add:

```go
// sidebarBeside is the narrowest window with room for the sidebar beside
// a list of 60 columns.
const sidebarBeside = sidebarW + 60

// contentWidth is the width left for the tab: the whole window when it's
// too narrow for the sidebar to sit beside the tab.
func (a *App) contentWidth() int {
	if a.width >= sidebarBeside {
		return a.width - sidebarW
	}
	return a.width
}
```

In `layout`, after `a.tabH` is set, size the tabs with `cw := a.contentWidth()` instead of `a.width`:

```go
	cw := a.contentWidth()
	a.installed.resize(cw, a.tabH)
	for _, t := range []*listTab{&a.outdated, &a.services, &a.taps, &a.jobs} {
		t.resize(cw, a.tabH)
	}
	a.discover.resize(cw, a.tabH)
```

In `render`, pass `a.contentWidth()` instead of `a.width` to `a.home.view`. Before the switch that builds `body`, set which panes look focused:

```go
	idle := a.sideFocus
	a.home.idle, a.installed.idle, a.discover.idle = idle, idle, idle
	a.outdated.idle, a.services.idle, a.taps.idle, a.jobs.idle = idle, idle, idle, idle
```

After the switch, before `drawer := a.drawerShown()`, put the sidebar in:

```go
	switch {
	case bodyH == 0:
	case a.width >= sidebarBeside:
		body = lipgloss.JoinHorizontal(lipgloss.Top, a.sidebarView(sidebarW, bodyH), body)
	case a.sideFocus:
		body = a.sidebarView(a.width, bodyH)
	}
```

Replace `parts := []string{a.tabBar()}` with `var parts []string`, and fix the comment on the `bodyH > 0` line to say "the drawer takes the whole body: no room for the tab".

In `statusBar`, change `left := st.Mode(badge).Render(badge) + " " + tabNames[a.tab]` to `left := st.Mode(badge).Render(badge) + " " + entries[a.sel].label`.

- [ ] **Step 4: Make the panes go idle while the sidebar has the focus.** Add a field to each of `installedTab`, `listTab`, `discoverTab` and `homeTab`:

```go
	idle bool // the sidebar has the focus: no pane here looks focused
```

- `installed.go` `view`: the list pane's `focused` becomes `!t.pv.focus && !t.idle`; the error pane's `true` becomes `!t.idle`.
- `discover.go` `view`: the same two changes.
- `list.go` `view`: every `pane(…, true)` becomes `pane(…, !t.idle)`. The preview pane's `sp.listW == 0` becomes `sp.listW == 0 && !t.idle`.
- `home.go` `view`: a tile is focused when `i == h.sel && !h.idle`.

- [ ] **Step 5: Replace the view bar with an about line.** In `internal/ui/components.go`, add:

```go
// aboutHeight is the line under a list's title saying what its view shows.
const aboutHeight = 1

// about is that line, cut to w.
func about(st theme.Styles, s string, w int) string {
	return st.Muted.Render(ansi.Truncate(s, w, "…"))
}
```

In `internal/ui/installed.go`:
- `views` loses `short`: `var views = [...]struct{ name, about string }{ … }`, with the same names and about lines as Task 1.
- `resize` and `view`: `viewBarHeight` becomes `aboutHeight`.
- `view`: `body := t.viewBar(iw) + "\n"` becomes `body := about(t.st, views[t.filter].about, iw) + "\n"`, and `title := "Installed"` becomes `title := views[t.filter].name`.
- Delete `(*installedTab).viewBar`.

In `internal/ui/discover.go`:
- `discoverViews` loses `short`, and `discAll`'s name becomes `"Search all"`.
- `size` and `view`: `viewBarHeight` becomes `aboutHeight`.
- `view`: `body := t.viewBar(iw) + "\n"` becomes `body := about(t.st, discoverViews[t.showing].about, iw) + "\n"`, and `title := "Discover"` becomes `title := discoverViews[t.showing].name`. The error pane keeps `"Discover"`.
- Delete `(*discoverTab).viewBar`.

Delete `internal/ui/viewbar.go`. Leave `theme.Styles.View`, `ViewActive`, `Tab` and `TabRule` if anything else uses them (`grep -rn 'st\.View\b\|ViewActive\|st\.Tab\b\|TabRule' internal`). Otherwise remove them from `internal/theme/theme.go`, together with their lines in `New`.

- [ ] **Step 6: Run the tests, and regenerate the goldens.**

```bash
go test ./internal/ui -count=1
go test ./internal/ui -run TestScreens -update -count=1
git diff internal/ui/testdata/golden | head -80
```

Expected: every test passes except `TestScreens` before `-update`. In the golden diff, the tab bar is gone, the sidebar is on the left, the list titles name the entry ("Top-level", "Search all"), and the status bar's trail names the entry. If the sidebar runs past the bottom of the 40-row screen, something added lines; it needs 25 rows.

- [ ] **Step 7: Verify.**

Run: `go test ./... && go vet ./... && go vet -tags screenshots ./internal/ui && golangci-lint run ./...`
Expected: all pass.

Then look at it. Run `make build && ./brewery` in a terminal at least 122 columns wide and check:
1. It opens on Home with the sidebar's border purple.
2. `j` walks the entries and the list follows.
3. `l` moves into the list, `l` again into the details, and `h` back out each time.
4. `4` goes to Discover and `[`/`]` step through entries.
5. Narrowed below 82 columns, it shows the sidebar or the tab, whichever has the focus.

- [ ] **Step 8: Commit.**

```bash
git add -A internal/ui internal/theme
git commit -m "feat(ui): the sidebar replaces the tab bar; lists name their view"
```

---

### Task 5: README and screenshots

**Files:**
- Modify: `README.md` (Tabs → Sidebar; Keys)
- Modify: `internal/ui/screenshots_test.go` (only if Task 3's mapping left a screenshot showing the wrong place)
- Regenerate: `docs/screenshots/*.ansi`, `docs/screenshots/*.png`

- [ ] **Step 1: Rewrite the README's Tabs section** as:

```markdown
## Sidebar

- **Home**: counts, doctor's warnings, recent jobs
- **Packages**: All, Top-level, Formulae, Casks, Dependencies, Outdated, Pinned,
  each with a preview of the selected package
- **Manage**: Services, with each service's log; Taps, where `x` untaps one
- **Discover**: Popular tools, Popular apps, Rising, New, Fonts, and Search all
  for the whole catalogue
```

- [ ] **Step 2: Update the Keys table.** Replace these rows:

| Old row | New row(s) |
|---|---|
| `` `1`–`6`, `[` `]` `` · switch tabs | `` `h` `l` `` · move between the sidebar, the list and the preview; `` `1`–`4` `` · Home, Packages, Manage, Discover; `` `[` `]` `` · previous or next entry |
| `` `h` `l` `` · move between Home's tiles; `enter` opens one | `` `j` `k` `` on Home · move between its tiles; `enter` opens one |
| `` `f` `F` `` · next or previous view | delete |
| `` `s` `S` `R` `` · …; `l` follows its log | …; `L` follows its log |
| `` `tab` `enter` `` · preview, then `j` `k` scroll | `` `l` `tab` `enter` `` · preview, then `j` `k` scroll |

Update the screenshot captions that say "tab": "The Installed tab: …" becomes "Top-level: …", and "The Discover tab: …" becomes "Popular tools: …".

- [ ] **Step 3: Regenerate the screenshots.**

```bash
command -v freeze && make screenshots
```

If `freeze` isn't installed, run only `go test -tags screenshots -count=1 -run TestScreenshots ./internal/ui` to rewrite the `.ansi` files, and tell the user that the PNGs need `go install github.com/charmbracelet/freeze@v0.2.2` and `make screenshots`. Don't install tools without asking. Read each PNG (or `.ansi`) to check that it shows the sidebar and the right entry.

- [ ] **Step 4: Commit.**

```bash
git add README.md docs/screenshots internal/ui/screenshots_test.go
git commit -m "docs: the README describes the sidebar"
```

---

## After this plan

Steps 2 (sliding columns, `split.go` replaced) and 3 (History and Maintenance) of the spec get their own plans, written against the code this plan leaves.
