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

// syncMaint rebuilds Maintenance's rows from what doctor, the
// dry-runs, the service files and History say now.
func (a *App) syncMaint() {
	st := a.st
	doctor := theme.Tint(st.P.Overlay0, "…")
	switch n := len(a.snap.Doctor); {
	case a.errs[qDoctor] != nil:
		doctor = theme.Tint(st.P.Red, "✗ failed")
	case !a.loaded[qDoctor]:
	case a.snap.DoctorRan && n == 0:
		doctor = theme.Tint(st.P.Green, "all clear")
	default:
		doctor = theme.Tint(st.P.Yellow, plural(n, "warning"))
	}
	cl := a.snap.Cleanup
	var cleanup string
	switch {
	case a.errs[qCleanup] != nil:
		cleanup = theme.Tint(st.P.Red, "✗ failed")
	case !a.loaded[qCleanup]:
		cleanup = theme.Tint(st.P.Overlay0, "…")
	case cl.Standard.Size == "" && cl.All.Size == "":
		cleanup = theme.Tint(st.P.Green, "nothing to clean")
	case cl.Standard.Size != "":
		cleanup = theme.Tint(st.P.Blue, "frees "+cl.Standard.Size)
	default:
		cleanup = theme.Tint(st.P.Blue, "frees "+cl.All.Size+" (whole cache)")
	}
	autoremove := theme.Tint(st.P.Overlay0, "…")
	switch n := len(a.snap.Autoremove); {
	case a.errs[qAutoremove] != nil:
		autoremove = theme.Tint(st.P.Red, "✗ failed")
	case !a.loaded[qAutoremove]:
	case n == 0:
		autoremove = theme.Tint(st.P.Green, "nothing to remove")
	default:
		autoremove = theme.Tint(st.P.Blue, "removes "+formulae(n))
	}
	services := theme.Tint(st.P.Overlay0, "…")
	remove, stop := unusedServices(a.snap.ServiceFiles, a.snap.Services)
	switch {
	case a.errs[qServiceFiles] != nil || a.errs[qServices] != nil:
		services = theme.Tint(st.P.Red, "✗ failed")
	case !a.loaded[qServiceFiles] || !a.loaded[qServices]:
	case len(remove)+len(stop) == 0:
		services = theme.Tint(st.P.Green, "nothing to clean")
	default:
		var parts []string
		if len(remove) > 0 {
			parts = append(parts, plural(len(remove), "unused file"))
		}
		if len(stop) > 0 {
			parts = append(parts, "stops "+plural(len(stop), "service"))
		}
		services = theme.Tint(st.P.Blue, strings.Join(parts, " · "))
	}
	update := theme.Tint(st.P.Overlay0, "not yet")
	if r, ok := a.lastUpdate(); ok {
		update = ago(a.now(), r.Ended)
	}
	a.maint.setRows([]table.Row{{"Doctor", doctor}, {"Cleanup", cleanup}, {"Autoremove", autoremove},
		{"Services cleanup", services}, {"Update Homebrew", update}})
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
		switch {
		case a.errs[qAutoremove] != nil:
			body = st.Err.Render("couldn't check · r retries")
		case !a.loaded[qAutoremove]:
			body = st.Muted.Render("still checking")
		case len(a.snap.Autoremove) == 0:
			body = st.OK.Render("nothing to remove")
		default:
			body = strings.Join(autoremoveLines(a.snap.Autoremove), "\n") + "\n\n" + st.Muted.Render("enter removes them")
		}
		return "Autoremove", body
	case maintServices:
		remove, stop := unusedServices(a.snap.ServiceFiles, a.snap.Services)
		switch {
		case a.errs[qServiceFiles] != nil || a.errs[qServices] != nil:
			body = st.Err.Render("couldn't check · r retries")
		case !a.loaded[qServiceFiles] || !a.loaded[qServices]:
			body = st.Muted.Render("still checking")
		case len(remove)+len(stop) == 0:
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
