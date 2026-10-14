package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/jobs"
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

// mode is how key presses are routed.
type mode int

const (
	modeNormal  mode = iota // keys drive brewery
	modeInsert              // keys go to the running brew command
	modeCommand             // keys edit the ":" command line
)

// App is the root Bubble Tea model.
type App struct {
	src       Source
	start     Starter
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

	queue     jobs.Queue
	acts      map[int]action // what each job was asked to do, by job ID
	proc      Proc           // the current job's process, or nil
	canceled  bool           // the user stopped the current run
	backoff   func(attempt int) time.Duration
	tickEvery time.Duration // spinner speed; 0 turns the spinner off
	ticking   bool
	frame     int

	hist     *jobs.History // where finished jobs are saved, or nil
	history  []jobs.Record // finished jobs, newest first
	histErr  error         // the last save failed
	jobs     listTab       // the ":jobs" view
	jobsOpen bool

	mode    mode
	cmd     cmdline     // the ":" prompt
	confirm *confirmBox // open confirmation, or nil
	flash   string      // one-off status message, cleared by the next key

	comp     brew.Completions // brew's commands, for completing the prompt
	loadComp func(context.Context) (brew.Completions, error)

	tab           int
	width, height int
	tabH          int    // height left for the tab when the drawer is shown
	laidOut       [3]int // width, height and drawer state of the last layout

	// brew asked a question while the command line was open; answer it
	// once the line closes.
	promptPending bool

	drawer    drawer
	home      homeTab
	installed installedTab
	outdated  listTab
	services  listTab
	taps      listTab
}

// New builds the App. src answers queries and start runs brew commands.
// snap is the cached snapshot (possibly empty) so the first frame can show
// data before any query returns.
func New(src Source, start Starter, snap state.Snapshot, cachePath string) *App {
	st := theme.New(theme.Macchiato)
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		src: src, start: start, cachePath: cachePath, st: st, now: time.Now, ctx: ctx, cancel: cancel,
		snap: snap, errs: map[query]error{}, loaded: map[query]bool{}, tab: tabHome,
		inflight: map[query]bool{}, again: map[query]bool{},
		acts: map[int]action{}, backoff: jobs.Backoff, tickEvery: 100 * time.Millisecond,
		drawer:    newDrawer(st),
		cmd:       newCmdline(st),
		installed: newInstalledTab(st),
		outdated:  newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:  newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
		taps:      newListTab(st, "Taps", tapCols, firstCell, "no taps installed"),
		jobs:      newListTab(st, "Jobs", jobCols, jobKey, "no jobs yet"),
	}
	// The history is read before the app starts, so the jobs view is never
	// waiting for it.
	a.jobs.loaded = true
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

func (a *App) Init() tea.Cmd { return tea.Batch(a.refresh(), a.loadCompletions()) }

// UseHistory saves finished jobs to h, starting from the records already
// there (newest first).
func (a *App) UseHistory(h jobs.History, recs []jobs.Record) {
	a.hist, a.history = &h, recs
	a.jobs.setRows(jobRows(recs, a.now()))
}

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

// quit stops running queries and any running job, then exits.
func (a *App) quit() tea.Cmd {
	a.cancel()
	if a.proc != nil {
		a.proc.Kill()
	}
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
	cmd := a.update(msg)
	if a.promptPending && a.mode == modeNormal {
		a.promptPending = false
		if a.proc != nil && a.drawer.prompt() {
			a.mode, a.drawer.open = modeInsert, true
		}
	}
	a.layout()
	return a, cmd
}

func (a *App) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
	case resultMsg:
		return a.handleResult(msg)
	case savedMsg:
		a.saveErr = msg.err
	case historySavedMsg:
		a.histErr = msg.err
	case completionsMsg:
		if msg.err == nil {
			a.comp = msg.comp // without them, only brewery's own commands complete
		}
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	case tea.PasteMsg:
		switch {
		case a.mode == modeInsert && a.proc != nil:
			a.proc.Write([]byte(msg.Content)) // raw: sudo wants no bracketed-paste wrapping
		case a.mode == modeCommand:
			cmd := a.cmd.update(msg)
			a.suggest(false)
			return cmd
		}
	case jobOutputMsg, jobEOFMsg, jobExitMsg, jobRetryMsg, tickMsg:
		return a.handleJob(msg)
	}
	return nil
}

// drawerShown reports whether the drawer takes space under the tab.
func (a *App) drawerShown() bool {
	return a.drawer.open && a.drawer.job != nil && drawerHeight(a.bodyHeight()) > 0
}

// layout sizes the tabs and the drawer's terminal for the current window
// and drawer state. It does nothing if neither has changed.
func (a *App) layout() {
	shown := 0
	if a.drawerShown() {
		shown = 1
	}
	if a.laidOut == [3]int{a.width, a.height, shown} {
		return
	}
	a.laidOut = [3]int{a.width, a.height, shown}
	bodyH := a.bodyHeight()
	dh := drawerHeight(bodyH)
	// The drawer's terminal is always sized for when it is shown, so a job
	// starts at the right size even while the drawer is hidden.
	a.drawer.resize(a.width-4, dh-4)
	if a.proc != nil {
		a.proc.Resize(a.drawer.cols, a.drawer.rows)
	}
	a.tabH = bodyH
	if shown == 1 {
		a.tabH -= dh
	}
	a.installed.resize(a.width, a.tabH)
	for _, t := range []*listTab{&a.outdated, &a.services, &a.taps, &a.jobs} {
		t.resize(a.width, a.tabH)
	}
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

// handleKey routes a key press by mode first: an open confirmation gets
// every key, and in insert mode keys go to brew, so typing an answer can
// never trigger a brewery shortcut such as q.
func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	a.flash = ""
	if a.confirm != nil {
		ch, ok := a.confirm.answer(k)
		if !ok {
			return nil
		}
		a.confirm = nil
		if ch.run == nil {
			return nil
		}
		return ch.run()
	}
	if a.mode == modeInsert {
		if k == "esc" {
			a.mode = modeNormal
		} else if a.proc != nil {
			if k == "ctrl+c" {
				a.canceled = true // brew's death by 0x03 is a stop, not a failure
			}
			a.proc.Write(keyBytes(msg))
		}
		return nil
	}
	if a.mode == modeCommand {
		return a.cmdKey(msg)
	}
	if a.jobsOpen {
		if cmd, ok := a.jobsKey(msg); ok {
			return cmd
		}
	}

	switch k {
	case "q":
		return a.askQuit()
	case ":":
		return a.openCmdline()
	case "ctrl+c":
		if a.queue.Current() != nil {
			return a.askStop()
		}
		return a.quit()
	case "esc":
		a.drawer.open = false
		return nil
	case "o":
		if a.drawer.job != nil {
			a.drawer.open = !a.drawer.open
		}
		return nil
	case "i":
		if a.proc != nil {
			a.mode, a.drawer.open = modeInsert, true
		}
		return nil
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
	if cmd, ok := a.actionKey(k); ok {
		return cmd
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
	bodyH := a.tabH
	var body string
	switch a.tab {
	case tabHome:
		body = a.home.view(a.snap, a.history, a.now(), a.width, bodyH)
	case tabInstalled:
		body = a.installed.view(a.errs[qInstalled])
	case tabOutdated:
		body = a.outdated.view(a.errs[qOutdated])
	case tabServices:
		body = a.services.view(a.errs[qServices])
	case tabTaps:
		body = a.taps.view(a.errs[qTaps])
	}
	if a.jobsOpen {
		body = a.jobs.view(nil)
	}
	if a.confirm != nil {
		body = overlay(body, a.confirm.view(a.st, a.width), a.width, bodyH)
	}
	parts := []string{a.tabBar(), body}
	if a.drawerShown() {
		parts = append(parts, a.drawer.view(a.width, a.bodyHeight()-bodyH, a.now(), a.frame, a.mode == modeInsert))
	}
	screen := lipgloss.JoinVertical(lipgloss.Left, append(parts, a.statusBar())...)
	if p := a.cmd.popup(a.st, a.width); a.mode == modeCommand && p != "" {
		h := lipgloss.Height(screen)
		screen = overlayAt(screen, p, a.width, h, 0, max(h-1-lipgloss.Height(p), 0))
	}
	return screen
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
	if a.mode == modeCommand {
		badge := st.Mode("COMMAND").Render("COMMAND") + " "
		hint := st.Muted.Render("tab complete · ↑↓ history · esc cancel ")
		room := a.width - lipgloss.Width(badge) - 1
		if room-lipgloss.Width(hint) >= 30 {
			room -= lipgloss.Width(hint)
		} else {
			hint = ""
		}
		line := badge + a.cmd.view(room) + hint
		return st.StatusBar.Width(a.width).Render(ansi.Truncate(line, a.width, ""))
	}
	badge := "NORMAL"
	switch {
	case a.mode == modeInsert:
		badge = "INSERT"
	case a.queue.Current() != nil:
		badge = "RUNNING"
	}
	left := st.Mode(badge).Render(badge) + " " + tabNames[a.tab]
	switch {
	case a.jobsOpen:
		left = st.Mode(badge).Render(badge) + " Jobs"
	case a.tab == tabInstalled:
		if p, ok := a.installed.selected(); ok {
			left += " › " + p.Name
		}
	}

	var right []string
	if a.flash != "" {
		right = append(right, a.flash)
	}
	if j := a.queue.Current(); j != nil {
		job := spinnerFrames[a.frame%len(spinnerFrames)] + " " + j.Title
		if j.State == jobs.Waiting {
			job = "⏸ " + j.Title + " · waiting for brew's lock"
		}
		if n := a.queue.Queued(); n > 0 {
			job += fmt.Sprintf(" +%d queued", n)
		}
		right = append(right, st.Info.Render(job))
	}
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
	if a.histErr != nil {
		right = append(right, st.Warn.Render("history not saved"))
	}
	r := strings.Join(right, "  ")

	// Keep to one line: drop the key hints first, then shorten the left side.
	room := a.width - 1 // trailing space
	hints := a.actionHints() + " · : command · q quit"
	if a.drawer.job != nil && !a.drawer.open {
		hints = "o output · " + hints
	}
	hints = st.Muted.Render(hints)
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
