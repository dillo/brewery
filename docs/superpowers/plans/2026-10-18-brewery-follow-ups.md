# Layout Follow-ups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the sidebar layout's loose ends: an empty list keeps the focus, Services' details scroll the log, and the test gaps the reviews found are covered, with every package at 90% statement coverage or more.

**Architecture:** One rule decides whether a tab has details: the tab kind (`hasDetails`), Discover's catalogue, and now whether its list shows any rows; `Update` drops a details focus that has nothing to show. Services' log keeps its own scroll position, counted back from the newest line. `cmd/brewery`'s `run` gets a swappable program runner so its wiring can be tested.

**Tech Stack:** Go 1.26, Bubble Tea v2, Lip Gloss v2, bubbles `table`.

## Global Constraints

- Decisions (from the user): an empty list keeps the focus on the list; in Services' details, `j`/`k` scroll the log when there is more of it than fits, and do nothing otherwise; cover the test gaps the reviews found; aim for 90% statement coverage or more.
- Coverage target: each package at ≥ 90% (`go test ./... -cover`), and the total ≥ 90% (`go tool cover -func`). `main()` itself (one line) may stay uncovered.
- Tests verify behaviour through the app's real key and render paths where they can (`keys`, `show`, `render`), not by poking fields, and never assert something that can't fail.
- No behaviour changes beyond the two decisions above. Tests added for coverage must not need production changes, except the runner seam in Task 4.
- Code style: match the surrounding code — short doc comments in plain English on every declaration, no new dependencies.
- Checks before every commit: `go test ./... -count=1`, `go vet ./...`, `golangci-lint run` all clean.
- Commits authored by the repo's git config; messages in the repo's style (`fix(ui): …`, `feat(ui): …`, `test: …`).

---

### Task 1: An empty list keeps the focus

**Files:**
- Modify: `internal/ui/app.go` (`tabHasDetails`, `Update`), and where needed `installed.go`, `discover.go`, `list.go` for a rows count
- Test: `internal/ui/columns_test.go`

**Interfaces:**
- Produces: `(a *App) tabHasDetails(tab int) bool` is also false when the tab's list shows no rows (after the `/` filter); `Update` sets `a.focus = colList` when `a.focus == colDetails && !a.hasDetails()`, before `syncMaint`/`layout`.

- [ ] **Step 1: Write the failing tests**

```go
func TestEmptyListKeepsTheFocus(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseHistory(jobs.History{Dir: t.TempDir()}, nil)
	show(t, a, "History")
	keys(t, a, "enter")
	if a.focus != colList {
		t.Errorf("enter on an empty History: focus %d, want the list", a.focus)
	}
	keys(t, a, "l", "tab")
	if a.focus != colList {
		t.Errorf("l and tab on an empty list: focus %d, want the list", a.focus)
	}
	if hint := stripANSI(a.statusBar()); strings.Contains(hint, "j/k scroll") {
		t.Errorf("an empty list's hints don't offer scrolling: %s", hint)
	}
}

func TestFilteringToNothingLeavesTheDetails(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "All")
	keys(t, a, "l")
	if a.focus != colDetails {
		t.Fatalf("l: focus %d", a.focus)
	}
	keys(t, a, "/", "z", "z", "z", "q", "enter") // matches nothing
	if a.focus != colList {
		t.Errorf("a filter that empties the list moves the focus to it: %d", a.focus)
	}
}

func TestSwitchingToAnEmptyTabKeepsTheList(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseHistory(jobs.History{Dir: t.TempDir()}, nil)
	show(t, a, "Search all")
	keys(t, a, "l", "5") // History, which is empty
	if label(a) != "History" || a.focus != colList {
		t.Errorf("5 from Discover's details onto an empty History: %s, focus %d", label(a), a.focus)
	}
}
```

Adapt to the real helpers (`/` then text then `enter` applies the filter in this app; check `filter_test.go`); `Search all` needs its catalogue loaded in `newJobApp` — if it isn't, use `All` and an entry whose list is empty in the fixtures (e.g. `Pinned` if no fixture package is pinned).

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/ui -run 'TestEmptyListKeepsTheFocus|TestFilteringToNothing|TestSwitchingToAnEmptyTab' -count=1` — expected FAIL (the focus moves to the details).

- [ ] **Step 3: Implement**

Give each tab a way to say how many rows its list shows (Installed and Discover: `len(t.rows)`; listTab: `t.n`), and make `tabHasDetails` false at zero. In `Update`, before `syncMaint` and `layout`, drop a details focus with nothing to show. Remove any now-redundant special case (the Discover-before-load check can stay as one of the conditions). Existing tests (including `TestEmptyJobsView`) change only where they asserted the old behaviour; say which in the report.

- [ ] **Step 4: Check and commit**

Run the checks, then `git commit -am "fix(ui): an empty list keeps the focus"` (add new files if any).

---

### Task 2: Services' details scroll the log

**Files:**
- Modify: `internal/ui/services.go` (`logTail`, `servicePreview`), `internal/ui/app.go` (`tabKey`'s details case), `internal/ui/actions.go` (`actionHints`)
- Test: `internal/ui/services_test.go`

**Interfaces:**
- Produces: `logTail.back int` — how many lines the view is scrolled back from the newest; `(a *App) scrollLog(k string)`; `servicePreview` shows the `room` lines ending `back` lines before the newest, clamping `back` to what the log has.

- [ ] **Step 1: Write the failing tests**

```go
func TestServicesDetailsScrollTheLog(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	var log strings.Builder
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&log, "log line %d\n", i)
	}
	a.readLog = (&fakeLogs{text: map[string]string{<the selected fixture service's log path>: log.String()}}).read
	show(t, a, "Services")
	keys(t, a, "l")
	out := stripANSI(a.render())
	if !strings.Contains(out, "log line 200") {
		t.Fatalf("the details start at the newest line:\n%s", out)
	}
	keys(t, a, "k")
	if out := stripANSI(a.render()); strings.Contains(out, "log line 200") || !strings.Contains(out, "log line 199") {
		t.Errorf("k scrolls back one line:\n%s", out)
	}
	keys(t, a, "j")
	if !strings.Contains(stripANSI(a.render()), "log line 200") {
		t.Error("j scrolls forward again")
	}
	keys(t, a, "g", "g")
	if !strings.Contains(stripANSI(a.render()), "log line 1\n") { // allow for the pane's border after it
		t.Error("gg goes to the oldest line read")
	}
	keys(t, a, "G")
	if !strings.Contains(stripANSI(a.render()), "log line 200") {
		t.Error("G goes back to the newest line")
	}
}

func TestShortLogDoesNotScroll(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{text: map[string]string{<path>: "only line\n"}}).read
	show(t, a, "Services")
	keys(t, a, "l")
	before := stripANSI(a.render())
	keys(t, a, "k", "k", "ctrl+u")
	if after := stripANSI(a.render()); after != before {
		t.Errorf("a log that fits doesn't move:\n%s", after)
	}
}

func TestLogScrollStartsAtTheNewestForEachService(t *testing.T) {
	// scroll back on one service, j to the next row in the list, and its log starts at its newest line
}
```

Fill in the path from the services fixture (`fixtureSnapshot`'s first service with a log, as `TestScreens`' `fakeLogs` uses), make the line-1 assertion robust to the pane's right border (check the line `"log line 1"` exists as a whole word, e.g. with a regexp `\blog line 1\b`), and write the third test out in full.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/ui -run 'TestServicesDetailsScroll|TestShortLog|TestLogScrollStarts' -count=1` — expected FAIL.

- [ ] **Step 3: Implement**

- `logTail.back`, reset to 0 whenever the log shown changes (another service, or a fresh read of the same path after a refresh keeps it — say which you chose and why).
- `scrollLog`: `k`/`up` back+1; `j`/`down` back-1; `ctrl+u`/`ctrl+d` half a pane; `home` (what `gg` sends in the details) to the oldest; `G`/`end` to the newest; never below 0. `tabKey`'s details case calls it on Services instead of `a.services.scroll`.
- `servicePreview` clamps `back` to `len(lines)-room` and shows `lines[len-room-back : len-back]`. The heading reads `End of the log` at 0 and `The log · N lines back` otherwise.
- Services' details hint: `"j/k scroll the log · h back to the list · s start · S stop · R restart · L log"`; update `help.go`'s Services group with `{"j k", "in the details, scroll the log"}` if it fits the 56-column box, and the tests that pin either.

- [ ] **Step 4: Check and commit**

Run the checks; `git commit -am "feat(ui): Services' details scroll the log"`.

---

### Task 3: The UI's test gaps

**Files:**
- Test only: `internal/ui/columns_test.go`, `installed_test.go`, `historytab_test.go`, `maintenance_test.go`, `links_test.go`, `marks_test.go` (whichever fits each case)

Add a test (or a row in a table test) for each of these, gaps the reviews recorded. Each must fail if the behaviour it names broke:

- [ ] `TestColumnsFor`: rows for `hasDetails=false` at `81, colDetails` → `{0, 81, 0}` and at `110, colList` → `{22, 88, 0}`.
- [ ] Wide Services details: at 140×40, `l` on Services focuses the details (the details pane has the focused border colour — compare with how `sidebar_test.go` checks a focused border, or assert via `a.focus` plus the status bar hint), then `h` and `esc` each bring the focus back to the list.
- [ ] `space` in the details: on Installed with the details focused, `space` marks the selected package and moves the list's cursor to the next row.
- [ ] `TestPreviewFocusIsLeftBehind` (installed_test.go): assert `a.focus == colList` while on Outdated, inside the loop.
- [ ] `y` from the details alone: at 90 columns, on Installed's details alone, `y` copies the selected package's name.
- [ ] History: a record with no output shows `nothing printed`; a Canceled record shows `stopped` in both the list and the details; the details' `started` field shows the date and time in the app's clock zone (`2006-01-02 15:04`).
- [ ] Maintenance: the Doctor row reads `all clear` with no warnings; the Cleanup row reads `nothing to clean` when both estimates are empty, and `frees <size> (whole cache)` when only the whole cache frees anything; Update's details show `unknown` with no version, and `<ago> · <date>` after an update; tighten `TestMaintenanceStates` so it checks each row's own status cell (find the row's line in the render) rather than counting `…` across the screen.

Then run `go test ./internal/ui -count=1 -cover`: the package stays ≥ 95%. Commit: `git commit -am "test(ui): cover the gaps the layout reviews found"` (add new files if any).

---

### Task 4: Every package at 90% or more

**Files:**
- Modify: `cmd/brewery/main.go` (a swappable runner), tests in `cmd/brewery`, `internal/analytics`, `internal/state`
- Modify: `Makefile` (a `cover` target)

**Interfaces:**
- Produces: in `cmd/brewery`, `var runProgram = func(m tea.Model) error { _, err := tea.NewProgram(m).Run(); return err }`, used by `run` in place of the direct call, so tests can run `run` end to end without a terminal.

- [ ] **Step 1: See what's uncovered**

Run: `go test ./... -count=1 -coverprofile=cover.out && go tool cover -func=cover.out | grep -v 100.0%` (write `cover.out` to a temp dir, not the repo). Today: `cmd/brewery` 55.6% (`run` 30%, `readBrewVersion` 0%), `internal/analytics` 88.1% (`store.go` `save`, `SetTried`, `Cached`, `Fetch`; `Compare`), `internal/state` 88.4% (`Index.Get` 0%, `Save`, `Load`).

- [ ] **Step 2: cmd/brewery**

- Add `runProgram` and use it in `run`. Tests set it (restoring it with `t.Cleanup`) along with `locateBrew` and `brewVersion`, and point `HOME`, `XDG_CACHE_HOME` and `XDG_STATE_HOME` at `t.TempDir()` with `t.Setenv`, so nothing touches the real machine.
- Cover: a normal run (the fake runner receives a `*ui.App`, returns nil, `run` returns 0); the runner returning an error; a crash during the run (the fake runner writes to the crash log's stderr and panics or returns as the crash test helpers expect — follow `crash_test.go`) so the "brewery crashed" message prints and `run` returns 1; crash logs left from an earlier run ("crashed last time"); `readBrewVersion` against a fake brew: a shell script in `t.TempDir()` that prints `Homebrew 4.6.3` (and one that exits non-zero).
- Read the rest of `run` (after the program returns) and cover each branch it has.

- [ ] **Step 3: analytics and state**

Cover the uncovered branches listed in Step 1 through each package's public API: saving into a directory that can't be written (e.g. a path under a regular file), `SetTried` and `Cached`/`Fetch` around a failed save, `Compare`'s missing branch, `Index.Get` for a known and an unknown name, `Snapshot` `Save`/`Load` errors (unwritable dir, corrupt JSON, wrong version — whatever branches exist).

- [ ] **Step 4: Makefile**

Add a `cover` target that prints each package's coverage and the total:

```make
cover:
	go test -count=1 -coverprofile=cover.out ./... && go tool cover -func=cover.out | tail -1 && rm -f cover.out
```

and add `cover` to `.PHONY`; add `cover.out` to `.gitignore` if there is one.

- [ ] **Step 5: Check and commit**

Run: `make cover` — each package ≥ 90% (except as allowed above) and the total ≥ 90%; then the checks. Commit: `git commit -am "test: every package at 90% coverage or more; make cover"` (add new files).
