package ui

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/state"
)

func TestHumanSize(t *testing.T) {
	for n, want := range map[int64]string{
		0: "0B", 999: "999B", 1536: "1.5KB", 52_480_000: "50.0MB", 2_362_232_012: "2.2GB",
	} {
		if got := humanSize(n); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestInstalledShowsSizes(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "2", "j", "j", "j") // git
	out := stripANSI(a.render())
	for _, want := range []string{"SIZE", "2.2GB", "50.0MB", "size       50.0MB"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	a.snap.Sizes = nil // not measured yet: no size rather than 0B
	a.sync()
	if out := stripANSI(a.render()); strings.Contains(out, "0B") || strings.Contains(out, "size ") {
		t.Errorf("unmeasured packages show no size:\n%s", out)
	}
}

func TestSizesAreMeasuredAfterRefreshAndChanges(t *testing.T) {
	src := newCountingSource(t)
	fs := &fakeStarter{}
	a := New(src, fs.start, state.Snapshot{}, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	drain(t, a, a.Init())
	if src.calls["sizes"] != 1 || len(a.snap.Sizes) != 3 {
		t.Fatalf("a full refresh measures sizes once: %v, %v", src.calls, a.snap.Sizes)
	}
	keys(t, a, "2", "j", "p") // pin fd: nothing on disk changes
	if src.calls["sizes"] != 1 {
		t.Errorf("pinning shouldn't measure again: %v", src.calls)
	}
	keys(t, a, "x", "y") // uninstall fd
	if src.calls["sizes"] != 2 {
		t.Errorf("uninstalling measures again: %v", src.calls)
	}
}

func TestSizesAreMeasuredFromTheFreshPackageList(t *testing.T) {
	src := newCountingSource(t)
	a := New(src, (&fakeStarter{}).start, state.Snapshot{}, filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	drain(t, a, a.Init())
	src.dropAfter, src.sized = "fd", nil // from now on brew no longer lists fd
	keys(t, a, "2", "j", "x", "y")       // uninstall fd
	if len(src.sized) != 1 {
		t.Fatalf("one measurement after the uninstall, got %v", src.sized)
	}
	if len(src.sized[0]) == 0 || slices.Contains(src.sized[0], "fd") {
		t.Errorf("sizes were measured for the old list: %v", src.sized[0])
	}
}

func TestSizesRunAtOnceWhenRequestedAlone(t *testing.T) {
	src := newCountingSource(t)
	a := New(src, (&fakeStarter{}).start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	keys(t, a, "c", "y") // cleanup: no installed query to wait for
	if src.calls["sizes"] != 1 || src.calls["installed"] != 0 || a.sizesAfterInstalled {
		t.Errorf("sizes should run straight away: %v waiting=%v", src.calls, a.sizesAfterInstalled)
	}
}
