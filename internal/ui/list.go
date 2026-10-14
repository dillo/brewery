package ui

import (
	"fmt"
	"strconv"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
	"brewery/internal/theme"
)

// listTab is a table used by the Outdated, Services and Taps tabs, full
// width or beside a preview.
type listTab struct {
	st     theme.Styles
	name   string
	cols   []col
	empty  string // shown when there are no rows
	loaded bool   // a result has arrived, so an empty table means "nothing"
	n      int
	key    func(table.Row) string // identifies a row across refreshes
	table  table.Model
	w, h   int

	// preview, if set, fills a pane beside the table with what it returns
	// for a w×h pane.
	preview func(w, h int) (title, body string)
}

func newListTab(st theme.Styles, name string, cols []col, key func(table.Row) string, empty string) listTab {
	return listTab{st: st, name: name, cols: cols, key: key, empty: empty, table: newTable(st, cols)}
}

func (t *listTab) setRows(rows []table.Row) {
	t.n = len(rows)
	setRowsKeeping(&t.table, rows, t.key)
}

// cursor is the selected row's index into the slice the rows were built
// from, or -1 when the table is empty.
func (t *listTab) cursor() int {
	if t.n == 0 {
		return -1
	}
	return t.table.Cursor()
}

// firstCell identifies a row by its first column (service and tap names).
func firstCell(r table.Row) string { return r[0] }

// outdatedKey identifies an Outdated row by name and type.
func outdatedKey(r table.Row) string { return r[0] + "\x00" + r[3] }

func (t *listTab) update(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}

// listWidth is the table pane's width: all of it, or 55% beside a preview.
func (t *listTab) listWidth() int {
	if t.preview == nil {
		return t.w
	}
	return t.w * 55 / 100
}

func (t *listTab) resize(w, h int) {
	t.w, t.h = w, h
	iw, ih := paneInner(t.listWidth(), h)
	sizeTable(&t.table, t.cols, iw, ih)
}

// view sizes the table each time because a failed refresh shares the pane
// with a banner above it.
func (t *listTab) view(err error) string {
	w, h, listW := t.w, t.h, t.listWidth()
	iw, ih := paneInner(listW, h)
	switch {
	case err != nil && t.n == 0:
		return pane(t.st, t.name, errorBody(t.st, err), w, h, true)
	case t.n == 0 && !t.loaded:
		return pane(t.st, t.name, t.st.Muted.Render("loading…"), w, h, true)
	case t.n == 0:
		return pane(t.st, t.name, t.st.OK.Render(t.empty), w, h, true)
	}
	body := ""
	if err != nil {
		body = refreshBanner(t.st, err, iw) + "\n"
		ih -= bannerHeight
	}
	t.table.SetHeight(ih)
	list := pane(t.st, fmt.Sprintf("%s · %d", t.name, t.n), body+t.table.View(), listW, h, true)
	if t.preview == nil {
		return list
	}
	title, preview := t.preview(w-listW, h)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, pane(t.st, title, preview, w-listW, h, false))
}

func outdatedRows(items []brew.Outdated) []table.Row {
	rows := make([]table.Row, len(items))
	for i, o := range items {
		pinned := ""
		if o.Pinned {
			pinned = "pinned"
		}
		rows[i] = table.Row{o.Name, o.Installed, o.Current, o.Kind.String(), pinned}
	}
	return rows
}

func serviceRows(items []brew.Service) []table.Row {
	rows := make([]table.Row, len(items))
	for i, s := range items {
		rows[i] = table.Row{s.Name, serviceStatus(s)}
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
	outdatedCols = []col{{"NAME", 0}, {"INSTALLED", 18}, {"LATEST", 18}, {"TYPE", 7}, {"PINNED", 6}}
	serviceCols  = []col{{"NAME", 0}, {"STATUS", 16}}
	tapCols      = []col{{"NAME", 24}, {"FORMULAE", 8}, {"CASKS", 6}, {"OFFICIAL", 8}, {"REMOTE", 0}}
)
