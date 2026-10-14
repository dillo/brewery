package ui

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

func TestCleanupOffersBoth(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "c") // c works on any tab
	if a.confirm == nil || a.confirm.title != "Clean up Homebrew?" {
		t.Fatalf("c should ask: %+v", a.confirm)
	}
	out := stripANSI(a.render())
	for _, want := range []string{"The standard cleanup frees 3.9KB:", "Clearing the download cache frees 86.7MB:", "[y] standard", "[a] whole cache", "[n] cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	keys(t, a, "y")
	if want := []string{"cleanup"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	if env := fs.envs[len(fs.envs)-1]; !slices.Contains(env, "HOMEBREW_NO_AUTO_UPDATE=1") {
		t.Errorf("a pane action gets JobEnv: %v", env)
	}
}

func TestCleanupWholeCache(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	a.snap.Cleanup.Standard = brew.CleanupEstimate{}
	keys(t, a, "c")
	if out := stripANSI(a.render()); !strings.Contains(out, "The standard cleanup has nothing to remove.") || strings.Contains(out, "[y]") {
		t.Errorf("only the whole cache is on offer:\n%s", out)
	}
	keys(t, a, "a")
	if want := []string{"cleanup", "--prune=all"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestCleanupWithNothingToDo(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.snap.Cleanup = brew.Cleanup{}
	keys(t, a, "c")
	if a.confirm != nil || !strings.Contains(stripANSI(a.statusBar()), "nothing to clean up") {
		t.Errorf("nothing to clean: no dialog, just say so: %+v %q", a.confirm, stripANSI(a.statusBar()))
	}
	a.loaded[qCleanup] = false
	keys(t, a, "c")
	if a.confirm != nil || !strings.Contains(stripANSI(a.statusBar()), "still measuring what cleanup would free") {
		t.Errorf("before the dry-runs: %q", stripANSI(a.statusBar()))
	}
}

func TestCleanupFromTheHomeTile(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "l", "l", "l", "l", "enter")
	if a.confirm == nil || a.confirm.title != "Clean up Homebrew?" {
		t.Fatalf("enter on the Cleanup tile should ask: %+v", a.confirm)
	}
}

func TestCleanupJobMeasuresAgain(t *testing.T) {
	src := newCountingSource(t)
	a := New(src, (&fakeStarter{}).start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	keys(t, a, "c", "y") // the fake cleanup finishes at once
	if src.calls["cleanup"] != 1 || src.calls["installed"] != 0 {
		t.Errorf("after a cleanup only the dry-runs run again: %v", src.calls)
	}
}

func TestCleanupAfterTheDryRunsFailed(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.errs[qCleanup] = errors.New("brew cleanup --dry-run: exit status 1")
	keys(t, a, "c")
	if a.confirm != nil || !strings.Contains(stripANSI(a.statusBar()), "couldn't measure the cleanup · r retries") {
		t.Errorf("no dialog without numbers: %+v %q", a.confirm, stripANSI(a.statusBar()))
	}
}
