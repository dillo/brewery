package ui

import (
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/theme"
)

// restyle changes every part's copy of the styles with f. Each part keeps
// its own copy, so changing a.st alone would show in only some places.
func (a *App) restyle(f func(*theme.Styles)) {
	for _, st := range []*theme.Styles{&a.st, &a.installed.st, &a.outdated.st, &a.services.st, &a.taps.st, &a.jobs.st, &a.maint.st, &a.home.st, &a.drawer.st, &a.discover.st} {
		f(st)
	}
}

// UseTheme switches to the theme called name, or to the default when
// brewery has none by that name. It isn't saved: keepTheme does that.
func (a *App) UseTheme(name string) {
	t := theme.Lookup(name)
	st := theme.New(t.Palette)
	st.Icons = a.st.Icons
	a.restyle(func(s *theme.Styles) { *s = st })
	a.themeName = t.Name
	// Styles set when a part was built, and rows coloured when they were set.
	for _, tb := range []*table.Model{&a.installed.table, &a.outdated.table, &a.services.table, &a.taps.table, &a.jobs.table, &a.maint.table, &a.discover.table} {
		tb.SetStyles(tableStyles(st))
	}
	styleInput(&a.cmd.input, st, st.P.Blue)
	styleInput(&a.filterIn, st, st.P.Teal)
	a.jobs.setRows(jobRows(st, a.history, a.now()))
	a.discover.apply()
	a.sync()
	a.syncMaint()
}

// themeBox is the theme picker. The theme under the cursor is in use
// while it's open: enter keeps it, esc puts back the one from before.
type themeBox struct {
	cursor   int    // index into theme.Themes
	was      string // the theme in use when the picker opened
	gPending bool   // g was pressed: g next goes to the top
}

// themesFooter says how to use the picker.
const themesFooter = "j/k preview · enter keep · esc cancel"

// key handles a key press while the picker is open. done is true when it
// should close, and keep then says whether the theme under the cursor
// stays. Keys it doesn't use do nothing.
func (b *themeBox) key(k string) (done, keep bool) {
	n := len(theme.Themes)
	if b.gPending {
		b.gPending = false
		if k == "g" {
			b.cursor = 0
			return false, false
		}
	}
	switch k {
	case "esc", "q":
		return true, false
	case "enter":
		return true, true
	case "j", "down":
		b.cursor = (b.cursor + 1) % n
	case "k", "up":
		b.cursor = (b.cursor + n - 1) % n
	case "G", "end":
		b.cursor = n - 1
	case "home":
		b.cursor = 0
	case "g":
		b.gPending = true
	}
	return false, false
}

// view renders the picker, at most maxW columns wide: each theme by its
// title, a ✓ on the one in use when it opened, and how to use it.
func (b *themeBox) view(st theme.Styles, maxW int) string {
	inner := min(lipgloss.Width(themesFooter), max(maxW-4, 10))
	lines := []string{st.Title.Render("Theme"), ""}
	for i, t := range theme.Themes {
		label := t.Title
		if t.Name == b.was {
			label += " ✓"
		}
		mark := "  "
		if i == b.cursor {
			mark, label = st.Accent.Render("› "), st.Accent.Bold(true).Render(label)
		}
		lines = append(lines, ansi.Truncate(mark+label, inner, "…"))
	}
	lines = append(lines, "", st.Muted.Render(ansi.Truncate(themesFooter, inner, "…")))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(st.P.Mauve).
		Padding(0, 1).Render(strings.Join(lines, "\n"))
}

// UseThemeDir sets where the chosen theme is saved. Without it, a theme
// lasts until brewery quits.
func (a *App) UseThemeDir(dir string) { a.themeDir = dir }

// openThemes opens the picker on the theme in use.
func (a *App) openThemes() {
	b := &themeBox{was: a.themeName}
	for i, t := range theme.Themes {
		if t.Name == a.themeName {
			b.cursor = i
		}
	}
	a.themes = b
}

// themesKey handles a key while the picker is open: a move switches to the
// theme under the cursor, enter keeps it, esc puts the old one back.
func (a *App) themesKey(k string) tea.Cmd {
	b := a.themes
	before := b.cursor
	done, keep := b.key(k)
	switch {
	case done && keep:
		a.themes = nil
		return a.keepTheme(theme.Themes[b.cursor].Name)
	case done:
		a.cancelThemes()
	case b.cursor != before:
		a.UseTheme(theme.Themes[b.cursor].Name)
	}
	return nil
}

// cancelThemes closes the picker and puts back the theme in use when it
// opened.
func (a *App) cancelThemes() {
	if a.themes != nil {
		a.UseTheme(a.themes.was)
		a.themes = nil
	}
}

// keepTheme switches to the theme called name and saves it for next time.
// A theme that can't be saved still applies until brewery quits.
func (a *App) keepTheme(name string) tea.Cmd {
	a.UseTheme(name)
	t := theme.Lookup(name)
	if a.themeDir != "" {
		if err := theme.SaveTheme(a.themeDir, t.Name); err != nil {
			a.flash = a.st.Err.Render("✗ theme: " + t.Title + " for now; couldn't save it: " + err.Error())
			return nil
		}
	}
	a.flash = a.st.OK.Render("theme: " + t.Title)
	return nil
}

// themeCommand is :theme. With no name it opens the picker; with one it
// switches to that theme and saves it.
func (a *App) themeCommand(args []string) tea.Cmd {
	if len(args) == 0 {
		a.openThemes()
		return nil
	}
	for _, t := range theme.Themes {
		if t.Name == args[0] {
			return a.keepTheme(t.Name)
		}
	}
	return a.note("no theme called " + args[0] + " · :theme lists them")
}

// themeCandidates are the names :theme takes, each with its title.
func themeCandidates() []candidate {
	cs := make([]candidate, len(theme.Themes))
	for i, t := range theme.Themes {
		cs[i] = candidate{t.Name, t.Title}
	}
	return cs
}
