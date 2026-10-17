package ui

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
)

// markPrefix flags a row picked with space. Row keys ignore it.
const markPrefix = "● "

// marked shows a row's name with the mark when on is true.
func marked(name string, on bool) string {
	if on {
		return markPrefix + name
	}
	return name
}

// unmarked is a name cell without its mark.
func unmarked(cell string) string { return strings.TrimPrefix(cell, markPrefix) }

// toggleMark marks or unmarks the row under the cursor on Installed or
// Outdated, then moves down, so space can mark several in a row.
func (a *App) toggleMark() tea.Cmd {
	down := tea.KeyPressMsg{Code: tea.KeyDown}
	if a.tab == tabInstalled {
		a.installed.toggleMark()
		return a.installed.update(down)
	}
	a.outdated.toggleMark()
	return a.outdated.update(down)
}

// marks is how many rows are marked on the current tab.
func (a *App) marks() int {
	switch a.tab {
	case tabInstalled:
		return len(a.installed.marked)
	case tabOutdated:
		return len(a.outdated.marked)
	}
	return 0
}

// hiddenMarks is how many marked rows the current tab isn't showing, hidden
// by its view or its "/" filter.
func (a *App) hiddenMarks() int {
	switch a.tab {
	case tabInstalled:
		return a.installed.hiddenMarks()
	case tabOutdated:
		return a.outdated.hiddenMarks()
	}
	return 0
}

// clearMarks unmarks every row on the current tab.
func (a *App) clearMarks() {
	switch a.tab {
	case tabInstalled:
		a.installed.marked = nil
		a.installed.apply()
	case tabOutdated:
		a.outdated.marked = nil
		a.outdated.apply()
	}
}

// target is a package an action names, with its kind.
type target struct {
	name string
	kind brew.Kind
}

// byKind runs one action per kind present in ts, since casks need --cask:
// formulae first. build makes the action for one kind's names.
func (a *App) byKind(ts []target, build func(kind brew.Kind, names []string) action) tea.Cmd {
	var cmds []tea.Cmd
	for _, kind := range []brew.Kind{brew.KindFormula, brew.KindCask} {
		var names []string
		for _, t := range ts {
			if t.kind == kind {
				names = append(names, t.name)
			}
		}
		if len(names) > 0 {
			cmds = append(cmds, a.run(build(kind, names)))
		}
	}
	return tea.Batch(cmds...)
}

// upgradeMarked upgrades the marked packages that brew would: outdated and
// not pinned.
func (a *App) upgradeMarked() tea.Cmd {
	var ts []target
	if a.tab == tabInstalled {
		for _, p := range a.installed.markedPackages() {
			if p.Outdated && !p.Pinned {
				ts = append(ts, target{p.FullName, p.Kind})
			}
		}
	} else {
		for _, i := range a.outdated.markedIdx() {
			if o := a.snap.Outdated[i]; !o.Pinned {
				ts = append(ts, target{o.Name, o.Kind})
			}
		}
	}
	// brew would refuse those with a dependency pinned behind: leave them out.
	var skipped, behind []string
	ts = slices.DeleteFunc(ts, func(t target) bool {
		b := a.pinnedBehind(t.name, t.kind)
		if len(b) > 0 {
			skipped, behind = append(skipped, t.name), append(behind, b...)
		}
		return len(b) > 0
	})
	why := ""
	if len(skipped) > 0 {
		why = "skipped " + strings.Join(skipped, ", ") + ": needs " + strings.Join(a.ix.PinnedBehind(behind...), ", ") + " unpinned"
	}
	if len(ts) == 0 {
		if why != "" {
			return a.note(why)
		}
		return a.note("nothing marked needs upgrading (pinned or up to date)")
	}
	a.clearMarks()
	if why != "" {
		a.flash = "" // so only what run() sets below is kept
	}
	cmd := a.byKind(ts, func(kind brew.Kind, names []string) action {
		return action{title: "upgrade " + strings.Join(names, " "), args: kindArgs("upgrade", kind, names...), refresh: diskQueries}
	})
	if why != "" {
		note := a.st.Warn.Render(why)
		if a.flash != "" { // run() queued the job or couldn't start it
			note = a.flash + " · " + note
		}
		a.flash = note
	}
	return cmd
}

// askUninstallMarked confirms uninstalling the marked packages. If an
// unmarked package still needs one, it explains why brew would refuse.
func (a *App) askUninstallMarked() {
	pkgs := a.installed.markedPackages()
	gone := map[string]bool{}
	var names, full []string
	for _, p := range pkgs {
		gone[p.FullName] = true
		names = append(names, p.Name)
		full = append(full, p.FullName)
	}
	var blocked []string
	for _, p := range pkgs {
		var by []string
		for _, u := range a.ix.UsedBy(p.FullName) {
			if !gone[u] {
				by = append(by, u)
			}
		}
		if len(by) > 0 {
			blocked = append(blocked, p.Name+" is required by "+strings.Join(by, ", ")+".")
		}
	}
	if len(blocked) > 0 {
		a.confirm = &confirmBox{
			title:   "Can't uninstall these",
			body:    append(blocked, "Unmark it, or mark those too."),
			choices: []choice{{"n", "close", nil}},
		}
		return
	}
	refresh := append(slices.Clone(diskQueries), qServices)
	ts := make([]target, len(pkgs))
	for i, p := range pkgs {
		ts[i] = target{p.FullName, p.Kind}
	}
	run := func(extra []string) func() tea.Cmd {
		return func() tea.Cmd {
			a.clearMarks()
			all := slices.Clone(ts)
			for _, o := range extra {
				all = append(all, target{o, brew.KindFormula}) // only formulae are installed as dependencies
			}
			return a.byKind(all, func(kind brew.Kind, names []string) action {
				return action{title: "uninstall " + strings.Join(names, " "), args: kindArgs("uninstall", kind, names...), refresh: refresh}
			})
		}
	}
	box := &confirmBox{
		title:   fmt.Sprintf("Uninstall %d packages?", len(pkgs)),
		body:    []string{strings.Join(names, ", ")},
		choices: []choice{{"y", "uninstall", run(nil)}},
	}
	if orphans := a.ix.Orphans(full...); len(orphans) > 0 {
		noun := "dependency"
		if len(orphans) > 1 {
			noun = "dependencies"
		}
		box.body = append(box.body, "", fmt.Sprintf("That leaves %d %s nothing else needs:", len(orphans), noun), strings.Join(orphans, ", "))
		box.choices = append(box.choices, choice{"o", "uninstall + orphans", run(orphans)})
	}
	box.choices = append(box.choices, choice{"n", "cancel", nil})
	a.confirm = box
}

// markedPackages are the marked packages, by name.
func (t *installedTab) markedPackages() []brew.Package {
	var out []brew.Package
	for _, p := range t.all {
		if t.marked[pkgKey(p)] {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b brew.Package) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// pkgKey identifies a package by name and kind: a formula and a cask can
// share a name.
func pkgKey(p brew.Package) string { return p.Name + "\x00" + p.Kind.String() }

// toggleMark marks or unmarks the selected package.
func (t *installedTab) toggleMark() {
	p, ok := t.selected()
	if !ok {
		return
	}
	if t.marked == nil {
		t.marked = map[string]bool{}
	}
	if k := pkgKey(p); t.marked[k] {
		delete(t.marked, k)
	} else {
		t.marked[k] = true
	}
	t.apply()
}

// toggleMark marks or unmarks the row under the cursor.
func (t *listTab) toggleMark() {
	row := t.table.SelectedRow()
	if t.n == 0 || row == nil {
		return
	}
	if t.marked == nil {
		t.marked = map[string]bool{}
	}
	if k := t.key(row); t.marked[k] {
		delete(t.marked, k)
	} else {
		t.marked[k] = true
	}
	t.apply()
}

// hiddenMarks is how many marked packages aren't among the rows.
func (t *installedTab) hiddenMarks() int {
	shown := 0
	for _, p := range t.rows {
		if t.marked[pkgKey(p)] {
			shown++
		}
	}
	return len(t.marked) - shown
}

// hiddenMarks is how many marked rows the filter hides.
func (t *listTab) hiddenMarks() int {
	shown := 0
	for _, i := range t.idx {
		if t.marked[t.key(t.all[i])] {
			shown++
		}
	}
	return len(t.marked) - shown
}

// markedIdx are the marked rows' indexes into the slice the rows were
// built from.
func (t *listTab) markedIdx() []int {
	var out []int
	for i, r := range t.all {
		if t.marked[t.key(r)] {
			out = append(out, i)
		}
	}
	return out
}

// pruneMarks drops marks on rows that are gone.
func pruneMarks(m map[string]bool, keys func(yield func(string) bool)) {
	have := map[string]bool{}
	for k := range keys {
		have[k] = true
	}
	for k := range maps.Keys(m) {
		if !have[k] {
			delete(m, k)
		}
	}
}

// markRow returns r with its first cell marked when on is true.
func markRow(r table.Row, on bool) table.Row {
	if !on {
		return r
	}
	r = slices.Clone(r)
	r[0] = marked(r[0], true)
	return r
}
