package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// section is a group of sidebar entries; 1-4 jump to them.
type section int

const (
	secHome section = iota
	secPackages
	secManage
	secDiscover
)

// sectionNames head each section in the sidebar. Home has no heading.
var sectionNames = [...]string{secPackages: "PACKAGES", secManage: "MANAGE", secDiscover: "DISCOVER"}

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
// attention, otherwise a muted count, or nothing before the data is in.
func (a *App) badge(e entry) string {
	st := a.st
	if q, ok := tabQueries[e.tab]; ok && e.tab != tabHome && a.errs[q] != nil {
		return st.ChipStale.Render("⚠") // stale data outranks any count
	}
	n := -1
	switch e.tab {
	case tabInstalled:
		if a.loaded[qInstalled] {
			n = a.installed.count(installedFilter(e.view))
		}
	case tabOutdated:
		if k := len(a.snap.Outdated); k > 0 {
			return st.ChipWarn.Render(fmt.Sprintf("↑%d", k))
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
			return st.ChipErr.Render(fmt.Sprintf("✗%d", failing))
		}
		if a.loaded[qServices] {
			n = len(a.snap.Services)
		}
	case tabTaps:
		if a.loaded[qTaps] {
			n = len(a.snap.Taps)
		}
	}
	if n < 0 {
		return ""
	}
	return st.Muted.Render(fmt.Sprint(n))
}

// sidebarW is the sidebar's width, border included: room for
// " Dependencies  145 " inside it.
const sidebarW = 22

// sidebarView draws the sidebar in a w×h pane. The selected entry is a
// solid block, as the active tab was.
func (a *App) sidebarView(w, h int) string {
	st := a.st
	iw, _ := paneInner(w, h)
	var lines []string
	for i, e := range entries {
		if i > 0 && e.sec != entries[i-1].sec {
			lines = append(lines, "", st.Muted.Render(sectionNames[e.sec]))
		}
		b := a.badge(e)
		if i == a.sel {
			b = ansi.Strip(b) // the block's colours carry it
		}
		label := ansi.Truncate(e.label, max(iw-3-ansi.StringWidth(b), 1), "…")
		gap := max(iw-2-ansi.StringWidth(label)-ansi.StringWidth(b), 1)
		line := " " + label + strings.Repeat(" ", gap) + b + " "
		if i == a.sel {
			line = st.TabActive.Render(line)
		}
		lines = append(lines, line)
	}
	return pane(st, "brewery", strings.Join(lines, "\n"), w, h, a.sideFocus)
}
