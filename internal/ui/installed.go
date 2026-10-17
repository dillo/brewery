package ui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/state"
	"github.com/dillo/brewery/internal/theme"
)

type installedFilter int

const (
	filterTop installedFilter = iota
	filterAll
	filterCasks
	filterDeps
	filterFormulae
	filterPinned
)

// views are the Installed tab's views, in the sidebar's order,
// with a line saying what each shows.
var views = [...]struct{ name, about string }{
	filterTop:      {"Top-level", "Things you installed that nothing else needs."},
	filterAll:      {"All", "Everything installed."},
	filterCasks:    {"Casks", "macOS apps and fonts (casks)."},
	filterDeps:     {"Dependencies", "Installed because something else needed them."},
	filterFormulae: {"Formulae", "Command-line tools and libraries (formulae)."},
	filterPinned:   {"Pinned", "Held at their version: brew upgrade skips them."},
}

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
	idle   bool // the sidebar has the focus: no pane here looks focused
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

// shows reports whether view f lists p. Casks count as top-level: brew
// doesn't record whether a cask came in for another one, so they are all
// taken as things you chose to install.
func (t *installedTab) shows(f installedFilter, p brew.Package) bool {
	switch f {
	case filterTop:
		return p.Kind == brew.KindCask || t.ix.IsLeaf(p)
	case filterCasks:
		return p.Kind == brew.KindCask
	case filterDeps:
		return p.Kind == brew.KindFormula && !p.OnRequest
	case filterFormulae:
		return p.Kind == brew.KindFormula
	case filterPinned:
		return p.Pinned
	}
	return true
}

// count is how many installed packages view f lists, before the "/" filter.
func (t *installedTab) count(f installedFilter) int {
	n := 0
	for _, p := range t.all {
		if t.shows(f, p) {
			n++
		}
	}
	return n
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
	case t.pv.focus:
		sp := splitFor(t.w, t.h, true)
		_, ih := paneInner(sp.pvW, sp.pvH)
		t.pv.scroll(k, ih/2)
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
	sizeTable(&t.table, installedCols, iw, max(ih-aboutHeight, 0))
}

// listWidth is the list pane's width with the preview closed.
func (t *installedTab) listWidth() int { return splitFor(t.w, t.h, false).listW }

// view sizes the table each time because a failed refresh shares the list
// pane with a banner above it.
func (t *installedTab) view(err error) string {
	w, h := t.w, t.h
	if err != nil && len(t.all) == 0 {
		return pane(t.st, "Installed", errorBody(t.st, err), w, h, !t.idle)
	}
	sp := splitFor(w, h, t.pv.focus)
	iw, ih := paneInner(sp.listW, sp.listH)
	body := about(t.st, views[t.filter].about, iw) + "\n"
	ih -= aboutHeight
	if err != nil {
		body += refreshBanner(t.st, err, iw) + "\n"
		ih -= bannerHeight
	}
	t.table.SetHeight(max(ih, 0))
	title := views[t.filter].name
	if t.match != "" {
		title += " · /" + t.match
	}
	rows := t.table.View()
	if !t.loaded && len(t.all) == 0 {
		rows = skeleton(t.st, iw, ih) // nothing yet: show the shape of what's coming
	}
	list := pane(t.st, title, body+rows, sp.listW, sp.listH, !t.pv.focus && !t.idle)

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
		t.writeDeps(&b, p)
	}
	if p.Caveats != "" {
		b.WriteString("\n" + st.Accent.Render("Caveats") + "\n" + p.Caveats + "\n")
	}
	return b.String()
}

// writeDeps writes p's dependency tree, any pinned dependency that will
// stop an upgrade, and what depends on p.
func (t *installedTab) writeDeps(b *strings.Builder, p brew.Package) {
	st := t.st
	b.WriteString("\n" + st.Accent.Render("Dependencies") + "\n")
	if len(p.DirectDeps) == 0 {
		b.WriteString(st.Muted.Render("none") + "\n")
	} else {
		b.WriteString(strings.Join(t.ix.Tree(p.FullName, t.depLabel), "\n") + "\n")
	}
	if behind := t.ix.PinnedBehind(p.Deps...); p.Kind == brew.KindFormula && len(behind) > 0 {
		b.WriteString(st.Warn.Render("⚠ upgrading needs "+strings.Join(behind, ", ")+" unpinned") + "\n")
	}
	b.WriteString("\n" + st.Accent.Render("Used by") + "\n")
	if used := t.ix.UsedBy(p.FullName); len(used) == 0 {
		b.WriteString(st.Muted.Render("nothing installed depends on this") + "\n")
	} else {
		b.WriteString(strings.Join(used, ", ") + "\n")
	}
}

// depLabel names a dependency in the preview's tree, marking it if it's
// pinned, and with its versions if it's pinned behind: brew won't install
// or upgrade what needs it until it's unpinned.
func (t *installedTab) depLabel(name string) string {
	d, ok := t.ix.Get(name)
	if !ok || !d.Pinned {
		return name
	}
	s := name + " " + theme.With(t.st.Icons.Pinned, "pinned")
	if d.Outdated {
		s += t.st.Warn.Render(" · " + d.Version + " → " + d.Latest)
	}
	return s
}
