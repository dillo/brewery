package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dillo/brewery/internal/brew"
)

func TestSidebarHasEveryViewOnce(t *testing.T) {
	seen := map[[2]int]int{}
	for _, e := range entries {
		seen[[2]int{e.tab, e.view}]++
	}
	for f := range views {
		if n := seen[[2]int{tabInstalled, f}]; n != 1 {
			t.Errorf("Installed view %s is in the sidebar %d times", views[f].name, n)
		}
	}
	for v := range discoverViews {
		if n := seen[[2]int{tabDiscover, v}]; n != 1 {
			t.Errorf("Discover view %s is in the sidebar %d times", discoverViews[v].name, n)
		}
	}
	for _, tab := range []int{tabHome, tabOutdated, tabServices, tabTaps} {
		if n := seen[[2]int{tab, 0}]; n != 1 {
			t.Errorf("tab %d is in the sidebar %d times", tab, n)
		}
	}
}

func TestSelectEntryShowsItsTabAndView(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	for _, tt := range []struct {
		label string
		tab   int
		check func() bool
	}{
		{"Casks", tabInstalled, func() bool { return a.installed.filter == filterCasks }},
		{"Pinned", tabInstalled, func() bool { return a.installed.filter == filterPinned }},
		{"Rising", tabDiscover, func() bool { return a.discover.showing == discRising }},
		{"Search all", tabDiscover, func() bool { return a.discover.showing == discAll }},
		{"Taps", tabTaps, func() bool { return true }},
	} {
		settle(t, a, a.selectEntry(entryIndex(tt.label)))
		if a.tab != tt.tab || !tt.check() || entries[a.sel].label != tt.label {
			t.Errorf("%s: tab %d, sel %s", tt.label, a.tab, entries[a.sel].label)
		}
	}
}

func TestSelectSectionReturnsToItsLastEntry(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	settle(t, a, a.selectSection(secDiscover))
	if got := entries[a.sel].label; got != "Popular tools" {
		t.Errorf("first visit to Discover: %s, want its first entry", got)
	}
	settle(t, a, a.selectEntry(entryIndex("Taps")))
	settle(t, a, a.selectSection(secHome))
	settle(t, a, a.selectSection(secManage))
	if got := entries[a.sel].label; got != "Taps" {
		t.Errorf("back to Manage: %s, want Taps", got)
	}
}

func TestStepEntryWraps(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	settle(t, a, a.stepEntry(-1))
	if got := entries[a.sel].label; got != "Search all" {
		t.Errorf("before Home: %s, want the last entry", got)
	}
	settle(t, a, a.stepEntry(1))
	if got := entries[a.sel].label; got != "Home" {
		t.Errorf("after the last entry: %s, want Home", got)
	}
}

func TestSidebarShowsCountsAndChips(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	out := stripANSI(a.sidebarView(sidebarW, 40))
	for _, want := range []string{
		"All 7", "Top-level 5", "Formulae 5", "Casks 2", "Dependencies 2", "Pinned 1",
		fmt.Sprintf("Outdated ↑%d", len(a.snap.Outdated)),
		fmt.Sprintf("Taps %d", len(a.snap.Taps)),
		"PACKAGES", "MANAGE", "DISCOVER", "Search all",
	} {
		if !strings.Contains(squash(out), want) {
			t.Errorf("sidebar lacks %q:\n%s", want, out)
		}
	}
	for i, l := range strings.Split(out, "\n") {
		if w := lipgloss.Width(l); w > sidebarW {
			t.Errorf("line %d is %d wide, want <= %d: %q", i, w, sidebarW, l)
		}
	}
	raw := strings.Split(a.sidebarView(sidebarW, 40), "\n")
	open, _, _ := strings.Cut(a.st.TabActive.Render("x"), "x") // the block's opening escape
	for _, l := range raw {
		if got, want := strings.Contains(l, open), strings.Contains(l, "Home"); got != want {
			t.Errorf("only the selected entry, Home, is a TabActive block: %q", l)
		}
	}
	failing := 1 // the fixtures have one already
	a.snap.Services = append(a.snap.Services, brew.Service{Name: "broken", Status: "error"})
	if out := squash(stripANSI(a.sidebarView(sidebarW, 40))); !strings.Contains(out, fmt.Sprintf("Services ✗%d", failing+1)) {
		t.Errorf("failing services should show a chip: %s", out)
	}
	a.errs[qTaps] = errors.New("offline")
	if out := squash(stripANSI(a.sidebarView(sidebarW, 40))); !strings.Contains(out, "Taps ⚠") {
		t.Errorf("a failed refresh should mark its entry: %s", out)
	}
}

func label(a *App) string { return entries[a.sel].label }

func TestSidebarKeysMoveThroughEntries(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	if label(a) != "Home" || !a.sideFocus {
		t.Fatalf("brewery should open on Home with the sidebar focused: %s %v", label(a), a.sideFocus)
	}
	keys(t, a, "k")
	if label(a) != "Home" {
		t.Errorf("k at the top: %s", label(a))
	}
	keys(t, a, "j", "j", "j", "j")
	if label(a) != "Casks" || a.tab != tabInstalled || a.installed.filter != filterCasks || !a.sideFocus {
		t.Errorf("4×j: %s tab %d filter %d focus %v", label(a), a.tab, a.installed.filter, a.sideFocus)
	}
	keys(t, a, "G")
	if label(a) != "Search all" {
		t.Errorf("G: %s", label(a))
	}
	keys(t, a, "g", "g")
	if label(a) != "Home" {
		t.Errorf("gg: %s", label(a))
	}
}

func TestFocusMovesAcrossColumns(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "j", "l")
	if a.sideFocus || a.installed.pv.focus {
		t.Fatalf("l from the sidebar should focus the list")
	}
	keys(t, a, "l")
	if !a.installed.pv.focus {
		t.Fatalf("l from the list should focus the details")
	}
	keys(t, a, "h")
	if a.installed.pv.focus || a.sideFocus {
		t.Fatalf("h from the details should focus the list")
	}
	keys(t, a, "h")
	if !a.sideFocus {
		t.Fatalf("h from the list should focus the sidebar")
	}
	keys(t, a, "enter")
	if a.sideFocus {
		t.Errorf("enter in the sidebar should focus the list")
	}
}

func TestNumberKeysJumpToSections(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "4", "j")
	if label(a) != "Popular apps" || !a.sideFocus {
		t.Fatalf("4 then j: %s, focus %v", label(a), a.sideFocus)
	}
	keys(t, a, "1", "4")
	if label(a) != "Popular apps" {
		t.Errorf("4 should return to Discover's last entry: %s", label(a))
	}
	keys(t, a, "3")
	if label(a) != "Services" {
		t.Errorf("3: %s", label(a))
	}
}

func TestBracketsStepEntriesAndKeepTheFocus(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Taps")
	keys(t, a, "]")
	if label(a) != "Popular tools" || a.sideFocus {
		t.Errorf("] from Taps: %s, focus %v", label(a), a.sideFocus)
	}
	keys(t, a, "[")
	if label(a) != "Taps" {
		t.Errorf("[: %s", label(a))
	}
}

func TestRowKeysDoNothingInTheSidebar(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "j", "u", "x", "p", "space", "y")
	if len(fs.calls) != 0 || a.confirm != nil || a.marks() != 0 {
		t.Errorf("row keys in the sidebar acted: %v %+v %d", fs.calls, a.confirm, a.marks())
	}
	keys(t, a, "U")
	if got := lastCall(fs); len(got) == 0 || got[0] != "update" {
		t.Errorf("U should work from the sidebar: %v", got)
	}
}

func TestSlashFromTheSidebarOnHomeKeepsTheFocus(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "/")
	if a.mode == modeFilter || !a.sideFocus {
		t.Errorf("Home has no filter: mode %d, focus %v", a.mode, a.sideFocus)
	}
}

func TestSidebarKeepsItsSelectedEntryInView(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	for _, l := range []string{"Search all", "Home", "Pinned", "Fonts"} {
		settle(t, a, a.selectEntry(entryIndex(l)))
		out := a.sidebarView(sidebarW, 20-4)
		if !strings.Contains(squash(stripANSI(out)), l) {
			t.Errorf("%s is out of view:\n%s", l, stripANSI(out))
		}
		if n := lipgloss.Height(out); n != 20-4 {
			t.Errorf("%s: sidebar is %d lines, want %d", l, n, 20-4)
		}
		if strings.Contains(stripANSI(out), "more") {
			t.Errorf("%s: the pane cut the sidebar:\n%s", l, stripANSI(out))
		}
		if full := stripANSI(a.render()); !strings.Contains(squash(full), l) {
			t.Errorf("%s is out of view in the 120×20 screen:\n%s", l, full)
		}
	}
}

func TestSlashFromTheSidebarFiltersTheList(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "j", "/")
	if a.mode != modeFilter || a.sideFocus {
		t.Errorf("/ should filter the list: mode %d, focus %v", a.mode, a.sideFocus)
	}
}

func TestHomeTileOpensItsEntry(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "l", "l", "enter") // Home's first tile, then the Outdated tile
	if label(a) != "Outdated" || a.sideFocus {
		t.Errorf("enter on the Outdated tile: %s, focus %v", label(a), a.sideFocus)
	}
}

func TestHomeKeysWalkTheTilesAndBackToTheSidebar(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "l", "l", "l")
	if a.sideFocus || a.home.sel != 2 {
		t.Fatalf("l from the sidebar, then l twice: focus %v, tile %d; want the Services tile", a.sideFocus, a.home.sel)
	}
	keys(t, a, "h", "h", "h")
	if !a.sideFocus || label(a) != "Home" {
		t.Fatalf("h past the first tile should reach the sidebar: focus %v, %s", a.sideFocus, label(a))
	}
	keys(t, a, "l")
	if a.home.sel != 0 || a.home.area != areaTiles {
		t.Errorf("l from the sidebar should start on Installed: tile %d, area %d", a.home.sel, a.home.area)
	}
	keys(t, a, "j", "h")
	if !a.sideFocus {
		t.Errorf("h from Doctor should reach the sidebar")
	}
}

func TestEnterOnRecentJobsOpensTheJobsList(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "l", "j", "l", "enter")
	if !a.jobsOpen {
		t.Errorf("enter on Recent jobs should open the jobs list")
	}
}
