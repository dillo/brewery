package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

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
	it.update(press("f")) // all
	for range 4 {
		it.update(press("j")) // docker-desktop fd fontbase git json-c
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
	if _, ok := it.selected(); ok {
		t.Error("nothing should be selected in an empty list")
	}
	if out := stripANSI(it.view(nil)); !strings.Contains(out, "nothing matches this filter") {
		t.Errorf("empty preview missing:\n%s", out)
	}
}
