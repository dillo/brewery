# brewery Plan 3: Command line — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run any brew command from brewery. `:` opens a command line with a completion popup (commands, subcommands, flags, and your installed packages, taps and services). Editors and shells take over the whole terminal. Every finished job is kept in a history on disk: `:jobs` lists it and replays any job's output, and Home shows the recent ones.

**Architecture:**
- **`internal/jobs/history.go`** stores finished jobs as one JSON file each, newest 100 kept. It is pure file I/O, no Bubble Tea.
- **`internal/brew/completions.go`** reads the fish completion script brew ships, which describes every command, subcommand and flag, and names the kind of argument each takes.
- **`internal/ui`** adds a COMMAND mode with a line editor (`bubbles/textinput`), a completion popup drawn over the bottom of the screen, a `:jobs` view, and full-screen takeover through Bubble Tea's `ExecProcess`.
- **The UI still never runs brew itself.** `main` hands it the history store, a completions loader and a takeover command builder, next to the `Starter` from Plan 2.

**Tech Stack:**
- Go ≥ 1.25
- Bubble Tea v2, Bubbles v2 (`textinput`, new here), Lip Gloss v2
- `textinput` brings in `github.com/atotto/clipboard` v0.1.4 (Charm's dependency, for ctrl+v)

**Spec:** `docs/superpowers/specs/2026-10-12-brewery-design.md`, section 3 (the `:` command line, takeover, history and completion) and the COMMAND mode from section 4.

**Roadmap change (decided with the user):** the old Plan 3 outline is split in two again, so each plan stays about Plan 2's size.
- **This plan, "Command line":** `:` with completion, takeover, job history with `:jobs`, Home's recent jobs, and a fix for brew 7's own confirmation prompt.
- **Plan 4, "Services, cleanup and follow-ups":** the `l` service log (from `brew services info --all --json`, which has `log_path`), cleanup with a `--dry-run` preview, untap, and the Plan 2 follow-ups (a job whose grandchild keeps the pseudo-terminal open, exit 0 after ctrl+c in INSERT labelled "stopped", orphans that ignore cask dependencies, `serviceKey`'s row-order coupling, and the missing multi-round orphans and cask-uninstall tests).
- **Plan 5, "Polish and ship":** the old Polish & ship outline, plus the Plan 2 items that concern the plan's design (quit sends SIGKILL; only 4 output rows at 80×24; the drawer overflowing 8–9-row windows; `IsLockError` matching anywhere in the output).

**Two departures from the spec, both found while building this:**
- **Completion data comes from brew's fish completion script** (`<brew --repository>/completions/fish/brew.fish`), not from `brew commands --quiet` plus parsing `brew <cmd> --help`. The script already holds every command's and flag's description, its subcommands, and the kind of argument each takes; reading it takes 0.02s, where running `--help` for all 118 commands takes about 40s. So nothing needs caching per brew version.
- **A job's log lives inside its history record** (the last 128 KiB of output) rather than in a separate log file.

**Found while building this: brew 7 asks for confirmation itself.** `brew upgrade` and `brew install` now print `==> Do you want to proceed with the upgrade? [y/n]`, end the line, and wait for one key, whenever they run in a terminal (the drawer is one). Plan 2's prompt detection only looked at the cursor's own line, so the drawer never switched to INSERT. Task 1 fixes the detection. It also skips brew's question for `A` (upgrade all), where brewery's dialog already asked; the user chose this. `HOMEBREW_NO_ASK=1` is used rather than `--no-ask`, because older brews ignore an unknown variable but reject an unknown flag.

**Verified:** every code block in this plan was compiled and tested before the plan was written.
- **The edits are exact.** A checker applied this plan's edits to `master` (69a6581) and confirmed each task reproduces the tested code byte for byte, that each task's tests fail before its code and pass after, and that `go mod tidy` stays clean. Every task's commit passes `go test -race` on its own.
- **Against real brew:** the completion parser read the real fish script (live test). The real binary was driven in a pseudo-terminal, with its history and cache pointed at a scratch directory: the `:up` popup, a read-only `:info fd` job in the drawer, `:jobs` and replaying it, a `:!--version` takeover returning to an intact screen, and Home listing both jobs.

## Global Constraints

These come from Plans 1 and 2 and still hold:
- Module path `brewery`; Go ≥ 1.25.
- Only `internal/brew` and `main` run brew. `internal/ui` gets data through `Source`, starts jobs through `Starter`, and now also takes a history store, a completions loader and a takeover command builder from `main`.
- Views use `theme.Styles` only.
- One job at a time; lock retries (5 attempts, backoff 2s, 4s, 8s, 16s, 30s); pane actions get `brew.JobEnv`.
- Key routing is by mode first: an open confirmation gets every key; in INSERT every key but `esc` goes to brew; **in COMMAND every key edits the line** (new); otherwise the global keys and actions apply.
- The emulator is never closed, and its replies are always drained (and a reply the process can't take is dropped).
- Refreshes are never dropped; the cache is saved before doctor runs.

Added by this plan:
- **Typed commands run exactly as typed.** A line at `:` runs with no confirmation and with brew's normal environment (no `JobEnv`, so brew may auto-update). A leading `brew` is optional. Arguments split like a shell would (spaces, `'…'`, `"…"`, backslash escapes) but no shell runs. An unclosed quote runs nothing and says so.
- **Brewery's own commands:** `:jobs` opens the jobs view; `:q` and `:quit` quit (asking first if a job runs).
- **Refresh after typed commands:** nothing after a read-only command (`info`, `list`, `search`, `deps`, `uses`, `desc`, `home`, `config`, `outdated`, `leaves`, `commands`, `help`, `options`, `log`, `cat`, `formulae`, `casks`, `tap-info`, `missing`, `doctor`, `shellenv`, `docs`, anything starting with `-`, and `services`/`services list`/`services info`); just services after another `services` subcommand; everything (doctor included) after anything else.
- **Takeover:** `edit`, `sh`, `irb`, `bundle edit`, and any line starting with `!` get the whole terminal through `tea.ExecProcess`. A takeover waits while a job runs (nothing would read the job's output meanwhile). It is recorded in the history, and everything refreshes afterwards.
- **History:** `$XDG_STATE_HOME/brewery/jobs/` (default `~/.local/state/brewery/jobs/`), one JSON file per job named by its start time (`20060102T150405.000000000.json`, UTC), the newest 100 kept. A record keeps the job's title, args, state (stored by name), exit code, start and end, and the last 128 KiB of output, starting at a line where it can. Typed input is never stored. A failed save shows "history not saved" in the status bar; the record is still kept for the session.
- **Jobs view:** newest first, with status (`✓ done`, `✗ exit N`, `stopped`), command, start (clock today, `Jan 2 15:04` before), and duration. `enter` replays the output in the drawer, except while a job runs; `esc` and `q` close it; tab keys close it and switch; `:`, `o`, `r` and `ctrl+c` still work.
- **COMMAND mode** (blue badge): the status bar becomes the prompt, with `tab complete · ↑↓ history · esc cancel` when there's room. `↑`/`↓` step through this session's earlier commands and back to the draft.
- **Completion popup:** up to 8 rows over the bottom of the screen, just above the prompt, scrolling to keep the selection in view. A first word completes to brewery's and brew's commands, everyday commands first (`install`, `upgrade`, `search`, `services`, …); after a command come its subcommands, its flags (once the word starts with `-`), or values from the snapshot for the argument kinds brew names (installed formulae and casks, outdated ones, taps, services). Values already on the line aren't offered again. `tab`/`shift+tab` cycle and insert; with nothing shown, `tab` lists everything. Kinds that need brew's whole catalogue (`formulae_all`, `casks_all`) complete nothing until Discover.
- **Brew 7's confirmation:** the drawer switches to INSERT when the cursor starts an empty line under a line ending in `[y/n]`. `A` (upgrade all) sets `HOMEBREW_NO_ASK=1`.

## File Structure

```
internal/jobs/history.go       Record, NewRecord, Tail, History (Save/Load), DefaultHistoryDir, Keep, MaxOutput
internal/brew/completions.go   Completions, Command, Flag, ParseFish, fishTokens, (*Client) Completions
internal/brew/testdata/brew.fish  an excerpt of the real fish script, for tests
internal/ui/cmdline.go         cmdline (line editor + past commands), COMMAND mode, execute, splitArgs, refreshAfter
internal/ui/complete.go        candidates, popup, cycle, everyday ranking, UseCompletions
internal/ui/jobsview.go        jobRows, openJobs, jobsKey, showRecord
internal/ui/takeover.go        UseTakeover, takesOver, takeover, takeoverDone
internal/ui/run.go             + record (history), env from the action
internal/ui/drawer.go          + brew 7 [y/n] detection, replay
internal/ui/home.go            + recent jobs pane, ago; doctor lines styled one by one
internal/ui/app.go             + COMMAND mode routing, jobs view, popup overlay, UseHistory
internal/ui/confirm.go         + overlayAt
cmd/brewery/main.go            wires the history, completions and takeover
```

---

### Task 1: Answer brew 7's own confirmation in the drawer

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/actions_test.go`
- Modify: `internal/ui/drawer.go`
- Modify: `internal/ui/drawer_test.go`
- Modify: `internal/ui/run.go`
- Modify: `internal/ui/run_test.go`

**Interfaces:**
- **Consumes:** Plan 2's `drawer.prompt`, `promptRe`, `action`, `launch`, `askUpgradeAll`.
- **Produces:**
  - `askRe`: matches a line ending in `[y/n]`.
  - `drawer.prompt()` also reports true when the cursor starts an empty line under such a line.
  - `action.env []string`, added to the job's environment after `brew.JobEnv`.
  - `askUpgradeAll` runs `upgrade` with `HOMEBREW_NO_ASK=1`.

Brew 7 prints its question with `ohai`, which ends the line, then reads one key with `getch` (`y` proceeds; `n`, `esc`, ctrl+c and ctrl+d abort). The look-above applies only to `[y/n]` questions, so a password prompt already answered with Enter doesn't count as still asking.

- [ ] **Step 1: Edit `internal/ui/actions_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
```

   with:

```go
import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
```

2. Replace:

```go
	if want := []string{"upgrade"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUninstallOffersOrphans(t *testing.T) {
```

   with:

```go
	if want := []string{"upgrade"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	if env := fs.envs[len(fs.envs)-1]; !slices.Contains(env, "HOMEBREW_NO_ASK=1") {
		t.Errorf("confirmed here, so brew needn't ask again: env %v", env)
	}
}

func TestUninstallOffersOrphans(t *testing.T) {
```

- [ ] **Step 2: Edit `internal/ui/drawer_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		{"Proceed (y/n)? ", true},
		{"==> Downloading https://example.com/x.dmg\r\n", false},
		{"Password:\r\nok, thanks\r\n", false}, // answered: the cursor moved on
	}
	for _, tt := range tests {
		d := newTestDrawer(t, runningJob())
```

   with:

```go
		{"Proceed (y/n)? ", true},
		{"==> Downloading https://example.com/x.dmg\r\n", false},
		{"Password:\r\nok, thanks\r\n", false}, // answered: the cursor moved on
		{"Password:\r\n", false},               // answered with enter
		// brew 7's own confirmation ends its line and waits for one key.
		{"==> Upgrading 2 outdated packages:\r\n==> Do you want to proceed with the upgrade? [y/n]\r\n", true},
		{"==> Do you want to proceed with the upgrade? [y/n]\r\ny\r\n", false},
	}
	for _, tt := range tests {
		d := newTestDrawer(t, runningJob())
```

- [ ] **Step 3: Edit `internal/ui/run_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	resume(t, a, waiting)
}

func TestInsertModeEndsWithTheJob(t *testing.T) {
	p := newFakeProc(0, "Proceed? [y/N] ")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
```

   with:

```go
	resume(t, a, waiting)
}

func TestBrewsOwnConfirmationSwitchesToInsert(t *testing.T) {
	p := newFakeProc(0, "==> Upgrading fd\r\n  fd 10.5.0 -> 10.6.0\r\n==> Do you want to proceed with the upgrade? [y/n]\r\n")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	if a.mode != modeInsert {
		t.Fatalf("brew's [y/n] question should switch to insert, mode %v", a.mode)
	}
	keys(t, a, "y")
	if p.input() != "y" {
		t.Errorf("brew got %q, want y", p.input())
	}
	p.exit()
	resume(t, a, waiting)
}

func TestInsertModeEndsWithTheJob(t *testing.T) {
	p := newFakeProc(0, "Proceed? [y/N] ")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    --- FAIL: TestUpgradeAllAsksFirst (0.01s)
    --- FAIL: TestDrawerPromptDetection (0.00s)
    --- FAIL: TestBrewsOwnConfirmationSwitchesToInsert (0.11s)
```

- [ ] **Step 5: Edit `internal/ui/actions.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	args    []string // brew's arguments
	refresh []query  // queries to re-run when it finishes
	full    bool     // re-run every query, doctor included (after brew update)
}

// kindArgs builds brew arguments for verb on packages of one kind.
```

   with:

```go
	args    []string // brew's arguments
	refresh []query  // queries to re-run when it finishes
	full    bool     // re-run every query, doctor included (after brew update)
	env     []string // added to the job's environment
}

// kindArgs builds brew arguments for verb on packages of one kind.
```

2. Replace:

```go
	if pinned > 0 {
		body = append(body, fmt.Sprintf("%d pinned will be skipped.", pinned))
	}
	all := action{title: "upgrade all", args: []string{"upgrade"}, refresh: packageQueries}
	a.confirm = &confirmBox{
		title: fmt.Sprintf("Upgrade all %d outdated?", n),
		body:  body,
```

   with:

```go
	if pinned > 0 {
		body = append(body, fmt.Sprintf("%d pinned will be skipped.", pinned))
	}
	// This dialog is the confirmation, so brew 7 needn't ask again. The
	// variable, unlike --no-ask, is ignored by older brews.
	all := action{title: "upgrade all", args: []string{"upgrade"}, env: []string{"HOMEBREW_NO_ASK=1"}, refresh: packageQueries}
	a.confirm = &confirmBox{
		title: fmt.Sprintf("Upgrade all %d outdated?", n),
		body:  body,
```

- [ ] **Step 6: Edit `internal/ui/drawer.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
// password prompt (sudo) or a yes/no question.
var promptRe = regexp.MustCompile(`(?i)(password|passphrase)[^:]*:\s*$|[\[(]y/n[\])]\??\s*$`)

// spinnerFrames animate a running job's title.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
```

   with:

```go
// password prompt (sudo) or a yes/no question.
var promptRe = regexp.MustCompile(`(?i)(password|passphrase)[^:]*:\s*$|[\[(]y/n[\])]\??\s*$`)

// askRe matches brew's own confirmation ("==> Do you want to proceed with
// the upgrade? [y/n]"), which ends its line before waiting for one key.
var askRe = regexp.MustCompile(`(?i)\[y/n\]\s*$`)

// spinnerFrames animate a running job's title.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
```

2. Replace:

```go
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
```

   with:

```go
}

// prompt reports whether the process is waiting for typed input: the text
// before the cursor looks like a password prompt or a y/n question, or the
// cursor starts an empty line under brew's own [y/n] question.
func (d *drawer) prompt() bool {
	pos := d.term.CursorPosition()
	lines := strings.Split(ansi.Strip(d.term.Render()), "\n")
	if pos.Y < 0 || pos.Y >= len(lines) {
		return false
	}
	before := ansi.Truncate(lines[pos.Y], pos.X, "")
	if strings.TrimSpace(before) == "" && pos.Y > 0 && askRe.MatchString(lines[pos.Y-1]) {
		return true
	}
	return promptRe.MatchString(before)
}

// view renders the drawer as a w×h pane. frame picks the spinner frame;
```

- [ ] **Step 7: Edit `internal/ui/run.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
import (
	"fmt"
	"io"
	"time"

	tea "charm.land/bubbletea/v2"
```

   with:

```go
import (
	"fmt"
	"io"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
```

2. Replace:

```go

// launch starts a run of j and shows it in the drawer.
func (a *App) launch(j *jobs.Job) tea.Cmd {
	p, err := a.start(j.Args, brew.JobEnv, a.drawer.cols, a.drawer.rows)
	if err != nil {
		a.queue.Finish(j, -1, false, err.Error(), a.now())
		a.flash = a.st.Err.Render(fmt.Sprintf("✗ %s: couldn't start brew: %v", j.Title, err))
```

   with:

```go

// launch starts a run of j and shows it in the drawer.
func (a *App) launch(j *jobs.Job) tea.Cmd {
	env := append(slices.Clone(brew.JobEnv), a.acts[j.ID].env...)
	p, err := a.start(j.Args, env, a.drawer.cols, a.drawer.rows)
	if err != nil {
		a.queue.Finish(j, -1, false, err.Error(), a.now())
		a.flash = a.st.Err.Render(fmt.Sprintf("✗ %s: couldn't start brew: %v", j.Title, err))
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "fix(ui): answer brew 7's own confirmation in the drawer"
```

---

### Task 2: Keep a history of finished jobs on disk

**Files:**
- Create: `internal/jobs/history.go`
- Create: `internal/jobs/history_test.go`

**Interfaces:**
- **Consumes:** `jobs.Job`, `jobs.State`.
- **Produces:** in package `jobs`:
  - `Keep = 100`, `MaxOutput = 128 << 10`.
  - `Record{Title, Args, State, ExitCode, Started, Ended, Output}` with JSON tags; `NewRecord(j *Job, output string) Record`; `(Record) Job() *Job`.
  - `Tail(s string, n int) string`: the last n bytes, starting at a line when the cut falls mid-line.
  - `State` gains `MarshalText`/`UnmarshalText`, so records store `"done"`, `"failed"`, … rather than numbers.
  - `History{Dir string}` with `Save(Record) error` (writes `<start time>.json` through a temp file, then removes the oldest beyond `Keep`) and `Load() ([]Record, error)` (newest first; a missing directory is empty; unreadable files are skipped).
  - `DefaultHistoryDir() string`.

- [ ] **Step 1: Create `internal/jobs/history_test.go`**

```go
package jobs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func record(title string, started time.Time) Record {
	return Record{Title: title, Args: strings.Fields(title), State: Done, Started: started, Ended: started.Add(time.Second)}
}

func TestHistorySavesAndLoadsNewestFirst(t *testing.T) {
	h := History{Dir: filepath.Join(t.TempDir(), "jobs")} // created on first save
	a := record("upgrade fd", t0)
	b := record("pin git", t0.Add(time.Minute))
	b.State, b.ExitCode, b.Output = Failed, 1, "Error: no such keg\n"
	for _, r := range []Record{a, b} {
		if err := h.Save(r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := h.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Title != "pin git" || got[1].Title != "upgrade fd" {
		t.Fatalf("want newest first, got %+v", got)
	}
	if !reflect.DeepEqual(got[0], b) {
		t.Errorf("round trip changed the record:\n got %+v\nwant %+v", got[0], b)
	}
}

func TestHistoryKeepsTheNewest(t *testing.T) {
	h := History{Dir: t.TempDir()}
	for i := range Keep + 3 {
		if err := h.Save(record("job", t0.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := h.Load()
	if len(got) != Keep {
		t.Fatalf("kept %d records, want %d", len(got), Keep)
	}
	if want := t0.Add((Keep + 2) * time.Second); !got[0].Started.Equal(want) {
		t.Errorf("newest record started %v, want %v", got[0].Started, want)
	}
	if want := t0.Add(3 * time.Second); !got[Keep-1].Started.Equal(want) {
		t.Errorf("oldest kept record started %v, want %v (the 3 oldest are dropped)", got[Keep-1].Started, want)
	}
}

func TestHistoryLoadSkipsBadFilesAndMissingDir(t *testing.T) {
	got, err := History{Dir: filepath.Join(t.TempDir(), "none")}.Load()
	if err != nil || len(got) != 0 {
		t.Errorf("a missing history is empty, got %v %v", got, err)
	}
	h := History{Dir: t.TempDir()}
	h.Save(record("upgrade fd", t0))
	os.WriteFile(filepath.Join(h.Dir, "20990101T000000.000000000.json"), []byte("{not json"), 0o644)
	got, err = h.Load()
	if err != nil || len(got) != 1 || got[0].Title != "upgrade fd" {
		t.Errorf("a corrupt file should be skipped: %+v %v", got, err)
	}
}

func TestStateIsStoredByName(t *testing.T) {
	b, _ := Canceled.MarshalText()
	if string(b) != "canceled" {
		t.Errorf("Canceled is stored as %q", b)
	}
	var s State
	if err := s.UnmarshalText([]byte("failed")); err != nil || s != Failed {
		t.Errorf("read %v, %v", s, err)
	}
	if err := s.UnmarshalText([]byte("exploded")); err == nil {
		t.Error("an unknown state should be an error")
	}
}

func TestNewRecordKeepsTheEndOfTheOutput(t *testing.T) {
	j := &Job{Title: "upgrade fd", Args: []string{"upgrade", "fd"}, State: Done, Started: t0, Ended: t0.Add(time.Second)}
	out := strings.Repeat("progress line\n", MaxOutput/14+10) + "==> fd 10.5.0 installed\n"
	r := NewRecord(j, out)
	if len(r.Output) > MaxOutput || !strings.HasSuffix(r.Output, "==> fd 10.5.0 installed\n") {
		t.Errorf("record output is %d bytes and ends %q", len(r.Output), r.Output[max(len(r.Output)-30, 0):])
	}
	if !strings.HasPrefix(r.Output, "progress line\n") {
		t.Errorf("the kept output should start at a line: %q", r.Output[:20])
	}
	if back := r.Job(); back.Title != j.Title || back.State != Done || !back.Ended.Equal(j.Ended) {
		t.Errorf("Job() lost fields: %+v", back)
	}
}

func TestTail(t *testing.T) {
	for _, tt := range []struct {
		s    string
		n    int
		want string
	}{
		{"short", 10, "short"},
		{"one\ntwo\nthree", 7, "three"},      // cut mid-line, then skip to the next line
		{"one\ntwo\nthree", 9, "two\nthree"}, // the cut is at a line start: keep it
		{"aaaaaaaaaaaa", 4, "aaaa"},          // no newline to start at
		{"abc\n", 2, "c\n"},                  // the only newline ends the text
	} {
		if got := Tail(tt.s, tt.n); got != tt.want {
			t.Errorf("Tail(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/jobs/history_test.go:12:46: undefined: Record
    internal/jobs/history_test.go:13:9: undefined: Record
    internal/jobs/history_test.go:17:7: undefined: History
```

- [ ] **Step 3: Create `internal/jobs/history.go`**

```go
package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Keep is how many finished jobs the history holds.
const Keep = 100

// MaxOutput is how much of a job's output its record keeps: the end, where
// brew reports what happened.
const MaxOutput = 128 << 10

// Record is a finished job as the history keeps it.
type Record struct {
	Title    string    `json:"title"`
	Args     []string  `json:"args"`
	State    State     `json:"state"`
	ExitCode int       `json:"exit_code"`
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended"`
	Output   string    `json:"output"` // the last MaxOutput bytes it printed
}

// NewRecord records a finished job and the output it printed.
func NewRecord(j *Job, output string) Record {
	return Record{
		Title: j.Title, Args: j.Args, State: j.State, ExitCode: j.ExitCode,
		Started: j.Started, Ended: j.Ended, Output: Tail(output, MaxOutput),
	}
}

// Job rebuilds the job a record describes, for showing it again.
func (r Record) Job() *Job {
	return &Job{Title: r.Title, Args: r.Args, State: r.State, ExitCode: r.ExitCode, Started: r.Started, Ended: r.Ended}
}

// Tail is the end of s, at most n bytes, starting at a line when it can.
func Tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := len(s) - n
	midLine := s[cut-1] != '\n'
	s = s[cut:]
	if i := strings.IndexByte(s, '\n'); midLine && i >= 0 && i < len(s)-1 {
		s = s[i+1:]
	}
	return s
}

// MarshalText stores a state by name, so records survive reordering.
func (s State) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// UnmarshalText reads a state stored by name.
func (s *State) UnmarshalText(b []byte) error {
	for i := Queued; i <= Canceled; i++ {
		if i.String() == string(b) {
			*s = i
			return nil
		}
	}
	return fmt.Errorf("unknown job state %q", b)
}

// History stores finished jobs as one JSON file each in Dir, named by
// start time so they sort oldest first.
type History struct{ Dir string }

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
func (h History) Save(r Record) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(h.Dir, 0o755); err != nil {
		return err
	}
	name := filepath.Join(h.Dir, r.Started.UTC().Format("20060102T150405.000000000")+".json")
	tmp := name + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, name); err != nil {
		os.Remove(tmp)
		return err
	}
	files, err := h.files()
	if err != nil {
		return err
	}
	for _, f := range files[:max(len(files)-Keep, 0)] {
		os.Remove(f)
	}
	return nil
}

// Load reads the history, newest first. A missing directory is an empty
// history, and files that can't be read are skipped.
func (h History) Load() ([]Record, error) {
	files, err := h.files()
	if err != nil {
		return nil, err
	}
	var recs []Record
	for i := len(files) - 1; i >= 0; i-- {
		b, err := os.ReadFile(files[i])
		if err != nil {
			continue
		}
		var r Record
		if json.Unmarshal(b, &r) == nil {
			recs = append(recs, r)
		}
	}
	return recs, nil
}

// files lists the record files, oldest first.
func (h History) files() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(h.Dir, "*.json"))
	sort.Strings(files)
	return files, err
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(jobs): keep a history of finished jobs on disk"
```

---

### Task 3: Record finished jobs in the history

**Files:**
- Modify: `cmd/brewery/main.go`
- Modify: `internal/ui/app.go`
- Create: `internal/ui/history_test.go`
- Modify: `internal/ui/run.go`

**Interfaces:**
- **Consumes:** Task 2's `History`, `Record`, `NewRecord`, `Keep`.
- **Produces:**
  - `(*App) UseHistory(h jobs.History, recs []jobs.Record)`: where finished jobs are saved, and the records already there (newest first).
  - `App.history []jobs.Record` (newest first, at most `Keep`), `App.hist *jobs.History` (nil: keep for the session only), `App.histErr`.
  - `(*App) record(j *jobs.Job, output string) tea.Cmd`: drops the job's action from `acts` (a Plan 2 follow-up), adds the record, and saves it in the background.
  - `historySavedMsg{err}`; the status bar shows "history not saved" after a failed save.
- **`main.go`:** loads the history from `jobs.DefaultHistoryDir()` and calls `UseHistory`.

Every finished job is recorded: done, failed, stopped, and one that couldn't start (its output is the start error).

- [ ] **Step 1: Create `internal/ui/history_test.go`**

```go
package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"brewery/internal/jobs"
)

// newHistoryApp is newJobApp with its job history kept in a temp dir.
func newHistoryApp(t *testing.T, fs *fakeStarter) (*App, jobs.History) {
	t.Helper()
	a := newJobApp(t, fs)
	h := jobs.History{Dir: t.TempDir()}
	a.UseHistory(h, nil)
	return a, h
}

func TestFinishedJobsAreRecorded(t *testing.T) {
	p := newFakeProc(1, "==> Upgrading fd\r\n", "Error: download failed\r\n")
	p.exit()
	a, h := newHistoryApp(t, &fakeStarter{procs: []*fakeProc{p}})
	settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}, refresh: packageQueries}))

	if len(a.history) != 1 {
		t.Fatalf("history has %d records, want 1", len(a.history))
	}
	r := a.history[0]
	if r.Title != "upgrade fd" || r.State != jobs.Failed || r.ExitCode != 1 || !strings.Contains(r.Output, "download failed") {
		t.Errorf("record: %+v", r)
	}
	if !r.Started.Equal(testNow) || !r.Ended.Equal(testNow) {
		t.Errorf("record times %v–%v, want %v", r.Started, r.Ended, testNow)
	}
	saved, err := h.Load()
	if err != nil || len(saved) != 1 || saved[0].Title != "upgrade fd" {
		t.Errorf("the record should be saved to disk: %+v %v", saved, err)
	}
	if len(a.acts) != 0 {
		t.Errorf("a finished job's action should be dropped: %v", a.acts)
	}
}

func TestStartFailureIsRecorded(t *testing.T) {
	a, _ := newHistoryApp(t, &fakeStarter{err: errors.New("fork/exec brew: no such file")})
	settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	if len(a.history) != 1 || a.history[0].State != jobs.Failed || !strings.Contains(a.history[0].Output, "couldn't start brew: fork/exec brew") {
		t.Errorf("a job that could not start should be recorded as failed: %+v", a.history)
	}
	if len(a.acts) != 0 {
		t.Errorf("its action should be dropped: %v", a.acts)
	}
}

func TestNewestRecordComesFirstAndHistoryIsCapped(t *testing.T) {
	a, _ := newHistoryApp(t, &fakeStarter{})
	old := make([]jobs.Record, jobs.Keep)
	for i := range old {
		old[i] = jobs.Record{Title: "old", Started: testNow.Add(-time.Duration(i+1) * time.Hour)}
	}
	a.UseHistory(jobs.History{Dir: t.TempDir()}, old)
	settle(t, a, a.run(action{title: "pin git", args: []string{"pin", "git"}}))
	if len(a.history) != jobs.Keep || a.history[0].Title != "pin git" {
		t.Errorf("want %d records with the new one first, got %d starting %q", jobs.Keep, len(a.history), a.history[0].Title)
	}
}

func TestHistorySaveFailureShowsInStatusBar(t *testing.T) {
	a, _ := newHistoryApp(t, &fakeStarter{})
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0o644)
	a.UseHistory(jobs.History{Dir: filepath.Join(file, "jobs")}, nil) // can't make a dir under a file
	settle(t, a, a.run(action{title: "pin git", args: []string{"pin", "git"}}))
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "history not saved") {
		t.Errorf("status bar: %q", bar)
	}
	if len(a.history) != 1 {
		t.Error("the record is still kept for this session")
	}
}

func TestJobsAreRecordedWithoutAHistoryDir(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	settle(t, a, a.run(action{title: "pin git", args: []string{"pin", "git"}}))
	if len(a.history) != 1 || a.history[0].State != jobs.Done {
		t.Errorf("without a history dir, jobs are still kept for the session: %+v", a.history)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/history_test.go:19:4: a.UseHistory undefined (type *App has no field or method UseHistory)
    internal/ui/history_test.go:29:11: a.history undefined (type *App has no field or method history)
    internal/ui/history_test.go:30:52: a.history undefined (type *App has no field or method history)
```

- [ ] **Step 3: Edit `cmd/brewery/main.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
	"brewery/internal/ui"
)
```

   with:

```go
	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/jobs"
	"brewery/internal/state"
	"brewery/internal/ui"
)
```

2. Replace:

```go
		return brew.StartPTY(bin, args, env, cols, rows)
	}

	if _, err := tea.NewProgram(ui.New(client, start, snap, cachePath)).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
		return 1
	}
```

   with:

```go
		return brew.StartPTY(bin, args, env, cols, rows)
	}

	app := ui.New(client, start, snap, cachePath)
	hist := jobs.History{Dir: jobs.DefaultHistoryDir()}
	recs, _ := hist.Load() // no history yet is fine
	app.UseHistory(hist, recs)

	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
		return 1
	}
```

- [ ] **Step 4: Edit `internal/ui/app.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go
	ticking   bool
	frame     int

	mode    mode
	confirm *confirmBox // open confirmation, or nil
	flash   string      // one-off status message, cleared by the next key
```

   with:

```go
	ticking   bool
	frame     int

	hist    *jobs.History // where finished jobs are saved, or nil
	history []jobs.Record // finished jobs, newest first
	histErr error         // the last save failed

	mode    mode
	confirm *confirmBox // open confirmation, or nil
	flash   string      // one-off status message, cleared by the next key
```

2. Replace:

```go

func (a *App) Init() tea.Cmd { return a.refresh() }

// refresh re-runs every query: the JSON ones in parallel, then doctor.
func (a *App) refresh() tea.Cmd {
	a.doctorAfter = true
```

   with:

```go

func (a *App) Init() tea.Cmd { return a.refresh() }

// UseHistory saves finished jobs to h, starting from the records already
// there (newest first).
func (a *App) UseHistory(h jobs.History, recs []jobs.Record) {
	a.hist, a.history = &h, recs
}

// refresh re-runs every query: the JSON ones in parallel, then doctor.
func (a *App) refresh() tea.Cmd {
	a.doctorAfter = true
```

3. Replace:

```go
		return a.handleResult(msg)
	case savedMsg:
		a.saveErr = msg.err
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	case tea.PasteMsg:
```

   with:

```go
		return a.handleResult(msg)
	case savedMsg:
		a.saveErr = msg.err
	case historySavedMsg:
		a.histErr = msg.err
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	case tea.PasteMsg:
```

4. Replace:

```go
	if a.saveErr != nil {
		right = append(right, st.Warn.Render("cache not saved"))
	}
	r := strings.Join(right, "  ")

	// Keep to one line: drop the key hints first, then shorten the left side.
```

   with:

```go
	if a.saveErr != nil {
		right = append(right, st.Warn.Render("cache not saved"))
	}
	if a.histErr != nil {
		right = append(right, st.Warn.Render("history not saved"))
	}
	r := strings.Join(right, "  ")

	// Keep to one line: drop the key hints first, then shorten the left side.
```

- [ ] **Step 5: Edit `internal/ui/run.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	if err != nil {
		a.queue.Finish(j, -1, false, err.Error(), a.now())
		a.flash = a.st.Err.Render(fmt.Sprintf("✗ %s: couldn't start brew: %v", j.Title, err))
		return a.runNext()
	}
	a.proc, a.canceled = p, false
	a.drawer.start(j, p)
```

   with:

```go
	if err != nil {
		a.queue.Finish(j, -1, false, err.Error(), a.now())
		a.flash = a.st.Err.Render(fmt.Sprintf("✗ %s: couldn't start brew: %v", j.Title, err))
		return tea.Batch(a.record(j, "couldn't start brew: "+err.Error()), a.runNext())
	}
	a.proc, a.canceled = p, false
	a.drawer.start(j, p)
```

2. Replace:

```go
		a.flash = a.st.Err.Render(msg)
	}
	act := a.acts[j.ID]
	if act.full {
		return a.refresh()
	}
	return a.request(act.refresh...)
}

// stopJob interrupts the current job, as ctrl+c in a terminal would. A job
```

   with:

```go
		a.flash = a.st.Err.Render(msg)
	}
	act := a.acts[j.ID]
	saving := a.record(j, a.drawer.output())
	if act.full {
		return tea.Batch(saving, a.refresh())
	}
	return tea.Batch(saving, a.request(act.refresh...))
}

// historySavedMsg reports whether a finished job was saved to the history.
type historySavedMsg struct{ err error }

// record adds a finished job to the history and saves it. The job's
// action is no longer needed.
func (a *App) record(j *jobs.Job, output string) tea.Cmd {
	delete(a.acts, j.ID)
	r := jobs.NewRecord(j, output)
	a.history = append([]jobs.Record{r}, a.history[:min(len(a.history), jobs.Keep-1)]...)
	if a.hist == nil {
		return nil
	}
	h := *a.hist
	return func() tea.Msg { return historySavedMsg{err: h.Save(r)} }
}

// stopJob interrupts the current job, as ctrl+c in a terminal would. A job
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(ui): record finished jobs in the history"
```

---

### Task 4: Run any brew command from the : command line

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/app.go`
- Create: `internal/ui/cmdline.go`
- Create: `internal/ui/cmdline_test.go`
- Modify: `internal/ui/helpers_test.go`
- Modify: `internal/ui/run.go`

**Interfaces:**
- **Consumes:** `App.run`, `action`, `askQuit`, `note`, `theme.Styles.Mode("COMMAND")` (already blue).
- **Produces:**
  - `modeCommand`.
  - `cmdline` with `newCmdline(st)`, `open()`, `value()`, `remember(line)`, `update(msg) tea.Cmd` (↑/↓ walk the past), `show(s)`, `view(w) string`.
  - `App.cmd cmdline`; `(*App) openCmdline()`, `cmdKey(msg)`, `execute(line)`.
  - `action.typed bool`: a typed command gets no `JobEnv`.
  - `refreshAfter(args) (refresh []query, full bool)`, `readOnly`, `splitArgs(line) ([]string, error)`, `errUnclosedQuote`.
- **Test helpers:** `typeText`; `press` learns `down`, `tab`, `shift+tab`.

The status bar becomes the prompt in COMMAND mode. A paste goes into the line. `textinput`'s cursor is set not to blink, so no timer runs while the prompt is open.

- [ ] **Step 1: Add the line editor's dependency**

```bash
go get charm.land/bubbles/v2/textinput@v2.2.1
```

- [ ] **Step 2: Create `internal/ui/cmdline_test.go`**

```go
package ui

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// typeText presses a key for each character of s.
func typeText(t *testing.T, a *App, s string) {
	t.Helper()
	for _, r := range s {
		keys(t, a, string(r))
	}
}

func TestColonRunsATypedCommandAsIs(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":")
	if a.mode != modeCommand {
		t.Fatalf("mode %v, want command", a.mode)
	}
	typeText(t, a, "info fd")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "COMMAND") || !strings.Contains(bar, ":info fd") {
		t.Errorf("status bar should be the prompt: %q", bar)
	}
	keys(t, a, "enter")
	if want := []string{"info", "fd"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	if fs.envs[0] != nil {
		t.Errorf("typed commands run unchanged, but got env %v", fs.envs[0])
	}
	if a.mode != modeNormal {
		t.Errorf("enter should leave command mode, mode %v", a.mode)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "✓ info fd") {
		t.Errorf("status bar: %q", stripANSI(a.statusBar()))
	}
}

func TestColonStripsALeadingBrewAndHonoursQuotes(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":")
	typeText(t, a, `brew search "visual studio"`)
	keys(t, a, "enter")
	if want := []string{"search", "visual studio"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestSplitArgs(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want []string
		err  bool
	}{
		{"upgrade fd git", []string{"upgrade", "fd", "git"}, false},
		{"  info   fd  ", []string{"info", "fd"}, false},
		{`search "a b" 'c d'`, []string{"search", "a b", "c d"}, false},
		{`desc "say \"hi\""`, []string{"desc", `say "hi"`}, false},
		{`info a\ b`, []string{"info", "a b"}, false},
		{`info ""`, []string{"info", ""}, false},
		{`info 'it''s'`, []string{"info", "its"}, false},
		{`search "open`, nil, true},
		{"", nil, false},
	} {
		got, err := splitArgs(tt.in)
		if (err != nil) != tt.err || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitArgs(%q) = %q, %v; want %q, err %v", tt.in, got, err, tt.want, tt.err)
		}
	}
}

func TestUnclosedQuoteRunsNothing(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":")
	typeText(t, a, `search "open`)
	keys(t, a, "enter")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "unclosed quote") {
		t.Errorf("ran %v; status bar %q", fs.calls, stripANSI(a.statusBar()))
	}
}

func TestEscCancelsTheCommandLine(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":")
	typeText(t, a, "upgrade")
	keys(t, a, "esc")
	if a.mode != modeNormal || len(fs.calls) != 0 {
		t.Errorf("esc should cancel: mode %v, ran %v", a.mode, fs.calls)
	}
	keys(t, a, ":")
	if a.cmd.value() != "" {
		t.Errorf("the prompt should open empty, has %q", a.cmd.value())
	}
}

func TestEmptyCommandDoesNothing(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":", " ", "enter")
	if len(fs.calls) != 0 || a.mode != modeNormal {
		t.Errorf("ran %v, mode %v", fs.calls, a.mode)
	}
}

func TestShortcutKeysAreTextInCommandMode(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":")
	typeText(t, a, "uninstall x") // u, x, 2, ... must not act or switch tabs
	if a.tab != tabHome || a.confirm != nil || len(fs.calls) != 0 || a.cmd.value() != "uninstall x" {
		t.Errorf("tab %d, confirm %+v, ran %v, value %q", a.tab, a.confirm, fs.calls, a.cmd.value())
	}
}

func TestColonQQuits(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, ":", "q")
	_, cmd := a.Update(press("enter"))
	if cmd == nil {
		t.Fatal(":q returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error(":q should quit")
	}
}

func TestUpAndDownRecallPastCommands(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	for _, line := range []string{"info fd", "deps git"} {
		keys(t, a, ":")
		typeText(t, a, line)
		keys(t, a, "enter")
	}
	keys(t, a, ":")
	typeText(t, a, "se")
	keys(t, a, "up")
	if v := a.cmd.value(); v != "deps git" {
		t.Errorf("up once: %q", v)
	}
	keys(t, a, "up", "up") // stops at the oldest
	if v := a.cmd.value(); v != "info fd" {
		t.Errorf("up three times: %q", v)
	}
	keys(t, a, "down", "down")
	if v := a.cmd.value(); v != "se" {
		t.Errorf("down past the newest should restore the draft, got %q", v)
	}
}

func TestPasteGoesIntoTheCommandLine(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, ":")
	a.Update(tea.PasteMsg{Content: "info fd"})
	if v := a.cmd.value(); v != "info fd" {
		t.Errorf("value %q", v)
	}
}

func TestTypedCommandsRefreshWhatTheyMayChange(t *testing.T) {
	for _, tt := range []struct {
		line string
		want []string // queries that should run once afterwards
	}{
		{"info fd", nil},
		{"--prefix", nil},
		{"services list", nil},
		{"services stop redis", []string{"services"}},
		{"upgrade fd", []string{"installed", "outdated", "services", "taps", "doctor"}},
	} {
		src := newCountingSource(t)
		a := New(src, (&fakeStarter{}).start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
		a.tickEvery = 0
		a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		keys(t, a, ":")
		typeText(t, a, tt.line)
		keys(t, a, "enter")
		var ran []string
		for _, q := range []string{"installed", "outdated", "services", "taps", "doctor"} {
			if src.calls[q] > 0 {
				ran = append(ran, q)
			}
		}
		if !reflect.DeepEqual(ran, tt.want) {
			t.Errorf(":%s refreshed %v, want %v", tt.line, ran, tt.want)
		}
	}
}

func TestHintsMentionTheCommandLine(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, ": command") {
		t.Errorf("hints should mention the command line: %q", bar)
	}
}
```

- [ ] **Step 3: Edit `internal/ui/helpers_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
```

   with:

```go
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/cmdline_test.go:24:15: undefined: modeCommand
    internal/ui/cmdline_test.go:73:15: undefined: splitArgs
    internal/ui/cmdline_test.go:101:7: a.cmd undefined (type *App has no field or method cmd)
```

- [ ] **Step 5: Edit `internal/ui/actions.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	refresh []query  // queries to re-run when it finishes
	full    bool     // re-run every query, doctor included (after brew update)
	env     []string // added to the job's environment
}

// kindArgs builds brew arguments for verb on packages of one kind.
```

   with:

```go
	refresh []query  // queries to re-run when it finishes
	full    bool     // re-run every query, doctor included (after brew update)
	env     []string // added to the job's environment
	typed   bool     // typed at ":": runs as brew would, auto-update included
}

// kindArgs builds brew arguments for verb on packages of one kind.
```

- [ ] **Step 6: Edit `internal/ui/app.go`** (7 replacements; each old block appears exactly once)

1. Replace:

```go
type mode int

const (
	modeNormal mode = iota // keys drive brewery
	modeInsert             // keys go to the running brew command
)

// App is the root Bubble Tea model.
```

   with:

```go
type mode int

const (
	modeNormal  mode = iota // keys drive brewery
	modeInsert              // keys go to the running brew command
	modeCommand             // keys edit the ":" command line
)

// App is the root Bubble Tea model.
```

2. Replace:

```go
	histErr error         // the last save failed

	mode    mode
	confirm *confirmBox // open confirmation, or nil
	flash   string      // one-off status message, cleared by the next key
```

   with:

```go
	histErr error         // the last save failed

	mode    mode
	cmd     cmdline     // the ":" prompt
	confirm *confirmBox // open confirmation, or nil
	flash   string      // one-off status message, cleared by the next key
```

3. Replace:

```go
		inflight: map[query]bool{}, again: map[query]bool{},
		acts: map[int]action{}, backoff: jobs.Backoff, tickEvery: 100 * time.Millisecond,
		drawer:    newDrawer(st),
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
```

   with:

```go
		inflight: map[query]bool{}, again: map[query]bool{},
		acts: map[int]action{}, backoff: jobs.Backoff, tickEvery: 100 * time.Millisecond,
		drawer:    newDrawer(st),
		cmd:       newCmdline(st),
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
```

4. Replace:

```go
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	case tea.PasteMsg:
		if a.mode == modeInsert && a.proc != nil {
			a.proc.Write([]byte(msg.Content)) // raw: sudo wants no bracketed-paste wrapping
		}
	case jobOutputMsg, jobEOFMsg, jobExitMsg, jobRetryMsg, tickMsg:
		return a.handleJob(msg)
```

   with:

```go
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	case tea.PasteMsg:
		switch {
		case a.mode == modeInsert && a.proc != nil:
			a.proc.Write([]byte(msg.Content)) // raw: sudo wants no bracketed-paste wrapping
		case a.mode == modeCommand:
			return a.cmd.update(msg)
		}
	case jobOutputMsg, jobEOFMsg, jobExitMsg, jobRetryMsg, tickMsg:
		return a.handleJob(msg)
```

5. Replace:

```go
		}
		return nil
	}

	switch k {
	case "q":
		return a.askQuit()
	case "ctrl+c":
		if a.queue.Current() != nil {
			return a.askStop()
```

   with:

```go
		}
		return nil
	}
	if a.mode == modeCommand {
		return a.cmdKey(msg)
	}

	switch k {
	case "q":
		return a.askQuit()
	case ":":
		return a.openCmdline()
	case "ctrl+c":
		if a.queue.Current() != nil {
			return a.askStop()
```

6. Replace:

```go

func (a *App) statusBar() string {
	st := a.st
	badge := "NORMAL"
	switch {
	case a.mode == modeInsert:
```

   with:

```go

func (a *App) statusBar() string {
	st := a.st
	if a.mode == modeCommand {
		badge := st.Mode("COMMAND").Render("COMMAND") + " "
		line := badge + a.cmd.view(a.width-lipgloss.Width(badge)-1)
		return st.StatusBar.Width(a.width).Render(ansi.Truncate(line, a.width, ""))
	}
	badge := "NORMAL"
	switch {
	case a.mode == modeInsert:
```

7. Replace:

```go

	// Keep to one line: drop the key hints first, then shorten the left side.
	room := a.width - 1 // trailing space
	hints := a.actionHints() + " · q quit"
	if a.drawer.job != nil && !a.drawer.open {
		hints = "o output · " + hints
	}
```

   with:

```go

	// Keep to one line: drop the key hints first, then shorten the left side.
	room := a.width - 1 // trailing space
	hints := a.actionHints() + " · : command · q quit"
	if a.drawer.job != nil && !a.drawer.open {
		hints = "o output · " + hints
	}
```

- [ ] **Step 7: Create `internal/ui/cmdline.go`**

```go
package ui

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/theme"
)

// cmdline is the ":" prompt: a one-line editor that remembers the
// session's earlier commands.
type cmdline struct {
	input textinput.Model
	past  []string // earlier commands, oldest first
	back  int      // how far up into past the line is; 0 is the draft
	draft string   // what was typed before browsing past commands
}

func newCmdline(st theme.Styles) cmdline {
	in := textinput.New()
	in.Prompt = ":"
	s := in.Styles()
	s.Cursor.Blink = false
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(st.P.Blue).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(st.P.Text)
	in.SetStyles(s)
	return cmdline{input: in}
}

// open starts an empty line.
func (c *cmdline) open() {
	c.input.Reset()
	c.input.Focus()
	c.back, c.draft = 0, ""
}

func (c *cmdline) value() string { return c.input.Value() }

// remember adds a run command to the past, skipping a repeat of the last.
func (c *cmdline) remember(line string) {
	if n := len(c.past); n == 0 || c.past[n-1] != line {
		c.past = append(c.past, line)
	}
}

// update edits the line. up and down step through past commands.
func (c *cmdline) update(msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "up":
			if c.back < len(c.past) {
				if c.back == 0 {
					c.draft = c.input.Value()
				}
				c.back++
				c.show(c.past[len(c.past)-c.back])
			}
			return nil
		case "down":
			if c.back > 0 {
				c.back--
				if c.back == 0 {
					c.show(c.draft)
				} else {
					c.show(c.past[len(c.past)-c.back])
				}
			}
			return nil
		}
	}
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

// show puts s on the line with the cursor at the end.
func (c *cmdline) show(s string) {
	c.input.SetValue(s)
	c.input.CursorEnd()
}

// view renders the prompt in w columns.
func (c *cmdline) view(w int) string {
	c.input.SetWidth(max(w-2, 1)) // the prompt and the cursor cell
	return c.input.View()
}

// openCmdline switches to command mode with an empty prompt.
func (a *App) openCmdline() tea.Cmd {
	a.mode = modeCommand
	a.cmd.open()
	return nil
}

// cmdKey handles a key in command mode: enter runs the line, esc cancels,
// and everything else edits it.
func (a *App) cmdKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		a.mode = modeNormal
		return nil
	case "enter":
		a.mode = modeNormal
		line := strings.TrimSpace(a.cmd.value())
		if line == "" {
			return nil
		}
		a.cmd.remember(line)
		return a.execute(line)
	}
	return a.cmd.update(msg)
}

// execute runs a line typed at the prompt. A leading "brew" is optional.
// Lines run exactly as typed: no confirmation, and brew may auto-update.
func (a *App) execute(line string) tea.Cmd {
	args, err := splitArgs(line)
	if err != nil {
		return a.note(err.Error())
	}
	if len(args) > 0 && args[0] == "brew" {
		args = args[1:]
	}
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "q", "quit":
		return a.askQuit()
	}
	refresh, full := refreshAfter(args)
	return a.run(action{title: strings.Join(args, " "), args: args, refresh: refresh, full: full, typed: true})
}

// readOnly are brew commands that change nothing brewery shows, so no
// refresh follows them. Commands starting with "-" (--prefix, --cache,
// --version, …) only print paths and versions.
var readOnly = map[string]bool{
	"info": true, "abv": true, "list": true, "ls": true, "search": true, "deps": true,
	"uses": true, "desc": true, "home": true, "homepage": true, "config": true,
	"outdated": true, "leaves": true, "commands": true, "help": true, "options": true,
	"log": true, "cat": true, "formulae": true, "casks": true, "tap-info": true,
	"missing": true, "doctor": true, "dr": true, "shellenv": true, "docs": true,
}

// refreshAfter says what to re-run after a typed command: nothing after a
// read-only one, just services after a services change, else everything.
func refreshAfter(args []string) (refresh []query, full bool) {
	cmd := args[0]
	switch {
	case readOnly[cmd] || strings.HasPrefix(cmd, "-"):
		return nil, false
	case cmd == "services":
		if len(args) == 1 || args[1] == "list" || args[1] == "info" {
			return nil, false
		}
		return []query{qServices}, false
	}
	return nil, true
}

var errUnclosedQuote = errors.New("unclosed quote")

// splitArgs splits a line into arguments the way a shell would, without
// running one: spaces separate, quotes group, and a backslash escapes the
// next character (outside single quotes).
func splitArgs(line string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inArg, quote, escaped := false, rune(0), false
	for _, r := range line {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped, inArg = true, true
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			cur.WriteRune(r)
		case r == '"' || r == '\'':
			quote, inArg = r, true
		case r == ' ' || r == '\t':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if quote != 0 || escaped {
		return nil, errUnclosedQuote
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args, nil
}
```

- [ ] **Step 8: Edit `internal/ui/run.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go

// launch starts a run of j and shows it in the drawer.
func (a *App) launch(j *jobs.Job) tea.Cmd {
	env := append(slices.Clone(brew.JobEnv), a.acts[j.ID].env...)
	p, err := a.start(j.Args, env, a.drawer.cols, a.drawer.rows)
	if err != nil {
		a.queue.Finish(j, -1, false, err.Error(), a.now())
```

   with:

```go

// launch starts a run of j and shows it in the drawer.
func (a *App) launch(j *jobs.Job) tea.Cmd {
	act := a.acts[j.ID]
	env := append(slices.Clone(brew.JobEnv), act.env...)
	if act.typed {
		env = nil
	}
	p, err := a.start(j.Args, env, a.drawer.cols, a.drawer.rows)
	if err != nil {
		a.queue.Finish(j, -1, false, err.Error(), a.now())
```

- [ ] **Step 9: Tidy the module**

```bash
go mod tidy
```

- [ ] **Step 10: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "feat(ui): run any brew command from the : command line"
```

---

### Task 5: List past jobs with :jobs and replay their output

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/cmdline.go`
- Modify: `internal/ui/drawer.go`
- Create: `internal/ui/jobsview.go`
- Create: `internal/ui/jobsview_test.go`
- Modify: `internal/ui/run.go`

**Interfaces:**
- **Consumes:** `App.history`, `listTab`, `drawer`, `execute`.
- **Produces:**
  - `jobCols`, `jobKey`, `jobRows(recs, now) []table.Row`.
  - `App.jobs listTab`, `App.jobsOpen`; `(*App) openJobs()`, `jobsKey(msg) (tea.Cmd, bool)`, `showRecord()`.
  - `(*drawer) replay(j *jobs.Job, output string)`: a clean screen with the saved output.
  - `:jobs` opens the view; the status bar names it and hints `enter show output · esc close`.
- **Test helpers:** `pastJobs()`, `newJobsViewApp`.

The list stays current: a job finishing while the view is open joins it. Times are shown in the clock's own zone (`now.Location()`), so tests don't depend on the machine's zone.

- [ ] **Step 1: Create `internal/ui/jobsview_test.go`**

```go
package ui

import (
	"strings"
	"testing"
	"time"

	"brewery/internal/jobs"
)

// pastJobs are two finished jobs: a failed upgrade an hour ago and a pin
// yesterday.
func pastJobs() []jobs.Record {
	return []jobs.Record{
		{Title: "upgrade fd", Args: []string{"upgrade", "fd"}, State: jobs.Failed, ExitCode: 1,
			Started: testNow.Add(-time.Hour), Ended: testNow.Add(-time.Hour + 75*time.Second),
			Output: "==> Upgrading fd\r\nError: download failed\r\n"},
		{Title: "pin git", Args: []string{"pin", "git"}, State: jobs.Done,
			Started: testNow.Add(-24 * time.Hour), Ended: testNow.Add(-24*time.Hour + 2*time.Second)},
	}
}

func newJobsViewApp(t *testing.T, fs *fakeStarter) *App {
	t.Helper()
	a := newJobApp(t, fs)
	a.UseHistory(jobs.History{Dir: t.TempDir()}, pastJobs())
	return a
}

func TestColonJobsListsThePastJobs(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter")
	if !a.jobsOpen {
		t.Fatal(":jobs should open the jobs view")
	}
	out := stripANSI(a.render())
	for _, want := range []string{"Jobs · 2", "✗ exit 1", "brew upgrade fd", "1:15", "✓ done", "brew pin git", "Oct 12"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "upgrade fd") > strings.Index(out, "pin git") {
		t.Error("the newest job should be listed first")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "Jobs") || !strings.Contains(bar, "enter show output") {
		t.Errorf("status bar: %q", bar)
	}
}

func TestEnterReplaysAJobsOutputInTheDrawer(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	a.openJobs()
	keys(t, a, "enter")
	out := stripANSI(a.render())
	for _, want := range []string{"Error: download failed", "✗ brew upgrade fd · failed, exit 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	keys(t, a, "j", "enter")
	if out := stripANSI(a.render()); !strings.Contains(out, "✓ brew pin git · exit 0") || strings.Contains(out, "download failed") {
		t.Errorf("the drawer should now show pin git alone:\n%s", out)
	}
}

func TestReplayWaitsWhileAJobRuns(t *testing.T) {
	p := newFakeProc(0, "==> Upgrading git\r\n")
	a := newJobsViewApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade git", args: []string{"upgrade", "git"}}))
	a.openJobs()
	keys(t, a, "enter")
	if out := stripANSI(a.render()); strings.Contains(out, "download failed") || !strings.Contains(out, "Upgrading git") {
		t.Errorf("the running job keeps the drawer:\n%s", out)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "upgrade git is running") {
		t.Errorf("status bar should say why: %q", bar)
	}
	p.exit()
	resume(t, a, waiting)
	if out := stripANSI(a.render()); !strings.Contains(out, "Jobs · 3") {
		t.Errorf("the finished job should join the open list:\n%s", out)
	}
}

func TestJobsViewKeys(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobsViewApp(t, fs)
	keys(t, a, "2")
	a.openJobs()
	keys(t, a, "u", "x", "p") // Installed actions must not reach the hidden tab
	if len(fs.calls) != 0 || a.confirm != nil {
		t.Errorf("ran %v, confirm %+v", fs.calls, a.confirm)
	}
	keys(t, a, "esc")
	if a.jobsOpen || a.tab != tabInstalled {
		t.Errorf("esc closes the view: open %v, tab %d", a.jobsOpen, a.tab)
	}
	a.openJobs()
	keys(t, a, "3")
	if a.jobsOpen || a.tab != tabOutdated {
		t.Errorf("a tab key closes the view and switches: open %v, tab %d", a.jobsOpen, a.tab)
	}
	a.openJobs()
	keys(t, a, "q")
	if a.jobsOpen {
		t.Error("q closes the view rather than quitting")
	}
}

func TestEmptyJobsView(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.openJobs()
	if out := stripANSI(a.render()); !strings.Contains(out, "no jobs yet") {
		t.Errorf("empty view:\n%s", out)
	}
	keys(t, a, "enter") // nothing to show
	if a.drawer.job != nil {
		t.Error("enter on an empty list shows nothing")
	}
}

func TestJobRows(t *testing.T) {
	rows := jobRows(append(pastJobs(), jobs.Record{Title: "upgrade", Args: []string{"upgrade"}, State: jobs.Canceled,
		Started: testNow.Add(-2 * time.Minute), Ended: testNow.Add(-time.Minute)}), testNow)
	want := [][]string{
		{"✗ exit 1", "brew upgrade fd", "08:30", "1:15"},
		{"✓ done", "brew pin git", "Oct 12 09:30", "0:02"},
		{"stopped", "brew upgrade", "09:28", "1:00"},
	}
	for i, w := range want {
		if strings.Join(rows[i], "|") != strings.Join(w, "|") {
			t.Errorf("row %d = %q, want %q", i, rows[i], w)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/jobsview_test.go:35:8: a.jobsOpen undefined (type *App has no field or method jobsOpen)
    internal/ui/jobsview_test.go:54:4: a.openJobs undefined (type *App has no field or method openJobs)
    internal/ui/jobsview_test.go:72:4: a.openJobs undefined (type *App has no field or method openJobs)
```

- [ ] **Step 3: Edit `internal/ui/actions.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go

// actionHints are the status-bar hints for the current tab's actions.
func (a *App) actionHints() string {
	switch a.tab {
	case tabInstalled:
		return "u upgrade · x uninstall · p pin · f/F view"
```

   with:

```go

// actionHints are the status-bar hints for the current tab's actions.
func (a *App) actionHints() string {
	if a.jobsOpen {
		return "enter show output · esc close"
	}
	switch a.tab {
	case tabInstalled:
		return "u upgrade · x uninstall · p pin · f/F view"
```

- [ ] **Step 4: Edit `internal/ui/app.go`** (7 replacements; each old block appears exactly once)

1. Replace:

```go
	ticking   bool
	frame     int

	hist    *jobs.History // where finished jobs are saved, or nil
	history []jobs.Record // finished jobs, newest first
	histErr error         // the last save failed

	mode    mode
	cmd     cmdline     // the ":" prompt
```

   with:

```go
	ticking   bool
	frame     int

	hist     *jobs.History // where finished jobs are saved, or nil
	history  []jobs.Record // finished jobs, newest first
	histErr  error         // the last save failed
	jobs     listTab       // the ":jobs" view
	jobsOpen bool

	mode    mode
	cmd     cmdline     // the ":" prompt
```

2. Replace:

```go
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
		taps:      newListTab(st, "Taps", tapCols, firstCell, "no taps installed"),
	}
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
		for _, q := range []query{qInstalled, qOutdated, qServices, qTaps} {
```

   with:

```go
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
		taps:      newListTab(st, "Taps", tapCols, firstCell, "no taps installed"),
		jobs:      newListTab(st, "Jobs", jobCols, jobKey, "no jobs yet"),
	}
	// The history is read before the app starts, so the jobs view is never
	// waiting for it.
	a.jobs.loaded = true
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
		for _, q := range []query{qInstalled, qOutdated, qServices, qTaps} {
```

3. Replace:

```go
// there (newest first).
func (a *App) UseHistory(h jobs.History, recs []jobs.Record) {
	a.hist, a.history = &h, recs
}

// refresh re-runs every query: the JSON ones in parallel, then doctor.
```

   with:

```go
// there (newest first).
func (a *App) UseHistory(h jobs.History, recs []jobs.Record) {
	a.hist, a.history = &h, recs
	a.jobs.setRows(jobRows(recs, a.now()))
}

// refresh re-runs every query: the JSON ones in parallel, then doctor.
```

4. Replace:

```go
		a.tabH -= dh
	}
	a.installed.resize(a.width, a.tabH)
	for _, t := range []*listTab{&a.outdated, &a.services, &a.taps} {
		t.resize(a.width, a.tabH)
	}
}
```

   with:

```go
		a.tabH -= dh
	}
	a.installed.resize(a.width, a.tabH)
	for _, t := range []*listTab{&a.outdated, &a.services, &a.taps, &a.jobs} {
		t.resize(a.width, a.tabH)
	}
}
```

5. Replace:

```go
	if a.mode == modeCommand {
		return a.cmdKey(msg)
	}

	switch k {
	case "q":
```

   with:

```go
	if a.mode == modeCommand {
		return a.cmdKey(msg)
	}
	if a.jobsOpen {
		if cmd, ok := a.jobsKey(msg); ok {
			return cmd
		}
	}

	switch k {
	case "q":
```

6. Replace:

```go
	case tabTaps:
		body = a.taps.view(a.errs[qTaps])
	}
	if a.confirm != nil {
		body = overlay(body, a.confirm.view(a.st, a.width), a.width, bodyH)
	}
```

   with:

```go
	case tabTaps:
		body = a.taps.view(a.errs[qTaps])
	}
	if a.jobsOpen {
		body = a.jobs.view(nil)
	}
	if a.confirm != nil {
		body = overlay(body, a.confirm.view(a.st, a.width), a.width, bodyH)
	}
```

7. Replace:

```go
		badge = "RUNNING"
	}
	left := st.Mode(badge).Render(badge) + " " + tabNames[a.tab]
	if a.tab == tabInstalled {
		if p, ok := a.installed.selected(); ok {
			left += " › " + p.Name
		}
```

   with:

```go
		badge = "RUNNING"
	}
	left := st.Mode(badge).Render(badge) + " " + tabNames[a.tab]
	switch {
	case a.jobsOpen:
		left = st.Mode(badge).Render(badge) + " Jobs"
	case a.tab == tabInstalled:
		if p, ok := a.installed.selected(); ok {
			left += " › " + p.Name
		}
```

- [ ] **Step 5: Edit `internal/ui/cmdline.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	switch args[0] {
	case "q", "quit":
		return a.askQuit()
	}
	refresh, full := refreshAfter(args)
	return a.run(action{title: strings.Join(args, " "), args: args, refresh: refresh, full: full, typed: true})
```

   with:

```go
	switch args[0] {
	case "q", "quit":
		return a.askQuit()
	case "jobs":
		return a.openJobs()
	}
	refresh, full := refreshAfter(args)
	return a.run(action{title: strings.Join(args, " "), args: args, refresh: refresh, full: full, typed: true})
```

- [ ] **Step 6: Edit `internal/ui/drawer.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	d.reply.set(reply)
}

// stop detaches the finished process: later replies are dropped.
func (d *drawer) stop() { d.reply.set(nil) }
```

   with:

```go
	d.reply.set(reply)
}

// replay shows a finished job again from the output it printed.
func (d *drawer) replay(j *jobs.Job, output string) {
	d.start(j, nil)
	d.write([]byte(output))
}

// stop detaches the finished process: later replies are dropped.
func (d *drawer) stop() { d.reply.set(nil) }
```

- [ ] **Step 7: Create `internal/ui/jobsview.go`**

```go
package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/jobs"
)

var jobCols = []col{{"STATUS", 9}, {"COMMAND", 0}, {"STARTED", 12}, {"TOOK", 6}}

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
		if y, m, d := now.Date(); started.Year() != y || started.Month() != m || started.Day() != d {
			when = started.Format("Jan 2 15:04")
		}
		rows[i] = table.Row{status, "brew " + strings.Join(r.Args, " "), when, elapsed(r.Ended.Sub(r.Started))}
	}
	return rows
}

// openJobs shows the jobs view over the current tab.
func (a *App) openJobs() tea.Cmd {
	a.jobsOpen = true
	a.jobs.setRows(jobRows(a.history, a.now()))
	return nil
}

// jobsKey handles a key while the jobs view is open. ok is false for keys
// that still do what they do everywhere (tabs, :, o, r, ctrl+c); tab keys
// also close the view.
func (a *App) jobsKey(msg tea.KeyPressMsg) (cmd tea.Cmd, ok bool) {
	switch msg.String() {
	case "esc", "q":
		a.jobsOpen = false
		return nil, true
	case "enter":
		return a.showRecord(), true
	case "1", "2", "3", "4", "5", "[", "]":
		a.jobsOpen = false
		return nil, false
	case ":", "o", "r", "ctrl+c":
		return nil, false
	}
	return a.jobs.update(msg), true
}

// showRecord replays the selected job's output in the drawer. A running
// job keeps the drawer until it finishes.
func (a *App) showRecord() tea.Cmd {
	i := a.jobs.cursor()
	if i < 0 {
		return nil
	}
	if cur := a.queue.Current(); cur != nil {
		return a.note(cur.Title + " is running · its output stays in the drawer until it finishes")
	}
	r := a.history[i]
	a.drawer.replay(r.Job(), r.Output)
	return nil
}
```

- [ ] **Step 8: Edit `internal/ui/run.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	delete(a.acts, j.ID)
	r := jobs.NewRecord(j, output)
	a.history = append([]jobs.Record{r}, a.history[:min(len(a.history), jobs.Keep-1)]...)
	if a.hist == nil {
		return nil
	}
```

   with:

```go
	delete(a.acts, j.ID)
	r := jobs.NewRecord(j, output)
	a.history = append([]jobs.Record{r}, a.history[:min(len(a.history), jobs.Keep-1)]...)
	a.jobs.setRows(jobRows(a.history, a.now()))
	if a.hist == nil {
		return nil
	}
```

- [ ] **Step 9: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat(ui): list past jobs with :jobs and replay their output"
```

---

### Task 6: Show recent jobs on Home

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/home.go`
- Modify: `internal/ui/home_test.go`

**Interfaces:**
- **Consumes:** `App.history`.
- **Produces:**
  - `homeTab.view(snap, recent []jobs.Record, now time.Time, w, height int)` (gains `recent` and `now`).
  - `(*homeTab) recentJobs(recent, now, w, h) string`, `ago(now, t) string` ("just now", "5m ago", "3h ago", "yesterday", "3d ago").

Below the tiles, doctor takes three fifths of the width and "Recent jobs" the rest, ending with a `:jobs shows them all` hint. Doctor's warning bodies are now styled line by line: styled as one block, every line was padded to the longest, and the narrower pane cut the padding off with "…" even on short lines.

- [ ] **Step 1: Edit `internal/ui/home_test.go`** (6 replacements; each old block appears exactly once)

1. Replace:

```go
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
```

   with:

```go
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
```

2. Replace:

```go

func TestHomeView(t *testing.T) {
	h := loadedHome()
	view := h.view(fixtureSnapshot(t), 140, 30)
	out := stripANSI(view)
	for _, want := range []string{"Installed", "5 formulae · 2 casks", "lua-language-server, docker-", "1 error",
		"⚠ The following taps are not trusted:", "brew untap oven-sh/bun", "⚠ Some installed casks are deprecated"} {
```

   with:

```go

func TestHomeView(t *testing.T) {
	h := loadedHome()
	view := h.view(fixtureSnapshot(t), nil, testNow, 140, 30)
	out := stripANSI(view)
	for _, want := range []string{"Installed", "5 formulae · 2 casks", "lua-language-server, docker-", "1 error",
		"⚠ The following taps are not trusted:", "brew untap oven-sh/bun", "⚠ Some installed casks are deprecated"} {
```

3. Replace:

```go
	if lipgloss.Height(view) != 30 {
		t.Errorf("home view height = %d, want 30", lipgloss.Height(view))
	}
	clean := stripANSI(h.view(state.Snapshot{DoctorRan: true}, 140, 30))
	if !strings.Contains(clean, "Your system is ready to brew.") {
		t.Error("clean doctor message missing")
	}
```

   with:

```go
	if lipgloss.Height(view) != 30 {
		t.Errorf("home view height = %d, want 30", lipgloss.Height(view))
	}
	clean := stripANSI(h.view(state.Snapshot{DoctorRan: true}, nil, testNow, 140, 30))
	if !strings.Contains(clean, "Your system is ready to brew.") {
		t.Error("clean doctor message missing")
	}
```

4. Replace:

```go
	if got := h.tiles(state.Snapshot{})[3]; got.value != "✗ failed" || got.detail != "see below" || got.tab != -1 {
		t.Errorf("doctor tile = %+v", got)
	}
	out := stripANSI(h.view(state.Snapshot{}, 140, 30))
	for _, want := range []string{"brew doctor failed", "timed out after 30s", "press r to retry"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
```

   with:

```go
	if got := h.tiles(state.Snapshot{})[3]; got.value != "✗ failed" || got.detail != "see below" || got.tab != -1 {
		t.Errorf("doctor tile = %+v", got)
	}
	out := stripANSI(h.view(state.Snapshot{}, nil, testNow, 140, 30))
	for _, want := range []string{"brew doctor failed", "timed out after 30s", "press r to retry"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
```

5. Replace:

```go
		t.Error("failed doctor should not claim to be running")
	}

	out = stripANSI(h.view(fixtureSnapshot(t), 140, 30))
	for _, want := range []string{"Warnings from the last successful check:", "The following taps are not trusted"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q with earlier warnings", want)
```

   with:

```go
		t.Error("failed doctor should not claim to be running")
	}

	out = stripANSI(h.view(fixtureSnapshot(t), nil, testNow, 140, 30))
	for _, want := range []string{"Warnings from the last successful check:", "The following taps are not trusted"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q with earlier warnings", want)
```

6. Replace:

```go
	}
}

func TestPlural(t *testing.T) {
	if plural(1, "warning") != "1 warning" || plural(3, "error") != "3 errors" {
		t.Error("plural wrong")
```

   with:

```go
	}
}

func TestHomeShowsRecentJobs(t *testing.T) {
	h := loadedHome()
	for _, size := range [][2]int{{140, 30}, {80, 24}} {
		view := h.view(fixtureSnapshot(t), pastJobs(), testNow, size[0], size[1])
		if lipgloss.Width(view) != size[0] || lipgloss.Height(view) != size[1] {
			t.Errorf("at %v the view is %dx%d", size, lipgloss.Width(view), lipgloss.Height(view))
		}
		out := stripANSI(view)
		for _, want := range []string{"Recent jobs", "✗ upgrade fd", "1h ago", "✓ pin git", "yesterday", ":jobs"} {
			if !strings.Contains(out, want) {
				t.Errorf("at %v missing %q:\n%s", size, want, out)
			}
		}
	}
	out := stripANSI(h.view(fixtureSnapshot(t), nil, testNow, 140, 30))
	if !strings.Contains(out, "no jobs yet") {
		t.Errorf("empty recent jobs:\n%s", out)
	}
}

func TestShortDoctorLinesAreNotCutOff(t *testing.T) {
	h := loadedHome()
	snap := state.Snapshot{DoctorRan: true, Doctor: []brew.DoctorWarning{{
		Title: "The following taps are not trusted:",
		Body:  "oven-sh/bun\nHomebrew is currently ignoring formulae, casks and commands from these taps because tap trust is required.",
	}}}
	out := stripANSI(h.view(snap, nil, testNow, 100, 30))
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "oven-sh/bun") && strings.Contains(line, "…") {
			t.Errorf("a short line is padded to the long one and cut off: %q", line)
		}
	}
}

func TestAgo(t *testing.T) {
	for _, tt := range []struct {
		d    time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{30 * time.Hour, "yesterday"},
		{72 * time.Hour, "3d ago"},
	} {
		if got := ago(testNow, testNow.Add(-tt.d)); got != tt.want {
			t.Errorf("ago(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestPlural(t *testing.T) {
	if plural(1, "warning") != "1 warning" || plural(3, "error") != "3 errors" {
		t.Error("plural wrong")
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/home_test.go:106:51: too many arguments in call to h.view
    internal/ui/home_test.go:117:75: too many arguments in call to h.view
    internal/ui/home_test.go:128:58: too many arguments in call to h.view
```

- [ ] **Step 3: Edit `internal/ui/app.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	var body string
	switch a.tab {
	case tabHome:
		body = a.home.view(a.snap, a.width, bodyH)
	case tabInstalled:
		body = a.installed.view(a.errs[qInstalled])
	case tabOutdated:
```

   with:

```go
	var body string
	switch a.tab {
	case tabHome:
		body = a.home.view(a.snap, a.history, a.now(), a.width, bodyH)
	case tabInstalled:
		body = a.installed.view(a.errs[qInstalled])
	case tabOutdated:
```

- [ ] **Step 4: Edit `internal/ui/home.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/state"
	"brewery/internal/theme"
)
```

   with:

```go
import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/jobs"
	"brewery/internal/state"
	"brewery/internal/theme"
)
```

2. Replace:

```go
	return -1
}

func (h *homeTab) view(snap state.Snapshot, w, height int) string {
	st := h.st
	tiles := h.tiles(snap)
	tileH := 5
```

   with:

```go
	return -1
}

// view renders the tiles over doctor's findings and the recent jobs
// (newest first).
func (h *homeTab) view(snap state.Snapshot, recent []jobs.Record, now time.Time, w, height int) string {
	st := h.st
	tiles := h.tiles(snap)
	tileH := 5
```

3. Replace:

```go
	default:
		h.writeWarnings(&b, snap.Doctor)
	}
	doctor := pane(st, "Doctor", b.String(), w, max(height-tileH, 3), false)
	return lipgloss.JoinVertical(lipgloss.Left, row, doctor)
}

// writeWarnings renders each doctor warning as a title and indented body.
```

   with:

```go
	default:
		h.writeWarnings(&b, snap.Doctor)
	}
	lowH := max(height-tileH, 3)
	doctorW := w * 3 / 5
	doctor := pane(st, "Doctor", b.String(), doctorW, lowH, false)
	jobsPane := pane(st, "Recent jobs", h.recentJobs(recent, now, w-doctorW, lowH), w-doctorW, lowH, false)
	return lipgloss.JoinVertical(lipgloss.Left, row, lipgloss.JoinHorizontal(lipgloss.Top, doctor, jobsPane))
}

// recentJobs lists as many recent jobs as fit a w×h pane, each with how
// long ago it started, above a hint for the full list.
func (h *homeTab) recentJobs(recent []jobs.Record, now time.Time, w, height int) string {
	st := h.st
	iw, ih := paneInner(w, height)
	hint := st.Muted.Render(":jobs shows them all")
	if len(recent) == 0 {
		return st.Muted.Render("no jobs yet") + "\n" + hint
	}
	var lines []string
	for _, r := range recent[:min(len(recent), max(ih-1, 0))] {
		icon := st.OK.Render("✓")
		switch r.State {
		case jobs.Failed:
			icon = st.Err.Render("✗")
		case jobs.Canceled:
			icon = st.Warn.Render("⊘")
		}
		when := st.Muted.Render(ago(now, r.Started))
		title := ansi.Truncate(r.Title, max(iw-4-lipgloss.Width(when), 1), "…")
		gap := max(iw-2-lipgloss.Width(title)-lipgloss.Width(when), 1)
		lines = append(lines, icon+" "+title+strings.Repeat(" ", gap)+when)
	}
	return strings.Join(append(lines, hint), "\n")
}

// ago says roughly how long before now t was.
func ago(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// writeWarnings renders each doctor warning as a title and indented body.
```

4. Replace:

```go
	for _, d := range warnings {
		b.WriteString(st.Warn.Render("⚠ "+d.Title) + "\n")
		if d.Body != "" {
			b.WriteString(st.Muted.Render(indent(d.Body, "  ")) + "\n")
		}
		b.WriteString("\n")
	}
```

   with:

```go
	for _, d := range warnings {
		b.WriteString(st.Warn.Render("⚠ "+d.Title) + "\n")
		if d.Body != "" {
			// Style each line alone: as one block, every line would be
			// padded to the longest, and a narrow pane cuts the padding off.
			for _, line := range strings.Split(d.Body, "\n") {
				b.WriteString(st.Muted.Render("  "+line) + "\n")
			}
		}
		b.WriteString("\n")
	}
```

5. Replace:

```go
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func indent(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}
```

   with:

```go
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(ui): show recent jobs on Home"
```

---

### Task 7: Read command completions from brew's fish script

**Files:**
- Create: `internal/brew/completions.go`
- Create: `internal/brew/completions_test.go`
- Modify: `internal/brew/live_test.go`
- Create: `internal/brew/testdata/brew.fish`

**Interfaces:**
- **Produces:** in package `brew`:
  - `Completions{Commands []Command}` (sorted) with `Command(name) (Command, bool)`.
  - `Command{Name, Desc, Flags []Flag, Args []string, Subs []Command}` with `Sub(name)`.
  - `Flag{Name, Desc}` (the name keeps its `--`).
  - `ParseFish(r io.Reader) (Completions, error)`, `fishTokens(line) []string`.
  - `(*Client) Completions(ctx) (Completions, error)`: runs `brew --repository` and reads `completions/fish/brew.fish` there.

The script's lines are `__fish_brew_complete_cmd 'name' 'desc'`, `__fish_brew_complete_arg 'name[; condition]' -l flag -d 'desc'` or `-a '(__fish_brew_suggest_<kind>)'`, and `__fish_brew_complete_sub_cmd`/`_sub_arg` for subcommands (`services start`, …). One sub_arg line can list aliases (`'start launch load s l'`); only subcommands with their own sub_cmd line are kept. When a command is described twice (a tap appends its own block), the first description wins. The fixture is an excerpt of the script Homebrew 7.0.9 ships; the live test reads the real one.

- [ ] **Step 1: Create `internal/brew/completions_test.go`**

```go
package brew

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func parseFixtureCompletions(t *testing.T) Completions {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "brew.fish"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c, err := ParseFish(f)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestParseFishCommands(t *testing.T) {
	c := parseFixtureCompletions(t)
	var names []string
	for _, cmd := range c.Commands {
		names = append(names, cmd.Name)
	}
	want := []string{"--cache", "edit", "info", "pin", "services", "uninstall", "untap", "upgrade"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("commands %v, want %v", names, want)
	}
	cache, _ := c.Command("--cache")
	if cache.Desc != "Display Homebrew's download cache" {
		t.Errorf("escaped quote not undone: %q", cache.Desc)
	}
	services, _ := c.Command("services")
	if !strings.HasPrefix(services.Desc, "Manage background services") {
		t.Errorf("the first description should win: %q", services.Desc)
	}
	if _, ok := c.Command("bundle"); ok {
		t.Error("bundle isn't in the fixture")
	}
}

func TestParseFishFlagsAndArgs(t *testing.T) {
	c := parseFixtureCompletions(t)
	up, _ := c.Command("upgrade")
	flags := map[string]string{}
	for _, f := range up.Flags {
		if _, dup := flags[f.Name]; dup {
			t.Errorf("flag %s listed twice", f.Name)
		}
		flags[f.Name] = f.Desc
	}
	if flags["--dry-run"] != "Show what would be upgraded, but do not actually upgrade anything" {
		t.Errorf("--dry-run: %q", flags["--dry-run"])
	}
	for _, name := range []string{"--cask", "--formula", "--greedy", "--verbose"} {
		if _, ok := flags[name]; !ok {
			t.Errorf("upgrade is missing %s", name)
		}
	}
	for _, tt := range []struct {
		cmd  string
		want []string
	}{
		{"upgrade", []string{"formulae_installed", "casks_installed"}},
		{"uninstall", []string{"formulae_installed", "casks_installed"}},
		{"untap", []string{"taps_installed"}},
		{"info", []string{"formulae_all", "casks_all"}},
	} {
		cmd, _ := c.Command(tt.cmd)
		if !reflect.DeepEqual(cmd.Args, tt.want) {
			t.Errorf("%s args %v, want %v", tt.cmd, cmd.Args, tt.want)
		}
	}
}

func TestParseFishSubcommands(t *testing.T) {
	c := parseFixtureCompletions(t)
	services, _ := c.Command("services")
	var names []string
	for _, s := range services.Subs {
		names = append(names, s.Name)
	}
	if want := []string{"list", "run", "start", "stop"}; !reflect.DeepEqual(names, want) {
		t.Errorf("subcommands %v, want %v", names, want)
	}
	start, ok := services.Sub("start")
	if !ok || start.Desc != "Start the service formula immediately and register it to launch at login (or boot)" {
		t.Errorf("start: %+v", start)
	}
	if !reflect.DeepEqual(start.Args, []string{"services"}) {
		t.Errorf("start args %v", start.Args)
	}
	var all bool
	for _, f := range start.Flags {
		all = all || f.Name == "--all"
	}
	if !all {
		t.Errorf("start should have --all: %+v", start.Flags)
	}
	if _, ok := services.Sub("launch"); ok {
		t.Error("an alias without its own sub_cmd line should not become a subcommand")
	}
}

func TestFishTokens(t *testing.T) {
	for _, tt := range []struct {
		line string
		want []string
	}{
		{`__fish_brew_complete_cmd '--cache' 'Display Homebrew\'s download cache'`,
			[]string{"__fish_brew_complete_cmd", "--cache", "Display Homebrew's download cache"}},
		{`__fish_brew_complete_cmd 'services' "Integrates macOS's \"launchctl\""`,
			[]string{"__fish_brew_complete_cmd", "services", `Integrates macOS's "launchctl"`}},
		{`__fish_brew_complete_arg 'pin' -l debug -d 'x\\y'`,
			[]string{"__fish_brew_complete_arg", "pin", "-l", "debug", "-d", `x\y`}},
	} {
		if got := fishTokens(tt.line); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("fishTokens(%s) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestClientCompletionsReadsBrewsFishFile(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "completions", "fish")
	os.MkdirAll(dir, 0o755)
	b, _ := os.ReadFile(filepath.Join("testdata", "brew.fish"))
	os.WriteFile(filepath.Join(dir, "brew.fish"), b, 0o644)

	c := NewClient(fakeRunner{out: map[string][]byte{"--repository": []byte(repo + "\n")}})
	got, err := c.Completions(context.Background())
	if err != nil || len(got.Commands) != 8 {
		t.Fatalf("got %d commands, err %v", len(got.Commands), err)
	}

	empty := NewClient(fakeRunner{out: map[string][]byte{"--repository": []byte(t.TempDir())}})
	if _, err := empty.Completions(context.Background()); err == nil {
		t.Error("a missing completion file should be an error")
	}
}
```

- [ ] **Step 2: Edit `internal/brew/live_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Errorf("brew --version via PTY: code %d, err %v, output %q", code, err, out)
	}
}
```

   with:

```go
		t.Errorf("brew --version via PTY: code %d, err %v, output %q", code, err, out)
	}
}

// TestLiveCompletions reads the fish completions the installed brew ships.
func TestLiveCompletions(t *testing.T) {
	bin, err := Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	c, err := NewClient(ExecRunner{Bin: bin, Env: QueryEnv}).Completions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) < 50 {
		t.Errorf("only %d commands parsed", len(c.Commands))
	}
	up, ok := c.Command("upgrade")
	if !ok || up.Desc == "" || len(up.Flags) < 10 || len(up.Args) == 0 {
		t.Errorf("upgrade parsed as %+v", up)
	}
	services, _ := c.Command("services")
	if _, ok := services.Sub("start"); !ok {
		t.Error("services start is missing")
	}
}
```

- [ ] **Step 3: Create `internal/brew/testdata/brew.fish`**

```fish
# Excerpt of the fish completions Homebrew 7.0.9 ships (completions/fish/brew.fish).

function __fish_brew_complete_cmd -a cmd -d "A shortcut for defining brew commands completions"
    set -e argv[1]
    complete -f -c brew -n 'not __fish_brew_command' -a $cmd -d $argv
end

__fish_brew_complete_cmd '--cache' 'Display Homebrew\'s download cache'
__fish_brew_complete_arg '--cache' -l HEAD -d 'Show the cache file used when building from HEAD'
__fish_brew_complete_arg '--cache' -l arch -d 'Show cache file for the given CPU architecture. (Pass `all` to show cache files for all architectures.)'
__fish_brew_complete_arg '--cache' -l bottle-tag -d 'Show the cache file used when pouring a bottle for the given tag'
__fish_brew_complete_arg '--cache' -l cask -d 'Only show cache files for casks'
__fish_brew_complete_arg '--cache' -l debug -d 'Display any debugging information'
__fish_brew_complete_arg '--cache' -l formula -d 'Only show cache files for formulae'
__fish_brew_complete_arg '--cache' -l help -d 'Show this message'
__fish_brew_complete_arg '--cache' -l os -d 'Show cache file for the given operating system. (Pass `all` to show cache files for all operating systems.)'
__fish_brew_complete_arg '--cache' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_arg '--cache' -l verbose -d 'Make some output more verbose'
__fish_brew_complete_arg '--cache; and not __fish_seen_argument -l cask -l casks' -a '(__fish_brew_suggest_formulae_all)'
__fish_brew_complete_arg '--cache; and not __fish_seen_argument -l formula -l formulae' -a '(__fish_brew_suggest_casks_all)'
__fish_brew_complete_cmd 'edit' 'Open a formula, cask or tap in the editor set by `$EDITOR` or `$HOMEBREW_EDITOR`, or open the Homebrew repository for editing if no argument is provided'
__fish_brew_complete_arg 'edit' -l cask -d 'Treat all named arguments as casks'
__fish_brew_complete_arg 'edit' -l debug -d 'Display any debugging information'
__fish_brew_complete_arg 'edit' -l formula -d 'Treat all named arguments as formulae'
__fish_brew_complete_arg 'edit' -l help -d 'Show this message'
__fish_brew_complete_arg 'edit' -l print-path -d 'Print the file path to be edited, without opening an editor'
__fish_brew_complete_arg 'edit' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_arg 'edit' -l verbose -d 'Make some output more verbose'
__fish_brew_complete_arg 'edit; and not __fish_seen_argument -l cask -l casks' -a '(__fish_brew_suggest_formulae_all)'
__fish_brew_complete_arg 'edit; and not __fish_seen_argument -l formula -l formulae' -a '(__fish_brew_suggest_casks_all)'
__fish_brew_complete_arg 'edit' -a '(__fish_brew_suggest_taps_installed)'
__fish_brew_complete_cmd 'info' 'Display brief statistics for your Homebrew installation'
__fish_brew_complete_arg 'info' -l cask -d 'Treat all named arguments as casks'
__fish_brew_complete_arg 'info' -l debug -d 'Display any debugging information'
__fish_brew_complete_arg 'info' -l formula -d 'Treat all named arguments as formulae'
__fish_brew_complete_arg 'info' -l help -d 'Show this message'
__fish_brew_complete_arg 'info' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_arg 'info' -l verbose -d 'Show more verbose data for formula, or full information with `--installed`'
__fish_brew_complete_arg 'info; and not __fish_seen_argument -l cask -l casks' -a '(__fish_brew_suggest_formulae_all)'
__fish_brew_complete_arg 'info; and not __fish_seen_argument -l formula -l formulae' -a '(__fish_brew_suggest_casks_all)'
__fish_brew_complete_cmd 'pin' 'Pin the specified package, preventing it from being upgraded when issuing the `brew upgrade` formula or cask command'
__fish_brew_complete_arg 'pin' -l cask -d 'Treat all named arguments as casks'
__fish_brew_complete_arg 'pin' -l debug -d 'Display any debugging information'
__fish_brew_complete_arg 'pin' -l formula -d 'Treat all named arguments as formulae'
__fish_brew_complete_arg 'pin' -l help -d 'Show this message'
__fish_brew_complete_arg 'pin' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_arg 'pin' -l verbose -d 'Make some output more verbose'
__fish_brew_complete_arg 'pin; and not __fish_seen_argument -l cask -l casks' -a '(__fish_brew_suggest_formulae_installed)'
__fish_brew_complete_arg 'pin; and not __fish_seen_argument -l formula -l formulae' -a '(__fish_brew_suggest_casks_installed)'
__fish_brew_complete_cmd 'services' 'Manage background services with macOS\' `launchctl`(1) daemon manager or Linux\'s `systemctl`(1) service manager'
__fish_brew_complete_sub_cmd 'services' 'stop' 'Stop the service formula immediately and unregister it from launching at login (or boot),'
__fish_brew_complete_sub_cmd 'services' 'start' 'Start the service formula immediately and register it to launch at login (or boot)'
__fish_brew_complete_sub_cmd 'services' 'run' 'Run the service formula without registering to launch at login (or boot)'
__fish_brew_complete_sub_cmd 'services' 'list' 'List information about all managed services for the current user (or root)'
__fish_brew_complete_arg 'services; and [ (count (__fish_brew_args)) = 1 ]' -l debug -d 'Display any debugging information'
__fish_brew_complete_arg 'services; and [ (count (__fish_brew_args)) = 1 ]' -l help -d 'Show this message'
__fish_brew_complete_arg 'services; and [ (count (__fish_brew_args)) = 1 ]' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_arg 'services; and [ (count (__fish_brew_args)) = 1 ]' -l sudo-service-user -d 'When run as root on macOS, run the service(s) as this user'
__fish_brew_complete_arg 'services; and [ (count (__fish_brew_args)) = 1 ]' -l verbose -d 'Make some output more verbose'
__fish_brew_complete_sub_arg 'services' 'stop unload terminate term t u' -l all -d 'Stop all services and unregister them from launching at login (or boot), unless `--keep` is specified'
__fish_brew_complete_sub_arg 'services' 'stop unload terminate term t u' -l debug -d 'Display any debugging information'
__fish_brew_complete_sub_arg 'services' 'stop unload terminate term t u' -l help -d 'Show this message'
__fish_brew_complete_sub_arg 'services' 'stop unload terminate term t u' -l keep -d 'When stopped, don\'t unregister the service from launching at login (or boot)'
__fish_brew_complete_sub_arg 'services' 'stop unload terminate term t u' -l max-wait -d 'Wait at most this many seconds for `stop` to finish stopping a service. Defaults to 60. Set this to zero (0) seconds to wait indefinitely'
__fish_brew_complete_sub_arg 'services' 'stop unload terminate term t u' -l no-wait -d 'Don\'t wait for `stop` to finish stopping the service'
__fish_brew_complete_sub_arg 'services' 'stop unload terminate term t u' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_sub_arg 'services' 'stop unload terminate term t u' -l sudo-service-user -d 'When run as root on macOS, run the service(s) as this user'
__fish_brew_complete_sub_arg 'services' 'stop unload terminate term t u' -l verbose -d 'Make some output more verbose'
__fish_brew_complete_sub_arg 'services' 'stop unload terminate term t u' -a '(__fish_brew_suggest_services)'
__fish_brew_complete_sub_arg 'services' 'start launch load s l' -l all -d 'Start all services and register them to launch at login (or boot)'
__fish_brew_complete_sub_arg 'services' 'start launch load s l' -l debug -d 'Display any debugging information'
__fish_brew_complete_sub_arg 'services' 'start launch load s l' -l file -d 'Use the service file from this location to `start` the service'
__fish_brew_complete_sub_arg 'services' 'start launch load s l' -l help -d 'Show this message'
__fish_brew_complete_sub_arg 'services' 'start launch load s l' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_sub_arg 'services' 'start launch load s l' -l sudo-service-user -d 'When run as root on macOS, run the service(s) as this user'
__fish_brew_complete_sub_arg 'services' 'start launch load s l' -l verbose -d 'Make some output more verbose'
__fish_brew_complete_sub_arg 'services' 'start launch load s l' -a '(__fish_brew_suggest_services)'
__fish_brew_complete_sub_arg 'services' 'run' -l all -d 'Run all services without registering them to launch at login (or boot)'
__fish_brew_complete_sub_arg 'services' 'run' -l debug -d 'Display any debugging information'
__fish_brew_complete_sub_arg 'services' 'run' -l file -d 'Use the service file from this location to `run` the service'
__fish_brew_complete_sub_arg 'services' 'run' -l help -d 'Show this message'
__fish_brew_complete_sub_arg 'services' 'run' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_sub_arg 'services' 'run' -l sudo-service-user -d 'When run as root on macOS, run the service(s) as this user'
__fish_brew_complete_sub_arg 'services' 'run' -l verbose -d 'Make some output more verbose'
__fish_brew_complete_sub_arg 'services' 'run' -a '(__fish_brew_suggest_services)'
__fish_brew_complete_sub_arg 'services' 'list ls' -l debug -d 'Display any debugging information'
__fish_brew_complete_sub_arg 'services' 'list ls' -l help -d 'Show this message'
__fish_brew_complete_sub_arg 'services' 'list ls' -l json -d 'Output as JSON'
__fish_brew_complete_sub_arg 'services' 'list ls' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_sub_arg 'services' 'list ls' -l sudo-service-user -d 'When run as root on macOS, run the service(s) as this user'
__fish_brew_complete_sub_arg 'services' 'list ls' -l verbose -d 'Make some output more verbose'
__fish_brew_complete_cmd 'uninstall' 'Uninstall a formula or cask'
__fish_brew_complete_arg 'uninstall' -l cask -d 'Treat all named arguments as casks'
__fish_brew_complete_arg 'uninstall' -l debug -d 'Display any debugging information'
__fish_brew_complete_arg 'uninstall' -l force -d 'Delete all installed versions of formula. Uninstall even if cask is not installed, overwrite existing files and ignore errors when removing files'
__fish_brew_complete_arg 'uninstall' -l formula -d 'Treat all named arguments as formulae'
__fish_brew_complete_arg 'uninstall' -l help -d 'Show this message'
__fish_brew_complete_arg 'uninstall' -l ignore-dependencies -d 'Don\'t fail uninstall, even if formula is a dependency of any installed formulae'
__fish_brew_complete_arg 'uninstall' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_arg 'uninstall' -l verbose -d 'Make some output more verbose'
__fish_brew_complete_arg 'uninstall' -l zap -d 'Remove all files associated with a cask. *May remove files which are shared between applications.*'
__fish_brew_complete_arg 'uninstall; and not __fish_seen_argument -l cask -l casks' -a '(__fish_brew_suggest_formulae_installed)'
__fish_brew_complete_arg 'uninstall; and not __fish_seen_argument -l formula -l formulae' -a '(__fish_brew_suggest_casks_installed)'
__fish_brew_complete_cmd 'untap' 'Remove a tapped formula repository'
__fish_brew_complete_arg 'untap' -l debug -d 'Display any debugging information'
__fish_brew_complete_arg 'untap' -l force -d 'Uninstall all formulae and casks from this tap with `--force` before untapping'
__fish_brew_complete_arg 'untap' -l help -d 'Show this message'
__fish_brew_complete_arg 'untap' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_arg 'untap' -l verbose -d 'Make some output more verbose'
__fish_brew_complete_arg 'untap' -a '(__fish_brew_suggest_taps_installed)'
__fish_brew_complete_cmd 'upgrade' 'Upgrade outdated, unpinned packages using the same options they were originally installed with, plus any appended brew formula options'
__fish_brew_complete_arg 'upgrade' -l cask -d 'Treat all named arguments as casks. If no named arguments are specified, upgrade only outdated casks'
__fish_brew_complete_arg 'upgrade' -l debug -d 'If brewing fails, open an interactive debugging session with access to IRB or a shell inside the temporary build directory'
__fish_brew_complete_arg 'upgrade' -l dry-run -d 'Show what would be upgraded, but do not actually upgrade anything'
__fish_brew_complete_arg 'upgrade' -l force -d 'Install formulae without checking for previously installed keg-only or non-migrated versions. When installing casks, overwrite existing files (binaries and symlinks are excluded, unless originally from the same cask)'
__fish_brew_complete_arg 'upgrade' -l formula -d 'Treat all named arguments as formulae. If no named arguments are specified, upgrade only outdated formulae'
__fish_brew_complete_arg 'upgrade' -l greedy -d 'Also include casks with `version :latest` and `auto_updates true` casks that would otherwise be skipped. Enabled by default if `$HOMEBREW_UPGRADE_GREEDY` is set'
__fish_brew_complete_arg 'upgrade' -l help -d 'Show this message'
__fish_brew_complete_arg 'upgrade' -l no-ask -d 'Do not ask for confirmation before downloading and upgrading. Ask mode is the default. Enabled by default if `$HOMEBREW_NO_ASK` is set'
__fish_brew_complete_arg 'upgrade' -l quiet -d 'Make some output more quiet'
__fish_brew_complete_arg 'upgrade' -l verbose -d 'Print the verification and post-install steps'
__fish_brew_complete_arg 'upgrade; and not __fish_seen_argument -l cask -l casks' -a '(__fish_brew_suggest_formulae_installed)'
__fish_brew_complete_arg 'upgrade; and not __fish_seen_argument -l formula -l formulae' -a '(__fish_brew_suggest_casks_installed)'
__fish_brew_complete_cmd 'services' "Integrates Homebrew formulae with macOS's launchctl manager"
__fish_brew_complete_arg 'services; and [ (count (__fish_brew_args)) = 1 ]' -s v -l verbose -d "Print more details"
__fish_brew_complete_sub_cmd 'services' 'list'    "List all running services for the current user"
__fish_brew_complete_sub_cmd 'services' 'run'     "Run service without starting at login/boot"
__fish_brew_complete_sub_cmd 'services' 'start'   "Start service immediately and register it to launch at login/boot"
__fish_brew_complete_sub_cmd 'services' 'stop'    "Stop service immediately and unregister it from launching at login/boot"
__fish_brew_complete_sub_arg 'services' 'run start stop restart' -l all -d "Run all available services"
__fish_brew_complete_sub_arg 'services' 'run start stop restart' -a '(__fish_brew_suggest_services)'
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/brew/completions_test.go:12:44: undefined: Completions
    internal/brew/completions_test.go:19:12: undefined: ParseFish
    internal/brew/completions_test.go:124:13: undefined: fishTokens
```

- [ ] **Step 5: Create `internal/brew/completions.go`**

```go
package brew

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Completions describes brew's commands for completing a command line. It
// comes from the fish completion script brew ships, which lists every
// command, subcommand and flag with a description, and names the kind of
// argument each command takes.
type Completions struct {
	Commands []Command // sorted by name
}

// Command is one brew command (or a subcommand of one).
type Command struct {
	Name  string
	Desc  string
	Flags []Flag
	Args  []string  // kinds of argument, e.g. "formulae_installed", "services"
	Subs  []Command // subcommands, sorted by name
}

// Flag is a long option such as --dry-run.
type Flag struct {
	Name string // with the leading "--"
	Desc string
}

// Command looks up a command by name.
func (c Completions) Command(name string) (Command, bool) {
	return find(c.Commands, name)
}

// Sub looks up a subcommand by name.
func (c Command) Sub(name string) (Command, bool) {
	return find(c.Subs, name)
}

func find(cmds []Command, name string) (Command, bool) {
	i, ok := slices.BinarySearchFunc(cmds, name, func(c Command, n string) int { return strings.Compare(c.Name, n) })
	if !ok {
		return Command{}, false
	}
	return cmds[i], true
}

// suggestPrefix starts the fish functions that list argument values.
const suggestPrefix = "(__fish_brew_suggest_"

// ParseFish reads brew's fish completion script. Lines it doesn't
// recognise are skipped; when a command is described twice (a tap can
// append its own block), the first description wins.
func ParseFish(r io.Reader) (Completions, error) {
	cmds := map[string]*Command{}
	subs := map[string]map[string]*Command{} // command → subcommand
	get := func(name string) *Command {
		if cmds[name] == nil {
			cmds[name] = &Command{Name: name}
		}
		return cmds[name]
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	var subArgs [][]string // applied once every subcommand is known
	for sc.Scan() {
		tok := fishTokens(sc.Text())
		if len(tok) < 2 {
			continue
		}
		switch tok[0] {
		case "__fish_brew_complete_cmd":
			if c := get(tok[1]); c.Desc == "" && len(tok) > 2 {
				c.Desc = tok[2]
			}
		case "__fish_brew_complete_arg":
			// The first argument is the command, maybe followed by a fish
			// condition ("upgrade; and not __fish_seen_argument …").
			name, _, _ := strings.Cut(tok[1], ";")
			addOptions(get(strings.TrimSpace(name)), tok[2:])
		case "__fish_brew_complete_sub_cmd":
			if len(tok) < 3 {
				continue
			}
			if subs[tok[1]] == nil {
				subs[tok[1]] = map[string]*Command{}
			}
			if s := subs[tok[1]][tok[2]]; s == nil {
				s = &Command{Name: tok[2]}
				if len(tok) > 3 {
					s.Desc = tok[3]
				}
				subs[tok[1]][tok[2]] = s
			}
		case "__fish_brew_complete_sub_arg":
			subArgs = append(subArgs, tok)
		}
	}
	if err := sc.Err(); err != nil {
		return Completions{}, err
	}
	for _, tok := range subArgs {
		if len(tok) < 3 {
			continue
		}
		// One line can apply to several subcommands and their aliases:
		// 'start launch load s l'. Only described subcommands are kept.
		names, _, _ := strings.Cut(tok[2], ";")
		for _, name := range strings.Fields(names) {
			if s := subs[tok[1]][name]; s != nil {
				addOptions(s, tok[3:])
			}
		}
	}
	var out Completions
	for name, c := range cmds {
		for _, s := range subs[name] {
			c.Subs = append(c.Subs, *s)
		}
		sortByName(c.Subs)
		out.Commands = append(out.Commands, *c)
	}
	sortByName(out.Commands)
	return out, nil
}

// addOptions reads "-l flag -d desc" and "-a '(__fish_brew_suggest_kind)'"
// from the rest of a completion line.
func addOptions(c *Command, opts []string) {
	var long, desc, arg string
	for i := 0; i+1 < len(opts); i++ {
		switch opts[i] {
		case "-l":
			long = opts[i+1]
		case "-d":
			desc = opts[i+1]
		case "-a":
			arg = opts[i+1]
		default:
			continue
		}
		i++
	}
	if long != "" && !slices.ContainsFunc(c.Flags, func(f Flag) bool { return f.Name == "--"+long }) {
		c.Flags = append(c.Flags, Flag{Name: "--" + long, Desc: desc})
	}
	if kind, ok := strings.CutPrefix(arg, suggestPrefix); ok {
		kind = strings.TrimSuffix(kind, ")")
		if !slices.Contains(c.Args, kind) {
			c.Args = append(c.Args, kind)
		}
	}
}

func sortByName(cmds []Command) {
	slices.SortFunc(cmds, func(a, b Command) int { return strings.Compare(a.Name, b.Name) })
}

// fishTokens splits a fish command line into words, undoing quotes and
// backslash escapes.
func fishTokens(line string) []string {
	var toks []string
	var cur strings.Builder
	inTok, quote, escaped := false, rune(0), false
	for _, r := range line {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped, inTok = true, true
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			cur.WriteRune(r)
		case r == '\'' || r == '"':
			quote, inTok = r, true
		case r == ' ' || r == '\t':
			if inTok {
				toks = append(toks, cur.String())
				cur.Reset()
				inTok = false
			}
		default:
			cur.WriteRune(r)
			inTok = true
		}
	}
	if inTok {
		toks = append(toks, cur.String())
	}
	return toks
}

// Completions reads the fish completion script from brew's repository.
func (c *Client) Completions(ctx context.Context) (Completions, error) {
	out, err := c.query(ctx, "--repository")
	if err != nil {
		return Completions{}, err
	}
	f, err := os.Open(filepath.Join(strings.TrimSpace(string(out)), "completions", "fish", "brew.fish"))
	if err != nil {
		return Completions{}, err
	}
	defer f.Close()
	return ParseFish(f)
}
```

- [ ] **Step 6: Run the live test**

```bash
go test -tags live -run TestLiveCompletions -count=1 ./internal/brew/   # expect ok
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(brew): read command completions from brew's fish script"
```

---

### Task 8: Complete commands, flags and packages in a popup

**Files:**
- Modify: `cmd/brewery/main.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/cmdline.go`
- Create: `internal/ui/complete.go`
- Create: `internal/ui/complete_test.go`
- Modify: `internal/ui/confirm.go`

**Interfaces:**
- **Consumes:** Task 7's `brew.Completions`; `cmdline`; `overlay`.
- **Produces:**
  - `candidate{text, desc}`, `builtins`, `everyday`, `popupRows = 8`.
  - `(*App) UseCompletions(load func(context.Context) (brew.Completions, error))`; `Init` now also loads them (`completionsMsg`). Without them, brewery's own commands still complete.
  - `(*App) candidates(line) (word string, cs []candidate)`, `argValues(kind)`, `matching`, `suggest(force bool)`.
  - `cmdline` gains `items`, `sel`, `base`, `cycle(dir)`, `popup(st, w) string`; `popupStart(n, sel, rows) int`.
  - `overlayAt(body, box string, w, h, x, y int) string`; `overlay` now centres through it.
- **`main.go`:** `app.UseCompletions(client.Completions)`.

The popup refreshes after every edit. `tab` and `shift+tab` cycle through it, putting the selection in place of the word being typed; typing again recomputes it.

- [ ] **Step 1: Create `internal/ui/complete_test.go`**

```go
package ui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
)

// testCompletions is a small slice of what brew's fish script describes.
var testCompletions = brew.Completions{Commands: []brew.Command{
	{Name: "info", Desc: "Display brief statistics", Args: []string{"formulae_all", "casks_all"}},
	{Name: "pin", Desc: "Pin the specified package", Args: []string{"formulae_installed"}},
	{Name: "services", Desc: "Manage background services", Subs: []brew.Command{
		{Name: "start", Desc: "Start the service", Args: []string{"services"}, Flags: []brew.Flag{{Name: "--all", Desc: "Start all services"}}},
		{Name: "stop", Desc: "Stop the service", Args: []string{"services"}},
	}},
	{Name: "uninstall", Desc: "Uninstall a formula or cask", Args: []string{"formulae_installed", "casks_installed"},
		Flags: []brew.Flag{{Name: "--force", Desc: "Delete all installed versions"}, {Name: "--zap", Desc: "Remove all files"}}},
	{Name: "untap", Desc: "Remove a tapped formula repository", Args: []string{"taps_installed"}},
	{Name: "update", Desc: "Fetch the newest version of Homebrew"},
	{Name: "update-report", Desc: "The output of brew update"},
	{Name: "upgrade", Desc: "Upgrade outdated packages", Args: []string{"formulae_installed", "casks_installed"},
		Flags: []brew.Flag{{Name: "--dry-run", Desc: "Show what would be upgraded"}}},
}}

func newCompletingApp(t *testing.T, fs *fakeStarter) *App {
	t.Helper()
	a := newJobApp(t, fs)
	a.comp = testCompletions
	return a
}

func candidateTexts(cs []candidate) string {
	var s []string
	for _, c := range cs {
		s = append(s, c.text)
	}
	return strings.Join(s, " ")
}

func TestCandidates(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	for _, tt := range []struct {
		line, word, want string
	}{
		{"u", "u", "uninstall untap update upgrade update-report"}, // everyday commands first
		{"brew up", "up", "update upgrade update-report"},
		{"j", "j", "jobs"},
		{"upgrade ", "", "fd git json-c lua-language-server pcre2 docker-desktop fontbase"},
		{"upgrade f", "f", "fd fontbase"},
		{"upgrade fd ", "", "git json-c lua-language-server pcre2 docker-desktop fontbase"},
		{"uninstall --", "--", "--force --zap"},
		{"services ", "", "start stop"},
		{"services st", "st", "start stop"},
		{"services start ", "", "postgresql@16 redis"},
		{"services start --", "--", "--all"},
		{"untap ", "", "anomalyco/tap homebrew/core"},
		{"info ", "", ""},
		{"frobnicate x", "x", ""},
	} {
		word, cs := a.candidates(tt.line)
		if word != tt.word || candidateTexts(cs) != tt.want {
			t.Errorf("candidates(%q) = %q, %q; want %q, %q", tt.line, word, candidateTexts(cs), tt.word, tt.want)
		}
	}
	_, cs := a.candidates("upgrade f")
	if cs[0].desc != "Simple, fast and user-friendly alternative to find" {
		t.Errorf("a package's description should come along: %q", cs[0].desc)
	}
}

func TestTypingShowsThePopup(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "up")
	out := stripANSI(a.render())
	for _, want := range []string{"update", "Fetch the newest version of Homebrew", "upgrade", "Upgrade outdated packages"} {
		if !strings.Contains(out, want) {
			t.Errorf("popup missing %q:\n%s", want, out)
		}
	}
	if h := lipgloss.Height(a.render()); h != 40 {
		t.Errorf("frame is %d lines with the popup, want 40", h)
	}
	lines := strings.Split(out, "\n")
	if !strings.HasPrefix(lines[len(lines)-2], "╰") {
		t.Errorf("the popup should sit just above the prompt:\n%s", strings.Join(lines[len(lines)-4:], "\n"))
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "tab complete") {
		t.Errorf("the prompt should hint at tab: %q", bar)
	}
}

func TestTabCyclesAndInserts(t *testing.T) {
	fs := &fakeStarter{}
	a := newCompletingApp(t, fs)
	keys(t, a, ":")
	typeText(t, a, "up")
	keys(t, a, "tab")
	if v := a.cmd.value(); v != "update" {
		t.Errorf("first tab: %q", v)
	}
	keys(t, a, "tab")
	if v := a.cmd.value(); v != "upgrade" {
		t.Errorf("second tab: %q", v)
	}
	keys(t, a, "shift+tab")
	if v := a.cmd.value(); v != "update" {
		t.Errorf("shift+tab: %q", v)
	}
	keys(t, a, "tab", " ", "l", "tab")
	if v := a.cmd.value(); v != "upgrade lua-language-server" {
		t.Errorf("completing a package: %q", v)
	}
	keys(t, a, "enter")
	if want := []string{"upgrade", "lua-language-server"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestTabOnAnEmptyLineListsCommands(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	keys(t, a, ":")
	if out := stripANSI(a.render()); strings.Contains(out, "Upgrade outdated packages") {
		t.Error("the popup should wait for input or tab")
	}
	keys(t, a, "tab")
	if v := a.cmd.value(); v != "info" {
		t.Errorf("tab on an empty line should start at the first command, got %q", v)
	}
}

func TestNoPopupWithoutMatches(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "zz")
	if len(a.cmd.items) != 0 || a.cmd.popup(a.st, a.width) != "" {
		t.Errorf("no matches, no popup: %+v", a.cmd.items)
	}
	keys(t, a, "tab")
	if v := a.cmd.value(); v != "zz" {
		t.Errorf("tab with no matches changes nothing, got %q", v)
	}
}

func TestCompletionsLoadAtStart(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseCompletions(func(context.Context) (brew.Completions, error) { return testCompletions, nil })
	drain(t, a, a.Init())
	if _, ok := a.comp.Command("upgrade"); !ok {
		t.Fatal("completions should load when the app starts")
	}

	b := newJobApp(t, &fakeStarter{})
	b.UseCompletions(func(context.Context) (brew.Completions, error) {
		return brew.Completions{}, errors.New("no fish script")
	})
	drain(t, b, b.Init())
	if _, cs := b.candidates("j"); candidateTexts(cs) != "jobs" {
		t.Errorf("without brew's completions, brewery's own commands still complete: %q", candidateTexts(cs))
	}
}

func TestPopupWindow(t *testing.T) {
	for _, tt := range []struct{ n, sel, rows, start int }{
		{5, -1, 8, 0},
		{20, -1, 8, 0},
		{20, 7, 8, 0},
		{20, 8, 8, 1},
		{20, 19, 8, 12},
	} {
		if got := popupStart(tt.n, tt.sel, tt.rows); got != tt.start {
			t.Errorf("popupStart(%d, %d, %d) = %d, want %d", tt.n, tt.sel, tt.rows, got, tt.start)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/complete_test.go:35:4: a.comp undefined (type *App has no field or method comp)
    internal/ui/complete_test.go:39:26: undefined: candidate
    internal/ui/complete_test.go:67:17: a.candidates undefined (type *App has no field or method candidates)
```

- [ ] **Step 3: Edit `cmd/brewery/main.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	hist := jobs.History{Dir: jobs.DefaultHistoryDir()}
	recs, _ := hist.Load() // no history yet is fine
	app.UseHistory(hist, recs)

	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
```

   with:

```go
	hist := jobs.History{Dir: jobs.DefaultHistoryDir()}
	recs, _ := hist.Load() // no history yet is fine
	app.UseHistory(hist, recs)
	app.UseCompletions(client.Completions)

	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
```

- [ ] **Step 4: Edit `internal/ui/app.go`** (7 replacements; each old block appears exactly once)

1. Replace:

```go
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/jobs"
	"brewery/internal/state"
	"brewery/internal/theme"
```

   with:

```go
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/jobs"
	"brewery/internal/state"
	"brewery/internal/theme"
```

2. Replace:

```go
	confirm *confirmBox // open confirmation, or nil
	flash   string      // one-off status message, cleared by the next key

	tab           int
	width, height int
	tabH          int    // height left for the tab when the drawer is shown
```

   with:

```go
	confirm *confirmBox // open confirmation, or nil
	flash   string      // one-off status message, cleared by the next key

	comp     brew.Completions // brew's commands, for completing the prompt
	loadComp func(context.Context) (brew.Completions, error)

	tab           int
	width, height int
	tabH          int    // height left for the tab when the drawer is shown
```

3. Replace:

```go
	return a
}

func (a *App) Init() tea.Cmd { return a.refresh() }

// UseHistory saves finished jobs to h, starting from the records already
// there (newest first).
```

   with:

```go
	return a
}

func (a *App) Init() tea.Cmd { return tea.Batch(a.refresh(), a.loadCompletions()) }

// UseHistory saves finished jobs to h, starting from the records already
// there (newest first).
```

4. Replace:

```go
		a.saveErr = msg.err
	case historySavedMsg:
		a.histErr = msg.err
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	case tea.PasteMsg:
```

   with:

```go
		a.saveErr = msg.err
	case historySavedMsg:
		a.histErr = msg.err
	case completionsMsg:
		if msg.err == nil {
			a.comp = msg.comp // without them, only brewery's own commands complete
		}
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	case tea.PasteMsg:
```

5. Replace:

```go
		case a.mode == modeInsert && a.proc != nil:
			a.proc.Write([]byte(msg.Content)) // raw: sudo wants no bracketed-paste wrapping
		case a.mode == modeCommand:
			return a.cmd.update(msg)
		}
	case jobOutputMsg, jobEOFMsg, jobExitMsg, jobRetryMsg, tickMsg:
		return a.handleJob(msg)
```

   with:

```go
		case a.mode == modeInsert && a.proc != nil:
			a.proc.Write([]byte(msg.Content)) // raw: sudo wants no bracketed-paste wrapping
		case a.mode == modeCommand:
			cmd := a.cmd.update(msg)
			a.suggest(false)
			return cmd
		}
	case jobOutputMsg, jobEOFMsg, jobExitMsg, jobRetryMsg, tickMsg:
		return a.handleJob(msg)
```

6. Replace:

```go
	if a.drawerShown() {
		parts = append(parts, a.drawer.view(a.width, a.bodyHeight()-bodyH, a.now(), a.frame, a.mode == modeInsert))
	}
	return lipgloss.JoinVertical(lipgloss.Left, append(parts, a.statusBar())...)
}

// bodyHeight is the space between the tab bar (tabs + rule) and the status bar.
```

   with:

```go
	if a.drawerShown() {
		parts = append(parts, a.drawer.view(a.width, a.bodyHeight()-bodyH, a.now(), a.frame, a.mode == modeInsert))
	}
	screen := lipgloss.JoinVertical(lipgloss.Left, append(parts, a.statusBar())...)
	if p := a.cmd.popup(a.st, a.width); a.mode == modeCommand && p != "" {
		h := lipgloss.Height(screen)
		screen = overlayAt(screen, p, a.width, h, 0, max(h-1-lipgloss.Height(p), 0))
	}
	return screen
}

// bodyHeight is the space between the tab bar (tabs + rule) and the status bar.
```

7. Replace:

```go
	st := a.st
	if a.mode == modeCommand {
		badge := st.Mode("COMMAND").Render("COMMAND") + " "
		line := badge + a.cmd.view(a.width-lipgloss.Width(badge)-1)
		return st.StatusBar.Width(a.width).Render(ansi.Truncate(line, a.width, ""))
	}
	badge := "NORMAL"
```

   with:

```go
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
```

- [ ] **Step 5: Edit `internal/ui/cmdline.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go
	past  []string // earlier commands, oldest first
	back  int      // how far up into past the line is; 0 is the draft
	draft string   // what was typed before browsing past commands
}

func newCmdline(st theme.Styles) cmdline {
```

   with:

```go
	past  []string // earlier commands, oldest first
	back  int      // how far up into past the line is; 0 is the draft
	draft string   // what was typed before browsing past commands

	items []candidate // the completion popup, empty when hidden
	sel   int         // the selected item, -1 for none
	base  string      // the line before the word being completed
}

func newCmdline(st theme.Styles) cmdline {
```

2. Replace:

```go
func (c *cmdline) open() {
	c.input.Reset()
	c.input.Focus()
	c.back, c.draft = 0, ""
}

func (c *cmdline) value() string { return c.input.Value() }
```

   with:

```go
func (c *cmdline) open() {
	c.input.Reset()
	c.input.Focus()
	c.back, c.draft, c.items = 0, "", nil
}

func (c *cmdline) value() string { return c.input.Value() }
```

3. Replace:

```go
	case "esc":
		a.mode = modeNormal
		return nil
	case "enter":
		a.mode = modeNormal
		line := strings.TrimSpace(a.cmd.value())
```

   with:

```go
	case "esc":
		a.mode = modeNormal
		return nil
	case "tab", "shift+tab":
		if len(a.cmd.items) == 0 {
			a.suggest(true)
		}
		dir := 1
		if msg.String() == "shift+tab" {
			dir = -1
		}
		a.cmd.cycle(dir)
		return nil
	case "enter":
		a.mode = modeNormal
		line := strings.TrimSpace(a.cmd.value())
```

4. Replace:

```go
		a.cmd.remember(line)
		return a.execute(line)
	}
	return a.cmd.update(msg)
}

// execute runs a line typed at the prompt. A leading "brew" is optional.
```

   with:

```go
		a.cmd.remember(line)
		return a.execute(line)
	}
	cmd := a.cmd.update(msg)
	a.suggest(false)
	return cmd
}

// execute runs a line typed at the prompt. A leading "brew" is optional.
```

- [ ] **Step 6: Create `internal/ui/complete.go`**

```go
package ui

import (
	"context"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/theme"
)

// candidate is one way to complete the word being typed.
type candidate struct{ text, desc string }

// builtins are brewery's own commands, completed alongside brew's.
var builtins = []candidate{{"jobs", "List finished jobs and replay their output"}}

// everyday are the commands most people use. They complete before brew's
// many developer commands, so "up" offers upgrade before update-report.
var everyday = map[string]bool{
	"autoremove": true, "bundle": true, "cleanup": true, "config": true, "deps": true,
	"desc": true, "doctor": true, "edit": true, "home": true, "info": true, "install": true,
	"jobs": true, "leaves": true, "link": true, "list": true, "outdated": true, "pin": true,
	"reinstall": true, "search": true, "services": true, "tap": true, "uninstall": true,
	"unlink": true, "unpin": true, "untap": true, "update": true, "upgrade": true, "uses": true,
}

// popupRows is how many candidates the popup shows at once.
const popupRows = 8

// completionsMsg carries brew's completions, read once at start.
type completionsMsg struct {
	comp brew.Completions
	err  error
}

// UseCompletions sets how the app reads brew's command completions; it
// reads them once when it starts.
func (a *App) UseCompletions(load func(context.Context) (brew.Completions, error)) {
	a.loadComp = load
}

func (a *App) loadCompletions() tea.Cmd {
	if a.loadComp == nil {
		return nil
	}
	load, ctx := a.loadComp, a.ctx
	return func() tea.Msg {
		c, err := load(ctx)
		return completionsMsg{c, err}
	}
}

// candidates lists what can complete the last word of line, and returns
// that word. A first word completes to a command; after it come the
// command's subcommands, its flags (once the word starts with "-"), or
// the installed packages, taps or services it takes.
func (a *App) candidates(line string) (word string, cs []candidate) {
	words := strings.Fields(line)
	if !strings.HasSuffix(line, " ") && len(words) > 0 {
		word, words = words[len(words)-1], words[:len(words)-1]
	}
	if len(words) > 0 && words[0] == "brew" {
		words = words[1:]
	}
	if len(words) == 0 {
		cs = append(cs, builtins...)
		for _, c := range a.comp.Commands {
			cs = append(cs, candidate{c.Name, c.Desc})
		}
		slices.SortStableFunc(cs, func(x, y candidate) int {
			if everyday[x.text] != everyday[y.text] {
				if everyday[x.text] {
					return -1
				}
				return 1
			}
			return strings.Compare(x.text, y.text)
		})
		return word, matching(cs, word)
	}
	cmd, ok := a.comp.Command(words[0])
	if !ok {
		return word, nil
	}
	if len(words) > 1 {
		if sub, ok := cmd.Sub(words[1]); ok {
			cmd = sub
		}
	} else if len(cmd.Subs) > 0 && !strings.HasPrefix(word, "-") {
		for _, s := range cmd.Subs {
			cs = append(cs, candidate{s.Name, s.Desc})
		}
		return word, matching(cs, word)
	}
	if strings.HasPrefix(word, "-") {
		for _, f := range cmd.Flags {
			cs = append(cs, candidate{f.Name, f.Desc})
		}
		return word, matching(cs, word)
	}
	for _, kind := range cmd.Args {
		cs = append(cs, a.argValues(kind)...)
	}
	cs = slices.DeleteFunc(cs, func(c candidate) bool { return slices.Contains(words, c.text) })
	return word, matching(cs, word)
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
			want = brew.KindCask
		}
		for _, p := range a.snap.Packages {
			if p.Kind == want {
				cs = append(cs, candidate{p.Name, p.Desc})
			}
		}
	case "formulae_outdated", "casks_outdated":
		want := brew.KindFormula
		if kind == "casks_outdated" {
			want = brew.KindCask
		}
		for _, o := range a.snap.Outdated {
			if o.Kind == want {
				cs = append(cs, candidate{o.Name, o.Installed + " → " + o.Current})
			}
		}
	case "taps_installed":
		for _, t := range a.snap.Taps {
			cs = append(cs, candidate{t.Name, t.Remote})
		}
	case "services":
		for _, s := range a.snap.Services {
			cs = append(cs, candidate{s.Name, s.Status})
		}
	case "commands":
		for _, c := range a.comp.Commands {
			cs = append(cs, candidate{c.Name, c.Desc})
		}
	}
	slices.SortStableFunc(cs, func(x, y candidate) int { return strings.Compare(x.text, y.text) })
	return cs
}

// matching keeps the candidates that start with word.
func matching(cs []candidate, word string) []candidate {
	return slices.DeleteFunc(cs, func(c candidate) bool { return !strings.HasPrefix(c.text, word) })
}

// suggest refreshes the popup for the line as typed. Without force, an
// empty line shows nothing; tab forces the full list.
func (a *App) suggest(force bool) {
	line := a.cmd.value()
	if strings.TrimSpace(line) == "" && !force {
		a.cmd.items = nil
		return
	}
	word, cs := a.candidates(line)
	a.cmd.items, a.cmd.sel, a.cmd.base = cs, -1, line[:len(line)-len(word)]
}

// cycle moves the popup's selection by dir (1 or -1) and puts the
// selected candidate in place of the word being typed.
func (c *cmdline) cycle(dir int) {
	n := len(c.items)
	if n == 0 {
		return
	}
	if c.sel < 0 && dir < 0 {
		c.sel = 0
	}
	c.sel = (c.sel + dir + n) % n
	c.show(c.base + c.items[c.sel].text)
}

// popup renders the candidates as a list for above the prompt, or ""
// when there are none. Long lists scroll to keep the selection in view.
func (c *cmdline) popup(st theme.Styles, w int) string {
	if len(c.items) == 0 {
		return ""
	}
	nameW, descW := 0, 0
	for _, it := range c.items {
		nameW = max(nameW, lipgloss.Width(it.text))
		descW = max(descW, lipgloss.Width(it.desc))
	}
	nameW = min(nameW, 30)
	inner := min(nameW+2+descW, 72, max(w-4, 10))
	start := popupStart(len(c.items), c.sel, popupRows)
	var rows []string
	for i := start; i < min(start+popupRows, len(c.items)); i++ {
		it := c.items[i]
		name := ansi.Truncate(it.text, nameW, "…")
		desc := ansi.Truncate(it.desc, max(inner-nameW-2, 0), "…")
		row := name + strings.Repeat(" ", nameW-lipgloss.Width(name)+2)
		if i == c.sel {
			rows = append(rows, st.TableSelected.Width(inner).Render(row+desc))
		} else {
			rows = append(rows, st.Text.Render(row)+st.Muted.Render(desc))
		}
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(st.P.Blue).Padding(0, 1)
	return box.Width(inner + 4).Render(strings.Join(rows, "\n"))
}

// popupStart is the first of rows visible candidates out of n, scrolled so
// the selection sel (-1 for none) is in view.
func popupStart(n, sel, rows int) int {
	if sel < rows {
		return 0
	}
	return min(sel-rows+1, n-rows)
}
```

- [ ] **Step 7: Edit `internal/ui/confirm.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go

// overlay draws box centered over body, which is w×h; the result is w×h.
func overlay(body, box string, w, h int) string {
	x := max((w-lipgloss.Width(box))/2, 0)
	y := max((h-lipgloss.Height(box))/2, 0)
	canvas := lipgloss.NewCanvas(w, h)
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(body),
```

   with:

```go

// overlay draws box centered over body, which is w×h; the result is w×h.
func overlay(body, box string, w, h int) string {
	return overlayAt(body, box, w, h, max((w-lipgloss.Width(box))/2, 0), max((h-lipgloss.Height(box))/2, 0))
}

// overlayAt draws box over body with its top-left corner at x, y.
func overlayAt(body, box string, w, h, x, y int) string {
	canvas := lipgloss.NewCanvas(w, h)
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(body),
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(ui): complete commands, flags and packages in a popup"
```

---

### Task 9: Hand the terminal to editors and shells

**Files:**
- Modify: `cmd/brewery/main.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/cmdline.go`
- Create: `internal/ui/takeover.go`
- Create: `internal/ui/takeover_test.go`

**Interfaces:**
- **Consumes:** `execute`, `record`, `refresh`, `note`.
- **Produces:**
  - `(*App) UseTakeover(cmd func(args []string) *exec.Cmd)`.
  - `takesOver(args) bool`: `edit`, `sh`, `irb`, `bundle edit`.
  - `(*App) takeover(args) tea.Cmd`, `takeoverDone(takeoverDoneMsg) tea.Cmd`; `App.takeoverCmd`, `App.takingOver`.
  - `execute` treats a leading `!` as "take over".
- **`main.go`:** `app.UseTakeover(func(args []string) *exec.Cmd { return exec.Command(bin, args...) })`.
- **Test helpers:** `fakeTakeover`, `runLine`.

`tea.ExecProcess` runs the command with the terminal's own input and output and resumes brewery when it exits. The exit code comes from `*exec.ExitError`; any other error means brew couldn't start.

- [ ] **Step 1: Create `internal/ui/takeover_test.go`**

```go
package ui

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/jobs"
)

// fakeTakeover records the commands handed the whole terminal.
type fakeTakeover struct{ calls [][]string }

func (f *fakeTakeover) cmd(args []string) *exec.Cmd {
	f.calls = append(f.calls, args)
	return exec.Command("true")
}

// runLine types line at the prompt and presses enter, returning the
// command enter produced without running it.
func runLine(t *testing.T, a *App, line string) tea.Cmd {
	t.Helper()
	keys(t, a, ":")
	typeText(t, a, line)
	_, cmd := a.Update(press("enter"))
	return cmd
}

func TestFullScreenCommandsTakeOver(t *testing.T) {
	for _, tt := range []struct {
		line string
		want []string // nil: runs in the drawer
	}{
		{"edit fd", []string{"edit", "fd"}},
		{"sh", []string{"sh"}},
		{"irb", []string{"irb"}},
		{"bundle edit", []string{"bundle", "edit"}},
		{"!info fd", []string{"info", "fd"}},
		{"! brew log git", []string{"log", "git"}},
		{"bundle list", nil},
		{"info fd", nil},
	} {
		fs, tk := &fakeStarter{}, &fakeTakeover{}
		a := newJobApp(t, fs)
		a.UseTakeover(tk.cmd)
		cmd := runLine(t, a, tt.line)
		if tt.want == nil {
			if len(tk.calls) != 0 {
				t.Errorf(":%s should run in the drawer, took over with %v", tt.line, tk.calls)
			}
			continue
		}
		if len(tk.calls) != 1 || !reflect.DeepEqual(tk.calls[0], tt.want) || cmd == nil {
			t.Errorf(":%s took over with %v (cmd %v), want %v", tt.line, tk.calls, cmd != nil, tt.want)
		}
		if len(fs.calls) != 0 {
			t.Errorf(":%s also started a drawer job: %v", tt.line, fs.calls)
		}
	}
}

func TestTakeoverWaitsForTheRunningJob(t *testing.T) {
	p := newFakeProc(0)
	tk := &fakeTakeover{}
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	a.UseTakeover(tk.cmd)
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	runLine(t, a, "edit fd")
	if len(tk.calls) != 0 {
		t.Error("a full-screen command must wait while a job runs")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "wait for upgrade fd to finish") {
		t.Errorf("status bar should say why: %q", bar)
	}
	p.exit()
	resume(t, a, waiting)
}

func TestTakeoverWithoutAProgram(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	if cmd := runLine(t, a, "edit fd"); cmd != nil {
		t.Error("without a takeover program nothing should run")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "full-screen commands aren't available") {
		t.Errorf("status bar: %q", bar)
	}
}

func TestFinishedTakeoverIsRecordedAndRefreshes(t *testing.T) {
	src := newCountingSource(t)
	tk := &fakeTakeover{}
	a := New(src, (&fakeStarter{}).start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.UseTakeover(tk.cmd)
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	runLine(t, a, "edit fd")
	drain(t, a, func() tea.Msg { return takeoverDoneMsg{job: a.takingOver, err: nil} })

	if len(a.history) != 1 || a.history[0].State != jobs.Done || a.history[0].Title != "edit fd" {
		t.Fatalf("history: %+v", a.history)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "✓ edit fd") {
		t.Errorf("status bar: %q", stripANSI(a.statusBar()))
	}
	for _, q := range []string{"installed", "outdated", "services", "taps", "doctor"} {
		if src.calls[q] != 1 {
			t.Errorf("after a full-screen command, %s ran %d times, want 1", q, src.calls[q])
		}
	}
}

func TestFailedTakeoverKeepsItsExitCode(t *testing.T) {
	tk := &fakeTakeover{}
	a := newJobApp(t, &fakeStarter{})
	a.UseTakeover(tk.cmd)
	runLine(t, a, "sh")
	exitErr := exec.Command("sh", "-c", "exit 3").Run()
	drain(t, a, func() tea.Msg { return takeoverDoneMsg{job: a.takingOver, err: exitErr} })
	if r := a.history[0]; r.State != jobs.Failed || r.ExitCode != 3 {
		t.Errorf("record: %+v", r)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "✗ sh failed (exit 3)") {
		t.Errorf("status bar: %q", bar)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/takeover_test.go:50:5: a.UseTakeover undefined (type *App has no field or method UseTakeover)
    internal/ui/takeover_test.go:71:4: a.UseTakeover undefined (type *App has no field or method UseTakeover)
    internal/ui/takeover_test.go:99:4: a.UseTakeover undefined (type *App has no field or method UseTakeover)
```

- [ ] **Step 3: Edit `cmd/brewery/main.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
```

   with:

```go
	"fmt"
	"io"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
```

2. Replace:

```go
	recs, _ := hist.Load() // no history yet is fine
	app.UseHistory(hist, recs)
	app.UseCompletions(client.Completions)

	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
```

   with:

```go
	recs, _ := hist.Load() // no history yet is fine
	app.UseHistory(hist, recs)
	app.UseCompletions(client.Completions)
	app.UseTakeover(func(args []string) *exec.Cmd { return exec.Command(bin, args...) })

	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
```

- [ ] **Step 4: Edit `internal/ui/app.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
import (
	"context"
	"fmt"
	"strings"
	"time"
```

   with:

```go
import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
```

2. Replace:

```go
	comp     brew.Completions // brew's commands, for completing the prompt
	loadComp func(context.Context) (brew.Completions, error)

	tab           int
	width, height int
	tabH          int    // height left for the tab when the drawer is shown
```

   with:

```go
	comp     brew.Completions // brew's commands, for completing the prompt
	loadComp func(context.Context) (brew.Completions, error)

	takeoverCmd func(args []string) *exec.Cmd // builds full-screen commands
	takingOver  *jobs.Job                     // the full-screen command running now

	tab           int
	width, height int
	tabH          int    // height left for the tab when the drawer is shown
```

3. Replace:

```go
		a.saveErr = msg.err
	case historySavedMsg:
		a.histErr = msg.err
	case completionsMsg:
		if msg.err == nil {
			a.comp = msg.comp // without them, only brewery's own commands complete
```

   with:

```go
		a.saveErr = msg.err
	case historySavedMsg:
		a.histErr = msg.err
	case takeoverDoneMsg:
		return a.takeoverDone(msg)
	case completionsMsg:
		if msg.err == nil {
			a.comp = msg.comp // without them, only brewery's own commands complete
```

- [ ] **Step 5: Edit `internal/ui/cmdline.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go

// execute runs a line typed at the prompt. A leading "brew" is optional.
// Lines run exactly as typed: no confirmation, and brew may auto-update.
func (a *App) execute(line string) tea.Cmd {
	args, err := splitArgs(line)
	if err != nil {
		return a.note(err.Error())
	}
```

   with:

```go

// execute runs a line typed at the prompt. A leading "brew" is optional.
// Lines run exactly as typed: no confirmation, and brew may auto-update.
// Editors and shells take over the terminal, as does any line starting
// with "!".
func (a *App) execute(line string) tea.Cmd {
	full, force := strings.CutPrefix(line, "!")
	args, err := splitArgs(full)
	if err != nil {
		return a.note(err.Error())
	}
```

2. Replace:

```go
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "q", "quit":
		return a.askQuit()
	case "jobs":
		return a.openJobs()
	}
	refresh, full := refreshAfter(args)
	return a.run(action{title: strings.Join(args, " "), args: args, refresh: refresh, full: full, typed: true})
}

// readOnly are brew commands that change nothing brewery shows, so no
```

   with:

```go
	if len(args) == 0 {
		return nil
	}
	if force || takesOver(args) {
		return a.takeover(args)
	}
	switch args[0] {
	case "q", "quit":
		return a.askQuit()
	case "jobs":
		return a.openJobs()
	}
	refresh, all := refreshAfter(args)
	return a.run(action{title: strings.Join(args, " "), args: args, refresh: refresh, full: all, typed: true})
}

// readOnly are brew commands that change nothing brewery shows, so no
```

- [ ] **Step 6: Create `internal/ui/takeover.go`**

```go
package ui

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/jobs"
)

// takeoverDoneMsg reports that a full-screen command has exited.
type takeoverDoneMsg struct {
	job *jobs.Job
	err error
}

// UseTakeover sets how the app builds a brew command that takes over the
// whole terminal (an editor, a shell). The UI never runs brew itself.
func (a *App) UseTakeover(cmd func(args []string) *exec.Cmd) {
	a.takeoverCmd = cmd
}

// takesOver reports whether brew args need the whole terminal: they open
// an editor or an interactive shell.
func takesOver(args []string) bool {
	switch args[0] {
	case "edit", "sh", "irb":
		return true
	case "bundle":
		return len(args) > 1 && args[1] == "edit"
	}
	return false
}

// takeover suspends brewery and gives brew the terminal until it exits.
// It waits for a running job: while brewery is suspended, nothing reads
// the job's output.
func (a *App) takeover(args []string) tea.Cmd {
	if a.takeoverCmd == nil {
		return a.note("full-screen commands aren't available")
	}
	if cur := a.queue.Current(); cur != nil {
		return a.note("wait for " + cur.Title + " to finish: full-screen commands take over the terminal")
	}
	j := &jobs.Job{Title: strings.Join(args, " "), Args: args, State: jobs.Running, Started: a.now()}
	a.takingOver = j
	return tea.ExecProcess(a.takeoverCmd(args), func(err error) tea.Msg { return takeoverDoneMsg{j, err} })
}

// takeoverDone records how a full-screen command ended and refreshes
// everything, since an editor or shell could have changed anything.
func (a *App) takeoverDone(m takeoverDoneMsg) tea.Cmd {
	a.takingOver = nil
	j, output := m.job, "ran full screen"
	j.Ended, j.State = a.now(), jobs.Done
	var exitErr *exec.ExitError
	switch {
	case errors.As(m.err, &exitErr):
		j.State, j.ExitCode = jobs.Failed, exitErr.ExitCode()
	case m.err != nil:
		j.State, j.ExitCode, output = jobs.Failed, -1, "couldn't start brew: "+m.err.Error()
	}
	if j.State == jobs.Done {
		a.flash = a.st.OK.Render("✓ " + j.Title)
	} else {
		a.flash = a.st.Err.Render(fmt.Sprintf("✗ %s failed (exit %d)", j.Title, j.ExitCode))
	}
	return tea.Batch(a.record(j, output), a.refresh())
}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(ui): hand the terminal to editors and shells"
```

---

### Task 10: End-to-end check in a real terminal

**Files:** none (verification only)

- [ ] **Step 1: Build and run**

Run: `make run`

- [ ] **Step 2: Check each of these by hand.** None of these change anything.
  1. **Popup:** press `:` and type `up`. A blue-bordered list appears just above the prompt with `update` and `upgrade` first, each with its description. Press `tab` twice: the line becomes `update`, then `upgrade`. Type a space: your installed packages appear with their descriptions. Press `esc`.
  2. **A command in the drawer:** type `:info fd` (or any installed package) and press `enter`. The drawer shows brew's output and ends `✓ brew info fd · exit 0`.
  3. **History:** type `:jobs` and press `enter`. The job is listed as `✓ done` with its start time and duration. Press `enter`: the drawer replays its output. Press `esc` twice.
  4. **Home:** press `1`. "Recent jobs" lists `info fd · just now`.
  5. **Takeover:** type `:!--version` and press `enter`. brew prints its version on the plain terminal and brewery comes straight back, flashing `✓ --version`.
  6. **Recall:** press `:` then `↑`: the last command comes back. Press `esc`, then `q`.

- [ ] **Step 3 (optional; this changes your system): answer brew's own question**

On the Outdated tab, press `u` on a package whose upgrade also upgrades a dependency. brew asks `Do you want to proceed with the upgrade? [y/n]`; the drawer switches to INSERT on its own. Press `y`.

- [ ] **Step 4: Final checks**

```bash
go test -race -count=1 ./...
go test -tags live -count=1 ./internal/brew/ ./internal/ui/
go vet ./...
gofmt -l .
go mod tidy -diff
```

Expected: all `ok`, with no output from vet, gofmt, or `go mod tidy -diff`.
