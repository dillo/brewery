package ui

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/jobs"
)

func TestMaintenanceRows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseBrewVersion("4.6.3")
	show(t, a, "Maintenance")
	out := stripANSI(a.render())
	for _, want := range []string{"TOOLS", "Doctor", "Cleanup", "Update Homebrew", "not yet"} {
		if !strings.Contains(out, want) {
			t.Errorf("Maintenance lacks %q:\n%s", want, out)
		}
	}
	if n := len(a.snap.Doctor); n > 0 && !strings.Contains(out, plural(n, "warning")) {
		t.Errorf("the Doctor row counts %d warnings:\n%s", n, out)
	}
	if s := a.snap.Cleanup.Standard.Size; s != "" && !strings.Contains(out, s) {
		t.Errorf("the Cleanup row says what the standard cleanup frees (%s):\n%s", s, out)
	}
}

func TestMaintenanceDetails(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseBrewVersion("4.6.3")
	show(t, a, "Maintenance")
	if w := a.snap.Doctor; len(w) > 0 && !strings.Contains(stripANSI(a.render()), w[0].Title) {
		t.Errorf("Doctor's details show the warnings:\n%s", stripANSI(a.render()))
	}
	if _, body := a.maintPreview(60, 30); !strings.HasSuffix(stripANSI(body), "\n\nenter runs brew doctor again") ||
		strings.Contains(stripANSI(body), "\n\n\nenter runs") {
		t.Errorf("one blank line goes before Doctor's hint:\n%q", stripANSI(body))
	}
	keys(t, a, "j")
	if out := stripANSI(a.render()); !strings.Contains(out, "The standard cleanup frees") {
		t.Errorf("Cleanup's details word the estimates as the prompt does:\n%s", out)
	}
	keys(t, a, "j", "j", "j")
	out := stripANSI(a.render())
	if !strings.Contains(out, "4.6.3") || !strings.Contains(out, "hasn't run brew update") {
		t.Errorf("Update's details: the version, and no update yet:\n%s", out)
	}
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{{Title: "update", Args: []string{"update"},
		State: jobs.Done, Started: a.now().Add(-3 * time.Hour), Ended: a.now().Add(-3 * time.Hour)}})
	a.Update(nil)
	if out := stripANSI(a.render()); !strings.Contains(out, "3h ago") {
		t.Errorf("after a brew update, the row and details say when:\n%s", out)
	}
}

// rowLine is the render's line for a Maintenance row, found by its task name.
func rowLine(t *testing.T, a *App, task string) string {
	t.Helper()
	for l := range strings.SplitSeq(stripANSI(a.render()), "\n") {
		// the render's panes sit side by side: the sidebar, the list, the details
		if panes := strings.Split(l, "││"); len(panes) > 1 && strings.HasPrefix(strings.TrimLeft(panes[1], " │"), task+" ") {
			return panes[1]
		}
	}
	t.Fatalf("no row for %q:\n%s", task, stripANSI(a.render()))
	return ""
}

func TestMaintenanceStates(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.loaded[qDoctor], a.loaded[qCleanup] = false, false
	show(t, a, "Maintenance")
	for _, task := range []string{"Doctor", "Cleanup"} {
		if l := rowLine(t, a, task); !strings.Contains(l, "…") {
			t.Errorf("%s still running reads as loading: %q", task, l)
		}
	}
	a.loaded[qDoctor], a.loaded[qCleanup] = true, true
	a.errs[qDoctor], a.errs[qCleanup] = errors.New("boom"), errors.New("boom")
	a.home.doctorErr = a.errs[qDoctor]
	a.Update(nil)
	for _, task := range []string{"Doctor", "Cleanup"} {
		if l := rowLine(t, a, task); !strings.Contains(l, "✗ failed") {
			t.Errorf("failed %s reads as failed: %q", task, l)
		}
	}
}

func TestMaintenanceRowsWhenClean(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.snap.Doctor, a.snap.DoctorRan = nil, true
	a.loaded[qDoctor], a.loaded[qCleanup] = true, true
	a.snap.Cleanup = brew.Cleanup{}
	show(t, a, "Maintenance")
	if l := rowLine(t, a, "Doctor"); !strings.Contains(l, "all clear") {
		t.Errorf("no warnings reads all clear: %q", l)
	}
	if l := rowLine(t, a, "Cleanup"); !strings.Contains(l, "nothing to clean") {
		t.Errorf("no estimates reads nothing to clean: %q", l)
	}
	a.snap.Cleanup = brew.Cleanup{All: brew.CleanupEstimate{Items: 3, Size: "1.2GB"}}
	a.Update(nil)
	if l := rowLine(t, a, "Cleanup"); !strings.Contains(l, "frees 1.2GB (whole cache)") {
		t.Errorf("only the whole cache freeing says so: %q", l)
	}
}

func TestMaintenanceUpdateDetails(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Maintenance")
	keys(t, a, "j", "j", "j", "j")
	out := stripANSI(a.render())
	if !strings.Contains(out, "unknown") {
		t.Errorf("with no version the details say unknown:\n%s", out)
	}
	ended := a.now().Add(-3 * time.Hour)
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{{Title: "update", Args: []string{"update"},
		State: jobs.Done, Started: ended, Ended: ended}})
	a.Update(nil)
	if out := stripANSI(a.render()); !strings.Contains(out, "3h ago · 2026-10-13 06:30") {
		t.Errorf("after an update the details say how long ago and when:\n%s", out)
	}
}

func TestMaintenanceEnter(t *testing.T) {
	src := newCountingSource(t)
	fs := &fakeStarter{}
	a := New(src, fs.start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	show(t, a, "Maintenance")
	keys(t, a, "enter") // Doctor
	if src.calls["doctor"] != 1 {
		t.Errorf("enter on Doctor runs brew doctor again: %v", src.calls)
	}
	keys(t, a, "j", "enter") // Cleanup
	if a.confirm == nil || a.confirm.title != "Clean up Homebrew?" {
		t.Fatalf("enter on Cleanup asks to clean up: %+v", a.confirm)
	}
	keys(t, a, "n", "j", "j", "j", "enter") // Update Homebrew
	if c := lastCall(fs); strings.Join(c, " ") != "update" {
		t.Errorf("enter on Update Homebrew runs brew update: %v", c)
	}
}

func TestHomeTilesSelectMaintenanceRows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "l", "l", "l", "l", "enter") // the Doctor tile
	if label(a) != "Maintenance" || a.focus != colList || a.maint.cursor() != maintDoctor {
		t.Errorf("Doctor tile: %s, focus %d, row %d", label(a), a.focus, a.maint.cursor())
	}
	keys(t, a, "1", "l", "l", "l", "l", "l", "enter") // the Cleanup tile
	if label(a) != "Maintenance" || a.maint.cursor() != maintCleanup || a.confirm != nil {
		t.Errorf("Cleanup tile: %s, row %d, confirm %v", label(a), a.maint.cursor(), a.confirm)
	}
}

func TestMaintenanceTidyRows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Maintenance")
	if got := rowLine(t, a, "Autoremove"); !strings.Contains(got, "removes 2 formulae") {
		t.Errorf("autoremove row: %q", got)
	}
	if got := rowLine(t, a, "Services cleanup"); !strings.Contains(got, "1 unused file") {
		t.Errorf("services row: %q", got)
	}

	a.snap.Autoremove = []string{"m4"}
	a.snap.ServiceFiles = a.snap.ServiceFiles[1:] // mysql's file is gone: nothing unused
	a.syncMaint()
	if got := rowLine(t, a, "Autoremove"); !strings.Contains(got, "removes 1 formula") {
		t.Errorf("one formula: %q", got)
	}
	if got := rowLine(t, a, "Services cleanup"); !strings.Contains(got, "nothing to clean") {
		t.Errorf("nothing unused: %q", got)
	}

	a.snap.Autoremove = nil
	a.snap.ServiceFiles = a.snap.ServiceFiles[1:] // redis is loaded, but its file is gone
	a.syncMaint()
	if got := rowLine(t, a, "Autoremove"); !strings.Contains(got, "nothing to remove") {
		t.Errorf("nothing to remove: %q", got)
	}
	if got := rowLine(t, a, "Services cleanup"); !strings.Contains(got, "stops 1 service") {
		t.Errorf("a service to stop: %q", got)
	}

	a.errs[qAutoremove], a.loaded[qServiceFiles] = errors.New("boom"), false
	a.syncMaint()
	if got := rowLine(t, a, "Autoremove"); !strings.Contains(got, "✗ failed") {
		t.Errorf("failed: %q", got)
	}
	if got := rowLine(t, a, "Services cleanup"); !strings.Contains(got, "…") {
		t.Errorf("not loaded: %q", got)
	}
	a.errs[qServices] = errors.New("boom")
	a.syncMaint()
	if got := rowLine(t, a, "Services cleanup"); !strings.Contains(got, "✗ failed") {
		t.Errorf("services list failed: %q", got)
	}
}

func TestMaintenanceAutoremove(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Maintenance")
	keys(t, a, "j", "j") // Doctor, Cleanup, Autoremove
	out := stripANSI(a.render())
	for _, want := range []string{"Unneeded formulae", "libyaml", "m4", "enter removes them"} {
		if !strings.Contains(out, want) {
			t.Errorf("details lack %q:\n%s", want, out)
		}
	}
	keys(t, a, "enter")
	if a.confirm == nil || a.confirm.title != "Remove unneeded formulae?" {
		t.Fatalf("enter asks first: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"autoremove"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestMaintenanceServicesCleanup(t *testing.T) {
	t.Setenv("HOME", "/Users/locutus") // the fixtures' home, so their paths shorten to ~
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Maintenance")
	keys(t, a, "j", "j", "j")
	out := stripANSI(a.render())
	for _, want := range []string{"Unused service files in ~/Library/LaunchAgents:", "homebrew.mxcl.mysql.plist", "enter cleans up"} {
		if !strings.Contains(out, want) {
			t.Errorf("details lack %q:\n%s", want, out)
		}
	}
	keys(t, a, "enter")
	if a.confirm == nil || a.confirm.title != "Clean up services?" {
		t.Fatalf("enter asks first: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"services", "cleanup"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestMaintenanceTidyNotes(t *testing.T) {
	for _, tt := range []struct {
		name string
		down []string
		set  func(a *App)
		want string
	}{
		{"autoremove: nothing", []string{"j", "j"}, func(a *App) { a.snap.Autoremove = nil }, "nothing to remove"},
		{"autoremove: checking", []string{"j", "j"}, func(a *App) { a.loaded[qAutoremove] = false }, "still checking"},
		{"autoremove: failed", []string{"j", "j"}, func(a *App) { a.errs[qAutoremove] = errors.New("boom") }, "couldn't check · r retries"},
		{"services: nothing", []string{"j", "j", "j"}, func(a *App) { a.snap.ServiceFiles = a.snap.ServiceFiles[1:] }, "nothing to clean"},
		{"services: checking", []string{"j", "j", "j"}, func(a *App) { a.loaded[qServiceFiles] = false }, "still checking"},
		{"services: failed", []string{"j", "j", "j"}, func(a *App) { a.errs[qServiceFiles] = errors.New("boom") }, "couldn't check · r retries"},
		{"services: list failed", []string{"j", "j", "j"}, func(a *App) { a.errs[qServices] = errors.New("boom") }, "couldn't check · r retries"},
	} {
		a := newJobApp(t, &fakeStarter{})
		show(t, a, "Maintenance")
		keys(t, a, tt.down...)
		tt.set(a)
		a.syncMaint()
		keys(t, a, "enter")
		if a.confirm != nil || !strings.Contains(stripANSI(a.statusBar()), tt.want) {
			t.Errorf("%s: confirm %+v, status %q", tt.name, a.confirm, stripANSI(a.statusBar()))
		}
		if !strings.Contains(stripANSI(a.render()), tt.want) {
			t.Errorf("%s: the details say %q too:\n%s", tt.name, tt.want, stripANSI(a.render()))
		}
	}
}
