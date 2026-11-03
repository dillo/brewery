# What's New Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A details pane beside the Outdated list that shows, when the user presses `w` (or picks **What's new** from the action menu), the GitHub release notes for the versions between the installed one and the one brew would install.

**Architecture:**
- `internal/github` gains `Releases`, `Tags` and `SourceRepo`, sharing one request helper with `Repo`.
- `internal/brew` reads where each package is downloaded from (`Package.Sources`).
- A new package, `internal/notes`, compares versions, picks the releases in range, builds the fallback link, spots breaking changes and runs the one or two requests. It has no UI in it.
- The snapshot keeps what was fetched (`ReleaseNotes`, version 9).
- `internal/ui` draws the pane with a small markdown renderer of its own. It reaches GitHub through a `ReleaseFetcher` set with `UseReleases`, so tests use a fake.

**Tech Stack:** Go 1.26 (`net/http`, `net/http/httptest`, `regexp`), Bubble Tea v2, Lip Gloss v2, `github.com/charmbracelet/x/ansi`, golangci-lint v2.

**Spec:** `docs/superpowers/specs/2026-11-02-brewery-whats-new-design.md`

## Global Constraints

- **GitHub only on request:**
  - Brewery calls GitHub for release notes only when the user presses `w` or picks What's new.
  - Nothing fetches in the background, on refresh, or for Upgrade all.
- **No new dependencies:** `go.mod` doesn't change. No markdown library.
- **Lint:**
  - `.golangci.yml` is fixed: gocognit and gocyclo `min-complexity: 15`, both excluded for `_test.go`. Don't edit it, and don't add `//nolint`.
  - `golangci-lint run ./...` must report `0 issues.` at the end of every task.
  - If a function you touch goes over a limit, split it into a named helper.
- **Coverage:** `make cover` stays at 90% or above in total and per package.
- **Network:** tests never touch the network. Use `httptest` in `internal/github`, and fakes in `internal/notes` and `internal/ui`.
- **Goldens:**
  - Update with `go test ./internal/ui -run TestScreens -update -count=1`, and only in Tasks 5 and 7.
  - Read `git diff internal/ui/testdata/golden` before committing.
  - Task 5 may change only `outdated.txt`. Task 7 may only add `outdated-notes.txt`.
- **Style:**
  - Match the surrounding code's comment density, naming and idioms.
  - Every function and exported name gets a short doc comment in plain sentences.
  - Run `gofmt -w` on every Go file you touch.
- **Exact texts:** user-facing strings are exactly as written in this plan.
- **Commits:**
  - Work on a branch, `feat/whats-new`, never on `master`.
  - One commit per task, conventional style, with the message given in the task.

## Commands

| Check | Command |
|---|---|
| Tests | `go test ./... -count=1` |
| One package | `go test ./internal/notes -count=1` |
| Goldens | `go test ./internal/ui -run TestScreens -count=1` |
| Lint | `golangci-lint run ./...` |
| Coverage | `make cover` |

## File map

| File | Task | Responsibility |
|---|---|---|
| `internal/github/github.go` | 1 | shared `get`; `Release`, `Releases`, `Tags`, `ErrNotFound`, `SourceRepo` |
| `internal/github/github_test.go` | 1 | their tests |
| `internal/brew/types.go`, `internal/brew/parse.go` | 2 | `Package.Sources` |
| `internal/brew/parse_test.go` | 2 | its test |
| `internal/notes/notes.go` (new) | 3 | versions, `Pick`, `Link`, `Breaking`, `Notes`, `Fetch` |
| `internal/notes/notes_test.go` (new) | 3 | its tests |
| `internal/state/snapshot.go`, `snapshot_test.go` | 3 | `ReleaseNotes`, version 9 |
| `internal/ui/notesmd.go` (new) | 4 | the markdown renderer |
| `internal/ui/notesmd_test.go` (new) | 4 | its tests |
| `internal/ui/notes.go` (new) | 5, 6 | the pane, `w`, kept notes, breaking warnings |
| `internal/ui/notes_test.go` (new) | 5, 6 | their tests |
| `internal/ui/app.go`, `columns.go` | 5 | wiring: fields, preview, details, scrolling, message |
| `internal/ui/actions.go` | 5, 6 | `w` on Outdated; hints; Upgrade all's warning |
| `internal/ui/menu.go`, `help.go`, `links.go`, `marks.go` | 6 | menu item, help, `gx`, marked upgrade's warning |
| `cmd/brewery/main.go` | 5 | passes the GitHub client |
| `internal/ui/golden_test.go`, `screenshots_test.go`, `testdata/demo/info.json`, `README.md` | 7 | goldens, demo, docs |

---

### Task 1: Releases, tags and source repositories in the GitHub client

**Files:**
- Modify: `internal/github/github.go`
- Test: `internal/github/github_test.go`

**Interfaces:**
- Produces, for later tasks:
  - `github.Release{Tag, Name, Body, URL string; Published time.Time; Prerelease bool}`;
  - `github.ErrNotFound` (text `the repository is gone`);
  - `(*Client).Releases(ctx, owner, repo string) ([]Release, error)`;
  - `(*Client).Tags(ctx, owner, repo string) ([]string, error)`;
  - `github.SourceRepo(addr string) (owner, repo string, ok bool)`.
- `Repo` and `RepoOf` keep their behaviour. Their existing tests must pass unchanged.

- [ ] **Step 1: Create the branch**

```bash
git switch -c feat/whats-new
```

- [ ] **Step 2: Write the failing tests**

Add to the end of `internal/github/github_test.go`:

```go
// fakeReleases answers like GitHub's releases and tags endpoints, and
// records the query of the first request for o/r's releases.
func fakeReleases(t *testing.T, query *string) *Client {
	t.Helper()
	mux := http.NewServeMux()
	answer := func(path, body string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/o/r/releases" && *query == "" {
				*query = r.URL.RawQuery
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
	}
	answer("/repos/o/r/releases", `[
	  {"tag_name": "v2", "name": "Two", "body": "notes", "html_url": "https://github.com/o/r/releases/tag/v2",
	   "published_at": "2026-10-01T12:00:00Z", "prerelease": false},
	  {"tag_name": "v3-rc1", "name": "", "body": "", "html_url": "", "published_at": null, "prerelease": true}
	]`)
	answer("/repos/o/r/tags", `[{"name": "v2"}, {"name": "v1"}]`)
	answer("/repos/o/empty/releases", `[]`)
	answer("/repos/o/badjson/releases", `[{`)
	for _, path := range []string{"/repos/o/limited/releases", "/repos/o/limited/tags"} {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusForbidden)
		})
	}
	mux.HandleFunc("/repos/o/broken/releases", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	// Anything else, /repos/o/gone/releases included, is a 404.
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient()
	c.BaseURL = srv.URL
	return c
}

func TestReleases(t *testing.T) {
	var query string
	c := fakeReleases(t, &query)
	ctx := context.Background()
	got, err := c.Releases(ctx, "o", "r")
	want := []Release{
		{Tag: "v2", Name: "Two", Body: "notes", URL: "https://github.com/o/r/releases/tag/v2",
			Published: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)},
		{Tag: "v3-rc1", Prerelease: true},
	}
	if err != nil || len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("releases %+v, %v; want %+v", got, err, want)
	}
	if query != "per_page=100" {
		t.Errorf("query %q, want per_page=100", query)
	}
	if got, err := c.Releases(ctx, "o", "empty"); err != nil || len(got) != 0 {
		t.Errorf("no releases: %+v, %v", got, err)
	}
	if _, err := c.Releases(ctx, "o", "gone"); !errors.Is(err, ErrNotFound) || err.Error() != "the repository is gone" {
		t.Errorf("a 404 is ErrNotFound: %v", err)
	}
	if _, err := c.Releases(ctx, "o", "limited"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("a 403 with no requests left is the rate limit: %v", err)
	}
	if _, err := c.Releases(ctx, "o", "broken"); err == nil || errors.Is(err, ErrRateLimited) || errors.Is(err, ErrNotFound) {
		t.Errorf("a 500 is an error of its own: %v", err)
	}
	if _, err := c.Releases(ctx, "o", "badjson"); err == nil {
		t.Error("a 200 with a bad body should be an error")
	}
}

func TestTags(t *testing.T) {
	var query string
	c := fakeReleases(t, &query)
	ctx := context.Background()
	if got, err := c.Tags(ctx, "o", "r"); err != nil || strings.Join(got, " ") != "v2 v1" {
		t.Errorf("tags %v, %v; want v2 v1", got, err)
	}
	if _, err := c.Tags(ctx, "o", "limited"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("a 403 with no requests left is the rate limit: %v", err)
	}
	if _, err := c.Tags(ctx, "o", "gone"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a 404 is ErrNotFound: %v", err)
	}
}

func TestSourceRepo(t *testing.T) {
	for _, tt := range []struct {
		addr, owner, repo string
		ok                bool
	}{
		{"https://github.com/cli/cli/archive/refs/tags/v2.104.0.tar.gz", "cli", "cli", true},
		{"https://github.com/cli/cli/releases/download/v2.104.0/gh.zip", "cli", "cli", true},
		{"https://codeload.github.com/o/r/tar.gz/refs/tags/v1", "o", "r", true},
		{"https://github.com/o/r.git", "o", "r", true},
		{"https://github.com/sharkdp/fd", "sharkdp", "fd", true},
		{"https://github.com/json-c/json-c/wiki", "json-c", "json-c", true},
		{"https://github.com/o/r?tab=readme", "o", "r", true},
		{"https://cli.github.com/", "", "", false},
		{"https://gitlab.com/o/r", "", "", false},
		{"http://github.com/o/r", "", "", false},
		{"https://github.com/o", "", "", false},
		{"https://github.com/o/", "", "", false},
		{"", "", "", false},
	} {
		owner, repo, ok := SourceRepo(tt.addr)
		if owner != tt.owner || repo != tt.repo || ok != tt.ok {
			t.Errorf("%q: %q %q %v; want %q %q %v", tt.addr, owner, repo, ok, tt.owner, tt.repo, tt.ok)
		}
	}
}
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `go test ./internal/github -count=1`
Expected: the build fails with `undefined: Release`, `c.Releases undefined`, `c.Tags undefined`, `undefined: ErrNotFound`, `undefined: SourceRepo`.

- [ ] **Step 4: Implement**

In `internal/github/github.go`:

1. Change the package comment to:

```go
// Package github asks GitHub's REST API about repositories: how a tap's
// repository stands, and a package's releases and tags.
package github
```

2. After `var ErrRateLimited = …`, add:

```go
// ErrNotFound means GitHub has no such repository.
var ErrNotFound = errors.New("the repository is gone")

// Release is one of a repository's releases on GitHub.
type Release struct {
	Tag        string    `json:"tag_name"`
	Name       string    `json:"name"`
	Body       string    `json:"body"` // the notes, in GitHub's markdown
	URL        string    `json:"html_url"`
	Published  time.Time `json:"published_at"` // zero when GitHub gives none
	Prerelease bool      `json:"prerelease"`
}
```

3. Replace the whole `Repo` function with `get`, the new `Repo` and the list functions:

```go
// get asks GitHub for path, for the caller to read and close. A timeout or
// a failed connection comes back as its cause, without the request.
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "brewery")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		var ue *url.Error
		switch {
		case !errors.As(err, &ue):
			return nil, err
		case ue.Timeout():
			return nil, errors.New("timed out")
		}
		return nil, ue.Err // the cause, without the request it wraps
	}
	return resp, nil
}

// repoPath is the API's path for owner/repo.
func repoPath(owner, repo string) string {
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
}

// Repo asks how owner/repo stands. A renamed or transferred repository
// redirects, which the HTTP client follows, so its name differs in the
// answer. The TapCheck's Tap is left for the caller to fill in.
func (c *Client) Repo(ctx context.Context, owner, repo string) (TapCheck, error) {
	resp, err := c.get(ctx, repoPath(owner, repo))
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
	case body.FullName == "":
		return TapCheck{State: Fine}, nil // no name to compare
	case !strings.EqualFold(body.FullName, owner+"/"+repo):
		return TapCheck{State: Moved, MovedTo: body.FullName}, nil
	}
	return TapCheck{State: Fine}, nil
}

// list reads the first page, of up to 100, of one of owner/repo's lists
// (releases or tags) into out.
func (c *Client) list(ctx context.Context, owner, repo, what string, out any) error {
	resp, err := c.get(ctx, repoPath(owner, repo)+"/"+what+"?per_page=100")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case rateLimited(resp):
		return ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("GitHub answered %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("read GitHub's answer: %w", err)
	}
	return nil
}

// Releases asks for owner/repo's newest releases, newest first as GitHub
// lists them. Drafts aren't shown without a token.
func (c *Client) Releases(ctx context.Context, owner, repo string) ([]Release, error) {
	var rels []Release
	if err := c.list(ctx, owner, repo, "releases", &rels); err != nil {
		return nil, err
	}
	return rels, nil
}

// Tags asks for the names of owner/repo's newest tags.
func (c *Client) Tags(ctx context.Context, owner, repo string) ([]string, error) {
	var tags []struct {
		Name string `json:"name"`
	}
	if err := c.list(ctx, owner, repo, "tags", &tags); err != nil {
		return nil, err
	}
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return names, nil
}
```

4. At the end of the file, add:

```go
// SourceRepo reads the owner and repository from an address on GitHub:
// https://github.com/<owner>/<repo> or https://codeload.github.com/<owner>/<repo>,
// followed by anything (an archive, a release download, a wiki). A
// trailing .git is dropped.
func SourceRepo(addr string) (owner, repo string, ok bool) {
	rest, found := strings.CutPrefix(addr, "https://github.com/")
	if !found {
		rest, found = strings.CutPrefix(addr, "https://codeload.github.com/")
	}
	if !found {
		return "", "", false
	}
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest = rest[:i]
	}
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 {
		return "", "", false
	}
	owner, repo = parts[0], strings.TrimSuffix(parts[1], ".git")
	if owner == "" || repo == "" {
		return "", "", false
	}
	return owner, repo, true
}
```

- [ ] **Step 5: Run the tests to see them pass**

Run: `go test ./internal/github -count=1`
Expected: `ok`. The existing `TestRepo`, `TestRepoTimeout`, `TestRepoTransportFailure` and `TestRepoOf` pass too.

- [ ] **Step 6: Lint**

Run: `golangci-lint run ./...`
Expected: `0 issues.`

- [ ] **Step 7: Commit**

```bash
git add internal/github
git commit -m "feat(github): ask for a repository's releases and tags"
```

---

### Task 2: Where each package comes from

**Files:**
- Modify: `internal/brew/types.go` (one field on `Package`)
- Modify: `internal/brew/parse.go` (`formulaJSON`, `caskJSON`, both `toPackage`)
- Test: `internal/brew/parse_test.go`

**Interfaces:**
- Produces: `brew.Package.Sources []string`. This holds where brew downloads the package from, in order: a formula's stable then head URL, or a cask's URL. Empty ones are left out, and it is nil when there are none. The homepage is not in it.

- [ ] **Step 1: Write the failing test**

Add to `internal/brew/parse_test.go`:

```go
func TestParseInfoSources(t *testing.T) {
	in := `{"formulae": [
	  {"name": "gh", "full_name": "gh", "homepage": "https://cli.github.com/",
	   "urls": {"stable": {"url": "https://github.com/cli/cli/archive/refs/tags/v2.102.0.tar.gz", "tag": null},
	            "head": {"url": "https://github.com/cli/cli.git", "branch": "trunk"}}},
	  {"name": "make", "full_name": "make", "urls": {"stable": {"url": "https://ftpmirror.gnu.org/make/make-4.4.tar.lz"}, "head": null}},
	  {"name": "bare", "full_name": "bare"}
	], "casks": [
	  {"token": "firefox", "full_token": "firefox", "url": "https://download-installer.cdn.mozilla.net/firefox.dmg"},
	  {"token": "nourl", "full_token": "nourl"}
	]}`
	pkgs, err := ParseInfo([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		want []string
	}{
		{"gh", []string{"https://github.com/cli/cli/archive/refs/tags/v2.102.0.tar.gz", "https://github.com/cli/cli.git"}},
		{"make", []string{"https://ftpmirror.gnu.org/make/make-4.4.tar.lz"}},
		{"bare", nil},
		{"firefox", []string{"https://download-installer.cdn.mozilla.net/firefox.dmg"}},
		{"nourl", nil},
	} {
		if got := byName(pkgs, tt.name).Sources; !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: sources %q, want %q", tt.name, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/brew -run TestParseInfoSources -count=1`
Expected: the build fails with `byName(pkgs, tt.name).Sources undefined`.

- [ ] **Step 3: Implement**

In `internal/brew/types.go`, add to `Package` after `Homepage`:

```go
	Sources         []string  `json:"sources,omitempty"` // where brew downloads it from: a formula's stable then head URL, a cask's URL
```

In `internal/brew/parse.go`:

1. In `formulaJSON`, after `Homepage`, add:

```go
	URLs     struct {
		Stable struct {
			URL string `json:"url"`
		} `json:"stable"`
		Head struct {
			URL string `json:"url"`
		} `json:"head"`
	} `json:"urls"`
```

2. In `caskJSON`, after `Homepage`, add:

```go
	URL           string   `json:"url"`
```

3. In `formulaJSON.toPackage`, after the `p := Package{…}` literal, add:

```go
	p.Sources = nonEmpty(f.URLs.Stable.URL, f.URLs.Head.URL)
```

4. In `caskJSON.toPackage`, after the `p := Package{…}` literal, add:

```go
	p.Sources = nonEmpty(c.URL)
```

5. After `deref`, add:

```go
// nonEmpty is the strings that aren't "", in order, or nil when none are.
func nonEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
```

- [ ] **Step 4: Run the tests to see them pass**

Run: `gofmt -w internal/brew && go test ./internal/brew -count=1`
Expected: `ok`.

- [ ] **Step 5: Run everything, and lint**

Run: `go test ./... -count=1 && golangci-lint run ./...`
Expected:
- All packages `ok`. The fixtures have no `urls`, so nothing that shows packages changes, goldens included.
- `0 issues.`

- [ ] **Step 6: Commit**

```bash
git add internal/brew
git commit -m "feat(brew): keep where each package is downloaded from"
```

---

### Task 3: The notes package, and keeping notes in the snapshot

**Files:**
- Create: `internal/notes/notes.go`
- Create: `internal/notes/notes_test.go`
- Modify: `internal/state/snapshot.go` (one field; `snapshotVersion` to 9)
- Test: `internal/state/snapshot_test.go`

**Interfaces:**
- Consumes: `github.Release`, from Task 1.
- Produces, for Tasks 4 to 7:
  - `notes.Notes{Repo, From, To string; Releases []github.Release; Link string; At time.Time}`.
  - `notes.Fetcher` interface: `Releases(ctx, owner, repo) ([]github.Release, error)` and `Tags(ctx, owner, repo) ([]string, error)`. `*github.Client` satisfies it.
  - `notes.Fetch(ctx, f Fetcher, owner, repo, name, from, to string, now time.Time) (Notes, error)`.
  - `notes.Breaking(body string) bool` and `notes.BreakingSpans(s string) [][]int`.
  - `notes.Clean(v string) string`, `notes.Less(a, b string) bool`, `notes.TagVersion(tag, repo, name string) (string, bool)`, `notes.Pick(…)` and `notes.Link(…)`.
  - `state.Snapshot.ReleaseNotes map[string]notes.Notes`.

- [ ] **Step 1: Write the failing tests**

Create `internal/notes/notes_test.go`:

```go
package notes

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dillo/brewery/internal/github"
)

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"2.101.0":        "2.101.0",
		"2.101.0_1":      "2.101.0",
		"4.39.0,184744":  "4.39.0",
		"1.4.2,abc_1":    "1.4.2",
		"1.0_rc":         "1.0_rc", // only a numeric revision goes
		"20260817.0_12":  "20260817.0",
		"":               "",
	} {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLess(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		less bool
	}{
		{"2.9", "2.10", true},
		{"2.10", "2.9", false},
		{"2.1", "2.1.1", true}, // a missing part is lower
		{"2.1.1", "2.1", false},
		{"2.1", "2.1", false},
		{"1.0-beta", "1.0-rc", true}, // text compares as text
		{"3.19.1", "3.19.2", true},
		{"20260817.0", "20261001.0", true},
	} {
		if got := Less(tt.a, tt.b); got != tt.less {
			t.Errorf("Less(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.less)
		}
	}
}

func TestTagVersion(t *testing.T) {
	for _, tt := range []struct {
		tag, want string
		ok        bool
	}{
		{"v2.104.0", "2.104.0", true},
		{"V2.104.0", "2.104.0", true},
		{"2.104.0", "2.104.0", true},
		{"release-1.2", "1.2", true},
		{"cli-2.1", "2.1", true}, // the repository's name
		{"gh-v2.1", "2.1", true}, // the package's name, then v
		{"nightly", "", false},
		{"v", "", false},
		{"", "", false},
	} {
		got, ok := TagVersion(tt.tag, "cli", "gh")
		if got != tt.want || ok != tt.ok {
			t.Errorf("TagVersion(%q) = %q, %v; want %q, %v", tt.tag, got, ok, tt.want, tt.ok)
		}
	}
}

func tags(rels []github.Release) []string {
	out := make([]string, len(rels))
	for i, r := range rels {
		out[i] = r.Tag
	}
	return out
}

func TestPick(t *testing.T) {
	rels := []github.Release{
		{Tag: "v2.105.0"},                  // after to
		{Tag: "v2.104.0"},                  // to: in
		{Tag: "v2.104.0-rc1", Prerelease: true},
		{Tag: "nightly", Name: "2.103.5"},  // the name stands in for the tag
		{Tag: "v2.103.0"},                  // in
		{Tag: "weekly", Name: "Weekly"},    // neither parses: out
		{Tag: "v2.101.0"},                  // from: out
		{Tag: "v2.100.0"},
	}
	got := tags(Pick(rels, "cli", "gh", "2.101.0_1", "2.104.0"))
	if want := []string{"v2.104.0", "nightly", "v2.103.0"}; !slices.Equal(got, want) {
		t.Errorf("picked %v, want %v", got, want)
	}
	pre := []github.Release{{Tag: "v3.0.0-rc2", Prerelease: true}, {Tag: "v3.0.0-rc1", Prerelease: true}, {Tag: "v2.9.0"}}
	if got := tags(Pick(pre, "cli", "gh", "2.9.0", "3.0.0-rc2")); !slices.Equal(got, []string{"v3.0.0-rc2", "v3.0.0-rc1"}) {
		t.Errorf("to a pre-release, pre-releases count: %v", got)
	}
	if got := tags(Pick(pre, "cli", "gh", "2.9.0", "3.0.0")); len(got) != 0 {
		t.Errorf("to a release, pre-releases don't: %v", got)
	}
	shuffled := []github.Release{{Tag: "v1.1"}, {Tag: "v1.3"}, {Tag: "v1.2"}}
	if got := tags(Pick(shuffled, "r", "n", "1.0", "1.3")); !slices.Equal(got, []string{"v1.3", "v1.2", "v1.1"}) {
		t.Errorf("newest first: %v", got)
	}
}

func TestLink(t *testing.T) {
	base := "https://github.com/cli/cli"
	for _, tt := range []struct {
		tags []string
		want string
	}{
		{[]string{"v2.104.0", "v2.101.0"}, base + "/compare/v2.101.0...v2.104.0"},
		{[]string{"v2.104.0"}, base + "/releases"},
		{nil, base + "/releases"},
	} {
		if got := Link("cli", "cli", "gh", tt.tags, "2.101.0_1", "2.104.0"); got != tt.want {
			t.Errorf("tags %v: %q, want %q", tt.tags, got, tt.want)
		}
	}
}

func TestBreaking(t *testing.T) {
	for body, want := range map[string]bool{
		"BREAKING: the flag changed":        true,
		"`--old` was removed":                true,
		"Deprecated the old API":             true,
		"an incompatible change":             true,
		"see the migration guide":            true,
		"an irremovable bug is fixed":        false,
		"fixed a crash":                      false,
	} {
		if got := Breaking(body); got != want {
			t.Errorf("Breaking(%q) = %v, want %v", body, got, want)
		}
	}
	if got := BreakingSpans("x removed y"); len(got) != 1 || got[0][0] != 2 || got[0][1] != 9 {
		t.Errorf("spans %v, want [[2 9]]", got)
	}
}

// fakeFetcher answers from fields, and records what it was asked.
type fakeFetcher struct {
	rels           []github.Release
	tags           []string
	relErr, tagErr error
	asked          []string
}

func (f *fakeFetcher) Releases(_ context.Context, owner, repo string) ([]github.Release, error) {
	f.asked = append(f.asked, "releases "+owner+"/"+repo)
	return f.rels, f.relErr
}

func (f *fakeFetcher) Tags(_ context.Context, owner, repo string) ([]string, error) {
	f.asked = append(f.asked, "tags "+owner+"/"+repo)
	return f.tags, f.tagErr
}

func TestFetch(t *testing.T) {
	ctx, now := context.Background(), time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	long := strings.Repeat("x", maxBody+10)
	f := &fakeFetcher{rels: []github.Release{{Tag: "v2", Body: long}, {Tag: "v1"}}}
	n, err := Fetch(ctx, f, "o", "r", "r", "1", "2", now)
	if err != nil || n.Repo != "o/r" || n.From != "1" || n.To != "2" || !n.At.Equal(now) || len(n.Releases) != 1 || n.Link != "" {
		t.Fatalf("releases found: %+v, %v", n, err)
	}
	if len(n.Releases[0].Body) != maxBody {
		t.Errorf("a long body is kept to %d bytes, got %d", maxBody, len(n.Releases[0].Body))
	}
	if !slices.Equal(f.asked, []string{"releases o/r"}) {
		t.Errorf("with releases found, only they are asked for: %v", f.asked)
	}

	f = &fakeFetcher{tags: []string{"v2", "v1"}}
	if n, err := Fetch(ctx, f, "o", "r", "r", "1", "2", now); err != nil || len(n.Releases) != 0 || n.Link != "https://github.com/o/r/compare/v1...v2" {
		t.Errorf("no releases: %+v, %v", n, err)
	}
	f = &fakeFetcher{tagErr: github.ErrRateLimited}
	if n, err := Fetch(ctx, f, "o", "r", "r", "1", "2", now); err != nil || n.Link != "https://github.com/o/r/releases" {
		t.Errorf("tags failing still gives the releases page: %+v, %v", n, err)
	}
	f = &fakeFetcher{relErr: github.ErrNotFound}
	if _, err := Fetch(ctx, f, "o", "r", "r", "1", "2", now); !errors.Is(err, github.ErrNotFound) {
		t.Errorf("releases failing is the error: %v", err)
	}
}
```

Add to `internal/state/snapshot_test.go`:
- Add the import `"github.com/dillo/brewery/internal/notes"`.
- In `TestSaveLoadRoundTrip`'s `in := Snapshot{…}`, after `VulnsAt`, add:

```go
		ReleaseNotes: map[string]notes.Notes{"cask/docker-desktop": {
			Repo: "docker/desktop", From: "4.39.0", To: "4.95.0",
			Releases: []github.Release{{Tag: "v4.95.0", Body: "b", Published: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}},
			At:       time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC),
		}},
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/notes ./internal/state -count=1`
Expected: the build fails: the `internal/notes` package doesn't exist yet, and `ReleaseNotes` is an unknown field.

- [ ] **Step 3: Implement the notes package**

Create `internal/notes/notes.go`:

```go
// Package notes picks a package's release notes on GitHub: the releases
// between the version installed and the one brew would install.
package notes

import (
	"cmp"
	"context"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dillo/brewery/internal/github"
)

// Notes are what was fetched about one package's upgrade.
type Notes struct {
	Repo     string           `json:"repo"`     // owner/repo
	From     string           `json:"from"`     // the installed version, as brew writes it
	To       string           `json:"to"`       // the version brew would install
	Releases []github.Release `json:"releases"` // in range, newest first
	Link     string           `json:"link"`     // the compare or releases page, when Releases is empty
	At       time.Time        `json:"at"`       // when they were fetched
}

// Fetcher asks GitHub for a repository's releases and tags.
type Fetcher interface {
	Releases(ctx context.Context, owner, repo string) ([]github.Release, error)
	Tags(ctx context.Context, owner, repo string) ([]string, error)
}

var _ Fetcher = (*github.Client)(nil)

// maxBody is the most of one release's notes that is kept, in bytes.
const maxBody = 16 << 10

// Fetch asks GitHub what's new in owner/repo after from, up to to. When no
// release falls in between, it asks for the tags too, for a page to read
// instead: the compare page, or the releases page when the tags don't say
// (or can't be had). name is the package's, for reading tags. now stamps
// the result.
func Fetch(ctx context.Context, f Fetcher, owner, repo, name, from, to string, now time.Time) (Notes, error) {
	rels, err := f.Releases(ctx, owner, repo)
	if err != nil {
		return Notes{}, err
	}
	n := Notes{Repo: owner + "/" + repo, From: from, To: to, At: now, Releases: Pick(rels, repo, name, from, to)}
	for i := range n.Releases {
		n.Releases[i].Body = trimBody(n.Releases[i].Body)
	}
	if len(n.Releases) > 0 {
		return n, nil
	}
	tags, err := f.Tags(ctx, owner, repo)
	if err != nil {
		tags = nil // the releases page needs no tags
	}
	n.Link = Link(owner, repo, name, tags, from, to)
	return n, nil
}

// trimBody keeps at most maxBody bytes of a release's notes, without
// cutting a character in two.
func trimBody(s string) string {
	if len(s) <= maxBody {
		return s
	}
	return strings.ToValidUTF8(s[:maxBody], "")
}

// Clean is brew's version as upstream writes it: without a formula's
// revision (2.101.0_1) or what follows a cask's first comma (4.39.0,184744).
func Clean(v string) string {
	v, _, _ = strings.Cut(v, ",")
	if i := strings.LastIndexByte(v, '_'); i > 0 && isNumber(v[i+1:]) {
		v = v[:i]
	}
	return v
}

// isNumber reports whether s is one or more digits.
func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil && !strings.ContainsAny(s, "+-")
}

// Less reports whether version a comes before b. Parts split on . and -;
// numeric parts compare as numbers, others as text, and a version that
// runs out of parts first is the lower.
func Less(a, b string) bool { return compare(a, b) < 0 }

// compare is Less as -1, 0 or 1.
func compare(a, b string) int {
	pa, pb := parts(a), parts(b)
	for i := range min(len(pa), len(pb)) {
		if c := comparePart(pa[i], pb[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(pa), len(pb))
}

// parts splits a version on . and -.
func parts(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' })
}

// comparePart compares two parts of a version: as numbers when both are,
// otherwise as text.
func comparePart(a, b string) int {
	if isNumber(a) && isNumber(b) {
		na, _ := strconv.Atoi(a)
		nb, _ := strconv.Atoi(b)
		return cmp.Compare(na, nb)
	}
	return strings.Compare(a, b)
}

// TagVersion is the version a tag (or a release's name) stands for: with
// the repository's or package's name and a dash, then release-, then v,
// taken off the front. ok is false when what's left doesn't start with a
// digit.
func TagVersion(tag, repo, name string) (string, bool) {
	t := strings.TrimSpace(tag)
	for _, p := range []string{repo + "-", name + "-", "release-", "v"} {
		if len(t) > len(p) && strings.EqualFold(t[:len(p)], p) {
			t = t[len(p):]
		}
	}
	if t == "" || t[0] < '0' || t[0] > '9' {
		return "", false
	}
	return t, true
}

// releaseVersion is the version r stands for: its tag's, or else its
// name's.
func releaseVersion(r github.Release, repo, name string) (string, bool) {
	if v, ok := TagVersion(r.Tag, repo, name); ok {
		return v, true
	}
	return TagVersion(r.Name, repo, name)
}

// isPre reports whether version v is a pre-release: it has a letter in it.
func isPre(v string) bool {
	return strings.ContainsAny(strings.ToLower(v), "abcdefghijklmnopqrstuvwxyz")
}

// Pick is the releases after from, up to and including to, newest first.
// from and to are brew's versions. A release whose tag and name both fail
// to read as versions is left out, and so are pre-releases, unless to is
// one.
func Pick(rels []github.Release, repo, name, from, to string) []github.Release {
	from, to = Clean(from), Clean(to)
	type found struct {
		rel github.Release
		v   string
	}
	var in []found
	for _, r := range rels {
		v, ok := releaseVersion(r, repo, name)
		if !ok || (r.Prerelease && !isPre(to)) {
			continue
		}
		if compare(v, from) > 0 && compare(v, to) <= 0 {
			in = append(in, found{r, v})
		}
	}
	slices.SortStableFunc(in, func(a, b found) int { return compare(b.v, a.v) })
	out := make([]github.Release, len(in))
	for i, f := range in {
		out[i] = f.rel
	}
	return out
}

// Link is where to read about the change from from to to on GitHub: the
// compare page when both versions have a tag among tags, otherwise the
// releases page.
func Link(owner, repo, name string, tags []string, from, to string) string {
	from, to = Clean(from), Clean(to)
	var old, cur string
	for _, t := range tags {
		v, ok := TagVersion(t, repo, name)
		switch {
		case !ok:
		case old == "" && compare(v, from) == 0:
			old = t
		case cur == "" && compare(v, to) == 0:
			cur = t
		}
	}
	base := "https://github.com/" + owner + "/" + repo
	if old == "" || cur == "" {
		return base + "/releases"
	}
	return base + "/compare/" + url.PathEscape(old) + "..." + url.PathEscape(cur)
}

// breakingWords are the words that flag a breaking change, whole and in
// any case.
var breakingWords = regexp.MustCompile(`(?i)\b(breaking|removed|deprecated|incompatible|migration)\b`)

// Breaking reports whether release notes mention a breaking change.
func Breaking(body string) bool { return breakingWords.MatchString(body) }

// BreakingSpans are where in s the words that flag a breaking change are,
// as start and end byte offsets.
func BreakingSpans(s string) [][]int { return breakingWords.FindAllStringIndex(s, -1) }
```

- [ ] **Step 4: Keep notes in the snapshot**

In `internal/state/snapshot.go`:
- Add the import `"github.com/dillo/brewery/internal/notes"`.
- Change `const snapshotVersion = 8` to `const snapshotVersion = 9`.
- In `Snapshot`, after `VulnsAt`, add:

```go
	ReleaseNotes    map[string]notes.Notes `json:"release_notes"` // what's new in each upgrade, by Package.Key; fetched only when asked
```

- [ ] **Step 5: Run the tests to see them pass**

Run: `gofmt -w internal/notes internal/state && go test ./internal/notes ./internal/state -count=1`
Expected: both `ok`.

- [ ] **Step 6: Run everything, lint and coverage**

Run: `go test ./... -count=1 && golangci-lint run ./... && make cover`
Expected:
- All `ok`, and `0 issues.`
- `internal/notes` at 90% or above.

- [ ] **Step 7: Commit**

```bash
git add internal/notes internal/state
git commit -m "feat(notes): pick the releases between two versions, and keep them"
```

---

### Task 4: Drawing release notes

**Files:**
- Create: `internal/ui/notesmd.go`
- Create: `internal/ui/notesmd_test.go`

**Interfaces:**
- Consumes: `notes.BreakingSpans`, from Task 3.
- Produces, for Task 5: `renderNotes(st theme.Styles, body string, w int) []string`. It returns styled lines, each at most `w` cells wide, with blank runs collapsed and no blank line first or last. It returns nil for a body with nothing to show.

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/notesmd_test.go`:

```go
package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/theme"
)

// ghNotes is how GitHub's generated release notes look.
const ghNotes = "<!-- Release notes generated using configuration in .github/release.yml -->\r\n" +
	"## What's Changed\r\n" +
	"### Features\r\n" +
	"* Add `gh pr revert` by @octocat in https://github.com/cli/cli/pull/1\r\n" +
	"  - nested [link](https://example.com)\r\n" +
	"![screenshot](https://example.com/a.png)\r\n" +
	"<img src=\"x.png\" width=\"10\">\r\n" +
	"\r\n" +
	"**BREAKING**: removed the old flag\r\n" +
	"\r\n" +
	"\r\n" +
	"```sh\r\n" +
	"gh pr revert 1\r\n" +
	"```\r\n"

func plainLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = ansi.Strip(l)
	}
	return out
}

func TestRenderNotes(t *testing.T) {
	st := theme.New(theme.Macchiato)
	got := renderNotes(st, ghNotes, 100)
	want := []string{
		"What's Changed",
		"Features",
		"• Add gh pr revert by @octocat in https://github.com/cli/cli/pull/1",
		"  • nested link",
		"",
		"BREAKING: removed the old flag",
		"",
		"gh pr revert 1",
	}
	if p := plainLines(got); !slices.Equal(p, want) {
		t.Errorf("rendered\n%q\nwant\n%q", p, want)
	}
	if !strings.Contains(got[2], st.Accent.Render("gh pr revert")) {
		t.Errorf("code is in the accent colour: %q", got[2])
	}
	if !strings.Contains(got[5], st.Warn.Bold(true).Render("BREAKING")) || !strings.Contains(got[5], st.Warn.Render("removed")) {
		t.Errorf("breaking words are in the warning colour, bold inside **: %q", got[5])
	}
	if !strings.Contains(got[7], st.Muted.Render("gh pr revert 1")) {
		t.Errorf("code blocks are muted: %q", got[7])
	}
}

func TestRenderNotesDetails(t *testing.T) {
	st := theme.New(theme.Macchiato)
	for _, tt := range []struct {
		body string
		want []string
	}{
		{"", nil},
		{"\n\n<!-- nothing -->\n\n", nil},
		{"# Title #", []string{"Title"}},
		{"+ plus\n\t- tab", []string{"• plus", "• tab"}},
		{"      - deep", []string{"  • deep"}}, // two levels at most
		{"an `unclosed tick", []string{"an `unclosed tick"}},
		{"__bold__ and *em* and _em_", []string{"bold and *em* and _em_"}},
		{"---", []string{"---"}},
	} {
		if got := plainLines(renderNotes(st, tt.body, 80)); !slices.Equal(got, tt.want) {
			t.Errorf("%q: %q, want %q", tt.body, got, tt.want)
		}
	}
	for _, l := range renderNotes(st, strings.Repeat("word ", 30)+strings.Repeat("x", 50), 20) {
		if w := ansi.StringWidth(l); w > 20 {
			t.Errorf("a line %d wide, past 20: %q", w, ansi.Strip(l))
		}
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/ui -run TestRenderNotes -count=1`
Expected: the build fails with `undefined: renderNotes`.

- [ ] **Step 3: Implement**

Create `internal/ui/notesmd.go`:

```go
package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/notes"
	"github.com/dillo/brewery/internal/theme"
)

// Release notes are GitHub markdown. This draws the little of it they use,
// in the theme's styles, without a markdown library: headings, lists,
// bold, code and links, and drops images, HTML and comments.
var (
	mdComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	mdImage   = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	mdLink    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdTag     = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	mdHeading = regexp.MustCompile(`^#{1,6}\s+(.*?)[\s#]*$`)
	mdItem    = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	mdBold    = regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
)

// renderNotes draws a release's notes as lines at most w wide. Runs of
// blank lines become one, and none starts or ends it.
func renderNotes(st theme.Styles, body string, w int) []string {
	body = mdComment.ReplaceAllString(strings.ReplaceAll(body, "\r\n", "\n"), "")
	var out []string
	fenced, blank := false, true // blank: the last line kept is blank, or there is none yet
	for _, l := range strings.Split(body, "\n") {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fenced = !fenced
			continue
		}
		line := st.Muted.Render(l)
		if !fenced {
			line = mdLine(st, l)
		}
		if strings.TrimSpace(ansi.Strip(line)) == "" {
			if !blank {
				out = append(out, "")
			}
			blank = true
			continue
		}
		blank = false
		out = append(out, strings.Split(ansi.Wrap(line, w, ""), "\n")...)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// mdLine draws one line outside a code block: a heading in bold, a list
// item after a bullet, indented one level at most, and anything else as
// it is.
func mdLine(st theme.Styles, l string) string {
	l = mdTag.ReplaceAllString(mdLink.ReplaceAllString(mdImage.ReplaceAllString(l, ""), "$1"), "")
	if m := mdHeading.FindStringSubmatch(l); m != nil {
		return mdInline(st, m[1], true)
	}
	if m := mdItem.FindStringSubmatch(l); m != nil {
		indent := strings.Repeat("  ", min(len(m[1])/2, 1))
		return indent + "• " + mdInline(st, m[2], false)
	}
	return mdInline(st, strings.TrimRight(l, " \t"), false)
}

// mdInline styles a line's text: `code` in the accent colour, **bold** in
// bold, and the words that flag a breaking change in the warning colour.
// bold draws all of it in bold, for headings.
func mdInline(st theme.Styles, s string, bold bool) string {
	parts := strings.Split(s, "`")
	if len(parts)%2 == 0 { // an unclosed `: the text after it is plain
		last := len(parts) - 1
		parts[last-1] += "`" + parts[last]
		parts = parts[:last]
	}
	var b strings.Builder
	for i, p := range parts {
		if i%2 == 1 {
			b.WriteString(st.Accent.Render(p))
			continue
		}
		b.WriteString(mdBoldRuns(st, p, bold))
	}
	return b.String()
}

// mdBoldRuns draws text with its **bold** and __bold__ runs in bold,
// without the markers.
func mdBoldRuns(st theme.Styles, s string, bold bool) string {
	var b strings.Builder
	at := 0
	for _, m := range mdBold.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(mdWords(st, s[at:m[0]], bold))
		start, end := m[2], m[3]
		if start < 0 { // __bold__
			start, end = m[4], m[5]
		}
		b.WriteString(mdWords(st, s[start:end], true))
		at = m[1]
	}
	b.WriteString(mdWords(st, s[at:], bold))
	return b.String()
}

// mdWords draws plain text, with the words that flag a breaking change in
// the warning colour.
func mdWords(st theme.Styles, s string, bold bool) string {
	if s == "" {
		return ""
	}
	text, warn := st.Text, st.Warn
	if bold {
		text, warn = text.Bold(true), warn.Bold(true)
	}
	var b strings.Builder
	at := 0
	for _, m := range notes.BreakingSpans(s) {
		if m[0] > at {
			b.WriteString(text.Render(s[at:m[0]]))
		}
		b.WriteString(warn.Render(s[m[0]:m[1]]))
		at = m[1]
	}
	if at < len(s) {
		b.WriteString(text.Render(s[at:]))
	}
	return b.String()
}
```

- [ ] **Step 4: Run the tests to see them pass**

Run: `gofmt -w internal/ui && go test ./internal/ui -run TestRenderNotes -count=1`
Expected: `ok`.

If the `**BREAKING**` assertion fails because Lip Gloss renders `Warn.Bold(true)` with its codes in another order, compare instead against `st.Warn.Bold(true).Render("BREAKING")` built the same way in `mdWords`. Don't loosen the test into a plain-text check.

- [ ] **Step 5: Lint**

Run: `golangci-lint run ./...`
Expected: `0 issues.`

- [ ] **Step 6: Commit**

```bash
git add internal/ui/notesmd.go internal/ui/notesmd_test.go
git commit -m "feat(ui): draw release notes' markdown in the theme's styles"
```

---

### Task 5: The Outdated details pane, and `w`

**Files:**
- Create: `internal/ui/notes.go`
- Create: `internal/ui/notes_test.go`
- Modify: `internal/ui/app.go` (fields, `New`, `update`, `sync`, `scrollDetails`, `rowCount`)
- Modify: `internal/ui/columns.go` (`hasDetails`)
- Modify: `internal/ui/actions.go` (`outdatedKey`, `tabHints`)
- Modify: `cmd/brewery/main.go`
- Golden: `internal/ui/testdata/golden/outdated.txt`

**Interfaces:**
- Consumes:
  - `notes.Notes`, `notes.Fetcher`, `notes.Fetch` and `notes.Breaking` (Task 3);
  - `renderNotes` (Task 4);
  - `github.SourceRepo` (Task 1);
  - `brew.Package.Sources` (Task 2).
- Produces, for Tasks 6 and 7:
  - `type ReleaseFetcher = notes.Fetcher` and `(*App).UseReleases(f ReleaseFetcher)`;
  - `(*App).outdatedSelected() (brew.Outdated, brew.Package, bool)`;
  - `notesRepo(p brew.Package) (owner, repo string, ok bool)`;
  - `(*App).keptNotes(o brew.Outdated) (notes.Notes, bool)`;
  - `hasBreaking(n notes.Notes) bool`;
  - the test helpers `fakeReleases`, `luaReleases` and `notesApp`.

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/notes_test.go`:

```go
package ui

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/github"
	"github.com/dillo/brewery/internal/state"
)

// fakeReleases answers release and tag requests from tables, by
// owner/repo, and records what it was asked.
type fakeReleases struct {
	mu    sync.Mutex
	asked []string
	rels  map[string][]github.Release
	tags  map[string][]string
	err   error // every answer's, when set
}

func (f *fakeReleases) Releases(_ context.Context, owner, repo string) ([]github.Release, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, "releases "+owner+"/"+repo)
	return f.rels[owner+"/"+repo], f.err
}

func (f *fakeReleases) Tags(_ context.Context, owner, repo string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, "tags "+owner+"/"+repo)
	return f.tags[owner+"/"+repo], f.err
}

// luaRepo is where the fixture's lua-language-server (3.19.1, outdated by
// 3.19.2) comes from, by its homepage.
const luaRepo = "LuaLS/lua-language-server"

// luaReleases are its releases: one in range, one past it, and the one
// installed.
var luaReleases = []github.Release{
	{Tag: "3.19.3", Body: "past the upgrade"},
	{Tag: "3.19.2", URL: "https://github.com/LuaLS/lua-language-server/releases/tag/3.19.2",
		Published: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC),
		Body:      "## Changes\n* **Breaking**: removed `--old`\n* fixed a crash"},
	{Tag: "3.19.1", Body: "already installed"},
}

// notesApp is the fixture app on Outdated, wide enough for the details,
// with lua-language-server selected and f answering for GitHub.
func notesApp(t *testing.T, f *fakeReleases) *App {
	t.Helper()
	a := newJobApp(t, &fakeStarter{})
	if f != nil {
		a.UseReleases(f)
	}
	a.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	selectOutdated(t, a, "lua-language-server")
	return a
}

func mustShow(t *testing.T, a *App, wants ...string) {
	t.Helper()
	out := stripANSI(a.render())
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("the screen lacks %q:\n%s", w, out)
		}
	}
}

func mustNotShow(t *testing.T, a *App, unwanted ...string) {
	t.Helper()
	out := stripANSI(a.render())
	for _, w := range unwanted {
		if strings.Contains(out, w) {
			t.Errorf("the screen shows %q:\n%s", w, out)
		}
	}
}

func TestWhatsNewBeforeFetching(t *testing.T) {
	a := notesApp(t, &fakeReleases{})
	mustShow(t, a, "lua-language-server 3.19.1 → 3.19.2", luaRepo, "w shows what's new, from GitHub")
}

func TestWhatsNewFetches(t *testing.T) {
	f := &fakeReleases{rels: map[string][]github.Release{luaRepo: luaReleases}}
	a := notesApp(t, f)
	keys(t, a, "w")
	mustShow(t, a, "1 release since 3.19.1", "fetched just now", "mentions breaking changes",
		"3.19.2", "2026-10-10", "• Breaking: removed --old", "• fixed a crash")
	mustNotShow(t, a, "past the upgrade", "already installed")
	keys(t, a, "w")
	if len(f.asked) != 1 || f.asked[0] != "releases "+luaRepo {
		t.Errorf("notes kept for these versions aren't fetched again: %v", f.asked)
	}
	saved, err := state.Load(a.cachePath)
	if n, ok := saved.ReleaseNotes["formula/lua-language-server"]; err != nil || !ok || len(n.Releases) != 1 {
		t.Errorf("the notes should be saved: %+v, %v", saved.ReleaseNotes, err)
	}
}

func TestWhatsNewNewerVersion(t *testing.T) {
	a := notesApp(t, &fakeReleases{rels: map[string][]github.Release{luaRepo: luaReleases}})
	keys(t, a, "w")
	a.snap.Outdated[0].Current = "3.19.3"
	a.sync()
	mustShow(t, a, "lua-language-server 3.19.1 → 3.19.3", "w shows what's new, from GitHub")
	if _, ok := a.snap.ReleaseNotes["formula/lua-language-server"]; !ok {
		t.Error("notes for a package still outdated are kept")
	}
	a.snap.Outdated = a.snap.Outdated[1:]
	a.sync()
	if _, ok := a.snap.ReleaseNotes["formula/lua-language-server"]; ok {
		t.Error("notes for a package no longer outdated are dropped")
	}
}

func TestWhatsNewNoReleasesInRange(t *testing.T) {
	f := &fakeReleases{tags: map[string][]string{luaRepo: {"3.19.2", "3.19.1"}}}
	a := notesApp(t, f)
	keys(t, a, "w")
	mustShow(t, a, "no release notes between these versions", "gx opens it")
	want := "https://github.com/LuaLS/lua-language-server/compare/3.19.1...3.19.2"
	if got := a.snap.ReleaseNotes["formula/lua-language-server"].Link; got != want {
		t.Errorf("link %q, want %q", got, want)
	}
}

func TestWhatsNewFailure(t *testing.T) {
	f := &fakeReleases{err: github.ErrRateLimited}
	a := notesApp(t, f)
	keys(t, a, "w")
	mustShow(t, a, "couldn't fetch: rate limit reached", "w tries again")
	if _, ok := a.snap.ReleaseNotes["formula/lua-language-server"]; ok {
		t.Error("a failure isn't kept")
	}
	f.err, f.rels = nil, map[string][]github.Release{luaRepo: luaReleases}
	keys(t, a, "w")
	mustShow(t, a, "1 release since 3.19.1")
	mustNotShow(t, a, "couldn't fetch")
}

func TestWhatsNewNotOnGitHub(t *testing.T) {
	f := &fakeReleases{}
	a := notesApp(t, f)
	keys(t, a, "j") // docker-desktop, from www.docker.com
	mustShow(t, a, "release notes come from GitHub", "www.docker.com", "gx opens the homepage")
	keys(t, a, "w")
	if len(f.asked) != 0 || !strings.Contains(stripANSI(a.statusBar()), "docker-desktop's release notes aren't on GitHub") {
		t.Errorf("w on a package not on GitHub only notes: %v %q", f.asked, stripANSI(a.statusBar()))
	}
}

func TestWhatsNewOnlyNotes(t *testing.T) {
	a := notesApp(t, nil)
	keys(t, a, "w")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "release notes aren't available") {
		t.Errorf("no fetcher: %q", bar)
	}

	f := &fakeReleases{}
	a = notesApp(t, f)
	keys(t, a, "space", "w")
	if bar := stripANSI(a.statusBar()); len(f.asked) != 0 || !strings.Contains(bar, "w shows one package at a time") {
		t.Errorf("with marks: %v %q", f.asked, bar)
	}

	a = notesApp(t, f)
	a.fetchingNotes, a.fetchingName = "formula/fd", "fd"
	keys(t, a, "w")
	if bar := stripANSI(a.statusBar()); len(f.asked) != 0 || !strings.Contains(bar, "already asking GitHub about fd") {
		t.Errorf("while fetching: %v %q", f.asked, bar)
	}
	mustShow(t, a, "w shows what's new")
	a.fetchingNotes = "formula/lua-language-server"
	if !regexp.MustCompile(`asking GitHub…`).MatchString(stripANSI(a.render())) {
		t.Errorf("the pane says it's asking:\n%s", stripANSI(a.render()))
	}
}

func TestWhatsNewLongNotes(t *testing.T) {
	body := strings.Repeat("* a line\n", noteLines+5)
	rels := []github.Release{{Tag: "3.19.2", Body: body, URL: "https://example.com/r"}}
	a := notesApp(t, &fakeReleases{rels: map[string][]github.Release{luaRepo: rels}})
	keys(t, a, "w")
	lines := a.releaseLines(rels[0], 80)
	if len(lines) != noteLines+2 || !strings.Contains(stripANSI(lines[noteLines]), "… 5 more lines") || stripANSI(lines[noteLines+1]) != "https://example.com/r" {
		t.Errorf("long notes end after %d lines with how many more and where: %q", noteLines, lines[noteLines:])
	}
	if got := a.releaseLines(github.Release{Tag: "x"}, 80); len(got) != 1 || stripANSI(got[0]) != "no notes" {
		t.Errorf("empty notes: %q", got)
	}
}

func TestWhatsNewScrolls(t *testing.T) {
	body := strings.Repeat("* a line\n", 30)
	rels := []github.Release{{Tag: "3.19.2", Body: body}}
	a := notesApp(t, &fakeReleases{rels: map[string][]github.Release{luaRepo: rels}})
	keys(t, a, "w", "l", "ctrl+d")
	if a.outdated.pv.off == 0 {
		t.Error("j/k and ctrl+d scroll the details once they have the focus")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "j/k scroll") {
		t.Errorf("the hints say so: %q", bar)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/ui -run TestWhatsNew -count=1`
Expected: the build fails with `a.UseReleases undefined`, `a.fetchingNotes undefined`, `undefined: noteLines` and `a.releaseLines undefined`.

- [ ] **Step 3: Write `notes.go`**

Create `internal/ui/notes.go`:

```go
package ui

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/github"
	"github.com/dillo/brewery/internal/notes"
)

// ReleaseFetcher asks GitHub for a repository's releases and tags.
// *github.Client satisfies it; tests use a fake.
type ReleaseFetcher = notes.Fetcher

// UseReleases sets how Outdated fetches release notes from GitHub.
func (a *App) UseReleases(f ReleaseFetcher) { a.releases = f }

// notesMsg carries one fetch's notes, or why it failed.
type notesMsg struct {
	key   string
	notes notes.Notes
	err   error
}

// noteLines is the most of one release's notes the details show.
const noteLines = 40

// outdatedSelected is the selected Outdated row and its package. ok is
// false when no row is shown.
func (a *App) outdatedSelected() (brew.Outdated, brew.Package, bool) {
	i := a.outdated.cursor()
	if i < 0 {
		return brew.Outdated{}, brew.Package{}, false
	}
	o := a.snap.Outdated[i]
	return o, a.packageNamed(o.Name, o.Kind), true
}

// notesKey is the key o's notes are kept under: its package's key, or
// its kind and name when the package isn't known.
func (a *App) notesKey(o brew.Outdated) string {
	if p := a.packageNamed(o.Name, o.Kind); p.FullName != "" {
		return p.Key()
	}
	return o.Kind.String() + "/" + o.Name
}

// notesRepo is the GitHub repository p's release notes come from: the
// first of its download addresses, then its homepage, that is on GitHub.
func notesRepo(p brew.Package) (owner, repo string, ok bool) {
	for _, addr := range append(slices.Clone(p.Sources), p.Homepage) {
		if owner, repo, ok = github.SourceRepo(addr); ok {
			return owner, repo, true
		}
	}
	return "", "", false
}

// keptNotes are the notes fetched for o's upgrade, when they're for the
// versions it goes between now.
func (a *App) keptNotes(o brew.Outdated) (notes.Notes, bool) {
	n, ok := a.snap.ReleaseNotes[a.notesKey(o)]
	return n, ok && n.From == o.Installed && n.To == o.Current
}

// hasBreaking reports whether any of n's releases mentions a breaking
// change.
func hasBreaking(n notes.Notes) bool {
	return slices.ContainsFunc(n.Releases, func(r github.Release) bool { return notes.Breaking(r.Body) })
}

// whatsNew fetches the selected upgrade's release notes from GitHub,
// unless they're kept already.
func (a *App) whatsNew() tea.Cmd {
	if a.marks() > 0 {
		return a.note("w shows one package at a time")
	}
	o, p, ok := a.outdatedSelected()
	if !ok {
		return nil
	}
	owner, repo, onGitHub := notesRepo(p)
	switch {
	case a.releases == nil:
		return a.note("release notes aren't available")
	case a.fetchingNotes != "":
		return a.note("already asking GitHub about " + a.fetchingName)
	case !onGitHub:
		return a.note(o.Name + "'s release notes aren't on GitHub")
	}
	if _, kept := a.keptNotes(o); kept {
		return nil // already showing
	}
	key := a.notesKey(o)
	a.fetchingNotes, a.fetchingName = key, o.Name
	delete(a.notesErr, key)
	ctx, f, now := a.ctx, a.releases, a.now
	name, from, to := o.Name, o.Installed, o.Current
	return func() tea.Msg {
		n, err := notes.Fetch(ctx, f, owner, repo, name, from, to, now())
		return notesMsg{key: key, notes: n, err: err}
	}
}

// gotNotes keeps a fetch's notes and saves them, or remembers why it
// failed until the next try. The details start again at the top.
func (a *App) gotNotes(m notesMsg) tea.Cmd {
	a.fetchingNotes, a.fetchingName = "", ""
	a.outdated.pv.off = 0
	if m.err != nil {
		a.notesErr[m.key] = m.err.Error()
		return nil
	}
	if a.snap.ReleaseNotes == nil {
		a.snap.ReleaseNotes = map[string]notes.Notes{}
	}
	a.snap.ReleaseNotes[m.key] = m.notes
	return save(a.cachePath, a.snap)
}

// pruneNotes forgets the notes of packages no longer outdated.
func (a *App) pruneNotes() {
	if len(a.snap.ReleaseNotes) == 0 {
		return
	}
	keep := map[string]bool{}
	for _, o := range a.snap.Outdated {
		keep[a.notesKey(o)] = true
	}
	maps.DeleteFunc(a.snap.ReleaseNotes, func(k string, _ notes.Notes) bool { return !keep[k] })
}

// notesPreview is Outdated's details: what's new in the selected upgrade,
// or how to see it.
func (a *App) notesPreview(w, h int) (title, body string) {
	o, p, ok := a.outdatedSelected()
	if !ok {
		return "Details", ""
	}
	iw, _ := paneInner(w, h)
	return o.Name + " " + o.Installed + " → " + o.Current, ansi.Wrap(a.notesBody(o, p, iw), iw, "")
}

// notesBody is the details' text for o, w wide.
func (a *App) notesBody(o brew.Outdated, p brew.Package, w int) string {
	st := a.st
	key := a.notesKey(o)
	owner, repo, onGitHub := notesRepo(p)
	n, kept := a.keptNotes(o)
	switch {
	case !onGitHub:
		return a.notOnGitHub(o, p)
	case a.fetchingNotes == key:
		return "asking GitHub…"
	case a.notesErr[key] != "":
		return st.Warn.Render("couldn't fetch: "+ansi.Truncate(a.notesErr[key], reasonW, "…")) + "\n" + st.Muted.Render("w tries again")
	case !kept:
		return st.Muted.Render(owner+"/"+repo) + "\n\n" + "w shows what's new, from GitHub"
	case len(n.Releases) == 0:
		return "no release notes between these versions\n" + st.Muted.Render(n.Link) + "\n" + st.Muted.Render("gx opens it")
	}
	return a.releasesBody(n, o, w)
}

// notOnGitHub says why a package has no release notes to show.
func (a *App) notOnGitHub(o brew.Outdated, p brew.Package) string {
	st := a.st
	from := "brew doesn't say where " + o.Name + " comes from"
	addr := p.Homepage
	if len(p.Sources) > 0 {
		addr = p.Sources[0]
	}
	if u, err := url.Parse(addr); err == nil && u.Host != "" {
		from = o.Name + " comes from " + u.Host
	}
	body := st.Muted.Render("release notes come from GitHub, and " + from)
	if webPage(p.Homepage) {
		body += "\n" + st.Muted.Render("gx opens the homepage")
	}
	return body
}

// releasesBody shows each release in n, newest first, after how many
// there are and whether any mentions a breaking change.
func (a *App) releasesBody(n notes.Notes, o brew.Outdated, w int) string {
	st := a.st
	lines := []string{plural(len(n.Releases), "release") + " since " + o.Installed + " · fetched " + ago(a.now(), n.At)}
	if hasBreaking(n) {
		lines = append(lines, st.Warn.Render("mentions breaking changes"))
	}
	for _, r := range n.Releases {
		lines = append(lines, "", st.Title.Render(r.Tag))
		if !r.Published.IsZero() {
			lines = append(lines, st.Muted.Render(r.Published.In(a.now().Location()).Format("2006-01-02")))
		}
		lines = append(lines, a.releaseLines(r, w)...)
	}
	return strings.Join(lines, "\n")
}

// releaseLines are one release's notes, w wide: at most noteLines of
// them, then how many more there are and where to read them.
func (a *App) releaseLines(r github.Release, w int) []string {
	st := a.st
	body := renderNotes(st, r.Body, w)
	switch {
	case len(body) == 0:
		return []string{st.Muted.Render("no notes")}
	case len(body) <= noteLines:
		return body
	}
	more := st.Muted.Render(fmt.Sprintf("… %d more lines", len(body)-noteLines))
	return append(body[:noteLines:noteLines], more, st.Muted.Render(r.URL))
}
```

- [ ] **Step 4: Wire it into the App**

In `internal/ui/app.go`:

1. In `App`, after `checkingTaps bool`, add:

```go
	releases      ReleaseFetcher    // asks GitHub for release notes, or nil
	fetchingNotes string            // the key of the package whose notes are being fetched, or ""
	fetchingName  string            // that package's name, for notes
	notesErr      map[string]string // why each package's last fetch failed, until it's tried again
```

2. In `New`'s `a := &App{…}` literal, after `acts: map[int]action{}, …` line, add `notesErr: map[string]string{},` on its own line.

3. In `New`, after `a.outdated.note = a.dataAge`, add:

```go
	a.outdated.preview = a.notesPreview
```

4. In `update`, after `case tapsCheckedMsg: return a.gotTapChecks(msg)`, add:

```go
	case notesMsg:
		return a.gotNotes(msg)
```

5. In `sync`, before `a.outdated.setRows(…)`, add:

```go
	a.pruneNotes()
```

6. In `scrollDetails`, after `case tabInstalled: a.installed.scroll(k)`, add:

```go
	case tabOutdated:
		a.outdated.scroll(k)
```

7. In `rowCount`, after the `tabDiscover` case, add:

```go
	case tabOutdated:
		return a.outdated.n
```

In `internal/ui/columns.go`, change `hasDetails` to:

```go
func hasDetails(tab int) bool {
	switch tab {
	case tabInstalled, tabOutdated, tabDiscover, tabServices, tabHistory, tabMaintenance:
		return true
	}
	return false
}
```

Also change the comment in `columns_test.go` at line 41 from `// no details (Home, Outdated, Taps)` to `// no details (Home, Taps)`.

In `internal/ui/actions.go`:

1. In `outdatedKey`, add a case before `case "f":`:

```go
	case "w":
		return a.whatsNew(), true
```

2. In `tabHints`, replace `case tabOutdated: return "enter actions · space mark · A upgrade all"` with:

```go
	case tabOutdated:
		if a.focus == colDetails {
			return "enter actions · j/k scroll · h back to the list"
		}
		return "enter actions · space mark · A upgrade all"
```

In `cmd/brewery/main.go`, replace `app.UseTapChecker(github.NewClient()) // only used when the user runs Check taps` with:

```go
	gh := github.NewClient() // only used when the user asks: Check taps, and What's new
	app.UseTapChecker(gh)
	app.UseReleases(gh)
```

- [ ] **Step 5: Run the new tests to see them pass**

Run: `gofmt -w internal/ui cmd/brewery && go test ./internal/ui -run 'TestWhatsNew|TestRenderNotes' -count=1`
Expected: `ok`.

- [ ] **Step 6: Run everything**

Run: `go test ./... -count=1`
Expected:
- `TestScreens` fails on `outdated.txt` only: Outdated now has details.
- Any other failure is a test that assumed Outdated has no details pane. For each one:
  - read it;
  - if it only assumed the old layout, update its expectation;
  - if the behaviour is really wrong, fix the code.
  - Note each one in the commit message body.

- [ ] **Step 7: Update the golden**

Run: `go test ./internal/ui -run TestScreens -update -count=1 && git diff --stat internal/ui/testdata/golden`
Expected:
- Only `internal/ui/testdata/golden/outdated.txt` changed.
- Read `git diff internal/ui/testdata/golden/outdated.txt`. It must show a details pane titled `lua-language-server 3.19.1 → 3.19.2` containing `LuaLS/lua-language-server` and `w shows what's new, from GitHub`.

- [ ] **Step 8: Run everything, lint and coverage**

Run: `go test ./... -count=1 && golangci-lint run ./... && make cover`
Expected: all `ok`, `0 issues.`, and coverage at 90% or above per package.

- [ ] **Step 9: Commit**

```bash
git add internal/ui cmd/brewery
git commit -m "feat(ui): show what's new in an upgrade, from GitHub, beside Outdated"
```

---

### Task 6: The menu, help, `gx`, and warnings before upgrading

**Files:**
- Modify: `internal/ui/menu.go` (`outdatedMenu`, `copyItems`)
- Modify: `internal/ui/help.go` (the Outdated group)
- Modify: `internal/ui/links.go` (`selection`)
- Modify: `internal/ui/notes.go` (`breakingIn`, `breakingText`, `openLabel`)
- Modify: `internal/ui/actions.go` (`askUpgradeAll`)
- Modify: `internal/ui/marks.go` (`upgradeMarked`)
- Test: `internal/ui/notes_test.go`

**Interfaces:**
- Consumes, from Task 5: `outdatedSelected`, `notesRepo`, `keptNotes`, `hasBreaking`, `notesApp`, `fakeReleases`, `luaReleases` and `luaRepo`.
- Produces: `(*App).breakingIn(ts []target) []string` and `breakingText(names []string) string`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/notes_test.go`:

```go
func TestWhatsNewMenu(t *testing.T) {
	a := notesApp(t, &fakeReleases{rels: map[string][]github.Release{luaRepo: luaReleases}})
	keys(t, a, "enter")
	if !regexp.MustCompile(`Fetch 3\.19\.2\s+f`).MatchString(stripANSI(a.render())) ||
		!regexp.MustCompile(`What's new\s+w`).MatchString(stripANSI(a.render())) {
		t.Errorf("the menu offers What's new after Fetch:\n%s", stripANSI(a.render()))
	}
	keys(t, a, "w") // picks it from the menu
	mustShow(t, a, "1 release since 3.19.1")
	keys(t, a, "enter")
	mustShow(t, a, "What's new (fetched)")
	keys(t, a, "esc", "j", "enter") // docker-desktop, not on GitHub
	mustNotShow(t, a, "What's new")
}

func TestWhatsNewHelp(t *testing.T) {
	a := notesApp(t, nil)
	keys(t, a, "?")
	mustShow(t, a, "what's new, from GitHub")
}

func TestWhatsNewOpensTheLink(t *testing.T) {
	a := notesApp(t, &fakeReleases{tags: map[string][]string{luaRepo: {"3.19.2", "3.19.1"}}})
	var opened string
	a.opener = func(u string) error { opened = u; return nil }
	keys(t, a, "w", "enter")
	mustShow(t, a, "Open on GitHub")
	keys(t, a, "esc", "g", "x")
	if want := "https://github.com/LuaLS/lua-language-server/compare/3.19.1...3.19.2"; opened != want {
		t.Errorf("gx opened %q, want %q", opened, want)
	}
	opened = ""
	keys(t, a, "j", "g", "x") // docker-desktop: its homepage
	if opened != "https://www.docker.com/products/docker-desktop" {
		t.Errorf("gx opened %q, want the homepage", opened)
	}
}

func TestUpgradeAllWarnsOfBreakingChanges(t *testing.T) {
	f := &fakeReleases{rels: map[string][]github.Release{luaRepo: luaReleases}}
	a := notesApp(t, f)
	keys(t, a, "A")
	mustNotShow(t, a, "breaking changes")
	keys(t, a, "esc", "w", "A")
	mustShow(t, a, "release notes mention breaking changes") // the names are cut to the box's width
	if len(f.asked) != 1 {
		t.Errorf("Upgrade all never fetches: %v", f.asked)
	}
}

func TestUpgradeMarkedWarnsOfBreakingChanges(t *testing.T) {
	a := notesApp(t, &fakeReleases{rels: map[string][]github.Release{luaRepo: luaReleases}})
	keys(t, a, "w", "space", "u")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "release notes mention breaking changes: lua-language-server") {
		t.Errorf("the note warns: %q", bar)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/ui -run 'TestWhatsNew(Menu|Help|OpensTheLink)|BreakingChanges' -count=1`
Expected: FAIL, each on its missing text (`What's new`, `what's new, from GitHub`, `Open on GitHub`, `breaking changes`).

- [ ] **Step 3: Implement**

In `internal/ui/notes.go`, add at the end:

```go
// breakingIn names the targets whose kept release notes, for the upgrade
// brew would make now, mention a breaking change. It never fetches.
func (a *App) breakingIn(ts []target) []string {
	var names []string
	for _, o := range a.snap.Outdated {
		aimed := slices.ContainsFunc(ts, func(t target) bool {
			return t.kind == o.Kind && (t.name == o.Name || strings.HasSuffix(t.name, "/"+o.Name))
		})
		if n, kept := a.keptNotes(o); aimed && kept && hasBreaking(n) {
			names = append(names, o.Name)
		}
	}
	return names
}

// breakingText warns that names' release notes mention breaking changes.
func breakingText(names []string) string {
	return "release notes mention breaking changes: " + strings.Join(names, ", ")
}

// openLabel names what gx opens on the current row: the GitHub page that
// stands in for missing release notes, or the homepage.
func (a *App) openLabel(addr string) string {
	if a.tab == tabOutdated {
		if o, _, ok := a.outdatedSelected(); ok {
			if n, kept := a.keptNotes(o); kept && n.Link != "" && n.Link == addr {
				return "Open on GitHub"
			}
		}
	}
	return "Open the homepage"
}
```

In `internal/ui/links.go`, replace the `tabOutdated` case of `selection` with:

```go
	case tabOutdated:
		if o, p, ok := a.outdatedSelected(); ok {
			if n, kept := a.keptNotes(o); kept && len(n.Releases) == 0 && n.Link != "" {
				return o.Name, n.Link, true // the page that stands in for the notes
			}
			return o.Name, p.Homepage, true
		}
```

In `internal/ui/menu.go`:

1. In `copyItems`, replace `items = append(items, a.keyItem("gx", "Open the homepage"))` with:

```go
		items = append(items, a.keyItem("gx", a.openLabel(url)))
```

2. In `outdatedMenu`, replace `items = append(items, a.keyItem("f", "Fetch "+o.Current), a.keyItem("A", "Upgrade all…"))` with:

```go
	items = append(items, a.keyItem("f", "Fetch "+o.Current))
	if _, _, onGitHub := notesRepo(a.packageNamed(o.Name, o.Kind)); onGitHub {
		label := "What's new"
		if _, kept := a.keptNotes(o); kept {
			label = "What's new (fetched)"
		}
		items = append(items, a.keyItem("w", label))
	}
	items = append(items, a.keyItem("A", "Upgrade all…"))
```

In `internal/ui/help.go`, change the Outdated group to:

```go
		here = helpGroup{"Outdated", [][2]string{
			{"enter", "actions"}, {"u", "upgrade"}, {"f", "fetch the newer version"}, {"w", "what's new, from GitHub"},
			{"A", "upgrade all"}, {"space", "mark for several"}, {"l tab", "details"},
		}}
```

In `internal/ui/actions.go`, in `askUpgradeAll`:
- In the first loop, also build the targets. Add `var ts []target` before the loop, and `ts = append(ts, target{o.Name, o.Kind})` next to `names = append(names, o.Name)`.
- After the `if len(refused) > 0 { … }` block, add:

```go
	if breaking := a.breakingIn(ts); len(breaking) > 0 {
		body = append(body, a.st.Warn.Render(breakingText(breaking)))
	}
```

In `internal/ui/marks.go`, replace `upgradeMarked`'s body from `a.clearMarks()` to the end with:

```go
	breaking := a.breakingIn(ts)
	a.clearMarks()
	if why != "" || len(breaking) > 0 {
		a.flash = "" // so only what run() sets below is kept
	}
	cmd := a.byKind(ts, func(kind brew.Kind, names []string) action {
		return action{title: "upgrade " + strings.Join(names, " "), args: kindArgs("upgrade", kind, names...), refresh: diskQueries}
	})
	var warns []string
	if why != "" {
		warns = append(warns, why)
	}
	if len(breaking) > 0 {
		warns = append(warns, breakingText(breaking))
	}
	if len(warns) > 0 {
		note := a.st.Warn.Render(strings.Join(warns, " · "))
		if a.flash != "" { // run() queued the job or couldn't start it
			note = a.flash + " · " + note
		}
		a.flash = note
	}
	return cmd
```

- [ ] **Step 4: Run the tests to see them pass**

Run: `gofmt -w internal/ui && go test ./internal/ui -count=1`
Expected:
- `ok`.
- If `TestScreens` fails, a golden shows a menu or help on Outdated. None of the current screens does, so investigate rather than update.

- [ ] **Step 5: Lint and coverage**

Run: `golangci-lint run ./... && make cover`
Expected: `0 issues.`, and 90% or above.

- [ ] **Step 6: Commit**

```bash
git add internal/ui
git commit -m "feat(ui): What's new in the menu and help, and warnings before upgrading"
```

---

### Task 7: Goldens, the demo and the README

**Files:**
- Modify: `internal/ui/golden_test.go`
- Golden: `internal/ui/testdata/golden/outdated-notes.txt` (new)
- Modify: `internal/ui/testdata/demo/info.json` (gh's `urls`)
- Modify: `internal/ui/screenshots_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: `fakeReleases`, `luaReleases`, `luaRepo` and `selectOutdated`.

- [ ] **Step 1: Add the golden screen**

In `internal/ui/golden_test.go`'s `TestScreens`:
- After `a.UseBrewVersion("4.6.3")`, add:

```go
	a.UseReleases(&fakeReleases{rels: map[string][]github.Release{luaRepo: luaReleases}})
```

- Add the import `"github.com/dillo/brewery/internal/github"`.
- In the screens table, after `{"outdated", []string{"Outdated"}, nil},`, add:

```go
		{"outdated-notes", []string{"Outdated"}, []string{"w"}}, // lua-language-server's release notes
```

- [ ] **Step 2: Write it and check it**

Run: `go test ./internal/ui -run TestScreens -update -count=1 && git status --short internal/ui/testdata/golden`
Expected:
- Only `?? internal/ui/testdata/golden/outdated-notes.txt` is listed.
- Open it. The details show:
  - `1 release since 3.19.1 · fetched just now` (perhaps wrapped);
  - `mentions breaking changes`;
  - `3.19.2`, `2026-10-10`, `• Breaking: removed --old` and `• fixed a crash`.

Then run `go test ./internal/ui -run TestScreens -count=1`. Expected: `ok`.

- [ ] **Step 3: Give the demo machine release notes**

In `internal/ui/testdata/demo/info.json`, in the `gh` formula (around line 292), after its `"homepage": "https://cli.github.com/",` line, add:

```json
   "urls": {
    "stable": {
     "url": "https://github.com/cli/cli/archive/refs/tags/v2.102.0.tar.gz"
    }
   },
```

Its homepage, `cli.github.com`, isn't a repository. So this also shows that brewery finds a repository from the download address.

In `internal/ui/screenshots_test.go`:

1. After `demoNow`, add:

```go
// ghReleases are gh's releases on the demo machine: the one it would
// upgrade to, and the one installed.
var ghReleases = []github.Release{
	{Tag: "v2.102.0", URL: "https://github.com/cli/cli/releases/tag/v2.102.0", Published: demoNow.Add(-50 * time.Hour),
		Body: "## What's Changed\n" +
			"### ✨ Features\n" +
			"* `gh pr revert` reverts a merged pull request\n" +
			"* `gh run watch` shows each job's steps as they finish\n" +
			"### 🐛 Fixes\n" +
			"* `gh auth status` no longer hangs without a network\n" +
			"* `gh extension install` checks the archive it downloads\n" +
			"### Breaking changes\n" +
			"* `gh repo sync --force` is removed: use `gh repo sync --hard`\n"},
	{Tag: "v2.101.0", Body: "installed"},
}
```

2. In `demoApp`, after `a.detailsDelay = 0`, add:

```go
	a.UseReleases(&fakeReleases{rels: map[string][]github.Release{"cli/cli": ghReleases}})
```

3. In `TestScreenshots`, after the Maintenance shot (`shoot(t, a, "maintenance")`), add:

```go
	selectOutdated(t, a, "gh") // what's new in gh 2.102.0
	keys(t, a, "w")
	shoot(t, a, "whats-new")
```

Run: `go test -tags screenshots -count=1 -run TestScreenshots ./internal/ui`
Expected:
- `ok`.
- `docs/screenshots/whats-new.ansi` is written, and `cat docs/screenshots/whats-new.ansi` shows gh's notes.
- The existing `upgrade.ansi` now has the Outdated details beside the list, as expected.

Then run `make screenshots`. This needs Charm's `freeze` and the JetBrains Mono Nerd Font (see the Makefile).
- **If it runs:** `docs/screenshots/whats-new.png` is new, and `upgrade.png` has changed.
- **If `freeze` or the font is missing:** don't install anything. Skip the PNGs and the README image in Step 4, and say so in the commit message and your report.

The demo video (`make demo`) isn't re-recorded here.

- [ ] **Step 4: Document it**

In `README.md`, under **Packages**, after the `Uninstall, with an offer…` bullet, add:

```markdown
- What's new: press `w` on an outdated package to read the release notes
  from GitHub for every version you'd be skipping over, newest first. Words
  like *breaking* and *removed* are highlighted, and Upgrade all warns when
  notes you've read mention them. Brewery asks GitHub only when you press
  `w`, sends it only the repository's name, and keeps what it fetched.
  Packages that aren't on GitHub link to their homepage instead.
```

If `whats-new.png` was made in Step 3, add this under **A look around**, after the menu screenshot's paragraph:

```markdown
What's new in an upgrade, from the project's release notes on GitHub.

![What's new in gh 2.102.0: its features, fixes and a breaking change, highlighted, beside the Outdated list](docs/screenshots/whats-new.png)
```

- [ ] **Step 5: Run everything, lint and coverage**

Run: `go test ./... -count=1 && golangci-lint run ./... && make cover`
Expected: all `ok`, `0 issues.`, and 90% or above.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/golden_test.go internal/ui/testdata README.md internal/ui/screenshots_test.go docs/screenshots/*.png
git commit -m "docs: What's new in the goldens, the screenshots and the README"
```

---

## Spec coverage

| Spec section | Task |
|---|---|
| When it runs (`w`, menu, marks, kept) | 5, 6 |
| The repository (stable, head, homepage; cask url, homepage) | 1, 2, 5 |
| The releases (one request, pre-releases) | 1, 3 |
| Which releases (clean, tags, comparison, names) | 3 |
| When nothing matches (tags, compare or releases page) | 3, 5 |
| Failures (rate limit, gone, other; not kept) | 1, 5 |
| The Outdated details pane (every situation) | 5 |
| Scrolling, long notes | 5 |
| Rendering the notes, breaking words | 4 |
| Upgrade all and marked upgrades | 6 |
| The action menu, help | 6 |
| What's kept (snapshot v9, pruning, 16 KB) | 3, 5 |
| Code shape | 1–6 |
| Testing, goldens, demo | 1–7 |

**Deviations from the spec:**
- **`Sources`, not `Source`.** `brew.Package` gets `Sources []string` (download addresses only) instead of `Source string`. A single address can't express "the first of these on GitHub". The homepage is already on `Package`, and is tried after the sources.
- **No source at all.** When a package has neither a download address nor a homepage, the pane says `brew doesn't say where <name> comes from`. The spec doesn't cover this case.
- **Bare `w` on a row not on GitHub.** The menu leaves the item out there, as the spec says. A bare `w` notes `<name>'s release notes aren't on GitHub`, which the spec doesn't specify.
- **The `gx` menu label.** When `gx` would open the compare or releases page, its menu item reads `Open on GitHub` instead of `Open the homepage`.
- **Failed tags request.** When the tags request fails, the pane still offers the releases page, so it isn't an error.
