package ui

import (
	"reflect"
	"strings"
	"testing"

	"brewery/internal/brew"
)

func TestMarkAndUpgradeSeveral(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "space")           // docker-desktop; the cursor moves on to fd
	keys(t, a, "j", "j", "j", "space") // lua-language-server
	out := stripANSI(a.render())
	if !strings.Contains(out, "● docker-desktop") || !strings.Contains(out, "● lua-language-server") {
		t.Errorf("marked rows should show it:\n%s", out)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "2 marked") {
		t.Errorf("status bar %q", bar)
	}
	keys(t, a, "u")
	want := [][]string{{"upgrade", "lua-language-server"}, {"upgrade", "--cask", "docker-desktop"}}
	if !reflect.DeepEqual(fs.calls, want) {
		t.Errorf("ran %v, want %v (one job per kind)", fs.calls, want)
	}
	if a.marks() != 0 {
		t.Error("marks clear once their jobs are queued")
	}
}

func TestMarkedUpgradeSkipsWhatBrewWould(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "j", "space", "u") // fd is up to date
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "nothing marked needs upgrading") {
		t.Errorf("ran %v, bar %q", fs.calls, stripANSI(a.statusBar()))
	}
}

func TestMarkOnOutdated(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "3", "space", "space") // lua-language-server, docker-desktop
	a.sync()                          // a refresh keeps the marks
	if a.marks() != 2 {
		t.Fatalf("marks after refresh: %d", a.marks())
	}
	keys(t, a, "u")
	want := [][]string{{"upgrade", "lua-language-server"}, {"upgrade", "--cask", "docker-desktop"}}
	if !reflect.DeepEqual(fs.calls, want) {
		t.Errorf("ran %v, want %v", fs.calls, want)
	}
}

func TestUninstallSeveralWithTheirOrphans(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "f", "j", "j", "j", "space") // all: git
	keys(t, a, "j", "j", "space")                // pcre2, which only git needs
	keys(t, a, "x")
	if a.confirm == nil || a.confirm.title != "Uninstall 2 packages?" {
		t.Fatalf("x should ask: %+v", a.confirm)
	}
	out := stripANSI(a.render())
	for _, want := range []string{"git, pcre2", "That leaves 1 dependency nothing else needs:", "json-c"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	keys(t, a, "o")
	if want := []string{"uninstall", "git", "pcre2", "json-c"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUninstallSeveralRefusesWhatOthersNeed(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "f", "j", "j", "j", "j", "space", "x") // all: json-c, which git and pcre2 need
	if a.confirm == nil || a.confirm.title != "Can't uninstall these" {
		t.Fatalf("want an explanation, got %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "json-c is required by git, pcre2.") {
		t.Errorf("should say what needs it:\n%s", out)
	}
	keys(t, a, "esc", "esc")
	if a.marks() != 0 || len(fs.calls) != 0 {
		t.Errorf("esc closes the box, then clears the marks: %d marked, ran %v", a.marks(), fs.calls)
	}
}

func TestStatusBarCountsMarksTheFilterHides(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "2", "space") // docker-desktop
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "1 marked") || strings.Contains(bar, "hidden") {
		t.Errorf("a shown mark: bar %q", bar)
	}
	keys(t, a, "/")
	typeText(t, a, "git")
	keys(t, a, "enter")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "1 marked · 1 hidden") {
		t.Errorf("a filtered-out mark: bar %q", bar)
	}
	keys(t, a, "/", "esc")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "1 marked") || strings.Contains(bar, "hidden") {
		t.Errorf("shown again: bar %q", bar)
	}
}

func TestStatusBarCountsMarksTheViewHides(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "2", "f") // all
	keys(t, a, "j", "j", "j", "space")
	if a.marks() != 1 || strings.Contains(stripANSI(a.statusBar()), "hidden") {
		t.Fatalf("one shown mark: bar %q", stripANSI(a.statusBar()))
	}
	for i := 0; i < 6 && !strings.Contains(stripANSI(a.statusBar()), "hidden"); i++ {
		keys(t, a, "f")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "1 marked · 1 hidden") {
		t.Errorf("some view should hide the mark: bar %q", bar)
	}
}

func TestStatusBarCountsOutdatedMarksTheFilterHides(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "3", "space") // lua-language-server
	keys(t, a, "/")
	typeText(t, a, "dock")
	keys(t, a, "enter")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "1 marked · 1 hidden") {
		t.Errorf("bar %q", bar)
	}
	keys(t, a, "/", "esc")
	if bar := stripANSI(a.statusBar()); strings.Contains(bar, "hidden") {
		t.Errorf("shown again: bar %q", bar)
	}
}

func TestMarksOnVanishedRowsAreDropped(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "2", "space") // docker-desktop
	if a.marks() != 1 {
		t.Fatalf("marks: %d", a.marks())
	}
	var keep []brew.Package
	for _, p := range a.snap.Packages {
		if p.Name != "docker-desktop" {
			keep = append(keep, p)
		}
	}
	a.snap.Packages = keep
	a.sync()
	if a.marks() != 0 {
		t.Errorf("a mark on a removed package should go: %d", a.marks())
	}
}
