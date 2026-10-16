package ui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dillo/brewery/internal/jobs"
)

// fakeLinks records the pages opened and the text copied.
type fakeLinks struct {
	opened, copied []string
	err            error
}

func (f *fakeLinks) open(url string) error  { f.opened = append(f.opened, url); return f.err }
func (f *fakeLinks) copy(text string) error { f.copied = append(f.copied, text); return f.err }

func newLinksApp(t *testing.T) (*App, *fakeLinks) {
	t.Helper()
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	l := &fakeLinks{}
	a.UseOpener(l.open)
	a.UseClipboard(l.copy)
	return a, l
}

func TestGGGoesToTheTop(t *testing.T) {
	a, _ := newLinksApp(t)
	keys(t, a, "2", "j", "j", "g", "g")
	if p, _ := a.installed.selected(); p.Name != "docker-desktop" {
		t.Errorf("gg should select the first row, got %s", p.Name)
	}
	keys(t, a, "g", "3")
	if a.tab != tabOutdated {
		t.Error("after g, any other key does what it always does")
	}
}

func TestGXOpensOnlyWebPages(t *testing.T) {
	// Homepages come from tap metadata; macOS open would launch a local
	// file or read "-…" as an option.
	for _, page := range []string{"file:///tmp/x.command", "-aTerminal", "javascript:alert(1)", "ftp://example.com", "HTTPS://example.com"} {
		a, l := newLinksApp(t)
		for i, p := range a.snap.Packages {
			if p.Name == "fd" {
				a.snap.Packages[i].Homepage = page
			}
		}
		a.sync()
		keys(t, a, "2", "j", "g", "x") // fd
		if len(l.opened) != 0 || !strings.Contains(stripANSI(a.statusBar()), "no web page for fd") {
			t.Errorf("%q: opened %q, bar %q", page, l.opened, stripANSI(a.statusBar()))
		}
	}
	a, l := newLinksApp(t)
	for i, p := range a.snap.Packages {
		if p.Name == "fd" {
			a.snap.Packages[i].Homepage = "http://example.com"
		}
	}
	a.sync()
	keys(t, a, "2", "j", "g", "x")
	if !reflect.DeepEqual(l.opened, []string{"http://example.com"}) {
		t.Errorf("plain http opens too: %q", l.opened)
	}
}

func TestGXOpensTheHomepage(t *testing.T) {
	a, l := newLinksApp(t)
	keys(t, a, "2", "j", "g", "x") // fd
	fd, _ := a.installed.selected()
	keys(t, a, "3", "g", "x") // lua-language-server, from its installed package
	lua, _ := a.packageNamed("lua-language-server", fd.Kind)
	keys(t, a, "5", "g", "x") // anomalyco/tap: its remote, without .git
	want := []string{fd.Homepage, lua.Homepage, "https://github.com/anomalyco/homebrew-tap"}
	if !reflect.DeepEqual(l.opened, want) {
		t.Errorf("opened %q, want %q", l.opened, want)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "opening https://github.com/anomalyco/homebrew-tap") {
		t.Errorf("status bar %q", bar)
	}
	keys(t, a, "j", "g", "x") // homebrew/core has no remote
	if bar := stripANSI(a.statusBar()); len(l.opened) != 3 || !strings.Contains(bar, "no web page for homebrew/core") {
		t.Errorf("opened %q, bar %q", l.opened, bar)
	}
}

func TestYCopiesTheName(t *testing.T) {
	a, l := newLinksApp(t)
	keys(t, a, "4", "y")
	if !reflect.DeepEqual(l.copied, []string{"postgresql@16"}) || !strings.Contains(stripANSI(a.statusBar()), "copied postgresql@16") {
		t.Errorf("copied %q, bar %q", l.copied, stripANSI(a.statusBar()))
	}
	a.UseHistory(jobs.History{Dir: t.TempDir()}, pastJobs())
	keys(t, a, ":")
	typeText(t, a, "jobs")
	keys(t, a, "enter", "y")
	if got := l.copied[len(l.copied)-1]; got != "brew upgrade fd" {
		t.Errorf("in :jobs y copies the command, got %q", got)
	}
}

func TestLinkErrorsShow(t *testing.T) {
	a, l := newLinksApp(t)
	l.err = errors.New("no browser")
	keys(t, a, "2", "g", "x")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "couldn't open the page: no browser") {
		t.Errorf("status bar %q", bar)
	}
	keys(t, a, "y")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "couldn't copy: no browser") {
		t.Errorf("status bar %q", bar)
	}
}

func TestBrewsQuestionForgetsAPendingG(t *testing.T) {
	p := newFakeProc(0, "working\r\n")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	l := &fakeLinks{}
	a.UseOpener(l.open)
	waiting := settle(t, a, a.run(action{title: "upgrade", args: []string{"upgrade"}}))
	keys(t, a, "2", "g") // half of a gx…
	p.out <- []byte("Password:")
	waiting = resume(t, a, waiting) // …when brew asks
	if a.mode != modeInsert {
		t.Fatalf("brew's question switches to insert, mode %v", a.mode)
	}
	keys(t, a, "esc", "x")
	if len(l.opened) != 0 {
		t.Errorf("the g before the question was forgotten, so x isn't gx: opened %v", l.opened)
	}
	p.exit()
	resume(t, a, waiting)
}

func TestGGOnOutdated(t *testing.T) {
	a, _ := newLinksApp(t)
	keys(t, a, "3", "j", "g", "g", "u")
	if a.outdated.cursor() != 0 {
		t.Errorf("gg goes to the top of Outdated: cursor %d", a.outdated.cursor())
	}
}
