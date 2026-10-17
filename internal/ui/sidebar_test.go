package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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
	a.errs[qTaps] = errors.New("offline")
	if out := squash(stripANSI(a.sidebarView(sidebarW, 40))); !strings.Contains(out, "Taps ⚠") {
		t.Errorf("a failed refresh should mark its entry: %s", out)
	}
}

func TestShowSelectsEntryAndFocusesList(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.sideFocus = true
	show(t, a, "Casks")
	if a.sideFocus || entries[a.sel].label != "Casks" || a.installed.filter != filterCasks {
		t.Errorf("show: sideFocus %v, sel %s, filter %d", a.sideFocus, entries[a.sel].label, a.installed.filter)
	}
}
