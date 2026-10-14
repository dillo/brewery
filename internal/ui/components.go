package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/theme"
)

// pane draws a rounded box of exactly w×h cells with a title on its first
// line. The body is clipped to the box before the border goes on, so long
// content never wraps or pushes the bottom border out of view.
func pane(st theme.Styles, title, body string, w, h int, focused bool) string {
	box, ts := st.Pane, st.PaneTitle
	if focused {
		box, ts = st.PaneFocused, st.PaneTitleFocused
	}
	iw, ih := paneInner(w, h)
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) > ih {
		keep := max(ih-1, 0)
		lines = append(lines[:keep], st.Muted.Render(fmt.Sprintf("… %d more", len(lines)-keep)))
	}
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, iw, "…")
	}
	inner := ts.Render(ansi.Truncate(title, iw, "…")) + "\n" + strings.Join(lines, "\n")
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
// single-letter page keys (u, d, b, f) that brewery uses for actions, and
// g: brewery reads gg (top) and gx (homepage) itself.
func tableKeys() table.KeyMap {
	return table.KeyMap{
		LineUp:       key.NewBinding(key.WithKeys("up", "k")),
		LineDown:     key.NewBinding(key.WithKeys("down", "j")),
		PageUp:       key.NewBinding(key.WithKeys("pgup")),
		PageDown:     key.NewBinding(key.WithKeys("pgdown")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d")),
		GotoTop:      key.NewBinding(key.WithKeys("home")), // gg is handled by the app
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

// minFlex is the narrowest a flexible column may get.
const minFlex = 8

// sizeTable fits t into a w×h area, giving flexible columns the leftover
// width. When the columns don't fit, fixed columns shrink from the last one
// down to their title width, then trailing columns are hidden. A hidden
// column keeps its slot with width 0, which the table skips: the table
// indexes columns by row cell, so removing columns would make it panic on
// rows that still carry every cell.
func sizeTable(t *table.Model, cols []col, w, h int) {
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = c.width
	}
	shown := len(cols)
	// used is the width of the shown columns with padding, and how many
	// of them are flexible.
	used := func() (fixed, flex int) {
		for i := range shown {
			fixed += widths[i] + 2 // cell padding
			if cols[i].width == 0 {
				flex++
			}
		}
		return fixed, flex
	}
	over := func() int { f, n := used(); return f + n*minFlex - w }

	for i := len(cols) - 1; i >= 0 && over() > 0; i-- {
		if cols[i].width > 0 {
			widths[i] = max(widths[i]-over(), len(cols[i].title))
		}
	}
	for shown > 1 && over() > 0 {
		shown--
	}

	fixed, flex := used()
	tc := make([]table.Column, len(cols))
	for i, c := range cols {
		width := widths[i]
		switch {
		case i >= shown:
			width = 0
		case c.width == 0:
			width = max((w-fixed)/max(flex, 1), minFlex)
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

// setRowsKeeping replaces the rows and keeps the cursor on the same item,
// as identified by key, so a refresh never moves the selection to a
// different package. If the item is gone, the cursor stays in range.
func setRowsKeeping(t *table.Model, rows []table.Row, key func(table.Row) string) {
	var want string
	if old := t.SelectedRow(); old != nil {
		want = key(old)
	}
	setRows(t, rows)
	if want == "" {
		return
	}
	for i, r := range rows {
		if key(r) == want {
			t.SetCursor(i)
			return
		}
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

// bannerHeight is the number of lines refreshBanner takes.
const bannerHeight = 2

// refreshBanner tops a pane that still shows earlier data after its latest
// refresh failed: the failure, then the error's first line cut to width w.
func refreshBanner(st theme.Styles, err error, w int) string {
	first, _, _ := strings.Cut(err.Error(), "\n")
	return st.Err.Render("✗ refresh failed · r to retry") + "\n" + st.Muted.Render(ansi.Truncate(first, w, "…"))
}
