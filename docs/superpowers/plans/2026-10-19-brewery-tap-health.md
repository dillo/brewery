# Tap Health Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Maintenance row, **Check taps**, that the user runs with `enter`. It asks GitHub whether each third-party tap's repository is fine, archived, moved or gone, and the results show on that row and in a new `GITHUB` column on Taps.

**Architecture:**
- A new package, `internal/github`, makes the one REST request per repository and parses a tap's remote.
- The UI reaches it through a `TapChecker` interface set with `UseTapChecker`, as Discover's analytics are, so tests use a fake.
- Results live in the snapshot cache (`TapChecks`, `TapsCheckedAt`; version 6).

**Tech Stack:** Go 1.26 (`net/http`, `net/http/httptest`), Bubble Tea v2, Lip Gloss v2, golangci-lint v2.15.0.

**Spec:** `docs/superpowers/specs/2026-10-19-brewery-tap-health-design.md`

## Global Constraints

- **Lint:**
  - `.golangci.yml` is fixed: gocognit and gocyclo `min-complexity: 20`, both excluded for `_test.go`. Don't edit it, and don't add `//nolint`.
  - `golangci-lint run ./...` must report `0 issues.` at the end of every task.
  - If a function you touch goes over a limit, split it into a named helper.
- **Coverage:** `make cover` stays at 90% or above in total and per package.
- **Network:** tests never touch the network. Use `httptest` in `internal/github` and a fake checker in `internal/ui`.
- **Goldens:**
  - Update with `go test ./internal/ui -run TestScreens -update -count=1` only in Task 2.
  - Read `git diff internal/ui/testdata/golden` first. Only the Maintenance screen may change, along with screens whose background shows it.
- **Style:**
  - Match the surrounding code's comment density, naming and idioms.
  - Every function and exported name gets a short doc comment in plain sentences.
- **Exact texts:** user-facing strings are exactly as written in this plan.
- **Commits:** one commit per task, conventional style, with the message given in the task.

## Commands

| Check | Command |
|---|---|
| Tests | `go test ./... -count=1` |
| One package | `go test ./internal/github -count=1` |
| Goldens | `go test ./internal/ui -run TestScreens -count=1` |
| Lint | `golangci-lint run ./...` |
| Coverage | `make cover` |

---

### Task 1: The GitHub client, and the snapshot fields

**Files:**
- Create: `internal/github/github.go`
- Create: `internal/github/github_test.go`
- Modify: `internal/state/snapshot.go` (two fields; `snapshotVersion` to 6)
- Test: `internal/state/snapshot_test.go` (one round-trip assertion)

**Interfaces:**
- Produces, for Task 2:
  - `github.State` with `Fine`, `Archived`, `Moved`, `Gone` and `Unknown`;
  - `github.TapCheck{Tap string; State State; MovedTo, Reason string}`;
  - `github.ErrRateLimited`;
  - `github.NewClient() *Client`;
  - `(*Client).Repo(ctx, owner, repo string) (TapCheck, error)`;
  - `github.RepoOf(remote string) (owner, repo string, ok bool)`;
  - `state.Snapshot.TapChecks []github.TapCheck` and `state.Snapshot.TapsCheckedAt time.Time`.

- [ ] **Step 1: Write the failing tests.** Create `internal/github/github_test.go`:

```go
package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeGitHub answers like GitHub's /repos endpoint for a few repositories,
// and records the last request's headers.
func fakeGitHub(t *testing.T, got *http.Header) *Client {
	t.Helper()
	mux := http.NewServeMux()
	answer := func(path, body string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			*got = r.Header.Clone()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
	}
	answer("/repos/o/fine", `{"full_name": "o/fine", "archived": false}`)
	answer("/repos/o/old", `{"full_name": "o/old", "archived": true}`)
	answer("/repositories/42", `{"full_name": "n/new", "archived": false}`)
	mux.HandleFunc("/repos/o/renamed", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/repositories/42", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/repos/o/limited", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/repos/o/broken", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	// Anything else, /repos/o/gone included, is a 404.
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient()
	c.BaseURL = srv.URL
	return c
}

func TestRepo(t *testing.T) {
	var hdr http.Header
	c := fakeGitHub(t, &hdr)
	for _, tt := range []struct {
		repo string
		want TapCheck
	}{
		{"fine", TapCheck{State: Fine}},
		{"old", TapCheck{State: Archived}},
		{"renamed", TapCheck{State: Moved, MovedTo: "n/new"}},
		{"gone", TapCheck{State: Gone}},
	} {
		got, err := c.Repo(context.Background(), "o", tt.repo)
		if err != nil || got != tt.want {
			t.Errorf("%s: %+v, %v; want %+v", tt.repo, got, err, tt.want)
		}
	}
	if hdr.Get("Accept") != "application/vnd.github+json" || hdr.Get("User-Agent") != "brewery" {
		t.Errorf("headers %v", hdr)
	}
	if _, err := c.Repo(context.Background(), "o", "limited"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("a 403 with no requests left is the rate limit: %v", err)
	}
	if _, err := c.Repo(context.Background(), "o", "broken"); err == nil || errors.Is(err, ErrRateLimited) {
		t.Errorf("a 500 is an error, not the rate limit: %v", err)
	}
}

func TestRepoOf(t *testing.T) {
	for _, tt := range []struct {
		remote, owner, repo string
		ok                  bool
	}{
		{"https://github.com/anomalyco/homebrew-tap.git", "anomalyco", "homebrew-tap", true},
		{"https://github.com/anomalyco/homebrew-tap", "anomalyco", "homebrew-tap", true},
		{"http://github.com/anomalyco/homebrew-tap", "", "", false},
		{"https://gitlab.com/someone/homebrew-tap.git", "", "", false},
		{"https://github.com/anomalyco", "", "", false},
		{"", "", "", false},
	} {
		owner, repo, ok := RepoOf(tt.remote)
		if owner != tt.owner || repo != tt.repo || ok != tt.ok {
			t.Errorf("%q: %q %q %v; want %q %q %v", tt.remote, owner, repo, ok, tt.owner, tt.repo, tt.ok)
		}
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/github -count=1`. Expected: it fails to compile, as the package has no code yet.

- [ ] **Step 3: Write `internal/github/github.go`:**

```go
// Package github asks GitHub's REST API how a tap's repository stands:
// still there as it was, archived, moved or gone.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// State is how a tap's repository stands on GitHub.
type State string

// The states a check can find. Unknown means it couldn't check; the
// TapCheck's Reason says why.
const (
	Fine     State = "fine"
	Archived State = "archived"
	Moved    State = "moved"
	Gone     State = "gone"
	Unknown  State = "unknown"
)

// TapCheck is what checking one tap's repository found.
type TapCheck struct {
	Tap     string `json:"tap"`
	State   State  `json:"state"`
	MovedTo string `json:"moved_to,omitempty"` // the repository's owner/name now, when moved
	Reason  string `json:"reason,omitempty"`   // why it couldn't be checked, when unknown
}

// ErrRateLimited means GitHub won't answer more requests for now.
var ErrRateLimited = errors.New("rate limit reached")

// Client asks GitHub's REST API about repositories, without a token.
type Client struct {
	BaseURL string // https://api.github.com, or a test server
	HTTP    *http.Client
}

// NewClient is a Client for GitHub itself, waiting at most 10 seconds for
// each answer.
func NewClient() *Client {
	return &Client{BaseURL: "https://api.github.com", HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Repo asks how owner/repo stands. A renamed or transferred repository
// redirects, which the HTTP client follows, so its name differs in the
// answer. The TapCheck's Tap is left for the caller to fill in.
func (c *Client) Repo(ctx context.Context, owner, repo string) (TapCheck, error) {
	u := c.BaseURL + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return TapCheck{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "brewery")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return TapCheck{}, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return TapCheck{State: Gone}, nil
	case rateLimited(resp):
		return TapCheck{}, ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		return TapCheck{}, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var body struct {
		FullName string `json:"full_name"`
		Archived bool   `json:"archived"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return TapCheck{}, fmt.Errorf("read GitHub's answer: %w", err)
	}
	switch {
	case body.Archived:
		return TapCheck{State: Archived}, nil
	case !strings.EqualFold(body.FullName, owner+"/"+repo):
		return TapCheck{State: Moved, MovedTo: body.FullName}, nil
	}
	return TapCheck{State: Fine}, nil
}

// rateLimited reports whether resp refuses for want of requests left.
func rateLimited(resp *http.Response) bool {
	refused := resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests
	return refused && resp.Header.Get("X-RateLimit-Remaining") == "0"
}

// RepoOf reads the owner and repository from a tap's remote when it's on
// GitHub: https://github.com/<owner>/<repo>, with or without .git.
func RepoOf(remote string) (owner, repo string, ok bool) {
	rest, found := strings.CutPrefix(remote, "https://github.com/")
	if !found {
		return "", "", false
	}
	rest = strings.TrimSuffix(strings.TrimSuffix(rest, "/"), ".git")
	owner, repo, ok = strings.Cut(rest, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return "", "", false
	}
	return owner, repo, true
}
```

- [ ] **Step 4: Run them.** `go test ./internal/github -count=1`. Expected: PASS.

- [ ] **Step 5: Add the snapshot fields.** In `internal/state/snapshot.go`:
  - Import `"github.com/dillo/brewery/internal/github"`.
  - Set `const snapshotVersion = 6`.
  - Add to `Snapshot`, after `ServiceFilesRan` (let gofmt align it):

```go
	TapChecks       []github.TapCheck    `json:"tap_checks"`      // what the last tap check found, by tap
	TapsCheckedAt   time.Time            `json:"taps_checked_at"` // when it finished; zero if never
```

- [ ] **Step 6: Test the round trip.**
  - Read `internal/state/snapshot_test.go` to find the test that saves a snapshot and loads it back.
  - In its input, set `TapChecks: []github.TapCheck{{Tap: "o/t", State: github.Moved, MovedTo: "n/t"}}` and a non-zero `TapsCheckedAt`.
  - Assert they come back equal. If that test compares whole snapshots with `reflect.DeepEqual`, setting the fields in the input is enough. Use `time.Time.Equal` or a UTC time if a monotonic-clock difference trips the comparison.
  - Run `go test ./internal/state -count=1`. Expected: PASS.

- [ ] **Step 7: Check.** `go test ./... -count=1`, `golangci-lint run ./...` (`0 issues.`), `make cover`.

- [ ] **Step 8: Commit.** `git add internal/github internal/state && git commit -m "feat(github): ask GitHub how a tap's repository stands"`

---

### Task 2: The Check taps row, the Taps column, and the wiring

**Files:**
- Create: `internal/ui/taphealth.go`
- Create: `internal/ui/taphealth_test.go`
- Modify: `internal/ui/maintenance.go`:
  - `maintTaps` between `maintServices` and `maintUpdate`;
  - `syncMaint`, `maintPreview`, `maintRun`.
- Modify: `internal/ui/app.go`:
  - two `App` fields;
  - `sync` (the taps rows);
  - the `Update` message switch.
- Modify: `internal/ui/list.go` (`tapRows`, `tapCols`)
- Modify: `cmd/brewery/main.go` (`app.UseTapChecker(github.NewClient())`)
- Modify: tests that walk Maintenance's rows with `j` past Services cleanup (e.g. `maintenance_test.go` near line 148, `"n", "j", "j", "j", "enter" // Update Homebrew`), which now need one more `j`
- Goldens: `maintenance.txt` gains the row, plus any screen whose background shows Maintenance

**Interfaces:**
- Consumes, from Task 1: everything in its Produces list.
- Consumes, existing in `internal/ui`:
  - `a.note(string) tea.Cmd`, `save(path, snap) tea.Cmd`, `a.cachePath`, `a.ctx`, `a.now`;
  - `ago(now, t time.Time) string`, `kv(st, pairs...)`, `plural`, `theme.Tint`;
  - `st.P.Overlay0/Yellow/Green`, `st.Muted`, `st.Warn`, `st.OK`.

- [ ] **Step 1: Write the failing tests.** Create `internal/ui/taphealth_test.go`:

```go
package ui

import (
	"context"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/github"
	"github.com/dillo/brewery/internal/state"
)

// fakeChecker answers tap checks from a table, by owner/repo, and records
// what it was asked.
type fakeChecker struct {
	mu      sync.Mutex
	asked   []string
	answers map[string]github.TapCheck
	errs    map[string]error
}

func (f *fakeChecker) Repo(_ context.Context, owner, repo string) (github.TapCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := owner + "/" + repo
	f.asked = append(f.asked, k)
	if err := f.errs[k]; err != nil {
		return github.TapCheck{}, err
	}
	return f.answers[k], nil
}

// extraTaps join the fixture's anomalyco/tap (on GitHub) and homebrew/core
// (official): one more on GitHub, and one hosted elsewhere.
var extraTaps = []brew.Tap{
	{Name: "old/tap", Remote: "https://github.com/old/homebrew-tap"},
	{Name: "self/hosted", Remote: "https://git.example.com/self/homebrew-hosted.git"},
}

// tapsApp is an app whose taps, in its snapshot and from its source, are
// the fixture's plus extra, and whose checker is fc.
func tapsApp(t *testing.T, fc *fakeChecker, extra ...brew.Tap) *App {
	t.Helper()
	src := newFakeSource(t)
	src.snap.Taps = append(src.snap.Taps, extra...)
	snap := fixtureSnapshot(t)
	snap.Taps = slices.Clone(src.snap.Taps)
	a := New(src, (&fakeStarter{}).start, snap, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	if fc != nil {
		a.UseTapChecker(fc)
	}
	show(t, a, "Maintenance")
	a.maint.table.SetCursor(maintTaps)
	return a
}

// tapsRow is the Check taps row as drawn.
func tapsRow(t *testing.T, a *App) string {
	t.Helper()
	for _, l := range strings.Split(stripANSI(a.render()), "\n") {
		if i := strings.Index(l, "Check taps"); i >= 0 {
			return l[i:]
		}
	}
	t.Fatalf("no Check taps row:\n%s", stripANSI(a.render()))
	return ""
}

func TestCheckTaps(t *testing.T) {
	fc := &fakeChecker{answers: map[string]github.TapCheck{
		"anomalyco/homebrew-tap": {State: github.Fine},
		"old/homebrew-tap":       {State: github.Archived},
	}}
	a := tapsApp(t, fc, extraTaps...)
	if row := tapsRow(t, a); !regexp.MustCompile(`Check taps\s+not checked yet`).MatchString(row) {
		t.Errorf("before a check: %q", row)
	}
	keys(t, a, "enter")
	if want := []string{"anomalyco/homebrew-tap", "old/homebrew-tap"}; !slices.Equal(fc.asked, want) {
		t.Errorf("asked %v, want %v (not homebrew/core, not self/hosted)", fc.asked, want)
	}
	if row := tapsRow(t, a); !regexp.MustCompile(`Check taps\s+1 archived`).MatchString(row) {
		t.Errorf("after the check: %q", row)
	}
	out := stripANSI(a.render())
	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`checked just now`),
		regexp.MustCompile(`anomalyco/tap\s+fine`),
		regexp.MustCompile(`old/tap\s+archived`),
		regexp.MustCompile(`self/hosted\s+not on GitHub`),
		regexp.MustCompile(`enter checks them on GitHub`),
	} {
		if !want.MatchString(out) {
			t.Errorf("details lack %v:\n%s", want, out)
		}
	}
	saved, err := state.Load(a.cachePath)
	if err != nil || len(saved.TapChecks) != 2 || saved.TapsCheckedAt.IsZero() {
		t.Errorf("the results should be saved: %+v, %v", saved.TapChecks, err)
	}
}

func TestCheckTapsSummaries(t *testing.T) {
	for _, tt := range []struct {
		answers map[string]github.TapCheck
		errs    map[string]error
		want    string
	}{
		{map[string]github.TapCheck{"anomalyco/homebrew-tap": {State: github.Fine}, "old/homebrew-tap": {State: github.Fine}}, nil, "2 fine"},
		{map[string]github.TapCheck{"anomalyco/homebrew-tap": {State: github.Moved, MovedTo: "anomaly/tap"}, "old/homebrew-tap": {State: github.Gone}}, nil, "1 moved · 1 gone"},
		{map[string]github.TapCheck{"anomalyco/homebrew-tap": {State: github.Fine}}, map[string]error{"old/homebrew-tap": context.DeadlineExceeded}, "1 fine · 1 couldn't check"},
		{nil, map[string]error{"anomalyco/homebrew-tap": context.DeadlineExceeded, "old/homebrew-tap": context.DeadlineExceeded}, "2 couldn't check"},
	} {
		a := tapsApp(t, &fakeChecker{answers: tt.answers, errs: tt.errs}, extraTaps...)
		keys(t, a, "enter")
		if row := tapsRow(t, a); !strings.Contains(row, tt.want) {
			t.Errorf("row %q, want %q", row, tt.want)
		}
	}
}

func TestCheckTapsStopsAtTheRateLimit(t *testing.T) {
	fc := &fakeChecker{errs: map[string]error{"anomalyco/homebrew-tap": github.ErrRateLimited}}
	a := tapsApp(t, fc, extraTaps...)
	keys(t, a, "enter")
	if len(fc.asked) != 1 {
		t.Errorf("after the rate limit nothing more is asked: %v", fc.asked)
	}
	if out := stripANSI(a.render()); !regexp.MustCompile(`old/tap\s+couldn't check: rate limit reached`).MatchString(out) {
		t.Errorf("the rest say why:\n%s", out)
	}
}

func TestCheckTapsOnceAtATime(t *testing.T) {
	a := tapsApp(t, &fakeChecker{}, extraTaps...)
	cmd := a.maintRun()
	if cmd == nil || !a.checkingTaps {
		t.Fatal("enter should start a check")
	}
	if row := tapsRow(t, a); !strings.Contains(row, "checking…") {
		t.Errorf("while checking: %q", row)
	}
	if again := a.maintRun(); again != nil || !strings.Contains(stripANSI(a.statusBar()), "already checking taps") {
		t.Errorf("a second enter only notes: %v %q", again, stripANSI(a.statusBar()))
	}
	settle(t, a, cmd)
	if a.checkingTaps {
		t.Error("the check should have finished")
	}
}

func TestCheckTapsWithNothingToCheck(t *testing.T) {
	a := tapsApp(t, &fakeChecker{}) // only anomalyco/tap and homebrew/core
	a.snap.Taps = slices.DeleteFunc(a.snap.Taps, func(t brew.Tap) bool { return !t.Official })
	a.sync()
	if row := tapsRow(t, a); !strings.Contains(row, "no taps to check") {
		t.Errorf("row %q", row)
	}
	keys(t, a, "enter")
	if !strings.Contains(stripANSI(a.statusBar()), "no taps to check") {
		t.Errorf("status %q", stripANSI(a.statusBar()))
	}

	a = tapsApp(t, nil, extraTaps...)
	keys(t, a, "enter")
	if !strings.Contains(stripANSI(a.statusBar()), "checking taps isn't available") {
		t.Errorf("no checker: %q", stripANSI(a.statusBar()))
	}
}

func TestTapsColumnShowsTheCheck(t *testing.T) {
	fc := &fakeChecker{answers: map[string]github.TapCheck{
		"anomalyco/homebrew-tap": {State: github.Fine},
		"old/homebrew-tap":       {State: github.Gone},
	}}
	a := tapsApp(t, fc, extraTaps...)
	keys(t, a, "enter")
	show(t, a, "Taps")
	out := stripANSI(a.render())
	for _, want := range []string{"GITHUB", "fine", "gone"} {
		if !strings.Contains(out, want) {
			t.Errorf("Taps lacks %q:\n%s", want, out)
		}
	}
	if !regexp.MustCompile(`old/tap.*gone`).MatchString(out) {
		t.Errorf("old/tap's row should say gone:\n%s", out)
	}
}
```

If `show(t, a, "Taps")` re-queries taps, the source already returns the same taps (that's why `tapsApp` sets both).

- [ ] **Step 2: Run them.** `go test ./internal/ui -run 'CheckTaps|TapsColumn' -count=1`. Expected: it fails to compile (`maintTaps`, `UseTapChecker`, `checkingTaps` undefined).

- [ ] **Step 3: Write `internal/ui/taphealth.go`:**

```go
package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/github"
	"github.com/dillo/brewery/internal/theme"
)

// TapChecker asks GitHub how a tap's repository stands. *github.Client
// satisfies it; tests use a fake.
type TapChecker interface {
	Repo(ctx context.Context, owner, repo string) (github.TapCheck, error)
}

var _ TapChecker = (*github.Client)(nil)

// UseTapChecker sets how Maintenance checks taps on GitHub.
func (a *App) UseTapChecker(c TapChecker) { a.tapChecker = c }

// tapsCheckedMsg carries a whole tap check's results, and when it ended.
type tapsCheckedMsg struct {
	checks []github.TapCheck
	at     time.Time
}

// reasonW is how much of a failed check's reason the details show.
const reasonW = 40

// thirdPartyTaps are the installed taps that aren't brew's own.
func (a *App) thirdPartyTaps() []brew.Tap {
	var out []brew.Tap
	for _, t := range a.snap.Taps {
		if !t.Official {
			out = append(out, t)
		}
	}
	return out
}

// tapsToCheck are the third-party taps on GitHub.
func (a *App) tapsToCheck() []brew.Tap {
	var out []brew.Tap
	for _, t := range a.thirdPartyTaps() {
		if _, _, ok := github.RepoOf(t.Remote); ok {
			out = append(out, t)
		}
	}
	return out
}

// checkTaps starts asking GitHub about each tap, unless a check is already
// running or there's nothing to check.
func (a *App) checkTaps() tea.Cmd {
	taps := a.tapsToCheck()
	switch {
	case a.tapChecker == nil:
		return a.note("checking taps isn't available")
	case a.checkingTaps:
		return a.note("already checking taps")
	case len(taps) == 0:
		return a.note("no taps to check")
	}
	a.checkingTaps = true
	ctx, checker, now := a.ctx, a.tapChecker, a.now
	return tea.Batch(a.note("checking taps on GitHub…"), func() tea.Msg {
		return tapsCheckedMsg{checks: runTapChecks(ctx, checker, taps), at: now()}
	})
}

// runTapChecks checks each tap in turn. Once GitHub's rate limit is
// reached, the rest aren't asked about and say so.
func runTapChecks(ctx context.Context, c TapChecker, taps []brew.Tap) []github.TapCheck {
	checks := make([]github.TapCheck, len(taps))
	limited := false
	for i, t := range taps {
		checks[i] = github.TapCheck{Tap: t.Name, State: github.Unknown, Reason: github.ErrRateLimited.Error()}
		if limited {
			continue
		}
		owner, repo, _ := github.RepoOf(t.Remote)
		res, err := c.Repo(ctx, owner, repo)
		switch {
		case errors.Is(err, github.ErrRateLimited):
			limited = true
		case err != nil:
			checks[i].Reason = err.Error()
		default:
			res.Tap = t.Name
			checks[i] = res
		}
	}
	return checks
}

// gotTapChecks keeps a finished check's results and saves them.
func (a *App) gotTapChecks(m tapsCheckedMsg) tea.Cmd {
	a.checkingTaps = false
	a.snap.TapChecks, a.snap.TapsCheckedAt = m.checks, m.at
	a.sync()
	return save(a.cachePath, a.snap)
}

// tapCheck is the last check's result for the tap named name.
func (a *App) tapCheck(name string) (github.TapCheck, bool) {
	for _, c := range a.snap.TapChecks {
		if c.Tap == name {
			return c, true
		}
	}
	return github.TapCheck{}, false
}

// checkText says what c found, and whether that's a problem.
func checkText(c github.TapCheck) (text string, problem bool) {
	switch c.State {
	case github.Fine:
		return "fine", false
	case github.Moved:
		return "moved to " + c.MovedTo, true
	case github.Unknown:
		return "couldn't check: " + c.Reason, true
	}
	return string(c.State), true // archived or gone
}

// tapsStatus is the Check taps row's status.
func (a *App) tapsStatus() string {
	st := a.st
	taps := a.tapsToCheck()
	switch {
	case len(taps) == 0:
		return theme.Tint(st.P.Overlay0, "no taps to check")
	case a.checkingTaps:
		return theme.Tint(st.P.Overlay0, "checking…")
	case a.snap.TapsCheckedAt.IsZero():
		return theme.Tint(st.P.Overlay0, "not checked yet")
	}
	n := map[github.State]int{}
	for _, t := range taps {
		if c, ok := a.tapCheck(t.Name); ok {
			n[c.State]++
		}
	}
	var parts []string
	for _, s := range []github.State{github.Archived, github.Moved, github.Gone} {
		if n[s] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n[s], s))
		}
	}
	color := st.P.Yellow
	if len(parts) == 0 && n[github.Fine] > 0 {
		parts, color = []string{fmt.Sprintf("%d fine", n[github.Fine])}, st.P.Green
	}
	if n[github.Unknown] > 0 {
		parts = append(parts, fmt.Sprintf("%d couldn't check", n[github.Unknown]))
	}
	return theme.Tint(color, strings.Join(parts, " · "))
}

// tapsPreview is the Check taps row's details: when the check last ran,
// and what it found for each third-party tap.
func (a *App) tapsPreview() string {
	st := a.st
	taps := a.thirdPartyTaps()
	if len(taps) == 0 {
		return st.Muted.Render("no third-party taps to check")
	}
	when := "not checked yet"
	switch {
	case a.checkingTaps:
		when = "checking…"
	case !a.snap.TapsCheckedAt.IsZero():
		when = "checked " + ago(a.now(), a.snap.TapsCheckedAt)
	}
	var pairs []string
	for _, t := range taps {
		pairs = append(pairs, t.Name, a.tapLine(t))
	}
	return st.Muted.Render(when) + "\n\n" + kv(st, pairs...) + "\n" + st.Muted.Render("enter checks them on GitHub")
}

// tapLine is one tap's line in the details, coloured by what was found.
func (a *App) tapLine(t brew.Tap) string {
	st := a.st
	if _, _, ok := github.RepoOf(t.Remote); !ok {
		return st.Muted.Render("not on GitHub")
	}
	c, ok := a.tapCheck(t.Name)
	if !ok {
		return st.Muted.Render("not checked yet")
	}
	text, problem := checkText(c)
	text = ansi.Truncate(text, reasonW, "…")
	if problem {
		return st.Warn.Render(text)
	}
	return st.OK.Render(text)
}

// tapColumn is a tap's GITHUB cell: empty unless it has been checked.
func (a *App) tapColumn(t brew.Tap) string {
	c, ok := a.tapCheck(t.Name)
	if t.Official || !ok {
		return ""
	}
	text, problem := checkText(c)
	if problem {
		return theme.Tint(a.st.P.Yellow, text)
	}
	return text
}
```

Notes:
- A tap that isn't on GitHub never gets a result, so `tapColumn` leaves it empty.
- `checkText`'s `"couldn't check: " + reason` is truncated in the details only; the table cell truncates on its own.

- [ ] **Step 4: Wire it in.**

**`internal/ui/app.go`:**
- Add to `App`, near `analytics`:

```go
	tapChecker   TapChecker // asks GitHub about taps, or nil
	checkingTaps bool       // a tap check is running
```

- In the message switch that has `case cachedStatsMsg: return a.gotCachedStats(msg)` (around line 385), add next to it:

```go
	case tapsCheckedMsg:
		return a.gotTapChecks(msg)
```
- In `sync`, change `a.taps.setRows(tapRows(a.snap.Taps))` to `a.taps.setRows(a.tapRows())`.

**`internal/ui/list.go`:** replace `tapRows` with a method, and add the column:

```go
// tapRows are Taps' rows, with what the last tap check found.
func (a *App) tapRows() []table.Row {
	rows := make([]table.Row, len(a.snap.Taps))
	for i, t := range a.snap.Taps {
		official := ""
		if t.Official {
			official = "yes"
		}
		rows[i] = table.Row{t.Name, strconv.Itoa(t.Formulae), strconv.Itoa(t.Casks), official, a.tapColumn(t), t.Remote}
	}
	return rows
}
```

```go
	tapCols      = []col{{"NAME", 24, 12}, {"FORMULAE", 8, 0}, {"CASKS", 6, 0}, {"OFFICIAL", 8, 0}, {"GITHUB", 16, 0}, {"REMOTE", 0, 0}}
```

Read `col`'s `width, min` meaning in `components.go` and how `OFFICIAL` behaves. `GITHUB` should be fixed at 16 and should hide before `NAME` shrinks, like `OFFICIAL`. If any other code calls `tapRows(…)`, update it.

**`internal/ui/maintenance.go`:**
- Insert `maintTaps` between `maintServices` and `maintUpdate` in the row constants.
- In `syncMaint`, insert `{"Check taps", a.tapsStatus()},` before `{"Update Homebrew", …}`.
- In `maintPreview`, add `case maintTaps: return "Check taps", a.tapsPreview()`. If `maintPreview` goes over a complexity limit, move the Cleanup, Autoremove and Services cases' bodies into helpers (`cleanupPreview`, `autoremovePreview`, `servicesPreview`), each returning the body.
- In `maintRun`, add `case maintTaps: return a.checkTaps()`. Update the doc comments of `syncMaint`, `maintPreview` and `maintRun` to mention the tap check.

**`cmd/brewery/main.go`:** after `app.UseAnalytics(…)`:

```go
	app.UseTapChecker(github.NewClient()) // only used when the user runs Check taps
```

and import `"github.com/dillo/brewery/internal/github"`.

- [ ] **Step 5: Run the new tests.** `go test ./internal/ui -run 'CheckTaps|TapsColumn' -count=1`. Expected: PASS.

- [ ] **Step 6: Fix row walks and goldens.**
  - Run `go test ./internal/ui -count=1`.
  - Tests that walk Maintenance with `j` to Update Homebrew now land on Check taps; add one `j`. Tests that count Maintenance's rows (`Maintenance · 5`) now see 6.
  - Then run `go test ./internal/ui -run TestScreens -update -count=1`, and read `git diff internal/ui/testdata/golden`. Expected: `maintenance.txt` gains `Check taps          not checked yet`, and its count goes from 5 to 6. Any other screen may change only where Maintenance shows through.
  - If `make screenshots` covers Maintenance, run `make screenshots FREEZE="go run github.com/charmbracelet/freeze@v0.2.2"` and include the changed images. Skip it and say so if a tool is missing.

- [ ] **Step 7: Check.** `go test ./... -count=1`, `golangci-lint run ./...` (`0 issues.`), `make cover`.

- [ ] **Step 8: Commit.** `git add -A internal cmd && git commit -m "feat(ui): check taps on GitHub from Maintenance, and show it on Taps"`
