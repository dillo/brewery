package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestColumnsFor(t *testing.T) {
	for _, tt := range []struct {
		w          int
		focus      column
		hasDetails bool
		want       columns
	}{
		// ≥122: all three, whatever has the focus
		{140, colSidebar, true, columns{22, 64, 54}},
		{140, colList, true, columns{22, 64, 54}},
		{140, colDetails, true, columns{22, 64, 54}},
		{122, colList, true, columns{22, 55, 45}},
		// 100–121: the sidebar and list, or the list and details
		{121, colSidebar, true, columns{22, 99, 0}},
		{121, colList, true, columns{0, 66, 55}},
		{121, colDetails, true, columns{0, 66, 55}},
		{100, colList, true, columns{0, 55, 45}},
		{100, colSidebar, true, columns{22, 78, 0}},
		// 82–99: the sidebar and list, or the details alone
		{99, colSidebar, true, columns{22, 77, 0}},
		{99, colList, true, columns{22, 77, 0}},
		{99, colDetails, true, columns{0, 0, 99}},
		{82, colList, true, columns{22, 60, 0}},
		// <82: the focused column alone, at full width
		{81, colSidebar, true, columns{81, 0, 0}},
		{81, colList, true, columns{0, 81, 0}},
		{81, colDetails, true, columns{0, 0, 81}},
		// no details (Home, Outdated, Taps, jobs): sidebar and list, or one
		{140, colList, false, columns{22, 118, 0}},
		{140, colDetails, false, columns{22, 118, 0}}, // a details focus is the list's
		{82, colSidebar, false, columns{22, 60, 0}},
		{81, colSidebar, false, columns{81, 0, 0}},
		{81, colList, false, columns{0, 81, 0}},
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
