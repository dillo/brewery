package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// section is a group of sidebar entries; 1-5 jump to them.
type section int

const (
	secHome section = iota
	secPackages
	secManage
	secDiscover
	secTools
)

// sectionNames head each section in the sidebar. Home has no heading.
var sectionNames = [...]string{secPackages: "PACKAGES", secManage: "MANAGE", secDiscover: "DISCOVER", secTools: "TOOLS"}

// entry is one line of the sidebar: the tab it shows and, for Installed
// and Discover, which of the tab's views (an installedFilter or a
// discoverView).
type entry struct {
	label string
	sec   section
	tab   int
	view  int
}

var entries = []entry{
	{"Home", secHome, tabHome, 0},
	{"All", secPackages, tabInstalled, int(filterAll)},
	{"Top-level", secPackages, tabInstalled, int(filterTop)},
	{"Formulae", secPackages, tabInstalled, int(filterFormulae)},
	{"Casks", secPackages, tabInstalled, int(filterCasks)},
	{"Dependencies", secPackages, tabInstalled, int(filterDeps)},
	{"Outdated", secPackages, tabOutdated, 0},
	{"Pinned", secPackages, tabInstalled, int(filterPinned)},
	{"Services", secManage, tabServices, 0},
	{"Taps", secManage, tabTaps, 0},
	{"Popular tools", secDiscover, tabDiscover, int(discTools)},
	{"Popular apps", secDiscover, tabDiscover, int(discApps)},
	{"Rising", secDiscover, tabDiscover, int(discRising)},
	{"New", secDiscover, tabDiscover, int(discNew)},
	{"Fonts", secDiscover, tabDiscover, int(discFonts)},
	{"Search all", secDiscover, tabDiscover, int(discAll)},
	{"History", secTools, tabHistory, 0},
	{"Maintenance", secTools, tabMaintenance, 0},
}

// entryIndex is the index of the entry labelled label. The labels are
// fixed, so an unknown one is a bug.
func entryIndex(label string) int {
	for i, e := range entries {
		if e.label == label {
			return i
		}
	}
	panic("no sidebar entry " + label)
}

// selectEntry shows entry i: its tab, and its view of that tab. The focus
// stays in its column.
func (a *App) selectEntry(i int) tea.Cmd {
	e := entries[i]
	a.sel = i
	a.lastIn[e.sec] = i
	switch e.tab {
	case tabInstalled:
		if f := installedFilter(e.view); a.installed.filter != f {
			a.installed.filter = f
			a.installed.apply() // keeps the selected package if the new view shows it
		}
	case tabDiscover:
		if v := discoverView(e.view); a.discover.showing != v {
			a.discover.showing = v
			a.discover.apply()
		}
	case tabHistory:
		a.jobs.setRows(jobRows(a.st, a.history, a.now())) // a day on, today's jobs show their date
	}
	return a.showTab(e.tab)
}

// selectSection shows the entry last used in section s, or its first.
func (a *App) selectSection(s section) tea.Cmd {
	if i, ok := a.lastIn[s]; ok {
		return a.selectEntry(i)
	}
	for i, e := range entries {
		if e.sec == s {
			return a.selectEntry(i)
		}
	}
	return nil
}

// stepEntry shows the entry d places on, wrapping around.
func (a *App) stepEntry(d int) tea.Cmd {
	n := len(entries)
	return a.selectEntry(((a.sel+d)%n + n) % n)
}

// badge is what an entry shows on its right: a chip when something needs
// attention (chip is true), otherwise a muted count, or nothing before the
// data is in.
func (a *App) badge(e entry) (b string, chip bool) {
	st := a.st
	if q, ok := tabQueries[e.tab]; ok && e.tab != tabHome && a.errs[q] != nil {
		return st.ChipStale.Render("⚠"), true // stale data outranks any count
	}
	n, alert := a.badgeCount(e)
	if alert != "" {
		return alert, true
	}
	if n < 0 {
		return "", false
	}
	return st.Muted.Render(fmt.Sprint(n)), false
}

// badgeCount is the count an entry shows, -1 before its data is in, or
// the chip that replaces the count when something needs attention. When
// chip isn't empty, n means nothing.
func (a *App) badgeCount(e entry) (n int, chip string) {
	st := a.st
	n = -1
	switch e.tab {
	case tabInstalled:
		if a.loaded[qInstalled] {
			n = a.installed.count(installedFilter(e.view))
		}
	case tabOutdated:
		if k := len(a.snap.Outdated); k > 0 {
			return -1, st.ChipWarn.Render(fmt.Sprintf("↑%d", k))
		}
		if a.loaded[qOutdated] {
			n = 0
		}
	case tabServices:
		failing := 0
		for _, s := range a.snap.Services {
			if s.Status == "error" {
				failing++
			}
		}
		if failing > 0 {
			return -1, st.ChipErr.Render(fmt.Sprintf("✗%d", failing))
		}
		if a.loaded[qServices] {
			n = len(a.snap.Services)
		}
	case tabTaps:
		if a.loaded[qTaps] {
			n = len(a.snap.Taps)
		}
	case tabHistory:
		n = len(a.history)
	}
	return n, ""
}

// sidebarW is the sidebar's width, border included: room for
// " Dependencies    1450 " inside it.
const sidebarW = 26

// sidebarView draws the sidebar in a w×h pane. The selected entry is a
// solid block, as the active tab was.
func (a *App) sidebarView(w, h int) string {
	st := a.st
	iw, ih := paneInner(w, h)
	var lines []string
	selLine := 0
	for i, e := range entries {
		if i > 0 && e.sec != entries[i-1].sec {
			lines = append(lines, "", st.Muted.Render(sectionNames[e.sec]))
		}
		b, chip := a.badge(e)
		if i == a.sel {
			b = ansi.Strip(b) // the block's colours carry it
		}
		// A chip's right padding takes the margin, so its text ends in the
		// same column as the counts.
		end := " "
		if chip {
			end = ""
		}
		bw := ansi.StringWidth(b) + len(end)
		label := ansi.Truncate(e.label, max(iw-3-bw, 1), "…")
		gap := max(iw-1-ansi.StringWidth(label)-bw, 1)
		head, tail := " "+label, strings.Repeat(" ", gap)+b+end
		line := head + tail
		if i == a.sel {
			// Only the label is bold, so the count reads apart from it.
			line = st.TabActive.Render(head) + st.TabActive.Bold(false).Render(tail)
			selLine = len(lines)
		}
		lines = append(lines, line)
	}
	if len(lines) > ih && ih > 0 {
		// Too tall for the pane: show the lines that end at the selected
		// one, so it is in view with as much above it as fits. Cutting the
		// bottom instead, as pane would, hides everything below Manage.
		end := max(selLine+1, ih)
		lines = lines[end-ih : end]
	}
	return pane(st, "brewery", strings.Join(lines, "\n"), w, h, a.focus == colSidebar)
}

// sidebarKey handles a key while the sidebar has the focus. ok is false
// for the keys that work the same from any column.
func (a *App) sidebarKey(k string) (cmd tea.Cmd, ok bool) {
	switch k {
	case "j", "down":
		return a.selectEntry(min(a.sel+1, len(entries)-1)), true
	case "k", "up":
		return a.selectEntry(max(a.sel-1, 0)), true
	case "G", "end":
		return a.selectEntry(len(entries) - 1), true
	case "home":
		return a.selectEntry(0), true
	case "l", "right", "enter", "tab":
		a.focus = colList
		if a.tab == tabHome {
			a.home.enter() // Home starts on its first tile, whichever had the focus before
		}
		return nil, true
	case "/":
		if a.filterTarget() != nil {
			a.focus = colList // the filter is the list's: go there to type it
		}
		return nil, false // Home has none: the note says so, and the focus stays
	case "A":
		// Upgrade all needs no row, so on Outdated it works from the
		// sidebar; anywhere else A does nothing, and the sidebar eats it.
		if a.tab == tabOutdated {
			return nil, false
		}
		return nil, true
	case "q", ":", "?", "ctrl+c", "esc", "o", "i", "r", "U", "c", "[", "]", "1", "2", "3", "4", "5":
		return nil, false
	}
	return nil, true // the rest act on a row, and the sidebar has none
}

// focusDetails moves the focus from the list to the details, on tabs
// that have them.
func (a *App) focusDetails() {
	if a.hasDetails() {
		a.focus = colDetails
	}
}
