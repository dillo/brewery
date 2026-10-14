package ui

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// typeText presses a key for each character of s.
func typeText(t *testing.T, a *App, s string) {
	t.Helper()
	for _, r := range s {
		keys(t, a, string(r))
	}
}

func TestColonRunsATypedCommandAsIs(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":")
	if a.mode != modeCommand {
		t.Fatalf("mode %v, want command", a.mode)
	}
	typeText(t, a, "info fd")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "COMMAND") || !strings.Contains(bar, ":info fd") {
		t.Errorf("status bar should be the prompt: %q", bar)
	}
	keys(t, a, "enter")
	if want := []string{"info", "fd"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	if fs.envs[0] != nil {
		t.Errorf("typed commands run unchanged, but got env %v", fs.envs[0])
	}
	if a.mode != modeNormal {
		t.Errorf("enter should leave command mode, mode %v", a.mode)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "✓ info fd") {
		t.Errorf("status bar: %q", stripANSI(a.statusBar()))
	}
}

func TestColonStripsALeadingBrewAndHonoursQuotes(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":")
	typeText(t, a, `brew search "visual studio"`)
	keys(t, a, "enter")
	if want := []string{"search", "visual studio"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestSplitArgs(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want []string
		err  bool
	}{
		{"upgrade fd git", []string{"upgrade", "fd", "git"}, false},
		{"  info   fd  ", []string{"info", "fd"}, false},
		{`search "a b" 'c d'`, []string{"search", "a b", "c d"}, false},
		{`desc "say \"hi\""`, []string{"desc", `say "hi"`}, false},
		{`info a\ b`, []string{"info", "a b"}, false},
		{`info ""`, []string{"info", ""}, false},
		{`info 'it''s'`, []string{"info", "its"}, false},
		{`search "open`, nil, true},
		{"", nil, false},
	} {
		got, err := splitArgs(tt.in)
		if (err != nil) != tt.err || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitArgs(%q) = %q, %v; want %q, err %v", tt.in, got, err, tt.want, tt.err)
		}
	}
}

func TestUnclosedQuoteRunsNothing(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":")
	typeText(t, a, `search "open`)
	keys(t, a, "enter")
	if len(fs.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "unclosed quote") {
		t.Errorf("ran %v; status bar %q", fs.calls, stripANSI(a.statusBar()))
	}
}

func TestEscCancelsTheCommandLine(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":")
	typeText(t, a, "upgrade")
	keys(t, a, "esc")
	if a.mode != modeNormal || len(fs.calls) != 0 {
		t.Errorf("esc should cancel: mode %v, ran %v", a.mode, fs.calls)
	}
	keys(t, a, ":")
	if a.cmd.value() != "" {
		t.Errorf("the prompt should open empty, has %q", a.cmd.value())
	}
}

func TestEmptyCommandDoesNothing(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":", " ", "enter")
	if len(fs.calls) != 0 || a.mode != modeNormal {
		t.Errorf("ran %v, mode %v", fs.calls, a.mode)
	}
}

func TestShortcutKeysAreTextInCommandMode(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, ":")
	typeText(t, a, "uninstall x") // u, x, 2, ... must not act or switch tabs
	if a.tab != tabHome || a.confirm != nil || len(fs.calls) != 0 || a.cmd.value() != "uninstall x" {
		t.Errorf("tab %d, confirm %+v, ran %v, value %q", a.tab, a.confirm, fs.calls, a.cmd.value())
	}
}

func TestColonQQuits(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, ":", "q")
	_, cmd := a.Update(press("enter"))
	if cmd == nil {
		t.Fatal(":q returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error(":q should quit")
	}
}

func TestUpAndDownRecallPastCommands(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	for _, line := range []string{"info fd", "deps git"} {
		keys(t, a, ":")
		typeText(t, a, line)
		keys(t, a, "enter")
	}
	keys(t, a, ":")
	typeText(t, a, "se")
	keys(t, a, "up")
	if v := a.cmd.value(); v != "deps git" {
		t.Errorf("up once: %q", v)
	}
	keys(t, a, "up", "up") // stops at the oldest
	if v := a.cmd.value(); v != "info fd" {
		t.Errorf("up three times: %q", v)
	}
	keys(t, a, "down", "down")
	if v := a.cmd.value(); v != "se" {
		t.Errorf("down past the newest should restore the draft, got %q", v)
	}
}

func TestPasteGoesIntoTheCommandLine(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, ":")
	a.Update(tea.PasteMsg{Content: "info fd"})
	if v := a.cmd.value(); v != "info fd" {
		t.Errorf("value %q", v)
	}
}

func TestTypedCommandsRefreshWhatTheyMayChange(t *testing.T) {
	for _, tt := range []struct {
		line string
		want []string // queries that should run once afterwards
	}{
		{"info fd", nil},
		{"--prefix", nil},
		{"services list", nil},
		{"services stop redis", []string{"services"}},
		{"upgrade fd", []string{"installed", "outdated", "services", "taps", "doctor"}},
	} {
		src := newCountingSource(t)
		a := New(src, (&fakeStarter{}).start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
		a.tickEvery = 0
		a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		keys(t, a, ":")
		typeText(t, a, tt.line)
		keys(t, a, "enter")
		var ran []string
		for _, q := range []string{"installed", "outdated", "services", "taps", "doctor"} {
			if src.calls[q] > 0 {
				ran = append(ran, q)
			}
		}
		if !reflect.DeepEqual(ran, tt.want) {
			t.Errorf(":%s refreshed %v, want %v", tt.line, ran, tt.want)
		}
	}
}

func TestHintsMentionTheCommandLine(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, ": command") {
		t.Errorf("hints should mention the command line: %q", bar)
	}
}
