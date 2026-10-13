package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// picked records which choice ran.
var picked string

func testConfirm() *confirmBox {
	pick := func(name string) func() tea.Cmd {
		return func() tea.Cmd { picked = name; return nil }
	}
	return &confirmBox{
		title: "Uninstall git 2.51.0?",
		body:  []string{"It leaves 2 orphaned dependencies: json-c, pcre2"},
		choices: []choice{
			{"y", "uninstall", pick("uninstall")},
			{"o", "uninstall + orphans", pick("orphans")},
			{"n", "cancel", nil},
		},
	}
}

func TestConfirmAnswers(t *testing.T) {
	c := testConfirm()
	ch, ok := c.answer("o")
	if !ok || ch.run == nil {
		t.Fatalf("o should be a choice: %+v %v", ch, ok)
	}
	if ch.run(); picked != "orphans" {
		t.Errorf("o ran %q, want orphans", picked)
	}
	if ch, ok := c.answer("n"); !ok || ch.run != nil {
		t.Errorf("n cancels: %+v", ch)
	}
	if ch, ok := c.answer("esc"); !ok || ch.run != nil {
		t.Errorf("esc always cancels: %+v", ch)
	}
	if _, ok := c.answer("q"); ok {
		t.Error("keys that are not choices are ignored, so a stray q can't quit or confirm")
	}
}

func TestConfirmView(t *testing.T) {
	out := stripANSI(testConfirm().view(testStyles, 80))
	for _, want := range []string{"Uninstall git 2.51.0?", "orphaned dependencies: json-c, pcre2", "[y] uninstall", "[o] uninstall + orphans", "[n] cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if w := lipgloss.Width(testConfirm().view(testStyles, 30)); w > 30 {
		t.Errorf("box is %d wide with 30 columns available", w)
	}
}

func TestOverlayKeepsSizeAndShowsBoth(t *testing.T) {
	body := strings.TrimRight(strings.Repeat(strings.Repeat("·", 60)+"\n", 20), "\n")
	box := testConfirm().view(testStyles, 60)
	out := overlay(body, box, 60, 20)
	if lipgloss.Height(out) != 20 || lipgloss.Width(out) != 60 {
		t.Errorf("overlay is %dx%d, want 60x20", lipgloss.Width(out), lipgloss.Height(out))
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "Uninstall git") {
		t.Error("the box should be drawn")
	}
	lines := strings.Split(plain, "\n")
	if lines[0] != strings.Repeat("·", 60) {
		t.Errorf("the body should show around the box: %q", lines[0])
	}
}

func TestConfirmWithoutBodyHasOneBlankLine(t *testing.T) {
	c := &confirmBox{title: "Uninstall fd 10.5.0?", choices: []choice{{"y", "uninstall", nil}, {"n", "cancel", nil}}}
	lines := strings.Split(stripANSI(c.view(testStyles, 80)), "\n")
	if len(lines) != 5 { // border, title, blank, choices, border
		t.Errorf("want 5 lines, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}
}
