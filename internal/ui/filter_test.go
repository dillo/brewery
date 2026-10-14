package ui

import (
	"reflect"
	"strings"
	"testing"
)

func TestFilterInstalled(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "2", "/")
	if bar := stripANSI(a.statusBar()); a.mode != modeFilter || !strings.Contains(bar, "FILTER") || !strings.Contains(bar, "enter keep · esc clear") {
		t.Fatalf("/ should open the filter: mode %d, bar %q", a.mode, stripANSI(a.statusBar()))
	}
	typeText(t, a, "GI") // case doesn't matter
	if got := rowNames(a.installed); got != "git" {
		t.Errorf("rows %q, want just git", got)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Installed · /GI") {
		t.Errorf("the pane should name the filter:\n%s", out)
	}
	keys(t, a, "enter")
	if a.mode != modeNormal || rowNames(a.installed) != "git" {
		t.Errorf("enter keeps the filter: mode %d, rows %q", a.mode, rowNames(a.installed))
	}
	keys(t, a, "/")
	if a.filterIn.Value() != "GI" {
		t.Errorf("/ again starts from the filter, got %q", a.filterIn.Value())
	}
	keys(t, a, "esc")
	if a.mode != modeNormal || rowNames(a.installed) != "docker-desktop fd fontbase git lua-language-server" {
		t.Errorf("esc clears the filter: rows %q", rowNames(a.installed))
	}
}

func TestFilterKeysEditTheLine(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "2", "/")
	typeText(t, a, "qux")
	if len(fs.calls) != 0 || a.tab != tabInstalled || a.confirm != nil {
		t.Errorf("letters edit the filter, never act: %v", fs.calls)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing in this view matches /qux") {
		t.Errorf("an empty result should say why:\n%s", out)
	}
}

func TestFilterOutdatedActsOnTheShownRow(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "3", "/")
	typeText(t, a, "dock")
	keys(t, a, "enter", "u")
	if want := []string{"upgrade", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestFilterIsPerList(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "5", "/")
	typeText(t, a, "zzz")
	keys(t, a, "enter")
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing matches /zzz") {
		t.Errorf("no tap matches:\n%s", out)
	}
	keys(t, a, "4")
	if a.services.n != 2 {
		t.Errorf("Services keeps its own filter: %d rows", a.services.n)
	}
	keys(t, a, "5")
	if a.taps.n != 0 || a.taps.match != "zzz" {
		t.Errorf("Taps' filter survives switching tabs: %d rows, %q", a.taps.n, a.taps.match)
	}
}

func TestFilterJobs(t *testing.T) {
	a := newJobsViewApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter", "/")
	typeText(t, a, "pin")
	keys(t, a, "enter", "enter") // replay the one shown
	if a.drawer.job == nil || a.drawer.job.Title != "pin git" {
		t.Errorf("replayed %+v, want pin git", a.drawer.job)
	}
}

func TestFilterOnHome(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "/")
	if a.mode != modeNormal || !strings.Contains(stripANSI(a.statusBar()), "Home has none") {
		t.Errorf("Home has no list: mode %d, bar %q", a.mode, stripANSI(a.statusBar()))
	}
}

func TestPromptWaitsForTheFilter(t *testing.T) {
	p := newFakeProc(0, "==> Installing docker-desktop\r\n")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade docker-desktop", args: []string{"upgrade", "--cask", "docker-desktop"}}))
	keys(t, a, "2", "/")
	typeText(t, a, "gi")
	p.out <- []byte("Password:")
	waiting = resume(t, a, waiting)
	if a.mode != modeFilter || p.input() != "" {
		t.Fatalf("a prompt must not take over the filter (mode %v, brew got %q)", a.mode, p.input())
	}
	keys(t, a, "enter")
	if a.mode != modeInsert {
		t.Errorf("closing the filter should answer the waiting prompt (mode %v)", a.mode)
	}
	p.exit()
	resume(t, a, waiting)
}
