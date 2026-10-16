package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
)

// pinBehind pins pcre2 at 10.49 while 10.50 is out, and makes git, which
// needs pcre2, outdated and not pinned: brew would refuse to upgrade git.
func pinBehind(snap *state.Snapshot) {
	for i := range snap.Packages {
		p := &snap.Packages[i]
		switch p.Name {
		case "pcre2":
			p.Pinned, p.Outdated, p.Latest = true, true, "10.50"
		case "git":
			p.Pinned, p.Outdated, p.Latest = false, true, "2.52.0"
		}
	}
	snap.Outdated = append(snap.Outdated,
		brew.Outdated{Name: "git", Kind: brew.KindFormula, Installed: "2.51.0", Current: "2.52.0"},
		brew.Outdated{Name: "pcre2", Kind: brew.KindFormula, Installed: "10.49", Current: "10.50", Pinned: true})
}

// pinnedApp is a job app whose packages, and brew's later answers, are
// as pinBehind leaves them.
func pinnedApp(t *testing.T) (*App, *fakeStarter) {
	t.Helper()
	fs, src := &fakeStarter{}, newFakeSource(t)
	pinBehind(&src.snap)
	snap := fixtureSnapshot(t)
	pinBehind(&snap)
	a := New(src, fs.start, snap, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return a, fs
}

func TestPreviewMarksPinnedDependencies(t *testing.T) {
	a, _ := pinnedApp(t)
	selectInstalled(t, a, "git")
	out := stripANSI(a.render())
	for _, want := range []string{"└─ pcre2 pinned · 10.49 → 10.50", "⚠ upgrading needs pcre2 unpinned"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	a, _ = pinnedApp(t)
	selectInstalled(t, a, "fd")
	if out := stripANSI(a.render()); strings.Contains(out, "upgrading needs") {
		t.Errorf("fd needs nothing pinned:\n%s", out)
	}
}
