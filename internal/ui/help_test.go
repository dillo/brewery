package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestHelpListsTheKeysForWhereYouAre(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "?")
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
	keys(t, a, "4", "?")
	if out := stripANSI(a.render()); !strings.Contains(out, "follow the log full screen") || strings.Contains(out, "uninstall") {
		t.Errorf("Services help:\n%s", out)
	}
}

func TestHelpInTheJobsView(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter", "?")
	if out := stripANSI(a.render()); !a.help || !strings.Contains(out, "replay the job's output") {
		t.Errorf("? works in :jobs too:\n%s", out)
	}
}

func TestHelpHintAndSmallWindows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "? help") {
		t.Errorf("the status bar should mention ? help: %q", bar)
	}
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	keys(t, a, "2", "?")
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
