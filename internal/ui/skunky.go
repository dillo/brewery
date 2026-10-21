package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

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
	if a.errs[qVulns] != nil {
		return -1, a.st.ChipStale.Render("⚠") // the last scan failed: what's shown may be old
	}
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
