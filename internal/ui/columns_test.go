package ui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/jobs"
)

func TestColumnsFor(t *testing.T) {
	for _, tt := range []struct {
		w          int
		focus      column
		hasDetails bool
		want       columns
	}{
		// ≥126: all three, whatever has the focus
		{140, colSidebar, true, columns{26, 62, 52}},
		{140, colList, true, columns{26, 62, 52}},
		{140, colDetails, true, columns{26, 62, 52}},
		{126, colList, true, columns{26, 55, 45}},
		// 100–125: the sidebar and list, or the list and details
		{125, colSidebar, true, columns{26, 99, 0}},
		{125, colList, true, columns{0, 68, 57}},
		{125, colDetails, true, columns{0, 68, 57}},
		{100, colList, true, columns{0, 55, 45}},
		{100, colSidebar, true, columns{26, 74, 0}},
		// 86–99: the sidebar and list, or the details alone
		{99, colSidebar, true, columns{26, 73, 0}},
		{99, colList, true, columns{26, 73, 0}},
		{99, colDetails, true, columns{0, 0, 99}},
		{86, colList, true, columns{26, 60, 0}},
		// <86: the focused column alone, at full width
		{85, colSidebar, true, columns{85, 0, 0}},
		{85, colList, true, columns{0, 85, 0}},
		{85, colDetails, true, columns{0, 0, 85}},
		// no details (Home, Outdated, Taps): sidebar and list, or one
		{140, colList, false, columns{26, 114, 0}},
		{140, colDetails, false, columns{26, 114, 0}}, // a details focus is the list's
		{86, colSidebar, false, columns{26, 60, 0}},
		{85, colSidebar, false, columns{85, 0, 0}},
		{85, colList, false, columns{0, 85, 0}},
		{85, colDetails, false, columns{0, 85, 0}}, // a details focus is the list's
		{110, colList, false, columns{26, 84, 0}},
	} {
		if got := columnsFor(tt.w, tt.focus, tt.hasDetails); got != tt.want {
			t.Errorf("columnsFor(%d, %d, %v) = %+v, want %+v", tt.w, tt.focus, tt.hasDetails, got, tt.want)
		}
	}
}

func TestColumnsSlideWithTheFocus(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	show(t, a, "Top-level")
	out := stripANSI(a.render())
	if !strings.Contains(out, "PACKAGES") || !strings.Contains(out, "NAME") || strings.Contains(out, "App to build and share") {
		t.Errorf("at 90 columns the list sits beside the sidebar, without details:\n%s", out)
	}
	keys(t, a, "l")
	out = stripANSI(a.render())
	if strings.Contains(out, "NAME") || strings.Contains(out, "PACKAGES") || !strings.Contains(out, "App to build and share") {
		t.Errorf("l turns the page to the details, alone:\n%s", out)
	}
	keys(t, a, "h")
	if out := stripANSI(a.render()); !strings.Contains(out, "NAME") || !strings.Contains(out, "PACKAGES") {
		t.Errorf("h turns back to the sidebar and list:\n%s", out)
	}

	a.Update(tea.WindowSizeMsg{Width: 110, Height: 30})
	keys(t, a, "l")
	out = stripANSI(a.render())
	if strings.Contains(out, "PACKAGES") || !strings.Contains(out, "NAME") || !strings.Contains(out, "App to build and share") {
		t.Errorf("at 110 columns the details push the sidebar out:\n%s", out)
	}
}

func TestRowKeysWorkFromTheDetailsAlone(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	show(t, a, "Services")
	keys(t, a, "l")
	if out := stripANSI(a.render()); !strings.Contains(out, "End of the log") || strings.Contains(out, "STATUS") {
		t.Fatalf("l shows the details alone:\n%s", out)
	}
	name, _, _ := a.selection()
	keys(t, a, "S")
	if a.confirm == nil || !strings.Contains(a.confirm.title, name) {
		t.Errorf("S from the details asks to stop %s: %+v", name, a.confirm)
	}
	keys(t, a, "n", "esc")
	if out := stripANSI(a.render()); !strings.Contains(out, "STATUS") {
		t.Errorf("esc goes back to the list:\n%s", out)
	}
}

func TestFilterFromTheDetailsGoesToTheList(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	show(t, a, "All")
	keys(t, a, "l", "/")
	if a.mode != modeFilter || a.focus != colList {
		t.Errorf("/ from the details filters the list, with the focus on it: mode %d, focus %d", a.mode, a.focus)
	}
}

func TestDetailsShowAloneAfterAResize(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	show(t, a, "Services")
	keys(t, a, "l")
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	if out := stripANSI(a.render()); !strings.Contains(out, "End of the log") || strings.Contains(out, "STATUS") {
		t.Errorf("after the resize the details show alone:\n%s", out)
	}
	keys(t, a, "esc")
	if out := stripANSI(a.render()); !strings.Contains(out, "STATUS") {
		t.Errorf("esc brings back the list:\n%s", out)
	}
}

func TestHistoryHasDetails(t *testing.T) {
	if !hasDetails(tabHistory) {
		t.Error("History shows the selected job's details beside its list")
	}
}

func TestEmptyListKeepsTheFocus(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseHistory(jobs.History{Dir: t.TempDir()}, nil)
	show(t, a, "History")
	keys(t, a, "enter")
	if a.focus != colList {
		t.Errorf("enter on an empty History: focus %d, want the list", a.focus)
	}
	keys(t, a, "l", "tab")
	if a.focus != colList {
		t.Errorf("l and tab on an empty list: focus %d, want the list", a.focus)
	}
	if hint := stripANSI(a.statusBar()); strings.Contains(hint, "j/k scroll") {
		t.Errorf("an empty list's hints don't offer scrolling: %s", hint)
	}
}

func TestFilteringToNothingLeavesTheDetails(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Top-level")
	keys(t, a, "l")
	keys(t, a, "/")
	typeText(t, a, "zzzq") // matches nothing
	keys(t, a, "enter")
	if out := stripANSI(a.render()); a.focus != colList || !strings.Contains(out, a.installed.emptyNote()) {
		t.Errorf("a filter that empties the list shows its note in the list: focus %d\n%s", a.focus, out)
	}
}

// The guard in Update, not the filter, takes the focus off details that a
// refresh left with nothing to show.
func TestRefreshEmptyingAListLeavesTheDetails(t *testing.T) {
	t.Run("services", func(t *testing.T) {
		a := newJobApp(t, &fakeStarter{})
		show(t, a, "Services")
		keys(t, a, "l")
		if a.focus != colDetails {
			t.Fatalf("l: focus %d", a.focus)
		}
		a.Update(resultMsg{q: qServices, services: []brew.Service{}})
		if h := a.actionHints(); a.focus != colList || !strings.HasPrefix(h, "enter actions") {
			t.Errorf("focus %d, hints %q", a.focus, h)
		}
	})
	t.Run("pinned", func(t *testing.T) {
		a := newJobApp(t, &fakeStarter{})
		show(t, a, "Pinned") // the fixture pins git
		pkgs := slices.Clone(a.snap.Packages)
		keys(t, a, "l")
		if a.focus != colDetails {
			t.Fatalf("l on a pinned package: focus %d", a.focus)
		}
		unpinned := slices.Clone(pkgs)
		for i := range unpinned {
			unpinned[i].Pinned = false
		}
		a.Update(resultMsg{q: qInstalled, packages: unpinned})
		if h := a.actionHints(); a.focus != colList || !strings.HasPrefix(h, "enter actions") || !strings.Contains(h, "l details") {
			t.Errorf("focus %d, hints %q", a.focus, h)
		}
	})
}

func TestSwitchingToAnEmptyTabKeepsTheList(t *testing.T) {
	a, _ := newDiscoverApp(t)
	a.UseHistory(jobs.History{Dir: t.TempDir()}, nil)
	show(t, a, "Search all")
	keys(t, a, "l")
	if a.focus != colDetails {
		t.Fatalf("l on Search all: focus %d (catalogue loaded: %v)", a.focus, a.discover.loaded)
	}
	show(t, a, "History")
	if label(a) != "History" || a.focus != colList {
		t.Errorf("onto an empty History: %s, focus %d", label(a), a.focus)
	}
}
