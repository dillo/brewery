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
