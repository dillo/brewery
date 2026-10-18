package ui

import (
	"context"
	"fmt"
	"maps"
	"os/exec"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/jobs"
	"github.com/dillo/brewery/internal/state"
	"github.com/dillo/brewery/internal/theme"
)

const (
	tabHome = iota
	tabInstalled
	tabOutdated
	tabServices
	tabTaps
	tabDiscover
	tabHistory
	tabMaintenance
	tabCount // how many tabs there are
)

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
	modeFilter              // keys edit the "/" filter
)

// App is the root Bubble Tea model.
type App struct {
	src       Source
	catalog   Catalog   // Discover's catalogue, or nil
	analytics Analytics // Discover's install counts, or nil
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
	doctorAfter bool           // run doctor and cleanup's dry-runs when the JSON queries are done
	// sizesAfterInstalled holds a sizes request made while the package list
	// is being fetched: sizes run once that result is in, so they measure
	// the fresh list, not the one from before the change.
	sizesAfterInstalled bool
	dirty               bool // a query succeeded since the last cache save
	saveErr             error

	queue     jobs.Queue
	acts      map[int]action // what each job was asked to do, by job ID
	proc      Proc           // the current job's process, or nil
	canceled  bool           // the user stopped the current run
	backoff   func(attempt int) time.Duration
	tickEvery time.Duration // spinner speed; 0 turns the spinner off
	ticking   bool
	frame     int
	quitGrace time.Duration // how long a running job gets to stop when brewery quits
	quitting  bool          // brewery is waiting for the running job to stop, then exits

	hist    *jobs.History // where finished jobs are saved, or nil
	history []jobs.Record // finished jobs, newest first
	histErr error         // the last save failed
	jobs    listTab       // History's list

	maint       listTab // Maintenance's rows: doctor, cleanup and brew update
	brewVersion string  // Homebrew's version, for Maintenance; "" when unknown

	detailsDelay time.Duration // how long the Discover cursor rests before brew is asked

	mode       mode
	cmd        cmdline         // the ":" prompt
	filterIn   textinput.Model // the "/" prompt
	confirm    *confirmBox     // open confirmation, or nil
	installing discRow         // the row the open install dialog is for
	help       bool            // the ? overlay is open
	flash      string          // one-off status message, cleared by the next key

	comp     brew.Completions // brew's commands, for completing the prompt
	loadComp func(context.Context) (brew.Completions, error)

	takeoverCmd func(args []string) *exec.Cmd // builds full-screen commands
	takingOver  *jobs.Job                     // the full-screen command running now

	opener    func(url string) error  // opens a web page
	clipboard func(text string) error // copies text
	gPending  bool                    // g was pressed: the next key completes gg or gx

	readLog  func(path string) (string, error) // reads the end of a log; tests swap it
	log      logTail                           // the services preview's log
	pagerCmd func(files []string) *exec.Cmd    // builds the full-screen log viewer

	tab           int
	sel           int             // the sidebar entry shown
	focus         column          // the sidebar, the list or the details
	lastIn        map[section]int // each section's last-used entry, for 1-5
	width, height int
	tabH          int    // height left for the tab when the drawer is shown
	laidOut       [6]int // width, height, drawer state, focus, tab and which tabs have details, at the last layout

	// brew asked a question while the command line was open; answer it
	// once the line closes.
	promptPending bool

	drawer    drawer
	home      homeTab
	installed installedTab
	outdated  listTab
	services  listTab
	taps      listTab
	discover  discoverTab
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
		focus: colSidebar, lastIn: map[section]int{},
		inflight: map[query]bool{}, again: map[query]bool{},
		acts: map[int]action{}, backoff: jobs.Backoff, tickEvery: 100 * time.Millisecond, quitGrace: quitGrace,
		detailsDelay: detailsDelay,
		readLog:      readLogTail,
		drawer:       newDrawer(st),
		cmd:          newCmdline(st),
		filterIn:     newFilterInput(st),
		installed:    newInstalledTab(st),
		outdated:     newListTab(st, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date"),
		services:     newListTab(st, "Services", serviceCols, firstCell, "no services registered"),
		taps:         newListTab(st, "Taps", tapCols, firstCell, "no taps installed"),
		jobs:         newListTab(st, "History", jobCols, jobKey, "no jobs yet"),
		maint:        newListTab(st, "Maintenance", maintCols, firstCell, ""),
		discover:     newDiscoverTab(st),
	}
	// The history is read before the app starts, so History is never
	// waiting for it.
	a.jobs.loaded = true
	a.jobs.matchCol = 1 // jobs are found by their command
	a.services.preview = a.servicePreview
	a.jobs.preview = a.recordPreview
	a.maint.loaded = true // its rows are always there, each saying how it stands
	a.maint.preview = a.maintPreview
	a.outdated.note = a.dataAge
	a.discover.note = a.statsAge
	a.home = homeTab{st: st, errs: a.errs, loaded: a.loaded} // shares the maps, so it stays in sync
	if !snap.FetchedAt.IsZero() || len(snap.Packages) > 0 {
		for _, q := range []query{qInstalled, qOutdated, qServices, qTaps} {
			a.loaded[q] = true
		}
	}
	a.loaded[qDoctor] = snap.DoctorRan
	a.loaded[qCleanup] = snap.CleanupRan
	a.loaded[qSizes] = snap.Sizes != nil
	a.loaded[qAutoremove] = snap.AutoremoveRan
	a.loaded[qServiceFiles] = snap.ServiceFilesRan
	a.sync()
	return a
}

func (a *App) Init() tea.Cmd { return tea.Batch(a.refresh(), a.loadCompletions()) }

// dataAge says how old brew's package data is, which is what Outdated is
// measured against, or "" if that isn't known.
func (a *App) dataAge() string {
	if a.snap.DataAt.IsZero() {
		return ""
	}
	return "brew's data from " + ago(a.now(), a.snap.DataAt)
}

// UseIcons turns Nerd Font icons on or off. They're off until this is
// called, so tests and other callers get plain words.
func (a *App) UseIcons(on bool) {
	ic := theme.Icons{}
	if on {
		ic = theme.Nerd
	}
	// Every part keeps its own copy of the styles.
	for _, st := range []*theme.Styles{&a.st, &a.installed.st, &a.outdated.st, &a.services.st, &a.taps.st, &a.jobs.st, &a.maint.st, &a.home.st, &a.drawer.st, &a.discover.st} {
		st.Icons = ic
	}
	a.sync()
}

// UseHistory saves finished jobs to h, starting from the records already
// there (newest first).
func (a *App) UseHistory(h jobs.History, recs []jobs.Record) {
	a.hist, a.history = &h, recs
	a.jobs.setRows(jobRows(a.st, recs, a.now()))
}

// refresh re-runs every query: the JSON ones in parallel, then doctor and
// cleanup's dry-runs. Discover's catalogue is read again if it was read.
func (a *App) refresh() tea.Cmd {
	a.doctorAfter = true
	return tea.Batch(a.request(jsonQueries...), a.reloadCatalogue())
}

// request starts the given queries. A query that is already running is not
// started twice; it runs once more when the current run returns, so a
// refresh asked for after a change is never lost. Sizes asked for together
// with the package list, or while it is running, wait for it (see
// sizesAfterInstalled).
func (a *App) request(qs ...query) tea.Cmd {
	var cmds []tea.Cmd
	listing := a.inflight[qInstalled] || slices.Contains(qs, qInstalled)
	for _, q := range qs {
		if a.inflight[q] {
			a.again[q] = true
			continue
		}
		if q == qSizes && listing {
			a.sizesAfterInstalled = true
			continue
		}
		a.inflight[q] = true
		var pkgs []brew.Package
		if q == qSizes {
			pkgs = slices.Clone(a.snap.Packages) // measured in the background
		}
		cmds = append(cmds, fetch(a.ctx, a.src, q, pkgs))
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

// quit stops running queries and exits. A running job first gets ctrl+c,
// as it would in a terminal, so brew can clean up after itself; brewery
// exits once the job has stopped and been recorded, or kills it after
// quitGrace.
func (a *App) quit() tea.Cmd {
	a.cancel()
	if a.proc == nil {
		return tea.Quit
	}
	a.quitting, a.canceled = true, true
	_ = a.proc.Interrupt()
	a.flash = a.st.Warn.Render("stopping " + a.queue.Current().Title + "… · ctrl+c quits now")
	return tea.Tick(a.quitGrace, func(time.Time) tea.Msg { return quitNowMsg{} })
}

// quitNow kills a job that hasn't stopped, and exits.
func (a *App) quitNow() tea.Cmd {
	if a.proc != nil {
		_ = a.proc.Kill()
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
	a.installed.sizes = a.snap.Sizes
	a.installed.setData(a.snap.Packages, a.ix)
	a.outdated.setRows(outdatedRows(a.st.Icons, a.snap.Outdated))
	a.services.setRows(serviceRows(a.st, a.snap.Services))
	a.taps.setRows(tapRows(a.snap.Taps))
	a.discover.setHave(a.snap.Packages)
}

func (a *App) apply(m resultMsg) {
	if m.q == qSizes && m.err != nil && len(m.sizes) > 0 {
		// Measuring ran out of time: keep what it measured, and the older
		// sizes for the rest, which the next run measures again.
		merged := make(map[string]int64, len(a.snap.Sizes)+len(m.sizes))
		maps.Copy(merged, a.snap.Sizes)
		maps.Copy(merged, m.sizes)
		m.sizes, m.err = merged, nil
	}
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
		a.snap.Outdated, a.snap.DataAt = m.outdated, m.dataAt
	case qServices:
		a.snap.Services = m.services
		a.log.stale = true // the service may have written more since
	case qTaps:
		a.snap.Taps = m.taps
	case qDoctor:
		a.snap.Doctor, a.snap.DoctorRan = m.doctor, true
	case qCleanup:
		a.snap.Cleanup, a.snap.CleanupRan = m.cleanup, true
	case qAutoremove:
		a.snap.Autoremove, a.snap.AutoremoveRan = m.autoremove, true
	case qServiceFiles:
		a.snap.ServiceFiles, a.snap.ServiceFilesRan = m.serviceFiles, true
	case qSizes:
		a.snap.Sizes = m.sizes
	}
	a.sync()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := a.update(msg)
	if a.tab == tabServices {
		cmd = tea.Batch(cmd, a.wantLog())
	}
	if a.tab == tabDiscover {
		cmd = tea.Batch(cmd, a.wantDetails())
	}
	if a.promptPending && a.mode == modeNormal {
		a.promptPending = false
		if a.proc != nil && a.drawer.prompt() {
			a.typeIntoJob()
		}
	}
	if a.focus == colDetails && !a.hasDetails() {
		a.focus = colList // nothing to show details for
	}
	a.syncMaint()
	a.layout()
	return a, cmd
}

func (a *App) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
	case resultMsg:
		return a.handleResult(msg)
	case catalogueMsg:
		return a.gotCatalogue(msg)
	case cachedStatsMsg:
		return a.gotCachedStats(msg)
	case fetchedStatsMsg:
		return a.gotStats(msg)
	case detailsDueMsg:
		return a.detailsDue(msg)
	case detailsMsg:
		a.gotDetails(msg)
	case savedMsg:
		a.saveErr = msg.err
	case historySavedMsg:
		a.histErr = msg.err
	case takeoverDoneMsg:
		return a.takeoverDone(msg)
	case quitNowMsg:
		return a.quitDue()
	case logMsg:
		a.gotLog(msg)
	case pagerDoneMsg:
		a.pagerDone(msg)
	case openedMsg, copiedMsg:
		a.linkDone(msg)
	case completionsMsg:
		a.gotCompletions(msg)
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	case tea.PasteMsg:
		return a.paste(msg)
	case jobOutputMsg, jobEOFMsg, jobExitMsg, jobRetryMsg, tickMsg:
		return a.handleJob(msg)
	}
	return nil
}

// quitDue quits at once, unless the job has already stopped.
func (a *App) quitDue() tea.Cmd {
	if a.proc == nil {
		return nil // the job has stopped and brewery is already saving it on the way out
	}
	return a.quitNow()
}

// gotCompletions keeps the shell completions that loaded.
func (a *App) gotCompletions(msg completionsMsg) {
	if msg.err == nil {
		a.comp = msg.comp // without them, only brewery's own commands complete
	}
}

// paste sends pasted text to the job in insert mode, or to the prompt
// being typed. Anywhere else it is ignored.
func (a *App) paste(msg tea.PasteMsg) tea.Cmd {
	switch {
	case a.mode == modeInsert && a.proc != nil:
		_, _ = a.proc.Write([]byte(msg.Content)) // raw: sudo wants no bracketed-paste wrapping
	case a.mode == modeCommand:
		cmd := a.cmd.update(msg)
		a.suggest(false)
		return cmd
	case a.mode == modeFilter:
		return a.filterKey(msg)
	}
	return nil
}

// selectedName is the name of the selected row on the tabs that name one
// in the status bar.
func (a *App) selectedName() (string, bool) {
	switch a.tab {
	case tabInstalled:
		p, ok := a.installed.selected()
		return p.Name, ok
	case tabDiscover:
		r, ok := a.discover.selected()
		return r.Name, ok
	case tabServices:
		s, ok := a.selectedService()
		return s.Name, ok
	case tabHistory:
		if i := a.jobs.cursor(); i >= 0 {
			return a.history[i].Title, true
		}
	case tabMaintenance:
		if r := a.maint.table.SelectedRow(); a.maint.n > 0 && r != nil {
			return r[0], true
		}
	}
	return "", false
}

// drawerShown reports whether the drawer takes space under the tab.
func (a *App) drawerShown() bool {
	return a.drawer.open && a.drawer.job != nil && drawerHeight(a.bodyHeight()) > 0
}

// layout sizes the tabs and the drawer's terminal for the current window,
// the focus, the tab and the drawer. It does nothing if
// none of them has changed.
func (a *App) layout() {
	shown := 0
	if a.drawerShown() {
		shown = 1
	}
	withDetails := 0 // which tabs have a details column, one bit each
	for tab := range tabCount {
		if a.tabHasDetails(tab) {
			withDetails |= 1 << tab
		}
	}
	key := [6]int{a.width, a.height, shown, int(a.focus), a.tab, withDetails}
	if a.laidOut == key {
		return
	}
	a.laidOut = key
	bodyH := a.bodyHeight()
	dh := drawerHeight(bodyH)
	// The drawer's terminal is always sized for when it is shown, so a job
	// starts at the right size even while the drawer is hidden.
	a.drawer.resize(a.width-4, dh-4)
	if a.proc != nil {
		_ = a.proc.Resize(a.drawer.cols, a.drawer.rows)
	}
	a.tabH = bodyH
	if shown == 1 {
		a.tabH -= dh
	}
	for tab, resize := range map[int]func(listW, pvW, h int){
		tabInstalled: a.installed.resize, tabOutdated: a.outdated.resize,
		tabServices: a.services.resize, tabTaps: a.taps.resize, tabDiscover: a.discover.resize,
		tabHistory: a.jobs.resize, tabMaintenance: a.maint.resize,
	} {
		c := a.columnsFor(tab)
		resize(c.list, c.details, a.tabH)
	}
}

// columnsFor is how the columns share the window for tab, as the focus
// stands.
func (a *App) columnsFor(tab int) columns {
	return columnsFor(a.width, a.focus, a.tabHasDetails(tab))
}

// handleResult applies one query result and decides what runs next: a
// repeat of the same query, doctor and cleanup's dry-runs after a full
// refresh, and a cache save once the JSON queries are all back.
func (a *App) handleResult(m resultMsg) tea.Cmd {
	a.inflight[m.q] = false
	if m.err != nil && a.ctx.Err() != nil {
		return nil // quitting: the query was cancelled, not broken
	}
	a.apply(m)
	var again, sizes, saving, doctor tea.Cmd
	if a.again[m.q] {
		a.again[m.q] = false
		again = a.request(m.q)
	}
	if m.q == qInstalled && a.sizesAfterInstalled {
		a.sizesAfterInstalled = false // the list is as fresh as it gets, even if the fetch failed
		sizes = a.request(qSizes)
	}
	if a.dirty && !a.jsonBusy() {
		a.dirty = false
		a.snap.FetchedAt = a.now()
		saving = save(a.cachePath, a.snap)
	}
	if a.doctorAfter && !a.jsonBusy() {
		a.doctorAfter = false
		doctor = a.request(qDoctor, qCleanup, qSizes, qAutoremove, qServiceFiles)
	}
	// Save before doctor starts: doctor can take 30s, and quitting
	// meanwhile must not lose the JSON results.
	return tea.Batch(again, sizes, tea.Sequence(saving, doctor))
}

// typeIntoJob switches to INSERT with the drawer open, so keys go to the
// running job. A g pressed just before is forgotten: after INSERT, the next
// key mustn't complete a gg or gx. Help closes, so it can't eat the first
// key of the answer.
func (a *App) typeIntoJob() {
	a.mode, a.drawer.open, a.gPending, a.help = modeInsert, true, false, false
}

// handleKey routes a key press by mode first: an open confirmation gets
// every key, and in insert mode keys go to brew, so typing an answer can
// never trigger a brewery shortcut such as q.
func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if a.quitting {
		if k == "ctrl+c" {
			return a.quitNow()
		}
		return nil // on the way out: nothing else may start
	}
	if a.mode != modeCommand {
		a.flash = "" // in COMMAND the bar shows only the prompt: keep it for when the line closes
	}
	if cmd, ok := a.modalKey(msg); ok {
		return cmd
	}
	if cmd, ok := a.gPrefixKey(k); ok {
		return cmd
	}
	if a.focus == colSidebar {
		if cmd, ok := a.sidebarKey(k); ok {
			return cmd
		}
	} else if a.tab == tabHome {
		if cmd, ok := a.homeKey(msg); ok {
			return cmd
		}
	}
	if cmd, ok := a.globalKey(k); ok {
		return cmd
	}
	if cmd, ok := a.actionKey(k); ok {
		return cmd
	}
	return a.tabKey(msg)
}

// modalKey gives the key to whatever has taken over the keyboard: help,
// an open confirmation, insert mode (brew), or a prompt being typed. ok is
// false when nothing has.
func (a *App) modalKey(msg tea.KeyPressMsg) (cmd tea.Cmd, ok bool) {
	k := msg.String()
	switch {
	case a.help:
		a.help = false // any key closes help, and does nothing else
		return nil, true
	case a.confirm != nil:
		ch, ok := a.confirm.answer(k)
		if !ok {
			return nil, true
		}
		a.confirm = nil
		if ch.run == nil {
			return nil, true
		}
		return ch.run(), true
	case a.mode == modeInsert:
		if k == "esc" {
			a.mode = modeNormal
		} else if a.proc != nil {
			if k == "ctrl+c" {
				a.canceled = true // a ctrl+c the user sends marks the run as stopped, unless brew then finishes cleanly
			}
			_, _ = a.proc.Write(keyBytes(msg))
		}
		return nil, true
	case a.mode == modeCommand:
		return a.cmdKey(msg), true
	case a.mode == modeFilter:
		return a.filterKey(msg), true
	}
	return nil, false
}

// gPrefixKey reads g as the start of gg or gx, and the key after it. ok is
// false for a key that isn't part of one, which then does what it always
// does.
func (a *App) gPrefixKey(k string) (cmd tea.Cmd, ok bool) {
	if a.gPending {
		a.gPending = false
		return a.gKey(k)
	}
	if k == "g" {
		a.gPending = true
		return nil, true
	}
	return nil, false
}

// globalKey handles the keys that do the same on every tab. ok is false
// for any other key.
func (a *App) globalKey(k string) (cmd tea.Cmd, ok bool) {
	switch k {
	case "q":
		return a.askQuit(), true
	case ":":
		return a.openCmdline(), true
	case "/":
		return a.openFilter(), true
	case "?":
		a.help = true
	case "y":
		return a.copyName(), true
	case "ctrl+c":
		if a.queue.Current() != nil {
			a.askStop()
			return nil, true
		}
		return a.quit(), true
	case "esc":
		a.escape()
	case "o":
		if a.drawer.job != nil {
			a.drawer.open = !a.drawer.open
		}
	case "i":
		if a.proc != nil {
			a.typeIntoJob()
		}
	case "1", "2", "3", "4", "5":
		return a.selectSection(section(k[0] - '1')), true
	case "]":
		return a.stepEntry(1), true
	case "[":
		return a.stepEntry(-1), true
	case "h", "left":
		a.focus = max(a.focus-1, colSidebar)
	case "l", "right":
		a.focusDetails()
	case "r":
		if a.tab == tabDiscover {
			return tea.Batch(a.refresh(), a.fetchStats(true)), true
		}
		return a.refresh(), true
	default:
		return nil, false
	}
	return nil, true
}

// escape backs out one step: from the details to the list, then clears
// the marks, then hides the drawer.
func (a *App) escape() {
	switch {
	case a.focus == colDetails:
		a.focus = colList // back to the list first
	case a.marks() > 0 && !a.drawerShown():
		a.clearMarks()
	default:
		a.drawer.open = false
	}
}

// homeKey moves around Home's tiles and panes, which take the arrow keys
// and h, j, k, l that elsewhere move between columns. ok is false for any
// other key.
func (a *App) homeKey(msg tea.KeyPressMsg) (cmd tea.Cmd, ok bool) {
	switch msg.String() {
	case "h", "j", "k", "l", "left", "down", "up", "right", "enter", "ctrl+d", "ctrl+u", "G", "end", "home":
	default:
		return nil, false
	}
	switch t := a.home.update(msg, a.snap); {
	case t >= 0:
		tile := a.home.tiles(a.snap)[t]
		cmd := a.selectEntry(entryIndex(tile.entry))
		a.focus = colList
		if tile.row >= 0 {
			a.maint.table.SetCursor(tile.row)
		}
		return cmd, true
	case t == leaveHome:
		a.focus = colSidebar
	case t == wantHistory:
		return a.showHistory(), true
	}
	return nil, true
}

// tabKey gives the key to the current tab: the list's keys move its
// selection, and once the details have the focus, keys scroll them. tab
// and enter go from the list to the details; tab goes back. On
// Maintenance, enter runs the selected row instead, from either column.
func (a *App) tabKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	switch {
	case a.tab == tabMaintenance && k == "enter":
		return a.maintRun()
	case a.focus == colList && (k == "tab" || k == "enter") && a.hasDetails():
		a.focus = colDetails
		return nil
	case a.focus == colDetails && k == "tab":
		a.focus = colList
		return nil
	case a.focus == colDetails:
		a.scrollDetails(k)
		return nil
	}
	return a.listUpdate(msg)
}

// scrollDetails scrolls the details of the current tab.
func (a *App) scrollDetails(k string) {
	switch a.tab {
	case tabInstalled:
		a.installed.scroll(k)
	case tabDiscover:
		a.discover.scroll(k)
	case tabServices:
		a.scrollLog(k)
	case tabHistory:
		a.jobs.scroll(k)
	case tabMaintenance:
		a.maint.scroll(k)
	}
}

// listUpdate gives the key to the current tab's list.
func (a *App) listUpdate(msg tea.KeyPressMsg) tea.Cmd {
	switch a.tab {
	case tabInstalled:
		return a.installed.update(msg)
	case tabOutdated:
		return a.outdated.update(msg)
	case tabServices:
		return a.services.update(msg)
	case tabTaps:
		return a.taps.update(msg)
	case tabDiscover:
		return a.discover.update(msg)
	case tabHistory:
		return a.jobs.update(msg)
	case tabMaintenance:
		return a.maint.update(msg)
	}
	return nil
}

func (a *App) View() tea.View {
	v := tea.NewView(a.render())
	v.AltScreen = true
	v.WindowTitle = "brewery"
	return v
}

// hasDetails reports whether what's showing has a details column.
func (a *App) hasDetails() bool { return a.tabHasDetails(a.tab) }

// tabHasDetails is hasDetails for tab, but a tab with no row shown has
// none: Discover before its catalogue has loaded, or any list that is
// empty (or filtered to nothing), as there is nothing to show them for.
func (a *App) tabHasDetails(tab int) bool {
	return hasDetails(tab) && a.rowCount(tab) > 0
}

// rowCount is how many rows tab's list shows, from what it already holds.
func (a *App) rowCount(tab int) int {
	switch tab {
	case tabInstalled:
		return len(a.installed.rows)
	case tabDiscover:
		if !a.discover.loaded {
			return 0
		}
		return len(a.discover.rows)
	case tabServices:
		return a.services.n
	case tabHistory:
		return a.jobs.n
	case tabMaintenance:
		return a.maint.n
	}
	return 0
}

// tabBody draws the active tab, or nothing when only the sidebar shows.
func (a *App) tabBody(c columns, bodyH int) string {
	switch {
	case c.list == 0 && c.details == 0: // the sidebar alone
		return ""
	case a.tab == tabHome:
		return a.home.view(a.snap, a.history, a.now(), c.list, bodyH, a.focus)
	case a.tab == tabInstalled:
		return a.installed.view(a.errs[qInstalled], a.focus)
	case a.tab == tabOutdated:
		return a.outdated.view(a.errs[qOutdated], a.focus)
	case a.tab == tabServices:
		return a.services.view(a.errs[qServices], a.focus)
	case a.tab == tabTaps:
		return a.taps.view(a.errs[qTaps], a.focus)
	case a.tab == tabDiscover:
		return a.discover.view(a.focus)
	case a.tab == tabHistory:
		return a.jobs.view(nil, a.focus)
	case a.tab == tabMaintenance:
		return a.maint.view(nil, a.focus)
	}
	return ""
}

func (a *App) render() string {
	if a.width == 0 {
		return "starting brewery…"
	}
	bodyH := a.tabH
	c := a.columnsFor(a.tab)
	body := a.tabBody(c, bodyH)
	if bodyH > 0 && c.side > 0 {
		body = joinColumns(a.sidebarView(c.side, bodyH), body)
	}
	drawer := a.drawerShown()
	if bodyH == 0 && (a.confirm != nil || a.help) {
		// The drawer fills a tiny window, but a question or help must show.
		bodyH, drawer = a.bodyHeight(), false
	}
	body = clipLines(body, bodyH) // a tab too tall for a small window mustn't push the rest off it
	if a.confirm != nil {
		body = overlay(body, a.confirm.view(a.st, a.width), a.width, bodyH)
	}
	if a.help {
		body = overlay(body, a.helpView(a.width, bodyH), a.width, bodyH)
	}
	var parts []string
	if bodyH > 0 {
		parts = append(parts, body) // the drawer takes the whole body: no room for the tab
	}
	if drawer {
		parts = append(parts, a.drawer.view(a.width, a.bodyHeight()-bodyH, a.now(), a.frame, a.mode == modeInsert))
	}
	screen := clipLines(lipgloss.JoinVertical(lipgloss.Left, append(parts, a.statusBar())...), a.height)
	if p := a.cmd.popup(a.st, a.width); a.mode == modeCommand && p != "" {
		h := lipgloss.Height(screen)
		screen = overlayAt(screen, p, a.width, h, 0, max(h-1-lipgloss.Height(p), 0))
	}
	return screen
}

// clipLines keeps at most the first n lines of s.
func clipLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	return strings.Join(lines[:min(len(lines), max(n, 0))], "\n")
}

// bodyHeight is the space above the status bar.
func (a *App) bodyHeight() int { return max(a.height-1, 3) }

// promptBar is the status bar while a line is being typed: the mode's
// badge, the line in what room is left, and a hint when it fits.
func (a *App) promptBar(mode, hint string, line func(w int) string) string {
	st := a.st
	badge := st.Mode(mode).Render(mode) + " "
	hint = st.Muted.Render(hint + " ")
	room := a.width - lipgloss.Width(badge) - 1
	if room-lipgloss.Width(hint) >= 30 {
		room -= lipgloss.Width(hint)
	} else {
		hint = ""
	}
	return st.StatusBar.Width(a.width).Render(ansi.Truncate(badge+line(room)+hint, a.width, ""))
}

// statusLeft is the mode's badge, the open entry and what is selected or
// marked.
func (a *App) statusLeft() string {
	st := a.st
	badge := "NORMAL"
	switch {
	case a.mode == modeInsert:
		badge = "INSERT"
	case a.queue.Current() != nil:
		badge = "RUNNING"
	}
	left := st.Mode(badge).Render(badge) + " " + entries[a.sel].label
	if name, ok := a.selectedName(); ok {
		left += " › " + name
	}
	if n := a.marks(); n > 0 {
		left += fmt.Sprintf(" · %d marked", n)
		if h := a.hiddenMarks(); h > 0 {
			left += fmt.Sprintf(" · %d hidden", h)
		}
	}
	return left
}

// statusRight is the flash, the running job and the state of the data,
// joined for the right of the status bar.
func (a *App) statusRight() string {
	st := a.st
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
	return strings.Join(right, "  ")
}

func (a *App) statusBar() string {
	st := a.st
	switch a.mode {
	case modeCommand:
		return a.promptBar("COMMAND", "tab complete · ↑↓ history · esc cancel", a.cmd.view)
	case modeFilter:
		return a.promptBar("FILTER", "enter keep · esc clear", func(w int) string {
			a.filterIn.SetWidth(max(w-2, 1)) // the prompt and the cursor cell
			return a.filterIn.View()
		})
	}
	left := a.statusLeft()
	r := a.statusRight()

	// Keep to one line: drop the key hints first, then shorten the left side.
	room := a.width - 1 // trailing space
	hints := a.actionHints() + " · : command · ? help · q quit"
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
	line := ansi.Truncate(left+strings.Repeat(" ", gap)+r+" ", a.width, "…") // … says something was cut
	return st.StatusBar.Width(a.width).Render(line)
}
