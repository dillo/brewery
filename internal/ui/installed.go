package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/state"
	"brewery/internal/theme"
)

type installedFilter int

const (
	filterTop installedFilter = iota
	filterAll
	filterApps
	filterDeps
)

// views are the Installed tab's views, in the order f steps through them,
// with a short name for narrow panes and a line saying what each shows.
var views = [...]struct{ name, short, about string }{
	filterTop:  {"Top-level", "Top", "Things you installed that nothing else needs."},
	filterAll:  {"All", "All", "Everything installed."},
	filterApps: {"Apps", "Apps", "macOS apps (casks)."},
	filterDeps: {"Dependencies", "Deps", "Installed because something else needed them."},
}

// viewBarHeight is the bar of views plus the line about the active one.
const viewBarHeight = 2

var installedCols = []col{{"NAME", 0, 20}, {"VERSION", 16, 8}, {"TYPE", 9, 4}, {"STATUS", 10, 6}, {"SIZE", 8, -1}}

// installedTab lists installed packages with a details preview.
type installedTab struct {
	st     theme.Styles
	filter installedFilter
	all    []brew.Package
	ix     *state.Index
	rows   []brew.Package   // the filtered rows, in table order
	match  string           // the "/" filter on names
	pv     scroller         // the preview, which tab or enter focuses
	marked map[string]bool  // packages picked with space, by pkgKey
	sizes  map[string]int64 // bytes on disk, by Package.Key; nil until measured
	loaded bool
	table  table.Model
	w, h   int
}

func newInstalledTab(st theme.Styles) installedTab {
	return installedTab{st: st, table: newTable(st, installedCols), ix: state.NewIndex(nil)}
}

func (t *installedTab) setData(pkgs []brew.Package, ix *state.Index) {
	t.all, t.ix = pkgs, ix
	pruneMarks(t.marked, func(yield func(string) bool) {
		for _, p := range pkgs {
			if !yield(pkgKey(p)) {
				return
			}
		}
	})
	t.apply()
}

// shows reports whether view f lists p. Apps count as top-level: brew
// doesn't record whether a cask came in for another one, so they are all
// taken as things you chose to install.
func (t *installedTab) shows(f installedFilter, p brew.Package) bool {
	switch f {
	case filterTop:
		return p.Kind == brew.KindCask || t.ix.IsLeaf(p)
	case filterApps:
		return p.Kind == brew.KindCask
	case filterDeps:
		return p.Kind == brew.KindFormula && !p.OnRequest
	}
	return true
}

func (t *installedTab) apply() {
	t.rows = t.rows[:0]
	for _, p := range t.all {
		if t.shows(t.filter, p) && matches(p.Name, t.match) {
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
		rows[i] = table.Row{marked(p.Name, t.marked[pkgKey(p)]), p.Version, kindCell(t.st.Icons, p.Kind), status(t.st, p), t.size(p)}
	}
	before := t.selectedKey()
	setRowsKeeping(&t.table, rows, installedKey)
	t.followSelection(before)
}

// selectedKey identifies the selected package, or is "" for none.
func (t *installedTab) selectedKey() string {
	p, ok := t.selected()
	if !ok {
		return ""
	}
	return pkgKey(p)
}

// followSelection starts the preview at the top when the selection has
// moved on from before.
func (t *installedTab) followSelection(before string) {
	if t.selectedKey() != before {
		t.pv.off = 0
	}
}

// installedKey identifies an Installed row by name and type: a formula
// and a cask can share a name.
func installedKey(r table.Row) string { return unmarked(r[0]) + "\x00" + r[2] }

// size is p's size on disk, or "" before it has been measured.
func (t *installedTab) size(p brew.Package) string {
	if n, ok := t.sizes[p.Key()]; ok {
		return humanSize(n)
	}
	return ""
}

func status(st theme.Styles, p brew.Package) string {
	var s []string
	if p.Pinned {
		s = append(s, theme.With(st.Icons.Pinned, "pinned"))
	}
	if p.Outdated {
		s = append(s, theme.Tint(st.P.Yellow, "↑"))
	}
	return strings.Join(s, " ")
}

func (t *installedTab) matchText() string { return t.match }

func (t *installedTab) setMatch(m string) {
	t.match = m
	t.apply()
}

func (t *installedTab) selected() (brew.Package, bool) {
	c := t.table.Cursor()
	if c < 0 || c >= len(t.rows) {
		return brew.Package{}, false
	}
	return t.rows[c], true
}

func (t *installedTab) update(msg tea.KeyPressMsg) tea.Cmd {
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
		step := installedFilter(1)
		if k == "F" {
			step = installedFilter(len(views) - 1)
		}
		t.filter = (t.filter + step) % installedFilter(len(views))
		t.apply() // keeps the selected package if the new view shows it
		return nil
	}
	before := t.selectedKey()
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	t.followSelection(before)
	return cmd
}

// resize must run before key handling: the table ignores navigation
// until it has been sized.
func (t *installedTab) resize(w, h int) {
	t.w, t.h = w, h
	sp := splitFor(w, h, false)
	iw, ih := paneInner(sp.listW, sp.listH)
	sizeTable(&t.table, installedCols, iw, max(ih-viewBarHeight, 0))
}

// listWidth is the list pane's width with the preview closed.
func (t *installedTab) listWidth() int { return splitFor(t.w, t.h, false).listW }

// view sizes the table each time because a failed refresh shares the list
// pane with a banner above it.
func (t *installedTab) view(err error) string {
	w, h := t.w, t.h
	if err != nil && len(t.all) == 0 {
		return pane(t.st, "Installed", errorBody(t.st, err), w, h, true)
	}
	sp := splitFor(w, h, t.pv.focus)
	iw, ih := paneInner(sp.listW, sp.listH)
	body := t.viewBar(iw) + "\n"
	ih -= viewBarHeight
	if err != nil {
		body += refreshBanner(t.st, err, iw) + "\n"
		ih -= bannerHeight
	}
	t.table.SetHeight(max(ih, 0))
	title := "Installed"
	if t.match != "" {
		title += " · /" + t.match
	}
	rows := t.table.View()
	if !t.loaded && len(t.all) == 0 {
		rows = skeleton(t.st, iw, ih) // nothing yet: show the shape of what's coming
	}
	list := pane(t.st, title, body+rows, sp.listW, sp.listH, !t.pv.focus)

	p, ok := t.selected()
	previewTitle, body := "Preview", t.st.Muted.Render("nothing in this view")
	if t.match != "" {
		body = t.st.Muted.Render("nothing in this view matches /" + t.match)
	}
	switch {
	case ok:
		_, ph := paneInner(sp.pvW, sp.pvH)
		previewTitle, body = p.Name, t.pv.window(t.preview(p), ph)
	case !t.loaded && err == nil:
		body = t.st.Muted.Render("loading…")
	}
	return sp.join(list, pane(t.st, previewTitle, body, sp.pvW, sp.pvH, t.pv.focus))
}

// viewBar lists every view with its count, the active one highlighted,
// and under it a line saying what the active view shows. Narrow panes get
// the short names.
func (t *installedTab) viewBar(w int) string {
	bar := func(short bool) string {
		var segs []string
		for f, v := range views {
			n := 0
			for _, p := range t.all {
				if t.shows(installedFilter(f), p) {
					n++
				}
			}
			name, style := v.name, t.st.View
			if short {
				name = v.short
			}
			if installedFilter(f) == t.filter {
				style = t.st.ViewActive
			}
			segs = append(segs, style.Render(fmt.Sprintf("%s %d", name, n)))
		}
		return strings.Join(segs, " ")
	}
	b := bar(false)
	if lipgloss.Width(b) > w {
		b = bar(true)
	}
	// The keys always show, shortened before the description is: on narrow
	// terminals the status bar has no room for hints.
	about, keys := views[t.filter].about, "f/F to switch"
	if len(about)+2+len(keys) > w {
		keys = "f/F"
		about = ansi.Truncate(about, max(w-2-len(keys), 0), "…")
	}
	gap := max(w-ansi.StringWidth(about)-len(keys), 1)
	return b + "\n" + t.st.Muted.Render(about+strings.Repeat(" ", gap)+keys)
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
	b.WriteString(kv(st, "type", p.Kind.String(), "tap", p.Tap, "size", t.size(p), "installed", how,
		"latest", latest, "pinned", pinned, "deprecated", deprecated, "homepage", p.Homepage))

	if p.Kind == brew.KindFormula || len(p.DirectDeps) > 0 || len(t.ix.UsedBy(p.FullName)) > 0 {
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
