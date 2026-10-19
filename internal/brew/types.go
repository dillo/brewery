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
	License     string    `json:"license"` // a formula's SPDX licence
	Version     string    `json:"version"` // installed version
	Latest      string    `json:"latest"`  // newest version known to brew
	OnRequest   bool      `json:"on_request"`
	Pinned      bool      `json:"pinned"`
	Linked      bool      `json:"linked"`   // a formula's keg is linked into brew's prefix
	KegOnly     bool      `json:"keg_only"` // brew doesn't link the formula unless forced
	Outdated    bool      `json:"outdated"`
	Deprecated  bool      `json:"deprecated"`
	Caveats     string    `json:"caveats"`
	InstalledAt time.Time `json:"installed_at"`
	Deps        []string  `json:"deps"`        // all runtime deps (transitive), full names
	DirectDeps  []string  `json:"direct_deps"` // runtime deps declared directly
	Targets     []string  `json:"targets"`     // a cask's installed files: apps, fonts…
}

// Key identifies a package: a formula and a cask can share a name.
func (p Package) Key() string { return p.Kind.String() + "/" + p.FullName }

// Entry is one package in brew's catalogue: anything it can install.
type Entry struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
	Kind Kind   `json:"kind"`
}

// Outdated is one row of `brew outdated --json=v2`.
type Outdated struct {
	Name      string `json:"name"`
	Kind      Kind   `json:"kind"`
	Installed string `json:"installed"`
	Current   string `json:"current"`
	Pinned    bool   `json:"pinned"`
}

// Service is one entry of `brew services info --all --json`.
type Service struct {
	Name     string `json:"name"`
	Status   string `json:"status"`       // started, stopped, none, error, scheduled, unknown
	Label    string `json:"service_name"` // launchd's label, homebrew.mxcl.<name>
	Loaded   bool   `json:"loaded"`       // in launchctl list, which brew counts as running
	User     string `json:"user"`
	File     string `json:"file"`
	ExitCode *int   `json:"exit_code"`
	PID      int    `json:"pid"`            // 0 when it isn't running
	Command  string `json:"command"`        // what launchd runs
	Log      string `json:"log_path"`       // where its output goes, if anywhere
	ErrorLog string `json:"error_log_path"` // where its errors go; often the same file
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
