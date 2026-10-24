package ui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
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

func TestThemeBoxKeys(t *testing.T) {
	n := len(theme.Themes)
	b := &themeBox{}
	for _, tt := range []struct {
		keys       []string
		cursor     int
		done, keep bool
	}{
		{[]string{"k"}, n - 1, false, false}, // wraps to the last
		{[]string{"j"}, 0, false, false},     // and back round
		{[]string{"down", "down", "up"}, 1, false, false},
		{[]string{"G"}, n - 1, false, false},
		{[]string{"g", "g"}, 0, false, false},
		{[]string{"j", "enter"}, 1, true, true},
		{[]string{"esc"}, 1, true, false},
		{[]string{"q"}, 1, true, false},
		{[]string{"g", "esc"}, 1, true, false},
	} {
		var done, keep bool
		for _, k := range tt.keys {
			done, keep = b.key(k)
		}
		if b.cursor != tt.cursor || done != tt.done || keep != tt.keep {
			t.Errorf("%v: cursor %d, done %v, keep %v; want %d, %v, %v", tt.keys, b.cursor, done, keep, tt.cursor, tt.done, tt.keep)
		}
	}
}

// pickerApp is an app that saves its theme to a temp directory.
func pickerApp(t *testing.T) (*App, string) {
	t.Helper()
	a := newJobApp(t, &fakeStarter{})
	dir := t.TempDir()
	a.UseThemeDir(dir)
	show(t, a, "Home")
	return a, dir
}

// saved reports whether a theme file was written to dir.
func saved(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "theme"))
	return err == nil
}

func TestPickerPreviewsAndCancels(t *testing.T) {
	a, dir := pickerApp(t)
	keys(t, a, "t")
	if a.themes == nil || a.themes.cursor != 0 || a.themes.was != "catppuccin-macchiato" {
		t.Fatalf("t opens the picker on the theme in use: %+v", a.themes)
	}
	keys(t, a, "j")
	if a.themeName != theme.Themes[1].Name {
		t.Errorf("j previews the next theme: in use %q", a.themeName)
	}
	if !strings.Contains(a.render(), "j/k preview · enter keep · esc cancel") {
		t.Error("the picker's footer should show")
	}
	keys(t, a, "esc")
	if a.themes != nil || a.themeName != "catppuccin-macchiato" {
		t.Errorf("esc closes the picker and puts the old theme back: in use %q", a.themeName)
	}
	if saved(dir) {
		t.Error("previewing and cancelling mustn't save anything")
	}
}

func TestPickerKeepsAndSaves(t *testing.T) {
	a, dir := pickerApp(t)
	keys(t, a, "t", "j", "j", "enter")
	want := theme.Themes[2]
	if a.themes != nil || a.themeName != want.Name {
		t.Errorf("enter keeps the theme under the cursor: in use %q", a.themeName)
	}
	if got := theme.LoadTheme(dir); got != want.Name {
		t.Errorf("saved %q, want %q", got, want.Name)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "theme: "+want.Title) {
		t.Errorf("the status bar should say which theme: %q", stripANSI(a.statusBar()))
	}
	keys(t, a, "t")
	if a.themes.cursor != 2 || a.themes.was != want.Name {
		t.Errorf("the picker opens on the kept theme: %+v", a.themes)
	}
}

func TestPickerSaveFailureStillApplies(t *testing.T) {
	a, _ := pickerApp(t)
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	a.UseThemeDir(filepath.Join(file, "brewery")) // can't make a dir under a file
	keys(t, a, "t", "j", "enter")
	if a.themeName != theme.Themes[1].Name {
		t.Errorf("the theme applies even when it can't be saved: in use %q", a.themeName)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "couldn't save") {
		t.Errorf("the status bar should say it couldn't be saved: %q", stripANSI(a.statusBar()))
	}
}

func TestTypingIntoAJobCancelsThePicker(t *testing.T) {
	a, _ := pickerApp(t)
	keys(t, a, "t", "j")
	a.typeIntoJob()
	if a.themes != nil || a.themeName != "catppuccin-macchiato" {
		t.Errorf("brew asking for input closes the picker and puts the old theme back: in use %q", a.themeName)
	}
}

func TestThemeCommand(t *testing.T) {
	a, dir := pickerApp(t)
	keys(t, a, ":")
	typeText(t, a, "theme dracula")
	keys(t, a, "enter")
	if a.themeName != "dracula" || theme.LoadTheme(dir) != "dracula" {
		t.Errorf(":theme dracula switches and saves: in use %q, saved %q", a.themeName, theme.LoadTheme(dir))
	}
	if a.themes != nil {
		t.Error(":theme with a name doesn't open the picker")
	}

	keys(t, a, ":")
	typeText(t, a, "theme nope")
	keys(t, a, "enter")
	if a.themeName != "dracula" {
		t.Errorf("an unknown name changes nothing: in use %q", a.themeName)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "no theme called nope") {
		t.Errorf("an unknown name says so: %q", stripANSI(a.statusBar()))
	}

	keys(t, a, ":")
	typeText(t, a, "theme")
	keys(t, a, "enter")
	if a.themes == nil || a.themes.was != "dracula" {
		t.Errorf(":theme alone opens the picker on the theme in use: %+v", a.themes)
	}
}

func TestThemeCompletion(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	for _, tt := range []struct{ line, word, want string }{
		{"th", "th", "theme"},
		{"theme d", "d", "dracula"},
		{"theme catppuccin-m", "catppuccin-m", "catppuccin-macchiato catppuccin-mocha"},
		{"theme nord ", "", ""}, // one name only
	} {
		word, cs := a.candidates(tt.line)
		if word != tt.word || candidateTexts(cs) != tt.want {
			t.Errorf("candidates(%q) = %q, %q; want %q, %q", tt.line, word, candidateTexts(cs), tt.word, tt.want)
		}
	}
	if _, cs := a.candidates("theme d"); len(cs) == 0 || cs[0].desc != "Dracula" {
		t.Errorf("a theme's title comes along as its description: %v", cs)
	}
}

func TestPickerOpensFromTheSidebar(t *testing.T) {
	a, _ := pickerApp(t)
	a.focus = colSidebar
	keys(t, a, "t")
	if a.themes == nil {
		t.Fatal("t should open the picker while the sidebar has the focus")
	}
}
