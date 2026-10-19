package ui

import (
	"cmp"
	"fmt"
	"strconv"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/theme"
)

// listTab is a table used by the Outdated, Services, Taps, History and
// Maintenance tabs, full width or beside a preview.
type listTab struct {
	st     theme.Styles
	name   string
	cols   []col
	empty  string                 // shown when there are no rows
	loaded bool                   // a result has arrived, so an empty table means "nothing"
	n      int                    // rows shown
	key    func(table.Row) string // identifies a row across refreshes
	table  table.Model
	listW  int // the list pane's width; 0 hides it
	pvW    int // the preview's width; 0 hides it
	h      int

	all      []table.Row     // every row, before the filter
	idx      []int           // for each row shown, its index in all
	match    string          // the "/" filter
	matchCol int             // the column the filter looks in
	marked   map[string]bool // rows picked with space, by key

	// preview, if set, fills a pane beside the table with what it returns
	// for a w×h pane.
	preview func(w, h int) (title, body string)
	pv      scroller // where the preview is scrolled to

	note func() string // if set and not "", added to the title
}

func newListTab(st theme.Styles, name string, cols []col, key func(table.Row) string, empty string) listTab {
	return listTab{st: st, name: name, cols: cols, key: key, empty: empty, table: newTable(st, cols)}
}

func (t *listTab) setRows(rows []table.Row) {
	t.all = rows
	pruneMarks(t.marked, func(yield func(string) bool) {
		for _, r := range rows {
			if !yield(t.key(r)) {
				return
			}
		}
	})
	t.apply()
}

// apply shows the rows the filter lets through.
func (t *listTab) apply() {
	var shown []table.Row
	t.idx = t.idx[:0]
	for i, r := range t.all {
		if matches(r[t.matchCol], t.match) {
			shown = append(shown, markRow(r, t.marked[t.key(r)]))
			t.idx = append(t.idx, i)
		}
	}
	t.n = len(shown)
	setRowsKeeping(&t.table, shown, t.key)
}

func (t *listTab) matchText() string { return t.match }

func (t *listTab) setMatch(m string) {
	t.match = m
	t.apply()
}

// cursor is the selected row's index into the slice the rows were built
// from, or -1 when no row is shown.
func (t *listTab) cursor() int {
	c := t.table.Cursor()
	if t.n == 0 || c < 0 || c >= len(t.idx) {
		return -1
	}
	return t.idx[c]
}

// firstCell identifies a row by its first column (service and tap names).
func firstCell(r table.Row) string { return r[0] }

// outdatedKey identifies an Outdated row by name and type.
func outdatedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[4] }

// update moves the table's selection. A new row's preview starts at the
// top.
func (t *listTab) update(msg tea.KeyPressMsg) tea.Cmd {
	was := t.cursor()
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	if t.cursor() != was {
		t.pv.off = 0
	}
	return cmd
}

// scroll scrolls the preview by a key.
func (t *listTab) scroll(k string) {
	_, ih := paneInner(t.pvW, t.h)
	t.pv.scroll(k, ih/2)
}

// title names the list, counts what it shows, and names its filter.
func (t *listTab) title() string {
	s := fmt.Sprintf("%s · %d", t.name, t.n)
	if t.match != "" {
		s += " · /" + t.match
	}
	if t.note != nil {
		if n := t.note(); n != "" {
			s += " · " + n
		}
	}
	return s
}

// resize sizes the table for a list pane listW wide, beside a preview
// pvW wide. A hidden list is sized to the preview, so it still takes
// navigation.
func (t *listTab) resize(listW, pvW, h int) {
	t.listW, t.pvW, t.h = listW, pvW, h
	iw, ih := paneInner(cmp.Or(listW, pvW), h)
	sizeTable(&t.table, t.cols, iw, ih)
}

// view sizes the table each time because a failed refresh shares the pane
// with a banner above it.
func (t *listTab) view(err error, focus column) string {
	w, h := t.listW+t.pvW, t.h
	iw, ih := paneInner(cmp.Or(t.listW, w), h) // the state panes below span the tab
	switch {
	case err != nil && t.n == 0:
		return pane(t.st, t.name, errorBody(t.st, err), w, h, focus != colSidebar)
	case t.n == 0 && !t.loaded:
		return pane(t.st, t.name, t.st.Muted.Render("loading…")+"\n"+skeleton(t.st, iw, ih-1), w, h, focus != colSidebar)
	case t.n == 0 && len(t.all) > 0:
		return pane(t.st, t.title(), t.st.Muted.Render("nothing matches /"+t.match+" · / then esc clears it"), w, h, focus != colSidebar)
	case t.n == 0:
		return pane(t.st, t.title(), t.st.OK.Render(t.empty), w, h, focus != colSidebar) // "up to date" says as of when
	}
	body := ""
	if err != nil {
		body = refreshBanner(t.st, err, iw) + "\n"
		ih -= bannerHeight
	}
	t.table.SetHeight(ih)
	var list string
	if t.listW > 0 {
		list = pane(t.st, t.title(), body+t.table.View(), t.listW, h, focus == colList)
	}
	if t.pvW == 0 || t.preview == nil {
		return list
	}
	title, preview := t.preview(t.pvW, h)
	_, pvH := paneInner(t.pvW, h)
	preview = t.pv.window(preview, pvH)
	return joinColumns(list, pane(t.st, title, preview, t.pvW, h, focus == colDetails))
}

// kindCell is a package's TYPE cell: "formula" or "cask", after its icon.
func kindCell(ic theme.Icons, k brew.Kind) string {
	if k == brew.KindCask {
		return theme.With(ic.Cask, k.String())
	}
	return theme.With(ic.Formula, k.String())
}

func outdatedRows(ic theme.Icons, items []brew.Outdated) []table.Row {
	rows := make([]table.Row, len(items))
	for i, o := range items {
		pinned := ""
		if o.Pinned {
			pinned = theme.With(ic.Pinned, "pinned")
		}
		rows[i] = table.Row{o.Name, o.Installed, o.Current, pinned, kindCell(ic, o.Kind)}
	}
	return rows
}

func serviceRows(st theme.Styles, items []brew.Service) []table.Row {
	rows := make([]table.Row, len(items))
	for i, s := range items {
		rows[i] = table.Row{s.Name, theme.Tint(serviceColor(st, s.Status), theme.With(serviceIcon(st.Icons, s.Status), serviceStatus(s)))}
	}
	return rows
}

// tapRows are Taps' rows, with what the last tap check found.
func (a *App) tapRows() []table.Row {
	rows := make([]table.Row, len(a.snap.Taps))
	for i, t := range a.snap.Taps {
		official := ""
		if t.Official {
			official = "yes"
		}
		rows[i] = table.Row{t.Name, strconv.Itoa(t.Formulae), strconv.Itoa(t.Casks), official, a.tapColumn(t), t.Remote}
	}
	return rows
}

var (
	// PINNED comes before TYPE so a narrow window hides TYPE first: a pin
	// decides what U upgrades.
	outdatedCols = []col{{"NAME", 0, 16}, {"INSTALLED", 18, 8}, {"LATEST", 18, 8}, {"PINNED", 8, 0}, {"TYPE", 9, 4}}
	serviceCols  = []col{{"NAME", 0, 16}, {"STATUS", 18, 10}} // fits "error (exit 127)" after an icon
	tapCols      = []col{{"NAME", 24, 12}, {"FORMULAE", 8, 0}, {"CASKS", 6, 0}, {"OFFICIAL", 8, 0}, {"GITHUB", 16, -1}, {"REMOTE", 0, 0}}
)
