package ui

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/analytics"
	"brewery/internal/brew"
	"brewery/internal/theme"
)

// Catalog is brew's catalogue of everything it can install, for Discover.
// *brew.Client satisfies it; tests use a fake.
type Catalog interface {
	Catalogue(ctx context.Context) ([]brew.Entry, error)
	NewSinceUpdate(ctx context.Context) ([]brew.Entry, error)
	Details(ctx context.Context, name string, kind brew.Kind) (brew.Details, error)
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
// it opens, and the analytics as wantStats says.
func (a *App) showTab(t int) tea.Cmd {
	a.tab = t
	if t != tabDiscover {
		return nil
	}
	var cat tea.Cmd
	if !a.discover.loaded && a.discover.err == nil {
		cat = a.readCatalogue()
	}
	return tea.Batch(cat, a.wantStats())
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
	discTools discoverView = iota
	discApps
	discRising
	discNew
	discFonts
	discAll
)

// discoverViews are Discover's views, in the order f steps through them.
var discoverViews = [...]struct{ name, short, about string }{
	discTools:  {"Popular tools", "Tools", "The most-installed tools of the last 30 days."},
	discApps:   {"Popular apps", "Apps", "The most-installed apps of the last 30 days."},
	discRising: {"Rising", "Rising", "Growing fastest: the last 30 days against the 60 before."},
	discNew:    {"New", "New", "Added to Homebrew since brew last updated."},
	discFonts:  {"Fonts", "Fonts", "Fonts, the most installed first."},
	discAll:    {"All", "All", "Everything brew can install. / searches names and descriptions."},
}

// ranked reports whether view v is ordered by the analytics.
func (v discoverView) ranked() bool { return v != discNew && v != discAll }

// Descriptions keep 20 columns while names and the rest give way.
var (
	rankedCols = []col{{"#", 3, 0}, {"NAME", 20, 12}, {"DESCRIPTION", 0, 20}, {"INSTALLS", 8, 0}}
	plainCols  = []col{{"NAME", 20, 12}, {"TYPE", 9, 4}, {"DESCRIPTION", 0, 20}}
)

// discoverCols are each view's columns.
var discoverCols = [...][]col{
	discTools:  rankedCols,
	discApps:   rankedCols,
	discRising: {{"#", 3, 0}, {"NAME", 20, 12}, {"TYPE", 9, 4}, {"GROWTH", 6, 0}, {"DESCRIPTION", 0, 20}},
	discNew:    plainCols,
	discFonts:  rankedCols,
	discAll:    plainCols,
}

// How many rows the Popular views and Rising show.
const (
	popularRows = 500
	risingRows  = 50
)

// entryKey identifies a package by kind and name: a formula and a cask
// can share a name.
func entryKey(name string, k brew.Kind) string { return k.String() + "/" + name }

// discRow is one package in a Discover view.
type discRow struct {
	brew.Entry
	rank     int     // its place in a ranked view, from 1; 0 for none
	installs int     // in the last 30 days; 0 if not known
	growth   float64 // Rising: how many times its installs grew
	fresh    bool    // Rising: (almost) no installs before
}

// discoverTab lists packages you could install.
type discoverTab struct {
	st      theme.Styles
	showing discoverView    // the view on screen
	cat     []brew.Entry    // everything brew can install
	added   []brew.Entry    // what brew's last update added, with descriptions
	have    map[string]bool // installed packages, by entryKey
	byView  [len(discoverViews)][]discRow
	rows    []discRow // the view's rows that pass the filter, in table order
	match   string    // the "/" filter on names and descriptions
	loaded  bool      // the catalogue has been read
	loading bool
	err     error // the last read failed
	table   table.Model
	w, h    int

	pv         scroller                // the preview, which tab or enter focuses
	details    map[string]brew.Details // what brew said about each package, by entryKey
	detailsErr map[string]error        // why brew couldn't say, by entryKey
	waiting    string                  // the package the cursor is resting on, before brew is asked
	asking     string                  // the package brew is being asked about

	stats     map[string]analytics.List // by list name
	counts    map[string]map[string]int // each list's installs by name
	statsAt   map[string]time.Time      // when each list was fetched
	statsRead bool                      // the kept copies have been read
	fetching  bool
	fetchErr  error  // the last fetch failed
	triedOn   string // the day a fetch last started, as 2006-01-02
	note      func() string
}

func newDiscoverTab(st theme.Styles) discoverTab {
	return discoverTab{
		st: st, table: newTable(st, discoverCols[discTools]),
		stats: map[string]analytics.List{}, statsAt: map[string]time.Time{},
		details: map[string]brew.Details{}, detailsErr: map[string]error{},
	}
}

// statsFrom reports whether every list was fetched on day.
func (t *discoverTab) statsFrom(day string) bool {
	for _, l := range analytics.Lists {
		if at, ok := t.statsAt[l]; !ok || at.Format(time.DateOnly) != day {
			return false
		}
	}
	return true
}

// setCatalogue takes a fresh catalogue. What the last update added comes
// as bare names: their descriptions are in the catalogue.
func (t *discoverTab) setCatalogue(entries, added []brew.Entry) {
	desc := make(map[string]string, len(entries))
	for _, e := range entries {
		desc[entryKey(e.Name, e.Kind)] = e.Desc
	}
	// brew update can bring new versions: ask brew again.
	t.details, t.detailsErr = map[string]brew.Details{}, map[string]error{}
	t.cat, t.added, t.loaded = entries, make([]brew.Entry, len(added)), true
	for i, e := range added {
		e.Desc = desc[entryKey(e.Name, e.Kind)]
		t.added[i] = e
	}
	t.rebuild()
}

// rebuild works out every view's rows from the catalogue and the
// analytics. Packages the catalogue doesn't have, such as ones from taps
// you haven't tapped, are left out.
func (t *discoverTab) rebuild() {
	byKey := make(map[string]brew.Entry, len(t.cat))
	all := make([]discRow, len(t.cat))
	for i, e := range t.cat {
		byKey[entryKey(e.Name, e.Kind)] = e
		all[i] = discRow{Entry: e}
	}
	// top is list's packages of kind that keep accepts, at most n (all if
	// n < 0), ranked by installs.
	top := func(list string, kind brew.Kind, keep func(name string) bool, n int) []discRow {
		var rows []discRow
		for _, it := range t.stats[list].Items {
			if e, ok := byKey[entryKey(it.Name, kind)]; ok && keep(it.Name) && len(rows) != n {
				rows = append(rows, discRow{Entry: e, rank: len(rows) + 1, installs: it.Count})
			}
		}
		return rows
	}
	font := func(name string) bool { return strings.HasPrefix(name, "font-") }
	t.byView[discTools] = top(analytics.Tools30, brew.KindFormula, func(string) bool { return true }, popularRows)
	t.byView[discApps] = top(analytics.Apps30, brew.KindCask, func(n string) bool { return !font(n) }, popularRows)
	fonts := top(analytics.Apps30, brew.KindCask, font, -1)
	ranked := make(map[string]bool, len(fonts))
	for _, r := range fonts {
		ranked[r.Name] = true
	}
	for _, e := range t.cat {
		if e.Kind == brew.KindCask && font(e.Name) && !ranked[e.Name] {
			fonts = append(fonts, discRow{Entry: e}) // no installs: after the ranked ones, by name
		}
	}
	t.byView[discFonts] = fonts
	t.byView[discRising] = t.rising(byKey)
	t.byView[discNew] = make([]discRow, len(t.added))
	for i, e := range t.added {
		t.byView[discNew][i] = discRow{Entry: e}
	}
	t.byView[discAll] = all
	t.counts = make(map[string]map[string]int, len(t.stats))
	for name, l := range t.stats {
		t.counts[name] = l.Counts()
	}
	t.apply()
}

// rising is the top risers among formulae and casks together. It needs
// both the 30-day and the 90-day lists.
func (t *discoverTab) rising(byKey map[string]brew.Entry) []discRow {
	type riser struct {
		analytics.Riser
		e brew.Entry
	}
	var rs []riser
	for _, src := range []struct {
		d30, d90 string
		kind     brew.Kind
	}{{analytics.Tools30, analytics.Tools90, brew.KindFormula}, {analytics.Apps30, analytics.Apps90, brew.KindCask}} {
		d30, ok30 := t.stats[src.d30]
		d90, ok90 := t.stats[src.d90]
		if !ok30 || !ok90 {
			continue
		}
		for _, r := range analytics.Rising(d30, d90) {
			if e, ok := byKey[entryKey(r.Name, src.kind)]; ok {
				rs = append(rs, riser{r, e})
			}
		}
	}
	slices.SortStableFunc(rs, func(a, b riser) int { return analytics.Compare(a.Riser, b.Riser) })
	rows := make([]discRow, min(len(rs), risingRows))
	for i := range rows {
		r := rs[i]
		rows[i] = discRow{Entry: r.e, rank: i + 1, installs: r.Count, growth: r.Growth, fresh: r.New}
	}
	return rows
}

// setHave notes which packages are installed.
func (t *discoverTab) setHave(pkgs []brew.Package) {
	t.have = make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		t.have[entryKey(p.Name, p.Kind)] = true
	}
	t.apply()
}

func (t *discoverTab) apply() {
	before := t.selectedKey()
	t.rows = t.rows[:0]
	for _, r := range t.byView[t.showing] {
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
	t.followSelection(before)
}

// followSelection starts the preview at the top when the selection has
// moved on from before.
func (t *discoverTab) followSelection(before string) {
	if t.selectedKey() != before {
		t.pv.off = 0
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
	var row table.Row
	switch t.showing {
	case discRising:
		row = table.Row{rankCell(r.rank), name, kindCell(t.st.Icons, r.Kind), growthCell(r), r.Desc}
	case discNew, discAll:
		row = table.Row{name, kindCell(t.st.Icons, r.Kind), r.Desc}
	default:
		row = table.Row{rankCell(r.rank), name, r.Desc, shortCount(r.installs)}
	}
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
	k := msg.String()
	switch {
	case k == "tab" || (k == "enter" && !t.pv.focus):
		t.pv.focus = !t.pv.focus
		return nil
	case t.pv.focus && k != "f" && k != "F":
		sp := splitFor(t.w, t.h, true)
		_, ih := paneInner(sp.pvW, sp.pvH)
		t.pv.scroll(k, ih/2)
		return nil
	}
	if k == "f" || k == "F" {
		step := discoverView(1)
		if k == "F" {
			step = discoverView(len(discoverViews) - 1)
		}
		t.showing = (t.showing + step) % discoverView(len(discoverViews))
		t.apply()
		return nil
	}
	before := t.selectedKey()
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	t.followSelection(before)
	return cmd
}

// size fits the table to the list pane, under the bar of views.
func (t *discoverTab) size() {
	sp := splitFor(t.w, t.h, false)
	iw, ih := paneInner(sp.listW, sp.listH)
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
	sp := splitFor(w, h, t.pv.focus)
	if !t.loaded {
		sp = split{listW: w, listH: h} // nothing to preview yet
	}
	iw, ih := paneInner(sp.listW, sp.listH)
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
	if t.note != nil && t.showing.ranked() {
		if n := t.note(); n != "" {
			title += " · " + n
		}
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
	list := pane(t.st, title, body, sp.listW, sp.listH, !t.pv.focus)
	if sp.pvW == 0 {
		return list
	}
	previewTitle, preview := "Preview", t.st.Muted.Render("nothing in this view")
	if r, ok := t.selected(); ok {
		_, ph := paneInner(sp.pvW, sp.pvH)
		previewTitle, preview = r.Name, t.pv.window(t.preview(r), ph)
	}
	return sp.join(list, pane(t.st, previewTitle, preview, sp.pvW, sp.pvH, t.pv.focus))
}

// empty says why the current view has no rows.
func (t *discoverTab) empty() string {
	switch {
	case t.showing == discNew:
		return "nothing new since brew last updated"
	case t.showing == discAll:
		return "brew's catalogue is empty"
	case t.fetching:
		return "fetching analytics from formulae.brew.sh…"
	case t.fetchErr != nil:
		return "couldn't reach formulae.brew.sh · r retries"
	case len(t.stats) == 0:
		return "no analytics yet · r fetches them"
	}
	return "nothing here yet"
}

// rankCell is a row's place in a ranked view, blank for none.
func rankCell(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// viewBar is the bar of Discover's views with their counts.
func (t *discoverTab) viewBar(w int) string {
	segs := make([]viewSeg, len(discoverViews))
	for v, d := range discoverViews {
		segs[v] = viewSeg{name: d.name, short: d.short, n: len(t.byView[v])}
	}
	return viewBar(t.st, segs, int(t.showing), discoverViews[t.showing].about, w)
}
