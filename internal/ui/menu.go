package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/brew"
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

// hasMenu reports whether tab's rows have an action menu. Maintenance's
// rows are each a single action, so enter runs them instead.
func hasMenu(tab int) bool {
	switch tab {
	case tabInstalled, tabOutdated, tabServices, tabTaps, tabDiscover, tabHistory:
		return true
	}
	return false
}

// openMenu opens the action menu for the current tab's selection, or for
// its marked rows. It does nothing when there's no row to act on.
func (a *App) openMenu() {
	var m *menuBox
	switch a.tab {
	case tabInstalled:
		m = a.installedMenu()
	case tabOutdated:
		m = a.outdatedMenu()
	case tabServices:
		m = a.servicesMenu()
	case tabTaps:
		m = a.tapsMenu()
	case tabDiscover:
		m = a.discoverMenu()
	case tabHistory:
		m = a.historyMenu()
	}
	if m != nil && len(m.items) > 0 {
		a.menu = m
	}
}

// keyItem is a menu item that does what pressing k does on this tab, so
// the menu and the keys always agree.
func (a *App) keyItem(k, label string) menuItem {
	run := func() tea.Cmd {
		cmd, _ := a.actionKey(k)
		return cmd
	}
	switch k {
	case "y":
		run = a.copyName
	case "gx":
		run = a.openHomepage
	}
	return menuItem{k, label, run}
}

// copyItems end every menu: copying the name (or what copyLabel says),
// and opening the web page when there is one.
func (a *App) copyItems(copyLabel string) []menuItem {
	items := []menuItem{a.keyItem("y", copyLabel)}
	if _, url, ok := a.selection(); ok && webPage(url) {
		items = append(items, a.keyItem("gx", "Open the homepage"))
	}
	return items
}

// markedMenu is the menu for n marked rows: only what works on several.
func markedMenu(n int, items ...menuItem) *menuBox {
	return &menuBox{title: fmt.Sprintf("%d marked", n), items: items}
}

// installedMenu is the menu for the selected installed package.
func (a *App) installedMenu() *menuBox {
	if n := a.marks(); n > 0 {
		return markedMenu(n, a.keyItem("u", "Upgrade"), a.keyItem("f", "Fetch"),
			a.keyItem("R", "Reinstall…"), a.keyItem("x", "Uninstall…"))
	}
	p, ok := a.installed.selected()
	if !ok {
		return nil
	}
	var items []menuItem
	if p.Outdated && !p.Pinned {
		items = append(items, a.keyItem("u", "Upgrade to "+p.Latest))
	}
	if p.Outdated {
		items = append(items, a.keyItem("f", "Fetch "+p.Latest))
	}
	items = append(items, a.keyItem("R", "Reinstall…"))
	if p.Kind == brew.KindFormula {
		items = append(items, a.keyItem("L", linkLabel(p)))
	}
	pin := "Pin"
	if p.Pinned {
		pin = "Unpin"
	}
	items = append(items, a.keyItem("p", pin), a.keyItem("x", "Uninstall…"))
	return &menuBox{title: p.Name + " " + p.Version, items: append(items, a.copyItems("Copy the name")...)}
}

// linkLabel is what L does to the formula p; … where it asks first.
func linkLabel(p brew.Package) string {
	switch {
	case p.Linked:
		return "Unlink…"
	case p.KegOnly:
		return "Link…"
	}
	return "Link"
}

// outdatedMenu is the menu for the selected outdated package.
func (a *App) outdatedMenu() *menuBox {
	if n := a.marks(); n > 0 {
		return markedMenu(n, a.keyItem("u", "Upgrade"), a.keyItem("f", "Fetch"))
	}
	i := a.outdated.cursor()
	if i < 0 {
		return nil
	}
	o := a.snap.Outdated[i]
	var items []menuItem
	if !o.Pinned {
		items = append(items, a.keyItem("u", "Upgrade to "+o.Current))
	}
	items = append(items, a.keyItem("f", "Fetch "+o.Current), a.keyItem("A", "Upgrade all…"))
	return &menuBox{title: o.Name + " " + o.Installed, items: append(items, a.copyItems("Copy the name")...)}
}

// servicesMenu is the menu for the selected service.
func (a *App) servicesMenu() *menuBox {
	s, ok := a.selectedService()
	if !ok {
		return nil
	}
	var items []menuItem
	if s.Status == "started" {
		items = append(items, a.keyItem("S", "Stop…"), a.keyItem("R", "Restart"))
	} else {
		items = append(items, a.keyItem("s", "Start"))
	}
	if s.Log != "" || s.ErrorLog != "" {
		items = append(items, a.keyItem("L", "Follow the log"))
	}
	return &menuBox{title: s.Name, items: append(items, a.copyItems("Copy the name")...)}
}

// tapsMenu is the menu for the selected tap.
func (a *App) tapsMenu() *menuBox {
	t, ok := a.selectedTap()
	if !ok {
		return nil
	}
	return &menuBox{title: t.Name, items: append([]menuItem{a.keyItem("x", "Untap…")}, a.copyItems("Copy the name")...)}
}

// discoverMenu is the menu for the selected catalogue entry.
func (a *App) discoverMenu() *menuBox {
	r, ok := a.discover.selected()
	if !ok {
		return nil
	}
	return &menuBox{title: r.Name, items: append([]menuItem{a.keyItem("a", "Install…")}, a.copyItems("Copy the name")...)}
}

// historyMenu is the menu for the selected job.
func (a *App) historyMenu() *menuBox {
	i := a.jobs.cursor()
	if i < 0 {
		return nil
	}
	r := a.history[i]
	var items []menuItem
	if len(r.Args) > 0 {
		items = append(items, a.keyItem("R", "Run again…"))
	}
	return &menuBox{title: strings.TrimSpace("brew " + strings.Join(r.Args, " ")), items: append(items, a.copyItems("Copy the command")...)}
}
