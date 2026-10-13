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
