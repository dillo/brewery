# brewery Plan 1: Foundation ("See") Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a working `brewery` binary that opens instantly from a cached snapshot, refreshes from Homebrew in the background, and shows the Home, Installed, Outdated, Services and Taps tabs with real data (read-only).

**Architecture:** One Go module with four layers: `internal/brew` (the only code that runs brew or knows its formats), `internal/state` (snapshot cache + dependency index), `internal/theme` (Catppuccin Macchiato tokens), and `internal/ui` (Bubble Tea v2 models that talk to brew only through a `Source` interface). `cmd/brewery` wires them together.

**Tech Stack:** Go ≥ 1.25, Bubble Tea v2 (`charm.land/bubbletea/v2`), Bubbles v2 (`charm.land/bubbles/v2`: `table`, `key`), Lip Gloss v2 (`charm.land/lipgloss/v2`), `github.com/charmbracelet/x/ansi`.

**Spec:** `docs/superpowers/specs/2026-10-12-brewery-design.md`. This plan covers spec sections 1, 2 and 4 (read-only parts) and the error-handling and testing rules from section 5 that apply to them.

**Verified:** Every code block in this plan was compiled and tested (including `-race`, and the `live` tag against a real Homebrew 7.0.9 install) before the plan was written.

## Global Constraints

- Module path: `brewery`. It will be renamed to the GitHub path at release time (Plan 3).
- Go ≥ 1.25 (required by Bubbles v2.2.1). If the local Go is older, `GOTOOLCHAIN=auto` (Go's default) downloads a newer toolchain automatically.
- Pinned dependency versions: `charm.land/bubbletea/v2@v2.0.10`, `charm.land/lipgloss/v2@v2.0.6`, `charm.land/bubbles/v2@v2.2.1`.
- Only `internal/brew` may run `brew` or parse its output. `internal/ui` never shells out.
- Views use `theme.Styles` only, never raw color values.
- Read-only queries run with `HOMEBREW_NO_AUTO_UPDATE=1`, `HOMEBREW_NO_ENV_HINTS=1`, `HOMEBREW_NO_COLOR=1`.
- Query timeout: 30s.
- Snapshot cache: `$XDG_CACHE_HOME/brewery/snapshot.json`, falling back to `~/.cache/brewery/snapshot.json`. Writes are atomic (temp file + rename).
- A failed query shows its error only in the tab that needs it, keeps the last good data, and `r` retries.
- Tab keys: `1`–`5`, `[`/`]`. `r` refreshes, `q`/`ctrl+c` quits. Table navigation is `j/k`, `g/G`, `ctrl+d/ctrl+u`, `pgup/pgdown`. The single letters `u`, `d`, `b` and `f` are **not** bound to page movement, because Plan 2 uses them for actions. In the Installed tab, `f` cycles the filter.
- Mode badge colors: NORMAL mauve, COMMAND blue, FILTER teal, RUNNING peach, INSERT green.
- Work happens in `~/Dev/tools/brewery` (already a git repo containing the spec and `.gitignore`).

## File Structure

```
brewery/
├── Makefile                         build / test / test-live / run
├── go.mod, go.sum
├── cmd/brewery/
│   ├── main.go                      flags, brew lookup, cache load, starts the program
│   └── main_test.go
└── internal/
    ├── brew/
    │   ├── types.go                 Package, Outdated, Service, Tap, DoctorWarning, Kind
    │   ├── parse.go                 Parse{Info,Outdated,Services,Taps,Doctor}
    │   ├── client.go                Locate, Runner, ExecRunner, Client, QueryError
    │   ├── parse_test.go, client_test.go
    │   ├── live_test.go             //go:build live; runs real brew read-only
    │   └── testdata/                info.json, outdated.json, services.json, taps.json, doctor.txt
    ├── state/
    │   ├── index.go                 dependency index: UsedBy, IsLeaf, Tree
    │   ├── snapshot.go              Snapshot + Load/Save cache
    │   └── index_test.go, snapshot_test.go
    ├── theme/
    │   ├── theme.go                 Palette, Macchiato, Styles, Mode()
    │   └── theme_test.go
    └── ui/
        ├── source.go                Source interface, query messages, fetch/save commands
        ├── components.go            pane(), table helpers, kv(), errorBody()
        ├── list.go                  listTab + row builders for Outdated/Services/Taps
        ├── installed.go             installedTab: filters, table, preview
        ├── home.go                  homeTab: health tiles + doctor pane
        ├── app.go                   App root model: refresh flow, keys, tab bar, status bar
        └── helpers_test.go, list_test.go, installed_test.go, home_test.go, app_test.go
```

---

### Task 1: Brew data types and output parsers

**Files:**
- Create: `go.mod`
- Create: `internal/brew/types.go`, `internal/brew/parse.go`
- Create: `internal/brew/testdata/{info.json,outdated.json,services.json,taps.json,doctor.txt}`
- Test: `internal/brew/parse_test.go`

**Interfaces:**
- Produces: `brew.Kind` (`KindFormula`, `KindCask`, `String()`), `brew.Package`, `brew.Outdated`, `brew.Service`, `brew.Tap`, `brew.DoctorWarning`.
- Produces: `ParseInfo([]byte) ([]Package, error)`, `ParseOutdated([]byte) ([]Outdated, error)`, `ParseServices([]byte) ([]Service, error)`, `ParseTaps([]byte) ([]Tap, error)`, `ParseDoctor(string) []DoctorWarning`.
- Fixture facts later tasks rely on: 7 packages (5 formulae: fd, git, pcre2, json-c, lua-language-server; 2 casks: docker-desktop, fontbase); git → pcre2 → json-c; git is pinned with caveats; 2 outdated; postgresql@16 service in `error` (exit 1); 2 taps; 2 doctor warnings.

These fixtures are trimmed copies of real Homebrew 7.0.9 output. `outdated.json` deliberately starts with a `==> Downloading Homebrew API data` line, because real brew sometimes prints that before the JSON.

- [ ] **Step 1: Initialize the module**

```bash
cd ~/Dev/tools/brewery
go mod init brewery
mkdir -p internal/brew/testdata
```


- [ ] **Step 2: Create `internal/brew/testdata/info.json`**

```json
{
  "formulae": [
    {
      "name": "fd", "full_name": "fd", "tap": "homebrew/core",
      "desc": "Simple, fast and user-friendly alternative to find",
      "homepage": "https://github.com/sharkdp/fd",
      "versions": {"stable": "10.5.0", "head": "HEAD", "bottle": true},
      "installed": [{"version": "10.5.0", "time": 1788714516, "installed_on_request": true, "runtime_dependencies": []}],
      "pinned": false, "outdated": false, "deprecated": false, "caveats": null
    },
    {
      "name": "git", "full_name": "git", "tap": "homebrew/core",
      "desc": "Distributed revision control system",
      "homepage": "https://git-scm.com",
      "versions": {"stable": "2.51.0"},
      "installed": [{"version": "2.51.0", "time": 1788000000, "installed_on_request": true, "runtime_dependencies": [
        {"full_name": "pcre2", "version": "10.49", "declared_directly": true},
        {"full_name": "json-c", "version": "0.19", "declared_directly": false}
      ]}],
      "pinned": true, "outdated": false, "deprecated": false, "caveats": "  Bash completion has been installed.\n"
    },
    {
      "name": "pcre2", "full_name": "pcre2", "tap": "homebrew/core",
      "desc": "Perl compatible regular expressions library",
      "homepage": "https://www.pcre.org/",
      "versions": {"stable": "10.49"},
      "installed": [{"version": "10.49", "time": 1787000000, "installed_on_request": false, "runtime_dependencies": [
        {"full_name": "json-c", "version": "0.19", "declared_directly": true}
      ]}],
      "pinned": false, "outdated": false, "deprecated": false, "caveats": null
    },
    {
      "name": "json-c", "full_name": "json-c", "tap": "homebrew/core",
      "desc": null, "homepage": "https://github.com/json-c/json-c/wiki",
      "versions": {"stable": "0.19"},
      "installed": [{"version": "0.19", "time": 1787000000, "installed_on_request": false, "runtime_dependencies": []}],
      "pinned": false, "outdated": false, "deprecated": false, "caveats": null
    },
    {
      "name": "lua-language-server", "full_name": "lua-language-server", "tap": "homebrew/core",
      "desc": "Language Server for the Lua language",
      "homepage": "https://github.com/LuaLS/lua-language-server",
      "versions": {"stable": "3.19.2"},
      "installed": [
        {"version": "3.19.0", "time": 1780000000, "installed_on_request": true, "runtime_dependencies": []},
        {"version": "3.19.1", "time": 1785000000, "installed_on_request": true, "runtime_dependencies": []}
      ],
      "pinned": false, "outdated": true, "deprecated": false, "caveats": null
    }
  ],
  "casks": [
    {
      "token": "docker-desktop", "full_token": "docker-desktop", "tap": "homebrew/cask",
      "name": ["Docker Desktop", "Docker Community Edition"],
      "desc": "App to build and share containerised applications and microservices",
      "homepage": "https://www.docker.com/products/docker-desktop",
      "version": "4.95.0,242831", "installed": "4.39.0,184744", "installed_time": 1743035743,
      "pinned": false, "outdated": true, "deprecated": false, "caveats": null
    },
    {
      "token": "fontbase", "full_token": "fontbase", "tap": "homebrew/cask",
      "name": ["FontBase"], "desc": null, "homepage": "https://fontba.se/",
      "version": "2.22.0", "installed": "2.22.0", "installed_time": null,
      "pinned": false, "outdated": false, "deprecated": false, "caveats": null
    }
  ]
}
```

- [ ] **Step 3: Create `internal/brew/testdata/outdated.json`**

```text
==> Downloading Homebrew API data
{
  "formulae": [
    {"name": "lua-language-server", "installed_versions": ["3.19.0", "3.19.1"], "current_version": "3.19.2", "pinned": false, "pinned_version": null}
  ],
  "casks": [
    {"name": "docker-desktop", "installed_versions": ["4.39.0,184744"], "current_version": "4.95.0,242831", "pinned": false, "pinned_version": null}
  ]
}
```

- [ ] **Step 4: Create `internal/brew/testdata/services.json`**

```json
[
  {"name": "postgresql@16", "status": "error", "user": "locutus", "file": "/Users/locutus/Library/LaunchAgents/homebrew.mxcl.postgresql@16.plist", "exit_code": 1},
  {"name": "redis", "status": "none", "user": "locutus", "file": "/Users/locutus/Library/LaunchAgents/homebrew.mxcl.redis.plist", "exit_code": null}
]
```

- [ ] **Step 5: Create `internal/brew/testdata/taps.json`**

```json
[
  {"name": "anomalyco/tap", "user": "anomalyco", "repo": "tap", "path": "/opt/homebrew/Library/Taps/anomalyco/homebrew-tap", "installed": true, "official": false, "remote": "https://github.com/anomalyco/homebrew-tap.git", "formula_names": ["anomalyco/tap/opencode", "anomalyco/tap/sst"], "cask_tokens": ["anomalyco/tap/hex"]},
  {"name": "homebrew/core", "user": "Homebrew", "repo": "core", "path": "/opt/homebrew/Library/Taps/homebrew/homebrew-core", "installed": false, "official": true, "remote": null, "formula_names": [], "cask_tokens": []}
]
```

- [ ] **Step 6: Create `internal/brew/testdata/doctor.txt`**

```text
Please note that these warnings are just used to help the Homebrew maintainers
with debugging if you file an issue. If everything you use Homebrew for is
working fine: please don't worry or file an issue; just ignore this. Thanks!

Warning: The following taps are not trusted:
  oven-sh/bun

Untap them with:
  brew untap oven-sh/bun

Warning: Some installed casks are deprecated or disabled.
  fontbase
```

- [ ] **Step 7: Write the failing tests** in `internal/brew/parse_test.go`

```go
package brew

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func byName(pkgs []Package, name string) Package {
	for _, p := range pkgs {
		if p.Name == name {
			return p
		}
	}
	return Package{}
}

func TestParseInfo(t *testing.T) {
	pkgs, err := ParseInfo(fixture(t, "info.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 7 {
		t.Fatalf("got %d packages, want 7", len(pkgs))
	}

	git := byName(pkgs, "git")
	if git.Kind != KindFormula || git.Version != "2.51.0" || !git.OnRequest || !git.Pinned {
		t.Errorf("git parsed wrong: %+v", git)
	}
	if !reflect.DeepEqual(git.Deps, []string{"pcre2", "json-c"}) {
		t.Errorf("git deps = %v", git.Deps)
	}
	if !reflect.DeepEqual(git.DirectDeps, []string{"pcre2"}) {
		t.Errorf("git direct deps = %v", git.DirectDeps)
	}
	if git.Caveats != "Bash completion has been installed." {
		t.Errorf("git caveats = %q", git.Caveats)
	}
	if !git.InstalledAt.Equal(time.Unix(1788000000, 0)) {
		t.Errorf("git installed at = %v", git.InstalledAt)
	}

	if pcre := byName(pkgs, "pcre2"); pcre.OnRequest {
		t.Error("pcre2 should be installed as a dependency")
	}
	if lls := byName(pkgs, "lua-language-server"); lls.Version != "3.19.1" || lls.Latest != "3.19.2" || !lls.Outdated {
		t.Errorf("multi-keg formula should use newest keg: %+v", lls)
	}

	docker := byName(pkgs, "docker-desktop")
	if docker.Kind != KindCask || docker.Version != "4.39.0,184744" || docker.Latest != "4.95.0,242831" || !docker.Outdated || !docker.OnRequest {
		t.Errorf("docker-desktop parsed wrong: %+v", docker)
	}
	if fb := byName(pkgs, "fontbase"); fb.Desc != "FontBase" || !fb.InstalledAt.IsZero() {
		t.Errorf("cask with null desc should fall back to its name: %+v", fb)
	}
}

func TestParseInfoBadJSON(t *testing.T) {
	if _, err := ParseInfo([]byte("Error: not json")); err == nil {
		t.Fatal("want error for non-JSON output")
	}
}

func TestParseOutdatedSkipsLeadingNoise(t *testing.T) {
	got, err := ParseOutdated(fixture(t, "outdated.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Outdated{
		{Name: "lua-language-server", Kind: KindFormula, Installed: "3.19.1", Current: "3.19.2"},
		{Name: "docker-desktop", Kind: KindCask, Installed: "4.39.0,184744", Current: "4.95.0,242831"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestParseServices(t *testing.T) {
	got, err := ParseServices(fixture(t, "services.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "postgresql@16" || got[0].Status != "error" || *got[0].ExitCode != 1 {
		t.Errorf("got %+v", got)
	}
	if got[1].ExitCode != nil {
		t.Errorf("null exit code should stay nil, got %v", *got[1].ExitCode)
	}
}

func TestParseTaps(t *testing.T) {
	got, err := ParseTaps(fixture(t, "taps.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Tap{
		{Name: "anomalyco/tap", Remote: "https://github.com/anomalyco/homebrew-tap.git", Path: "/opt/homebrew/Library/Taps/anomalyco/homebrew-tap", Formulae: 2, Casks: 1},
		{Name: "homebrew/core", Path: "/opt/homebrew/Library/Taps/homebrew/homebrew-core", Official: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestParseDoctor(t *testing.T) {
	got := ParseDoctor(string(fixture(t, "doctor.txt")))
	want := []DoctorWarning{
		{Title: "The following taps are not trusted:", Body: "oven-sh/bun\n\nUntap them with:\n  brew untap oven-sh/bun"},
		{Title: "Some installed casks are deprecated or disabled.", Body: "fontbase"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestParseDoctorClean(t *testing.T) {
	if got := ParseDoctor("Your system is ready to brew.\n"); len(got) != 0 {
		t.Errorf("want no warnings, got %+v", got)
	}
}
```

- [ ] **Step 8: Run the tests to verify they fail**

Run: `go test ./internal/brew/`
Expected: FAIL to compile with `undefined: Package`, `undefined: ParseInfo` and similar.

- [ ] **Step 9: Create `internal/brew/types.go`**

```go
// Package brew is the only package that knows how to run Homebrew and parse
// its output. Everything else works with the plain Go types defined here.
package brew

import "time"

// Kind distinguishes formulae from casks.
type Kind int

const (
	KindFormula Kind = iota
	KindCask
)

func (k Kind) String() string {
	if k == KindCask {
		return "cask"
	}
	return "formula"
}

// Package is one installed formula or cask.
type Package struct {
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"` // "fd", or "user/tap/name" for tap formulae
	Kind        Kind      `json:"kind"`
	Tap         string    `json:"tap"`
	Desc        string    `json:"desc"`
	Homepage    string    `json:"homepage"`
	Version     string    `json:"version"` // installed version
	Latest      string    `json:"latest"`  // newest version known to brew
	OnRequest   bool      `json:"on_request"`
	Pinned      bool      `json:"pinned"`
	Outdated    bool      `json:"outdated"`
	Deprecated  bool      `json:"deprecated"`
	Caveats     string    `json:"caveats"`
	InstalledAt time.Time `json:"installed_at"`
	Deps        []string  `json:"deps"`        // all runtime deps (transitive), full names
	DirectDeps  []string  `json:"direct_deps"` // runtime deps declared directly
}

// Outdated is one row of `brew outdated --json=v2`.
type Outdated struct {
	Name      string `json:"name"`
	Kind      Kind   `json:"kind"`
	Installed string `json:"installed"`
	Current   string `json:"current"`
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
type Tap struct {
	Name     string `json:"name"`
	Remote   string `json:"remote"`
	Path     string `json:"path"`
	Official bool   `json:"official"`
	Formulae int    `json:"formulae"`
	Casks    int    `json:"casks"`
}

// DoctorWarning is one "Warning:" block from `brew doctor`.
type DoctorWarning struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}
```

- [ ] **Step 10: Create `internal/brew/parse.go`**

```go
package brew

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// jsonPayload drops anything brew prints before the JSON document, such as
// "==> Downloading Homebrew API data".
func jsonPayload(b []byte) []byte {
	if i := bytes.IndexAny(b, "{["); i > 0 {
		return b[i:]
	}
	return b
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type infoJSON struct {
	Formulae []struct {
		Name     string  `json:"name"`
		FullName string  `json:"full_name"`
		Tap      string  `json:"tap"`
		Desc     *string `json:"desc"`
		Homepage string  `json:"homepage"`
		Versions struct {
			Stable string `json:"stable"`
		} `json:"versions"`
		Installed []struct {
			Version            string `json:"version"`
			Time               int64  `json:"time"`
			InstalledOnRequest bool   `json:"installed_on_request"`
			RuntimeDeps        []struct {
				FullName         string `json:"full_name"`
				DeclaredDirectly bool   `json:"declared_directly"`
			} `json:"runtime_dependencies"`
		} `json:"installed"`
		Pinned     bool    `json:"pinned"`
		Outdated   bool    `json:"outdated"`
		Deprecated bool    `json:"deprecated"`
		Caveats    *string `json:"caveats"`
	} `json:"formulae"`
	Casks []struct {
		Token         string   `json:"token"`
		FullToken     string   `json:"full_token"`
		Tap           string   `json:"tap"`
		Names         []string `json:"name"`
		Desc          *string  `json:"desc"`
		Homepage      string   `json:"homepage"`
		Version       string   `json:"version"`
		Installed     *string  `json:"installed"`
		InstalledTime *int64   `json:"installed_time"`
		Pinned        bool     `json:"pinned"`
		Outdated      bool     `json:"outdated"`
		Deprecated    bool     `json:"deprecated"`
		Caveats       *string  `json:"caveats"`
	} `json:"casks"`
}

// ParseInfo parses `brew info --json=v2 --installed`.
func ParseInfo(b []byte) ([]Package, error) {
	var in infoJSON
	if err := json.Unmarshal(jsonPayload(b), &in); err != nil {
		return nil, fmt.Errorf("parse brew info: %w", err)
	}
	pkgs := make([]Package, 0, len(in.Formulae)+len(in.Casks))
	for _, f := range in.Formulae {
		p := Package{
			Name: f.Name, FullName: f.FullName, Kind: KindFormula, Tap: f.Tap,
			Desc: deref(f.Desc), Homepage: f.Homepage, Latest: f.Versions.Stable,
			Pinned: f.Pinned, Outdated: f.Outdated, Deprecated: f.Deprecated,
			Caveats: strings.TrimSpace(deref(f.Caveats)),
		}
		if n := len(f.Installed); n > 0 {
			inst := f.Installed[n-1] // newest keg
			p.Version = inst.Version
			p.OnRequest = inst.InstalledOnRequest
			p.InstalledAt = time.Unix(inst.Time, 0)
			for _, d := range inst.RuntimeDeps {
				p.Deps = append(p.Deps, d.FullName)
				if d.DeclaredDirectly {
					p.DirectDeps = append(p.DirectDeps, d.FullName)
				}
			}
		}
		pkgs = append(pkgs, p)
	}
	for _, c := range in.Casks {
		p := Package{
			Name: c.Token, FullName: c.FullToken, Kind: KindCask, Tap: c.Tap,
			Desc: deref(c.Desc), Homepage: c.Homepage, Version: deref(c.Installed),
			Latest: c.Version, OnRequest: true, Pinned: c.Pinned, Outdated: c.Outdated,
			Deprecated: c.Deprecated, Caveats: strings.TrimSpace(deref(c.Caveats)),
		}
		if p.Desc == "" && len(c.Names) > 0 {
			p.Desc = c.Names[0]
		}
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
}

type outdatedItem struct {
	Name              string   `json:"name"`
	InstalledVersions []string `json:"installed_versions"`
	CurrentVersion    string   `json:"current_version"`
	Pinned            bool     `json:"pinned"`
}

// ParseOutdated parses `brew outdated --json=v2`.
func ParseOutdated(b []byte) ([]Outdated, error) {
	var in outdatedJSON
	if err := json.Unmarshal(jsonPayload(b), &in); err != nil {
		return nil, fmt.Errorf("parse brew outdated: %w", err)
	}
	var out []Outdated
	add := func(items []outdatedItem, k Kind) {
		for _, it := range items {
			o := Outdated{Name: it.Name, Kind: k, Current: it.CurrentVersion, Pinned: it.Pinned}
			if n := len(it.InstalledVersions); n > 0 {
				o.Installed = it.InstalledVersions[n-1]
			}
			out = append(out, o)
		}
	}
	add(in.Formulae, KindFormula)
	add(in.Casks, KindCask)
	return out, nil
}

// ParseServices parses `brew services list --json`.
func ParseServices(b []byte) ([]Service, error) {
	var out []Service
	if err := json.Unmarshal(jsonPayload(b), &out); err != nil {
		return nil, fmt.Errorf("parse brew services: %w", err)
	}
	return out, nil
}

type tapJSON struct {
	Name         string   `json:"name"`
	Remote       *string  `json:"remote"`
	Path         string   `json:"path"`
	Official     bool     `json:"official"`
	FormulaNames []string `json:"formula_names"`
	CaskTokens   []string `json:"cask_tokens"`
}

// ParseTaps parses `brew tap-info --json --installed`.
func ParseTaps(b []byte) ([]Tap, error) {
	var in []tapJSON
	if err := json.Unmarshal(jsonPayload(b), &in); err != nil {
		return nil, fmt.Errorf("parse brew tap-info: %w", err)
	}
	out := make([]Tap, 0, len(in))
	for _, t := range in {
		out = append(out, Tap{
			Name: t.Name, Remote: deref(t.Remote), Path: t.Path, Official: t.Official,
			Formulae: len(t.FormulaNames), Casks: len(t.CaskTokens),
		})
	}
	return out, nil
}

// ParseDoctor splits `brew doctor` output into one warning per "Warning:"
// line. Text before the first warning (the standard disclaimer) is dropped.
func ParseDoctor(out string) []DoctorWarning {
	var ws []DoctorWarning
	var body []string
	flush := func() {
		if len(ws) > 0 {
			ws[len(ws)-1].Body = strings.TrimSpace(strings.Join(body, "\n"))
		}
		body = nil
	}
	for _, line := range strings.Split(out, "\n") {
		if title, ok := strings.CutPrefix(line, "Warning: "); ok {
			flush()
			ws = append(ws, DoctorWarning{Title: strings.TrimSpace(title)})
			continue
		}
		if len(ws) > 0 {
			body = append(body, line)
		}
	}
	flush()
	return ws
}
```

- [ ] **Step 11: Run the tests to verify they pass**

Run: `go test ./internal/brew/`
Expected: `ok  	brewery/internal/brew`

- [ ] **Step 12: Commit**

```bash
git add go.mod internal/brew
git commit -m "feat(brew): parse Homebrew JSON and doctor output"
```

---

### Task 2: Brew client (runs read-only queries)

**Files:**
- Create: `internal/brew/client.go`
- Create: `Makefile`
- Test: `internal/brew/client_test.go`, `internal/brew/live_test.go`

**Interfaces:**
- Consumes: the parsers from Task 1.
- Produces: `brew.ErrNotFound`, `brew.Locate() (string, error)`, `brew.Runner` interface `Run(ctx, args ...string) (stdout, stderr []byte, err error)`, `brew.ExecRunner{Bin string; Env []string}`, `brew.QueryEnv []string`, `brew.QueryError{Args []string; Stderr string; Err error}`, `brew.NewClient(Runner) *Client` with field `Timeout time.Duration` and methods `Installed`, `Outdated`, `Services`, `Taps`, `Doctor`, each `(ctx context.Context) ([]T, error)`.

`brew doctor` exits 1 whenever it finds warnings, so the client treats an `*exec.ExitError` from doctor as a normal result. Doctor's warnings go to stderr, so stdout and stderr are parsed together.


- [ ] **Step 1: Write the failing tests** in `internal/brew/client_test.go`

```go
package brew

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// fakeRunner returns canned output keyed by the joined argument list.
type fakeRunner struct {
	out   map[string][]byte
	err   map[string]error
	delay time.Duration
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
		return f.out[key], []byte("Error: something broke"), err
	}
	return f.out[key], nil, nil
}

func TestClientInstalled(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{"info --json=v2 --installed": fixture(t, "info.json")}})
	pkgs, err := c.Installed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 7 {
		t.Errorf("got %d packages", len(pkgs))
	}
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

func TestClientTimeout(t *testing.T) {
	c := NewClient(fakeRunner{delay: time.Second})
	c.Timeout = 10 * time.Millisecond
	_, err := c.Taps(context.Background())
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("want timeout error, got %v", err)
	}
}

func TestClientDoctorExitOneIsNotAnError(t *testing.T) {
	exitErr := exec.Command("false").Run() // a real *exec.ExitError with status 1
	c := NewClient(fakeRunner{
		out: map[string][]byte{"doctor": fixture(t, "doctor.txt")},
		err: map[string]error{"doctor": exitErr},
	})
	ws, err := c.Doctor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 {
		t.Errorf("got %d warnings, want 2", len(ws))
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/brew/`
Expected: FAIL to compile with `undefined: NewClient`, `undefined: QueryError`.

- [ ] **Step 3: Create `internal/brew/client.go`**

```go
package brew

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotFound means no brew executable could be located.
var ErrNotFound = errors.New("brew not found")

// Locate finds the brew executable: $HOMEBREW_PREFIX first, then the standard
// Apple Silicon and Intel prefixes, then $PATH.
func Locate() (string, error) {
	var candidates []string
	if prefix := os.Getenv("HOMEBREW_PREFIX"); prefix != "" {
		candidates = append(candidates, filepath.Join(prefix, "bin", "brew"))
	}
	candidates = append(candidates, "/opt/homebrew/bin/brew", "/usr/local/bin/brew")
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	if p, err := exec.LookPath("brew"); err == nil {
		return p, nil
	}
	return "", ErrNotFound
}

// Runner runs brew with the given arguments and returns its output.
type Runner interface {
	Run(ctx context.Context, args ...string) (stdout, stderr []byte, err error)
}

// QueryEnv keeps read-only queries fast and their output machine-readable.
var QueryEnv = []string{
	"HOMEBREW_NO_AUTO_UPDATE=1",
	"HOMEBREW_NO_ENV_HINTS=1",
	"HOMEBREW_NO_COLOR=1",
}

// ExecRunner runs the real brew binary.
type ExecRunner struct {
	Bin string
	Env []string // appended to os.Environ()
}

func (r ExecRunner) Run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, r.Bin, args...)
	cmd.Env = append(os.Environ(), r.Env...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	return out.Bytes(), errb.Bytes(), err
}

// QueryError reports a failed brew query with enough detail to show the user.
type QueryError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *QueryError) Error() string {
	msg := fmt.Sprintf("brew %s: %v", strings.Join(e.Args, " "), e.Err)
	if s := strings.TrimSpace(e.Stderr); s != "" {
		msg += "\n" + s
	}
	return msg
}

func (e *QueryError) Unwrap() error { return e.Err }

// Client runs read-only brew queries.
type Client struct {
	r       Runner
	Timeout time.Duration
}

func NewClient(r Runner) *Client {
	return &Client{r: r, Timeout: 30 * time.Second}
}

func (c *Client) query(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	out, errb, err := c.r.Run(ctx, args...)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timed out after %s", c.Timeout)
		}
		return nil, &QueryError{Args: args, Stderr: string(errb), Err: err}
	}
	return out, nil
}

func (c *Client) Installed(ctx context.Context) ([]Package, error) {
	out, err := c.query(ctx, "info", "--json=v2", "--installed")
	if err != nil {
		return nil, err
	}
	return ParseInfo(out)
}

func (c *Client) Outdated(ctx context.Context) ([]Outdated, error) {
	out, err := c.query(ctx, "outdated", "--json=v2")
	if err != nil {
		return nil, err
	}
	return ParseOutdated(out)
}

func (c *Client) Services(ctx context.Context) ([]Service, error) {
	out, err := c.query(ctx, "services", "list", "--json")
	if err != nil {
		return nil, err
	}
	return ParseServices(out)
}

func (c *Client) Taps(ctx context.Context) ([]Tap, error) {
	out, err := c.query(ctx, "tap-info", "--json", "--installed")
	if err != nil {
		return nil, err
	}
	return ParseTaps(out)
}

// Doctor runs `brew doctor`. It exits 1 whenever it finds warnings, so a
// non-zero exit with output is a normal result, not an error.
func (c *Client) Doctor(ctx context.Context) ([]DoctorWarning, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	out, errb, err := c.r.Run(ctx, "doctor")
	combined := string(out) + "\n" + string(errb)
	var exitErr *exec.ExitError
	if err != nil && (!errors.As(err, &exitErr) || ctx.Err() != nil) {
		return nil, &QueryError{Args: []string{"doctor"}, Stderr: string(errb), Err: err}
	}
	return ParseDoctor(combined), nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/brew/`
Expected: `ok  	brewery/internal/brew`

- [ ] **Step 5: Create `internal/brew/live_test.go`** (only runs with `-tags live`)

```go
//go:build live

package brew

import (
	"context"
	"testing"
)

// Runs the real brew, read-only, to catch output-format drift.
// Run with: go test -tags live ./internal/brew/
func TestLiveQueries(t *testing.T) {
	bin, err := Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	c := NewClient(ExecRunner{Bin: bin, Env: QueryEnv})
	ctx := context.Background()

	pkgs, err := c.Installed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Error("no installed packages parsed")
	}
	for _, p := range pkgs {
		if p.Name == "" || p.Version == "" {
			t.Errorf("incomplete package: %+v", p)
		}
	}
	if _, err := c.Outdated(ctx); err != nil {
		t.Error(err)
	}
	if _, err := c.Services(ctx); err != nil {
		t.Error(err)
	}
	if _, err := c.Taps(ctx); err != nil {
		t.Error(err)
	}
	if _, err := c.Doctor(ctx); err != nil {
		t.Error(err)
	}
}
```

- [ ] **Step 6: Create `Makefile`** (recipe lines must start with a tab)

```makefile
.PHONY: build test test-live run

build:
	go build -o brewery ./cmd/brewery

test:
	go test ./...

test-live:
	go test -tags live -count=1 ./internal/brew/

run: build
	./brewery
```

- [ ] **Step 7: Run the live test against real brew**

Run: `make test-live`
Expected: `ok  	brewery/internal/brew` (takes about 4–5s; it runs real read-only brew commands).

- [ ] **Step 8: Commit**

```bash
git add Makefile internal/brew
git commit -m "feat(brew): add query client with timeouts and live test"
```

---

### Task 3: Dependency index

**Files:**
- Create: `internal/state/index.go`
- Test: `internal/state/index_test.go`

**Interfaces:**
- Consumes: `brew.Package` (`FullName`, `Kind`, `Deps`, `DirectDeps`).
- Produces: `state.NewIndex([]brew.Package) *Index`, `(*Index).Get(fullName) (brew.Package, bool)`, `(*Index).UsedBy(fullName) []string` (sorted; transitive dependents, matching `runtime_dependencies`), `(*Index).IsLeaf(brew.Package) bool` (a formula nothing depends on, like `brew leaves`), `(*Index).Tree(fullName) []string` (box-drawing lines; repeated subtrees end with ` …`).


- [ ] **Step 1: Write the failing tests** in `internal/state/index_test.go`

```go
package state

import (
	"reflect"
	"testing"

	"brewery/internal/brew"
)

// git -> pcre2 -> json-c, git also depends on json-c indirectly; fd has no deps.
var testPkgs = []brew.Package{
	{Name: "git", FullName: "git", Kind: brew.KindFormula, OnRequest: true, Deps: []string{"pcre2", "json-c"}, DirectDeps: []string{"pcre2"}},
	{Name: "pcre2", FullName: "pcre2", Kind: brew.KindFormula, Deps: []string{"json-c"}, DirectDeps: []string{"json-c"}},
	{Name: "json-c", FullName: "json-c", Kind: brew.KindFormula},
	{Name: "fd", FullName: "fd", Kind: brew.KindFormula, OnRequest: true},
	{Name: "docker-desktop", FullName: "docker-desktop", Kind: brew.KindCask, OnRequest: true},
}

func TestUsedBy(t *testing.T) {
	ix := NewIndex(testPkgs)
	tests := map[string][]string{
		"json-c": {"git", "pcre2"},
		"pcre2":  {"git"},
		"fd":     nil,
	}
	for name, want := range tests {
		if got := ix.UsedBy(name); !reflect.DeepEqual(got, want) {
			t.Errorf("UsedBy(%s) = %v, want %v", name, got, want)
		}
	}
}

func TestIsLeaf(t *testing.T) {
	ix := NewIndex(testPkgs)
	want := map[string]bool{"git": true, "pcre2": false, "json-c": false, "fd": true, "docker-desktop": false}
	for _, p := range testPkgs {
		if got := ix.IsLeaf(p); got != want[p.Name] {
			t.Errorf("IsLeaf(%s) = %v, want %v", p.Name, got, want[p.Name])
		}
	}
}

func TestTree(t *testing.T) {
	ix := NewIndex(testPkgs)
	want := []string{"git", "└─ pcre2", "   └─ json-c"}
	if got := ix.Tree("git"); !reflect.DeepEqual(got, want) {
		t.Errorf("Tree(git) = %q, want %q", got, want)
	}
	if got := ix.Tree("fd"); !reflect.DeepEqual(got, []string{"fd"}) {
		t.Errorf("Tree(fd) = %q", got)
	}
}

func TestTreeMarksRepeats(t *testing.T) {
	pkgs := []brew.Package{
		{FullName: "a", DirectDeps: []string{"b", "c"}},
		{FullName: "b", DirectDeps: []string{"c"}},
		{FullName: "c"},
	}
	want := []string{"a", "├─ b", "│  └─ c", "└─ c …"}
	if got := NewIndex(pkgs).Tree("a"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/state/`
Expected: FAIL to compile with `undefined: NewIndex`.

- [ ] **Step 3: Create `internal/state/index.go`**

```go
// Package state holds brewery's picture of the Homebrew installation and the
// facts derived from it that brew does not report directly.
package state

import (
	"sort"

	"brewery/internal/brew"
)

// Index answers dependency questions about a set of installed packages.
type Index struct {
	byFull map[string]brew.Package
	usedBy map[string][]string // full name -> full names of installed packages that depend on it
}

func NewIndex(pkgs []brew.Package) *Index {
	ix := &Index{byFull: make(map[string]brew.Package, len(pkgs)), usedBy: map[string][]string{}}
	for _, p := range pkgs {
		ix.byFull[p.FullName] = p
	}
	for _, p := range pkgs {
		for _, d := range p.Deps {
			ix.usedBy[d] = append(ix.usedBy[d], p.FullName)
		}
	}
	for k := range ix.usedBy {
		sort.Strings(ix.usedBy[k])
	}
	return ix
}

// Get returns the package with the given full name.
func (ix *Index) Get(fullName string) (brew.Package, bool) {
	p, ok := ix.byFull[fullName]
	return p, ok
}

// UsedBy lists installed packages that depend on fullName, directly or not.
func (ix *Index) UsedBy(fullName string) []string {
	return ix.usedBy[fullName]
}

// IsLeaf reports whether p is a formula that no other installed package
// depends on (the same definition as `brew leaves`).
func (ix *Index) IsLeaf(p brew.Package) bool {
	return p.Kind == brew.KindFormula && len(ix.usedBy[p.FullName]) == 0
}

// Tree renders p's direct dependencies recursively, one line per node,
// using box-drawing prefixes. Repeated subtrees are shown once.
func (ix *Index) Tree(fullName string) []string {
	lines := []string{fullName}
	seen := map[string]bool{fullName: true}
	var walk func(name, prefix string)
	walk = func(name, prefix string) {
		p := ix.byFull[name]
		for i, d := range p.DirectDeps {
			last := i == len(p.DirectDeps)-1
			branch, next := "├─ ", "│  "
			if last {
				branch, next = "└─ ", "   "
			}
			if seen[d] {
				lines = append(lines, prefix+branch+d+" …")
				continue
			}
			seen[d] = true
			lines = append(lines, prefix+branch+d)
			walk(d, prefix+next)
		}
	}
	walk(fullName, "")
	return lines
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/state/`
Expected: `ok  	brewery/internal/state`

- [ ] **Step 5: Commit**

```bash
git add internal/state
git commit -m "feat(state): add dependency index (used-by, leaves, tree)"
```

---

### Task 4: Snapshot and cache

**Files:**
- Create: `internal/state/snapshot.go`
- Test: `internal/state/snapshot_test.go`

**Interfaces:**
- Consumes: the brew types.
- Produces: `state.Snapshot{Version int; Packages []brew.Package; Outdated []brew.Outdated; Services []brew.Service; Taps []brew.Tap; Doctor []brew.DoctorWarning; DoctorRan bool; FetchedAt time.Time}`, `state.DefaultCachePath() string`, `state.Load(path) (Snapshot, error)`, `state.Save(path, Snapshot) error`.

A cache file with a different `version` is ignored instead of half-loaded. `Save` always stamps the current version.


- [ ] **Step 1: Write the failing tests** in `internal/state/snapshot_test.go`

```go
package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"brewery/internal/brew"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "snapshot.json")
	in := Snapshot{
		Packages:  testPkgs,
		Outdated:  []brew.Outdated{{Name: "docker-desktop", Kind: brew.KindCask, Installed: "4.39.0", Current: "4.95.0"}},
		Taps:      []brew.Tap{{Name: "anomalyco/tap", Formulae: 2}},
		DoctorRan: true,
		FetchedAt: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC),
	}
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	in.Version = snapshotVersion
	if !reflect.DeepEqual(out, in) {
		t.Errorf("round trip changed snapshot:\n got %+v\nwant %+v", out, in)
	}
}

func TestLoadMissingFile(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil || len(s.Packages) != 0 {
		t.Errorf("want empty snapshot and error, got %+v, %v", s, err)
	}
}

func TestLoadIgnoresOtherVersions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	os.WriteFile(path, []byte(`{"version": 999, "packages": [{"name": "fd"}]}`), 0o644)
	if s, err := Load(path); err == nil || len(s.Packages) != 0 {
		t.Errorf("want version mismatch error, got %+v, %v", s, err)
	}
}

func TestDefaultCachePathHonorsXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg")
	if got := DefaultCachePath(); got != "/tmp/xdg/brewery/snapshot.json" {
		t.Errorf("got %s", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/state/`
Expected: FAIL to compile with `undefined: Snapshot`, `undefined: Save`.

- [ ] **Step 3: Create `internal/state/snapshot.go`**

```go
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"brewery/internal/brew"
)

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
// to ~/.cache/brewery/snapshot.json.
func DefaultCachePath() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "brewery", "snapshot.json")
}

// Load reads a cached snapshot. A missing, corrupt or outdated file returns
// an empty Snapshot and an error the caller may ignore.
func Load(path string) (Snapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot cache: %w", err)
	}
	if s.Version != snapshotVersion {
		return Snapshot{}, fmt.Errorf("snapshot cache version %d, want %d", s.Version, snapshotVersion)
	}
	return s, nil
}

// Save writes the snapshot atomically (temp file + rename).
func Save(path string, s Snapshot) error {
	s.Version = snapshotVersion
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "snapshot-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/state/`
Expected: `ok  	brewery/internal/state`

- [ ] **Step 5: Commit**

```bash
git add internal/state
git commit -m "feat(state): add snapshot cache with atomic writes"
```

---

### Task 5: Theme (Catppuccin Macchiato)

**Files:**
- Modify: `go.mod`, `go.sum` (add Charm dependencies)
- Create: `internal/theme/theme.go`
- Test: `internal/theme/theme_test.go`

**Interfaces:**
- Produces: `theme.Palette`, `theme.Macchiato`, `theme.New(Palette) Styles`, `(Styles).Mode(name string) lipgloss.Style`.
- `Styles` fields used by the UI: `P`, `Text`, `Muted`, `Accent`, `Title`, `OK`, `Warn`, `Err`, `Info`, `Pane`, `PaneFocused`, `PaneTitle`, `PaneTitleFocused`, `Tab`, `TabActive`, `StatusBar`, `TableHeader`, `TableCell`, `TableSelected`.

brewery uses the terminal's own background. The theme only sets foreground colors and a few accent backgrounds (the active tab, the selected row, mode badges, the status bar).

- [ ] **Step 1: Add the pinned Charm dependencies**

```bash
go get charm.land/bubbletea/v2@v2.0.10 charm.land/lipgloss/v2@v2.0.6 charm.land/bubbles/v2@v2.2.1 github.com/charmbracelet/x/ansi
```

Expected: `go.mod` lists the three `charm.land` modules. If the local Go is older than 1.25, Go prints `switching to go1.2x` and downloads a toolchain. That's expected.


- [ ] **Step 2: Write the failing tests** in `internal/theme/theme_test.go`

```go
package theme

import (
	"fmt"
	"testing"
)

func TestModeColorsAreDistinct(t *testing.T) {
	st := New(Macchiato)
	seen := map[string]string{}
	for _, mode := range []string{"NORMAL", "COMMAND", "FILTER", "RUNNING", "INSERT"} {
		bg := st.Mode(mode).GetBackground()
		r, g, b, _ := bg.RGBA()
		key := fmt.Sprint(r, g, b)
		if other, dup := seen[key]; dup {
			t.Errorf("%s and %s share a badge color", mode, other)
		}
		seen[key] = mode
	}
}

func TestUnknownModeFallsBackToNormal(t *testing.T) {
	st := New(Macchiato)
	if st.Mode("WHATEVER").GetBackground() != st.Mode("NORMAL").GetBackground() {
		t.Error("unknown modes should look like NORMAL")
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/theme/`
Expected: FAIL to compile with `undefined: New`, `undefined: Macchiato`.

- [ ] **Step 4: Create `internal/theme/theme.go`**

```go
// Package theme maps semantic UI roles to colors. Views use Styles only, never
// raw colors, so adding a palette (another Catppuccin flavor) touches one file.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Palette holds the Catppuccin color slots brewery uses.
type Palette struct {
	Mantle, Crust            color.Color
	Surface0, Surface1       color.Color
	Overlay0, Subtext0, Text color.Color
	Mauve, Blue, Teal, Green color.Color
	Yellow, Peach, Red, Pink color.Color
}

// Macchiato is Catppuccin Macchiato (https://catppuccin.com/palette).
var Macchiato = Palette{
	Mantle: lipgloss.Color("#1e2030"), Crust: lipgloss.Color("#181926"),
	Surface0: lipgloss.Color("#363a4f"), Surface1: lipgloss.Color("#494d64"),
	Overlay0: lipgloss.Color("#6e738d"), Subtext0: lipgloss.Color("#a5adcb"), Text: lipgloss.Color("#cad3f5"),
	Mauve: lipgloss.Color("#c6a0f6"), Blue: lipgloss.Color("#8aadf4"), Teal: lipgloss.Color("#8bd5ca"), Green: lipgloss.Color("#a6da95"),
	Yellow: lipgloss.Color("#eed49f"), Peach: lipgloss.Color("#f5a97f"), Red: lipgloss.Color("#ed8796"), Pink: lipgloss.Color("#f5bde6"),
}

// Styles are the semantic styles views render with.
type Styles struct {
	P Palette

	Text, Muted, Accent, Title lipgloss.Style
	OK, Warn, Err, Info        lipgloss.Style

	Pane, PaneFocused           lipgloss.Style
	PaneTitle, PaneTitleFocused lipgloss.Style

	Tab, TabActive lipgloss.Style
	StatusBar      lipgloss.Style

	TableHeader, TableCell, TableSelected lipgloss.Style
}

func New(p Palette) Styles {
	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	pane := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Surface1).Padding(0, 1)
	return Styles{
		P:    p,
		Text: fg(p.Text), Muted: fg(p.Overlay0), Accent: fg(p.Mauve), Title: fg(p.Blue).Bold(true),
		OK: fg(p.Green), Warn: fg(p.Yellow), Err: fg(p.Red), Info: fg(p.Teal),

		Pane:             pane,
		PaneFocused:      pane.BorderForeground(p.Mauve),
		PaneTitle:        fg(p.Subtext0),
		PaneTitleFocused: fg(p.Mauve).Bold(true),

		Tab:       fg(p.Subtext0).Padding(0, 1),
		TabActive: lipgloss.NewStyle().Foreground(p.Crust).Background(p.Mauve).Bold(true).Padding(0, 1),
		StatusBar: lipgloss.NewStyle().Foreground(p.Subtext0).Background(p.Mantle),

		TableHeader: fg(p.Subtext0).Bold(true).Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(p.Surface1),
		TableCell:     fg(p.Text).Padding(0, 1),
		TableSelected: lipgloss.NewStyle().Foreground(p.Text).Background(p.Surface0).Bold(true),
	}
}

// Mode returns the status-bar badge style for an editor-style mode name.
func (s Styles) Mode(mode string) lipgloss.Style {
	c := s.P.Mauve
	switch mode {
	case "COMMAND":
		c = s.P.Blue
	case "FILTER":
		c = s.P.Teal
	case "RUNNING":
		c = s.P.Peach
	case "INSERT":
		c = s.P.Green
	}
	return lipgloss.NewStyle().Foreground(s.P.Crust).Background(c).Bold(true).Padding(0, 1)
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/theme/`
Expected: `ok  	brewery/internal/theme`

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/theme
git commit -m "feat(theme): add Catppuccin Macchiato styles and mode badges"
```

---

### Task 6: UI building blocks and list tabs (Outdated, Services, Taps)

**Files:**
- Create: `internal/ui/components.go`, `internal/ui/list.go`
- Test: `internal/ui/helpers_test.go` (shared test helpers for every UI test), `internal/ui/list_test.go`

**Interfaces:**
- Consumes: `theme.Styles`, brew types, the brew fixtures (read from `../brew/testdata`).
- Produces (package-internal; later UI tasks use these exact names):
  - `pane(st theme.Styles, title, body string, w, h int, focused bool) string`: a box of exactly w×h.
  - `paneInner(w, h int) (int, int)`: the content size inside a pane.
  - `type col struct{ title string; width int }` (width 0 = flexible).
  - `tableKeys() table.KeyMap`, `newTable(st, cols []col) table.Model`, `sizeTable(*table.Model, []col, w, h int)`, `setRows(*table.Model, []table.Row)`.
  - `kv(st, pairs ...string) string`, `errorBody(st, err) string`.
  - `type listTab`, `newListTab(st, name string, cols []col, empty string) listTab`, methods `setRows([]table.Row)`, `resize(w, h int)`, `update(tea.KeyPressMsg) tea.Cmd`, `view(err error) string`.
  - `outdatedRows`, `serviceRows`, `tapRows` and `outdatedCols`, `serviceCols`, `tapCols`.
  - Test helpers: `testStyles`, `stripANSI`, `readFixture`, `fixtureSnapshot(t) state.Snapshot`, `press(key string) tea.KeyPressMsg`.

Two Bubbles table quirks the code handles (both were found by testing):
1. The table panics if rows are set before columns, so `newTable` sets columns right away.
2. An empty table parks its cursor at -1, and it stays there when rows arrive, so `setRows` moves it back to row 0.


- [ ] **Step 1: Create the shared test helpers** in `internal/ui/helpers_test.go`

```go
package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/state"
	"brewery/internal/theme"
)

var testStyles = theme.New(theme.Macchiato)

func stripANSI(s string) string { return ansi.Strip(s) }

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "brew", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// fixtureSnapshot is the brew package's test fixtures, parsed.
func fixtureSnapshot(t *testing.T) state.Snapshot {
	t.Helper()
	var s state.Snapshot
	var err error
	if s.Packages, err = brew.ParseInfo(readFixture(t, "info.json")); err != nil {
		t.Fatal(err)
	}
	if s.Outdated, err = brew.ParseOutdated(readFixture(t, "outdated.json")); err != nil {
		t.Fatal(err)
	}
	if s.Services, err = brew.ParseServices(readFixture(t, "services.json")); err != nil {
		t.Fatal(err)
	}
	if s.Taps, err = brew.ParseTaps(readFixture(t, "taps.json")); err != nil {
		t.Fatal(err)
	}
	s.Doctor, s.DoctorRan = brew.ParseDoctor(string(readFixture(t, "doctor.txt"))), true
	return s
}

func press(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	r := []rune(k)[0]
	return tea.KeyPressMsg{Code: r, Text: k}
}
```

- [ ] **Step 2: Write the failing tests** in `internal/ui/list_test.go`

```go
package ui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

func TestListTabRendersRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	lt := newListTab(testStyles, "Outdated", outdatedCols, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(snap.Outdated))
	out := stripANSI(lt.view(nil))
	for _, want := range []string{"Outdated · 2", "NAME", "lua-language-server", "3.19.1", "3.19.2", "4.95.0,242831", "cask"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if h, w := lipgloss.Height(lt.view(nil)), lipgloss.Width(lt.view(nil)); h != 10 || w != 120 {
		t.Errorf("rendered %dx%d, want 120x10", w, h)
	}
}

func TestListTabEmptyAndError(t *testing.T) {
	lt := newListTab(testStyles, "Outdated", outdatedCols, "✓ everything is up to date")
	lt.resize(80, 8)
	if out := stripANSI(lt.view(nil)); !strings.Contains(out, "everything is up to date") {
		t.Errorf("empty message missing:\n%s", out)
	}
	out := stripANSI(lt.view(errors.New("brew outdated --json=v2: exit status 1")))
	if !strings.Contains(out, "couldn't load this view") || !strings.Contains(out, "exit status 1") || !strings.Contains(out, "press r to retry") {
		t.Errorf("error body missing:\n%s", out)
	}
}

func TestServiceAndTapRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	if got := serviceRows(snap.Services)[0]; got[0] != "postgresql@16" || got[1] != "error (exit 1)" {
		t.Errorf("service row = %v", got)
	}
	if got := serviceRows(snap.Services)[1][1]; got != "none" {
		t.Errorf("status without exit code = %q", got)
	}
	if got := tapRows(snap.Taps)[0]; got[1] != "2" || got[2] != "1" || got[3] != "" {
		t.Errorf("tap row = %v", got)
	}
}

func TestSetRowsRecoversCursorAfterEmpty(t *testing.T) {
	tbl := newTable(testStyles, outdatedCols)
	setRows(&tbl, nil) // an empty table parks the cursor at -1
	setRows(&tbl, []table.Row{{"a", "", "", "", ""}, {"b", "", "", "", ""}})
	if tbl.Cursor() != 0 {
		t.Errorf("cursor = %d, want 0", tbl.Cursor())
	}
	tbl.SetCursor(1)
	setRows(&tbl, []table.Row{{"a", "", "", "", ""}})
	if tbl.Cursor() != 0 {
		t.Errorf("cursor = %d after shrinking, want 0", tbl.Cursor())
	}
}

func TestSizeTableSplitsFlexWidth(t *testing.T) {
	tbl := newTable(testStyles, outdatedCols)
	sizeTable(&tbl, outdatedCols, 100, 10)
	// fixed: 18+18+7+6 = 49, plus 2 padding per column (10) = 59; flex gets 100-59.
	if got := tbl.Columns()[0].Width; got != 41 {
		t.Errorf("flex column width = %d, want 41", got)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/ui/`
Expected: FAIL to compile with `undefined: newListTab`, `undefined: newTable`.

- [ ] **Step 4: Create `internal/ui/components.go`**

```go
package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"brewery/internal/theme"
)

// pane draws a rounded box of exactly w×h cells with a title on its first
// line. Content beyond the box is cut off, not wrapped into the next pane.
func pane(st theme.Styles, title, body string, w, h int, focused bool) string {
	box, ts := st.Pane, st.PaneTitle
	if focused {
		box, ts = st.PaneFocused, st.PaneTitleFocused
	}
	inner := ts.Render(title) + "\n" + body
	return box.Width(w).Height(h).MaxWidth(w).MaxHeight(h).Render(inner)
}

// paneInner returns the content area of a pane of size w×h, excluding the
// border, horizontal padding and title line.
func paneInner(w, h int) (int, int) {
	return max(w-4, 0), max(h-3, 0)
}

// col describes a table column. Width 0 means "take the remaining space".
type col struct {
	title string
	width int
}

// tableKeys is the table's built-in Vim-style navigation, minus the default
// single-letter page keys (u, d, b, f) that brewery uses for actions.
func tableKeys() table.KeyMap {
	return table.KeyMap{
		LineUp:       key.NewBinding(key.WithKeys("up", "k")),
		LineDown:     key.NewBinding(key.WithKeys("down", "j")),
		PageUp:       key.NewBinding(key.WithKeys("pgup")),
		PageDown:     key.NewBinding(key.WithKeys("pgdown")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d")),
		GotoTop:      key.NewBinding(key.WithKeys("home", "g")),
		GotoBottom:   key.NewBinding(key.WithKeys("end", "G")),
	}
}

// newTable creates a table with its columns already set: the table panics
// if rows arrive before columns. sizeTable sets the real widths at render.
func newTable(st theme.Styles, cols []col) table.Model {
	tc := make([]table.Column, len(cols))
	for i, c := range cols {
		tc[i] = table.Column{Title: c.title, Width: max(c.width, len(c.title))}
	}
	return table.New(
		table.WithColumns(tc),
		table.WithFocused(true),
		table.WithKeyMap(tableKeys()),
		table.WithStyles(table.Styles{Header: st.TableHeader, Cell: st.TableCell, Selected: st.TableSelected}),
	)
}

// sizeTable fits t into a w×h area, giving flexible columns the leftover width.
func sizeTable(t *table.Model, cols []col, w, h int) {
	fixed, flex := 0, 0
	for _, c := range cols {
		fixed += c.width + 2 // cell padding
		if c.width == 0 {
			flex++
		}
	}
	tc := make([]table.Column, len(cols))
	for i, c := range cols {
		width := c.width
		if width == 0 {
			width = max((w-fixed)/max(flex, 1), 8)
		}
		tc[i] = table.Column{Title: c.title, Width: width}
	}
	t.SetColumns(tc)
	t.SetWidth(w)
	t.SetHeight(h)
}

// setRows replaces the table rows and keeps the cursor inside the new range.
// An empty table parks its cursor at -1, so bring it back once rows arrive.
func setRows(t *table.Model, rows []table.Row) {
	t.SetRows(rows)
	if c := t.Cursor(); c < 0 || c >= len(rows) {
		t.SetCursor(max(min(c, len(rows)-1), 0))
	}
}

// kv renders aligned "label  value" lines for preview panes.
func kv(st theme.Styles, pairs ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			continue
		}
		b.WriteString(st.Muted.Render(lipgloss.NewStyle().Width(11).Render(pairs[i])))
		b.WriteString(pairs[i+1])
		b.WriteString("\n")
	}
	return b.String()
}

// errorBody explains a failed query inside the pane that needed its data.
func errorBody(st theme.Styles, err error) string {
	return st.Err.Render("✗ couldn't load this view") + "\n\n" +
		st.Muted.Render(err.Error()) + "\n\n" +
		st.Muted.Render("press r to retry")
}
```

- [ ] **Step 5: Create `internal/ui/list.go`**

```go
package ui

import (
	"fmt"
	"strconv"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/theme"
)

// listTab is a full-width table used by the Outdated, Services and Taps tabs.
type listTab struct {
	st    theme.Styles
	name  string
	cols  []col
	empty string // shown when there are no rows
	n     int
	table table.Model
	w, h  int
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
	return cmd
}

func (t *listTab) resize(w, h int) {
	t.w, t.h = w, h
	iw, ih := paneInner(w, h)
	sizeTable(&t.table, t.cols, iw, ih)
}

func (t *listTab) view(err error) string {
	w, h := t.w, t.h
	if err != nil && t.n == 0 {
		return pane(t.st, t.name, errorBody(t.st, err), w, h, true)
	}
	if t.n == 0 {
		return pane(t.st, t.name, t.st.OK.Render(t.empty), w, h, true)
	}
	return pane(t.st, fmt.Sprintf("%s · %d", t.name, t.n), t.table.View(), w, h, true)
}

func outdatedRows(items []brew.Outdated) []table.Row {
	rows := make([]table.Row, len(items))
	for i, o := range items {
		pinned := ""
		if o.Pinned {
			pinned = "pinned"
		}
		rows[i] = table.Row{o.Name, o.Installed, o.Current, o.Kind.String(), pinned}
	}
	return rows
}

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

func tapRows(items []brew.Tap) []table.Row {
	rows := make([]table.Row, len(items))
	for i, t := range items {
		official := ""
		if t.Official {
			official = "yes"
		}
		rows[i] = table.Row{t.Name, strconv.Itoa(t.Formulae), strconv.Itoa(t.Casks), official, t.Remote}
	}
	return rows
}

var (
	outdatedCols = []col{{"NAME", 0}, {"INSTALLED", 18}, {"LATEST", 18}, {"TYPE", 7}, {"PINNED", 6}}
	serviceCols  = []col{{"NAME", 20}, {"STATUS", 16}, {"USER", 10}, {"FILE", 0}}
	tapCols      = []col{{"NAME", 24}, {"FORMULAE", 8}, {"CASKS", 6}, {"OFFICIAL", 8}, {"REMOTE", 0}}
)
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/ui/`
Expected: `ok  	brewery/internal/ui`

- [ ] **Step 7: Commit**

```bash
git add internal/ui
git commit -m "feat(ui): add pane/table components and list tabs"
```

---

### Task 7: Installed tab (filters, table, preview)

**Files:**
- Create: `internal/ui/installed.go`
- Test: `internal/ui/installed_test.go`

**Interfaces:**
- Consumes: `newTable`, `sizeTable`, `setRows`, `pane`, `paneInner`, `kv`, `errorBody`, `col` (Task 6); `state.Index` (Task 3).
- Produces: `type installedTab`, `newInstalledTab(st) installedTab`, methods `setData([]brew.Package, *state.Index)`, `resize(w, h int)`, `selected() (brew.Package, bool)`, `update(tea.KeyPressMsg) tea.Cmd`, `view(err error) string`. Also the fields `rows []brew.Package`, `filter installedFilter`, `table table.Model`, and `filterNames`.

Filters cycle with `f`: **leaves** (default: formulae nothing depends on), **all**, **casks**, **deps** (formulae installed as dependencies). The list takes 55% of the width and the preview the rest. `resize` must run before key handling, because the table ignores navigation until it has been sized.


- [ ] **Step 1: Write the failing tests** in `internal/ui/installed_test.go`

```go
package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"brewery/internal/state"
)

func newFixtureInstalled(t *testing.T) installedTab {
	snap := fixtureSnapshot(t)
	it := newInstalledTab(testStyles)
	it.resize(140, 30)
	it.setData(snap.Packages, state.NewIndex(snap.Packages))
	return it
}

func rowNames(it installedTab) string {
	var n []string
	for _, p := range it.rows {
		n = append(n, p.Name)
	}
	return strings.Join(n, " ")
}

func TestInstalledFilters(t *testing.T) {
	it := newFixtureInstalled(t)
	want := []struct{ filter, names string }{
		{"leaves", "fd git lua-language-server"},
		{"all", "docker-desktop fd fontbase git json-c lua-language-server pcre2"},
		{"casks", "docker-desktop fontbase"},
		{"deps", "json-c pcre2"},
		{"leaves", "fd git lua-language-server"}, // f wraps around
	}
	for i, w := range want {
		if i > 0 {
			it.update(press("f"))
		}
		if filterNames[it.filter] != w.filter {
			t.Fatalf("step %d: filter %s, want %s", i, filterNames[it.filter], w.filter)
		}
		if got := rowNames(it); got != w.names {
			t.Errorf("filter %s: got %q, want %q", w.filter, got, w.names)
		}
	}
}

func TestInstalledNavigationAndPreview(t *testing.T) {
	it := newFixtureInstalled(t)
	it.update(press("j")) // fd -> git
	p, ok := it.selected()
	if !ok || p.Name != "git" {
		t.Fatalf("selected %q, want git", p.Name)
	}
	out := stripANSI(it.view(nil))
	for _, want := range []string{
		"Installed · leaves · 3", "git 2.51.0", "Distributed revision control system",
		"on request · ", "pinned", "Dependencies", "└─ pcre2", "json-c",
		"nothing installed depends on this", "Bash completion has been installed.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if h, w := lipgloss.Height(it.view(nil)), lipgloss.Width(it.view(nil)); h != 30 || w != 140 {
		t.Errorf("rendered %dx%d, want 140x30", w, h)
	}
}

func TestInstalledPreviewForDependencyAndCask(t *testing.T) {
	it := newFixtureInstalled(t)
	it.update(press("f")) // all
	for range 4 {
		it.update(press("j")) // docker-desktop fd fontbase git json-c
	}
	out := stripANSI(it.view(nil))
	if !strings.Contains(out, "as a dependency") || !strings.Contains(out, "git, pcre2") {
		t.Errorf("json-c preview should show it is a dependency used by git and pcre2:\n%s", out)
	}

	it.update(press("g")) // docker-desktop
	out = stripANSI(it.view(nil))
	if !strings.Contains(out, "4.95.0,242831 ↑") || strings.Contains(out, "Dependencies") {
		t.Errorf("cask preview should show the newer version and no dependency section:\n%s", out)
	}
}

func TestInstalledEmptyFilter(t *testing.T) {
	it := newInstalledTab(testStyles)
	it.resize(100, 12)
	it.setData(nil, state.NewIndex(nil))
	if _, ok := it.selected(); ok {
		t.Error("nothing should be selected in an empty list")
	}
	if out := stripANSI(it.view(nil)); !strings.Contains(out, "nothing matches this filter") {
		t.Errorf("empty preview missing:\n%s", out)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/`
Expected: FAIL to compile with `undefined: newInstalledTab`, `undefined: filterNames`.

- [ ] **Step 3: Create `internal/ui/installed.go`**

```go
package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
	"brewery/internal/theme"
)

type installedFilter int

const (
	filterLeaves installedFilter = iota
	filterAll
	filterCasks
	filterDeps
)

var filterNames = [...]string{"leaves", "all", "casks", "deps"}

var installedCols = []col{{"NAME", 0}, {"VERSION", 16}, {"TYPE", 7}, {"STATUS", 10}}

// installedTab lists installed packages with a details preview.
type installedTab struct {
	st     theme.Styles
	filter installedFilter
	all    []brew.Package
	ix     *state.Index
	rows   []brew.Package // the filtered rows, in table order
	table  table.Model
	w, h   int
}

func newInstalledTab(st theme.Styles) installedTab {
	return installedTab{st: st, table: newTable(st, installedCols), ix: state.NewIndex(nil)}
}

func (t *installedTab) setData(pkgs []brew.Package, ix *state.Index) {
	t.all, t.ix = pkgs, ix
	t.apply()
}

func (t *installedTab) apply() {
	t.rows = t.rows[:0]
	for _, p := range t.all {
		keep := false
		switch t.filter {
		case filterAll:
			keep = true
		case filterLeaves:
			keep = t.ix.IsLeaf(p)
		case filterCasks:
			keep = p.Kind == brew.KindCask
		case filterDeps:
			keep = p.Kind == brew.KindFormula && !p.OnRequest
		}
		if keep {
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
		s = append(s, "pinned")
	}
	if p.Outdated {
		s = append(s, "↑")
	}
	return strings.Join(s, " ")
}

func (t *installedTab) selected() (brew.Package, bool) {
	c := t.table.Cursor()
	if c < 0 || c >= len(t.rows) {
		return brew.Package{}, false
	}
	return t.rows[c], true
}

func (t *installedTab) update(msg tea.KeyPressMsg) tea.Cmd {
	if msg.String() == "f" {
		t.filter = (t.filter + 1) % installedFilter(len(filterNames))
		t.table.SetCursor(0)
		t.apply()
		return nil
	}
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}

// resize must run before key handling: the table ignores navigation
// until it has been sized.
func (t *installedTab) resize(w, h int) {
	t.w, t.h = w, h
	iw, ih := paneInner(t.listWidth(), h)
	sizeTable(&t.table, installedCols, iw, ih)
}

func (t *installedTab) listWidth() int { return t.w * 55 / 100 }

func (t *installedTab) view(err error) string {
	w, h, listW := t.w, t.h, t.listWidth()
	title := fmt.Sprintf("Installed · %s · %d   f: filter", filterNames[t.filter], len(t.rows))
	if err != nil && len(t.all) == 0 {
		return pane(t.st, "Installed", errorBody(t.st, err), w, h, true)
	}
	list := pane(t.st, title, t.table.View(), listW, h, true)

	p, ok := t.selected()
	previewTitle, body := "Preview", t.st.Muted.Render("nothing matches this filter")
	if ok {
		previewTitle, body = p.Name, t.preview(p)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, list, pane(t.st, previewTitle, body, w-listW, h, false))
}

func (t *installedTab) preview(p brew.Package) string {
	st := t.st
	var b strings.Builder
	b.WriteString(st.Title.Render(p.Name+" "+p.Version) + "\n")
	if p.Desc != "" {
		b.WriteString(st.Muted.Render(p.Desc) + "\n")
	}
	b.WriteString("\n")

	how := "as a dependency"
	if p.OnRequest {
		how = "on request"
	}
	if !p.InstalledAt.IsZero() {
		how += " · " + p.InstalledAt.Format("2006-01-02")
	}
	latest := p.Latest
	if p.Outdated {
		latest = st.Warn.Render(p.Latest + " ↑")
	}
	pinned := ""
	if p.Pinned {
		pinned = "yes"
	}
	deprecated := ""
	if p.Deprecated {
		deprecated = st.Warn.Render("yes")
	}
	b.WriteString(kv(st, "type", p.Kind.String(), "tap", p.Tap, "installed", how,
		"latest", latest, "pinned", pinned, "deprecated", deprecated, "homepage", p.Homepage))

	if p.Kind == brew.KindFormula {
		b.WriteString("\n" + st.Accent.Render("Dependencies") + "\n")
		if len(p.DirectDeps) == 0 {
			b.WriteString(st.Muted.Render("none") + "\n")
		} else {
			b.WriteString(strings.Join(t.ix.Tree(p.FullName), "\n") + "\n")
		}
		b.WriteString("\n" + st.Accent.Render("Used by") + "\n")
		if used := t.ix.UsedBy(p.FullName); len(used) == 0 {
			b.WriteString(st.Muted.Render("nothing installed depends on this") + "\n")
		} else {
			b.WriteString(strings.Join(used, ", ") + "\n")
		}
	}
	if p.Caveats != "" {
		b.WriteString("\n" + st.Accent.Render("Caveats") + "\n" + p.Caveats + "\n")
	}
	return b.String()
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ui/`
Expected: `ok  	brewery/internal/ui`

- [ ] **Step 5: Commit**

```bash
git add internal/ui
git commit -m "feat(ui): add Installed tab with filters and package preview"
```

---

### Task 8: Home tab (health tiles + doctor)

**Files:**
- Create: `internal/ui/home.go`
- Test: `internal/ui/home_test.go`

**Interfaces:**
- Consumes: `pane`, `paneInner` (Task 6); `state.Snapshot`.
- Consumes these tab-index constants, which are **defined in Task 9's `app.go`**: `tabInstalled`, `tabOutdated`, `tabServices`. To keep this task compiling on its own, Step 3 adds them to `home.go` temporarily, and Task 9 moves them.
- Produces: `type tile`, `type homeTab struct{ st theme.Styles; sel int }`, methods `tiles(state.Snapshot) []tile`, `update(tea.KeyPressMsg, state.Snapshot) (openTab int)` (returns -1 for "stay"), `view(snap, w, height int) string`; helpers `plural(n int, noun string) string` and `indent(s, prefix string) string`.

The tiles are Installed, Outdated, Services and Doctor. `h`/`l` select a tile, and `Enter` opens its tab. Doctor has no tab, so the full warnings are shown in a pane under the tiles. Tile details are cut with `…` so every tile stays 5 lines tall.


- [ ] **Step 1: Write the failing tests** in `internal/ui/home_test.go`

```go
package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
)

func TestHomeTiles(t *testing.T) {
	h := homeTab{st: testStyles}
	tiles := h.tiles(fixtureSnapshot(t))
	got := []string{}
	for _, tl := range tiles {
		got = append(got, tl.title+"="+tl.value+" ("+tl.detail+")")
	}
	want := []string{
		"Installed=7 (5 formulae · 2 casks)",
		"Outdated=2 (lua-language-server, docker-desktop)",
		"Services=1 error (postgresql@16)",
		"Doctor=2 warnings (see below)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("tiles:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestHomeTilesHealthy(t *testing.T) {
	h := homeTab{st: testStyles}
	snap := state.Snapshot{
		Services:  []brew.Service{{Name: "redis", Status: "started"}},
		DoctorRan: true,
	}
	tiles := h.tiles(snap)
	if tiles[1].value != "✓ 0" || tiles[2].value != "1 running" || tiles[3].value != "✓ clean" {
		t.Errorf("healthy tiles: %q %q %q", tiles[1].value, tiles[2].value, tiles[3].value)
	}
	if tiles[3].tab != -1 {
		t.Error("doctor tile has no tab to open")
	}
	if got := h.tiles(state.Snapshot{}); got[3].value != "…" {
		t.Errorf("doctor tile before doctor runs = %q", got[3].value)
	}
}

func TestHomeNavigation(t *testing.T) {
	h := homeTab{st: testStyles}
	snap := fixtureSnapshot(t)
	if tab := h.update(press("enter"), snap); tab != tabInstalled {
		t.Errorf("enter on first tile -> %d", tab)
	}
	h.update(press("l"), snap)
	h.update(press("l"), snap)
	if tab := h.update(press("enter"), snap); tab != tabServices {
		t.Errorf("enter on third tile -> %d", tab)
	}
	for range 5 {
		h.update(press("l"), snap)
	}
	if h.sel != 3 {
		t.Errorf("selection should stop at the last tile, got %d", h.sel)
	}
	if tab := h.update(press("enter"), snap); tab != -1 {
		t.Errorf("enter on doctor tile -> %d, want -1", tab)
	}
}

func TestHomeView(t *testing.T) {
	h := homeTab{st: testStyles}
	view := h.view(fixtureSnapshot(t), 140, 30)
	out := stripANSI(view)
	for _, want := range []string{"Installed", "5 formulae · 2 casks", "lua-language-server, docker-", "1 error",
		"⚠ The following taps are not trusted:", "brew untap oven-sh/bun", "⚠ Some installed casks are deprecated"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if lipgloss.Height(view) != 30 {
		t.Errorf("home view height = %d, want 30", lipgloss.Height(view))
	}
	clean := stripANSI(h.view(state.Snapshot{DoctorRan: true}, 140, 30))
	if !strings.Contains(clean, "Your system is ready to brew.") {
		t.Error("clean doctor message missing")
	}
}

func TestPlural(t *testing.T) {
	if plural(1, "warning") != "1 warning" || plural(3, "error") != "3 errors" {
		t.Error("plural wrong")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/`
Expected: FAIL to compile with `undefined: homeTab`, `undefined: plural`, `undefined: tabInstalled`.

- [ ] **Step 3: Create `internal/ui/home.go`**

```go
package ui

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

// tile is one health summary on the Home tab.
type tile struct {
	title  string
	value  string
	style  lipgloss.Style
	detail string
	tab    int // tab to open on Enter; -1 for none
}

// homeTab shows health tiles and doctor's warnings.
type homeTab struct {
	st  theme.Styles
	sel int
}

func (h *homeTab) tiles(snap state.Snapshot) []tile {
	st := h.st
	formulae, casks := 0, 0
	for _, p := range snap.Packages {
		if p.Kind == brew.KindCask {
			casks++
		} else {
			formulae++
		}
	}
	installed := tile{"Installed", fmt.Sprint(len(snap.Packages)), st.Title,
		fmt.Sprintf("%d formulae · %d casks", formulae, casks), tabInstalled}

	outdated := tile{"Outdated", "✓ 0", st.OK, "everything is up to date", tabOutdated}
	if n := len(snap.Outdated); n > 0 {
		names := make([]string, 0, n)
		for _, o := range snap.Outdated {
			names = append(names, o.Name)
		}
		outdated.value, outdated.style, outdated.detail = fmt.Sprint(n), st.Warn, strings.Join(names, ", ")
	}

	running, failed := 0, []string{}
	for _, s := range snap.Services {
		switch s.Status {
		case "started", "scheduled":
			running++
		case "error":
			failed = append(failed, s.Name)
		}
	}
	services := tile{"Services", fmt.Sprintf("%d running", running), st.OK,
		fmt.Sprintf("%d registered", len(snap.Services)), tabServices}
	if len(failed) > 0 {
		services.value, services.style, services.detail = plural(len(failed), "error"), st.Err, strings.Join(failed, ", ")
	}

	doctor := tile{"Doctor", "…", st.Muted, "checking", -1}
	if snap.DoctorRan {
		doctor.value, doctor.style, doctor.detail = "✓ clean", st.OK, "no warnings"
		if n := len(snap.Doctor); n > 0 {
			doctor.value, doctor.style, doctor.detail = plural(n, "warning"), st.Warn, "see below"
		}
	}
	return []tile{installed, outdated, services, doctor}
}

func (h *homeTab) update(msg tea.KeyPressMsg, snap state.Snapshot) (openTab int) {
	n := len(h.tiles(snap))
	switch msg.String() {
	case "l", "right":
		h.sel = min(h.sel+1, n-1)
	case "h", "left":
		h.sel = max(h.sel-1, 0)
	case "enter":
		return h.tiles(snap)[h.sel].tab
	}
	return -1
}

func (h *homeTab) view(snap state.Snapshot, w, height int) string {
	st := h.st
	tiles := h.tiles(snap)
	tileH := 5
	tileW := w / len(tiles)
	parts := make([]string, len(tiles))
	for i, t := range tiles {
		tw := tileW
		if i == len(tiles)-1 {
			tw = w - tileW*(len(tiles)-1)
		}
		iw, _ := paneInner(tw, tileH)
		body := t.style.Bold(true).Render(t.value) + "\n" + st.Muted.Render(ansi.Truncate(t.detail, iw, "…"))
		parts[i] = pane(st, t.title, body, tw, tileH, i == h.sel)
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	var b strings.Builder
	switch {
	case !snap.DoctorRan:
		b.WriteString(st.Muted.Render("brew doctor is running…"))
	case len(snap.Doctor) == 0:
		b.WriteString(st.OK.Render("✓ Your system is ready to brew."))
	default:
		for _, d := range snap.Doctor {
			b.WriteString(st.Warn.Render("⚠ "+d.Title) + "\n")
			if d.Body != "" {
				b.WriteString(st.Muted.Render(indent(d.Body, "  ")) + "\n")
			}
			b.WriteString("\n")
		}
	}
	doctor := pane(st, "Doctor", b.String(), w, max(height-tileH, 3), false)
	return lipgloss.JoinVertical(lipgloss.Left, row, doctor)
}

// plural formats a count with its noun: "1 warning", "2 warnings".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func indent(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}
```

Then append these temporary constants at the end of `home.go`, so the package compiles before `app.go` exists. Task 9 Step 3 deletes them:

```go
// Temporary: moved to app.go in Task 9.
const (
	tabHome = iota
	tabInstalled
	tabOutdated
	tabServices
	tabTaps
)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ui/`
Expected: `ok  	brewery/internal/ui`

- [ ] **Step 5: Commit**

```bash
git add internal/ui
git commit -m "feat(ui): add Home tab with health tiles and doctor warnings"
```

---

### Task 9: App root model (refresh flow, keys, tab bar, status bar)

**Files:**
- Create: `internal/ui/source.go`, `internal/ui/app.go`
- Modify: `internal/ui/home.go` (delete the temporary tab constants from Task 8)
- Test: `internal/ui/app_test.go`

**Interfaces:**
- Consumes: every earlier UI piece, `state.Save`, `state.NewIndex`, `theme.New(theme.Macchiato)`.
- Produces: `ui.Source` interface (`Installed`, `Outdated`, `Services`, `Taps`, `Doctor`, each `(ctx) ([]T, error)`; `*brew.Client` satisfies it), `ui.New(src Source, snap state.Snapshot, cachePath string) *App`, and `*App` implementing `tea.Model` (`Init`, `Update`, `View() tea.View`). Internal: `query` constants `qInstalled…qDoctor`, `resultMsg`, `savedMsg`, `fetch`, `save`, tab constants `tabHome…tabTaps`, `tabNames`, and the fields `snap`, `pending`, `tab`, `cachePath`, `now func() time.Time`.

Refresh flow (spec section 2):
1. `Init` starts the 4 JSON queries in parallel (`pending = 4`).
2. Each result updates the snapshot, or records an error and keeps the old data.
3. When pending reaches 0, doctor starts.
4. When doctor returns, `FetchedAt` is set and the snapshot is saved to the cache.
5. `r` during a refresh does nothing.

`View` turns on the alternate screen and sets the window title.


- [ ] **Step 1: Write the failing tests** in `internal/ui/app_test.go`

```go
package ui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
)

// fakeSource serves the fixture snapshot. servicesErr makes one query fail.
type fakeSource struct {
	snap        state.Snapshot
	servicesErr error
}

func newFakeSource(t *testing.T) *fakeSource { return &fakeSource{snap: fixtureSnapshot(t)} }

func (f *fakeSource) Installed(context.Context) ([]brew.Package, error) { return f.snap.Packages, nil }
func (f *fakeSource) Outdated(context.Context) ([]brew.Outdated, error) { return f.snap.Outdated, nil }
func (f *fakeSource) Taps(context.Context) ([]brew.Tap, error)          { return f.snap.Taps, nil }
func (f *fakeSource) Doctor(context.Context) ([]brew.DoctorWarning, error) {
	return f.snap.Doctor, nil
}
func (f *fakeSource) Services(context.Context) ([]brew.Service, error) {
	if f.servicesErr != nil {
		return nil, f.servicesErr
	}
	return f.snap.Services, nil
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

func newTestApp(t *testing.T, src Source) *App {
	t.Helper()
	a := New(src, state.Snapshot{}, filepath.Join(t.TempDir(), "snapshot.json"))
	a.now = func() time.Time { return testNow }
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return a
}

func TestRefreshLoadsEverythingThenDoctorThenSaves(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())

	if a.pending != 0 {
		t.Fatalf("pending = %d after refresh", a.pending)
	}
	if len(a.snap.Packages) != 7 || len(a.snap.Outdated) != 2 || len(a.snap.Services) != 2 || len(a.snap.Taps) != 2 {
		t.Errorf("snapshot incomplete: %d pkgs, %d outdated, %d services, %d taps",
			len(a.snap.Packages), len(a.snap.Outdated), len(a.snap.Services), len(a.snap.Taps))
	}
	if !a.snap.DoctorRan || len(a.snap.Doctor) != 2 {
		t.Errorf("doctor not applied: ran=%v warnings=%d", a.snap.DoctorRan, len(a.snap.Doctor))
	}
	saved, err := state.Load(a.cachePath)
	if err != nil {
		t.Fatalf("cache not written: %v", err)
	}
	if len(saved.Packages) != 7 || !saved.FetchedAt.Equal(testNow) {
		t.Errorf("cache has %d packages, fetched at %v", len(saved.Packages), saved.FetchedAt)
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

func TestCachedSnapshotShowsBeforeRefresh(t *testing.T) {
	cached := state.Snapshot{Packages: []brew.Package{{Name: "neovim", FullName: "neovim", Kind: brew.KindFormula, Version: "0.11.4", OnRequest: true}}}
	a := New(newFakeSource(t), cached, filepath.Join(t.TempDir(), "s.json"))
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	a.Update(press("2"))
	if !strings.Contains(stripANSI(a.render()), "neovim") {
		t.Error("cached package should render before any query returns")
	}
}

func TestTabKeys(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	steps := []struct {
		key  string
		want int
	}{
		{"3", tabOutdated}, {"]", tabServices}, {"]", tabTaps}, {"]", tabHome}, {"[", tabTaps}, {"1", tabHome},
	}
	for _, s := range steps {
		a.Update(press(s.key))
		if a.tab != s.want {
			t.Fatalf("after %q tab = %d, want %d", s.key, a.tab, s.want)
		}
	}
}

func TestQuit(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	for _, k := range []string{"q", "ctrl+c"} {
		_, cmd := a.Update(press(k))
		if cmd == nil {
			t.Fatalf("%s: no command", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%s did not quit", k)
		}
	}
}

func TestKeysReachActiveTab(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	a.Update(press("l")) // home: select Outdated tile
	a.Update(press("enter"))
	if a.tab != tabOutdated {
		t.Fatalf("enter on Outdated tile went to tab %d", a.tab)
	}
	a.Update(press("2"))
	a.Update(press("j"))
	if out := stripANSI(a.render()); !strings.Contains(out, "Installed › git") {
		t.Errorf("status bar should name the selected package:\n%s", out)
	}
}

func TestFailedQueryShowsErrorOnlyInItsTab(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("brew services list --json: exit status 1")
	a := newTestApp(t, src)
	drain(t, a, a.Init())

	a.Update(press("4"))
	out := stripANSI(a.render())
	if !strings.Contains(out, "couldn't load this view") || !strings.Contains(out, "exit status 1") {
		t.Errorf("services tab should show the error:\n%s", out)
	}
	if !strings.Contains(out, "1 queries failed") {
		t.Error("status bar should count the failure")
	}
	a.Update(press("5"))
	if out := stripANSI(a.render()); !strings.Contains(out, "anomalyco/tap") {
		t.Error("taps tab should still work")
	}
}

func TestFailedQueryKeepsLastGoodData(t *testing.T) {
	src := newFakeSource(t)
	a := newTestApp(t, src)
	drain(t, a, a.Init())
	src.servicesErr = errors.New("boom")
	drain(t, a, a.refresh())
	if len(a.snap.Services) != 2 {
		t.Errorf("services from the earlier refresh were dropped: %v", a.snap.Services)
	}
}

func TestStatusBar(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	a.Init()
	if out := stripANSI(a.statusBar()); !strings.Contains(out, "NORMAL") || !strings.Contains(out, "refreshing…") {
		t.Errorf("status bar during refresh:\n%s", out)
	}
	a = newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	if out := stripANSI(a.statusBar()); !strings.Contains(out, "updated 09:30") || !strings.Contains(out, "q quit") {
		t.Errorf("status bar after refresh:\n%s", out)
	}
}

func TestTabBarBadges(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	out := stripANSI(a.tabBar())
	if !strings.Contains(out, "3 Outdated 2") || !strings.Contains(out, "4 Services !") {
		t.Errorf("tab bar badges missing: %q", out)
	}
}

func TestViewFitsTerminal(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	for tab := range tabNames {
		a.tab = tab
		out := a.render()
		if h := lipgloss.Height(out); h != 40 {
			t.Errorf("tab %s renders %d lines, want 40", tabNames[tab], h)
		}
		if w := lipgloss.Width(out); w > 140 {
			t.Errorf("tab %s renders %d columns, want <= 140", tabNames[tab], w)
		}
	}
	if v := a.View(); !v.AltScreen {
		t.Error("brewery should use the alternate screen")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/`
Expected: FAIL to compile with `undefined: New`, `undefined: Source`, `undefined: App`.

- [ ] **Step 3: Delete the temporary tab constants** (the `// Temporary: moved to app.go in Task 9.` block) from the end of `internal/ui/home.go`.

- [ ] **Step 4: Create `internal/ui/source.go`**

```go
// Package ui is brewery's Bubble Tea front end. It never runs brew itself:
// it asks a Source for data and renders what comes back.
package ui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
)

// Source provides brew data. *brew.Client satisfies it; tests use a fake.
type Source interface {
	Installed(ctx context.Context) ([]brew.Package, error)
	Outdated(ctx context.Context) ([]brew.Outdated, error)
	Services(ctx context.Context) ([]brew.Service, error)
	Taps(ctx context.Context) ([]brew.Tap, error)
	Doctor(ctx context.Context) ([]brew.DoctorWarning, error)
}

type query int

const (
	qInstalled query = iota
	qOutdated
	qServices
	qTaps
	qDoctor
)

// resultMsg carries one query's result back to the App.
type resultMsg struct {
	q        query
	packages []brew.Package
	outdated []brew.Outdated
	services []brew.Service
	taps     []brew.Tap
	doctor   []brew.DoctorWarning
	err      error
}

func fetch(src Source, q query) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		m := resultMsg{q: q}
		switch q {
		case qInstalled:
			m.packages, m.err = src.Installed(ctx)
		case qOutdated:
			m.outdated, m.err = src.Outdated(ctx)
		case qServices:
			m.services, m.err = src.Services(ctx)
		case qTaps:
			m.taps, m.err = src.Taps(ctx)
		case qDoctor:
			m.doctor, m.err = src.Doctor(ctx)
		}
		return m
	}
}

// savedMsg reports the result of writing the snapshot cache.
type savedMsg struct{ err error }

func save(path string, snap state.Snapshot) tea.Cmd {
	return func() tea.Msg {
		return savedMsg{err: state.Save(path, snap)}
	}
}
```

- [ ] **Step 5: Create `internal/ui/app.go`**

```go
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/state"
	"brewery/internal/theme"
)

const (
	tabHome = iota
	tabInstalled
	tabOutdated
	tabServices
	tabTaps
)

var tabNames = [...]string{"Home", "Installed", "Outdated", "Services", "Taps"}

// App is the root Bubble Tea model.
type App struct {
	src       Source
	cachePath string
	st        theme.Styles
	now       func() time.Time

	snap state.Snapshot
	ix   *state.Index
	errs map[query]error

	pending      int  // queries in flight
	doctorQueued bool // doctor already started in this refresh
	saveErr      error

	tab           int
	width, height int

	home      homeTab
	installed installedTab
	outdated  listTab
	services  listTab
	taps      listTab
}

// New builds the App. snap is the cached snapshot (possibly empty) so the
// first frame can show data before any query returns.
func New(src Source, snap state.Snapshot, cachePath string) *App {
	st := theme.New(theme.Macchiato)
	a := &App{
		src: src, cachePath: cachePath, st: st, now: time.Now,
		snap: snap, errs: map[query]error{}, tab: tabHome,
		home:      homeTab{st: st},
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, "no services registered"),
		taps:      newListTab(st, "Taps", tapCols, "no taps installed"),
	}
	a.sync()
	return a
}

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

// sync pushes the snapshot into every tab.
func (a *App) sync() {
	a.ix = state.NewIndex(a.snap.Packages)
	a.installed.setData(a.snap.Packages, a.ix)
	a.outdated.setRows(outdatedRows(a.snap.Outdated))
	a.services.setRows(serviceRows(a.snap.Services))
	a.taps.setRows(tapRows(a.snap.Taps))
}

func (a *App) apply(m resultMsg) {
	a.errs[m.q] = m.err
	if m.err != nil {
		return // keep showing the last good data
	}
	switch m.q {
	case qInstalled:
		a.snap.Packages = m.packages
	case qOutdated:
		a.snap.Outdated = m.outdated
	case qServices:
		a.snap.Services = m.services
	case qTaps:
		a.snap.Taps = m.taps
	case qDoctor:
		a.snap.Doctor, a.snap.DoctorRan = m.doctor, true
	}
	a.sync()
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
		return a, nil

	case tea.KeyPressMsg:
		return a, a.handleKey(msg)
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch k := msg.String(); k {
	case "q", "ctrl+c":
		return tea.Quit
	case "1", "2", "3", "4", "5":
		a.tab = int(k[0] - '1')
		return nil
	case "]":
		a.tab = (a.tab + 1) % len(tabNames)
		return nil
	case "[":
		a.tab = (a.tab + len(tabNames) - 1) % len(tabNames)
		return nil
	case "r":
		return a.refresh()
	}

	switch a.tab {
	case tabHome:
		if t := a.home.update(msg, a.snap); t >= 0 {
			a.tab = t
		}
	case tabInstalled:
		return a.installed.update(msg)
	case tabOutdated:
		return a.outdated.update(msg)
	case tabServices:
		return a.services.update(msg)
	case tabTaps:
		return a.taps.update(msg)
	}
	return nil
}

func (a *App) View() tea.View {
	v := tea.NewView(a.render())
	v.AltScreen = true
	v.WindowTitle = "brewery"
	return v
}

func (a *App) render() string {
	if a.width == 0 {
		return "starting brewery…"
	}
	bodyH := a.bodyHeight()
	var body string
	switch a.tab {
	case tabHome:
		body = a.home.view(a.snap, a.width, bodyH)
	case tabInstalled:
		body = a.installed.view(a.errs[qInstalled])
	case tabOutdated:
		body = a.outdated.view(a.errs[qOutdated])
	case tabServices:
		body = a.services.view(a.errs[qServices])
	case tabTaps:
		body = a.taps.view(a.errs[qTaps])
	}
	return lipgloss.JoinVertical(lipgloss.Left, a.tabBar(), body, a.statusBar())
}

// bodyHeight is the space between the tab bar and the status bar.
func (a *App) bodyHeight() int { return max(a.height-2, 3) }

func (a *App) tabBar() string {
	st := a.st
	type badge struct {
		text  string
		style lipgloss.Style
	}
	badges := map[int]badge{}
	if n := len(a.snap.Outdated); n > 0 {
		badges[tabOutdated] = badge{fmt.Sprint(n), st.Warn}
	}
	for _, s := range a.snap.Services {
		if s.Status == "error" {
			badges[tabServices] = badge{"!", st.Err}
			break
		}
	}
	parts := make([]string, len(tabNames))
	for i, name := range tabNames {
		label := fmt.Sprintf("%d %s", i+1, name)
		b, hasBadge := badges[i]
		switch {
		case i == a.tab && hasBadge:
			parts[i] = st.TabActive.Render(label + " " + b.text) // badge color would clash with the active background
		case i == a.tab:
			parts[i] = st.TabActive.Render(label)
		case hasBadge:
			parts[i] = st.Tab.Render(label) + b.style.Render(b.text)
		default:
			parts[i] = st.Tab.Render(label)
		}
	}
	return strings.Join(parts, " ")
}

func (a *App) statusBar() string {
	st := a.st
	left := st.Mode("NORMAL").Render("NORMAL") + " " + tabNames[a.tab]
	if a.tab == tabInstalled {
		if p, ok := a.installed.selected(); ok {
			left += " › " + p.Name
		}
	}

	var right []string
	failed := 0
	for _, err := range a.errs {
		if err != nil {
			failed++
		}
	}
	switch {
	case a.pending > 0:
		right = append(right, st.Info.Render("refreshing…"))
	case failed > 0:
		right = append(right, st.Err.Render(fmt.Sprintf("✗ %d queries failed", failed)))
	case !a.snap.FetchedAt.IsZero():
		right = append(right, "updated "+a.snap.FetchedAt.Format("15:04"))
	}
	if a.saveErr != nil {
		right = append(right, st.Warn.Render("cache not saved"))
	}
	right = append(right, st.Muted.Render("1-5 tabs · r refresh · q quit"))
	r := strings.Join(right, "  ")

	gap := max(a.width-lipgloss.Width(left)-lipgloss.Width(r)-2, 1)
	return st.StatusBar.Width(a.width).Render(left + strings.Repeat(" ", gap) + r + " ")
}
```

- [ ] **Step 6: Run the whole suite with the race detector**

Run: `go test -race ./...`
Expected: every package `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/ui
git commit -m "feat(ui): add App root model with background refresh and caching"
```

---

### Task 10: `brewery` command

**Files:**
- Create: `cmd/brewery/main.go`
- Test: `cmd/brewery/main_test.go`

**Interfaces:**
- Consumes: `brew.Locate`, `brew.ErrNotFound`, `brew.NewClient`, `brew.ExecRunner`, `brew.QueryEnv`, `state.DefaultCachePath`, `state.Load`, `ui.New`.
- Produces: the `brewery` binary. `run(args []string, stdout, stderr io.Writer) int` returns exit codes 0 (ok), 1 (brew missing or a runtime error) and 2 (bad flags). `var version` is set by `-ldflags "-X main.version=..."` (Plan 3). `var locateBrew` exists so tests can replace it.


- [ ] **Step 1: Write the failing tests** in `cmd/brewery/main_test.go`

```go
package main

import (
	"bytes"
	"strings"
	"testing"

	"brewery/internal/brew"
)

func TestVersionFlag(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--version"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if got := out.String(); got != "brewery dev\n" {
		t.Errorf("got %q", got)
	}
}

func TestBrewMissing(t *testing.T) {
	locateBrew = func() (string, error) { return "", brew.ErrNotFound }
	t.Cleanup(func() { locateBrew = brew.Locate })
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "https://brew.sh") {
		t.Errorf("stderr should point to brew.sh: %q", errb.String())
	}
}

func TestUnknownFlag(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--nope"}, &out, &errb); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./cmd/brewery/`
Expected: FAIL to compile with `undefined: run`, `undefined: locateBrew`.

- [ ] **Step 3: Create `cmd/brewery/main.go`**

```go
// Command brewery is a terminal dashboard for Homebrew.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
	"brewery/internal/ui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// locateBrew is swapped out in tests.
var locateBrew = brew.Locate

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("brewery", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, "brewery", version)
		return 0
	}

	bin, err := locateBrew()
	if errors.Is(err, brew.ErrNotFound) {
		fmt.Fprintln(stderr, "brewery: Homebrew isn't installed (looked in $HOMEBREW_PREFIX, /opt/homebrew, /usr/local and $PATH).")
		fmt.Fprintln(stderr, "Install it from https://brew.sh, then run brewery again.")
		return 1
	}

	cachePath := state.DefaultCachePath()
	snap, _ := state.Load(cachePath) // no cache yet is fine: the first refresh fills it
	client := brew.NewClient(brew.ExecRunner{Bin: bin, Env: brew.QueryEnv})

	if _, err := tea.NewProgram(ui.New(client, snap, cachePath)).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
		return 1
	}
	return 0
}
```

- [ ] **Step 4: Run the full suite**

Run: `make test && go vet ./... && gofmt -l .`
Expected: every package `ok`, no vet output, and `gofmt -l` prints nothing.

- [ ] **Step 5: Smoke test in a real terminal**

Run: `make run`

Check each of these by hand:
- The first launch shows the Home tab with `…` in the Doctor tile, and the status bar says `refreshing…` before switching to `updated HH:MM`.
- `2` shows Installed with about 23 leaves. `j`/`k` move the selection and the preview follows. `f` cycles leaves → all → casks → deps.
- `3` lists outdated packages, `4` shows services (postgresql@16 as `error (exit 1)`), and `5` shows taps.
- `q` quits and the terminal comes back clean.
- A second `make run` shows data immediately (from `~/.cache/brewery/snapshot.json`) and then refreshes.

- [ ] **Step 6: Commit**

```bash
git add cmd
git commit -m "feat: add brewery command"
```

---

## Later plans (outline; each gets its own detailed plan after this one ships)

**Plan 2, "Maintain"** (spec section 3, plus the action keys from section 4):
- `internal/jobs`: a one-at-a-time job queue with `queued` state, history (last 100 jobs) under `~/.local/state/brewery/jobs/`, and lock-held detection with retry and backoff.
- `internal/brew/exec.go`: running jobs in a PTY (`creack/pty`), sending SIGINT to the process group on `^c`, and setting `HOMEBREW_NO_AUTO_UPDATE=1` for actions launched from a pane.
- First task: a time-boxed spike on Charm `x/vt` for the drawer's terminal emulator, with a fallback to a minimal renderer (`\r`, SGR colors, erase-line).
- UI: output drawer, RUNNING and INSERT modes (masked, unlogged input), the `:` command line with completion (`brew commands --quiet`, flags cached per brew version, package names), `ExecProcess` takeover for `edit`/`sh`/`irb`/`bundle edit`/`:!`, and `:jobs`.
- Actions: `u` upgrade, `x` uninstall (confirmation lists dependents and orphans; this adds `Index.Orphans`), `p` pin, `A` upgrade all, `s`/`S`/`R` for services, `l` service log, `U` update, cleanup with a `--dry-run` preview, untap. Each finished job triggers a targeted refresh.
- Home: a recent-jobs pane.

**Plan 3, "Polish and ship"** (the rest of sections 2, 4 and 5):
- `/` filter mode, the `Ctrl-P` palette, `?` help, `o` open homepage, `y` copy name, `space` multi-select, `gg`, and Tab focus with a scrollable preview.
- Disk sizes for visible rows (computed lazily and cached), the Home cache tile (`brew --cache` size), and the "last updated Nh ago" age in the Outdated tab.
- Responsive layout (preview stacks below the list under 100 columns and hides under 60), Nerd Font icons with an ASCII fallback (`--no-icons`), and colored service status.
- A crash log at `~/.local/state/brewery/crash.log`, `teatest` snapshot tests, golangci-lint, and GitHub Actions on macOS.
- goreleaser, the tap formula, and renaming the module path (needs the GitHub username).
