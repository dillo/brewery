package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dillo/brewery/internal/brew"
)

func TestUntapAsksFirst(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "5", "x") // anomalyco/tap: nothing installed from it
	if a.confirm == nil || a.confirm.title != "Untap anomalyco/tap?" {
		t.Fatalf("x should ask: %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Its 2 formulae and 1 cask won't be available") {
		t.Errorf("the dialog should say what goes:\n%s", out)
	}
	keys(t, a, "y")
	if want := []string{"untap", "anomalyco/tap"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "x untap") {
		t.Errorf("the hint should name x: %q", bar)
	}
}

func TestUntapRefusesATapInUse(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "5", "j", "x") // homebrew/core: fd, git, … came from it
	if a.confirm == nil || a.confirm.title != "Can't untap homebrew/core" {
		t.Fatalf("want an explanation, got %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Installed from it: fd, git, pcre2") {
		t.Errorf("should say what came from it:\n%s", out)
	}
	keys(t, a, "y")
	if len(fs.calls) != 0 {
		t.Errorf("nothing may run: %v", fs.calls)
	}
}

func TestTapContents(t *testing.T) {
	for _, tt := range []struct {
		formulae, casks int
		want            string
	}{
		{0, 0, ""},
		{1, 0, "1 formula"},
		{3, 0, "3 formulae"},
		{0, 1, "1 cask"},
		{2, 4, "2 formulae and 4 casks"},
	} {
		if got := tapContents(brew.Tap{Formulae: tt.formulae, Casks: tt.casks}); got != tt.want {
			t.Errorf("tapContents(%d, %d) = %q, want %q", tt.formulae, tt.casks, got, tt.want)
		}
	}
}
