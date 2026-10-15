package brew

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
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
		License  *string `json:"license"`
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
		DependsOn     struct {
			Formula []string `json:"formula"`
			Cask    []string `json:"cask"`
		} `json:"depends_on"`
		Artifacts []json.RawMessage `json:"artifacts"` // read one by one: their shapes vary
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
			Desc: deref(f.Desc), Homepage: f.Homepage, License: deref(f.License), Latest: f.Versions.Stable,
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
		p.DirectDeps = append(slices.Clone(c.DependsOn.Formula), c.DependsOn.Cask...)
		for _, raw := range c.Artifacts {
			var a struct {
				Target string `json:"target"`
			}
			if json.Unmarshal(raw, &a) == nil && a.Target != "" {
				p.Targets = append(p.Targets, a.Target) // where brew put an app, a font, a binary…
			}
		}
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

// ParseServices parses `brew services info --all --json`, which says
// everything `services list` does plus each service's command and logs.
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
