# brewery Plan 6: Looks and size — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make brewery look right at every window size, and show more of what matters.
- **Layout:** the preview goes under the list below 100 columns and takes turns with it below 60. Narrow tables shrink their roomiest columns before names. Tiny windows keep the status bar.
- **Looks:** Nerd Font icons (`--no-icons` for plain text), colored service and job states, a selected row that stays highlighted, and placeholder rows on a first run.
- **New information:**
  - what each package takes on disk (casks measured by their apps);
  - how old brew's package data is, on the Outdated tab.
- **Plan 5's remaining review items.**

**Architecture:**
- **`internal/brew`:**
  - parses each cask's artifact targets (its apps, fonts, binaries);
  - measures sizes itself: Cellar kegs, Caskroom and targets, walked without following symlinks;
  - finds when brew last fetched package data, from the change times of brew's own files.
- **`internal/theme`:** icons, and `Tint`, a foreground-only color for text inside table cells.
- **`internal/ui`:**
  - a `split` layout shared by Installed and Services;
  - optional columns and spare-width shrinking in `sizeTable`;
  - a seventh query, sizes, which follows a full refresh and any change on disk.
- **`main`:** sets the Homebrew prefix, and turns icons on unless `--no-icons` or `BREWERY_NO_ICONS` says not to.
- **The UI still never starts a program itself.**

**Tech Stack:**
- Go ≥ 1.25
- Bubble Tea v2, Bubbles v2, Lip Gloss v2, `x/ansi`
- No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-12-brewery-design.md`:
- section 2: disk sizes, and the Outdated data's age
- section 4: responsive layout, icons, colored service status, and skeleton rows on a first run

**Roadmap (decided with the user):** CI (GitHub Actions), golangci-lint and teatest snapshots move to Plan 7, "Release". The repo has no GitHub remote yet, so Actions can't run, and golangci-lint isn't installed here. Plan 7 also covers goreleaser, the tap and renaming the module path.

**Decided with the user:**
- **Icons are on by default,** since the user's Mac has Nerd Fonts. `--no-icons`, or `BREWERY_NO_ICONS` set to anything, gives plain text. Terminals don't report their font, so the spec's "automatic detection" isn't possible.
- **A cask's size includes its app** (and fonts, binaries): every artifact target brew's JSON lists, plus its Caskroom folder. The Caskroom alone would show a few KB for most casks.

**Found while building this:**
- **The selected row's highlight stopped after its first cell.** Each table cell carried its own foreground color, and that color's closing reset (`\x1b[m`) also cleared the row's background and bold. Cells now carry no color of their own, and colored text inside a cell resets only the foreground (`theme.Tint`). A test checks that a selected row resets exactly once, at its end.
- **brew 7 keeps API data in different files from brew 4.** brew 4 wrote `api/formula.jws.json`; brew 7 writes `api/formula_names.txt` and `api/internal/packages.*.jws.json`. The data age is the newest of a few known files, including the repository's `.git/FETCH_HEAD`, which `brew update` touches.
- **Measuring sizes is quick:** every keg in the Cellar in 0.7s, and big apps (Docker.app is 2.2GB) in about half a second more. It runs in the background after each refresh.
- **Cask artifacts are all JSON objects today,** but brewery reads each one separately, so an odd shape can't break the whole Installed query.

**Verified:** every code block in this plan was compiled and tested before the plan was written.
- **The edits are exact.** A checker applied this plan's edits to `master` (8143082) and confirmed that:
  - each task reproduces the tested code byte for byte;
  - each task's tests fail before its code and pass after;
  - `go mod tidy` stays clean.
  Every task's commit passes `go test -race` on its own.
- **Against real brew (read-only):** the live tests measure the user's packages (most have a size) and read the data age.
- **The real binary** ran at 120×30, 80×24 and 50×24 (`--no-icons`), with history and cache in a scratch directory:
  - Installed showed the SIZE column (Docker Desktop 2.2GB) and icons.
  - Outdated said "brew's data from 6h ago", matching the last `brew update`.
  - At 80 columns the preview sat under the list; at 50, `enter` on Services showed the preview instead of the list.
  - Every run exited 0.

## Global Constraints

These come from Plans 1–5 and still hold:
- Module path `brewery`; Go ≥ 1.25.
- **Only `internal/brew` and `main` run brew.** Measuring sizes is filesystem work and also lives in `internal/brew`, which knows Homebrew's layout.
- Views use `theme.Styles` only, and that now includes `Styles.Icons`.
- One job at a time; lock retries; pane actions get `brew.JobEnv`.
- Key routing is by mode first (Plan 5's order).
- Refreshes are never dropped; the cache is saved before doctor runs.

Added or changed by this plan:
- **Small windows:**
  - When the drawer takes the whole body, the tab gets no line, so the status bar stays at the bottom.
  - A confirmation or `?` opened then takes the body back for that frame.
  - A status bar cut short ends with `…`.
- **Narrow tables (`sizeTable`):** columns give way in this order:
  1. fixed columns shrink a cell at a time, from whichever has the most room above its floor;
  2. optional columns (`min < 0`) are hidden;
  3. flexible columns shrink to `minFlex`;
  4. trailing columns are hidden.
  A flexible column keeps its `min` until step 3.
- **Installed columns:** NAME (flexible, keeps 20), VERSION 16 (floor 8), TYPE 9 (floor 4), STATUS 10 (floor 6), SIZE 8 (optional). Services: NAME keeps 16, STATUS 16 (floor 10). Outdated: NAME keeps 16, INSTALLED and LATEST 18 (floor 8).
- **List and preview layout (`splitFor`), shared by Installed and Services:**
  - From 100 columns: side by side, the list taking 55%.
  - From 60: the preview under the list, the list taking `h - h*45/100` lines, as long as the preview gets at least 5.
  - Otherwise: the list alone. On Installed, `tab`/`enter` shows the preview instead (the preview focus from Plan 5); on Services, `tab`/`enter` toggles it. `esc` goes back.
- **Placeholder rows:** before a list's first result with nothing cached, it shows up to 8 muted `░` rows, under "loading…" on the simple lists.
- **Icons:**
  - The Nerd Font v3 glyphs are formula `\uf0fc` (beer), cask `\U000f003b`, pinned `\U000f0403`, started `\U000f040a`, stopped `\U000f04db` and failed `\U000f0028`.
  - Each goes before its word with a space. `App.UseIcons(on)` sets them; they're off until `main` turns them on.
  - They appear in TYPE cells, "pinned", service statuses (cells and preview), but not in keys or titles.
- **Colors in cells:**
  - Table cells have no color of their own; `theme.Tint` colors text and resets only the foreground.
  - Service status: started or scheduled green, error red, none or stopped muted, anything else yellow.
  - Job status: done green, failed red, stopped yellow. Installed's `↑` is yellow.
- **Sizes:**
  - What's measured: a formula is `<prefix>/Cellar/<name>`, all kegs; a cask is `<prefix>/Caskroom/<name>` plus every artifact `target` brew lists.
  - How: the sizes of files, not following symlinks. A missing path is 0, and unreadable folders are skipped.
  - When:
    - after every full refresh (with doctor and cleanup's dry-runs);
    - after an upgrade, an uninstall or a cleanup;
    - not after a pin.
  - Stored in the snapshot by `Package.Key()` (`formula/fd`, `cask/docker-desktop`). The snapshot is now version 3.
  - Shown on Installed as the SIZE column and a `size` line in the preview, formatted like brew (`humanSize`: 1024-based, `KB`/`MB`/`GB` with one decimal). Nothing shows before a package is measured.
- **Data age:**
  - `(*Client) DataUpdated` is the newest change time of `<cache>/api/formula_names.txt`, `<cache>/api/formula.jws.json` and `<repository>/.git/FETCH_HEAD`, with the cache and repository found via `brew --cache` and `brew --repository`.
  - It's read with each Outdated result and stored as `Snapshot.DataAt`.
  - The Outdated title adds `· brew's data from <ago>`, even when everything is up to date. If the time can't be found, it says nothing.
- **From Plan 5's final review:**
  - The quit grace timer does nothing once the job has stopped.
  - Entering INSERT forgets a pending `g`.
  - `k` right after `G` moves up at once.
  - In `:jobs`, help says `y` copies the command and leaves out `gx`.
  - A relative `XDG_STATE_HOME` or `XDG_CACHE_HOME` is ignored.

## File Structure

```
internal/ui/app.go             render: no body line when the drawer has it all; questions take it back; UseIcons; dataAge; typeIntoJob
internal/ui/components.go      col.min; sizeTable phases; skeleton; humanSize
internal/ui/split.go           split, splitFor, join (new)
internal/ui/installed.go       split layout; skeleton; SIZE column, size line; icons and tint in cells
internal/ui/list.go            split layout, pvOpen, note; kindCell; rows take icons/styles; skeleton
internal/ui/services.go        serviceColor, serviceIcon
internal/ui/jobsview.go        tinted job statuses
internal/ui/source.go          Source.Sizes, Source.DataUpdated, qSizes; fetch gets the packages
internal/ui/actions.go         diskQueries
internal/ui/help.go            jobs-view Lists group
internal/ui/scroll.go          scroller.last, sized
internal/theme/theme.go        Styles.Icons; plain TableCell; Tint
internal/theme/icons.go        Icons, Nerd, With (new)
internal/brew/parse.go         cask artifact targets
internal/brew/types.go         Package.Targets, Package.Key
internal/brew/sizes.go         PrefixOf, (*Client) Sizes, dirSize (new)
internal/brew/client.go        Client.Prefix; DataUpdated, dataFiles
internal/state/snapshot.go     version 3; Sizes, DataAt; relative XDG ignored
internal/jobs/history.go       relative XDG ignored
cmd/brewery/main.go            --no-icons, wantIcons; client.Prefix
```

---

### Task 1: Keep the status bar and questions on tiny windows

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`
- Modify: `internal/ui/drawer_test.go`

**Interfaces:**
- **Consumes:** Plan 5's `drawerHeight`, `clipLines`, `render`.
- **Produces:**
  - `render` drops the tab body when it has no lines. When the drawer has the whole body, a confirmation or help takes the body back (with the drawer hidden for that frame).
  - The status bar's final truncation ends with `…`.

- [ ] **Step 1: Edit `internal/ui/app_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	if out := stripANSI(a.statusBar()); !strings.Contains(out, "…") {
		t.Errorf("a very narrow status bar should truncate the left side: %q", out)
	}
}

func TestTabBarFitsWidth(t *testing.T) {
```

   with:

```go
	if out := stripANSI(a.statusBar()); !strings.Contains(out, "…") {
		t.Errorf("a very narrow status bar should truncate the left side: %q", out)
	}
	a.flash = "a flash far too long to fit in a status bar this narrow"
	a.Update(tea.WindowSizeMsg{Width: 20, Height: 24})
	if out := strings.TrimRight(stripANSI(a.statusBar()), " "); !strings.HasSuffix(out, "…") {
		t.Errorf("a status bar cut short should end with …: %q", out)
	}
}

func TestTabBarFitsWidth(t *testing.T) {
```

- [ ] **Step 2: Edit `internal/ui/drawer_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
		if h >= 10 && !strings.Contains(stripANSI(out), "working") {
			t.Errorf("height %d: the drawer should show the job's output:\n%s", h, stripANSI(out))
		}
	}
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{newFakeProc(0)}})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
```

   with:

```go
		if h >= 10 && !strings.Contains(stripANSI(out), "working") {
			t.Errorf("height %d: the drawer should show the job's output:\n%s", h, stripANSI(out))
		}
		if lines := strings.Split(stripANSI(out), "\n"); !strings.Contains(lines[len(lines)-1], "RUNNING") {
			t.Errorf("height %d: the status bar must stay at the bottom:\n%s", h, stripANSI(out))
		}
	}
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{newFakeProc(0)}})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
```

2. Replace:

```go
		t.Errorf("at 80x24 the drawer shows %d lines of output, want 6", a.drawer.rows)
	}
}
```

   with:

```go
		t.Errorf("at 80x24 the drawer shows %d lines of output, want 6", a.drawer.rows)
	}
}

func TestQuestionsShowInTinyWindows(t *testing.T) {
	p := newFakeProc(0, "working")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 7}) // the drawer takes the whole body
	settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	a.Update(press("ctrl+c"))
	out := stripANSI(a.render())
	if !strings.Contains(out, "Stop upgrade fd?") || lipgloss.Height(out) != 7 {
		t.Errorf("the stop question must show, in 7 lines:\n%s", out)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    --- FAIL: TestStatusBarIsOneLine (0.01s)
    --- FAIL: TestFrameFitsSmallWindows (1.28s)
    --- FAIL: TestQuestionsShowInTinyWindows (0.11s)
```

- [ ] **Step 4: Edit `internal/ui/app.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
			body = a.taps.view(a.errs[qTaps])
		}
	}
	body = clipLines(body, bodyH) // a tab too tall for a small window mustn't push the rest off it
	if a.confirm != nil {
		body = overlay(body, a.confirm.view(a.st, a.width), a.width, bodyH)
```

   with:

```go
			body = a.taps.view(a.errs[qTaps])
		}
	}
	drawer := a.drawerShown()
	if bodyH == 0 && (a.confirm != nil || a.help) {
		// The drawer fills a tiny window, but a question or help must show.
		bodyH, drawer = a.bodyHeight(), false
	}
	body = clipLines(body, bodyH) // a tab too tall for a small window mustn't push the rest off it
	if a.confirm != nil {
		body = overlay(body, a.confirm.view(a.st, a.width), a.width, bodyH)
```

2. Replace:

```go
	if a.help {
		body = overlay(body, a.helpView(a.width, bodyH), a.width, bodyH)
	}
	parts := []string{a.tabBar(), body}
	if a.drawerShown() {
		parts = append(parts, a.drawer.view(a.width, a.bodyHeight()-bodyH, a.now(), a.frame, a.mode == modeInsert))
	}
	screen := clipLines(lipgloss.JoinVertical(lipgloss.Left, append(parts, a.statusBar())...), a.height)
```

   with:

```go
	if a.help {
		body = overlay(body, a.helpView(a.width, bodyH), a.width, bodyH)
	}
	parts := []string{a.tabBar()}
	if bodyH > 0 {
		parts = append(parts, body) // when the drawer takes the whole body, the tab gets no line at all
	}
	if drawer {
		parts = append(parts, a.drawer.view(a.width, a.bodyHeight()-bodyH, a.now(), a.frame, a.mode == modeInsert))
	}
	screen := clipLines(lipgloss.JoinVertical(lipgloss.Left, append(parts, a.statusBar())...), a.height)
```

3. Replace:

```go
		left = ansi.Truncate(left, max(room-1-lipgloss.Width(r), 0), "…")
	}
	gap := max(room-lipgloss.Width(left)-lipgloss.Width(r), 1)
	line := ansi.Truncate(left+strings.Repeat(" ", gap)+r+" ", a.width, "")
	return st.StatusBar.Width(a.width).Render(line)
}
```

   with:

```go
		left = ansi.Truncate(left, max(room-1-lipgloss.Width(r), 0), "…")
	}
	gap := max(room-lipgloss.Width(left)-lipgloss.Width(r), 1)
	line := ansi.Truncate(left+strings.Repeat(" ", gap)+r+" ", a.width, "…") // … says something was cut
	return st.StatusBar.Width(a.width).Render(line)
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "fix(ui): keep the status bar and questions on tiny windows; mark a cut status bar"
```

---

### Task 2: Narrow tables shrink their roomiest columns first

**Files:**
- Modify: `internal/ui/components.go`
- Modify: `internal/ui/installed.go`
- Modify: `internal/ui/jobsview.go`
- Modify: `internal/ui/list.go`
- Modify: `internal/ui/list_test.go`

**Interfaces:**
- **Consumes:** `col`, `sizeTable` (Plan 1).
- **Produces:**
  - `col{title string; width, min int}`: every column literal gains its `min`.
  - `sizeTable` shrinks fixed columns by most room first, then flexible columns to `minFlex`, then hides trailing columns.

One test's expected widths change, and its name says what it now checks.

- [ ] **Step 1: Edit `internal/ui/list_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	return n
}

func TestSizeTableShrinksFixedColumnsFromTheEnd(t *testing.T) {
	tbl := newTable(testStyles, installedCols)
	sizeTable(&tbl, installedCols, 40, 10)
	// fixed 16+7+10 plus NAME's minimum 8, plus 8 padding = 49: 9 too many.
	// STATUS shrinks to its title (6), TYPE to its title (4), VERSION by 2.
	got := []int{}
	for _, c := range tbl.Columns() {
		got = append(got, c.Width)
	}
	if want := []int{8, 14, 4, 6}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("widths = %v, want %v", got, want)
	}
	if w := tableWidth(tbl); w > 40 {
		t.Errorf("table is %d wide, want <= 40", w)
	}
}

func TestSizeTableDropsTrailingColumns(t *testing.T) {
```

   with:

```go
	return n
}

func TestSizeTableShrinksTheRoomiestColumnsFirst(t *testing.T) {
	tbl := newTable(testStyles, installedCols)
	sizeTable(&tbl, installedCols, 40, 10)
	// NAME wants 16, VERSION 16, TYPE 7, STATUS 10, plus 8 padding: 17 too
	// many. VERSION gives way first (it has the most to spare), then STATUS
	// and TYPE, each to its floor (8, 6, 4); NAME gives the last 2.
	got := []int{}
	for _, c := range tbl.Columns() {
		got = append(got, c.Width)
	}
	if want := []int{14, 8, 4, 6}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("widths = %v, want %v", got, want)
	}
	if w := tableWidth(tbl); w > 40 {
		t.Errorf("table is %d wide, want <= 40", w)
	}
	sizeTable(&tbl, installedCols, 50, 10)
	got = got[:0]
	for _, c := range tbl.Columns() {
		got = append(got, c.Width)
	}
	// 7 too many: VERSION gives 6 and STATUS 1; NAME and TYPE keep theirs.
	if want := []int{16, 10, 7, 9}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("at 50, widths = %v, want %v", got, want)
	}
}

func TestSizeTableDropsTrailingColumns(t *testing.T) {
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    --- FAIL: TestSizeTableShrinksTheRoomiestColumnsFirst (0.00s)
```

- [ ] **Step 3: Edit `internal/ui/components.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
}

// col describes a table column. Width 0 means "take the remaining space".
type col struct {
	title string
	width int
}

// tableKeys is the table's built-in Vim-style navigation, minus the default
```

   with:

```go
}

// col describes a table column. Width 0 means "take the remaining space".
// min is how far it gives way when the table is too narrow: a flexible
// column keeps min (minFlex if 0) while fixed columns shrink, and a fixed
// column shrinks no further than min (its title's width if 0).
type col struct {
	title      string
	width, min int
}

// tableKeys is the table's built-in Vim-style navigation, minus the default
```

2. Replace:

```go
// minFlex is the narrowest a flexible column may get.
const minFlex = 8

// sizeTable fits t into a w×h area, giving flexible columns the leftover
// width. When the columns don't fit, fixed columns shrink from the last one
// down to their title width, then trailing columns are hidden. A hidden
// column keeps its slot with width 0, which the table skips: the table
// indexes columns by row cell, so removing columns would make it panic on
// rows that still carry every cell.
func sizeTable(t *table.Model, cols []col, w, h int) {
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = c.width
	}
	shown := len(cols)
	// used is the width of the shown columns with padding, and how many
	// of them are flexible.
	used := func() (fixed, flex int) {
		for i := range shown {
			fixed += widths[i] + 2 // cell padding
			if cols[i].width == 0 {
				flex++
			}
		}
		return fixed, flex
	}
	over := func() int { f, n := used(); return f + n*minFlex - w }

	for i := len(cols) - 1; i >= 0 && over() > 0; i-- {
		if cols[i].width > 0 {
			widths[i] = max(widths[i]-over(), len(cols[i].title))
		}
	}
	for shown > 1 && over() > 0 {
		shown--
	}

	fixed, flex := used()
	tc := make([]table.Column, len(cols))
	for i, c := range cols {
		width := widths[i]
```

   with:

```go
// minFlex is the narrowest a flexible column may get.
const minFlex = 8

// sizeTable fits t into a w×h area. When the columns don't fit, fixed
// columns give way first, a cell at a time from whichever has the most to
// spare (so a long VERSION shrinks before a short TYPE), then flexible
// columns down to minFlex, and then trailing columns are hidden. Flexible
// columns share whatever width is left. A hidden column keeps its slot with
// width 0, which the table skips: the table indexes columns by row cell, so
// removing columns would make it panic on rows that still carry every cell.
func sizeTable(t *table.Model, cols []col, w, h int) {
	widths := make([]int, len(cols))
	floor := make([]int, len(cols))
	for i, c := range cols {
		switch {
		case c.width == 0:
			widths[i], floor[i] = max(c.min, minFlex), minFlex
		case c.min > 0:
			widths[i], floor[i] = c.width, min(c.min, c.width)
		default:
			widths[i], floor[i] = c.width, min(len(c.title), c.width)
		}
	}
	shown := len(cols)
	// over is how much wider than w the shown columns are, with padding.
	over := func() int {
		n := 0
		for i := range shown {
			n += widths[i] + 2 // cell padding
		}
		return n - w
	}
	for _, flexible := range []bool{false, true} {
		for over() > 0 {
			give := -1
			for i := range shown {
				if (cols[i].width == 0) == flexible && widths[i] > floor[i] &&
					(give < 0 || widths[i]-floor[i] > widths[give]-floor[give]) {
					give = i
				}
			}
			if give < 0 {
				break
			}
			widths[give]--
		}
	}
	for shown > 1 && over() > 0 {
		shown--
	}

	flex := 0
	for i := range shown {
		if cols[i].width == 0 {
			flex++
		}
	}
	spare := max(-over(), 0)
	tc := make([]table.Column, len(cols))
	for i, c := range cols {
		width := widths[i]
```

3. Replace:

```go
		case i >= shown:
			width = 0
		case c.width == 0:
			width = max((w-fixed)/max(flex, 1), minFlex)
		}
		tc[i] = table.Column{Title: c.title, Width: width}
	}
```

   with:

```go
		case i >= shown:
			width = 0
		case c.width == 0:
			width += spare / flex
		}
		tc[i] = table.Column{Title: c.title, Width: width}
	}
```

- [ ] **Step 4: Edit `internal/ui/installed.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
// viewBarHeight is the bar of views plus the line about the active one.
const viewBarHeight = 2

var installedCols = []col{{"NAME", 0}, {"VERSION", 16}, {"TYPE", 7}, {"STATUS", 10}}

// installedTab lists installed packages with a details preview.
type installedTab struct {
```

   with:

```go
// viewBarHeight is the bar of views plus the line about the active one.
const viewBarHeight = 2

var installedCols = []col{{"NAME", 0, 16}, {"VERSION", 16, 8}, {"TYPE", 7, 4}, {"STATUS", 10, 6}}

// installedTab lists installed packages with a details preview.
type installedTab struct {
```

- [ ] **Step 5: Edit `internal/ui/jobsview.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	"brewery/internal/jobs"
)

var jobCols = []col{{"STATUS", 9}, {"COMMAND", 0}, {"STARTED", 12}, {"TOOK", 6}}

// jobKey identifies a jobs row by its command and start time.
func jobKey(r table.Row) string { return r[1] + "\x00" + r[2] }
```

   with:

```go
	"brewery/internal/jobs"
)

var jobCols = []col{{"STATUS", 9, 0}, {"COMMAND", 0, 20}, {"STARTED", 12, 5}, {"TOOK", 6, 0}}

// jobKey identifies a jobs row by its command and start time.
func jobKey(r table.Row) string { return r[1] + "\x00" + r[2] }
```

- [ ] **Step 6: Edit `internal/ui/list.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
}

var (
	outdatedCols = []col{{"NAME", 0}, {"INSTALLED", 18}, {"LATEST", 18}, {"TYPE", 7}, {"PINNED", 6}}
	serviceCols  = []col{{"NAME", 0}, {"STATUS", 16}}
	tapCols      = []col{{"NAME", 24}, {"FORMULAE", 8}, {"CASKS", 6}, {"OFFICIAL", 8}, {"REMOTE", 0}}
)
```

   with:

```go
}

var (
	outdatedCols = []col{{"NAME", 0, 16}, {"INSTALLED", 18, 8}, {"LATEST", 18, 8}, {"TYPE", 7, 4}, {"PINNED", 6, 0}}
	serviceCols  = []col{{"NAME", 0, 16}, {"STATUS", 16, 10}}
	tapCols      = []col{{"NAME", 24, 12}, {"FORMULAE", 8, 0}, {"CASKS", 6, 0}, {"OFFICIAL", 8, 0}, {"REMOTE", 0, 0}}
)
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "fix(ui): narrow tables shrink their roomiest columns first, so names stay readable"
```

---

### Task 3: The preview goes under the list below 100 columns, and takes turns below 60

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/installed.go`
- Modify: `internal/ui/installed_test.go`
- Modify: `internal/ui/list.go`
- Create: `internal/ui/split.go`
- Create: `internal/ui/split_test.go`

**Interfaces:**
- **Consumes:** `installedTab.pv` (Plan 5), `listTab.preview` (Plan 4).
- **Produces:**
  - In `split.go`: `sideBySideWidth = 100`, `previewMinWidth = 60`, `minPreviewH = 5`; `split{listW, listH, pvW, pvH int; stacked bool}`; `splitFor(w, h int, showPreview bool) split`; `(split) join(list, preview string) string`.
  - `listTab.layout()` and `listTab.pvOpen`; `tab`/`enter` toggle `pvOpen` on a narrow list with a preview, and `esc` on Services closes it.
  - `installedTab.listWidth()` now reports the split's list width.

One test about short view names now uses a 44-column tab. At 80 columns the list is full width, so its view bar no longer needs the short names there.

- [ ] **Step 1: Edit `internal/ui/installed_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go

func TestInstalledViewBarFitsNarrowPanes(t *testing.T) {
	it := newFixtureInstalled(t)
	it.resize(80, 24) // a 44-column list pane
	out := it.view(nil)
	if h, w := lipgloss.Height(out), lipgloss.Width(out); h != 24 || w != 80 {
		t.Errorf("rendered %dx%d, want 80x24", w, h)
	}
	plain := stripANSI(out)
	for _, want := range []string{"Top 5", "All 7", "Apps 2", "Deps 2", "…  f/F"} {
```

   with:

```go

func TestInstalledViewBarFitsNarrowPanes(t *testing.T) {
	it := newFixtureInstalled(t)
	it.resize(44, 24) // narrow: a 44-column list pane, the preview hidden
	out := it.view(nil)
	if h, w := lipgloss.Height(out), lipgloss.Width(out); h != 24 || w != 44 {
		t.Errorf("rendered %dx%d, want 44x24", w, h)
	}
	plain := stripANSI(out)
	for _, want := range []string{"Top 5", "All 7", "Apps 2", "Deps 2", "…  f/F"} {
```

- [ ] **Step 2: Create `internal/ui/split_test.go`**

```go
package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestSplitFor(t *testing.T) {
	for _, tt := range []struct {
		w, h int
		show bool
		want split
	}{
		{140, 30, false, split{listW: 77, listH: 30, pvW: 63, pvH: 30}},
		{100, 20, false, split{listW: 55, listH: 20, pvW: 45, pvH: 20}},
		{80, 21, false, split{listW: 80, listH: 12, pvW: 80, pvH: 9, stacked: true}},
		{80, 9, false, split{listW: 80, listH: 9}},   // a stacked preview would be too short
		{80, 9, true, split{pvW: 80, pvH: 9}},        // so it takes turns, like a narrow one
		{50, 20, false, split{listW: 50, listH: 20}}, // narrow: the list alone
		{50, 20, true, split{pvW: 50, pvH: 20}},      // or the preview alone
	} {
		if got := splitFor(tt.w, tt.h, tt.show); got != tt.want {
			t.Errorf("splitFor(%d, %d, %v) = %+v, want %+v", tt.w, tt.h, tt.show, got, tt.want)
		}
	}
}

func TestInstalledStacksThenHidesThePreview(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	keys(t, a, "2")
	out := stripANSI(a.render())
	lines := strings.Split(out, "\n")
	list, preview := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "Top-level 5") && list < 0 {
			list = i
		}
		if strings.Contains(l, "App to build and share") {
			preview = i
		}
	}
	if list < 0 || preview < list || lipgloss.Height(out) != 24 {
		t.Errorf("at 80 columns the preview goes under the list:\n%s", out)
	}

	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	if out := stripANSI(a.render()); strings.Contains(out, "App to build and share") || !strings.Contains(out, "docker-desktop") {
		t.Errorf("at 50 columns only the list shows:\n%s", out)
	}
	keys(t, a, "tab")
	if out := stripANSI(a.render()); !strings.Contains(out, "App to build and share") || strings.Contains(out, "Top 5") {
		t.Errorf("tab shows the preview instead of the list:\n%s", out)
	}
	keys(t, a, "esc")
	if out := stripANSI(a.render()); !strings.Contains(out, "Top 5") {
		t.Errorf("esc goes back to the list:\n%s", out)
	}
}

func TestServicesPreviewTakesTurnsWhenNarrow(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	keys(t, a, "4")
	if out := stripANSI(a.render()); strings.Contains(out, "End of the log") {
		t.Errorf("at 50 columns only the list shows:\n%s", out)
	}
	keys(t, a, "enter")
	if out := stripANSI(a.render()); !strings.Contains(out, "End of the log") || strings.Contains(out, "STATUS") {
		t.Errorf("enter shows the preview instead of the list:\n%s", out)
	}
	keys(t, a, "esc")
	if out := stripANSI(a.render()); !strings.Contains(out, "STATUS") {
		t.Errorf("esc goes back to the list:\n%s", out)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/split_test.go:15:8: undefined: split
    internal/ui/split_test.go:17:20: undefined: split
    internal/ui/split_test.go:18:20: undefined: split
```

- [ ] **Step 4: Edit `internal/ui/app.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		switch {
		case a.tab == tabInstalled && a.installed.pv.focus:
			a.installed.pv.focus = false // back to the list first
		case a.marks() > 0 && !a.drawerShown():
			a.clearMarks()
		default:
```

   with:

```go
		switch {
		case a.tab == tabInstalled && a.installed.pv.focus:
			a.installed.pv.focus = false // back to the list first
		case a.tab == tabServices && a.services.pvOpen:
			a.services.pvOpen = false
		case a.marks() > 0 && !a.drawerShown():
			a.clearMarks()
		default:
```

- [ ] **Step 5: Edit `internal/ui/installed.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go
		t.pv.focus = !t.pv.focus
		return nil
	case t.pv.focus && k != "f" && k != "F":
		_, ih := paneInner(t.w-t.listWidth(), t.h)
		t.pv.scroll(k, ih/2)
		return nil
	}
```

   with:

```go
		t.pv.focus = !t.pv.focus
		return nil
	case t.pv.focus && k != "f" && k != "F":
		sp := splitFor(t.w, t.h, true)
		_, ih := paneInner(sp.pvW, sp.pvH)
		t.pv.scroll(k, ih/2)
		return nil
	}
```

2. Replace:

```go
// until it has been sized.
func (t *installedTab) resize(w, h int) {
	t.w, t.h = w, h
	iw, ih := paneInner(t.listWidth(), h)
	sizeTable(&t.table, installedCols, iw, max(ih-viewBarHeight, 0))
}

func (t *installedTab) listWidth() int { return t.w * 55 / 100 }

// view sizes the table each time because a failed refresh shares the list
// pane with a banner above it.
func (t *installedTab) view(err error) string {
	w, h, listW := t.w, t.h, t.listWidth()
	if err != nil && len(t.all) == 0 {
		return pane(t.st, "Installed", errorBody(t.st, err), w, h, true)
	}
	iw, ih := paneInner(listW, h)
	body := t.viewBar(iw) + "\n"
	ih -= viewBarHeight
	if err != nil {
```

   with:

```go
// until it has been sized.
func (t *installedTab) resize(w, h int) {
	t.w, t.h = w, h
	sp := splitFor(w, h, false)
	iw, ih := paneInner(sp.listW, sp.listH)
	sizeTable(&t.table, installedCols, iw, max(ih-viewBarHeight, 0))
}

// listWidth is the list pane's width with the preview closed.
func (t *installedTab) listWidth() int { return splitFor(t.w, t.h, false).listW }

// view sizes the table each time because a failed refresh shares the list
// pane with a banner above it.
func (t *installedTab) view(err error) string {
	w, h := t.w, t.h
	if err != nil && len(t.all) == 0 {
		return pane(t.st, "Installed", errorBody(t.st, err), w, h, true)
	}
	sp := splitFor(w, h, t.pv.focus)
	iw, ih := paneInner(sp.listW, sp.listH)
	body := t.viewBar(iw) + "\n"
	ih -= viewBarHeight
	if err != nil {
```

3. Replace:

```go
	if t.match != "" {
		title += " · /" + t.match
	}
	list := pane(t.st, title, body+t.table.View(), listW, h, !t.pv.focus)

	p, ok := t.selected()
	previewTitle, body := "Preview", t.st.Muted.Render("nothing in this view")
```

   with:

```go
	if t.match != "" {
		title += " · /" + t.match
	}
	list := pane(t.st, title, body+t.table.View(), sp.listW, sp.listH, !t.pv.focus)

	p, ok := t.selected()
	previewTitle, body := "Preview", t.st.Muted.Render("nothing in this view")
```

4. Replace:

```go
	}
	switch {
	case ok:
		_, ph := paneInner(w-listW, h)
		previewTitle, body = p.Name, t.pv.window(t.preview(p), ph)
	case !t.loaded && err == nil:
		body = t.st.Muted.Render("loading…")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, list, pane(t.st, previewTitle, body, w-listW, h, t.pv.focus))
}

// viewBar lists every view with its count, the active one highlighted,
```

   with:

```go
	}
	switch {
	case ok:
		_, ph := paneInner(sp.pvW, sp.pvH)
		previewTitle, body = p.Name, t.pv.window(t.preview(p), ph)
	case !t.loaded && err == nil:
		body = t.st.Muted.Render("loading…")
	}
	return sp.join(list, pane(t.st, previewTitle, body, sp.pvW, sp.pvH, t.pv.focus))
}

// viewBar lists every view with its count, the active one highlighted,
```

- [ ] **Step 6: Edit `internal/ui/list.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
	"brewery/internal/theme"
```

   with:

```go

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/theme"
```

2. Replace:

```go
	// preview, if set, fills a pane beside the table with what it returns
	// for a w×h pane.
	preview func(w, h int) (title, body string)
}

func newListTab(st theme.Styles, name string, cols []col, key func(table.Row) string, empty string) listTab {
```

   with:

```go
	// preview, if set, fills a pane beside the table with what it returns
	// for a w×h pane.
	preview func(w, h int) (title, body string)
	pvOpen  bool // below previewMinWidth, the preview shows instead of the list
}

func newListTab(st theme.Styles, name string, cols []col, key func(table.Row) string, empty string) listTab {
```

3. Replace:

```go
func outdatedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[3] }

func (t *listTab) update(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
```

   with:

```go
func outdatedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[3] }

func (t *listTab) update(msg tea.KeyPressMsg) tea.Cmd {
	if k := msg.String(); t.preview != nil && t.w < previewMinWidth && (k == "tab" || k == "enter") {
		t.pvOpen = !t.pvOpen // narrow: the preview takes turns with the list
		return nil
	}
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
```

4. Replace:

```go
	return s
}

// listWidth is the table pane's width: all of it, or 55% beside a preview.
func (t *listTab) listWidth() int {
	if t.preview == nil {
		return t.w
	}
	return t.w * 55 / 100
}

func (t *listTab) resize(w, h int) {
	t.w, t.h = w, h
	iw, ih := paneInner(t.listWidth(), h)
	sizeTable(&t.table, t.cols, iw, ih)
}

// view sizes the table each time because a failed refresh shares the pane
// with a banner above it.
func (t *listTab) view(err error) string {
	w, h, listW := t.w, t.h, t.listWidth()
	iw, ih := paneInner(listW, h)
	switch {
	case err != nil && t.n == 0:
		return pane(t.st, t.name, errorBody(t.st, err), w, h, true)
```

   with:

```go
	return s
}

// layout is how the table and any preview share the tab.
func (t *listTab) layout() split {
	if t.preview == nil {
		return split{listW: t.w, listH: t.h}
	}
	return splitFor(t.w, t.h, t.pvOpen)
}

func (t *listTab) resize(w, h int) {
	t.w, t.h = w, h
	sp := t.layout()
	if sp.listW == 0 {
		sp = splitFor(w, h, false) // the table keeps its size while the preview shows
	}
	iw, ih := paneInner(sp.listW, sp.listH)
	sizeTable(&t.table, t.cols, iw, ih)
}

// view sizes the table each time because a failed refresh shares the pane
// with a banner above it.
func (t *listTab) view(err error) string {
	w, h, sp := t.w, t.h, t.layout()
	iw, ih := paneInner(sp.listW, sp.listH)
	switch {
	case err != nil && t.n == 0:
		return pane(t.st, t.name, errorBody(t.st, err), w, h, true)
```

5. Replace:

```go
		ih -= bannerHeight
	}
	t.table.SetHeight(ih)
	list := pane(t.st, t.title(), body+t.table.View(), listW, h, true)
	if t.preview == nil {
		return list
	}
	title, preview := t.preview(w-listW, h)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, pane(t.st, title, preview, w-listW, h, false))
}

func outdatedRows(items []brew.Outdated) []table.Row {
```

   with:

```go
		ih -= bannerHeight
	}
	t.table.SetHeight(ih)
	list := pane(t.st, t.title(), body+t.table.View(), sp.listW, sp.listH, true)
	if sp.pvW == 0 {
		return list
	}
	title, preview := t.preview(sp.pvW, sp.pvH)
	return sp.join(list, pane(t.st, title, preview, sp.pvW, sp.pvH, sp.listW == 0))
}

func outdatedRows(items []brew.Outdated) []table.Row {
```

- [ ] **Step 7: Create `internal/ui/split.go`**

```go
package ui

import "charm.land/lipgloss/v2"

// Widths at which a list and its preview change layout.
const (
	sideBySideWidth = 100 // and wider: the preview sits beside the list
	previewMinWidth = 60  // narrower: one at a time, the preview only when asked
	minPreviewH     = 5   // a stacked preview shorter than this isn't shown
)

// split is how a list and its preview share a w×h area. A zero width
// means that pane isn't shown.
type split struct {
	listW, listH int
	pvW, pvH     int
	stacked      bool // the preview is under the list
}

// splitFor lays out a list and its preview in w×h: side by side from 100
// columns, the preview under the list from 60, and below that (or when a
// stacked preview would be too short) the list alone, or the preview alone
// when showPreview is set.
func splitFor(w, h int, showPreview bool) split {
	if w >= sideBySideWidth {
		lw := w * 55 / 100
		return split{listW: lw, listH: h, pvW: w - lw, pvH: h}
	}
	if lh := h - h*45/100; w >= previewMinWidth && h-lh >= minPreviewH {
		return split{listW: w, listH: lh, pvW: w, pvH: h - lh, stacked: true}
	}
	if showPreview {
		return split{pvW: w, pvH: h}
	}
	return split{listW: w, listH: h}
}

// join puts the list and preview panes together as s lays them out.
func (s split) join(list, preview string) string {
	switch {
	case s.listW == 0:
		return preview
	case s.pvW == 0:
		return list
	case s.stacked:
		return lipgloss.JoinVertical(lipgloss.Left, list, preview)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, list, preview)
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(ui): the preview goes under the list below 100 columns, and takes turns below 60"
```

---

### Task 4: Placeholder rows while a list waits for its first results

**Files:**
- Modify: `internal/ui/components.go`
- Modify: `internal/ui/installed.go`
- Modify: `internal/ui/list.go`
- Modify: `internal/ui/list_test.go`

**Interfaces:**
- **Consumes:** `listTab.view`, `installedTab.view`.
- **Produces:** `skeletonRows = 8`, `skeleton(st theme.Styles, w, n int) string`.

- [ ] **Step 1: Edit `internal/ui/list_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

func TestListTabRendersRows(t *testing.T) {
```

   with:

```go

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"brewery/internal/state"
)

func TestListTabRendersRows(t *testing.T) {
```

2. Replace:

```go
		t.Errorf("cursor() on an empty table = %d, want -1", lt.cursor())
	}
}
```

   with:

```go
		t.Errorf("cursor() on an empty table = %d, want -1", lt.cursor())
	}
}

func TestSkeletonRowsBeforeTheFirstResult(t *testing.T) {
	lt := newListTab(testStyles, "Taps", tapCols, firstCell, "none")
	lt.resize(60, 20)
	out := stripANSI(lt.view(nil))
	n := 0
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "░") {
			n++
		}
	}
	if n != skeletonRows {
		t.Errorf("want %d placeholder rows, got %d:\n%s", skeletonRows, n, out)
	}
	it := newInstalledTab(testStyles)
	it.resize(140, 6)
	it.setData(nil, state.NewIndex(nil))
	out = stripANSI(it.view(nil))
	if !strings.Contains(out, "░░░░") || lipgloss.Height(it.view(nil)) != 6 {
		t.Errorf("Installed shows placeholders too, within its height:\n%s", out)
	}
	if got := skeleton(testStyles, 10, 3); lipgloss.Width(got) > 10 || lipgloss.Height(got) != 3 {
		t.Errorf("skeleton is %dx%d, want at most 10 wide and 3 tall", lipgloss.Width(got), lipgloss.Height(got))
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/list_test.go:216:10: undefined: skeletonRows
    internal/ui/list_test.go:217:53: undefined: skeletonRows
    internal/ui/list_test.go:226:12: undefined: skeleton
```

- [ ] **Step 3: Edit `internal/ui/components.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	return b.String()
}

// errorBody explains a failed query inside the pane that needed its data.
func errorBody(st theme.Styles, err error) string {
	return st.Err.Render("✗ couldn't load this view") + "\n\n" +
```

   with:

```go
	return b.String()
}

// skeletonRows is how many placeholder rows a list shows at most.
const skeletonRows = 8

// skeleton is n placeholder rows, at most w wide, for a list whose first
// results haven't come yet: the shape of what's coming, rather than a blank.
func skeleton(st theme.Styles, w, n int) string {
	lines := make([]string, min(n, skeletonRows))
	for i := range lines {
		row := strings.Repeat("░", 8+i*5%7) + "   " + strings.Repeat("░", 6+i*3%4) + "   " + strings.Repeat("░", 4)
		lines[i] = st.Muted.Render(ansi.Truncate(" "+row, w, ""))
	}
	return strings.Join(lines, "\n")
}

// errorBody explains a failed query inside the pane that needed its data.
func errorBody(st theme.Styles, err error) string {
	return st.Err.Render("✗ couldn't load this view") + "\n\n" +
```

- [ ] **Step 4: Edit `internal/ui/installed.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	if t.match != "" {
		title += " · /" + t.match
	}
	list := pane(t.st, title, body+t.table.View(), sp.listW, sp.listH, !t.pv.focus)

	p, ok := t.selected()
	previewTitle, body := "Preview", t.st.Muted.Render("nothing in this view")
```

   with:

```go
	if t.match != "" {
		title += " · /" + t.match
	}
	rows := t.table.View()
	if !t.loaded && len(t.all) == 0 {
		rows = skeleton(t.st, iw, ih) // nothing yet: show the shape of what's coming
	}
	list := pane(t.st, title, body+rows, sp.listW, sp.listH, !t.pv.focus)

	p, ok := t.selected()
	previewTitle, body := "Preview", t.st.Muted.Render("nothing in this view")
```

- [ ] **Step 5: Edit `internal/ui/list.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	case err != nil && t.n == 0:
		return pane(t.st, t.name, errorBody(t.st, err), w, h, true)
	case t.n == 0 && !t.loaded:
		return pane(t.st, t.name, t.st.Muted.Render("loading…"), w, h, true)
	case t.n == 0 && len(t.all) > 0:
		return pane(t.st, t.title(), t.st.Muted.Render("nothing matches /"+t.match+" · / then esc clears it"), w, h, true)
	case t.n == 0:
```

   with:

```go
	case err != nil && t.n == 0:
		return pane(t.st, t.name, errorBody(t.st, err), w, h, true)
	case t.n == 0 && !t.loaded:
		return pane(t.st, t.name, t.st.Muted.Render("loading…")+"\n"+skeleton(t.st, iw, ih-1), w, h, true)
	case t.n == 0 && len(t.all) > 0:
		return pane(t.st, t.title(), t.st.Muted.Render("nothing matches /"+t.match+" · / then esc clears it"), w, h, true)
	case t.n == 0:
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(ui): placeholder rows while a list waits for its first results"
```

---

### Task 5: Nerd Font icons, and --no-icons for plain text

**Files:**
- Modify: `cmd/brewery/main.go`
- Modify: `cmd/brewery/main_test.go`
- Create: `internal/theme/icons.go`
- Create: `internal/theme/icons_test.go`
- Modify: `internal/theme/theme.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/installed.go`
- Modify: `internal/ui/list.go`
- Modify: `internal/ui/list_test.go`
- Modify: `internal/ui/services.go`

**Interfaces:**
- **Consumes:** `theme.Styles`, the row builders.
- **Produces:**
  - In `theme`: `Icons{Formula, Cask, Pinned, Started, Stopped, Failed string}`, `Nerd`, `With(icon, word string) string`, `Styles.Icons`.
  - In `ui`: `(*App) UseIcons(on bool)` (sets every part's copy of the styles, then re-syncs), `kindCell(ic, kind)`, `serviceIcon(ic, status)`, `outdatedRows(ic, items)`, `serviceRows(ic, items)`, `status(ic, p)`.
  - In `main`: `--no-icons`, `wantIcons(noIcons bool, env string) bool`.

Icons are off in `New`, so tests and other callers get plain words; only `main` turns them on.

- [ ] **Step 1: Edit `cmd/brewery/main_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Errorf("exit %d, want 2", code)
	}
}
```

   with:

```go
		t.Errorf("exit %d, want 2", code)
	}
}

func TestWantIcons(t *testing.T) {
	for _, tt := range []struct {
		flag bool
		env  string
		want bool
	}{
		{false, "", true},
		{true, "", false},
		{false, "1", false},
	} {
		if got := wantIcons(tt.flag, tt.env); got != tt.want {
			t.Errorf("wantIcons(%v, %q) = %v", tt.flag, tt.env, got)
		}
	}
}

func TestNoIconsFlagParses(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--no-icons", "--version"}, &out, &errb); code != 0 {
		t.Errorf("exit %d, stderr %q", code, errb.String())
	}
}
```

- [ ] **Step 2: Create `internal/theme/icons_test.go`**

```go
package theme

import "testing"

func TestWith(t *testing.T) {
	if got := With("", "cask"); got != "cask" {
		t.Errorf("no icon: %q", got)
	}
	if got := With(Nerd.Cask, "cask"); got != "\U000f003b cask" {
		t.Errorf("with an icon: %q", got)
	}
}
```

- [ ] **Step 3: Edit `internal/ui/list_test.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
	"charm.land/lipgloss/v2"

	"brewery/internal/state"
)

func TestListTabRendersRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	lt := newListTab(testStyles, "Outdated", outdatedCols, outdatedKey, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(snap.Outdated))
	out := stripANSI(lt.view(nil))
	for _, want := range []string{"Outdated · 2", "NAME", "lua-language-server", "3.19.1", "3.19.2", "4.95.0,242831", "cask"} {
		if !strings.Contains(out, want) {
```

   with:

```go
	"charm.land/lipgloss/v2"

	"brewery/internal/state"
	"brewery/internal/theme"
)

func TestListTabRendersRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	lt := newListTab(testStyles, "Outdated", outdatedCols, outdatedKey, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(theme.Icons{}, snap.Outdated))
	out := stripANSI(lt.view(nil))
	for _, want := range []string{"Outdated · 2", "NAME", "lua-language-server", "3.19.1", "3.19.2", "4.95.0,242831", "cask"} {
		if !strings.Contains(out, want) {
```

2. Replace:

```go

func TestServiceAndTapRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	if got := serviceRows(snap.Services)[0]; got[0] != "postgresql@16" || got[1] != "error (exit 1)" {
		t.Errorf("service row = %v", got)
	}
	if got := serviceRows(snap.Services)[1][1]; got != "started" {
		t.Errorf("status without exit code = %q", got)
	}
	if got := tapRows(snap.Taps)[0]; got[1] != "2" || got[2] != "1" || got[3] != "" {
```

   with:

```go

func TestServiceAndTapRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	if got := serviceRows(theme.Icons{}, snap.Services)[0]; got[0] != "postgresql@16" || got[1] != "error (exit 1)" {
		t.Errorf("service row = %v", got)
	}
	if got := serviceRows(theme.Icons{}, snap.Services)[1][1]; got != "started" {
		t.Errorf("status without exit code = %q", got)
	}
	if got := tapRows(snap.Taps)[0]; got[1] != "2" || got[2] != "1" || got[3] != "" {
```

3. Replace:

```go
	if tbl.Columns()[0].Width < 8 {
		t.Errorf("NAME width = %d, want >= 8", tbl.Columns()[0].Width)
	}
	setRows(&tbl, outdatedRows(fixtureSnapshot(t).Outdated)) // rows still carry every cell
	out := stripANSI(tbl.View())
	if !strings.Contains(out, "lua-lan") || strings.Contains(out, "PINNED") || strings.Contains(out, "TYPE") {
		t.Errorf("dropped columns should not render, rows should:\n%s", out)
```

   with:

```go
	if tbl.Columns()[0].Width < 8 {
		t.Errorf("NAME width = %d, want >= 8", tbl.Columns()[0].Width)
	}
	setRows(&tbl, outdatedRows(theme.Icons{}, fixtureSnapshot(t).Outdated)) // rows still carry every cell
	out := stripANSI(tbl.View())
	if !strings.Contains(out, "lua-lan") || strings.Contains(out, "PINNED") || strings.Contains(out, "TYPE") {
		t.Errorf("dropped columns should not render, rows should:\n%s", out)
```

4. Replace:

```go
func TestListTabErrorWithRowsShowsBanner(t *testing.T) {
	lt := newListTab(testStyles, "Outdated", outdatedCols, outdatedKey, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(fixtureSnapshot(t).Outdated))
	view := lt.view(errors.New("brew outdated --json=v2: exit status 1\nError: second line"))
	out := stripANSI(view)
	for _, want := range []string{"refresh failed · r to retry", "exit status 1", "lua-language-server", "docker-desktop"} {
```

   with:

```go
func TestListTabErrorWithRowsShowsBanner(t *testing.T) {
	lt := newListTab(testStyles, "Outdated", outdatedCols, outdatedKey, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(theme.Icons{}, fixtureSnapshot(t).Outdated))
	view := lt.view(errors.New("brew outdated --json=v2: exit status 1\nError: second line"))
	out := stripANSI(view)
	for _, want := range []string{"refresh failed · r to retry", "exit status 1", "lua-language-server", "docker-desktop"} {
```

5. Replace:

```go
		t.Errorf("skeleton is %dx%d, want at most 10 wide and 3 tall", lipgloss.Width(got), lipgloss.Height(got))
	}
}
```

   with:

```go
		t.Errorf("skeleton is %dx%d, want at most 10 wide and 3 tall", lipgloss.Width(got), lipgloss.Height(got))
	}
}

func TestIconsInRows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseIcons(true)
	if got := a.outdated.all[1][3]; got != theme.Nerd.Cask+" cask" {
		t.Errorf("outdated TYPE with icons = %q", got)
	}
	if got := a.services.all[0][1]; got != theme.Nerd.Failed+" error (exit 1)" {
		t.Errorf("service status with icons = %q", got)
	}
	keys(t, a, "2", "j", "j", "j") // git, pinned
	if p, _ := a.installed.selected(); p.Name != "git" {
		t.Fatalf("selected %s", p.Name)
	}
	row := a.installed.table.SelectedRow()
	if row[2] != theme.Nerd.Formula+" formula" || row[3] != theme.Nerd.Pinned+" pinned" {
		t.Errorf("installed row with icons = %q", row)
	}
	a.UseIcons(false)
	if got := a.outdated.all[1][3]; got != "cask" {
		t.Errorf("icons off: %q", got)
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/theme/icons_test.go:6:12: undefined: With
    internal/theme/icons_test.go:9:12: undefined: With
    internal/theme/icons_test.go:9:17: undefined: Nerd
```

- [ ] **Step 5: Edit `cmd/brewery/main.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// locateBrew is swapped out in tests.
var locateBrew = brew.Locate
```

   with:

```go
// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// wantIcons reports whether to draw Nerd Font icons: yes, unless
// --no-icons was given or BREWERY_NO_ICONS is set to anything.
func wantIcons(noIcons bool, env string) bool { return !noIcons && env == "" }

// locateBrew is swapped out in tests.
var locateBrew = brew.Locate
```

2. Replace:

```go
	fs := flag.NewFlagSet("brewery", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
```

   with:

```go
	fs := flag.NewFlagSet("brewery", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	noIcons := fs.Bool("no-icons", false, "draw plain text instead of Nerd Font icons (or set BREWERY_NO_ICONS=1)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
```

3. Replace:

```go
	hist := jobs.History{Dir: jobs.DefaultHistoryDir()}
	recs, _ := hist.Load() // no history yet is fine
	app.UseHistory(hist, recs)
	app.UseCompletions(client.Completions)
	// Full-screen commands get the real stderr: while the TUI runs,
	// os.Stderr is the crash log.
```

   with:

```go
	hist := jobs.History{Dir: jobs.DefaultHistoryDir()}
	recs, _ := hist.Load() // no history yet is fine
	app.UseHistory(hist, recs)
	app.UseIcons(wantIcons(*noIcons, os.Getenv("BREWERY_NO_ICONS")))
	app.UseCompletions(client.Completions)
	// Full-screen commands get the real stderr: while the TUI runs,
	// os.Stderr is the crash log.
```

- [ ] **Step 6: Create `internal/theme/icons.go`**

```go
package theme

// Icons are glyphs brewery draws before some words. The Nerd Font ones
// need a Nerd Font in the terminal; with the zero Icons, brewery draws the
// words alone.
type Icons struct {
	Formula, Cask, Pinned    string
	Started, Stopped, Failed string // a service's state
}

// Nerd are Nerd Font (v3) glyphs.
var Nerd = Icons{
	Formula: "\uf0fc",     // nf-fa-beer
	Cask:    "\U000f003b", // nf-md-apps
	Pinned:  "\U000f0403", // nf-md-pin
	Started: "\U000f040a", // nf-md-play
	Stopped: "\U000f04db", // nf-md-stop
	Failed:  "\U000f0028", // nf-md-alert_circle
}

// With puts icon before word, or gives word alone when icon is empty.
func With(icon, word string) string {
	if icon == "" {
		return word
	}
	return icon + " " + word
}
```

- [ ] **Step 7: Edit `internal/theme/theme.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	StatusBar                    lipgloss.Style

	TableHeader, TableCell, TableSelected lipgloss.Style
}

func New(p Palette) Styles {
```

   with:

```go
	StatusBar                    lipgloss.Style

	TableHeader, TableCell, TableSelected lipgloss.Style

	Icons Icons // none until the app turns them on
}

func New(p Palette) Styles {
```

- [ ] **Step 8: Edit `internal/ui/app.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go

func (a *App) Init() tea.Cmd { return tea.Batch(a.refresh(), a.loadCompletions()) }

// UseHistory saves finished jobs to h, starting from the records already
// there (newest first).
func (a *App) UseHistory(h jobs.History, recs []jobs.Record) {
```

   with:

```go

func (a *App) Init() tea.Cmd { return tea.Batch(a.refresh(), a.loadCompletions()) }

// UseIcons turns Nerd Font icons on or off. They're off until this is
// called, so tests and other callers get plain words.
func (a *App) UseIcons(on bool) {
	ic := theme.Icons{}
	if on {
		ic = theme.Nerd
	}
	// Every part keeps its own copy of the styles.
	for _, st := range []*theme.Styles{&a.st, &a.installed.st, &a.outdated.st, &a.services.st, &a.taps.st, &a.jobs.st, &a.home.st, &a.drawer.st} {
		st.Icons = ic
	}
	a.sync()
}

// UseHistory saves finished jobs to h, starting from the records already
// there (newest first).
func (a *App) UseHistory(h jobs.History, recs []jobs.Record) {
```

2. Replace:

```go
	a.taps.loaded = a.loaded[qTaps]
	a.ix = state.NewIndex(a.snap.Packages)
	a.installed.setData(a.snap.Packages, a.ix)
	a.outdated.setRows(outdatedRows(a.snap.Outdated))
	a.services.setRows(serviceRows(a.snap.Services))
	a.taps.setRows(tapRows(a.snap.Taps))
}
```

   with:

```go
	a.taps.loaded = a.loaded[qTaps]
	a.ix = state.NewIndex(a.snap.Packages)
	a.installed.setData(a.snap.Packages, a.ix)
	a.outdated.setRows(outdatedRows(a.st.Icons, a.snap.Outdated))
	a.services.setRows(serviceRows(a.st.Icons, a.snap.Services))
	a.taps.setRows(tapRows(a.snap.Taps))
}
```

- [ ] **Step 9: Edit `internal/ui/installed.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	})
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{marked(p.Name, t.marked[pkgKey(p)]), p.Version, p.Kind.String(), status(p)}
	}
	before := t.selectedKey()
	setRowsKeeping(&t.table, rows, installedKey)
```

   with:

```go
	})
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{marked(p.Name, t.marked[pkgKey(p)]), p.Version, kindCell(t.st.Icons, p.Kind), status(t.st.Icons, p)}
	}
	before := t.selectedKey()
	setRowsKeeping(&t.table, rows, installedKey)
```

2. Replace:

```go
// and a cask can share a name.
func installedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[2] }

func status(p brew.Package) string {
	var s []string
	if p.Pinned {
		s = append(s, "pinned")
	}
	if p.Outdated {
		s = append(s, "↑")
```

   with:

```go
// and a cask can share a name.
func installedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[2] }

func status(ic theme.Icons, p brew.Package) string {
	var s []string
	if p.Pinned {
		s = append(s, theme.With(ic.Pinned, "pinned"))
	}
	if p.Outdated {
		s = append(s, "↑")
```

- [ ] **Step 10: Edit `internal/ui/list.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	return sp.join(list, pane(t.st, title, preview, sp.pvW, sp.pvH, sp.listW == 0))
}

func outdatedRows(items []brew.Outdated) []table.Row {
	rows := make([]table.Row, len(items))
	for i, o := range items {
		pinned := ""
		if o.Pinned {
			pinned = "pinned"
		}
		rows[i] = table.Row{o.Name, o.Installed, o.Current, o.Kind.String(), pinned}
	}
	return rows
}

func serviceRows(items []brew.Service) []table.Row {
	rows := make([]table.Row, len(items))
	for i, s := range items {
		rows[i] = table.Row{s.Name, serviceStatus(s)}
	}
	return rows
}
```

   with:

```go
	return sp.join(list, pane(t.st, title, preview, sp.pvW, sp.pvH, sp.listW == 0))
}

// kindCell is a package's TYPE cell: "formula" or "cask", after its icon.
func kindCell(ic theme.Icons, k brew.Kind) string {
	if k == brew.KindCask {
		return theme.With(ic.Cask, k.String())
	}
	return theme.With(ic.Formula, k.String())
}

func outdatedRows(ic theme.Icons, items []brew.Outdated) []table.Row {
	rows := make([]table.Row, len(items))
	for i, o := range items {
		pinned := ""
		if o.Pinned {
			pinned = theme.With(ic.Pinned, "pinned")
		}
		rows[i] = table.Row{o.Name, o.Installed, o.Current, kindCell(ic, o.Kind), pinned}
	}
	return rows
}

func serviceRows(ic theme.Icons, items []brew.Service) []table.Row {
	rows := make([]table.Row, len(items))
	for i, s := range items {
		rows[i] = table.Row{s.Name, theme.With(serviceIcon(ic, s.Status), serviceStatus(s))}
	}
	return rows
}
```

- [ ] **Step 11: Edit `internal/ui/services.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
)

// logTailBytes is how much of the end of a log the services preview reads.
```

   with:

```go
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/theme"
)

// logTailBytes is how much of the end of a log the services preview reads.
```

2. Replace:

```go
	}
}

// serviceStatus is a service's status, with its exit code if it failed.
func serviceStatus(s brew.Service) string {
	if s.ExitCode != nil && *s.ExitCode != 0 {
```

   with:

```go
	}
}

// serviceIcon is the icon for a service in this state.
func serviceIcon(ic theme.Icons, status string) string {
	switch status {
	case "started", "scheduled":
		return ic.Started
	case "error":
		return ic.Failed
	}
	return ic.Stopped
}

// serviceStatus is a service's status, with its exit code if it failed.
func serviceStatus(s brew.Service) string {
	if s.ExitCode != nil && *s.ExitCode != 0 {
```

3. Replace:

```go
	case "none", "stopped":
		style = st.Muted
	}
	head := style.Render(serviceStatus(s))
	if s.PID > 0 {
		head += st.Muted.Render(fmt.Sprintf(" · pid %d", s.PID))
	}
```

   with:

```go
	case "none", "stopped":
		style = st.Muted
	}
	head := style.Render(theme.With(serviceIcon(st.Icons, s.Status), serviceStatus(s)))
	if s.PID > 0 {
		head += st.Muted.Render(fmt.Sprintf(" · pid %d", s.PID))
	}
```

- [ ] **Step 12: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 13: Commit**

```bash
git add -A
git commit -m "feat: Nerd Font icons for kinds, pins and services; --no-icons for plain text"
```

---

### Task 6: Color service and job states, and keep the selected row highlighted

**Files:**
- Modify: `internal/theme/theme.go`
- Modify: `internal/theme/theme_test.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/installed.go`
- Modify: `internal/ui/jobsview.go`
- Modify: `internal/ui/jobsview_test.go`
- Modify: `internal/ui/list.go`
- Modify: `internal/ui/list_test.go`
- Modify: `internal/ui/run.go`
- Modify: `internal/ui/services.go`

**Interfaces:**
- **Consumes:** Task 5's row builders and `serviceIcon`.
- **Produces:**
  - `theme.Tint(c color.Color, s string) string`.
  - `Styles.TableCell` has no color.
  - `serviceColor(st, status) color.Color`; `serviceRows(st theme.Styles, items)`, `status(st theme.Styles, p)`, `jobRows(st theme.Styles, recs, now)`.

This is where the highlight bug is fixed (see "Found while building this"). The new test fails on the old cell style.

- [ ] **Step 1: Edit `internal/theme/theme_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		seen[key] = name
	}
}
```

   with:

```go
		seen[key] = name
	}
}

func TestTintResetsOnlyTheForeground(t *testing.T) {
	got := Tint(Macchiato.Green, "started")
	if got != "\x1b[38;2;166;218;149mstarted\x1b[39m" {
		t.Errorf("Tint = %q", got)
	}
}
```

- [ ] **Step 2: Edit `internal/ui/jobsview_test.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
	"time"

	"brewery/internal/jobs"
)

// pastJobs are two finished jobs: a failed upgrade an hour ago and a pin
```

   with:

```go
	"time"

	"brewery/internal/jobs"
	"brewery/internal/theme"
)

// pastJobs are two finished jobs: a failed upgrade an hour ago and a pin
```

2. Replace:

```go
}

func TestJobRows(t *testing.T) {
	rows := jobRows(append(pastJobs(), jobs.Record{Title: "upgrade", Args: []string{"upgrade"}, State: jobs.Canceled,
		Started: testNow.Add(-2 * time.Minute), Ended: testNow.Add(-time.Minute)}), testNow)
	want := [][]string{
		{"✗ exit 1", "brew upgrade fd", "08:30", "1:15"},
```

   with:

```go
}

func TestJobRows(t *testing.T) {
	rows := jobRows(testStyles, append(pastJobs(), jobs.Record{Title: "upgrade", Args: []string{"upgrade"}, State: jobs.Canceled,
		Started: testNow.Add(-2 * time.Minute), Ended: testNow.Add(-time.Minute)}), testNow)
	want := [][]string{
		{"✗ exit 1", "brew upgrade fd", "08:30", "1:15"},
```

3. Replace:

```go
		{"stopped", "brew upgrade", "09:28", "1:00"},
	}
	for i, w := range want {
		if strings.Join(rows[i], "|") != strings.Join(w, "|") {
			t.Errorf("row %d = %q, want %q", i, rows[i], w)
		}
	}
}
```

   with:

```go
		{"stopped", "brew upgrade", "09:28", "1:00"},
	}
	for i, w := range want {
		if stripANSI(strings.Join(rows[i], "|")) != strings.Join(w, "|") {
			t.Errorf("row %d = %q, want %q", i, rows[i], w)
		}
	}
	if rows[0][0] != theme.Tint(testStyles.P.Red, "✗ exit 1") || rows[1][0] != theme.Tint(testStyles.P.Green, "✓ done") {
		t.Errorf("statuses should be colored: %q, %q", rows[0][0], rows[1][0])
	}
}
```

- [ ] **Step 3: Edit `internal/ui/list_test.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go

func TestServiceAndTapRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	if got := serviceRows(theme.Icons{}, snap.Services)[0]; got[0] != "postgresql@16" || got[1] != "error (exit 1)" {
		t.Errorf("service row = %v", got)
	}
	if got := serviceRows(theme.Icons{}, snap.Services)[1][1]; got != "started" {
		t.Errorf("status without exit code = %q", got)
	}
	if got := tapRows(snap.Taps)[0]; got[1] != "2" || got[2] != "1" || got[3] != "" {
		t.Errorf("tap row = %v", got)
```

   with:

```go

func TestServiceAndTapRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	rows := serviceRows(testStyles, snap.Services)
	if got := rows[0]; got[0] != "postgresql@16" || stripANSI(got[1]) != "error (exit 1)" {
		t.Errorf("service row = %q", got)
	}
	if got := rows[1][1]; got != theme.Tint(testStyles.P.Green, "started") {
		t.Errorf("a started service should be green, without an exit code: %q", got)
	}
	if got := rows[0][1]; got != theme.Tint(testStyles.P.Red, "error (exit 1)") {
		t.Errorf("a failed service should be red: %q", got)
	}
	if got := tapRows(snap.Taps)[0]; got[1] != "2" || got[2] != "1" || got[3] != "" {
		t.Errorf("tap row = %v", got)
```

2. Replace:

```go
	if got := a.outdated.all[1][3]; got != theme.Nerd.Cask+" cask" {
		t.Errorf("outdated TYPE with icons = %q", got)
	}
	if got := a.services.all[0][1]; got != theme.Nerd.Failed+" error (exit 1)" {
		t.Errorf("service status with icons = %q", got)
	}
	keys(t, a, "2", "j", "j", "j") // git, pinned
```

   with:

```go
	if got := a.outdated.all[1][3]; got != theme.Nerd.Cask+" cask" {
		t.Errorf("outdated TYPE with icons = %q", got)
	}
	if got := stripANSI(a.services.all[0][1]); got != theme.Nerd.Failed+" error (exit 1)" {
		t.Errorf("service status with icons = %q", got)
	}
	keys(t, a, "2", "j", "j", "j") // git, pinned
```

3. Replace:

```go
		t.Errorf("icons off: %q", got)
	}
}
```

   with:

```go
		t.Errorf("icons off: %q", got)
	}
}

func TestSelectedRowStaysHighlighted(t *testing.T) {
	lt := newListTab(testStyles, "Services", serviceCols, firstCell, "none")
	lt.resize(60, 10)
	lt.setRows(serviceRows(testStyles, fixtureSnapshot(t).Services))
	for _, line := range strings.Split(lt.table.View(), "\n") {
		if !strings.Contains(line, "postgresql@16") {
			continue
		}
		// One full reset, at the very end: anything earlier would end the
		// highlight partway along the row.
		if n := strings.Count(line, "\x1b[m"); n != 1 || !strings.HasSuffix(line, "\x1b[m") {
			t.Errorf("the selected row resets %d times: %q", n, line)
		}
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/theme/theme_test.go:45:9: undefined: Tint
    internal/ui/jobsview_test.go:129:79: too many arguments in call to jobRows
    internal/ui/jobsview_test.go:140:25: undefined: theme.Tint
```

- [ ] **Step 5: Edit `internal/theme/theme.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
	"image/color"

	"charm.land/lipgloss/v2"
)

// Palette holds the Catppuccin color slots brewery uses.
```

   with:

```go
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Palette holds the Catppuccin color slots brewery uses.
```

2. Replace:

```go

		TableHeader: fg(p.Subtext0).Bold(true).Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(p.Surface1),
		TableCell:     fg(p.Text).Padding(0, 1),
		TableSelected: lipgloss.NewStyle().Foreground(p.Text).Background(p.Surface0).Bold(true),
	}
}
```

   with:

```go

		TableHeader: fg(p.Subtext0).Bold(true).Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(p.Surface1),
		// No colors on cells: a cell's closing reset would cut the selected
		// row's highlight short. Color inside a cell goes through Tint.
		TableCell:     lipgloss.NewStyle().Padding(0, 1),
		TableSelected: lipgloss.NewStyle().Foreground(p.Text).Background(p.Surface0).Bold(true),
	}
}
```

3. Replace:

```go
	}
	return lipgloss.NewStyle().Foreground(s.P.Crust).Background(c).Bold(true).Padding(0, 1)
}
```

   with:

```go
	}
	return lipgloss.NewStyle().Foreground(s.P.Crust).Background(c).Bold(true).Padding(0, 1)
}

// Tint colors s with c and then resets only the foreground, so text in a
// table cell keeps the selected row's background and bold.
func Tint(c color.Color, s string) string {
	return ansi.NewStyle().ForegroundColor(c).String() + s + ansi.NewStyle().DefaultForegroundColor().String()
}
```

- [ ] **Step 6: Edit `internal/ui/app.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
// there (newest first).
func (a *App) UseHistory(h jobs.History, recs []jobs.Record) {
	a.hist, a.history = &h, recs
	a.jobs.setRows(jobRows(recs, a.now()))
}

// refresh re-runs every query: the JSON ones in parallel, then doctor and
```

   with:

```go
// there (newest first).
func (a *App) UseHistory(h jobs.History, recs []jobs.Record) {
	a.hist, a.history = &h, recs
	a.jobs.setRows(jobRows(a.st, recs, a.now()))
}

// refresh re-runs every query: the JSON ones in parallel, then doctor and
```

2. Replace:

```go
	a.ix = state.NewIndex(a.snap.Packages)
	a.installed.setData(a.snap.Packages, a.ix)
	a.outdated.setRows(outdatedRows(a.st.Icons, a.snap.Outdated))
	a.services.setRows(serviceRows(a.st.Icons, a.snap.Services))
	a.taps.setRows(tapRows(a.snap.Taps))
}
```

   with:

```go
	a.ix = state.NewIndex(a.snap.Packages)
	a.installed.setData(a.snap.Packages, a.ix)
	a.outdated.setRows(outdatedRows(a.st.Icons, a.snap.Outdated))
	a.services.setRows(serviceRows(a.st, a.snap.Services))
	a.taps.setRows(tapRows(a.snap.Taps))
}
```

- [ ] **Step 7: Edit `internal/ui/installed.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	})
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{marked(p.Name, t.marked[pkgKey(p)]), p.Version, kindCell(t.st.Icons, p.Kind), status(t.st.Icons, p)}
	}
	before := t.selectedKey()
	setRowsKeeping(&t.table, rows, installedKey)
```

   with:

```go
	})
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{marked(p.Name, t.marked[pkgKey(p)]), p.Version, kindCell(t.st.Icons, p.Kind), status(t.st, p)}
	}
	before := t.selectedKey()
	setRowsKeeping(&t.table, rows, installedKey)
```

2. Replace:

```go
// and a cask can share a name.
func installedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[2] }

func status(ic theme.Icons, p brew.Package) string {
	var s []string
	if p.Pinned {
		s = append(s, theme.With(ic.Pinned, "pinned"))
	}
	if p.Outdated {
		s = append(s, "↑")
	}
	return strings.Join(s, " ")
}
```

   with:

```go
// and a cask can share a name.
func installedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[2] }

func status(st theme.Styles, p brew.Package) string {
	var s []string
	if p.Pinned {
		s = append(s, theme.With(st.Icons.Pinned, "pinned"))
	}
	if p.Outdated {
		s = append(s, theme.Tint(st.P.Yellow, "↑"))
	}
	return strings.Join(s, " ")
}
```

- [ ] **Step 8: Edit `internal/ui/jobsview.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
	tea "charm.land/bubbletea/v2"

	"brewery/internal/jobs"
)

var jobCols = []col{{"STATUS", 9, 0}, {"COMMAND", 0, 20}, {"STARTED", 12, 5}, {"TOOK", 6, 0}}
```

   with:

```go
	tea "charm.land/bubbletea/v2"

	"brewery/internal/jobs"
	"brewery/internal/theme"
)

var jobCols = []col{{"STATUS", 9, 0}, {"COMMAND", 0, 20}, {"STARTED", 12, 5}, {"TOOK", 6, 0}}
```

2. Replace:

```go
// jobKey identifies a jobs row by its command and start time.
func jobKey(r table.Row) string { return r[1] + "\x00" + r[2] }

// jobRows lists finished jobs, newest first as the history keeps them.
// Start times show the clock for today and the date before that.
func jobRows(recs []jobs.Record, now time.Time) []table.Row {
	rows := make([]table.Row, len(recs))
	for i, r := range recs {
		status := "✓ done"
		switch r.State {
		case jobs.Failed:
			status = fmt.Sprintf("✗ exit %d", r.ExitCode)
		case jobs.Canceled:
			status = "stopped"
		}
		started := r.Started.In(now.Location())
		when := started.Format("15:04")
```

   with:

```go
// jobKey identifies a jobs row by its command and start time.
func jobKey(r table.Row) string { return r[1] + "\x00" + r[2] }

// jobRows lists finished jobs, newest first as the history keeps them,
// their status colored. Start times show the clock for today and the date
// before that.
func jobRows(st theme.Styles, recs []jobs.Record, now time.Time) []table.Row {
	rows := make([]table.Row, len(recs))
	for i, r := range recs {
		status := theme.Tint(st.P.Green, "✓ done")
		switch r.State {
		case jobs.Failed:
			status = theme.Tint(st.P.Red, fmt.Sprintf("✗ exit %d", r.ExitCode))
		case jobs.Canceled:
			status = theme.Tint(st.P.Yellow, "stopped")
		}
		started := r.Started.In(now.Location())
		when := started.Format("15:04")
```

3. Replace:

```go
// openJobs shows the jobs view over the current tab.
func (a *App) openJobs() tea.Cmd {
	a.jobsOpen = true
	a.jobs.setRows(jobRows(a.history, a.now()))
	return nil
}
```

   with:

```go
// openJobs shows the jobs view over the current tab.
func (a *App) openJobs() tea.Cmd {
	a.jobsOpen = true
	a.jobs.setRows(jobRows(a.st, a.history, a.now()))
	return nil
}
```

- [ ] **Step 9: Edit `internal/ui/list.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	return rows
}

func serviceRows(ic theme.Icons, items []brew.Service) []table.Row {
	rows := make([]table.Row, len(items))
	for i, s := range items {
		rows[i] = table.Row{s.Name, theme.With(serviceIcon(ic, s.Status), serviceStatus(s))}
	}
	return rows
}
```

   with:

```go
	return rows
}

func serviceRows(st theme.Styles, items []brew.Service) []table.Row {
	rows := make([]table.Row, len(items))
	for i, s := range items {
		rows[i] = table.Row{s.Name, theme.Tint(serviceColor(st, s.Status), theme.With(serviceIcon(st.Icons, s.Status), serviceStatus(s)))}
	}
	return rows
}
```

- [ ] **Step 10: Edit `internal/ui/run.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	delete(a.acts, j.ID)
	r := jobs.NewRecord(j, output)
	a.history = append([]jobs.Record{r}, a.history[:min(len(a.history), jobs.Keep-1)]...)
	a.jobs.setRows(jobRows(a.history, a.now()))
	if a.hist == nil {
		return nil
	}
```

   with:

```go
	delete(a.acts, j.ID)
	r := jobs.NewRecord(j, output)
	a.history = append([]jobs.Record{r}, a.history[:min(len(a.history), jobs.Keep-1)]...)
	a.jobs.setRows(jobRows(a.st, a.history, a.now()))
	if a.hist == nil {
		return nil
	}
```

- [ ] **Step 11: Edit `internal/ui/services.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
```

   with:

```go
import (
	"errors"
	"fmt"
	"image/color"
	"io"
	"io/fs"
	"os"
```

2. Replace:

```go
	}
}

// serviceIcon is the icon for a service in this state.
func serviceIcon(ic theme.Icons, status string) string {
	switch status {
```

   with:

```go
	}
}

// serviceColor is the color of a service's state: green when it runs,
// red when it failed, muted when it's off, yellow for anything else.
func serviceColor(st theme.Styles, status string) color.Color {
	switch status {
	case "started", "scheduled":
		return st.P.Green
	case "error":
		return st.P.Red
	case "none", "stopped":
		return st.P.Overlay0
	}
	return st.P.Yellow
}

// serviceIcon is the icon for a service in this state.
func serviceIcon(ic theme.Icons, status string) string {
	switch status {
```

- [ ] **Step 12: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 13: Commit**

```bash
git add -A
git commit -m "fix(ui): color service and job states, and keep the selected row highlighted to its end"
```

---

### Task 7: Measure what each package takes on disk

**Files:**
- Modify: `cmd/brewery/main.go`
- Modify: `internal/brew/client.go`
- Modify: `internal/brew/live_test.go`
- Modify: `internal/brew/parse.go`
- Modify: `internal/brew/parse_test.go`
- Create: `internal/brew/sizes.go`
- Create: `internal/brew/sizes_test.go`
- Modify: `internal/brew/types.go`

**Interfaces:**
- **Consumes:** `ParseInfo`'s cask loop, `Client`.
- **Produces:**
  - `Package.Targets []string` (from each cask artifact's `target`), `(Package) Key() string`.
  - `Client.Prefix`, `PrefixOf(bin string) string`, `(*Client) Sizes(ctx, pkgs) (map[string]int64, error)` under the client's timeout, `dirSize(ctx, path)`.
  - `main` sets `client.Prefix`.

The live test now measures the real packages and expects at least half to have a size.

- [ ] **Step 1: Edit `internal/brew/live_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
		t.Skip("brew not installed")
	}
	c := NewClient(ExecRunner{Bin: bin, Env: QueryEnv})
	ctx := context.Background()

	pkgs, err := c.Installed(ctx)
```

   with:

```go
		t.Skip("brew not installed")
	}
	c := NewClient(ExecRunner{Bin: bin, Env: QueryEnv})
	c.Prefix = PrefixOf(bin)
	ctx := context.Background()

	pkgs, err := c.Installed(ctx)
```

2. Replace:

```go
			t.Errorf("incomplete package: %+v", p)
		}
	}
	if _, err := c.Outdated(ctx); err != nil {
		t.Error(err)
	}
```

   with:

```go
			t.Errorf("incomplete package: %+v", p)
		}
	}
	sizes, err := c.Sizes(ctx, pkgs)
	if err != nil {
		t.Error(err)
	}
	measured := 0
	for _, n := range sizes {
		if n > 0 {
			measured++
		}
	}
	if measured < len(pkgs)/2 {
		t.Errorf("only %d of %d packages have a size", measured, len(pkgs))
	}
	if _, err := c.Outdated(ctx); err != nil {
		t.Error(err)
	}
```

- [ ] **Step 2: Edit `internal/brew/parse_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	  {"name": "libusb", "full_name": "libusb", "installed": [{"version": "1.0", "runtime_dependencies": [{"full_name": "libffi", "declared_directly": true}]}]},
	  {"name": "libffi", "full_name": "libffi", "installed": [{"version": "3.5"}]}
	], "casks": [
	  {"token": "a", "full_token": "a", "depends_on": {"formula": ["libusb"], "macos": {">=": ["14"]}}},
	  {"token": "b", "full_token": "b", "depends_on": {"cask": ["a"]}},
	  {"token": "c", "full_token": "c", "depends_on": {}}
	]}`
```

   with:

```go
	  {"name": "libusb", "full_name": "libusb", "installed": [{"version": "1.0", "runtime_dependencies": [{"full_name": "libffi", "declared_directly": true}]}]},
	  {"name": "libffi", "full_name": "libffi", "installed": [{"version": "3.5"}]}
	], "casks": [
	  {"token": "a", "full_token": "a", "depends_on": {"formula": ["libusb"], "macos": {">=": ["14"]}},
	   "artifacts": [{"app": ["A.app"], "target": "/Applications/A.app"}, {"uninstall": [{"quit": "a"}]}, "odd", {"target": 7}, {"font": ["A.ttf"], "target": "/Users/x/Library/Fonts/A.ttf"}]},
	  {"token": "b", "full_token": "b", "depends_on": {"cask": ["a"]}},
	  {"token": "c", "full_token": "c", "depends_on": {}}
	]}`
```

2. Replace:

```go
		{"b", []string{"a", "libusb", "libffi"}, []string{"a"}},
		{"c", nil, nil},
	}
	for _, tt := range tests {
		p := byName(pkgs, tt.name)
		if !reflect.DeepEqual(p.Deps, tt.deps) || !reflect.DeepEqual(p.DirectDeps, tt.direct) {
```

   with:

```go
		{"b", []string{"a", "libusb", "libffi"}, []string{"a"}},
		{"c", nil, nil},
	}
	if a := byName(pkgs, "a"); !reflect.DeepEqual(a.Targets, []string{"/Applications/A.app", "/Users/x/Library/Fonts/A.ttf"}) {
		t.Errorf("a's targets = %v: only artifacts with a target count", a.Targets)
	}
	for _, tt := range tests {
		p := byName(pkgs, tt.name)
		if !reflect.DeepEqual(p.Deps, tt.deps) || !reflect.DeepEqual(p.DirectDeps, tt.direct) {
```

- [ ] **Step 3: Create `internal/brew/sizes_test.go`**

```go
package brew

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSizes(t *testing.T) {
	prefix, apps := t.TempDir(), t.TempDir()
	write(t, filepath.Join(prefix, "Cellar", "fd", "10.5.0", "bin", "fd"), 1000)
	write(t, filepath.Join(prefix, "Cellar", "fd", "10.4.0", "bin", "fd"), 500) // an old keg counts too
	write(t, filepath.Join(prefix, "Caskroom", "docker-desktop", "4.39.0", ".metadata"), 10)
	write(t, filepath.Join(apps, "Docker.app", "Contents", "MacOS", "Docker"), 4000)
	if err := os.Symlink("/etc/hosts", filepath.Join(apps, "Docker.app", "link")); err != nil {
		t.Fatal(err)
	}
	c := NewClient(fakeRunner{})
	c.Prefix = prefix
	pkgs := []Package{
		{Name: "fd", FullName: "fd", Kind: KindFormula},
		{Name: "docker-desktop", FullName: "docker-desktop", Kind: KindCask, Targets: []string{filepath.Join(apps, "Docker.app")}},
		{Name: "gone", FullName: "user/tap/gone", Kind: KindFormula},
	}
	got, err := c.Sizes(context.Background(), pkgs)
	if err != nil {
		t.Fatal(err)
	}
	link, _ := os.Lstat(filepath.Join(apps, "Docker.app", "link"))
	want := map[string]int64{
		"formula/fd":            1500,
		"cask/docker-desktop":   4010 + link.Size(),
		"formula/user/tap/gone": 0,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %d, want %d", k, got[k], w)
		}
	}
}

func TestSizesStopsWhenCancelled(t *testing.T) {
	prefix := t.TempDir()
	write(t, filepath.Join(prefix, "Cellar", "fd", "1", "f"), 1)
	c := NewClient(fakeRunner{})
	c.Prefix, c.Timeout = prefix, time.Minute
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Sizes(ctx, []Package{{Name: "fd", FullName: "fd"}}); err == nil {
		t.Error("a cancelled measure should fail")
	}
}

func TestPrefixOf(t *testing.T) {
	if got := PrefixOf("/opt/homebrew/bin/brew"); got != "/opt/homebrew" {
		t.Errorf("got %s", got)
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/brew/parse_test.go:94:50: a.Targets undefined (type Package has no field or method Targets)
    internal/brew/parse_test.go:95:70: a.Targets undefined (type Package has no field or method Targets)
    internal/brew/sizes_test.go:31:4: c.Prefix undefined (type *Client has no field or method Prefix)
```

- [ ] **Step 5: Edit `cmd/brewery/main.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	cachePath := state.DefaultCachePath()
	snap, _ := state.Load(cachePath) // no cache yet is fine: the first refresh fills it
	client := brew.NewClient(brew.ExecRunner{Bin: bin, Env: brew.QueryEnv})
	start := func(args, env []string, cols, rows int) (ui.Proc, error) {
		return brew.StartPTY(bin, args, env, cols, rows)
	}
```

   with:

```go
	cachePath := state.DefaultCachePath()
	snap, _ := state.Load(cachePath) // no cache yet is fine: the first refresh fills it
	client := brew.NewClient(brew.ExecRunner{Bin: bin, Env: brew.QueryEnv})
	client.Prefix = brew.PrefixOf(bin)
	start := func(args, env []string, cols, rows int) (ui.Proc, error) {
		return brew.StartPTY(bin, args, env, cols, rows)
	}
```

- [ ] **Step 6: Edit `internal/brew/client.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
type Client struct {
	r       Runner
	Timeout time.Duration
}

func NewClient(r Runner) *Client {
```

   with:

```go
type Client struct {
	r       Runner
	Timeout time.Duration
	Prefix  string // where brew is installed (/opt/homebrew): Cellar and Caskroom are in it
}

func NewClient(r Runner) *Client {
```

- [ ] **Step 7: Edit `internal/brew/parse.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
			Formula []string `json:"formula"`
			Cask    []string `json:"cask"`
		} `json:"depends_on"`
	} `json:"casks"`
}
```

   with:

```go
			Formula []string `json:"formula"`
			Cask    []string `json:"cask"`
		} `json:"depends_on"`
		Artifacts []json.RawMessage `json:"artifacts"` // read one by one: their shapes vary
	} `json:"casks"`
}
```

2. Replace:

```go
			p.InstalledAt = time.Unix(*c.InstalledTime, 0)
		}
		p.DirectDeps = append(slices.Clone(c.DependsOn.Formula), c.DependsOn.Cask...)
		pkgs = append(pkgs, p)
	}
	caskDeps(pkgs)
```

   with:

```go
			p.InstalledAt = time.Unix(*c.InstalledTime, 0)
		}
		p.DirectDeps = append(slices.Clone(c.DependsOn.Formula), c.DependsOn.Cask...)
		for _, raw := range c.Artifacts {
			var a struct {
				Target string `json:"target"`
			}
			if json.Unmarshal(raw, &a) == nil && a.Target != "" {
				p.Targets = append(p.Targets, a.Target) // where brew put an app, a font, a binary…
			}
		}
		pkgs = append(pkgs, p)
	}
	caskDeps(pkgs)
```

- [ ] **Step 8: Create `internal/brew/sizes.go`**

```go
package brew

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
)

// PrefixOf is the Homebrew prefix for the brew executable at bin, such as
// /opt/homebrew for /opt/homebrew/bin/brew.
func PrefixOf(bin string) string { return filepath.Dir(filepath.Dir(bin)) }

// Sizes measures the disk space each package uses, keyed by Package.Key: a
// formula's kegs in the Cellar, or a cask's files (its app in /Applications,
// fonts…) and its Caskroom folder. It walks the files itself rather than
// asking brew, which has no such command.
func (c *Client) Sizes(ctx context.Context, pkgs []Package) (map[string]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	sizes := make(map[string]int64, len(pkgs))
	for _, p := range pkgs {
		paths := []string{filepath.Join(c.Prefix, "Cellar", p.Name)}
		if p.Kind == KindCask {
			paths = append([]string{filepath.Join(c.Prefix, "Caskroom", p.Name)}, p.Targets...)
		}
		var n int64
		for _, path := range paths {
			size, err := dirSize(ctx, path)
			if err != nil {
				return nil, err
			}
			n += size
		}
		sizes[p.Key()] = n
	}
	return sizes, nil
}

// dirSize adds up the sizes of the files under path (or of path, if it is a
// file). Symlinks count as themselves, not what they point to. A missing
// path is 0; unreadable entries inside are skipped. It stops when ctx is
// done.
func dirSize(ctx context.Context, path string) (int64, error) {
	var n int64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return nil // a folder we can't read: count what we can
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if info, err := d.Info(); err == nil && !d.IsDir() {
			n += info.Size()
		}
		return nil
	})
	return n, err
}
```

- [ ] **Step 9: Edit `internal/brew/types.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	InstalledAt time.Time `json:"installed_at"`
	Deps        []string  `json:"deps"`        // all runtime deps (transitive), full names
	DirectDeps  []string  `json:"direct_deps"` // runtime deps declared directly
}

// Outdated is one row of `brew outdated --json=v2`.
type Outdated struct {
	Name      string `json:"name"`
```

   with:

```go
	InstalledAt time.Time `json:"installed_at"`
	Deps        []string  `json:"deps"`        // all runtime deps (transitive), full names
	DirectDeps  []string  `json:"direct_deps"` // runtime deps declared directly
	Targets     []string  `json:"targets"`     // a cask's installed files: apps, fonts…
}

// Key identifies a package: a formula and a cask can share a name.
func (p Package) Key() string { return p.Kind.String() + "/" + p.FullName }

// Outdated is one row of `brew outdated --json=v2`.
type Outdated struct {
	Name      string `json:"name"`
```

- [ ] **Step 10: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "feat(brew): measure what each package takes on disk"
```

---

### Task 8: Show what each package takes on disk

**Files:**
- Modify: `internal/state/snapshot.go`
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`
- Modify: `internal/ui/components.go`
- Modify: `internal/ui/helpers_test.go`
- Modify: `internal/ui/installed.go`
- Modify: `internal/ui/list_test.go`
- Modify: `internal/ui/marks.go`
- Modify: `internal/ui/marks_test.go`
- Create: `internal/ui/sizes_test.go`
- Modify: `internal/ui/source.go`

**Interfaces:**
- **Consumes:** Task 7's `Client.Sizes`, `Package.Key`; Task 2's `sizeTable`.
- **Produces:**
  - `Source.Sizes`, `qSizes` ("sizes" in the status bar), `resultMsg.sizes`; `fetch(ctx, src, q, pkgs)`, which `request` gives a copy of the snapshot's packages.
  - `diskQueries = {installed, outdated, sizes}` for upgrades and uninstalls; cleanup re-runs `{cleanup, sizes}`.
  - `Snapshot.Sizes` and version 3.
  - `installedTab.sizes`, `size(p)`; the SIZE column and the preview's `size` line; `humanSize`.
  - `col` with a negative `min` is optional (hidden before a flexible column gives way).

Installed's NAME keeps 20 and TYPE widens to 9 for the icon. Two expectations change with that: column widths, and a marked name cut at 120 columns.

- [ ] **Step 1: Edit `internal/ui/app_test.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
	}
	return f.snap.Doctor, nil
}
func (f *fakeSource) Cleanup(context.Context) (brew.Cleanup, error) {
	if f.cleanupErr != nil {
		return brew.Cleanup{}, f.cleanupErr
```

   with:

```go
	}
	return f.snap.Doctor, nil
}
func (f *fakeSource) Sizes(context.Context, []brew.Package) (map[string]int64, error) {
	return f.snap.Sizes, nil
}
func (f *fakeSource) Cleanup(context.Context) (brew.Cleanup, error) {
	if f.cleanupErr != nil {
		return brew.Cleanup{}, f.cleanupErr
```

2. Replace:

```go
	c.calls["taps"]++
	return c.fakeSource.Taps(ctx)
}
func (c *countingSource) Cleanup(ctx context.Context) (brew.Cleanup, error) {
	c.calls["cleanup"]++
	return c.fakeSource.Cleanup(ctx)
```

   with:

```go
	c.calls["taps"]++
	return c.fakeSource.Taps(ctx)
}
func (c *countingSource) Sizes(ctx context.Context, pkgs []brew.Package) (map[string]int64, error) {
	c.calls["sizes"]++
	return c.fakeSource.Sizes(ctx, pkgs)
}
func (c *countingSource) Cleanup(ctx context.Context) (brew.Cleanup, error) {
	c.calls["cleanup"]++
	return c.fakeSource.Cleanup(ctx)
```

3. Replace:

```go
	src.block = true
	a := newTestApp(t, src)
	done := make(chan tea.Msg)
	go func() { done <- fetch(a.ctx, src, qInstalled)() }()

	_, cmd := a.Update(press("q"))
	if _, ok := cmd().(tea.QuitMsg); !ok {
```

   with:

```go
	src.block = true
	a := newTestApp(t, src)
	done := make(chan tea.Msg)
	go func() { done <- fetch(a.ctx, src, qInstalled, nil)() }()

	_, cmd := a.Update(press("q"))
	if _, ok := cmd().(tea.QuitMsg); !ok {
```

- [ ] **Step 2: Edit `internal/ui/helpers_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		All:      brew.ParseCleanup(string(readFixture(t, "cleanup.txt"))),
	}
	s.CleanupRan = true
	return s
}
```

   with:

```go
		All:      brew.ParseCleanup(string(readFixture(t, "cleanup.txt"))),
	}
	s.CleanupRan = true
	s.Sizes = map[string]int64{"formula/git": 52_480_000, "cask/docker-desktop": 2_362_232_012, "formula/fd": 3_100_000}
	return s
}
```

- [ ] **Step 3: Edit `internal/ui/list_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
}

func TestSizeTableShrinksTheRoomiestColumnsFirst(t *testing.T) {
	tbl := newTable(testStyles, installedCols)
	sizeTable(&tbl, installedCols, 40, 10)
	// NAME wants 16, VERSION 16, TYPE 7, STATUS 10, plus 8 padding: 17 too
	// many. VERSION gives way first (it has the most to spare), then STATUS
	// and TYPE, each to its floor (8, 6, 4); NAME gives the last 2.
	got := []int{}
	for _, c := range tbl.Columns() {
		got = append(got, c.Width)
	}
	if want := []int{14, 8, 4, 6}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("widths = %v, want %v", got, want)
	}
	if w := tableWidth(tbl); w > 40 {
		t.Errorf("table is %d wide, want <= 40", w)
	}
	sizeTable(&tbl, installedCols, 50, 10)
	got = got[:0]
	for _, c := range tbl.Columns() {
		got = append(got, c.Width)
	}
	// 7 too many: VERSION gives 6 and STATUS 1; NAME and TYPE keep theirs.
	if want := []int{16, 10, 7, 9}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("at 50, widths = %v, want %v", got, want)
	}
}
```

   with:

```go
}

func TestSizeTableShrinksTheRoomiestColumnsFirst(t *testing.T) {
	widths := func(w int) string {
		tbl := newTable(testStyles, installedCols)
		sizeTable(&tbl, installedCols, w, 10)
		got := []int{}
		for _, c := range tbl.Columns() {
			got = append(got, c.Width)
		}
		if tw := tableWidth(tbl); tw > w {
			t.Errorf("at %d the table is %d wide", w, tw)
		}
		return fmt.Sprint(got)
	}
	// NAME wants 20; VERSION 16, TYPE 9, STATUS 10 and SIZE 8 (optional).
	for _, tt := range []struct {
		w    int
		want string
		why  string
	}{
		{62, "[20 10 6 8 8]", "VERSION gives way first, then STATUS and TYPE"},
		{50, "[24 8 4 6 0]", "fixed columns at their floors, then SIZE goes before NAME gives way"},
		{40, "[14 8 4 6 0]", "and only then does NAME shrink"},
	} {
		if got := widths(tt.w); got != tt.want {
			t.Errorf("at %d: widths %s, want %s: %s", tt.w, got, tt.want, tt.why)
		}
	}
}
```

- [ ] **Step 4: Edit `internal/ui/marks_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	keys(t, a, "2", "space")           // docker-desktop; the cursor moves on to fd
	keys(t, a, "j", "j", "j", "space") // lua-language-server
	out := stripANSI(a.render())
	if !strings.Contains(out, "● docker-desktop") || !strings.Contains(out, "● lua-language-server") {
		t.Errorf("marked rows should show it:\n%s", out)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "2 marked") {
```

   with:

```go
	keys(t, a, "2", "space")           // docker-desktop; the cursor moves on to fd
	keys(t, a, "j", "j", "j", "space") // lua-language-server
	out := stripANSI(a.render())
	if !strings.Contains(out, "● docker-desktop") || !strings.Contains(out, "● lua-language") {
		t.Errorf("marked rows should show it:\n%s", out)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "2 marked") {
```

- [ ] **Step 5: Create `internal/ui/sizes_test.go`**

```go
package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/state"
)

func TestHumanSize(t *testing.T) {
	for n, want := range map[int64]string{
		0: "0B", 999: "999B", 1536: "1.5KB", 52_480_000: "50.0MB", 2_362_232_012: "2.2GB",
	} {
		if got := humanSize(n); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestInstalledShowsSizes(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "2", "j", "j", "j") // git
	out := stripANSI(a.render())
	for _, want := range []string{"SIZE", "2.2GB", "50.0MB", "size       50.0MB"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	a.snap.Sizes = nil // not measured yet: no size rather than 0B
	a.sync()
	if out := stripANSI(a.render()); strings.Contains(out, "0B") || strings.Contains(out, "size ") {
		t.Errorf("unmeasured packages show no size:\n%s", out)
	}
}

func TestSizesAreMeasuredAfterRefreshAndChanges(t *testing.T) {
	src := newCountingSource(t)
	fs := &fakeStarter{}
	a := New(src, fs.start, state.Snapshot{}, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	drain(t, a, a.Init())
	if src.calls["sizes"] != 1 || len(a.snap.Sizes) != 3 {
		t.Fatalf("a full refresh measures sizes once: %v, %v", src.calls, a.snap.Sizes)
	}
	keys(t, a, "2", "j", "p") // pin fd: nothing on disk changes
	if src.calls["sizes"] != 1 {
		t.Errorf("pinning shouldn't measure again: %v", src.calls)
	}
	keys(t, a, "x", "y") // uninstall fd
	if src.calls["sizes"] != 2 {
		t.Errorf("uninstalling measures again: %v", src.calls)
	}
}
```

- [ ] **Step 6: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/app_test.go:41:16: f.snap.Sizes undefined (type state.Snapshot has no field or method Sizes)
    internal/ui/app_test.go:228:52: too many arguments in call to fetch
    internal/ui/helpers_test.go:52:4: s.Sizes undefined (type state.Snapshot has no field or method Sizes)
```

- [ ] **Step 7: Edit `internal/state/snapshot.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go

// snapshotVersion changes whenever Snapshot's shape changes; older cache
// files are ignored rather than half-loaded.
const snapshotVersion = 2

// Snapshot is everything brewery knows about the installation at one time.
type Snapshot struct {
```

   with:

```go

// snapshotVersion changes whenever Snapshot's shape changes; older cache
// files are ignored rather than half-loaded.
const snapshotVersion = 3

// Snapshot is everything brewery knows about the installation at one time.
type Snapshot struct {
```

2. Replace:

```go
	DoctorRan  bool                 `json:"doctor_ran"`
	Cleanup    brew.Cleanup         `json:"cleanup"`
	CleanupRan bool                 `json:"cleanup_ran"`
	FetchedAt  time.Time            `json:"fetched_at"`
}
```

   with:

```go
	DoctorRan  bool                 `json:"doctor_ran"`
	Cleanup    brew.Cleanup         `json:"cleanup"`
	CleanupRan bool                 `json:"cleanup_ran"`
	Sizes      map[string]int64     `json:"sizes"` // bytes on disk, by Package.Key
	FetchedAt  time.Time            `json:"fetched_at"`
}
```

- [ ] **Step 8: Edit `internal/ui/actions.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
	return append(args, names...)
}

// packageQueries are re-run after anything that installs, removes or pins.
var packageQueries = []query{qInstalled, qOutdated}

// actionKey handles the keys that ask brew to do something. ok is false
// when k is not an action on the current tab.
```

   with:

```go
	return append(args, names...)
}

// packageQueries are re-run after anything that installs, removes or pins;
// diskQueries after anything that also changes what's on disk.
var (
	packageQueries = []query{qInstalled, qOutdated}
	diskQueries    = []query{qInstalled, qOutdated, qSizes}
)

// actionKey handles the keys that ask brew to do something. ok is false
// when k is not an action on the current tab.
```

2. Replace:

```go
		}
		name, kind = o.Name, o.Kind
	}
	return a.run(action{title: "upgrade " + name, args: kindArgs("upgrade", kind, name), refresh: packageQueries})
}

// askUninstall confirms uninstalling the selected package. If something
```

   with:

```go
		}
		name, kind = o.Name, o.Kind
	}
	return a.run(action{title: "upgrade " + name, args: kindArgs("upgrade", kind, name), refresh: diskQueries})
}

// askUninstall confirms uninstalling the selected package. If something
```

3. Replace:

```go
		}
		return nil
	}
	refresh := append(slices.Clone(packageQueries), qServices) // its service goes too
	one := action{title: "uninstall " + p.Name, args: kindArgs("uninstall", p.Kind, p.FullName), refresh: refresh}
	box := &confirmBox{
		title:   fmt.Sprintf("Uninstall %s %s?", p.Name, p.Version),
```

   with:

```go
		}
		return nil
	}
	refresh := append(slices.Clone(diskQueries), qServices) // its service goes too
	one := action{title: "uninstall " + p.Name, args: kindArgs("uninstall", p.Kind, p.FullName), refresh: refresh}
	box := &confirmBox{
		title:   fmt.Sprintf("Uninstall %s %s?", p.Name, p.Version),
```

4. Replace:

```go
	}
	// This dialog is the confirmation, so brew 7 needn't ask again. The
	// variable, unlike --no-ask, is ignored by older brews.
	all := action{title: "upgrade all", args: []string{"upgrade"}, env: []string{"HOMEBREW_NO_ASK=1"}, refresh: packageQueries}
	a.confirm = &confirmBox{
		title: fmt.Sprintf("Upgrade all %d outdated?", n),
		body:  body,
```

   with:

```go
	}
	// This dialog is the confirmation, so brew 7 needn't ask again. The
	// variable, unlike --no-ask, is ignored by older brews.
	all := action{title: "upgrade all", args: []string{"upgrade"}, env: []string{"HOMEBREW_NO_ASK=1"}, refresh: diskQueries}
	a.confirm = &confirmBox{
		title: fmt.Sprintf("Upgrade all %d outdated?", n),
		body:  body,
```

5. Replace:

```go
	}
	box := &confirmBox{title: "Clean up Homebrew?"}
	if cl.Standard.Size != "" {
		std := action{title: "cleanup", args: []string{"cleanup"}, refresh: []query{qCleanup}}
		box.body = append(box.body, "The standard cleanup frees "+cl.Standard.Size+":", "  old versions, and downloads over 120 days old")
		box.choices = append(box.choices, choice{"y", "standard", func() tea.Cmd { return a.run(std) }})
	} else {
		box.body = append(box.body, "The standard cleanup has nothing to remove.")
	}
	if cl.All.Items > cl.Standard.Items {
		all := action{title: "cleanup --prune=all", args: []string{"cleanup", "--prune=all"}, refresh: []query{qCleanup}}
		box.body = append(box.body, "Clearing the download cache frees "+cl.All.Size+":", "  every download brew has kept")
		box.choices = append(box.choices, choice{"a", "whole cache", func() tea.Cmd { return a.run(all) }})
	}
```

   with:

```go
	}
	box := &confirmBox{title: "Clean up Homebrew?"}
	if cl.Standard.Size != "" {
		std := action{title: "cleanup", args: []string{"cleanup"}, refresh: []query{qCleanup, qSizes}}
		box.body = append(box.body, "The standard cleanup frees "+cl.Standard.Size+":", "  old versions, and downloads over 120 days old")
		box.choices = append(box.choices, choice{"y", "standard", func() tea.Cmd { return a.run(std) }})
	} else {
		box.body = append(box.body, "The standard cleanup has nothing to remove.")
	}
	if cl.All.Items > cl.Standard.Items {
		all := action{title: "cleanup --prune=all", args: []string{"cleanup", "--prune=all"}, refresh: []query{qCleanup, qSizes}}
		box.body = append(box.body, "Clearing the download cache frees "+cl.All.Size+":", "  every download brew has kept")
		box.choices = append(box.choices, choice{"a", "whole cache", func() tea.Cmd { return a.run(all) }})
	}
```

- [ ] **Step 9: Edit `internal/ui/app.go`** (6 replacements; each old block appears exactly once)

1. Replace:

```go
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
```

   with:

```go
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
```

2. Replace:

```go
	}
	a.loaded[qDoctor] = snap.DoctorRan
	a.loaded[qCleanup] = snap.CleanupRan
	a.sync()
	return a
}
```

   with:

```go
	}
	a.loaded[qDoctor] = snap.DoctorRan
	a.loaded[qCleanup] = snap.CleanupRan
	a.loaded[qSizes] = snap.Sizes != nil
	a.sync()
	return a
}
```

3. Replace:

```go
			continue
		}
		a.inflight[q] = true
		cmds = append(cmds, fetch(a.ctx, a.src, q))
	}
	return tea.Batch(cmds...)
}
```

   with:

```go
			continue
		}
		a.inflight[q] = true
		cmds = append(cmds, fetch(a.ctx, a.src, q, slices.Clone(a.snap.Packages)))
	}
	return tea.Batch(cmds...)
}
```

4. Replace:

```go
	a.services.loaded = a.loaded[qServices]
	a.taps.loaded = a.loaded[qTaps]
	a.ix = state.NewIndex(a.snap.Packages)
	a.installed.setData(a.snap.Packages, a.ix)
	a.outdated.setRows(outdatedRows(a.st.Icons, a.snap.Outdated))
	a.services.setRows(serviceRows(a.st, a.snap.Services))
```

   with:

```go
	a.services.loaded = a.loaded[qServices]
	a.taps.loaded = a.loaded[qTaps]
	a.ix = state.NewIndex(a.snap.Packages)
	a.installed.sizes = a.snap.Sizes
	a.installed.setData(a.snap.Packages, a.ix)
	a.outdated.setRows(outdatedRows(a.st.Icons, a.snap.Outdated))
	a.services.setRows(serviceRows(a.st, a.snap.Services))
```

5. Replace:

```go
		a.snap.Doctor, a.snap.DoctorRan = m.doctor, true
	case qCleanup:
		a.snap.Cleanup, a.snap.CleanupRan = m.cleanup, true
	}
	a.sync()
}
```

   with:

```go
		a.snap.Doctor, a.snap.DoctorRan = m.doctor, true
	case qCleanup:
		a.snap.Cleanup, a.snap.CleanupRan = m.cleanup, true
	case qSizes:
		a.snap.Sizes = m.sizes
	}
	a.sync()
}
```

6. Replace:

```go
	}
	if a.doctorAfter && !a.jsonBusy() {
		a.doctorAfter = false
		doctor = a.request(qDoctor, qCleanup)
	}
	// Save before doctor starts: doctor can take 30s, and quitting
	// meanwhile must not lose the JSON results.
```

   with:

```go
	}
	if a.doctorAfter && !a.jsonBusy() {
		a.doctorAfter = false
		doctor = a.request(qDoctor, qCleanup, qSizes)
	}
	// Save before doctor starts: doctor can take 30s, and quitting
	// meanwhile must not lose the JSON results.
```

- [ ] **Step 10: Edit `internal/ui/components.go`** (6 replacements; each old block appears exactly once)

1. Replace:

```go
// col describes a table column. Width 0 means "take the remaining space".
// min is how far it gives way when the table is too narrow: a flexible
// column keeps min (minFlex if 0) while fixed columns shrink, and a fixed
// column shrinks no further than min (its title's width if 0).
type col struct {
	title      string
	width, min int
```

   with:

```go
// col describes a table column. Width 0 means "take the remaining space".
// min is how far it gives way when the table is too narrow: a flexible
// column keeps min (minFlex if 0) while fixed columns shrink, and a fixed
// column shrinks no further than min (its title's width if 0). A fixed
// column with a negative min is optional: it never shrinks, and is hidden
// before a flexible column has to give way.
type col struct {
	title      string
	width, min int
```

2. Replace:

```go

// sizeTable fits t into a w×h area. When the columns don't fit, fixed
// columns give way first, a cell at a time from whichever has the most to
// spare (so a long VERSION shrinks before a short TYPE), then flexible
// columns down to minFlex, and then trailing columns are hidden. Flexible
// columns share whatever width is left. A hidden column keeps its slot with
// width 0, which the table skips: the table indexes columns by row cell, so
// removing columns would make it panic on rows that still carry every cell.
```

   with:

```go

// sizeTable fits t into a w×h area. When the columns don't fit, fixed
// columns give way first, a cell at a time from whichever has the most to
// spare (so a long VERSION shrinks before a short TYPE); then optional
// columns are hidden, then flexible columns shrink down to minFlex, and
// then trailing columns are hidden. Flexible
// columns share whatever width is left. A hidden column keeps its slot with
// width 0, which the table skips: the table indexes columns by row cell, so
// removing columns would make it panic on rows that still carry every cell.
```

3. Replace:

```go
		switch {
		case c.width == 0:
			widths[i], floor[i] = max(c.min, minFlex), minFlex
		case c.min > 0:
			widths[i], floor[i] = c.width, min(c.min, c.width)
		default:
```

   with:

```go
		switch {
		case c.width == 0:
			widths[i], floor[i] = max(c.min, minFlex), minFlex
		case c.min < 0:
			widths[i], floor[i] = c.width, c.width // optional: shown whole or not at all
		case c.min > 0:
			widths[i], floor[i] = c.width, min(c.min, c.width)
		default:
```

4. Replace:

```go
		}
	}
	shown := len(cols)
	// over is how much wider than w the shown columns are, with padding.
	over := func() int {
		n := 0
		for i := range shown {
			n += widths[i] + 2 // cell padding
		}
		return n - w
	}
	for _, flexible := range []bool{false, true} {
		for over() > 0 {
			give := -1
			for i := range shown {
```

   with:

```go
		}
	}
	shown := len(cols)
	hidden := make([]bool, len(cols))
	// over is how much wider than w the shown columns are, with padding.
	over := func() int {
		n := 0
		for i := range shown {
			if !hidden[i] {
				n += widths[i] + 2 // cell padding
			}
		}
		return n - w
	}
	for _, flexible := range []bool{false, true} {
		if flexible {
			for i := len(cols) - 1; i >= 0 && over() > 0; i-- {
				hidden[i] = hidden[i] || cols[i].min < 0
			}
		}
		for over() > 0 {
			give := -1
			for i := range shown {
```

5. Replace:

```go
	for i, c := range cols {
		width := widths[i]
		switch {
		case i >= shown:
			width = 0
		case c.width == 0:
			width += spare / flex
```

   with:

```go
	for i, c := range cols {
		width := widths[i]
		switch {
		case i >= shown || hidden[i]:
			width = 0
		case c.width == 0:
			width += spare / flex
```

6. Replace:

```go
	return b.String()
}

// skeletonRows is how many placeholder rows a list shows at most.
const skeletonRows = 8
```

   with:

```go
	return b.String()
}

// humanSize formats bytes the way brew does: 1024-based, one decimal,
// "KB", "MB" and "GB".
func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fGB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%dB", n)
}

// skeletonRows is how many placeholder rows a list shows at most.
const skeletonRows = 8
```

- [ ] **Step 11: Edit `internal/ui/installed.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
// viewBarHeight is the bar of views plus the line about the active one.
const viewBarHeight = 2

var installedCols = []col{{"NAME", 0, 16}, {"VERSION", 16, 8}, {"TYPE", 7, 4}, {"STATUS", 10, 6}}

// installedTab lists installed packages with a details preview.
type installedTab struct {
```

   with:

```go
// viewBarHeight is the bar of views plus the line about the active one.
const viewBarHeight = 2

var installedCols = []col{{"NAME", 0, 20}, {"VERSION", 16, 8}, {"TYPE", 9, 4}, {"STATUS", 10, 6}, {"SIZE", 8, -1}}

// installedTab lists installed packages with a details preview.
type installedTab struct {
```

2. Replace:

```go
	filter installedFilter
	all    []brew.Package
	ix     *state.Index
	rows   []brew.Package  // the filtered rows, in table order
	match  string          // the "/" filter on names
	pv     scroller        // the preview, which tab or enter focuses
	marked map[string]bool // packages picked with space, by pkgKey
	loaded bool
	table  table.Model
	w, h   int
```

   with:

```go
	filter installedFilter
	all    []brew.Package
	ix     *state.Index
	rows   []brew.Package   // the filtered rows, in table order
	match  string           // the "/" filter on names
	pv     scroller         // the preview, which tab or enter focuses
	marked map[string]bool  // packages picked with space, by pkgKey
	sizes  map[string]int64 // bytes on disk, by Package.Key; nil until measured
	loaded bool
	table  table.Model
	w, h   int
```

3. Replace:

```go
	})
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{marked(p.Name, t.marked[pkgKey(p)]), p.Version, kindCell(t.st.Icons, p.Kind), status(t.st, p)}
	}
	before := t.selectedKey()
	setRowsKeeping(&t.table, rows, installedKey)
```

   with:

```go
	})
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{marked(p.Name, t.marked[pkgKey(p)]), p.Version, kindCell(t.st.Icons, p.Kind), status(t.st, p), t.size(p)}
	}
	before := t.selectedKey()
	setRowsKeeping(&t.table, rows, installedKey)
```

4. Replace:

```go
// and a cask can share a name.
func installedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[2] }

func status(st theme.Styles, p brew.Package) string {
	var s []string
	if p.Pinned {
```

   with:

```go
// and a cask can share a name.
func installedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[2] }

// size is p's size on disk, or "" before it has been measured.
func (t *installedTab) size(p brew.Package) string {
	if n, ok := t.sizes[p.Key()]; ok {
		return humanSize(n)
	}
	return ""
}

func status(st theme.Styles, p brew.Package) string {
	var s []string
	if p.Pinned {
```

5. Replace:

```go
	if p.Deprecated {
		deprecated = st.Warn.Render("yes")
	}
	b.WriteString(kv(st, "type", p.Kind.String(), "tap", p.Tap, "installed", how,
		"latest", latest, "pinned", pinned, "deprecated", deprecated, "homepage", p.Homepage))

	if p.Kind == brew.KindFormula || len(p.DirectDeps) > 0 || len(t.ix.UsedBy(p.FullName)) > 0 {
```

   with:

```go
	if p.Deprecated {
		deprecated = st.Warn.Render("yes")
	}
	b.WriteString(kv(st, "type", p.Kind.String(), "tap", p.Tap, "size", t.size(p), "installed", how,
		"latest", latest, "pinned", pinned, "deprecated", deprecated, "homepage", p.Homepage))

	if p.Kind == brew.KindFormula || len(p.DirectDeps) > 0 || len(t.ix.UsedBy(p.FullName)) > 0 {
```

- [ ] **Step 12: Edit `internal/ui/marks.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	}
	a.clearMarks()
	return a.byKind(ts, func(kind brew.Kind, names []string) action {
		return action{title: "upgrade " + strings.Join(names, " "), args: kindArgs("upgrade", kind, names...), refresh: packageQueries}
	})
}
```

   with:

```go
	}
	a.clearMarks()
	return a.byKind(ts, func(kind brew.Kind, names []string) action {
		return action{title: "upgrade " + strings.Join(names, " "), args: kindArgs("upgrade", kind, names...), refresh: diskQueries}
	})
}
```

2. Replace:

```go
		}
		return nil
	}
	refresh := append(slices.Clone(packageQueries), qServices)
	ts := make([]target, len(pkgs))
	for i, p := range pkgs {
		ts[i] = target{p.FullName, p.Kind}
```

   with:

```go
		}
		return nil
	}
	refresh := append(slices.Clone(diskQueries), qServices)
	ts := make([]target, len(pkgs))
	for i, p := range pkgs {
		ts[i] = target{p.FullName, p.Kind}
```

- [ ] **Step 13: Edit `internal/ui/source.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go
	Taps(ctx context.Context) ([]brew.Tap, error)
	Doctor(ctx context.Context) ([]brew.DoctorWarning, error)
	Cleanup(ctx context.Context) (brew.Cleanup, error)
}

var _ Source = (*brew.Client)(nil)
```

   with:

```go
	Taps(ctx context.Context) ([]brew.Tap, error)
	Doctor(ctx context.Context) ([]brew.DoctorWarning, error)
	Cleanup(ctx context.Context) (brew.Cleanup, error)
	Sizes(ctx context.Context, pkgs []brew.Package) (map[string]int64, error)
}

var _ Source = (*brew.Client)(nil)
```

2. Replace:

```go
	qTaps
	qDoctor
	qCleanup
)

// queryNames name each query in the status bar.
var queryNames = [...]string{"installed", "outdated", "services", "taps", "doctor", "cleanup"}

// resultMsg carries one query's result back to the App.
type resultMsg struct {
```

   with:

```go
	qTaps
	qDoctor
	qCleanup
	qSizes
)

// queryNames name each query in the status bar.
var queryNames = [...]string{"installed", "outdated", "services", "taps", "doctor", "cleanup", "sizes"}

// resultMsg carries one query's result back to the App.
type resultMsg struct {
```

3. Replace:

```go
	taps     []brew.Tap
	doctor   []brew.DoctorWarning
	cleanup  brew.Cleanup
	err      error
}

// jsonQueries are the queries a full refresh runs in parallel; doctor and
// cleanup's dry-runs follow once they have all returned.
var jsonQueries = []query{qInstalled, qOutdated, qServices, qTaps}

// fetch runs one query. ctx is the App's root context, so quitting stops
// any brew process still running.
func fetch(ctx context.Context, src Source, q query) tea.Cmd {
	return func() tea.Msg {
		m := resultMsg{q: q}
		switch q {
```

   with:

```go
	taps     []brew.Tap
	doctor   []brew.DoctorWarning
	cleanup  brew.Cleanup
	sizes    map[string]int64
	err      error
}

// jsonQueries are the queries a full refresh runs in parallel; doctor,
// cleanup's dry-runs and the disk sizes follow once they have all returned.
var jsonQueries = []query{qInstalled, qOutdated, qServices, qTaps}

// fetch runs one query. ctx is the App's root context, so quitting stops
// any brew process still running. Sizes are measured for pkgs.
func fetch(ctx context.Context, src Source, q query, pkgs []brew.Package) tea.Cmd {
	return func() tea.Msg {
		m := resultMsg{q: q}
		switch q {
```

4. Replace:

```go
			m.doctor, m.err = src.Doctor(ctx)
		case qCleanup:
			m.cleanup, m.err = src.Cleanup(ctx)
		}
		return m
	}
```

   with:

```go
			m.doctor, m.err = src.Doctor(ctx)
		case qCleanup:
			m.cleanup, m.err = src.Cleanup(ctx)
		case qSizes:
			m.sizes, m.err = src.Sizes(ctx, pkgs)
		}
		return m
	}
```

- [ ] **Step 14: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 15: Commit**

```bash
git add -A
git commit -m "feat(ui): show what each package takes on disk"
```

---

### Task 9: Outdated says how old brew's package data is

**Files:**
- Modify: `internal/brew/client.go`
- Modify: `internal/brew/client_test.go`
- Modify: `internal/brew/live_test.go`
- Modify: `internal/state/snapshot.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`
- Modify: `internal/ui/list.go`
- Create: `internal/ui/outdated_test.go`
- Modify: `internal/ui/source.go`

**Interfaces:**
- **Consumes:** `Client.query`, `ago` (Home), `listTab.title`.
- **Produces:**
  - In `brew`: `dataFiles`, `(*Client) DataUpdated(ctx) (time.Time, error)`.
  - In `ui`: `Source.DataUpdated` (read with each Outdated result; errors ignored), `resultMsg.dataAt`, `Snapshot.DataAt`, `listTab.note func() string`, `(*App) dataAge()`.
  - Outdated's empty view now uses `title()`, so "up to date" says as of when.

- [ ] **Step 1: Edit `internal/brew/client_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
package brew

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)
```

   with:

```go
package brew

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)
```

2. Replace:

```go
	_, err := c.Doctor(context.Background())
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("want timeout error, got %v", err)
	}
}
```

   with:

```go
	_, err := c.Doctor(context.Background())
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("want timeout error, got %v", err)
	}
}

func TestClientDataUpdated(t *testing.T) {
	cache, repo := t.TempDir(), t.TempDir()
	older, newer := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	for path, at := range map[string]time.Time{
		filepath.Join(cache, "api", "formula_names.txt"): newer,
		filepath.Join(repo, ".git", "FETCH_HEAD"):        older,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	c := NewClient(fakeRunner{out: map[string][]byte{"--cache": []byte(cache + "\n"), "--repository": []byte(repo + "\n")}})
	got, err := c.DataUpdated(context.Background())
	if err != nil || !got.Equal(newer) {
		t.Errorf("got %v, %v; want the newest file's time %v", got, err, newer)
	}

	c = NewClient(fakeRunner{out: map[string][]byte{"--cache": []byte(t.TempDir()), "--repository": []byte(t.TempDir())}})
	if _, err := c.DataUpdated(context.Background()); err == nil {
		t.Error("with none of the files, it's an error")
	}
}
```

- [ ] **Step 2: Edit `internal/brew/live_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	"io"
	"strings"
	"testing"
)

// Runs the real brew, read-only, to catch output-format drift.
```

   with:

```go
	"io"
	"strings"
	"testing"
	"time"
)

// Runs the real brew, read-only, to catch output-format drift.
```

2. Replace:

```go
	if measured < len(pkgs)/2 {
		t.Errorf("only %d of %d packages have a size", measured, len(pkgs))
	}
	if _, err := c.Outdated(ctx); err != nil {
		t.Error(err)
	}
```

   with:

```go
	if measured < len(pkgs)/2 {
		t.Errorf("only %d of %d packages have a size", measured, len(pkgs))
	}
	if at, err := c.DataUpdated(ctx); err != nil || at.After(time.Now()) {
		t.Errorf("brew's data updated %v, %v", at, err)
	}
	if _, err := c.Outdated(ctx); err != nil {
		t.Error(err)
	}
```

- [ ] **Step 3: Edit `internal/ui/app_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	}
	return f.snap.Doctor, nil
}
func (f *fakeSource) Sizes(context.Context, []brew.Package) (map[string]int64, error) {
	return f.snap.Sizes, nil
}
```

   with:

```go
	}
	return f.snap.Doctor, nil
}
func (f *fakeSource) DataUpdated(context.Context) (time.Time, error) {
	return testNow.Add(-6 * time.Hour), nil
}
func (f *fakeSource) Sizes(context.Context, []brew.Package) (map[string]int64, error) {
	return f.snap.Sizes, nil
}
```

- [ ] **Step 4: Create `internal/ui/outdated_test.go`**

```go
package ui

import (
	"strings"
	"testing"
	"time"
)

func TestOutdatedSaysHowOldBrewsDataIs(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	keys(t, a, "3")
	if out := stripANSI(a.render()); !strings.Contains(out, "Outdated · 2 · brew's data from 6h ago") {
		t.Errorf("the title should say how old the data is:\n%s", out)
	}
	a.snap.Outdated = nil
	a.sync()
	if out := stripANSI(a.render()); !strings.Contains(out, "brew's data from 6h ago") || !strings.Contains(out, "up to date") {
		t.Errorf("\"up to date\" says as of when:\n%s", out)
	}
	a.snap.DataAt = time.Time{}
	if out := stripANSI(a.render()); strings.Contains(out, "brew's data") {
		t.Errorf("no time known, nothing said:\n%s", out)
	}
}
```

- [ ] **Step 5: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/brew/client_test.go:147:16: c.DataUpdated undefined (type *Client has no field or method DataUpdated)
    internal/brew/client_test.go:153:17: c.DataUpdated undefined (type *Client has no field or method DataUpdated)
    internal/ui/outdated_test.go:21:9: a.snap.DataAt undefined (type state.Snapshot has no field or method DataAt)
```

- [ ] **Step 6: Edit `internal/brew/client.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	}
	return warnings, nil
}
```

   with:

```go
	}
	return warnings, nil
}

// dataFiles are files brew rewrites when it fetches new package data, by
// path under its cache or its repository. Which exist depends on brew's
// version and whether it reads the API or a tapped homebrew/core.
var dataFiles = []struct {
	inCache bool
	path    string
}{
	{true, "api/formula_names.txt"},
	{true, "api/formula.jws.json"},
	{false, ".git/FETCH_HEAD"},
}

// DataUpdated is when brew last fetched package data (brew update, or its
// auto-update): the newest of its data files.
func (c *Client) DataUpdated(ctx context.Context) (time.Time, error) {
	cache, err := c.query(ctx, "--cache")
	if err != nil {
		return time.Time{}, err
	}
	repo, err := c.query(ctx, "--repository")
	if err != nil {
		return time.Time{}, err
	}
	var newest time.Time
	for _, f := range dataFiles {
		dir := strings.TrimSpace(string(repo))
		if f.inCache {
			dir = strings.TrimSpace(string(cache))
		}
		if info, err := os.Stat(filepath.Join(dir, f.path)); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	if newest.IsZero() {
		return time.Time{}, errors.New("found none of brew's data files")
	}
	return newest, nil
}
```

- [ ] **Step 7: Edit `internal/state/snapshot.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	DoctorRan  bool                 `json:"doctor_ran"`
	Cleanup    brew.Cleanup         `json:"cleanup"`
	CleanupRan bool                 `json:"cleanup_ran"`
	Sizes      map[string]int64     `json:"sizes"` // bytes on disk, by Package.Key
	FetchedAt  time.Time            `json:"fetched_at"`
}
```

   with:

```go
	DoctorRan  bool                 `json:"doctor_ran"`
	Cleanup    brew.Cleanup         `json:"cleanup"`
	CleanupRan bool                 `json:"cleanup_ran"`
	Sizes      map[string]int64     `json:"sizes"`   // bytes on disk, by Package.Key
	DataAt     time.Time            `json:"data_at"` // when brew last fetched package data
	FetchedAt  time.Time            `json:"fetched_at"`
}
```

- [ ] **Step 8: Edit `internal/ui/app.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
	a.jobs.loaded = true
	a.jobs.matchCol = 1 // jobs are found by their command
	a.services.preview = a.servicePreview
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
		for _, q := range []query{qInstalled, qOutdated, qServices, qTaps} {
```

   with:

```go
	a.jobs.loaded = true
	a.jobs.matchCol = 1 // jobs are found by their command
	a.services.preview = a.servicePreview
	a.outdated.note = a.dataAge
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
		for _, q := range []query{qInstalled, qOutdated, qServices, qTaps} {
```

2. Replace:

```go

func (a *App) Init() tea.Cmd { return tea.Batch(a.refresh(), a.loadCompletions()) }

// UseIcons turns Nerd Font icons on or off. They're off until this is
// called, so tests and other callers get plain words.
func (a *App) UseIcons(on bool) {
```

   with:

```go

func (a *App) Init() tea.Cmd { return tea.Batch(a.refresh(), a.loadCompletions()) }

// dataAge says how old brew's package data is, which is what Outdated is
// measured against, or "" if that isn't known.
func (a *App) dataAge() string {
	if a.snap.DataAt.IsZero() {
		return ""
	}
	return "brew's data from " + ago(a.now(), a.snap.DataAt)
}

// UseIcons turns Nerd Font icons on or off. They're off until this is
// called, so tests and other callers get plain words.
func (a *App) UseIcons(on bool) {
```

3. Replace:

```go
	case qInstalled:
		a.snap.Packages = m.packages
	case qOutdated:
		a.snap.Outdated = m.outdated
	case qServices:
		a.snap.Services = m.services
		a.log.stale = true // the service may have written more since
```

   with:

```go
	case qInstalled:
		a.snap.Packages = m.packages
	case qOutdated:
		a.snap.Outdated, a.snap.DataAt = m.outdated, m.dataAt
	case qServices:
		a.snap.Services = m.services
		a.log.stale = true // the service may have written more since
```

- [ ] **Step 9: Edit `internal/ui/list.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
	// for a w×h pane.
	preview func(w, h int) (title, body string)
	pvOpen  bool // below previewMinWidth, the preview shows instead of the list
}

func newListTab(st theme.Styles, name string, cols []col, key func(table.Row) string, empty string) listTab {
```

   with:

```go
	// for a w×h pane.
	preview func(w, h int) (title, body string)
	pvOpen  bool // below previewMinWidth, the preview shows instead of the list

	note func() string // if set and not "", added to the title
}

func newListTab(st theme.Styles, name string, cols []col, key func(table.Row) string, empty string) listTab {
```

2. Replace:

```go
	if t.match != "" {
		s += " · /" + t.match
	}
	return s
}
```

   with:

```go
	if t.match != "" {
		s += " · /" + t.match
	}
	if t.note != nil {
		if n := t.note(); n != "" {
			s += " · " + n
		}
	}
	return s
}
```

3. Replace:

```go
	case t.n == 0 && len(t.all) > 0:
		return pane(t.st, t.title(), t.st.Muted.Render("nothing matches /"+t.match+" · / then esc clears it"), w, h, true)
	case t.n == 0:
		return pane(t.st, t.name, t.st.OK.Render(t.empty), w, h, true)
	}
	body := ""
	if err != nil {
```

   with:

```go
	case t.n == 0 && len(t.all) > 0:
		return pane(t.st, t.title(), t.st.Muted.Render("nothing matches /"+t.match+" · / then esc clears it"), w, h, true)
	case t.n == 0:
		return pane(t.st, t.title(), t.st.OK.Render(t.empty), w, h, true) // "up to date" says as of when
	}
	body := ""
	if err != nil {
```

- [ ] **Step 10: Edit `internal/ui/source.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go

import (
	"context"

	tea "charm.land/bubbletea/v2"
```

   with:

```go

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
```

2. Replace:

```go
	Doctor(ctx context.Context) ([]brew.DoctorWarning, error)
	Cleanup(ctx context.Context) (brew.Cleanup, error)
	Sizes(ctx context.Context, pkgs []brew.Package) (map[string]int64, error)
}

var _ Source = (*brew.Client)(nil)
```

   with:

```go
	Doctor(ctx context.Context) ([]brew.DoctorWarning, error)
	Cleanup(ctx context.Context) (brew.Cleanup, error)
	Sizes(ctx context.Context, pkgs []brew.Package) (map[string]int64, error)
	DataUpdated(ctx context.Context) (time.Time, error)
}

var _ Source = (*brew.Client)(nil)
```

3. Replace:

```go
	doctor   []brew.DoctorWarning
	cleanup  brew.Cleanup
	sizes    map[string]int64
	err      error
}
```

   with:

```go
	doctor   []brew.DoctorWarning
	cleanup  brew.Cleanup
	sizes    map[string]int64
	dataAt   time.Time // with outdated: when brew last fetched package data, if known
	err      error
}
```

4. Replace:

```go
			m.packages, m.err = src.Installed(ctx)
		case qOutdated:
			m.outdated, m.err = src.Outdated(ctx)
		case qServices:
			m.services, m.err = src.Services(ctx)
		case qTaps:
```

   with:

```go
			m.packages, m.err = src.Installed(ctx)
		case qOutdated:
			m.outdated, m.err = src.Outdated(ctx)
			if m.err == nil {
				m.dataAt, _ = src.DataUpdated(ctx) // only a hint: no time is fine
			}
		case qServices:
			m.services, m.err = src.Services(ctx)
		case qTaps:
```

- [ ] **Step 11: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 12: Commit**

```bash
git add -A
git commit -m "feat: Outdated says how old brew's package data is"
```

---

### Task 10: Plan 5's review follow-ups

**Files:**
- Modify: `internal/jobs/history.go`
- Modify: `internal/jobs/history_test.go`
- Modify: `internal/state/snapshot.go`
- Modify: `internal/state/snapshot_test.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/help.go`
- Modify: `internal/ui/help_test.go`
- Modify: `internal/ui/links_test.go`
- Modify: `internal/ui/marks_test.go`
- Modify: `internal/ui/run.go`
- Modify: `internal/ui/run_test.go`
- Modify: `internal/ui/scroll.go`
- Modify: `internal/ui/scroll_test.go`

**Interfaces:**
- **Consumes:** Plan 5's `quitNowMsg`, `gPending`, `scroller`, `helpGroups`, `DefaultStateDir`, `DefaultCachePath`.
- **Produces:**
  - `(*App) typeIntoJob()` replaces the three places that switched to INSERT.
  - `scroller.last`, `scroller.sized`.
  - The jobs-view Lists help.
  - XDG variables must be absolute.

The tests added here also cover:
- `x` listing a mark the filter hides;
- the key that closes help acting on nothing;
- `gg` on Outdated.

- [ ] **Step 1: Edit `internal/jobs/history_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Errorf("without a home directory state goes in the temp directory, got %s", got)
	}
}
```

   with:

```go
		t.Errorf("without a home directory state goes in the temp directory, got %s", got)
	}
}

func TestDefaultStateDirIgnoresARelativeXDG(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "relative/state")
	if got := DefaultStateDir(); !filepath.IsAbs(got) {
		t.Errorf("a relative XDG_STATE_HOME is ignored, got %s", got)
	}
}
```

- [ ] **Step 2: Edit `internal/state/snapshot_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Errorf("temp file left behind: %v", left)
	}
}
```

   with:

```go
		t.Errorf("temp file left behind: %v", left)
	}
}

func TestDefaultCachePathIgnoresARelativeXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "relative/cache")
	if got := DefaultCachePath(); !filepath.IsAbs(got) {
		t.Errorf("a relative XDG_CACHE_HOME is ignored, got %s", got)
	}
}
```

- [ ] **Step 3: Edit `internal/ui/help_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

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

func TestHelpInJobsSaysWhatYCopies(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter", "?")
	out := stripANSI(a.render())
	if !strings.Contains(out, "copy the command") || strings.Contains(out, "open the homepage") {
		t.Errorf("in :jobs, y copies the command and gx has nothing to open:\n%s", out)
	}
}

func TestHelpClosesWithoutActing(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "j", "?", "x") // x would ask to uninstall fd
	if a.help || a.confirm != nil || len(fs.calls) != 0 {
		t.Errorf("the key that closes help does nothing else: help %v, confirm %+v", a.help, a.confirm)
	}
}
```

- [ ] **Step 4: Edit `internal/ui/links_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Errorf("status bar %q", bar)
	}
}
```

   with:

```go
		t.Errorf("status bar %q", bar)
	}
}

func TestBrewsQuestionForgetsAPendingG(t *testing.T) {
	p := newFakeProc(0, "working\r\n")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	l := &fakeLinks{}
	a.UseOpener(l.open)
	waiting := settle(t, a, a.run(action{title: "upgrade", args: []string{"upgrade"}}))
	keys(t, a, "2", "g") // half of a gx…
	p.out <- []byte("Password:")
	waiting = resume(t, a, waiting) // …when brew asks
	if a.mode != modeInsert {
		t.Fatalf("brew's question switches to insert, mode %v", a.mode)
	}
	keys(t, a, "esc", "x")
	if len(l.opened) != 0 {
		t.Errorf("the g before the question was forgotten, so x isn't gx: opened %v", l.opened)
	}
	p.exit()
	resume(t, a, waiting)
}

func TestGGOnOutdated(t *testing.T) {
	a, _ := newLinksApp(t)
	keys(t, a, "3", "j", "g", "g", "u")
	if a.outdated.cursor() != 0 {
		t.Errorf("gg goes to the top of Outdated: cursor %d", a.outdated.cursor())
	}
}
```

- [ ] **Step 5: Edit `internal/ui/marks_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Errorf("a mark on a removed package should go: %d", a.marks())
	}
}
```

   with:

```go
		t.Errorf("a mark on a removed package should go: %d", a.marks())
	}
}

func TestUninstallSeveralListsHiddenMarks(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "2", "j", "space") // fd
	keys(t, a, "/")
	typeText(t, a, "git")
	keys(t, a, "enter", "space", "x") // git; fd is now hidden by the filter
	if a.confirm == nil || a.confirm.title != "Uninstall 2 packages?" {
		t.Fatalf("x should ask about both: %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "fd, git") {
		t.Errorf("the hidden mark is listed too:\n%s", out)
	}
}
```

- [ ] **Step 6: Edit `internal/ui/run_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	retry.exit()
	resume(t, a, waiting)
}
```

   with:

```go
	retry.exit()
	resume(t, a, waiting)
}

func TestQuitTimerAfterTheJobStoppedDoesNothing(t *testing.T) {
	a, _, _, _ := quitWithJob(t)
	j := a.queue.Current()
	a.Update(jobExitMsg{j.ID, j.Attempts, 130}) // stopped: brewery saves it, then quits
	if _, cmd := a.Update(quitNowMsg{}); cmd != nil {
		t.Error("a late grace timer mustn't quit before the save is done")
	}
}
```

- [ ] **Step 7: Edit `internal/ui/scroll_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Errorf("coming back to git starts its preview at the top:\n%s", out)
	}
}
```

   with:

```go
		t.Errorf("coming back to git starts its preview at the top:\n%s", out)
	}
}

func TestScrollerKAfterGMovesUpAtOnce(t *testing.T) {
	var s scroller
	body := numbered(10)
	s.window(body, 4) // drawn once: the last screenful starts at line 7
	s.scroll("G", 2)
	s.scroll("k", 2)
	if s.off != 5 {
		t.Errorf("k after G should move up one line from the bottom: off %d, want 5", s.off)
	}
}
```

- [ ] **Step 8: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    --- FAIL: TestDefaultStateDirIgnoresARelativeXDG (0.00s)
    --- FAIL: TestDefaultCachePathIgnoresARelativeXDG (0.00s)
    --- FAIL: TestHelpInJobsSaysWhatYCopies (0.01s)
```

- [ ] **Step 9: Edit `internal/jobs/history.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
}

// DefaultStateDir is $XDG_STATE_HOME/brewery, falling back to
// ~/.local/state/brewery. Without a home directory it uses the temp
// directory rather than a path relative to wherever brewery started.
func DefaultStateDir() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = os.TempDir()
```

   with:

```go
}

// DefaultStateDir is $XDG_STATE_HOME/brewery, falling back to
// ~/.local/state/brewery when it's unset or not absolute. Without a home directory it uses the temp
// directory rather than a path relative to wherever brewery started.
func DefaultStateDir() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(dir) { // unset, or relative, which the XDG spec says to ignore
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = os.TempDir()
```

- [ ] **Step 10: Edit `internal/state/snapshot.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
}

// DefaultCachePath is $XDG_CACHE_HOME/brewery/snapshot.json, falling back
// to ~/.cache/brewery/snapshot.json. Without a home directory it uses the
// temp directory rather than a path relative to wherever brewery started.
func DefaultCachePath() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = os.TempDir()
```

   with:

```go
}

// DefaultCachePath is $XDG_CACHE_HOME/brewery/snapshot.json, falling back
// to ~/.cache/brewery/snapshot.json when it's unset or not absolute. Without a home directory it uses the
// temp directory rather than a path relative to wherever brewery started.
func DefaultCachePath() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(dir) { // unset, or relative, which the XDG spec says to ignore
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = os.TempDir()
```

- [ ] **Step 11: Edit `internal/ui/app.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go
	if a.promptPending && a.mode == modeNormal {
		a.promptPending = false
		if a.proc != nil && a.drawer.prompt() {
			a.mode, a.drawer.open = modeInsert, true
		}
	}
	a.layout()
```

   with:

```go
	if a.promptPending && a.mode == modeNormal {
		a.promptPending = false
		if a.proc != nil && a.drawer.prompt() {
			a.typeIntoJob()
		}
	}
	a.layout()
```

2. Replace:

```go
	case takeoverDoneMsg:
		return a.takeoverDone(msg)
	case quitNowMsg:
		return a.quitNow()
	case logMsg:
		a.gotLog(msg)
```

   with:

```go
	case takeoverDoneMsg:
		return a.takeoverDone(msg)
	case quitNowMsg:
		if a.proc == nil {
			return nil // the job has stopped and brewery is already saving it on the way out
		}
		return a.quitNow()
	case logMsg:
		a.gotLog(msg)
```

3. Replace:

```go
	return tea.Batch(again, tea.Sequence(saving, doctor))
}

// handleKey routes a key press by mode first: an open confirmation gets
// every key, and in insert mode keys go to brew, so typing an answer can
// never trigger a brewery shortcut such as q.
```

   with:

```go
	return tea.Batch(again, tea.Sequence(saving, doctor))
}

// typeIntoJob switches to INSERT with the drawer open, so keys go to the
// running job. A g pressed just before is forgotten: after INSERT, the next
// key mustn't complete a gg or gx.
func (a *App) typeIntoJob() {
	a.mode, a.drawer.open, a.gPending = modeInsert, true, false
}

// handleKey routes a key press by mode first: an open confirmation gets
// every key, and in insert mode keys go to brew, so typing an answer can
// never trigger a brewery shortcut such as q.
```

4. Replace:

```go
		return nil
	case "i":
		if a.proc != nil {
			a.mode, a.drawer.open = modeInsert, true
		}
		return nil
	case "1", "2", "3", "4", "5":
```

   with:

```go
		return nil
	case "i":
		if a.proc != nil {
			a.typeIntoJob()
		}
		return nil
	case "1", "2", "3", "4", "5":
```

- [ ] **Step 12: Edit `internal/ui/help.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	}
	groups := []helpGroup{here}
	if a.jobsOpen || a.tab != tabHome {
		groups = append(groups, helpGroup{"Lists", [][2]string{
			{"j k", "move"}, {"gg G", "top, bottom"}, {"ctrl+d/u", "half a page"},
			{"/", "filter by name"}, {"y", "copy the name"}, {"gx", "open the homepage"},
```

   with:

```go
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	}
	groups := []helpGroup{here}
	switch {
	case a.jobsOpen:
		groups = append(groups, helpGroup{"Lists", [][2]string{
			{"j k", "move"}, {"gg G", "top, bottom"}, {"ctrl+d/u", "half a page"},
			{"/", "filter by command"}, {"y", "copy the command"},
		}})
	case a.tab != tabHome:
		groups = append(groups, helpGroup{"Lists", [][2]string{
			{"j k", "move"}, {"gg G", "top, bottom"}, {"ctrl+d/u", "half a page"},
			{"/", "filter by name"}, {"y", "copy the name"}, {"gx", "open the homepage"},
```

- [ ] **Step 13: Edit `internal/ui/run.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		a.drawer.write(m.data)
		switch {
		case a.mode == modeNormal && a.drawer.prompt():
			a.mode, a.drawer.open = modeInsert, true // brew is asking: let the user answer
		case (a.mode == modeCommand || a.mode == modeFilter) && a.drawer.prompt():
			a.promptPending = true // answer once the line being typed closes
		}
```

   with:

```go
		a.drawer.write(m.data)
		switch {
		case a.mode == modeNormal && a.drawer.prompt():
			a.typeIntoJob() // brew is asking: let the user answer
		case (a.mode == modeCommand || a.mode == modeFilter) && a.drawer.prompt():
			a.promptPending = true // answer once the line being typed closes
		}
```

- [ ] **Step 14: Edit `internal/ui/scroll.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
type scroller struct {
	focus bool // keys scroll the preview rather than move through the list
	off   int  // lines scrolled past
}

// scroll moves by a scrolling key, page lines for ctrl+d and ctrl+u. ok
```

   with:

```go
type scroller struct {
	focus bool // keys scroll the preview rather than move through the list
	off   int  // lines scrolled past
	last  int  // the furthest off can go, as of the last window
	sized bool // a window has set last
}

// scroll moves by a scrolling key, page lines for ctrl+d and ctrl+u. ok
```

2. Replace:

```go
		return false
	}
	s.off = max(s.off, 0)
	return true
}
```

   with:

```go
		return false
	}
	s.off = max(s.off, 0)
	if s.sized {
		s.off = min(s.off, s.last) // so k right after G moves up at once
	}
	return true
}
```

3. Replace:

```go
// clamps the offset so the last line can't scroll out of view.
func (s *scroller) window(body string, h int) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	s.off = min(s.off, max(len(lines)-h, 0))
	return strings.Join(lines[s.off:], "\n")
}
```

   with:

```go
// clamps the offset so the last line can't scroll out of view.
func (s *scroller) window(body string, h int) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	s.last, s.sized = max(len(lines)-h, 0), true
	s.off = min(s.off, s.last)
	return strings.Join(lines[s.off:], "\n")
}
```

- [ ] **Step 15: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 16: Commit**

```bash
git add -A
git commit -m "fix: Plan 5 follow-ups — late quit timer, stale g, k after G, jobs help, relative XDG paths"
```

---

### Task 11: End-to-end check in a real terminal

**Files:** none (verification only)

- [ ] **Step 1: Build and run**

Run: `make run`

- [ ] **Step 2: Check each of these by hand.** None of these change anything.
  1. **Icons and sizes:** press `2`. TYPE shows the beer and app icons, and once the refresh settles a SIZE column fills in (casks show their app's size). The preview has a `size` line.
  2. **Highlight:** move with `j`. The selected row stays highlighted across its whole width.
  3. **Data age:** press `3`. The title ends `brew's data from …`.
  4. **Colors:** press `4`. Service statuses are colored: green for started, red for error.
  5. **Layout:** narrow the window below 100 columns: the preview moves under the list. Below 60, it disappears; `tab` (Installed) or `enter` (Services) shows it, and `esc` comes back.
  6. **Tiny windows:** make the window about 8 rows tall and run `:doctor`. The status bar stays at the bottom.
  7. **Plain text:** quit, then run `./brewery --no-icons`. No icons appear.

- [ ] **Step 3: Final checks**

```bash
go test -race -count=1 ./...
go test -tags live -count=1 ./internal/brew/ ./internal/ui/
go vet ./...
gofmt -l .
go mod tidy -diff
```

Expected: all `ok`, with no output from vet, gofmt, or `go mod tidy -diff`.
