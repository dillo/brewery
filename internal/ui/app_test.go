package ui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
)

// fakeSource serves the fixture snapshot. servicesErr and doctorErr make
// one query fail.
type fakeSource struct {
	snap        state.Snapshot
	servicesErr error
	doctorErr   error
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
func (f *fakeSource) Services(context.Context) ([]brew.Service, error) {
	if f.servicesErr != nil {
		return nil, f.servicesErr
	}
	return f.snap.Services, nil
}

// drain runs cmd and every command it produces, feeding the messages back
// into the app the way the Bubble Tea runtime would.
func drain(t *testing.T, a *App, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case nil:
		default:
			_, next := a.Update(msg)
			queue = append(queue, next)
		}
	}
}

var testNow = time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC)

func newTestApp(t *testing.T, src Source) *App {
	t.Helper()
	a := New(src, state.Snapshot{}, filepath.Join(t.TempDir(), "snapshot.json"))
	a.now = func() time.Time { return testNow }
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return a
}

func TestRefreshLoadsEverythingThenDoctorThenSaves(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())

	if a.pending != 0 {
		t.Fatalf("pending = %d after refresh", a.pending)
	}
	if len(a.snap.Packages) != 7 || len(a.snap.Outdated) != 2 || len(a.snap.Services) != 2 || len(a.snap.Taps) != 2 {
		t.Errorf("snapshot incomplete: %d pkgs, %d outdated, %d services, %d taps",
			len(a.snap.Packages), len(a.snap.Outdated), len(a.snap.Services), len(a.snap.Taps))
	}
	if !a.snap.DoctorRan || len(a.snap.Doctor) != 2 {
		t.Errorf("doctor not applied: ran=%v warnings=%d", a.snap.DoctorRan, len(a.snap.Doctor))
	}
	saved, err := state.Load(a.cachePath)
	if err != nil {
		t.Fatalf("cache not written: %v", err)
	}
	if len(saved.Packages) != 7 || !saved.FetchedAt.Equal(testNow) {
		t.Errorf("cache has %d packages, fetched at %v", len(saved.Packages), saved.FetchedAt)
	}
}

func TestRefreshIgnoredWhileRunning(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	if a.Init() == nil {
		t.Fatal("first refresh should start queries")
	}
	if _, cmd := a.Update(press("r")); cmd != nil {
		t.Error("r during a refresh should do nothing")
	}
}

func TestCachedSnapshotShowsBeforeRefresh(t *testing.T) {
	cached := state.Snapshot{Packages: []brew.Package{{Name: "neovim", FullName: "neovim", Kind: brew.KindFormula, Version: "0.11.4", OnRequest: true}}}
	a := New(newFakeSource(t), cached, filepath.Join(t.TempDir(), "s.json"))
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	a.Update(press("2"))
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
		{"3", tabOutdated}, {"]", tabServices}, {"]", tabTaps}, {"]", tabHome}, {"[", tabTaps}, {"1", tabHome},
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
	a.Update(press("l")) // home: select Outdated tile
	a.Update(press("enter"))
	if a.tab != tabOutdated {
		t.Fatalf("enter on Outdated tile went to tab %d", a.tab)
	}
	a.Update(press("2"))
	a.Update(press("j"))
	if out := stripANSI(a.render()); !strings.Contains(out, "Installed › git") {
		t.Errorf("status bar should name the selected package:\n%s", out)
	}
}

func TestFailedQueryShowsErrorOnlyInItsTab(t *testing.T) {
	src := newFakeSource(t)
	src.servicesErr = errors.New("brew services list --json: exit status 1")
	a := newTestApp(t, src)
	drain(t, a, a.Init())

	a.Update(press("4"))
	out := stripANSI(a.render())
	if !strings.Contains(out, "couldn't load this view") || !strings.Contains(out, "exit status 1") {
		t.Errorf("services tab should show the error:\n%s", out)
	}
	if !strings.Contains(out, "1 query failed") {
		t.Error("status bar should count the failure")
	}
	a.Update(press("5"))
	if out := stripANSI(a.render()); !strings.Contains(out, "anomalyco/tap") {
		t.Error("taps tab should still work")
	}
}

func TestDoctorFailureShowsOnHome(t *testing.T) {
	src := newFakeSource(t)
	src.doctorErr = errors.New("brew doctor: timed out after 30s")
	a := newTestApp(t, src)
	drain(t, a, a.Init())

	if a.pending != 0 {
		t.Fatalf("pending = %d after refresh", a.pending)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "brew doctor failed") || strings.Contains(out, "brew doctor is running") {
		t.Errorf("home should report the doctor failure:\n%s", out)
	}
	if !strings.Contains(out, "1 query failed") {
		t.Error("status bar should count the failure")
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

func TestTabBarBadges(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	out := stripANSI(a.tabBar())
	if !strings.Contains(out, "3 Outdated 2") || !strings.Contains(out, "4 Services !") {
		t.Errorf("tab bar badges missing: %q", out)
	}
}

func TestViewFitsTerminal(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	for tab := range tabNames {
		a.tab = tab
		out := a.render()
		if h := lipgloss.Height(out); h != 40 {
			t.Errorf("tab %s renders %d lines, want 40", tabNames[tab], h)
		}
		if w := lipgloss.Width(out); w > 140 {
			t.Errorf("tab %s renders %d columns, want <= 140", tabNames[tab], w)
		}
	}
	if v := a.View(); !v.AltScreen {
		t.Error("brewery should use the alternate screen")
	}
}
