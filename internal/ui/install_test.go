package ui

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

// newInstallApp is a Discover app that runs jobs, on the All view.
func newInstallApp(t *testing.T) (*App, *fakeStarter, *fakeSource, *fakeCatalog) {
	t.Helper()
	fs, src := &fakeStarter{}, newFakeSource(t)
	a := New(src, fs.start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	cat := newFakeCatalog(t)
	cat.details = map[string]brew.Details{"ripgrep": ripgrep}
	a.UseCatalog(cat)
	a.detailsDelay = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	keys(t, a, "6", "F")
	return a, fs, src, cat
}

// confirmText is the open confirmation's title and body, or "".
func confirmText(a *App) string {
	if a.confirm == nil {
		return ""
	}
	return a.confirm.title + "\n" + strings.Join(a.confirm.body, "\n")
}

func TestInstallAsksFirst(t *testing.T) {
	a, fs, src, _ := newInstallApp(t)
	selectDiscover(t, a, "ripgrep")
	keys(t, a, "a")
	want := "Install ripgrep 15.2.0?\nSearch tool like grep and The Silver Searcher\nalso installs 1 dependency: oniguruma"
	if got := confirmText(a); got != want {
		t.Errorf("asked\n%s\nwant\n%s", got, want)
	}
	keys(t, a, "n")
	if len(fs.calls) != 0 {
		t.Fatalf("n ran %v", fs.calls)
	}

	src.snap.Packages = append(src.snap.Packages, brew.Package{Name: "ripgrep", FullName: "ripgrep", Kind: brew.KindFormula, Version: "15.2.0"})
	keys(t, a, "a", "y")
	if want := []string{"install", "ripgrep"}; !slices.Equal(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	// The dialog asked: brew needn't ask again.
	if env := fs.envs[len(fs.envs)-1]; !slices.Contains(env, "HOMEBREW_NO_ASK=1") || !slices.Contains(env, "HOMEBREW_NO_AUTO_UPDATE=1") {
		t.Errorf("env %v", env)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "✓ install ripgrep") || !strings.Contains(out, "✓ ripgrep") {
		t.Errorf("want the job done and ripgrep marked installed:\n%s", out)
	}
}

func TestInstallSaysWhenInstalled(t *testing.T) {
	a, fs, _, _ := newInstallApp(t)
	selectDiscover(t, a, "git")
	keys(t, a, "a")
	if a.confirm != nil || len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "git is already installed") {
		t.Errorf("confirm %v, calls %v, status %q", a.confirm, fs.calls, stripANSI(a.statusBar()))
	}
}

func TestInstallCasks(t *testing.T) {
	a, fs, _, _ := newInstallApp(t)
	selectDiscover(t, a, "firefox")
	keys(t, a, "a")
	if got, want := confirmText(a), "Install firefox?\napp · Web browser"; got != want {
		t.Errorf("asked\n%s\nwant\n%s", got, want)
	}
	keys(t, a, "y")
	if want := []string{"install", "--cask", "firefox"}; !slices.Equal(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}

	selectDiscover(t, a, "font-fira-code")
	keys(t, a, "a")
	if got := confirmText(a); !strings.Contains(got, "\nfont · Fira Code") {
		t.Errorf("asked\n%s", got)
	}
}

func TestInstallBeforeBrewHasSaid(t *testing.T) {
	a, fs, _, _ := newInstallApp(t)
	selectDiscover(t, a, "python@3.15")
	a.Update(press("j")) // to ripgrep; brew hasn't been asked yet
	a.Update(press("a"))
	if got := confirmText(a); !strings.Contains(got, "checking dependencies…") {
		t.Fatalf("asked\n%s", got)
	}
	drain(t, a, a.detailsDue(detailsDueMsg{key: entryKey("ripgrep", brew.KindFormula)}))
	if got := confirmText(a); !strings.Contains(got, "also installs 1 dependency: oniguruma") {
		t.Errorf("the dialog didn't catch up:\n%s", got)
	}
	keys(t, a, "y")
	if want := []string{"install", "ripgrep"}; !slices.Equal(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestAlsoInstalls(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string
	}{
		{[]string{"pcre2"}, "also installs 1 dependency: pcre2"},
		{[]string{"a", "b", "c", "d", "e", "f", "g"}, "also installs 7 dependencies: a, b, c, d, e +2 more"},
	} {
		if got := alsoInstalls(c.names); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

func TestInstallDialogStaysOnItsPackage(t *testing.T) {
	a, fs, _, cat := newInstallApp(t)
	selectDiscover(t, a, "python@3.15")
	a.Update(press("j")) // to ripgrep; brew hasn't been asked yet
	a.Update(press("a"))
	// ripgrep leaves the view under the open dialog: the cursor lands elsewhere.
	var rest []brew.Entry
	for _, e := range cat.entries {
		if e.Name != "ripgrep" {
			rest = append(rest, e)
		}
	}
	a.Update(catalogueMsg{entries: rest})
	if r, ok := a.discover.selected(); ok && r.Name == "ripgrep" {
		t.Fatalf("setup: the cursor is still on ripgrep")
	}
	a.Update(detailsMsg{key: entryKey("ripgrep", brew.KindFormula), d: ripgrep})
	if got := confirmText(a); !strings.HasPrefix(got, "Install ripgrep 15.2.0?") {
		t.Errorf("the dialog changed package:\n%s", got)
	}
	keys(t, a, "y")
	if want := []string{"install", "ripgrep"}; !slices.Equal(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestInstallSaysWhenDependenciesCouldntBeChecked(t *testing.T) {
	a, fs, _, cat := newInstallApp(t)
	cat.detailsErr = errors.New("brew failed")
	selectDiscover(t, a, "python@3.15")
	a.Update(press("j"))
	a.Update(press("a"))
	if got := confirmText(a); !strings.Contains(got, "checking dependencies…") {
		t.Fatalf("before the answer:\n%s", got)
	}
	drain(t, a, a.detailsDue(detailsDueMsg{key: entryKey("ripgrep", brew.KindFormula)}))
	got := confirmText(a)
	if !strings.Contains(got, "couldn't check dependencies") || strings.Contains(got, "checking") {
		t.Errorf("after the failed answer:\n%s", got)
	}
	keys(t, a, "y")
	if want := []string{"install", "ripgrep"}; !slices.Equal(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	a.confirm = nil // open it again, now that brew has failed
	keys(t, a, "a")
	if got := confirmText(a); !strings.Contains(got, "couldn't check dependencies") {
		t.Errorf("opened after the failure:\n%s", got)
	}
}
