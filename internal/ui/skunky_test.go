package ui

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/state"
)

// appRowNames are the names Installed lists now, in order.
func appRowNames(a *App) []string {
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
	a.Update(tea.WindowSizeMsg{Width: 180, Height: 40}) // wide enough for the about line and SEVERITY
	show(t, a, "Skunky")
	if got, want := appRowNames(a), []string{"lua-language-server", "json-c"}; !slices.Equal(got, want) {
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
	if got := appRowNames(a); !slices.Equal(got, []string{"json-c"}) {
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
		if p.Name == "lua-language-server" {
			a.installed.table.SetCursor(i)
		}
	}
	show(t, a, "Skunky")
	if p, _ := a.installed.selected(); p.Name != "lua-language-server" {
		t.Errorf("selected %s after moving to Skunky, want lua-language-server", p.Name)
	}
}

func (c *countingSource) Vulns(ctx context.Context) (brew.VulnScan, error) {
	c.calls["vulns"]++
	return c.fakeSource.Vulns(ctx)
}

// newScanApp is an app on the fixtures whose last scan was since before now,
// or that has never scanned when scanned is false. It counts brew calls.
func newScanApp(t *testing.T, since time.Duration, scanned bool) (*App, *countingSource) {
	t.Helper()
	src := newCountingSource(t)
	snap := fixtureSnapshot(t)
	snap.VulnsAt = testNow.Add(-since)
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
		{"24 hours ago", 24 * time.Hour, true, 1},
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
	if a.errs[qVulns] != nil {
		t.Errorf("a brew without vulns isn't an error: %v", a.errs[qVulns])
	}
	if bar := stripANSI(a.statusBar()); strings.Contains(bar, "vulns failed") {
		t.Errorf("status bar %q mentions a failed scan", bar)
	}
	if n, chip := a.badgeCount(entries[entryIndex("Skunky")]); n != -1 || chip != "" {
		t.Errorf("badge %d %q, want none", n, chip)
	}
}

func TestFullRefreshJobTriesVulnsAgain(t *testing.T) {
	src := newCountingSource(t)
	fs := &fakeStarter{}
	a := New(src, fs.start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	a.snap.Vulns, a.snap.VulnsAt = nil, time.Time{}
	src.vulnsErr = brew.ErrNoVulns
	drain(t, a, a.refresh())
	if src.calls["vulns"] != 1 || !a.noVulns {
		t.Fatalf("%d scans, noVulns %v, want 1 and true", src.calls["vulns"], a.noVulns)
	}
	src.vulnsErr = nil
	keys(t, a, "U")
	if src.calls["vulns"] != 2 || a.snap.Vulns == nil || a.noVulns {
		t.Errorf("after brew update: %d scans, Vulns %v, noVulns %v; want 2, set, false",
			src.calls["vulns"], a.snap.Vulns, a.noVulns)
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
			if slices.Contains(g.keys, [2]string{"r", "refresh, and scan with brew vulns now"}) {
				return true
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

func TestNoSecondScanWhileOneRuns(t *testing.T) {
	a, _ := newScanApp(t, 0, false)
	for _, force := range []bool{false, true} {
		a.inflight[qVulns], a.forceVulns = true, force
		if slices.Contains(a.slowQueries(), qVulns) {
			t.Errorf("forceVulns=%v: a slow pass while a scan runs must not ask for another", force)
		}
		if a.forceVulns {
			t.Errorf("forceVulns=%v: the slow pass should spend the request", force)
		}
	}
	a.inflight[qVulns] = false
	if !slices.Contains(a.slowQueries(), qVulns) {
		t.Error("with no scan running and none yet, the slow pass should scan")
	}

	// End to end: a slow pass that starts mid-scan leads to one scan, not two.
	a, src := newScanApp(t, 0, false)
	scan := a.request(qVulns)
	slow := a.request(a.slowQueries()...)
	drain(t, a, scan)
	drain(t, a, slow)
	if src.calls["vulns"] != 1 {
		t.Errorf("%d scans, want 1", src.calls["vulns"])
	}
}

func TestNoVulnsClearsAnEarlierScanError(t *testing.T) {
	a, _ := newScanApp(t, 0, false)
	a.errs[qVulns] = errors.New("no network")
	a.apply(resultMsg{q: qVulns, err: brew.ErrNoVulns})
	if a.errs[qVulns] != nil {
		t.Errorf("a brew without vulns isn't an error, whatever failed before: %v", a.errs[qVulns])
	}
}
