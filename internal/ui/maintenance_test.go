package ui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

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
	keys(t, a, "j")
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

func TestMaintenanceStates(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.loaded[qDoctor], a.loaded[qCleanup] = false, false
	show(t, a, "Maintenance")
	if out := stripANSI(a.render()); strings.Count(out, "…") < 2 {
		t.Errorf("doctor and cleanup still running read as loading:\n%s", out)
	}
	a.loaded[qDoctor], a.loaded[qCleanup] = true, true
	a.errs[qDoctor], a.errs[qCleanup] = errors.New("boom"), errors.New("boom")
	a.home.doctorErr = a.errs[qDoctor]
	a.Update(nil)
	if out := stripANSI(a.render()); strings.Count(out, "✗ failed") < 2 {
		t.Errorf("failed doctor and cleanup read as failed:\n%s", out)
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
	keys(t, a, "n", "j", "enter") // Update Homebrew
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
