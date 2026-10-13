package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/state"
	"brewery/internal/theme"
)

const (
	tabHome = iota
	tabInstalled
	tabOutdated
	tabServices
	tabTaps
)

var tabNames = [...]string{"Home", "Installed", "Outdated", "Services", "Taps"}

// App is the root Bubble Tea model.
type App struct {
	src       Source
	cachePath string
	st        theme.Styles
	now       func() time.Time

	snap state.Snapshot
	ix   *state.Index
	errs map[query]error

	pending      int  // queries in flight
	doctorQueued bool // doctor already started in this refresh
	saveErr      error

	tab           int
	width, height int

	home      homeTab
	installed installedTab
	outdated  listTab
	services  listTab
	taps      listTab
}

// New builds the App. snap is the cached snapshot (possibly empty) so the
// first frame can show data before any query returns.
func New(src Source, snap state.Snapshot, cachePath string) *App {
	st := theme.New(theme.Macchiato)
	a := &App{
		src: src, cachePath: cachePath, st: st, now: time.Now,
		snap: snap, errs: map[query]error{}, tab: tabHome,
		home:      homeTab{st: st},
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, "no services registered"),
		taps:      newListTab(st, "Taps", tapCols, "no taps installed"),
	}
	a.sync()
	return a
}

func (a *App) Init() tea.Cmd { return a.refresh() }

// refresh starts the four JSON queries in parallel. Doctor follows once
// they have all returned.
func (a *App) refresh() tea.Cmd {
	if a.pending > 0 {
		return nil
	}
	a.pending, a.doctorQueued = 4, false
	return tea.Batch(fetch(a.src, qInstalled), fetch(a.src, qOutdated), fetch(a.src, qServices), fetch(a.src, qTaps))
}

// sync pushes the snapshot into every tab.
func (a *App) sync() {
	a.ix = state.NewIndex(a.snap.Packages)
	a.installed.setData(a.snap.Packages, a.ix)
	a.outdated.setRows(outdatedRows(a.snap.Outdated))
	a.services.setRows(serviceRows(a.snap.Services))
	a.taps.setRows(tapRows(a.snap.Taps))
}

func (a *App) apply(m resultMsg) {
	a.errs[m.q] = m.err
	if m.err != nil {
		return // keep showing the last good data
	}
	switch m.q {
	case qInstalled:
		a.snap.Packages = m.packages
	case qOutdated:
		a.snap.Outdated = m.outdated
	case qServices:
		a.snap.Services = m.services
	case qTaps:
		a.snap.Taps = m.taps
	case qDoctor:
		a.snap.Doctor, a.snap.DoctorRan = m.doctor, true
	}
	a.sync()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		bodyH := a.bodyHeight()
		a.installed.resize(a.width, bodyH)
		for _, t := range []*listTab{&a.outdated, &a.services, &a.taps} {
			t.resize(a.width, bodyH)
		}
		return a, nil

	case resultMsg:
		a.apply(msg)
		a.pending--
		if a.pending > 0 {
			return a, nil
		}
		if !a.doctorQueued {
			a.doctorQueued, a.pending = true, 1
			return a, fetch(a.src, qDoctor)
		}
		a.snap.FetchedAt = a.now()
		return a, save(a.cachePath, a.snap)

	case savedMsg:
		a.saveErr = msg.err
		return a, nil

	case tea.KeyPressMsg:
		return a, a.handleKey(msg)
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch k := msg.String(); k {
	case "q", "ctrl+c":
		return tea.Quit
	case "1", "2", "3", "4", "5":
		a.tab = int(k[0] - '1')
		return nil
	case "]":
		a.tab = (a.tab + 1) % len(tabNames)
		return nil
	case "[":
		a.tab = (a.tab + len(tabNames) - 1) % len(tabNames)
		return nil
	case "r":
		return a.refresh()
	}

	switch a.tab {
	case tabHome:
		if t := a.home.update(msg, a.snap); t >= 0 {
			a.tab = t
		}
	case tabInstalled:
		return a.installed.update(msg)
	case tabOutdated:
		return a.outdated.update(msg)
	case tabServices:
		return a.services.update(msg)
	case tabTaps:
		return a.taps.update(msg)
	}
	return nil
}

func (a *App) View() tea.View {
	v := tea.NewView(a.render())
	v.AltScreen = true
	v.WindowTitle = "brewery"
	return v
}

func (a *App) render() string {
	if a.width == 0 {
		return "starting brewery…"
	}
	bodyH := a.bodyHeight()
	var body string
	switch a.tab {
	case tabHome:
		body = a.home.view(a.snap, a.width, bodyH)
	case tabInstalled:
		body = a.installed.view(a.errs[qInstalled])
	case tabOutdated:
		body = a.outdated.view(a.errs[qOutdated])
	case tabServices:
		body = a.services.view(a.errs[qServices])
	case tabTaps:
		body = a.taps.view(a.errs[qTaps])
	}
	return lipgloss.JoinVertical(lipgloss.Left, a.tabBar(), body, a.statusBar())
}

// bodyHeight is the space between the tab bar and the status bar.
func (a *App) bodyHeight() int { return max(a.height-2, 3) }

func (a *App) tabBar() string {
	st := a.st
	type badge struct {
		text  string
		style lipgloss.Style
	}
	badges := map[int]badge{}
	if n := len(a.snap.Outdated); n > 0 {
		badges[tabOutdated] = badge{fmt.Sprint(n), st.Warn}
	}
	for _, s := range a.snap.Services {
		if s.Status == "error" {
			badges[tabServices] = badge{"!", st.Err}
			break
		}
	}
	parts := make([]string, len(tabNames))
	for i, name := range tabNames {
		label := fmt.Sprintf("%d %s", i+1, name)
		b, hasBadge := badges[i]
		switch {
		case i == a.tab && hasBadge:
			parts[i] = st.TabActive.Render(label + " " + b.text) // badge color would clash with the active background
		case i == a.tab:
			parts[i] = st.TabActive.Render(label)
		case hasBadge:
			parts[i] = st.Tab.Render(label) + b.style.Render(b.text)
		default:
			parts[i] = st.Tab.Render(label)
		}
	}
	return strings.Join(parts, " ")
}

func (a *App) statusBar() string {
	st := a.st
	left := st.Mode("NORMAL").Render("NORMAL") + " " + tabNames[a.tab]
	if a.tab == tabInstalled {
		if p, ok := a.installed.selected(); ok {
			left += " › " + p.Name
		}
	}

	var right []string
	failed := 0
	for _, err := range a.errs {
		if err != nil {
			failed++
		}
	}
	switch {
	case a.pending > 0:
		right = append(right, st.Info.Render("refreshing…"))
	case failed > 0:
		right = append(right, st.Err.Render(fmt.Sprintf("✗ %d queries failed", failed)))
	case !a.snap.FetchedAt.IsZero():
		right = append(right, "updated "+a.snap.FetchedAt.Format("15:04"))
	}
	if a.saveErr != nil {
		right = append(right, st.Warn.Render("cache not saved"))
	}
	right = append(right, st.Muted.Render("1-5 tabs · r refresh · q quit"))
	r := strings.Join(right, "  ")

	gap := max(a.width-lipgloss.Width(left)-lipgloss.Width(r)-2, 1)
	return st.StatusBar.Width(a.width).Render(left + strings.Repeat(" ", gap) + r + " ")
}
