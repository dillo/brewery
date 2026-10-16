package ui

import (
	"path/filepath"
	"slices"
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

// selectOutdated moves Outdated's cursor to name.
func selectOutdated(t *testing.T, a *App, name string) {
	t.Helper()
	keys(t, a, "3", "g", "g")
	for range len(a.snap.Outdated) {
		if i := a.outdated.cursor(); i >= 0 && a.snap.Outdated[i].Name == name {
			return
		}
		keys(t, a, "j")
	}
	t.Fatalf("%s not found on Outdated", name)
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

func TestUpgradeRefusesWhileADependencyIsPinnedBehind(t *testing.T) {
	a, fs := pinnedApp(t)
	selectInstalled(t, a, "git")
	keys(t, a, "u")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "git needs pcre2 unpinned first") {
		t.Errorf("calls %v, status %q", fs.calls, stripANSI(a.statusBar()))
	}
	selectOutdated(t, a, "git")
	keys(t, a, "u")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "git needs pcre2 unpinned first") {
		t.Errorf("Outdated: calls %v, status %q", fs.calls, stripANSI(a.statusBar()))
	}
}

func TestMarkedUpgradeSkipsBlockedFormulae(t *testing.T) {
	a, fs := pinnedApp(t)
	selectOutdated(t, a, "git")
	keys(t, a, "space")
	keys(t, a, "u")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "skipped git: needs pcre2 unpinned") {
		t.Errorf("only git marked: calls %v, status %q", fs.calls, stripANSI(a.statusBar()))
	}
	p := newFakeProc(0) // the upgrade keeps running while the note shows
	fs.procs = []*fakeProc{p}
	if a.marks() != 1 {
		t.Fatalf("git should still be marked: %d marks", a.marks())
	}
	selectOutdated(t, a, "lua-language-server")
	keys(t, a, "space")
	waiting := settle(t, a, a.handleKey(press("u")))
	if want := []string{"upgrade", "lua-language-server"}; !slices.Equal(lastCall(fs), want) || len(fs.calls) != 1 {
		t.Errorf("ran %v, want %v", fs.calls, want)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "skipped git: needs pcre2 unpinned") {
		t.Errorf("status %q", stripANSI(a.statusBar()))
	}
	p.exit()
	resume(t, a, waiting)
}

func TestUpgradeAllNamesWhatBrewWillRefuse(t *testing.T) {
	a, _ := pinnedApp(t)
	keys(t, a, "3", "A")
	if got := confirmText(a); !strings.Contains(got, "brew will refuse git: it needs pcre2 unpinned") {
		t.Errorf("dialog:\n%s", got)
	}
}

func TestInstallRefusesWhileADependencyIsPinnedBehind(t *testing.T) {
	a, fs, _, cat := newInstallApp(t)
	pinBehind(&a.snap)
	a.sync()
	rg := ripgrep
	rg.Needs = []string{"pcre2", "oniguruma"}
	cat.details["ripgrep"] = rg
	selectDiscover(t, a, "ripgrep")
	keys(t, a, "a")
	want := "Can't install ripgrep\nIt needs pcre2, which is pinned and outdated.\nUnpin it on Installed first, or brew will refuse."
	if got := confirmText(a); got != want {
		t.Errorf("asked\n%s\nwant\n%s", got, want)
	}
	keys(t, a, "y")
	if len(fs.calls) != 0 {
		t.Errorf("y ran %v", fs.calls)
	}
}

func TestInstallDialogLearnsOfAPinnedDependency(t *testing.T) {
	a, fs, _, cat := newInstallApp(t)
	pinBehind(&a.snap)
	a.sync()
	rg := ripgrep
	rg.Needs = []string{"pcre2"}
	cat.details["ripgrep"] = rg
	selectDiscover(t, a, "python@3.15")
	a.Update(press("j")) // to ripgrep; brew hasn't been asked yet
	a.Update(press("a"))
	if got := confirmText(a); !strings.Contains(got, "checking dependencies…") {
		t.Fatalf("asked\n%s", got)
	}
	drain(t, a, a.detailsDue(detailsDueMsg{key: entryKey("ripgrep", brew.KindFormula)}))
	if got := confirmText(a); !strings.HasPrefix(got, "Can't install ripgrep\n") {
		t.Errorf("the open dialog didn't learn of pcre2:\n%s", got)
	}
	keys(t, a, "y")
	if len(fs.calls) != 0 {
		t.Errorf("y ran %v", fs.calls)
	}
}
