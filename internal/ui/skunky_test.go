package ui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
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
		if p.Name == "json-c" {
			a.installed.table.SetCursor(i)
		}
	}
	show(t, a, "Skunky")
	if p, _ := a.installed.selected(); p.Name != "json-c" {
		t.Errorf("selected %s after moving to Skunky, want json-c", p.Name)
	}
}
