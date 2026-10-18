package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// helpGroup is one section of the ? overlay: a heading and its keys.
type helpGroup struct {
	title string
	keys  [][2]string // key, what it does
}

// helpGroups are the keys that work where the user is now: the current
// tab's first, then lists', then everywhere's.
func (a *App) helpGroups() []helpGroup {
	var here helpGroup
	switch {
	case a.focus == colSidebar:
		here = helpGroup{"Sidebar", [][2]string{{"j k", "choose"}, {"gg G", "first, last"}, {"l enter", "go to the list"}}}
	case a.tab == tabHome:
		here = helpGroup{"Home", [][2]string{
			{"h l", "move along the tiles; past the first, sidebar"},
			{"j k", "down to Doctor, back up; j scrolls Doctor"},
			{"l h", "Doctor to Recent jobs and back"},
			{"enter", "open the tile; on Recent jobs, all jobs"},
		}}
	case a.tab == tabInstalled:
		here = helpGroup{"Installed", [][2]string{
			{"u", "upgrade"}, {"x", "uninstall"}, {"p", "pin or unpin"}, {"space", "mark for u/x on several"},
			{"l tab enter", "details, then j/k scroll"},
		}}
	case a.tab == tabOutdated:
		here = helpGroup{"Outdated", [][2]string{{"u", "upgrade"}, {"A", "upgrade all"}, {"space", "mark for u on several"}}}
	case a.tab == tabServices:
		here = helpGroup{"Services", [][2]string{{"s", "start"}, {"S", "stop"}, {"R", "restart"}, {"l tab enter", "details"}, {"j k", "in the details, scroll the log"}, {"L", "follow the log full screen"}}}
	case a.tab == tabTaps:
		here = helpGroup{"Taps", [][2]string{{"x", "untap"}}}
	case a.tab == tabDiscover:
		here = helpGroup{"Discover", [][2]string{
			{"a", "install"}, {"/", "search names and descriptions"},
			{"l tab enter", "details, then j/k scroll"}, {"r", "fetch the analytics again"},
		}}
	case a.tab == tabHistory:
		here = helpGroup{"History", [][2]string{{"l tab enter", "details, then j/k scroll"}, {"R", "run the job again"}}}
	case a.tab == tabMaintenance:
		here = helpGroup{"Maintenance", [][2]string{{"enter", "run the selected task"}, {"l tab", "details"}}}
	}
	groups := []helpGroup{here}
	switch {
	case a.tab == tabHistory && a.focus != colSidebar:
		groups = append(groups, helpGroup{"Lists", [][2]string{
			{"j k", "move"}, {"gg G", "top, bottom"}, {"ctrl+d/u", "half a page"},
			{"/", "filter by command"}, {"/ esc", "clear the filter"}, {"y", "copy the command"},
		}})
	case a.tab == tabMaintenance && a.focus != colSidebar:
		groups = append(groups, helpGroup{"Lists", [][2]string{{"j k", "move"}, {"gg G", "top, bottom"}}})
	case a.tab != tabHome && a.focus != colSidebar:
		groups = append(groups, helpGroup{"Lists", [][2]string{
			{"j k", "move"}, {"gg G", "top, bottom"}, {"ctrl+d/u", "half a page"},
			{"/", "filter by name"}, {"/ esc", "clear the filter"}, {"y", "copy the name"}, {"gx", "open the homepage"},
		}})
	}
	return append(groups, helpGroup{"Everywhere", [][2]string{
		{"h l", "sidebar, list, details"}, {"1-5", "Home, Packages, Manage, Discover, Tools"},
		{"[ ]", "previous or next entry"}, {":", "run any brew command"}, {"r", "refresh"}, {"U", "brew update"},
		{"c", "clean up"}, {"o", "show or hide output"}, {"i", "type into the running job"},
		{"ctrl+c", "stop the running job"}, {"q", "quit"},
	}})
}

// helpKeyW is the width of the key column.
const helpKeyW = 11

// helpView renders the ? overlay in at most maxW×maxH: one column, or two
// side by side (this tab and lists, then everywhere) when one is too tall.
func (a *App) helpView(maxW, maxH int) string {
	st := a.st
	groups := a.helpGroups()
	block := func(gs []helpGroup, w int) []string {
		var lines []string
		for i, g := range gs {
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, st.Accent.Bold(true).Render(g.title))
			for _, k := range g.keys {
				key := st.Title.Render(k[0]) + strings.Repeat(" ", max(helpKeyW-lipgloss.Width(k[0]), 1))
				lines = append(lines, ansi.Truncate(key+k[1], w, "…"))
			}
		}
		return lines
	}
	footer := st.Muted.Render("any key closes this")
	inner := min(56, max(maxW-4, 20))
	lines := append(block(groups, inner), "", footer)
	if len(lines)+2 > maxH && len(groups) > 1 {
		colW := max((maxW-4-3)/2, 20) // two columns and the gap between
		left := block(groups[:len(groups)-1], colW)
		right := append(block(groups[len(groups)-1:], colW), "", footer)
		lines = strings.Split(lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(colW).Render(strings.Join(left, "\n")), "   ",
			strings.Join(right, "\n")), "\n")
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(st.P.Mauve).
		Padding(0, 1).Render(strings.Join(lines, "\n"))
}
