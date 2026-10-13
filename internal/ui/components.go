package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"brewery/internal/theme"
)

// pane draws a rounded box of exactly w×h cells with a title on its first
// line. Content beyond the box is cut off, not wrapped into the next pane.
func pane(st theme.Styles, title, body string, w, h int, focused bool) string {
	box, ts := st.Pane, st.PaneTitle
	if focused {
		box, ts = st.PaneFocused, st.PaneTitleFocused
	}
	inner := ts.Render(title) + "\n" + body
	return box.Width(w).Height(h).MaxWidth(w).MaxHeight(h).Render(inner)
}

// paneInner returns the content area of a pane of size w×h, excluding the
// border, horizontal padding and title line.
func paneInner(w, h int) (int, int) {
	return max(w-4, 0), max(h-3, 0)
}

// col describes a table column. Width 0 means "take the remaining space".
type col struct {
	title string
	width int
}

// tableKeys is the table's built-in Vim-style navigation, minus the default
// single-letter page keys (u, d, b, f) that brewery uses for actions.
func tableKeys() table.KeyMap {
	return table.KeyMap{
		LineUp:       key.NewBinding(key.WithKeys("up", "k")),
		LineDown:     key.NewBinding(key.WithKeys("down", "j")),
		PageUp:       key.NewBinding(key.WithKeys("pgup")),
		PageDown:     key.NewBinding(key.WithKeys("pgdown")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d")),
		GotoTop:      key.NewBinding(key.WithKeys("home", "g")),
		GotoBottom:   key.NewBinding(key.WithKeys("end", "G")),
	}
}

// newTable creates a table with its columns already set: the table panics
// if rows arrive before columns. sizeTable sets the real widths at render.
func newTable(st theme.Styles, cols []col) table.Model {
	tc := make([]table.Column, len(cols))
	for i, c := range cols {
		tc[i] = table.Column{Title: c.title, Width: max(c.width, len(c.title))}
	}
	return table.New(
		table.WithColumns(tc),
		table.WithFocused(true),
		table.WithKeyMap(tableKeys()),
		table.WithStyles(table.Styles{Header: st.TableHeader, Cell: st.TableCell, Selected: st.TableSelected}),
	)
}

// sizeTable fits t into a w×h area, giving flexible columns the leftover width.
func sizeTable(t *table.Model, cols []col, w, h int) {
	fixed, flex := 0, 0
	for _, c := range cols {
		fixed += c.width + 2 // cell padding
		if c.width == 0 {
			flex++
		}
	}
	tc := make([]table.Column, len(cols))
	for i, c := range cols {
		width := c.width
		if width == 0 {
			width = max((w-fixed)/max(flex, 1), 8)
		}
		tc[i] = table.Column{Title: c.title, Width: width}
	}
	t.SetColumns(tc)
	t.SetWidth(w)
	t.SetHeight(h)
}

// setRows replaces the table rows and keeps the cursor inside the new range.
// An empty table parks its cursor at -1, so bring it back once rows arrive.
func setRows(t *table.Model, rows []table.Row) {
	t.SetRows(rows)
	if c := t.Cursor(); c < 0 || c >= len(rows) {
		t.SetCursor(max(min(c, len(rows)-1), 0))
	}
}

// kv renders aligned "label  value" lines for preview panes.
func kv(st theme.Styles, pairs ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			continue
		}
		b.WriteString(st.Muted.Render(lipgloss.NewStyle().Width(11).Render(pairs[i])))
		b.WriteString(pairs[i+1])
		b.WriteString("\n")
	}
	return b.String()
}

// errorBody explains a failed query inside the pane that needed its data.
func errorBody(st theme.Styles, err error) string {
	return st.Err.Render("✗ couldn't load this view") + "\n\n" +
		st.Muted.Render(err.Error()) + "\n\n" +
		st.Muted.Render("press r to retry")
}
