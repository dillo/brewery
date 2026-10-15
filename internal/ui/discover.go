package ui

import (
	"context"
	"slices"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/theme"
)

// Catalog is brew's catalogue of everything it can install, for Discover.
// *brew.Client satisfies it; tests use a fake.
type Catalog interface {
	Catalogue(ctx context.Context) ([]brew.Entry, error)
	NewSinceUpdate(ctx context.Context) ([]brew.Entry, error)
}

var _ Catalog = (*brew.Client)(nil)

// UseCatalog sets where Discover's catalogue comes from.
func (a *App) UseCatalog(c Catalog) { a.catalog = c }

// catalogueMsg carries the catalogue and what brew's last update added.
type catalogueMsg struct {
	entries, added []brew.Entry
	err            error
}

func loadCatalogue(ctx context.Context, c Catalog) tea.Cmd {
	return func() tea.Msg {
		es, err := c.Catalogue(ctx)
		if err != nil {
			return catalogueMsg{err: err}
		}
		added, _ := c.NewSinceUpdate(ctx) // only a hint: without it, New is empty
		return catalogueMsg{entries: es, added: added}
	}
}

// showTab switches to tab t. Discover reads the catalogue the first time
// it opens.
func (a *App) showTab(t int) tea.Cmd {
	a.tab = t
	if t != tabDiscover || a.discover.loaded || a.discover.err != nil {
		return nil
	}
	return a.readCatalogue()
}

// reloadCatalogue reads the catalogue again if Discover has read it (or
// tried to): brew update can change it.
func (a *App) reloadCatalogue() tea.Cmd {
	if !a.discover.loaded && a.discover.err == nil {
		return nil
	}
	return a.readCatalogue()
}

func (a *App) readCatalogue() tea.Cmd {
	if a.catalog == nil || a.discover.loading {
		return nil
	}
	a.discover.loading = true
	return loadCatalogue(a.ctx, a.catalog)
}

func (a *App) gotCatalogue(m catalogueMsg) {
	a.discover.loading = false
	if m.err != nil && a.ctx.Err() != nil {
		return // quitting: the read was cancelled, not broken
	}
	a.discover.err = m.err
	if m.err == nil {
		a.discover.setCatalogue(m.entries, m.added)
	}
}

type discoverView int

const (
	discNew discoverView = iota
	discAll
)

// discoverViews are Discover's views, in the order f steps through them.
var discoverViews = [...]struct{ name, short, about string }{
	discNew: {"New", "New", "Added to Homebrew since brew last updated."},
	discAll: {"All", "All", "Everything brew can install. / searches names and descriptions."},
}

// discoverCols are each view's columns.
var discoverCols = [...][]col{
	discNew: {{"NAME", 24, 12}, {"TYPE", 9, 4}, {"DESCRIPTION", 0, 0}},
	discAll: {{"NAME", 24, 12}, {"TYPE", 9, 4}, {"DESCRIPTION", 0, 0}},
}

// entryKey identifies a package by kind and name: a formula and a cask
// can share a name.
func entryKey(name string, k brew.Kind) string { return k.String() + "/" + name }

// discRow is one package in a Discover view.
type discRow struct {
	brew.Entry
}

// discoverTab lists packages you could install.
type discoverTab struct {
	st      theme.Styles
	showing discoverView    // the view on screen
	cat     []brew.Entry    // everything brew can install
	added   []brew.Entry    // what brew's last update added, with descriptions
	have    map[string]bool // installed packages, by entryKey
	rows    []discRow       // the view's rows that pass the filter, in table order
	match   string          // the "/" filter on names and descriptions
	loaded  bool            // the catalogue has been read
	loading bool
	err     error // the last read failed
	table   table.Model
	w, h    int
}

func newDiscoverTab(st theme.Styles) discoverTab {
	return discoverTab{st: st, table: newTable(st, discoverCols[discNew])}
}

// setCatalogue takes a fresh catalogue. What the last update added comes
// as bare names: their descriptions are in the catalogue.
func (t *discoverTab) setCatalogue(entries, added []brew.Entry) {
	desc := make(map[string]string, len(entries))
	for _, e := range entries {
		desc[entryKey(e.Name, e.Kind)] = e.Desc
	}
	t.cat, t.added, t.loaded = entries, make([]brew.Entry, len(added)), true
	for i, e := range added {
		e.Desc = desc[entryKey(e.Name, e.Kind)]
		t.added[i] = e
	}
	t.apply()
}

// setHave notes which packages are installed.
func (t *discoverTab) setHave(pkgs []brew.Package) {
	t.have = make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		t.have[entryKey(p.Name, p.Kind)] = true
	}
	t.apply()
}

// entries is every package view v lists, before the filter.
func (t *discoverTab) entries(v discoverView) []brew.Entry {
	if v == discNew {
		return t.added
	}
	return t.cat
}

// source is every row view v lists, before the filter.
func (t *discoverTab) source(v discoverView) []discRow {
	es := t.entries(v)
	rows := make([]discRow, len(es))
	for i, e := range es {
		rows[i] = discRow{Entry: e}
	}
	return rows
}

func (t *discoverTab) apply() {
	before := t.selectedKey()
	t.rows = t.rows[:0]
	for _, r := range t.source(t.showing) {
		if matches(r.Name+" "+r.Desc, t.match) {
			t.rows = append(t.rows, r)
		}
	}
	rows := make([]table.Row, len(t.rows))
	for i, r := range t.rows {
		rows[i] = t.cells(r)
	}
	t.table.SetRows(nil) // the columns change with the view: old rows mustn't meet new columns
	t.size()
	setRows(&t.table, rows)
	if i := slices.IndexFunc(t.rows, func(r discRow) bool { return entryKey(r.Name, r.Kind) == before }); i >= 0 {
		t.table.SetCursor(i)
	}
}

// cells is r's row in the current view. Installed packages are marked
// and dimmed.
func (t *discoverTab) cells(r discRow) table.Row {
	name := r.Name
	have := t.have[entryKey(r.Name, r.Kind)]
	if have {
		name = "✓ " + name
	}
	row := table.Row{name, kindCell(t.st.Icons, r.Kind), r.Desc}
	if have {
		for i := range row {
			row[i] = theme.Tint(t.st.P.Overlay0, row[i])
		}
	}
	return row
}

// selected is the package under the cursor.
func (t *discoverTab) selected() (discRow, bool) {
	c := t.table.Cursor()
	if c < 0 || c >= len(t.rows) {
		return discRow{}, false
	}
	return t.rows[c], true
}

// selectedKey identifies the selected package, or is "" for none.
func (t *discoverTab) selectedKey() string {
	r, ok := t.selected()
	if !ok {
		return ""
	}
	return entryKey(r.Name, r.Kind)
}

func (t *discoverTab) matchText() string { return t.match }

func (t *discoverTab) setMatch(m string) {
	t.match = m
	t.apply()
}

func (t *discoverTab) update(msg tea.KeyPressMsg) tea.Cmd {
	if k := msg.String(); k == "f" || k == "F" {
		step := discoverView(1)
		if k == "F" {
			step = discoverView(len(discoverViews) - 1)
		}
		t.showing = (t.showing + step) % discoverView(len(discoverViews))
		t.apply()
		return nil
	}
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}

// size fits the table to the pane, under the bar of views.
func (t *discoverTab) size() {
	iw, ih := paneInner(t.w, t.h)
	sizeTable(&t.table, discoverCols[t.showing], iw, max(ih-viewBarHeight, 0))
}

// resize must run before key handling: the table ignores navigation
// until it has been sized.
func (t *discoverTab) resize(w, h int) {
	t.w, t.h = w, h
	t.size()
}

func (t *discoverTab) view() string {
	w, h := t.w, t.h
	if t.err != nil && !t.loaded {
		return pane(t.st, "Discover", errorBody(t.st, t.err), w, h, true)
	}
	iw, ih := paneInner(w, h)
	body := t.viewBar(iw) + "\n"
	ih -= viewBarHeight
	if t.err != nil {
		body += refreshBanner(t.st, t.err, iw) + "\n"
		ih -= bannerHeight
	}
	title := "Discover"
	if t.match != "" {
		title += " · /" + t.match
	}
	switch {
	case !t.loaded:
		body += t.st.Muted.Render("loading…") + "\n" + skeleton(t.st, iw, ih-1)
	case len(t.rows) == 0 && t.match != "":
		body += t.st.Muted.Render("nothing matches /" + t.match + " · / then esc clears it")
	case len(t.rows) == 0:
		body += t.st.Muted.Render(t.empty())
	default:
		t.table.SetHeight(max(ih, 0))
		body += t.table.View()
	}
	return pane(t.st, title, body, w, h, true)
}

// empty says why the current view has no rows.
func (t *discoverTab) empty() string {
	if t.showing == discNew {
		return "nothing new since brew last updated"
	}
	return "brew's catalogue is empty"
}

// viewBar is the bar of Discover's views with their counts.
func (t *discoverTab) viewBar(w int) string {
	segs := make([]viewSeg, len(discoverViews))
	for v, d := range discoverViews {
		segs[v] = viewSeg{name: d.name, short: d.short, n: len(t.entries(discoverView(v)))}
	}
	return viewBar(t.st, segs, int(t.showing), discoverViews[t.showing].about, w)
}
