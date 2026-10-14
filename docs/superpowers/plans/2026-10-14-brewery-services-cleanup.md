# brewery Plan 4: Services, cleanup and follow-ups — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish v1's "Maintain" actions. The Services tab gains a preview with each service's details and the end of its log, and `l` follows the log full screen. Cleanup shows what it would free before it runs, from Home or with `c`. Taps can be untapped. Plus the Plan 2 follow-ups: a stopped job that finishes anyway, orphans and "required by" that ignored what casks need, and services actions that relied on row order.

**Architecture:**
- **`internal/brew`** reads services from `brew services info --all --json`, which adds each service's pid, command and log paths. It also learns to run and read `brew cleanup --dry-run`, both the standard cleanup and `--prune=all`, and it fills in what casks depend on.
- **`internal/ui`**:
  - Plan 1's list tab learns to show a preview beside its table, and Services uses it. The preview reads the end of the log file in the background through a reader the tests replace.
  - `l` hands the terminal to a log viewer that `main` builds, the same way Plan 3's takeovers do.
  - Cleanup's dry-runs are a sixth query that follows every full refresh, next to doctor. A dialog and a fifth Home tile use its result.
- **The UI still never runs brew itself.** `main` adds one builder, for the log viewer (`less +F`).

**Tech Stack:**
- Go ≥ 1.25
- Bubble Tea v2, Bubbles v2, Lip Gloss v2
- No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-12-brewery-design.md`:
- section 3: the confirmations for cleanup and untap
- section 4: Services (`l` shows the log; the preview shows its end), Home's reclaimable-cache tile, Taps

**Decided with the user:**
- **Service log:** a preview pane, as on Installed, with the service's details and the end of its log. `l` opens the whole log in `less +F`, which follows new lines; ctrl+c stops following and `q` comes back.
- **Cleanup:** the dialog offers both cleanups, each with what its dry-run says it frees. `[y]` is the standard cleanup: old versions, and downloads more than 120 days old. `[a]` clears the whole download cache with `--prune=all`. On the user's machine the standard cleanup had nothing to remove, while `--prune=all` would free 678.9 MB.
- **Where cleanup lives:** a fifth Home tile shows what the whole cache would free; `c` on any tab, or `enter` on the tile, opens the dialog.

**Found while building this:**
- **On macOS, a background process can't hold a job open.** Plan 2 worried that a process brew leaves behind, still holding the pseudo-terminal, would keep a job from ever finishing. On macOS the terminal's output ends as soon as brew exits, background child or not. So nothing needs to change, and Task 1 pins this with a test. Linux keeps the terminal open until every holder exits, so that port would need a fix; the test would catch it.
- **`brew services info --all --json` costs the same as `services list`** (0.6s) and also gives each service's pid, command and log paths. So the Services query switches to it, and no second query is needed.
- **The two cleanup dry-runs take 1.3s together**, running in parallel. Neither needs brew's lock.
- **Bubble Tea ignores signals while another program has the terminal.** So ctrl+c in `less +F`, which stops following, can't quit brewery.

**Two departures from the spec:**
- **Services columns:** the table keeps NAME and STATUS only. USER and the plist path move into the preview, so names still fit beside it at 80 columns.
- **Home's reclaimable tile:** it shows what `brew cleanup --prune=all` says it would free, rather than the size of `brew --cache`. That is what pressing `a` would actually remove, old versions and logs included.

**Verified:** every code block in this plan was compiled and tested before the plan was written.
- **The edits are exact.** A checker applied this plan's edits to `master` (8017839) and confirmed that:
  - each task reproduces the tested code byte for byte;
  - each task's tests fail before its code and pass after;
  - `go mod tidy` stays clean.
  Every task's commit passes `go test -race` on its own.
- **Against real brew (read-only):** the live tests parse `services info --all --json` and both cleanup dry-runs. The real binary was driven in a pseudo-terminal, with its history and cache pointed at a scratch directory:
  - Home's Cleanup tile showed 678.9MB.
  - `c` offered only the whole cache, since the standard cleanup had nothing to remove, and `n` closed it.
  - The Services preview showed the end of postgres's log.
  - `l` opened it in `less +F`, and ctrl+c then `q` came back to an intact brewery.
  - `x` on a tap something was installed from explained why brew would refuse.

## Global Constraints

These come from Plans 1–3 and still hold:
- Module path `brewery`; Go ≥ 1.25.
- **Only `internal/brew` and `main` run brew.** `internal/ui` gets data through `Source` and starts jobs through `Starter`. It also takes a history store, a completions loader, a takeover command builder and, new here, a log viewer builder from `main`.
- Views use `theme.Styles` only.
- One job at a time; lock retries (5 attempts, backoff 2s, 4s, 8s, 16s, 30s); pane actions get `brew.JobEnv`.
- **Key routing is by mode first:** an open confirmation gets every key; in INSERT every key but `esc` goes to brew; in COMMAND every key edits the line; otherwise the global keys and actions apply.
- The emulator is never closed, and its replies are always drained.
- Refreshes are never dropped; the cache is saved before doctor runs.
- Full-screen programs wait while a job runs: nothing would read the job's output meanwhile.

Added by this plan:
- **A stopped job:** a job the user stopped (ctrl+c) counts as stopped only if it then fails. If brew took the ctrl+c as an answer and exits 0, it is done.
- **Casks' dependencies count:** a cask's `depends_on` formulae and casks, and everything those need, are its `Deps`; the ones it names itself are its `DirectDeps`. So "required by", orphans and leaves all see what casks need. Casks still count as installed on request (brew doesn't record otherwise).
- **Services query:** `brew services info --all --json`. `Service` gains `PID` (0 when not running), `Command`, `Log` (`log_path`) and `ErrorLog` (`error_log_path`). The snapshot cache's version becomes 2, so an older cache, which lacks these, is ignored rather than half-loaded.
- **Services preview** (the right 45% of the tab):
  - The service's status, colored, with ` · pid N` when it runs.
  - `command`, `user`, `file` and `log`, plus `errors` when the error log is a separate file. Paths in the home directory start with `~`.
  - Then, under "End of the log", as many of the log's last lines as fit. Brewery reads the last 16 KiB, starting at a whole line, with colors stripped and a redrawn line kept as drawn last.
  - The log is read when the selection reaches a service, and again after each services refresh or after `l`, never on every frame.
  - With no log path: "brew knows no log for this service". With a missing file: "nothing logged yet".
- **`l` on Services:** opens the log, then the error log if it's a separate file, in `less +F -- <files>`. Files that don't exist are left out; with none left it says "<name> hasn't logged anything yet". Nothing is recorded in the job history.
- **Cleanup's dry-runs:** `brew cleanup --dry-run` and `brew cleanup --dry-run --prune=all`, run in parallel. Each estimate counts the "Would remove…" lines and takes brew's total from "would free approximately <size> of disk space"; an empty output means nothing to remove. They run after every full refresh, alongside doctor, and after a cleanup job; they are cached in the snapshot.
- **Home's fifth tile, Cleanup:**
  - while measuring: `…` / `measuring`
  - after a failure: `✗ failed` / `r retries`
  - with nothing to remove: `✓ tidy` / `nothing to remove`
  - otherwise: the whole cache's size (or the standard cleanup's, if the whole cache's is empty) / `c to reclaim`
  `enter` on it opens the cleanup dialog.
- **`c` on any tab:** opens "Clean up Homebrew?".
  - `[y] standard` is offered if the standard cleanup frees anything; otherwise the dialog says it has nothing to remove.
  - `[a] whole cache` is offered if pruning everything removes more items.
  - `[n] cancel` is always there.
  - The jobs are `cleanup` and `cleanup --prune=all`. Instead of a dialog, a note says "still measuring what cleanup would free" before the dry-runs return, "couldn't measure the cleanup · r retries" after they fail, and "nothing to clean up" when neither would free anything.
- **`x` on Taps:**
  - If an installed package's tap is the selected one, it explains "Can't untap <tap>", lists those packages, and offers only `[n] close`.
  - Otherwise it asks "Untap <tap>?" and says what the tap holds, e.g. "Its 2 formulae and 1 cask won't be available until you tap it again."
  - The job is `untap <tap>` and refreshes taps.
- **Selection by name:** the services and taps actions find the selected row's item by name, not by its position in the snapshot.
- **Status-bar hints:**
  - Home: `1-5 tabs · r refresh · U update · c clean up`
  - Services: `s start · S stop · R restart · l log`
  - Taps: `x untap`

## File Structure

```
internal/jobs/jobs.go          Finish: stopped-then-exit-0 is done
internal/brew/pty_test.go      + a background child can't hold a job open (macOS)
internal/brew/parse.go         cask depends_on + caskDeps; ParseServices doc; ParseCleanup, freeRe
internal/brew/types.go         Service + PID, Command, Log, ErrorLog; CleanupEstimate, Cleanup
internal/brew/client.go        Services via services info --all; (*Client) Cleanup
internal/brew/testdata/services.json  now in `services info --all --json` form
internal/brew/testdata/cleanup.txt    a real `cleanup --dry-run` excerpt
internal/state/snapshot.go     version 2; Cleanup, CleanupRan
internal/ui/list.go            listTab.preview, listWidth; services columns NAME, STATUS
internal/ui/services.go        logTail, readLogTail, selectedService, wantLog, servicePreview, tilde, UsePager, openLog
internal/ui/source.go          Source.Cleanup, qCleanup
internal/ui/home.go            Cleanup tile, openCleanup
internal/ui/actions.go         c, askCleanup; x on Taps, askUntap, selectedTap, tapContents; serviceKey by name; hints
internal/ui/app.go             log state, pager builder, qCleanup after the JSON queries
internal/ui/installed.go       a cask's dependencies show in its preview
cmd/brewery/main.go            wires less +F as the log viewer
```

---

### Task 1: A stopped job that finishes anyway is done

**Files:**
- Modify: `internal/brew/pty_test.go`
- Modify: `internal/jobs/jobs.go`
- Modify: `internal/jobs/jobs_test.go`

**Interfaces:**
- **Consumes:** `jobs.Queue.Finish`.
- **Produces:** `Finish` marks a job `Canceled` only when it was stopped **and** its exit code isn't 0. No other signature changes.

The UI sets `canceled` when the user sends ctrl+c, from the stop dialog or straight to brew in INSERT. brew can take that ctrl+c as an answer to a prompt and still finish cleanly; then the job is `Done`, not "stopped".

The new pseudo-terminal test changes no code. It pins down the macOS behaviour described under "Found while building this": a background child that keeps the terminal open doesn't keep the job from finishing.

- [ ] **Step 1: Edit `internal/brew/pty_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Errorf("a killed command reports -1, got %d", code)
	}
}
```

   with:

```go
		t.Errorf("a killed command reports -1, got %d", code)
	}
}

// A process brew leaves running in the background (a daemon a post-install
// step started) keeps the terminal open, but on macOS the output still ends
// when brew exits, so the job finishes then.
func TestPTYEndsWhenTheCommandExitsDespiteABackgroundChild(t *testing.T) {
	p := sh(t, `(trap '' HUP; sleep 3) & echo started`)
	start := time.Now()
	out, code := finish(t, p)
	if !strings.Contains(out, "started") || code != 0 {
		t.Errorf("got %q, exit %d", out, code)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("output ended after %v: it waited for the background child", d)
	}
}
```

- [ ] **Step 2: Edit `internal/jobs/jobs_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		{0, false, "", Done},
		{1, false, "Error: No such keg", Failed},
		{-1, true, "", Canceled},
		{1, false, "Error: A `brew upgrade` process has already locked /opt/homebrew/Cellar/fd.lock.", Waiting},
	}
	for _, tt := range tests {
```

   with:

```go
		{0, false, "", Done},
		{1, false, "Error: No such keg", Failed},
		{-1, true, "", Canceled},
		{130, true, "", Canceled},
		{0, true, "", Done}, // brew took the ctrl+c as an answer and finished anyway
		{1, false, "Error: A `brew upgrade` process has already locked /opt/homebrew/Cellar/fd.lock.", Waiting},
	}
	for _, tt := range tests {
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    --- FAIL: TestFinishStates (0.00s)
```

- [ ] **Step 4: Edit `internal/jobs/jobs.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go

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
```

   with:

```go

// Finish records how a job's run ended. If brew refused to run because
// another brew process held the lock, the job waits to retry instead of
// failing, until MaxAttempts runs have been tried. A job the user stopped
// counts as stopped only if it then failed: brew can take ctrl+c as an
// answer and finish anyway. It reports whether the job will be retried.
func (q *Queue) Finish(j *Job, code int, canceled bool, output string, now time.Time) (retry bool) {
	j.ExitCode = code
	switch {
	case canceled && code != 0:
		j.State = Canceled
	case code == 0:
		j.State = Done
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "fix(jobs): a stopped job that exits 0 is done"
```

---

### Task 2: Count what casks depend on

**Files:**
- Modify: `internal/brew/parse.go`
- Modify: `internal/brew/parse_test.go`
- Modify: `internal/state/index_test.go`
- Modify: `internal/ui/actions_test.go`
- Modify: `internal/ui/installed.go`
- Modify: `internal/ui/installed_test.go`

**Interfaces:**
- **Consumes:** `brew.ParseInfo`, `state.Index` (unchanged).
- **Produces:**
  - Casks in `ParseInfo` get `DirectDeps` from `depends_on.formula` then `depends_on.cask`. Their `Deps` are those plus, transitively, everything those need: a formula's `Deps`, or a cask's own direct dependencies, followed down. Nothing is listed twice.
  - `caskDeps(pkgs []Package)` is unexported.
  - The Installed preview shows the Dependencies and Used-by sections for a cask that needs something or is needed; casks with neither still skip them.

`depends_on` also holds `macos` requirements, which are ignored. `state.Index` needs no change: with casks' `Deps` filled in, `UsedBy`, `Orphans` and `IsLeaf` see them. The new state tests cover that, along with an orphan case that only resolves over several rounds (a Plan 2 follow-up).

- [ ] **Step 1: Edit `internal/brew/parse_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	}
}

func TestParseInfoBadJSON(t *testing.T) {
	if _, err := ParseInfo([]byte("Error: not json")); err == nil {
		t.Fatal("want error for non-JSON output")
```

   with:

```go
	}
}

func TestParseInfoCaskDependencies(t *testing.T) {
	// b needs the cask a, which needs the formula libusb, which needs
	// libffi. brew lists only what each cask names itself.
	in := `{"formulae": [
	  {"name": "libusb", "full_name": "libusb", "installed": [{"version": "1.0", "runtime_dependencies": [{"full_name": "libffi", "declared_directly": true}]}]},
	  {"name": "libffi", "full_name": "libffi", "installed": [{"version": "3.5"}]}
	], "casks": [
	  {"token": "a", "full_token": "a", "depends_on": {"formula": ["libusb"], "macos": {">=": ["14"]}}},
	  {"token": "b", "full_token": "b", "depends_on": {"cask": ["a"]}},
	  {"token": "c", "full_token": "c", "depends_on": {}}
	]}`
	pkgs, err := ParseInfo([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		deps, direct []string
	}{
		{"a", []string{"libusb", "libffi"}, []string{"libusb"}},
		{"b", []string{"a", "libusb", "libffi"}, []string{"a"}},
		{"c", nil, nil},
	}
	for _, tt := range tests {
		p := byName(pkgs, tt.name)
		if !reflect.DeepEqual(p.Deps, tt.deps) || !reflect.DeepEqual(p.DirectDeps, tt.direct) {
			t.Errorf("%s: deps %v, direct %v; want %v, %v", tt.name, p.Deps, p.DirectDeps, tt.deps, tt.direct)
		}
	}
}

func TestParseInfoBadJSON(t *testing.T) {
	if _, err := ParseInfo([]byte("Error: not json")); err == nil {
		t.Fatal("want error for non-JSON output")
```

- [ ] **Step 2: Edit `internal/state/index_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Errorf("changing the result changed the index: %v", got)
	}
}
```

   with:

```go
		t.Errorf("changing the result changed the index: %v", got)
	}
}

func TestOrphansTakesSeveralRounds(t *testing.T) {
	// a needs b and c; b needs c. x, installed by hand, still needs b, so
	// b stays, and then c stays too because b needs it: c only turns out
	// to be needed once b has been dropped.
	pkgs := []brew.Package{
		{FullName: "a", OnRequest: true, Deps: []string{"b", "c"}},
		{FullName: "b", Deps: []string{"c"}},
		{FullName: "c"},
		{FullName: "x", OnRequest: true, Deps: []string{"b"}},
	}
	for range 20 { // map order varies: c may be looked at before or after b
		if got := NewIndex(pkgs).Orphans("a"); len(got) != 0 {
			t.Fatalf("x still needs b, and b needs c; got %v", got)
		}
	}
}

func TestCaskDependencies(t *testing.T) {
	// The cask a needs libusb, which needs libffi; both came in for it.
	pkgs := []brew.Package{
		{Name: "a", FullName: "a", Kind: brew.KindCask, OnRequest: true, Deps: []string{"libusb", "libffi"}, DirectDeps: []string{"libusb"}},
		{Name: "libusb", FullName: "libusb", Kind: brew.KindFormula, Deps: []string{"libffi"}, DirectDeps: []string{"libffi"}},
		{Name: "libffi", FullName: "libffi", Kind: brew.KindFormula},
	}
	ix := NewIndex(pkgs)
	if got := ix.UsedBy("libusb"); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("UsedBy(libusb) = %v: the cask needs it", got)
	}
	if got := ix.Orphans("a"); !reflect.DeepEqual(got, []string{"libffi", "libusb"}) {
		t.Errorf("Orphans(a) = %v", got)
	}
	if ix.IsLeaf(pkgs[1]) {
		t.Error("libusb isn't top-level while a cask needs it")
	}
}
```

- [ ] **Step 3: Edit `internal/ui/actions_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	}
}

func TestUninstallRefusesWhatOthersNeed(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
```

   with:

```go
	}
}

func TestUninstallCask(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "f", "f", "g", "x") // apps: docker-desktop
	if a.confirm == nil || a.confirm.title != "Uninstall docker-desktop 4.39.0,184744?" {
		t.Fatalf("x should ask: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"uninstall", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUninstallRefusesWhatACaskNeeds(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	for i, p := range a.snap.Packages {
		if p.Name == "docker-desktop" {
			a.snap.Packages[i].Deps, a.snap.Packages[i].DirectDeps = []string{"fd"}, []string{"fd"}
		}
	}
	a.sync()
	keys(t, a, "2", "f", "j", "x") // all: fd
	if a.confirm == nil || a.confirm.title != "Can't uninstall fd" {
		t.Fatalf("want an explanation, got %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Required by docker-desktop.") {
		t.Errorf("should say the cask needs it:\n%s", out)
	}
}

func TestUninstallRefusesWhatOthersNeed(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
```

- [ ] **Step 4: Edit `internal/ui/installed_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	}
}

func TestInstalledEmptyFilter(t *testing.T) {
	it := newInstalledTab(testStyles)
	it.resize(100, 12)
```

   with:

```go
	}
}

func TestInstalledPreviewForCaskWithDependencies(t *testing.T) {
	snap := fixtureSnapshot(t)
	for i, p := range snap.Packages {
		if p.Name == "docker-desktop" {
			snap.Packages[i].Deps, snap.Packages[i].DirectDeps = []string{"fd"}, []string{"fd"}
		}
	}
	it := newInstalledTab(testStyles)
	it.resize(140, 30)
	it.setData(snap.Packages, state.NewIndex(snap.Packages))
	out := stripANSI(it.view(nil)) // docker-desktop is first
	if !strings.Contains(out, "Dependencies") || !strings.Contains(out, "└─ fd") {
		t.Errorf("a cask that needs something should show it:\n%s", out)
	}
}

func TestInstalledEmptyFilter(t *testing.T) {
	it := newInstalledTab(testStyles)
	it.resize(100, 12)
```

- [ ] **Step 5: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    --- FAIL: TestParseInfoCaskDependencies (0.00s)
    --- FAIL: TestInstalledPreviewForCaskWithDependencies (0.00s)
```

- [ ] **Step 6: Edit `internal/brew/parse.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)
```

   with:

```go
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)
```

2. Replace:

```go
		Outdated      bool     `json:"outdated"`
		Deprecated    bool     `json:"deprecated"`
		Caveats       *string  `json:"caveats"`
	} `json:"casks"`
}
```

   with:

```go
		Outdated      bool     `json:"outdated"`
		Deprecated    bool     `json:"deprecated"`
		Caveats       *string  `json:"caveats"`
		DependsOn     struct {
			Formula []string `json:"formula"`
			Cask    []string `json:"cask"`
		} `json:"depends_on"`
	} `json:"casks"`
}
```

3. Replace:

```go
		if c.InstalledTime != nil {
			p.InstalledAt = time.Unix(*c.InstalledTime, 0)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

type outdatedJSON struct {
	Formulae []outdatedItem `json:"formulae"`
	Casks    []outdatedItem `json:"casks"`
```

   with:

```go
		if c.InstalledTime != nil {
			p.InstalledAt = time.Unix(*c.InstalledTime, 0)
		}
		p.DirectDeps = append(slices.Clone(c.DependsOn.Formula), c.DependsOn.Cask...)
		pkgs = append(pkgs, p)
	}
	caskDeps(pkgs)
	return pkgs, nil
}

// caskDeps fills in each cask's Deps: brew lists only what a cask names
// itself, but it needs everything those need too, as a formula's
// runtime_dependencies already say.
func caskDeps(pkgs []Package) {
	byName := map[string]*Package{}
	for i := range pkgs {
		byName[pkgs[i].FullName] = &pkgs[i]
	}
	for i := range pkgs {
		p := &pkgs[i]
		if p.Kind != KindCask {
			continue
		}
		seen := map[string]bool{p.FullName: true}
		var walk func(names []string)
		walk = func(names []string) {
			for _, d := range names {
				if seen[d] {
					continue
				}
				seen[d] = true
				p.Deps = append(p.Deps, d)
				if dp, ok := byName[d]; ok {
					if dp.Kind == KindCask {
						walk(dp.DirectDeps)
					} else {
						walk(dp.Deps)
					}
				}
			}
		}
		walk(p.DirectDeps)
	}
}

type outdatedJSON struct {
	Formulae []outdatedItem `json:"formulae"`
	Casks    []outdatedItem `json:"casks"`
```

- [ ] **Step 7: Edit `internal/ui/installed.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	t.apply()
}

// shows reports whether view f lists p. Apps count as top-level: nothing
// depends on a cask, so they are always things you chose to install.
func (t *installedTab) shows(f installedFilter, p brew.Package) bool {
	switch f {
	case filterTop:
```

   with:

```go
	t.apply()
}

// shows reports whether view f lists p. Apps count as top-level: brew
// doesn't record whether a cask came in for another one, so they are all
// taken as things you chose to install.
func (t *installedTab) shows(f installedFilter, p brew.Package) bool {
	switch f {
	case filterTop:
```

2. Replace:

```go
	b.WriteString(kv(st, "type", p.Kind.String(), "tap", p.Tap, "installed", how,
		"latest", latest, "pinned", pinned, "deprecated", deprecated, "homepage", p.Homepage))

	if p.Kind == brew.KindFormula {
		b.WriteString("\n" + st.Accent.Render("Dependencies") + "\n")
		if len(p.DirectDeps) == 0 {
			b.WriteString(st.Muted.Render("none") + "\n")
```

   with:

```go
	b.WriteString(kv(st, "type", p.Kind.String(), "tap", p.Tap, "installed", how,
		"latest", latest, "pinned", pinned, "deprecated", deprecated, "homepage", p.Homepage))

	if p.Kind == brew.KindFormula || len(p.DirectDeps) > 0 || len(t.ix.UsedBy(p.FullName)) > 0 {
		b.WriteString("\n" + st.Accent.Render("Dependencies") + "\n")
		if len(p.DirectDeps) == 0 {
			b.WriteString(st.Muted.Render("none") + "\n")
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "fix(brew): count what casks depend on"
```

---

### Task 3: Read services with their command and logs

**Files:**
- Modify: `internal/brew/client.go`
- Modify: `internal/brew/client_test.go`
- Modify: `internal/brew/live_test.go`
- Modify: `internal/brew/parse.go`
- Modify: `internal/brew/parse_test.go`
- Modify: `internal/brew/testdata/services.json`
- Modify: `internal/brew/types.go`
- Modify: `internal/ui/list_test.go`

**Interfaces:**
- **Consumes:** `brew.Client.query`.
- **Produces:**
  - `brew.Service` gains `PID int` (`json:"pid"`; 0 when not running), `Command string` (`command`), `Log string` (`log_path`) and `ErrorLog string` (`error_log_path`).
  - `(*Client).Services` runs `services info --all --json`; `ParseServices` is unchanged apart from its comment.

The fixture is rewritten in the new command's form, with every field brew prints. Its redis entry is now started with a pid, so one UI test's expected status changes from `none` to `started`.

- [ ] **Step 1: Edit `internal/brew/client_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
}

func TestClientQueryErrorIncludesStderr(t *testing.T) {
	c := NewClient(fakeRunner{err: map[string]error{"services list --json": errors.New("exit status 1")}})
	_, err := c.Services(context.Background())
	var qe *QueryError
	if !errors.As(err, &qe) {
		t.Fatalf("want *QueryError, got %v", err)
	}
	if !strings.Contains(err.Error(), "brew services list --json") || !strings.Contains(err.Error(), "something broke") {
		t.Errorf("error message missing details: %q", err)
	}
}
```

   with:

```go
}

func TestClientQueryErrorIncludesStderr(t *testing.T) {
	c := NewClient(fakeRunner{err: map[string]error{"services info --all --json": errors.New("exit status 1")}})
	_, err := c.Services(context.Background())
	var qe *QueryError
	if !errors.As(err, &qe) {
		t.Fatalf("want *QueryError, got %v", err)
	}
	if !strings.Contains(err.Error(), "brew services info --all --json") || !strings.Contains(err.Error(), "something broke") {
		t.Errorf("error message missing details: %q", err)
	}
}
```

- [ ] **Step 2: Edit `internal/brew/live_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	if _, err := c.Outdated(ctx); err != nil {
		t.Error(err)
	}
	if _, err := c.Services(ctx); err != nil {
		t.Error(err)
	}
	if _, err := c.Taps(ctx); err != nil {
		t.Error(err)
	}
```

   with:

```go
	if _, err := c.Outdated(ctx); err != nil {
		t.Error(err)
	}
	services, err := c.Services(ctx)
	if err != nil {
		t.Error(err)
	}
	for _, s := range services {
		if s.Name == "" || s.Status == "" {
			t.Errorf("incomplete service: %+v", s)
		}
	}
	if _, err := c.Taps(ctx); err != nil {
		t.Error(err)
	}
```

- [ ] **Step 3: Edit `internal/brew/parse_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	if got[1].ExitCode != nil {
		t.Errorf("null exit code should stay nil, got %v", *got[1].ExitCode)
	}
}

func TestParseTaps(t *testing.T) {
```

   with:

```go
	if got[1].ExitCode != nil {
		t.Errorf("null exit code should stay nil, got %v", *got[1].ExitCode)
	}
	redis := got[1]
	if redis.PID != 80476 || redis.Log != "/opt/homebrew/var/log/redis.log" || redis.ErrorLog != "" ||
		redis.Command != "/opt/homebrew/opt/redis/bin/redis-server /opt/homebrew/etc/redis.conf" {
		t.Errorf("redis details parsed wrong: %+v", redis)
	}
	if pg := got[0]; pg.PID != 0 || pg.ErrorLog != "/opt/homebrew/var/log/postgresql@16.log" {
		t.Errorf("a stopped service has no pid: %+v", pg)
	}
}

func TestParseTaps(t *testing.T) {
```

- [ ] **Step 4: Replace the whole of `internal/brew/testdata/services.json` with**

```json
[
  {
    "name": "postgresql@16",
    "service_name": "homebrew.mxcl.postgresql@16",
    "running": false,
    "loaded": true,
    "schedulable": false,
    "pid": null,
    "exit_code": 1,
    "user": "locutus",
    "status": "error",
    "file": "/Users/locutus/Library/LaunchAgents/homebrew.mxcl.postgresql@16.plist",
    "registered": true,
    "loaded_file": "/Users/locutus/Library/LaunchAgents/homebrew.mxcl.postgresql@16.plist",
    "command": "LC_ALL=\"en_US.UTF-8\" /opt/homebrew/opt/postgresql@16/bin/postgres -D /opt/homebrew/var/postgresql@16",
    "working_dir": "/opt/homebrew",
    "root_dir": null,
    "log_path": "/opt/homebrew/var/log/postgresql@16.log",
    "error_log_path": "/opt/homebrew/var/log/postgresql@16.log",
    "interval": null,
    "cron": null
  },
  {
    "name": "redis",
    "service_name": "homebrew.mxcl.redis",
    "running": true,
    "loaded": true,
    "schedulable": false,
    "pid": 80476,
    "exit_code": null,
    "user": "locutus",
    "status": "started",
    "file": "/Users/locutus/Library/LaunchAgents/homebrew.mxcl.redis.plist",
    "registered": true,
    "loaded_file": "/Users/locutus/Library/LaunchAgents/homebrew.mxcl.redis.plist",
    "command": "/opt/homebrew/opt/redis/bin/redis-server /opt/homebrew/etc/redis.conf",
    "working_dir": "/opt/homebrew/var",
    "root_dir": null,
    "log_path": "/opt/homebrew/var/log/redis.log",
    "error_log_path": null,
    "interval": null,
    "cron": null
  }
]
```

- [ ] **Step 5: Edit `internal/ui/list_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	if got := serviceRows(snap.Services)[0]; got[0] != "postgresql@16" || got[1] != "error (exit 1)" {
		t.Errorf("service row = %v", got)
	}
	if got := serviceRows(snap.Services)[1][1]; got != "none" {
		t.Errorf("status without exit code = %q", got)
	}
	if got := tapRows(snap.Taps)[0]; got[1] != "2" || got[2] != "1" || got[3] != "" {
```

   with:

```go
	if got := serviceRows(snap.Services)[0]; got[0] != "postgresql@16" || got[1] != "error (exit 1)" {
		t.Errorf("service row = %v", got)
	}
	if got := serviceRows(snap.Services)[1][1]; got != "started" {
		t.Errorf("status without exit code = %q", got)
	}
	if got := tapRows(snap.Taps)[0]; got[1] != "2" || got[2] != "1" || got[3] != "" {
```

- [ ] **Step 6: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/brew/parse_test.go:133:11: redis.PID undefined (type Service has no field or method PID)
    internal/brew/parse_test.go:133:33: redis.Log undefined (type Service has no field or method Log)
    internal/brew/parse_test.go:133:83: redis.ErrorLog undefined (type Service has no field or method ErrorLog)
```

- [ ] **Step 7: Edit `internal/brew/client.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
}

func (c *Client) Services(ctx context.Context) ([]Service, error) {
	out, err := c.query(ctx, "services", "list", "--json")
	if err != nil {
		return nil, err
	}
```

   with:

```go
}

func (c *Client) Services(ctx context.Context) ([]Service, error) {
	out, err := c.query(ctx, "services", "info", "--all", "--json")
	if err != nil {
		return nil, err
	}
```

- [ ] **Step 8: Edit `internal/brew/parse.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	return out, nil
}

// ParseServices parses `brew services list --json`.
func ParseServices(b []byte) ([]Service, error) {
	var out []Service
	if err := json.Unmarshal(jsonPayload(b), &out); err != nil {
```

   with:

```go
	return out, nil
}

// ParseServices parses `brew services info --all --json`, which says
// everything `services list` does plus each service's command and logs.
func ParseServices(b []byte) ([]Service, error) {
	var out []Service
	if err := json.Unmarshal(jsonPayload(b), &out); err != nil {
```

- [ ] **Step 9: Edit `internal/brew/types.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	Pinned    bool   `json:"pinned"`
}

// Service is one row of `brew services list --json`.
type Service struct {
	Name     string `json:"name"`
	Status   string `json:"status"` // started, stopped, none, error, scheduled, unknown
	User     string `json:"user"`
	File     string `json:"file"`
	ExitCode *int   `json:"exit_code"`
}

// Tap is one row of `brew tap-info --json --installed`.
```

   with:

```go
	Pinned    bool   `json:"pinned"`
}

// Service is one entry of `brew services info --all --json`.
type Service struct {
	Name     string `json:"name"`
	Status   string `json:"status"` // started, stopped, none, error, scheduled, unknown
	User     string `json:"user"`
	File     string `json:"file"`
	ExitCode *int   `json:"exit_code"`
	PID      int    `json:"pid"`            // 0 when it isn't running
	Command  string `json:"command"`        // what launchd runs
	Log      string `json:"log_path"`       // where its output goes, if anywhere
	ErrorLog string `json:"error_log_path"` // where its errors go; often the same file
}

// Tap is one row of `brew tap-info --json --installed`.
```

- [ ] **Step 10: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "feat(brew): read services with their command and logs"
```

---

### Task 4: Preview a service with the end of its log

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/list.go`
- Modify: `internal/ui/list_test.go`
- Create: `internal/ui/services.go`
- Create: `internal/ui/services_test.go`

**Interfaces:**
- **Consumes:** Task 3's `Service` fields; `pane`, `paneInner`, `kv`; `App.note`.
- **Produces:**
  - `listTab.preview func(w, h int) (title, body string)`: when set, the table takes 55% of the width (`listWidth()`) and the preview pane takes the rest.
  - In `services.go`:
    - `logTailBytes = 16 << 10`; `logTail{path, text, err, loaded, stale}`; `logMsg{path, text, err}`.
    - `readLogTail(path string) (string, error)`.
    - `(*App) selectedService() (brew.Service, bool)`, `(*App) wantLog() tea.Cmd`, `(*App) gotLog(logMsg)`, `(*App) servicePreview(w, h int) (string, string)`.
    - `serviceStatus(brew.Service) string`, `tilde(path string) string`.
  - `App.readLog func(path string) (string, error)` (default `readLogTail`; tests swap it) and `App.log logTail`.
  - `serviceKey` now finds the service by name (a Plan 2 follow-up).
  - `serviceCols` is NAME (flexible) and STATUS (16), and `serviceRows` builds two cells.
- **Test helpers:** `fakeLogs` (canned logs, counting reads), `numbered(n)`.

`Update` calls `wantLog` after every message while Services is showing and the jobs view is closed. It reads only when the selected service's log differs from the one held, or when a services result has marked it stale. So moving the cursor reads each log once, and frames don't read at all.

- [ ] **Step 1: Edit `internal/ui/list_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
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
```

   with:

```go
func TestListTabKeepsSelectionAcrossRefresh(t *testing.T) {
	lt := newListTab(testStyles, "Services", serviceCols, firstCell, "none")
	lt.resize(120, 10)
	lt.setRows([]table.Row{{"mysql", ""}, {"postgresql@16", ""}, {"redis", ""}})
	lt.update(press("j"))
	lt.update(press("j")) // redis
	lt.setRows([]table.Row{{"nginx", ""}, {"postgresql@16", ""}, {"redis", ""}, {"valkey", ""}})
	if got := lt.table.SelectedRow()[0]; got != "redis" {
		t.Errorf("selection moved to %q after refresh, want redis", got)
	}
	if lt.cursor() != 2 {
		t.Errorf("cursor() = %d, want 2", lt.cursor())
	}
	lt.setRows([]table.Row{{"nginx", ""}})
	if lt.cursor() != 0 {
		t.Errorf("when the item is gone the cursor stays in range, got %d", lt.cursor())
	}
```

- [ ] **Step 2: Create `internal/ui/services_test.go`**

```go
package ui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

// fakeLogs stands in for the log files, counting the reads.
type fakeLogs struct {
	text  map[string]string
	reads map[string]int
}

func (f *fakeLogs) read(path string) (string, error) {
	if f.reads == nil {
		f.reads = map[string]int{}
	}
	f.reads[path]++
	text, ok := f.text[path]
	if !ok {
		return "", fs.ErrNotExist
	}
	return text, nil
}

// numbered is n log lines: "line 1" to "line n".
func numbered(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	return strings.Join(lines, "\n")
}

func TestReadLogTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	if _, err := readLogTail(path); !os.IsNotExist(err) {
		t.Errorf("a missing log reports that it doesn't exist: %v", err)
	}
	long := strings.Repeat("x", logTailBytes) + "\nfirst whole line\n\x1b[31mred\x1b[0m\tthen a tab\r\n10%\r50%\r100%\n"
	if err := os.WriteFile(path, []byte(long), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readLogTail(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "first whole line\nred    then a tab\n100%"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestServicesPreview(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	logs := &fakeLogs{text: map[string]string{
		"/opt/homebrew/var/log/postgresql@16.log": numbered(100),
		"/opt/homebrew/var/log/redis.log":         "Ready to accept connections",
	}}
	a.readLog = logs.read
	keys(t, a, "4") // postgresql@16
	out := stripANSI(a.render())
	for _, want := range []string{
		"error (exit 1)", "LC_ALL=", "/opt/homebrew/var/log/postgresql@16.log",
		"End of the log", "line 100",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "line 60 ") || strings.Contains(out, "more") {
		t.Errorf("only the end of the log fits, and it isn't cut off:\n%s", out)
	}
	if strings.Contains(out, "errors ") {
		t.Errorf("one file for output and errors is shown once:\n%s", out)
	}

	keys(t, a, "j") // redis
	out = stripANSI(a.render())
	for _, want := range []string{"started · pid 80476", "Ready to accept connections"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if logs.reads["/opt/homebrew/var/log/redis.log"] != 1 || logs.reads["/opt/homebrew/var/log/postgresql@16.log"] != 1 {
		t.Errorf("each log is read once: %v", logs.reads)
	}
}

func TestServicesPreviewRereadsAfterARefresh(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	logs := &fakeLogs{text: map[string]string{"/opt/homebrew/var/log/postgresql@16.log": "old"}}
	a.readLog = logs.read
	keys(t, a, "4")
	logs.text["/opt/homebrew/var/log/postgresql@16.log"] = "new"
	_, cmd := a.Update(resultMsg{q: qServices, services: a.snap.Services})
	settle(t, a, cmd)
	if out := stripANSI(a.render()); !strings.Contains(out, "new") {
		t.Errorf("after services refresh the log is read again:\n%s", out)
	}
}

func TestServicesPreviewWithoutALog(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.snap.Services[1].Log, a.snap.Services[1].ErrorLog = "", "/tmp/redis.err"
	a.sync()
	keys(t, a, "4")
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing logged yet") {
		t.Errorf("a log that doesn't exist yet says so:\n%s", out)
	}
	keys(t, a, "j")
	out := stripANSI(a.render())
	if !strings.Contains(out, "brew knows no log for this service") || !strings.Contains(out, "/tmp/redis.err") {
		t.Errorf("no log path: say so, and show the error log:\n%s", out)
	}
}

func TestServicesPreviewAt80Columns(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	keys(t, a, "4")
	out := stripANSI(a.render())
	if !strings.Contains(out, "postgresql@16  ") || !strings.Contains(out, "error (exit 1)") {
		t.Errorf("names and statuses fit beside the preview:\n%s", out)
	}
}

func TestTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	if got := tilde(home + "/Library/LaunchAgents/x.plist"); got != "~/Library/LaunchAgents/x.plist" {
		t.Errorf("got %q", got)
	}
	if got := tilde("/opt/homebrew/var/log/x.log"); got != "/opt/homebrew/var/log/x.log" {
		t.Errorf("got %q", got)
	}
}

func TestServiceActionsFollowTheName(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	a.readLog = (&fakeLogs{}).read
	keys(t, a, "4", "j") // redis
	// A refresh lists redis first now: the cursor follows it.
	a.snap.Services = []brew.Service{a.snap.Services[1], a.snap.Services[0]}
	a.sync()
	keys(t, a, "R")
	if want := []string{"services", "restart", "redis"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/services_test.go:46:15: undefined: readLogTail
    internal/ui/services_test.go:49:30: undefined: logTailBytes
    internal/ui/services_test.go:53:14: undefined: readLogTail
```

- [ ] **Step 4: Edit `internal/ui/actions.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
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
```

   with:

```go
// serviceKey starts (s), stops (S, after asking) or restarts (R) the
// selected service.
func (a *App) serviceKey(k string) tea.Cmd {
	s, ok := a.selectedService()
	if !ok {
		return nil
	}
	name := s.Name
	verb := map[string]string{"s": "start", "S": "stop", "R": "restart"}[k]
	act := action{title: verb + " " + name, args: []string{"services", verb, name}, refresh: []query{qServices}}
	if verb != "stop" {
```

- [ ] **Step 5: Edit `internal/ui/app.go`** (6 replacements; each old block appears exactly once)

1. Replace:

```go
	takeoverCmd func(args []string) *exec.Cmd // builds full-screen commands
	takingOver  *jobs.Job                     // the full-screen command running now

	tab           int
	width, height int
	tabH          int    // height left for the tab when the drawer is shown
```

   with:

```go
	takeoverCmd func(args []string) *exec.Cmd // builds full-screen commands
	takingOver  *jobs.Job                     // the full-screen command running now

	readLog func(path string) (string, error) // reads the end of a log; tests swap it
	log     logTail                           // the services preview's log

	tab           int
	width, height int
	tabH          int    // height left for the tab when the drawer is shown
```

2. Replace:

```go
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		acts: map[int]action{}, backoff: jobs.Backoff, tickEvery: 100 * time.Millisecond,
		drawer:    newDrawer(st),
		cmd:       newCmdline(st),
		installed: newInstalledTab(st),
```

   with:

```go
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		acts: map[int]action{}, backoff: jobs.Backoff, tickEvery: 100 * time.Millisecond,
		readLog:   readLogTail,
		drawer:    newDrawer(st),
		cmd:       newCmdline(st),
		installed: newInstalledTab(st),
```

3. Replace:

```go
	// The history is read before the app starts, so the jobs view is never
	// waiting for it.
	a.jobs.loaded = true
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
		for _, q := range []query{qInstalled, qOutdated, qServices, qTaps} {
```

   with:

```go
	// The history is read before the app starts, so the jobs view is never
	// waiting for it.
	a.jobs.loaded = true
	a.services.preview = a.servicePreview
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
		for _, q := range []query{qInstalled, qOutdated, qServices, qTaps} {
```

4. Replace:

```go
		a.snap.Outdated = m.outdated
	case qServices:
		a.snap.Services = m.services
	case qTaps:
		a.snap.Taps = m.taps
	case qDoctor:
```

   with:

```go
		a.snap.Outdated = m.outdated
	case qServices:
		a.snap.Services = m.services
		a.log.stale = true // the service may have written more since
	case qTaps:
		a.snap.Taps = m.taps
	case qDoctor:
```

5. Replace:

```go

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := a.update(msg)
	if a.promptPending && a.mode == modeNormal {
		a.promptPending = false
		if a.proc != nil && a.drawer.prompt() {
```

   with:

```go

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := a.update(msg)
	if a.tab == tabServices && !a.jobsOpen {
		cmd = tea.Batch(cmd, a.wantLog())
	}
	if a.promptPending && a.mode == modeNormal {
		a.promptPending = false
		if a.proc != nil && a.drawer.prompt() {
```

6. Replace:

```go
		a.histErr = msg.err
	case takeoverDoneMsg:
		return a.takeoverDone(msg)
	case completionsMsg:
		if msg.err == nil {
			a.comp = msg.comp // without them, only brewery's own commands complete
```

   with:

```go
		a.histErr = msg.err
	case takeoverDoneMsg:
		return a.takeoverDone(msg)
	case logMsg:
		a.gotLog(msg)
	case completionsMsg:
		if msg.err == nil {
			a.comp = msg.comp // without them, only brewery's own commands complete
```

- [ ] **Step 6: Edit `internal/ui/list.go`** (6 replacements; each old block appears exactly once)

1. Replace:

```go

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/theme"
)

// listTab is a full-width table used by the Outdated, Services and Taps tabs.
type listTab struct {
	st     theme.Styles
	name   string
```

   with:

```go

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
	"brewery/internal/theme"
)

// listTab is a table used by the Outdated, Services and Taps tabs, full
// width or beside a preview.
type listTab struct {
	st     theme.Styles
	name   string
```

2. Replace:

```go
	key    func(table.Row) string // identifies a row across refreshes
	table  table.Model
	w, h   int
}

func newListTab(st theme.Styles, name string, cols []col, key func(table.Row) string, empty string) listTab {
```

   with:

```go
	key    func(table.Row) string // identifies a row across refreshes
	table  table.Model
	w, h   int

	// preview, if set, fills a pane beside the table with what it returns
	// for a w×h pane.
	preview func(w, h int) (title, body string)
}

func newListTab(st theme.Styles, name string, cols []col, key func(table.Row) string, empty string) listTab {
```

3. Replace:

```go
	return cmd
}

func (t *listTab) resize(w, h int) {
	t.w, t.h = w, h
	iw, ih := paneInner(w, h)
	sizeTable(&t.table, t.cols, iw, ih)
}

// view sizes the table each time because a failed refresh shares the pane
// with a banner above it.
func (t *listTab) view(err error) string {
	w, h := t.w, t.h
	iw, ih := paneInner(w, h)
	switch {
	case err != nil && t.n == 0:
		return pane(t.st, t.name, errorBody(t.st, err), w, h, true)
```

   with:

```go
	return cmd
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

4. Replace:

```go
		ih -= bannerHeight
	}
	t.table.SetHeight(ih)
	return pane(t.st, fmt.Sprintf("%s · %d", t.name, t.n), body+t.table.View(), w, h, true)
}

func outdatedRows(items []brew.Outdated) []table.Row {
```

   with:

```go
		ih -= bannerHeight
	}
	t.table.SetHeight(ih)
	list := pane(t.st, fmt.Sprintf("%s · %d", t.name, t.n), body+t.table.View(), listW, h, true)
	if t.preview == nil {
		return list
	}
	title, preview := t.preview(w-listW, h)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, pane(t.st, title, preview, w-listW, h, false))
}

func outdatedRows(items []brew.Outdated) []table.Row {
```

5. Replace:

```go
func serviceRows(items []brew.Service) []table.Row {
	rows := make([]table.Row, len(items))
	for i, s := range items {
		status := s.Status
		if s.ExitCode != nil && *s.ExitCode != 0 {
			status += " (exit " + strconv.Itoa(*s.ExitCode) + ")"
		}
		rows[i] = table.Row{s.Name, status, s.User, s.File}
	}
	return rows
}
```

   with:

```go
func serviceRows(items []brew.Service) []table.Row {
	rows := make([]table.Row, len(items))
	for i, s := range items {
		rows[i] = table.Row{s.Name, serviceStatus(s)}
	}
	return rows
}
```

6. Replace:

```go

var (
	outdatedCols = []col{{"NAME", 0}, {"INSTALLED", 18}, {"LATEST", 18}, {"TYPE", 7}, {"PINNED", 6}}
	serviceCols  = []col{{"NAME", 20}, {"STATUS", 16}, {"USER", 10}, {"FILE", 0}}
	tapCols      = []col{{"NAME", 24}, {"FORMULAE", 8}, {"CASKS", 6}, {"OFFICIAL", 8}, {"REMOTE", 0}}
)
```

   with:

```go

var (
	outdatedCols = []col{{"NAME", 0}, {"INSTALLED", 18}, {"LATEST", 18}, {"TYPE", 7}, {"PINNED", 6}}
	serviceCols  = []col{{"NAME", 0}, {"STATUS", 16}}
	tapCols      = []col{{"NAME", 24}, {"FORMULAE", 8}, {"CASKS", 6}, {"OFFICIAL", 8}, {"REMOTE", 0}}
)
```

- [ ] **Step 7: Create `internal/ui/services.go`**

```go
package ui

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
)

// logTailBytes is how much of the end of a log the services preview reads.
const logTailBytes = 16 << 10

// logTail is the end of the log the services preview shows.
type logTail struct {
	path   string // the log shown, or being read
	text   string
	err    error
	loaded bool // the read of path has come back
	stale  bool // services were refreshed: read it again
}

// logMsg carries the end of a log file back to the App.
type logMsg struct {
	path, text string
	err        error
}

// readLogTail returns the last lines of the file at path as plain text: a
// log can hold colors and progress redraws that would garble the pane.
func readLogTail(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	off := max(info.Size()-logTailBytes, 0)
	b := make([]byte, info.Size()-off)
	if _, err := f.ReadAt(b, off); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	s := strings.ToValidUTF8(string(b), "�")
	if off > 0 {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:] // the first line was cut
		}
	}
	lines := strings.Split(strings.TrimRight(ansi.Strip(s), "\r\n"), "\n")
	for i, l := range lines {
		l = strings.TrimSuffix(l, "\r")
		if j := strings.LastIndexByte(l, '\r'); j >= 0 {
			l = l[j+1:] // a redrawn line: keep what was drawn last
		}
		lines[i] = strings.ReplaceAll(l, "\t", "    ")
	}
	return strings.Join(lines, "\n"), nil
}

// selectedService is the service under the cursor, found by name.
func (a *App) selectedService() (brew.Service, bool) {
	row := a.services.table.SelectedRow()
	if a.services.n == 0 || row == nil {
		return brew.Service{}, false
	}
	for _, s := range a.snap.Services {
		if s.Name == row[0] {
			return s, true
		}
	}
	return brew.Service{}, false
}

// wantLog reads the selected service's log if the preview doesn't have it
// yet, or has an older copy from before the last refresh.
func (a *App) wantLog() tea.Cmd {
	s, ok := a.selectedService()
	if !ok || s.Log == "" || (s.Log == a.log.path && !a.log.stale) {
		return nil
	}
	if s.Log != a.log.path {
		a.log = logTail{path: s.Log}
	}
	a.log.stale = false
	read, path := a.readLog, s.Log
	return func() tea.Msg {
		text, err := read(path)
		return logMsg{path, text, err}
	}
}

// gotLog keeps a log read, unless the selection has moved to another log.
func (a *App) gotLog(m logMsg) {
	if m.path == a.log.path {
		a.log.text, a.log.err, a.log.loaded = m.text, m.err, true
	}
}

// tilde shortens a path in the home directory to start with ~.
func tilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if rest, ok := strings.CutPrefix(path, home+"/"); ok {
		return "~/" + rest
	}
	return path
}

// serviceStatus is a service's status, with its exit code if it failed.
func serviceStatus(s brew.Service) string {
	if s.ExitCode != nil && *s.ExitCode != 0 {
		return fmt.Sprintf("%s (exit %d)", s.Status, *s.ExitCode)
	}
	return s.Status
}

// servicePreview shows the selected service's details over as much of the
// end of its log as fits a w×h pane.
func (a *App) servicePreview(w, h int) (title, body string) {
	st := a.st
	s, ok := a.selectedService()
	if !ok {
		return "Preview", ""
	}
	style := st.Warn
	switch s.Status {
	case "started", "scheduled":
		style = st.OK
	case "error":
		style = st.Err
	case "none", "stopped":
		style = st.Muted
	}
	head := style.Render(serviceStatus(s))
	if s.PID > 0 {
		head += st.Muted.Render(fmt.Sprintf(" · pid %d", s.PID))
	}
	errLog := ""
	if s.ErrorLog != s.Log {
		errLog = s.ErrorLog
	}
	top := head + "\n" + kv(st, "command", s.Command, "user", s.User, "file", tilde(s.File), "log", tilde(s.Log), "errors", tilde(errLog)) +
		"\n" + st.Accent.Render("End of the log") + "\n"

	var lines []string
	switch {
	case s.Log == "":
		lines = []string{st.Muted.Render("brew knows no log for this service")}
	case a.log.path != s.Log || !a.log.loaded:
		lines = []string{st.Muted.Render("loading…")}
	case errors.Is(a.log.err, fs.ErrNotExist):
		lines = []string{st.Muted.Render("nothing logged yet")}
	case a.log.err != nil:
		lines = []string{st.Err.Render(a.log.err.Error())}
	case a.log.text == "":
		lines = []string{st.Muted.Render("the log is empty")}
	default:
		lines = strings.Split(a.log.text, "\n")
	}
	_, ih := paneInner(w, h)
	room := max(ih-strings.Count(top, "\n"), 1)
	lines = lines[max(len(lines)-room, 0):] // the newest lines, which pane would cut
	return s.Name, top + strings.Join(lines, "\n")
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(ui): preview a service with the end of its log"
```

---

### Task 5: l follows a service's log full screen

**Files:**
- Modify: `cmd/brewery/main.go`
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/services.go`
- Modify: `internal/ui/services_test.go`

**Interfaces:**
- **Consumes:** Task 4's `selectedService`, `logTail.stale`; `App.note`, `App.queue`.
- **Produces:**
  - `(*App) UsePager(cmd func(files []string) *exec.Cmd)` and `App.pagerCmd`.
  - `(*App) openLog() tea.Cmd`, `pagerDoneMsg{err}`, `(*App) pagerDone(pagerDoneMsg)`.
  - `l` on Services; the Services hint gains `l log`.
- **`main.go`:** `exec.Command("less", "+F", "--", files...)`.
- **Test helpers:** `fakePager`.

Like a takeover, the viewer goes through `tea.ExecProcess`, waits while a job runs, and gets the terminal's own input and output. Unlike one, it isn't recorded in the history and refreshes nothing but the preview's log.

- [ ] **Step 1: Edit `internal/ui/services_test.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
package ui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
```

   with:

```go
package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
```

2. Replace:

```go
	}
}

func TestServiceActionsFollowTheName(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
```

   with:

```go
	}
}

// fakePager records the files handed to the log viewer.
type fakePager struct{ calls [][]string }

func (f *fakePager) cmd(files []string) *exec.Cmd {
	f.calls = append(f.calls, files)
	return exec.Command("true")
}

func TestLogKeyOpensTheLogs(t *testing.T) {
	dir := t.TempDir()
	out, errs := filepath.Join(dir, "out.log"), filepath.Join(dir, "err.log")
	for _, f := range []string{out, errs} {
		if err := os.WriteFile(f, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	pg := &fakePager{}
	a.UsePager(pg.cmd)
	a.snap.Services[0].Log, a.snap.Services[0].ErrorLog = out, errs // postgresql@16
	a.snap.Services[1].Log, a.snap.Services[1].ErrorLog = out, out  // redis
	a.sync()
	keys(t, a, "4", "l", "j", "l")
	if want := [][]string{{out, errs}, {out}}; !reflect.DeepEqual(pg.calls, want) {
		t.Errorf("opened %v, want %v", pg.calls, want)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "l log") {
		t.Errorf("the hint should name l: %q", bar)
	}
}

func TestLogKeyExplainsMissingLogs(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	pg := &fakePager{}
	a.UsePager(pg.cmd)
	a.snap.Services[0].Log = filepath.Join(t.TempDir(), "none.log")
	a.snap.Services[0].ErrorLog = ""
	a.snap.Services[1].Log, a.snap.Services[1].ErrorLog = "", ""
	a.sync()
	keys(t, a, "4", "l")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "postgresql@16 hasn't logged anything yet") {
		t.Errorf("status bar %q", bar)
	}
	keys(t, a, "j", "l")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "brew knows no log for redis") {
		t.Errorf("status bar %q", bar)
	}
	if len(pg.calls) != 0 {
		t.Errorf("nothing to show, so no viewer: %v", pg.calls)
	}
}

func TestLogKeyWaitsForTheRunningJob(t *testing.T) {
	log := filepath.Join(t.TempDir(), "x.log")
	if err := os.WriteFile(log, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := newFakeProc(0)
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	a.readLog = (&fakeLogs{}).read
	pg := &fakePager{}
	a.UsePager(pg.cmd)
	a.snap.Services[0].Log = log
	a.sync()
	keys(t, a, "U", "4", "l")
	if len(pg.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "wait for update to finish") {
		t.Errorf("the viewer must wait for the job: %v %q", pg.calls, stripANSI(a.statusBar()))
	}
}

func TestPagerDoneRereadsTheLog(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	logs := &fakeLogs{text: map[string]string{"/opt/homebrew/var/log/postgresql@16.log": "x"}}
	a.readLog = logs.read
	keys(t, a, "4")
	_, cmd := a.Update(pagerDoneMsg{err: errors.New("exec: \"less\": executable file not found in $PATH")})
	settle(t, a, cmd)
	if logs.reads["/opt/homebrew/var/log/postgresql@16.log"] != 2 {
		t.Errorf("reads %v: the log is read again after viewing it", logs.reads)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "couldn't show the log") {
		t.Errorf("a failed viewer says so: %q", bar)
	}
}

func TestServiceActionsFollowTheName(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/services_test.go:173:4: a.UsePager undefined (type *App has no field or method UsePager)
    internal/ui/services_test.go:190:4: a.UsePager undefined (type *App has no field or method UsePager)
    internal/ui/services_test.go:217:4: a.UsePager undefined (type *App has no field or method UsePager)
```

- [ ] **Step 3: Edit `cmd/brewery/main.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	app.UseHistory(hist, recs)
	app.UseCompletions(client.Completions)
	app.UseTakeover(func(args []string) *exec.Cmd { return exec.Command(bin, args...) })

	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
```

   with:

```go
	app.UseHistory(hist, recs)
	app.UseCompletions(client.Completions)
	app.UseTakeover(func(args []string) *exec.Cmd { return exec.Command(bin, args...) })
	// less +F follows the log as it grows; ctrl+c stops following, q returns.
	app.UsePager(func(files []string) *exec.Cmd {
		return exec.Command("less", append([]string{"+F", "--"}, files...)...)
	})

	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
```

- [ ] **Step 4: Edit `internal/ui/actions.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
		return a.askUpgradeAll(), true
	case a.tab == tabServices && (k == "s" || k == "S" || k == "R"):
		return a.serviceKey(k), true
	}
	return nil, false
}
```

   with:

```go
		return a.askUpgradeAll(), true
	case a.tab == tabServices && (k == "s" || k == "S" || k == "R"):
		return a.serviceKey(k), true
	case a.tab == tabServices && k == "l":
		return a.openLog(), true
	}
	return nil, false
}
```

2. Replace:

```go
	case tabOutdated:
		return "u upgrade · A upgrade all"
	case tabServices:
		return "s start · S stop · R restart"
	}
	return "1-5 tabs · r refresh · U update"
}
```

   with:

```go
	case tabOutdated:
		return "u upgrade · A upgrade all"
	case tabServices:
		return "s start · S stop · R restart · l log"
	}
	return "1-5 tabs · r refresh · U update"
}
```

- [ ] **Step 5: Edit `internal/ui/app.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	takeoverCmd func(args []string) *exec.Cmd // builds full-screen commands
	takingOver  *jobs.Job                     // the full-screen command running now

	readLog func(path string) (string, error) // reads the end of a log; tests swap it
	log     logTail                           // the services preview's log

	tab           int
	width, height int
```

   with:

```go
	takeoverCmd func(args []string) *exec.Cmd // builds full-screen commands
	takingOver  *jobs.Job                     // the full-screen command running now

	readLog  func(path string) (string, error) // reads the end of a log; tests swap it
	log      logTail                           // the services preview's log
	pagerCmd func(files []string) *exec.Cmd    // builds the full-screen log viewer

	tab           int
	width, height int
```

2. Replace:

```go
		return a.takeoverDone(msg)
	case logMsg:
		a.gotLog(msg)
	case completionsMsg:
		if msg.err == nil {
			a.comp = msg.comp // without them, only brewery's own commands complete
```

   with:

```go
		return a.takeoverDone(msg)
	case logMsg:
		a.gotLog(msg)
	case pagerDoneMsg:
		a.pagerDone(msg)
	case completionsMsg:
		if msg.err == nil {
			a.comp = msg.comp // without them, only brewery's own commands complete
```

- [ ] **Step 6: Edit `internal/ui/services.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	"io"
	"io/fs"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
```

   with:

```go
	"io"
	"io/fs"
	"os"
	"os/exec"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
```

2. Replace:

```go
	return path
}

// serviceStatus is a service's status, with its exit code if it failed.
func serviceStatus(s brew.Service) string {
	if s.ExitCode != nil && *s.ExitCode != 0 {
```

   with:

```go
	return path
}

// pagerDoneMsg reports that the log viewer has exited.
type pagerDoneMsg struct{ err error }

// UsePager sets how the app builds the command that shows log files full
// screen, following what gets added. The UI never starts programs itself.
func (a *App) UsePager(cmd func(files []string) *exec.Cmd) {
	a.pagerCmd = cmd
}

// openLog shows the selected service's log full screen, and its error log
// when that is another file. Like a takeover, it waits for a running job.
func (a *App) openLog() tea.Cmd {
	s, ok := a.selectedService()
	if !ok {
		return nil
	}
	if a.pagerCmd == nil {
		return a.note("the log viewer isn't available")
	}
	var files []string
	for _, f := range []string{s.Log, s.ErrorLog} {
		if f != "" && !slices.Contains(files, f) {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return a.note("brew knows no log for " + s.Name)
	}
	files = slices.DeleteFunc(files, func(f string) bool {
		_, err := os.Stat(f)
		return err != nil
	})
	if len(files) == 0 {
		return a.note(s.Name + " hasn't logged anything yet")
	}
	if cur := a.queue.Current(); cur != nil {
		return a.note("wait for " + cur.Title + " to finish: the log takes over the terminal")
	}
	return tea.ExecProcess(a.pagerCmd(files), func(err error) tea.Msg { return pagerDoneMsg{err} })
}

// pagerDone reads the log again, since more may have been written while it
// was open, and says if the viewer failed.
func (a *App) pagerDone(m pagerDoneMsg) {
	a.log.stale = true
	if m.err != nil {
		a.flash = a.st.Err.Render("✗ couldn't show the log: " + m.err.Error())
	}
}

// serviceStatus is a service's status, with its exit code if it failed.
func serviceStatus(s brew.Service) string {
	if s.ExitCode != nil && *s.ExitCode != 0 {
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(ui): l follows a service's log full screen"
```

---

### Task 6: Measure what cleanup would free after each refresh

**Files:**
- Modify: `internal/brew/client.go`
- Modify: `internal/brew/client_test.go`
- Modify: `internal/brew/live_test.go`
- Modify: `internal/brew/parse.go`
- Modify: `internal/brew/parse_test.go`
- Create: `internal/brew/testdata/cleanup.txt`
- Modify: `internal/brew/types.go`
- Modify: `internal/state/snapshot.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`
- Modify: `internal/ui/helpers_test.go`
- Modify: `internal/ui/source.go`

**Interfaces:**
- **Consumes:** `brew.Runner`, `QueryError`; the App's query machinery (`request`, `handleResult`, `doctorAfter`).
- **Produces:**
  - In `brew`:
    - `CleanupEstimate{Items int; Size string}`, `Cleanup{Standard, All CleanupEstimate}` (JSON tags for the cache).
    - `ParseCleanup(out string) CleanupEstimate`.
    - `(*Client) Cleanup(ctx) (Cleanup, error)`: both dry-runs in parallel, each under the client's timeout; stdout and stderr are read together. If either fails, the error names its command.
  - In `ui`:
    - `Source.Cleanup`, `qCleanup`, `queryNames` gains `"cleanup"`, `resultMsg.cleanup`.
    - The full refresh's second stage requests `qDoctor` and `qCleanup` together.
  - In `state`: `Snapshot.Cleanup`, `Snapshot.CleanupRan`, `snapshotVersion = 2`.
- **Test helpers:** `fakeSource.cleanupErr`; `fixtureSnapshot` includes a cleanup (standard 1 item / 3.9KB, whole cache from `cleanup.txt`: 4 items / 86.7MB); `countingSource` counts `cleanup`.

Version 2 is for Task 3's service fields as much as for this task's.

- [ ] **Step 1: Edit `internal/brew/client_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	}
}

func TestClientQueryErrorIncludesStderr(t *testing.T) {
	c := NewClient(fakeRunner{err: map[string]error{"services info --all --json": errors.New("exit status 1")}})
	_, err := c.Services(context.Background())
```

   with:

```go
	}
}

func TestClientCleanup(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{
		"cleanup --dry-run":             []byte("Would remove: /x (3.9KB)\n==> This operation would free approximately 3.9KB of disk space.\n"),
		"cleanup --dry-run --prune=all": fixture(t, "cleanup.txt"),
	}})
	got, err := c.Cleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := Cleanup{Standard: CleanupEstimate{1, "3.9KB"}, All: CleanupEstimate{4, "86.7MB"}}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	c = NewClient(fakeRunner{err: map[string]error{"cleanup --dry-run --prune=all": errors.New("exit status 1")}})
	if _, err := c.Cleanup(context.Background()); err == nil || !strings.Contains(err.Error(), "brew cleanup --dry-run --prune=all") {
		t.Errorf("a failed dry-run is an error naming it: %v", err)
	}
}

func TestClientQueryErrorIncludesStderr(t *testing.T) {
	c := NewClient(fakeRunner{err: map[string]error{"services info --all --json": errors.New("exit status 1")}})
	_, err := c.Services(context.Background())
```

- [ ] **Step 2: Edit `internal/brew/live_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	if _, err := c.Doctor(ctx); err != nil {
		t.Error(err)
	}
}

// Runs a read-only brew command through the pseudo-terminal runner.
```

   with:

```go
	if _, err := c.Doctor(ctx); err != nil {
		t.Error(err)
	}
	if _, err := c.Cleanup(ctx); err != nil {
		t.Error(err)
	}
}

// Runs a read-only brew command through the pseudo-terminal runner.
```

- [ ] **Step 3: Edit `internal/brew/parse_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	}
}

func TestParseTaps(t *testing.T) {
	got, err := ParseTaps(fixture(t, "taps.json"))
	if err != nil {
```

   with:

```go
	}
}

func TestParseCleanup(t *testing.T) {
	if got, want := ParseCleanup(string(fixture(t, "cleanup.txt"))), (CleanupEstimate{Items: 4, Size: "86.7MB"}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := ParseCleanup(""); got != (CleanupEstimate{}) {
		t.Errorf("nothing to clean up: got %+v", got)
	}
}

func TestParseTaps(t *testing.T) {
	got, err := ParseTaps(fixture(t, "taps.json"))
	if err != nil {
```

- [ ] **Step 4: Create `internal/brew/testdata/cleanup.txt`**

```text
Would remove: /Users/locutus/Library/Caches/Homebrew/aom--3.15.2 (4.4MB)
Would remove: /Users/locutus/Library/Caches/Homebrew/aom_bottle_manifest--3.15.2 (19.8KB)
Would remove: /Users/locutus/Library/Caches/Homebrew/git--2.56.0_1 (24.2MB)
Would remove: /opt/homebrew/Cellar/git/2.50.0 (1,523 files, 58.1MB)
==> This operation would free approximately 86.7MB of disk space.
```

- [ ] **Step 5: Edit `internal/ui/app_test.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
	"brewery/internal/state"
)

// fakeSource serves the fixture snapshot. servicesErr and doctorErr make
// one query fail.
type fakeSource struct {
	snap        state.Snapshot
	servicesErr error
	doctorErr   error
}

func newFakeSource(t *testing.T) *fakeSource { return &fakeSource{snap: fixtureSnapshot(t)} }
```

   with:

```go
	"brewery/internal/state"
)

// fakeSource serves the fixture snapshot. servicesErr, doctorErr and
// cleanupErr make one query fail.
type fakeSource struct {
	snap        state.Snapshot
	servicesErr error
	doctorErr   error
	cleanupErr  error
}

func newFakeSource(t *testing.T) *fakeSource { return &fakeSource{snap: fixtureSnapshot(t)} }
```

2. Replace:

```go
	}
	return f.snap.Doctor, nil
}
func (f *fakeSource) Services(context.Context) ([]brew.Service, error) {
	if f.servicesErr != nil {
		return nil, f.servicesErr
```

   with:

```go
	}
	return f.snap.Doctor, nil
}
func (f *fakeSource) Cleanup(context.Context) (brew.Cleanup, error) {
	if f.cleanupErr != nil {
		return brew.Cleanup{}, f.cleanupErr
	}
	return f.snap.Cleanup, nil
}
func (f *fakeSource) Services(context.Context) ([]brew.Service, error) {
	if f.servicesErr != nil {
		return nil, f.servicesErr
```

3. Replace:

```go
	if !a.snap.DoctorRan || len(a.snap.Doctor) != 2 {
		t.Errorf("doctor not applied: ran=%v warnings=%d", a.snap.DoctorRan, len(a.snap.Doctor))
	}
	saved, err := state.Load(a.cachePath)
	if err != nil {
		t.Fatalf("cache not written: %v", err)
```

   with:

```go
	if !a.snap.DoctorRan || len(a.snap.Doctor) != 2 {
		t.Errorf("doctor not applied: ran=%v warnings=%d", a.snap.DoctorRan, len(a.snap.Doctor))
	}
	if !a.snap.CleanupRan || a.snap.Cleanup.All.Size != "86.7MB" {
		t.Errorf("cleanup's dry-runs not applied: ran=%v %+v", a.snap.CleanupRan, a.snap.Cleanup)
	}
	saved, err := state.Load(a.cachePath)
	if err != nil {
		t.Fatalf("cache not written: %v", err)
```

4. Replace:

```go
	c.calls["taps"]++
	return c.fakeSource.Taps(ctx)
}
func (c *countingSource) Doctor(ctx context.Context) ([]brew.DoctorWarning, error) {
	c.calls["doctor"]++
	if c.onDoctor != nil {
```

   with:

```go
	c.calls["taps"]++
	return c.fakeSource.Taps(ctx)
}
func (c *countingSource) Cleanup(ctx context.Context) (brew.Cleanup, error) {
	c.calls["cleanup"]++
	return c.fakeSource.Cleanup(ctx)
}
func (c *countingSource) Doctor(ctx context.Context) ([]brew.DoctorWarning, error) {
	c.calls["doctor"]++
	if c.onDoctor != nil {
```

5. Replace:

```go
	os.Remove(a.cachePath)

	drain(t, a, a.request(qServices))
	if src.calls["services"] != 1 || src.calls["doctor"] != 0 || src.calls["installed"] != 0 {
		t.Errorf("a targeted request ran %v", src.calls)
	}
	if _, err := state.Load(a.cachePath); err != nil {
```

   with:

```go
	os.Remove(a.cachePath)

	drain(t, a, a.request(qServices))
	if src.calls["services"] != 1 || src.calls["doctor"] != 0 || src.calls["cleanup"] != 0 || src.calls["installed"] != 0 {
		t.Errorf("a targeted request ran %v", src.calls)
	}
	if _, err := state.Load(a.cachePath); err != nil {
```

- [ ] **Step 6: Edit `internal/ui/helpers_test.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
		t.Fatal(err)
	}
	s.Doctor, s.DoctorRan = brew.ParseDoctor(string(readFixture(t, "doctor.txt"))), true
	return s
}
```

   with:

```go
		t.Fatal(err)
	}
	s.Doctor, s.DoctorRan = brew.ParseDoctor(string(readFixture(t, "doctor.txt"))), true
	s.Cleanup = brew.Cleanup{
		Standard: brew.CleanupEstimate{Items: 1, Size: "3.9KB"},
		All:      brew.ParseCleanup(string(readFixture(t, "cleanup.txt"))),
	}
	s.CleanupRan = true
	return s
}
```

- [ ] **Step 7: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/brew/client_test.go:50:16: c.Cleanup undefined (type *Client has no field or method Cleanup)
    internal/brew/client_test.go:54:10: undefined: Cleanup
    internal/brew/client_test.go:54:28: undefined: CleanupEstimate
```

- [ ] **Step 8: Edit `internal/brew/client.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	return ParseTaps(out)
}

// Doctor runs `brew doctor`. It exits 1 whenever it finds warnings, so a
// non-zero exit that produced at least one warning is a normal result. A
// timeout, a failure to run brew, or a non-zero exit with no warnings parsed
```

   with:

```go
	return ParseTaps(out)
}

// Cleanup asks brew what each cleanup would free, running both dry-runs
// at once.
func (c *Client) Cleanup(ctx context.Context) (Cleanup, error) {
	type result struct {
		e   CleanupEstimate
		err error
	}
	dryRun := func(args ...string) chan result {
		ch := make(chan result, 1)
		go func() {
			ctx, cancel := context.WithTimeout(ctx, c.Timeout)
			defer cancel()
			out, errb, err := c.r.Run(ctx, args...)
			if err != nil {
				if ctx.Err() != nil {
					err = fmt.Errorf("timed out after %s", c.Timeout)
				}
				ch <- result{err: &QueryError{Args: args, Stderr: string(errb), Err: err}}
				return
			}
			// Some lines go to stderr: read both.
			ch <- result{e: ParseCleanup(string(out) + "\n" + string(errb))}
		}()
		return ch
	}
	std, all := dryRun("cleanup", "--dry-run"), dryRun("cleanup", "--dry-run", "--prune=all")
	s, a := <-std, <-all
	if s.err != nil {
		return Cleanup{}, s.err
	}
	if a.err != nil {
		return Cleanup{}, a.err
	}
	return Cleanup{Standard: s.e, All: a.e}, nil
}

// Doctor runs `brew doctor`. It exits 1 whenever it finds warnings, so a
// non-zero exit that produced at least one warning is a normal result. A
// timeout, a failure to run brew, or a non-zero exit with no warnings parsed
```

- [ ] **Step 9: Edit `internal/brew/parse.go`** (2 replacements; each old block appears exactly once)

1. Replace:

```go
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
```

   with:

```go
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
```

2. Replace:

```go
	return out, nil
}

// ParseDoctor splits `brew doctor` output into one warning per "Warning:"
// line. Text before the first warning (the standard disclaimer) is dropped.
func ParseDoctor(out string) []DoctorWarning {
```

   with:

```go
	return out, nil
}

// freeRe finds the total in brew cleanup's last line: "==> This operation
// would free approximately 678.9MB of disk space."
var freeRe = regexp.MustCompile(`would free approximately (\S+) of disk space`)

// ParseCleanup reads the output of `brew cleanup --dry-run`: a "Would
// remove: …" line per file or directory, then the total. A cleanup with
// nothing to do prints nothing.
func ParseCleanup(out string) CleanupEstimate {
	var e CleanupEstimate
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Would remove") {
			e.Items++
		}
	}
	if m := freeRe.FindStringSubmatch(out); m != nil {
		e.Size = m[1]
	}
	return e
}

// ParseDoctor splits `brew doctor` output into one warning per "Warning:"
// line. Text before the first warning (the standard disclaimer) is dropped.
func ParseDoctor(out string) []DoctorWarning {
```

- [ ] **Step 10: Edit `internal/brew/types.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go
	Title string `json:"title"`
	Body  string `json:"body"`
}
```

   with:

```go
	Title string `json:"title"`
	Body  string `json:"body"`
}

// CleanupEstimate is what one `brew cleanup --dry-run` says it would do.
type CleanupEstimate struct {
	Items int    `json:"items"` // files and directories it would remove
	Size  string `json:"size"`  // brew's total, e.g. "678.9MB"; "" when it frees nothing
}

// Cleanup is what each of the cleanups brewery offers would free.
type Cleanup struct {
	Standard CleanupEstimate `json:"standard"` // brew cleanup: old versions, downloads over 120 days old
	All      CleanupEstimate `json:"all"`      // brew cleanup --prune=all: every cached download too
}
```

- [ ] **Step 11: Edit `internal/state/snapshot.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go

// snapshotVersion changes whenever Snapshot's shape changes; older cache
// files are ignored rather than half-loaded.
const snapshotVersion = 1

// Snapshot is everything brewery knows about the installation at one time.
type Snapshot struct {
	Version   int                  `json:"version"`
	Packages  []brew.Package       `json:"packages"`
	Outdated  []brew.Outdated      `json:"outdated"`
	Services  []brew.Service       `json:"services"`
	Taps      []brew.Tap           `json:"taps"`
	Doctor    []brew.DoctorWarning `json:"doctor"`
	DoctorRan bool                 `json:"doctor_ran"`
	FetchedAt time.Time            `json:"fetched_at"`
}

// DefaultCachePath is $XDG_CACHE_HOME/brewery/snapshot.json, falling back
```

   with:

```go

// snapshotVersion changes whenever Snapshot's shape changes; older cache
// files are ignored rather than half-loaded.
const snapshotVersion = 2

// Snapshot is everything brewery knows about the installation at one time.
type Snapshot struct {
	Version    int                  `json:"version"`
	Packages   []brew.Package       `json:"packages"`
	Outdated   []brew.Outdated      `json:"outdated"`
	Services   []brew.Service       `json:"services"`
	Taps       []brew.Tap           `json:"taps"`
	Doctor     []brew.DoctorWarning `json:"doctor"`
	DoctorRan  bool                 `json:"doctor_ran"`
	Cleanup    brew.Cleanup         `json:"cleanup"`
	CleanupRan bool                 `json:"cleanup_ran"`
	FetchedAt  time.Time            `json:"fetched_at"`
}

// DefaultCachePath is $XDG_CACHE_HOME/brewery/snapshot.json, falling back
```

- [ ] **Step 12: Edit `internal/ui/app.go`** (6 replacements; each old block appears exactly once)

1. Replace:

```go

	inflight    map[query]bool // queries running now
	again       map[query]bool // requested while running: run once more when done
	doctorAfter bool           // run doctor when the JSON queries are done
	dirty       bool           // a query succeeded since the last cache save
	saveErr     error
```

   with:

```go

	inflight    map[query]bool // queries running now
	again       map[query]bool // requested while running: run once more when done
	doctorAfter bool           // run doctor and cleanup's dry-runs when the JSON queries are done
	dirty       bool           // a query succeeded since the last cache save
	saveErr     error
```

2. Replace:

```go
		}
	}
	a.loaded[qDoctor] = snap.DoctorRan
	a.sync()
	return a
}
```

   with:

```go
		}
	}
	a.loaded[qDoctor] = snap.DoctorRan
	a.loaded[qCleanup] = snap.CleanupRan
	a.sync()
	return a
}
```

3. Replace:

```go
	a.jobs.setRows(jobRows(recs, a.now()))
}

// refresh re-runs every query: the JSON ones in parallel, then doctor.
func (a *App) refresh() tea.Cmd {
	a.doctorAfter = true
	return a.request(jsonQueries...)
```

   with:

```go
	a.jobs.setRows(jobRows(recs, a.now()))
}

// refresh re-runs every query: the JSON ones in parallel, then doctor and
// cleanup's dry-runs.
func (a *App) refresh() tea.Cmd {
	a.doctorAfter = true
	return a.request(jsonQueries...)
```

4. Replace:

```go
		a.snap.Taps = m.taps
	case qDoctor:
		a.snap.Doctor, a.snap.DoctorRan = m.doctor, true
	}
	a.sync()
}
```

   with:

```go
		a.snap.Taps = m.taps
	case qDoctor:
		a.snap.Doctor, a.snap.DoctorRan = m.doctor, true
	case qCleanup:
		a.snap.Cleanup, a.snap.CleanupRan = m.cleanup, true
	}
	a.sync()
}
```

5. Replace:

```go
}

// handleResult applies one query result and decides what runs next: a
// repeat of the same query, doctor after a full refresh, and a cache save
// once the JSON queries are all back.
func (a *App) handleResult(m resultMsg) tea.Cmd {
	a.inflight[m.q] = false
	if m.err != nil && a.ctx.Err() != nil {
```

   with:

```go
}

// handleResult applies one query result and decides what runs next: a
// repeat of the same query, doctor and cleanup's dry-runs after a full
// refresh, and a cache save once the JSON queries are all back.
func (a *App) handleResult(m resultMsg) tea.Cmd {
	a.inflight[m.q] = false
	if m.err != nil && a.ctx.Err() != nil {
```

6. Replace:

```go
	}
	if a.doctorAfter && !a.jsonBusy() {
		a.doctorAfter = false
		doctor = a.request(qDoctor)
	}
	// Save before doctor starts: doctor can take 30s, and quitting
	// meanwhile must not lose the JSON results.
```

   with:

```go
	}
	if a.doctorAfter && !a.jsonBusy() {
		a.doctorAfter = false
		doctor = a.request(qDoctor, qCleanup)
	}
	// Save before doctor starts: doctor can take 30s, and quitting
	// meanwhile must not lose the JSON results.
```

- [ ] **Step 13: Edit `internal/ui/source.go`** (4 replacements; each old block appears exactly once)

1. Replace:

```go
	Services(ctx context.Context) ([]brew.Service, error)
	Taps(ctx context.Context) ([]brew.Tap, error)
	Doctor(ctx context.Context) ([]brew.DoctorWarning, error)
}

var _ Source = (*brew.Client)(nil)
```

   with:

```go
	Services(ctx context.Context) ([]brew.Service, error)
	Taps(ctx context.Context) ([]brew.Tap, error)
	Doctor(ctx context.Context) ([]brew.DoctorWarning, error)
	Cleanup(ctx context.Context) (brew.Cleanup, error)
}

var _ Source = (*brew.Client)(nil)
```

2. Replace:

```go
	qServices
	qTaps
	qDoctor
)

// queryNames name each query in the status bar.
var queryNames = [...]string{"installed", "outdated", "services", "taps", "doctor"}

// resultMsg carries one query's result back to the App.
type resultMsg struct {
```

   with:

```go
	qServices
	qTaps
	qDoctor
	qCleanup
)

// queryNames name each query in the status bar.
var queryNames = [...]string{"installed", "outdated", "services", "taps", "doctor", "cleanup"}

// resultMsg carries one query's result back to the App.
type resultMsg struct {
```

3. Replace:

```go
	services []brew.Service
	taps     []brew.Tap
	doctor   []brew.DoctorWarning
	err      error
}

// jsonQueries are the queries a full refresh runs in parallel; doctor
// follows once they have all returned.
var jsonQueries = []query{qInstalled, qOutdated, qServices, qTaps}

// fetch runs one query. ctx is the App's root context, so quitting stops
```

   with:

```go
	services []brew.Service
	taps     []brew.Tap
	doctor   []brew.DoctorWarning
	cleanup  brew.Cleanup
	err      error
}

// jsonQueries are the queries a full refresh runs in parallel; doctor and
// cleanup's dry-runs follow once they have all returned.
var jsonQueries = []query{qInstalled, qOutdated, qServices, qTaps}

// fetch runs one query. ctx is the App's root context, so quitting stops
```

4. Replace:

```go
			m.taps, m.err = src.Taps(ctx)
		case qDoctor:
			m.doctor, m.err = src.Doctor(ctx)
		}
		return m
	}
```

   with:

```go
			m.taps, m.err = src.Taps(ctx)
		case qDoctor:
			m.doctor, m.err = src.Doctor(ctx)
		case qCleanup:
			m.cleanup, m.err = src.Cleanup(ctx)
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
git commit -m "feat: measure what cleanup would free after each refresh"
```

---

### Task 7: Clean up from Home or with c

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/app.go`
- Create: `internal/ui/cleanup_test.go`
- Modify: `internal/ui/home.go`
- Modify: `internal/ui/home_test.go`

**Interfaces:**
- **Consumes:** Task 6's `Snapshot.Cleanup`, `qCleanup`; `confirmBox`, `App.run`.
- **Produces:**
  - `openCleanup = -2`: the Cleanup tile's `tab`, which `handleKey` turns into `askCleanup`.
  - The fifth tile, Cleanup.
  - `(*App) askCleanup() tea.Cmd`; `c` in `actionKey`, on every tab.
  - The Home hint gains `c clean up`.

Choosing a cleanup runs it as a normal pane job (with `JobEnv`). When it finishes, the dry-runs run again, so the tile catches up.

- [ ] **Step 1: Create `internal/ui/cleanup_test.go`**

```go
package ui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"brewery/internal/brew"
)

func TestCleanupOffersBoth(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "c") // c works on any tab
	if a.confirm == nil || a.confirm.title != "Clean up Homebrew?" {
		t.Fatalf("c should ask: %+v", a.confirm)
	}
	out := stripANSI(a.render())
	for _, want := range []string{"The standard cleanup frees 3.9KB:", "Clearing the download cache frees 86.7MB:", "[y] standard", "[a] whole cache", "[n] cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	keys(t, a, "y")
	if want := []string{"cleanup"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	if env := fs.envs[len(fs.envs)-1]; !slices.Contains(env, "HOMEBREW_NO_AUTO_UPDATE=1") {
		t.Errorf("a pane action gets JobEnv: %v", env)
	}
}

func TestCleanupWholeCache(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	a.snap.Cleanup.Standard = brew.CleanupEstimate{}
	keys(t, a, "c")
	if out := stripANSI(a.render()); !strings.Contains(out, "The standard cleanup has nothing to remove.") || strings.Contains(out, "[y]") {
		t.Errorf("only the whole cache is on offer:\n%s", out)
	}
	keys(t, a, "a")
	if want := []string{"cleanup", "--prune=all"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestCleanupWithNothingToDo(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.snap.Cleanup = brew.Cleanup{}
	keys(t, a, "c")
	if a.confirm != nil || !strings.Contains(stripANSI(a.statusBar()), "nothing to clean up") {
		t.Errorf("nothing to clean: no dialog, just say so: %+v %q", a.confirm, stripANSI(a.statusBar()))
	}
	a.loaded[qCleanup] = false
	keys(t, a, "c")
	if a.confirm != nil || !strings.Contains(stripANSI(a.statusBar()), "still measuring what cleanup would free") {
		t.Errorf("before the dry-runs: %q", stripANSI(a.statusBar()))
	}
}

func TestCleanupFromTheHomeTile(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "l", "l", "l", "l", "enter")
	if a.confirm == nil || a.confirm.title != "Clean up Homebrew?" {
		t.Fatalf("enter on the Cleanup tile should ask: %+v", a.confirm)
	}
}
```

- [ ] **Step 2: Edit `internal/ui/home_test.go`** (5 replacements; each old block appears exactly once)

1. Replace:

```go
// loadedHome is a Home tab whose queries have all succeeded.
func loadedHome() homeTab {
	return homeTab{st: testStyles, errs: map[query]error{},
		loaded: map[query]bool{qInstalled: true, qOutdated: true, qServices: true, qTaps: true, qDoctor: true}}
}

func TestHomeTiles(t *testing.T) {
```

   with:

```go
// loadedHome is a Home tab whose queries have all succeeded.
func loadedHome() homeTab {
	return homeTab{st: testStyles, errs: map[query]error{},
		loaded: map[query]bool{qInstalled: true, qOutdated: true, qServices: true, qTaps: true, qDoctor: true, qCleanup: true}}
}

func TestHomeTiles(t *testing.T) {
```

2. Replace:

```go
		"Outdated=2 (lua-language-server, docker-desktop)",
		"Services=1 error (postgresql@16)",
		"Doctor=2 warnings (see below)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("tiles:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
```

   with:

```go
		"Outdated=2 (lua-language-server, docker-desktop)",
		"Services=1 error (postgresql@16)",
		"Doctor=2 warnings (see below)",
		"Cleanup=86.7MB (c to reclaim)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("tiles:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
```

3. Replace:

```go
	}
}

func TestHomeNavigation(t *testing.T) {
	h := homeTab{st: testStyles}
	snap := fixtureSnapshot(t)
```

   with:

```go
	}
}

func TestHomeCleanupTile(t *testing.T) {
	cleanupTile := func(h homeTab, snap state.Snapshot) string {
		tl := h.tiles(snap)[4]
		return tl.value + " (" + tl.detail + ")"
	}
	h := loadedHome()
	if got := cleanupTile(h, state.Snapshot{CleanupRan: true}); got != "✓ tidy (nothing to remove)" {
		t.Errorf("nothing to clean: %q", got)
	}
	only := state.Snapshot{CleanupRan: true, Cleanup: brew.Cleanup{Standard: brew.CleanupEstimate{Items: 1, Size: "3.9KB"}}}
	if got := cleanupTile(h, only); got != "3.9KB (c to reclaim)" {
		t.Errorf("standard cleanup only: %q", got)
	}
	if got := cleanupTile(homeTab{st: testStyles}, state.Snapshot{}); got != "… (measuring)" {
		t.Errorf("before the dry-runs: %q", got)
	}
	h.errs[qCleanup] = errors.New("brew cleanup --dry-run: exit status 1")
	if got := cleanupTile(h, fixtureSnapshot(t)); got != "✗ failed (r retries)" {
		t.Errorf("failed dry-run: %q", got)
	}
}

func TestHomeNavigation(t *testing.T) {
	h := homeTab{st: testStyles}
	snap := fixtureSnapshot(t)
```

4. Replace:

```go
	for range 5 {
		h.update(press("l"), snap)
	}
	if h.sel != 3 {
		t.Errorf("selection should stop at the last tile, got %d", h.sel)
	}
	if tab := h.update(press("enter"), snap); tab != -1 {
		t.Errorf("enter on doctor tile -> %d, want -1", tab)
	}
```

   with:

```go
	for range 5 {
		h.update(press("l"), snap)
	}
	if h.sel != 4 {
		t.Errorf("selection should stop at the last tile, got %d", h.sel)
	}
	if tab := h.update(press("enter"), snap); tab != openCleanup {
		t.Errorf("enter on cleanup tile -> %d, want openCleanup", tab)
	}
	h.update(press("h"), snap)
	if tab := h.update(press("enter"), snap); tab != -1 {
		t.Errorf("enter on doctor tile -> %d, want -1", tab)
	}
```

5. Replace:

```go
	h := loadedHome()
	view := h.view(fixtureSnapshot(t), nil, testNow, 140, 30)
	out := stripANSI(view)
	for _, want := range []string{"Installed", "5 formulae · 2 casks", "lua-language-server, docker-", "1 error",
		"⚠ The following taps are not trusted:", "brew untap oven-sh/bun", "⚠ Some installed casks are deprecated"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
```

   with:

```go
	h := loadedHome()
	view := h.view(fixtureSnapshot(t), nil, testNow, 140, 30)
	out := stripANSI(view)
	for _, want := range []string{"Installed", "5 formulae · 2 casks", "lua-language-server,", "1 error", "86.7MB",
		"⚠ The following taps are not trusted:", "brew untap oven-sh/bun", "⚠ Some installed casks are deprecated"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/home_test.go:122:51: undefined: openCleanup
```

- [ ] **Step 4: Edit `internal/ui/actions.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
	switch {
	case k == "U":
		return a.run(action{title: "update", args: []string{"update"}, full: true}), true
	case k == "u" && (a.tab == tabInstalled || a.tab == tabOutdated):
		return a.upgradeSelected(), true
	case k == "x" && a.tab == tabInstalled:
```

   with:

```go
	switch {
	case k == "U":
		return a.run(action{title: "update", args: []string{"update"}, full: true}), true
	case k == "c":
		return a.askCleanup(), true
	case k == "u" && (a.tab == tabInstalled || a.tab == tabOutdated):
		return a.upgradeSelected(), true
	case k == "x" && a.tab == tabInstalled:
```

2. Replace:

```go
	return nil
}

// serviceKey starts (s), stops (S, after asking) or restarts (R) the
// selected service.
func (a *App) serviceKey(k string) tea.Cmd {
```

   with:

```go
	return nil
}

// askCleanup offers brew's standard cleanup and pruning the whole download
// cache, each with what its dry-run says it would free.
func (a *App) askCleanup() tea.Cmd {
	cl := a.snap.Cleanup
	switch {
	case a.errs[qCleanup] != nil:
		return a.note("couldn't measure the cleanup · r retries")
	case !a.loaded[qCleanup]:
		return a.note("still measuring what cleanup would free")
	case cl.Standard.Size == "" && cl.All.Size == "":
		return a.note("nothing to clean up")
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
	box.choices = append(box.choices, choice{"n", "cancel", nil})
	a.confirm = box
	return nil
}

// serviceKey starts (s), stops (S, after asking) or restarts (R) the
// selected service.
func (a *App) serviceKey(k string) tea.Cmd {
```

3. Replace:

```go
	case tabServices:
		return "s start · S stop · R restart · l log"
	}
	return "1-5 tabs · r refresh · U update"
}
```

   with:

```go
	case tabServices:
		return "s start · S stop · R restart · l log"
	}
	return "1-5 tabs · r refresh · U update · c clean up"
}
```

- [ ] **Step 5: Edit `internal/ui/app.go`** (1 replacement; each old block appears exactly once)

1. Replace:

```go

	switch a.tab {
	case tabHome:
		if t := a.home.update(msg, a.snap); t >= 0 {
			a.tab = t
		}
	case tabInstalled:
		return a.installed.update(msg)
```

   with:

```go

	switch a.tab {
	case tabHome:
		switch t := a.home.update(msg, a.snap); {
		case t >= 0:
			a.tab = t
		case t == openCleanup:
			return a.askCleanup()
		}
	case tabInstalled:
		return a.installed.update(msg)
```

- [ ] **Step 6: Edit `internal/ui/home.go`** (3 replacements; each old block appears exactly once)

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
	"cmp"
	"fmt"
	"strings"
	"time"
```

2. Replace:

```go
	value  string
	style  lipgloss.Style
	detail string
	tab    int // tab to open on Enter; -1 for none
}

// homeTab shows health tiles and doctor's warnings.
type homeTab struct {
	st        theme.Styles
```

   with:

```go
	value  string
	style  lipgloss.Style
	detail string
	tab    int // tab to open on Enter; -1 for none, openCleanup for the cleanup dialog
}

// openCleanup is the Cleanup tile's Enter: it asks to clean up rather than
// opening a tab.
const openCleanup = -2

// homeTab shows health tiles and doctor's warnings.
type homeTab struct {
	st        theme.Styles
```

3. Replace:

```go
			doctor.value, doctor.style, doctor.detail = plural(n, "warning"), st.Warn, "see below"
		}
	}
	return []tile{installed, outdated, services, doctor}
}

// markUnknown replaces a tile's numbers when its query has failed or has
```

   with:

```go
			doctor.value, doctor.style, doctor.detail = plural(n, "warning"), st.Warn, "see below"
		}
	}
	cleanup := tile{"Cleanup", "✓ tidy", st.OK, "nothing to remove", openCleanup}
	switch size := cmp.Or(snap.Cleanup.All.Size, snap.Cleanup.Standard.Size); {
	case h.errs[qCleanup] != nil:
		cleanup.value, cleanup.style, cleanup.detail = "✗ failed", st.Err, "r retries"
	case !h.loaded[qCleanup]:
		cleanup.value, cleanup.style, cleanup.detail = "…", st.Muted, "measuring"
	case size != "":
		cleanup.value, cleanup.style, cleanup.detail = size, st.Title, "c to reclaim"
	}
	return []tile{installed, outdated, services, doctor, cleanup}
}

// markUnknown replaces a tile's numbers when its query has failed or has
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(ui): clean up from Home or with c, after showing what it frees"
```

---

### Task 8: Untap from the Taps tab

**Files:**
- Modify: `internal/ui/actions.go`
- Modify: `internal/ui/actions_test.go`
- Create: `internal/ui/taps_test.go`

**Interfaces:**
- **Consumes:** `Package.Tap`, `brew.Tap`; `confirmBox`, `App.run`, `plural`.
- **Produces:** `(*App) selectedTap() (brew.Tap, bool)`, `(*App) askUntap() tea.Cmd`, `tapContents(brew.Tap) string`; `x` on Taps; the Taps hint `x untap`.

brew refuses to untap a tap that something installed came from, so brewery explains that up front, as it does for an uninstall that something still needs.

- [ ] **Step 1: Edit `internal/ui/actions_test.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
	if want := []string{"update"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	for _, q := range []string{"installed", "outdated", "services", "taps", "doctor"} {
		if src.calls[q] != 1 {
			t.Errorf("after brew update, %s ran %d times, want 1", q, src.calls[q])
		}
```

   with:

```go
	if want := []string{"update"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	for _, q := range []string{"installed", "outdated", "services", "taps", "doctor", "cleanup"} {
		if src.calls[q] != 1 {
			t.Errorf("after brew update, %s ran %d times, want 1", q, src.calls[q])
		}
```

2. Replace:

```go
func TestActionKeysOnlyWorkOnTheirTab(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "3", "x", "p", "4", "u", "A", "5", "s")
	if len(fs.calls) != 0 || a.confirm != nil {
		t.Errorf("keys off their tab must do nothing: %v %+v", fs.calls, a.confirm)
	}
```

   with:

```go
func TestActionKeysOnlyWorkOnTheirTab(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "3", "x", "p", "l", "4", "u", "A", "x", "5", "s", "l")
	if len(fs.calls) != 0 || a.confirm != nil {
		t.Errorf("keys off their tab must do nothing: %v %+v", fs.calls, a.confirm)
	}
```

3. Replace:

```go
	for _, tt := range []struct{ key, want string }{
		{"2", "u upgrade · x uninstall · p pin · f/F view"},
		{"3", "u upgrade · A upgrade all"},
		{"4", "s start · S stop · R restart"},
		{"5", "U update"},
	} {
		keys(t, a, tt.key)
		if bar := stripANSI(a.statusBar()); !strings.Contains(bar, tt.want) {
```

   with:

```go
	for _, tt := range []struct{ key, want string }{
		{"2", "u upgrade · x uninstall · p pin · f/F view"},
		{"3", "u upgrade · A upgrade all"},
		{"4", "s start · S stop · R restart · l log"},
		{"5", "x untap"},
		{"1", "U update · c clean up"},
	} {
		keys(t, a, tt.key)
		if bar := stripANSI(a.statusBar()); !strings.Contains(bar, tt.want) {
```

- [ ] **Step 2: Create `internal/ui/taps_test.go`**

```go
package ui

import (
	"reflect"
	"strings"
	"testing"

	"brewery/internal/brew"
)

func TestUntapAsksFirst(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "5", "x") // anomalyco/tap: nothing installed from it
	if a.confirm == nil || a.confirm.title != "Untap anomalyco/tap?" {
		t.Fatalf("x should ask: %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Its 2 formulae and 1 cask won't be available") {
		t.Errorf("the dialog should say what goes:\n%s", out)
	}
	keys(t, a, "y")
	if want := []string{"untap", "anomalyco/tap"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "x untap") {
		t.Errorf("the hint should name x: %q", bar)
	}
}

func TestUntapRefusesATapInUse(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "5", "j", "x") // homebrew/core: fd, git, … came from it
	if a.confirm == nil || a.confirm.title != "Can't untap homebrew/core" {
		t.Fatalf("want an explanation, got %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Installed from it: fd, git, pcre2") {
		t.Errorf("should say what came from it:\n%s", out)
	}
	keys(t, a, "y")
	if len(fs.calls) != 0 {
		t.Errorf("nothing may run: %v", fs.calls)
	}
}

func TestTapContents(t *testing.T) {
	for _, tt := range []struct {
		formulae, casks int
		want            string
	}{
		{0, 0, ""},
		{1, 0, "1 formula"},
		{3, 0, "3 formulae"},
		{0, 1, "1 cask"},
		{2, 4, "2 formulae and 4 casks"},
	} {
		if got := tapContents(brew.Tap{Formulae: tt.formulae, Casks: tt.casks}); got != tt.want {
			t.Errorf("tapContents(%d, %d) = %q, want %q", tt.formulae, tt.casks, got, tt.want)
		}
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...`
Expected: FAIL. The first errors look like:

```text
    internal/ui/taps_test.go:57:13: undefined: tapContents
```

- [ ] **Step 4: Edit `internal/ui/actions.go`** (3 replacements; each old block appears exactly once)

1. Replace:

```go
		return a.serviceKey(k), true
	case a.tab == tabServices && k == "l":
		return a.openLog(), true
	}
	return nil, false
}
```

   with:

```go
		return a.serviceKey(k), true
	case a.tab == tabServices && k == "l":
		return a.openLog(), true
	case a.tab == tabTaps && k == "x":
		return a.askUntap(), true
	}
	return nil, false
}
```

2. Replace:

```go
	return nil
}

// serviceKey starts (s), stops (S, after asking) or restarts (R) the
// selected service.
func (a *App) serviceKey(k string) tea.Cmd {
```

   with:

```go
	return nil
}

// selectedTap is the tap under the cursor, found by name.
func (a *App) selectedTap() (brew.Tap, bool) {
	row := a.taps.table.SelectedRow()
	if a.taps.n == 0 || row == nil {
		return brew.Tap{}, false
	}
	for _, t := range a.snap.Taps {
		if t.Name == row[0] {
			return t, true
		}
	}
	return brew.Tap{}, false
}

// askUntap confirms untapping the selected tap. If anything installed came
// from it, it explains that brew would refuse instead.
func (a *App) askUntap() tea.Cmd {
	t, ok := a.selectedTap()
	if !ok {
		return nil
	}
	var from []string
	for _, p := range a.snap.Packages {
		if p.Tap == t.Name {
			from = append(from, p.Name)
		}
	}
	if len(from) > 0 {
		a.confirm = &confirmBox{
			title:   "Can't untap " + t.Name,
			body:    []string{"Installed from it: " + strings.Join(from, ", ") + ".", "Uninstall those first, or brew will refuse."},
			choices: []choice{{"n", "close", nil}},
		}
		return nil
	}
	untap := action{title: "untap " + t.Name, args: []string{"untap", t.Name}, refresh: []query{qTaps}}
	box := &confirmBox{
		title: "Untap " + t.Name + "?",
		choices: []choice{
			{"y", "untap", func() tea.Cmd { return a.run(untap) }},
			{"n", "cancel", nil},
		},
	}
	if what := tapContents(t); what != "" {
		box.body = []string{"Its " + what + " won't be available", "until you tap it again."}
	}
	a.confirm = box
	return nil
}

// tapContents says what a tap holds, such as "2 formulae and 1 cask".
func tapContents(t brew.Tap) string {
	var parts []string
	switch t.Formulae {
	case 0:
	case 1:
		parts = append(parts, "1 formula")
	default:
		parts = append(parts, fmt.Sprintf("%d formulae", t.Formulae))
	}
	if t.Casks > 0 {
		parts = append(parts, plural(t.Casks, "cask"))
	}
	return strings.Join(parts, " and ")
}

// serviceKey starts (s), stops (S, after asking) or restarts (R) the
// selected service.
func (a *App) serviceKey(k string) tea.Cmd {
```

3. Replace:

```go
		return "u upgrade · A upgrade all"
	case tabServices:
		return "s start · S stop · R restart · l log"
	}
	return "1-5 tabs · r refresh · U update · c clean up"
}
```

   with:

```go
		return "u upgrade · A upgrade all"
	case tabServices:
		return "s start · S stop · R restart · l log"
	case tabTaps:
		return "x untap"
	}
	return "1-5 tabs · r refresh · U update · c clean up"
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -race ./...`
Expected: every package `ok`. Then run `gofmt -l .` (no output) and `go vet ./...` (clean).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(ui): untap from the Taps tab"
```

---

### Task 9: End-to-end check in a real terminal

**Files:** none (verification only)

- [ ] **Step 1: Build and run**

Run: `make run`

- [ ] **Step 2: Check each of these by hand.** None of these change anything.
  1. **Home:** once the refresh settles, a fifth tile, Cleanup, shows a size and `c to reclaim`, or `✓ tidy`.
  2. **Cleanup dialog:** press `c`. "Clean up Homebrew?" lists what each cleanup frees, with `[y] standard` and/or `[a] whole cache`, and `[n] cancel`. Press `n`: nothing runs.
  3. **Services preview:** press `4`. The right pane shows the selected service's status and pid, command, plist and log path, and the end of its log. Move with `j`/`k`: the preview follows.
  4. **Log viewer:** press `l`. The log opens full screen and follows new lines ("Waiting for data…"). Press ctrl+c, then `q`: brewery comes back intact.
  5. **Untap dialog:** press `5`, select a tap something is installed from, and press `x`. It explains that brew would refuse. Press `esc`, then `q`.

- [ ] **Step 3 (optional; this changes your system): clean up**

Press `c`, then `y` or `a`. The drawer shows brew removing files; afterwards the Cleanup tile updates.

- [ ] **Step 4: Final checks**

```bash
go test -race -count=1 ./...
go test -tags live -count=1 ./internal/brew/ ./internal/ui/
go vet ./...
gofmt -l .
go mod tidy -diff
```

Expected: all `ok`, with no output from vet, gofmt, or `go mod tidy -diff`.
