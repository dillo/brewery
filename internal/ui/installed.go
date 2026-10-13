package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
	"brewery/internal/theme"
)

type installedFilter int

const (
	filterLeaves installedFilter = iota
	filterAll
	filterCasks
	filterDeps
)

var filterNames = [...]string{"leaves", "all", "casks", "deps"}

var installedCols = []col{{"NAME", 0}, {"VERSION", 16}, {"TYPE", 7}, {"STATUS", 10}}

// installedTab lists installed packages with a details preview.
type installedTab struct {
	st     theme.Styles
	filter installedFilter
	all    []brew.Package
	ix     *state.Index
	rows   []brew.Package // the filtered rows, in table order
	loaded bool
	table  table.Model
	w, h   int
}

func newInstalledTab(st theme.Styles) installedTab {
	return installedTab{st: st, table: newTable(st, installedCols), ix: state.NewIndex(nil)}
}

func (t *installedTab) setData(pkgs []brew.Package, ix *state.Index) {
	t.all, t.ix = pkgs, ix
	t.apply()
}

func (t *installedTab) apply() {
	t.rows = t.rows[:0]
	for _, p := range t.all {
		keep := false
		switch t.filter {
		case filterAll:
			keep = true
		case filterLeaves:
			keep = t.ix.IsLeaf(p)
		case filterCasks:
			keep = p.Kind == brew.KindCask
		case filterDeps:
			keep = p.Kind == brew.KindFormula && !p.OnRequest
		}
		if keep {
			t.rows = append(t.rows, p)
		}
	}
	sort.SliceStable(t.rows, func(i, j int) bool {
		if t.rows[i].Name != t.rows[j].Name {
			return t.rows[i].Name < t.rows[j].Name
		}
		return t.rows[i].Kind < t.rows[j].Kind
	})
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{p.Name, p.Version, p.Kind.String(), status(p)}
	}
	setRowsKeeping(&t.table, rows, installedKey)
}

// installedKey identifies an Installed row by name and type: a formula
// and a cask can share a name.
func installedKey(r table.Row) string { return r[0] + "\x00" + r[2] }

func status(p brew.Package) string {
	var s []string
	if p.Pinned {
		s = append(s, "pinned")
	}
	if p.Outdated {
		s = append(s, "↑")
	}
	return strings.Join(s, " ")
}

func (t *installedTab) selected() (brew.Package, bool) {
	c := t.table.Cursor()
	if c < 0 || c >= len(t.rows) {
		return brew.Package{}, false
	}
	return t.rows[c], true
}

func (t *installedTab) update(msg tea.KeyPressMsg) tea.Cmd {
	if msg.String() == "f" {
		t.filter = (t.filter + 1) % installedFilter(len(filterNames))
		t.apply() // keeps the selected package if the new filter shows it
		return nil
	}
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}

// resize must run before key handling: the table ignores navigation
// until it has been sized.
func (t *installedTab) resize(w, h int) {
	t.w, t.h = w, h
	iw, ih := paneInner(t.listWidth(), h)
	sizeTable(&t.table, installedCols, iw, ih)
}

func (t *installedTab) listWidth() int { return t.w * 55 / 100 }

// view sizes the table each time because a failed refresh shares the list
// pane with a banner above it.
func (t *installedTab) view(err error) string {
	w, h, listW := t.w, t.h, t.listWidth()
	title := fmt.Sprintf("Installed · %s · %d   f: filter", filterNames[t.filter], len(t.rows))
	if err != nil && len(t.all) == 0 {
		return pane(t.st, "Installed", errorBody(t.st, err), w, h, true)
	}
	iw, ih := paneInner(listW, h)
	body := ""
	if err != nil {
		body = refreshBanner(t.st, err, iw) + "\n"
		ih -= bannerHeight
	}
	t.table.SetHeight(ih)
	list := pane(t.st, title, body+t.table.View(), listW, h, true)

	p, ok := t.selected()
	previewTitle, body := "Preview", t.st.Muted.Render("nothing matches this filter")
	switch {
	case ok:
		previewTitle, body = p.Name, t.preview(p)
	case !t.loaded && err == nil:
		body = t.st.Muted.Render("loading…")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, list, pane(t.st, previewTitle, body, w-listW, h, false))
}

func (t *installedTab) preview(p brew.Package) string {
	st := t.st
	var b strings.Builder
	b.WriteString(st.Title.Render(p.Name+" "+p.Version) + "\n")
	if p.Desc != "" {
		b.WriteString(st.Muted.Render(p.Desc) + "\n")
	}
	b.WriteString("\n")

	how := "as a dependency"
	if p.OnRequest {
		how = "on request"
	}
	if !p.InstalledAt.IsZero() {
		how += " · " + p.InstalledAt.Format("2006-01-02")
	}
	latest := p.Latest
	if p.Outdated {
		latest = st.Warn.Render(p.Latest + " ↑")
	}
	pinned := ""
	if p.Pinned {
		pinned = "yes"
	}
	deprecated := ""
	if p.Deprecated {
		deprecated = st.Warn.Render("yes")
	}
	b.WriteString(kv(st, "type", p.Kind.String(), "tap", p.Tap, "installed", how,
		"latest", latest, "pinned", pinned, "deprecated", deprecated, "homepage", p.Homepage))

	if p.Kind == brew.KindFormula {
		b.WriteString("\n" + st.Accent.Render("Dependencies") + "\n")
		if len(p.DirectDeps) == 0 {
			b.WriteString(st.Muted.Render("none") + "\n")
		} else {
			b.WriteString(strings.Join(t.ix.Tree(p.FullName), "\n") + "\n")
		}
		b.WriteString("\n" + st.Accent.Render("Used by") + "\n")
		if used := t.ix.UsedBy(p.FullName); len(used) == 0 {
			b.WriteString(st.Muted.Render("nothing installed depends on this") + "\n")
		} else {
			b.WriteString(strings.Join(used, ", ") + "\n")
		}
	}
	if p.Caveats != "" {
		b.WriteString("\n" + st.Accent.Render("Caveats") + "\n" + p.Caveats + "\n")
	}
	return b.String()
}
