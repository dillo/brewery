# brewery Plan 5: Keys and robustness — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make brewery dependable, and give it the everyday keys the spec promised.
- **Dependable:** quitting gives a running job ctrl+c before killing it; lock retries trigger only on brew's own error; the drawer gets a useful size, and nothing spills past a small window; a crash leaves a log.
- **Everyday keys:** `/` filters any list, `?` shows the keys, `gg`/`gx`/`y` go to the top, open the homepage and copy the name, `tab`/`enter` focus a scrollable preview, and `space` marks several packages for `u` and `x`.

**Architecture:**
- **`internal/jobs`:** the lock check matches brew's error line.
- **`internal/state` and `internal/jobs`:** state paths never become relative. `Index.Orphans` takes several packages.
- **`internal/ui`:**
  - Quitting waits for the job.
  - The drawer has a new size rule, and every frame is clipped to the window.
  - A FILTER mode, a `g` prefix, marks, a preview scroller, and a help overlay.
- **`main`:** wraps the TUI in a crash log, and hands the UI two new builders: open a web page (`open`) and copy text (the clipboard).
- **The UI still never starts a program itself.**

**Tech Stack:**
- Go ≥ 1.25
- Bubble Tea v2, Bubbles v2, Lip Gloss v2
- `github.com/atotto/clipboard` v0.1.4, already in go.mod as Bubbles' dependency, becomes a direct one.

**Spec:** `docs/superpowers/specs/2026-10-12-brewery-design.md`:
- section 4: modes (FILTER), global and list keys, and the Home and Installed behaviour
- section 5: the crash log
- the carried-over items listed under "Plan 5" in Plans 2–4

**Roadmap (decided with the user):** the old "Polish and ship" outline is split three ways, and the user tests between plans.
- **This plan, "Keys and robustness."**
- **Plan 6, "Looks and size":**
  - layout: the responsive layout (the preview stacks under 100 columns and hides under 60), column shrink order (VERSION before NAME), skeleton rows on a first run, and a "…" when the status bar is cut
  - display: Nerd Font icons with an ASCII fallback, colored service status, disk sizes for visible rows, and how old the Outdated data is
  - quality: teatest snapshots, golangci-lint and GitHub Actions
- **Plan 7, "Release":** goreleaser, the tap, and renaming the module path. It needs the GitHub user or organization.

**Decided with the user:**
- **`gx` opens the homepage** (Neovim's "open the link under the cursor"). `o` keeps showing and hiding the job's output. `g` becomes a prefix: `gg` is the top, as the spec asks, and `G` stays the bottom.
- **No `Ctrl-P` palette.** `:` with its completion popup covers brew's commands, and `?` lists brewery's keys. The spec is updated.

**Found while building this:**
- **Bubble Tea already restores the terminal on a panic,** then prints the panic and its stack to `os.Stderr`. So the crash log points `os.Stderr` at a file while the TUI runs. That also catches panics in commands running in the background. Full-screen programs (editors, shells, `less`) are given the real stderr. Nothing else writes to stderr while the TUI is up, since it would garble the screen.
- **brew prints its lock error as ``Error: A `brew …` process has already locked <path>.``** (`exceptions.rb`), with "Error" colored in a terminal. The check strips colors and anchors on that line.
- **The drawer had only 4 output rows at 80×24,** and Home alone overflowed windows under 17 rows.

**Verified:** every code block in this plan was compiled and tested before the plan was written.
- **The edits are exact.** A checker applied this plan's edits to `master` (ecfe0ce) and confirmed that:
  - each task reproduces the tested code byte for byte;
  - each task's tests fail before its code and pass after;
  - `go mod tidy` stays clean once Task 7's tidy step runs.
  Every task's commit passes `go test -race` on its own.
- **Against the real binary,** with history and cache in a scratch directory:
  - `/git` on Installed narrowed the list; `enter` kept the filter and `esc` cleared it.
  - Two marks showed `●` and "2 marked".
  - `?` fitted in two columns, and `gg` went to the top of Outdated.
  - A stand-in brew command that traps ctrl+c showed the gentle quit: `q`, `y` sent ctrl+c, the command cleaned up and exited 130, and its job was saved as stopped before brewery exited 0.
  - A throwaway build that panics on a key restored the terminal, wrote the panic to `crash.log`, and printed where it is.

## Global Constraints

These come from Plans 1–4 and still hold:
- Module path `brewery`; Go ≥ 1.25.
- **Only `internal/brew` and `main` run brew.** `internal/ui` gets data through `Source` and starts jobs through `Starter`. Its other builders come from `main` (history, completions, takeover, log viewer, and now web pages and the clipboard).
- Views use `theme.Styles` only.
- One job at a time; lock retries (5 attempts, backoff 2s, 4s, 8s, 16s, 30s); pane actions get `brew.JobEnv`.
- The emulator is never closed, and its replies are always drained.
- Refreshes are never dropped; the cache is saved before doctor runs.
- Full-screen programs wait while a job runs.

Added or changed by this plan:
- **Key routing is by mode first:**
  - an open confirmation gets every key, and so does the `?` overlay, where any key closes it and does nothing else;
  - in INSERT every key but `esc` goes to brew;
  - in COMMAND every key edits the `:` line, and in FILTER every key edits the `/` line;
  - after `g`, the next key completes `gg` or `gx`, or else does what it always does;
  - otherwise the jobs view's keys, then the global keys, then the tab's.
- **Brew asking a question while a line is being typed** (COMMAND or FILTER) waits until the line closes, then switches to INSERT.
- **Quitting with a job running** (after the user confirms):
  - brewery sends the job ctrl+c (`Interrupt`), says `stopping <job>… · ctrl+c quits now`, and ignores every other key;
  - when the job exits, it is recorded (as stopped unless it exited 0) and brewery quits without starting queued jobs;
  - after 3 seconds (`quitGrace`), or a second ctrl+c, the job is killed and brewery quits.
- **Lock retries** only follow a line that starts ``Error: A `…` process has already locked``, after colors are stripped.
- **Drawer height** for a body of `bodyH` lines: `min(max(bodyH/2, 10), 16, bodyH-6)`. If that's under 5, it is `min(5, bodyH)` instead. At 80×24 that gives 6 lines of output; at 120×40, 12. The tab body and then the whole frame are clipped to the window, so no frame is ever taller than it.
- **Crash log:** while the TUI runs, `os.Stderr` is a temp file next to `$XDG_STATE_HOME/brewery/crash.log` (default `~/.local/state/brewery/`). If anything was written, it becomes `crash.log`, and brewery prints `brewery crashed. What happened is in <path>; please include it when reporting this.` and exits 1. Otherwise the file is removed.
- **State paths:** without a home directory, the cache and state go under the temp directory, never a relative path. A snapshot save that fails removes its temp file.
- **`/` filter (FILTER mode, teal):**
  - It narrows the list you're on: Installed (within its view), Outdated, Services, Taps, or the jobs view (by command).
  - Rows match when their name contains the typed text, ignoring case, and the list follows as you type.
  - `enter` keeps the filter and `esc` clears it; `/` again starts from the kept text.
  - Each list keeps its own filter. The pane title shows `· /text`; an empty result says "nothing matches /text".
  - On Home, a note says "/ filters a list · Home has none".
- **Rows by identity:** an action on a filtered list acts on the row shown. `listTab.cursor()` maps the shown row back to its index in the snapshot.
- **`gg` / `gx` / `y`:**
  - `gg` goes to the top of the list (or of a focused preview).
  - `gx` opens the selected item's web page: a package's homepage on Installed, Outdated and Services (a service's formula), or a tap's `https://` remote without `.git`. With none, it says "no web page for <name>".
  - `y` copies the name, or in the jobs view the command (`brew …`), and flashes `copied <text>`.
  - Errors from either show in the status bar.
- **Installed preview focus:**
  - `tab` or `enter` focuses the preview, which then takes the focused border. `j`/`k`, `ctrl+d`/`ctrl+u`, `gg`/`G` (and home/end) scroll it.
  - `tab` or `esc` goes back to the list; `f`/`F` and the action keys still work.
  - A different selection starts its preview at the top.
- **Marks (`space`) on Installed and Outdated:**
  - `space` marks or unmarks the row and moves down. Marked rows show `● ` before the name, the status bar says `N marked`, and `esc` clears them (unless the drawer is showing).
  - With marks, `u` upgrades the marked packages that are outdated and unpinned, as one job per kind (formulae, then casks with `--cask`), and clears the marks.
  - With marks, `x` on Installed refuses with "Can't uninstall these" when something unmarked needs one. Otherwise it asks "Uninstall N packages?", with `[o]` for the orphans of the whole set.
- **`?` help:** an overlay of the keys for where you are: this tab (or the jobs view), lists, and everywhere. It uses two columns when one doesn't fit. The status bar hints end `: command · ? help · q quit`.

## File Structure

```
internal/jobs/jobs.go           lockRe; IsLockError strips colors and anchors on brew's line
internal/jobs/history.go        DefaultStateDir (temp dir without a home)
internal/state/snapshot.go      DefaultCachePath without a home; Save removes its temp file on failure
internal/state/index.go         Orphans(fullNames ...string)
internal/ui/run.go              quitNowMsg, quitGrace; finish while quitting
internal/ui/app.go              quit/quitNow; clipLines; modeFilter; g prefix; help; marks in esc and the status bar
internal/ui/drawer.go           drawerHeight
internal/ui/filter.go           filterable, matches, FILTER mode
internal/ui/list.go             all/idx/match/matchCol/marked; cursor() maps shown rows back
internal/ui/installed.go        match, pv (preview scroller), marked
internal/ui/links.go            gKey, selection, openHomepage, copyName, UseOpener, UseClipboard
internal/ui/scroll.go           scroller
internal/ui/marks.go            marks, upgradeMarked, askUninstallMarked, byKind
internal/ui/help.go             helpGroups, helpView
internal/ui/services.go         wantLog forgets the log on a service without one
cmd/brewery/crash.go            crashLog
cmd/brewery/main.go             crash log around the TUI; real stderr for full-screen programs; open and clipboard
```

---

### Task 1: Retry only on brew's own lock error

**Files:**
- Modify: `internal/jobs/jobs.go`
- Modify: `internal/jobs/jobs_test.go`
- Modify: `internal/ui/drawer_test.go`
- Modify: `internal/ui/run_test.go`

**Interfaces:**
- **Consumes:** `jobs.IsLockError` (used by `Queue.Finish`).
- **Produces:** `lockRe`. `IsLockError(output)` is true only when, after `ansi.Strip`, a line starts with ``Error: A `…` process has already locked ``.

Lock errors in four existing tests gain the `Error: ` prefix that brew really prints.

- [ ] **Step 1: Edit `internal/jobs/jobs_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	j := q.Add("upgrade", []string{"upgrade"})
	other := q.Add("pin git", nil)
	q.Next(t0)
	locked := "A `brew update` process has already locked /opt/homebrew/var/homebrew/locks/update."
	for attempt := 1; attempt < MaxAttempts; attempt++ {
		if !q.Finish(j, 1, false, locked, t0) {
			t.Fatalf("attempt %d should retry", attempt)
```

   with:

```go
	j := q.Add("upgrade", []string{"upgrade"})
	other := q.Add("pin git", nil)
	q.Next(t0)
	locked := "Error: A `brew update` process has already locked /opt/homebrew/var/homebrew/locks/update."
	for attempt := 1; attempt < MaxAttempts; attempt++ {
		if !q.Finish(j, 1, false, locked, t0) {
			t.Fatalf("attempt %d should retry", attempt)
```

2. Replace:

```go
	}
}

func TestBackoff(t *testing.T) {
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for i, w := range want {
```

   with:

```go
	}
}

func TestIsLockError(t *testing.T) {
	for _, tt := range []struct {
		output string
		want   bool
	}{
		{"Error: A `brew upgrade` process has already locked /opt/homebrew/Cellar/fd.lock.", true},
		{"==> Upgrading fd\r\n\x1b[4;31mError\x1b[0m: A `brew upgrade fd` process has already locked /opt/homebrew/Cellar/fd.lock.\r\nPlease wait for it to finish or terminate it to continue.\r\n", true},
		{"A `brew update` process has already locked /x.", false},                         // not brew's error line
		{"==> Caveats\nThis tool says: Error: A `x` process has already locked y", false}, // mid-line
		{"Error: No such keg: /opt/homebrew/Cellar/fd", false},
	} {
		if got := IsLockError(tt.output); got != tt.want {
			t.Errorf("IsLockError(%q) = %v, want %v", tt.output, got, tt.want)
		}
	}
}

func TestBackoff(t *testing.T) {
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for i, w := range want {
```

- [ ] **Step 2: Edit `internal/ui/drawer_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	if out := stripANSI(d.view(70, 8, testNow, 0, true)); !strings.Contains(out, "typing goes to brew") {
		t.Errorf("insert hint missing:\n%s", out)
	}
	q.Finish(j, 1, false, "A `brew update` process has already locked /x.", testNow)
	d.retryAt = testNow.Add(4 * time.Second)
	if out := stripANSI(d.view(70, 8, testNow, 0, false)); !strings.Contains(out, "waiting for brew's lock") || !strings.Contains(out, "retry 2 of 5 in 0:04") {
		t.Errorf("waiting title wrong:\n%s", out)
```

   with:

```go
	if out := stripANSI(d.view(70, 8, testNow, 0, true)); !strings.Contains(out, "typing goes to brew") {
		t.Errorf("insert hint missing:\n%s", out)
	}
	q.Finish(j, 1, false, "Error: A `brew update` process has already locked /x.", testNow)
	d.retryAt = testNow.Add(4 * time.Second)
	if out := stripANSI(d.view(70, 8, testNow, 0, false)); !strings.Contains(out, "waiting for brew's lock") || !strings.Contains(out, "retry 2 of 5 in 0:04") {
		t.Errorf("waiting title wrong:\n%s", out)
```

- [ ] **Step 3: Edit `internal/ui/run_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
}

func TestStoppingAJobWaitingForTheLock(t *testing.T) {
	locked := newFakeProc(1, "A `brew update` process has already locked /x.\r\n")
	locked.exit()
	fs := &fakeStarter{procs: []*fakeProc{locked}}
	a := newJobApp(t, fs)
```

   with:

```go
}

func TestStoppingAJobWaitingForTheLock(t *testing.T) {
	locked := newFakeProc(1, "Error: A `brew update` process has already locked /x.\r\n")
	locked.exit()
	fs := &fakeStarter{procs: []*fakeProc{locked}}
	a := newJobApp(t, fs)
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    --- FAIL: TestIsLockError (0.00s)
```

- [ ] **Step 5: Edit `internal/jobs/jobs.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
package jobs

import (
	"strings"
	"time"
)

// State is where a job is in its life.
```

   with:

```go
package jobs

import (
	"regexp"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// State is where a job is in its life.
```

2. Replace:

```go
	return min(2*time.Second<<max(attempt-1, 0), 30*time.Second)
}

// IsLockError reports whether brew's output says another brew process
// holds the lock it needs ("A `brew …` process has already locked …").
func IsLockError(output string) bool {
	return strings.Contains(output, "has already locked")
}
```

   with:

```go
	return min(2*time.Second<<max(attempt-1, 0), 30*time.Second)
}

// lockRe matches the error brew prints when another brew process holds
// the lock it needs: "Error: A `brew upgrade` process has already locked
// /opt/homebrew/Cellar/fd.lock." It must start a line, so a package's own
// output that merely mentions the words can't trigger retries.
var lockRe = regexp.MustCompile("(?m)^Error: A `[^`\n]*` process has already locked ")

// IsLockError reports whether brew's output says another brew process
// holds the lock it needs. In a terminal brew colors "Error", so colors are
// stripped first.
func IsLockError(output string) bool {
	return lockRe.MatchString(ansi.Strip(output))
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "fix(jobs): retry only on brew's own lock error line"
```

---

### Task 2: Quitting stops a running job with ctrl+c first

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/run.go`
- Modify: `internal/ui/run_test.go`

**Interfaces:**
- **Consumes:** `Proc.Interrupt`/`Kill`, `App.record`, `queue.Finish`.
- **Produces:**
  - `quitGrace = 3 * time.Second`, `App.quitGrace` (tests set it), `App.quitting`.
  - `quitNowMsg`; `(*App) quitNow() tea.Cmd`.
  - `quit()` interrupts and waits instead of killing.
  - `finish()` records the job and returns `tea.Sequence(record, tea.Quit)` while quitting.
- **Test helpers:** `quits(cmd)` (whether a command, or the sequence it returns, quits without running long ticks), `quitWithJob`.

The record's save runs before `tea.Quit`, so the stopped job is in the history. A job that's only waiting for brew's lock has no process: quitting then exits at once, as before.

- [ ] **Step 1: Edit `internal/ui/run_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
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
```

   with:

```go
	_ = waiting // the hour-long retry timer is abandoned with the test
}

// quits reports whether cmd, or a batch or sequence it returns, quits.
// Commands other than tea.Quit are not run, so a long tick can't block.
func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	if reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.Quit).Pointer() {
		return true
	}
	msg := cmd()
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		for i := range v.Len() {
			if c := v.Index(i).Interface().(tea.Cmd); c != nil && reflect.ValueOf(c).Pointer() == reflect.ValueOf(tea.Quit).Pointer() {
				return true
			}
		}
	}
	_, ok := msg.(tea.QuitMsg)
	return ok
}

// quitWithJob starts a job (and queues a second), then quits with q, y.
func quitWithJob(t *testing.T) (*App, *fakeProc, *fakeStarter, tea.Cmd) {
	t.Helper()
	p := newFakeProc(0, "working")
	fs := &fakeStarter{procs: []*fakeProc{p}}
	a := newJobApp(t, fs)
	a.quitGrace = time.Hour
	settle(t, a, a.run(action{title: "upgrade", args: []string{"upgrade"}}))
	a.run(action{title: "pin git", args: []string{"pin", "git"}})
	_, cmd := a.Update(press("q"))
	if cmd != nil || a.confirm == nil {
		t.Fatal("q with a running job should ask first")
	}
	_, cmd = a.Update(press("y"))
	return a, p, fs, cmd
}

func TestQuitStopsTheRunningJobFirst(t *testing.T) {
	a, p, fs, cmd := quitWithJob(t)
	if !p.interrupted || p.killed || cmd == nil {
		t.Fatalf("y should send brew ctrl+c and wait (interrupted=%v killed=%v)", p.interrupted, p.killed)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "stopping upgrade… · ctrl+c quits now") {
		t.Errorf("status bar: %q", bar)
	}
	a.Update(press("2"))
	if a.tab != tabHome {
		t.Error("keys do nothing while brewery is quitting")
	}
	j := a.queue.Current()
	_, cmd = a.Update(jobExitMsg{j.ID, j.Attempts, 130})
	if !quits(cmd) {
		t.Error("once the job has stopped, brewery quits")
	}
	if len(a.history) != 1 || a.history[0].State != jobs.Canceled {
		t.Errorf("the stopped job is recorded: %+v", a.history)
	}
	if len(fs.calls) != 1 {
		t.Errorf("the queued job must not start: %v", fs.calls)
	}
}

func TestQuitKillsAJobThatWontStop(t *testing.T) {
	a, p, _, _ := quitWithJob(t)
	_, cmd := a.Update(quitNowMsg{})
	if !p.killed || !quits(cmd) {
		t.Errorf("after the grace period brew is killed and brewery quits (killed=%v)", p.killed)
	}
}

func TestCtrlCWhileQuittingQuitsNow(t *testing.T) {
	a, p, _, _ := quitWithJob(t)
	_, cmd := a.Update(press("ctrl+c"))
	if !p.killed || !quits(cmd) {
		t.Errorf("a second ctrl+c doesn't wait (killed=%v)", p.killed)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/run_test.go:511:4: a.quitGrace undefined (type *App has no field or method quitGrace)
    internal/ui/run_test.go:549:21: undefined: quitNowMsg
```

- [ ] **Step 3: Edit `internal/ui/app.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
	tickEvery time.Duration // spinner speed; 0 turns the spinner off
	ticking   bool
	frame     int

	hist     *jobs.History // where finished jobs are saved, or nil
	history  []jobs.Record // finished jobs, newest first
```

   with:

```go
	tickEvery time.Duration // spinner speed; 0 turns the spinner off
	ticking   bool
	frame     int
	quitGrace time.Duration // how long a running job gets to stop when brewery quits
	quitting  bool          // brewery is waiting for the running job to stop, then exits

	hist     *jobs.History // where finished jobs are saved, or nil
	history  []jobs.Record // finished jobs, newest first
```

2. Replace:

```go
		src: src, start: start, cachePath: cachePath, st: st, now: time.Now, ctx: ctx, cancel: cancel,
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		acts: map[int]action{}, backoff: jobs.Backoff, tickEvery: 100 * time.Millisecond,
		readLog:   readLogTail,
		drawer:    newDrawer(st),
		cmd:       newCmdline(st),
```

   with:

```go
		src: src, start: start, cachePath: cachePath, st: st, now: time.Now, ctx: ctx, cancel: cancel,
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		acts: map[int]action{}, backoff: jobs.Backoff, tickEvery: 100 * time.Millisecond, quitGrace: quitGrace,
		readLog:   readLogTail,
		drawer:    newDrawer(st),
		cmd:       newCmdline(st),
```

3. Replace:

```go
	return false
}

// quit stops running queries and any running job, then exits.
func (a *App) quit() tea.Cmd {
	a.cancel()
	if a.proc != nil {
		a.proc.Kill()
	}
```

   with:

```go
	return false
}

// quit stops running queries and exits. A running job first gets ctrl+c,
// as it would in a terminal, so brew can clean up after itself; brewery
// exits once the job has stopped and been recorded, or kills it after
// quitGrace.
func (a *App) quit() tea.Cmd {
	a.cancel()
	if a.proc == nil {
		return tea.Quit
	}
	a.quitting, a.canceled = true, true
	a.proc.Interrupt()
	a.flash = a.st.Warn.Render("stopping " + a.queue.Current().Title + "… · ctrl+c quits now")
	return tea.Tick(a.quitGrace, func(time.Time) tea.Msg { return quitNowMsg{} })
}

// quitNow kills a job that hasn't stopped, and exits.
func (a *App) quitNow() tea.Cmd {
	if a.proc != nil {
		a.proc.Kill()
	}
```

4. Replace:

```go
		a.histErr = msg.err
	case takeoverDoneMsg:
		return a.takeoverDone(msg)
	case logMsg:
		a.gotLog(msg)
	case pagerDoneMsg:
```

   with:

```go
		a.histErr = msg.err
	case takeoverDoneMsg:
		return a.takeoverDone(msg)
	case quitNowMsg:
		return a.quitNow()
	case logMsg:
		a.gotLog(msg)
	case pagerDoneMsg:
```

5. Replace:

```go
// never trigger a brewery shortcut such as q.
func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if a.mode != modeCommand {
		a.flash = "" // in COMMAND the bar shows only the prompt: keep it for when the line closes
	}
```

   with:

```go
// never trigger a brewery shortcut such as q.
func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if a.quitting {
		if k == "ctrl+c" {
			return a.quitNow()
		}
		return nil // on the way out: nothing else may start
	}
	if a.mode != modeCommand {
		a.flash = "" // in COMMAND the bar shows only the prompt: keep it for when the line closes
	}
```

- [ ] **Step 4: Edit `internal/ui/run.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	jobExitMsg  struct{ id, attempt, code int }
	jobRetryMsg struct{ id int }
	tickMsg     struct{}
)

// readOutput reads the next chunk of a job's output.
func readOutput(p Proc, id, attempt int) tea.Cmd {
	return func() tea.Msg {
```

   with:

```go
	jobExitMsg  struct{ id, attempt, code int }
	jobRetryMsg struct{ id int }
	tickMsg     struct{}
	quitNowMsg  struct{} // the running job had its chance to stop: quit anyway
)

// quitGrace is how long a running job gets to stop after ctrl+c when
// brewery quits, before it is killed.
const quitGrace = 3 * time.Second

// readOutput reads the next chunk of a job's output.
func readOutput(p Proc, id, attempt int) tea.Cmd {
	return func() tea.Msg {
```

2. Replace:

```go
	if a.mode == modeInsert {
		a.mode = modeNormal
	}
	if a.queue.Finish(j, code, a.canceled, a.drawer.output(), a.now()) {
		wait := a.backoff(j.Attempts)
		a.drawer.retryAt = a.now().Add(wait)
```

   with:

```go
	if a.mode == modeInsert {
		a.mode = modeNormal
	}
	if a.quitting {
		// Stopped on the way out: record it, start nothing else, and go.
		a.queue.Finish(j, code, true, a.drawer.output(), a.now())
		return tea.Sequence(a.record(j, a.drawer.output()), tea.Quit)
	}
	if a.queue.Finish(j, code, a.canceled, a.drawer.output(), a.now()) {
		wait := a.backoff(j.Attempts)
		a.drawer.retryAt = a.now().Add(wait)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "fix(ui): quitting stops a running job with ctrl+c before killing it"
```

---

### Task 3: A bigger drawer, and frames that fit small windows

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/drawer.go`
- Modify: `internal/ui/drawer_test.go`
- Modify: `internal/ui/run_test.go`

**Interfaces:**
- **Consumes:** `drawerHeight`, `App.render`.
- **Produces:** the new `drawerHeight`; `clipLines(s string, n int) string`; `render` clips the tab body to `tabH` and the frame to `a.height`.

Two existing expectations change with the new sizes: the drawer-height table, and the pty size after a resize (116×12 and 96×9).

- [ ] **Step 1: Edit `internal/ui/drawer_test.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"brewery/internal/jobs"
```

   with:

```go
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/jobs"
```

2. Replace:

```go
}

func TestDrawerHeight(t *testing.T) {
	for _, tt := range []struct{ body, want int }{{40, 14}, {21, 8}, {12, 6}, {7, 4}, {3, 0}} {
		if got := drawerHeight(tt.body); got != tt.want {
			t.Errorf("drawerHeight(%d) = %d, want %d", tt.body, got, tt.want)
		}
```

   with:

```go
}

func TestDrawerHeight(t *testing.T) {
	for _, tt := range []struct{ body, want int }{{40, 16}, {37, 16}, {21, 10}, {13, 7}, {12, 6}, {9, 5}, {7, 5}, {3, 3}} {
		if got := drawerHeight(tt.body); got != tt.want {
			t.Errorf("drawerHeight(%d) = %d, want %d", tt.body, got, tt.want)
		}
```

3. Replace:

```go
		t.Fatal("write blocked: the reply drain stopped after a failed reply")
	}
}
```

   with:

```go
		t.Fatal("write blocked: the reply drain stopped after a failed reply")
	}
}

func TestFrameFitsSmallWindows(t *testing.T) {
	for _, h := range []int{6, 7, 8, 9, 10, 12, 16, 24, 30, 40} {
		a := newJobApp(t, &fakeStarter{})
		a.Update(tea.WindowSizeMsg{Width: 80, Height: h})
		if got := lipgloss.Height(a.render()); got != h {
			t.Errorf("height %d, no job: rendered %d lines", h, got)
		}
		p := newFakeProc(0, "working")
		a = newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
		a.Update(tea.WindowSizeMsg{Width: 80, Height: h})
		settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
		out := a.render()
		if got := lipgloss.Height(out); got != h {
			t.Errorf("height %d, drawer open: rendered %d lines", h, got)
		}
		if h >= 10 && !strings.Contains(stripANSI(out), "working") {
			t.Errorf("height %d: the drawer should show the job's output:\n%s", h, stripANSI(out))
		}
	}
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{newFakeProc(0)}})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
	if a.drawer.rows != 6 {
		t.Errorf("at 80x24 the drawer shows %d lines of output, want 6", a.drawer.rows)
	}
}
```

- [ ] **Step 2: Edit `internal/ui/run_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
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
```

   with:

```go
	p := newFakeProc(0)
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
	if p.cols != 116 || p.rows != 12 {
		t.Errorf("pty is %dx%d, want the drawer's inner 116x12", p.cols, p.rows)
	}
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if p.cols != 96 || p.rows != 9 {
		t.Errorf("after resize pty is %dx%d, want 96x9", p.cols, p.rows)
	}
	p.exit()
	resume(t, a, waiting)
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    --- FAIL: TestDrawerHeight (0.00s)
    --- FAIL: TestFrameFitsSmallWindows (1.37s)
    --- FAIL: TestJobPTYSizeMatchesDrawer (0.11s)
```

- [ ] **Step 4: Edit `internal/ui/app.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
			body = a.taps.view(a.errs[qTaps])
		}
	}
	if a.confirm != nil {
		body = overlay(body, a.confirm.view(a.st, a.width), a.width, bodyH)
	}
```

   with:

```go
			body = a.taps.view(a.errs[qTaps])
		}
	}
	body = clipLines(body, bodyH) // a tab too tall for a small window mustn't push the rest off it
	if a.confirm != nil {
		body = overlay(body, a.confirm.view(a.st, a.width), a.width, bodyH)
	}
```

2. Replace:

```go
	if a.drawerShown() {
		parts = append(parts, a.drawer.view(a.width, a.bodyHeight()-bodyH, a.now(), a.frame, a.mode == modeInsert))
	}
	screen := lipgloss.JoinVertical(lipgloss.Left, append(parts, a.statusBar())...)
	if p := a.cmd.popup(a.st, a.width); a.mode == modeCommand && p != "" {
		h := lipgloss.Height(screen)
		screen = overlayAt(screen, p, a.width, h, 0, max(h-1-lipgloss.Height(p), 0))
```

   with:

```go
	if a.drawerShown() {
		parts = append(parts, a.drawer.view(a.width, a.bodyHeight()-bodyH, a.now(), a.frame, a.mode == modeInsert))
	}
	screen := clipLines(lipgloss.JoinVertical(lipgloss.Left, append(parts, a.statusBar())...), a.height)
	if p := a.cmd.popup(a.st, a.width); a.mode == modeCommand && p != "" {
		h := lipgloss.Height(screen)
		screen = overlayAt(screen, p, a.width, h, 0, max(h-1-lipgloss.Height(p), 0))
```

3. Replace:

```go
	return screen
}

// bodyHeight is the space between the tab bar (tabs + rule) and the status bar.
func (a *App) bodyHeight() int { return max(a.height-3, 3) }
```

   with:

```go
	return screen
}

// clipLines keeps at most the first n lines of s.
func clipLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	return strings.Join(lines[:min(len(lines), max(n, 0))], "\n")
}

// bodyHeight is the space between the tab bar (tabs + rule) and the status bar.
func (a *App) bodyHeight() int { return max(a.height-3, 3) }
```

- [ ] **Step 5: Edit `internal/ui/drawer.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
}

// drawerHeight is how tall the drawer is for a body of height bodyH: about
// two fifths, between 6 and 14 lines, leaving the tab at least 3.
func drawerHeight(bodyH int) int {
	return max(min(bodyH*2/5, 14, bodyH-3), min(6, bodyH-3), 0)
}

// start shows a new run of j on a clean screen. Terminal replies go to
```

   with:

```go
}

// drawerHeight is how tall the drawer is for a body of height bodyH: about
// half of it, between 10 lines (6 of output) and 16, leaving the tab at
// least 6. When that leaves no room for output, the drawer takes 5 lines
// (one of output), or the whole body if that's shorter: a running job's
// output matters more than the tab behind it.
func drawerHeight(bodyH int) int {
	h := min(max(bodyH/2, 10), 16, bodyH-6)
	if h < 5 {
		h = min(5, bodyH)
	}
	return max(h, 0)
}

// start shows a new run of j on a clean screen. Terminal replies go to
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "fix(ui): a bigger drawer, and frames that fit small windows"
```

---

### Task 4: Keep a crash log; never write state to a relative path

**Files:**
- Create: `cmd/brewery/crash.go`
- Create: `cmd/brewery/crash_test.go`
- Modify: `cmd/brewery/main.go`
- Modify: `internal/jobs/history.go`
- Modify: `internal/jobs/history_test.go`
- Modify: `internal/state/snapshot.go`
- Modify: `internal/state/snapshot_test.go`

**Interfaces:**
- **Consumes:** `jobs.DefaultHistoryDir`, `state.DefaultCachePath`, `state.Save`.
- **Produces:**
  - `jobs.DefaultStateDir()`; `DefaultHistoryDir` is `<state dir>/jobs`.
  - In `cmd/brewery/crash.go`: `crashLog{path, f, stderr}`, `startCrashLog(path) (*crashLog, error)`, `(*crashLog) stop() bool`.
  - `main` wraps `Program.Run` in it and gives takeover and log-viewer commands the real stderr.

`stop` renames the temp file to `crash.log`. If the rename fails, it keeps the temp file and reports that path instead.

- [ ] **Step 1: Create `cmd/brewery/crash_test.go`**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCrashLogKeepsWhatWasWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "crash.log")
	real := os.Stderr
	c, err := startCrashLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.Stderr == real {
		t.Fatal("stderr should point at the crash file while the TUI runs")
	}
	fmt.Fprint(os.Stderr, "Caught panic:\r\n\r\nboom\r\n")
	if !c.stop() {
		t.Fatal("something was written: stop should report a crash")
	}
	if os.Stderr != real {
		t.Error("stop must put stderr back")
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "Caught panic:\r\n\r\nboom\r\n" {
		t.Errorf("crash.log = %q, %v", b, err)
	}
}

func TestCrashLogLeavesNothingWithoutACrash(t *testing.T) {
	dir := t.TempDir()
	c, err := startCrashLog(filepath.Join(dir, "crash.log"))
	if err != nil {
		t.Fatal(err)
	}
	if c.stop() {
		t.Error("nothing was written: no crash")
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("files left behind: %v", left)
	}
}
```

- [ ] **Step 2: Edit `internal/jobs/history_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Fatalf("Load = %d records, %v; want 8", len(got), err)
	}
}
```

   with:

```go
		t.Fatalf("Load = %d records, %v; want 8", len(got), err)
	}
}

func TestDefaultStateDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg")
	if got := DefaultStateDir(); got != "/tmp/xdg/brewery" {
		t.Errorf("got %s", got)
	}
	if got := DefaultHistoryDir(); got != "/tmp/xdg/brewery/jobs" {
		t.Errorf("got %s", got)
	}
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")
	if got := DefaultStateDir(); !filepath.IsAbs(got) || !strings.HasPrefix(got, os.TempDir()) {
		t.Errorf("without a home directory state goes in the temp directory, got %s", got)
	}
}
```

- [ ] **Step 3: Edit `internal/state/snapshot_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
```

   with:

```go
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
```

2. Replace:

```go
		t.Errorf("got %s", got)
	}
}
```

   with:

```go
		t.Errorf("got %s", got)
	}
}

func TestDefaultCachePathWithoutHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	got := DefaultCachePath()
	if !filepath.IsAbs(got) || !strings.HasPrefix(got, os.TempDir()) {
		t.Errorf("without a home directory the cache goes in the temp directory, got %s", got)
	}
}

func TestSaveCleansUpWhenItCantReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	// A non-empty directory where the file should go: the rename fails.
	if err := os.MkdirAll(filepath.Join(path, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, Snapshot{}); err == nil {
		t.Fatal("Save should fail")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "snapshot-*.json")); len(left) != 0 {
		t.Errorf("temp file left behind: %v", left)
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/jobs/history_test.go:217:12: undefined: DefaultStateDir
    internal/jobs/history_test.go:225:12: undefined: DefaultStateDir
    cmd/brewery/crash_test.go:13:12: undefined: startCrashLog
```

- [ ] **Step 5: Create `cmd/brewery/crash.go`**

```go
package main

import (
	"os"
	"path/filepath"
)

// crashLog keeps what brewery prints if it crashes. Bubble Tea catches a
// panic, restores the terminal, then writes the panic and its stack to
// os.Stderr; while the TUI runs, os.Stderr points at a file instead, so
// that report lands in crash.log. Nothing else writes to stderr while the
// TUI is up: it would garble the screen.
type crashLog struct {
	path   string
	f      *os.File
	stderr *os.File // the real stderr, put back by stop
}

// startCrashLog points os.Stderr at a new temp file next to path.
func startCrashLog(path string) (*crashLog, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".crash-*.log")
	if err != nil {
		return nil, err
	}
	c := &crashLog{path: path, f: f, stderr: os.Stderr}
	os.Stderr = f
	return c, nil
}

// stop puts os.Stderr back. If anything was written, the file becomes the
// crash log and stop reports true; otherwise the file is removed.
func (c *crashLog) stop() bool {
	os.Stderr = c.stderr
	info, err := c.f.Stat()
	c.f.Close()
	if err != nil || info.Size() == 0 {
		os.Remove(c.f.Name())
		return false
	}
	if err := os.Rename(c.f.Name(), c.path); err != nil {
		c.path = c.f.Name() // keep it where it is
	}
	return true
}
```

- [ ] **Step 6: Edit `cmd/brewery/main.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	"io"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
```

   with:

```go
	"io"
	"os"
	"os/exec"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
```

2. Replace:

```go
	recs, _ := hist.Load() // no history yet is fine
	app.UseHistory(hist, recs)
	app.UseCompletions(client.Completions)
	app.UseTakeover(func(args []string) *exec.Cmd { return exec.Command(bin, args...) })
	// less +F follows the log as it grows; ctrl+c stops following, q returns.
	app.UsePager(func(files []string) *exec.Cmd {
		return exec.Command("less", append([]string{"+F", "--"}, files...)...)
	})

	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
		return 1
	}
```

   with:

```go
	recs, _ := hist.Load() // no history yet is fine
	app.UseHistory(hist, recs)
	app.UseCompletions(client.Completions)
	// Full-screen commands get the real stderr: while the TUI runs,
	// os.Stderr is the crash log.
	realStderr := os.Stderr
	fullScreen := func(cmd *exec.Cmd) *exec.Cmd {
		cmd.Stderr = realStderr
		return cmd
	}
	app.UseTakeover(func(args []string) *exec.Cmd { return fullScreen(exec.Command(bin, args...)) })
	// less +F follows the log as it grows; ctrl+c stops following, q returns.
	app.UsePager(func(files []string) *exec.Cmd {
		return fullScreen(exec.Command("less", append([]string{"+F", "--"}, files...)...))
	})

	crash, crashErr := startCrashLog(filepath.Join(jobs.DefaultStateDir(), "crash.log"))
	_, err = tea.NewProgram(app).Run()
	if crashErr == nil && crash.stop() {
		fmt.Fprintf(stderr, "brewery crashed. What happened is in %s; please include it when reporting this.\n", crash.path)
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
		return 1
	}
```

- [ ] **Step 7: Edit `internal/jobs/history.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
// DefaultHistoryDir is $XDG_STATE_HOME/brewery/jobs, falling back to
// ~/.local/state/brewery/jobs.
func DefaultHistoryDir() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "brewery", "jobs")
}

// Save writes r and removes the oldest records beyond Keep.
```

   with:

```go
// DefaultHistoryDir is $XDG_STATE_HOME/brewery/jobs, falling back to
// ~/.local/state/brewery/jobs.
func DefaultHistoryDir() string {
	return filepath.Join(DefaultStateDir(), "jobs")
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
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "brewery")
}

// Save writes r and removes the oldest records beyond Keep.
```

- [ ] **Step 8: Edit `internal/state/snapshot.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
}

// DefaultCachePath is $XDG_CACHE_HOME/brewery/snapshot.json, falling back
// to ~/.cache/brewery/snapshot.json.
func DefaultCachePath() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "brewery", "snapshot.json")
```

   with:

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
		}
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "brewery", "snapshot.json")
```

2. Replace:

```go
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
```

   with:

```go
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
```

- [ ] **Step 9: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat: keep a crash log; never write state to a relative path"
```

---

### Task 5: Small follow-ups from Plan 4

**Files:**
- Modify: `internal/brew/client_test.go`
- Modify: `internal/ui/app_test.go`
- Modify: `internal/ui/cleanup_test.go`
- Modify: `internal/ui/services.go`
- Modify: `internal/ui/services_test.go`

**Interfaces:**
- **Consumes:** Plan 4's `wantLog`, `Client.Cleanup`, `askCleanup`, `openLog`.
- **Produces:** `wantLog` resets the held log when the selected service has none, so coming back to a log reads it again. No other code changes.

The new tests cover the paths Plan 4's reviews found untested:
- a failing standard dry-run;
- a cleanup job re-measuring (only `cleanup` runs again);
- `c` after the dry-runs failed;
- `l` with no log viewer.

Fake error strings that still said `services list` now say `services info --all`.

- [ ] **Step 1: Edit `internal/brew/client_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	if _, err := c.Cleanup(context.Background()); err == nil || !strings.Contains(err.Error(), "brew cleanup --dry-run --prune=all") {
		t.Errorf("a failed dry-run is an error naming it: %v", err)
	}
}

func TestClientQueryErrorIncludesStderr(t *testing.T) {
```

   with:

```go
	if _, err := c.Cleanup(context.Background()); err == nil || !strings.Contains(err.Error(), "brew cleanup --dry-run --prune=all") {
		t.Errorf("a failed dry-run is an error naming it: %v", err)
	}
	c = NewClient(fakeRunner{err: map[string]error{"cleanup --dry-run": errors.New("exit status 1")}})
	if _, err := c.Cleanup(context.Background()); err == nil || !strings.Contains(err.Error(), "brew cleanup --dry-run: exit status 1") {
		t.Errorf("a failed standard dry-run is an error naming it: %v", err)
	}
}

func TestClientQueryErrorIncludesStderr(t *testing.T) {
```

- [ ] **Step 2: Edit `internal/ui/app_test.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go

func TestFailedQueryShowsErrorOnlyInItsTab(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("brew services list --json: exit status 1")
	a := newTestApp(t, src)
	drain(t, a, a.Init())
```

   with:

```go

func TestFailedQueryShowsErrorOnlyInItsTab(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("brew services info --all --json: exit status 1")
	a := newTestApp(t, src)
	drain(t, a, a.Init())
```

2. Replace:

```go
		for _, failing := range []bool{false, true} {
			src := newFakeSource(t)
			if failing {
				src.servicesErr = errors.New("brew services list --json: exit status 1\nError: launchctl bootstrap failed for a very long and descriptive reason that will not fit")
			}
			a := newTestApp(t, src)
			a.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
```

   with:

```go
		for _, failing := range []bool{false, true} {
			src := newFakeSource(t)
			if failing {
				src.servicesErr = errors.New("brew services info --all --json: exit status 1\nError: launchctl bootstrap failed for a very long and descriptive reason that will not fit")
			}
			a := newTestApp(t, src)
			a.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
```

3. Replace:

```go

func TestRefreshFailureShowsOverCachedRows(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("brew services list --json: exit status 1\nError: launchctl failed")
	a := New(src, nil, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
```

   with:

```go

func TestRefreshFailureShowsOverCachedRows(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("brew services info --all --json: exit status 1\nError: launchctl failed")
	a := New(src, nil, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
```

- [ ] **Step 3: Edit `internal/ui/cleanup_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
package ui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"brewery/internal/brew"
)
```

   with:

```go
package ui

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)
```

2. Replace:

```go
		t.Fatalf("enter on the Cleanup tile should ask: %+v", a.confirm)
	}
}
```

   with:

```go
		t.Fatalf("enter on the Cleanup tile should ask: %+v", a.confirm)
	}
}

func TestCleanupJobMeasuresAgain(t *testing.T) {
	src := newCountingSource(t)
	a := New(src, (&fakeStarter{}).start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	keys(t, a, "c", "y") // the fake cleanup finishes at once
	if src.calls["cleanup"] != 1 || src.calls["installed"] != 0 {
		t.Errorf("after a cleanup only the dry-runs run again: %v", src.calls)
	}
}

func TestCleanupAfterTheDryRunsFailed(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.errs[qCleanup] = errors.New("brew cleanup --dry-run: exit status 1")
	keys(t, a, "c")
	if a.confirm != nil || !strings.Contains(stripANSI(a.statusBar()), "couldn't measure the cleanup · r retries") {
		t.Errorf("no dialog without numbers: %+v %q", a.confirm, stripANSI(a.statusBar()))
	}
}
```

- [ ] **Step 4: Edit `internal/ui/services_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}
```

   with:

```go
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestServicesPreviewRereadsAfterAServiceWithoutALog(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	logs := &fakeLogs{text: map[string]string{"/opt/homebrew/var/log/postgresql@16.log": "x"}}
	a.readLog = logs.read
	a.snap.Services[1].Log = "" // redis
	a.sync()
	keys(t, a, "4", "j", "k") // postgresql@16, redis, postgresql@16
	if n := logs.reads["/opt/homebrew/var/log/postgresql@16.log"]; n != 2 {
		t.Errorf("coming back to a service reads its log again: %d reads", n)
	}
}

func TestLogKeyWithoutAViewer(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	keys(t, a, "4", "l")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "the log viewer isn't available") {
		t.Errorf("status bar %q", bar)
	}
}
```

- [ ] **Step 5: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    --- FAIL: TestServicesPreviewRereadsAfterAServiceWithoutALog (0.01s)
```

- [ ] **Step 6: Edit `internal/ui/services.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
// yet, or has an older copy from before the last refresh.
func (a *App) wantLog() tea.Cmd {
	s, ok := a.selectedService()
	if !ok || s.Log == "" || (s.Log == a.log.path && !a.log.stale) {
		return nil
	}
	if s.Log != a.log.path {
```

   with:

```go
// yet, or has an older copy from before the last refresh.
func (a *App) wantLog() tea.Cmd {
	s, ok := a.selectedService()
	if !ok || s.Log == "" {
		a.log = logTail{} // nothing to show: coming back to a log reads it afresh
		return nil
	}
	if s.Log == a.log.path && !a.log.stale {
		return nil
	}
	if s.Log != a.log.path {
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "fix(ui): reread a log after visiting a service without one; cover cleanup's edges"
```

---

### Task 6: / filters the list you're on

**Files:**
- Modify: `internal/ui/app.go`
- Create: `internal/ui/filter.go`
- Create: `internal/ui/filter_test.go`
- Modify: `internal/ui/installed.go`
- Modify: `internal/ui/jobsview.go`
- Modify: `internal/ui/list.go`
- Modify: `internal/ui/run.go`

**Interfaces:**
- **Consumes:** `listTab`, `installedTab`, `cmdline`'s textinput styling, `promptPending`.
- **Produces:**
  - In `filter.go`:
    - `filterable{matchText() string; setMatch(string)}`, `matches(s, match string) bool`.
    - `newFilterInput`, `(*App) filterTarget() filterable`.
    - `(*App) openFilter()`, `(*App) filterKey(tea.Msg)`.
  - `modeFilter`; `App.filterIn`; `(*App) promptBar(mode, hint, line)` draws both the COMMAND and FILTER bars.
  - `listTab`:
    - gains `all`, `idx`, `match`, `matchCol`, `apply()`, `title()`;
    - `cursor()` returns the shown row's index in `all`;
    - the jobs view matches column 1 (the command).
  - `installedTab` gains `match` and implements `filterable`.
  - `/` also passes through the jobs view.

- [ ] **Step 1: Create `internal/ui/filter_test.go`**

```go
package ui

import (
	"reflect"
	"strings"
	"testing"
)

func TestFilterInstalled(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "2", "/")
	if bar := stripANSI(a.statusBar()); a.mode != modeFilter || !strings.Contains(bar, "FILTER") || !strings.Contains(bar, "enter keep · esc clear") {
		t.Fatalf("/ should open the filter: mode %d, bar %q", a.mode, stripANSI(a.statusBar()))
	}
	typeText(t, a, "GI") // case doesn't matter
	if got := rowNames(a.installed); got != "git" {
		t.Errorf("rows %q, want just git", got)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Installed · /GI") {
		t.Errorf("the pane should name the filter:\n%s", out)
	}
	keys(t, a, "enter")
	if a.mode != modeNormal || rowNames(a.installed) != "git" {
		t.Errorf("enter keeps the filter: mode %d, rows %q", a.mode, rowNames(a.installed))
	}
	keys(t, a, "/")
	if a.filterIn.Value() != "GI" {
		t.Errorf("/ again starts from the filter, got %q", a.filterIn.Value())
	}
	keys(t, a, "esc")
	if a.mode != modeNormal || rowNames(a.installed) != "docker-desktop fd fontbase git lua-language-server" {
		t.Errorf("esc clears the filter: rows %q", rowNames(a.installed))
	}
}

func TestFilterKeysEditTheLine(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "/")
	typeText(t, a, "qux")
	if len(fs.calls) != 0 || a.tab != tabInstalled || a.confirm != nil {
		t.Errorf("letters edit the filter, never act: %v", fs.calls)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing in this view matches /qux") {
		t.Errorf("an empty result should say why:\n%s", out)
	}
}

func TestFilterOutdatedActsOnTheShownRow(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "3", "/")
	typeText(t, a, "dock")
	keys(t, a, "enter", "u")
	if want := []string{"upgrade", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestFilterIsPerList(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "5", "/")
	typeText(t, a, "zzz")
	keys(t, a, "enter")
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing matches /zzz") {
		t.Errorf("no tap matches:\n%s", out)
	}
	keys(t, a, "4")
	if a.services.n != 2 {
		t.Errorf("Services keeps its own filter: %d rows", a.services.n)
	}
	keys(t, a, "5")
	if a.taps.n != 0 || a.taps.match != "zzz" {
		t.Errorf("Taps' filter survives switching tabs: %d rows, %q", a.taps.n, a.taps.match)
	}
}

func TestFilterJobs(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter", "/")
	typeText(t, a, "pin")
	keys(t, a, "enter", "enter") // replay the one shown
	if a.drawer.job == nil || a.drawer.job.Title != "pin git" {
		t.Errorf("replayed %+v, want pin git", a.drawer.job)
	}
}

func TestFilterOnHome(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "/")
	if a.mode != modeNormal || !strings.Contains(stripANSI(a.statusBar()), "Home has none") {
		t.Errorf("Home has no list: mode %d, bar %q", a.mode, stripANSI(a.statusBar()))
	}
}

func TestPromptWaitsForTheFilter(t *testing.T) {
	p := newFakeProc(0, "==> Installing docker-desktop\r\n")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade docker-desktop", args: []string{"upgrade", "--cask", "docker-desktop"}}))
	keys(t, a, "2", "/")
	typeText(t, a, "gi")
	p.out <- []byte("Password:")
	waiting = resume(t, a, waiting)
	if a.mode != modeFilter || p.input() != "" {
		t.Fatalf("a prompt must not take over the filter (mode %v, brew got %q)", a.mode, p.input())
	}
	keys(t, a, "enter")
	if a.mode != modeInsert {
		t.Errorf("closing the filter should answer the waiting prompt (mode %v)", a.mode)
	}
	p.exit()
	resume(t, a, waiting)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/filter_test.go:12:48: undefined: modeFilter
    internal/ui/filter_test.go:27:7: a.filterIn undefined (type *App has no field or method filterIn)
    internal/ui/filter_test.go:28:56: a.filterIn undefined (type *App has no field or method filterIn)
```

- [ ] **Step 3: Edit `internal/ui/app.go`** (9 replacements; each old block appears exactly once)

1. Replace:

```go
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
```

   with:

```go
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
```

2. Replace:

```go
	modeNormal  mode = iota // keys drive brewery
	modeInsert              // keys go to the running brew command
	modeCommand             // keys edit the ":" command line
)

// App is the root Bubble Tea model.
```

   with:

```go
	modeNormal  mode = iota // keys drive brewery
	modeInsert              // keys go to the running brew command
	modeCommand             // keys edit the ":" command line
	modeFilter              // keys edit the "/" filter
)

// App is the root Bubble Tea model.
```

3. Replace:

```go
	jobs     listTab       // the ":jobs" view
	jobsOpen bool

	mode    mode
	cmd     cmdline     // the ":" prompt
	confirm *confirmBox // open confirmation, or nil
	flash   string      // one-off status message, cleared by the next key

	comp     brew.Completions // brew's commands, for completing the prompt
	loadComp func(context.Context) (brew.Completions, error)
```

   with:

```go
	jobs     listTab       // the ":jobs" view
	jobsOpen bool

	mode     mode
	cmd      cmdline         // the ":" prompt
	filterIn textinput.Model // the "/" prompt
	confirm  *confirmBox     // open confirmation, or nil
	flash    string          // one-off status message, cleared by the next key

	comp     brew.Completions // brew's commands, for completing the prompt
	loadComp func(context.Context) (brew.Completions, error)
```

4. Replace:

```go
		readLog:   readLogTail,
		drawer:    newDrawer(st),
		cmd:       newCmdline(st),
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
```

   with:

```go
		readLog:   readLogTail,
		drawer:    newDrawer(st),
		cmd:       newCmdline(st),
		filterIn:  newFilterInput(st),
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
```

5. Replace:

```go
	// The history is read before the app starts, so the jobs view is never
	// waiting for it.
	a.jobs.loaded = true
	a.services.preview = a.servicePreview
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
```

   with:

```go
	// The history is read before the app starts, so the jobs view is never
	// waiting for it.
	a.jobs.loaded = true
	a.jobs.matchCol = 1 // jobs are found by their command
	a.services.preview = a.servicePreview
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
```

6. Replace:

```go
			cmd := a.cmd.update(msg)
			a.suggest(false)
			return cmd
		}
	case jobOutputMsg, jobEOFMsg, jobExitMsg, jobRetryMsg, tickMsg:
		return a.handleJob(msg)
```

   with:

```go
			cmd := a.cmd.update(msg)
			a.suggest(false)
			return cmd
		case a.mode == modeFilter:
			return a.filterKey(msg)
		}
	case jobOutputMsg, jobEOFMsg, jobExitMsg, jobRetryMsg, tickMsg:
		return a.handleJob(msg)
```

7. Replace:

```go
	if a.mode == modeCommand {
		return a.cmdKey(msg)
	}
	if a.jobsOpen {
		if cmd, ok := a.jobsKey(msg); ok {
			return cmd
```

   with:

```go
	if a.mode == modeCommand {
		return a.cmdKey(msg)
	}
	if a.mode == modeFilter {
		return a.filterKey(msg)
	}
	if a.jobsOpen {
		if cmd, ok := a.jobsKey(msg); ok {
			return cmd
```

8. Replace:

```go
		return a.askQuit()
	case ":":
		return a.openCmdline()
	case "ctrl+c":
		if a.queue.Current() != nil {
			return a.askStop()
```

   with:

```go
		return a.askQuit()
	case ":":
		return a.openCmdline()
	case "/":
		return a.openFilter()
	case "ctrl+c":
		if a.queue.Current() != nil {
			return a.askStop()
```

9. Replace:

```go
	return tabs + "\n" + st.TabRule.Render(strings.Repeat("─", a.width))
}

func (a *App) statusBar() string {
	st := a.st
	if a.mode == modeCommand {
		badge := st.Mode("COMMAND").Render("COMMAND") + " "
		hint := st.Muted.Render("tab complete · ↑↓ history · esc cancel ")
		room := a.width - lipgloss.Width(badge) - 1
		if room-lipgloss.Width(hint) >= 30 {
			room -= lipgloss.Width(hint)
		} else {
			hint = ""
		}
		line := badge + a.cmd.view(room) + hint
		return st.StatusBar.Width(a.width).Render(ansi.Truncate(line, a.width, ""))
	}
	badge := "NORMAL"
	switch {
```

   with:

```go
	return tabs + "\n" + st.TabRule.Render(strings.Repeat("─", a.width))
}

// promptBar is the status bar while a line is being typed: the mode's
// badge, the line in what room is left, and a hint when it fits.
func (a *App) promptBar(mode, hint string, line func(w int) string) string {
	st := a.st
	badge := st.Mode(mode).Render(mode) + " "
	hint = st.Muted.Render(hint + " ")
	room := a.width - lipgloss.Width(badge) - 1
	if room-lipgloss.Width(hint) >= 30 {
		room -= lipgloss.Width(hint)
	} else {
		hint = ""
	}
	return st.StatusBar.Width(a.width).Render(ansi.Truncate(badge+line(room)+hint, a.width, ""))
}

func (a *App) statusBar() string {
	st := a.st
	switch a.mode {
	case modeCommand:
		return a.promptBar("COMMAND", "tab complete · ↑↓ history · esc cancel", a.cmd.view)
	case modeFilter:
		return a.promptBar("FILTER", "enter keep · esc clear", func(w int) string {
			a.filterIn.SetWidth(max(w-2, 1)) // the prompt and the cursor cell
			return a.filterIn.View()
		})
	}
	badge := "NORMAL"
	switch {
```

- [ ] **Step 4: Create `internal/ui/filter.go`**

```go
package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/theme"
)

// filterable is a list the "/" filter can narrow. Each list keeps its own
// filter, so switching tabs doesn't lose it.
type filterable interface {
	matchText() string
	setMatch(string)
}

// matches reports whether s contains the filter text, ignoring case.
func matches(s, match string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(match))
}

func newFilterInput(st theme.Styles) textinput.Model {
	in := textinput.New()
	in.Prompt = "/"
	s := in.Styles()
	s.Cursor.Blink = false
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(st.P.Teal).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(st.P.Text)
	in.SetStyles(s)
	return in
}

// filterTarget is the list "/" narrows now: the jobs view when it's open,
// otherwise the tab's list. Home has none.
func (a *App) filterTarget() filterable {
	if a.jobsOpen {
		return &a.jobs
	}
	switch a.tab {
	case tabInstalled:
		return &a.installed
	case tabOutdated:
		return &a.outdated
	case tabServices:
		return &a.services
	case tabTaps:
		return &a.taps
	}
	return nil
}

// openFilter switches to filter mode, starting from the list's filter.
func (a *App) openFilter() tea.Cmd {
	t := a.filterTarget()
	if t == nil {
		return a.note("/ filters a list · Home has none")
	}
	a.mode = modeFilter
	a.filterIn.SetValue(t.matchText())
	a.filterIn.CursorEnd()
	a.filterIn.Focus()
	return nil
}

// filterKey handles a key in filter mode: the list follows as you type,
// enter keeps the filter, and esc clears it.
func (a *App) filterKey(msg tea.Msg) tea.Cmd {
	t := a.filterTarget()
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "enter":
			a.mode = modeNormal
			return nil
		case "esc":
			a.mode = modeNormal
			t.setMatch("")
			return nil
		}
	}
	var cmd tea.Cmd
	a.filterIn, cmd = a.filterIn.Update(msg)
	t.setMatch(a.filterIn.Value())
	return cmd
}
```

- [ ] **Step 5: Edit `internal/ui/installed.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go
	all    []brew.Package
	ix     *state.Index
	rows   []brew.Package // the filtered rows, in table order
	loaded bool
	table  table.Model
	w, h   int
```

   with:

```go
	all    []brew.Package
	ix     *state.Index
	rows   []brew.Package // the filtered rows, in table order
	match  string         // the "/" filter on names
	loaded bool
	table  table.Model
	w, h   int
```

2. Replace:

```go
func (t *installedTab) apply() {
	t.rows = t.rows[:0]
	for _, p := range t.all {
		if t.shows(t.filter, p) {
			t.rows = append(t.rows, p)
		}
	}
```

   with:

```go
func (t *installedTab) apply() {
	t.rows = t.rows[:0]
	for _, p := range t.all {
		if t.shows(t.filter, p) && matches(p.Name, t.match) {
			t.rows = append(t.rows, p)
		}
	}
```

3. Replace:

```go
	return strings.Join(s, " ")
}

func (t *installedTab) selected() (brew.Package, bool) {
	c := t.table.Cursor()
	if c < 0 || c >= len(t.rows) {
```

   with:

```go
	return strings.Join(s, " ")
}

func (t *installedTab) matchText() string { return t.match }

func (t *installedTab) setMatch(m string) {
	t.match = m
	t.apply()
}

func (t *installedTab) selected() (brew.Package, bool) {
	c := t.table.Cursor()
	if c < 0 || c >= len(t.rows) {
```

4. Replace:

```go
		ih -= bannerHeight
	}
	t.table.SetHeight(max(ih, 0))
	list := pane(t.st, "Installed", body+t.table.View(), listW, h, true)

	p, ok := t.selected()
	previewTitle, body := "Preview", t.st.Muted.Render("nothing in this view")
	switch {
	case ok:
		previewTitle, body = p.Name, t.preview(p)
```

   with:

```go
		ih -= bannerHeight
	}
	t.table.SetHeight(max(ih, 0))
	title := "Installed"
	if t.match != "" {
		title += " · /" + t.match
	}
	list := pane(t.st, title, body+t.table.View(), listW, h, true)

	p, ok := t.selected()
	previewTitle, body := "Preview", t.st.Muted.Render("nothing in this view")
	if t.match != "" {
		body = t.st.Muted.Render("nothing in this view matches /" + t.match)
	}
	switch {
	case ok:
		previewTitle, body = p.Name, t.preview(p)
```

- [ ] **Step 6: Edit `internal/ui/jobsview.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	case "1", "2", "3", "4", "5", "[", "]":
		a.jobsOpen = false
		return nil, false
	case ":", "o", "r", "ctrl+c":
		return nil, false
	}
	return a.jobs.update(msg), true
```

   with:

```go
	case "1", "2", "3", "4", "5", "[", "]":
		a.jobsOpen = false
		return nil, false
	case ":", "/", "o", "r", "ctrl+c":
		return nil, false
	}
	return a.jobs.update(msg), true
```

- [ ] **Step 7: Edit `internal/ui/list.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
	st     theme.Styles
	name   string
	cols   []col
	empty  string // shown when there are no rows
	loaded bool   // a result has arrived, so an empty table means "nothing"
	n      int
	key    func(table.Row) string // identifies a row across refreshes
	table  table.Model
	w, h   int

	// preview, if set, fills a pane beside the table with what it returns
	// for a w×h pane.
	preview func(w, h int) (title, body string)
```

   with:

```go
	st     theme.Styles
	name   string
	cols   []col
	empty  string                 // shown when there are no rows
	loaded bool                   // a result has arrived, so an empty table means "nothing"
	n      int                    // rows shown
	key    func(table.Row) string // identifies a row across refreshes
	table  table.Model
	w, h   int

	all      []table.Row // every row, before the filter
	idx      []int       // for each row shown, its index in all
	match    string      // the "/" filter
	matchCol int         // the column the filter looks in

	// preview, if set, fills a pane beside the table with what it returns
	// for a w×h pane.
	preview func(w, h int) (title, body string)
```

2. Replace:

```go
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
```

   with:

```go
}

func (t *listTab) setRows(rows []table.Row) {
	t.all = rows
	t.apply()
}

// apply shows the rows the filter lets through.
func (t *listTab) apply() {
	var shown []table.Row
	t.idx = t.idx[:0]
	for i, r := range t.all {
		if matches(r[t.matchCol], t.match) {
			shown = append(shown, r)
			t.idx = append(t.idx, i)
		}
	}
	t.n = len(shown)
	setRowsKeeping(&t.table, shown, t.key)
}

func (t *listTab) matchText() string { return t.match }

func (t *listTab) setMatch(m string) {
	t.match = m
	t.apply()
}

// cursor is the selected row's index into the slice the rows were built
// from, or -1 when no row is shown.
func (t *listTab) cursor() int {
	c := t.table.Cursor()
	if t.n == 0 || c < 0 || c >= len(t.idx) {
		return -1
	}
	return t.idx[c]
}

// firstCell identifies a row by its first column (service and tap names).
```

3. Replace:

```go
	return cmd
}

// listWidth is the table pane's width: all of it, or 55% beside a preview.
func (t *listTab) listWidth() int {
	if t.preview == nil {
```

   with:

```go
	return cmd
}

// title names the list, counts what it shows, and names its filter.
func (t *listTab) title() string {
	s := fmt.Sprintf("%s · %d", t.name, t.n)
	if t.match != "" {
		s += " · /" + t.match
	}
	return s
}

// listWidth is the table pane's width: all of it, or 55% beside a preview.
func (t *listTab) listWidth() int {
	if t.preview == nil {
```

4. Replace:

```go
		return pane(t.st, t.name, errorBody(t.st, err), w, h, true)
	case t.n == 0 && !t.loaded:
		return pane(t.st, t.name, t.st.Muted.Render("loading…"), w, h, true)
	case t.n == 0:
		return pane(t.st, t.name, t.st.OK.Render(t.empty), w, h, true)
	}
```

   with:

```go
		return pane(t.st, t.name, errorBody(t.st, err), w, h, true)
	case t.n == 0 && !t.loaded:
		return pane(t.st, t.name, t.st.Muted.Render("loading…"), w, h, true)
	case t.n == 0 && len(t.all) > 0:
		return pane(t.st, t.title(), t.st.Muted.Render("nothing matches /"+t.match+" · / then esc clears it"), w, h, true)
	case t.n == 0:
		return pane(t.st, t.name, t.st.OK.Render(t.empty), w, h, true)
	}
```

5. Replace:

```go
		ih -= bannerHeight
	}
	t.table.SetHeight(ih)
	list := pane(t.st, fmt.Sprintf("%s · %d", t.name, t.n), body+t.table.View(), listW, h, true)
	if t.preview == nil {
		return list
	}
```

   with:

```go
		ih -= bannerHeight
	}
	t.table.SetHeight(ih)
	list := pane(t.st, t.title(), body+t.table.View(), listW, h, true)
	if t.preview == nil {
		return list
	}
```

- [ ] **Step 8: Edit `internal/ui/run.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		switch {
		case a.mode == modeNormal && a.drawer.prompt():
			a.mode, a.drawer.open = modeInsert, true // brew is asking: let the user answer
		case a.mode == modeCommand && a.drawer.prompt():
			a.promptPending = true // answer once the command line closes
		}
		return readOutput(a.proc, m.id, m.attempt)
	case jobEOFMsg:
```

   with:

```go
		switch {
		case a.mode == modeNormal && a.drawer.prompt():
			a.mode, a.drawer.open = modeInsert, true // brew is asking: let the user answer
		case (a.mode == modeCommand || a.mode == modeFilter) && a.drawer.prompt():
			a.promptPending = true // answer once the line being typed closes
		}
		return readOutput(a.proc, m.id, m.attempt)
	case jobEOFMsg:
```

- [ ] **Step 9: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat(ui): / filters the list you're on"
```

---

### Task 7: gg goes to the top, gx opens the homepage, y copies the name

**Files:**
- Modify: `cmd/brewery/main.go`
- Modify: `internal/ui/actions_test.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`
- Modify: `internal/ui/components.go`
- Modify: `internal/ui/helpers_test.go`
- Modify: `internal/ui/installed_test.go`
- Modify: `internal/ui/jobsview.go`
- Create: `internal/ui/links.go`
- Create: `internal/ui/links_test.go`

**Interfaces:**
- **Consumes:** `selectedService`, `selectedTap`, `installed.selected`, `outdated.cursor`, `history`.
- **Produces:**
  - In `links.go`:
    - `openedMsg`, `copiedMsg`.
    - `(*App) UseOpener(func(url string) error)`, `(*App) UseClipboard(func(text string) error)`.
    - `(*App) gKey(k) (tea.Cmd, bool)`, `(*App) packageNamed(name, kind)`, `(*App) selection() (name, url string, ok bool)`.
    - `(*App) openHomepage()`, `(*App) copyName()`, `(*App) linkDone(tea.Msg)`.
  - `App.gPending`.
  - The table's own `g` binding is gone (`home` remains), so tests that pressed `g` once now press it twice.
  - `y` passes through the jobs view.
- **`main.go`:** `exec.Command("open", url).Run()` and `clipboard.WriteAll`.
- **Test helpers:** `fakeLinks`; `press` learns `home` and `space`.

`clipboard.WriteAll` makes `github.com/atotto/clipboard` a direct dependency, hence the tidy step.

- [ ] **Step 1: Edit `internal/ui/actions_test.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
func TestUninstallCask(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "f", "f", "g", "x") // apps: docker-desktop
	if a.confirm == nil || a.confirm.title != "Uninstall docker-desktop 4.39.0,184744?" {
		t.Fatalf("x should ask: %+v", a.confirm)
	}
```

   with:

```go
func TestUninstallCask(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "f", "f", "g", "g", "x") // apps: docker-desktop
	if a.confirm == nil || a.confirm.title != "Uninstall docker-desktop 4.39.0,184744?" {
		t.Fatalf("x should ask: %+v", a.confirm)
	}
```

2. Replace:

```go
func TestUninstallRefusesWhatOthersNeed(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "f", "f", "f", "g", "x") // deps filter, top row: json-c, used by git and pcre2
	if a.confirm == nil || a.confirm.title != "Can't uninstall json-c" {
		t.Fatalf("want an explanation, got %+v", a.confirm)
	}
```

   with:

```go
func TestUninstallRefusesWhatOthersNeed(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "f", "f", "f", "g", "g", "x") // deps filter, top row: json-c, used by git and pcre2
	if a.confirm == nil || a.confirm.title != "Can't uninstall json-c" {
		t.Fatalf("want an explanation, got %+v", a.confirm)
	}
```

3. Replace:

```go
	if want := []string{"unpin", "git"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	keys(t, a, "f", "f", "g", "p") // casks: docker-desktop
	if want := []string{"pin", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
```

   with:

```go
	if want := []string{"unpin", "git"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	keys(t, a, "f", "f", "g", "g", "p") // casks: docker-desktop
	if want := []string{"pin", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
```

- [ ] **Step 2: Edit `internal/ui/app_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	a.Update(press("2"))
	a.Update(press("f")) // all
	a.Update(press("g"))
	for range len(a.installed.rows) {
		if p, ok := a.installed.selected(); ok && p.Name == name {
			return
```

   with:

```go
	a.Update(press("2"))
	a.Update(press("f")) // all
	a.Update(press("g"))
	a.Update(press("g"))
	for range len(a.installed.rows) {
		if p, ok := a.installed.selected(); ok && p.Name == name {
			return
```

- [ ] **Step 3: Edit `internal/ui/helpers_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	r := []rune(k)[0]
	return tea.KeyPressMsg{Code: r, Text: k}
```

   with:

```go
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	r := []rune(k)[0]
	return tea.KeyPressMsg{Code: r, Text: k}
```

- [ ] **Step 4: Edit `internal/ui/installed_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Errorf("json-c preview should show it is a dependency used by git and pcre2:\n%s", out)
	}

	it.update(press("g")) // docker-desktop
	out = stripANSI(it.view(nil))
	if !strings.Contains(out, "4.95.0,242831 ↑") || strings.Contains(out, "Used by") {
		t.Errorf("cask preview should show the newer version and no dependency sections:\n%s", out)
```

   with:

```go
		t.Errorf("json-c preview should show it is a dependency used by git and pcre2:\n%s", out)
	}

	it.update(press("home")) // docker-desktop
	out = stripANSI(it.view(nil))
	if !strings.Contains(out, "4.95.0,242831 ↑") || strings.Contains(out, "Used by") {
		t.Errorf("cask preview should show the newer version and no dependency sections:\n%s", out)
```

- [ ] **Step 5: Create `internal/ui/links_test.go`**

```go
package ui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"brewery/internal/jobs"
)

// fakeLinks records the pages opened and the text copied.
type fakeLinks struct {
	opened, copied []string
	err            error
}

func (f *fakeLinks) open(url string) error  { f.opened = append(f.opened, url); return f.err }
func (f *fakeLinks) copy(text string) error { f.copied = append(f.copied, text); return f.err }

func newLinksApp(t *testing.T) (*App, *fakeLinks) {
	t.Helper()
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	l := &fakeLinks{}
	a.UseOpener(l.open)
	a.UseClipboard(l.copy)
	return a, l
}

func TestGGGoesToTheTop(t *testing.T) {
	a, _ := newLinksApp(t)
	keys(t, a, "2", "j", "j", "g", "g")
	if p, _ := a.installed.selected(); p.Name != "docker-desktop" {
		t.Errorf("gg should select the first row, got %s", p.Name)
	}
	keys(t, a, "g", "3")
	if a.tab != tabOutdated {
		t.Error("after g, any other key does what it always does")
	}
}

func TestGXOpensTheHomepage(t *testing.T) {
	a, l := newLinksApp(t)
	keys(t, a, "2", "j", "g", "x") // fd
	fd, _ := a.installed.selected()
	keys(t, a, "3", "g", "x") // lua-language-server, from its installed package
	lua, _ := a.packageNamed("lua-language-server", fd.Kind)
	keys(t, a, "5", "g", "x") // anomalyco/tap: its remote, without .git
	want := []string{fd.Homepage, lua.Homepage, "https://github.com/anomalyco/homebrew-tap"}
	if !reflect.DeepEqual(l.opened, want) {
		t.Errorf("opened %q, want %q", l.opened, want)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "opening https://github.com/anomalyco/homebrew-tap") {
		t.Errorf("status bar %q", bar)
	}
	keys(t, a, "j", "g", "x") // homebrew/core has no remote
	if bar := stripANSI(a.statusBar()); len(l.opened) != 3 || !strings.Contains(bar, "no web page for homebrew/core") {
		t.Errorf("opened %q, bar %q", l.opened, bar)
	}
}

func TestYCopiesTheName(t *testing.T) {
	a, l := newLinksApp(t)
	keys(t, a, "4", "y")
	if !reflect.DeepEqual(l.copied, []string{"postgresql@16"}) || !strings.Contains(stripANSI(a.statusBar()), "copied postgresql@16") {
		t.Errorf("copied %q, bar %q", l.copied, stripANSI(a.statusBar()))
	}
	a.UseHistory(jobs.History{Dir: t.TempDir()}, pastJobs())
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter", "y")
	if got := l.copied[len(l.copied)-1]; got != "brew upgrade fd" {
		t.Errorf("in :jobs y copies the command, got %q", got)
	}
}

func TestLinkErrorsShow(t *testing.T) {
	a, l := newLinksApp(t)
	l.err = errors.New("no browser")
	keys(t, a, "2", "g", "x")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "couldn't open the page: no browser") {
		t.Errorf("status bar %q", bar)
	}
	keys(t, a, "y")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "couldn't copy: no browser") {
		t.Errorf("status bar %q", bar)
	}
}
```

- [ ] **Step 6: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/links_test.go:26:4: a.UseOpener undefined (type *App has no field or method UseOpener)
    internal/ui/links_test.go:27:4: a.UseClipboard undefined (type *App has no field or method UseClipboard)
    internal/ui/links_test.go:48:14: a.packageNamed undefined (type *App has no field or method packageNamed)
```

- [ ] **Step 7: Edit `cmd/brewery/main.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/jobs"
```

   with:

```go
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"brewery/internal/brew"
	"brewery/internal/jobs"
```

2. Replace:

```go
		return fullScreen(exec.Command("less", append([]string{"+F", "--"}, files...)...))
	})

	crash, crashErr := startCrashLog(filepath.Join(jobs.DefaultStateDir(), "crash.log"))
	_, err = tea.NewProgram(app).Run()
	if crashErr == nil && crash.stop() {
```

   with:

```go
		return fullScreen(exec.Command("less", append([]string{"+F", "--"}, files...)...))
	})

	app.UseOpener(func(url string) error { return exec.Command("open", url).Run() }) // macOS's open
	app.UseClipboard(clipboard.WriteAll)

	crash, crashErr := startCrashLog(filepath.Join(jobs.DefaultStateDir(), "crash.log"))
	_, err = tea.NewProgram(app).Run()
	if crashErr == nil && crash.stop() {
```

- [ ] **Step 8: Edit `internal/ui/app.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go
	takeoverCmd func(args []string) *exec.Cmd // builds full-screen commands
	takingOver  *jobs.Job                     // the full-screen command running now

	readLog  func(path string) (string, error) // reads the end of a log; tests swap it
	log      logTail                           // the services preview's log
	pagerCmd func(files []string) *exec.Cmd    // builds the full-screen log viewer
```

   with:

```go
	takeoverCmd func(args []string) *exec.Cmd // builds full-screen commands
	takingOver  *jobs.Job                     // the full-screen command running now

	opener    func(url string) error  // opens a web page
	clipboard func(text string) error // copies text
	gPending  bool                    // g was pressed: the next key completes gg or gx

	readLog  func(path string) (string, error) // reads the end of a log; tests swap it
	log      logTail                           // the services preview's log
	pagerCmd func(files []string) *exec.Cmd    // builds the full-screen log viewer
```

2. Replace:

```go
		a.gotLog(msg)
	case pagerDoneMsg:
		a.pagerDone(msg)
	case completionsMsg:
		if msg.err == nil {
			a.comp = msg.comp // without them, only brewery's own commands complete
```

   with:

```go
		a.gotLog(msg)
	case pagerDoneMsg:
		a.pagerDone(msg)
	case openedMsg, copiedMsg:
		a.linkDone(msg)
	case completionsMsg:
		if msg.err == nil {
			a.comp = msg.comp // without them, only brewery's own commands complete
```

3. Replace:

```go
	if a.mode == modeFilter {
		return a.filterKey(msg)
	}
	if a.jobsOpen {
		if cmd, ok := a.jobsKey(msg); ok {
			return cmd
```

   with:

```go
	if a.mode == modeFilter {
		return a.filterKey(msg)
	}
	if a.gPending {
		a.gPending = false
		if cmd, ok := a.gKey(k); ok {
			return cmd
		}
	} else if k == "g" {
		a.gPending = true
		return nil
	}
	if a.jobsOpen {
		if cmd, ok := a.jobsKey(msg); ok {
			return cmd
```

4. Replace:

```go
		return a.openCmdline()
	case "/":
		return a.openFilter()
	case "ctrl+c":
		if a.queue.Current() != nil {
			return a.askStop()
```

   with:

```go
		return a.openCmdline()
	case "/":
		return a.openFilter()
	case "y":
		return a.copyName()
	case "ctrl+c":
		if a.queue.Current() != nil {
			return a.askStop()
```

- [ ] **Step 9: Edit `internal/ui/components.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
}

// tableKeys is the table's built-in Vim-style navigation, minus the default
// single-letter page keys (u, d, b, f) that brewery uses for actions.
func tableKeys() table.KeyMap {
	return table.KeyMap{
		LineUp:       key.NewBinding(key.WithKeys("up", "k")),
```

   with:

```go
}

// tableKeys is the table's built-in Vim-style navigation, minus the default
// single-letter page keys (u, d, b, f) that brewery uses for actions, and
// g: brewery reads gg (top) and gx (homepage) itself.
func tableKeys() table.KeyMap {
	return table.KeyMap{
		LineUp:       key.NewBinding(key.WithKeys("up", "k")),
```

2. Replace:

```go
		PageDown:     key.NewBinding(key.WithKeys("pgdown")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d")),
		GotoTop:      key.NewBinding(key.WithKeys("home", "g")),
		GotoBottom:   key.NewBinding(key.WithKeys("end", "G")),
	}
}
```

   with:

```go
		PageDown:     key.NewBinding(key.WithKeys("pgdown")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d")),
		GotoTop:      key.NewBinding(key.WithKeys("home")), // gg is handled by the app
		GotoBottom:   key.NewBinding(key.WithKeys("end", "G")),
	}
}
```

- [ ] **Step 10: Edit `internal/ui/jobsview.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
}

// jobsKey handles a key while the jobs view is open. ok is false for keys
// that still do what they do everywhere (tabs, :, o, r, ctrl+c); tab keys
// also close the view.
func (a *App) jobsKey(msg tea.KeyPressMsg) (cmd tea.Cmd, ok bool) {
	switch msg.String() {
```

   with:

```go
}

// jobsKey handles a key while the jobs view is open. ok is false for keys
// that still do what they do everywhere (tabs, :, /, y, o, r, ctrl+c); tab keys
// also close the view.
func (a *App) jobsKey(msg tea.KeyPressMsg) (cmd tea.Cmd, ok bool) {
	switch msg.String() {
```

2. Replace:

```go
	case "1", "2", "3", "4", "5", "[", "]":
		a.jobsOpen = false
		return nil, false
	case ":", "/", "o", "r", "ctrl+c":
		return nil, false
	}
	return a.jobs.update(msg), true
```

   with:

```go
	case "1", "2", "3", "4", "5", "[", "]":
		a.jobsOpen = false
		return nil, false
	case ":", "/", "y", "o", "r", "ctrl+c":
		return nil, false
	}
	return a.jobs.update(msg), true
```

- [ ] **Step 11: Create `internal/ui/links.go`**

```go
package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

// Messages from opening a homepage and copying a name.
type (
	openedMsg struct{ err error }
	copiedMsg struct {
		text string
		err  error
	}
)

// UseOpener sets how the app opens a web page; UseClipboard how it copies
// text. The UI never starts programs itself.
func (a *App) UseOpener(open func(url string) error)     { a.opener = open }
func (a *App) UseClipboard(copy func(text string) error) { a.clipboard = copy }

// gKey handles the key after g: g goes to the top of the list, x opens
// the selected item's homepage. ok is false for any other key, which then
// does what it always does.
func (a *App) gKey(k string) (cmd tea.Cmd, ok bool) {
	switch k {
	case "g":
		home := tea.KeyPressMsg{Code: tea.KeyHome}
		if a.jobsOpen {
			return a.jobs.update(home), true
		}
		switch a.tab {
		case tabInstalled:
			return a.installed.update(home), true
		case tabOutdated:
			return a.outdated.update(home), true
		case tabServices:
			return a.services.update(home), true
		case tabTaps:
			return a.taps.update(home), true
		}
		return nil, true
	case "x":
		return a.openHomepage(), true
	}
	return nil, false
}

// packageNamed finds an installed package by name and kind.
func (a *App) packageNamed(name string, kind brew.Kind) (brew.Package, bool) {
	for _, p := range a.snap.Packages {
		if p.Name == name && p.Kind == kind {
			return p, true
		}
	}
	return brew.Package{}, false
}

// selection is what the cursor is on in the current list: a name to copy
// and, where there is one, a web page.
func (a *App) selection() (name, url string, ok bool) {
	if a.jobsOpen {
		if i := a.jobs.cursor(); i >= 0 {
			return "brew " + strings.Join(a.history[i].Args, " "), "", true
		}
		return "", "", false
	}
	switch a.tab {
	case tabInstalled:
		if p, ok := a.installed.selected(); ok {
			return p.Name, p.Homepage, true
		}
	case tabOutdated:
		if i := a.outdated.cursor(); i >= 0 {
			o := a.snap.Outdated[i]
			p, _ := a.packageNamed(o.Name, o.Kind)
			return o.Name, p.Homepage, true
		}
	case tabServices:
		if s, ok := a.selectedService(); ok {
			p, _ := a.packageNamed(s.Name, brew.KindFormula)
			return s.Name, p.Homepage, true
		}
	case tabTaps:
		if t, ok := a.selectedTap(); ok {
			url := ""
			if strings.HasPrefix(t.Remote, "https://") {
				url = strings.TrimSuffix(t.Remote, ".git")
			}
			return t.Name, url, true
		}
	}
	return "", "", false
}

// openHomepage opens the selected item's web page in the browser.
func (a *App) openHomepage() tea.Cmd {
	name, url, ok := a.selection()
	switch {
	case !ok:
		return nil
	case url == "":
		return a.note("no web page for " + name)
	case a.opener == nil:
		return a.note("opening web pages isn't available")
	}
	a.flash = a.st.Info.Render("opening " + url)
	open := a.opener
	return func() tea.Msg { return openedMsg{err: open(url)} }
}

// copyName copies the selected item's name.
func (a *App) copyName() tea.Cmd {
	name, _, ok := a.selection()
	switch {
	case !ok:
		return nil
	case a.clipboard == nil:
		return a.note("copying isn't available")
	}
	copy := a.clipboard
	return func() tea.Msg { return copiedMsg{text: name, err: copy(name)} }
}

// linkDone reports how opening a page or copying went.
func (a *App) linkDone(msg tea.Msg) {
	switch m := msg.(type) {
	case openedMsg:
		if m.err != nil {
			a.flash = a.st.Err.Render("✗ couldn't open the page: " + m.err.Error())
		}
	case copiedMsg:
		if m.err != nil {
			a.flash = a.st.Err.Render("✗ couldn't copy: " + m.err.Error())
		} else {
			a.flash = a.st.OK.Render("copied " + m.text)
		}
	}
}
```

- [ ] **Step 12: Tidy the module (atotto/clipboard becomes a direct dependency)**

```bash
go mod tidy
```

- [ ] **Step 13: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 14: Commit**

```bash
git add -A
git commit -m "feat(ui): gg goes to the top, gx opens the homepage, y copies the name"
```

---

### Task 8: Tab or enter focuses the Installed preview, which then scrolls

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/installed.go`
- Create: `internal/ui/scroll.go`
- Create: `internal/ui/scroll_test.go`

**Interfaces:**
- **Consumes:** `installedTab.preview`, `pane`'s `focused` flag.
- **Produces:**
  - In `scroll.go`: `scroller{focus, off}`, `(*scroller) scroll(k string, page int) bool`, `(*scroller) window(body string, h int) string`.
  - `installedTab.pv`, `selectedKey()`, `followSelection(before)`.
  - `esc` unfocuses the preview first.
  - The Installed hint becomes `… f/F view · tab preview`, or `j/k scroll · tab back to the list · …` while the preview is focused.

The scroll resets when the selection changes: in `update` and `apply`, not at render. So `j` then `k` between frames still returns to the top.

- [ ] **Step 1: Create `internal/ui/scroll_test.go`**

```go
package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestScrollerWindow(t *testing.T) {
	var s scroller
	body := numbered(10)
	s.scroll("j", 3)
	s.scroll("ctrl+d", 3)
	if got := s.window(body, 4); !strings.HasPrefix(got, "line 5\n") {
		t.Errorf("4 lines down, got %q", got)
	}
	s.scroll("G", 3)
	if got := s.window(body, 4); got != "line 7\nline 8\nline 9\nline 10" || s.off != 6 {
		t.Errorf("G stops at the last screenful, got %q (off %d)", got, s.off)
	}
	s.scroll("k", 3)
	s.scroll("ctrl+u", 3)
	if s.off != 2 {
		t.Errorf("back up 4 lines: off %d", s.off)
	}
	s.scroll("home", 3)
	s.scroll("k", 3)
	if s.off != 0 {
		t.Errorf("never above the top: off %d", s.off)
	}
	if s.scroll("x", 3) {
		t.Error("x isn't a scrolling key")
	}
}

func TestInstalledPreviewTakesTheFocus(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 16})
	selectInstalled(t, a, "git")
	before := stripANSI(a.render())
	keys(t, a, "tab")
	if !a.installed.pv.focus || !strings.Contains(stripANSI(a.statusBar()), "j/k scroll · tab back to the list") {
		t.Fatalf("tab should focus the preview: %q", stripANSI(a.statusBar()))
	}
	keys(t, a, "j", "j", "j")
	if p, _ := a.installed.selected(); p.Name != "git" {
		t.Errorf("j scrolls the preview, the selection stays: %s", p.Name)
	}
	if after := stripANSI(a.render()); after == before || !strings.Contains(before, "git 2.51.0") || strings.Contains(after, "git 2.51.0") {
		t.Errorf("the preview should have scrolled past its first lines:\n%s", after)
	}
	keys(t, a, "esc")
	if a.installed.pv.focus {
		t.Error("esc goes back to the list")
	}
	keys(t, a, "j")
	if p, _ := a.installed.selected(); p.Name == "git" {
		t.Error("with the list focused, j moves the selection")
	}
	keys(t, a, "k", "enter")
	if !a.installed.pv.focus {
		t.Error("enter focuses the preview too")
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "git 2.51.0") {
		t.Errorf("coming back to git starts its preview at the top:\n%s", out)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/scroll_test.go:11:8: undefined: scroller
    internal/ui/scroll_test.go:43:18: a.installed.pv undefined (type installedTab has no field or method pv)
    internal/ui/scroll_test.go:54:17: a.installed.pv undefined (type installedTab has no field or method pv)
```

- [ ] **Step 3: Edit `internal/ui/actions.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	}
	switch a.tab {
	case tabInstalled:
		return "u upgrade · x uninstall · p pin · f/F view"
	case tabOutdated:
		return "u upgrade · A upgrade all"
	case tabServices:
```

   with:

```go
	}
	switch a.tab {
	case tabInstalled:
		if a.installed.pv.focus {
			return "j/k scroll · tab back to the list · u upgrade · x uninstall"
		}
		return "u upgrade · x uninstall · p pin · f/F view · tab preview"
	case tabOutdated:
		return "u upgrade · A upgrade all"
	case tabServices:
```

- [ ] **Step 4: Edit `internal/ui/app.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		}
		return a.quit()
	case "esc":
		a.drawer.open = false
		return nil
	case "o":
```

   with:

```go
		}
		return a.quit()
	case "esc":
		if a.tab == tabInstalled && a.installed.pv.focus {
			a.installed.pv.focus = false // back to the list first
			return nil
		}
		a.drawer.open = false
		return nil
	case "o":
```

- [ ] **Step 5: Edit `internal/ui/installed.go`** (6 replacements; each old block appears exactly once)

1. Replace:

```go
	ix     *state.Index
	rows   []brew.Package // the filtered rows, in table order
	match  string         // the "/" filter on names
	loaded bool
	table  table.Model
	w, h   int
```

   with:

```go
	ix     *state.Index
	rows   []brew.Package // the filtered rows, in table order
	match  string         // the "/" filter on names
	pv     scroller       // the preview, which tab or enter focuses
	loaded bool
	table  table.Model
	w, h   int
```

2. Replace:

```go
	for i, p := range t.rows {
		rows[i] = table.Row{p.Name, p.Version, p.Kind.String(), status(p)}
	}
	setRowsKeeping(&t.table, rows, installedKey)
}

// installedKey identifies an Installed row by name and type: a formula
```

   with:

```go
	for i, p := range t.rows {
		rows[i] = table.Row{p.Name, p.Version, p.Kind.String(), status(p)}
	}
	before := t.selectedKey()
	setRowsKeeping(&t.table, rows, installedKey)
	t.followSelection(before)
}

// selectedKey identifies the selected package, or is "" for none.
func (t *installedTab) selectedKey() string {
	p, ok := t.selected()
	if !ok {
		return ""
	}
	return p.Name + "\x00" + p.Kind.String()
}

// followSelection starts the preview at the top when the selection has
// moved on from before.
func (t *installedTab) followSelection(before string) {
	if t.selectedKey() != before {
		t.pv.off = 0
	}
}

// installedKey identifies an Installed row by name and type: a formula
```

3. Replace:

```go
}

func (t *installedTab) update(msg tea.KeyPressMsg) tea.Cmd {
	if k := msg.String(); k == "f" || k == "F" {
		step := installedFilter(1)
		if k == "F" {
			step = installedFilter(len(views) - 1)
```

   with:

```go
}

func (t *installedTab) update(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	switch {
	case k == "tab" || (k == "enter" && !t.pv.focus):
		t.pv.focus = !t.pv.focus
		return nil
	case t.pv.focus && k != "f" && k != "F":
		_, ih := paneInner(t.w-t.listWidth(), t.h)
		t.pv.scroll(k, ih/2)
		return nil
	}
	if k == "f" || k == "F" {
		step := installedFilter(1)
		if k == "F" {
			step = installedFilter(len(views) - 1)
```

4. Replace:

```go
		t.apply() // keeps the selected package if the new view shows it
		return nil
	}
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}
```

   with:

```go
		t.apply() // keeps the selected package if the new view shows it
		return nil
	}
	before := t.selectedKey()
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	t.followSelection(before)
	return cmd
}
```

5. Replace:

```go
	if t.match != "" {
		title += " · /" + t.match
	}
	list := pane(t.st, title, body+t.table.View(), listW, h, true)

	p, ok := t.selected()
	previewTitle, body := "Preview", t.st.Muted.Render("nothing in this view")
```

   with:

```go
	if t.match != "" {
		title += " · /" + t.match
	}
	list := pane(t.st, title, body+t.table.View(), listW, h, !t.pv.focus)

	p, ok := t.selected()
	previewTitle, body := "Preview", t.st.Muted.Render("nothing in this view")
```

6. Replace:

```go
	}
	switch {
	case ok:
		previewTitle, body = p.Name, t.preview(p)
	case !t.loaded && err == nil:
		body = t.st.Muted.Render("loading…")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, list, pane(t.st, previewTitle, body, w-listW, h, false))
}

// viewBar lists every view with its count, the active one highlighted,
```

   with:

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

- [ ] **Step 6: Create `internal/ui/scroll.go`**

```go
package ui

import "strings"

// scroller lets a preview scroll once it has the focus.
type scroller struct {
	focus bool // keys scroll the preview rather than move through the list
	off   int  // lines scrolled past
}

// scroll moves by a scrolling key, page lines for ctrl+d and ctrl+u. ok
// is false for any other key.
func (s *scroller) scroll(k string, page int) (ok bool) {
	switch k {
	case "j", "down":
		s.off++
	case "k", "up":
		s.off--
	case "ctrl+d":
		s.off += page
	case "ctrl+u":
		s.off -= page
	case "home":
		s.off = 0
	case "end", "G":
		s.off = 1 << 30 // window clamps it to the last screenful
	default:
		return false
	}
	s.off = max(s.off, 0)
	return true
}

// window is body from the scroll offset, for a pane h lines tall. It
// clamps the offset so the last line can't scroll out of view.
func (s *scroller) window(body string, h int) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	s.off = min(s.off, max(len(lines)-h, 0))
	return strings.Join(lines[s.off:], "\n")
}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(ui): tab or enter focuses the Installed preview, which then scrolls"
```

---

### Task 9: space marks rows on Installed and Outdated for u and x

**Files:**
- Modify: `internal/state/index.go`
- Modify: `internal/state/index_test.go`
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/installed.go`
- Modify: `internal/ui/list.go`
- Create: `internal/ui/marks.go`
- Create: `internal/ui/marks_test.go`

**Interfaces:**
- **Consumes:** `Index.UsedBy`, `kindArgs`, `App.run`, `packageQueries`.
- **Produces:**
  - `state.Index.Orphans(fullNames ...string)`: the existing one-name calls are unchanged.
  - In `marks.go`:
    - `markPrefix = "● "`, `marked`, `unmarked`, `markRow`, `pruneMarks`, `pkgKey`, `target`.
    - `(*App) toggleMark()`, `marks()`, `clearMarks()`, `byKind()`, `upgradeMarked()`, `askUninstallMarked()`.
    - `installedTab.markedPackages()`/`toggleMark()`, `listTab.toggleMark()`/`markedIdx()`.
  - Row keys (`installedKey`, `outdatedKey`) ignore the mark.
  - Marks on rows that disappear in a refresh are dropped.

Casks are never installed as dependencies, so orphans always join the formulae job.

- [ ] **Step 1: Edit `internal/state/index_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Error("libusb isn't top-level while a cask needs it")
	}
}
```

   with:

```go
		t.Error("libusb isn't top-level while a cask needs it")
	}
}

func TestOrphansOfSeveral(t *testing.T) {
	// jq and git both need json-c; removing only one leaves it needed.
	pkgs := append(slices.Clone(testPkgs),
		brew.Package{Name: "jq", FullName: "jq", Kind: brew.KindFormula, OnRequest: true, Deps: []string{"json-c"}, DirectDeps: []string{"json-c"}})
	ix := NewIndex(pkgs)
	if got := ix.Orphans("git", "jq"); !reflect.DeepEqual(got, []string{"json-c", "pcre2"}) {
		t.Errorf("Orphans(git, jq) = %v", got)
	}
	if got := ix.Orphans("git", "pcre2"); got != nil {
		t.Errorf("json-c is still needed by jq: %v", got)
	}
	if got := NewIndex(testPkgs).Orphans("git", "pcre2"); !reflect.DeepEqual(got, []string{"json-c"}) {
		t.Errorf("a package being removed isn't its own orphan: %v", got)
	}
}
```

- [ ] **Step 2: Create `internal/ui/marks_test.go`**

```go
package ui

import (
	"reflect"
	"strings"
	"testing"
)

func TestMarkAndUpgradeSeveral(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "space")           // docker-desktop; the cursor moves on to fd
	keys(t, a, "j", "j", "j", "space") // lua-language-server
	out := stripANSI(a.render())
	if !strings.Contains(out, "● docker-desktop") || !strings.Contains(out, "● lua-language-server") {
		t.Errorf("marked rows should show it:\n%s", out)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "2 marked") {
		t.Errorf("status bar %q", bar)
	}
	keys(t, a, "u")
	want := [][]string{{"upgrade", "lua-language-server"}, {"upgrade", "--cask", "docker-desktop"}}
	if !reflect.DeepEqual(fs.calls, want) {
		t.Errorf("ran %v, want %v (one job per kind)", fs.calls, want)
	}
	if a.marks() != 0 {
		t.Error("marks clear once their jobs are queued")
	}
}

func TestMarkedUpgradeSkipsWhatBrewWould(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "j", "space", "u") // fd is up to date
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "nothing marked needs upgrading") {
		t.Errorf("ran %v, bar %q", fs.calls, stripANSI(a.statusBar()))
	}
}

func TestMarkOnOutdated(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "3", "space", "space") // lua-language-server, docker-desktop
	a.sync()                          // a refresh keeps the marks
	if a.marks() != 2 {
		t.Fatalf("marks after refresh: %d", a.marks())
	}
	keys(t, a, "u")
	want := [][]string{{"upgrade", "lua-language-server"}, {"upgrade", "--cask", "docker-desktop"}}
	if !reflect.DeepEqual(fs.calls, want) {
		t.Errorf("ran %v, want %v", fs.calls, want)
	}
}

func TestUninstallSeveralWithTheirOrphans(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "f", "j", "j", "j", "space") // all: git
	keys(t, a, "j", "j", "space")                // pcre2, which only git needs
	keys(t, a, "x")
	if a.confirm == nil || a.confirm.title != "Uninstall 2 packages?" {
		t.Fatalf("x should ask: %+v", a.confirm)
	}
	out := stripANSI(a.render())
	for _, want := range []string{"git, pcre2", "That leaves 1 dependency nothing else needs:", "json-c"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	keys(t, a, "o")
	if want := []string{"uninstall", "git", "pcre2", "json-c"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUninstallSeveralRefusesWhatOthersNeed(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "f", "j", "j", "j", "j", "space", "x") // all: json-c, which git and pcre2 need
	if a.confirm == nil || a.confirm.title != "Can't uninstall these" {
		t.Fatalf("want an explanation, got %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "json-c is required by git, pcre2.") {
		t.Errorf("should say what needs it:\n%s", out)
	}
	keys(t, a, "esc", "esc")
	if a.marks() != 0 || len(fs.calls) != 0 {
		t.Errorf("esc closes the box, then clears the marks: %d marked, ran %v", a.marks(), fs.calls)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/state/index_test.go:145:30: too many arguments in call to ix.Orphans
    internal/state/index_test.go:148:30: too many arguments in call to ix.Orphans
    internal/state/index_test.go:151:46: too many arguments in call to NewIndex(testPkgs).Orphans
```

- [ ] **Step 4: Edit `internal/state/index.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
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
```

   with:

```go
	return slices.Clone(ix.usedBy[fullName])
}

// Orphans lists the dependencies that removing the given packages would
// leave unneeded: packages installed as dependencies (not on request) that
// are used only by those packages or by other such orphans. Sorted; nil
// when there are none.
func (ix *Index) Orphans(fullNames ...string) []string {
	gone := map[string]bool{}
	for _, n := range fullNames {
		gone[n] = true
	}
	cand := map[string]bool{}
	for _, n := range fullNames {
		for _, d := range ix.byFull[n].Deps {
			if dp, ok := ix.byFull[d]; ok && !dp.OnRequest && !gone[d] {
				cand[d] = true
			}
		}
	}
	// Drop any candidate something else still needs, until nothing changes.
```

2. Replace:

```go
		changed = false
		for c := range cand {
			for _, u := range ix.usedBy[c] {
				if u != fullName && !cand[u] {
					delete(cand, c)
					changed = true
					break
```

   with:

```go
		changed = false
		for c := range cand {
			for _, u := range ix.usedBy[c] {
				if !gone[u] && !cand[u] {
					delete(cand, c)
					changed = true
					break
```

3. Replace:

```go
			}
		}
	}
	return slices.Sorted(maps.Keys(cand))
}
```

   with:

```go
			}
		}
	}
	if len(cand) == 0 {
		return nil
	}
	return slices.Sorted(maps.Keys(cand))
}
```

- [ ] **Step 5: Edit `internal/ui/actions.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		return a.run(action{title: "update", args: []string{"update"}, full: true}), true
	case k == "c":
		return a.askCleanup(), true
	case k == "u" && (a.tab == tabInstalled || a.tab == tabOutdated):
		return a.upgradeSelected(), true
	case k == "x" && a.tab == tabInstalled:
		return a.askUninstall(), true
	case k == "p" && a.tab == tabInstalled:
		return a.togglePin(), true
```

   with:

```go
		return a.run(action{title: "update", args: []string{"update"}, full: true}), true
	case k == "c":
		return a.askCleanup(), true
	case k == "space" && (a.tab == tabInstalled || a.tab == tabOutdated):
		return a.toggleMark(), true
	case k == "u" && (a.tab == tabInstalled || a.tab == tabOutdated):
		if a.marks() > 0 {
			return a.upgradeMarked(), true
		}
		return a.upgradeSelected(), true
	case k == "x" && a.tab == tabInstalled:
		if a.marks() > 0 {
			return a.askUninstallMarked(), true
		}
		return a.askUninstall(), true
	case k == "p" && a.tab == tabInstalled:
		return a.togglePin(), true
```

- [ ] **Step 6: Edit `internal/ui/app.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
		}
		return a.quit()
	case "esc":
		if a.tab == tabInstalled && a.installed.pv.focus {
			a.installed.pv.focus = false // back to the list first
			return nil
		}
		a.drawer.open = false
		return nil
	case "o":
		if a.drawer.job != nil {
```

   with:

```go
		}
		return a.quit()
	case "esc":
		switch {
		case a.tab == tabInstalled && a.installed.pv.focus:
			a.installed.pv.focus = false // back to the list first
		case a.marks() > 0 && !a.drawerShown():
			a.clearMarks()
		default:
			a.drawer.open = false
		}
		return nil
	case "o":
		if a.drawer.job != nil {
```

2. Replace:

```go
			left += " › " + p.Name
		}
	}

	var right []string
	if a.flash != "" {
```

   with:

```go
			left += " › " + p.Name
		}
	}
	if n := a.marks(); n > 0 && !a.jobsOpen {
		left += fmt.Sprintf(" · %d marked", n)
	}

	var right []string
	if a.flash != "" {
```

- [ ] **Step 7: Edit `internal/ui/installed.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
	filter installedFilter
	all    []brew.Package
	ix     *state.Index
	rows   []brew.Package // the filtered rows, in table order
	match  string         // the "/" filter on names
	pv     scroller       // the preview, which tab or enter focuses
	loaded bool
	table  table.Model
	w, h   int
```

   with:

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

2. Replace:

```go

func (t *installedTab) setData(pkgs []brew.Package, ix *state.Index) {
	t.all, t.ix = pkgs, ix
	t.apply()
}
```

   with:

```go

func (t *installedTab) setData(pkgs []brew.Package, ix *state.Index) {
	t.all, t.ix = pkgs, ix
	pruneMarks(t.marked, func(yield func(string) bool) {
		for _, p := range pkgs {
			if !yield(pkgKey(p)) {
				return
			}
		}
	})
	t.apply()
}
```

3. Replace:

```go
	})
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{p.Name, p.Version, p.Kind.String(), status(p)}
	}
	before := t.selectedKey()
	setRowsKeeping(&t.table, rows, installedKey)
```

   with:

```go
	})
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{marked(p.Name, t.marked[pkgKey(p)]), p.Version, p.Kind.String(), status(p)}
	}
	before := t.selectedKey()
	setRowsKeeping(&t.table, rows, installedKey)
```

4. Replace:

```go
	if !ok {
		return ""
	}
	return p.Name + "\x00" + p.Kind.String()
}

// followSelection starts the preview at the top when the selection has
```

   with:

```go
	if !ok {
		return ""
	}
	return pkgKey(p)
}

// followSelection starts the preview at the top when the selection has
```

5. Replace:

```go

// installedKey identifies an Installed row by name and type: a formula
// and a cask can share a name.
func installedKey(r table.Row) string { return r[0] + "\x00" + r[2] }

func status(p brew.Package) string {
	var s []string
```

   with:

```go

// installedKey identifies an Installed row by name and type: a formula
// and a cask can share a name.
func installedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[2] }

func status(p brew.Package) string {
	var s []string
```

- [ ] **Step 8: Edit `internal/ui/list.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go
	table  table.Model
	w, h   int

	all      []table.Row // every row, before the filter
	idx      []int       // for each row shown, its index in all
	match    string      // the "/" filter
	matchCol int         // the column the filter looks in

	// preview, if set, fills a pane beside the table with what it returns
	// for a w×h pane.
```

   with:

```go
	table  table.Model
	w, h   int

	all      []table.Row     // every row, before the filter
	idx      []int           // for each row shown, its index in all
	match    string          // the "/" filter
	matchCol int             // the column the filter looks in
	marked   map[string]bool // rows picked with space, by key

	// preview, if set, fills a pane beside the table with what it returns
	// for a w×h pane.
```

2. Replace:

```go

func (t *listTab) setRows(rows []table.Row) {
	t.all = rows
	t.apply()
}
```

   with:

```go

func (t *listTab) setRows(rows []table.Row) {
	t.all = rows
	pruneMarks(t.marked, func(yield func(string) bool) {
		for _, r := range rows {
			if !yield(t.key(r)) {
				return
			}
		}
	})
	t.apply()
}
```

3. Replace:

```go
	t.idx = t.idx[:0]
	for i, r := range t.all {
		if matches(r[t.matchCol], t.match) {
			shown = append(shown, r)
			t.idx = append(t.idx, i)
		}
	}
```

   with:

```go
	t.idx = t.idx[:0]
	for i, r := range t.all {
		if matches(r[t.matchCol], t.match) {
			shown = append(shown, markRow(r, t.marked[t.key(r)]))
			t.idx = append(t.idx, i)
		}
	}
```

4. Replace:

```go
func firstCell(r table.Row) string { return r[0] }

// outdatedKey identifies an Outdated row by name and type.
func outdatedKey(r table.Row) string { return r[0] + "\x00" + r[3] }

func (t *listTab) update(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
```

   with:

```go
func firstCell(r table.Row) string { return r[0] }

// outdatedKey identifies an Outdated row by name and type.
func outdatedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[3] }

func (t *listTab) update(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
```

- [ ] **Step 9: Create `internal/ui/marks.go`**

```go
package ui

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

// markPrefix flags a row picked with space. Row keys ignore it.
const markPrefix = "● "

// marked shows a row's name with the mark when on is true.
func marked(name string, on bool) string {
	if on {
		return markPrefix + name
	}
	return name
}

// unmarked is a name cell without its mark.
func unmarked(cell string) string { return strings.TrimPrefix(cell, markPrefix) }

// toggleMark marks or unmarks the row under the cursor on Installed or
// Outdated, then moves down, so space can mark several in a row.
func (a *App) toggleMark() tea.Cmd {
	down := tea.KeyPressMsg{Code: tea.KeyDown}
	if a.tab == tabInstalled {
		a.installed.toggleMark()
		return a.installed.update(down)
	}
	a.outdated.toggleMark()
	return a.outdated.update(down)
}

// marks is how many rows are marked on the current tab.
func (a *App) marks() int {
	switch a.tab {
	case tabInstalled:
		return len(a.installed.marked)
	case tabOutdated:
		return len(a.outdated.marked)
	}
	return 0
}

// clearMarks unmarks every row on the current tab.
func (a *App) clearMarks() {
	switch a.tab {
	case tabInstalled:
		a.installed.marked = nil
		a.installed.apply()
	case tabOutdated:
		a.outdated.marked = nil
		a.outdated.apply()
	}
}

// target is a package an action names, with its kind.
type target struct {
	name string
	kind brew.Kind
}

// byKind runs one action per kind present in ts, since casks need --cask:
// formulae first. build makes the action for one kind's names.
func (a *App) byKind(ts []target, build func(kind brew.Kind, names []string) action) tea.Cmd {
	var cmds []tea.Cmd
	for _, kind := range []brew.Kind{brew.KindFormula, brew.KindCask} {
		var names []string
		for _, t := range ts {
			if t.kind == kind {
				names = append(names, t.name)
			}
		}
		if len(names) > 0 {
			cmds = append(cmds, a.run(build(kind, names)))
		}
	}
	return tea.Batch(cmds...)
}

// upgradeMarked upgrades the marked packages that brew would: outdated and
// not pinned.
func (a *App) upgradeMarked() tea.Cmd {
	var ts []target
	if a.tab == tabInstalled {
		for _, p := range a.installed.markedPackages() {
			if p.Outdated && !p.Pinned {
				ts = append(ts, target{p.FullName, p.Kind})
			}
		}
	} else {
		for _, i := range a.outdated.markedIdx() {
			if o := a.snap.Outdated[i]; !o.Pinned {
				ts = append(ts, target{o.Name, o.Kind})
			}
		}
	}
	if len(ts) == 0 {
		return a.note("nothing marked needs upgrading (pinned or up to date)")
	}
	a.clearMarks()
	return a.byKind(ts, func(kind brew.Kind, names []string) action {
		return action{title: "upgrade " + strings.Join(names, " "), args: kindArgs("upgrade", kind, names...), refresh: packageQueries}
	})
}

// askUninstallMarked confirms uninstalling the marked packages. If an
// unmarked package still needs one, it explains why brew would refuse.
func (a *App) askUninstallMarked() tea.Cmd {
	pkgs := a.installed.markedPackages()
	gone := map[string]bool{}
	var names, full []string
	for _, p := range pkgs {
		gone[p.FullName] = true
		names = append(names, p.Name)
		full = append(full, p.FullName)
	}
	var blocked []string
	for _, p := range pkgs {
		var by []string
		for _, u := range a.ix.UsedBy(p.FullName) {
			if !gone[u] {
				by = append(by, u)
			}
		}
		if len(by) > 0 {
			blocked = append(blocked, p.Name+" is required by "+strings.Join(by, ", ")+".")
		}
	}
	if len(blocked) > 0 {
		a.confirm = &confirmBox{
			title:   "Can't uninstall these",
			body:    append(blocked, "Unmark it, or mark those too."),
			choices: []choice{{"n", "close", nil}},
		}
		return nil
	}
	refresh := append(slices.Clone(packageQueries), qServices)
	ts := make([]target, len(pkgs))
	for i, p := range pkgs {
		ts[i] = target{p.FullName, p.Kind}
	}
	run := func(extra []string) func() tea.Cmd {
		return func() tea.Cmd {
			a.clearMarks()
			all := ts
			for _, o := range extra {
				all = append(all, target{o, brew.KindFormula}) // only formulae are installed as dependencies
			}
			return a.byKind(all, func(kind brew.Kind, names []string) action {
				return action{title: "uninstall " + strings.Join(names, " "), args: kindArgs("uninstall", kind, names...), refresh: refresh}
			})
		}
	}
	box := &confirmBox{
		title:   fmt.Sprintf("Uninstall %d packages?", len(pkgs)),
		body:    []string{strings.Join(names, ", ")},
		choices: []choice{{"y", "uninstall", run(nil)}},
	}
	if orphans := a.ix.Orphans(full...); len(orphans) > 0 {
		noun := "dependency"
		if len(orphans) > 1 {
			noun = "dependencies"
		}
		box.body = append(box.body, "", fmt.Sprintf("That leaves %d %s nothing else needs:", len(orphans), noun), strings.Join(orphans, ", "))
		box.choices = append(box.choices, choice{"o", "uninstall + orphans", run(orphans)})
	}
	box.choices = append(box.choices, choice{"n", "cancel", nil})
	a.confirm = box
	return nil
}

// markedPackages are the marked packages, by name.
func (t *installedTab) markedPackages() []brew.Package {
	var out []brew.Package
	for _, p := range t.all {
		if t.marked[pkgKey(p)] {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b brew.Package) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// pkgKey identifies a package by name and kind: a formula and a cask can
// share a name.
func pkgKey(p brew.Package) string { return p.Name + "\x00" + p.Kind.String() }

// toggleMark marks or unmarks the selected package.
func (t *installedTab) toggleMark() {
	p, ok := t.selected()
	if !ok {
		return
	}
	if t.marked == nil {
		t.marked = map[string]bool{}
	}
	if k := pkgKey(p); t.marked[k] {
		delete(t.marked, k)
	} else {
		t.marked[k] = true
	}
	t.apply()
}

// toggleMark marks or unmarks the row under the cursor.
func (t *listTab) toggleMark() {
	row := t.table.SelectedRow()
	if t.n == 0 || row == nil {
		return
	}
	if t.marked == nil {
		t.marked = map[string]bool{}
	}
	if k := t.key(row); t.marked[k] {
		delete(t.marked, k)
	} else {
		t.marked[k] = true
	}
	t.apply()
}

// markedIdx are the marked rows' indexes into the slice the rows were
// built from.
func (t *listTab) markedIdx() []int {
	var out []int
	for i, r := range t.all {
		if t.marked[t.key(r)] {
			out = append(out, i)
		}
	}
	return out
}

// pruneMarks drops marks on rows that are gone.
func pruneMarks(m map[string]bool, keys func(yield func(string) bool)) {
	have := map[string]bool{}
	for k := range keys {
		have[k] = true
	}
	for k := range maps.Keys(m) {
		if !have[k] {
			delete(m, k)
		}
	}
}

// markRow returns r with its first cell marked when on is true.
func markRow(r table.Row, on bool) table.Row {
	if !on {
		return r
	}
	r = slices.Clone(r)
	r[0] = marked(r[0], true)
	return r
}
```

- [ ] **Step 10: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "feat(ui): space marks rows on Installed and Outdated for u and x"
```

---

### Task 10: ? shows the keys for where you are

**Files:**
- Modify: `internal/ui/app.go`
- Create: `internal/ui/help.go`
- Create: `internal/ui/help_test.go`
- Modify: `internal/ui/jobsview.go`

**Interfaces:**
- **Consumes:** the keys from Tasks 6–9; `overlay`.
- **Produces:**
  - In `help.go`: `helpGroup`, `(*App) helpGroups()`, `helpKeyW = 11`, `(*App) helpView(maxW, maxH int) string`.
  - `App.help`; `?` opens it, and it passes through the jobs view.
  - The status-bar hints gain `? help`.

- [ ] **Step 1: Create `internal/ui/help_test.go`**

```go
package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestHelpListsTheKeysForWhereYouAre(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "?")
	out := stripANSI(a.render())
	for _, want := range []string{"Installed", "space", "mark for u/x on several", "gx", "open the homepage", "Everywhere", "any key closes this"} {
		if !strings.Contains(out, want) {
			t.Errorf("Installed help is missing %q:\n%s", want, out)
		}
	}
	keys(t, a, "u")
	if a.help || len(fs.calls) != 0 {
		t.Errorf("a key closes help and does nothing else: help %v, ran %v", a.help, fs.calls)
	}
	keys(t, a, "4", "?")
	if out := stripANSI(a.render()); !strings.Contains(out, "follow the log full screen") || strings.Contains(out, "uninstall") {
		t.Errorf("Services help:\n%s", out)
	}
}

func TestHelpInTheJobsView(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter", "?")
	if out := stripANSI(a.render()); !a.help || !strings.Contains(out, "replay the job's output") {
		t.Errorf("? works in :jobs too:\n%s", out)
	}
}

func TestHelpHintAndSmallWindows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "? help") {
		t.Errorf("the status bar should mention ? help: %q", bar)
	}
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	keys(t, a, "2", "?")
	out := stripANSI(a.render())
	if h := len(strings.Split(out, "\n")); h != 24 {
		t.Errorf("help must fit the window: %d lines", h)
	}
	for _, want := range []string{"space", "gx", "Everywhere", "quit", "any key closes this"} {
		if !strings.Contains(out, want) {
			t.Errorf("at 80x24 help goes to two columns and keeps %q:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/help_test.go:21:7: a.help undefined (type *App has no field or method help)
    internal/ui/help_test.go:22:74: a.help undefined (type *App has no field or method help)
    internal/ui/help_test.go:35:38: a.help undefined (type *App has no field or method help)
```

- [ ] **Step 3: Edit `internal/ui/app.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
	cmd      cmdline         // the ":" prompt
	filterIn textinput.Model // the "/" prompt
	confirm  *confirmBox     // open confirmation, or nil
	flash    string          // one-off status message, cleared by the next key

	comp     brew.Completions // brew's commands, for completing the prompt
```

   with:

```go
	cmd      cmdline         // the ":" prompt
	filterIn textinput.Model // the "/" prompt
	confirm  *confirmBox     // open confirmation, or nil
	help     bool            // the ? overlay is open
	flash    string          // one-off status message, cleared by the next key

	comp     brew.Completions // brew's commands, for completing the prompt
```

2. Replace:

```go
	if a.mode != modeCommand {
		a.flash = "" // in COMMAND the bar shows only the prompt: keep it for when the line closes
	}
	if a.confirm != nil {
		ch, ok := a.confirm.answer(k)
		if !ok {
```

   with:

```go
	if a.mode != modeCommand {
		a.flash = "" // in COMMAND the bar shows only the prompt: keep it for when the line closes
	}
	if a.help {
		a.help = false // any key closes help, and does nothing else
		return nil
	}
	if a.confirm != nil {
		ch, ok := a.confirm.answer(k)
		if !ok {
```

3. Replace:

```go
		return a.openCmdline()
	case "/":
		return a.openFilter()
	case "y":
		return a.copyName()
	case "ctrl+c":
```

   with:

```go
		return a.openCmdline()
	case "/":
		return a.openFilter()
	case "?":
		a.help = true
		return nil
	case "y":
		return a.copyName()
	case "ctrl+c":
```

4. Replace:

```go
	if a.confirm != nil {
		body = overlay(body, a.confirm.view(a.st, a.width), a.width, bodyH)
	}
	parts := []string{a.tabBar(), body}
	if a.drawerShown() {
		parts = append(parts, a.drawer.view(a.width, a.bodyHeight()-bodyH, a.now(), a.frame, a.mode == modeInsert))
```

   with:

```go
	if a.confirm != nil {
		body = overlay(body, a.confirm.view(a.st, a.width), a.width, bodyH)
	}
	if a.help {
		body = overlay(body, a.helpView(a.width, bodyH), a.width, bodyH)
	}
	parts := []string{a.tabBar(), body}
	if a.drawerShown() {
		parts = append(parts, a.drawer.view(a.width, a.bodyHeight()-bodyH, a.now(), a.frame, a.mode == modeInsert))
```

5. Replace:

```go
	if a.jobsOpen {
		quit = "close" // q closes the jobs view
	}
	hints := a.actionHints() + " · : command · q " + quit
	if a.drawer.job != nil && !a.drawer.open {
		hints = "o output · " + hints
	}
```

   with:

```go
	if a.jobsOpen {
		quit = "close" // q closes the jobs view
	}
	hints := a.actionHints() + " · : command · ? help · q " + quit
	if a.drawer.job != nil && !a.drawer.open {
		hints = "o output · " + hints
	}
```

- [ ] **Step 4: Create `internal/ui/help.go`**

```go
package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// helpGroup is one section of the ? overlay: a heading and its keys.
type helpGroup struct {
	title string
	keys  [][2]string // key, what it does
}

// helpGroups are the keys that work where the user is now: the current
// tab's (or the jobs view's) first, then lists', then everywhere's.
func (a *App) helpGroups() []helpGroup {
	var here helpGroup
	switch {
	case a.jobsOpen:
		here = helpGroup{"Jobs", [][2]string{{"enter", "replay the job's output"}, {"esc q", "close"}}}
	case a.tab == tabHome:
		here = helpGroup{"Home", [][2]string{{"h l", "move between tiles"}, {"enter", "open the tile's tab (Cleanup: clean up)"}}}
	case a.tab == tabInstalled:
		here = helpGroup{"Installed", [][2]string{
			{"u", "upgrade"}, {"x", "uninstall"}, {"p", "pin or unpin"}, {"space", "mark for u/x on several"},
			{"f F", "next or previous view"}, {"tab enter", "preview, then j/k scroll"},
		}}
	case a.tab == tabOutdated:
		here = helpGroup{"Outdated", [][2]string{{"u", "upgrade"}, {"A", "upgrade all"}, {"space", "mark for u on several"}}}
	case a.tab == tabServices:
		here = helpGroup{"Services", [][2]string{{"s", "start"}, {"S", "stop"}, {"R", "restart"}, {"l", "follow the log full screen"}}}
	case a.tab == tabTaps:
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	}
	groups := []helpGroup{here}
	if a.jobsOpen || a.tab != tabHome {
		groups = append(groups, helpGroup{"Lists", [][2]string{
			{"j k", "move"}, {"gg G", "top, bottom"}, {"ctrl+d/u", "half a page"},
			{"/", "filter by name"}, {"y", "copy the name"}, {"gx", "open the homepage"},
		}})
	}
	return append(groups, helpGroup{"Everywhere", [][2]string{
		{"1-5 [ ]", "switch tabs"}, {":", "run any brew command"}, {"r", "refresh"}, {"U", "brew update"},
		{"c", "clean up"}, {"o", "show or hide output"}, {"i", "type into the running job"},
		{"ctrl+c", "stop the running job"}, {"q", "quit"},
	}})
}

// helpKeyW is the width of the key column.
const helpKeyW = 11

// helpView renders the ? overlay in at most maxW×maxH: one column, or two
// side by side (this tab and lists, then everywhere) when one is too tall.
func (a *App) helpView(maxW, maxH int) string {
	st := a.st
	groups := a.helpGroups()
	block := func(gs []helpGroup, w int) []string {
		var lines []string
		for i, g := range gs {
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, st.Accent.Bold(true).Render(g.title))
			for _, k := range g.keys {
				key := st.Title.Render(k[0]) + strings.Repeat(" ", max(helpKeyW-lipgloss.Width(k[0]), 1))
				lines = append(lines, ansi.Truncate(key+k[1], w, "…"))
			}
		}
		return lines
	}
	footer := st.Muted.Render("any key closes this")
	inner := min(56, max(maxW-4, 20))
	lines := append(block(groups, inner), "", footer)
	if len(lines)+2 > maxH && len(groups) > 1 {
		colW := max((maxW-4-3)/2, 20) // two columns and the gap between
		left := block(groups[:len(groups)-1], colW)
		right := append(block(groups[len(groups)-1:], colW), "", footer)
		lines = strings.Split(lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(colW).Render(strings.Join(left, "\n")), "   ",
			strings.Join(right, "\n")), "\n")
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(st.P.Mauve).
		Padding(0, 1).Render(strings.Join(lines, "\n"))
}
```

- [ ] **Step 5: Edit `internal/ui/jobsview.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
}

// jobsKey handles a key while the jobs view is open. ok is false for keys
// that still do what they do everywhere (tabs, :, /, y, o, r, ctrl+c); tab keys
// also close the view.
func (a *App) jobsKey(msg tea.KeyPressMsg) (cmd tea.Cmd, ok bool) {
	switch msg.String() {
```

   with:

```go
}

// jobsKey handles a key while the jobs view is open. ok is false for keys
// that still do what they do everywhere (tabs, :, /, ?, y, o, r, ctrl+c); tab keys
// also close the view.
func (a *App) jobsKey(msg tea.KeyPressMsg) (cmd tea.Cmd, ok bool) {
	switch msg.String() {
```

2. Replace:

```go
	case "1", "2", "3", "4", "5", "[", "]":
		a.jobsOpen = false
		return nil, false
	case ":", "/", "y", "o", "r", "ctrl+c":
		return nil, false
	}
	return a.jobs.update(msg), true
```

   with:

```go
	case "1", "2", "3", "4", "5", "[", "]":
		a.jobsOpen = false
		return nil, false
	case ":", "/", "?", "y", "o", "r", "ctrl+c":
		return nil, false
	}
	return a.jobs.update(msg), true
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(ui): ? shows the keys for where you are"
```

---

### Task 11: End-to-end check in a real terminal

**Files:** none (verification only)

- [ ] **Step 1: Build and run**

Run: `make run`

- [ ] **Step 2: Check each of these by hand.** None of these change anything on your system, except steps 4 and 5, which only open a browser tab and set the clipboard.
  1. **Filter:** press `2`, then `/`, and type `git`. The list narrows as you type, the badge says FILTER, and the pane title says `/git`. Press `enter`: the filter stays. Press `/` and then `esc`: it's gone.
  2. **Preview:** select a package with caveats, press `tab`, then `j`. The preview scrolls if it's longer than the pane. Press `esc` to go back to the list.
  3. **Marks:** press `f` for All, then `space` twice. Two rows show `●`, and the status bar says `2 marked`. Press `esc`: the marks clear.
  4. **Homepage:** press `gx` on a package. Its homepage opens in your browser.
  5. **Copy:** press `y`. The status bar says `copied <name>`; paste somewhere to check.
  6. **Help:** press `?`. The keys for the tab show, and any key closes them. Press `gg` on Outdated: the cursor goes to the top.
  7. **Small window:** shrink the terminal to about 80×24 and run `:doctor`. The drawer shows about 6 lines of output, and nothing spills off the bottom.

- [ ] **Step 3 (optional; this changes your system): several at once**

On Outdated, mark two packages with `space` and press `u`. One job per kind runs (formulae, then casks).

- [ ] **Step 4: Final checks**

```bash
go test -race -count=1 ./...
go test -tags live -count=1 ./internal/brew/ ./internal/ui/
go vet ./...
gofmt -l .
go mod tidy -diff
```

Expected: all `ok`, with no output from vet, gofmt, or `go mod tidy -diff`.
