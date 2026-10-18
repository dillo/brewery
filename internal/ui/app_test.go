package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/state"
)

// fakeSource serves the fixture snapshot. servicesErr, doctorErr and
// cleanupErr make one query fail.
type fakeSource struct {
	snap        state.Snapshot
	servicesErr error
	doctorErr   error
	cleanupErr  error
}

func newFakeSource(t *testing.T) *fakeSource { return &fakeSource{snap: fixtureSnapshot(t)} }

func (f *fakeSource) Installed(context.Context) ([]brew.Package, error) { return f.snap.Packages, nil }
func (f *fakeSource) Outdated(context.Context) ([]brew.Outdated, error) { return f.snap.Outdated, nil }
func (f *fakeSource) Taps(context.Context) ([]brew.Tap, error)          { return f.snap.Taps, nil }
func (f *fakeSource) Doctor(context.Context) ([]brew.DoctorWarning, error) {
	if f.doctorErr != nil {
		return nil, f.doctorErr
	}
	return f.snap.Doctor, nil
}
func (f *fakeSource) DataUpdated(context.Context) (time.Time, error) {
	return testNow.Add(-6 * time.Hour), nil
}
func (f *fakeSource) Sizes(context.Context, []brew.Package) (map[string]int64, error) {
	return f.snap.Sizes, nil
}
func (f *fakeSource) Cleanup(context.Context) (brew.Cleanup, error) {
	if f.cleanupErr != nil {
		return brew.Cleanup{}, f.cleanupErr
	}
	return f.snap.Cleanup, nil
}
func (f *fakeSource) Services(context.Context) ([]brew.Service, error) {
	if f.servicesErr != nil {
		return nil, f.servicesErr
	}
	return f.snap.Services, nil
}

// drain runs cmd and every command it produces, feeding the messages back
// into the app the way the Bubble Tea runtime would. A tea.Sequence (an
// unexported slice of commands) runs in order, each to completion.
func drain(t *testing.T, a *App, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeFor[tea.Cmd]() {
		for i := range v.Len() {
			drain(t, a, v.Index(i).Interface().(tea.Cmd))
		}
		return
	}
	if msg == nil {
		return
	}
	_, next := a.Update(msg)
	drain(t, a, next)
}

var testNow = time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC)

func newTestApp(t *testing.T, src Source) *App {
	t.Helper()
	a := New(src, nil, state.Snapshot{}, filepath.Join(t.TempDir(), "snapshot.json"))
	a.now = func() time.Time { return testNow }
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return a
}

func TestRefreshLoadsEverythingThenDoctorThenSaves(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())

	if a.busy() {
		t.Fatalf("queries still in flight after refresh: %v", a.inflight)
	}
	if len(a.snap.Packages) != 7 || len(a.snap.Outdated) != 2 || len(a.snap.Services) != 2 || len(a.snap.Taps) != 2 {
		t.Errorf("snapshot incomplete: %d pkgs, %d outdated, %d services, %d taps",
			len(a.snap.Packages), len(a.snap.Outdated), len(a.snap.Services), len(a.snap.Taps))
	}
	if !a.snap.DoctorRan || len(a.snap.Doctor) != 2 {
		t.Errorf("doctor not applied: ran=%v warnings=%d", a.snap.DoctorRan, len(a.snap.Doctor))
	}
	if !a.snap.CleanupRan || a.snap.Cleanup.All.Size != "86.7MB" {
		t.Errorf("cleanup's dry-runs not applied: ran=%v %+v", a.snap.CleanupRan, a.snap.Cleanup)
	}
	saved, err := state.Load(a.cachePath)
	if err != nil {
		t.Fatalf("cache not written: %v", err)
	}
	if len(saved.Packages) != 7 || !saved.FetchedAt.Equal(testNow) {
		t.Errorf("cache has %d packages, fetched at %v", len(saved.Packages), saved.FetchedAt)
	}
}

// countingSource wraps fakeSource, counting calls per query. onDoctor runs
// at the start of each doctor call; block makes Installed wait for ctx.
type countingSource struct {
	*fakeSource
	calls    map[string]int
	onDoctor func()
	block    bool
	// dropAfter, once set, makes later Installed calls omit that package, as
	// after an uninstall; sized records the package names each Sizes call got.
	dropAfter string
	sized     [][]string
}

func newCountingSource(t *testing.T) *countingSource {
	return &countingSource{fakeSource: newFakeSource(t), calls: map[string]int{}}
}

func (c *countingSource) Installed(ctx context.Context) ([]brew.Package, error) {
	c.calls["installed"]++
	if c.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	pkgs, err := c.fakeSource.Installed(ctx)
	if c.dropAfter != "" {
		pkgs = slices.DeleteFunc(slices.Clone(pkgs), func(p brew.Package) bool { return p.Name == c.dropAfter })
	}
	return pkgs, err
}
func (c *countingSource) Outdated(ctx context.Context) ([]brew.Outdated, error) {
	c.calls["outdated"]++
	return c.fakeSource.Outdated(ctx)
}
func (c *countingSource) Services(ctx context.Context) ([]brew.Service, error) {
	c.calls["services"]++
	return c.fakeSource.Services(ctx)
}
func (c *countingSource) Taps(ctx context.Context) ([]brew.Tap, error) {
	c.calls["taps"]++
	return c.fakeSource.Taps(ctx)
}
func (c *countingSource) Sizes(ctx context.Context, pkgs []brew.Package) (map[string]int64, error) {
	c.calls["sizes"]++
	var names []string
	for _, p := range pkgs {
		names = append(names, p.Name)
	}
	c.sized = append(c.sized, names)
	return c.fakeSource.Sizes(ctx, pkgs)
}
func (c *countingSource) Cleanup(ctx context.Context) (brew.Cleanup, error) {
	c.calls["cleanup"]++
	return c.fakeSource.Cleanup(ctx)
}
func (c *countingSource) Doctor(ctx context.Context) ([]brew.DoctorWarning, error) {
	c.calls["doctor"]++
	if c.onDoctor != nil {
		c.onDoctor()
	}
	return c.fakeSource.Doctor(ctx)
}

func TestRefreshDuringRefreshRunsAgain(t *testing.T) {
	src := newCountingSource(t)
	a := newTestApp(t, src)
	first := a.Init()
	a.Update(press("r")) // every query is still in flight
	drain(t, a, first)
	for _, q := range []string{"installed", "outdated", "services", "taps"} {
		if src.calls[q] != 2 {
			t.Errorf("%s ran %d times, want 2 (the second r must not be dropped)", q, src.calls[q])
		}
	}
	if a.busy() {
		t.Errorf("still busy: %v", a.inflight)
	}
}

func TestTargetedRequestSkipsDoctorAndSaves(t *testing.T) {
	src := newCountingSource(t)
	a := newTestApp(t, src)
	drain(t, a, a.Init())
	src.calls = map[string]int{}
	os.Remove(a.cachePath)

	drain(t, a, a.request(qServices))
	if src.calls["services"] != 1 || src.calls["doctor"] != 0 || src.calls["cleanup"] != 0 || src.calls["installed"] != 0 {
		t.Errorf("a targeted request ran %v", src.calls)
	}
	if _, err := state.Load(a.cachePath); err != nil {
		t.Errorf("a successful targeted request should save the cache: %v", err)
	}
}

func TestCacheSavedBeforeDoctorRuns(t *testing.T) {
	src := newCountingSource(t)
	a := newTestApp(t, src)
	savedFirst := false
	src.onDoctor = func() {
		_, err := state.Load(a.cachePath)
		savedFirst = err == nil
	}
	drain(t, a, a.Init())
	if !savedFirst {
		t.Error("the JSON results should be saved before doctor starts")
	}
}

func TestAllFailedRefreshKeepsFetchedAt(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("boom")
	src.doctorErr = errors.New("boom")
	cached := fixtureSnapshot(t)
	cached.FetchedAt = testNow.Add(-time.Hour)
	a := New(src, nil, cached, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	drain(t, a, a.request(qServices))
	if !a.snap.FetchedAt.Equal(testNow.Add(-time.Hour)) {
		t.Errorf("a failed query must not advance FetchedAt, got %v", a.snap.FetchedAt)
	}
}

func TestQuitCancelsRunningQueries(t *testing.T) {
	src := newCountingSource(t)
	src.block = true
	a := newTestApp(t, src)
	done := make(chan tea.Msg)
	go func() { done <- fetch(a.ctx, src, qInstalled, nil)() }()

	_, cmd := a.Update(press("q"))
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q should quit")
	}
	select {
	case msg := <-done:
		a.Update(msg)
		if a.errs[qInstalled] != nil {
			t.Errorf("a query cancelled by quitting is not a failure: %v", a.errs[qInstalled])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("quitting did not cancel the running query")
	}
}

func TestCachedSnapshotShowsBeforeRefresh(t *testing.T) {
	cached := state.Snapshot{Packages: []brew.Package{{Name: "neovim", FullName: "neovim", Kind: brew.KindFormula, Version: "0.11.4", OnRequest: true}}}
	a := New(newFakeSource(t), nil, cached, filepath.Join(t.TempDir(), "s.json"))
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	show(t, a, "Top-level")
	if !strings.Contains(stripANSI(a.render()), "neovim") {
		t.Error("cached package should render before any query returns")
	}
}

func TestTabKeys(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	steps := []struct {
		key  string
		want int
	}{
		{"3", tabServices}, {"]", tabTaps}, {"]", tabDiscover}, {"[", tabTaps},
		{"4", tabDiscover}, {"5", tabHistory}, {"2", tabInstalled}, {"1", tabHome}, {"[", tabHistory}, // wraps from Home to the last entry
	}
	for _, s := range steps {
		a.Update(press(s.key))
		if a.tab != s.want {
			t.Fatalf("after %q tab = %d, want %d", s.key, a.tab, s.want)
		}
	}
}

func TestQuit(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	for _, k := range []string{"q", "ctrl+c"} {
		_, cmd := a.Update(press(k))
		if cmd == nil {
			t.Fatalf("%s: no command", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%s did not quit", k)
		}
	}
}

func TestKeysReachActiveTab(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	a.Update(press("l")) // home: into the tiles
	a.Update(press("l")) // select the Outdated tile
	a.Update(press("enter"))
	if a.tab != tabOutdated {
		t.Fatalf("enter on Outdated tile went to tab %d", a.tab)
	}
	show(t, a, "Top-level")
	a.Update(press("j"))
	if out := stripANSI(a.render()); !strings.Contains(out, "Top-level › fd") {
		t.Errorf("status bar should name the selected package:\n%s", out)
	}
}

func TestFailedQueryShowsErrorOnlyInItsTab(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("brew services info --all --json: exit status 1")
	a := newTestApp(t, src)
	drain(t, a, a.Init())

	show(t, a, "Services")
	out := stripANSI(a.render())
	if !strings.Contains(out, "couldn't load this view") || !strings.Contains(out, "exit status 1") {
		t.Errorf("services tab should show the error:\n%s", out)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "✗ services failed") {
		t.Errorf("status bar should name the failed query: %q", bar)
	}
	show(t, a, "Taps")
	if out := stripANSI(a.render()); !strings.Contains(out, "anomalyco/tap") {
		t.Error("taps tab should still work")
	}
}

func TestDoctorFailureShowsOnHome(t *testing.T) {
	src := newFakeSource(t)
	src.doctorErr = errors.New("brew doctor: timed out after 30s")
	a := newTestApp(t, src)
	drain(t, a, a.Init())

	if a.busy() {
		t.Fatalf("queries still in flight after refresh: %v", a.inflight)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "brew doctor failed") || strings.Contains(out, "brew doctor is running") {
		t.Errorf("home should report the doctor failure:\n%s", out)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "✗ doctor failed") {
		t.Errorf("status bar should name the failed query: %q", bar)
	}

	src.doctorErr = nil
	drain(t, a, a.refresh())
	out = stripANSI(a.render())
	if strings.Contains(out, "brew doctor failed") || !strings.Contains(out, "2 warnings") {
		t.Errorf("a successful doctor run should clear the failure:\n%s", out)
	}
}

func TestFailedQueryKeepsLastGoodData(t *testing.T) {
	src := newFakeSource(t)
	a := newTestApp(t, src)
	drain(t, a, a.Init())
	src.servicesErr = errors.New("boom")
	drain(t, a, a.refresh())
	if len(a.snap.Services) != 2 {
		t.Errorf("services from the earlier refresh were dropped: %v", a.snap.Services)
	}
}

func TestStatusBar(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	a.Init()
	if out := stripANSI(a.statusBar()); !strings.Contains(out, "NORMAL") || !strings.Contains(out, "refreshing…") {
		t.Errorf("status bar during refresh:\n%s", out)
	}
	a = newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	if out := stripANSI(a.statusBar()); !strings.Contains(out, "updated 09:30") || !strings.Contains(out, "q quit") {
		t.Errorf("status bar after refresh:\n%s", out)
	}
}

// squash collapses runs of spaces so tests don't depend on chip padding.
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

func TestSidebarBadges(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	got := squash(stripANSI(a.render()))
	if !strings.Contains(got, "Outdated ↑2") || !strings.Contains(got, "Services ✗1") {
		t.Errorf("sidebar badges missing: %q", got)
	}
}

func TestSidebarCountsEveryFailingService(t *testing.T) {
	src := newFakeSource(t)
	src.snap.Services = append(src.snap.Services, brew.Service{Name: "mysql", Status: "error"})
	a := newTestApp(t, src)
	drain(t, a, a.Init())
	if got := squash(stripANSI(a.render())); !strings.Contains(got, "Services ✗2") {
		t.Errorf("want ✗2 for two failing services: %q", got)
	}
}

func TestNarrowWindowShowsTheSidebarOrTheTab(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 70, Height: 30})
	keys(t, a, "j") // All, the sidebar still focused
	if out := stripANSI(a.render()); !strings.Contains(out, "PACKAGES") || strings.Contains(out, "NAME") {
		t.Errorf("narrow, sidebar focused: the sidebar alone\n%s", out)
	}
	keys(t, a, "l")
	if out := stripANSI(a.render()); strings.Contains(out, "PACKAGES") || !strings.Contains(out, "NAME") {
		t.Errorf("narrow, list focused: the list alone\n%s", out)
	}
}

func TestViewFitsTerminal(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	for i, e := range entries {
		settle(t, a, a.selectEntry(i))
		out := a.render()
		if h := lipgloss.Height(out); h != 40 {
			t.Errorf("%s renders %d lines, want 40", e.label, h)
		}
		if w := lipgloss.Width(out); w > 140 {
			t.Errorf("%s renders %d columns, want <= 140", e.label, w)
		}
	}
	if v := a.View(); !v.AltScreen {
		t.Error("brewery should use the alternate screen")
	}
}

// selectInstalled opens the Installed tab with the "all" filter and moves
// the cursor to name.
func selectInstalled(t *testing.T, a *App, name string) {
	t.Helper()
	show(t, a, "All")
	a.Update(press("g"))
	a.Update(press("g"))
	for range len(a.installed.rows) {
		if p, ok := a.installed.selected(); ok && p.Name == name {
			return
		}
		a.Update(press("j"))
	}
	t.Fatalf("%s not found on the Installed tab", name)
}

func TestViewFitsSmallTerminals(t *testing.T) {
	sizes := []struct{ w, h int }{{80, 24}, {100, 30}, {60, 20}}
	for _, size := range sizes {
		for _, failing := range []bool{false, true} {
			src := newFakeSource(t)
			if failing {
				src.servicesErr = errors.New("brew services info --all --json: exit status 1\nError: launchctl bootstrap failed for a very long and descriptive reason that will not fit")
			}
			a := newTestApp(t, src)
			a.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
			drain(t, a, a.Init())
			selectInstalled(t, a, "lua-language-server")
			for i, e := range entries {
				settle(t, a, a.selectEntry(i))
				out := a.render()
				if h := lipgloss.Height(out); h != size.h {
					t.Errorf("%dx%d failing=%v: %s renders %d lines, want %d\n%s", size.w, size.h, failing, e.label, h, size.h, stripANSI(out))
				}
				if w := lipgloss.Width(out); w > size.w {
					t.Errorf("%dx%d failing=%v: %s renders %d columns, want <= %d", size.w, size.h, failing, e.label, w, size.w)
				}
			}
		}
	}
}

func TestStatusBarIsOneLine(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	selectInstalled(t, a, "lua-language-server")
	for _, w := range []int{140, 80, 50, 30} {
		a.Update(tea.WindowSizeMsg{Width: w, Height: 24})
		bar := a.statusBar()
		if lipgloss.Height(bar) != 1 || lipgloss.Width(bar) > w {
			t.Errorf("width %d: status bar is %dx%d:\n%s", w, lipgloss.Width(bar), lipgloss.Height(bar), stripANSI(bar))
		}
	}
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	if out := stripANSI(a.statusBar()); strings.Contains(out, "q quit") || !strings.Contains(out, "updated 09:30") {
		t.Errorf("a narrow status bar should drop the key hints first: %q", out)
	}
	a.Update(tea.WindowSizeMsg{Width: 30, Height: 24})
	if out := stripANSI(a.statusBar()); !strings.Contains(out, "…") {
		t.Errorf("a very narrow status bar should truncate the left side: %q", out)
	}
	a.flash = "a flash far too long to fit in a status bar this narrow"
	a.Update(tea.WindowSizeMsg{Width: 20, Height: 24})
	if out := strings.TrimRight(stripANSI(a.statusBar()), " "); !strings.HasSuffix(out, "…") {
		t.Errorf("a status bar cut short should end with …: %q", out)
	}
}

func TestSidebarFitsNarrowWidth(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	a.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	for i, line := range strings.Split(a.render(), "\n") {
		if w := lipgloss.Width(line); w > 40 {
			t.Errorf("line %d is %d columns at width 40", i, w)
		}
	}
}

func TestRefreshFailureShowsOverCachedRows(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("brew services info --all --json: exit status 1\nError: launchctl failed")
	a := New(src, nil, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	drain(t, a, a.Init())

	if bar := squash(stripANSI(a.render())); !strings.Contains(bar, "Services ⚠") {
		t.Errorf("the sidebar should mark Services as stale: %q", bar)
	}
	show(t, a, "Services")
	out := stripANSI(a.render())
	for _, want := range []string{"postgresql@16", "refresh failed · r to retry", "exit status 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "launchctl failed") {
		t.Error("the banner should show only the first line of the error")
	}
	if bar := squash(stripANSI(a.render())); !strings.Contains(bar, "Services ⚠") {
		t.Errorf("the active entry should keep its ⚠ badge: %q", bar)
	}
	if h := lipgloss.Height(a.render()); h != 40 {
		t.Errorf("render is %d lines, want 40", h)
	}
}

func TestStatusBarNamesEveryFailedQuery(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	a.errs[qServices] = errors.New("boom")
	a.errs[qOutdated] = errors.New("boom")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "✗ outdated, services failed") {
		t.Errorf("status bar = %q", bar)
	}
}

func TestNothingShowsHealthyBeforeItLoads(t *testing.T) {
	a := newTestApp(t, newFakeSource(t)) // no cache, no results yet
	out := stripANSI(a.render())
	if strings.Contains(out, "up to date") || strings.Contains(out, "registered") {
		t.Errorf("home should not report health before any query returns:\n%s", out)
	}
	show(t, a, "Outdated")
	out = stripANSI(a.render())
	if !strings.Contains(out, "loading…") || strings.Contains(out, "up to date") {
		t.Errorf("outdated tab before its first result:\n%s", out)
	}
	a.apply(resultMsg{q: qOutdated}) // brew outdated succeeded with nothing to report
	if out := stripANSI(a.render()); !strings.Contains(out, "everything is up to date") {
		t.Errorf("outdated tab after an empty result:\n%s", out)
	}
}

func TestCachedSnapshotCountsAsLoaded(t *testing.T) {
	a := New(newFakeSource(t), nil, fixtureSnapshot(t), "")
	for _, q := range []query{qInstalled, qOutdated, qServices, qTaps, qDoctor} {
		if !a.loaded[q] {
			t.Errorf("%s not loaded from a non-empty cache", queryNames[q])
		}
	}
	if a := New(newFakeSource(t), nil, state.Snapshot{}, ""); len(a.loaded) != 0 && (a.loaded[qOutdated] || a.loaded[qDoctor]) {
		t.Error("an empty cache should load nothing")
	}
}
