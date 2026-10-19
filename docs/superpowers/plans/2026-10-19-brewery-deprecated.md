# Deprecated and Disabled Packages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Brewery reads brew's deprecation and disable fields, and shows them in three places:
- a `deprecated` or `disabled` mark in Installed's STATUS column;
- brew-worded rows in the details: the reason, the disable date and the replacement;
- a new **Deprecated** sidebar view.

**Architecture:**
- **brew:** a shared `retireJSON` struct, embedded in both `formulaJSON` and `caskJSON`, fills new `Package` fields.
- **ui:** a new `internal/ui/retire.go` holds the wording: brew's reason phrases, the disable-date line and the details rows. Installed's status, details and views use it.

**Tech Stack:** Go 1.26, Bubble Tea v2, Lip Gloss v2, golangci-lint v2.15.0.

**Spec:** `docs/superpowers/specs/2026-10-19-brewery-deprecated-design.md`

## Global Constraints

- **Lint:**
  - `.golangci.yml` is fixed (gocognit and gocyclo `min-complexity: 20`, tests excluded). Don't edit it, and don't add `//nolint`.
  - `golangci-lint run ./...` must report `0 issues.` at the end of every task.
- **Coverage:** `make cover` stays at 90% or above in total and per package.
- **Goldens:**
  - Update only in Task 2, with `go test ./internal/ui -run TestScreens -update -count=1`.
  - Read `git diff internal/ui/testdata/golden` first. Only these may change: the Installed marks, the sidebar's new Deprecated entry on every screen, and Help's background.
- **Style:** match the surrounding code's comment density, naming and idioms. Every function gets a short doc comment.
- **Exact texts:** user-facing strings are exactly as written here.
- **Commits:** one commit per task, with the message given.

---

### Task 1: Read the deprecation and disable fields

**Files:**
- Modify: `internal/brew/types.go` (`Package`)
- Modify: `internal/brew/parse.go` (`formulaJSON`, `caskJSON`, both `toPackage`)
- Modify: `internal/brew/testdata/info.json` (json-c and fontbase)
- Modify: `internal/state/snapshot.go` (`snapshotVersion` 6 → 7)
- Test: `internal/brew/parse_test.go`

**Interfaces:**
- Produces, for Task 2: `brew.Package` fields `Deprecated bool` (already there), `Disabled bool`, `DeprecationDate string`, `DisableDate string`, `Reason string` and `Replacement string`.
- In the fixture:
  - json-c is deprecated, with reason `unmaintained`, date `2026-03-01` and replacement `--formula json-c-ng`;
  - fontbase is disabled, with reason `discontinued` and disable date `2026-09-01`;
  - everything else is neither.

- [ ] **Step 1: Write the failing tests.** Add to `internal/brew/parse_test.go`:

```go
func TestParseInfoRetirement(t *testing.T) {
	pkgs, err := ParseInfo(fixture(t, "info.json"))
	if err != nil {
		t.Fatal(err)
	}
	jsonc := byName(pkgs, "json-c")
	if !jsonc.Deprecated || jsonc.Disabled || jsonc.Reason != "unmaintained" ||
		jsonc.DeprecationDate != "2026-03-01" || jsonc.DisableDate != "" || jsonc.Replacement != "--formula json-c-ng" {
		t.Errorf("json-c: %+v", jsonc)
	}
	fontbase := byName(pkgs, "fontbase")
	if fontbase.Deprecated || !fontbase.Disabled || fontbase.Reason != "discontinued" ||
		fontbase.DisableDate != "2026-09-01" || fontbase.Replacement != "" {
		t.Errorf("fontbase: %+v", fontbase)
	}
	fd := byName(pkgs, "fd")
	if fd.Deprecated || fd.Disabled || fd.Reason != "" || fd.Replacement != "" {
		t.Errorf("fd is neither: %+v", fd)
	}
}

func TestReplacementIsWrittenAsBrewWritesIt(t *testing.T) {
	for _, tt := range []struct{ formula, cask, want string }{
		{"foo", "foo", "foo"},
		{"foo", "", "--formula foo"},
		{"", "bar", "--cask bar"},
		{"foo", "bar", "--formula foo"},
		{"", "", ""},
	} {
		if got := replacement(tt.formula, tt.cask); got != tt.want {
			t.Errorf("replacement(%q, %q) = %q, want %q", tt.formula, tt.cask, got, tt.want)
		}
	}
}

func TestReasonComesFromTheStateBrewReports(t *testing.T) {
	r := retireJSON{
		Deprecated: true, DeprecationReason: ptr("unmaintained"),
		Disabled: true, DisableReason: ptr("does_not_build"), DisableFormula: ptr("other"),
	}
	var p Package
	r.apply(&p)
	if p.Reason != "unmaintained" || p.Replacement != "" {
		t.Errorf("deprecated wins, as in brew: %+v", p)
	}
	r.Deprecated = false
	p = Package{}
	r.apply(&p)
	if p.Reason != "does_not_build" || p.Replacement != "--formula other" {
		t.Errorf("disabled: %+v", p)
	}
}

func ptr(s string) *string { return &s }
```

If a `ptr` helper already exists in the package's tests, use it and don't redeclare it.

- [ ] **Step 2: Run them.** `go test ./internal/brew -count=1`. Expected: it fails to compile (`retireJSON`, `replacement` and the new fields are undefined).

- [ ] **Step 3: Add the fields.** In `internal/brew/types.go`, after `Deprecated` in `Package` (let gofmt align):

```go
	Disabled        bool      `json:"disabled"`         // brew won't install it any more
	DeprecationDate string    `json:"deprecation_date"` // YYYY-MM-DD, or ""
	DisableDate     string    `json:"disable_date"`     // YYYY-MM-DD, or ""
	Reason          string    `json:"reason"`           // brew's keyword or text for why it's deprecated or disabled
	Replacement     string    `json:"replacement"`      // what to brew install instead, as brew writes it
```

- [ ] **Step 4: Parse them.** In `internal/brew/parse.go`:
  - **Remove** the `Deprecated` field from both `formulaJSON` and `caskJSON`. The embedded struct supplies it, and a duplicate `deprecated` tag would make encoding/json ignore both.
  - Embed `retireJSON` in both structs: add a line `retireJSON` to each.
  - Remove `Deprecated: f.Deprecated` and `Deprecated: c.Deprecated` from the `toPackage` literals.
  - In both `toPackage` methods, call `f.apply(&p)` (or `c.apply(&p)`) before `return p`.
  - Add:

```go
// retireJSON is the deprecation and disable fields that formulae and
// casks share.
type retireJSON struct {
	Deprecated         bool    `json:"deprecated"`
	DeprecationDate    *string `json:"deprecation_date"`
	DeprecationReason  *string `json:"deprecation_reason"`
	DeprecationFormula *string `json:"deprecation_replacement_formula"`
	DeprecationCask    *string `json:"deprecation_replacement_cask"`
	Disabled           bool    `json:"disabled"`
	DisableDate        *string `json:"disable_date"`
	DisableReason      *string `json:"disable_reason"`
	DisableFormula     *string `json:"disable_replacement_formula"`
	DisableCask        *string `json:"disable_replacement_cask"`
}

// apply copies the fields into p. The reason and replacement come from
// the state brew reports: deprecated before disabled, as brew does.
func (r retireJSON) apply(p *Package) {
	p.Deprecated, p.Disabled = r.Deprecated, r.Disabled
	p.DeprecationDate, p.DisableDate = deref(r.DeprecationDate), deref(r.DisableDate)
	switch {
	case r.Deprecated:
		p.Reason, p.Replacement = deref(r.DeprecationReason), replacement(deref(r.DeprecationFormula), deref(r.DeprecationCask))
	case r.Disabled:
		p.Reason, p.Replacement = deref(r.DisableReason), replacement(deref(r.DisableFormula), deref(r.DisableCask))
	}
}

// replacement writes what to install instead as brew does: the bare name
// when the formula and cask are the same, else --formula or --cask first.
func replacement(formula, cask string) string {
	switch {
	case formula != "" && formula == cask:
		return formula
	case formula != "":
		return "--formula " + formula
	case cask != "":
		return "--cask " + cask
	}
	return ""
}
```

Calling a promoted method (`f.apply(&p)`) works because the struct is embedded. If a linter prefers `f.retireJSON.apply`, follow it.

- [ ] **Step 5: Update the fixture.** In `internal/brew/testdata/info.json`:
  - **json-c:** replace `"deprecated": false` with
    `"deprecated": true, "deprecation_date": "2026-03-01", "deprecation_reason": "unmaintained", "deprecation_replacement_formula": "json-c-ng", "deprecation_replacement_cask": null,`
    followed by `"disabled": false, "disable_date": null, "disable_reason": null,` and its remaining keys as they are. Mind the commas.
  - **fontbase:** replace `"deprecated": false` with
    `"deprecated": false, "disabled": true, "disable_date": "2026-09-01", "disable_reason": "discontinued",`
    and its remaining keys.

- [ ] **Step 6: Bump the snapshot.** In `internal/state/snapshot.go`, set `const snapshotVersion = 7`.

- [ ] **Step 7: Run and check.**
  - `go test ./internal/brew -count=1`. Expected: PASS.
  - Then `go test ./... -count=1`. The UI goldens may already differ, because the fixture now marks json-c and fontbase. If only `TestScreens` fails, leave the goldens for Task 2 and note it in the report. Any other failure must be fixed here.
  - `golangci-lint run ./...` (`0 issues.`).

- [ ] **Step 8: Commit.** `git add internal/brew internal/state && git commit -m "feat(brew): read why a package is deprecated or disabled"`

---

### Task 2: Show them in Installed, its details and a Deprecated view

**Files:**
- Create: `internal/ui/retire.go`
- Create: `internal/ui/retire_test.go`
- Modify: `internal/ui/installed.go`:
  - `status`;
  - `preview`;
  - the views (`filterDeprecated`, `views`, `shows`);
  - `emptyNote`;
  - a `now` field.
- Modify: `internal/ui/sidebar.go` (`entries`: Deprecated after Pinned)
- Modify: `internal/ui/app.go` (`sync`: `a.installed.now = a.now`)
- Modify: tests that walk the sidebar with `j` past Pinned, or count its entries
- Goldens: as the Global Constraints allow

**Interfaces:**
- Consumes, from Task 1: the `Package` fields above, and the fixture's json-c and fontbase.

- [ ] **Step 1: Write the failing tests.** Create `internal/ui/retire_test.go`:

```go
package ui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dillo/brewery/internal/brew"
)

func TestReasonText(t *testing.T) {
	for _, tt := range []struct {
		p    brew.Package
		want string
	}{
		{brew.Package{Kind: brew.KindFormula, Reason: "unmaintained"}, "is not maintained upstream"},
		{brew.Package{Kind: brew.KindFormula, Reason: "repo_archived"}, "has an archived upstream repository"},
		{brew.Package{Kind: brew.KindCask, Reason: "discontinued"}, "is discontinued upstream"},
		{brew.Package{Kind: brew.KindCask, Reason: "repo_archived"}, "repo_archived"}, // a formula keyword on a cask stays as it is
		{brew.Package{Kind: brew.KindFormula, Reason: "it moved to a new name"}, "it moved to a new name"},
		{brew.Package{Kind: brew.KindFormula}, ""},
	} {
		if got := reasonText(tt.p); got != tt.want {
			t.Errorf("%v %q: %q, want %q", tt.p.Kind, tt.p.Reason, got, tt.want)
		}
	}
}

func TestDisableLine(t *testing.T) {
	now := time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC)
	for _, tt := range []struct {
		p    brew.Package
		want string
	}{
		{brew.Package{DisableDate: "2026-09-01"}, "disabled on 2026-09-01"},
		{brew.Package{DisableDate: "2026-10-13"}, "will be disabled on 2026-10-13"}, // today counts as to come, as in brew
		{brew.Package{DisableDate: "2026-12-01"}, "will be disabled on 2026-12-01"},
		{brew.Package{DeprecationDate: "2026-03-01"}, "will be disabled on 2027-03-01"},
		{brew.Package{DeprecationDate: "2025-01-15"}, "disabled on 2026-01-15"},
		{brew.Package{DeprecationDate: "2026-03-01", DisableDate: "2026-11-01"}, "will be disabled on 2026-11-01"},
		{brew.Package{}, ""},
		{brew.Package{DisableDate: "soon"}, ""},
	} {
		if got := disableLine(tt.p, now); got != tt.want {
			t.Errorf("%+v: %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestInstalledShowsRetirement(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "All")
	out := stripANSI(a.render())
	for _, want := range []string{`json-c\s.*deprecated`, `fontbase\s.*disabled`} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("STATUS lacks %s:\n%s", want, out)
		}
	}

	selectInstalled(t, a, "json-c")
	out = stripANSI(a.render())
	for _, want := range []string{
		`deprecated\s+is not maintained upstream`,
		`will be disabled on 2027-03-01`,
		`replacement\s+brew install --formula json-c-ng`,
	} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("json-c's details lack %s:\n%s", want, out)
		}
	}

	selectInstalled(t, a, "fontbase")
	out = stripANSI(a.render())
	for _, want := range []string{`disabled\s+is discontinued upstream`, `disabled on 2026-09-01`} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("fontbase's details lack %s:\n%s", want, out)
		}
	}

	for _, p := range a.snap.Packages {
		if p.Name == "fd" {
			if pv := stripANSI(a.installed.preview(p)); strings.Contains(pv, "deprecated") || strings.Contains(pv, "disabled") {
				t.Errorf("fd is neither:\n%s", pv)
			}
		}
	}
}

func TestDeprecatedView(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Deprecated")
	var names []string
	for _, p := range a.installed.rows {
		names = append(names, p.Name)
	}
	if strings.Join(names, ",") != "fontbase,json-c" && strings.Join(names, ",") != "json-c,fontbase" {
		t.Errorf("Deprecated lists %v, want json-c and fontbase", names)
	}
	if !regexp.MustCompile(`Deprecated\s+2`).MatchString(stripANSI(a.render())) {
		t.Errorf("the sidebar should count 2:\n%s", stripANSI(a.render()))
	}
	for i := range a.snap.Packages {
		a.snap.Packages[i].Deprecated, a.snap.Packages[i].Disabled = false, false
	}
	a.sync()
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing installed is deprecated or disabled") {
		t.Errorf("empty Deprecated view:\n%s", out)
	}
}
```

If the details pane wraps lines differently from these patterns, check the rendered output and match its layout. Don't loosen what's asserted.

- [ ] **Step 2: Run them.** `go test ./internal/ui -run 'ReasonText|DisableLine|Retirement|DeprecatedView' -count=1`. Expected: it fails to compile (`reasonText` and `disableLine` are undefined).

- [ ] **Step 3: Write `internal/ui/retire.go`:**

```go
package ui

import (
	"cmp"
	"time"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/theme"
)

// formulaReasons and caskReasons are brew's phrases for its deprecation
// and disable keywords (Library/Homebrew/deprecate_disable.rb). A few long
// ones are shortened to fit the details.
var (
	formulaReasons = map[string]string{
		"does_not_build":      "does not build",
		"no_license":          "has no license",
		"repo_archived":       "has an archived upstream repository",
		"repo_removed":        "has a removed upstream repository",
		"unmaintained":        "is not maintained upstream",
		"unreachable":         "is no longer reliably reachable upstream",
		"unsupported":         "is not supported upstream",
		"deprecated_upstream": "is deprecated upstream",
		"versioned_formula":   "is a versioned formula",
		"checksum_mismatch":   "has a source file whose checksum changed upstream",
	}
	caskReasons = map[string]string{
		"discontinued":             "is discontinued upstream",
		"moved_to_mas":             "is now only on the Mac App Store",
		"no_longer_available":      "is no longer available upstream",
		"no_longer_meets_criteria": "no longer meets the criteria for casks",
		"unmaintained":             "is not maintained upstream",
		"fails_gatekeeper_check":   "does not pass the macOS Gatekeeper check",
		"unreachable":              "is no longer reliably reachable upstream",
	}
)

// retireState is "deprecated" or "disabled", as brew counts p, or "".
// Like brew, it takes deprecated first.
func retireState(p brew.Package) string {
	switch {
	case p.Deprecated:
		return "deprecated"
	case p.Disabled:
		return "disabled"
	}
	return ""
}

// reasonText is why p is deprecated or disabled, in brew's words: its
// phrase for a known keyword, or the reason as it is.
func reasonText(p brew.Package) string {
	phrases := formulaReasons
	if p.Kind == brew.KindCask {
		phrases = caskReasons
	}
	if s, ok := phrases[p.Reason]; ok {
		return s
	}
	return p.Reason
}

// disableLine says when p was or will be disabled: its disable date, or
// else a year after it was deprecated, as brew reckons it. It's "" when
// there's no date.
func disableLine(p brew.Package, now time.Time) string {
	day, err := time.Parse(time.DateOnly, p.DisableDate)
	if err != nil {
		deprecated, err := time.Parse(time.DateOnly, p.DeprecationDate)
		if err != nil {
			return ""
		}
		day = deprecated.AddDate(0, 12, 0)
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if day.Before(today) {
		return "disabled on " + day.Format(time.DateOnly)
	}
	return "will be disabled on " + day.Format(time.DateOnly)
}

// retireRows are the details' rows for a deprecated or disabled package,
// as kv pairs: the state and why, when it's disabled, and what to install
// instead. None for any other package.
func retireRows(st theme.Styles, p brew.Package, now time.Time) []string {
	state := retireState(p)
	if state == "" {
		return nil
	}
	style := st.Warn
	if state == "disabled" {
		style = st.Err
	}
	rows := []string{state, style.Render(cmp.Or(reasonText(p), "yes"))}
	if line := disableLine(p, now); line != "" {
		rows = append(rows, " ", st.Muted.Render(line)) // a blank key continues the row above
	}
	if p.Replacement != "" {
		rows = append(rows, "replacement", "brew install "+p.Replacement)
	}
	return rows
}
```

Check that `kv` (components.go) renders a key of `" "` as a blank key column. If it skips or misaligns it, use whatever key renders blank.

- [ ] **Step 4: Use it in Installed.** In `internal/ui/installed.go`:
  - **`status`:** after the `Outdated` mark, add:

```go
	switch retireState(p) {
	case "deprecated":
		s = append(s, theme.Tint(st.P.Yellow, "deprecated"))
	case "disabled":
		s = append(s, theme.Tint(st.P.Red, "disabled"))
	}
```

  - **`installedTab`:** add a field `now func() time.Time // today, for the disable date`. Set it to `time.Now` in `newInstalledTab`. In `internal/ui/app.go`'s `sync`, set `a.installed.now = a.now`, so tests' fixed clock applies.
  - **`preview`:** delete the `deprecated` variable. Build the `kv` pairs so the retirement rows sit where `deprecated` was, between `linked` and `homepage`:

```go
	pairs := []string{"type", p.Kind.String(), "tap", p.Tap, "size", t.size(p), "installed", how,
		"latest", latest, "pinned", pinned, "linked", linkedText(p)}
	pairs = append(pairs, retireRows(st, p, t.now())...)
	pairs = append(pairs, "homepage", p.Homepage)
	b.WriteString(kv(st, pairs...))
```

  - **Views:**
    - add `filterDeprecated` at the end of the `installedFilter` constants;
    - add its `views` entry: `filterDeprecated: {"Deprecated", "On their way out: deprecated, or disabled and no longer installable."},`;
    - in `shows`, add `case filterDeprecated: return retireState(p) != ""`.
  - **`emptyNote`:** before the final return, add:

```go
	if t.filter == filterDeprecated {
		return "nothing installed is deprecated or disabled"
	}
```

  - **STATUS width:** if the column (`{"STATUS", 10, 6}`) cuts `deprecated` short in the goldens, widen it to 12. Say so in the report.

- [ ] **Step 5: Add the sidebar entry.** In `internal/ui/sidebar.go`'s `entries`, after Pinned:

```go
	{"Deprecated", secPackages, tabInstalled, int(filterDeprecated)},
```

- [ ] **Step 6: Run the tests.**
  - `go test ./internal/ui -run 'ReasonText|DisableLine|Retirement|DeprecatedView' -count=1`. Expected: PASS.
  - Then `go test ./internal/ui -count=1`. Tests that walk the sidebar with `j` past Pinned, or count its entries, need adjusting. Keep each test's intent, and list every change in the report.

- [ ] **Step 7: Update the goldens.**
  - Run `go test ./internal/ui -run TestScreens -update -count=1`.
  - Read `git diff internal/ui/testdata/golden`. Only these may appear:
    - the sidebar's new `Deprecated` entry (with its count) on each screen;
    - the `deprecated`/`disabled` marks in Installed's STATUS;
    - shifts that follow from them.
  - If `make screenshots` covers these screens, run `make screenshots FREEZE="go run github.com/charmbracelet/freeze@v0.2.2"` and include the changed images. Skip it and say so if a tool is missing.

- [ ] **Step 8: Check.** `go test ./... -count=1`, `golangci-lint run ./...` (`0 issues.`), `make cover`.

- [ ] **Step 9: Commit.** `git add -A internal && git commit -m "feat(ui): show deprecated and disabled packages, with brew's reasons"`
