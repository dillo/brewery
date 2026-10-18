package ui

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/analytics"
	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/theme"
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
	if t != a.tab {
		if a.focus == colDetails && !a.tabHasDetails(t) {
			a.focus = colList // the new tab has no details: its list has the focus
		}
	}
	a.tab = t
	if t != tabDiscover {
		return nil
	}
	return tea.Batch(a.wantCatalogue(), a.wantStats())
}

// wantCatalogue reads the catalogue unless it has been read (or tried).
func (a *App) wantCatalogue() tea.Cmd {
	if a.discover.loaded || a.discover.err != nil {
		return nil
	}
	return a.readCatalogue()
}

// reloadCatalogue reads the catalogue again if Discover has read it (or
// tried to): brew update can change it.
func (a *App) reloadCatalogue() tea.Cmd {
	if a.discover.loading {
		a.discover.reloadAfter = true // read again once this read returns
		return nil
	}
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

func (a *App) gotCatalogue(m catalogueMsg) tea.Cmd {
	a.discover.loading = false
	again := a.discover.reloadAfter
	a.discover.reloadAfter = false
	if m.err != nil && a.ctx.Err() != nil {
		return nil // quitting: the read was cancelled, not broken
	}
	a.discover.err = m.err
	if m.err == nil {
		a.discover.setCatalogue(m.entries, m.added)
	}
	if a.mode == modeCommand {
		a.suggest(false) // the line can now complete any package
	}
	if again {
		return a.readCatalogue()
	}
	return nil
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

// discoverViews are Discover's views, in the sidebar's order.
var discoverViews = [...]struct{ name, about string }{
	discTools:  {"Popular tools", "The most-installed tools of the last 30 days."},
	discApps:   {"Popular apps", "The most-installed apps of the last 30 days."},
	discRising: {"Rising", "Growing fastest: the last 30 days against the 60 before."},
	discNew:    {"New", "Added to Homebrew since brew last updated."},
	discFonts:  {"Fonts", "Fonts, the most installed first."},
	discAll:    {"Search all", "Everything brew can install. / searches names and descriptions."},
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
	// reloadAfter: a refresh came while a read was in flight, so read again.
	reloadAfter bool
	err         error // the last read failed
	table       table.Model
	listW       int // the list pane's width; 0 hides it
	pvW         int // the preview's width; 0 hides it
	h           int

	pv         scroller                // where the preview is scrolled to
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
		// brew deps names a tap's packages in full: user/tap/foo.
		t.have[entryKey(p.FullName, p.Kind)] = true
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

// update moves the table's selection.
func (t *discoverTab) update(msg tea.KeyPressMsg) tea.Cmd {
	before := t.selectedKey()
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	t.followSelection(before)
	return cmd
}

// scroll scrolls the preview by a key.
func (t *discoverTab) scroll(k string) {
	_, ih := paneInner(t.pvW, t.h)
	t.pv.scroll(k, ih/2)
}

// size fits the table to the list pane, under the one-line description. A hidden
// list is sized to the preview, so it still takes navigation.
func (t *discoverTab) size() {
	iw, ih := paneInner(cmp.Or(t.listW, t.pvW), t.h)
	sizeTable(&t.table, discoverCols[t.showing], iw, max(ih-aboutHeight, 0))
}

// resize must run before key handling: the table ignores navigation
// until it has been sized.
func (t *discoverTab) resize(listW, pvW, h int) {
	t.listW, t.pvW, t.h = listW, pvW, h
	t.size()
}

func (t *discoverTab) view(focus column) string {
	if t.err != nil && !t.loaded {
		return pane(t.st, "Discover", errorBody(t.st, t.err), t.listW+t.pvW, t.h, focus != colSidebar)
	}
	listW, pvW := t.listW, t.pvW
	if !t.loaded {
		listW, pvW = listW+pvW, 0 // nothing to preview yet
	}
	iw, ih := paneInner(listW, t.h)
	body := about(t.st, discoverViews[t.showing].about, iw) + "\n"
	ih -= aboutHeight
	if t.err != nil {
		body += refreshBanner(t.st, t.err, iw) + "\n"
		ih -= bannerHeight
	}
	title := discoverViews[t.showing].name
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
	var list string
	if listW > 0 {
		list = pane(t.st, title, body, listW, t.h, focus == colList)
	}
	if pvW == 0 {
		return list
	}
	previewTitle, preview := "Preview", t.st.Muted.Render("nothing in this view")
	if r, ok := t.selected(); ok {
		_, ph := paneInner(pvW, t.h)
		previewTitle, preview = r.Name, t.pv.window(t.preview(r), ph)
	}
	return joinColumns(list, pane(t.st, previewTitle, preview, pvW, t.h, focus == colDetails))
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
		return fetchFailure(t.fetchErr) + " · r retries"
	case len(t.stats) == 0:
		return "no analytics yet · r fetches them"
	}
	return "nothing here yet"
}

// fetchFailure says why the analytics couldn't be fetched.
func fetchFailure(err error) string {
	var status *analytics.StatusError
	switch {
	case errors.As(err, &status):
		return fmt.Sprintf("formulae.brew.sh answered %d", status.Code)
	case errors.Is(err, analytics.ErrBadList):
		return "couldn't read formulae.brew.sh's lists"
	}
	return "couldn't reach formulae.brew.sh"
}

// rankCell is a row's place in a ranked view, blank for none.
func rankCell(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}
