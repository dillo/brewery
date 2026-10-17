package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestSplitFor(t *testing.T) {
	for _, tt := range []struct {
		w, h int
		show bool
		want split
	}{
		{140, 30, false, split{listW: 77, listH: 30, pvW: 63, pvH: 30}},
		{100, 20, false, split{listW: 55, listH: 20, pvW: 45, pvH: 20}},
		{80, 21, false, split{listW: 80, listH: 12, pvW: 80, pvH: 9, stacked: true}},
		{80, 9, false, split{listW: 80, listH: 9}},   // a stacked preview would be too short
		{80, 9, true, split{pvW: 80, pvH: 9}},        // so it takes turns, like a narrow one
		{50, 20, false, split{listW: 50, listH: 20}}, // narrow: the list alone
		{50, 20, true, split{pvW: 50, pvH: 20}},      // or the preview alone
	} {
		if got := splitFor(tt.w, tt.h, tt.show); got != tt.want {
			t.Errorf("splitFor(%d, %d, %v) = %+v, want %+v", tt.w, tt.h, tt.show, got, tt.want)
		}
	}
}

func TestInstalledStacksThenHidesThePreview(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	show(t, a, "Top-level")
	out := stripANSI(a.render())
	lines := strings.Split(out, "\n")
	list, preview := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "Top-level 5") && list < 0 {
			list = i
		}
		if strings.Contains(l, "App to build and share") {
			preview = i
		}
	}
	if list < 0 || preview < list || lipgloss.Height(out) != 24 {
		t.Errorf("at 80 columns the preview goes under the list:\n%s", out)
	}

	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	if out := stripANSI(a.render()); strings.Contains(out, "App to build and share") || !strings.Contains(out, "docker-desktop") {
		t.Errorf("at 50 columns only the list shows:\n%s", out)
	}
	keys(t, a, "tab")
	if out := stripANSI(a.render()); !strings.Contains(out, "App to build and share") || strings.Contains(out, "Top 5") {
		t.Errorf("tab shows the preview instead of the list:\n%s", out)
	}
	keys(t, a, "esc")
	if out := stripANSI(a.render()); !strings.Contains(out, "Top 5") {
		t.Errorf("esc goes back to the list:\n%s", out)
	}
}

func TestServicesPreviewTakesTurnsWhenNarrow(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	show(t, a, "Services")
	if out := stripANSI(a.render()); strings.Contains(out, "End of the log") {
		t.Errorf("at 50 columns only the list shows:\n%s", out)
	}
	keys(t, a, "enter")
	if out := stripANSI(a.render()); !strings.Contains(out, "End of the log") || strings.Contains(out, "STATUS") {
		t.Errorf("enter shows the preview instead of the list:\n%s", out)
	}
	keys(t, a, "esc")
	if out := stripANSI(a.render()); !strings.Contains(out, "STATUS") {
		t.Errorf("esc goes back to the list:\n%s", out)
	}
}

func TestServicesPreviewTakesTurnsWhenShort(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 14})
	show(t, a, "Services")
	if s := splitFor(a.services.w, a.services.h, false); s.pvW != 0 {
		t.Fatalf("test needs a short tab, got %dx%d: %+v", a.services.w, a.services.h, s)
	}
	if out := stripANSI(a.render()); strings.Contains(out, "End of the log") || !strings.Contains(out, "STATUS") {
		t.Errorf("a short tab shows only the list:\n%s", out)
	}
	keys(t, a, "enter")
	if out := stripANSI(a.render()); !strings.Contains(out, "End of the log") || strings.Contains(out, "STATUS") {
		t.Errorf("enter shows the preview instead of the list:\n%s", out)
	}
	keys(t, a, "esc")
	if out := stripANSI(a.render()); !strings.Contains(out, "STATUS") {
		t.Errorf("esc goes back to the list:\n%s", out)
	}
}

func TestStalePreviewFlagDoesNotSwallowEsc(t *testing.T) {
	p := newFakeProc(0)
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
	show(t, a, "Services")
	a.services.pvOpen = true // left over from a narrower layout
	if !a.drawer.open {
		t.Fatal("the drawer should be open")
	}
	keys(t, a, "esc")
	if a.drawer.open {
		t.Error("esc hides the drawer; the stale flag must not take it")
	}
	p.exit()
	resume(t, a, waiting)
}

func TestServicesPreviewAloneIgnoresListKeys(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	var copied string
	a.UseClipboard(func(text string) error { copied = text; return nil })
	show(t, a, "Services")
	keys(t, a, "j")
	before := a.services.cursor()
	keys(t, a, "enter", "j", "space", "S", "g", "g", "/")
	if a.services.cursor() != before || a.marks() != 0 || a.confirm != nil || len(fs.calls) != 0 {
		t.Errorf("keys reached the hidden list: cursor %d→%d, marks %d, confirm %v, calls %v",
			before, a.services.cursor(), a.marks(), a.confirm, fs.calls)
	}
	if a.mode != modeNormal {
		t.Errorf("/ opened a filter on the hidden list: mode %d", a.mode)
	}
	if !a.services.previewShown() {
		t.Fatal("the preview should still show")
	}
	name, _, _ := a.selection()
	_, cmd := a.Update(press("y")) // y copies the service the preview shows
	if cmd != nil {
		cmd()
	}
	if name == "" || copied != name {
		t.Errorf("y copied %q, want the selected service %q", copied, name)
	}
	keys(t, a, "tab")
	if out := stripANSI(a.render()); !strings.Contains(out, "STATUS") {
		t.Errorf("tab goes back to the list:\n%s", out)
	}
	keys(t, a, "enter", "2")
	if a.tab != tabInstalled {
		t.Errorf("tab keys still work under the preview: on %d", a.tab)
	}
}

func TestServicesPreviewFlagClearsWhenThereIsRoom(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	show(t, a, "Services")
	keys(t, a, "enter")
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40}) // room beside the list
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})  // and narrow again
	if out := stripANSI(a.render()); !strings.Contains(out, "STATUS") || strings.Contains(out, "End of the log") {
		t.Errorf("after shrinking again, the list shows first:\n%s", out)
	}
}
