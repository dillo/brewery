# History and Maintenance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Step 3 of the sidebar layout: a TOOLS section with History (the `:jobs` view as a tab with details) and Maintenance (doctor, cleanup and brew update as rows), reached with `5`.

**Architecture:** Both are ordinary tabs built on `listTab`, so they get the sliding columns, the focus, `/`, `y` and the status bar's trail like every other tab. History replaces the jobs view that sat over the current tab (`jobsOpen` and its special cases go). Maintenance's three rows are rebuilt from the snapshot and the history after every update. Home's tiles name the sidebar entry (and Maintenance row) they open.

**Tech Stack:** Go 1.26, Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2, bubbles `table`, `github.com/charmbracelet/x/ansi`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-10-17-sidebar-layout-design.md`, section "3. History and Maintenance", plus section 1's sidebar look and keys and section 2's columns, which still bind.
- Sidebar: a `TOOLS` heading (muted capitals, like the others) with `History` and `Maintenance` under it, after DISCOVER. `5` jumps to TOOLS: the entry last used there, or History.
- History list: finished jobs, newest first, with today's `:jobs` rows (`jobRows`: STATUS, COMMAND, STARTED, TOOK).
- History details: the brew command (`brew ` + args), the exit code, when it started and how long it took, then the saved output (`Record.Output`), scrollable. `enter` focuses the details. `:jobs` selects History.
- Home's Recent jobs hint becomes `5 History has them all`; `enter` on Recent jobs selects History.
- Maintenance rows, in this order: `Doctor`, `Cleanup`, `Update Homebrew`.
  - Doctor — list: `N warnings` (`1 warning`), or `all clear`; details: doctor's warnings as Home shows them; `enter`: run doctor again.
  - Cleanup — list: what the standard cleanup frees; details: both dry-run estimates, as the cleanup prompt words them; `enter`: the cleanup prompt, as `c`.
  - Update Homebrew — list: how long since brewery last ran `brew update`; details: Homebrew's version, and when brewery last ran `brew update` (from History), or that it hasn't; `enter`: `brew update`, as `U`.
- Loading, failed and not-yet-measured states read as they do on Home and in the cleanup prompt today. No new queries: Homebrew's version is the one `cmd/brewery` already reads at start.
- Home's Doctor tile selects Maintenance › Doctor and its Cleanup tile Maintenance › Cleanup, focusing the list. `c` and `U` still work from anywhere.
- Code style: match the surrounding code — short doc comments in plain English on every declaration, no new dependencies.
- Checks before every commit: `go test ./... -count=1`, `go vet ./...`, `golangci-lint run` all clean.
- Golden files: `go test ./internal/ui -run TestScreens -update -count=1` rewrites them; read the diff before committing.
- Commits authored by the repo's git config; messages in the repo's style (`feat(ui): …`, `docs: …`).

---

### Task 1: History

**Files:**
- Modify: `internal/ui/app.go`, `sidebar.go`, `columns.go`, `list.go`, `jobsview.go` (rename to `history.go` with `git mv`), `home.go`, `cmdline.go`, `complete.go`, `filter.go`, `links.go`, `actions.go`, `help.go`, `drawer.go`, `run.go`
- Test: `internal/ui/jobsview_test.go` (rename to `historytab_test.go` with `git mv`; `history_test.go` already exists and tests recording), `sidebar_test.go`, `home_test.go`, `columns_test.go`, any test using `jobsOpen` / `openJobs`

**Interfaces:**
- Produces:
  - `tabHistory` (after `tabDiscover` in the tab constants); `secTools` (after `secDiscover`), `sectionNames[secTools] = "TOOLS"`; the sidebar entry `{"History", secTools, tabHistory, 0}`.
  - `hasDetails(tabHistory)` is true.
  - `listTab.pv scroller` and `func (t *listTab) scroll(k string)`: every listTab's preview scrolls once the details have the focus, and starts at the top when the selection moves.
  - `func plainOutput(s string) string` in `history.go`.
  - `func (a *App) recordPreview(w, h int) (title, body string)`: History's details.
  - `func (a *App) showHistory() tea.Cmd`: selects History and focuses its list (used by `:jobs` and Home).
  - Home's `showJobs` constant is renamed `showHistory`.
- Goes: `App.jobsOpen`, `openJobs`, `jobsKey`, `showRecord`, `drawer.replay` (if nothing else calls it), and every `a.jobsOpen` branch.

- [ ] **Step 1: Write the failing tests**

In `historytab_test.go` (after the `git mv`), replace the jobs-view tests with these, keeping `TestJobRows` and `TestEmptyJobsView`'s intent (an empty History says `no jobs yet`):

```go
// historyApp is newJobApp with two finished jobs in its history, newest first.
func historyApp(t *testing.T) *App {
	t.Helper()
	a := newJobApp(t, &fakeStarter{})
	now := a.now()
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{
		{Title: "upgrade fd", Args: []string{"upgrade", "fd"}, State: jobs.Failed, ExitCode: 1,
			Started: now.Add(-time.Minute), Ended: now.Add(-time.Minute + 3*time.Second),
			Output: "==> Upgrading fd\r\n\x1b[31mError:\x1b[0m download failed\r\n"},
		{Title: "update", Args: []string{"update"}, State: jobs.Done,
			Started: now.Add(-2 * time.Hour), Ended: now.Add(-2*time.Hour + 4*time.Second),
			Output: "Updated 2 taps.\r\n"},
	})
	return a
}

func TestFiveSelectsHistory(t *testing.T) {
	a := historyApp(t)
	keys(t, a, "5")
	if label(a) != "History" || a.tab != tabHistory {
		t.Fatalf("5: %s, tab %d", label(a), a.tab)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "TOOLS") || !strings.Contains(out, "brew upgrade fd") {
		t.Errorf("History shows under TOOLS with its jobs:\n%s", out)
	}
}

func TestHistoryDetails(t *testing.T) {
	a := historyApp(t)
	show(t, a, "History")
	keys(t, a, "enter")
	if a.focus != colDetails {
		t.Fatalf("enter on History's list focuses the details: %d", a.focus)
	}
	out := a.render()
	plain := stripANSI(out)
	for _, want := range []string{"brew upgrade fd", "exit 1", "3s", "Error: download failed"} {
		if !strings.Contains(plain, want) {
			t.Errorf("details lack %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "\r") {
		t.Errorf("details show raw carriage returns:\n%q", plain)
	}
}

func TestColonJobsSelectsHistory(t *testing.T) {
	a := historyApp(t)
	show(t, a, "Taps")
	keys(t, a, ":", "j", "o", "b", "s", "enter")
	if label(a) != "History" || a.focus != colList {
		t.Errorf(":jobs selects History with its list focused: %s, focus %d", label(a), a.focus)
	}
}

func TestHistoryDetailsScroll(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	long := strings.Repeat("line\r\n", 200) + "the end\r\n"
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{{Title: "x", Args: []string{"x"}, State: jobs.Done,
		Started: a.now(), Ended: a.now(), Output: long}})
	show(t, a, "History")
	keys(t, a, "l")
	if strings.Contains(stripANSI(a.render()), "the end") {
		t.Fatal("the end of a long output starts out of view")
	}
	keys(t, a, "G")
	if !strings.Contains(stripANSI(a.render()), "the end") {
		t.Error("G scrolls the details to the end of the output")
	}
}

func TestPlainOutput(t *testing.T) {
	for in, want := range map[string]string{
		"a\r\nb\r\n":                 "a\nb\n",
		"\x1b[1mbold\x1b[0m\n":       "bold\n",
		"10%\r50%\r100%\r\ndone\r\n": "100%\ndone\n",
		"trailing\r\n":               "trailing\n",
	} {
		if got := plainOutput(in); got != want {
			t.Errorf("plainOutput(%q) = %q, want %q", in, got, want)
		}
	}
}
```

In `home_test.go`, add:

```go
func TestRecentJobsOpensHistory(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "l", "j", "l") // the first tile, down to Doctor, across to Recent jobs
	if !strings.Contains(stripANSI(a.render()), "5 History has them all") {
		t.Errorf("Recent jobs' hint names History:\n%s", stripANSI(a.render()))
	}
	keys(t, a, "enter")
	if label(a) != "History" || a.focus != colList {
		t.Errorf("enter on Recent jobs: %s, focus %d", label(a), a.focus)
	}
}
```

(imports as the files need: `strings`, `time`, `github.com/dillo/brewery/internal/jobs`.) If the fixture's clock (`a.now`) is fixed in `newJobApp`, use it as shown; if the record's `TOOK` renders differently from `3s`, use what `elapsed` prints.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/ui -run 'TestFiveSelectsHistory|TestHistoryDetails|TestColonJobsSelectsHistory|TestPlainOutput|TestRecentJobsOpensHistory' -count=1`
Expected: FAIL to compile (`tabHistory`, `plainOutput` undefined).

- [ ] **Step 3: History as a tab**

1. Tabs and sidebar: add `tabHistory`; `secTools` and its heading `TOOLS`; the History entry last in `entries`. `globalKey` takes `"5"` with `"1"`…`"4"`, and `sidebarKey`'s pass-through list includes `"5"`. `hasDetails` (columns.go) includes `tabHistory`. `badge`: History shows a muted count of `len(a.history)` (no chip).

2. The jobs listTab (`a.jobs`) is History's list. In `New`, set `a.jobs.preview = a.recordPreview`. `layout()` sizes it with the other tabs (through the same tab → resize list) and the `jobs` slot of the cache key goes. `render` draws `a.jobs.view(nil, a.focus)` for `tabHistory`.

3. `list.go`: add `pv scroller // where the preview is scrolled to`. `update` resets `t.pv.off = 0` when the cursor's row changes (compare `t.cursor()` before and after). Add

```go
// scroll scrolls the preview by a key.
func (t *listTab) scroll(k string) {
	_, ih := paneInner(t.pvW, t.h)
	t.pv.scroll(k, ih/2)
}
```

and in `view` window the preview's body: `title, preview := t.preview(t.pvW, t.h)` then `preview = t.pv.window(preview, ih)` with `_, ih := paneInner(t.pvW, t.h)`. Services' preview already fits its pane, so windowing it changes nothing. `tabKey`'s details case calls `scroll` for Services and History too.

4. `history.go` (the renamed `jobsview.go`): keep `jobCols`, `jobKey`, `jobRows`. Add

```go
// plainOutput is a job's saved terminal output as plain lines: escape
// sequences dropped, and of a line brew rewrote with \r (a progress bar),
// only what it said last.
func plainOutput(s string) string {
	lines := strings.Split(strings.ReplaceAll(ansi.Strip(s), "\r\n", "\n"), "\n")
	for i, l := range lines {
		l = strings.TrimRight(l, "\r")
		if j := strings.LastIndex(l, "\r"); j >= 0 {
			l = l[j+1:]
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}

// recordPreview is History's details for the selected job: its command,
// how it ended, when it ran, and what it printed.
func (a *App) recordPreview(w, h int) (title, body string) {
	i := a.jobs.cursor()
	if i < 0 {
		return "Details", ""
	}
	r, st := a.history[i], a.st
	ended := st.OK.Render("✓ done")
	switch r.State {
	case jobs.Failed:
		ended = st.Err.Render(fmt.Sprintf("✗ exit %d", r.ExitCode))
	case jobs.Canceled:
		ended = st.Warn.Render("stopped")
	}
	started := r.Started.In(a.now().Location()).Format("2006-01-02 15:04")
	body = kv(st, "command", "brew "+strings.Join(r.Args, " "), "ended", ended,
		"started", started, "took", elapsed(r.Ended.Sub(r.Started))) +
		"\n" + st.Accent.Render("Output") + "\n"
	if out := plainOutput(r.Output); strings.TrimSpace(out) != "" {
		body += out
	} else {
		body += st.Muted.Render("nothing printed")
	}
	return r.Title, body
}

// showHistory selects History with its list focused.
func (a *App) showHistory() tea.Cmd {
	cmd := a.selectEntry(entryIndex("History"))
	a.focus = colList
	return cmd
}
```

The `ended` wording must agree with `jobRows`' STATUS cell; if `jobRows` says `✗ exit 1`, so does this. Delete `openJobs`, `jobsKey`, `showRecord`.

5. Remove `jobsOpen`: the field, and every branch on it in `app.go` (`Update`'s `wantLog`/`wantDetails` guards, `handleKey`, `hasDetails`, `columnsFor`, `layout`, `render`, `statusBar`'s ` Jobs` trail), `filter.go`, `links.go` (`gKey`, `selection`: History's selection is `"brew " + args` with no URL, as the jobs view's was; `gx` on History does nothing), `actions.go` (`actionHints`), `help.go`. `selectedName` (the trail) names the selected record's title on History. Delete `drawer.replay` and its tests if nothing else calls it.

6. `cmdline.go`: `:jobs` calls `a.showHistory()`. `complete.go`: the builtin's description becomes `"Show History: finished jobs and their output"`.

7. Home: `showJobs` → `showHistory`; `homeKey` calls `a.showHistory()`. `recentJobs`' hint becomes `5 History has them all`.

8. Hints and help: `actionHints` on History's list `"l details · / filter by command · y copy the command"`, on its details `"j/k scroll · h back to the list"`. `helpGroups`: a `History` group `{{"l tab enter", "details: the command and its output"}}`, and on History the `Lists` group says `filter by command` and `copy the command` as the jobs view's did. The sidebar hint and help's `1-4` become `1-5`, and the Everywhere row names `Home, Packages, Manage, Discover, Tools`; check it fits the 56-column help box (`helpKeyW` 11 + text), and if not, shorten the text.

- [ ] **Step 4: Migrate the tests**

Tests that opened the jobs view (`openJobs`, `:jobs`, `jobsOpen`) now select History; tests of replaying into the drawer are deleted (the details show the output now). Keep every other test's intent. List each changed or deleted test and why in the report.

- [ ] **Step 5: Run everything and read the goldens**

Run: `go test ./... -count=1 && go vet ./... && golangci-lint run && grep -rn 'jobsOpen\|openJobs\|jobsKey\|showRecord' --include='*.go' internal`
Expected: clean; the grep prints nothing. The goldens change only where the sidebar gains TOOLS (rewrite them with `-update` and read the diff: every screen's sidebar gains the heading and two entries, or keeps them out of view when it is too short, and nothing else moves).

- [ ] **Step 6: Commit**

```bash
git add -A internal/ui
git commit -m "feat(ui): History in the sidebar: past jobs with their command and output"
```

---

### Task 2: Maintenance

**Files:**
- Create: `internal/ui/maintenance.go`, `internal/ui/maintenance_test.go`
- Modify: `internal/ui/app.go`, `sidebar.go`, `columns.go`, `home.go`, `actions.go` (`askCleanup`, `actionHints`), `help.go`, `cmd/brewery/main.go`
- Test: `internal/ui/home_test.go`, `cleanup_test.go`, `sidebar_test.go`

**Interfaces:**
- Consumes: `tabHistory`, `secTools`, `a.history`, `listTab.scroll` from Task 1.
- Produces:
  - `tabMaintenance` (after `tabHistory`); the sidebar entry `{"Maintenance", secTools, tabMaintenance, 0}`, after History; `hasDetails(tabMaintenance)` is true.
  - `App.maint listTab`, columns `maintCols = []col{{"TASK", 18, 0}, {"STATUS", 0, 12}}`, keyed by `firstCell`.
  - Row indexes `maintDoctor = 0`, `maintCleanup = 1`, `maintUpdate = 2`.
  - `func (a *App) syncMaint()`: rebuilds the three rows; `Update` calls it before `layout()`.
  - `func (a *App) maintPreview(w, h int) (title, body string)` and `func (a *App) maintRun() tea.Cmd` (the selected row's `enter`).
  - `func doctorBody(st theme.Styles, snap state.Snapshot, err error) string`: what Home's Doctor pane shows, moved out of `homeTab.view` so both use it.
  - `func cleanupLines(cl brew.Cleanup) []string`: the estimate lines `askCleanup` puts in its box, moved out so both use them.
  - `func (a *App) lastUpdate() (jobs.Record, bool)`: the newest History record with `Args` exactly `["update"]` and `State == jobs.Done`.
  - `func (a *App) UseBrewVersion(v string)`: Homebrew's version text (`"4.6.3"`), set from `cmd/brewery`; "" when unknown.
  - `tile` gains `entry string` (the sidebar label it opens, "" for none) and `row int` (a Maintenance row, or -1); `tab` and `tileEntries` and `openCleanup` go. `homeTab.update` returns the tile's index when `enter` opens it.

- [ ] **Step 1: Write the failing tests**

`internal/ui/maintenance_test.go`:

```go
package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dillo/brewery/internal/jobs"
)

func TestMaintenanceRows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseBrewVersion("4.6.3")
	show(t, a, "Maintenance")
	out := stripANSI(a.render())
	for _, want := range []string{"TOOLS", "Doctor", "Cleanup", "Update Homebrew", "not yet"} {
		if !strings.Contains(out, want) {
			t.Errorf("Maintenance lacks %q:\n%s", want, out)
		}
	}
	if n := len(a.snap.Doctor); n > 0 && !strings.Contains(out, plural(n, "warning")) {
		t.Errorf("the Doctor row counts %d warnings:\n%s", n, out)
	}
	if s := a.snap.Cleanup.Standard.Size; s != "" && !strings.Contains(out, s) {
		t.Errorf("the Cleanup row says what the standard cleanup frees (%s):\n%s", s, out)
	}
}

func TestMaintenanceDetails(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseBrewVersion("4.6.3")
	show(t, a, "Maintenance")
	if w := a.snap.Doctor; len(w) > 0 && !strings.Contains(stripANSI(a.render()), w[0].Title) {
		t.Errorf("Doctor's details show the warnings:\n%s", stripANSI(a.render()))
	}
	keys(t, a, "j")
	if out := stripANSI(a.render()); !strings.Contains(out, "The standard cleanup frees") {
		t.Errorf("Cleanup's details word the estimates as the prompt does:\n%s", out)
	}
	keys(t, a, "j")
	out := stripANSI(a.render())
	if !strings.Contains(out, "4.6.3") || !strings.Contains(out, "hasn't run brew update") {
		t.Errorf("Update's details: the version, and no update yet:\n%s", out)
	}
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{{Title: "update", Args: []string{"update"},
		State: jobs.Done, Started: a.now().Add(-3 * time.Hour), Ended: a.now().Add(-3 * time.Hour)}})
	a.Update(nil)
	if out := stripANSI(a.render()); !strings.Contains(out, "3h ago") {
		t.Errorf("after a brew update, the row and details say when:\n%s", out)
	}
}

func TestMaintenanceStates(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.loaded[qDoctor], a.loaded[qCleanup] = false, false
	show(t, a, "Maintenance")
	if out := stripANSI(a.render()); strings.Count(out, "…") < 2 {
		t.Errorf("doctor and cleanup still running read as loading:\n%s", out)
	}
	a.loaded[qDoctor], a.loaded[qCleanup] = true, true
	a.errs[qDoctor], a.errs[qCleanup] = errors.New("boom"), errors.New("boom")
	a.home.doctorErr = a.errs[qDoctor]
	a.Update(nil)
	if out := stripANSI(a.render()); strings.Count(out, "✗ failed") < 2 {
		t.Errorf("failed doctor and cleanup read as failed:\n%s", out)
	}
}

func TestMaintenanceEnter(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Maintenance")
	keys(t, a, "j", "enter") // Cleanup
	if a.confirm == nil || a.confirm.title != "Clean up Homebrew?" {
		t.Fatalf("enter on Cleanup asks to clean up: %+v", a.confirm)
	}
	keys(t, a, "n", "j", "enter") // Update Homebrew
	if c := lastCall(fs); c == nil || strings.Join(c.args, " ") != "update" {
		t.Errorf("enter on Update Homebrew runs brew update: %+v", c)
	}
}

func TestHomeTilesSelectMaintenanceRows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "l", "l", "l", "l", "enter") // the Doctor tile
	if label(a) != "Maintenance" || a.focus != colList || a.maint.cursor() != maintDoctor {
		t.Errorf("Doctor tile: %s, focus %d, row %d", label(a), a.focus, a.maint.cursor())
	}
	keys(t, a, "1", "l", "l", "l", "l", "l", "enter") // the Cleanup tile
	if label(a) != "Maintenance" || a.maint.cursor() != maintCleanup || a.confirm != nil {
		t.Errorf("Cleanup tile: %s, row %d, confirm %v", label(a), a.maint.cursor(), a.confirm)
	}
}
```

Adjust to the real helpers where they differ: `lastCall(fs)`'s return type (actions_test.go), how `fakeStarter` records args, how a doctor run is requested (for `enter` on Doctor, assert the source was asked for doctor again — follow how existing tests observe `request(qDoctor)`, and add that assertion to `TestMaintenanceEnter`). `1` from Maintenance's list goes to Home with the list focused, so `l` there moves along the tiles from wherever Home's focus was; if that makes the key sequence wrong, use `h` back to the sidebar and `l` in again to start on the first tile.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/ui -run 'TestMaintenance|TestHomeTilesSelectMaintenanceRows' -count=1`
Expected: FAIL to compile (`UseBrewVersion`, `maintDoctor` undefined).

- [ ] **Step 3: Share Home's doctor text and the prompt's estimates**

Move the `switch` that builds Home's Doctor pane body out of `homeTab.view` into `doctorBody(st, snap, err)` (with `writeWarnings` becoming a function of `st` rather than a method, or staying a method called from it — whichever reads cleaner); `homeTab.view` calls it. Move the estimate lines out of `askCleanup` into `cleanupLines(cl)`, returning exactly the lines `askCleanup` appends to `box.body` today (`"The standard cleanup frees …:"`, `"  old versions, and downloads over 120 days old"`, or `"The standard cleanup has nothing to remove."`, then the whole-cache pair when `cl.All.Items > cl.Standard.Items`); `askCleanup` uses it. Its existing tests in `cleanup_test.go` must still pass unchanged.

- [ ] **Step 4: The Maintenance tab**

`maintenance.go`:

- `maintCols`, the row constants, and `syncMaint` building rows `{"Doctor", <status>}`, `{"Cleanup", <status>}`, `{"Update Homebrew", <status>}` with `a.maint.setRows`. Statuses, each coloured as Home's matching tile colours its value:
  - Doctor: `…` while `!a.loaded[qDoctor]` and no error; `✗ failed` when `a.errs[qDoctor] != nil`; `all clear` when `snap.DoctorRan` with no warnings; else `plural(n, "warning")`.
  - Cleanup: `✗ failed` (error), `…` (not loaded), `nothing to clean` when both sizes are empty, else `frees ` + the standard size (or the whole cache's size, labelled `frees <size> (whole cache)`, when the standard frees nothing).
  - Update Homebrew: `ago(a.now(), r.Ended)` of `lastUpdate()`, or `not yet`.
- `maintPreview`: titles `Doctor`, `Cleanup`, `Update Homebrew`. Doctor's body is `doctorBody(...)` then a muted `enter runs brew doctor again`. Cleanup's is `cleanupLines(...)` joined, or the same loading/failed wording as the cleanup prompt's notes (`still measuring what cleanup would free`, `couldn't measure the cleanup · r retries`, `nothing to clean up`), then `enter cleans up` when there is something to clean. Update's is `kv(st, "homebrew", <version or "unknown">, "last update", <"3h ago · 2026-10-17 11:04" or "brewery hasn't run brew update">)` then `enter runs brew update`.
- `maintRun`: Doctor → `a.request(qDoctor)` with a note `checking with brew doctor…`; Cleanup → `a.askCleanup()`; Update → the same action `U` runs (move `U`'s `action{…}` into a helper `brewUpdate()` that both use).

In the app:

- `tabMaintenance`, the sidebar entry (no badge), `hasDetails`, `layout()` and `render` as for the other list tabs; `a.maint.preview = a.maintPreview` in `New`.
- `Update` calls `a.syncMaint()` before `a.layout()`.
- `tabKey`: on Maintenance, `enter` runs `a.maintRun()` from the list or the details (it is the exception to `enter` focusing the details; `l` and `tab` still do).
- `selectedName` names the selected row on Maintenance; `selection` (for `y`) gives nothing there.
- `UseBrewVersion` stores the text; `cmd/brewery/main.go` calls `app.UseBrewVersion(v.Text)` with the version it already read (it is in scope where `ui.New` is called; if it isn't, pass it down rather than reading it twice).
- Hints: Maintenance list `"enter run · l details · c clean up · U update"`, details `"enter run · h back to the list"`. Help: a `Maintenance` group `{{"enter", "run doctor, clean up, or brew update"}, {"l tab", "details"}}`.

- [ ] **Step 5: Home's tiles open entries**

`tile` loses `tab` and gains `entry` and `row`: Installed → `"All"`, Outdated → `"Outdated"`, Services → `"Services"`, Doctor → `"Maintenance"` row `maintDoctor`, Cleanup → `"Maintenance"` row `maintCleanup`; `row` is -1 for the others. `tileKey`'s `enter` returns `h.sel` (the tile's index) when the tile has an entry, else `homeStay`. Delete `openCleanup` and `tileEntries`. `homeKey`:

```go
	switch t := a.home.update(msg, a.snap); {
	case t >= 0:
		tile := a.home.tiles(a.snap)[t]
		cmd := a.selectEntry(entryIndex(tile.entry))
		a.focus = colList
		if tile.row >= 0 {
			a.maint.table.SetCursor(tile.row)
		}
		return cmd, true
	case t == leaveHome:
		...
```

Update the comments on `homeStay` and the constants. Tests that pressed `enter` on the Cleanup tile and expected the prompt (`TestCleanupFromTheHomeTile`) now expect Maintenance › Cleanup, and a second `enter` there opens the prompt.

- [ ] **Step 6: Run everything and read the goldens**

Run: `go test ./... -count=1 && go vet ./... && golangci-lint run && grep -rn 'openCleanup\|tileEntries' --include='*.go' internal`
Expected: clean; the grep prints nothing. Rewrite the goldens with `-update` and read the diff: only the sidebar's Maintenance entry is new.

- [ ] **Step 7: Commit**

```bash
git add -A internal/ui cmd/brewery
git commit -m "feat(ui): Maintenance in the sidebar: doctor, cleanup and brew update"
```

---

### Task 3: Goldens, README and screenshots

**Files:**
- Modify: `internal/ui/golden_test.go`, `internal/ui/screenshots_test.go` (only if a screenshot should show the new tabs; see Step 2), `README.md`, `docs/screenshots/*.png` (regenerated)

- [ ] **Step 1: Goldens for the new tabs**

In `TestScreens`, give the app two history records before the loop (as `historyApp` in `historytab_test.go` does, with fixed times relative to `a.now()`), call `a.UseBrewVersion("4.6.3")`, and add screens `{"history", []string{"History"}, []string{"l"}}` (the details of the newest job) and `{"maintenance", []string{"Maintenance"}, nil}`. Rewrite with `-update` and read both: History shows the list and the record's command, ending, times and output; Maintenance shows the three rows and doctor's warnings beside them.

- [ ] **Step 2: README**

- The Sidebar section's list gains TOOLS: History (past jobs, their command and output) and Maintenance (doctor, cleanup, `brew update`).
- Keys: `1`–`5`; `:jobs` now selects History; `enter` on Maintenance runs the row.
- Anything that still says the jobs view opens over the current tab, or that `enter` replays a job in the drawer, goes.
- Keep the README's voice: short plain sentences.

- [ ] **Step 3: Screenshots**

Run: `make screenshots FREEZE="go run github.com/charmbracelet/freeze@v0.2.2"` and look at each regenerated PNG with the Read tool; the sidebar shows TOOLS. No new screenshot is needed.

- [ ] **Step 4: Run everything**

Run: `go test ./... -count=1 && go vet ./... && golangci-lint run`
Expected: all clean.

- [ ] **Step 5: Commit**

```bash
git add -A internal/ui README.md docs/screenshots
git commit -m "docs: History and Maintenance in the goldens, README and screenshots"
```
