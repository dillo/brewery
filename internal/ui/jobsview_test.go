package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/dillo/brewery/internal/jobs"
	"github.com/dillo/brewery/internal/theme"
)

// pastJobs are two finished jobs: a failed upgrade an hour ago and a pin
// yesterday.
func pastJobs() []jobs.Record {
	return []jobs.Record{
		{Title: "upgrade fd", Args: []string{"upgrade", "fd"}, State: jobs.Failed, ExitCode: 1,
			Started: testNow.Add(-time.Hour), Ended: testNow.Add(-time.Hour + 75*time.Second),
			Output: "==> Upgrading fd\r\nError: download failed\r\n"},
		{Title: "pin git", Args: []string{"pin", "git"}, State: jobs.Done,
			Started: testNow.Add(-24 * time.Hour), Ended: testNow.Add(-24*time.Hour + 2*time.Second)},
	}
}

func newJobsViewApp(t *testing.T, fs *fakeStarter) *App {
	t.Helper()
	a := newJobApp(t, fs)
	a.UseHistory(jobs.History{Dir: t.TempDir()}, pastJobs())
	return a
}

func TestColonJobsListsThePastJobs(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter")
	if !a.jobsOpen {
		t.Fatal(":jobs should open the jobs view")
	}
	out := stripANSI(a.render())
	for _, want := range []string{"Jobs · 2", "✗ exit 1", "brew upgrade fd", "1:15", "✓ done", "brew pin git", "Oct 12"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "upgrade fd") > strings.Index(out, "pin git") {
		t.Error("the newest job should be listed first")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "Jobs") || !strings.Contains(bar, "enter show output") {
		t.Errorf("status bar: %q", bar)
	} else if !strings.Contains(bar, "q close") || strings.Contains(bar, "q quit") {
		t.Errorf("q closes the jobs view, so the hint should say so: %q", bar)
	}
}

func TestEnterReplaysAJobsOutputInTheDrawer(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	a.openJobs()
	keys(t, a, "enter")
	out := stripANSI(a.render())
	for _, want := range []string{"Error: download failed", "✗ brew upgrade fd · failed, exit 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	keys(t, a, "j", "enter")
	if out := stripANSI(a.render()); !strings.Contains(out, "✓ brew pin git · exit 0") || strings.Contains(out, "download failed") {
		t.Errorf("the drawer should now show pin git alone:\n%s", out)
	}
}

func TestReplayWaitsWhileAJobRuns(t *testing.T) {
	p := newFakeProc(0, "==> Upgrading git\r\n")
	a := newJobsViewApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade git", args: []string{"upgrade", "git"}}))
	a.openJobs()
	keys(t, a, "enter")
	if out := stripANSI(a.render()); strings.Contains(out, "download failed") || !strings.Contains(out, "Upgrading git") {
		t.Errorf("the running job keeps the drawer:\n%s", out)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "upgrade git is running") {
		t.Errorf("status bar should say why: %q", bar)
	}
	p.exit()
	resume(t, a, waiting)
	if out := stripANSI(a.render()); !strings.Contains(out, "Jobs · 3") {
		t.Errorf("the finished job should join the open list:\n%s", out)
	}
}

func TestJobsViewKeys(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobsViewApp(t, fs)
	show(t, a, "Top-level")
	a.openJobs()
	keys(t, a, "u", "x", "p") // Installed actions must not reach the hidden tab
	if len(fs.calls) != 0 || a.confirm != nil {
		t.Errorf("ran %v, confirm %+v", fs.calls, a.confirm)
	}
	keys(t, a, "esc")
	if a.jobsOpen || a.tab != tabInstalled {
		t.Errorf("esc closes the view: open %v, tab %d", a.jobsOpen, a.tab)
	}
	a.openJobs()
	keys(t, a, "3")
	if a.jobsOpen || a.tab != tabServices {
		t.Errorf("a tab key closes the view and switches: open %v, tab %d", a.jobsOpen, a.tab)
	}
	a.openJobs()
	keys(t, a, "q")
	if a.jobsOpen {
		t.Error("q closes the view rather than quitting")
	}
}

func TestEmptyJobsView(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.openJobs()
	if out := stripANSI(a.render()); !strings.Contains(out, "no jobs yet") {
		t.Errorf("empty view:\n%s", out)
	}
	keys(t, a, "enter") // nothing to show
	if a.drawer.job != nil {
		t.Error("enter on an empty list shows nothing")
	}
}

func TestJobRows(t *testing.T) {
	rows := jobRows(testStyles, append(pastJobs(), jobs.Record{Title: "upgrade", Args: []string{"upgrade"}, State: jobs.Canceled,
		Started: testNow.Add(-2 * time.Minute), Ended: testNow.Add(-time.Minute)}), testNow)
	want := [][]string{
		{"✗ exit 1", "brew upgrade fd", "08:30", "1:15"},
		{"✓ done", "brew pin git", "Oct 12 09:30", "0:02"},
		{"stopped", "brew upgrade", "09:28", "1:00"},
	}
	for i, w := range want {
		if stripANSI(strings.Join(rows[i], "|")) != strings.Join(w, "|") {
			t.Errorf("row %d = %q, want %q", i, rows[i], w)
		}
	}
	if rows[0][0] != theme.Tint(testStyles.P.Red, "✗ exit 1") || rows[1][0] != theme.Tint(testStyles.P.Green, "✓ done") {
		t.Errorf("statuses should be colored: %q, %q", rows[0][0], rows[1][0])
	}
}
