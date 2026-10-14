package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/theme"
)

// choice is one answer to a confirmation: the key that picks it, a label,
// and what it does. A nil run cancels.
type choice struct {
	key   string
	label string
	run   func() tea.Cmd
}

// confirmBox asks before brewery does something destructive.
type confirmBox struct {
	title   string
	body    []string
	choices []choice
}

// answer maps a key press to a choice. esc always cancels; keys that are
// not choices are ignored (ok is false).
func (c *confirmBox) answer(key string) (picked choice, ok bool) {
	if key == "esc" {
		return choice{}, true
	}
	for _, ch := range c.choices {
		if ch.key == key {
			return ch, true
		}
	}
	return choice{}, false
}

// view renders the box, at most maxW columns wide.
func (c *confirmBox) view(st theme.Styles, maxW int) string {
	inner := min(56, max(maxW-4, 10))
	lines := []string{st.Warn.Bold(true).Render(ansi.Truncate(c.title, inner, "…"))}
	if len(c.body) > 0 {
		lines = append(lines, "")
	}
	for _, l := range c.body {
		lines = append(lines, ansi.Truncate(l, inner, "…"))
	}
	keys := make([]string, len(c.choices))
	for i, ch := range c.choices {
		keys[i] = st.Accent.Bold(true).Render("["+ch.key+"]") + " " + ch.label
	}
	lines = append(lines, "", ansi.Truncate(strings.Join(keys, "   "), inner, "…"))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(st.P.Peach).
		Padding(0, 1).Render(strings.Join(lines, "\n"))
}

// overlay draws box centered over body, which is w×h; the result is w×h.
func overlay(body, box string, w, h int) string {
	return overlayAt(body, box, w, h, max((w-lipgloss.Width(box))/2, 0), max((h-lipgloss.Height(box))/2, 0))
}

// overlayAt draws box over body with its top-left corner at x, y.
func overlayAt(body, box string, w, h, x, y int) string {
	canvas := lipgloss.NewCanvas(w, h)
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(body),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	))
	return canvas.Render()
}
