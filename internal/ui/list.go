package ui

import (
	"fmt"
	"strconv"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/theme"
)

// listTab is a table used by the Outdated, Services and Taps tabs, full
// width or beside a preview.
type listTab struct {
	st     theme.Styles
	name   string
	cols   []col
	empty  string                 // shown when there are no rows
	loaded bool                   // a result has arrived, so an empty table means "nothing"
	n      int                    // rows shown
	key    func(table.Row) string // identifies a row across refreshes
	table  table.Model
	w, h   int

	all      []table.Row     // every row, before the filter
	idx      []int           // for each row shown, its index in all
	match    string          // the "/" filter
	matchCol int             // the column the filter looks in
	marked   map[string]bool // rows picked with space, by key

	// preview, if set, fills a pane beside the table with what it returns
	// for a w×h pane.
	preview func(w, h int) (title, body string)
	pvOpen  bool // when the list would stand alone, the preview shows instead of it
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
func outdatedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[3] }

func (t *listTab) update(msg tea.KeyPressMsg) tea.Cmd {
	if k := msg.String(); t.previewAlone() && (k == "tab" || k == "enter") {
		t.pvOpen = !t.pvOpen // the list stands alone: the preview takes turns with it
		return nil
	}
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}

// title names the list, counts what it shows, and names its filter.
func (t *listTab) title() string {
	s := fmt.Sprintf("%s · %d", t.name, t.n)
	if t.match != "" {
		s += " · /" + t.match
	}
	return s
}

// layout is how the table and any preview share the tab.
func (t *listTab) layout() split {
	if t.preview == nil {
		return split{listW: t.w, listH: t.h}
	}
	return splitFor(t.w, t.h, t.pvOpen)
}

// previewAlone reports whether the tab has a preview that the layout
// doesn't show beside or under the list, so it takes turns with the list.
func (t *listTab) previewAlone() bool {
	return t.preview != nil && splitFor(t.w, t.h, false).pvW == 0
}

func (t *listTab) resize(w, h int) {
	t.w, t.h = w, h
	sp := t.layout()
	if sp.listW == 0 {
		sp = splitFor(w, h, false) // the table keeps its size while the preview shows
	}
	iw, ih := paneInner(sp.listW, sp.listH)
	sizeTable(&t.table, t.cols, iw, ih)
}

// view sizes the table each time because a failed refresh shares the pane
// with a banner above it.
func (t *listTab) view(err error) string {
	w, h, sp := t.w, t.h, t.layout()
	iw, ih := paneInner(sp.listW, sp.listH)
	switch {
	case err != nil && t.n == 0:
		return pane(t.st, t.name, errorBody(t.st, err), w, h, true)
	case t.n == 0 && !t.loaded:
		return pane(t.st, t.name, t.st.Muted.Render("loading…")+"\n"+skeleton(t.st, iw, ih-1), w, h, true)
	case t.n == 0 && len(t.all) > 0:
		return pane(t.st, t.title(), t.st.Muted.Render("nothing matches /"+t.match+" · / then esc clears it"), w, h, true)
	case t.n == 0:
		return pane(t.st, t.name, t.st.OK.Render(t.empty), w, h, true)
	}
	body := ""
	if err != nil {
		body = refreshBanner(t.st, err, iw) + "\n"
		ih -= bannerHeight
	}
	t.table.SetHeight(ih)
	list := pane(t.st, t.title(), body+t.table.View(), sp.listW, sp.listH, true)
	if sp.pvW == 0 {
		return list
	}
	title, preview := t.preview(sp.pvW, sp.pvH)
	return sp.join(list, pane(t.st, title, preview, sp.pvW, sp.pvH, sp.listW == 0))
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
		rows[i] = table.Row{o.Name, o.Installed, o.Current, kindCell(ic, o.Kind), pinned}
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

func tapRows(items []brew.Tap) []table.Row {
	rows := make([]table.Row, len(items))
	for i, t := range items {
		official := ""
		if t.Official {
			official = "yes"
		}
		rows[i] = table.Row{t.Name, strconv.Itoa(t.Formulae), strconv.Itoa(t.Casks), official, t.Remote}
	}
	return rows
}

var (
	outdatedCols = []col{{"NAME", 0, 16}, {"INSTALLED", 18, 8}, {"LATEST", 18, 8}, {"TYPE", 7, 4}, {"PINNED", 6, 0}}
	serviceCols  = []col{{"NAME", 0, 16}, {"STATUS", 16, 10}}
	tapCols      = []col{{"NAME", 24, 12}, {"FORMULAE", 8, 0}, {"CASKS", 6, 0}, {"OFFICIAL", 8, 0}, {"REMOTE", 0, 0}}
)
