package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/theme"
)

// viewBarHeight is the bar of views plus the line about the active one.
const viewBarHeight = 2

// viewSeg is one view in a tab's bar of views: its name, a shorter name
// for narrow panes, and how many rows it has.
type viewSeg struct {
	name, short string
	n           int
}

// viewBar lists every view with its count, the active one highlighted,
// and under it about, a line saying what the active view shows. Narrow
// panes get the short names.
func viewBar(st theme.Styles, segs []viewSeg, active int, about string, w int) string {
	bar := func(short bool) string {
		parts := make([]string, len(segs))
		for i, v := range segs {
			name, style := v.name, st.View
			if short {
				name = v.short
			}
			if i == active {
				style = st.ViewActive
			}
			parts[i] = style.Render(fmt.Sprintf("%s %d", name, v.n))
		}
		return strings.Join(parts, " ")
	}
	b := bar(false)
	if lipgloss.Width(b) > w {
		b = bar(true)
	}
	// The keys always show, shortened before the description is: on narrow
	// terminals the status bar has no room for hints.
	keys := "f/F to switch"
	if len(about)+2+len(keys) > w {
		keys = "f/F"
		about = ansi.Truncate(about, max(w-2-len(keys), 0), "…")
	}
	gap := max(w-ansi.StringWidth(about)-len(keys), 1)
	return b + "\n" + st.Muted.Render(about+strings.Repeat(" ", gap)+keys)
}
