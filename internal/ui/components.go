package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/theme"
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
// min is how far it gives way when the table is too narrow: a flexible
// column keeps min (minFlex if 0) while fixed columns shrink, and a fixed
// column shrinks no further than min (its title's width if 0). A fixed
// column with a negative min is optional: it never shrinks, and is hidden
// before a flexible column has to give way.
type col struct {
	title      string
	width, min int
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

// sizeTable fits t into a w×h area. When the columns don't fit, fixed
// columns give way first, a cell at a time from whichever has the most to
// spare (so a long VERSION shrinks before a short TYPE); then optional
// columns are hidden, then flexible columns shrink down to minFlex, and
// then trailing columns are hidden. Flexible
// columns share whatever width is left. A hidden column keeps its slot with
// width 0, which the table skips: the table indexes columns by row cell, so
// removing columns would make it panic on rows that still carry every cell.
func sizeTable(t *table.Model, cols []col, w, h int) {
	widths := make([]int, len(cols))
	floor := make([]int, len(cols))
	for i, c := range cols {
		switch {
		case c.width == 0:
			widths[i], floor[i] = max(c.min, minFlex), minFlex
		case c.min < 0:
			widths[i], floor[i] = c.width, c.width // optional: shown whole or not at all
		case c.min > 0:
			widths[i], floor[i] = c.width, min(c.min, c.width)
		default:
			widths[i], floor[i] = c.width, min(len(c.title), c.width)
		}
	}
	shown := len(cols)
	hidden := make([]bool, len(cols))
	// over is how much wider than w the shown columns are, with padding.
	over := func() int {
		n := 0
		for i := range shown {
			if !hidden[i] {
				n += widths[i] + 2 // cell padding
			}
		}
		return n - w
	}
	for _, flexible := range []bool{false, true} {
		if flexible {
			for i := len(cols) - 1; i >= 0 && over() > 0; i-- {
				hidden[i] = hidden[i] || cols[i].min < 0
			}
		}
		for over() > 0 {
			give := -1
			for i := range shown {
				if (cols[i].width == 0) == flexible && widths[i] > floor[i] &&
					(give < 0 || widths[i]-floor[i] > widths[give]-floor[give]) {
					give = i
				}
			}
			if give < 0 {
				break
			}
			widths[give]--
		}
	}
	for shown > 1 && over() > 0 {
		shown--
	}

	flex := 0
	for i := range shown {
		if cols[i].width == 0 {
			flex++
		}
	}
	spare := max(-over(), 0)
	tc := make([]table.Column, len(cols))
	for i, c := range cols {
		width := widths[i]
		switch {
		case i >= shown || hidden[i]:
			width = 0
		case c.width == 0:
			width += spare / flex
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

// humanSize formats bytes the way brew does: 1024-based, one decimal,
// "KB", "MB" and "GB".
func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fGB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%dB", n)
}

// skeletonRows is how many placeholder rows a list shows at most.
const skeletonRows = 8

// skeleton is n placeholder rows, at most w wide, for a list whose first
// results haven't come yet: the shape of what's coming, rather than a blank.
func skeleton(st theme.Styles, w, n int) string {
	lines := make([]string, max(0, min(n, skeletonRows)))
	for i := range lines {
		row := strings.Repeat("░", 8+i*5%7) + "   " + strings.Repeat("░", 6+i*3%4) + "   " + strings.Repeat("░", 4)
		lines[i] = st.Muted.Render(ansi.Truncate(" "+row, w, ""))
	}
	return strings.Join(lines, "\n")
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
