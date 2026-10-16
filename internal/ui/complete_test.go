package ui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/dillo/brewery/internal/brew"
)

// testCompletions is a small slice of what brew's fish script describes.
var testCompletions = brew.Completions{Commands: []brew.Command{
	{Name: "info", Desc: "Display brief statistics", Args: []string{"formulae_all", "casks_all"}},
	{Name: "install", Desc: "Install a formula or cask", Args: []string{"formulae_all", "casks_all"}},
	{Name: "pin", Desc: "Pin the specified package", Args: []string{"formulae_installed"}},
	{Name: "services", Desc: "Manage background services", Subs: []brew.Command{
		{Name: "start", Desc: "Start the service", Args: []string{"services"}, Flags: []brew.Flag{{Name: "--all", Desc: "Start all services"}}},
		{Name: "stop", Desc: "Stop the service", Args: []string{"services"}},
	}},
	{Name: "uninstall", Desc: "Uninstall a formula or cask", Args: []string{"formulae_installed", "casks_installed"},
		Flags: []brew.Flag{{Name: "--force", Desc: "Delete all installed versions"}, {Name: "--zap", Desc: "Remove all files"}}},
	{Name: "untap", Desc: "Remove a tapped formula repository", Args: []string{"taps_installed"}},
	{Name: "update", Desc: "Fetch the newest version of Homebrew"},
	{Name: "update-report", Desc: "The output of brew update"},
	{Name: "upgrade", Desc: "Upgrade outdated packages", Args: []string{"formulae_installed", "casks_installed"},
		Flags: []brew.Flag{{Name: "--dry-run", Desc: "Show what would be upgraded"}}},
}}

func newCompletingApp(t *testing.T, fs *fakeStarter) *App {
	t.Helper()
	a := newJobApp(t, fs)
	a.comp = testCompletions
	return a
}

func candidateTexts(cs []candidate) string {
	var s []string
	for _, c := range cs {
		s = append(s, c.text)
	}
	return strings.Join(s, " ")
}

func TestCandidates(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	for _, tt := range []struct {
		line, word, want string
	}{
		{"u", "u", "uninstall untap update upgrade update-report"}, // everyday commands first
		{"brew up", "up", "update upgrade update-report"},
		{"j", "j", "jobs"},
		{"upgrade ", "", "fd git json-c lua-language-server pcre2 docker-desktop fontbase"},
		{"upgrade f", "f", "fd fontbase"},
		{"upgrade fd ", "", "git json-c lua-language-server pcre2 docker-desktop fontbase"},
		{"uninstall --", "--", "--force --zap"},
		{"services ", "", "start stop"},
		{"services st", "st", "start stop"},
		{"services start ", "", "postgresql@16 redis"},
		{"services start --", "--", "--all"},
		{"untap ", "", "anomalyco/tap homebrew/core"},
		{"info ", "", ""},
		{"frobnicate x", "x", ""},
	} {
		word, cs := a.candidates(tt.line)
		if word != tt.word || candidateTexts(cs) != tt.want {
			t.Errorf("candidates(%q) = %q, %q; want %q, %q", tt.line, word, candidateTexts(cs), tt.word, tt.want)
		}
	}
	_, cs := a.candidates("upgrade f")
	if cs[0].desc != "Simple, fast and user-friendly alternative to find" {
		t.Errorf("a package's description should come along: %q", cs[0].desc)
	}
}

func TestTypingShowsThePopup(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "up")
	out := stripANSI(a.render())
	for _, want := range []string{"update", "Fetch the newest version of Homebrew", "upgrade", "Upgrade outdated packages"} {
		if !strings.Contains(out, want) {
			t.Errorf("popup missing %q:\n%s", want, out)
		}
	}
	if h := lipgloss.Height(a.render()); h != 40 {
		t.Errorf("frame is %d lines with the popup, want 40", h)
	}
	lines := strings.Split(out, "\n")
	if !strings.HasPrefix(lines[len(lines)-2], "╰") {
		t.Errorf("the popup should sit just above the prompt:\n%s", strings.Join(lines[len(lines)-4:], "\n"))
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "tab complete") {
		t.Errorf("the prompt should hint at tab: %q", bar)
	}
}

func TestTabCyclesAndInserts(t *testing.T) {
	fs := &fakeStarter{}
	a := newCompletingApp(t, fs)
	keys(t, a, ":")
	typeText(t, a, "up")
	keys(t, a, "tab")
	if v := a.cmd.value(); v != "update" {
		t.Errorf("first tab: %q", v)
	}
	keys(t, a, "tab")
	if v := a.cmd.value(); v != "upgrade" {
		t.Errorf("second tab: %q", v)
	}
	keys(t, a, "shift+tab")
	if v := a.cmd.value(); v != "update" {
		t.Errorf("shift+tab: %q", v)
	}
	keys(t, a, "tab", " ", "l", "tab")
	if v := a.cmd.value(); v != "upgrade lua-language-server" {
		t.Errorf("completing a package: %q", v)
	}
	keys(t, a, "enter")
	if want := []string{"upgrade", "lua-language-server"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestTabCompletesAfterABang(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "!up")
	keys(t, a, "tab")
	if v := a.cmd.value(); v != "!update" {
		t.Errorf("tab after !: %q", v)
	}
}

func TestTabOnAnEmptyLineListsCommands(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	keys(t, a, ":")
	if out := stripANSI(a.render()); strings.Contains(out, "Upgrade outdated packages") {
		t.Error("the popup should wait for input or tab")
	}
	keys(t, a, "tab")
	if v := a.cmd.value(); v != "info" {
		t.Errorf("tab on an empty line should start at the first command, got %q", v)
	}
}

func TestNoPopupWithoutMatches(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	keys(t, a, ":")
	typeText(t, a, "zz")
	if len(a.cmd.items) != 0 || a.cmd.popup(a.st, a.width) != "" {
		t.Errorf("no matches, no popup: %+v", a.cmd.items)
	}
	keys(t, a, "tab")
	if v := a.cmd.value(); v != "zz" {
		t.Errorf("tab with no matches changes nothing, got %q", v)
	}
}

func TestCompletionsLoadAtStart(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseCompletions(func(context.Context) (brew.Completions, error) { return testCompletions, nil })
	drain(t, a, a.Init())
	if _, ok := a.comp.Command("upgrade"); !ok {
		t.Fatal("completions should load when the app starts")
	}

	b := newJobApp(t, &fakeStarter{})
	b.UseCompletions(func(context.Context) (brew.Completions, error) {
		return brew.Completions{}, errors.New("no fish script")
	})
	drain(t, b, b.Init())
	if _, cs := b.candidates("j"); candidateTexts(cs) != "jobs" {
		t.Errorf("without brew's completions, brewery's own commands still complete: %q", candidateTexts(cs))
	}
}

func TestPopupWindow(t *testing.T) {
	for _, tt := range []struct{ n, sel, rows, start int }{
		{5, -1, 8, 0},
		{20, -1, 8, 0},
		{20, 7, 8, 0},
		{20, 8, 8, 1},
		{20, 19, 8, 12},
	} {
		if got := popupStart(tt.n, tt.sel, tt.rows); got != tt.start {
			t.Errorf("popupStart(%d, %d, %d) = %d, want %d", tt.n, tt.sel, tt.rows, got, tt.start)
		}
	}
}

func TestCompletesFromTheCatalogue(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	a.UseCatalog(newFakeCatalog(t))
	_, load := a.Update(press(":")) // the catalogue is read when the line opens
	for _, k := range "install rip" {
		a.Update(press(string(k)))
	}
	if len(a.cmd.items) != 0 {
		t.Fatalf("suggested %v before the catalogue came", candidateTexts(a.cmd.items))
	}
	drain(t, a, load)
	if got := candidateTexts(a.cmd.items); got != "ripgrep" {
		t.Errorf("once the catalogue came: %q, want ripgrep", got)
	}
	if _, cs := a.candidates("info fo"); candidateTexts(cs) != "font-fira-code font-jetbrains-mono fontbase" {
		t.Errorf("info fo: %q", candidateTexts(cs))
	}
}

// The popup has no Tint, so its selected row keeps the theme's text color
// (TableSelected itself carries no foreground).
func TestPopupSelectedRowKeepsTextColor(t *testing.T) {
	c := cmdline{items: []candidate{{text: "install", desc: "Install"}, {text: "pin", desc: "Pin"}}, sel: 0}
	want := lipgloss.NewStyle().Foreground(testStyles.P.Text).Render("x")
	seq := strings.TrimSuffix(strings.TrimPrefix(want[:strings.Index(want, "x")], "\x1b["), "m") // SGR parameters; styles merge into one sequence
	var sel string
	for _, l := range strings.Split(c.popup(testStyles, 80), "\n") {
		if strings.Contains(l, "install") {
			sel = l
		}
	}
	if sel == "" || !strings.Contains(sel, seq) {
		t.Errorf("selected popup row %q lacks the Text foreground %q", sel, seq)
	}
}
