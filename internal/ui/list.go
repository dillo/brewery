package ui

import (
	"fmt"
	"strconv"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/theme"
)

// listTab is a full-width table used by the Outdated, Services and Taps tabs.
type listTab struct {
	st     theme.Styles
	name   string
	cols   []col
	empty  string // shown when there are no rows
	loaded bool   // a result has arrived, so an empty table means "nothing"
	n      int
	table  table.Model
	w, h   int
}

func newListTab(st theme.Styles, name string, cols []col, empty string) listTab {
	return listTab{st: st, name: name, cols: cols, empty: empty, table: newTable(st, cols)}
}

func (t *listTab) setRows(rows []table.Row) {
	t.n = len(rows)
	setRows(&t.table, rows)
}

func (t *listTab) update(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}

func (t *listTab) resize(w, h int) {
	t.w, t.h = w, h
	iw, ih := paneInner(w, h)
	sizeTable(&t.table, t.cols, iw, ih)
}

// view sizes the table each time because a failed refresh shares the pane
// with a banner above it.
func (t *listTab) view(err error) string {
	w, h := t.w, t.h
	iw, ih := paneInner(w, h)
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
	return pane(t.st, fmt.Sprintf("%s · %d", t.name, t.n), body+t.table.View(), w, h, true)
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
		status := s.Status
		if s.ExitCode != nil && *s.ExitCode != 0 {
			status += " (exit " + strconv.Itoa(*s.ExitCode) + ")"
		}
		rows[i] = table.Row{s.Name, status, s.User, s.File}
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
	serviceCols  = []col{{"NAME", 20}, {"STATUS", 16}, {"USER", 10}, {"FILE", 0}}
	tapCols      = []col{{"NAME", 24}, {"FORMULAE", 8}, {"CASKS", 6}, {"OFFICIAL", 8}, {"REMOTE", 0}}
)
