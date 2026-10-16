# brewery Plan 7½: Follow-ups — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the deferred items still open after Plan 7, before the release.
- **Discover:** tap dependencies count as installed; an install already queued isn't asked again; analytics failures say what went wrong; a failed fetch is remembered across restarts.
- **Sizes:** time running out keeps what was measured; brew's folders are asked once; only sizes copy the package list.
- **Looks:** the selected row keeps one text color after a tinted cell; Services STATUS fits `error (exit 127)` with an icon; `--no-icons` help.
- **Keys:** the list behind a lone preview ignores its keys; previews start on the list when you come back to a tab; help closes when brew asks something.
- **Robustness:** crash logs catch the runtime's fatal errors, keep the newest five, and survive a run that died; history saves work without hard links.

**Architecture:** No new packages. Each fix stays in the package that owns the behaviour:
- `internal/jobs`: `Queue.Pending`; `History.publish` with a rename fallback.
- `internal/analytics`: `StatusError`, `ErrBadList`, `Store.Tried`/`SetTried`.
- `internal/brew`: partial `Sizes`; `Client.dataDirs` caches `brew --cache`/`--repository`.
- `internal/theme`: `TableSelected` without a foreground.
- `internal/ui`: the rest.
- `cmd/brewery`: the crash log.

**Tech Stack:**
- Go ≥ 1.25
- Bubble Tea v2, Bubbles v2, Lip Gloss v2
- `golang.org/x/sys/unix` (already in `go.sum` as an indirect dependency; Task 6 makes it direct)

**Spec:** `docs/superpowers/specs/2026-10-16-brewery-follow-ups-design.md`, all of it.

**Roadmap:** the release is still Plan 8, and it waits until the user asks.

**Found while building this:**
- **The spec's `Tint` fix was wrong.** Unselected cells use the terminal's own text color (`TableCell` has none), so making `Tint` return to `p.Text` would recolor text after a tint on every other row. The bug was the selected row's own foreground (`p.Text`): after a tinted cell, the rest of the selected row fell back to the terminal's color. On `:jobs`, STATUS comes first, so the whole row did. The selected row now keeps the terminal's text color; its background and bold carry the highlight.
- **A fatal error never reaches `stop`.** The runtime exits at once, so pointing fd 2 at the temp file alone would leave a hidden `.crash-*.log` nobody is told about. Temp files now carry the process ID, and the next start turns a dead run's file into a crash log and says so on exit.
- **The `:` line gap from Plan 7** was one test calling `wantCatalogue` directly. Completing `:install rip` from the catalogue was already tested key by key.
- **macOS has two "not supported" errors:** `ENOTSUP` (45) and `EOPNOTSUPP` (102). The hard-link fallback checks both.

**Verified:** every code block in this plan was compiled and tested in a prototype (branch `proto/follow-ups`, 3c32d39..07810e4) before the plan was written.
- Each task's new tests fail before its code and pass after. (Tasks 5–7 change signatures or fields the tests use, so some fail to compile instead.)
- `go test -race ./...`, the live tests, `go vet`, `gofmt -l` and `go mod tidy -diff` are clean at the end.
- **The real binary** ran in a PTY at 50×24 with scratch XDG dirs: `4` then `enter` showed the Services preview alone, `j` left it unchanged, widening to 120×30 showed both, and narrowing to 50×24 again started on the list.

## Global Constraints

These come from Plans 1–7 and still hold:
- Module path `brewery`; Go ≥ 1.25.
- **Only `internal/brew` and `main` run brew.** Only `internal/analytics` uses the network.
- Views use `theme.Styles` only. Table cells have no color of their own; `theme.Tint` colors text in a cell.
- One job at a time; lock retries; pane actions get `brew.JobEnv`.
- Key routing is by mode first (Plan 5's order). A confirmation gets every key.
- Refreshes are never dropped; the cache is saved before doctor runs.
- `splitFor`: side by side from 100 columns, the preview under the list from 60, otherwise one at a time.
- Real-binary runs must use **absolute** scratch paths for `XDG_STATE_HOME` and `XDG_CACHE_HOME`.
- **Never install anything for real while testing.** Open the install dialog and answer `n`.

Added by this plan:
- **Already queued:** `a` on Discover, or a typed `install …`, whose exact brew arguments match a queued, running or lock-waiting job flashes "`<name>` is already queued" (`install <args>` for a typed line) and starts nothing.
- **Analytics errors** (Discover's empty analytics views):

  | Cause | Text |
  |---|---|
  | anything else (network) | couldn't reach formulae.brew.sh · r retries |
  | `*analytics.StatusError` | formulae.brew.sh answered 404 · r retries |
  | `analytics.ErrBadList` | couldn't read formulae.brew.sh's lists · r retries |

- **Tried day:** `<cache dir>/analytics/tried` holds `2006-01-02`, the day a fetch last started. Opening Discover doesn't fetch again that day; `r` does.
- **Sizes:** a partial result (some sizes with an error) is merged over the older sizes and is no failure. No sizes with an error is a failure, as before.
- **Services STATUS** is 18 wide (floor 10).
- **Crash logs:** `$XDG_STATE_HOME/brewery/crash-YYYYMMDD-HHMMSS.log`, newest 5 kept. Temp files are `.crash-<pid>-*.log`.

## File Structure

```
internal/jobs/jobs.go            Queue.Pending
internal/jobs/history.go         History.link, publish, noHardLinks
internal/analytics/store.go      StatusError, ErrBadList, Tried, SetTried
internal/brew/sizes.go           Sizes returns what it measured with the error
internal/brew/client.go          Client.dataDirs (cache, repo); Client.measured (tests)
internal/theme/theme.go          TableSelected has no foreground
internal/ui/install.go           askInstall: already queued
internal/ui/cmdline.go           execute: a typed install already queued
internal/ui/discover.go          setHave adds FullName; fetchFailure; showTab drops preview focus
internal/ui/stats.go             Analytics.Tried/SetTried; cachedStatsMsg.tried
internal/ui/app.go               apply merges partial sizes; request clones only for sizes; typeIntoJob closes help; lone-preview keys
internal/ui/source.go            fetch's comment
internal/ui/list.go              previewShown; resize clears pvOpen; STATUS 18
cmd/brewery/crash.go             fd 2, timed names, keep 5, adopt leftovers
cmd/brewery/main.go              crash log in the state dir; earlier crashes; --no-icons help
```

---

### Task 1: Tap dependencies count as installed; an install already queued isn't asked again

**Files:**
- Modify: `internal/jobs/jobs.go`
- Modify: `internal/jobs/jobs_test.go`
- Modify: `internal/ui/discover.go`
- Modify: `internal/ui/install.go`
- Modify: `internal/ui/cmdline.go`
- Modify: `internal/ui/install_test.go`

**Interfaces:**
- **Consumes:** `kindArgs`, `entryKey`, `App.note`, `discoverTab.setHave`.
- **Produces:** `(*jobs.Queue) Pending(args []string) *jobs.Job`: the first job not `Finished()` whose `Args` equal `args` exactly, or nil.

- [ ] **Step 1: Append to `internal/jobs/jobs_test.go`**

```go

func TestPending(t *testing.T) {
	var q Queue
	a := q.Add("install fd", []string{"install", "fd"})
	q.Add("install --cask fd", []string{"install", "--cask", "fd"})
	if q.Pending([]string{"install", "fd"}) != a {
		t.Error("a queued job is pending")
	}
	q.Next(t0)
	if q.Pending([]string{"install", "fd"}) != a {
		t.Error("a running job is pending")
	}
	q.Finish(a, 0, false, "", t0)
	if j := q.Pending([]string{"install", "fd"}); j != nil {
		t.Errorf("a finished job isn't pending: %+v", j)
	}
	if q.Pending([]string{"install"}) != nil {
		t.Error("args must match exactly")
	}
}
```

- [ ] **Step 2: Append to `internal/ui/install_test.go`**

```go

func TestInstallKnowsTapDependencies(t *testing.T) {
	a, _, _, cat := newInstallApp(t)
	rg := ripgrep
	rg.Needs = []string{"oniguruma", "acme/tools/rgx"} // brew deps names tap packages in full
	cat.details["ripgrep"] = rg
	a.discover.setHave(append(slices.Clone(a.snap.Packages), brew.Package{Name: "rgx", FullName: "acme/tools/rgx", Kind: brew.KindFormula}))
	selectDiscover(t, a, "ripgrep")
	keys(t, a, "a")
	if got := confirmText(a); !strings.HasSuffix(got, "also installs 1 dependency: oniguruma") {
		t.Errorf("rgx from acme/tools is installed:\n%s", got)
	}
}

func TestInstallSaysWhenQueued(t *testing.T) {
	a, fs, _, _ := newInstallApp(t)
	fs.procs = []*fakeProc{newFakeProc(0)} // the install keeps running
	selectDiscover(t, a, "ripgrep")
	keys(t, a, "a", "y", "a")
	if a.confirm != nil || len(fs.calls) != 1 || !strings.Contains(stripANSI(a.statusBar()), "ripgrep is already queued") {
		t.Errorf("confirm %v, calls %v, status %q", a.confirm, fs.calls, stripANSI(a.statusBar()))
	}
	keys(t, a, ":")
	for _, k := range "install ripgrep" {
		keys(t, a, string(k))
	}
	keys(t, a, "enter")
	if len(fs.calls) != 1 || !strings.Contains(stripANSI(a.statusBar()), "install ripgrep is already queued") {
		t.Errorf("calls %v, status %q", fs.calls, stripANSI(a.statusBar()))
	}
}
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `go test ./internal/jobs/ ./internal/ui/ -run 'TestPending|TestInstall'`
Expected: FAIL. `jobs` doesn't compile (`q.Pending undefined`); once it does, `TestInstallKnowsTapDependencies` says "rgx from acme/tools is installed" and `TestInstallSaysWhenQueued` shows the dialog open again.

- [ ] **Step 4: Add `Pending` to `internal/jobs/jobs.go`**

Add `"slices"` to the imports:

```go
import (
	"regexp"
	"slices"
	"time"

	"github.com/charmbracelet/x/ansi"
)
```

Insert just above `// Finish records how a job's run ended.`:

```go
// Pending is the unfinished job (queued, running or waiting) with exactly
// args, or nil.
func (q *Queue) Pending(args []string) *Job {
	for _, j := range q.jobs {
		if !j.Finished() && slices.Equal(j.Args, args) {
			return j
		}
	}
	return nil
}

```

- [ ] **Step 5: Count tap packages by their full name, in `internal/ui/discover.go`**

Replace:

```go
	for _, p := range pkgs {
		t.have[entryKey(p.Name, p.Kind)] = true
	}
```

with:

```go
	for _, p := range pkgs {
		t.have[entryKey(p.Name, p.Kind)] = true
		// brew deps names a tap's packages in full: user/tap/foo.
		t.have[entryKey(p.FullName, p.Kind)] = true
	}
```

- [ ] **Step 6: Say "already queued" in `internal/ui/install.go`**

In `askInstall`, replace:

```go
		return a.note(r.Name + " is already installed")
	}
```

with:

```go
		return a.note(r.Name + " is already installed")
	}
	if a.queue.Pending(kindArgs("install", r.Kind, r.Name)) != nil {
		return a.note(r.Name + " is already queued")
	}
```

- [ ] **Step 7: The same for a typed install, in `internal/ui/cmdline.go`**

In `execute`, replace:

```go
	refresh, all := refreshAfter(args)
	return a.run(
```

with:

```go
	if args[0] == "install" && a.queue.Pending(args) != nil {
		return a.note(strings.Join(args, " ") + " is already queued")
	}
	refresh, all := refreshAfter(args)
	return a.run(
```

- [ ] **Step 8: Run the tests**

Run: `go test -race ./internal/jobs/ ./internal/ui/`
Expected: `ok` for both.

- [ ] **Step 9: Commit**

```bash
gofmt -l internal
git add internal/jobs/jobs.go internal/jobs/jobs_test.go internal/ui/discover.go internal/ui/install.go internal/ui/cmdline.go internal/ui/install_test.go
git commit -m "fix(ui): tap dependencies count as installed; an install already queued isn't asked again"
```

---

### Task 2: Discover says why analytics failed, and remembers a failed fetch across restarts

**Files:**
- Modify: `internal/analytics/store.go`
- Modify: `internal/analytics/store_test.go`
- Modify: `internal/ui/stats.go`
- Modify: `internal/ui/discover.go`
- Modify: `internal/ui/stats_test.go`

**Interfaces:**
- **Consumes:** `Store.Fetch`, `discoverTab.empty`, `App.fetchStats`, `App.gotCachedStats`.
- **Produces:**
  - `analytics.StatusError{List string; Code int}` (pointer receiver `Error()`: `"<list>: 404 Not Found"`), returned by `Fetch` for a status other than 200.
  - `analytics.ErrBadList`, wrapped by `Fetch` when the body doesn't parse: `"<list>: not an analytics list: <parse error>"`.
  - `(Store) Tried() string` and `(Store) SetTried(day string) error`, kept in `<Dir>/tried`.
  - `ui.Analytics` gains `Tried() string` and `SetTried(day string) error`.
  - `fetchFailure(err error) string` in `ui`.

- [ ] **Step 1: Update `internal/analytics/store_test.go`**

Add `"errors"` to the imports:

```go
import (
	"context"
	"errors"
	"net/http"
```

In `TestFetchFailureKeepsTheCache`, replace:

```go
		if name == "missing" && !strings.Contains(err.Error(), "404") {
			t.Errorf("missing: %v should give the status", err)
		}
```

with:

```go
		var status *StatusError
		if name == "missing" && (!errors.As(err, &status) || status.Code != http.StatusNotFound || !strings.Contains(err.Error(), "404")) {
			t.Errorf("missing: %v should give the status", err)
		}
		if got := errors.Is(err, ErrBadList); got != (name == "broken") {
			t.Errorf("%s: errors.Is(%v, ErrBadList) = %v", name, err, got)
		}
```

Append:

```go

func TestTried(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "analytics"), "dev")
	if got := s.Tried(); got != "" {
		t.Errorf("nothing tried yet, got %q", got)
	}
	if err := s.SetTried("2026-10-16"); err != nil {
		t.Fatal(err)
	}
	if got := s.Tried(); got != "2026-10-16" {
		t.Errorf("got %q", got)
	}
}
```

- [ ] **Step 2: Update `internal/ui/stats_test.go`**

Add `"fmt"` and `"sync"` to the imports:

```go
import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"brewery/internal/analytics"
)
```

Replace the `fakeStats` type and its comment:

```go
// fakeStats serves the analytics fixtures. cached are the copies kept from
// earlier downloads, fetched at cachedAt; err makes downloads fail.
type fakeStats struct {
	fresh    map[string]analytics.List
	cached   map[string]analytics.List
	cachedAt time.Time
	err      error
	fetched  []string // lists downloaded, in order
}
```

with:

```go
// fakeStats serves the analytics fixtures. cached are the copies kept from
// earlier downloads, fetched at cachedAt; err makes downloads fail. tried
// is the day a fetch last started.
type fakeStats struct {
	fresh    map[string]analytics.List
	cached   map[string]analytics.List
	cachedAt time.Time
	err      error
	fetched  []string // lists downloaded, in order
	mu       sync.Mutex
	tried    string
}
```

Insert above `// newStatsApp is a Discover app with analytics to read, still on Home.`:

```go
func (f *fakeStats) Tried() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tried
}

func (f *fakeStats) SetTried(day string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tried = day
	return nil
}

```

At the end of `TestDiscoverFetchesOnceADay`, replace:

```go
		t.Errorf("fetched %v: a failed fetch isn't retried by opening Discover again the same day", st.fetched)
	}
}
```

with:

```go
		t.Errorf("fetched %v: a failed fetch isn't retried by opening Discover again the same day", st.fetched)
	}
	if st.tried != "2026-10-13" {
		t.Errorf("tried %q: the day is kept for the next start", st.tried)
	}

	// The next start the same day remembers the failed try.
	a, st2 := newStatsApp(t)
	st2.tried, st2.err = st.tried, st.err
	keys(t, a, "6")
	if len(st2.fetched) != 0 {
		t.Errorf("fetched %v after a restart, though a fetch failed earlier today", st2.fetched)
	}
	keys(t, a, "r")
	if len(st2.fetched) != 1 {
		t.Errorf("r fetched %v: it fetches anyway", st2.fetched)
	}

	// A try on an earlier day doesn't count.
	a, st3 := newStatsApp(t)
	st3.tried = "2026-10-12"
	keys(t, a, "6")
	if len(st3.fetched) != 5 {
		t.Errorf("fetched %v, want every list", st3.fetched)
	}
}

func TestDiscoverSaysWhyTheFetchFailed(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{errors.New("dial tcp: no route to host"), "couldn't reach formulae.brew.sh · r retries"},
		{&analytics.StatusError{List: analytics.Tools30, Code: 404}, "formulae.brew.sh answered 404 · r retries"},
		{fmt.Errorf("%s: %w: eof", analytics.Tools30, analytics.ErrBadList), "couldn't read formulae.brew.sh's lists · r retries"},
	} {
		a, st := newStatsApp(t)
		st.err = c.err
		keys(t, a, "6")
		if out := stripANSI(a.render()); !strings.Contains(out, c.want) {
			t.Errorf("%v: want %q in\n%s", c.err, c.want, out)
		}
	}
}
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `go test ./internal/analytics/ ./internal/ui/ -run 'TestFetchFailure|TestTried|TestDiscover'`
Expected: FAIL. `analytics` doesn't compile (`undefined: StatusError`).

- [ ] **Step 4: Typed errors and the tried day, in `internal/analytics/store.go`**

Add `"errors"` to the imports:

```go
import (
	"context"
	"errors"
	"fmt"
```

Insert above `// Store downloads analytics lists and keeps the last copy of each in Dir.`:

```go
// StatusError is a list formulae.brew.sh answered with a status other
// than 200 OK.
type StatusError struct {
	List string
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s: %d %s", e.List, e.Code, http.StatusText(e.Code))
}

// ErrBadList is a list that came back but isn't one: an error page, or
// JSON in another shape.
var ErrBadList = errors.New("not an analytics list")

```

In `Fetch`, replace:

```go
		return List{}, fmt.Errorf("%s: %s", list, resp.Status)
```

with:

```go
		return List{}, &StatusError{List: list, Code: resp.StatusCode}
```

and replace:

```go
	l, err := Parse(b)
	if err != nil {
		return List{}, err
	}
	_ = s.save
```

with:

```go
	l, err := Parse(b)
	if err != nil {
		return List{}, fmt.Errorf("%s: %w: %v", list, ErrBadList, err)
	}
	_ = s.save
```

Insert above `// save writes the copy of list atomically, so a crash midway never leaves`:

```go
// triedFile holds the day a fetch last started.
const triedFile = "tried"

// Tried is the day a fetch last started, as 2006-01-02, or "" if none
// is known.
func (s Store) Tried() string {
	b, err := os.ReadFile(filepath.Join(s.Dir, triedFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// SetTried records day (2006-01-02) as the day a fetch last started, so a
// fetch that failed isn't tried again by every start that day.
func (s Store) SetTried(day string) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, triedFile), []byte(day+"\n"), 0o644)
}

```

- [ ] **Step 5: Read and keep the tried day, in `internal/ui/stats.go`**

Replace the `Analytics` interface:

```go
type Analytics interface {
	Cached(list string) (analytics.List, time.Time, error)
	Fetch(ctx context.Context, list string) (analytics.List, error)
}
```

with:

```go
type Analytics interface {
	Cached(list string) (analytics.List, time.Time, error)
	Fetch(ctx context.Context, list string) (analytics.List, error)
	Tried() string // the day a fetch last started, as 2006-01-02
	SetTried(day string) error
}
```

Replace:

```go
	cachedStatsMsg struct {
		lists map[string]analytics.List
		at    map[string]time.Time
	}
```

with:

```go
	cachedStatsMsg struct {
		lists map[string]analytics.List
		at    map[string]time.Time
		tried string // the day a fetch last started, in this run or an earlier one
	}
```

In `wantStats`, replace:

```go
			m := cachedStatsMsg{lists: map[string]analytics.List{}, at: map[string]time.Time{}}
```

with:

```go
			m := cachedStatsMsg{lists: map[string]analytics.List{}, at: map[string]time.Time{}, tried: an.Tried()}
```

In `gotCachedStats`, replace:

```go
	d := &a.discover
	for name, l := range m.lists {
```

with:

```go
	d := &a.discover
	if d.triedOn == "" {
		d.triedOn = m.tried
	}
	for name, l := range m.lists {
```

Replace the comment above `fetchStats`:

```go
// fetchStats downloads every list, one after another, unless the copies
// are all from today or a fetch already started today. force (r) fetches
// anyway. Only one fetch runs at a time.
```

with:

```go
// fetchStats downloads every list, one after another, unless the copies
// are all from today or a fetch already started today, in this run or an
// earlier one. force (r) fetches anyway. Only one fetch runs at a time.
```

and in `fetchStats`, replace:

```go
	d.fetching, d.triedOn, d.fetchErr = true, today, nil
	return a.fetchList(0)
```

with:

```go
	d.fetching, d.triedOn, d.fetchErr = true, today, nil
	an := a.analytics
	remember := func() tea.Msg {
		_ = an.SetTried(today) // without it, the next start may fetch again today
		return nil
	}
	return tea.Batch(a.fetchList(0), remember)
```

- [ ] **Step 6: Say why, in `internal/ui/discover.go`**

Add `"errors"` and `"fmt"` to the imports:

```go
import (
	"context"
	"errors"
	"fmt"
	"slices"
```

In `empty`, replace:

```go
	case t.fetchErr != nil:
		return "couldn't reach formulae.brew.sh · r retries"
```

with:

```go
	case t.fetchErr != nil:
		return fetchFailure(t.fetchErr) + " · r retries"
```

Insert above `// rankCell is a row's place in a ranked view, blank for none.`:

```go
// fetchFailure says why the analytics couldn't be fetched.
func fetchFailure(err error) string {
	var status *analytics.StatusError
	switch {
	case errors.As(err, &status):
		return fmt.Sprintf("formulae.brew.sh answered %d", status.Code)
	case errors.Is(err, analytics.ErrBadList):
		return "couldn't read formulae.brew.sh's lists"
	}
	return "couldn't reach formulae.brew.sh"
}

```

- [ ] **Step 7: Run the tests**

Run: `go test -race ./internal/analytics/ ./internal/ui/ && go build ./...`
Expected: `ok` for both, and the build passes (`var _ Analytics = analytics.Store{}` in `stats.go` checks the store still fits).

- [ ] **Step 8: Commit**

```bash
gofmt -l internal
git add internal/analytics/store.go internal/analytics/store_test.go internal/ui/stats.go internal/ui/discover.go internal/ui/stats_test.go
git commit -m "fix: Discover says why analytics failed, and remembers a failed fetch across restarts"
```

---

### Task 3: Sizes keep what was measured in time; brew's folders are asked once; only sizes copy the package list

**Files:**
- Modify: `internal/brew/sizes.go`
- Modify: `internal/brew/sizes_test.go`
- Modify: `internal/brew/client.go`
- Modify: `internal/brew/client_test.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/source.go`
- Modify: `internal/ui/sizes_test.go`

**Interfaces:**
- **Consumes:** `Client.query`, `dirSize`, `App.apply`, `App.request`, `fetch`.
- **Produces:**
  - `(*Client) Sizes` returns the map of packages fully measured so far together with the error when it stops early.
  - `(*Client) dataDirs(ctx) (cache, repo string, err error)`: `brew --cache` and `brew --repository`, trimmed, asked once per `Client` and asked again after a failure.
  - `Client.measured func(Package)`: unexported test hook, called after each package is measured.
  - `App.apply` merges a partial sizes result over the old sizes and clears its error.

- [ ] **Step 1: Append to `internal/brew/sizes_test.go`**

```go

func TestSizesKeepsWhatItMeasuredInTime(t *testing.T) {
	prefix := t.TempDir()
	write(t, filepath.Join(prefix, "Cellar", "fd", "1", "f"), 100)
	write(t, filepath.Join(prefix, "Cellar", "git", "1", "f"), 200)
	c := NewClient(fakeRunner{})
	c.Prefix, c.Timeout = prefix, time.Minute
	ctx, cancel := context.WithCancel(context.Background())
	// Time runs out once fd is measured: the first file of git's walk sees it.
	pkgs := []Package{{Name: "fd", FullName: "fd"}, {Name: "git", FullName: "git"}}
	c.measured = func(p Package) {
		if p.Name == "fd" {
			cancel()
		}
	}
	got, err := c.Sizes(ctx, pkgs)
	if err == nil {
		t.Error("want the error that stopped it")
	}
	if len(got) != 1 || got["formula/fd"] != 100 {
		t.Errorf("got %v, want fd alone", got)
	}
}
```

- [ ] **Step 2: Update `internal/brew/client_test.go`**

Add `"sync"` to the imports:

```go
	"strings"
	"sync"
	"testing"
	"time"
```

Append:

```go

// countingRunner counts each command it runs.
type countingRunner struct {
	fakeRunner
	mu sync.Mutex
	n  map[string]int
}

func (c *countingRunner) Run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	c.mu.Lock()
	c.n[strings.Join(args, " ")]++
	c.mu.Unlock()
	return c.fakeRunner.Run(ctx, args...)
}

func TestClientDataUpdatedAsksForDirsOnce(t *testing.T) {
	cache := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cache, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "api", "formula_names.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r := &countingRunner{n: map[string]int{}, fakeRunner: fakeRunner{
		out: map[string][]byte{"--cache": []byte(cache + "\n"), "--repository": []byte(t.TempDir() + "\n")},
		err: map[string]error{"--repository": errors.New("exit status 1")},
	}}
	c := NewClient(r)
	if _, err := c.DataUpdated(context.Background()); err == nil {
		t.Fatal("brew --repository failed: want an error")
	}
	delete(r.err, "--repository")
	for range 3 {
		if _, err := c.DataUpdated(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if r.n["--cache"] != 2 || r.n["--repository"] != 2 {
		t.Errorf("ran %v: a failure asks again, then the dirs are kept", r.n)
	}
}
```

- [ ] **Step 3: Update `internal/ui/sizes_test.go`**

Add `"context"` and `"maps"` to the imports:

```go
import (
	"context"
	"maps"
	"path/filepath"
```

Append:

```go

func TestSizesKeepWhatWasMeasuredInTime(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	before := maps.Clone(a.snap.Sizes)
	if len(before) < 2 {
		t.Fatalf("the fixture needs sizes: %v", before)
	}
	a.Update(resultMsg{q: qSizes, sizes: map[string]int64{"formula/fd": 7}, err: context.DeadlineExceeded})
	if a.errs[qSizes] != nil {
		t.Errorf("a partial measure is no failure: %v", a.errs[qSizes])
	}
	for k, n := range before {
		want := n
		if k == "formula/fd" {
			want = 7
		}
		if a.snap.Sizes[k] != want {
			t.Errorf("%s = %d, want %d", k, a.snap.Sizes[k], want)
		}
	}

	a.Update(resultMsg{q: qSizes, err: context.DeadlineExceeded}) // nothing measured in time
	if a.errs[qSizes] == nil || a.snap.Sizes["formula/fd"] != 7 {
		t.Errorf("err %v, sizes %v: nothing measured is a failure, and the sizes stay", a.errs[qSizes], a.snap.Sizes)
	}
}
```

- [ ] **Step 4: Run the tests to see them fail**

Run: `go test ./internal/brew/ ./internal/ui/ -run 'Sizes|DataUpdated'`
Expected: FAIL. `brew` doesn't compile (`c.measured undefined`), and `ui` says "a partial measure is no failure: context deadline exceeded".

- [ ] **Step 5: Return what was measured, in `internal/brew/sizes.go`**

Replace the comment above `Sizes`:

```go
// Sizes measures the disk space each package uses, keyed by Package.Key: a
// formula's kegs in the Cellar, or a cask's files (its app in /Applications,
// fonts…) and its Caskroom folder. It walks the files itself rather than
// asking brew, which has no such command.
```

with:

```go
// Sizes measures the disk space each package uses, keyed by Package.Key: a
// formula's kegs in the Cellar, or a cask's files (its app in /Applications,
// fonts…) and its Caskroom folder. It walks the files itself rather than
// asking brew, which has no such command. If it runs out of time, it
// returns the packages measured so far with the error.
```

Replace:

```go
			size, err := dirSize(ctx, path)
			if err != nil {
				return nil, err
			}
```

with:

```go
			size, err := dirSize(ctx, path)
			if err != nil {
				return sizes, err // p is half measured: leave it out
			}
```

and replace:

```go
		sizes[p.Key()] = n
	}
```

with:

```go
		sizes[p.Key()] = n
		if c.measured != nil {
			c.measured(p)
		}
	}
```

- [ ] **Step 6: Ask for brew's folders once, in `internal/brew/client.go`**

Add `"sync"` to the imports:

```go
	"strings"
	"sync"
	"syscall"
	"time"
```

Replace the `Client` type:

```go
// Client runs read-only brew queries.
type Client struct {
	r       Runner
	Timeout time.Duration
	Prefix  string // where brew is installed (/opt/homebrew): Cellar and Caskroom are in it
}
```

with:

```go
// Client runs read-only brew queries.
type Client struct {
	r       Runner
	Timeout time.Duration
	Prefix  string // where brew is installed (/opt/homebrew): Cellar and Caskroom are in it

	dirsMu      sync.Mutex
	cache, repo string // brew --cache and --repository, once asked

	measured func(Package) // tests: called after Sizes measures each package
}
```

In `DataUpdated`, replace:

```go
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
```

with:

```go
	cache, repo, err := c.dataDirs(ctx)
	if err != nil {
		return time.Time{}, err
	}
	var newest time.Time
	for _, f := range dataFiles {
		dir := repo
		if f.inCache {
			dir = cache
		}
```

Append to the file:

```go

// dataDirs are brew's cache and repository. They don't move while brewery
// runs, so brew is asked once; a failure is asked again next time.
func (c *Client) dataDirs(ctx context.Context) (cache, repo string, err error) {
	c.dirsMu.Lock()
	defer c.dirsMu.Unlock()
	if c.cache != "" && c.repo != "" {
		return c.cache, c.repo, nil
	}
	cb, err := c.query(ctx, "--cache")
	if err != nil {
		return "", "", err
	}
	rb, err := c.query(ctx, "--repository")
	if err != nil {
		return "", "", err
	}
	c.cache, c.repo = strings.TrimSpace(string(cb)), strings.TrimSpace(string(rb))
	return c.cache, c.repo, nil
}
```

- [ ] **Step 7: Merge partial sizes, and copy packages only for sizes, in `internal/ui/app.go`**

Add `"maps"` to the imports:

```go
import (
	"context"
	"fmt"
	"maps"
	"os/exec"
```

In `request`, replace:

```go
		cmds = append(cmds, fetch(a.ctx, a.src, q, slices.Clone(a.snap.Packages)))
```

with:

```go
		var pkgs []brew.Package
		if q == qSizes {
			pkgs = slices.Clone(a.snap.Packages) // measured in the background
		}
		cmds = append(cmds, fetch(a.ctx, a.src, q, pkgs))
```

Replace the start of `apply`:

```go
func (a *App) apply(m resultMsg) {
	a.errs[m.q] = m.err
```

with:

```go
func (a *App) apply(m resultMsg) {
	if m.q == qSizes && m.err != nil && len(m.sizes) > 0 {
		// Measuring ran out of time: keep what it measured, and the older
		// sizes for the rest, which the next run measures again.
		merged := make(map[string]int64, len(a.snap.Sizes)+len(m.sizes))
		maps.Copy(merged, a.snap.Sizes)
		maps.Copy(merged, m.sizes)
		m.sizes, m.err = merged, nil
	}
	a.errs[m.q] = m.err
```

- [ ] **Step 8: Update `fetch`'s comment in `internal/ui/source.go`**

Replace:

```go
// any brew process still running. Sizes are measured for pkgs.
```

with:

```go
// any brew process still running. Sizes are measured for pkgs, which
// only qSizes is given.
```

- [ ] **Step 9: Run the tests**

Run: `go test -race ./internal/brew/ ./internal/ui/`
Expected: `ok` for both.

- [ ] **Step 10: Commit**

```bash
gofmt -l internal
git add internal/brew/sizes.go internal/brew/sizes_test.go internal/brew/client.go internal/brew/client_test.go internal/ui/app.go internal/ui/source.go internal/ui/sizes_test.go
git commit -m "fix: sizes keep what was measured in time; brew's dirs asked once; only sizes copy the package list"
```

---

### Task 4: The selected row keeps one text color after a tint; Services STATUS fits exit 127; `--no-icons` help

**Files:**
- Modify: `internal/theme/theme.go`
- Modify: `internal/theme/theme_test.go`
- Modify: `internal/ui/list.go`
- Modify: `internal/ui/list_test.go`
- Modify: `cmd/brewery/main.go`

**Interfaces:**
- **Consumes:** `theme.New`, `theme.With`, `theme.Nerd`, `serviceCols`.
- **Produces:** `Styles.TableSelected` with no foreground; `serviceCols` STATUS `{"STATUS", 18, 10}`.

- [ ] **Step 1: Update `internal/theme/theme_test.go`**

Add Lip Gloss to the imports:

```go
import (
	"fmt"
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)
```

Insert above `func TestTintResetsOnlyTheForeground(t *testing.T) {`:

```go
func TestSelectedRowKeepsTheTerminalText(t *testing.T) {
	// Tint resets to the terminal's text color, so the selected row must use
	// it too, or the cells after a tinted one would change color.
	if c := New(Macchiato).TableSelected.GetForeground(); c != (lipgloss.NoColor{}) {
		t.Errorf("TableSelected has foreground %v", c)
	}
}

```

- [ ] **Step 2: Append to `internal/ui/list_test.go`**

```go

func TestServiceStatusFitsWithAnIcon(t *testing.T) {
	widest := theme.With(theme.Nerd.Failed, "error (exit 127)")
	if w := lipgloss.Width(widest); w > serviceCols[1].width {
		t.Errorf("%q is %d wide; STATUS has %d", widest, w, serviceCols[1].width)
	}
}
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `go test ./internal/theme/ ./internal/ui/ -run 'SelectedRowKeeps|ServiceStatusFits'`
Expected: FAIL with `TableSelected has foreground {202 211 245 255}` and `"\U000f0028 error (exit 127)" is 18 wide; STATUS has 16`.

- [ ] **Step 4: Drop the selected row's foreground, in `internal/theme/theme.go`**

Replace:

```go
		// No colors on cells: a cell's closing reset would cut the selected
		// row's highlight short. Color inside a cell goes through Tint.
		TableCell:     lipgloss.NewStyle().Padding(0, 1),
		TableSelected: lipgloss.NewStyle().Foreground(p.Text).Background(p.Surface0).Bold(true),
```

with:

```go
		// No colors on cells: a cell's closing reset would cut the selected
		// row's highlight short. Color inside a cell goes through Tint.
		TableCell: lipgloss.NewStyle().Padding(0, 1),
		// The selected row keeps the terminal's text color, as the other rows
		// do: Tint goes back to it, so text after a tinted cell matches.
		TableSelected: lipgloss.NewStyle().Background(p.Surface0).Bold(true),
```

- [ ] **Step 5: Widen Services STATUS, in `internal/ui/list.go`**

Replace:

```go
	serviceCols  = []col{{"NAME", 0, 16}, {"STATUS", 16, 10}}
```

with:

```go
	serviceCols  = []col{{"NAME", 0, 16}, {"STATUS", 18, 10}} // fits "error (exit 127)" after an icon
```

- [ ] **Step 6: Fix the `--no-icons` help, in `cmd/brewery/main.go`**

Replace:

```go
	noIcons := fs.Bool("no-icons", false, "draw plain text instead of Nerd Font icons (or set BREWERY_NO_ICONS=1)")
```

with:

```go
	noIcons := fs.Bool("no-icons", false, "draw plain text instead of Nerd Font icons (or set BREWERY_NO_ICONS to any value)")
```

- [ ] **Step 7: Run the tests**

Run: `go test -race ./...`
Expected: all `ok`. (`TestSelectedRowStaysHighlighted` still finds one reset at the end of the selected row.)

- [ ] **Step 8: Commit**

```bash
gofmt -l internal cmd
git add internal/theme/theme.go internal/theme/theme_test.go internal/ui/list.go internal/ui/list_test.go cmd/brewery/main.go
git commit -m "fix: the selected row keeps one text color after a tint; Services STATUS fits exit 127; --no-icons help"
```

---

### Task 5: The list behind a lone preview ignores its keys; previews start on the list; help closes when brew asks

**Files:**
- Modify: `internal/ui/list.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/discover.go`
- Modify: `internal/ui/split_test.go`
- Modify: `internal/ui/help_test.go`
- Modify: `internal/ui/installed_test.go`

**Interfaces:**
- **Consumes:** `listTab.previewAlone`, `listTab.pvOpen`, `App.handleKey`, `App.showTab`, `App.typeIntoJob`.
- **Produces:**
  - `(*listTab) previewShown() bool`: `pvOpen && previewAlone()`.
  - `listTab.resize` clears `pvOpen` whenever the preview isn't alone.
  - `tab`/`enter` on a list whose preview would stand alone only open it; with the preview shown, `handleKey` closes it on `tab`/`enter` and ignores every other non-global key.
  - `showTab` drops preview focus on Installed and Discover when the tab changes.
  - `typeIntoJob` closes help.

- [ ] **Step 1: Append to `internal/ui/split_test.go`**

```go

func TestServicesPreviewAloneIgnoresListKeys(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	keys(t, a, "4")
	before := a.services.cursor()
	keys(t, a, "enter", "j", "space", "S")
	if a.services.cursor() != before || a.marks() != 0 || a.confirm != nil || len(fs.calls) != 0 {
		t.Errorf("keys reached the hidden list: cursor %d→%d, marks %d, confirm %v, calls %v",
			before, a.services.cursor(), a.marks(), a.confirm, fs.calls)
	}
	if !a.services.previewShown() {
		t.Fatal("the preview should still show")
	}
	keys(t, a, "tab")
	if out := stripANSI(a.render()); !strings.Contains(out, "STATUS") {
		t.Errorf("tab goes back to the list:\n%s", out)
	}
	keys(t, a, "enter", "2")
	if a.tab != tabInstalled {
		t.Errorf("tab keys still work under the preview: on %d", a.tab)
	}
}

func TestServicesPreviewFlagClearsWhenThereIsRoom(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	keys(t, a, "4", "enter")
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40}) // room beside the list
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})  // and narrow again
	if out := stripANSI(a.render()); !strings.Contains(out, "STATUS") || strings.Contains(out, "End of the log") {
		t.Errorf("after shrinking again, the list shows first:\n%s", out)
	}
}
```

- [ ] **Step 2: Append to `internal/ui/help_test.go`**

```go

func TestHelpClosesWhenBrewAsks(t *testing.T) {
	p := newFakeProc(0)
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "install x", args: []string{"install", "x"}}))
	keys(t, a, "?")
	p.out <- []byte("Password:")
	waiting = resume(t, a, waiting)
	if a.help || a.mode != modeInsert {
		t.Fatalf("help %v, mode %v: brew's question closes help", a.help, a.mode)
	}
	keys(t, a, "s")
	if got := p.input(); got != "s" {
		t.Errorf("brew got %q: the first key of the answer must reach it", got)
	}
	p.exit()
	resume(t, a, waiting)
}
```

- [ ] **Step 3: Append to `internal/ui/installed_test.go`**

```go

func TestPreviewFocusIsLeftBehind(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "2", "tab")
	if !a.installed.pv.focus {
		t.Fatal("tab should focus the preview")
	}
	keys(t, a, "3", "2")
	if a.installed.pv.focus {
		t.Error("coming back to Installed starts on the list")
	}
}
```

- [ ] **Step 4: Run the tests to see them fail**

Run: `go test ./internal/ui/ -run 'PreviewAlone|FlagClears|ClosesWhenBrewAsks|FocusIsLeftBehind'`
Expected: FAIL to compile (`a.services.previewShown undefined`).

- [ ] **Step 5: `previewShown`, and forget a stale flag, in `internal/ui/list.go`**

In `update`, replace:

```go
		t.pvOpen = !t.pvOpen // the list stands alone: the preview takes turns with it
```

with:

```go
		t.pvOpen = true // the list stands alone: the preview takes turns with it
```

Replace the start of `resize`:

```go
func (t *listTab) resize(w, h int) {
	t.w, t.h = w, h
```

with:

```go
// previewShown reports whether the preview stands in for the list.
func (t *listTab) previewShown() bool { return t.pvOpen && t.previewAlone() }

func (t *listTab) resize(w, h int) {
	t.w, t.h = w, h
	if !t.previewAlone() {
		t.pvOpen = false // the preview has room beside the list: a later shrink starts on the list
	}
```

- [ ] **Step 6: Route keys and close help, in `internal/ui/app.go`**

Replace `typeIntoJob` and its comment:

```go
// typeIntoJob switches to INSERT with the drawer open, so keys go to the
// running job. A g pressed just before is forgotten: after INSERT, the next
// key mustn't complete a gg or gx.
func (a *App) typeIntoJob() {
	a.mode, a.drawer.open, a.gPending = modeInsert, true, false
}
```

with:

```go
// typeIntoJob switches to INSERT with the drawer open, so keys go to the
// running job. A g pressed just before is forgotten: after INSERT, the next
// key mustn't complete a gg or gx. Help closes, so it can't eat the first
// key of the answer.
func (a *App) typeIntoJob() {
	a.mode, a.drawer.open, a.gPending, a.help = modeInsert, true, false, false
}
```

In `handleKey`'s `esc` case, replace:

```go
		case a.tab == tabServices && a.services.pvOpen && a.services.previewAlone():
```

with:

```go
		case a.tab == tabServices && a.services.previewShown():
```

and replace:

```go
	if cmd, ok := a.actionKey(k); ok {
		return cmd
	}
```

with:

```go
	if a.tab == tabServices && !a.jobsOpen && a.services.previewShown() {
		// The list is hidden behind the preview: its keys would act on rows
		// you can't see. tab and enter go back to it.
		if k == "tab" || k == "enter" {
			a.services.pvOpen = false
		}
		return nil
	}
	if cmd, ok := a.actionKey(k); ok {
		return cmd
	}
```

- [ ] **Step 7: Drop preview focus when leaving a tab, in `internal/ui/discover.go`**

Replace:

```go
func (a *App) showTab(t int) tea.Cmd {
	a.tab = t
```

with:

```go
func (a *App) showTab(t int) tea.Cmd {
	if t != a.tab {
		// Coming back to a tab starts on its list, not a preview left focused.
		a.installed.pv.focus, a.discover.pv.focus = false, false
	}
	a.tab = t
```

- [ ] **Step 8: Run the tests**

Run: `go test -race ./internal/ui/`
Expected: `ok`. The existing `TestServicesPreviewTakesTurnsWhenNarrow`, `TestServicesPreviewTakesTurnsWhenShort` and `TestStalePreviewFlagDoesNotSwallowEsc` still pass.

- [ ] **Step 9: Commit**

```bash
gofmt -l internal
git add internal/ui/list.go internal/ui/app.go internal/ui/discover.go internal/ui/split_test.go internal/ui/help_test.go internal/ui/installed_test.go
git commit -m "fix(ui): the list behind a lone preview ignores its keys; previews start on the list; help closes when brew asks"
```

---

### Task 6: Crash logs catch the runtime's fatal errors, keep the newest five, and survive a run that died

**Files:**
- Modify: `cmd/brewery/crash.go` (rewritten)
- Modify: `cmd/brewery/crash_test.go` (rewritten)
- Modify: `cmd/brewery/main.go`
- Modify: `go.mod` (`go mod tidy` makes `golang.org/x/sys` direct)

**Interfaces:**
- **Consumes:** `jobs.DefaultStateDir`.
- **Produces:**
  - `startCrashLog(dir string) (*crashLog, error)`: adopts leftovers, then points `os.Stderr` and fd 2 at `.crash-<pid>-*.log` in `dir`.
  - `(*crashLog) stop(now time.Time) string`: puts both back; returns the crash log's path, or "".
  - `crashLog.earlier []string`: crash logs adopted from runs that died.
  - `keepCrashes = 5`, `keepCrash`, `adoptLeftovers`, `running`.

- [ ] **Step 1: Replace `cmd/brewery/crash_test.go` with:**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

var crashAt = time.Date(2026, 10, 16, 15, 4, 5, 0, time.Local)

// crashFiles lists the names in dir.
func crashFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestCrashLogKeepsWhatWasWritten(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	real := os.Stderr
	c, err := startCrashLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if os.Stderr == real {
		t.Fatal("stderr should point at the crash file while the TUI runs")
	}
	fmt.Fprint(os.Stderr, "Caught panic:\r\n\r\nboom\r\n")
	// The runtime's fatal errors write to file descriptor 2 directly.
	unix.Write(2, []byte("fatal error: concurrent map writes\n"))
	path := c.stop(crashAt)
	if path != filepath.Join(dir, "crash-20261016-150405.log") {
		t.Fatalf("stop returned %q", path)
	}
	if os.Stderr != real {
		t.Error("stop must put stderr back")
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "Caught panic:\r\n\r\nboom\r\nfatal error: concurrent map writes\n" {
		t.Errorf("crash log = %q, %v", b, err)
	}
}

func TestCrashLogLeavesNothingWithoutACrash(t *testing.T) {
	dir := t.TempDir()
	c, err := startCrashLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path := c.stop(crashAt); path != "" {
		t.Errorf("nothing was written: no crash, got %q", path)
	}
	if left := crashFiles(t, dir); len(left) != 0 {
		t.Errorf("files left behind: %v", left)
	}
}

func TestCrashLogsKeepTheNewestFive(t *testing.T) {
	dir := t.TempDir()
	for i := range 7 {
		c, err := startCrashLog(dir)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "crash %d\n", i)
		c.stop(crashAt.Add(time.Duration(i) * time.Minute))
	}
	want := []string{
		"crash-20261016-150605.log", "crash-20261016-150705.log", "crash-20261016-150805.log",
		"crash-20261016-150905.log", "crash-20261016-151005.log",
	}
	if got := crashFiles(t, dir); !slices.Equal(got, want) {
		t.Errorf("kept %v, want %v", got, want)
	}
}

func TestCrashLogAdoptsWhatADeadRunLeft(t *testing.T) {
	dir := t.TempDir()
	// A run killed by a fatal error left its temp file; pid 1 is launchd,
	// whose pid stays taken, so a file naming it counts as in use.
	dead := filepath.Join(dir, ".crash-2147483646-1.log")
	live := filepath.Join(dir, ".crash-1-2.log")
	empty := filepath.Join(dir, ".crash-2147483645-3.log")
	for path, body := range map[string]string{dead: "fatal error: out of memory\n", live: "", empty: ""} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(dead, crashAt, crashAt); err != nil {
		t.Fatal(err)
	}
	c, err := startCrashLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	c.stop(crashAt)
	adopted := filepath.Join(dir, "crash-20261016-150405.log")
	if !slices.Equal(c.earlier, []string{adopted}) {
		t.Errorf("earlier = %v", c.earlier)
	}
	if b, err := os.ReadFile(adopted); err != nil || string(b) != "fatal error: out of memory\n" {
		t.Errorf("adopted log = %q, %v", b, err)
	}
	if got := crashFiles(t, dir); !slices.Equal(got, []string{".crash-1-2.log", "crash-20261016-150405.log"}) {
		t.Errorf("left %v: the empty leftover goes, the live one stays", got)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./cmd/brewery/`
Expected: FAIL to compile (`not enough arguments in call to c.stop`).

- [ ] **Step 3: Replace `cmd/brewery/crash.go` with:**

```go
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"golang.org/x/sys/unix"
)

// keepCrashes is how many crash logs are kept; older ones are removed.
const keepCrashes = 5

// crashLog keeps what brewery prints if it crashes. Bubble Tea catches a
// panic, restores the terminal, then writes the panic and its stack to
// os.Stderr; while the TUI runs, os.Stderr points at a file instead, so
// that report lands in a crash log. File descriptor 2 points there too, for
// the Go runtime's fatal errors (concurrent map writes, out of memory),
// which bypass os.Stderr and end the process at once. Nothing else writes
// to stderr while the TUI is up: it would garble the screen.
type crashLog struct {
	dir     string
	f       *os.File
	stderr  *os.File // the real stderr, put back by stop
	fd2     int      // a copy of the real file descriptor 2, put back by stop
	earlier []string // crash logs left by runs that died without stop
}

// startCrashLog points os.Stderr and file descriptor 2 at a new temp file
// in dir. A run killed by a fatal error never calls stop, so temp files
// left in dir by earlier runs become crash logs first (or are removed when
// empty), and earlier lists them.
func startCrashLog(dir string) (*crashLog, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	earlier := adoptLeftovers(dir)
	f, err := os.CreateTemp(dir, fmt.Sprintf(".crash-%d-*.log", os.Getpid()))
	if err != nil {
		return nil, err
	}
	fd2, err := unix.Dup(2)
	if err == nil {
		err = unix.Dup2(int(f.Fd()), 2)
		if err != nil {
			unix.Close(fd2)
		}
	}
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	c := &crashLog{dir: dir, f: f, stderr: os.Stderr, fd2: fd2, earlier: earlier}
	os.Stderr = f
	return c, nil
}

// stop puts stderr back. If anything was written, the file becomes a crash
// log named for now, and stop returns its path; otherwise the file is
// removed and stop returns "".
func (c *crashLog) stop(now time.Time) string {
	unix.Dup2(c.fd2, 2)
	unix.Close(c.fd2)
	os.Stderr = c.stderr
	info, err := c.f.Stat()
	c.f.Close()
	if err != nil || info.Size() == 0 {
		os.Remove(c.f.Name())
		return ""
	}
	return keepCrash(c.dir, c.f.Name(), now)
}

// keepCrash renames the temp file tmp to a crash log named for at, removes
// the oldest logs beyond keepCrashes, and returns where the log is.
func keepCrash(dir, tmp string, at time.Time) string {
	path := filepath.Join(dir, "crash-"+at.Format("20060102-150405")+".log")
	if err := os.Rename(tmp, path); err != nil {
		path = tmp // keep it where it is
	}
	logs, _ := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	slices.Sort(logs) // the names sort by time
	for _, old := range logs[:max(len(logs)-keepCrashes, 0)] {
		os.Remove(old)
	}
	return path
}

// adoptLeftovers turns temp files left by runs that never reached stop
// into crash logs, named for when they were last written, and returns
// them. Empty ones are removed. A temp file names its run's process, so
// one that another brewery is still using is left alone.
func adoptLeftovers(dir string) []string {
	tmps, _ := filepath.Glob(filepath.Join(dir, ".crash-*.log"))
	var kept []string
	for _, tmp := range tmps {
		if running(tmp) {
			continue
		}
		info, err := os.Stat(tmp)
		if err != nil {
			continue
		}
		if info.Size() == 0 {
			os.Remove(tmp)
			continue
		}
		kept = append(kept, keepCrash(dir, tmp, info.ModTime()))
	}
	return kept
}

// running reports whether the process named in the temp file's name
// (.crash-<pid>-….log) is still running.
func running(tmp string) bool {
	var pid int
	if _, err := fmt.Sscanf(filepath.Base(tmp), ".crash-%d-", &pid); err != nil || pid <= 0 {
		return false
	}
	err := unix.Kill(pid, 0)
	return err == nil || errors.Is(err, unix.EPERM)
}
```

- [ ] **Step 4: Use it from `cmd/brewery/main.go`**

Add `"time"` to the imports:

```go
	"os/exec"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
```

Replace:

```go
	crash, crashErr := startCrashLog(filepath.Join(jobs.DefaultStateDir(), "crash.log"))
	_, err = tea.NewProgram(app).Run()
	if crashErr == nil && crash.stop() {
		fmt.Fprintf(stderr, "brewery crashed. What happened is in %s; please include it when reporting this.\n", crash.path)
		return 1
	}
```

with:

```go
	crash, crashErr := startCrashLog(jobs.DefaultStateDir())
	_, err = tea.NewProgram(app).Run()
	if crashErr == nil {
		if path := crash.stop(time.Now()); path != "" {
			fmt.Fprintf(stderr, "brewery crashed. What happened is in %s; please include it when reporting this.\n", path)
			return 1
		}
		for _, path := range crash.earlier {
			fmt.Fprintf(stderr, "brewery crashed last time. What happened is in %s; please include it when reporting this.\n", path)
		}
	}
```

- [ ] **Step 5: Tidy and run the tests**

Run: `go mod tidy && go test -race -count=1 ./cmd/... && go vet ./cmd/...`
Expected: `go.mod` moves `golang.org/x/sys` from the indirect block to the direct one; tests `ok`; vet prints nothing.

- [ ] **Step 6: Commit**

```bash
gofmt -l cmd
git add cmd/brewery/crash.go cmd/brewery/crash_test.go cmd/brewery/main.go go.mod go.sum
git commit -m "fix: crash logs catch the runtime's fatal errors, keep the newest five, and survive a run that died"
```

---

### Task 7: History saves fall back to check-then-rename without hard links

**Files:**
- Modify: `internal/jobs/history.go`
- Modify: `internal/jobs/history_test.go`

**Interfaces:**
- **Consumes:** `History.Save`, `record` and `t0` (test helpers in `history_test.go` and `jobs_test.go`).
- **Produces:**
  - `History.link func(oldname, newname string) error`: unexported; nil means `os.Link`.
  - `(History) publish(tmp, name string) error`: `fs.ErrExist` when `name` is taken.
  - `noHardLinks(err error) bool`: `EPERM`, `ENOTSUP`, `EOPNOTSUPP` or `EXDEV`.

- [ ] **Step 1: Update `internal/jobs/history_test.go`**

Add `"errors"` and `"syscall"` to the imports:

```go
import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"
)
```

Insert above `func TestHistoryFilesArePrivate(t *testing.T) {`:

```go
func TestHistoryWithoutHardLinks(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.EPERM, syscall.ENOTSUP, syscall.EOPNOTSUPP, syscall.EXDEV} {
		h := History{
			Dir:  filepath.Join(t.TempDir(), "jobs"),
			link: func(o, n string) error { return &os.LinkError{Op: "link", Old: o, New: n, Err: errno} },
		}
		for _, title := range []string{"one", "two"} {
			if err := h.Save(record(title, t0)); err != nil {
				t.Fatalf("%v: %v", errno, err)
			}
		}
		got, err := h.Load()
		if err != nil || len(got) != 2 {
			t.Errorf("%v: Load = %d records, %v; want 2", errno, len(got), err)
		}
		if left, _ := filepath.Glob(filepath.Join(h.Dir, "*.tmp")); len(left) != 0 {
			t.Errorf("%v: temp files left: %v", errno, left)
		}
	}
}

func TestHistoryLinkFailureIsAnError(t *testing.T) {
	h := History{
		Dir:  filepath.Join(t.TempDir(), "jobs"),
		link: func(o, n string) error { return &os.LinkError{Op: "link", Old: o, New: n, Err: syscall.EIO} },
	}
	if err := h.Save(record("one", t0)); !errors.Is(err, syscall.EIO) {
		t.Errorf("Save = %v, want the I/O error", err)
	}
	if left, _ := os.ReadDir(h.Dir); len(left) != 0 {
		t.Errorf("files left: %v", left)
	}
}

```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/jobs/`
Expected: FAIL to compile (`unknown field link in struct literal`).

- [ ] **Step 3: Fall back to rename, in `internal/jobs/history.go`**

Replace the imports with:

```go
import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)
```

Replace:

```go
// History stores finished jobs as one JSON file each in Dir, named by
// start time so they sort oldest first.
type History struct{ Dir string }
```

with:

```go
// History stores finished jobs as one JSON file each in Dir, named by
// start time so they sort oldest first.
type History struct {
	Dir  string
	link func(oldname, newname string) error // os.Link; tests make it fail
}
```

In `Save`, replace:

```go
	// Jobs that start within the clock's precision share a name. Linking
	// fails if the name is taken, so a concurrent save is never replaced:
	// try "-1", "-2", … instead. (The order among records with the same
	// start time is unspecified.)
	stem := filepath.Join(h.Dir, r.Started.UTC().Format("20060102T150405.000000000"))
	name := stem + ".json"
	for i := 1; ; i++ {
		err := os.Link(tmp, name)
		if err == nil {
```

with:

```go
	// Jobs that start within the clock's precision share a name. publish
	// fails if the name is taken, so another save is never replaced: try
	// "-1", "-2", … instead. (The order among records with the same start
	// time is unspecified.)
	stem := filepath.Join(h.Dir, r.Started.UTC().Format("20060102T150405.000000000"))
	name := stem + ".json"
	for i := 1; ; i++ {
		err := h.publish(tmp, name)
		if err == nil {
```

and replace:

```go
	os.Remove(tmp)
	files, err := h.files()
```

with:

```go
	os.Remove(tmp) // still there after a link; already gone after a rename
	files, err := h.files()
```

Insert above `// Load reads the history, newest first.`:

```go
// publish puts the file tmp at name, failing with fs.ErrExist if name is
// taken. A hard link does that atomically. Some file systems (network
// shares, FUSE) have no hard links: there it checks, then renames, which a
// save at the same instant could race.
func (h History) publish(tmp, name string) error {
	link := h.link
	if link == nil {
		link = os.Link
	}
	err := link(tmp, name)
	if !noHardLinks(err) {
		return err
	}
	switch _, err := os.Lstat(name); {
	case err == nil:
		return fs.ErrExist
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	return os.Rename(tmp, name)
}

// noHardLinks reports whether err says the file system can't make hard
// links.
func noHardLinks(err error) bool {
	for _, no := range []syscall.Errno{syscall.EPERM, syscall.ENOTSUP, syscall.EOPNOTSUPP, syscall.EXDEV} {
		if errors.Is(err, no) {
			return true
		}
	}
	return false
}

```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./internal/jobs/`
Expected: `ok`. `TestHistoryConcurrentSavesWithTheSameStartAllSurvive` still passes through the real `os.Link`.

- [ ] **Step 5: Commit**

```bash
gofmt -l internal
git add internal/jobs/history.go internal/jobs/history_test.go
git commit -m "fix(jobs): history saves fall back to check-then-rename without hard links"
```

---

### Task 8: Test gaps and a stale comment

**Files:**
- Modify: `internal/ui/discover_test.go`
- Modify: `internal/ui/list_test.go`

**Interfaces:**
- **Consumes:** `newDiscoverApp`, `newFakeStats`, `newListTab`, `serviceRows`.
- **Produces:** tests only.

- [ ] **Step 1: Open the `:` line by key, in `internal/ui/discover_test.go`**

In `TestOpeningDuringReadReadsOnce`, replace:

```go
	keys(t, a, "1", "6")
	a.wantCatalogue()
	drain(t, a, first)
```

with:

```go
	keys(t, a, "1", "6", ":", "esc") // the : line wants the catalogue too
	drain(t, a, first)
```

Append:

```go

func TestRefreshBeforeDiscoverOpens(t *testing.T) {
	a, cat := newDiscoverApp(t)
	st := newFakeStats(t)
	a.UseAnalytics(st)
	keys(t, a, "r")
	if cat.calls != 0 || len(st.fetched) != 0 {
		t.Errorf("r on Home read the catalogue %d times and fetched %v: Discover hasn't opened", cat.calls, st.fetched)
	}
	keys(t, a, "6")
	if cat.calls != 1 || len(st.fetched) != 5 {
		t.Errorf("opening Discover read the catalogue %d times and fetched %v", cat.calls, st.fetched)
	}
}
```

- [ ] **Step 2: Fix the widths comment, in `internal/ui/list_test.go`**

In `TestSizeTableDropsTrailingColumns`, replace:

```go
	// NAME 8 + INSTALLED 9 + LATEST 6 + padding 6 = 29; TYPE and PINNED don't fit.
```

with:

```go
	// NAME 8 + INSTALLED 8 + LATEST 8 + padding 6 = 30; TYPE and PINNED don't fit.
```

- [ ] **Step 3: The highlight test must find its row, in `internal/ui/list_test.go`**

In `TestSelectedRowStaysHighlighted`, replace:

```go
	lt.setRows(serviceRows(testStyles, fixtureSnapshot(t).Services))
	for _, line := range strings.Split(lt.table.View(), "\n") {
		if !strings.Contains(line, "postgresql@16") {
			continue
		}
```

with:

```go
	lt.setRows(serviceRows(testStyles, fixtureSnapshot(t).Services))
	found := false
	for _, line := range strings.Split(lt.table.View(), "\n") {
		if !strings.Contains(line, "postgresql@16") {
			continue
		}
		found = true
```

and replace the end of the test:

```go
			t.Errorf("the selected row resets %d times: %q", n, line)
		}
	}
}
```

with:

```go
			t.Errorf("the selected row resets %d times: %q", n, line)
		}
	}
	if !found {
		t.Errorf("the selected row isn't shown:\n%s", lt.table.View())
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./internal/ui/ -run 'RefreshBeforeDiscover|OpeningDuringRead|SelectedRowStays|SizeTableDrops' -v`
Expected: all four PASS. (These pin behaviour that already works, so there is no failing run first.)

- [ ] **Step 5: Commit**

```bash
gofmt -l internal
git add internal/ui/discover_test.go internal/ui/list_test.go
git commit -m "test: a refresh before Discover opens; the : line asks for the catalogue by key; highlight test finds its row"
```

---

### Task 9: End-to-end check

**Files:** none (verification only)

- [ ] **Step 1: Final checks**

```bash
go test -race -count=1 ./...
go test -tags live -count=1 ./internal/...
go vet ./... && go vet -tags live ./...
gofmt -l .
go mod tidy -diff
```

Expected: all `ok`, with no output from vet, gofmt or `go mod tidy -diff`.

- [ ] **Step 2: Drive the branch binary in a PTY** with absolute scratch `XDG_STATE_HOME` and `XDG_CACHE_HOME`:
  1. **At 50×24:** `4`, then `enter` shows the Services preview alone. `j`, `space` and `S` change nothing (no dialog). `tab` goes back to the list.
  2. **Resize:** `enter` again, widen to 120×30 (both show), narrow to 50×24: the list shows first.
  3. **At 120×30:** `2`, `tab` focuses the preview, `3` then `2`: the list is focused. `:jobs` shows earlier jobs, and the selected row's COMMAND, STARTED and TOOK are in the same color as the other rows.
  4. **Discover:** `6`. The analytics are fetched (or the title says why not), and `<scratch cache>/brewery/analytics/tried` holds today's date. Quit and start again: nothing is fetched until `r`.

  Help closing on a brew prompt is covered by `TestHelpClosesWhenBrewAsks`; a real prompt would need a real install, which testing must never do.

- [ ] **Step 3: Crash log, with a throwaway build** (don't commit it): add `panic("test crash")` as the first line of `(*App).Init`, `go build -o /tmp/<scratch>/brewery-crash ./cmd/brewery`, run it with the scratch XDG dirs, and check:
  - brewery exits 1 and prints "brewery crashed. What happened is in …/brewery/crash-<date>-<time>.log";
  - the file holds "test crash" and a stack;
  - no `.crash-*.log` is left in the state dir.
  Then `git checkout internal/ui` to drop the panic.
