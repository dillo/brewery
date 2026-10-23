package ui

import (
	"charm.land/bubbles/v2/table"

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
