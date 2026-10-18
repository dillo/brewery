# Sliding Columns Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Step 2 of the sidebar layout: the sidebar, list and details become three columns that slide with the focus, replacing today's list/preview split (`split.go`).

**Architecture:** One `focus column` on the App (sidebar, list, details) replaces `sideFocus`, `scroller.focus` and `listTab.pvOpen`. A pure `columnsFor(w, focus, hasDetails)` decides which columns show and how wide each is. Tabs are told their list and details widths by the app and draw only those panes; the app puts the sidebar to their left.

**Tech Stack:** Go 1.26, Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2, bubbles `table`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-10-17-sidebar-layout-design.md`, section "2. The sliding columns".
- Minimum widths: sidebar 22 (`sidebarW`), list 60, details 40. Home's content counts as one column with a minimum of 60, in the list's place, with no details column.
- The rule: (1) the focused column is always shown, at full width if it's the only one; (2) add the column to its right if the minimum widths still fit, then the column to its left; (3) the sidebar keeps 22; what's left is split 55/45 between the list and the details, or goes to the one shown.
- Bands, from the rule (the spec's table is corrected to these in Task 1): ≥122 all three for any focus; 100–121 focus sidebar → sidebar, list; focus list or details → list, details; 82–99 focus sidebar or list → sidebar, list; focus details → details; <82 the focused column alone.
- Tabs with details: Installed, Discover, Services. Without: Home, Outdated, Taps, the jobs view.
- Goes: `splitFor`, `split`, `sideBySideWidth`, `previewMinWidth`, `minPreviewH`, `pvOpen`, `previewAlone`, `previewShown`, `servicesPreviewAlone`, `sidebarBeside`, `contentWidth`, the `idle` fields, and their special cases in key handling and `esc`.
- Unchanged: the drawer spans the full width under the columns; confirm prompts, help and the `:` popup overlay the body; Discover's pause before asking brew for details; vim keys; status bar; colours.
- `[` `]` and `1`–`4` keep the focus in its column. A tab without details takes a details focus as the list.
- Code style: match the surrounding code — short doc comments in plain English on every declaration, no new dependencies.
- Checks before every commit: `go test ./... -count=1`, `go vet ./...`, `golangci-lint run` all clean.
- Golden files: `go test ./internal/ui -run TestScreens -update -count=1` rewrites them; read the diff before committing.
- Commits authored by the repo's git config; messages in the repo's style (`feat(ui): …`, `docs: …`, `test(ui): …`).

---

### Task 1: The column rule

**Files:**
- Create: `internal/ui/columns.go`
- Create: `internal/ui/columns_test.go`
- Modify: `docs/superpowers/specs/2026-10-17-sidebar-layout-design.md` (the band table in section 2)

**Interfaces:**
- Produces: `type column int` with `colSidebar`, `colList`, `colDetails`; `type columns struct{ side, list, details int }`; `func columnsFor(w int, focus column, hasDetails bool) columns`; constants `listMinW = 60`, `detailsMinW = 40`.

- [ ] **Step 1: Write the failing test**

`internal/ui/columns_test.go`:

```go
package ui

import "testing"

func TestColumnsFor(t *testing.T) {
	for _, tt := range []struct {
		w          int
		focus      column
		hasDetails bool
		want       columns
	}{
		// ≥122: all three, whatever has the focus
		{140, colSidebar, true, columns{22, 64, 54}},
		{140, colList, true, columns{22, 64, 54}},
		{140, colDetails, true, columns{22, 64, 54}},
		{122, colList, true, columns{22, 55, 45}},
		// 100–121: the sidebar and list, or the list and details
		{121, colSidebar, true, columns{22, 99, 0}},
		{121, colList, true, columns{0, 66, 55}},
		{121, colDetails, true, columns{0, 66, 55}},
		{100, colList, true, columns{0, 55, 45}},
		{100, colSidebar, true, columns{22, 78, 0}},
		// 82–99: the sidebar and list, or the details alone
		{99, colSidebar, true, columns{22, 77, 0}},
		{99, colList, true, columns{22, 77, 0}},
		{99, colDetails, true, columns{0, 0, 99}},
		{82, colList, true, columns{22, 60, 0}},
		// <82: the focused column alone, at full width
		{81, colSidebar, true, columns{81, 0, 0}},
		{81, colList, true, columns{0, 81, 0}},
		{81, colDetails, true, columns{0, 0, 81}},
		// no details (Home, Outdated, Taps, jobs): sidebar and list, or one
		{140, colList, false, columns{22, 118, 0}},
		{140, colDetails, false, columns{22, 118, 0}}, // a details focus is the list's
		{82, colSidebar, false, columns{22, 60, 0}},
		{81, colSidebar, false, columns{81, 0, 0}},
		{81, colList, false, columns{0, 81, 0}},
	} {
		if got := columnsFor(tt.w, tt.focus, tt.hasDetails); got != tt.want {
			t.Errorf("columnsFor(%d, %d, %v) = %+v, want %+v", tt.w, tt.focus, tt.hasDetails, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/ui -run TestColumnsFor -count=1`
Expected: FAIL to compile, `undefined: column`.

- [ ] **Step 3: Write the rule**

`internal/ui/columns.go`:

```go
package ui

// column is where the focus is: the sidebar, the list, or the details of
// the list's selected row.
type column int

const (
	colSidebar column = iota
	colList
	colDetails
)

// The narrowest the list and the details may be. Home's content takes the
// list's place, and its minimum.
const (
	listMinW    = 60
	detailsMinW = 40
)

// columns is how wide each column is across the body; 0 means it isn't
// shown.
type columns struct{ side, list, details int }

// columnsFor lays out the columns in a window w wide with the focus on
// focus. hasDetails is false for a tab with nothing beside its list, which
// takes a details focus as the list's. The focused column always shows;
// then the column to its right joins if the minimum widths still fit, then
// the one to its left, until neither does. The sidebar keeps sidebarW, and
// the list and the details share the rest 55/45.
func columnsFor(w int, focus column, hasDetails bool) columns {
	mins := [...]int{colSidebar: sidebarW, colList: listMinW, colDetails: detailsMinW}
	last := colDetails
	if !hasDetails {
		last, focus = colList, min(focus, colList)
	}
	lo, hi, used := focus, focus, mins[focus]
	for grew := true; grew; {
		grew = false
		if hi < last && used+mins[hi+1] <= w {
			hi, used, grew = hi+1, used+mins[hi+1], true
		}
		if lo > colSidebar && used+mins[lo-1] <= w {
			lo, used, grew = lo-1, used+mins[lo-1], true
		}
	}
	if lo == hi && lo == colSidebar {
		return columns{side: w} // the sidebar alone fills the window
	}
	var c columns
	rest := w
	if lo == colSidebar {
		c.side, rest = sidebarW, w-sidebarW
	}
	switch {
	case lo <= colList && hi == colDetails:
		c.list = rest * 55 / 100
		c.details = rest - c.list
	case hi == colDetails:
		c.details = rest
	default:
		c.list = rest
	}
	return c
}
```

- [ ] **Step 4: Run it to see it pass**

Run: `go test ./internal/ui -run TestColumnsFor -count=1`
Expected: PASS. Then `golangci-lint run ./internal/ui/...` (unused-code warnings for the new names are expected until Task 3 uses them; if `unused` fails the run, that is the only acceptable finding).

- [ ] **Step 5: Correct the spec's band table**

In `docs/superpowers/specs/2026-10-17-sidebar-layout-design.md`, section "2. The sliding columns", the rule gives list + details from 100 columns (60 + 40), so replace the two middle rows of the table:

```
| 100–121 | sidebar, list | list, details | list, details |
| 82–99 | sidebar, list | sidebar, list | details |
```

- [ ] **Step 6: Commit**

```bash
git add internal/ui/columns.go internal/ui/columns_test.go docs/superpowers/specs/2026-10-17-sidebar-layout-design.md
git commit -m "feat(ui): the rule for which columns show at each width"
```

---

### Task 2: One focus for the whole app

The split layout stays for now; this task only changes where the focus lives. Behaviour is the same as today except where noted.

**Files:**
- Modify: `internal/ui/app.go`, `sidebar.go`, `home.go`, `installed.go`, `discover.go`, `list.go`, `scroll.go`, `actions.go` (`actionHints`), `help.go`, `links.go` (`gKey`), `jobsview.go`
- Test: `internal/ui/sidebar_test.go`, `helpers_test.go`, `details_test.go`, `installed_test.go`, `scroll_test.go`, `split_test.go`, plus a new test in `sidebar_test.go`

**Interfaces:**
- Consumes: `column`, `colSidebar`, `colList`, `colDetails` from Task 1.
- Produces:
  - `App.focus column` (replaces `App.sideFocus`); starts at `colSidebar`.
  - `func hasDetails(tab int) bool` in `columns.go`: true for `tabInstalled`, `tabDiscover`, `tabServices`.
  - `func (a *App) hasDetails() bool`: `!a.jobsOpen && hasDetails(a.tab)`.
  - Views take the focus instead of reading an `idle` field: `installedTab.view(err error, focus column)`, `discoverTab.view(focus column)`, `listTab.view(err error, focus column)`, `homeTab.view(snap, recent, now, w, h int, focus column)`. A pane looks focused when `focus` names it: the list pane when `focus == colList`, the details pane when `focus == colDetails`; nothing when `focus == colSidebar`. Home's tiles and panes look focused when `focus != colSidebar`.
  - `installedTab.scroll(k string)` and `discoverTab.scroll(k string)`: scroll the preview by a key (page = half the preview's inner height, as `update` computes it today). Their `update(msg)` now only moves the table.
  - `scroller` loses its `focus` field.
  - `listTab` loses `pvOpen` and `previewShown`; `previewAlone` stays until Task 3.

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/sidebar_test.go` (it fails to compile until `a.focus` exists):

```go
func TestEntryKeysKeepTheFocusInItsColumn(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "All")
	keys(t, a, "l")
	if a.focus != colDetails {
		t.Fatalf("l from the list: focus %d, want details", a.focus)
	}
	keys(t, a, "]") // Top-level: Installed again, with details
	if label(a) != "Top-level" || a.focus != colDetails {
		t.Errorf("] keeps the details focus: %s, focus %d", label(a), a.focus)
	}
	keys(t, a, "4") // Popular tools: Discover has details too
	if a.tab != tabDiscover || a.focus != colDetails {
		t.Errorf("4 keeps the details focus: tab %d, focus %d", a.tab, a.focus)
	}
	keys(t, a, "3") // Services
	if a.focus != colDetails {
		t.Errorf("3 keeps the details focus on Services: %d", a.focus)
	}
	keys(t, a, "]") // Taps has no details: the focus is its list's
	if label(a) != "Taps" || a.focus != colList {
		t.Errorf("] onto Taps: %s, focus %d, want the list", label(a), a.focus)
	}
	keys(t, a, "h")
	if a.focus != colSidebar {
		t.Errorf("h from the list: focus %d, want the sidebar", a.focus)
	}
	keys(t, a, "2")
	if a.focus != colSidebar {
		t.Errorf("2 from the sidebar keeps the sidebar focus: %d", a.focus)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/ui -run TestEntryKeysKeepTheFocusInItsColumn -count=1`
Expected: FAIL to compile, `a.focus undefined`.

- [ ] **Step 3: Move the focus onto the App**

1. `columns.go`: add

```go
// hasDetails reports whether tab shows the selected row's details beside
// its list.
func hasDetails(tab int) bool {
	return tab == tabInstalled || tab == tabDiscover || tab == tabServices
}
```

and in `app.go`

```go
// hasDetails reports whether what's showing has a details column.
func (a *App) hasDetails() bool { return !a.jobsOpen && hasDetails(a.tab) }
```

2. `app.go`: replace the field `sideFocus bool // keys go to the sidebar, not the tab` with `focus column // the sidebar, the list or the details`, and `sideFocus: true` in `New` with `focus: colSidebar`. Replace every `a.sideFocus` read with `a.focus == colSidebar`, every `a.sideFocus = true` with `a.focus = colSidebar`, and every `a.sideFocus = false` with `a.focus = colList` (in `sidebar.go`, `jobsview.go`, `links.go`, `help.go`, `actions.go`, `app.go`).

3. `scroll.go`: delete the `focus` field and its comment; the doc comment becomes `// scroller is how far a preview has scrolled.`

4. `installed.go` and `discover.go`: delete the `idle` field. Split `update`:

```go
// update moves the table's selection.
func (t *installedTab) update(msg tea.KeyPressMsg) tea.Cmd {
	before := t.selectedKey()
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	t.followSelection(before)
	return cmd
}

// scroll scrolls the preview by a key.
func (t *installedTab) scroll(k string) {
	sp := splitFor(t.w, t.h, true)
	_, ih := paneInner(sp.pvW, sp.pvH)
	t.pv.scroll(k, ih/2)
}
```

(the same pair on `discoverTab`). `view` gains `focus column`: use `splitFor(w, h, focus == colDetails)`, the list pane is focused when `focus == colList`, the preview when `focus == colDetails`, and the error pane when `focus != colSidebar`.

5. `list.go`: delete `idle`, `pvOpen` and `previewShown`; `update` becomes only the table update; `layout()` becomes `layout(showPreview bool)` returning `splitFor(t.w, t.h, showPreview)` when there's a preview; `resize` drops the `pvOpen` reset; `view(err error, focus column)` lays out with `t.layout(focus == colDetails)`, focuses the list pane when `focus == colList` and the preview when `focus == colDetails`, and the full-size state panes when `focus != colSidebar`.

6. `home.go`: delete `idle`; `view` gains `focus column` and uses `focus != colSidebar` where it used `!h.idle`.

7. `app.go` `render`: delete the two lines that set `idle`, and pass `a.focus` into each view. `a.jobs.view(nil, colList)` when the jobs view is open (its list always has the focus there).

8. `sidebar.go`: `focusDetails` and `unfocusDetails` become

```go
// focusDetails moves the focus from the list to the details, on tabs
// that have them.
func (a *App) focusDetails() {
	if a.hasDetails() {
		a.focus = colDetails
	}
}
```

Delete `unfocusDetails`. The sidebar's `l` / `enter` / `tab` case sets `a.focus = colList`.

9. `app.go` `globalKey`: `h` / `left` become

```go
	case "h", "left":
		a.focus = max(a.focus-1, colSidebar)
```

and add `"tab"` and `"enter"` handling to the list/details routing in `tabKey`:

```go
// tabKey gives the key to the current tab: the list's keys move its
// selection, and once the details have the focus, keys scroll them. l,
// tab and enter go from the list to the details; tab goes back.
func (a *App) tabKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	switch {
	case a.focus == colList && (k == "tab" || k == "enter") && a.hasDetails():
		a.focus = colDetails
		return nil
	case a.focus == colDetails && k == "tab":
		a.focus = colList
		return nil
	case a.focus == colDetails:
		switch a.tab {
		case tabInstalled:
			a.installed.scroll(k)
		case tabDiscover:
			a.discover.scroll(k)
		} // Services' details show the end of the log, which L follows: nothing to scroll
		return nil
	}
	switch a.tab {
	... // today's table dispatch
	}
	return nil
}
```

10. `escape()`: the three details cases become one first case, `case a.focus == colDetails: a.focus = colList`.

11. `servicesPreviewAlone()` becomes `a.tab == tabServices && !a.jobsOpen && a.focus == colDetails && a.services.previewAlone()`. The guard in `handleKey` drops its `a.services.pvOpen = false` line and becomes: `if k == "tab" || k == "enter" { a.focus = colList }`.

12. `discover.go` `showTab`: replace the reset of `pv.focus` with

```go
	if a.focus == colDetails && !hasDetails(t) {
		a.focus = colList // the new tab has no details: its list has the focus
	}
```

13. `actionHints` and `helpGroups`: `a.installed.pv.focus` / `a.discover.pv.focus` become `a.focus == colDetails`.

14. `homeKey`: `leaveHome` sets `a.focus = colSidebar`.

- [ ] **Step 4: Migrate the tests**

In the test files listed above: `a.sideFocus` → `a.focus == colSidebar` (and `!a.sideFocus` → `a.focus != colSidebar`); `a.installed.pv.focus` / `a.discover.pv.focus` → `a.focus == colDetails`, and assignments to them → `a.focus = colDetails` / `colList`; `helpers_test.go` `show` sets `a.focus = colList`; `a.services.pvOpen = true` in `TestStalePreviewFlagDoesNotSwallowEsc` → delete that test (with one focus there is no stale flag left to test); `a.services.previewShown()` → `a.servicesPreviewAlone()`; `a.installed.update(...)` calls in tests that toggled the preview go through `keys(t, a, "tab")` instead. Every test's intent stays the same.

- [ ] **Step 5: Run everything**

Run: `go test ./... -count=1 && go vet ./... && golangci-lint run`
Expected: all PASS, including `TestEntryKeysKeepTheFocusInItsColumn`; golden files unchanged (`git status` shows no change under `testdata/golden`).

- [ ] **Step 6: Commit**

```bash
git add -A internal/ui
git commit -m "feat(ui): one focus for the sidebar, list and details; [ ] and 1-4 keep it"
```

---

### Task 3: The columns replace the split

**Files:**
- Modify: `internal/ui/app.go`, `installed.go`, `discover.go`, `list.go`, `columns.go`, `sidebar.go`, `filter.go` (or wherever `openFilter` lives)
- Delete: `internal/ui/split.go`, `internal/ui/split_test.go`
- Test: `internal/ui/columns_test.go` (app-level tests), `run_test.go` (`newJobApp` size), golden files

**Interfaces:**
- Consumes: `columnsFor`, `columns`, `hasDetails`, `App.focus` from Tasks 1–2.
- Produces:
  - `func (a *App) columnsFor(tab int) columns`: `columnsFor(a.width, a.focus, hasDetails(tab) && !a.jobsOpen)`.
  - Tab sizing: `installedTab.resize(listW, pvW, h int)`, `discoverTab.resize(listW, pvW, h int)`, `listTab.resize(listW, pvW, h int)`, storing `listW`, `pvW`, `h` in place of `w`, `h`. A table whose list is hidden (`listW == 0`) is sized to `pvW` so it still takes navigation.
  - `func joinColumns(panes ...string) string` in `columns.go`: `lipgloss.JoinHorizontal(lipgloss.Top, …)` of the non-empty panes.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/columns_test.go`:

```go
func TestColumnsSlideWithTheFocus(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	show(t, a, "Top-level")
	out := stripANSI(a.render())
	if !strings.Contains(out, "PACKAGES") || !strings.Contains(out, "NAME") || strings.Contains(out, "App to build and share") {
		t.Errorf("at 90 columns the list sits beside the sidebar, without details:\n%s", out)
	}
	keys(t, a, "l")
	out = stripANSI(a.render())
	if strings.Contains(out, "NAME") || strings.Contains(out, "PACKAGES") || !strings.Contains(out, "App to build and share") {
		t.Errorf("l turns the page to the details, alone:\n%s", out)
	}
	keys(t, a, "h")
	if out := stripANSI(a.render()); !strings.Contains(out, "NAME") || !strings.Contains(out, "PACKAGES") {
		t.Errorf("h turns back to the sidebar and list:\n%s", out)
	}

	a.Update(tea.WindowSizeMsg{Width: 110, Height: 30})
	keys(t, a, "l")
	out = stripANSI(a.render())
	if strings.Contains(out, "PACKAGES") || !strings.Contains(out, "NAME") || !strings.Contains(out, "App to build and share") {
		t.Errorf("at 110 columns the details push the sidebar out:\n%s", out)
	}
}

func TestRowKeysWorkFromTheDetailsAlone(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	show(t, a, "Services")
	keys(t, a, "l")
	if out := stripANSI(a.render()); !strings.Contains(out, "End of the log") || strings.Contains(out, "STATUS") {
		t.Fatalf("l shows the details alone:\n%s", out)
	}
	name, _, _ := a.selection()
	keys(t, a, "S")
	if a.confirm == nil || !strings.Contains(a.confirm.title, name) {
		t.Errorf("S from the details asks to stop %s: %+v", name, a.confirm)
	}
	keys(t, a, "n", "esc")
	if out := stripANSI(a.render()); !strings.Contains(out, "STATUS") {
		t.Errorf("esc goes back to the list:\n%s", out)
	}
}

func TestFilterFromTheDetailsGoesToTheList(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	show(t, a, "All")
	keys(t, a, "l", "/")
	if a.mode != modeFilter || a.focus != colList {
		t.Errorf("/ from the details filters the list, with the focus on it: mode %d, focus %d", a.mode, a.focus)
	}
}
```

(imports: `strings`, `tea "charm.land/bubbletea/v2"`.) If the fixture's preview text or service names differ, use the strings `TestInstalledStacksThenHidesThePreview` and `TestServicesPreviewTakesTurnsWhenNarrow` in `split_test.go` use today — they come from the same fixtures. The sidebar is detected by its `PACKAGES` heading. If `S` on the first fixture service only flashes a note (a stopped service has nothing to stop), move to a running one with `j` first, as `TestServicesPreviewAloneIgnoresListKeys` does.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/ui -run 'TestColumnsSlide|TestRowKeysWorkFromTheDetails|TestFilterFromTheDetails' -count=1`
Expected: FAIL (at 90 columns today the preview stacks under the list; S is swallowed under the services preview).

- [ ] **Step 3: Lay out by columns**

1. `app.go` `layout()`: the cache key also covers what the columns depend on: `laidOut [6]int // width, height, drawer state, focus, tab and jobs view of the last layout`, compared against `[6]int{a.width, a.height, shown, int(a.focus), a.tab, jobs}` with `jobs` 1 when `a.jobsOpen`. Size each tab with its own columns:

```go
	for tab, resize := range map[int]func(listW, pvW, h int){
		tabInstalled: a.installed.resize, tabOutdated: a.outdated.resize,
		tabServices: a.services.resize, tabTaps: a.taps.resize, tabDiscover: a.discover.resize,
	} {
		c := a.columnsFor(tab)
		resize(c.list, c.details, a.tabH)
	}
	c := columnsFor(a.width, a.focus, false)
	a.jobs.resize(c.list, 0, a.tabH)
```

and add

```go
// columnsFor is how the columns share the window for tab, as the focus
// stands.
func (a *App) columnsFor(tab int) columns {
	return columnsFor(a.width, a.focus, hasDetails(tab) && !a.jobsOpen)
}
```

2. `app.go` `render`: compute `c := a.columnsFor(a.tab)`; Home draws at `c.list` wide; then replace the sidebar `switch` with

```go
	if bodyH > 0 && c.side > 0 {
		body = joinColumns(a.sidebarView(c.side, bodyH), body)
	}
```

where `body` is `""` when `c.list == 0 && c.details == 0` (the sidebar alone). Delete `sidebarBeside` and `contentWidth`.

3. `installed.go`, `discover.go`, `list.go`: `resize(listW, pvW, h int)` stores them and sizes the table to `paneInner(cmp.Or(listW, pvW), h)` (minus `aboutHeight` where it is today). `view` draws the list pane at `t.listW × t.h` when `t.listW > 0` and the details pane at `t.pvW × t.h` when `t.pvW > 0`, and returns `joinColumns(list, details)`. The whole-tab state panes (error with nothing loaded, loading, empty, nothing matches) span `t.listW + t.pvW`. Discover before the catalogue has loaded draws only the list, across `t.listW + t.pvW`. `scroll` takes its page from `paneInner(t.pvW, t.h)`. `installedTab.listWidth()` is deleted; its one test reads `a.installed.listW`.

4. Delete `split.go`, `listTab.previewAlone`, `App.servicesPreviewAlone`, the guard in `handleKey` that uses it, and the `(k == "g" || k == "/") && a.servicesPreviewAlone()` line in `gPrefixKey`.

5. `openFilter`: when `a.focus == colDetails`, set `a.focus = colList` before opening the filter (the filter is the list's; at narrow widths the details hide it).

6. `columns.go`:

```go
// joinColumns puts the shown columns side by side.
func joinColumns(panes ...string) string {
	var shown []string
	for _, p := range panes {
		if p != "" {
			shown = append(shown, p)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, shown...)
}
```

7. `run_test.go` `newJobApp`: size the app at `140×40` instead of `120×40`, the width the goldens and screenshots use (at 120 the list's focus hides the sidebar now, which most tests don't mean to test). `golden_test.go`: update the `TestScreens` doc comment to say 140×40.

- [ ] **Step 4: Fix the tests the layout breaks**

Delete `split_test.go`: `TestSplitFor` is replaced by `TestColumnsFor`; the narrow-window behaviour its other tests covered is in the Step 1 tests. Run `go test ./internal/ui -count=1` and, for each other failure, decide whether the test asserts the old layout (update the expected layout to the column rule) or something else that broke (fix the code). List each changed test and why in the report.

- [ ] **Step 5: Regenerate the goldens and read them**

Run: `go test ./internal/ui -run TestScreens -update -count=1 && git diff --stat internal/ui/testdata/golden`
Check by eye in `git diff internal/ui/testdata/golden`: every screen is 140 wide with the sidebar on the left; Installed and Discover show list and details side by side; Home's tiles span the rest beside the sidebar; help still overlays.

- [ ] **Step 6: Run everything**

Run: `go test ./... -count=1 && go vet ./... && golangci-lint run && grep -rn 'splitFor\|sideBySideWidth\|previewMinWidth\|minPreviewH\|pvOpen\|previewAlone\|previewShown\|servicesPreviewAlone\|sidebarBeside\|contentWidth\|idle' internal/ui`
Expected: tests, vet and lint clean; the grep prints nothing.

- [ ] **Step 7: Commit**

```bash
git add -A internal/ui
git commit -m "feat(ui): the sidebar, list and details slide with the focus"
```

---

### Task 4: A narrow golden, help and the README

**Files:**
- Modify: `internal/ui/golden_test.go`, `internal/ui/help.go`, `internal/ui/actions.go` (`actionHints`), `README.md`, `docs/screenshots/*.png` (regenerated)
- Test: `internal/ui/help_test.go`, golden files

**Interfaces:**
- Consumes: everything above.

- [ ] **Step 1: Add the narrow golden**

In `TestScreens`, after the existing screens, snapshot Top-level at 90×40, list focused, and then its details alone:

```go
	a.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	show(t, a, "Top-level")
	golden(t, "narrow-list", stripANSI(a.render()))
	keys(t, a, "l")
	golden(t, "narrow-details", stripANSI(a.render()))
```

Run: `go test ./internal/ui -run TestScreens -update -count=1`, then read both new files: `narrow-list.txt` is the sidebar and the list; `narrow-details.txt` is git's preview alone, 90 wide.

- [ ] **Step 2: Help and hints**

- `help.go`, Services group: add `{"l tab enter", "details"}` before `{"L", …}`.
- `help.go`, Everywhere group: `{"h l", "sidebar, list, details; narrow, a page at a time"}` — check it fits the 56-column box (`helpKeyW` 11 + text ≤ 56); if not, `{"h l", "sidebar, list, details"}` stays.
- `actionHints`, Services list: `"s start · S stop · R restart · L log · l details"`; Services details: `"h back to the list · s start · S stop · R restart · L log"`.
- `help_test.go`: assert the Services group lists `l tab enter`, and that `actionHints` on Services' details starts with `h back to the list`.

- [ ] **Step 3: README**

- Line 43's sentence about each list having "a preview of the selected package": say the sidebar, list and details sit side by side from 122 columns, and in a narrower window the columns slide with the focus, so `h` and `l` turn pages.
- Keys table: the `h` `l` row reads "move between the sidebar, the list and the details; in a narrow window, a page at a time". The `l` `tab` `enter` row says "details" instead of "preview".
- Regenerate the screenshots: `make screenshots FREEZE="go run github.com/charmbracelet/freeze@v0.2.2"`, and look at each PNG with the Read tool before committing.

- [ ] **Step 4: Run everything**

Run: `go test ./... -count=1 && go vet ./... && golangci-lint run`
Expected: all clean.

- [ ] **Step 5: Commit**

```bash
git add -A internal/ui README.md docs/screenshots
git commit -m "docs: the sliding columns in help, the README and the screenshots"
```
