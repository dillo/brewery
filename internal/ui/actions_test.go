package ui

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// keys presses each key in turn and settles the commands they start.
func keys(t *testing.T, a *App, ks ...string) {
	t.Helper()
	for _, k := range ks {
		_, cmd := a.Update(press(k))
		settle(t, a, cmd)
	}
}

// lastCall is the most recent brew command started, or nil.
func lastCall(fs *fakeStarter) []string {
	if len(fs.calls) == 0 {
		return nil
	}
	return fs.calls[len(fs.calls)-1]
}

func TestUpgradeFromInstalled(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Top-level")
	keys(t, a, "j", "u") // fd, up to date
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "fd is up to date") {
		t.Errorf("u on an up-to-date package should just say so: %v %q", fs.calls, stripANSI(a.statusBar()))
	}
	keys(t, a, "j", "j", "j", "u") // lua-language-server, outdated
	if want := []string{"upgrade", "lua-language-server"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUpgradeSkipsPinned(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	for i := range a.snap.Packages {
		if a.snap.Packages[i].Name == "lua-language-server" {
			a.snap.Packages[i].Pinned = true
		}
	}
	a.snap.Outdated[0].Pinned = true
	a.sync()
	show(t, a, "Top-level")
	keys(t, a, "j", "j", "j", "j", "u") // lua-language-server
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "lua-language-server is pinned · p unpins it") {
		t.Errorf("u on a pinned package should say so: %v %q", fs.calls, stripANSI(a.statusBar()))
	}
	show(t, a, "Outdated")
	keys(t, a, "u")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "is pinned · unpin it on Installed") {
		t.Errorf("u on a pinned outdated row should say so: %v", fs.calls)
	}
}

func TestUpgradeCaskFromOutdated(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Outdated")
	keys(t, a, "j", "u")
	if want := []string{"upgrade", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUpgradeAllAsksFirst(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Outdated")
	keys(t, a, "A")
	if a.confirm == nil || a.confirm.title != "Upgrade all 2 outdated?" {
		t.Fatalf("A should ask: %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "lua-language-server, docker-desktop") {
		t.Errorf("the confirmation should list what gets upgraded:\n%s", out)
	}
	keys(t, a, "y")
	if want := []string{"upgrade"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	if env := fs.envs[len(fs.envs)-1]; !slices.Contains(env, "HOMEBREW_NO_ASK=1") {
		t.Errorf("confirmed here, so brew needn't ask again: env %v", env)
	}
}

func TestUninstallOffersOrphans(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Top-level")
	keys(t, a, "j", "j", "j", "x") // git: leaves pcre2 and json-c behind
	if a.confirm == nil || a.confirm.title != "Uninstall git 2.51.0?" {
		t.Fatalf("x should ask: %+v", a.confirm)
	}
	out := stripANSI(a.render())
	for _, want := range []string{"That leaves 2 dependencies nothing else needs:", "json-c, pcre2", "[o] uninstall + orphans"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	keys(t, a, "o")
	if want := []string{"uninstall", "git", "json-c", "pcre2"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUninstallWithoutOrphans(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Top-level")
	keys(t, a, "j", "x") // fd
	if len(a.confirm.choices) != 2 {
		t.Errorf("fd has no orphans: offer only uninstall and cancel, got %+v", a.confirm.choices)
	}
	keys(t, a, "n")
	if len(fs.calls) != 0 || a.confirm != nil {
		t.Fatal("n cancels")
	}
	keys(t, a, "x", "y")
	if want := []string{"uninstall", "fd"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUninstallCask(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Casks")
	keys(t, a, "g", "g", "x") // docker-desktop
	if a.confirm == nil || a.confirm.title != "Uninstall docker-desktop 4.39.0,184744?" {
		t.Fatalf("x should ask: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"uninstall", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUninstallRefusesWhatACaskNeeds(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	for i, p := range a.snap.Packages {
		if p.Name == "docker-desktop" {
			a.snap.Packages[i].Deps, a.snap.Packages[i].DirectDeps = []string{"fd"}, []string{"fd"}
		}
	}
	a.sync()
	show(t, a, "All")
	keys(t, a, "j", "x") // fd
	if a.confirm == nil || a.confirm.title != "Can't uninstall fd" {
		t.Fatalf("want an explanation, got %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Required by docker-desktop.") {
		t.Errorf("should say the cask needs it:\n%s", out)
	}
}

func TestUninstallRefusesWhatOthersNeed(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Dependencies")
	keys(t, a, "g", "g", "x") // top row: json-c, used by git and pcre2
	if a.confirm == nil || a.confirm.title != "Can't uninstall json-c" {
		t.Fatalf("want an explanation, got %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Required by git, pcre2.") {
		t.Errorf("should say what needs it:\n%s", out)
	}
	keys(t, a, "y") // not a choice here
	if len(fs.calls) != 0 {
		t.Errorf("nothing may run: %v", fs.calls)
	}
	keys(t, a, "esc")
	if a.confirm != nil {
		t.Error("esc closes the explanation")
	}
}

func TestPinToggles(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Top-level")
	keys(t, a, "j", "p") // fd
	if want := []string{"pin", "fd"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	keys(t, a, "j", "j", "p") // git
	if want := []string{"unpin", "git"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	show(t, a, "Casks")
	keys(t, a, "g", "g", "p") // docker-desktop
	if want := []string{"pin", "--cask", "docker-desktop"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestServiceKeys(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Services")
	keys(t, a, "s")
	if want := []string{"services", "start", "postgresql@16"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	keys(t, a, "j", "R")
	if want := []string{"services", "restart", "redis"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	keys(t, a, "S")
	if a.confirm == nil || a.confirm.title != "Stop redis?" {
		t.Fatalf("S should ask: %+v", a.confirm)
	}
	keys(t, a, "y")
	if want := []string{"services", "stop", "redis"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestUpdateRefreshesEverything(t *testing.T) {
	src := newCountingSource(t)
	fs := &fakeStarter{}
	a := New(src, fs.start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	keys(t, a, "U")
	if want := []string{"update"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	for _, q := range []string{"installed", "outdated", "services", "taps", "doctor", "cleanup"} {
		if src.calls[q] != 1 {
			t.Errorf("after brew update, %s ran %d times, want 1", q, src.calls[q])
		}
	}
}

func TestActionKeysOnlyWorkOnTheirTab(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	show(t, a, "Outdated")
	keys(t, a, "x", "p", "L")
	show(t, a, "Services")
	keys(t, a, "u", "A", "x")
	show(t, a, "Taps")
	keys(t, a, "s", "L")
	if len(fs.calls) != 0 || a.confirm != nil {
		t.Errorf("keys off their tab must do nothing: %v %+v", fs.calls, a.confirm)
	}
}

func TestStatusHintsFollowTheTab(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	for _, tt := range []struct{ entry, want string }{
		{"Top-level", "u upgrade · x uninstall · p pin · l details"},
		{"Outdated", "u upgrade · A upgrade all"},
		{"Services", "s start · S stop · R restart · L log"},
		{"Taps", "x untap"},
		{"Home", "U update · c clean up"},
	} {
		show(t, a, tt.entry)
		if bar := stripANSI(a.statusBar()); !strings.Contains(bar, tt.want) {
			t.Errorf("%s hints: %q", tt.entry, bar)
		}
	}
}

func TestUpgradeAllCountsOnlyUnpinned(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.snap.Outdated[0].Pinned = true
	show(t, a, "Outdated")
	keys(t, a, "A")
	if a.confirm == nil || a.confirm.title != "Upgrade all 1 outdated?" {
		t.Fatalf("pinned rows must not be counted: %+v", a.confirm)
	}
	if want := []string{"docker-desktop", "1 pinned will be skipped."}; !reflect.DeepEqual(a.confirm.body, want) {
		t.Errorf("body %q, want %q", a.confirm.body, want)
	}
}
