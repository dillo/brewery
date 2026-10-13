package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
)

func TestHomeTiles(t *testing.T) {
	h := homeTab{st: testStyles}
	tiles := h.tiles(fixtureSnapshot(t))
	got := []string{}
	for _, tl := range tiles {
		got = append(got, tl.title+"="+tl.value+" ("+tl.detail+")")
	}
	want := []string{
		"Installed=7 (5 formulae · 2 casks)",
		"Outdated=2 (lua-language-server, docker-desktop)",
		"Services=1 error (postgresql@16)",
		"Doctor=2 warnings (see below)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("tiles:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestHomeTilesHealthy(t *testing.T) {
	h := homeTab{st: testStyles}
	snap := state.Snapshot{
		Services:  []brew.Service{{Name: "redis", Status: "started"}},
		DoctorRan: true,
	}
	tiles := h.tiles(snap)
	if tiles[1].value != "✓ 0" || tiles[2].value != "1 running" || tiles[3].value != "✓ clean" {
		t.Errorf("healthy tiles: %q %q %q", tiles[1].value, tiles[2].value, tiles[3].value)
	}
	if tiles[3].tab != -1 {
		t.Error("doctor tile has no tab to open")
	}
	if got := h.tiles(state.Snapshot{}); got[3].value != "…" {
		t.Errorf("doctor tile before doctor runs = %q", got[3].value)
	}
}

func TestHomeNavigation(t *testing.T) {
	h := homeTab{st: testStyles}
	snap := fixtureSnapshot(t)
	if tab := h.update(press("enter"), snap); tab != tabInstalled {
		t.Errorf("enter on first tile -> %d", tab)
	}
	h.update(press("l"), snap)
	h.update(press("l"), snap)
	if tab := h.update(press("enter"), snap); tab != tabServices {
		t.Errorf("enter on third tile -> %d", tab)
	}
	for range 5 {
		h.update(press("l"), snap)
	}
	if h.sel != 3 {
		t.Errorf("selection should stop at the last tile, got %d", h.sel)
	}
	if tab := h.update(press("enter"), snap); tab != -1 {
		t.Errorf("enter on doctor tile -> %d, want -1", tab)
	}
}

func TestHomeView(t *testing.T) {
	h := homeTab{st: testStyles}
	view := h.view(fixtureSnapshot(t), 140, 30)
	out := stripANSI(view)
	for _, want := range []string{"Installed", "5 formulae · 2 casks", "lua-language-server, docker-", "1 error",
		"⚠ The following taps are not trusted:", "brew untap oven-sh/bun", "⚠ Some installed casks are deprecated"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if lipgloss.Height(view) != 30 {
		t.Errorf("home view height = %d, want 30", lipgloss.Height(view))
	}
	clean := stripANSI(h.view(state.Snapshot{DoctorRan: true}, 140, 30))
	if !strings.Contains(clean, "Your system is ready to brew.") {
		t.Error("clean doctor message missing")
	}
}

func TestPlural(t *testing.T) {
	if plural(1, "warning") != "1 warning" || plural(3, "error") != "3 errors" {
		t.Error("plural wrong")
	}
}
