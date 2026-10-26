package ui

import (
	"cmp"
	"slices"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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
	filterDeprecated
	filterSkunky
)

// views are the Installed tab's views, in the sidebar's order,
// with a line saying what each shows.
var views = [...]struct{ name, about string }{
	filterTop:        {"Top-level", "Things you installed that nothing else needs."},
	filterAll:        {"All", "Everything installed."},
	filterCasks:      {"Casks", "macOS apps and fonts (casks)."},
	filterDeps:       {"Dependencies", "Installed because something else needed them."},
	filterFormulae:   {"Formulae", "Command-line tools and libraries (formulae)."},
	filterPinned:     {"Pinned", "Held at their version: brew upgrade skips them."},
	filterDeprecated: {"Deprecated", "On their way out: deprecated, or disabled and no longer installable."},
	filterSkunky:     {"Skunky", "Installed formulae with known security problems, worst first."},
}

var installedCols = []col{{"NAME", 0, 20}, {"VERSION", 16, 8}, {"TYPE", 9, 4}, {"STATUS", 10, 6}, {"SIZE", 8, -1}}

// installedTab lists installed packages with a details preview.
type installedTab struct {
	st                  theme.Styles
	filter              installedFilter
	all                 []brew.Package
	ix                  *state.Index
	rows                []brew.Package              // the filtered rows, in table order
	match               string                      // the "/" filter on names
	pv                  scroller                    // where the preview is scrolled to
	marked              map[string]bool             // packages picked with space, by pkgKey
	sizes               map[string]int64            // bytes on disk, by Package.Key; nil until measured
	vulns               map[string]brew.VulnFinding // the last scan's findings, by formula; nil before one
	now                 func() time.Time            // today, for the disable date
	scanNote, scanEmpty func() string               // Skunky's notes, from the App; nil in tests that don't set them
	loaded              bool
	table               table.Model
	listW               int // the list pane's width; 0 hides it
	pvW                 int // the preview's width; 0 hides it
	h                   int
}

func newInstalledTab(st theme.Styles) installedTab {
	return installedTab{st: st, table: newTable(st, installedCols), now: time.Now, ix: state.NewIndex(nil)}
}

// setStyles draws the tab, its table included, with st from now on.
func (t *installedTab) setStyles(st theme.Styles) {
	t.st = st
	t.table.SetStyles(tableStyles(st))
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
	case filterDeprecated:
		return retireState(p) != ""
	case filterSkunky:
		_, ok := t.finding(p)
		return ok
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
	before := t.selectedKey()
	t.rows = t.rows[:0]
	for _, p := range t.all {
		if t.shows(t.filter, p) && matches(p.Name, t.match) {
			t.rows = append(t.rows, p)
		}
	}
	sort.SliceStable(t.rows, t.less)
	rows := make([]table.Row, len(t.rows))
	for i, p := range t.rows {
		rows[i] = table.Row{marked(p.Name, t.marked[pkgKey(p)]), p.Version, t.typeCell(p), status(t.st, p), t.size(p)}
	}
	if t.h > 0 {
		t.resize(t.listW, t.pvW, t.h) // the view's columns may differ
	}
	setRows(&t.table, rows)
	t.selectKey(before)
	t.followSelection(before)
}

// less orders rows by name, a formula before a cask of the same name; on
// Skunky, by worst finding first.
func (t *installedTab) less(i, j int) bool {
	p, q := t.rows[i], t.rows[j]
	if t.filter == filterSkunky {
		if rp, rq := t.worstRank(p), t.worstRank(q); rp != rq {
			return rp < rq
		}
	}
	if p.Name != q.Name {
		return p.Name < q.Name
	}
	return p.Kind < q.Kind
}

// selectKey moves the cursor to the package whose pkgKey is key, if the
// view lists it.
func (t *installedTab) selectKey(key string) {
	for i, p := range t.rows {
		if pkgKey(p) == key {
			t.table.SetCursor(i)
			return
		}
	}
}

// cols are Installed's columns. On Skunky, every row is a formula, so
// TYPE gives way to SEVERITY.
func (t *installedTab) cols() []col {
	if t.filter != filterSkunky {
		return installedCols
	}
	c := slices.Clone(installedCols)
	c[2] = col{"SEVERITY", 9, 8} // CRITICAL and SEVERITY are 8 cells: never squeezed
	return c
}

// typeCell is the TYPE column's cell: on Skunky, the worst finding's
// severity.
func (t *installedTab) typeCell(p brew.Package) string {
	if t.filter == filterSkunky {
		f, _ := t.finding(p)
		return severityText(t.st, worst(f))
	}
	return kindCell(t.st.Icons, p.Kind)
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
	switch retireState(p) {
	case "deprecated":
		s = append(s, theme.Tint(st.P.Yellow, "⚠"))
	case "disabled":
		s = append(s, theme.Tint(st.P.Red, "✗"))
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

// update moves the table's selection.
func (t *installedTab) update(msg tea.KeyPressMsg) tea.Cmd {
	before := t.selectedKey()
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	t.followSelection(before)
	return cmd
}

// scroll scrolls the preview by a key.
func (t *installedTab) scroll(k string) {
	_, ih := paneInner(t.pvW, t.h)
	t.pv.scroll(k, ih/2)
}

// resize must run before key handling: the table ignores navigation
// until it has been sized. A hidden list is sized to the preview, so it
// still takes navigation.
func (t *installedTab) resize(listW, pvW, h int) {
	t.listW, t.pvW, t.h = listW, pvW, h
	iw, ih := paneInner(cmp.Or(listW, pvW), h)
	sizeTable(&t.table, t.cols(), iw, max(ih-aboutHeight, 0))
}

// scanFooter is Skunky's coverage note under the list, fitted to width w;
// "" in other views or when there's no note.
func (t *installedTab) scanFooter(w int) string {
	if t.filter != filterSkunky || t.scanNote == nil {
		return ""
	}
	n := t.scanNote()
	if n == "" {
		return ""
	}
	if ansi.StringWidth(n) > w {
		n = strings.TrimSuffix(n, casksClause) // keep the counts whole
	}
	return "\n" + about(t.st, n, w)
}

// view sizes the table each time because a failed refresh shares the list
// pane with a banner above it.
func (t *installedTab) view(err error, focus column) string {
	if err != nil && len(t.all) == 0 {
		return pane(t.st, "Installed", errorBody(t.st, err), t.listW+t.pvW, t.h, focus != colSidebar)
	}
	iw, ih := paneInner(t.listW, t.h)
	body := about(t.st, views[t.filter].about, iw) + "\n"
	ih -= aboutHeight
	if err != nil {
		body += refreshBanner(t.st, err, iw) + "\n"
		ih -= bannerHeight
	}
	footer := t.scanFooter(iw)
	if footer != "" {
		ih--
	}
	t.table.SetHeight(max(ih, 0))
	title := views[t.filter].name
	if t.match != "" {
		title += " · /" + t.match
	}
	rows := t.table.View()
	if !t.loaded && len(t.all) == 0 {
		rows = skeleton(t.st, iw, ih) // nothing yet: show the shape of what's coming
	} else if len(t.rows) == 0 {
		rows = t.st.Muted.Render(t.emptyNote()) // no details column to say it
	}
	var list string
	if t.listW > 0 {
		list = pane(t.st, title, body+rows+footer, t.listW, t.h, focus == colList)
	}
	if t.pvW == 0 {
		return list
	}

	// A details column exists only while the view has rows, so one is selected.
	p, _ := t.selected()
	_, ph := paneInner(t.pvW, t.h)
	body = t.pv.window(t.preview(p), ph)
	previewTitle := p.Name
	return joinColumns(list, pane(t.st, previewTitle, body, t.pvW, t.h, focus == colDetails))
}

// emptyNote says why the view shows no rows.
func (t *installedTab) emptyNote() string {
	if t.match != "" {
		return "nothing in this view matches /" + t.match
	}
	if t.filter == filterSkunky && t.scanEmpty != nil {
		return t.scanEmpty()
	}
	if t.filter == filterDeprecated {
		return "nothing installed is deprecated or disabled"
	}
	return "nothing in this view"
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
	pairs := []string{"type", p.Kind.String(), "tap", p.Tap, "size", t.size(p), "installed", how,
		"latest", latest, "pinned", pinned, "linked", linkedText(p)}
	pairs = append(pairs, retireRows(st, p, t.now())...)
	pairs = append(pairs, "homepage", p.Homepage)
	b.WriteString(kv(st, pairs...))
	t.writeVulns(&b, p)

	if p.Kind == brew.KindFormula || len(p.DirectDeps) > 0 || len(t.ix.UsedBy(p.FullName)) > 0 {
		t.writeDeps(&b, p)
	}
	if p.Caveats != "" {
		b.WriteString("\n" + st.Accent.Render("Caveats") + "\n" + p.Caveats + "\n")
	}
	return b.String()
}

// linkedText is the details' linked row for p: empty for a cask, which
// brew doesn't link.
func linkedText(p brew.Package) string {
	switch {
	case p.Kind == brew.KindCask:
		return ""
	case p.Linked:
		return "yes"
	case p.KegOnly:
		return "no · keg-only"
	}
	return "no"
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
