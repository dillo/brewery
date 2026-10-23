package ui

import (
	"fmt"
	"image/color"
	"reflect"
	"strings"
	"testing"

	"github.com/dillo/brewery/internal/jobs"
	"github.com/dillo/brewery/internal/theme"
)

// code is how c appears in the escape codes the screen is drawn with,
// as a foreground (38;2;…) or a background (48;2;…).
func code(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("2;%d;%d;%d", r>>8, g>>8, b>>8)
}

// colors lists every slot of p.
func colors(p theme.Palette) []color.Color {
	v := reflect.ValueOf(p)
	cs := make([]color.Color, v.NumField())
	for i := range cs {
		cs[i] = v.Field(i).Interface().(color.Color)
	}
	return cs
}

// noMacchiato fails when the screen still uses one of Macchiato's colours.
func noMacchiato(t *testing.T, where, screen string) {
	t.Helper()
	for _, c := range colors(theme.Macchiato) {
		if strings.Contains(screen, code(c)) {
			t.Errorf("%s: still drawn in Macchiato's %s", where, code(c))
		}
	}
}

func TestUseThemeReachesEveryPart(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseHistory(jobs.History{Dir: t.TempDir()}, recentJobs(a.now())) // History's rows are coloured when set
	a.UseTheme("dracula")
	if a.themeName != "dracula" {
		t.Fatalf("themeName = %q", a.themeName)
	}
	dracula := theme.Lookup("dracula").Palette
	for _, label := range []string{"Home", "Top-level", "Outdated", "Services", "History", "Maintenance"} {
		show(t, a, label)
		screen := a.render()
		noMacchiato(t, label, screen)
		if !strings.Contains(screen, code(dracula.Mauve)) {
			t.Errorf("%s: the focused pane should be in Dracula's accent", label)
		}
	}
	keys(t, a, ":")
	screen := a.render()
	noMacchiato(t, "the : prompt", screen)
	if !strings.Contains(screen, code(dracula.Blue)) {
		t.Error("the : prompt should be in Dracula's blue")
	}
	keys(t, a, "esc", "/")
	noMacchiato(t, "the / prompt", a.render())
}

func TestUseThemeUnknownIsTheDefault(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseTheme("dracula")
	a.UseTheme("nope")
	if a.themeName != "catppuccin-macchiato" {
		t.Errorf("themeName = %q, want the default", a.themeName)
	}
}

func TestUseThemeKeepsIcons(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseIcons(true)
	a.UseTheme("nord")
	if a.st.Icons != theme.Nerd || a.installed.st.Icons != theme.Nerd {
		t.Error("switching theme should keep the Nerd Font icons")
	}
}
