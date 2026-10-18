package ui

import "charm.land/lipgloss/v2"

// column is where the focus is: the sidebar, the list, or the details of
// the list's selected row.
type column int

const (
	colSidebar column = iota
	colList
	colDetails
)

// The narrowest the list and the details may be. Home's content takes the
// list's place, and its minimum.
const (
	listMinW    = 60
	detailsMinW = 40
)

// columns is how wide each column is across the body; 0 means it isn't
// shown.
type columns struct{ side, list, details int }

// columnsFor lays out the columns in a window w wide with the focus on
// focus. hasDetails is false for a tab with nothing beside its list, which
// takes a details focus as the list's. The focused column always shows;
// then the column to its right joins if the minimum widths still fit, then
// the one to its left, until neither does. The sidebar keeps sidebarW, and
// the list and the details share the rest 55/45.
func columnsFor(w int, focus column, hasDetails bool) columns {
	mins := [...]int{colSidebar: sidebarW, colList: listMinW, colDetails: detailsMinW}
	last := colDetails
	if !hasDetails {
		last, focus = colList, min(focus, colList)
	}
	lo, hi, used := focus, focus, mins[focus]
	for grew := true; grew; {
		grew = false
		if hi < last && used+mins[hi+1] <= w {
			hi, used, grew = hi+1, used+mins[hi+1], true
		}
		if lo > colSidebar && used+mins[lo-1] <= w {
			lo, used, grew = lo-1, used+mins[lo-1], true
		}
	}
	if hi == colSidebar { // lo <= hi, so this is lo == hi == colSidebar
		return columns{side: w} // the sidebar alone fills the window
	}
	var c columns
	rest := w
	if lo == colSidebar {
		c.side, rest = sidebarW, w-sidebarW
	}
	switch {
	case lo <= colList && hi == colDetails:
		c.list = rest * 55 / 100
		c.details = rest - c.list
	case hi == colDetails:
		c.details = rest
	default:
		c.list = rest
	}
	return c
}

// hasDetails reports whether tab shows the selected row's details beside
// its list.
func hasDetails(tab int) bool {
	return tab == tabInstalled || tab == tabDiscover || tab == tabServices
}

// joinColumns puts the shown columns side by side.
func joinColumns(panes ...string) string {
	var shown []string
	for _, p := range panes {
		if p != "" {
			shown = append(shown, p)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, shown...)
}
