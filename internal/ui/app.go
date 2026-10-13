package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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

// tabQueries is the query each tab's data comes from. Home shows doctor.
var tabQueries = map[int]query{
	tabHome: qDoctor, tabInstalled: qInstalled, tabOutdated: qOutdated, tabServices: qServices, tabTaps: qTaps,
}

// App is the root Bubble Tea model.
type App struct {
	src       Source
	cachePath string
	st        theme.Styles
	now       func() time.Time
	ctx       context.Context // cancelled on quit, stopping running queries
	cancel    context.CancelFunc

	snap   state.Snapshot
	ix     *state.Index
	errs   map[query]error
	loaded map[query]bool // true once a query has succeeded, here or in the cache

	inflight    map[query]bool // queries running now
	again       map[query]bool // requested while running: run once more when done
	doctorAfter bool           // run doctor when the JSON queries are done
	dirty       bool           // a query succeeded since the last cache save
	saveErr     error

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
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		src: src, cachePath: cachePath, st: st, now: time.Now, ctx: ctx, cancel: cancel,
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
		taps:      newListTab(st, "Taps", tapCols, firstCell, "no taps installed"),
	}
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
		for _, q := range []query{qInstalled, qOutdated, qServices, qTaps} {
			a.loaded[q] = true
		}
	}
	a.loaded[qDoctor] = snap.DoctorRan
	a.sync()
	return a
}

func (a *App) Init() tea.Cmd { return a.refresh() }

// refresh re-runs every query: the JSON ones in parallel, then doctor.
func (a *App) refresh() tea.Cmd {
	a.doctorAfter = true
	return a.request(jsonQueries...)
}

// request starts the given queries. A query that is already running is not
// started twice; it runs once more when the current run returns, so a
// refresh asked for after a change is never lost.
func (a *App) request(qs ...query) tea.Cmd {
	var cmds []tea.Cmd
	for _, q := range qs {
		if a.inflight[q] {
			a.again[q] = true
			continue
		}
		a.inflight[q] = true
		cmds = append(cmds, fetch(a.ctx, a.src, q))
	}
	return tea.Batch(cmds...)
}

// busy reports whether any query is running.
func (a *App) busy() bool {
	for _, running := range a.inflight {
		if running {
			return true
		}
	}
	return false
}

// jsonBusy reports whether any of the four JSON queries is running.
func (a *App) jsonBusy() bool {
	for _, q := range jsonQueries {
		if a.inflight[q] {
			return true
		}
	}
	return false
}

// quit stops running queries and exits.
func (a *App) quit() tea.Cmd {
	a.cancel()
	return tea.Quit
}

// sync pushes the snapshot and loaded state into every tab.
func (a *App) sync() {
	a.installed.loaded = a.loaded[qInstalled]
	a.outdated.loaded = a.loaded[qOutdated]
	a.services.loaded = a.loaded[qServices]
	a.taps.loaded = a.loaded[qTaps]
	a.ix = state.NewIndex(a.snap.Packages)
	a.installed.setData(a.snap.Packages, a.ix)
	a.outdated.setRows(outdatedRows(a.snap.Outdated))
	a.services.setRows(serviceRows(a.snap.Services))
	a.taps.setRows(tapRows(a.snap.Taps))
}

func (a *App) apply(m resultMsg) {
	a.errs[m.q] = m.err
	a.home.doctorErr = a.errs[qDoctor]
	if m.err != nil {
		return // keep showing the last good data
	}
	a.loaded[m.q], a.dirty = true, true
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
		return a, a.handleResult(msg)

	case savedMsg:
		a.saveErr = msg.err
		return a, nil

	case tea.KeyPressMsg:
		return a, a.handleKey(msg)
	}
	return a, nil
}

// handleResult applies one query result and decides what runs next: a
// repeat of the same query, doctor after a full refresh, and a cache save
// once the JSON queries are all back.
func (a *App) handleResult(m resultMsg) tea.Cmd {
	a.inflight[m.q] = false
	if m.err != nil && a.ctx.Err() != nil {
		return nil // quitting: the query was cancelled, not broken
	}
	a.apply(m)
	var again, saving, doctor tea.Cmd
	if a.again[m.q] {
		a.again[m.q] = false
		again = a.request(m.q)
	}
	if a.dirty && !a.jsonBusy() {
		a.dirty = false
		a.snap.FetchedAt = a.now()
		saving = save(a.cachePath, a.snap)
	}
	if a.doctorAfter && !a.jsonBusy() {
		a.doctorAfter = false
		doctor = a.request(qDoctor)
	}
	// Save before doctor starts: doctor can take 30s, and quitting
	// meanwhile must not lose the JSON results.
	return tea.Batch(again, tea.Sequence(saving, doctor))
}

func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch k := msg.String(); k {
	case "q", "ctrl+c":
		return a.quit()
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

// bodyHeight is the space between the tab bar (tabs + rule) and the status bar.
func (a *App) bodyHeight() int { return max(a.height-3, 3) }

// tabBar renders the tabs as solid blocks over a full-width rule. Badges
// are chips: ↑N updates available, ✗N services in error, ⚠ refresh failed.
func (a *App) tabBar() string {
	st := a.st
	chips := map[int]string{}
	if n := len(a.snap.Outdated); n > 0 {
		chips[tabOutdated] = st.ChipWarn.Render(fmt.Sprintf("↑%d", n))
	}
	failing := 0
	for _, s := range a.snap.Services {
		if s.Status == "error" {
			failing++
		}
	}
	if failing > 0 {
		chips[tabServices] = st.ChipErr.Render(fmt.Sprintf("✗%d", failing))
	}
	for tab, q := range tabQueries {
		if a.errs[q] != nil {
			chips[tab] = st.ChipStale.Render("⚠") // stale data outranks any count
		}
	}
	parts := make([]string, len(tabNames))
	for i, name := range tabNames {
		block := st.Tab
		if i == a.tab {
			block = st.TabActive
		}
		parts[i] = block.Render(fmt.Sprintf(" %d %s ", i+1, name))
		if c, ok := chips[i]; ok {
			parts[i] += c + block.Render(" ")
		}
	}
	tabs := ansi.Truncate(strings.Join(parts, " "), a.width, "…")
	return tabs + "\n" + st.TabRule.Render(strings.Repeat("─", a.width))
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
	var failed []string
	for q, name := range queryNames {
		if a.errs[query(q)] != nil {
			failed = append(failed, name)
		}
	}
	switch {
	case a.busy():
		right = append(right, st.Info.Render("refreshing…"))
	case len(failed) > 0:
		right = append(right, st.Err.Render("✗ "+strings.Join(failed, ", ")+" failed"))
	case !a.snap.FetchedAt.IsZero():
		right = append(right, "updated "+a.snap.FetchedAt.Format("15:04"))
	}
	if a.saveErr != nil {
		right = append(right, st.Warn.Render("cache not saved"))
	}
	r := strings.Join(right, "  ")

	// Keep to one line: drop the key hints first, then shorten the left side.
	room := a.width - 1 // trailing space
	hints := st.Muted.Render("1-5 tabs · r refresh · q quit")
	if r != "" {
		hints = r + "  " + hints
	}
	if lipgloss.Width(left)+1+lipgloss.Width(hints) <= room {
		r = hints
	}
	if lipgloss.Width(left)+1+lipgloss.Width(r) > room {
		left = ansi.Truncate(left, max(room-1-lipgloss.Width(r), 0), "…")
	}
	gap := max(room-lipgloss.Width(left)-lipgloss.Width(r), 1)
	line := ansi.Truncate(left+strings.Repeat(" ", gap)+r+" ", a.width, "")
	return st.StatusBar.Width(a.width).Render(line)
}
