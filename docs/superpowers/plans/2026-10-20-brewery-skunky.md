# Skunky Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Brewery runs `brew vulns --json` at most once a day (or on `r` in the view), and shows installed formulae with known vulnerabilities:
- a new **Skunky** sidebar view, worst first, with a SEVERITY column and a coverage note;
- a red `!N` chip in the sidebar when anything is HIGH or CRITICAL;
- a `Vulnerabilities` section in Installed's details.

**Architecture:**
- **brew:** a new `internal/brew/vulns.go` holds the types, `ParseVulns` and `(*Client).Vulns`.
- **state:** the snapshot keeps the last scan and when it ran.
- **ui:** a new `internal/ui/skunky.go` holds everything Skunky-specific: severity ranking and colour, version matching, the badge, the scheduling and the notes. Installed gains a `filterSkunky` view and the details section. The scan is a new query, `qVulns`, that joins the slow pass after the JSON queries when it's due.

**Tech Stack:** Go 1.26, Bubble Tea v2, Lip Gloss v2, golangci-lint v2.

**Spec:** `docs/superpowers/specs/2026-10-20-brewery-skunky-design.md`

## Global Constraints

- **Branch:** work on `feat/skunky`. Commits use the repo's configured identity (`4197+dillo@users.noreply.github.com`).
- **Lint:**
  - `.golangci.yml` is fixed (gocognit and gocyclo `min-complexity: 20`, tests excluded). Don't edit it, and don't add `//nolint`.
  - `golangci-lint run ./...` must report `0 issues.` at the end of every task.
- **Coverage:** `make cover` stays at 90% or above in total and per package.
- **Goldens:**
  - Update only with `go test ./internal/ui -run TestScreens -update -count=1`, and only in the tasks that say so.
  - Read `git diff internal/ui/testdata/golden` first, and check that only what the task lists changed.
- **Style:** match the surrounding code's comment density, naming and idioms. Every function gets a short doc comment.
- **Exact texts:** user-facing strings are exactly as written here.
- **Commits:** one commit per task, with the message given.

## Files

| File | Change |
|---|---|
| `internal/brew/vulns.go` | new: `ErrNoVulns`, `VulnScan`, `VulnFinding`, `Vuln`, `ParseVulns`, `(*Client).Vulns` |
| `internal/brew/vulns_test.go` | new |
| `internal/brew/testdata/vulns.json` | new fixture |
| `internal/brew/client_test.go` | `fakeRunner` can set stderr |
| `internal/brew/live_test.go` | a live check of `brew vulns` |
| `internal/state/snapshot.go` | `Vulns`, `VulnsAt`; version 8 |
| `internal/state/snapshot_test.go` | round trip covers them |
| `internal/ui/skunky.go` | new |
| `internal/ui/skunky_test.go` | new |
| `internal/ui/installed.go` | `filterSkunky`, findings, SEVERITY column, sorting, cursor by package, notes, details |
| `internal/ui/sidebar.go` | the Skunky entry and badge |
| `internal/ui/source.go` | `Source.Vulns`, `qVulns` |
| `internal/ui/app.go` | apply the scan, schedule it, `r` on Skunky |
| `internal/ui/help.go` | `r` on Skunky |
| `internal/ui/helpers_test.go`, `app_test.go`, `screenshots_test.go`, `golden_test.go` | fixtures and fakes |
| `README.md` | the view in Features and Sidebar |

---

### Task 1: brew reads `brew vulns`

**Files:**
- Create: `internal/brew/vulns.go`, `internal/brew/vulns_test.go`, `internal/brew/testdata/vulns.json`
- Modify: `internal/brew/client_test.go` (`fakeRunner`), `internal/brew/live_test.go`

**Interfaces:**
- Produces:
  - `var ErrNoVulns error`
  - `type VulnScan struct { Findings []VulnFinding; Checked int; Skipped []string }`
  - `type VulnFinding struct { Formula, Version string; Vulns []Vuln }`
  - `type Vuln struct { ID, Severity, Summary string; Fixed []string }`
  - `func ParseVulns(b []byte) (VulnScan, error)`
  - `func (c *Client) Vulns(ctx context.Context) (VulnScan, error)`

- [ ] **Step 1: Add the fixture**

`internal/brew/testdata/vulns.json`. Its formulae are the other fixtures' (`info.json`):
- git's finding is for 2.50.0, not the 2.51.0 installed;
- pcre2 has only `patched` findings;
- json-c has three: a fix in a commit, a fix in a version and a commit, and no fix;
- lua-language-server is HIGH, fixed in a version newer than brew's latest;
- fd was skipped.

```json
{
  "findings": [
    {
      "formula": "git",
      "version": "2.50.0",
      "tag": "v2.50.0",
      "repo_url": "https://github.com/git/git",
      "vulnerabilities": [
        {"id": "CVE-2025-48386", "severity": "HIGH", "summary": "Credential helper overflow", "aliases": [], "fixed_versions": ["2.50.1"]}
      ],
      "patched": [
        {"id": "CVE-2025-48384", "severity": "HIGH", "summary": "Carriage return in submodule path", "aliases": [], "fixed_versions": ["2.50.1"]}
      ]
    },
    {
      "formula": "pcre2",
      "version": "10.49",
      "tag": "pcre2-10.49",
      "repo_url": "https://github.com/PCRE2Project/pcre2",
      "vulnerabilities": [],
      "patched": [
        {"id": "OSV-2025-101", "severity": "HIGH", "summary": "Heap-buffer-overflow in match", "aliases": [], "fixed_versions": ["0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c"]}
      ]
    },
    {
      "formula": "json-c",
      "version": "0.19",
      "tag": "json-c-0.19",
      "repo_url": "https://github.com/json-c/json-c",
      "vulnerabilities": [
        {"id": "CVE-2026-31337", "severity": "UNKNOWN", "summary": "Stack exhaustion in tokener", "aliases": [], "fixed_versions": ["a1b2c3d4e5f60718293a4b5c6d7e8f9012345678"]},
        {"id": "OSV-2024-512", "severity": "MEDIUM", "summary": "Integer overflow in printbuf", "aliases": [], "fixed_versions": ["0.20", "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c"]},
        {"id": "OSV-2023-77", "severity": "LOW", "summary": "Uninitialised read in printer", "aliases": [], "fixed_versions": []}
      ],
      "patched": []
    },
    {
      "formula": "lua-language-server",
      "version": "3.19.0",
      "tag": "3.19.0",
      "repo_url": "https://github.com/LuaLS/lua-language-server",
      "vulnerabilities": [
        {"id": "GHSA-7f3x-lua-0001", "severity": "HIGH", "summary": "Path traversal in workspace", "aliases": [], "fixed_versions": ["3.20.0"]}
      ],
      "patched": []
    }
  ],
  "skipped_formulae": ["fd"],
  "checked": 4,
  "complete": false,
  "outdated_without_sbom": [],
  "unscanned_formulae": {}
}
```

- [ ] **Step 2: Let `fakeRunner` set stderr**

In `internal/brew/client_test.go`, `fakeRunner` gains a `stderr` map, used in place of the default text when a command fails:

```go
// fakeRunner returns canned output keyed by the joined argument list.
// A failing command writes stderr's text for it, or "Error: something
// broke".
type fakeRunner struct {
	out    map[string][]byte
	err    map[string]error
	stderr map[string]string
	delay  time.Duration
}

func (f fakeRunner) Run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
	key := strings.Join(args, " ")
	if err := f.err[key]; err != nil {
		errb := "Error: something broke"
		if s, ok := f.stderr[key]; ok {
			errb = s
		}
		return f.out[key], []byte(errb), err
	}
	return f.out[key], nil, nil
}
```

- [ ] **Step 3: Write the failing tests**

`internal/brew/vulns_test.go`:

```go
package brew

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseVulns(t *testing.T) {
	scan, err := ParseVulns(fixture(t, "vulns.json"))
	if err != nil {
		t.Fatal(err)
	}
	if scan.Checked != 4 || !slices.Equal(scan.Skipped, []string{"fd"}) {
		t.Errorf("checked %d, skipped %v; want 4 and [fd]", scan.Checked, scan.Skipped)
	}
	var names []string
	for _, f := range scan.Findings {
		names = append(names, f.Formula+" "+f.Version)
	}
	if want := []string{"git 2.50.0", "json-c 0.19", "lua-language-server 3.19.0"}; !slices.Equal(names, want) {
		t.Errorf("findings %v, want %v: pcre2 has only patched ones", names, want)
	}
	want := Vuln{ID: "OSV-2024-512", Severity: "MEDIUM", Summary: "Integer overflow in printbuf",
		Fixed: []string{"0.20", "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c"}}
	if jc := scan.Findings[1]; len(jc.Vulns) != 3 || !reflect.DeepEqual(jc.Vulns[1], want) {
		t.Errorf("json-c's findings: %+v", jc.Vulns)
	}
	if git := scan.Findings[0]; len(git.Vulns) != 1 || git.Vulns[0].ID != "CVE-2025-48386" {
		t.Errorf("git keeps only its open finding, not the patched one: %+v", git.Vulns)
	}
}

func TestParseVulnsBadJSON(t *testing.T) {
	if _, err := ParseVulns([]byte("not json")); err == nil {
		t.Error("want an error for output that isn't JSON")
	}
}

func TestClientVulns(t *testing.T) {
	exitErr := exec.Command("false").Run() // a real *exec.ExitError with status 1
	const key = "vulns --json"
	ctx := context.Background()

	// brew vulns exits 1 whenever it finds anything.
	c := NewClient(fakeRunner{out: map[string][]byte{key: fixture(t, "vulns.json")}, err: map[string]error{key: exitErr}})
	if scan, err := c.Vulns(ctx); err != nil || len(scan.Findings) != 3 {
		t.Errorf("exit 1 with JSON: %d findings, %v; want 3 and no error", len(scan.Findings), err)
	}
	c = NewClient(fakeRunner{out: map[string][]byte{key: fixture(t, "vulns.json")}})
	if scan, err := c.Vulns(ctx); err != nil || scan.Checked != 4 {
		t.Errorf("exit 0: %+v, %v", scan, err)
	}

	c = NewClient(fakeRunner{err: map[string]error{key: exitErr}})
	if _, err := c.Vulns(ctx); err == nil || !strings.Contains(err.Error(), "something broke") {
		t.Errorf("exit 1 without JSON should be an error with brew's stderr: %v", err)
	} else if _, ok := errors.AsType[*QueryError](err); !ok {
		t.Errorf("want *QueryError, got %T", err)
	}

	c = NewClient(fakeRunner{err: map[string]error{key: exitErr},
		stderr: map[string]string{key: "Error: Invalid usage: Unknown command: brew vulns"}})
	if _, err := c.Vulns(ctx); !errors.Is(err, ErrNoVulns) {
		t.Errorf("a brew without vulns: %v, want ErrNoVulns", err)
	}

	c = NewClient(fakeRunner{out: map[string][]byte{key: fixture(t, "vulns.json")}, err: map[string]error{key: errors.New("fork failed")}})
	if _, err := c.Vulns(ctx); err == nil {
		t.Error("brew not running at all is an error, whatever it printed")
	}

	c = NewClient(fakeRunner{delay: time.Second})
	c.Timeout = 10 * time.Millisecond
	if _, err := c.Vulns(ctx); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("want a timeout, got %v", err)
	}
}
```

- [ ] **Step 4: Run them to see them fail**

Run: `go test ./internal/brew -run 'Vulns' -count=1`
Expected: FAIL to build: `undefined: ParseVulns`.

- [ ] **Step 5: Implement**

`internal/brew/vulns.go`:

```go
package brew

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNoVulns means this brew has no vulns command: it's older than the
// scanner.
var ErrNoVulns = errors.New("brew has no vulns command")

// VulnScan is what `brew vulns --json` found in the installed formulae.
type VulnScan struct {
	Findings []VulnFinding `json:"findings"` // formulae with open findings
	Checked  int           `json:"checked"`  // how many formulae were looked up
	Skipped  []string      `json:"skipped"`  // ones brew couldn't scan
}

// VulnFinding is one formula's open findings, at the version scanned.
type VulnFinding struct {
	Formula string `json:"formula"`
	Version string `json:"version"` // as brew vulns gives it: without brew's revision
	Vulns   []Vuln `json:"vulns"`
}

// Vuln is one known vulnerability, as OSV.dev records it.
type Vuln struct {
	ID       string   `json:"id"`
	Severity string   `json:"severity"` // CRITICAL, HIGH, MEDIUM, LOW or UNKNOWN, as brew gives it
	Summary  string   `json:"summary"`
	Fixed    []string `json:"fixed"` // versions or commit hashes that fix it
}

type vulnsJSON struct {
	Findings []struct {
		Formula         string     `json:"formula"`
		Version         string     `json:"version"`
		Vulnerabilities []vulnJSON `json:"vulnerabilities"`
	} `json:"findings"`
	Checked int      `json:"checked"`
	Skipped []string `json:"skipped_formulae"`
}

type vulnJSON struct {
	ID            string   `json:"id"`
	Severity      string   `json:"severity"`
	Summary       string   `json:"summary"`
	FixedVersions []string `json:"fixed_versions"`
}

// ParseVulns parses `brew vulns --json`. Findings that one of brew's
// patches already fixes (its "patched" list) aren't kept, nor are formulae
// left with no open findings.
func ParseVulns(b []byte) (VulnScan, error) {
	var in vulnsJSON
	if err := json.Unmarshal(jsonPayload(b), &in); err != nil {
		return VulnScan{}, fmt.Errorf("parse brew vulns: %w", err)
	}
	scan := VulnScan{Checked: in.Checked, Skipped: in.Skipped}
	for _, f := range in.Findings {
		if len(f.Vulnerabilities) == 0 {
			continue
		}
		vf := VulnFinding{Formula: f.Formula, Version: f.Version}
		for _, v := range f.Vulnerabilities {
			vf.Vulns = append(vf.Vulns, Vuln{ID: v.ID, Severity: v.Severity, Summary: v.Summary, Fixed: v.FixedVersions})
		}
		scan.Findings = append(scan.Findings, vf)
	}
	return scan, nil
}

// Vulns runs `brew vulns --json`, which looks the installed formulae up on
// OSV.dev. It exits 1 whenever it finds anything, so a non-zero exit is a
// result as long as its output parses. A brew without the command gives
// ErrNoVulns.
func (c *Client) Vulns(ctx context.Context) (VulnScan, error) {
	args := []string{"vulns", "--json"}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	out, errb, err := c.r.Run(ctx, args...)
	if err != nil && ctx.Err() != nil {
		return VulnScan{}, &QueryError{Args: args, Stderr: string(errb), Err: fmt.Errorf("timed out after %s", c.Timeout)}
	}
	if strings.Contains(string(errb), "Unknown command") {
		return VulnScan{}, ErrNoVulns
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return VulnScan{}, &QueryError{Args: args, Stderr: string(errb), Err: err}
	}
	scan, perr := ParseVulns(out)
	if perr != nil {
		if err == nil {
			err = perr
		}
		return VulnScan{}, &QueryError{Args: args, Stderr: string(errb), Err: err}
	}
	return scan, nil
}
```

- [ ] **Step 6: Add a live check**

In `internal/brew/live_test.go`, append a test of its own, so its 10-odd seconds and the call to OSV.dev stay out of `TestLiveQueries`:

```go
// TestLiveVulns scans with the real brew vulns, which asks OSV.dev.
func TestLiveVulns(t *testing.T) {
	bin, err := Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	c := NewClient(ExecRunner{Bin: bin, Env: QueryEnv})
	c.Timeout = 2 * time.Minute
	scan, err := c.Vulns(context.Background())
	if errors.Is(err, ErrNoVulns) {
		t.Skip("this brew has no vulns command")
	}
	if err != nil {
		t.Fatal(err)
	}
	if scan.Checked == 0 {
		t.Error("no formulae checked")
	}
	for _, f := range scan.Findings {
		if f.Formula == "" || f.Version == "" || len(f.Vulns) == 0 {
			t.Errorf("incomplete finding: %+v", f)
		}
		for _, v := range f.Vulns {
			if v.ID == "" || v.Severity == "" {
				t.Errorf("%s: incomplete vulnerability: %+v", f.Formula, v)
			}
		}
	}
}
```

Add `"errors"` to that file's imports.

- [ ] **Step 7: Run the tests**

Run: `go test ./internal/brew -count=1 && go test -tags live -count=1 -run TestLiveVulns ./internal/brew/`
Expected: `ok` for both (the live one scans this Mac).

- [ ] **Step 8: Lint and commit**

Run: `golangci-lint run ./...` → `0 issues.`

```bash
git add internal/brew
git commit -m "feat(brew): read brew vulns: open findings by formula, and what it couldn't scan"
```

---

### Task 2: the snapshot keeps the scan

**Files:**
- Modify: `internal/state/snapshot.go`, `internal/state/snapshot_test.go`

**Interfaces:**
- Consumes: `brew.VulnScan` (Task 1).
- Produces: `Snapshot.Vulns *brew.VulnScan` (nil: never scanned) and `Snapshot.VulnsAt time.Time`.

- [ ] **Step 1: Write the failing test**

In `TestSaveLoadRoundTrip` (`internal/state/snapshot_test.go`), add to `in`:

```go
		Vulns: &brew.VulnScan{Checked: 2, Skipped: []string{"fd"}, Findings: []brew.VulnFinding{
			{Formula: "json-c", Version: "0.19", Vulns: []brew.Vuln{{ID: "OSV-1", Severity: "HIGH", Summary: "s", Fixed: []string{"0.20"}}}},
		}},
		VulnsAt: time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC),
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/state -count=1`
Expected: FAIL to build: `unknown field Vulns in struct literal`.

- [ ] **Step 3: Implement**

In `internal/state/snapshot.go`, set `const snapshotVersion = 8`, and add after `TapsCheckedAt`:

```go
	Vulns           *brew.VulnScan       `json:"vulns"`           // what brew vulns last found; nil if it never ran
	VulnsAt         time.Time            `json:"vulns_at"`        // when it ran
```

Run `gofmt -w internal/state/snapshot.go` to realign the struct.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/state -count=1`
Expected: `ok`.

- [ ] **Step 5: Lint and commit**

Run: `golangci-lint run ./...` → `0 issues.`

```bash
git add internal/state
git commit -m "feat(state): keep the last vulnerability scan and when it ran"
```

---

### Task 3: the Skunky view and its badge

**Files:**
- Create: `internal/ui/skunky.go`, `internal/ui/skunky_test.go`
- Modify: `internal/ui/installed.go`, `internal/ui/sidebar.go`, `internal/ui/app.go` (`sync`), `internal/ui/helpers_test.go` (`fixtureSnapshot`), goldens

**Interfaces:**
- Consumes: `Snapshot.Vulns` (Task 2), `brew.VulnFinding` (Task 1).
- Produces:
  - `filterSkunky installedFilter`
  - `severityRanks map[string]int`, `severityRank(s string) int`, `worst(f brew.VulnFinding) string`, `serious(s string) bool`, `severityText(st theme.Styles, s string) string`, `upstreamVersion(v string) string`
  - `(t *installedTab) setVulns(scan *brew.VulnScan)`, `(t *installedTab) finding(p brew.Package) (brew.VulnFinding, bool)`
  - `(a *App) skunkyBadge() (n int, chip string)`
  - test helper `rowNames(a *App) []string`

- [ ] **Step 1: Load the fixture scan in `fixtureSnapshot`**

In `internal/ui/helpers_test.go`, before `return s` in `fixtureSnapshot`:

```go
	scan, err := brew.ParseVulns(readFixture(t, "vulns.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.Vulns, s.VulnsAt = &scan, testNow.Add(-3*time.Hour)
```

Add `"time"` to its imports.

- [ ] **Step 2: Write the failing tests**

`internal/ui/skunky_test.go`:

```go
package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/dillo/brewery/internal/brew"
)

// rowNames are the names Installed lists now, in order.
func rowNames(a *App) []string {
	var names []string
	for _, p := range a.installed.rows {
		names = append(names, p.Name)
	}
	return names
}

// setVersion changes the installed version of the package named name.
func setVersion(a *App, name, v string) {
	for i, p := range a.snap.Packages {
		if p.Name == name {
			a.snap.Packages[i].Version = v
		}
	}
	a.sync()
}

func TestSkunkyListsFindingsWorstFirst(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Skunky")
	if got, want := rowNames(a), []string{"lua-language-server", "json-c"}; !slices.Equal(got, want) {
		t.Errorf("Skunky lists %v, want %v: git's finding is for 2.50.0, not the 2.51.0 installed", got, want)
	}
	out := stripANSI(a.render())
	for _, s := range []string{"Installed formulae with known security problems, worst first.", "SEVERITY", "HIGH", "MEDIUM"} {
		if !strings.Contains(out, s) {
			t.Errorf("Skunky should show %q:\n%s", s, out)
		}
	}
	show(t, a, "All")
	if out := stripANSI(a.render()); strings.Contains(out, "SEVERITY") || !strings.Contains(out, "TYPE") {
		t.Errorf("other views keep TYPE:\n%s", out)
	}
}

func TestSkunkyMatchesTheInstalledVersion(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	setVersion(a, "json-c", "0.19_1")              // a rebuild: brew vulns says 0.19
	setVersion(a, "lua-language-server", "3.19.2") // upgraded since the scan
	show(t, a, "Skunky")
	if got := rowNames(a); !slices.Equal(got, []string{"json-c"}) {
		t.Errorf("Skunky lists %v, want only json-c", got)
	}
}

func TestUpstreamVersion(t *testing.T) {
	for in, want := range map[string]string{
		"0.12.0_1": "0.12.0", "3.8.13_12": "3.8.13", "2.51.0": "2.51.0", "1.0_beta": "1.0_beta", "_1": "_1",
	} {
		if got := upstreamVersion(in); got != want {
			t.Errorf("upstreamVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSeverityRank(t *testing.T) {
	order := []string{"CRITICAL", "HIGH", "MEDIUM", "UNKNOWN", "LOW"}
	for i := 1; i < len(order); i++ {
		if severityRank(order[i-1]) >= severityRank(order[i]) {
			t.Errorf("%s should rank above %s", order[i-1], order[i])
		}
	}
	if severityRank("MODERATE") != severityRank("UNKNOWN") || severityRank("high") != severityRank("HIGH") {
		t.Error("an unrecognised severity ranks with UNKNOWN, and case doesn't matter")
	}
	f := brew.VulnFinding{Vulns: []brew.Vuln{{Severity: "LOW"}, {Severity: "UNKNOWN"}, {Severity: "MEDIUM"}}}
	if got := worst(f); got != "MEDIUM" {
		t.Errorf("worst = %q, want MEDIUM", got)
	}
	if !serious("CRITICAL") || !serious("HIGH") || serious("MEDIUM") || serious("UNKNOWN") {
		t.Error("serious means HIGH or CRITICAL")
	}
}

func TestSkunkyBadge(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	skunky := entries[entryIndex("Skunky")]
	if n, chip := a.badgeCount(skunky); n != -1 || strings.TrimSpace(stripANSI(chip)) != "!2" {
		t.Errorf("with a HIGH finding: %d %q, want the chip !2", n, stripANSI(chip))
	}
	setVersion(a, "lua-language-server", "3.19.2")
	if n, chip := a.badgeCount(skunky); n != 1 || chip != "" {
		t.Errorf("with MEDIUM at worst: %d %q, want a count of 1", n, chip)
	}
	a.snap.Vulns = nil
	a.sync()
	if n, chip := a.badgeCount(skunky); n != -1 || chip != "" {
		t.Errorf("before any scan: %d %q, want nothing", n, chip)
	}
}

func TestSkunkyKeepsTheSelectedPackage(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "All")
	for i, p := range a.installed.rows {
		if p.Name == "json-c" {
			a.installed.table.SetCursor(i)
		}
	}
	show(t, a, "Skunky")
	if p, _ := a.installed.selected(); p.Name != "json-c" {
		t.Errorf("selected %s after moving to Skunky, want json-c", p.Name)
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/ui -run 'Skunky|UpstreamVersion|SeverityRank' -count=1`
Expected: FAIL to build: `undefined: upstreamVersion` (and others).

- [ ] **Step 4: Add `internal/ui/skunky.go`**

```go
package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/theme"
)

// severityRanks order brew vulns' severities, worst first. UNKNOWN means
// not scored yet, not minor, so it sits above LOW.
var severityRanks = map[string]int{"CRITICAL": 0, "HIGH": 1, "MEDIUM": 2, "UNKNOWN": 3, "LOW": 4}

// severityRank is s's place in severityRanks. Anything else brew prints
// ranks with UNKNOWN.
func severityRank(s string) int {
	if r, ok := severityRanks[strings.ToUpper(s)]; ok {
		return r
	}
	return severityRanks["UNKNOWN"]
}

// worst is the most severe of f's findings.
func worst(f brew.VulnFinding) string {
	var w string
	for i, v := range f.Vulns {
		if i == 0 || severityRank(v.Severity) < severityRank(w) {
			w = v.Severity
		}
	}
	return w
}

// serious reports whether s is HIGH or CRITICAL.
func serious(s string) bool { return severityRank(s) <= severityRanks["HIGH"] }

// severityText is s coloured by how bad it is: red from HIGH up, yellow
// for MEDIUM, muted for the rest.
func severityText(st theme.Styles, s string) string {
	switch {
	case serious(s):
		return theme.Tint(st.P.Red, s)
	case severityRank(s) == severityRanks["MEDIUM"]:
		return theme.Tint(st.P.Yellow, s)
	}
	return st.Muted.Render(s)
}

// upstreamVersion drops brew's revision from an installed version
// (0.12.0_1 is 0.12.0): brew vulns reports versions without it.
func upstreamVersion(v string) string {
	if i := strings.LastIndexByte(v, '_'); i > 0 {
		if _, err := strconv.Atoi(v[i+1:]); err == nil {
			return v[:i]
		}
	}
	return v
}

// setVulns keeps scan's findings by formula, for finding. A nil scan
// (none yet) keeps none.
func (t *installedTab) setVulns(scan *brew.VulnScan) {
	t.vulns = nil
	if scan == nil {
		return
	}
	t.vulns = make(map[string]brew.VulnFinding, len(scan.Findings))
	for _, f := range scan.Findings {
		t.vulns[f.Formula] = f
	}
}

// finding is the last scan's open findings for p, when they are for the
// version installed now. After an upgrade they no longer are, and p waits
// for the next scan to judge it.
func (t *installedTab) finding(p brew.Package) (brew.VulnFinding, bool) {
	if p.Kind != brew.KindFormula {
		return brew.VulnFinding{}, false
	}
	f, ok := t.vulns[p.FullName]
	if !ok {
		f, ok = t.vulns[p.Name]
	}
	if !ok || f.Version != upstreamVersion(p.Version) {
		return brew.VulnFinding{}, false
	}
	return f, true
}

// worstRank is the rank of p's worst finding, for Skunky's order.
func (t *installedTab) worstRank(p brew.Package) int {
	f, _ := t.finding(p)
	return severityRank(worst(f))
}

// skunkyBadge is Skunky's sidebar count: the formulae with findings, as a
// red chip when any is HIGH or CRITICAL, and nothing before the first scan.
func (a *App) skunkyBadge() (n int, chip string) {
	if a.snap.Vulns == nil {
		return -1, ""
	}
	bad := false
	for _, p := range a.installed.all {
		if f, ok := a.installed.finding(p); ok {
			n++
			bad = bad || serious(worst(f))
		}
	}
	if bad {
		return -1, a.st.ChipErr.Render(fmt.Sprintf("!%d", n))
	}
	return n, ""
}
```

- [ ] **Step 5: Add the view to `internal/ui/installed.go`**

1. Add `filterSkunky` after `filterDeprecated` in the `installedFilter` constants, and to `views`:

```go
	filterSkunky:     {"Skunky", "Installed formulae with known security problems, worst first."},
```

2. Add a field to `installedTab`, after `sizes`:

```go
	vulns  map[string]brew.VulnFinding // the last scan's findings, by formula; nil before one
```

3. In `shows`, add a case:

```go
	case filterSkunky:
		_, ok := t.finding(p)
		return ok
```

4. Replace `apply` and delete `installedKey` (the TYPE cell is a severity on Skunky, so rows can no longer be keyed by it; the selection follows the package instead):

```go
func (t *installedTab) apply() {
	before := t.selectedKey()
	t.rows = t.rows[:0]
	for _, p := range t.all {
		if t.shows(t.filter, p) && matches(p.Name, t.match) {
			t.rows = append(t.rows, p)
		}
	}
	sort.SliceStable(t.rows, t.less)
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{marked(p.Name, t.marked[pkgKey(p)]), p.Version, t.typeCell(p), status(t.st, p), t.size(p)}
	}
	tc := t.table.Columns()
	tc[2].Title = t.cols()[2].title
	t.table.SetColumns(tc)
	setRows(&t.table, rows)
	t.selectKey(before)
	t.followSelection(before)
}

// less orders rows by name, a formula before a cask of the same name; on
// Skunky, by worst finding first.
func (t *installedTab) less(i, j int) bool {
	p, q := t.rows[i], t.rows[j]
	if t.filter == filterSkunky {
		if rp, rq := t.worstRank(p), t.worstRank(q); rp != rq {
			return rp < rq
		}
	}
	if p.Name != q.Name {
		return p.Name < q.Name
	}
	return p.Kind < q.Kind
}

// selectKey moves the cursor to the package whose pkgKey is key, if the
// view lists it.
func (t *installedTab) selectKey(key string) {
	for i, p := range t.rows {
		if pkgKey(p) == key {
			t.table.SetCursor(i)
			return
		}
	}
}

// cols are Installed's columns. On Skunky, every row is a formula, so
// TYPE gives way to SEVERITY.
func (t *installedTab) cols() []col {
	if t.filter != filterSkunky {
		return installedCols
	}
	c := slices.Clone(installedCols)
	c[2].title = "SEVERITY"
	return c
}

// typeCell is the TYPE column's cell: on Skunky, the worst finding's
// severity.
func (t *installedTab) typeCell(p brew.Package) string {
	if t.filter == filterSkunky {
		f, _ := t.finding(p)
		return severityText(t.st, worst(f))
	}
	return kindCell(t.st.Icons, p.Kind)
}
```

Add `"slices"` to the imports.

5. In `resize`, size with `t.cols()` in place of `installedCols`:

```go
	sizeTable(&t.table, t.cols(), iw, max(ih-aboutHeight, 0))
```

- [ ] **Step 6: Wire the sidebar and `sync`**

In `internal/ui/sidebar.go`, add after the Deprecated entry:

```go
	{"Skunky", secPackages, tabInstalled, int(filterSkunky)},
```

and at the top of `badgeCount`'s `case tabInstalled:`:

```go
	case tabInstalled:
		if installedFilter(e.view) == filterSkunky {
			return a.skunkyBadge()
		}
		if a.loaded[qInstalled] {
```

In `internal/ui/app.go`'s `sync`, before `a.installed.setData(...)`:

```go
	a.installed.setVulns(a.snap.Vulns)
```

- [ ] **Step 7: Run the new tests**

Run: `go test ./internal/ui -run 'Skunky|UpstreamVersion|SeverityRank|Sidebar|Deprecated' -count=1`
Expected: `ok`.

- [ ] **Step 8: Update the goldens**

Run: `go test ./internal/ui -count=1`
Expected: only `TestScreens` fails, because every screen's sidebar gains `Skunky  !2` under Deprecated.

Run: `go test ./internal/ui -run TestScreens -update -count=1 && git diff internal/ui/testdata/golden`
Check: the only changes are the sidebar's new line (and Help's background, if Help shows the sidebar). If anything else changed, stop and fix the cause, not the golden.

Run: `go test ./... -count=1`
Expected: `ok` for every package.

- [ ] **Step 9: Lint and commit**

Run: `golangci-lint run ./...` → `0 issues.`

```bash
git add internal/ui
git commit -m "feat(ui): a Skunky view of formulae with known vulnerabilities, worst first"
```

---

### Task 4: scan daily and on `r`, with the notes

**Files:**
- Modify: `internal/ui/source.go`, `internal/ui/app.go`, `internal/ui/skunky.go`, `internal/ui/installed.go`, `internal/ui/app_test.go` (`fakeSource`), `internal/ui/screenshots_test.go` (`demoSource`), `internal/ui/skunky_test.go`

**Interfaces:**
- Consumes: `(*brew.Client).Vulns`, `brew.ErrNoVulns` (Task 1); `skunkyBadge`, `filterSkunky` (Task 3).
- Produces:
  - `Source.Vulns(ctx context.Context) (brew.VulnScan, error)`
  - `qVulns query`
  - `App.forceVulns`, `App.noVulns`
  - `(a *App) slowQueries() []query`, `(a *App) vulnsDue() bool`
  - `(a *App) vulnsNote() string`, `(a *App) vulnsEmpty() string`
  - `installedTab.scanNote`, `installedTab.scanEmpty func() string`
  - test fakes: `fakeSource.vulnsErr`, `(*countingSource).Vulns`, `newScanApp`

- [ ] **Step 1: Give the fakes `Vulns`**

In `internal/ui/app_test.go`, add a field to `fakeSource` (and mention it in its comment: "servicesErr, …, serviceFilesErr and vulnsErr make one query fail"):

```go
	vulnsErr        error
```

and a method next to the others:

```go
func (f *fakeSource) Vulns(context.Context) (brew.VulnScan, error) {
	if f.vulnsErr != nil {
		return brew.VulnScan{}, f.vulnsErr
	}
	if f.snap.Vulns == nil {
		return brew.VulnScan{}, nil
	}
	return *f.snap.Vulns, nil
}
```

In `internal/ui/screenshots_test.go`, next to `demoSource`'s other methods:

```go
func (d demoSource) Vulns(context.Context) (brew.VulnScan, error) { return brew.VulnScan{}, nil }
```

- [ ] **Step 2: Write the failing tests**

Append to `internal/ui/skunky_test.go` (and add `"errors"`, `"path/filepath"`, `"time"`, `tea "charm.land/bubbletea/v2"` and `"github.com/dillo/brewery/internal/state"` to its imports):

```go
func (c *countingSource) Vulns(ctx context.Context) (brew.VulnScan, error) {
	c.calls["vulns"]++
	return c.fakeSource.Vulns(ctx)
}

// newScanApp is an app on the fixtures whose last scan was ago before now,
// or that has never scanned when scanned is false. It counts brew calls.
func newScanApp(t *testing.T, ago time.Duration, scanned bool) (*App, *countingSource) {
	t.Helper()
	src := newCountingSource(t)
	snap := fixtureSnapshot(t)
	snap.VulnsAt = testNow.Add(-ago)
	if !scanned {
		snap.Vulns, snap.VulnsAt = nil, time.Time{}
	}
	a := New(src, nil, snap, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return a, src
}

func TestVulnsScanAtMostDaily(t *testing.T) {
	for _, c := range []struct {
		name    string
		ago     time.Duration
		scanned bool
		want    int
	}{
		{"never scanned", 0, false, 1},
		{"23 hours ago", 23 * time.Hour, true, 0},
		{"25 hours ago", 25 * time.Hour, true, 1},
	} {
		a, src := newScanApp(t, c.ago, c.scanned)
		drain(t, a, a.refresh())
		if src.calls["vulns"] != c.want {
			t.Errorf("%s: %d scans, want %d", c.name, src.calls["vulns"], c.want)
		}
	}
	a, _ := newScanApp(t, 25*time.Hour, true)
	drain(t, a, a.refresh())
	saved, err := state.Load(a.cachePath)
	if err != nil || saved.Vulns == nil || !saved.VulnsAt.Equal(testNow) {
		t.Errorf("the scan should be saved with its time: %v %+v %v", err, saved.Vulns, saved.VulnsAt)
	}
}

func TestROnSkunkyScansNow(t *testing.T) {
	a, src := newScanApp(t, time.Hour, true)
	show(t, a, "Top-level")
	keys(t, a, "r")
	if src.calls["vulns"] != 0 {
		t.Errorf("r elsewhere keeps the daily rule: %d scans", src.calls["vulns"])
	}
	show(t, a, "Skunky")
	keys(t, a, "r")
	keys(t, a, "r")
	if src.calls["vulns"] != 2 {
		t.Errorf("r on Skunky scans every time: %d scans, want 2", src.calls["vulns"])
	}
}

func TestNoVulnsCommandStopsAsking(t *testing.T) {
	a, src := newScanApp(t, 0, false)
	src.vulnsErr = brew.ErrNoVulns
	drain(t, a, a.refresh())
	drain(t, a, a.refresh())
	if src.calls["vulns"] != 1 {
		t.Errorf("%d scans, want 1: a brew without vulns isn't asked again", src.calls["vulns"])
	}
	if got, want := a.vulnsNote(), "brew vulns isn't available · U updates brew"; got != want {
		t.Errorf("note %q, want %q", got, want)
	}
	if got, want := a.vulnsEmpty(), "no scan to show"; got != want {
		t.Errorf("empty note %q, want %q", got, want)
	}
}

func TestFailedScanKeepsTheLastOne(t *testing.T) {
	a, src := newScanApp(t, 50*time.Hour, true)
	src.vulnsErr = errors.New("no network")
	drain(t, a, a.refresh())
	if n := a.installed.count(filterSkunky); n != 2 {
		t.Errorf("Skunky lists %d after a failed scan, want the last scan's 2", n)
	}
	if got, want := a.vulnsNote(), "couldn't scan: no network · showing the scan from 2d ago"; got != want {
		t.Errorf("note %q, want %q", got, want)
	}
	if _, chip := a.badgeCount(entries[entryIndex("Skunky")]); !strings.Contains(stripANSI(chip), "⚠") {
		t.Errorf("badge %q, want the stale chip", stripANSI(chip))
	}
	if !a.snap.VulnsAt.Equal(testNow.Add(-50 * time.Hour)) {
		t.Errorf("a failed scan mustn't move VulnsAt: %v", a.snap.VulnsAt)
	}
	drain(t, a, a.refresh())
	if src.calls["vulns"] != 2 {
		t.Errorf("%d scans, want 2: the next refresh tries again", src.calls["vulns"])
	}
	a.snap.Vulns = nil
	if got, want := a.vulnsNote(), "couldn't scan: no network"; got != want {
		t.Errorf("note with no earlier scan %q, want %q", got, want)
	}
}

func TestSkunkyNotes(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	if got, want := a.vulnsNote(), "scanned 3h ago · 4 of 5 formulae checked · casks aren't scanned"; got != want {
		t.Errorf("note %q, want %q", got, want)
	}
	show(t, a, "Skunky")
	if out := stripANSI(a.render()); !strings.Contains(out, "scanned 3h ago · 4 of 5 formulae checked") {
		t.Errorf("Skunky should show its note:\n%s", out)
	}
	a.snap.Vulns = &brew.VulnScan{Checked: 4}
	a.sync()
	if out := stripANSI(a.render()); !strings.Contains(out, "no known vulnerabilities in the 4 formulae checked") {
		t.Errorf("a clean scan's empty note:\n%s", out)
	}
	a.snap.Vulns = nil
	a.sync()
	if got, want := a.vulnsEmpty(), "not scanned yet · r scans now"; got != want {
		t.Errorf("empty note %q, want %q", got, want)
	}
	if got := a.vulnsNote(); got != "" {
		t.Errorf("no note before any scan, got %q", got)
	}
	a.inflight[qVulns] = true
	if got, want := a.vulnsEmpty(), "checking with brew vulns…"; got != want {
		t.Errorf("empty note while scanning %q, want %q", got, want)
	}
}
```

Also add `"context"` to the imports.

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/ui -run 'Vulns|Skunky|Scan' -count=1`
Expected: FAIL to build: `undefined: qVulns` (and others).

- [ ] **Step 4: Add the query to `internal/ui/source.go`**

- `Source` gains `Vulns(ctx context.Context) (brew.VulnScan, error)` after `DataUpdated`.
- `qVulns` goes after `qServiceFiles` in the `query` constants, and `"vulns"` at the end of `queryNames`.
- `resultMsg` gains `vulns brew.VulnScan` after `serviceFiles`.
- `fetch` gains:

```go
		case qVulns:
			m.vulns, m.err = src.Vulns(ctx)
```

- `jsonQueries`' comment becomes: "jsonQueries are the queries a full refresh runs in parallel; doctor, cleanup's and autoremove's dry-runs, the service files, the disk sizes and, when it's due, the vulnerability scan follow once they have all returned."

- [ ] **Step 5: Apply and schedule it in `internal/ui/app.go`**

1. `App` gains, after `doctorAfter`:

```go
	forceVulns  bool           // r on Skunky: scan in the next slow pass, however recent the last scan
	noVulns     bool           // brew has no vulns command: don't ask again
```

2. In `New`, after `a.loaded[qServiceFiles] = …`:

```go
	a.loaded[qVulns] = snap.Vulns != nil
```

and next to `a.outdated.note = a.dataAge`:

```go
	a.installed.scanNote, a.installed.scanEmpty = a.vulnsNote, a.vulnsEmpty
```

3. In `apply`, before `if m.err != nil {`:

```go
	if errors.Is(m.err, brew.ErrNoVulns) {
		a.noVulns = true
	}
```

and a case in its switch:

```go
	case qVulns:
		a.snap.Vulns, a.snap.VulnsAt = &m.vulns, a.now()
```

4. In `handleResult`, the slow pass becomes:

```go
		doctor = a.request(a.slowQueries()...)
```

5. In `globalKey`'s `case "r":`, before `return a.refresh(), true`:

```go
		if a.tab == tabInstalled && a.installed.filter == filterSkunky {
			a.forceVulns = true // refreshing the view refreshes what it shows
		}
```

Add `"errors"` to the imports.

- [ ] **Step 6: Add the scheduling and notes to `internal/ui/skunky.go`**

Add `"time"` and `"github.com/charmbracelet/x/ansi"` to its imports, then:

```go
// vulnsEvery is how often the slow pass scans with brew vulns, which sends
// the installed formulae to OSV.dev.
const vulnsEvery = 24 * time.Hour

// slowQueries are what runs once the JSON queries are in: doctor, the
// dry-runs, the service files, the sizes, and the vulnerability scan when
// it's due. A scan asked for with r is spent here.
func (a *App) slowQueries() []query {
	qs := []query{qDoctor, qCleanup, qSizes, qAutoremove, qServiceFiles}
	if a.vulnsDue() {
		qs = append(qs, qVulns)
	}
	a.forceVulns = false
	return qs
}

// vulnsDue reports whether the slow pass should scan: when r on Skunky
// asked for it, or when there's no scan or it's a day old, unless brew has
// no vulns command.
func (a *App) vulnsDue() bool {
	switch {
	case a.noVulns:
		return false
	case a.forceVulns:
		return true
	}
	return a.snap.Vulns == nil || a.now().Sub(a.snap.VulnsAt) >= vulnsEvery
}

// vulnsNote is the line under Skunky's list: when brew vulns last ran and
// how much it could check, or why it couldn't run. It's "" before any scan,
// when the empty note says so.
func (a *App) vulnsNote() string {
	err := a.errs[qVulns]
	switch {
	case a.noVulns:
		return "brew vulns isn't available · U updates brew"
	case err != nil && a.snap.Vulns != nil:
		return "couldn't scan: " + scanReason(err) + " · showing the scan from " + ago(a.now(), a.snap.VulnsAt)
	case err != nil:
		return "couldn't scan: " + scanReason(err)
	case a.snap.Vulns == nil:
		return ""
	}
	return fmt.Sprintf("scanned %s · %d of %d formulae checked · casks aren't scanned",
		ago(a.now(), a.snap.VulnsAt), a.snap.Vulns.Checked, a.formulaCount())
}

// scanReason is err's first line, shortened to fit the note.
func scanReason(err error) string {
	first, _, _ := strings.Cut(err.Error(), "\n")
	return ansi.Truncate(first, reasonW, "…")
}

// formulaCount is how many formulae are installed.
func (a *App) formulaCount() int {
	n := 0
	for _, p := range a.snap.Packages {
		if p.Kind == brew.KindFormula {
			n++
		}
	}
	return n
}

// vulnsEmpty says why Skunky has no rows. It never says anything is safe,
// only what was checked.
func (a *App) vulnsEmpty() string {
	switch {
	case a.snap.Vulns != nil:
		return fmt.Sprintf("no known vulnerabilities in the %d formulae checked", a.snap.Vulns.Checked)
	case a.noVulns:
		return "no scan to show"
	case a.inflight[qVulns]:
		return "checking with brew vulns…"
	}
	return "not scanned yet · r scans now"
}
```

At the top of `skunkyBadge`, before the nil check:

```go
	if a.errs[qVulns] != nil {
		return -1, a.st.ChipStale.Render("⚠") // the last scan failed: what's shown may be old
	}
```

- [ ] **Step 7: Show the notes in `internal/ui/installed.go`**

1. `installedTab` gains, after `now`:

```go
	scanNote, scanEmpty func() string // Skunky's notes, from the App; nil in tests that don't set them
```

2. In `emptyNote`, after the `t.match` case:

```go
	if t.filter == filterSkunky && t.scanEmpty != nil {
		return t.scanEmpty()
	}
```

3. In `view`, after the `if err != nil { … refreshBanner … }` block, before `t.table.SetHeight(…)`:

```go
	footer := ""
	if t.filter == filterSkunky && t.scanNote != nil {
		if n := t.scanNote(); n != "" {
			footer = "\n" + about(t.st, n, iw)
			ih--
		}
	}
```

and in the list pane, `body+rows` becomes `body+rows+footer`:

```go
		list = pane(t.st, title, body+rows+footer, t.listW, t.h, focus == colList)
```

- [ ] **Step 8: Run the tests**

Run: `go test ./internal/ui -count=1 && go vet -tags screenshots ./internal/ui`
Expected: `ok`, and vet reports nothing. No golden changes: no golden shows Skunky yet. If another test fails because the slow pass now has one more query, read it: an empty-cache test (`newTestApp`) now scans once, which is the intended behaviour; fix the test's expectation, not the code.

- [ ] **Step 9: Lint and commit**

Run: `golangci-lint run ./...` → `0 issues.`

```bash
git add internal/ui
git commit -m "feat(ui): scan with brew vulns once a day, or now with r on Skunky, and say what it checked"
```

---

### Task 5: the details and Help

**Files:**
- Modify: `internal/ui/skunky.go`, `internal/ui/installed.go` (`preview`), `internal/ui/help.go`, `internal/ui/skunky_test.go`, goldens

**Interfaces:**
- Consumes: `finding`, `severityRank`, `severityText` (Task 3).
- Produces: `(t *installedTab) writeVulns(b *strings.Builder, p brew.Package)`, `fixedText(fixed []string) string`, `vulnIndent`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/skunky_test.go` (add `"charm.land/lipgloss/v2"` to its imports):

```go
// pkg is the fixture package named name.
func pkg(t *testing.T, a *App, name string) brew.Package {
	t.Helper()
	for _, p := range a.snap.Packages {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no package %s", name)
	return brew.Package{}
}

func TestSkunkyDetails(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.installed.pvW, a.installed.h = 60, 30
	jc := stripANSI(a.installed.preview(pkg(t, a, "json-c")))
	for _, want := range []string{
		"Vulnerabilities",
		"  MEDIUM    OSV-2024-512",
		"            Integer overflow in printbuf",
		"            fixed in 0.20",
		"  UNKNOWN   CVE-2026-31337",
		"            fixed upstream, not yet released",
		"  LOW       OSV-2023-77",
	} {
		if !strings.Contains(jc, want+"\n") {
			t.Errorf("json-c's details should have the line %q:\n%s", want, jc)
		}
	}
	if i, j, k := strings.Index(jc, "OSV-2024-512"), strings.Index(jc, "CVE-2026-31337"), strings.Index(jc, "OSV-2023-77"); i > j || j > k {
		t.Errorf("findings should go worst first:\n%s", jc)
	}
	if strings.Contains(jc, "printer\n            fixed") || strings.Contains(jc, "is available") {
		t.Errorf("LOW has no fix, and json-c isn't outdated:\n%s", jc)
	}
	if strings.Index(jc, "Vulnerabilities") > strings.Index(jc, "Dependencies") {
		t.Errorf("the findings come before the dependencies:\n%s", jc)
	}

	lua := stripANSI(a.installed.preview(pkg(t, a, "lua-language-server")))
	for _, want := range []string{"  HIGH      GHSA-7f3x-lua-0001", "            fixed in 3.20.0", "  3.19.2 is available · u upgrades"} {
		if !strings.Contains(lua, want+"\n") {
			t.Errorf("lua-language-server's details should have the line %q:\n%s", want, lua)
		}
	}
	for _, name := range []string{"fd", "git"} { // nothing found; a finding for another version
		if out := stripANSI(a.installed.preview(pkg(t, a, name))); strings.Contains(out, "Vulnerabilities") {
			t.Errorf("%s has no open findings at its version:\n%s", name, out)
		}
	}
}

func TestSkunkyDetailsWrapSummaries(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	for i, f := range a.snap.Vulns.Findings {
		if f.Formula == "lua-language-server" {
			a.snap.Vulns.Findings[i].Vulns[0].Summary = strings.Repeat("overflow ", 12)
		}
	}
	a.sync()
	a.installed.pvW, a.installed.h = 52, 30
	iw, _ := paneInner(a.installed.pvW, a.installed.h)
	out := stripANSI(a.installed.preview(pkg(t, a, "lua-language-server")))
	if n := strings.Count(out, "            overflow"); n < 2 {
		t.Errorf("a long summary should wrap onto indented lines, got %d:\n%s", n, out)
	}
	for l := range strings.SplitSeq(out, "\n") {
		if strings.Contains(l, "overflow") && lipgloss.Width(l) > iw {
			t.Errorf("line wider than the pane's %d: %q", iw, l)
		}
	}
}

func TestFixedText(t *testing.T) {
	commit := "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678"
	for _, c := range []struct {
		fixed []string
		want  string
	}{
		{[]string{"0.20", "0.19.1"}, "fixed in 0.20, 0.19.1"},
		{[]string{commit}, "fixed upstream, not yet released"},
		{[]string{"0.20", commit}, "fixed in 0.20"},
		{nil, ""},
	} {
		if got := fixedText(c.fixed); got != c.want {
			t.Errorf("fixedText(%v) = %q, want %q", c.fixed, got, c.want)
		}
	}
}

func TestHelpOnSkunky(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	scans := func() bool {
		for _, g := range a.helpGroups() {
			for _, k := range g.keys {
				if k == [2]string{"r", "refresh, and scan with brew vulns now"} {
					return true
				}
			}
		}
		return false
	}
	show(t, a, "Top-level")
	if scans() {
		t.Error("Help mentions the scan off Skunky")
	}
	show(t, a, "Skunky")
	if !scans() {
		t.Error("Help on Skunky should say r scans")
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/ui -run 'SkunkyDetails|FixedText|HelpOnSkunky' -count=1`
Expected: FAIL to build: `undefined: fixedText`.

- [ ] **Step 3: Add the section to `internal/ui/skunky.go`**

Add `"cmp"`, `"regexp"` and `"slices"` to its imports, then:

```go
// vulnIndent is where a finding's ID, summary and fix start in the
// details, past its severity.
const vulnIndent = 12

// commitHash matches a git commit: OSV often records a fix as the commit
// that made it, before any release has it.
var commitHash = regexp.MustCompile(`^[0-9a-f]{40}$`)

// fixedText says what fixes a finding: the versions, or that only a commit
// does so far, or "" when nothing does yet.
func fixedText(fixed []string) string {
	var versions []string
	for _, f := range fixed {
		if !commitHash.MatchString(f) {
			versions = append(versions, f)
		}
	}
	switch {
	case len(versions) > 0:
		return "fixed in " + strings.Join(versions, ", ")
	case len(fixed) > 0:
		return "fixed upstream, not yet released"
	}
	return ""
}

// writeVulns writes p's open findings, worst first, when the last scan has
// them for the version installed. It doesn't say an upgrade fixes them:
// brew's versions and OSV's fixes are written too differently to compare
// safely, and a wrong "fixed" is worse than none.
func (t *installedTab) writeVulns(b *strings.Builder, p brew.Package) {
	f, ok := t.finding(p)
	if !ok {
		return
	}
	st := t.st
	iw, _ := paneInner(t.pvW, t.h)
	indent := strings.Repeat(" ", vulnIndent)
	vulns := slices.Clone(f.Vulns)
	slices.SortStableFunc(vulns, func(x, y brew.Vuln) int {
		return cmp.Or(cmp.Compare(severityRank(x.Severity), severityRank(y.Severity)), strings.Compare(x.ID, y.ID))
	})
	b.WriteString("\n" + st.Accent.Render("Vulnerabilities") + "\n")
	for _, v := range vulns {
		gap := strings.Repeat(" ", max(vulnIndent-2-len(v.Severity), 1))
		b.WriteString("  " + severityText(st, v.Severity) + gap + v.ID + "\n")
		if v.Summary != "" {
			for l := range strings.SplitSeq(ansi.Wrap(v.Summary, max(iw-vulnIndent, minFlex), ""), "\n") {
				b.WriteString(indent + l + "\n")
			}
		}
		if fixed := fixedText(v.Fixed); fixed != "" {
			b.WriteString(indent + st.Muted.Render(fixed) + "\n")
		}
	}
	if p.Outdated {
		b.WriteString(st.Muted.Render("  "+p.Latest+" is available · u upgrades") + "\n")
	}
}
```

`ansi.Wrap` can leave a trailing space on a wrapped line; the width test allows that as long as the line fits. If the `overflow` lines measure one wider than `iw`, trim each line with `strings.TrimRight(l, " ")` before writing it.

- [ ] **Step 4: Call it from the preview**

In `internal/ui/installed.go`'s `preview`, right after `b.WriteString(kv(st, pairs...))`:

```go
	t.writeVulns(&b, p)
```

- [ ] **Step 5: Help**

In `internal/ui/help.go`, the Installed case becomes:

```go
	case a.tab == tabInstalled:
		ks := [][2]string{
			{"enter", "actions"}, {"u", "upgrade"}, {"x", "uninstall"}, {"p", "pin or unpin"},
			{"R", "reinstall"}, {"L", "link or unlink"}, {"f", "fetch the newer version"},
			{"space", "mark for several"}, {"l tab", "details, then j/k scroll"},
		}
		if a.installed.filter == filterSkunky {
			ks = append(ks, [2]string{"r", "refresh, and scan with brew vulns now"})
		}
		here = helpGroup{"Installed", ks}
```

- [ ] **Step 6: Run the tests and update the goldens**

Run: `go test ./internal/ui -count=1`
Expected: the new tests pass. `TestScreens` may fail on `installed-menu`: its details pane is lua-language-server's, which gains the Vulnerabilities section.

Run: `go test ./internal/ui -run TestScreens -update -count=1 && git diff internal/ui/testdata/golden`
Check: only `installed-menu.txt` changed, and only lua-language-server's details. Then `go test ./... -count=1` → `ok`.

- [ ] **Step 7: Lint and commit**

Run: `golangci-lint run ./...` → `0 issues.`

```bash
git add internal/ui
git commit -m "feat(ui): show each finding in the details, worst first, with what fixes it"
```

---

### Task 6: the Skunky golden, README, and final checks

**Files:**
- Modify: `internal/ui/golden_test.go`, `README.md`
- Create: `internal/ui/testdata/golden/skunky.txt` (by `-update`)

- [ ] **Step 1: Add the screen**

In `internal/ui/golden_test.go`'s `TestScreens`, after the loop and before `a.Update(tea.WindowSizeMsg{Width: 90, Height: 40})`:

```go
	show(t, a, "Skunky")
	keys(t, a, "g", "g") // lua-language-server: HIGH, and outdated
	golden(t, "skunky", stripANSI(a.render()))
	show(t, a, "Top-level")
	keys(t, a, "g", "g", "j", "j", "j") // back to git for the narrow screens
```

- [ ] **Step 2: Write the golden and read it**

Run: `go test ./internal/ui -run TestScreens -update -count=1 && git status --short internal/ui/testdata/golden && cat internal/ui/testdata/golden/skunky.txt`
Check:
- only `skunky.txt` is new, and no other golden changed;
- the list shows lua-language-server (HIGH) and json-c (MEDIUM) under SEVERITY;
- the note reads `scanned 3h ago · 4 of 5 formulae checked · casks aren't scanned` (it may be cut with `…` at this width);
- the details show the Vulnerabilities section with `3.19.2 is available · u upgrades`.

- [ ] **Step 3: README**

In `README.md`:

1. Under **Packages** in Features, the first bullet's list becomes `Dependencies, Outdated, Pinned, Deprecated and Skunky, each with a count in the sidebar.`, and after the Deprecated bullet add:

```markdown
- Skunky lists installed formulae with known vulnerabilities, worst first,
  from `brew vulns` (OSV.dev). It scans at most once a day, or now with `r`
  on the view. The details list each finding and what fixes it. Casks, and
  formulae brew can't match to an upstream source, aren't scanned, so an
  empty list means nothing known was found, not that all is safe.
```

2. In the **Sidebar** section, `**Packages**: All, Top-level, Formulae, Casks, Dependencies, Outdated, Pinned and Deprecated` becomes `…, Pinned, Deprecated and Skunky` (rewrap to the file's width).

- [ ] **Step 4: Final checks**

Run each, and read the output:
- `go test ./... -count=1` → `ok` for every package.
- `go vet -tags screenshots ./internal/ui` → nothing.
- `make cover` → every package and the total at 90% or above.
- `golangci-lint run ./...` → `0 issues.`
- `make test-live` → `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/ui README.md
git commit -m "docs: Skunky in the README; a golden of the view"
```
