package ui

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/jobs"
	"github.com/dillo/brewery/internal/theme"
)

// maintCols are Maintenance's columns: the task, and how it stands.
var maintCols = []col{{"TASK", 18, 0}, {"STATUS", 0, 12}}

// Maintenance's rows, in the order it lists them.
const (
	maintDoctor = iota
	maintCleanup
	maintAutoremove
	maintServices
	maintUpdate
)

// tidyState is how far an Autoremove or Services cleanup row has got.
type tidyState int

const (
	tidyFailed   tidyState = iota // a query it needs failed
	tidyChecking                  // a query it needs hasn't answered yet
	tidyNone                      // there is nothing to do
	tidySome                      // there is something to do
)

// autoremoveState is where Autoremove stands, failure first, then
// checking.
func (a *App) autoremoveState() tidyState {
	switch {
	case a.errs[qAutoremove] != nil:
		return tidyFailed
	case !a.loaded[qAutoremove]:
		return tidyChecking
	case len(a.snap.Autoremove) == 0:
		return tidyNone
	}
	return tidySome
}

// servicesState is where Services cleanup stands, and the files it would
// remove and the services it would stop.
func (a *App) servicesState() (state tidyState, remove, stop []string) {
	remove, stop = unusedServices(a.snap.ServiceFiles, a.snap.Services)
	switch {
	case a.errs[qServiceFiles] != nil || a.errs[qServices] != nil:
		state = tidyFailed
	case !a.loaded[qServiceFiles] || !a.loaded[qServices]:
		state = tidyChecking
	case len(remove)+len(stop) == 0:
		state = tidyNone
	default:
		state = tidySome
	}
	return state, remove, stop
}

// syncMaint rebuilds Maintenance's rows from what doctor, the
// dry-runs, the service files and History say now.
func (a *App) syncMaint() {
	a.maint.setRows([]table.Row{
		{"Doctor", a.doctorStatus()},
		{"Cleanup", a.cleanupStatus()},
		{"Autoremove", a.autoremoveStatus()},
		{"Services cleanup", a.servicesStatus()},
		{"Update Homebrew", a.updateStatus()},
	})
}

// doctorStatus is the Doctor row's status.
func (a *App) doctorStatus() string {
	st := a.st
	n := len(a.snap.Doctor)
	switch {
	case a.errs[qDoctor] != nil:
		return theme.Tint(st.P.Red, "✗ failed")
	case !a.loaded[qDoctor]:
		return theme.Tint(st.P.Overlay0, "…")
	case a.snap.DoctorRan && n == 0:
		return theme.Tint(st.P.Green, "all clear")
	}
	return theme.Tint(st.P.Yellow, plural(n, "warning"))
}

// cleanupStatus is the Cleanup row's status.
func (a *App) cleanupStatus() string {
	st := a.st
	cl := a.snap.Cleanup
	switch {
	case a.errs[qCleanup] != nil:
		return theme.Tint(st.P.Red, "✗ failed")
	case !a.loaded[qCleanup]:
		return theme.Tint(st.P.Overlay0, "…")
	case cl.Standard.Size == "" && cl.All.Size == "":
		return theme.Tint(st.P.Green, "nothing to clean")
	case cl.Standard.Size != "":
		return theme.Tint(st.P.Blue, "frees "+cl.Standard.Size)
	}
	return theme.Tint(st.P.Blue, "frees "+cl.All.Size+" (whole cache)")
}

// autoremoveStatus is the Autoremove row's status.
func (a *App) autoremoveStatus() string {
	st := a.st
	switch a.autoremoveState() {
	case tidyFailed:
		return theme.Tint(st.P.Red, "✗ failed")
	case tidyChecking:
		return theme.Tint(st.P.Overlay0, "…")
	case tidyNone:
		return theme.Tint(st.P.Green, "nothing to remove")
	}
	return theme.Tint(st.P.Blue, "removes "+formulae(len(a.snap.Autoremove)))
}

// servicesStatus is the Services cleanup row's status.
func (a *App) servicesStatus() string {
	st := a.st
	state, remove, stop := a.servicesState()
	switch state {
	case tidyFailed:
		return theme.Tint(st.P.Red, "✗ failed")
	case tidyChecking:
		return theme.Tint(st.P.Overlay0, "…")
	case tidyNone:
		return theme.Tint(st.P.Green, "nothing to clean")
	}
	var parts []string
	if len(remove) > 0 {
		parts = append(parts, plural(len(remove), "unused file"))
	}
	if len(stop) > 0 {
		parts = append(parts, "stops "+plural(len(stop), "service"))
	}
	return theme.Tint(st.P.Blue, strings.Join(parts, " · "))
}

// updateStatus is the Update Homebrew row's status: when brew update last
// finished.
func (a *App) updateStatus() string {
	if r, ok := a.lastUpdate(); ok {
		return ago(a.now(), r.Ended)
	}
	return theme.Tint(a.st.P.Overlay0, "not yet")
}

// lastUpdate is the newest brew update in History that finished cleanly.
func (a *App) lastUpdate() (jobs.Record, bool) {
	for _, r := range a.history {
		if slices.Equal(r.Args, []string{"update"}) && r.State == jobs.Done {
			return r, true
		}
	}
	return jobs.Record{}, false
}

// UseBrewVersion gives Maintenance Homebrew's version text, such as
// "4.6.3"; "" when it isn't known.
func (a *App) UseBrewVersion(v string) { a.brewVersion = v }

// maintPreview is Maintenance's details for the selected row: what doctor
// found, what each cleanup would free, what autoremove and services cleanup
// would do, or Homebrew's version and its last update, each above what enter
// does.
func (a *App) maintPreview(w, h int) (title, body string) {
	st := a.st
	switch a.maint.cursor() {
	case maintDoctor:
		return "Doctor", strings.TrimRight(doctorBody(st, a.snap, a.errs[qDoctor]), "\n") + "\n\n" + st.Muted.Render("enter runs brew doctor again")
	case maintCleanup:
		cl := a.snap.Cleanup
		switch {
		case a.errs[qCleanup] != nil:
			body = st.Err.Render("couldn't measure the cleanup · r retries")
		case !a.loaded[qCleanup]:
			body = st.Muted.Render("still measuring what cleanup would free")
		case cl.Standard.Size == "" && cl.All.Size == "":
			body = st.OK.Render("nothing to clean up")
		default:
			body = strings.Join(cleanupLines(cl), "\n") + "\n\n" + st.Muted.Render("enter cleans up")
		}
		return "Cleanup", body
	case maintAutoremove:
		switch a.autoremoveState() {
		case tidyFailed:
			body = st.Err.Render("couldn't check · r retries")
		case tidyChecking:
			body = st.Muted.Render("still checking")
		case tidyNone:
			body = st.OK.Render("nothing to remove")
		default:
			body = strings.Join(autoremoveLines(a.snap.Autoremove), "\n") + "\n\n" + st.Muted.Render("enter removes them")
		}
		return "Autoremove", body
	case maintServices:
		state, remove, stop := a.servicesState()
		switch state {
		case tidyFailed:
			body = st.Err.Render("couldn't check · r retries")
		case tidyChecking:
			body = st.Muted.Render("still checking")
		case tidyNone:
			body = st.OK.Render("nothing to clean")
		default:
			body = strings.Join(servicesCleanupLines(remove, stop), "\n") + "\n\n" + st.Muted.Render("enter cleans up")
		}
		return "Services cleanup", body
	case maintUpdate:
		last := "brewery hasn't run brew update"
		if r, ok := a.lastUpdate(); ok {
			last = ago(a.now(), r.Ended) + " · " + r.Ended.In(a.now().Location()).Format("2006-01-02 15:04")
		}
		body = kv(st, "homebrew", cmp.Or(a.brewVersion, "unknown"), "last update", last) +
			"\n" + st.Muted.Render("enter runs brew update")
		return "Update Homebrew", body
	}
	return "Details", ""
}

// maintRun does what the selected Maintenance row offers: doctor again,
// the cleanup, autoremove or services cleanup prompt, or brew update.
func (a *App) maintRun() tea.Cmd {
	switch a.maint.cursor() {
	case maintDoctor:
		return tea.Batch(a.request(qDoctor), a.note("checking with brew doctor…"))
	case maintCleanup:
		return a.askCleanup()
	case maintAutoremove:
		return a.askAutoremove()
	case maintServices:
		return a.askServicesCleanup()
	case maintUpdate:
		return a.brewUpdate()
	}
	return nil
}

// unusedServices is what brew services cleanup would do, by Homebrew's own
// rule: remove each service file whose service launchd hasn't loaded, and
// stop each loaded service whose file is gone. Loaded is what brew calls
// running, a service in error included.
func unusedServices(files []string, svcs []brew.Service) (remove, stop []string) {
	have := make(map[string]bool, len(files))
	for _, f := range files {
		have[f] = true
	}
	loaded := map[string]bool{}
	for _, s := range svcs {
		if !s.Loaded {
			continue
		}
		loaded[s.Label] = true
		if s.File != "" && !have[s.File] {
			stop = append(stop, s.Name)
		}
	}
	for _, f := range files {
		if !loaded[strings.TrimSuffix(filepath.Base(f), ".plist")] {
			remove = append(remove, f)
		}
	}
	return remove, stop
}
