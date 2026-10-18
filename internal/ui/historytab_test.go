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

// historyApp is newJobApp with two finished jobs in its history, newest first.
func historyApp(t *testing.T) *App {
	t.Helper()
	a := newJobApp(t, &fakeStarter{})
	a.UseHistory(jobs.History{Dir: t.TempDir()}, recentJobs(a.now()))
	return a
}

// recentJobs are two finished jobs, newest first: a failed upgrade a minute
// before now and an update two hours before.
func recentJobs(now time.Time) []jobs.Record {
	return []jobs.Record{
		{Title: "upgrade fd", Args: []string{"upgrade", "fd"}, State: jobs.Failed, ExitCode: 1,
			Started: now.Add(-time.Minute), Ended: now.Add(-time.Minute + 3*time.Second),
			Output: "==> Upgrading fd\r\n\x1b[31mError:\x1b[0m download failed\r\n"},
		{Title: "update", Args: []string{"update"}, State: jobs.Done,
			Started: now.Add(-2 * time.Hour), Ended: now.Add(-2*time.Hour + 4*time.Second),
			Output: "Updated 2 taps.\r\n"},
	}
}

func TestFiveSelectsHistory(t *testing.T) {
	a := historyApp(t)
	keys(t, a, "5")
	if label(a) != "History" || a.tab != tabHistory {
		t.Fatalf("5: %s, tab %d", label(a), a.tab)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "TOOLS") || !strings.Contains(out, "brew upgrade fd") {
		t.Errorf("History shows under TOOLS with its jobs:\n%s", out)
	}
}

func TestHistoryListsThePastJobs(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	show(t, a, "History")
	out := stripANSI(a.render())
	for _, want := range []string{"History · 2", "✗ exit 1", "brew upgrade fd", "1:15", "✓ done", "brew pin git", "Oct 12"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "brew upgrade fd") > strings.Index(out, "brew pin git") {
		t.Error("the newest job should be listed first")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "History › upgrade fd") || !strings.Contains(bar, "l details") {
		t.Errorf("status bar: %q", bar)
	}
}

func TestHistoryDetails(t *testing.T) {
	a := historyApp(t)
	show(t, a, "History")
	keys(t, a, "enter")
	if a.focus != colDetails {
		t.Fatalf("enter on History's list focuses the details: %d", a.focus)
	}
	out := a.render()
	plain := stripANSI(out)
	for _, want := range []string{"brew upgrade fd", "exit 1", "0:03", "Error: download failed"} {
		if !strings.Contains(plain, want) {
			t.Errorf("details lack %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "\r") {
		t.Errorf("details show raw carriage returns:\n%q", plain)
	}
}

func TestColonJobsSelectsHistory(t *testing.T) {
	a := historyApp(t)
	show(t, a, "Taps")
	keys(t, a, ":", "j", "o", "b", "s", "enter")
	if label(a) != "History" || a.focus != colList {
		t.Errorf(":jobs selects History with its list focused: %s, focus %d", label(a), a.focus)
	}
}

func TestHistoryDetailsScroll(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	long := strings.Repeat("line\r\n", 200) + "the end\r\n"
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{{Title: "x", Args: []string{"x"}, State: jobs.Done,
		Started: a.now(), Ended: a.now(), Output: long}})
	show(t, a, "History")
	keys(t, a, "l")
	if strings.Contains(stripANSI(a.render()), "the end") {
		t.Fatal("the end of a long output starts out of view")
	}
	keys(t, a, "G")
	if !strings.Contains(stripANSI(a.render()), "the end") {
		t.Error("G scrolls the details to the end of the output")
	}
}

func TestPlainOutput(t *testing.T) {
	for in, want := range map[string]string{
		"a\r\nb\r\n":                 "a\nb\n",
		"\x1b[1mbold\x1b[0m\n":       "bold\n",
		"10%\r50%\r100%\r\ndone\r\n": "100%\ndone\n",
		"trailing\r\n":               "trailing\n",
	} {
		if got := plainOutput(in); got != want {
			t.Errorf("plainOutput(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFinishedJobJoinsHistory(t *testing.T) {
	p := newFakeProc(0, "==> Upgrading git\r\n")
	a := newJobsViewApp(t, &fakeStarter{procs: []*fakeProc{p}})
	show(t, a, "History")
	waiting := settle(t, a, a.run(action{title: "upgrade git", args: []string{"upgrade", "git"}}))
	p.exit()
	resume(t, a, waiting)
	if out := stripANSI(a.render()); !strings.Contains(out, "History · 3") {
		t.Errorf("the finished job should join the list on show:\n%s", out)
	}
}

func TestHistoryIgnoresPackageKeys(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobsViewApp(t, fs)
	show(t, a, "History")
	keys(t, a, "u", "x", "p", "space") // Installed's actions have no package here
	if len(fs.calls) != 0 || a.confirm != nil {
		t.Errorf("ran %v, confirm %+v", fs.calls, a.confirm)
	}
}

func TestEmptyJobsView(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "History")
	if out := stripANSI(a.render()); !strings.Contains(out, "no jobs yet") {
		t.Errorf("empty History:\n%s", out)
	}
	keys(t, a, "enter") // no job, so empty details
	if a.focus != colDetails {
		t.Errorf("enter on an empty History focuses the details: %d", a.focus)
	}
	if title, body := a.recordPreview(60, 30); title != "Details" || body != "" {
		t.Errorf("an empty History's details are empty: %q, %q", title, body)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "no jobs yet") { // and the render doesn't panic
		t.Errorf("empty History after enter:\n%s", out)
	}
}

func TestHistoryWrapsLongLines(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	long := strings.Repeat("word ", 60) + "LASTWORD\r\n"
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{{Title: "x", Args: []string{"x"}, State: jobs.Failed,
		ExitCode: 1, Started: a.now(), Ended: a.now(), Output: long}})
	show(t, a, "History")
	if out := stripANSI(a.render()); !strings.Contains(out, "LASTWORD") {
		t.Errorf("a line longer than the pane wraps, so its end shows:\n%s", out)
	}
}

func TestHistoryDatesJobsAfterMidnight(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{{Title: "pin git", Args: []string{"pin", "git"}, State: jobs.Done,
		Started: testNow.Add(-time.Hour), Ended: testNow.Add(-time.Hour)}})
	show(t, a, "History")
	if out := stripANSI(a.render()); strings.Contains(out, "Oct 13 08:30") || !strings.Contains(out, "08:30") {
		t.Fatalf("today's job shows the clock alone:\n%s", out)
	}
	show(t, a, "Taps")
	a.now = func() time.Time { return testNow.Add(24 * time.Hour) }
	show(t, a, "History")
	if out := stripANSI(a.render()); !strings.Contains(out, "Oct 13 08:30") {
		t.Errorf("a day on, the job shows its date:\n%s", out)
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
