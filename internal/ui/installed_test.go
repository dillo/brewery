package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/state"
)

func newFixtureInstalled(t *testing.T) installedTab {
	snap := fixtureSnapshot(t)
	it := newInstalledTab(testStyles)
	it.resize(77, 63, 30)
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

func TestInstalledViews(t *testing.T) {
	it := newFixtureInstalled(t)
	for _, w := range []struct {
		f     installedFilter
		names string
	}{
		{filterTop, "docker-desktop fd fontbase git lua-language-server"},
		{filterAll, "docker-desktop fd fontbase git json-c lua-language-server pcre2"},
		{filterFormulae, "fd git json-c lua-language-server pcre2"},
		{filterCasks, "docker-desktop fontbase"},
		{filterDeps, "json-c pcre2"},
		{filterPinned, "git"},
	} {
		it.filter = w.f
		it.apply()
		if got := rowNames(it); got != w.names {
			t.Errorf("view %s: got %q, want %q", views[w.f].name, got, w.names)
		}
		if got, want := it.count(w.f), len(strings.Fields(w.names)); got != want {
			t.Errorf("view %s: count %d, want %d", views[w.f].name, got, want)
		}
	}
}

func TestInstalledTitleNamesTheViewAndSaysWhatItShows(t *testing.T) {
	it := newFixtureInstalled(t)
	it.filter = filterCasks
	it.apply()
	it.resize(66, 54, 30)
	out := stripANSI(it.view(nil, colList))
	for _, want := range []string{"Casks", "macOS apps and fonts (casks)."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "f/F") {
		t.Errorf("no view keys any more:\n%s", out)
	}
}

func TestInstalledNavigationAndPreview(t *testing.T) {
	it := newFixtureInstalled(t)
	it.update(press("j"))
	it.update(press("j"))
	it.update(press("j")) // docker-desktop fd fontbase [git]
	p, ok := it.selected()
	if !ok || p.Name != "git" {
		t.Fatalf("selected %q, want git", p.Name)
	}
	out := stripANSI(it.view(nil, colList))
	for _, want := range []string{
		"Top-level", "git 2.51.0", "Distributed revision control system",
		"on request · ", "pinned", "Dependencies", "└─ pcre2", "json-c",
		"nothing installed depends on this", "Bash completion has been installed.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if h, w := lipgloss.Height(it.view(nil, colList)), lipgloss.Width(it.view(nil, colList)); h != 30 || w != 140 {
		t.Errorf("rendered %dx%d, want 140x30", w, h)
	}
}

func TestInstalledPreviewForDependencyAndCask(t *testing.T) {
	it := newFixtureInstalled(t)
	it.filter = filterAll // docker-desktop stays selected
	it.apply()
	for range 4 {
		it.update(press("j")) // [docker-desktop] fd fontbase git json-c
	}
	out := stripANSI(it.view(nil, colList))
	if !strings.Contains(out, "as a dependency") || !strings.Contains(out, "git, pcre2") {
		t.Errorf("json-c preview should show it is a dependency used by git and pcre2:\n%s", out)
	}

	it.update(press("home")) // docker-desktop
	out = stripANSI(it.view(nil, colList))
	if !strings.Contains(out, "4.95.0,242831 ↑") || strings.Contains(out, "Used by") {
		t.Errorf("cask preview should show the newer version and no dependency sections:\n%s", out)
	}
}

func TestInstalledPreviewForCaskWithDependencies(t *testing.T) {
	snap := fixtureSnapshot(t)
	for i, p := range snap.Packages {
		if p.Name == "docker-desktop" {
			snap.Packages[i].Deps, snap.Packages[i].DirectDeps = []string{"fd"}, []string{"fd"}
		}
	}
	it := newInstalledTab(testStyles)
	it.resize(77, 63, 30)
	it.setData(snap.Packages, state.NewIndex(snap.Packages))
	out := stripANSI(it.view(nil, colList)) // docker-desktop is first
	if !strings.Contains(out, "Dependencies") || !strings.Contains(out, "└─ fd") {
		t.Errorf("a cask that needs something should show it:\n%s", out)
	}
}

func TestInstalledEmptyFilter(t *testing.T) {
	it := newInstalledTab(testStyles)
	it.resize(55, 45, 12)
	it.setData(nil, state.NewIndex(nil))
	if out := stripANSI(it.view(nil, colList)); !strings.Contains(out, "loading…") || strings.Contains(out, "nothing in this view") {
		t.Errorf("before its first result the preview should say it is loading:\n%s", out)
	}
	it.loaded = true
	if _, ok := it.selected(); ok {
		t.Error("nothing should be selected in an empty list")
	}
	if out := stripANSI(it.view(nil, colList)); !strings.Contains(out, "nothing in this view") {
		t.Errorf("empty preview missing:\n%s", out)
	}
}

func TestInstalledPreviewKeepsBorderWithManyDeps(t *testing.T) {
	big := brew.Package{Name: "big", FullName: "big", Kind: brew.KindFormula, Version: "1.0", OnRequest: true}
	pkgs := []brew.Package{}
	for i := range 40 {
		dep := fmt.Sprintf("dep%02d", i)
		big.DirectDeps = append(big.DirectDeps, dep)
		big.Deps = append(big.Deps, dep)
		pkgs = append(pkgs, brew.Package{Name: dep, FullName: dep, Kind: brew.KindFormula, Version: "1.0"})
	}
	pkgs = append(pkgs, big)
	it := newInstalledTab(testStyles)
	it.resize(77, 63, 30)
	it.setData(pkgs, state.NewIndex(pkgs))
	if p, ok := it.selected(); !ok || p.Name != "big" {
		t.Fatalf("selected %q, want big", p.Name)
	}
	out := it.view(nil, colList)
	if h := lipgloss.Height(out); h != 30 {
		t.Errorf("height = %d, want 30", h)
	}
	rows := strings.Split(stripANSI(out), "\n")
	last := []rune(rows[len(rows)-1])
	if preview := string(last[it.listW:]); !strings.HasPrefix(preview, "╰") || !strings.HasSuffix(preview, "╯") {
		t.Errorf("preview pane lost its bottom border: %q", preview)
	}
}

func TestInstalledErrorWithRowsShowsBanner(t *testing.T) {
	it := newFixtureInstalled(t)
	view := it.view(errors.New("brew info --json=v2 --installed: exit status 1\nError: second line"), colList)
	out := stripANSI(view)
	for _, want := range []string{"refresh failed · r to retry", "exit status 1", "lua-language-server", "10.5.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "second line") || strings.Contains(out, "couldn't load") {
		t.Errorf("banner should be one line of error above the rows:\n%s", out)
	}
	if h := lipgloss.Height(view); h != 30 {
		t.Errorf("height = %d, want 30", h)
	}
}

func TestInstalledKeepsSelectionAcrossRefresh(t *testing.T) {
	it := newFixtureInstalled(t)
	it.update(press("j"))
	it.update(press("j"))
	it.update(press("j")) // git
	snap := fixtureSnapshot(t)
	more := append([]brew.Package{{Name: "act", FullName: "act", Kind: brew.KindFormula, OnRequest: true}}, snap.Packages...)
	it.setData(more, state.NewIndex(more))
	if p, _ := it.selected(); p.Name != "git" {
		t.Errorf("selection moved to %q when a package was added above it", p.Name)
	}
}

func TestInstalledSortsSameNameFormulaBeforeCask(t *testing.T) {
	it := newInstalledTab(testStyles)
	it.resize(77, 63, 30)
	it.filter = filterAll
	pkgs := []brew.Package{
		{Name: "docker", FullName: "docker", Kind: brew.KindCask},
		{Name: "docker", FullName: "docker", Kind: brew.KindFormula, OnRequest: true},
	}
	it.setData(pkgs, state.NewIndex(pkgs))
	if it.rows[0].Kind != brew.KindFormula || it.rows[1].Kind != brew.KindCask {
		t.Errorf("same-name rows should sort formula first: %+v", it.rows)
	}
}

func TestInstalledFilterChangeKeepsSelection(t *testing.T) {
	it := newFixtureInstalled(t)
	for range 3 {
		it.update(press("j")) // git
	}
	it.filter = filterAll
	it.apply()
	if p, _ := it.selected(); p.Name != "git" {
		t.Errorf("switching to all should keep git selected, got %q", p.Name)
	}
	it.filter = filterCasks // git isn't shown
	it.apply()
	if p, ok := it.selected(); !ok || p.Kind != brew.KindCask {
		t.Errorf("a view that hides the selection should still select a row, got %+v", p)
	}
}

func TestPreviewFocusIsLeftBehind(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Top-level")
	keys(t, a, "tab")
	if a.focus != colDetails {
		t.Fatal("tab should focus the preview")
	}
	for i := 0; label(a) != "Outdated" && i < 10; i++ {
		keys(t, a, "]") // Outdated has no details: the focus drops to its list
	}
	for i := 0; label(a) != "Top-level" && i < 10; i++ {
		keys(t, a, "[")
	}
	if a.focus == colDetails {
		t.Error("coming back to Installed starts on the list")
	}
}
