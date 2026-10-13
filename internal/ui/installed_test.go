package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
)

func newFixtureInstalled(t *testing.T) installedTab {
	snap := fixtureSnapshot(t)
	it := newInstalledTab(testStyles)
	it.resize(140, 30)
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

func TestInstalledFilters(t *testing.T) {
	it := newFixtureInstalled(t)
	want := []struct{ filter, names string }{
		{"leaves", "fd git lua-language-server"},
		{"all", "docker-desktop fd fontbase git json-c lua-language-server pcre2"},
		{"casks", "docker-desktop fontbase"},
		{"deps", "json-c pcre2"},
		{"leaves", "fd git lua-language-server"}, // f wraps around
	}
	for i, w := range want {
		if i > 0 {
			it.update(press("f"))
		}
		if filterNames[it.filter] != w.filter {
			t.Fatalf("step %d: filter %s, want %s", i, filterNames[it.filter], w.filter)
		}
		if got := rowNames(it); got != w.names {
			t.Errorf("filter %s: got %q, want %q", w.filter, got, w.names)
		}
	}
}

func TestInstalledNavigationAndPreview(t *testing.T) {
	it := newFixtureInstalled(t)
	it.update(press("j")) // fd -> git
	p, ok := it.selected()
	if !ok || p.Name != "git" {
		t.Fatalf("selected %q, want git", p.Name)
	}
	out := stripANSI(it.view(nil))
	for _, want := range []string{
		"Installed · leaves · 3", "git 2.51.0", "Distributed revision control system",
		"on request · ", "pinned", "Dependencies", "└─ pcre2", "json-c",
		"nothing installed depends on this", "Bash completion has been installed.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if h, w := lipgloss.Height(it.view(nil)), lipgloss.Width(it.view(nil)); h != 30 || w != 140 {
		t.Errorf("rendered %dx%d, want 140x30", w, h)
	}
}

func TestInstalledPreviewForDependencyAndCask(t *testing.T) {
	it := newFixtureInstalled(t)
	it.update(press("f")) // all: fd stays selected
	for range 3 {
		it.update(press("j")) // docker-desktop [fd] fontbase git json-c
	}
	out := stripANSI(it.view(nil))
	if !strings.Contains(out, "as a dependency") || !strings.Contains(out, "git, pcre2") {
		t.Errorf("json-c preview should show it is a dependency used by git and pcre2:\n%s", out)
	}

	it.update(press("g")) // docker-desktop
	out = stripANSI(it.view(nil))
	if !strings.Contains(out, "4.95.0,242831 ↑") || strings.Contains(out, "Dependencies") {
		t.Errorf("cask preview should show the newer version and no dependency section:\n%s", out)
	}
}

func TestInstalledEmptyFilter(t *testing.T) {
	it := newInstalledTab(testStyles)
	it.resize(100, 12)
	it.setData(nil, state.NewIndex(nil))
	if out := stripANSI(it.view(nil)); !strings.Contains(out, "loading…") || strings.Contains(out, "nothing matches") {
		t.Errorf("before its first result the preview should say it is loading:\n%s", out)
	}
	it.loaded = true
	if _, ok := it.selected(); ok {
		t.Error("nothing should be selected in an empty list")
	}
	if out := stripANSI(it.view(nil)); !strings.Contains(out, "nothing matches this filter") {
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
	it.resize(140, 30)
	it.setData(pkgs, state.NewIndex(pkgs))
	if p, ok := it.selected(); !ok || p.Name != "big" {
		t.Fatalf("selected %q, want big", p.Name)
	}
	out := it.view(nil)
	if h := lipgloss.Height(out); h != 30 {
		t.Errorf("height = %d, want 30", h)
	}
	rows := strings.Split(stripANSI(out), "\n")
	last := []rune(rows[len(rows)-1])
	if preview := string(last[it.listWidth():]); !strings.HasPrefix(preview, "╰") || !strings.HasSuffix(preview, "╯") {
		t.Errorf("preview pane lost its bottom border: %q", preview)
	}
}

func TestInstalledErrorWithRowsShowsBanner(t *testing.T) {
	it := newFixtureInstalled(t)
	view := it.view(errors.New("brew info --json=v2 --installed: exit status 1\nError: second line"))
	out := stripANSI(view)
	for _, want := range []string{"refresh failed · r to retry", "exit status 1", "lua-language-server", "fd 10.5.0"} {
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
	it.update(press("j")) // fd -> git
	snap := fixtureSnapshot(t)
	more := append([]brew.Package{{Name: "act", FullName: "act", Kind: brew.KindFormula, OnRequest: true}}, snap.Packages...)
	it.setData(more, state.NewIndex(more))
	if p, _ := it.selected(); p.Name != "git" {
		t.Errorf("selection moved to %q when a package was added above it", p.Name)
	}
}

func TestInstalledSortsSameNameFormulaBeforeCask(t *testing.T) {
	it := newInstalledTab(testStyles)
	it.resize(140, 30)
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
	it.update(press("j")) // git
	it.update(press("f")) // all
	if p, _ := it.selected(); p.Name != "git" {
		t.Errorf("switching to all should keep git selected, got %q", p.Name)
	}
	it.update(press("f")) // casks: git isn't shown
	if p, ok := it.selected(); !ok || p.Kind != brew.KindCask {
		t.Errorf("a filter that hides the selection should still select a row, got %+v", p)
	}
}
