package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestScrollerWindow(t *testing.T) {
	var s scroller
	body := numbered(10)
	s.scroll("j", 3)
	s.scroll("ctrl+d", 3)
	if got := s.window(body, 4); !strings.HasPrefix(got, "line 5\n") {
		t.Errorf("4 lines down, got %q", got)
	}
	s.scroll("G", 3)
	if got := s.window(body, 4); got != "line 7\nline 8\nline 9\nline 10" || s.off != 6 {
		t.Errorf("G stops at the last screenful, got %q (off %d)", got, s.off)
	}
	s.scroll("k", 3)
	s.scroll("ctrl+u", 3)
	if s.off != 2 {
		t.Errorf("back up 4 lines: off %d", s.off)
	}
	s.scroll("home", 3)
	s.scroll("k", 3)
	if s.off != 0 {
		t.Errorf("never above the top: off %d", s.off)
	}
	if s.scroll("x", 3) {
		t.Error("x isn't a scrolling key")
	}
}

func TestInstalledPreviewTakesTheFocus(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 16})
	selectInstalled(t, a, "git")
	before := stripANSI(a.render())
	keys(t, a, "tab")
	if !a.installed.pv.focus || !strings.Contains(stripANSI(a.statusBar()), "j/k scroll · h back to the list") {
		t.Fatalf("tab should focus the preview: %q", stripANSI(a.statusBar()))
	}
	keys(t, a, "j", "j", "j")
	if p, _ := a.installed.selected(); p.Name != "git" {
		t.Errorf("j scrolls the preview, the selection stays: %s", p.Name)
	}
	if after := stripANSI(a.render()); after == before || !strings.Contains(before, "git 2.51.0") || strings.Contains(after, "git 2.51.0") {
		t.Errorf("the preview should have scrolled past its first lines:\n%s", after)
	}
	keys(t, a, "esc")
	if a.installed.pv.focus {
		t.Error("esc goes back to the list")
	}
	keys(t, a, "j")
	if p, _ := a.installed.selected(); p.Name == "git" {
		t.Error("with the list focused, j moves the selection")
	}
	keys(t, a, "k", "enter")
	if !a.installed.pv.focus {
		t.Error("enter focuses the preview too")
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "git 2.51.0") {
		t.Errorf("coming back to git starts its preview at the top:\n%s", out)
	}
}

func TestScrollerKAfterGMovesUpAtOnce(t *testing.T) {
	var s scroller
	body := numbered(10)
	s.window(body, 4) // drawn once: the last screenful starts at line 7
	s.scroll("G", 2)
	s.scroll("k", 2)
	if s.off != 5 {
		t.Errorf("k after G should move up one line from the bottom: off %d, want 5", s.off)
	}
}
