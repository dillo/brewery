# Autoremove, Services Cleanup and Retry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `brew autoremove` and `brew services cleanup` to Maintenance, each with a preview and a confirm prompt, and let `R` on History run a finished job again.

**Architecture:**
- **New queries:** two background queries, `qAutoremove` and `qServiceFiles`, run beside doctor and cleanup's dry runs and are saved in the snapshot.
- **Services cleanup preview:** Homebrew's own rule, applied in the UI to the service files and the services list.
- **Retry:** reuses the confirm box, the job queue, `takeover` and `refreshAfter`.

**Tech Stack:** Go 1.26, Bubble Tea v2, Lip Gloss v2, bubbles table.

**Spec:** `docs/superpowers/specs/2026-10-18-brewery-tidy-and-retry-design.md`

## Global Constraints

- **Statuses:** Maintenance's statuses and texts are exactly as the spec's table: `✗ failed`, `…`, `nothing to remove`, `removes N formula/formulae`, `nothing to clean`, `N unused file(s)`, `stops N service(s)`.
- **Row order:** Doctor, Cleanup, Autoremove, Services cleanup, Update Homebrew.
- **Running services:** a service counts as running when `Loaded` is true (it's in `launchctl list`), not by its status.
- **Retry environment:** a retried job runs with `typed: false`, so brewery's usual environment applies and brew doesn't auto-update.
- **Isolation:** tests never read or write the real `~/Library/LaunchAgents`, the real brew, or the user's cache or history.
- **Coverage:** `make cover` stays at 90% or above in total and per package.
- **Writing style:** comments and user-facing text are plain, short sentences, matching the surrounding code.
- **Commits:** conventional style (`feat(ui): …`, `test: …`).
- **Leave alone:** `.golangci.yml`.

## Test and lint commands

| Purpose | Command |
|---|---|
| All tests | `go test ./...` |
| Rewrite goldens, after checking the diff is intended | `go test ./internal/ui -run TestScreens -update -count=1` |
| Lint | `golangci-lint run ./...` (must report `0 issues.`) |
| Coverage | `make cover` |

---

### Task 1: brew: autoremove's dry run, service files, and service labels

**Files:**
- Modify: `internal/brew/types.go` (`Service`)
- Modify: `internal/brew/parse.go` (add `ParseAutoremove`)
- Modify: `internal/brew/client.go` (`Client.AgentsDir`, `NewClient`, `Autoremove`, `ServiceFiles`)
- Test: `internal/brew/parse_test.go`, `internal/brew/client_test.go`

**Interfaces:**
- Produces:
  - `brew.Service` gains `Label string` (JSON `service_name`) and `Loaded bool` (JSON `loaded`).
  - `func ParseAutoremove(out string) []string`
  - `func (c *Client) Autoremove(ctx context.Context) ([]string, error)`
  - `func (c *Client) ServiceFiles(ctx context.Context) ([]string, error)`
  - `Client.AgentsDir string`

- [ ] **Step 1: Write the failing tests**

In `internal/brew/parse_test.go`:

```go
func TestParseAutoremove(t *testing.T) {
	for _, tt := range []struct {
		name, out string
		want      []string
	}{
		{"several", "==> Would autoremove 2 unneeded formulae:\nlibyaml\nm4\n", []string{"libyaml", "m4"}},
		{"one", "==> Would autoremove 1 unneeded formula:\nm4\n", []string{"m4"}},
		{"none", "", nil},
		{"stops at the next heading", "==> Would autoremove 1 unneeded formula:\nm4\n==> Something else\nnot-a-formula\n", []string{"m4"}},
		{"stops at a blank line", "==> Would autoremove 1 unneeded formula:\nm4\n\nWarning: hi\n", []string{"m4"}},
	} {
		if got := ParseAutoremove(tt.out); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestParseServicesLabelAndLoaded(t *testing.T) {
	svcs, err := ParseServices(fixture(t, "services.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s := svcs[0]; s.Label != "homebrew.mxcl.postgresql@16" || !s.Loaded {
		t.Errorf("got label %q, loaded %v", s.Label, s.Loaded)
	}
}
```

Add `"slices"` to the imports if it isn't there.

In `internal/brew/client_test.go`:

```go
func TestClientAutoremove(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{
		"autoremove --dry-run": []byte("==> Would autoremove 2 unneeded formulae:\nlibyaml\nm4\n"),
	}})
	got, err := c.Autoremove(context.Background())
	if err != nil || !slices.Equal(got, []string{"libyaml", "m4"}) {
		t.Errorf("got %q, %v", got, err)
	}
	c = NewClient(fakeRunner{err: map[string]error{"autoremove --dry-run": errors.New("exit status 1")}})
	if _, err := c.Autoremove(context.Background()); err == nil || !strings.Contains(err.Error(), "brew autoremove --dry-run") {
		t.Errorf("a failed dry run is an error naming it: %v", err)
	}
}

func TestClientServiceFiles(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"homebrew.mxcl.redis.plist", "sh.brew.thing.plist", "com.apple.other.plist", "homebrew.notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := NewClient(fakeRunner{})
	c.AgentsDir = dir
	got, err := c.ServiceFiles(context.Background())
	want := []string{filepath.Join(dir, "homebrew.mxcl.redis.plist"), filepath.Join(dir, "sh.brew.thing.plist")}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("got %q, %v; want %q", got, err, want)
	}

	c.AgentsDir = filepath.Join(dir, "missing")
	if got, err := c.ServiceFiles(context.Background()); err != nil || len(got) != 0 {
		t.Errorf("a missing directory has no files: %q, %v", got, err)
	}

	if os.Geteuid() == 0 {
		t.Skip("root reads any directory")
	}
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	c.AgentsDir = locked
	if _, err := c.ServiceFiles(context.Background()); err == nil {
		t.Error("an unreadable directory is an error")
	}
}

func TestNewClientFindsLaunchAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got, want := NewClient(fakeRunner{}).AgentsDir, filepath.Join(home, "Library", "LaunchAgents"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
```

Add `"slices"` to the imports.

- [ ] **Step 2: Run the tests and see them fail**

Run: `go test ./internal/brew/ -run 'Autoremove|ServiceFiles|LaunchAgents|LabelAndLoaded' -count=1`

Expected: the build fails with `undefined: ParseAutoremove`, and `Label`, `Loaded` and `AgentsDir` are undefined.

- [ ] **Step 3: Implement**

In `internal/brew/types.go`, add two fields to `Service` after `Status`:

```go
	Label    string `json:"service_name"` // launchd's label, homebrew.mxcl.<name>
	Loaded   bool   `json:"loaded"`       // in launchctl list, which brew counts as running
```

In `internal/brew/parse.go`, after `ParseCleanup`:

```go
// ParseAutoremove reads `brew autoremove --dry-run`: a "==> Would
// autoremove N unneeded formulae:" heading, then one name a line. With
// nothing to remove brew prints nothing.
func ParseAutoremove(out string) []string {
	var names []string
	listing := false
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "==> Would autoremove"):
			listing = true
		case !listing:
		case line == "" || strings.HasPrefix(line, "==>"):
			return names
		default:
			names = append(names, line)
		}
	}
	return names
}
```

In `internal/brew/client.go`, add `AgentsDir` to `Client`, after `Prefix`:

```go
	AgentsDir string // where brew services keeps its launchd files
```

Change `NewClient` to fill it in:

```go
func NewClient(r Runner) *Client {
	c := &Client{r: r, Timeout: 30 * time.Second}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		c.AgentsDir = filepath.Join(home, "Library", "LaunchAgents")
	}
	return c
}
```

Add `Autoremove` and `ServiceFiles` after `Cleanup`:

```go
// Autoremove asks brew which formulae autoremove would uninstall: those
// installed only as dependencies that nothing needs any more.
func (c *Client) Autoremove(ctx context.Context) ([]string, error) {
	out, err := c.query(ctx, "autoremove", "--dry-run")
	if err != nil {
		return nil, err
	}
	return ParseAutoremove(string(out)), nil
}

// ServiceFiles lists brew's launchd files in AgentsDir, sorted, which
// brew services cleanup removes when their service isn't loaded. A
// missing directory has none.
func (c *Client) ServiceFiles(context.Context) ([]string, error) {
	entries, err := os.ReadDir(c.AgentsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		n := e.Name()
		if (strings.HasPrefix(n, "homebrew.") || strings.HasPrefix(n, "sh.brew.")) && strings.HasSuffix(n, ".plist") {
			files = append(files, filepath.Join(c.AgentsDir, n))
		}
	}
	return files, nil
}
```

Add `"io/fs"` to the imports. `os.ReadDir` already returns entries sorted by name.

- [ ] **Step 4: Run the tests and see them pass**

Run: `go test ./internal/brew/ -count=1`

Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/brew
git commit -m "feat(brew): autoremove's dry run, brew's service files, service labels"
```

---

### Task 2: Plumbing: snapshot, queries and refreshes

**Files:**
- Modify: `internal/state/snapshot.go`
- Modify: `internal/ui/source.go`
- Modify: `internal/ui/app.go` (`New`, `apply`, `handleResult`)
- Modify: `internal/ui/actions.go` (`diskQueries`, `askUninstall`, `serviceKey`)
- Modify: `internal/ui/marks.go` (uninstalling marked packages)
- Modify: `internal/ui/cmdline.go` (`refreshAfter`)
- Modify: `internal/ui/maintenance.go` (add `unusedServices`)
- Modify (fakes): `internal/ui/app_test.go` (`fakeSource`, `countingSource`), `internal/ui/helpers_test.go` (`fixtureSnapshot`), `internal/ui/screenshots_test.go` (`demoSource`, demo snapshot)
- Test: `internal/ui/tidy_test.go` (new), `internal/state/snapshot_test.go` if a version test exists

**Interfaces:**
- Consumes (Task 1): `brew.Client.Autoremove`, `brew.Client.ServiceFiles`, `brew.Service.Label` and `.Loaded`.
- Produces:
  - `state.Snapshot` gains `Autoremove []string`, `AutoremoveRan bool`, `ServiceFiles []string` and `ServiceFilesRan bool`, with JSON names `autoremove`, `autoremove_ran`, `service_files` and `service_files_ran`.
  - `snapshotVersion = 4`.
  - The `ui.Source` interface gains `Autoremove(ctx) ([]string, error)` and `ServiceFiles(ctx) ([]string, error)`.
  - New queries `qAutoremove` and `qServiceFiles`, after `qSizes`.
  - `func unusedServices(files []string, svcs []brew.Service) (remove, stop []string)` in `internal/ui/maintenance.go`.
  - `fakeSource` gains `autoremoveErr error` and `serviceFilesErr error`.
  - `countingSource` counts `"autoremove"` and `"service files"`.

- [ ] **Step 1: Snapshot fields**

In `internal/state/snapshot.go`, set `const snapshotVersion = 4`, and add after `CleanupRan`:

```go
	Autoremove      []string `json:"autoremove"`        // formulae brew autoremove would uninstall
	AutoremoveRan   bool     `json:"autoremove_ran"`
	ServiceFiles    []string `json:"service_files"`     // brew's launchd files, by full path
	ServiceFilesRan bool     `json:"service_files_ran"`
```

Run gofmt so the struct's alignment holds. If a state test hard-codes version 3, change it to 4.

- [ ] **Step 2: Write the failing UI tests**

Create `internal/ui/tidy_test.go`:

```go
package ui

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
)

func TestUnusedServices(t *testing.T) {
	dir := "/Users/me/Library/LaunchAgents"
	file := func(label string) string { return filepath.Join(dir, label+".plist") }
	svcs := []brew.Service{
		{Name: "redis", Label: "homebrew.mxcl.redis", Loaded: true, Status: "started", File: file("homebrew.mxcl.redis")},
		{Name: "postgresql@16", Label: "homebrew.mxcl.postgresql@16", Loaded: true, Status: "error", File: file("homebrew.mxcl.postgresql@16")},
		{Name: "mysql", Label: "homebrew.mxcl.mysql", Status: "stopped", File: file("homebrew.mxcl.mysql")},
		{Name: "ghost", Label: "homebrew.mxcl.ghost", Loaded: true, Status: "started", File: file("homebrew.mxcl.ghost")},
	}
	files := []string{file("homebrew.mxcl.redis"), file("homebrew.mxcl.postgresql@16"), file("homebrew.mxcl.mysql"), file("sh.brew.old")}
	remove, stop := unusedServices(files, svcs)
	if want := []string{file("homebrew.mxcl.mysql"), file("sh.brew.old")}; !slices.Equal(remove, want) {
		t.Errorf("remove %q, want %q (a loaded service keeps its file, even in error)", remove, want)
	}
	if want := []string{"ghost"}; !slices.Equal(stop, want) {
		t.Errorf("stop %q, want %q (loaded, but its file is gone)", stop, want)
	}
}

func TestTidyQueriesFollowDoctor(t *testing.T) {
	src := newCountingSource(t)
	a := newTestApp(t, src)
	drain(t, a, a.Init())
	if src.calls["autoremove"] != 1 || src.calls["service files"] != 1 {
		t.Errorf("a full refresh checks autoremove and the service files once each: %v", src.calls)
	}
	if !a.loaded[qAutoremove] || !a.loaded[qServiceFiles] {
		t.Error("both are loaded after the refresh")
	}
}

func TestTidyQueriesAfterActions(t *testing.T) {
	for _, tt := range []struct {
		name  string
		entry string
		keys  []string
		want  string
	}{
		{"uninstall checks autoremove", "Top-level", []string{"x", "y"}, "autoremove"},
		{"uninstall checks the service files", "Top-level", []string{"x", "y"}, "service files"},
		{"a service start checks the service files", "Services", []string{"s"}, "service files"},
	} {
		src := newCountingSource(t)
		a := New(src, (&fakeStarter{}).start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
		a.now = func() time.Time { return testNow }
		a.tickEvery = 0
		a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
		show(t, a, tt.entry)
		keys(t, a, tt.keys...) // the fake job finishes at once
		if src.calls[tt.want] != 1 {
			t.Errorf("%s: calls %v", tt.name, src.calls)
		}
	}
}

func TestTidyQueryErrorsKeepTheLastResult(t *testing.T) {
	src := newFakeSource(t)
	src.autoremoveErr, src.serviceFilesErr = errors.New("boom"), errors.New("boom")
	a := New(src, nil, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	before := slices.Clone(a.snap.Autoremove)
	drain(t, a, a.Init())
	if a.errs[qAutoremove] == nil || a.errs[qServiceFiles] == nil || !slices.Equal(a.snap.Autoremove, before) {
		t.Errorf("errors are kept, data isn't wiped: %v %v %q", a.errs[qAutoremove], a.errs[qServiceFiles], a.snap.Autoremove)
	}
}
```

`newTestApp(t, src)` in `app_test.go` starts from an empty snapshot with no job starter. If the first package on Top-level has something that needs it, so `x` shows "Can't uninstall", pick a row that `askUninstall` accepts by moving the cursor first, as other uninstall tests do. Copy their setup.

- [ ] **Step 3: Update the fakes**

In `internal/ui/app_test.go`:
- Add `autoremoveErr` and `serviceFilesErr error` fields to `fakeSource`, and update its comment.
- Add these methods:

```go
func (f *fakeSource) Autoremove(context.Context) ([]string, error) {
	if f.autoremoveErr != nil {
		return nil, f.autoremoveErr
	}
	return f.snap.Autoremove, nil
}
func (f *fakeSource) ServiceFiles(context.Context) ([]string, error) {
	if f.serviceFilesErr != nil {
		return nil, f.serviceFilesErr
	}
	return f.snap.ServiceFiles, nil
}
```

Add these to `countingSource`:

```go
func (c *countingSource) Autoremove(ctx context.Context) ([]string, error) {
	c.calls["autoremove"]++
	return c.fakeSource.Autoremove(ctx)
}
func (c *countingSource) ServiceFiles(ctx context.Context) ([]string, error) {
	c.calls["service files"]++
	return c.fakeSource.ServiceFiles(ctx)
}
```

In `internal/ui/helpers_test.go`, `fixtureSnapshot` gets these lines after `s.CleanupRan = true`:

```go
	s.Autoremove, s.AutoremoveRan = []string{"libyaml", "m4"}, true
	agents := "/Users/locutus/Library/LaunchAgents/"
	s.ServiceFiles = []string{agents + "homebrew.mxcl.mysql.plist", agents + "homebrew.mxcl.postgresql@16.plist", agents + "homebrew.mxcl.redis.plist"}
	s.ServiceFilesRan = true
```

Both fixture services are loaded, so only mysql's file is unused.

In `internal/ui/screenshots_test.go`, `demoSource` gets methods returning `d.snap.Autoremove, nil` and `d.snap.ServiceFiles, nil`. The demo snapshot sets `s.AutoremoveRan, s.ServiceFilesRan = true, true`, and `s.ServiceFiles` to each demo service's `File` that isn't empty, so the demo shows nothing to clean.

- [ ] **Step 4: Run the tests and see them fail**

Run: `go test ./internal/ui -run 'Unused|Tidy' -count=1`

Expected: the build fails with `undefined: unusedServices`, `qAutoremove` and `qServiceFiles`.

- [ ] **Step 5: Implement the queries**

In `internal/ui/source.go`:
- Add to `Source`:

```go
	Autoremove(ctx context.Context) ([]string, error)
	ServiceFiles(ctx context.Context) ([]string, error)
```

- Add `qAutoremove` and `qServiceFiles` after `qSizes`.
- `queryNames` becomes `{"installed", "outdated", "services", "taps", "doctor", "cleanup", "sizes", "autoremove", "service files"}`.
- `resultMsg` gains these fields, after `sizes`:

```go
	autoremove   []string
	serviceFiles []string
```

- `fetch` gains these cases:

```go
		case qAutoremove:
			m.autoremove, m.err = src.Autoremove(ctx)
		case qServiceFiles:
			m.serviceFiles, m.err = src.ServiceFiles(ctx)
```

- The `jsonQueries` comment becomes: "jsonQueries are the queries a full refresh runs in parallel; doctor, cleanup's and autoremove's dry-runs, the service files and the disk sizes follow once they have all returned."

In `internal/ui/app.go`:
- In `New`, after `a.loaded[qSizes] = …`:

```go
	a.loaded[qAutoremove] = snap.AutoremoveRan
	a.loaded[qServiceFiles] = snap.ServiceFilesRan
```

- In `apply`, after the `qCleanup` case:

```go
	case qAutoremove:
		a.snap.Autoremove, a.snap.AutoremoveRan = m.autoremove, true
	case qServiceFiles:
		a.snap.ServiceFiles, a.snap.ServiceFilesRan = m.serviceFiles, true
```

- In `handleResult`, change `doctor = a.request(qDoctor, qCleanup, qSizes)` to:

```go
		doctor = a.request(qDoctor, qCleanup, qSizes, qAutoremove, qServiceFiles)
```

- Update that function's comment and `refresh`'s comment so they name the new checks.

In `internal/ui/actions.go`:
- `diskQueries = []query{qInstalled, qOutdated, qSizes, qAutoremove}`. Its comment becomes: "diskQueries after anything that also changes what's on disk, and so what autoremove would remove."
- `askUninstall`: `refresh := append(slices.Clone(diskQueries), qServices, qServiceFiles) // its service goes too`
- `serviceKey`: `refresh: []query{qServices, qServiceFiles}`

In `internal/ui/marks.go`, find the uninstall refresh built for marked packages (it appends `qServices` to `diskQueries`), and append `qServiceFiles` the same way.

In `internal/ui/cmdline.go`, `refreshAfter`'s `services` case returns `[]query{qServices, qServiceFiles}, false`.

In `internal/ui/maintenance.go`, add the following, with `"path/filepath"` and `"github.com/dillo/brewery/internal/brew"` in the imports:

```go
// unusedServices is what brew services cleanup would do, by Homebrew's own
// rule: remove each service file whose service launchd hasn't loaded, and
// stop each loaded service whose file is gone. Loaded is what brew calls
// running, a service in error included.
func unusedServices(files []string, svcs []brew.Service) (remove, stop []string) {
	have := make(map[string]bool, len(files))
	for _, f := range files {
		have[f] = true
	}
	loaded := map[string]bool{}
	for _, s := range svcs {
		if !s.Loaded {
			continue
		}
		loaded[s.Label] = true
		if s.File != "" && !have[s.File] {
			stop = append(stop, s.Name)
		}
	}
	for _, f := range files {
		if !loaded[strings.TrimSuffix(filepath.Base(f), ".plist")] {
			remove = append(remove, f)
		}
	}
	return remove, stop
}
```

- [ ] **Step 6: Run the tests and see them pass**

Run: `go test ./... -count=1`

Expected: everything passes. If an existing test counts every query the refresh makes, or lists `queryNames`, add the two new ones; the change is intended. Don't update goldens in this task: nothing visible has changed yet. If a golden fails, find out why before going on.

- [ ] **Step 7: Lint and commit**

Run `golangci-lint run ./...`, which must report `0 issues.`, then:

```bash
git add internal
git commit -m "feat(ui): check autoremove and brew's service files with doctor"
```

---

### Task 3: Maintenance: Autoremove and Services cleanup rows

**Files:**
- Modify: `internal/ui/maintenance.go` (row constants, `syncMaint`, `maintPreview`, `maintRun`)
- Modify: `internal/ui/actions.go` (add `askAutoremove`, `askServicesCleanup`, `autoremoveLines`, `servicesCleanupLines`, `formulae`)
- Modify: `internal/ui/help.go` (Maintenance's enter line)
- Modify: `internal/ui/testdata/golden/maintenance.txt` and `help.txt`, via `-update`
- Test: `internal/ui/maintenance_test.go`

**Interfaces:**
- Consumes (Task 2): `a.snap.Autoremove`, `a.snap.ServiceFiles`, `a.loaded` and `a.errs` for `qAutoremove` and `qServiceFiles`, `unusedServices`, and `diskQueries`, which now includes `qAutoremove`.
- Produces: the constants `maintDoctor`, `maintCleanup`, `maintAutoremove`, `maintServices` and `maintUpdate`, in that order.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/maintenance_test.go`, reusing its `rowLine` helper:

```go
func TestMaintenanceTidyRows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Maintenance")
	if got := rowLine(t, a, "Autoremove"); !strings.Contains(got, "removes 2 formulae") {
		t.Errorf("autoremove row: %q", got)
	}
	if got := rowLine(t, a, "Services cleanup"); !strings.Contains(got, "1 unused file") {
		t.Errorf("services row: %q", got)
	}

	a.snap.Autoremove = []string{"m4"}
	a.snap.ServiceFiles = a.snap.ServiceFiles[1:] // mysql's file is gone: nothing unused
	a.syncMaint()
	if got := rowLine(t, a, "Autoremove"); !strings.Contains(got, "removes 1 formula") {
		t.Errorf("one formula: %q", got)
	}
	if got := rowLine(t, a, "Services cleanup"); !strings.Contains(got, "nothing to clean") {
		t.Errorf("nothing unused: %q", got)
	}

	a.snap.Autoremove = nil
	a.snap.ServiceFiles = a.snap.ServiceFiles[1:] // redis is loaded, but its file is gone
	a.syncMaint()
	if got := rowLine(t, a, "Autoremove"); !strings.Contains(got, "nothing to remove") {
		t.Errorf("nothing to remove: %q", got)
	}
	if got := rowLine(t, a, "Services cleanup"); !strings.Contains(got, "stops 1 service") {
		t.Errorf("a service to stop: %q", got)
	}

	a.errs[qAutoremove], a.loaded[qServiceFiles] = errors.New("boom"), false
	a.syncMaint()
	if got := rowLine(t, a, "Autoremove"); !strings.Contains(got, "✗ failed") {
		t.Errorf("failed: %q", got)
	}
	if got := rowLine(t, a, "Services cleanup"); !strings.Contains(got, "…") {
		t.Errorf("not loaded: %q", got)
	}
}

func TestMaintenanceAutoremove(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Maintenance")
	keys(t, a, "j", "j") // Doctor, Cleanup, Autoremove
	out := stripANSI(a.render())
	for _, want := range []string{"Unneeded formulae", "libyaml", "m4", "enter removes them"} {
		if !strings.Contains(out, want) {
			t.Errorf("details lack %q:\n%s", want, out)
		}
	}
	keys(t, a, "enter")
	if a.confirm == nil || a.confirm.title != "Remove unneeded formulae?" {
		t.Fatalf("enter asks first: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"autoremove"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestMaintenanceServicesCleanup(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Maintenance")
	keys(t, a, "j", "j", "j")
	out := stripANSI(a.render())
	for _, want := range []string{"Unused service files", "homebrew.mxcl.mysql.plist", "enter cleans up"} {
		if !strings.Contains(out, want) {
			t.Errorf("details lack %q:\n%s", want, out)
		}
	}
	keys(t, a, "enter")
	if a.confirm == nil || a.confirm.title != "Clean up services?" {
		t.Fatalf("enter asks first: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"services", "cleanup"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestMaintenanceTidyNotes(t *testing.T) {
	for _, tt := range []struct {
		name string
		down []string
		set  func(a *App)
		want string
	}{
		{"autoremove: nothing", []string{"j", "j"}, func(a *App) { a.snap.Autoremove = nil }, "nothing to remove"},
		{"autoremove: checking", []string{"j", "j"}, func(a *App) { a.loaded[qAutoremove] = false }, "still checking"},
		{"autoremove: failed", []string{"j", "j"}, func(a *App) { a.errs[qAutoremove] = errors.New("boom") }, "couldn't check · r retries"},
		{"services: nothing", []string{"j", "j", "j"}, func(a *App) { a.snap.ServiceFiles = a.snap.ServiceFiles[1:] }, "nothing to clean"},
		{"services: checking", []string{"j", "j", "j"}, func(a *App) { a.loaded[qServiceFiles] = false }, "still checking"},
		{"services: failed", []string{"j", "j", "j"}, func(a *App) { a.errs[qServiceFiles] = errors.New("boom") }, "couldn't check · r retries"},
	} {
		a := newJobApp(t, &fakeStarter{})
		show(t, a, "Maintenance")
		keys(t, a, tt.down...)
		tt.set(a)
		a.syncMaint()
		keys(t, a, "enter")
		if a.confirm != nil || !strings.Contains(stripANSI(a.statusBar()), tt.want) {
			t.Errorf("%s: confirm %+v, status %q", tt.name, a.confirm, stripANSI(a.statusBar()))
		}
		if !strings.Contains(stripANSI(a.render()), tt.want) {
			t.Errorf("%s: the details say %q too:\n%s", tt.name, tt.want, stripANSI(a.render()))
		}
	}
}
```

Add `"errors"` and `"reflect"` to the imports if they're missing. `a.sync()` doesn't rebuild Maintenance's rows (`Update` calls `syncMaint`), so the tests call `a.syncMaint()` after changing the snapshot. If "still checking" doesn't appear in the details when not loaded, see Step 3's wording: both the details and the note say it.

- [ ] **Step 2: Run the tests and see them fail**

Run: `go test ./internal/ui -run 'MaintenanceTidy|MaintenanceAutoremove|MaintenanceServicesCleanup' -count=1`

Expected: FAIL, because there's no Autoremove row.

- [ ] **Step 3: Implement**

In `internal/ui/maintenance.go`, the constants become:

```go
const (
	maintDoctor = iota
	maintCleanup
	maintAutoremove
	maintServices
	maintUpdate
)
```

In `syncMaint`, before `update := …`:

```go
	autoremove := theme.Tint(st.P.Overlay0, "…")
	switch n := len(a.snap.Autoremove); {
	case a.errs[qAutoremove] != nil:
		autoremove = theme.Tint(st.P.Red, "✗ failed")
	case !a.loaded[qAutoremove]:
	case n == 0:
		autoremove = theme.Tint(st.P.Green, "nothing to remove")
	default:
		autoremove = theme.Tint(st.P.Blue, "removes "+formulae(n))
	}
	services := theme.Tint(st.P.Overlay0, "…")
	remove, stop := unusedServices(a.snap.ServiceFiles, a.snap.Services)
	switch {
	case a.errs[qServiceFiles] != nil:
		services = theme.Tint(st.P.Red, "✗ failed")
	case !a.loaded[qServiceFiles] || !a.loaded[qServices]:
	case len(remove)+len(stop) == 0:
		services = theme.Tint(st.P.Green, "nothing to clean")
	default:
		var parts []string
		if len(remove) > 0 {
			parts = append(parts, plural(len(remove), "unused file"))
		}
		if len(stop) > 0 {
			parts = append(parts, "stops "+plural(len(stop), "service"))
		}
		services = theme.Tint(st.P.Blue, strings.Join(parts, " · "))
	}
```

The rows become:

```go
	a.maint.setRows([]table.Row{{"Doctor", doctor}, {"Cleanup", cleanup}, {"Autoremove", autoremove},
		{"Services cleanup", services}, {"Update Homebrew", update}})
```

Update `syncMaint`'s comment: "…from what doctor, the dry-runs, the service files and History say now."

In `maintPreview`, add these cases before `maintUpdate`:

```go
	case maintAutoremove:
		switch {
		case a.errs[qAutoremove] != nil:
			body = st.Err.Render("couldn't check · r retries")
		case !a.loaded[qAutoremove]:
			body = st.Muted.Render("still checking")
		case len(a.snap.Autoremove) == 0:
			body = st.OK.Render("nothing to remove")
		default:
			body = strings.Join(autoremoveLines(a.snap.Autoremove), "\n") + "\n\n" + st.Muted.Render("enter removes them")
		}
		return "Autoremove", body
	case maintServices:
		remove, stop := unusedServices(a.snap.ServiceFiles, a.snap.Services)
		switch {
		case a.errs[qServiceFiles] != nil:
			body = st.Err.Render("couldn't check · r retries")
		case !a.loaded[qServiceFiles] || !a.loaded[qServices]:
			body = st.Muted.Render("still checking")
		case len(remove)+len(stop) == 0:
			body = st.OK.Render("nothing to clean")
		default:
			body = strings.Join(servicesCleanupLines(remove, stop), "\n") + "\n\n" + st.Muted.Render("enter cleans up")
		}
		return "Services cleanup", body
```

Update `maintPreview`'s comment to mention them. In `maintRun`, add:

```go
	case maintAutoremove:
		return a.askAutoremove()
	case maintServices:
		return a.askServicesCleanup()
```

Update `maintRun`'s comment: "doctor again, the cleanup, autoremove or services cleanup prompt, or brew update."

In `internal/ui/actions.go`, after `cleanupLines`:

```go
// askAutoremove confirms uninstalling the formulae autoremove's dry run
// found, after naming them.
func (a *App) askAutoremove() tea.Cmd {
	names := a.snap.Autoremove
	switch {
	case a.errs[qAutoremove] != nil:
		return a.note("couldn't check · r retries")
	case !a.loaded[qAutoremove]:
		return a.note("still checking")
	case len(names) == 0:
		return a.note("nothing to remove")
	}
	act := action{title: "autoremove", args: []string{"autoremove"}, refresh: diskQueries}
	a.confirm = &confirmBox{
		title: "Remove unneeded formulae?",
		body:  autoremoveLines(names),
		choices: []choice{
			{"y", "remove", func() tea.Cmd { return a.run(act) }},
			{"n", "cancel", nil},
		},
	}
	return nil
}

// askServicesCleanup confirms brew services cleanup, after naming the
// files it would remove and the services it would stop.
func (a *App) askServicesCleanup() tea.Cmd {
	remove, stop := unusedServices(a.snap.ServiceFiles, a.snap.Services)
	switch {
	case a.errs[qServiceFiles] != nil:
		return a.note("couldn't check · r retries")
	case !a.loaded[qServiceFiles] || !a.loaded[qServices]:
		return a.note("still checking")
	case len(remove)+len(stop) == 0:
		return a.note("nothing to clean")
	}
	act := action{title: "services cleanup", args: []string{"services", "cleanup"}, refresh: []query{qServices, qServiceFiles}}
	a.confirm = &confirmBox{
		title: "Clean up services?",
		body:  servicesCleanupLines(remove, stop),
		choices: []choice{
			{"y", "clean up", func() tea.Cmd { return a.run(act) }},
			{"n", "cancel", nil},
		},
	}
	return nil
}

// autoremoveLines names what autoremove would uninstall, as its prompt
// and Maintenance's details show it.
func autoremoveLines(names []string) []string {
	lines := []string{"Unneeded formulae, which only other formulae needed:"}
	for _, n := range names {
		lines = append(lines, "  "+n)
	}
	return lines
}

// servicesCleanupLines names the files services cleanup would remove and
// the services it would stop.
func servicesCleanupLines(remove, stop []string) []string {
	var lines []string
	if len(remove) > 0 {
		lines = append(lines, "Unused service files:")
		for _, f := range remove {
			lines = append(lines, "  "+tilde(f))
		}
	}
	if len(stop) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "Running services whose file is gone:")
		for _, s := range stop {
			lines = append(lines, "  "+s)
		}
	}
	return lines
}

// formulae counts formulae: "1 formula", "2 formulae".
func formulae(n int) string {
	if n == 1 {
		return "1 formula"
	}
	return fmt.Sprintf("%d formulae", n)
}
```

`tilde` is in `services.go`. If it reads the home directory in a way tests can't control, check that the test's `homebrew.mxcl.mysql.plist` assertion still holds; it only matches the base name, so it will.

In `internal/ui/help.go`, Maintenance's line becomes `{"enter", "run doctor, clean up, autoremove, or brew update"}`.

- [ ] **Step 4: Run the tests, then update the goldens**

Run: `go test ./internal/ui -count=1`

The new tests pass. The `maintenance.txt` and `help.txt` goldens fail. Look at their diffs to confirm the only changes are the two new rows and the help line, then:

Run: `go test ./internal/ui -run TestScreens -update -count=1 && go test ./... -count=1`

Expected: all pass. Fix any other test that used the old `maintUpdate` value, or expected three rows.

- [ ] **Step 5: Lint and commit**

Run `golangci-lint run ./...`, which must report `0 issues.`, then:

```bash
git add internal
git commit -m "feat(ui): autoremove and services cleanup on Maintenance"
```

---

### Task 4: History: R runs a job again

**Files:**
- Modify: `internal/jobs/jobs.go` (`Job.Takeover`)
- Modify: `internal/jobs/history.go` (`Record.Takeover`, `NewRecord`)
- Modify: `internal/ui/takeover.go` (`takeover` sets `Takeover`)
- Modify: `internal/ui/actions.go` (`actionKey`, add `askRunAgain` and `runAgain`, History's hints)
- Modify: `internal/ui/help.go` (History's group)
- Modify: goldens `history.txt` and `help.txt` if they show the changed hints, via `-update`
- Test: `internal/ui/historytab_test.go`, `internal/jobs/history_test.go`

**Interfaces:**
- Consumes: `takesOver(args) bool`, `a.takeover(args) tea.Cmd`, `refreshAfter(args) ([]query, bool)`, `a.queue.Pending(args) *jobs.Job`, `a.jobs.cursor() int`, `a.history []jobs.Record`, `confirmBox`, and `choice`.
- Produces:
  - `jobs.Job.Takeover bool`
  - `jobs.Record.Takeover bool` (JSON `takeover,omitempty`)
  - `func (a *App) askRunAgain() tea.Cmd`
  - `func (a *App) runAgain(args []string, fullScreen bool) tea.Cmd`

- [ ] **Step 1: Write the failing tests**

In `internal/jobs/history_test.go`, check the existing test file's helpers first, then add:

```go
func TestNewRecordKeepsTakeover(t *testing.T) {
	j := &Job{Title: "info fd", Args: []string{"info", "fd"}, State: Done, Takeover: true}
	if r := NewRecord(j, ""); !r.Takeover {
		t.Error("a full-screen job's record says so")
	}
	b, err := json.Marshal(NewRecord(&Job{Args: []string{"update"}}, ""))
	if err != nil || strings.Contains(string(b), "takeover") {
		t.Errorf("a drawer job leaves the field out: %s %v", b, err)
	}
}
```

Add `"encoding/json"` and `"strings"` to the imports if they're missing.

In `internal/ui/historytab_test.go`, building on how its tests seed `a.history`, for example with `UseHistory`:

```go
func TestHistoryRunAgain(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{
		{Title: "upgrade fd", Args: []string{"upgrade", "fd"}, State: jobs.Failed, ExitCode: 1, Started: testNow.Add(-time.Hour), Ended: testNow.Add(-time.Hour)},
	})
	show(t, a, "History")
	keys(t, a, "R")
	if a.confirm == nil || a.confirm.title != "Run again?" || !slices.Contains(a.confirm.body, "brew upgrade fd") {
		t.Fatalf("R asks first: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"upgrade", "fd"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	if env := fs.envs[len(fs.envs)-1]; !slices.Contains(env, "HOMEBREW_NO_AUTO_UPDATE=1") {
		t.Errorf("a retry runs as brewery's own jobs do, without auto-update: %v", env)
	}

	keys(t, a, "l", "R") // from the details too
	if a.confirm == nil {
		t.Error("R works from the details")
	}
}

func TestHistoryRunAgainTakesOver(t *testing.T) {
	for _, r := range []jobs.Record{
		{Title: "edit fd", Args: []string{"edit", "fd"}},                 // by its args
		{Title: "info fd", Args: []string{"info", "fd"}, Takeover: true}, // a ! line
	} {
		fs, tk := &fakeStarter{}, &fakeTakeover{}
		a := newJobApp(t, fs)
		a.UseTakeover(tk.cmd)
		r.State, r.Started, r.Ended = jobs.Done, testNow, testNow
		a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{r})
		show(t, a, "History")
		a.Update(press("R"))
		_, cmd := a.Update(press("y"))
		if len(tk.calls) != 1 || !reflect.DeepEqual(tk.calls[0], r.Args) || cmd == nil || len(fs.calls) != 0 {
			t.Errorf("%v: took over with %v, drawer %v", r.Args, tk.calls, fs.calls)
		}
	}
}

func TestHistoryRunAgainEdges(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseHistory(jobs.History{Dir: t.TempDir()}, nil)
	show(t, a, "History")
	keys(t, a, "R")
	if a.confirm != nil {
		t.Error("no job, no prompt")
	}

	fs := &fakeStarter{procs: []*fakeProc{newFakeProc(0)}} // never exits: the install stays running
	a = newJobApp(t, fs)
	install := []string{"install", "fd"}
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{{Title: "install fd", Args: install, State: jobs.Failed, Started: testNow, Ended: testNow}})
	show(t, a, "History")
	keys(t, a, "R", "y", "R", "y")
	if !strings.Contains(stripANSI(a.statusBar()), "install fd is already queued") {
		t.Errorf("a second install waits for the first: %q", stripANSI(a.statusBar()))
	}
}
```

`fakeStarter` hands out its `procs` in order, and a `fakeProc` that is never told to exit keeps its job running. `queue.Pending` matches queued, running and waiting jobs, so the second `R y` finds the first install.

- [ ] **Step 2: Run the tests and see them fail**

Run: `go test ./internal/jobs ./internal/ui -run 'Takeover|RunAgain' -count=1`

Expected: the build fails because `Takeover` is unknown. Once it builds, R does nothing.

- [ ] **Step 3: Implement**

In `internal/jobs/jobs.go`, add to `Job` after `Attempts`:

```go
	Takeover bool // ran full screen, so running it again does too
```

In `internal/jobs/history.go`, add to `Record` after `ExitCode`:

```go
	Takeover bool      `json:"takeover,omitempty"` // ran full screen
```

`NewRecord` copies it: add `Takeover: j.Takeover,` to the literal. Run gofmt.

In `internal/ui/takeover.go`, the job `takeover` builds gains `Takeover: true`.

In `internal/ui/actions.go`, `actionKey` gains this case, next to the Services ones:

```go
	case a.tab == tabHistory && k == "R":
		return a.askRunAgain(), true
```

Then add, after `serviceKey`:

```go
// askRunAgain offers to run the selected History job again.
func (a *App) askRunAgain() tea.Cmd {
	i := a.jobs.cursor()
	if i < 0 || len(a.history[i].Args) == 0 {
		return nil
	}
	r := a.history[i]
	args := slices.Clone(r.Args)
	a.confirm = &confirmBox{
		title: "Run again?",
		body:  []string{"brew " + strings.Join(args, " ")},
		choices: []choice{
			{"y", "run", func() tea.Cmd { return a.runAgain(args, r.Takeover) }},
			{"n", "cancel", nil},
		},
	}
	return nil
}

// runAgain runs args as brewery runs its own jobs: full screen if the job
// took the terminal before, otherwise in the drawer without brew's
// auto-update, refreshing what the command touches.
func (a *App) runAgain(args []string, fullScreen bool) tea.Cmd {
	if fullScreen || takesOver(args) {
		return a.takeover(args)
	}
	if args[0] == "install" && a.queue.Pending(args) != nil {
		return a.note(strings.Join(args, " ") + " is already queued")
	}
	refresh, all := refreshAfter(args)
	return a.run(action{title: strings.Join(args, " "), args: args, refresh: refresh, full: all})
}
```

In `actionHints`, History's details hint becomes `"j/k scroll · h back to the list · R run again"`, and its list hint becomes `"l details · R run again · / filter by command · y copy the command"`.

In `internal/ui/help.go`, History's group becomes:

```go
		here = helpGroup{"History", [][2]string{{"l tab enter", "details, then j/k scroll"}, {"R", "run the job again"}}}
```

- [ ] **Step 4: Run the tests, then update the goldens**

Run: `go test ./... -count=1`

The new tests pass. Any golden that shows History's hints or help fails. Check that the diff is only those strings, then:

Run: `go test ./internal/ui -run TestScreens -update -count=1 && go test ./... -count=1`

Expected: all pass.

- [ ] **Step 5: Lint, check coverage, commit**

Run `golangci-lint run ./...`, which must report `0 issues.`

Run `make cover`. The total and every package must be at 90% or above. If a new branch isn't covered and pulls a package under, add a test for it.

```bash
git add internal
git commit -m "feat(ui): R on History runs a job again"
```
