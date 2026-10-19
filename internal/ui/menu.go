package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/theme"
)

// menuItem is one thing the action menu offers: the key that does the
// same outside the menu, a label, and what it does.
type menuItem struct {
	key   string
	label string
	run   func() tea.Cmd
}

// menuBox lists what can be done to the selected row, or to the marked
// rows. enter, or an item's key, runs it.
type menuBox struct {
	title    string
	items    []menuItem
	cursor   int
	gPending bool // g was pressed: x next picks gx, esc or q still closes
}

// menuFooter says how to use the menu.
const menuFooter = "j/k move · enter run · esc close"

// key handles a key press while the menu is open. done is true when the
// menu should close; run is what was picked, or nil. After g, x picks gx,
// esc and q close the menu, and any other key does nothing.
func (m *menuBox) key(k string) (run func() tea.Cmd, done bool) {
	if m.gPending {
		m.gPending = false
		switch k {
		case "x":
			return m.pick("gx")
		case "esc", "q":
			return nil, true
		}
		return nil, false
	}
	switch k {
	case "esc", "q":
		return nil, true
	case "j", "down":
		m.cursor = (m.cursor + 1) % len(m.items)
	case "k", "up":
		m.cursor = (m.cursor + len(m.items) - 1) % len(m.items)
	case "enter":
		return m.items[m.cursor].run, true
	case "g":
		m.gPending = true
	default:
		return m.pick(k)
	}
	return nil, false
}

// pick finds the item whose key is k; done is false when there is none.
func (m *menuBox) pick(k string) (run func() tea.Cmd, done bool) {
	for _, it := range m.items {
		if it.key == k {
			return it.run, true
		}
	}
	return nil, false
}

// view renders the menu, at most maxW columns wide: the title, the items
// with their keys at the right, and how to use it.
func (m *menuBox) view(st theme.Styles, maxW int) string {
	labelW, keyW := 0, 0
	for _, it := range m.items {
		labelW = max(labelW, lipgloss.Width(it.label))
		keyW = max(keyW, lipgloss.Width(it.key))
	}
	inner := min(max(2+labelW+2+keyW, lipgloss.Width(menuFooter)), 56, max(maxW-4, 10))
	lines := []string{st.Title.Render(ansi.Truncate(m.title, inner, "…")), ""}
	for i, it := range m.items {
		label := ansi.Truncate(it.label, max(inner-2-2-keyW, 1), "…")
		gap := strings.Repeat(" ", max(inner-2-lipgloss.Width(label)-lipgloss.Width(it.key), 1))
		mark := "  "
		if i == m.cursor {
			mark, label = st.Accent.Render("› "), st.Accent.Bold(true).Render(label)
		}
		lines = append(lines, mark+label+gap+st.Muted.Render(it.key))
	}
	lines = append(lines, "", st.Muted.Render(ansi.Truncate(menuFooter, inner, "…")))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(st.P.Mauve).
		Padding(0, 1).Render(strings.Join(lines, "\n"))
}
