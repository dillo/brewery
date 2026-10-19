package ui

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestReinstallAsksThenRuns(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	selectInstalled(t, a, "fd")
	keys(t, a, "R")
	if a.confirm == nil || a.confirm.title != "Reinstall fd 10.5.0?" ||
		!reflect.DeepEqual(a.confirm.body, []string{"brew removes it and installs it again."}) {
		t.Fatalf("R should ask: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"reinstall", "fd"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestReinstallMarkedRunsOneJobPerKind(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	selectInstalled(t, a, "fd")
	keys(t, a, "space")
	selectInstalled(t, a, "docker-desktop")
	keys(t, a, "space", "R")
	if a.confirm == nil || a.confirm.title != "Reinstall 2 packages?" ||
		!reflect.DeepEqual(a.confirm.body, []string{"docker-desktop, fd"}) {
		t.Fatalf("R with marks should ask once: %+v", a.confirm)
	}
	keys(t, a, "y")
	want := [][]string{{"reinstall", "fd"}, {"reinstall", "--cask", "docker-desktop"}}
	if !reflect.DeepEqual(fs.calls, want) {
		t.Errorf("ran %v, want %v", fs.calls, want)
	}
	if a.marks() != 0 {
		t.Errorf("marks should clear: %d", a.marks())
	}
}

func TestLinkKey(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	selectInstalled(t, a, "fontbase")
	keys(t, a, "L")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "casks aren't linked") {
		t.Errorf("L on a cask: %v %q", fs.calls, stripANSI(a.statusBar()))
	}

	selectInstalled(t, a, "fd") // linked
	keys(t, a, "L")
	if a.confirm == nil || a.confirm.title != "Unlink fd?" {
		t.Fatalf("L on a linked formula should ask: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"unlink", "fd"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}

	selectInstalled(t, a, "json-c") // keg-only, unlinked
	keys(t, a, "L")
	if a.confirm == nil || a.confirm.title != "json-c is keg-only" {
		t.Fatalf("L on a keg-only formula should ask: %+v", a.confirm)
	}
	keys(t, a, "y") // not a choice here: nothing happens
	if a.confirm == nil {
		t.Fatal("y isn't a choice on the keg-only box")
	}
	keys(t, a, "f")
	if want := []string{"link", "--force", "json-c"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}

	for i := range a.snap.Packages {
		if a.snap.Packages[i].Name == "pcre2" {
			a.snap.Packages[i].Linked = false
		}
	}
	a.sync()
	selectInstalled(t, a, "pcre2") // unlinked, not keg-only
	keys(t, a, "L")
	if a.confirm != nil {
		t.Fatalf("linking shouldn't ask: %+v", a.confirm)
	}
	if want := []string{"link", "pcre2"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestFetchKey(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	selectInstalled(t, a, "fd")
	keys(t, a, "f")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "fd is up to date") {
		t.Errorf("f on an up-to-date package: %v %q", fs.calls, stripANSI(a.statusBar()))
	}
	selectInstalled(t, a, "lua-language-server")
	keys(t, a, "f")
	if want := []string{"fetch", "lua-language-server"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}

	selectInstalled(t, a, "fd")
	keys(t, a, "space", "f")
	if !strings.Contains(stripANSI(a.statusBar()), "nothing marked is outdated") {
		t.Errorf("f with only up-to-date marks: %q", stripANSI(a.statusBar()))
	}
	selectInstalled(t, a, "docker-desktop")
	keys(t, a, "space")
	selectInstalled(t, a, "lua-language-server")
	keys(t, a, "space")
	fs.calls = nil
	keys(t, a, "f")
	want := [][]string{{"fetch", "lua-language-server"}, {"fetch", "--cask", "docker-desktop"}}
	if !reflect.DeepEqual(fs.calls, want) {
		t.Errorf("ran %v, want %v", fs.calls, want)
	}
	if a.marks() != 3 {
		t.Errorf("fetch keeps the marks: %d", a.marks())
	}

	fs.calls = nil
	selectOutdated(t, a, "docker-desktop")
	keys(t, a, "f")
	if want := []string{"fetch", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("Outdated: ran %v, want %v", lastCall(fs), want)
	}
}

// refreshedBy runs keys on Installed, with name selected, against a
// counting source, and reports how often each query ran afterwards.
func refreshedBy(t *testing.T, name string, ks ...string) map[string]int {
	t.Helper()
	src := newCountingSource(t)
	fs := &fakeStarter{}
	a := New(src, fs.start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	selectInstalled(t, a, name)
	clear(src.calls)
	keys(t, a, ks...)
	return src.calls
}

func TestNewActionsRefreshWhatTheyChange(t *testing.T) {
	if got := refreshedBy(t, "lua-language-server", "f"); got["cleanup"] == 0 || got["installed"] != 0 {
		t.Errorf("fetch should measure cleanup again, and only that: %v", got)
	}
	if got := refreshedBy(t, "fd", "L", "y"); got["installed"] == 0 || got["outdated"] != 0 {
		t.Errorf("unlink should read Installed again, and only that: %v", got)
	}
	if got := refreshedBy(t, "fd", "R", "y"); got["installed"] == 0 || got["outdated"] == 0 || got["sizes"] == 0 {
		t.Errorf("reinstall should refresh what's on disk: %v", got)
	}
}

func TestMarkedFetchOnOutdatedIncludesPinned(t *testing.T) {
	a, fs := pinnedApp(t)
	selectOutdated(t, a, "pcre2")
	keys(t, a, "space")
	selectOutdated(t, a, "git")
	keys(t, a, "space")
	fs.calls = nil
	keys(t, a, "f")
	if want := [][]string{{"fetch", "pcre2", "git"}}; !sameCalls(fs.calls, want) {
		t.Errorf("ran %v, want one fetch of both, pinned included", fs.calls)
	}
	if a.marks() != 2 {
		t.Errorf("fetch keeps the marks: %d", a.marks())
	}
}

// sameCalls reports whether got has the calls in want, with each call's
// names in any order.
func sameCalls(got, want [][]string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		g, w := slices.Clone(got[i]), slices.Clone(want[i])
		slices.Sort(g)
		slices.Sort(w)
		if !slices.Equal(g, w) {
			return false
		}
	}
	return true
}
