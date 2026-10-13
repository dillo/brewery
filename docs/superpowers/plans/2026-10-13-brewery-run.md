# brewery Plan 2: Run — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let brewery change things. With this plan you can upgrade, uninstall (with orphan clean-up), pin, upgrade all, `brew update`, and start, stop and restart services from the panes, and watch brew's output live in a drawer. Jobs run in a real pseudo-terminal, so sudo password prompts and yes/no questions can be answered right in the drawer.

**Architecture:**
- **`internal/brew/pty.go`** runs brew in a pseudo-terminal (`creack/pty`).
- **`internal/jobs`** is a pure queue: one job at a time, with retries when another brew holds the lock.
- **`internal/ui`** reads job output through Bubble Tea commands and renders it with the `x/vt` terminal emulator in a drawer under the tab. Destructive actions ask first in a dialog floated over the tab.
- **The UI still never shells out.** It gets a `Starter` function from `main`.

**Tech Stack:**
- Go ≥ 1.25
- Bubble Tea v2, Bubbles v2, Lip Gloss v2
- `github.com/creack/pty` v1.1.24
- `github.com/charmbracelet/x/vt` v0.1.0

**Spec:** `docs/superpowers/specs/2026-10-12-brewery-design.md`. This plan covers section 3 (Running commands), except the `:` command line, takeover and `:jobs`, plus the action keys from section 4.

**Roadmap change (decided with the user):** the old Plan 2 outline is split in two.
- **This plan, "Run":** the groundwork, the job engine, the drawer, and the action keys.
- **Plan 3, "Command line":** `:` with completion, full-screen takeover (`edit`, `sh`, `irb`, `:!`), job history persisted to `~/.local/state/brewery/jobs/` with a `:jobs` view, a Home recent-jobs pane, the `l` service log, cleanup with a `--dry-run` preview, and untap.
- **Plan 4, "Polish and ship":** the old Plan 3.

**Verified:** every code block in this plan was compiled and tested before the plan was written.
- **The edits are exact.** A checker applied this plan's edits to `master` (7ac67f4) and confirmed each task reproduces the tested code byte for byte, that each task's tests fail before its code and pass after, and that `go mod tidy` stays clean.
- **Concurrency:** the job and pseudo-terminal tests were stress-run 15–30 times under `-race`.
- **Against real brew:** a live job ran `brew --version` through the real runner. The real binary was also driven in a pseudo-terminal (opening and cancelling the uninstall, upgrade-all and stop-service dialogs, then quitting) without changing anything.

## Global Constraints

These come from Plan 1 and still hold:
- Module path `brewery`; Go ≥ 1.25.
- Only `internal/brew` runs brew. `internal/ui` gets data through `Source` and starts jobs through `Starter`.
- Views use `theme.Styles` only.
- Table keys leave the bare letters u, d, b and f unbound by the table. In this plan, `u` upgrades and `f` filters Installed.

Added by this plan:
- **One job at a time:** mutating jobs run one at a time, in the order requested.
- **Lock retries:** a run that fails with brew's lock message (``A `brew …` process has already locked …``, matched on `has already locked`) waits and retries: 5 attempts, with backoff 2s, 4s, 8s, 16s, then 30s.
- **Pane actions don't auto-update:** jobs started from a pane get `HOMEBREW_NO_AUTO_UPDATE=1` and `HOMEBREW_NO_ENV_HINTS=1` (`brew.JobEnv`).
- **Key routing is by mode first:**
  1. An open confirmation gets every key. `esc` always cancels, and keys that aren't choices are ignored.
  2. In INSERT mode every key goes to brew, except `esc`, which leaves INSERT.
  3. Otherwise the global keys and actions apply.
- **Mode badges:**
  - **INSERT** (green): keys are going to brew.
  - **RUNNING** (peach): a job is current.
  - **NORMAL**: anything else.
- **Prompts switch to INSERT on their own.** When the text before the cursor looks like a password prompt or a `[y/N]`/`(y/n)` question, the drawer opens in INSERT. Typed input is never logged, and only output is kept.
- **Drawer size:** about two fifths of the body, between 6 and 14 lines, leaving the tab at least 3. The pseudo-terminal's size equals the drawer's inner area and follows window resizes.
- **The emulator is never closed.** In x/vt v0.1.0, `Close` races with the reply reader, so one emulator per drawer is reused and reset between runs. Its replies are always drained.
- **Confirmations come first** for uninstall, upgrade all, stopping a service, stopping a job, and quitting while a job runs.
- **Refresh after jobs:** after any job, even a failed one, re-run what it touched:
  - upgrade and pin: installed + outdated
  - uninstall: installed + outdated + services
  - services: services
  - `brew update`: everything, then doctor
- **Refreshes are never dropped.** A query requested while it is already running runs once more when it returns. The cache is saved once the JSON queries are back, *before* doctor runs, and `FetchedAt` only advances after a success.
- **Quitting cancels in-flight queries** (root context) and kills a running job.
- **Selection stays on the same package across refreshes.** Rows are matched by name and type, and same-name rows sort formula before cask.

## File Structure

```
internal/brew/pty.go        StartPTY, Proc (Read/Write/Resize/Interrupt/Kill/Wait), JobEnv
internal/jobs/jobs.go       Queue, Job, State, Backoff, IsLockError, MaxAttempts
internal/state/index.go     + Orphans, UsedBy returns a copy
internal/ui/source.go       fetch takes the root context; jsonQueries
internal/ui/app.go          per-query refresh bookkeeping, modes, layout with drawer, routing
internal/ui/components.go   + setRowsKeeping (selection by identity)
internal/ui/list.go         + row keys, cursor()
internal/ui/installed.go    stable sort, selection kept on refresh and filter change
internal/ui/drawer.go       drawer: x/vt emulator, prompt detection, titles and hints
internal/ui/confirm.go      confirmBox, choice, overlay (Lip Gloss canvas)
internal/ui/run.go          Proc and Starter interfaces, job messages, run/launch/finish/stop, keyBytes
internal/ui/actions.go      action type, action keys u x p A U s S R, confirmations, hints
cmd/brewery/main.go         passes a Starter built on brew.StartPTY
```

---

### Task 1: Refresh bookkeeping and quitting cleanly

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`
- Modify: `internal/ui/source.go`

**Interfaces:**
- **Consumes:** Plan 1's `App`, `Source`, `resultMsg`, `save`.
- **Produces:**
  - `fetch(ctx context.Context, src Source, q query) tea.Cmd`, which gains a context.
  - `jsonQueries`.
  - The `App` fields `ctx`, `cancel`, `inflight`, `again`, `doctorAfter` and `dirty`. These replace `pending` and `doctorQueued`.
  - The methods `(*App) request(qs ...query) tea.Cmd`, `refresh()`, `busy()`, `jsonBusy()`, `quit()` and `handleResult(resultMsg) tea.Cmd`.
- **Test helpers:**
  - `drain` now runs a `tea.Sequence` in order.
  - `countingSource` wraps `fakeSource`, counting calls and offering hooks.

This fixes the final-review follow-ups:
- a refresh asked for during a refresh was dropped
- queries kept running after quit
- the cache was saved only after doctor
- `FetchedAt` advanced even when every query failed

`tea.Sequence(save, doctor)` guarantees the save happens first.

- [ ] **Step 1: Edit `internal/ui/app_test.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
```

   with:

```go
import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
```

2. Replace:

```go
}

// drain runs cmd and every command it produces, feeding the messages back
// into the app the way the Bubble Tea runtime would.
func drain(t *testing.T, a *App, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case nil:
		default:
			_, next := a.Update(msg)
			queue = append(queue, next)
		}
	}
}

var testNow = time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC)
```

   with:

```go
}

// drain runs cmd and every command it produces, feeding the messages back
// into the app the way the Bubble Tea runtime would. A tea.Sequence (an
// unexported slice of commands) runs in order, each to completion.
func drain(t *testing.T, a *App, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		for i := range v.Len() {
			drain(t, a, v.Index(i).Interface().(tea.Cmd))
		}
		return
	}
	if msg == nil {
		return
	}
	_, next := a.Update(msg)
	drain(t, a, next)
}

var testNow = time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC)
```

3. Replace:

```go
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())

	if a.pending != 0 {
		t.Fatalf("pending = %d after refresh", a.pending)
	}
	if len(a.snap.Packages) != 7 || len(a.snap.Outdated) != 2 || len(a.snap.Services) != 2 || len(a.snap.Taps) != 2 {
		t.Errorf("snapshot incomplete: %d pkgs, %d outdated, %d services, %d taps",
```

   with:

```go
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())

	if a.busy() {
		t.Fatalf("queries still in flight after refresh: %v", a.inflight)
	}
	if len(a.snap.Packages) != 7 || len(a.snap.Outdated) != 2 || len(a.snap.Services) != 2 || len(a.snap.Taps) != 2 {
		t.Errorf("snapshot incomplete: %d pkgs, %d outdated, %d services, %d taps",
```

4. Replace:

```go
	}
}

func TestRefreshIgnoredWhileRunning(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	if a.Init() == nil {
		t.Fatal("first refresh should start queries")
	}
	if _, cmd := a.Update(press("r")); cmd != nil {
		t.Error("r during a refresh should do nothing")
	}
}
```

   with:

```go
	}
}

// countingSource wraps fakeSource, counting calls per query. onDoctor runs
// at the start of each doctor call; block makes Installed wait for ctx.
type countingSource struct {
	*fakeSource
	calls    map[string]int
	onDoctor func()
	block    bool
}

func newCountingSource(t *testing.T) *countingSource {
	return &countingSource{fakeSource: newFakeSource(t), calls: map[string]int{}}
}

func (c *countingSource) Installed(ctx context.Context) ([]brew.Package, error) {
	c.calls["installed"]++
	if c.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return c.fakeSource.Installed(ctx)
}
func (c *countingSource) Outdated(ctx context.Context) ([]brew.Outdated, error) {
	c.calls["outdated"]++
	return c.fakeSource.Outdated(ctx)
}
func (c *countingSource) Services(ctx context.Context) ([]brew.Service, error) {
	c.calls["services"]++
	return c.fakeSource.Services(ctx)
}
func (c *countingSource) Taps(ctx context.Context) ([]brew.Tap, error) {
	c.calls["taps"]++
	return c.fakeSource.Taps(ctx)
}
func (c *countingSource) Doctor(ctx context.Context) ([]brew.DoctorWarning, error) {
	c.calls["doctor"]++
	if c.onDoctor != nil {
		c.onDoctor()
	}
	return c.fakeSource.Doctor(ctx)
}

func TestRefreshDuringRefreshRunsAgain(t *testing.T) {
	src := newCountingSource(t)
	a := newTestApp(t, src)
	first := a.Init()
	a.Update(press("r")) // every query is still in flight
	drain(t, a, first)
	for _, q := range []string{"installed", "outdated", "services", "taps"} {
		if src.calls[q] != 2 {
			t.Errorf("%s ran %d times, want 2 (the second r must not be dropped)", q, src.calls[q])
		}
	}
	if a.busy() {
		t.Errorf("still busy: %v", a.inflight)
	}
}

func TestTargetedRequestSkipsDoctorAndSaves(t *testing.T) {
	src := newCountingSource(t)
	a := newTestApp(t, src)
	drain(t, a, a.Init())
	src.calls = map[string]int{}
	os.Remove(a.cachePath)

	drain(t, a, a.request(qServices))
	if src.calls["services"] != 1 || src.calls["doctor"] != 0 || src.calls["installed"] != 0 {
		t.Errorf("a targeted request ran %v", src.calls)
	}
	if _, err := state.Load(a.cachePath); err != nil {
		t.Errorf("a successful targeted request should save the cache: %v", err)
	}
}

func TestCacheSavedBeforeDoctorRuns(t *testing.T) {
	src := newCountingSource(t)
	a := newTestApp(t, src)
	savedFirst := false
	src.onDoctor = func() {
		_, err := state.Load(a.cachePath)
		savedFirst = err == nil
	}
	drain(t, a, a.Init())
	if !savedFirst {
		t.Error("the JSON results should be saved before doctor starts")
	}
}

func TestAllFailedRefreshKeepsFetchedAt(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("boom")
	src.doctorErr = errors.New("boom")
	cached := fixtureSnapshot(t)
	cached.FetchedAt = testNow.Add(-time.Hour)
	a := New(src, cached, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	drain(t, a, a.request(qServices))
	if !a.snap.FetchedAt.Equal(testNow.Add(-time.Hour)) {
		t.Errorf("a failed query must not advance FetchedAt, got %v", a.snap.FetchedAt)
	}
}

func TestQuitCancelsRunningQueries(t *testing.T) {
	src := newCountingSource(t)
	src.block = true
	a := newTestApp(t, src)
	done := make(chan tea.Msg)
	go func() { done <- fetch(a.ctx, src, qInstalled)() }()

	_, cmd := a.Update(press("q"))
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q should quit")
	}
	select {
	case msg := <-done:
		a.Update(msg)
		if a.errs[qInstalled] != nil {
			t.Errorf("a query cancelled by quitting is not a failure: %v", a.errs[qInstalled])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("quitting did not cancel the running query")
	}
}
```

5. Replace:

```go
	a := newTestApp(t, src)
	drain(t, a, a.Init())

	if a.pending != 0 {
		t.Fatalf("pending = %d after refresh", a.pending)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "brew doctor failed") || strings.Contains(out, "brew doctor is running") {
```

   with:

```go
	a := newTestApp(t, src)
	drain(t, a, a.Init())

	if a.busy() {
		t.Fatalf("queries still in flight after refresh: %v", a.inflight)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "brew doctor failed") || strings.Contains(out, "brew doctor is running") {
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/app_test.go:82:7: a.busy undefined (type *App has no field or method busy)
    internal/ui/app_test.go:83:59: a.inflight undefined (type *App has no field or method inflight)
    internal/ui/app_test.go:153:7: a.busy undefined (type *App has no field or method busy)
```

- [ ] **Step 3: Edit `internal/ui/app.go`** (8 replacements; each old block appears exactly once)

1. Replace:

```go
package ui

import (
	"fmt"
	"strings"
	"time"
```

   with:

```go
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"
```

2. Replace:

```go
	cachePath string
	st        theme.Styles
	now       func() time.Time

	snap   state.Snapshot
	ix     *state.Index
	errs   map[query]error
	loaded map[query]bool // true once a query has succeeded, here or in the cache

	pending      int  // queries in flight
	doctorQueued bool // doctor already started in this refresh
	saveErr      error

	tab           int
	width, height int
```

   with:

```go
	cachePath string
	st        theme.Styles
	now       func() time.Time
	ctx       context.Context // cancelled on quit, stopping running queries
	cancel    context.CancelFunc

	snap   state.Snapshot
	ix     *state.Index
	errs   map[query]error
	loaded map[query]bool // true once a query has succeeded, here or in the cache

	inflight    map[query]bool // queries running now
	again       map[query]bool // requested while running: run once more when done
	doctorAfter bool           // run doctor when the JSON queries are done
	dirty       bool           // a query succeeded since the last cache save
	saveErr     error

	tab           int
	width, height int
```

3. Replace:

```go
// first frame can show data before any query returns.
func New(src Source, snap state.Snapshot, cachePath string) *App {
	st := theme.New(theme.Macchiato)
	a := &App{
		src: src, cachePath: cachePath, st: st, now: time.Now,
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, "no services registered"),
```

   with:

```go
// first frame can show data before any query returns.
func New(src Source, snap state.Snapshot, cachePath string) *App {
	st := theme.New(theme.Macchiato)
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		src: src, cachePath: cachePath, st: st, now: time.Now, ctx: ctx, cancel: cancel,
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, "no services registered"),
```

4. Replace:

```go

func (a *App) Init() tea.Cmd { return a.refresh() }

// refresh starts the four JSON queries in parallel. Doctor follows once
// they have all returned.
func (a *App) refresh() tea.Cmd {
	if a.pending > 0 {
		return nil
	}
	a.pending, a.doctorQueued = 4, false
	return tea.Batch(fetch(a.src, qInstalled), fetch(a.src, qOutdated), fetch(a.src, qServices), fetch(a.src, qTaps))
}

// sync pushes the snapshot and loaded state into every tab.
```

   with:

```go

func (a *App) Init() tea.Cmd { return a.refresh() }

// refresh re-runs every query: the JSON ones in parallel, then doctor.
func (a *App) refresh() tea.Cmd {
	a.doctorAfter = true
	return a.request(jsonQueries...)
}

// request starts the given queries. A query that is already running is not
// started twice; it runs once more when the current run returns, so a
// refresh asked for after a change is never lost.
func (a *App) request(qs ...query) tea.Cmd {
	var cmds []tea.Cmd
	for _, q := range qs {
		if a.inflight[q] {
			a.again[q] = true
			continue
		}
		a.inflight[q] = true
		cmds = append(cmds, fetch(a.ctx, a.src, q))
	}
	return tea.Batch(cmds...)
}

// busy reports whether any query is running.
func (a *App) busy() bool {
	for _, running := range a.inflight {
		if running {
			return true
		}
	}
	return false
}

// jsonBusy reports whether any of the four JSON queries is running.
func (a *App) jsonBusy() bool {
	for _, q := range jsonQueries {
		if a.inflight[q] {
			return true
		}
	}
	return false
}

// quit stops running queries and exits.
func (a *App) quit() tea.Cmd {
	a.cancel()
	return tea.Quit
}

// sync pushes the snapshot and loaded state into every tab.
```

5. Replace:

```go
	if m.err != nil {
		return // keep showing the last good data
	}
	a.loaded[m.q] = true
	switch m.q {
	case qInstalled:
		a.snap.Packages = m.packages
```

   with:

```go
	if m.err != nil {
		return // keep showing the last good data
	}
	a.loaded[m.q], a.dirty = true, true
	switch m.q {
	case qInstalled:
		a.snap.Packages = m.packages
```

6. Replace:

```go
		return a, nil

	case resultMsg:
		a.apply(msg)
		a.pending--
		if a.pending > 0 {
			return a, nil
		}
		if !a.doctorQueued {
			a.doctorQueued, a.pending = true, 1
			return a, fetch(a.src, qDoctor)
		}
		a.snap.FetchedAt = a.now()
		return a, save(a.cachePath, a.snap)

	case savedMsg:
		a.saveErr = msg.err
```

   with:

```go
		return a, nil

	case resultMsg:
		return a, a.handleResult(msg)

	case savedMsg:
		a.saveErr = msg.err
```

7. Replace:

```go
	return a, nil
}

func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch k := msg.String(); k {
	case "q", "ctrl+c":
		return tea.Quit
	case "1", "2", "3", "4", "5":
		a.tab = int(k[0] - '1')
		return nil
```

   with:

```go
	return a, nil
}

// handleResult applies one query result and decides what runs next: a
// repeat of the same query, doctor after a full refresh, and a cache save
// once the JSON queries are all back.
func (a *App) handleResult(m resultMsg) tea.Cmd {
	a.inflight[m.q] = false
	if m.err != nil && a.ctx.Err() != nil {
		return nil // quitting: the query was cancelled, not broken
	}
	a.apply(m)
	var again, saving, doctor tea.Cmd
	if a.again[m.q] {
		a.again[m.q] = false
		again = a.request(m.q)
	}
	if a.dirty && !a.jsonBusy() {
		a.dirty = false
		a.snap.FetchedAt = a.now()
		saving = save(a.cachePath, a.snap)
	}
	if a.doctorAfter && !a.jsonBusy() {
		a.doctorAfter = false
		doctor = a.request(qDoctor)
	}
	// Save before doctor starts: doctor can take 30s, and quitting
	// meanwhile must not lose the JSON results.
	return tea.Batch(again, tea.Sequence(saving, doctor))
}

func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch k := msg.String(); k {
	case "q", "ctrl+c":
		return a.quit()
	case "1", "2", "3", "4", "5":
		a.tab = int(k[0] - '1')
		return nil
```

8. Replace:

```go
		}
	}
	switch {
	case a.pending > 0:
		right = append(right, st.Info.Render("refreshing…"))
	case len(failed) > 0:
		right = append(right, st.Err.Render("✗ "+strings.Join(failed, ", ")+" failed"))
```

   with:

```go
		}
	}
	switch {
	case a.busy():
		right = append(right, st.Info.Render("refreshing…"))
	case len(failed) > 0:
		right = append(right, st.Err.Render("✗ "+strings.Join(failed, ", ")+" failed"))
```

- [ ] **Step 4: Edit `internal/ui/source.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	err      error
}

func fetch(src Source, q query) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		m := resultMsg{q: q}
		switch q {
		case qInstalled:
```

   with:

```go
	err      error
}

// jsonQueries are the queries a full refresh runs in parallel; doctor
// follows once they have all returned.
var jsonQueries = []query{qInstalled, qOutdated, qServices, qTaps}

// fetch runs one query. ctx is the App's root context, so quitting stops
// any brew process still running.
func fetch(ctx context.Context, src Source, q query) tea.Cmd {
	return func() tea.Msg {
		m := resultMsg{q: q}
		switch q {
		case qInstalled:
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(ui): track queries individually so refreshes are never dropped"
```

---

### Task 2: Keep the selection on the same package; find orphans

**Files:**
- Modify: `internal/state/index.go`
- Modify: `internal/state/index_test.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/components.go`
- Modify: `internal/ui/installed.go`
- Modify: `internal/ui/installed_test.go`
- Modify: `internal/ui/list.go`
- Modify: `internal/ui/list_test.go`

**Interfaces:**
- **Produces:**
  - `(*state.Index) Orphans(fullName string) []string` (sorted). These are the dependencies installed as dependencies that only `fullName`, or other orphans, use.
  - `(*state.Index) UsedBy` now returns a copy.
  - `setRowsKeeping(t *table.Model, rows []table.Row, key func(table.Row) string)`.
  - `newListTab(st, name, cols, key, empty)`, which gains a `key` parameter.
  - `(*listTab) cursor() int`, which returns -1 when empty.
  - The keys `firstCell`, `outdatedKey` and `installedKey`.
- **Behavior changes:**
  - The Installed tab sorts same-name rows formula first, using a stable sort.
  - Changing the filter keeps the selected package if the new filter shows it.

Plan 2's actions act on "the selected row", so the selection must not jump to a different package when a refresh lands.

- [ ] **Step 1: Edit `internal/state/index_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go

import (
	"reflect"
	"testing"

	"brewery/internal/brew"
```

   with:

```go

import (
	"reflect"
	"slices"
	"testing"

	"brewery/internal/brew"
```

2. Replace:

```go
		t.Errorf("got %q, want %q", got, want)
	}
}
```

   with:

```go
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestOrphans(t *testing.T) {
	ix := NewIndex(testPkgs)
	if got := ix.Orphans("git"); !reflect.DeepEqual(got, []string{"json-c", "pcre2"}) {
		t.Errorf("Orphans(git) = %v", got)
	}
	if got := ix.Orphans("fd"); len(got) != 0 {
		t.Errorf("fd has no deps, got %v", got)
	}
	if got := ix.Orphans("nope"); got != nil {
		t.Errorf("unknown package: %v", got)
	}
}

func TestOrphansKeepsWhatOthersNeed(t *testing.T) {
	pkgs := append(slices.Clone(testPkgs),
		brew.Package{Name: "jq", FullName: "jq", Kind: brew.KindFormula, OnRequest: true, Deps: []string{"json-c"}, DirectDeps: []string{"json-c"}})
	if got := NewIndex(pkgs).Orphans("git"); !reflect.DeepEqual(got, []string{"pcre2"}) {
		t.Errorf("json-c is still used by jq; got %v", got)
	}
}

func TestOrphansSkipsPackagesInstalledOnRequest(t *testing.T) {
	pkgs := slices.Clone(testPkgs)
	pkgs[1].OnRequest = true // pcre2 was installed by hand
	if got := NewIndex(pkgs).Orphans("git"); len(got) != 0 {
		t.Errorf("pcre2 was installed on request, and json-c is still used by it; got %v", got)
	}
}

func TestUsedByReturnsACopy(t *testing.T) {
	ix := NewIndex(testPkgs)
	ix.UsedBy("json-c")[0] = "mangled"
	if got := ix.UsedBy("json-c"); got[0] != "git" {
		t.Errorf("changing the result changed the index: %v", got)
	}
}
```

- [ ] **Step 2: Edit `internal/ui/installed_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go

func TestInstalledPreviewForDependencyAndCask(t *testing.T) {
	it := newFixtureInstalled(t)
	it.update(press("f")) // all
	for range 4 {
		it.update(press("j")) // docker-desktop fd fontbase git json-c
	}
	out := stripANSI(it.view(nil))
	if !strings.Contains(out, "as a dependency") || !strings.Contains(out, "git, pcre2") {
```

   with:

```go

func TestInstalledPreviewForDependencyAndCask(t *testing.T) {
	it := newFixtureInstalled(t)
	it.update(press("f")) // all: fd stays selected
	for range 3 {
		it.update(press("j")) // docker-desktop [fd] fontbase git json-c
	}
	out := stripANSI(it.view(nil))
	if !strings.Contains(out, "as a dependency") || !strings.Contains(out, "git, pcre2") {
```

2. Replace:

```go
		t.Errorf("height = %d, want 30", h)
	}
}
```

   with:

```go
		t.Errorf("height = %d, want 30", h)
	}
}

func TestInstalledKeepsSelectionAcrossRefresh(t *testing.T) {
	it := newFixtureInstalled(t)
	it.update(press("j")) // fd -> git
	snap := fixtureSnapshot(t)
	more := append([]brew.Package{{Name: "act", FullName: "act", Kind: brew.KindFormula, OnRequest: true}}, snap.Packages...)
	it.setData(more, state.NewIndex(more))
	if p, _ := it.selected(); p.Name != "git" {
		t.Errorf("selection moved to %q when a package was added above it", p.Name)
	}
}

func TestInstalledSortsSameNameFormulaBeforeCask(t *testing.T) {
	it := newInstalledTab(testStyles)
	it.resize(140, 30)
	it.filter = filterAll
	pkgs := []brew.Package{
		{Name: "docker", FullName: "docker", Kind: brew.KindCask},
		{Name: "docker", FullName: "docker", Kind: brew.KindFormula, OnRequest: true},
	}
	it.setData(pkgs, state.NewIndex(pkgs))
	if it.rows[0].Kind != brew.KindFormula || it.rows[1].Kind != brew.KindCask {
		t.Errorf("same-name rows should sort formula first: %+v", it.rows)
	}
}

func TestInstalledFilterChangeKeepsSelection(t *testing.T) {
	it := newFixtureInstalled(t)
	it.update(press("j")) // git
	it.update(press("f")) // all
	if p, _ := it.selected(); p.Name != "git" {
		t.Errorf("switching to all should keep git selected, got %q", p.Name)
	}
	it.update(press("f")) // casks: git isn't shown
	if p, ok := it.selected(); !ok || p.Kind != brew.KindCask {
		t.Errorf("a filter that hides the selection should still select a row, got %+v", p)
	}
}
```

- [ ] **Step 3: Edit `internal/ui/list_test.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go

func TestListTabRendersRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	lt := newListTab(testStyles, "Outdated", outdatedCols, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(snap.Outdated))
	out := stripANSI(lt.view(nil))
```

   with:

```go

func TestListTabRendersRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	lt := newListTab(testStyles, "Outdated", outdatedCols, outdatedKey, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(snap.Outdated))
	out := stripANSI(lt.view(nil))
```

2. Replace:

```go
}

func TestListTabEmptyAndError(t *testing.T) {
	lt := newListTab(testStyles, "Outdated", outdatedCols, "✓ everything is up to date")
	lt.resize(80, 8)
	if out := stripANSI(lt.view(nil)); !strings.Contains(out, "loading…") || strings.Contains(out, "up to date") {
		t.Errorf("before its first result the tab should say it is loading:\n%s", out)
```

   with:

```go
}

func TestListTabEmptyAndError(t *testing.T) {
	lt := newListTab(testStyles, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date")
	lt.resize(80, 8)
	if out := stripANSI(lt.view(nil)); !strings.Contains(out, "loading…") || strings.Contains(out, "up to date") {
		t.Errorf("before its first result the tab should say it is loading:\n%s", out)
```

3. Replace:

```go
}

func TestListTabErrorWithRowsShowsBanner(t *testing.T) {
	lt := newListTab(testStyles, "Outdated", outdatedCols, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(fixtureSnapshot(t).Outdated))
	view := lt.view(errors.New("brew outdated --json=v2: exit status 1\nError: second line"))
```

   with:

```go
}

func TestListTabErrorWithRowsShowsBanner(t *testing.T) {
	lt := newListTab(testStyles, "Outdated", outdatedCols, outdatedKey, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(fixtureSnapshot(t).Outdated))
	view := lt.view(errors.New("brew outdated --json=v2: exit status 1\nError: second line"))
```

4. Replace:

```go
		t.Errorf("bottom border missing:\n%s", out)
	}
}
```

   with:

```go
		t.Errorf("bottom border missing:\n%s", out)
	}
}

func TestListTabKeepsSelectionAcrossRefresh(t *testing.T) {
	lt := newListTab(testStyles, "Services", serviceCols, firstCell, "none")
	lt.resize(120, 10)
	lt.setRows([]table.Row{{"mysql", "", "", ""}, {"postgresql@16", "", "", ""}, {"redis", "", "", ""}})
	lt.update(press("j"))
	lt.update(press("j")) // redis
	lt.setRows([]table.Row{{"nginx", "", "", ""}, {"postgresql@16", "", "", ""}, {"redis", "", "", ""}, {"valkey", "", "", ""}})
	if got := lt.table.SelectedRow()[0]; got != "redis" {
		t.Errorf("selection moved to %q after refresh, want redis", got)
	}
	if lt.cursor() != 2 {
		t.Errorf("cursor() = %d, want 2", lt.cursor())
	}
	lt.setRows([]table.Row{{"nginx", "", "", ""}})
	if lt.cursor() != 0 {
		t.Errorf("when the item is gone the cursor stays in range, got %d", lt.cursor())
	}
	lt.setRows(nil)
	if lt.cursor() != -1 {
		t.Errorf("cursor() on an empty table = %d, want -1", lt.cursor())
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/state/index_test.go:69:15: ix.Orphans undefined (type *Index has no field or method Orphans)
    internal/state/index_test.go:72:15: ix.Orphans undefined (type *Index has no field or method Orphans)
    internal/state/index_test.go:75:15: ix.Orphans undefined (type *Index has no field or method Orphans)
```

- [ ] **Step 5: Edit `internal/state/index.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
package state

import (
	"sort"

	"brewery/internal/brew"
```

   with:

```go
package state

import (
	"maps"
	"slices"
	"sort"

	"brewery/internal/brew"
```

2. Replace:

```go
}

// UsedBy lists installed packages that depend on fullName, directly or not.
func (ix *Index) UsedBy(fullName string) []string {
	return ix.usedBy[fullName]
}

// IsLeaf reports whether p is a formula that no other installed package
```

   with:

```go
}

// UsedBy lists installed packages that depend on fullName, directly or not.
// The slice is a copy the caller may keep or change.
func (ix *Index) UsedBy(fullName string) []string {
	return slices.Clone(ix.usedBy[fullName])
}

// Orphans lists the dependencies that removing fullName would leave
// unneeded: packages installed as dependencies (not on request) that are
// used only by fullName or by other such orphans. Sorted.
func (ix *Index) Orphans(fullName string) []string {
	p, ok := ix.byFull[fullName]
	if !ok {
		return nil
	}
	cand := map[string]bool{}
	for _, d := range p.Deps {
		if dp, ok := ix.byFull[d]; ok && !dp.OnRequest {
			cand[d] = true
		}
	}
	// Drop any candidate something else still needs, until nothing changes.
	for changed := true; changed; {
		changed = false
		for c := range cand {
			for _, u := range ix.usedBy[c] {
				if u != fullName && !cand[u] {
					delete(cand, c)
					changed = true
					break
				}
			}
		}
	}
	return slices.Sorted(maps.Keys(cand))
}

// IsLeaf reports whether p is a formula that no other installed package
```

- [ ] **Step 6: Edit `internal/ui/app.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, "no services registered"),
		taps:      newListTab(st, "Taps", tapCols, "no taps installed"),
	}
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
```

   with:

```go
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
		taps:      newListTab(st, "Taps", tapCols, firstCell, "no taps installed"),
	}
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
```

- [ ] **Step 7: Edit `internal/ui/components.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	}
}

// kv renders aligned "label  value" lines for preview panes.
func kv(st theme.Styles, pairs ...string) string {
	var b strings.Builder
```

   with:

```go
	}
}

// setRowsKeeping replaces the rows and keeps the cursor on the same item,
// as identified by key, so a refresh never moves the selection to a
// different package. If the item is gone, the cursor stays in range.
func setRowsKeeping(t *table.Model, rows []table.Row, key func(table.Row) string) {
	var want string
	if old := t.SelectedRow(); old != nil {
		want = key(old)
	}
	setRows(t, rows)
	if want == "" {
		return
	}
	for i, r := range rows {
		if key(r) == want {
			t.SetCursor(i)
			return
		}
	}
}

// kv renders aligned "label  value" lines for preview panes.
func kv(st theme.Styles, pairs ...string) string {
	var b strings.Builder
```

- [ ] **Step 8: Edit `internal/ui/installed.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
			t.rows = append(t.rows, p)
		}
	}
	sort.Slice(t.rows, func(i, j int) bool { return t.rows[i].Name < t.rows[j].Name })
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{p.Name, p.Version, p.Kind.String(), status(p)}
	}
	setRows(&t.table, rows)
}

func status(p brew.Package) string {
	var s []string
	if p.Pinned {
```

   with:

```go
			t.rows = append(t.rows, p)
		}
	}
	sort.SliceStable(t.rows, func(i, j int) bool {
		if t.rows[i].Name != t.rows[j].Name {
			return t.rows[i].Name < t.rows[j].Name
		}
		return t.rows[i].Kind < t.rows[j].Kind
	})
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{p.Name, p.Version, p.Kind.String(), status(p)}
	}
	setRowsKeeping(&t.table, rows, installedKey)
}

// installedKey identifies an Installed row by name and type: a formula
// and a cask can share a name.
func installedKey(r table.Row) string { return r[0] + "\x00" + r[2] }

func status(p brew.Package) string {
	var s []string
	if p.Pinned {
```

2. Replace:

```go
func (t *installedTab) update(msg tea.KeyPressMsg) tea.Cmd {
	if msg.String() == "f" {
		t.filter = (t.filter + 1) % installedFilter(len(filterNames))
		t.table.SetCursor(0)
		t.apply()
		return nil
	}
	var cmd tea.Cmd
```

   with:

```go
func (t *installedTab) update(msg tea.KeyPressMsg) tea.Cmd {
	if msg.String() == "f" {
		t.filter = (t.filter + 1) % installedFilter(len(filterNames))
		t.apply() // keeps the selected package if the new filter shows it
		return nil
	}
	var cmd tea.Cmd
```

- [ ] **Step 9: Edit `internal/ui/list.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	empty  string // shown when there are no rows
	loaded bool   // a result has arrived, so an empty table means "nothing"
	n      int
	table  table.Model
	w, h   int
}

func newListTab(st theme.Styles, name string, cols []col, empty string) listTab {
	return listTab{st: st, name: name, cols: cols, empty: empty, table: newTable(st, cols)}
}

func (t *listTab) setRows(rows []table.Row) {
	t.n = len(rows)
	setRows(&t.table, rows)
}

func (t *listTab) update(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
```

   with:

```go
	empty  string // shown when there are no rows
	loaded bool   // a result has arrived, so an empty table means "nothing"
	n      int
	key    func(table.Row) string // identifies a row across refreshes
	table  table.Model
	w, h   int
}

func newListTab(st theme.Styles, name string, cols []col, key func(table.Row) string, empty string) listTab {
	return listTab{st: st, name: name, cols: cols, key: key, empty: empty, table: newTable(st, cols)}
}

func (t *listTab) setRows(rows []table.Row) {
	t.n = len(rows)
	setRowsKeeping(&t.table, rows, t.key)
}

// cursor is the selected row's index into the slice the rows were built
// from, or -1 when the table is empty.
func (t *listTab) cursor() int {
	if t.n == 0 {
		return -1
	}
	return t.table.Cursor()
}

// firstCell identifies a row by its first column (service and tap names).
func firstCell(r table.Row) string { return r[0] }

// outdatedKey identifies an Outdated row by name and type.
func outdatedKey(r table.Row) string { return r[0] + "\x00" + r[3] }

func (t *listTab) update(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
```

- [ ] **Step 10: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "feat: keep selection across refreshes and find orphaned dependencies"
```

---

### Task 3: Run brew in a pseudo-terminal

**Files:**
- Modify: `internal/brew/live_test.go`
- Create: `internal/brew/pty.go`
- Create: `internal/brew/pty_test.go`

**Interfaces:**
- **Produces:**
  - `brew.JobEnv` (`HOMEBREW_NO_AUTO_UPDATE=1`, `HOMEBREW_NO_ENV_HINTS=1`).
  - `brew.StartPTY(bin string, args, env []string, cols, rows int) (*brew.Proc, error)`.
  - `*Proc` methods:
    - `Read`: output, then `io.EOF`. On Linux, EIO is reported as EOF.
    - `Write`: keystrokes.
    - `Resize(cols, rows) error`.
    - `Interrupt() error`: SIGINT to the process group.
    - `Kill() error`: SIGKILL to the process group.
    - `Wait() (int, error)`: the exit code, or -1 if a signal ended the process.

`pty.StartWithSize` makes the command a session leader, so its process group ID equals its PID, and `-pid` reaches everything it starts. The tests use `/bin/sh` scripts, and the live test runs `brew --version`.

- [ ] **Step 1: Add the pseudo-terminal library**

```bash
go get github.com/creack/pty@v1.1.24
```

- [ ] **Step 2: Edit `internal/brew/live_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go

import (
	"context"
	"testing"
)
```

   with:

```go

import (
	"context"
	"io"
	"strings"
	"testing"
)
```

2. Replace:

```go
		t.Error(err)
	}
}
```

   with:

```go
		t.Error(err)
	}
}

// Runs a read-only brew command through the pseudo-terminal runner.
func TestLivePTY(t *testing.T) {
	bin, err := Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	p, err := StartPTY(bin, []string{"--version"}, JobEnv, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(p)
	code, err := p.Wait()
	if err != nil || code != 0 || !strings.Contains(string(out), "Homebrew") {
		t.Errorf("brew --version via PTY: code %d, err %v, output %q", code, err, out)
	}
}
```

- [ ] **Step 3: Create `internal/brew/pty_test.go`**

```go
package brew

import (
	"io"
	"strings"
	"testing"
	"time"
)

// sh starts a shell script in a pseudo-terminal.
func sh(t *testing.T, script string, env ...string) *Proc {
	t.Helper()
	p, err := StartPTY("/bin/sh", []string{"-c", script}, env, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// finish reads all output, then waits, failing the test if that takes
// longer than 5s.
func finish(t *testing.T, p *Proc) (string, int) {
	t.Helper()
	type result struct {
		out  string
		code int
	}
	done := make(chan result)
	go func() {
		b, _ := io.ReadAll(p)
		code, err := p.Wait()
		if err != nil {
			t.Error(err)
		}
		done <- result{string(b), code}
	}()
	select {
	case r := <-done:
		return r.out, r.code
	case <-time.After(5 * time.Second):
		t.Fatal("command did not finish")
		return "", 0
	}
}

func TestPTYOutputAndExitCode(t *testing.T) {
	out, code := finish(t, sh(t, `printf 'hello\nworld'; exit 3`))
	if !strings.Contains(out, "hello\r\nworld") {
		t.Errorf("output %q: a terminal turns \\n into \\r\\n", out)
	}
	if code != 3 {
		t.Errorf("exit code %d, want 3", code)
	}
}

func TestPTYIsATerminalOfTheRightSize(t *testing.T) {
	out, _ := finish(t, sh(t, `[ -t 1 ] && echo tty; stty size`))
	if !strings.Contains(out, "tty") || !strings.Contains(out, "24 80") {
		t.Errorf("want a 80x24 terminal, got %q", out)
	}
}

func TestPTYResize(t *testing.T) {
	p := sh(t, `sleep 0.3; stty size`)
	if err := p.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	if out, _ := finish(t, p); !strings.Contains(out, "30 100") {
		t.Errorf("want 100x30 after resize, got %q", out)
	}
}

func TestPTYInput(t *testing.T) {
	p := sh(t, `read answer; echo "got:$answer"`)
	if _, err := p.Write([]byte("yes\r")); err != nil {
		t.Fatal(err)
	}
	if out, _ := finish(t, p); !strings.Contains(out, "got:yes") {
		t.Errorf("input not delivered: %q", out)
	}
}

func TestPTYEnv(t *testing.T) {
	if out, _ := finish(t, sh(t, `echo "v=$BREWERY_TEST"`, "BREWERY_TEST=42")); !strings.Contains(out, "v=42") {
		t.Errorf("env not passed: %q", out)
	}
}

func TestPTYInterruptReachesChildren(t *testing.T) {
	p := sh(t, `sleep 30 & wait`)
	time.Sleep(100 * time.Millisecond)
	if err := p.Interrupt(); err != nil {
		t.Fatal(err)
	}
	if _, code := finish(t, p); code == 0 {
		t.Error("an interrupted command should not report success")
	}
}

func TestPTYKill(t *testing.T) {
	p := sh(t, `trap '' INT; sleep 30`)
	time.Sleep(100 * time.Millisecond)
	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}
	if _, code := finish(t, p); code != -1 {
		t.Errorf("a killed command reports -1, got %d", code)
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/brew/pty_test.go:11:54: undefined: Proc
    internal/brew/pty_test.go:13:12: undefined: StartPTY
    internal/brew/pty_test.go:22:30: undefined: Proc
```

- [ ] **Step 5: Create `internal/brew/pty.go`**

```go
package brew

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// JobEnv is added to the environment of commands started from a pane: an
// action should do what it says without running `brew update` first.
var JobEnv = []string{"HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1"}

// Proc is a brew command running in a pseudo-terminal, so it behaves as it
// would in a shell: colors, progress bars and password prompts all work.
type Proc struct {
	cmd *exec.Cmd
	pty *os.File
}

// StartPTY starts bin with args in a new pseudo-terminal of cols×rows.
// env is appended to the current environment. The command leads its own
// process group, so Interrupt and Kill reach everything it started.
func StartPTY(bin string, args, env []string, cols, rows int) (*Proc, error) {
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	f, err := pty.StartWithSize(cmd, winsize(cols, rows))
	if err != nil {
		return nil, err
	}
	return &Proc{cmd: cmd, pty: f}, nil
}

func winsize(cols, rows int) *pty.Winsize {
	return &pty.Winsize{Cols: uint16(max(cols, 1)), Rows: uint16(max(rows, 1))}
}

// Read returns the command's output. Once the command has exited and its
// output is drained, it returns io.EOF.
func (p *Proc) Read(b []byte) (int, error) {
	n, err := p.pty.Read(b)
	if errors.Is(err, syscall.EIO) {
		err = io.EOF // the terminal's other side closed: the command is gone
	}
	return n, err
}

// Write sends keystrokes to the command, as typing in a terminal would.
func (p *Proc) Write(b []byte) (int, error) { return p.pty.Write(b) }

// Resize tells the command its terminal is now cols×rows.
func (p *Proc) Resize(cols, rows int) error { return pty.Setsize(p.pty, winsize(cols, rows)) }

// Interrupt sends SIGINT to the command's process group, like ctrl+c.
func (p *Proc) Interrupt() error { return syscall.Kill(-p.cmd.Process.Pid, syscall.SIGINT) }

// Kill ends the command's process group immediately.
func (p *Proc) Kill() error { return syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL) }

// Wait waits for the command to exit and returns its exit code, or -1 if a
// signal ended it. Call it after Read has returned io.EOF.
func (p *Proc) Wait() (int, error) {
	err := p.cmd.Wait()
	p.pty.Close()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}
```

- [ ] **Step 6: Tidy the module and run the live test**

```bash
go mod tidy
make test-live   # expect ok: real brew, read-only, plus brew --version through the PTY
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(brew): run commands in a pseudo-terminal"
```

---

### Task 4: Job queue and lock retries

**Files:**
- Create: `internal/jobs/jobs.go`
- Create: `internal/jobs/jobs_test.go`

**Interfaces:**
- **Produces:** package `jobs`, with:
  - **State** values: `Queued`, `Running`, `Waiting`, `Done`, `Failed` and `Canceled`, plus `String()`.
  - **`Job`:** `ID`, `Title`, `Args`, `State`, `ExitCode`, `Attempts`, `Started`, `Ended`, and `Finished()`.
  - **`Queue`:** `Add(title, args) *Job`, `Current() *Job`, `Next(now) *Job`, `Queued() int`, `Finish(j, code, canceled, output, now) (retry bool)` and `Retry(j)`.
  - **Package-level:** `MaxAttempts = 5`, `Backoff(attempt) time.Duration` and `IsLockError(output) bool`.

The package is pure: no I/O and no Bubble Tea. While a job waits for the lock it stays current, so no other job can start.

- [ ] **Step 1: Create `internal/jobs/jobs_test.go`**

```go
package jobs

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)

func TestJobsRunOneAtATimeInOrder(t *testing.T) {
	var q Queue
	a := q.Add("upgrade fd", []string{"upgrade", "fd"})
	b := q.Add("pin git", []string{"pin", "git"})
	if a.ID == b.ID {
		t.Fatal("job IDs must be unique")
	}
	if got := q.Next(t0); got != a || a.State != Running || a.Attempts != 1 || !a.Started.Equal(t0) {
		t.Fatalf("first Next should start a: %+v", got)
	}
	if q.Next(t0) != nil {
		t.Error("a second job must not start while one is running")
	}
	if q.Queued() != 1 || q.Current() != a {
		t.Errorf("queued %d, current %+v", q.Queued(), q.Current())
	}
	if retry := q.Finish(a, 0, false, "", t0.Add(time.Second)); retry || a.State != Done || !a.Finished() {
		t.Errorf("exit 0 is done: %+v", a)
	}
	if q.Next(t0) != b || q.Queued() != 0 {
		t.Error("b should start after a finishes")
	}
}

func TestFinishStates(t *testing.T) {
	tests := []struct {
		code     int
		canceled bool
		output   string
		want     State
	}{
		{0, false, "", Done},
		{1, false, "Error: No such keg", Failed},
		{-1, true, "", Canceled},
		{1, false, "Error: A `brew upgrade` process has already locked /opt/homebrew/Cellar/fd.lock.", Waiting},
	}
	for _, tt := range tests {
		var q Queue
		j := q.Add("x", nil)
		q.Next(t0)
		retry := q.Finish(j, tt.code, tt.canceled, tt.output, t0)
		if j.State != tt.want || retry != (tt.want == Waiting) || j.ExitCode != tt.code {
			t.Errorf("Finish(%d, %v, %q) -> %v retry=%v", tt.code, tt.canceled, tt.output, j.State, retry)
		}
	}
}

func TestLockedJobRetriesThenFails(t *testing.T) {
	var q Queue
	j := q.Add("upgrade", []string{"upgrade"})
	other := q.Add("pin git", nil)
	q.Next(t0)
	locked := "A `brew update` process has already locked /opt/homebrew/var/homebrew/locks/update."
	for attempt := 1; attempt < MaxAttempts; attempt++ {
		if !q.Finish(j, 1, false, locked, t0) {
			t.Fatalf("attempt %d should retry", attempt)
		}
		if q.Next(t0) != nil || q.Current() != j {
			t.Fatal("nothing else may start while a job waits for the lock")
		}
		q.Retry(j)
		if j.State != Running || j.Attempts != attempt+1 {
			t.Fatalf("after retry: %+v", j)
		}
	}
	if q.Finish(j, 1, false, locked, t0) || j.State != Failed {
		t.Errorf("after %d attempts the job fails: %+v", MaxAttempts, j)
	}
	if q.Next(t0) != other {
		t.Error("the next job starts once the locked one gives up")
	}
}

func TestBackoff(t *testing.T) {
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for i, w := range want {
		if got := Backoff(i + 1); got != w {
			t.Errorf("Backoff(%d) = %v, want %v", i+1, got, w)
		}
	}
}

func TestStateString(t *testing.T) {
	if Waiting.String() != "waiting" || Canceled.String() != "canceled" {
		t.Error("state names wrong")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/jobs/jobs_test.go:11:8: undefined: Queue
    internal/jobs/jobs_test.go:17:47: undefined: Running
    internal/jobs/jobs_test.go:26:82: undefined: Done
```

- [ ] **Step 3: Create `internal/jobs/jobs.go`**

```go
// Package jobs tracks the brew commands brewery runs. They run one at a
// time, in the order they were asked for, because brew holds a global lock.
package jobs

import (
	"strings"
	"time"
)

// State is where a job is in its life.
type State int

const (
	Queued   State = iota
	Running        // started and not yet exited
	Waiting        // exited because another brew held the lock; will retry
	Done           // exited 0
	Failed         // exited non-zero, or could not start
	Canceled       // stopped by the user
)

func (s State) String() string {
	return [...]string{"queued", "running", "waiting", "done", "failed", "canceled"}[s]
}

// Job is one brew command.
type Job struct {
	ID       int
	Title    string   // what the user asked for, e.g. "upgrade fd"
	Args     []string // brew's arguments
	State    State
	ExitCode int
	Attempts int // starts so far, counting retries after a lock
	Started  time.Time
	Ended    time.Time
}

// Finished reports whether the job has stopped for good.
func (j *Job) Finished() bool { return j.State >= Done }

// Queue holds the session's jobs, oldest first.
type Queue struct {
	jobs   []*Job
	nextID int
}

// Add queues a job and returns it.
func (q *Queue) Add(title string, args []string) *Job {
	q.nextID++
	j := &Job{ID: q.nextID, Title: title, Args: args}
	q.jobs = append(q.jobs, j)
	return j
}

// Current is the running or lock-waiting job, or nil.
func (q *Queue) Current() *Job {
	for _, j := range q.jobs {
		if j.State == Running || j.State == Waiting {
			return j
		}
	}
	return nil
}

// Next starts the oldest queued job and returns it. It returns nil while
// another job is current or when nothing is queued.
func (q *Queue) Next(now time.Time) *Job {
	if q.Current() != nil {
		return nil
	}
	for _, j := range q.jobs {
		if j.State == Queued {
			j.State, j.Started, j.Attempts = Running, now, 1
			return j
		}
	}
	return nil
}

// Queued counts jobs waiting their turn.
func (q *Queue) Queued() int {
	n := 0
	for _, j := range q.jobs {
		if j.State == Queued {
			n++
		}
	}
	return n
}

// Finish records how a job's run ended. If brew refused to run because
// another brew process held the lock, the job waits to retry instead of
// failing, until MaxAttempts runs have been tried. It reports whether the
// job will be retried.
func (q *Queue) Finish(j *Job, code int, canceled bool, output string, now time.Time) (retry bool) {
	j.ExitCode = code
	switch {
	case canceled:
		j.State = Canceled
	case code == 0:
		j.State = Done
	case IsLockError(output) && j.Attempts < MaxAttempts:
		j.State = Waiting
		return true
	default:
		j.State = Failed
	}
	j.Ended = now
	return false
}

// Retry runs a waiting job again.
func (q *Queue) Retry(j *Job) {
	if j.State == Waiting {
		j.State = Running
		j.Attempts++
	}
}

// MaxAttempts is how many times a job runs before a held lock fails it.
const MaxAttempts = 5

// Backoff is how long to wait before retry number attempt (1-based):
// 2s, 4s, 8s, 16s, then 30s.
func Backoff(attempt int) time.Duration {
	return min(2*time.Second<<max(attempt-1, 0), 30*time.Second)
}

// IsLockError reports whether brew's output says another brew process
// holds the lock it needs ("A `brew …` process has already locked …").
func IsLockError(output string) bool {
	return strings.Contains(output, "has already locked")
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(jobs): add a one-at-a-time job queue with lock retries"
```

---

### Task 5: Output drawer

**Files:**
- Create: `internal/ui/drawer.go`
- Create: `internal/ui/drawer_test.go`

**Interfaces:**
- **Consumes:** `jobs.Job`, plus `pane` and `elapsed`. `elapsed` is defined in this task.
- **Produces:**
  - **`drawer` fields:** `open`, `job`, `cols`, `rows`, `retryAt`.
  - **Setup:** `newDrawer(st) drawer` and `drawerHeight(bodyH int) int`.
  - **Methods:**
    - `start(j *jobs.Job, reply io.Writer)`: resets the terminal for a new run.
    - `stop()`: detaches replies.
    - `write([]byte)`.
    - `output() string`: this run's raw output.
    - `resize(cols, rows)`.
    - `prompt() bool`.
    - `view(w, h int, now time.Time, frame int, insert bool) string`.
  - **Package-level:** `spinnerFrames`, `promptRe`, `elapsed(time.Duration) string`.

**Never close the emulator.** x/vt v0.1.0's `Close` races with the goroutine reading terminal replies; the race detector catches it. One emulator per drawer is reused instead: each run starts with `ESC c` (full reset) plus `ClearScrollback`. The reply goroutine lives as long as the drawer and forwards to whichever process is current, through `replyTo`. Replies must always be read, because the emulator's `Write` blocks on a terminal query (such as `ESC[6n`) when nobody reads.

- [ ] **Step 1: Add the terminal emulator**

```bash
go get github.com/charmbracelet/x/vt@v0.1.0
```

- [ ] **Step 2: Create `internal/ui/drawer_test.go`**

```go
package ui

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"brewery/internal/jobs"
)

// replies collects what the terminal sends back to the process.
type replies struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (r *replies) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(b)
}

func (r *replies) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

func runningJob() *jobs.Job {
	var q jobs.Queue
	j := q.Add("upgrade fd", []string{"upgrade", "fd"})
	q.Next(testNow)
	return j
}

func newTestDrawer(t *testing.T, j *jobs.Job) *drawer {
	d := newDrawer(testStyles)
	d.resize(60, 6)
	d.start(j, io.Discard)
	t.Cleanup(d.stop)
	return &d
}

func TestDrawerRendersLikeATerminal(t *testing.T) {
	d := newTestDrawer(t, runningJob())
	d.write([]byte("\x1b[34m==>\x1b[0m Upgrading fd\r\n"))
	d.write([]byte("####      20%"))
	d.write([]byte("\r##########100%\r\n"))
	view := d.view(64, 10, testNow.Add(42*time.Second), 2, false)
	out := stripANSI(view)
	for _, want := range []string{"⠹ brew upgrade fd · 0:42", "==> Upgrading fd", "##########100%", "esc hide · ctrl+c stop"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "20%") {
		t.Error("\\r should overwrite the progress line, as a terminal does")
	}
	if lipgloss.Height(view) != 10 || lipgloss.Width(view) != 64 {
		t.Errorf("drawer is %dx%d, want 64x10", lipgloss.Width(view), lipgloss.Height(view))
	}
}

func TestDrawerShowsTheNewestLines(t *testing.T) {
	d := newTestDrawer(t, runningJob())
	for i := range 20 {
		d.write([]byte("line " + string(rune('a'+i)) + "\r\n"))
	}
	out := stripANSI(d.view(64, 10, testNow, 0, false))
	if !strings.Contains(out, "line s") || strings.Contains(out, "line a") {
		t.Errorf("the drawer should follow the end of the output:\n%s", out)
	}
}

func TestDrawerPromptDetection(t *testing.T) {
	tests := []struct {
		output string
		want   bool
	}{
		{"==> Installing docker-desktop\r\nPassword:", true},
		{"[sudo] password for dillo: ", true},
		{"Do you want to continue? [y/N] ", true},
		{"Proceed (y/n)? ", true},
		{"==> Downloading https://example.com/x.dmg\r\n", false},
		{"Password:\r\nok, thanks\r\n", false}, // answered: the cursor moved on
	}
	for _, tt := range tests {
		d := newTestDrawer(t, runningJob())
		d.write([]byte(tt.output))
		if got := d.prompt(); got != tt.want {
			t.Errorf("prompt() after %q = %v, want %v", tt.output, got, tt.want)
		}
	}
}

func TestDrawerForwardsTerminalReplies(t *testing.T) {
	d := newDrawer(testStyles)
	r := &replies{}
	d.start(runningJob(), r)
	defer d.stop()
	d.write([]byte("\x1b[6n")) // "where is the cursor?" would block without a reader
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(r.String(), "\x1b[") {
		if time.Now().After(deadline) {
			t.Fatal("the cursor-position reply never reached the process")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDrawerTitles(t *testing.T) {
	var q jobs.Queue
	j := q.Add("upgrade fd", []string{"upgrade", "fd"})
	q.Next(testNow)
	d := newTestDrawer(t, j)

	if out := stripANSI(d.view(70, 8, testNow, 0, true)); !strings.Contains(out, "typing goes to brew") {
		t.Errorf("insert hint missing:\n%s", out)
	}
	q.Finish(j, 1, false, "A `brew update` process has already locked /x.", testNow)
	d.retryAt = testNow.Add(4 * time.Second)
	if out := stripANSI(d.view(70, 8, testNow, 0, false)); !strings.Contains(out, "waiting for brew's lock") || !strings.Contains(out, "retry 2 of 5 in 0:04") {
		t.Errorf("waiting title wrong:\n%s", out)
	}
	j.State, j.ExitCode = jobs.Done, 0
	if out := stripANSI(d.view(70, 8, testNow, 0, false)); !strings.Contains(out, "✓ brew upgrade fd · exit 0") {
		t.Errorf("done title wrong:\n%s", out)
	}
	j.State, j.ExitCode = jobs.Failed, 1
	if out := stripANSI(d.view(70, 8, testNow, 0, false)); !strings.Contains(out, "✗ brew upgrade fd · failed, exit 1") {
		t.Errorf("failed title wrong:\n%s", out)
	}
}

func TestDrawerStartsEachRunOnACleanScreen(t *testing.T) {
	d := newTestDrawer(t, runningJob())
	d.write([]byte("\x1b[31mfirst run\r\n"))
	d.start(d.job, io.Discard)
	if d.output() != "" {
		t.Errorf("a new run starts with empty output, got %q", d.output())
	}
	d.write([]byte("second"))
	out := d.view(64, 10, testNow, 0, false)
	if strings.Contains(stripANSI(out), "first run") {
		t.Errorf("the previous run's text should be gone:\n%s", stripANSI(out))
	}
	if strings.Contains(out, "\x1b[31msecond") {
		t.Error("colors from the previous run must not leak into the next")
	}
}

func TestDrawerDropsRepliesAfterStop(t *testing.T) {
	d := newDrawer(testStyles)
	r := &replies{}
	d.start(runningJob(), r)
	d.stop()
	d.write([]byte("\x1b[6n"))
	time.Sleep(50 * time.Millisecond)
	if r.String() != "" {
		t.Errorf("a stopped run must not get replies: %q", r.String())
	}
}

func TestDrawerHeight(t *testing.T) {
	for _, tt := range []struct{ body, want int }{{40, 14}, {21, 8}, {12, 6}, {7, 4}, {3, 0}} {
		if got := drawerHeight(tt.body); got != tt.want {
			t.Errorf("drawerHeight(%d) = %d, want %d", tt.body, got, tt.want)
		}
	}
}

func TestElapsed(t *testing.T) {
	if elapsed(65*time.Second) != "1:05" || elapsed(0) != "0:00" {
		t.Error("elapsed format wrong")
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/drawer_test.go:41:48: undefined: drawer
    internal/ui/drawer_test.go:42:7: undefined: newDrawer
    internal/ui/drawer_test.go:102:7: undefined: newDrawer
```

- [ ] **Step 4: Create `internal/ui/drawer.go`**

```go
package ui

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"

	"brewery/internal/jobs"
	"brewery/internal/theme"
)

// promptRe matches a terminal line that is waiting for the user: a
// password prompt (sudo) or a yes/no question.
var promptRe = regexp.MustCompile(`(?i)(password|passphrase)[^:]*:\s*$|[\[(]y/n[\])]\??\s*$`)

// spinnerFrames animate a running job's title.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// drawer shows the current job's output live, under the tab. Output goes
// through a terminal emulator, so progress bars that redraw themselves
// with \r or cursor movement look as they would in a real terminal.
//
// One emulator serves every job and is reset between runs. It is never
// closed: in x/vt v0.1.0, Close races with the goroutine reading its
// replies, so that goroutine instead lives as long as the drawer.
type drawer struct {
	st         theme.Styles
	open       bool
	job        *jobs.Job
	term       *vt.SafeEmulator
	reply      *replyTo
	raw        strings.Builder // this run's output, for spotting lock errors
	cols, rows int
	retryAt    time.Time // when a lock-waiting job runs again
}

func newDrawer(st theme.Styles) drawer {
	d := drawer{st: st, cols: 80, rows: 10, reply: &replyTo{}}
	d.term = vt.NewSafeEmulator(d.cols, d.rows)
	// The emulator answers terminal queries ("where is the cursor?") on its
	// read side and blocks if nobody reads, so replies are always drained.
	go io.Copy(d.reply, d.term)
	return d
}

// replyTo forwards terminal replies to the running process, or drops them
// when no process is running.
type replyTo struct {
	mu sync.Mutex
	w  io.Writer
}

func (r *replyTo) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.w == nil {
		return len(b), nil
	}
	return r.w.Write(b)
}

func (r *replyTo) set(w io.Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.w = w
}

// drawerHeight is how tall the drawer is for a body of height bodyH: about
// two fifths, between 6 and 14 lines, leaving the tab at least 3.
func drawerHeight(bodyH int) int {
	return max(min(bodyH*2/5, 14, bodyH-3), min(6, bodyH-3), 0)
}

// start shows a new run of j on a clean screen. Terminal replies go to
// reply, the running process.
func (d *drawer) start(j *jobs.Job, reply io.Writer) {
	d.job, d.open = j, true
	d.raw.Reset()
	d.term.Write([]byte("\x1bc")) // full terminal reset: screen, cursor, colors
	d.term.ClearScrollback()
	d.reply.set(reply)
}

// stop detaches the finished process: later replies are dropped.
func (d *drawer) stop() { d.reply.set(nil) }

// write feeds process output to the terminal.
func (d *drawer) write(b []byte) {
	d.term.Write(b)
	d.raw.Write(b)
}

// output is everything this run has printed.
func (d *drawer) output() string { return d.raw.String() }

// resize sets the terminal's size, which is the drawer's inner area.
func (d *drawer) resize(cols, rows int) {
	d.cols, d.rows = max(cols, 1), max(rows, 1)
	d.term.Resize(d.cols, d.rows)
}

// prompt reports whether the process is waiting for typed input: the text
// before the cursor looks like a password prompt or a y/n question.
func (d *drawer) prompt() bool {
	pos := d.term.CursorPosition()
	lines := strings.Split(ansi.Strip(d.term.Render()), "\n")
	if pos.Y < 0 || pos.Y >= len(lines) {
		return false
	}
	return promptRe.MatchString(ansi.Truncate(lines[pos.Y], pos.X, ""))
}

// view renders the drawer as a w×h pane. frame picks the spinner frame;
// insert says whether keys currently go to the process.
func (d *drawer) view(w, h int, now time.Time, frame int, insert bool) string {
	st := d.st
	j := d.job
	if j == nil {
		return ""
	}
	cmd := "brew " + strings.Join(j.Args, " ")
	var title, hint string
	switch j.State {
	case jobs.Running:
		title = spinnerFrames[frame%len(spinnerFrames)] + " " + cmd + " · " + elapsed(now.Sub(j.Started))
		hint = "esc hide · ctrl+c stop · i type into brew"
		if insert {
			hint = st.OK.Render("typing goes to brew") + st.Muted.Render(" · esc stop typing")
		}
	case jobs.Waiting:
		title = "⏸ " + cmd + " · waiting for brew's lock"
		hint = fmt.Sprintf("another brew is running · retry %d of %d in %s · ctrl+c stop",
			j.Attempts+1, jobs.MaxAttempts, elapsed(max(d.retryAt.Sub(now), 0)))
	case jobs.Done:
		title = st.OK.Render("✓ "+cmd) + " · exit 0"
		hint = "esc close"
	default: // Failed, Canceled
		title = st.Err.Render("✗ "+cmd) + fmt.Sprintf(" · %s, exit %d", j.State, j.ExitCode)
		hint = "esc close"
	}
	body := d.term.Render()
	inner := max(h-4, 0) // border, title and hint lines
	lines := strings.Split(body, "\n")
	if len(lines) > inner {
		lines = lines[len(lines)-inner:]
	}
	for len(lines) < inner {
		lines = append(lines, "")
	}
	lines = append(lines, st.Muted.Render(hint))
	return pane(st, title, strings.Join(lines, "\n"), w, h, true)
}

// elapsed formats a duration as m:ss.
func elapsed(d time.Duration) string {
	s := int(d.Round(time.Second).Seconds())
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
```

- [ ] **Step 5: Tidy the module**

```bash
go mod tidy
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(ui): add the output drawer with a terminal emulator"
```

---

### Task 6: Confirmation dialog

**Files:**
- Create: `internal/ui/actions.go`
- Create: `internal/ui/confirm.go`
- Create: `internal/ui/confirm_test.go`

**Interfaces:**
- **Produces:**
  - `action{title string; args []string; refresh []query; full bool}`. Task 8 adds behavior to `actions.go`.
  - `choice{key, label string; run func() tea.Cmd}`. A nil `run` cancels.
  - `confirmBox{title string; body []string; choices []choice}`, with `answer(key) (choice, ok bool)` and `view(st, maxW) string`.
  - `overlay(body, box string, w, h int) string`, which composites with a Lip Gloss canvas.

`answer` returns `ok=false` for keys that aren't choices, so a stray `q` neither quits nor confirms.

- [ ] **Step 1: Create `internal/ui/confirm_test.go`**

```go
package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// picked records which choice ran.
var picked string

func testConfirm() *confirmBox {
	pick := func(name string) func() tea.Cmd {
		return func() tea.Cmd { picked = name; return nil }
	}
	return &confirmBox{
		title: "Uninstall git 2.51.0?",
		body:  []string{"It leaves 2 orphaned dependencies: json-c, pcre2"},
		choices: []choice{
			{"y", "uninstall", pick("uninstall")},
			{"o", "uninstall + orphans", pick("orphans")},
			{"n", "cancel", nil},
		},
	}
}

func TestConfirmAnswers(t *testing.T) {
	c := testConfirm()
	ch, ok := c.answer("o")
	if !ok || ch.run == nil {
		t.Fatalf("o should be a choice: %+v %v", ch, ok)
	}
	if ch.run(); picked != "orphans" {
		t.Errorf("o ran %q, want orphans", picked)
	}
	if ch, ok := c.answer("n"); !ok || ch.run != nil {
		t.Errorf("n cancels: %+v", ch)
	}
	if ch, ok := c.answer("esc"); !ok || ch.run != nil {
		t.Errorf("esc always cancels: %+v", ch)
	}
	if _, ok := c.answer("q"); ok {
		t.Error("keys that are not choices are ignored, so a stray q can't quit or confirm")
	}
}

func TestConfirmView(t *testing.T) {
	out := stripANSI(testConfirm().view(testStyles, 80))
	for _, want := range []string{"Uninstall git 2.51.0?", "orphaned dependencies: json-c, pcre2", "[y] uninstall", "[o] uninstall + orphans", "[n] cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if w := lipgloss.Width(testConfirm().view(testStyles, 30)); w > 30 {
		t.Errorf("box is %d wide with 30 columns available", w)
	}
}

func TestOverlayKeepsSizeAndShowsBoth(t *testing.T) {
	body := strings.TrimRight(strings.Repeat(strings.Repeat("·", 60)+"\n", 20), "\n")
	box := testConfirm().view(testStyles, 60)
	out := overlay(body, box, 60, 20)
	if lipgloss.Height(out) != 20 || lipgloss.Width(out) != 60 {
		t.Errorf("overlay is %dx%d, want 60x20", lipgloss.Width(out), lipgloss.Height(out))
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "Uninstall git") {
		t.Error("the box should be drawn")
	}
	lines := strings.Split(plain, "\n")
	if lines[0] != strings.Repeat("·", 60) {
		t.Errorf("the body should show around the box: %q", lines[0])
	}
}

func TestConfirmWithoutBodyHasOneBlankLine(t *testing.T) {
	c := &confirmBox{title: "Uninstall fd 10.5.0?", choices: []choice{{"y", "uninstall", nil}, {"n", "cancel", nil}}}
	lines := strings.Split(stripANSI(c.view(testStyles, 80)), "\n")
	if len(lines) != 5 { // border, title, blank, choices, border
		t.Errorf("want 5 lines, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/confirm_test.go:14:21: undefined: confirmBox
    internal/ui/confirm_test.go:18:10: undefined: confirmBox
    internal/ui/confirm_test.go:21:14: undefined: choice
```

- [ ] **Step 3: Create `internal/ui/actions.go`**

```go
package ui

// action is something the user asked brew to do from a pane.
type action struct {
	title   string   // shown in the drawer and status bar, e.g. "upgrade fd"
	args    []string // brew's arguments
	refresh []query  // queries to re-run when it finishes
	full    bool     // re-run every query, doctor included (after brew update)
}
```

- [ ] **Step 4: Create `internal/ui/confirm.go`**

```go
package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/theme"
)

// choice is one answer to a confirmation: the key that picks it, a label,
// and what it does. A nil run cancels.
type choice struct {
	key   string
	label string
	run   func() tea.Cmd
}

// confirmBox asks before brewery does something destructive.
type confirmBox struct {
	title   string
	body    []string
	choices []choice
}

// answer maps a key press to a choice. esc always cancels; keys that are
// not choices are ignored (ok is false).
func (c *confirmBox) answer(key string) (picked choice, ok bool) {
	if key == "esc" {
		return choice{}, true
	}
	for _, ch := range c.choices {
		if ch.key == key {
			return ch, true
		}
	}
	return choice{}, false
}

// view renders the box, at most maxW columns wide.
func (c *confirmBox) view(st theme.Styles, maxW int) string {
	inner := min(56, max(maxW-4, 10))
	lines := []string{st.Warn.Bold(true).Render(ansi.Truncate(c.title, inner, "…"))}
	if len(c.body) > 0 {
		lines = append(lines, "")
	}
	for _, l := range c.body {
		lines = append(lines, ansi.Truncate(l, inner, "…"))
	}
	keys := make([]string, len(c.choices))
	for i, ch := range c.choices {
		keys[i] = st.Accent.Bold(true).Render("["+ch.key+"]") + " " + ch.label
	}
	lines = append(lines, "", ansi.Truncate(strings.Join(keys, "   "), inner, "…"))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(st.P.Peach).
		Padding(0, 1).Render(strings.Join(lines, "\n"))
}

// overlay draws box centered over body, which is w×h; the result is w×h.
func overlay(body, box string, w, h int) string {
	x := max((w-lipgloss.Width(box))/2, 0)
	y := max((h-lipgloss.Height(box))/2, 0)
	canvas := lipgloss.NewCanvas(w, h)
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(body),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	))
	return canvas.Render()
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(ui): add confirmation dialogs drawn over the tab"
```

---

### Task 7: Run jobs from the app

**Files:**
- Modify: `cmd/brewery/main.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`
- Modify: `internal/ui/helpers_test.go`
- Create: `internal/ui/run.go`
- Create: `internal/ui/run_test.go`

**Interfaces:**
- **Consumes:** Tasks 1–6.
- **Produces:**
  - **`ui.Proc`:** an interface of `io.Reader`, `io.Writer`, `Resize`, `Interrupt`, `Kill` and `Wait`. `*brew.Proc` satisfies it.
  - **`ui.Starter`:** `func(args, env []string, cols, rows int) (Proc, error)`.
  - **`ui.New`:** `New(src Source, start Starter, snap state.Snapshot, cachePath string)`. The start parameter is new.
  - **`App` methods:** `run(action) tea.Cmd`, `runNext`, `launch`, `finish`, `afterJob`, `stopJob`, `askStop`, `askQuit`, `layout`, `drawerShown`.
  - **`mode`:** `modeNormal` and `modeInsert`.
  - **`keyBytes(tea.KeyPressMsg) []byte`.**
  - **Job messages:** `jobOutputMsg`, `jobEOFMsg`, `jobExitMsg`, `jobRetryMsg`, `tickMsg`.
- **Test fields:** `App.tickEvery` (0 turns the spinner off) and `App.backoff`.
- **Test helpers:** `fakeProc`, `fakeStarter`, `newJobApp`, and `settle`/`resume`, which park commands that block (a read waiting for output) so a test can act and then continue. The `press` helper learns `esc`, `backspace` and `up`.
- **`main.go`:** builds the starter from `brew.StartPTY`.

Output is read one chunk per command and the read is re-issued after each chunk. EOF leads to `Wait`, then `finish`. Each message carries the job ID and attempt, so late messages from an earlier run are ignored.

Keys:
- `esc` hides the drawer.
- `o` toggles it.
- `i` types into brew.
- `ctrl+c` with a job running asks, then stops it. Without a job, it quits.
- `q` asks when a job is running.

`layout()` runs after every message, but does work only when the size or the drawer's visibility changed.

- [ ] **Step 1: Edit `internal/ui/app_test.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go

func newTestApp(t *testing.T, src Source) *App {
	t.Helper()
	a := New(src, state.Snapshot{}, filepath.Join(t.TempDir(), "snapshot.json"))
	a.now = func() time.Time { return testNow }
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return a
```

   with:

```go

func newTestApp(t *testing.T, src Source) *App {
	t.Helper()
	a := New(src, nil, state.Snapshot{}, filepath.Join(t.TempDir(), "snapshot.json"))
	a.now = func() time.Time { return testNow }
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return a
```

2. Replace:

```go
	src.doctorErr = errors.New("boom")
	cached := fixtureSnapshot(t)
	cached.FetchedAt = testNow.Add(-time.Hour)
	a := New(src, cached, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	drain(t, a, a.request(qServices))
	if !a.snap.FetchedAt.Equal(testNow.Add(-time.Hour)) {
```

   with:

```go
	src.doctorErr = errors.New("boom")
	cached := fixtureSnapshot(t)
	cached.FetchedAt = testNow.Add(-time.Hour)
	a := New(src, nil, cached, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	drain(t, a, a.request(qServices))
	if !a.snap.FetchedAt.Equal(testNow.Add(-time.Hour)) {
```

3. Replace:

```go

func TestCachedSnapshotShowsBeforeRefresh(t *testing.T) {
	cached := state.Snapshot{Packages: []brew.Package{{Name: "neovim", FullName: "neovim", Kind: brew.KindFormula, Version: "0.11.4", OnRequest: true}}}
	a := New(newFakeSource(t), cached, filepath.Join(t.TempDir(), "s.json"))
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	a.Update(press("2"))
	if !strings.Contains(stripANSI(a.render()), "neovim") {
```

   with:

```go

func TestCachedSnapshotShowsBeforeRefresh(t *testing.T) {
	cached := state.Snapshot{Packages: []brew.Package{{Name: "neovim", FullName: "neovim", Kind: brew.KindFormula, Version: "0.11.4", OnRequest: true}}}
	a := New(newFakeSource(t), nil, cached, filepath.Join(t.TempDir(), "s.json"))
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	a.Update(press("2"))
	if !strings.Contains(stripANSI(a.render()), "neovim") {
```

4. Replace:

```go
func TestRefreshFailureShowsOverCachedRows(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("brew services list --json: exit status 1\nError: launchctl failed")
	a := New(src, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	drain(t, a, a.Init())
```

   with:

```go
func TestRefreshFailureShowsOverCachedRows(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("brew services list --json: exit status 1\nError: launchctl failed")
	a := New(src, nil, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	drain(t, a, a.Init())
```

5. Replace:

```go
}

func TestCachedSnapshotCountsAsLoaded(t *testing.T) {
	a := New(newFakeSource(t), fixtureSnapshot(t), "")
	for _, q := range []query{qInstalled, qOutdated, qServices, qTaps, qDoctor} {
		if !a.loaded[q] {
			t.Errorf("%s not loaded from a non-empty cache", queryNames[q])
		}
	}
	if a := New(newFakeSource(t), state.Snapshot{}, ""); len(a.loaded) != 0 && (a.loaded[qOutdated] || a.loaded[qDoctor]) {
		t.Error("an empty cache should load nothing")
	}
}
```

   with:

```go
}

func TestCachedSnapshotCountsAsLoaded(t *testing.T) {
	a := New(newFakeSource(t), nil, fixtureSnapshot(t), "")
	for _, q := range []query{qInstalled, qOutdated, qServices, qTaps, qDoctor} {
		if !a.loaded[q] {
			t.Errorf("%s not loaded from a non-empty cache", queryNames[q])
		}
	}
	if a := New(newFakeSource(t), nil, state.Snapshot{}, ""); len(a.loaded) != 0 && (a.loaded[qOutdated] || a.loaded[qDoctor]) {
		t.Error("an empty cache should load nothing")
	}
}
```

- [ ] **Step 2: Edit `internal/ui/helpers_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
```

   with:

```go
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
```

- [ ] **Step 3: Create `internal/ui/run_test.go`**

```go
package ui

import (
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/jobs"
)

// fakeProc is a scripted brew process. Output chunks are delivered in
// order; after the last one, Read blocks until the test calls exit (or the
// process is interrupted or killed), then returns io.EOF.
type fakeProc struct {
	mu          sync.Mutex
	out         chan []byte
	in          strings.Builder
	code        int
	exited      bool
	interrupted bool
	killed      bool
	cols, rows  int
}

func newFakeProc(code int, chunks ...string) *fakeProc {
	p := &fakeProc{out: make(chan []byte, len(chunks)+8), code: code}
	for _, c := range chunks {
		p.out <- []byte(c)
	}
	return p
}

// exit lets Read return io.EOF once the queued output is read.
func (p *fakeProc) exit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.exited {
		p.exited = true
		close(p.out)
	}
}

func (p *fakeProc) Read(b []byte) (int, error) {
	chunk, ok := <-p.out
	if !ok {
		return 0, io.EOF
	}
	return copy(b, chunk), nil
}

func (p *fakeProc) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.in.Write(b)
}

func (p *fakeProc) input() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.in.String()
}

func (p *fakeProc) Resize(cols, rows int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cols, p.rows = cols, rows
	return nil
}

func (p *fakeProc) Interrupt() error {
	p.mu.Lock()
	p.interrupted, p.code = true, 130
	p.mu.Unlock()
	p.exit()
	return nil
}

func (p *fakeProc) Kill() error {
	p.mu.Lock()
	p.killed, p.code = true, -1
	p.mu.Unlock()
	p.exit()
	return nil
}

func (p *fakeProc) Wait() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.code, nil
}

// fakeStarter hands out scripted processes in order and records each
// command it was asked to start.
type fakeStarter struct {
	mu    sync.Mutex
	procs []*fakeProc
	calls [][]string
	envs  [][]string
	err   error
}

func (f *fakeStarter) start(args, env []string, cols, rows int) (Proc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, args)
	f.envs = append(f.envs, env)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.procs) == 0 {
		p := newFakeProc(0)
		p.exit()
		return p, nil
	}
	p := f.procs[0]
	f.procs = f.procs[1:]
	p.Resize(cols, rows)
	return p, nil
}

// newJobApp builds an app whose jobs run on fs, with the spinner off and no
// waiting between lock retries.
func newJobApp(t *testing.T, fs *fakeStarter) *App {
	t.Helper()
	a := New(newFakeSource(t), fs.start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.backoff = func(int) time.Duration { return 0 }
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return a
}

// parked holds the results of commands still running when settle gave up
// waiting, typically a read waiting for more output.
type parked []chan tea.Msg

// settle runs cmd and everything it leads to, like drain, but parks any
// command that takes longer than 100ms instead of blocking the test.
func settle(t *testing.T, a *App, cmd tea.Cmd) parked {
	t.Helper()
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		return deliver(t, a, msg)
	case <-time.After(100 * time.Millisecond):
		return parked{ch}
	}
}

// deliver feeds one message to the app and settles what follows.
func deliver(t *testing.T, a *App, msg tea.Msg) parked {
	t.Helper()
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		var out parked
		for i := range v.Len() {
			out = append(out, settle(t, a, v.Index(i).Interface().(tea.Cmd))...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	_, next := a.Update(msg)
	return settle(t, a, next)
}

// resume waits for parked commands (after the test let them continue) and
// settles what follows.
func resume(t *testing.T, a *App, p parked) parked {
	t.Helper()
	var out parked
	for _, ch := range p {
		select {
		case msg := <-ch:
			out = append(out, deliver(t, a, msg)...)
		case <-time.After(2 * time.Second):
			t.Fatal("a parked command never finished")
		}
	}
	return out
}

func TestJobRunsAndShowsOutput(t *testing.T) {
	p := newFakeProc(0, "==> Upgrading fd\r\n", "🍺 fd 10.6.0 installed\r\n")
	fs := &fakeStarter{procs: []*fakeProc{p}}
	a := newJobApp(t, fs)

	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}, refresh: []query{qInstalled, qOutdated}}))
	if !reflect.DeepEqual(fs.calls, [][]string{{"upgrade", "fd"}}) {
		t.Fatalf("started %v", fs.calls)
	}
	if !reflect.DeepEqual(fs.envs[0], []string{"HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1"}) {
		t.Errorf("pane actions must not auto-update: env %v", fs.envs[0])
	}
	out := stripANSI(a.render())
	for _, want := range []string{"brew upgrade fd", "fd 10.6.0 installed", "RUNNING", "upgrade fd"} {
		if !strings.Contains(out, want) {
			t.Errorf("while running, missing %q:\n%s", want, out)
		}
	}
	if lipgloss.Height(a.render()) != 40 {
		t.Errorf("frame is %d lines with the drawer open, want 40", lipgloss.Height(a.render()))
	}

	p.exit()
	resume(t, a, waiting)
	if j := a.queue.Current(); j != nil {
		t.Fatalf("job still current: %+v", j)
	}
	out = stripANSI(a.render())
	if !strings.Contains(out, "✓ upgrade fd") || !strings.Contains(out, "NORMAL") {
		t.Errorf("after exit 0, missing the done flash or NORMAL:\n%s", out)
	}
	if !strings.Contains(out, "✓ brew upgrade fd · exit 0") {
		t.Errorf("the drawer should stay open showing the result:\n%s", out)
	}
}

func TestJobPTYSizeMatchesDrawer(t *testing.T) {
	p := newFakeProc(0)
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
	if p.cols != 116 || p.rows != 10 {
		t.Errorf("pty is %dx%d, want the drawer's inner 116x10", p.cols, p.rows)
	}
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if p.cols != 96 || p.rows != 6 {
		t.Errorf("after resize pty is %dx%d, want 96x6", p.cols, p.rows)
	}
	p.exit()
	resume(t, a, waiting)
}

func TestJobsQueueBehindTheRunningOne(t *testing.T) {
	first, second := newFakeProc(0), newFakeProc(0)
	fs := &fakeStarter{procs: []*fakeProc{first, second}}
	a := newJobApp(t, fs)
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	waiting = append(waiting, settle(t, a, a.run(action{title: "pin git", args: []string{"pin", "git"}}))...)
	if len(fs.calls) != 1 {
		t.Fatalf("the second job must wait: started %v", fs.calls)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "+1 queued") {
		t.Errorf("status bar should count the queued job: %q", bar)
	}
	first.exit()
	waiting = resume(t, a, waiting)
	if len(fs.calls) != 2 || fs.calls[1][0] != "pin" {
		t.Fatalf("the queued job should start next: %v", fs.calls)
	}
	second.exit()
	resume(t, a, waiting)
}

func TestFinishedJobRefreshesWhatItTouched(t *testing.T) {
	src := newCountingSource(t)
	p := newFakeProc(0)
	p.exit()
	a := New(src, (&fakeStarter{procs: []*fakeProc{p}}).start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	settle(t, a, a.run(action{title: "restart redis", args: []string{"services", "restart", "redis"}, refresh: []query{qServices}}))
	if src.calls["services"] != 1 || src.calls["installed"] != 0 || src.calls["doctor"] != 0 {
		t.Errorf("a services action should refresh only services: %v", src.calls)
	}
}

func TestFailedJobStillRefreshesAndSaysSo(t *testing.T) {
	p := newFakeProc(1, "Error: No such keg: /opt/homebrew/Cellar/fd\r\n")
	p.exit()
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	settle(t, a, a.run(action{title: "uninstall fd", args: []string{"uninstall", "fd"}, refresh: []query{qInstalled}}))
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "✗ uninstall fd failed (exit 1)") {
		t.Errorf("status bar: %q", bar)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "No such keg") || !strings.Contains(out, "failed, exit 1") {
		t.Errorf("the drawer should keep the error visible:\n%s", out)
	}
}

func TestStartFailureIsReported(t *testing.T) {
	a := newJobApp(t, &fakeStarter{err: errors.New("fork/exec /opt/homebrew/bin/brew: no such file")})
	settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "couldn't start brew") {
		t.Errorf("status bar: %q", bar)
	}
	if a.queue.Current() != nil {
		t.Error("a job that could not start must not stay current")
	}
}

func TestLockedJobWaitsAndRetries(t *testing.T) {
	locked := newFakeProc(1, "Error: A `brew update` process has already locked /opt/homebrew/var/homebrew/locks/update.\r\n")
	locked.exit()
	ok := newFakeProc(0, "done\r\n")
	ok.exit()
	fs := &fakeStarter{procs: []*fakeProc{locked, ok}}
	a := newJobApp(t, fs)
	settle(t, a, a.run(action{title: "upgrade", args: []string{"upgrade"}}))
	if len(fs.calls) != 2 {
		t.Fatalf("the job should be retried after the lock error: %v", fs.calls)
	}
	j := a.queue.Current()
	if j != nil {
		t.Fatalf("job should be finished: %+v", j)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "✓ upgrade") {
		t.Errorf("status bar: %q", bar)
	}
}

func TestPasswordPromptSwitchesToInsert(t *testing.T) {
	p := newFakeProc(0, "==> Installing docker-desktop\r\nPassword:")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	a.drawer.open = false
	waiting := settle(t, a, a.run(action{title: "upgrade docker-desktop", args: []string{"upgrade", "--cask", "docker-desktop"}}))
	if a.mode != modeInsert || !a.drawer.open {
		t.Fatalf("a password prompt should open the drawer in insert mode (mode %v, open %v)", a.mode, a.drawer.open)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "INSERT") {
		t.Errorf("status bar should show INSERT: %q", bar)
	}
	for _, k := range []string{"h", "u", "n", "t", "e", "r", "q", "enter"} {
		a.Update(press(k))
	}
	if got := p.input(); got != "hunterq\r" {
		t.Errorf("typed %q, want the keys passed straight to brew (q included)", got)
	}
	if a.confirm != nil {
		t.Error("q in insert mode must not ask to quit")
	}
	a.Update(press("esc"))
	if a.mode != modeNormal {
		t.Error("esc leaves insert mode")
	}
	p.exit()
	resume(t, a, waiting)
}

func TestInsertModeEndsWithTheJob(t *testing.T) {
	p := newFakeProc(0, "Proceed? [y/N] ")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
	p.exit()
	resume(t, a, waiting)
	if a.mode != modeNormal {
		t.Error("insert mode should end when the job exits")
	}
}

func TestCtrlCAsksThenStopsTheJob(t *testing.T) {
	p := newFakeProc(0, "downloading…")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))

	a.Update(press("ctrl+c"))
	if a.confirm == nil || !strings.Contains(a.confirm.title, "Stop upgrade fd?") {
		t.Fatalf("ctrl+c should ask before stopping: %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "[y] stop it") {
		t.Errorf("the confirmation should be drawn:\n%s", out)
	}
	a.Update(press("n"))
	if p.interrupted || a.confirm != nil {
		t.Fatal("n keeps the job running")
	}
	a.Update(press("ctrl+c"))
	_, cmd := a.Update(press("y"))
	if !p.interrupted {
		t.Fatal("y should interrupt brew")
	}
	waiting = append(waiting, settle(t, a, cmd)...)
	resume(t, a, waiting)
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "stopped upgrade fd") {
		t.Errorf("status bar: %q", bar)
	}
}

func TestStoppingAJobWaitingForTheLock(t *testing.T) {
	locked := newFakeProc(1, "A `brew update` process has already locked /x.\r\n")
	locked.exit()
	fs := &fakeStarter{procs: []*fakeProc{locked}}
	a := newJobApp(t, fs)
	a.backoff = func(int) time.Duration { return time.Hour } // keep it waiting
	waiting := settle(t, a, a.run(action{title: "upgrade", args: []string{"upgrade"}}))
	if j := a.queue.Current(); j == nil || j.State != jobs.Waiting {
		t.Fatalf("job should be waiting for the lock: %+v", j)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "waiting for brew's lock") {
		t.Errorf("status bar: %q", bar)
	}
	a.Update(press("ctrl+c"))
	_, cmd := a.Update(press("y"))
	settle(t, a, cmd)
	if a.queue.Current() != nil {
		t.Error("a stopped waiting job is done")
	}
	_ = waiting // the hour-long retry timer is abandoned with the test
}

func TestQuitWhileRunningAsksAndKills(t *testing.T) {
	p := newFakeProc(0, "working")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	settle(t, a, a.run(action{title: "upgrade", args: []string{"upgrade"}}))
	_, cmd := a.Update(press("q"))
	if cmd != nil || a.confirm == nil {
		t.Fatal("q with a running job should ask first")
	}
	_, cmd = a.Update(press("y"))
	if _, ok := cmd().(tea.QuitMsg); !ok || !p.killed {
		t.Errorf("y should kill brew and quit (killed=%v)", p.killed)
	}
}

func TestDrawerKeys(t *testing.T) {
	p := newFakeProc(0)
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
	a.Update(press("esc"))
	if a.drawer.open {
		t.Fatal("esc hides the drawer")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "o output") {
		t.Errorf("a hidden drawer should be findable: %q", bar)
	}
	if h := lipgloss.Height(a.render()); h != 40 {
		t.Errorf("frame is %d lines with the drawer hidden", h)
	}
	a.Update(press("o"))
	if !a.drawer.open {
		t.Error("o shows it again")
	}
	a.Update(press("i"))
	if a.mode != modeInsert {
		t.Error("i types into the running job")
	}
	a.Update(press("esc"))
	p.exit()
	resume(t, a, waiting)
}

func TestKeyBytes(t *testing.T) {
	tests := map[string]string{"a": "a", "enter": "\r", "backspace": "\x7f", "ctrl+c": "\x03", "up": "\x1b[A"}
	for k, want := range tests {
		if got := string(keyBytes(press(k))); got != want {
			t.Errorf("keyBytes(%s) = %q, want %q", k, got, want)
		}
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/run_test.go:110:66: undefined: Proc
    internal/ui/run_test.go:133:59: too many arguments in call to New
    internal/ui/run_test.go:135:4: a.tickEvery undefined (type *App has no field or method tickEvery)
```

- [ ] **Step 5: Edit `cmd/brewery/main.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	cachePath := state.DefaultCachePath()
	snap, _ := state.Load(cachePath) // no cache yet is fine: the first refresh fills it
	client := brew.NewClient(brew.ExecRunner{Bin: bin, Env: brew.QueryEnv})

	if _, err := tea.NewProgram(ui.New(client, snap, cachePath)).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
		return 1
	}
```

   with:

```go
	cachePath := state.DefaultCachePath()
	snap, _ := state.Load(cachePath) // no cache yet is fine: the first refresh fills it
	client := brew.NewClient(brew.ExecRunner{Bin: bin, Env: brew.QueryEnv})
	start := func(args, env []string, cols, rows int) (ui.Proc, error) {
		return brew.StartPTY(bin, args, env, cols, rows)
	}

	if _, err := tea.NewProgram(ui.New(client, start, snap, cachePath)).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
		return 1
	}
```

- [ ] **Step 6: Edit `internal/ui/app.go`** (12 replacements; each old block appears exactly once)

1. Replace:

```go
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/state"
	"brewery/internal/theme"
)
```

   with:

```go
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/jobs"
	"brewery/internal/state"
	"brewery/internal/theme"
)
```

2. Replace:

```go
	tabHome: qDoctor, tabInstalled: qInstalled, tabOutdated: qOutdated, tabServices: qServices, tabTaps: qTaps,
}

// App is the root Bubble Tea model.
type App struct {
	src       Source
	cachePath string
	st        theme.Styles
	now       func() time.Time
```

   with:

```go
	tabHome: qDoctor, tabInstalled: qInstalled, tabOutdated: qOutdated, tabServices: qServices, tabTaps: qTaps,
}

// mode is how key presses are routed.
type mode int

const (
	modeNormal mode = iota // keys drive brewery
	modeInsert             // keys go to the running brew command
)

// App is the root Bubble Tea model.
type App struct {
	src       Source
	start     Starter
	cachePath string
	st        theme.Styles
	now       func() time.Time
```

3. Replace:

```go
	dirty       bool           // a query succeeded since the last cache save
	saveErr     error

	tab           int
	width, height int

	home      homeTab
	installed installedTab
	outdated  listTab
```

   with:

```go
	dirty       bool           // a query succeeded since the last cache save
	saveErr     error

	queue     jobs.Queue
	acts      map[int]action // what each job was asked to do, by job ID
	proc      Proc           // the current job's process, or nil
	canceled  bool           // the user stopped the current run
	backoff   func(attempt int) time.Duration
	tickEvery time.Duration // spinner speed; 0 turns the spinner off
	ticking   bool
	frame     int

	mode    mode
	confirm *confirmBox // open confirmation, or nil
	flash   string      // one-off status message, cleared by the next key

	tab           int
	width, height int
	tabH          int    // height left for the tab when the drawer is shown
	laidOut       [3]int // width, height and drawer state of the last layout

	drawer    drawer
	home      homeTab
	installed installedTab
	outdated  listTab
```

4. Replace:

```go
	taps      listTab
}

// New builds the App. snap is the cached snapshot (possibly empty) so the
// first frame can show data before any query returns.
func New(src Source, snap state.Snapshot, cachePath string) *App {
	st := theme.New(theme.Macchiato)
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		src: src, cachePath: cachePath, st: st, now: time.Now, ctx: ctx, cancel: cancel,
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
```

   with:

```go
	taps      listTab
}

// New builds the App. src answers queries and start runs brew commands.
// snap is the cached snapshot (possibly empty) so the first frame can show
// data before any query returns.
func New(src Source, start Starter, snap state.Snapshot, cachePath string) *App {
	st := theme.New(theme.Macchiato)
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		src: src, start: start, cachePath: cachePath, st: st, now: time.Now, ctx: ctx, cancel: cancel,
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		acts: map[int]action{}, backoff: jobs.Backoff, tickEvery: 100 * time.Millisecond,
		drawer:    newDrawer(st),
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
```

5. Replace:

```go
	return false
}

// quit stops running queries and exits.
func (a *App) quit() tea.Cmd {
	a.cancel()
	return tea.Quit
}
```

   with:

```go
	return false
}

// quit stops running queries and any running job, then exits.
func (a *App) quit() tea.Cmd {
	a.cancel()
	if a.proc != nil {
		a.proc.Kill()
	}
	return tea.Quit
}
```

6. Replace:

```go
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		bodyH := a.bodyHeight()
		a.installed.resize(a.width, bodyH)
		for _, t := range []*listTab{&a.outdated, &a.services, &a.taps} {
			t.resize(a.width, bodyH)
		}
		return a, nil

	case resultMsg:
		return a, a.handleResult(msg)

	case savedMsg:
		a.saveErr = msg.err
		return a, nil

	case tea.KeyPressMsg:
		return a, a.handleKey(msg)
	}
	return a, nil
}

// handleResult applies one query result and decides what runs next: a
```

   with:

```go
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := a.update(msg)
	a.layout()
	return a, cmd
}

func (a *App) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
	case resultMsg:
		return a.handleResult(msg)
	case savedMsg:
		a.saveErr = msg.err
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	case jobOutputMsg, jobEOFMsg, jobExitMsg, jobRetryMsg, tickMsg:
		return a.handleJob(msg)
	}
	return nil
}

// drawerShown reports whether the drawer takes space under the tab.
func (a *App) drawerShown() bool {
	return a.drawer.open && a.drawer.job != nil && drawerHeight(a.bodyHeight()) > 0
}

// layout sizes the tabs and the drawer's terminal for the current window
// and drawer state. It does nothing if neither has changed.
func (a *App) layout() {
	shown := 0
	if a.drawerShown() {
		shown = 1
	}
	if a.laidOut == [3]int{a.width, a.height, shown} {
		return
	}
	a.laidOut = [3]int{a.width, a.height, shown}
	bodyH := a.bodyHeight()
	dh := drawerHeight(bodyH)
	// The drawer's terminal is always sized for when it is shown, so a job
	// starts at the right size even while the drawer is hidden.
	a.drawer.resize(a.width-4, dh-4)
	if a.proc != nil {
		a.proc.Resize(a.drawer.cols, a.drawer.rows)
	}
	a.tabH = bodyH
	if shown == 1 {
		a.tabH -= dh
	}
	a.installed.resize(a.width, a.tabH)
	for _, t := range []*listTab{&a.outdated, &a.services, &a.taps} {
		t.resize(a.width, a.tabH)
	}
}

// handleResult applies one query result and decides what runs next: a
```

7. Replace:

```go
	return tea.Batch(again, tea.Sequence(saving, doctor))
}

func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch k := msg.String(); k {
	case "q", "ctrl+c":
		return a.quit()
	case "1", "2", "3", "4", "5":
		a.tab = int(k[0] - '1')
		return nil
```

   with:

```go
	return tea.Batch(again, tea.Sequence(saving, doctor))
}

// handleKey routes a key press by mode first: an open confirmation gets
// every key, and in insert mode keys go to brew, so typing an answer can
// never trigger a brewery shortcut such as q.
func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	a.flash = ""
	if a.confirm != nil {
		ch, ok := a.confirm.answer(k)
		if !ok {
			return nil
		}
		a.confirm = nil
		if ch.run == nil {
			return nil
		}
		return ch.run()
	}
	if a.mode == modeInsert {
		if k == "esc" {
			a.mode = modeNormal
		} else if a.proc != nil {
			a.proc.Write(keyBytes(msg))
		}
		return nil
	}

	switch k {
	case "q":
		return a.askQuit()
	case "ctrl+c":
		if a.queue.Current() != nil {
			return a.askStop()
		}
		return a.quit()
	case "esc":
		a.drawer.open = false
		return nil
	case "o":
		if a.drawer.job != nil {
			a.drawer.open = !a.drawer.open
		}
		return nil
	case "i":
		if a.proc != nil {
			a.mode, a.drawer.open = modeInsert, true
		}
		return nil
	case "1", "2", "3", "4", "5":
		a.tab = int(k[0] - '1')
		return nil
```

8. Replace:

```go
	if a.width == 0 {
		return "starting brewery…"
	}
	bodyH := a.bodyHeight()
	var body string
	switch a.tab {
	case tabHome:
```

   with:

```go
	if a.width == 0 {
		return "starting brewery…"
	}
	bodyH := a.tabH
	var body string
	switch a.tab {
	case tabHome:
```

9. Replace:

```go
	case tabTaps:
		body = a.taps.view(a.errs[qTaps])
	}
	return lipgloss.JoinVertical(lipgloss.Left, a.tabBar(), body, a.statusBar())
}

// bodyHeight is the space between the tab bar (tabs + rule) and the status bar.
```

   with:

```go
	case tabTaps:
		body = a.taps.view(a.errs[qTaps])
	}
	if a.confirm != nil {
		body = overlay(body, a.confirm.view(a.st, a.width), a.width, bodyH)
	}
	parts := []string{a.tabBar(), body}
	if a.drawerShown() {
		parts = append(parts, a.drawer.view(a.width, a.bodyHeight()-bodyH, a.now(), a.frame, a.mode == modeInsert))
	}
	return lipgloss.JoinVertical(lipgloss.Left, append(parts, a.statusBar())...)
}

// bodyHeight is the space between the tab bar (tabs + rule) and the status bar.
```

10. Replace:

```go

func (a *App) statusBar() string {
	st := a.st
	left := st.Mode("NORMAL").Render("NORMAL") + " " + tabNames[a.tab]
	if a.tab == tabInstalled {
		if p, ok := a.installed.selected(); ok {
			left += " › " + p.Name
```

   with:

```go

func (a *App) statusBar() string {
	st := a.st
	badge := "NORMAL"
	switch {
	case a.mode == modeInsert:
		badge = "INSERT"
	case a.queue.Current() != nil:
		badge = "RUNNING"
	}
	left := st.Mode(badge).Render(badge) + " " + tabNames[a.tab]
	if a.tab == tabInstalled {
		if p, ok := a.installed.selected(); ok {
			left += " › " + p.Name
```

11. Replace:

```go
	}

	var right []string
	var failed []string
	for q, name := range queryNames {
		if a.errs[query(q)] != nil {
```

   with:

```go
	}

	var right []string
	if a.flash != "" {
		right = append(right, a.flash)
	}
	if j := a.queue.Current(); j != nil {
		job := spinnerFrames[a.frame%len(spinnerFrames)] + " " + j.Title
		if j.State == jobs.Waiting {
			job = "⏸ " + j.Title + " · waiting for brew's lock"
		}
		if n := a.queue.Queued(); n > 0 {
			job += fmt.Sprintf(" +%d queued", n)
		}
		right = append(right, st.Info.Render(job))
	}
	var failed []string
	for q, name := range queryNames {
		if a.errs[query(q)] != nil {
```

12. Replace:

```go

	// Keep to one line: drop the key hints first, then shorten the left side.
	room := a.width - 1 // trailing space
	hints := st.Muted.Render("1-5 tabs · r refresh · q quit")
	if r != "" {
		hints = r + "  " + hints
	}
```

   with:

```go

	// Keep to one line: drop the key hints first, then shorten the left side.
	room := a.width - 1 // trailing space
	hints := "1-5 tabs · r refresh · q quit"
	if a.drawer.job != nil && !a.drawer.open {
		hints = "o output · " + hints
	}
	hints = st.Muted.Render(hints)
	if r != "" {
		hints = r + "  " + hints
	}
```

- [ ] **Step 7: Create `internal/ui/run.go`**

```go
package ui

import (
	"fmt"
	"io"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/jobs"
)

// Proc is a running brew command. *brew.Proc satisfies it; tests use a fake.
type Proc interface {
	io.Reader // output; io.EOF once the command has exited
	io.Writer // keystrokes
	Resize(cols, rows int) error
	Interrupt() error
	Kill() error
	Wait() (int, error)
}

// Starter starts brew with args in a pseudo-terminal of cols×rows, adding
// env to its environment.
type Starter func(args, env []string, cols, rows int) (Proc, error)

// Messages from a running job. id and attempt name the run they belong to,
// so a late message from an earlier run is ignored.
type (
	jobOutputMsg struct {
		id, attempt int
		data        []byte
	}
	jobEOFMsg   struct{ id, attempt int }
	jobExitMsg  struct{ id, attempt, code int }
	jobRetryMsg struct{ id int }
	tickMsg     struct{}
)

// readOutput reads the next chunk of a job's output.
func readOutput(p Proc, id, attempt int) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 32*1024)
		n, err := p.Read(buf)
		if n > 0 || err == nil {
			return jobOutputMsg{id, attempt, buf[:n]}
		}
		return jobEOFMsg{id, attempt}
	}
}

// waitExit waits for a job's process to exit once its output is drained.
func waitExit(p Proc, id, attempt int) tea.Cmd {
	return func() tea.Msg {
		code, err := p.Wait()
		if err != nil {
			code = -1
		}
		return jobExitMsg{id, attempt, code}
	}
}

// run queues act and starts it unless another job is running.
func (a *App) run(act action) tea.Cmd {
	j := a.queue.Add(act.title, act.args)
	a.acts[j.ID] = act
	if cur := a.queue.Current(); cur != nil {
		a.flash = a.st.Info.Render(fmt.Sprintf("queued %s · runs after %s", act.title, cur.Title))
		return nil
	}
	return a.runNext()
}

// runNext starts the oldest queued job, if nothing is running.
func (a *App) runNext() tea.Cmd {
	j := a.queue.Next(a.now())
	if j == nil {
		return nil
	}
	return a.launch(j)
}

// launch starts a run of j and shows it in the drawer.
func (a *App) launch(j *jobs.Job) tea.Cmd {
	p, err := a.start(j.Args, brew.JobEnv, a.drawer.cols, a.drawer.rows)
	if err != nil {
		a.queue.Finish(j, -1, false, err.Error(), a.now())
		a.flash = a.st.Err.Render(fmt.Sprintf("✗ %s: couldn't start brew: %v", j.Title, err))
		return a.runNext()
	}
	a.proc, a.canceled = p, false
	a.drawer.start(j, p)
	return tea.Batch(readOutput(p, j.ID, j.Attempts), a.tick())
}

// tick animates the spinner while a job is current.
func (a *App) tick() tea.Cmd {
	if a.ticking || a.tickEvery == 0 {
		return nil
	}
	a.ticking = true
	return tea.Tick(a.tickEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

// onScreen reports whether a message belongs to the run being shown.
func (a *App) onScreen(id, attempt int) bool {
	j := a.queue.Current()
	return a.proc != nil && j != nil && j.ID == id && j.Attempts == attempt
}

// handleJob reacts to a job message and returns what to do next.
func (a *App) handleJob(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case jobOutputMsg:
		if !a.onScreen(m.id, m.attempt) {
			return nil
		}
		a.drawer.write(m.data)
		if a.mode != modeInsert && a.drawer.prompt() {
			a.mode, a.drawer.open = modeInsert, true // brew is asking: let the user answer
		}
		return readOutput(a.proc, m.id, m.attempt)
	case jobEOFMsg:
		if !a.onScreen(m.id, m.attempt) {
			return nil
		}
		return waitExit(a.proc, m.id, m.attempt)
	case jobExitMsg:
		if !a.onScreen(m.id, m.attempt) {
			return nil
		}
		return a.finish(a.queue.Current(), m.code)
	case jobRetryMsg:
		j := a.queue.Current()
		if j == nil || j.ID != m.id || j.State != jobs.Waiting {
			return nil // stopped while waiting
		}
		a.queue.Retry(j)
		return a.launch(j)
	case tickMsg:
		a.ticking = false
		a.frame++
		if a.queue.Current() != nil {
			return a.tick()
		}
	}
	return nil
}

// finish records how a run ended. A run that hit brew's lock is retried
// after a backoff; anything else refreshes what the job changed (even a
// failure may have changed something) and starts the next job.
func (a *App) finish(j *jobs.Job, code int) tea.Cmd {
	a.drawer.stop()
	a.proc = nil
	if a.mode == modeInsert {
		a.mode = modeNormal
	}
	if a.queue.Finish(j, code, a.canceled, a.drawer.output(), a.now()) {
		wait := a.backoff(j.Attempts)
		a.drawer.retryAt = a.now().Add(wait)
		return tea.Tick(wait, func(time.Time) tea.Msg { return jobRetryMsg{j.ID} })
	}
	return tea.Batch(a.afterJob(j), a.runNext())
}

// afterJob reports a finished job and refreshes what it touched.
func (a *App) afterJob(j *jobs.Job) tea.Cmd {
	switch j.State {
	case jobs.Done:
		a.flash = a.st.OK.Render("✓ " + j.Title)
	case jobs.Canceled:
		a.flash = a.st.Warn.Render("stopped " + j.Title)
	default:
		a.flash = a.st.Err.Render(fmt.Sprintf("✗ %s failed (exit %d) · o shows output", j.Title, j.ExitCode))
	}
	act := a.acts[j.ID]
	if act.full {
		return a.refresh()
	}
	return a.request(act.refresh...)
}

// stopJob interrupts the current job, as ctrl+c in a terminal would. A job
// waiting for brew's lock is simply dropped.
func (a *App) stopJob() tea.Cmd {
	j := a.queue.Current()
	if j == nil {
		return nil
	}
	a.canceled = true
	if a.proc != nil {
		a.proc.Interrupt() // the exit message follows
		return nil
	}
	a.queue.Finish(j, -1, true, "", a.now())
	return tea.Batch(a.afterJob(j), a.runNext())
}

// askStop confirms before stopping the current job.
func (a *App) askStop() tea.Cmd {
	j := a.queue.Current()
	if j == nil {
		return nil
	}
	a.confirm = &confirmBox{
		title: "Stop " + j.Title + "?",
		body:  []string{"brew gets ctrl+c and may leave this half done."},
		choices: []choice{
			{"y", "stop it", a.stopJob},
			{"n", "keep running", nil},
		},
	}
	return nil
}

// askQuit confirms before quitting while a job is running.
func (a *App) askQuit() tea.Cmd {
	if a.queue.Current() == nil {
		return a.quit()
	}
	a.confirm = &confirmBox{
		title: "Quit brewery?",
		body:  []string{a.queue.Current().Title + " is still running and will be stopped."},
		choices: []choice{
			{"y", "quit", a.quit},
			{"n", "stay", nil},
		},
	}
	return nil
}

// keyBytes is what a terminal would send for a key press.
func keyBytes(msg tea.KeyPressMsg) []byte {
	switch k := msg.String(); k {
	case "enter":
		return []byte("\r")
	case "backspace":
		return []byte{0x7f}
	case "tab":
		return []byte("\t")
	case "up":
		return []byte("\x1b[A")
	case "down":
		return []byte("\x1b[B")
	case "right":
		return []byte("\x1b[C")
	case "left":
		return []byte("\x1b[D")
	default:
		if len(k) == 6 && k[:5] == "ctrl+" && k[5] >= 'a' && k[5] <= 'z' {
			return []byte{k[5] - 'a' + 1} // ctrl+c is 0x03, and so on
		}
		return []byte(msg.Text)
	}
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(ui): run brew jobs with a live output drawer"
```

---

### Task 8: Action keys

**Files:**
- Modify: `internal/ui/actions.go`
- Create: `internal/ui/actions_test.go`
- Modify: `internal/ui/app.go`
- Create: `internal/ui/live_test.go`

**Interfaces:**
- **Consumes:** `App.run`, `confirmBox`, `Index.Orphans`/`UsedBy`, `listTab.cursor`, `installedTab.selected`.
- **Produces:**
  - `kindArgs(verb, kind, names...) []string`, which adds `--cask` for casks.
  - `packageQueries`.
  - The `App` methods `actionKey(k) (tea.Cmd, bool)`, `upgradeSelected`, `askUninstall`, `togglePin`, `askUpgradeAll`, `serviceKey`, `note`, `actionHints`.

Keys:
- **`u` (Installed or Outdated):** upgrade. If the package is up to date or pinned, it says so in the status bar instead.
- **`x` (Installed):**
  - If something still needs the package, the dialog explains why brew would refuse, and the only choice is to close.
  - Otherwise it asks before uninstalling, and offers `[o] uninstall + orphans` when there are orphans.
- **`p` (Installed):** pin or unpin. Casks use `--cask`.
- **`A` (Outdated):** asks, listing the packages and how many pinned ones will be skipped.
- **`U` (anywhere):** `brew update`, then a full refresh.
- **Services:** `s` start, `R` restart, `S` stop (after asking).

The status-bar hints follow the tab. The live test (`-tags live`) runs `brew --version` as a real job through the real pseudo-terminal runner.

- [ ] **Step 1: Create `internal/ui/actions_test.go`**

```go
package ui

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// keys presses each key in turn and settles the commands they start.
func keys(t *testing.T, a *App, ks ...string) {
	t.Helper()
	for _, k := range ks {
		_, cmd := a.Update(press(k))
		settle(t, a, cmd)
	}
}

// lastCall is the most recent brew command started, or nil.
func lastCall(fs *fakeStarter) []string {
	if len(fs.calls) == 0 {
		return nil
	}
	return fs.calls[len(fs.calls)-1]
}

func TestUpgradeFromInstalled(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "u") // fd, up to date
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "fd is up to date") {
		t.Errorf("u on an up-to-date package should just say so: %v %q", fs.calls, stripANSI(a.statusBar()))
	}
	keys(t, a, "j", "j", "u") // lua-language-server, outdated
	if want := []string{"upgrade", "lua-language-server"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUpgradeSkipsPinned(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	for i := range a.snap.Packages {
		if a.snap.Packages[i].Name == "lua-language-server" {
			a.snap.Packages[i].Pinned = true
		}
	}
	a.snap.Outdated[0].Pinned = true
	a.sync()
	keys(t, a, "2", "j", "j", "u")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "lua-language-server is pinned · p unpins it") {
		t.Errorf("u on a pinned package should say so: %v %q", fs.calls, stripANSI(a.statusBar()))
	}
	keys(t, a, "3", "u")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "is pinned · unpin it on Installed") {
		t.Errorf("u on a pinned outdated row should say so: %v", fs.calls)
	}
}

func TestUpgradeCaskFromOutdated(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "3", "j", "u")
	if want := []string{"upgrade", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUpgradeAllAsksFirst(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "3", "A")
	if a.confirm == nil || a.confirm.title != "Upgrade all 2 outdated?" {
		t.Fatalf("A should ask: %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "lua-language-server, docker-desktop") {
		t.Errorf("the confirmation should list what gets upgraded:\n%s", out)
	}
	keys(t, a, "y")
	if want := []string{"upgrade"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUninstallOffersOrphans(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "j", "x") // git: leaves pcre2 and json-c behind
	if a.confirm == nil || a.confirm.title != "Uninstall git 2.51.0?" {
		t.Fatalf("x should ask: %+v", a.confirm)
	}
	out := stripANSI(a.render())
	for _, want := range []string{"That leaves 2 dependencies nothing else needs:", "json-c, pcre2", "[o] uninstall + orphans"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	keys(t, a, "o")
	if want := []string{"uninstall", "git", "json-c", "pcre2"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUninstallWithoutOrphans(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "x")
	if len(a.confirm.choices) != 2 {
		t.Errorf("fd has no orphans: offer only uninstall and cancel, got %+v", a.confirm.choices)
	}
	keys(t, a, "n")
	if len(fs.calls) != 0 || a.confirm != nil {
		t.Fatal("n cancels")
	}
	keys(t, a, "x", "y")
	if want := []string{"uninstall", "fd"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUninstallRefusesWhatOthersNeed(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "f", "f", "f", "g", "x") // deps filter, top row: json-c, used by git and pcre2
	if a.confirm == nil || a.confirm.title != "Can't uninstall json-c" {
		t.Fatalf("want an explanation, got %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Required by git, pcre2.") {
		t.Errorf("should say what needs it:\n%s", out)
	}
	keys(t, a, "y") // not a choice here
	if len(fs.calls) != 0 {
		t.Errorf("nothing may run: %v", fs.calls)
	}
	keys(t, a, "esc")
	if a.confirm != nil {
		t.Error("esc closes the explanation")
	}
}

func TestPinToggles(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "p")
	if want := []string{"pin", "fd"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	keys(t, a, "j", "p")
	if want := []string{"unpin", "git"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	keys(t, a, "f", "f", "g", "p") // casks: docker-desktop
	if want := []string{"pin", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestServiceKeys(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "4", "s")
	if want := []string{"services", "start", "postgresql@16"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	keys(t, a, "j", "R")
	if want := []string{"services", "restart", "redis"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	keys(t, a, "S")
	if a.confirm == nil || a.confirm.title != "Stop redis?" {
		t.Fatalf("S should ask: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"services", "stop", "redis"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUpdateRefreshesEverything(t *testing.T) {
	src := newCountingSource(t)
	fs := &fakeStarter{}
	a := New(src, fs.start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	keys(t, a, "U")
	if want := []string{"update"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	for _, q := range []string{"installed", "outdated", "services", "taps", "doctor"} {
		if src.calls[q] != 1 {
			t.Errorf("after brew update, %s ran %d times, want 1", q, src.calls[q])
		}
	}
}

func TestActionKeysOnlyWorkOnTheirTab(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "3", "x", "p", "4", "u", "A", "5", "s")
	if len(fs.calls) != 0 || a.confirm != nil {
		t.Errorf("keys off their tab must do nothing: %v %+v", fs.calls, a.confirm)
	}
}

func TestStatusHintsFollowTheTab(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	for _, tt := range []struct{ key, want string }{
		{"2", "u upgrade · x uninstall · p pin · f filter"},
		{"3", "u upgrade · A upgrade all"},
		{"4", "s start · S stop · R restart"},
		{"5", "U update"},
	} {
		keys(t, a, tt.key)
		if bar := stripANSI(a.statusBar()); !strings.Contains(bar, tt.want) {
			t.Errorf("tab %s hints: %q", tt.key, bar)
		}
	}
}
```

- [ ] **Step 2: Create `internal/ui/live_test.go`**

```go
//go:build live

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

// Runs a harmless brew command as a job through the real pseudo-terminal
// runner and checks it shows up in the drawer.
// Run with: go test -tags live ./internal/ui/
func TestLiveJob(t *testing.T) {
	bin, err := brew.Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	start := func(args, env []string, cols, rows int) (Proc, error) {
		return brew.StartPTY(bin, args, env, cols, rows)
	}
	a := New(newFakeSource(t), start, state.Snapshot{}, filepath.Join(t.TempDir(), "s.json"))
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	waiting := settle(t, a, a.run(action{title: "brew --version", args: []string{"--version"}}))
	for deadline := time.Now().Add(10 * time.Second); len(waiting) > 0; {
		if time.Now().After(deadline) {
			t.Fatal("brew --version did not finish")
		}
		waiting = resume(t, a, waiting)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "Homebrew") || !strings.Contains(out, "✓ brew --version · exit 0") {
		t.Errorf("drawer after brew --version:\n%s", out)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    --- FAIL: TestUpgradeFromInstalled (0.00s)
    --- FAIL: TestUpgradeSkipsPinned (0.00s)
    --- FAIL: TestUpgradeCaskFromOutdated (0.00s)
```

- [ ] **Step 4: Replace the whole of `internal/ui/actions.go` with**

```go
package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

// action is something the user asked brew to do from a pane.
type action struct {
	title   string   // shown in the drawer and status bar, e.g. "upgrade fd"
	args    []string // brew's arguments
	refresh []query  // queries to re-run when it finishes
	full    bool     // re-run every query, doctor included (after brew update)
}

// kindArgs builds brew arguments for verb on packages of one kind.
func kindArgs(verb string, kind brew.Kind, names ...string) []string {
	args := []string{verb}
	if kind == brew.KindCask {
		args = append(args, "--cask")
	}
	return append(args, names...)
}

// packageQueries are re-run after anything that installs, removes or pins.
var packageQueries = []query{qInstalled, qOutdated}

// actionKey handles the keys that ask brew to do something. ok is false
// when k is not an action on the current tab.
func (a *App) actionKey(k string) (cmd tea.Cmd, ok bool) {
	switch {
	case k == "U":
		return a.run(action{title: "update", args: []string{"update"}, full: true}), true
	case k == "u" && (a.tab == tabInstalled || a.tab == tabOutdated):
		return a.upgradeSelected(), true
	case k == "x" && a.tab == tabInstalled:
		return a.askUninstall(), true
	case k == "p" && a.tab == tabInstalled:
		return a.togglePin(), true
	case k == "A" && a.tab == tabOutdated:
		return a.askUpgradeAll(), true
	case a.tab == tabServices && (k == "s" || k == "S" || k == "R"):
		return a.serviceKey(k), true
	}
	return nil, false
}

// note shows a one-off hint in the status bar.
func (a *App) note(msg string) tea.Cmd {
	a.flash = a.st.Warn.Render(msg)
	return nil
}

// upgradeSelected upgrades the package under the cursor.
func (a *App) upgradeSelected() tea.Cmd {
	var name string
	var kind brew.Kind
	switch a.tab {
	case tabInstalled:
		p, ok := a.installed.selected()
		if !ok {
			return nil
		}
		if !p.Outdated {
			return a.note(p.Name + " is up to date")
		}
		if p.Pinned {
			return a.note(p.Name + " is pinned · p unpins it")
		}
		name, kind = p.FullName, p.Kind
	case tabOutdated:
		i := a.outdated.cursor()
		if i < 0 {
			return nil
		}
		o := a.snap.Outdated[i]
		if o.Pinned {
			return a.note(o.Name + " is pinned · unpin it on Installed")
		}
		name, kind = o.Name, o.Kind
	}
	return a.run(action{title: "upgrade " + name, args: kindArgs("upgrade", kind, name), refresh: packageQueries})
}

// askUninstall confirms uninstalling the selected package. If something
// still needs it, it explains why brew would refuse instead. If it leaves
// dependencies nothing else needs, it offers to remove those too.
func (a *App) askUninstall() tea.Cmd {
	p, ok := a.installed.selected()
	if !ok {
		return nil
	}
	if used := a.ix.UsedBy(p.FullName); len(used) > 0 {
		a.confirm = &confirmBox{
			title:   "Can't uninstall " + p.Name,
			body:    []string{"Required by " + strings.Join(used, ", ") + ".", "Uninstall those first, or brew will refuse."},
			choices: []choice{{"n", "close", nil}},
		}
		return nil
	}
	refresh := append(slices.Clone(packageQueries), qServices) // its service goes too
	one := action{title: "uninstall " + p.Name, args: kindArgs("uninstall", p.Kind, p.FullName), refresh: refresh}
	box := &confirmBox{
		title:   fmt.Sprintf("Uninstall %s %s?", p.Name, p.Version),
		choices: []choice{{"y", "uninstall", func() tea.Cmd { return a.run(one) }}},
	}
	if orphans := a.ix.Orphans(p.FullName); len(orphans) > 0 {
		noun := "dependency"
		if len(orphans) > 1 {
			noun = "dependencies"
		}
		box.body = []string{fmt.Sprintf("That leaves %d %s nothing else needs:", len(orphans), noun), strings.Join(orphans, ", ")}
		all := action{
			title:   fmt.Sprintf("uninstall %s + %d orphaned", p.Name, len(orphans)),
			args:    append(kindArgs("uninstall", p.Kind, p.FullName), orphans...),
			refresh: refresh,
		}
		box.choices = append(box.choices, choice{"o", "uninstall + orphans", func() tea.Cmd { return a.run(all) }})
	}
	box.choices = append(box.choices, choice{"n", "cancel", nil})
	a.confirm = box
	return nil
}

// togglePin pins the selected package, or unpins it if it is pinned.
func (a *App) togglePin() tea.Cmd {
	p, ok := a.installed.selected()
	if !ok {
		return nil
	}
	verb := "pin"
	if p.Pinned {
		verb = "unpin"
	}
	return a.run(action{title: verb + " " + p.Name, args: kindArgs(verb, p.Kind, p.FullName), refresh: packageQueries})
}

// askUpgradeAll confirms upgrading every outdated package.
func (a *App) askUpgradeAll() tea.Cmd {
	n := len(a.snap.Outdated)
	if n == 0 {
		return a.note("everything is up to date")
	}
	names := make([]string, 0, n)
	pinned := 0
	for _, o := range a.snap.Outdated {
		names = append(names, o.Name)
		if o.Pinned {
			pinned++
		}
	}
	body := []string{strings.Join(names, ", ")}
	if pinned > 0 {
		body = append(body, fmt.Sprintf("%d pinned will be skipped.", pinned))
	}
	all := action{title: "upgrade all", args: []string{"upgrade"}, refresh: packageQueries}
	a.confirm = &confirmBox{
		title: fmt.Sprintf("Upgrade all %d outdated?", n),
		body:  body,
		choices: []choice{
			{"y", "upgrade all", func() tea.Cmd { return a.run(all) }},
			{"n", "cancel", nil},
		},
	}
	return nil
}

// serviceKey starts (s), stops (S, after asking) or restarts (R) the
// selected service.
func (a *App) serviceKey(k string) tea.Cmd {
	i := a.services.cursor()
	if i < 0 {
		return nil
	}
	name := a.snap.Services[i].Name
	verb := map[string]string{"s": "start", "S": "stop", "R": "restart"}[k]
	act := action{title: verb + " " + name, args: []string{"services", verb, name}, refresh: []query{qServices}}
	if verb != "stop" {
		return a.run(act)
	}
	a.confirm = &confirmBox{
		title: "Stop " + name + "?",
		body:  []string{"It won't start at login again until you start it."},
		choices: []choice{
			{"y", "stop", func() tea.Cmd { return a.run(act) }},
			{"n", "cancel", nil},
		},
	}
	return nil
}

// actionHints are the status-bar hints for the current tab's actions.
func (a *App) actionHints() string {
	switch a.tab {
	case tabInstalled:
		return "u upgrade · x uninstall · p pin · f filter"
	case tabOutdated:
		return "u upgrade · A upgrade all"
	case tabServices:
		return "s start · S stop · R restart"
	}
	return "1-5 tabs · r refresh · U update"
}
```

- [ ] **Step 5: Edit `internal/ui/app.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	case "r":
		return a.refresh()
	}

	switch a.tab {
	case tabHome:
```

   with:

```go
	case "r":
		return a.refresh()
	}
	if cmd, ok := a.actionKey(k); ok {
		return cmd
	}

	switch a.tab {
	case tabHome:
```

2. Replace:

```go

	// Keep to one line: drop the key hints first, then shorten the left side.
	room := a.width - 1 // trailing space
	hints := "1-5 tabs · r refresh · q quit"
	if a.drawer.job != nil && !a.drawer.open {
		hints = "o output · " + hints
	}
```

   with:

```go

	// Keep to one line: drop the key hints first, then shorten the left side.
	room := a.width - 1 // trailing space
	hints := a.actionHints() + " · q quit"
	if a.drawer.job != nil && !a.drawer.open {
		hints = "o output · " + hints
	}
```

- [ ] **Step 6: Run the live tests**

```bash
go test -tags live -count=1 ./internal/brew/ ./internal/ui/   # expect ok for both
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(ui): add upgrade, uninstall, pin, update and service actions"
```

---

### Task 9: End-to-end check in a real terminal

**Files:** none (verification only)

- [ ] **Step 1: Build and run**

Run: `make run`

- [ ] **Step 2: Check each of these by hand.** None of these change anything; every dialog is cancelled.
  1. **Uninstall:** press `2`, then `x` on any leaf. A peach-bordered dialog floats over the list: "Uninstall … ?" with `[y] uninstall   [n] cancel`, and an `[o]` choice if the package has orphans. Press `n`.
  2. **Upgrade all:** press `3`, then `A`. "Upgrade all N outdated?" lists the packages. Press `esc`.
  3. **Stop a service:** press `4`, then `S`. "Stop …?" appears. Press `n`.
  4. **Hints:** the status-bar hints change per tab.
  5. **Quit:** press `q`. brewery quits at once, because no job is running.

- [ ] **Step 3 (optional; this changes your system): run one real job**

On a package you don't mind upgrading, press `u` on its Outdated row. The drawer opens and shows brew's live output with a spinner and a timer. When it finishes, the drawer shows `✓ … exit 0`, the status bar flashes `✓ upgrade …`, and the Outdated tab refreshes. Press `esc` to close the drawer.

If it is a cask that asks for your password, the drawer switches to INSERT on its own. Type the password and press Enter.

- [ ] **Step 4: Final checks**

```bash
go test -race -count=1 ./...
go vet ./...
gofmt -l .
go mod tidy -diff
```

Expected: all `ok`, with no output from vet, gofmt, or `go mod tidy -diff`.
