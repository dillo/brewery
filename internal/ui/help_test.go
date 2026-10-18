package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestHelpListsTheKeysForWhereYouAre(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Top-level")
	keys(t, a, "?")
	out := stripANSI(a.render())
	for _, want := range []string{"Installed", "space", "mark for u/x on several", "gx", "open the homepage", "Everywhere", "any key closes this"} {
		if !strings.Contains(out, want) {
			t.Errorf("Installed help is missing %q:\n%s", want, out)
		}
	}
	keys(t, a, "u")
	if a.help || len(fs.calls) != 0 {
		t.Errorf("a key closes help and does nothing else: help %v, ran %v", a.help, fs.calls)
	}
	show(t, a, "Services")
	keys(t, a, "?")
	if out := stripANSI(a.render()); !strings.Contains(out, "follow the log full screen") || !strings.Contains(out, "l tab enter") || !strings.Contains(out, "scroll the log") || strings.Contains(out, "uninstall") {
		t.Errorf("Services help:\n%s", out)
	}
}

func TestServicesDetailsHints(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{text: map[string]string{pgLog: longLog(200)}}).read
	show(t, a, "Services")
	keys(t, a, "l")
	if h := a.actionHints(); !strings.HasPrefix(h, "j/k scroll the log · h back to the list") {
		t.Errorf("a long log scrolls, and the hint says so: %q", h)
	}
}

func TestServicesDetailsHintsForAShortLog(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{text: map[string]string{pgLog: "only line\n"}}).read
	show(t, a, "Services")
	keys(t, a, "l")
	h := a.actionHints()
	if a.focus != colDetails || strings.Contains(h, "scroll") || !strings.HasPrefix(h, "h back to the list") {
		t.Errorf("a log that fits has nothing to scroll: focus %d, hint %q", a.focus, h)
	}
}

func TestHelpInHistory(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter", "?")
	if out := stripANSI(a.render()); !a.help || !strings.Contains(out, "details, then j/k scroll") {
		t.Errorf("? works in History too:\n%s", out)
	}
}

func TestHelpHintAndSmallWindows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "? help") {
		t.Errorf("the status bar should mention ? help: %q", bar)
	}
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	show(t, a, "Top-level")
	keys(t, a, "?")
	out := stripANSI(a.render())
	if h := len(strings.Split(out, "\n")); h != 24 {
		t.Errorf("help must fit the window: %d lines", h)
	}
	for _, want := range []string{"space", "gx", "Everywhere", "quit", "any key closes this"} {
		if !strings.Contains(out, want) {
			t.Errorf("at 80x24 help goes to two columns and keeps %q:\n%s", want, out)
		}
	}
}

func TestHelpInJobsSaysWhatYCopies(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter", "?")
	out := stripANSI(a.render())
	if !strings.Contains(out, "copy the command") || strings.Contains(out, "open the homepage") {
		t.Errorf("in :jobs, y copies the command and gx has nothing to open:\n%s", out)
	}
}

func TestHelpOnMaintenance(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Maintenance")
	keys(t, a, "?")
	out := stripANSI(a.render())
	for _, want := range []string{"Maintenance", "run doctor, clean up, autoremove", "top, bottom"} {
		if !strings.Contains(out, want) {
			t.Errorf("Maintenance help lacks %q:\n%s", want, out)
		}
	}
	for _, not := range []string{"filter by name", "copy the name", "open the homepage"} {
		if strings.Contains(out, not) {
			t.Errorf("Maintenance help offers %q, which does nothing there:\n%s", not, out)
		}
	}
}

func TestHelpClosesWithoutActing(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Top-level")
	keys(t, a, "j", "?", "x") // x would ask to uninstall fd
	if a.help || a.confirm != nil || len(fs.calls) != 0 {
		t.Errorf("the key that closes help does nothing else: help %v, confirm %+v", a.help, a.confirm)
	}
}

func TestHelpClosesWhenBrewAsks(t *testing.T) {
	p := newFakeProc(0)
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "install x", args: []string{"install", "x"}}))
	keys(t, a, "?")
	p.out <- []byte("Password:")
	waiting = resume(t, a, waiting)
	if a.help || a.mode != modeInsert {
		t.Fatalf("help %v, mode %v: brew's question closes help", a.help, a.mode)
	}
	keys(t, a, "s")
	if got := p.input(); got != "s" {
		t.Errorf("brew got %q: the first key of the answer must reach it", got)
	}
	p.exit()
	resume(t, a, waiting)
}

func TestHelpSaysHowToClearTheFilter(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Top-level")
	keys(t, a, "?")
	if out := stripANSI(a.render()); !strings.Contains(out, "/ esc") || !strings.Contains(out, "clear the filter") {
		t.Errorf("Installed help doesn't say how to clear the filter:\n%s", out)
	}

	a = newJobsViewApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter", "?")
	if out := stripANSI(a.render()); !strings.Contains(out, "/ esc") || !strings.Contains(out, "clear the filter") {
		t.Errorf(":jobs help doesn't say how to clear the filter:\n%s", out)
	}
}
