package ui

import (
	"context"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/github"
	"github.com/dillo/brewery/internal/state"
)

// fakeChecker answers tap checks from a table, by owner/repo, and records
// what it was asked.
type fakeChecker struct {
	mu      sync.Mutex
	asked   []string
	answers map[string]github.TapCheck
	errs    map[string]error
}

func (f *fakeChecker) Repo(_ context.Context, owner, repo string) (github.TapCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := owner + "/" + repo
	f.asked = append(f.asked, k)
	if err := f.errs[k]; err != nil {
		return github.TapCheck{}, err
	}
	return f.answers[k], nil
}

// extraTaps join the fixture's anomalyco/tap (on GitHub) and homebrew/core
// (official): one more on GitHub, and one hosted elsewhere.
var extraTaps = []brew.Tap{
	{Name: "old/tap", Remote: "https://github.com/old/homebrew-tap"},
	{Name: "self/hosted", Remote: "https://git.example.com/self/homebrew-hosted.git"},
}

// tapsApp is an app whose taps, in its snapshot and from its source, are
// the fixture's plus extra, and whose checker is fc.
func tapsApp(t *testing.T, fc *fakeChecker, extra ...brew.Tap) *App {
	t.Helper()
	src := newFakeSource(t)
	src.snap.Taps = append(src.snap.Taps, extra...)
	snap := fixtureSnapshot(t)
	snap.Taps = slices.Clone(src.snap.Taps)
	a := New(src, (&fakeStarter{}).start, snap, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	if fc != nil {
		a.UseTapChecker(fc)
	}
	show(t, a, "Maintenance")
	a.maint.table.SetCursor(maintTaps)
	return a
}

// tapsRow is the Check taps row as drawn.
func tapsRow(t *testing.T, a *App) string {
	t.Helper()
	for l := range strings.SplitSeq(stripANSI(a.render()), "\n") {
		// The details' title says Check taps too, on the table's title line.
		if i := strings.Index(l, "Check taps"); i >= 0 && !strings.Contains(l, "Maintenance ·") {
			return l[i:]
		}
	}
	t.Fatalf("no Check taps row:\n%s", stripANSI(a.render()))
	return ""
}

func TestCheckTaps(t *testing.T) {
	fc := &fakeChecker{answers: map[string]github.TapCheck{
		"anomalyco/homebrew-tap": {State: github.Fine},
		"old/homebrew-tap":       {State: github.Archived},
	}}
	a := tapsApp(t, fc, extraTaps...)
	if row := tapsRow(t, a); !regexp.MustCompile(`Check taps\s+not checked yet`).MatchString(row) {
		t.Errorf("before a check: %q", row)
	}
	keys(t, a, "enter")
	if want := []string{"anomalyco/homebrew-tap", "old/homebrew-tap"}; !slices.Equal(fc.asked, want) {
		t.Errorf("asked %v, want %v (not homebrew/core, not self/hosted)", fc.asked, want)
	}
	if row := tapsRow(t, a); !regexp.MustCompile(`Check taps\s+1 archived`).MatchString(row) {
		t.Errorf("after the check: %q", row)
	}
	out := stripANSI(a.render())
	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`checked just now`),
		regexp.MustCompile(`anomalyco/tap\s+fine`),
		regexp.MustCompile(`old/tap\s+archived`),
		regexp.MustCompile(`self/hosted\s+not on GitHub`),
		regexp.MustCompile(`enter checks them on GitHub`),
	} {
		if !want.MatchString(out) {
			t.Errorf("details lack %v:\n%s", want, out)
		}
	}
	saved, err := state.Load(a.cachePath)
	if err != nil || len(saved.TapChecks) != 2 || saved.TapsCheckedAt.IsZero() {
		t.Errorf("the results should be saved: %+v, %v", saved.TapChecks, err)
	}
}

func TestCheckTapsSummaries(t *testing.T) {
	for _, tt := range []struct {
		answers map[string]github.TapCheck
		errs    map[string]error
		want    string
	}{
		{map[string]github.TapCheck{"anomalyco/homebrew-tap": {State: github.Fine}, "old/homebrew-tap": {State: github.Fine}}, nil, "2 fine"},
		{map[string]github.TapCheck{"anomalyco/homebrew-tap": {State: github.Moved, MovedTo: "anomaly/tap"}, "old/homebrew-tap": {State: github.Gone}}, nil, "1 moved · 1 gone"},
		{map[string]github.TapCheck{"anomalyco/homebrew-tap": {State: github.Fine}}, map[string]error{"old/homebrew-tap": context.DeadlineExceeded}, "1 fine · 1 couldn't check"},
		{nil, map[string]error{"anomalyco/homebrew-tap": context.DeadlineExceeded, "old/homebrew-tap": context.DeadlineExceeded}, "2 couldn't check"},
	} {
		a := tapsApp(t, &fakeChecker{answers: tt.answers, errs: tt.errs}, extraTaps...)
		keys(t, a, "enter")
		if row := tapsRow(t, a); !strings.Contains(row, tt.want) {
			t.Errorf("row %q, want %q", row, tt.want)
		}
	}
}

func TestCheckTapsStopsAtTheRateLimit(t *testing.T) {
	fc := &fakeChecker{errs: map[string]error{"anomalyco/homebrew-tap": github.ErrRateLimited}}
	a := tapsApp(t, fc, extraTaps...)
	keys(t, a, "enter")
	if len(fc.asked) != 1 {
		t.Errorf("after the rate limit nothing more is asked: %v", fc.asked)
	}
	if out := stripANSI(a.render()); !regexp.MustCompile(`old/tap\s+couldn't check: rate limit reached`).MatchString(out) {
		t.Errorf("the rest say why:\n%s", out)
	}
}

func TestCheckTapsOnceAtATime(t *testing.T) {
	a := tapsApp(t, &fakeChecker{}, extraTaps...)
	cmd := a.maintRun()
	if cmd == nil || !a.checkingTaps {
		t.Fatal("enter should start a check")
	}
	a.syncMaint() // maintRun was called directly, not through Update
	if row := tapsRow(t, a); !strings.Contains(row, "checking…") {
		t.Errorf("while checking: %q", row)
	}
	if again := a.maintRun(); again != nil || !strings.Contains(stripANSI(a.statusBar()), "already checking taps") {
		t.Errorf("a second enter only notes: %v %q", again, stripANSI(a.statusBar()))
	}
	settle(t, a, cmd)
	if a.checkingTaps {
		t.Error("the check should have finished")
	}
}

func TestCheckTapsWithNothingToCheck(t *testing.T) {
	a := tapsApp(t, &fakeChecker{}) // only anomalyco/tap and homebrew/core
	a.snap.Taps = slices.DeleteFunc(a.snap.Taps, func(t brew.Tap) bool { return !t.Official })
	a.sync()
	a.syncMaint() // as Update would
	if row := tapsRow(t, a); !strings.Contains(row, "no taps to check") {
		t.Errorf("row %q", row)
	}
	keys(t, a, "enter")
	if !strings.Contains(stripANSI(a.statusBar()), "no taps to check") {
		t.Errorf("status %q", stripANSI(a.statusBar()))
	}

	a = tapsApp(t, nil, extraTaps...)
	keys(t, a, "enter")
	if !strings.Contains(stripANSI(a.statusBar()), "checking taps isn't available") {
		t.Errorf("no checker: %q", stripANSI(a.statusBar()))
	}
}

func TestTapsColumnShowsTheCheck(t *testing.T) {
	fc := &fakeChecker{answers: map[string]github.TapCheck{
		"anomalyco/homebrew-tap": {State: github.Fine},
		"old/homebrew-tap":       {State: github.Gone},
	}}
	a := tapsApp(t, fc, extraTaps...)
	keys(t, a, "enter")
	show(t, a, "Taps")
	out := stripANSI(a.render())
	for _, want := range []string{"GITHUB", "fine", "gone"} {
		if !strings.Contains(out, want) {
			t.Errorf("Taps lacks %q:\n%s", want, out)
		}
	}
	if !regexp.MustCompile(`old/tap.*gone`).MatchString(out) {
		t.Errorf("old/tap's row should say gone:\n%s", out)
	}
}
