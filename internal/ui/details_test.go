package ui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dillo/brewery/internal/brew"
)

var ripgrep = brew.Details{
	Package: brew.Package{
		Name: "ripgrep", FullName: "ripgrep", Kind: brew.KindFormula, Tap: "homebrew/core",
		Desc: "Search tool like grep and The Silver Searcher", Homepage: "https://github.com/BurntSushi/ripgrep",
		License: "Unlicense", Latest: "15.2.0",
	},
	Needs: []string{"pcre2", "oniguruma"},
}

// selectDiscover moves the Discover cursor to name, from the top.
func selectDiscover(t *testing.T, a *App, name string) {
	t.Helper()
	keys(t, a, "g", "g")
	for range len(a.discover.rows) {
		if r, ok := a.discover.selected(); ok && r.Name == name {
			return
		}
		keys(t, a, "j")
	}
	t.Fatalf("%s not found on Discover", name)
}

func TestDiscoverPreview(t *testing.T) {
	a, cat := newDiscoverApp(t)
	cat.details = map[string]brew.Details{"ripgrep": ripgrep}
	show(t, a, "Search all")
	selectDiscover(t, a, "ripgrep")
	out := stripANSI(a.render())
	for _, want := range []string{
		"ripgrep 15.2.0", "Search tool like grep", "formula", "homebrew/core", "Unlicense",
		"https://github.com/BurntSushi/ripgrep", "Would also install", "oniguruma",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("preview is missing %q:\n%s", want, out)
		}
	}
	// pcre2 is installed already.
	if r, _ := a.discover.selected(); strings.Contains(stripANSI(a.discover.preview(r)), "pcre2") {
		t.Errorf("pcre2 is installed, so it isn't extra:\n%s", out)
	}

	selectDiscover(t, a, "git")
	if out := stripANSI(a.render()); !strings.Contains(out, "installed  yes") {
		t.Errorf("git is installed:\n%s", out)
	}
}

func TestDiscoverPreviewInstalls(t *testing.T) {
	a, _ := newStatsApp(t)
	show(t, a, "Popular tools") // Popular tools, gh first
	out := stripANSI(a.render())
	for _, want := range []string{"Installs", "30 days    217,104", "90 days    710,524", "365 days   2,994,048"} {
		if !strings.Contains(out, want) {
			t.Errorf("preview is missing %q:\n%s", want, out)
		}
	}
	show(t, a, "Popular apps") // codex: apps have no 365-day list
	if out := stripANSI(a.render()); !strings.Contains(out, "90 days    345,161") || strings.Contains(out, "365 days") {
		t.Errorf("cask installs wrong:\n%s", out)
	}
}

func TestDiscoverAsksBrewOnlyWhereTheCursorRests(t *testing.T) {
	a, cat := newDiscoverApp(t)
	show(t, a, "Search all") // asks about abcl, the first row
	cat.asked = nil
	var cmds []func()
	for range 3 {
		_, cmd := a.Update(press("j"))
		cmds = append(cmds, func() { drain(t, a, cmd) })
	}
	for _, run := range cmds { // the waits end after the cursor has moved on
		run()
	}
	if !slices.Equal(cat.asked, []string{"gh"}) {
		t.Errorf("asked about %v, want only gh, where the cursor stopped", cat.asked)
	}
	keys(t, a, "k", "j") // fd, then gh again: gh is known
	if !slices.Equal(cat.asked, []string{"gh", "fd"}) {
		t.Errorf("asked about %v", cat.asked)
	}
}

func TestDiscoverPreviewFailsAndWaits(t *testing.T) {
	a, cat := newDiscoverApp(t)
	cat.detailsErr = errors.New("brew info: exit status 1")
	show(t, a, "Search all")
	if out := stripANSI(a.render()); !strings.Contains(out, "couldn't ask brew") {
		t.Errorf("the failure isn't shown:\n%s", out)
	}

	a, _ = newDiscoverApp(t)
	show(t, a, "Popular tools")
	a.selectEntry(entryIndex("Search all")) // to All, and brew's answer about abcl hasn't come
	a.layout()                              // as Update would
	if out := stripANSI(a.render()); !strings.Contains(out, "asking brew…") {
		t.Errorf("want a note while brew is asked:\n%s", out)
	}
}

func TestDiscoverPreviewFocusAndLinks(t *testing.T) {
	a, cat := newDiscoverApp(t)
	cat.details = map[string]brew.Details{"ripgrep": ripgrep}
	var opened, copied string
	a.UseOpener(func(url string) error { opened = url; return nil })
	a.UseClipboard(func(text string) error { copied = text; return nil })
	show(t, a, "Search all")
	selectDiscover(t, a, "ripgrep")
	keys(t, a, "g", "x", "y")
	if opened != "https://github.com/BurntSushi/ripgrep" || copied != "ripgrep" {
		t.Errorf("opened %q, copied %q", opened, copied)
	}
	keys(t, a, "tab")
	if a.focus != colDetails {
		t.Fatal("tab should focus the preview")
	}
	keys(t, a, "j")
	if r, _ := a.discover.selected(); r.Name != "ripgrep" {
		t.Errorf("j scrolls the focused preview, not the list: on %s", r.Name)
	}
	keys(t, a, "esc")
	if a.focus == colDetails {
		t.Error("esc goes back to the list")
	}
}

func TestDiscoverFitsSmallTerminals(t *testing.T) {
	for _, size := range []struct{ w, h int }{{140, 40}, {100, 30}, {80, 24}, {60, 20}, {50, 16}} {
		a, cat := newDiscoverApp(t)
		cat.details = map[string]brew.Details{"ripgrep": ripgrep}
		a.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		show(t, a, "Search all")
		selectDiscover(t, a, "ripgrep")
		for _, focus := range []bool{false, true} {
			a.focus = colList
			if focus {
				a.focus = colDetails
			}
			out := a.render()
			if h := lipgloss.Height(out); h != size.h {
				t.Errorf("%dx%d focus=%v: %d lines\n%s", size.w, size.h, focus, h, stripANSI(out))
			}
			if w := lipgloss.Width(out); w > size.w {
				t.Errorf("%dx%d focus=%v: %d columns", size.w, size.h, focus, w)
			}
		}
	}
}
