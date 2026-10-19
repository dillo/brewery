package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
)

// askReinstall confirms reinstalling the marked packages, or else the
// selected one.
func (a *App) askReinstall() {
	if a.marks() > 0 {
		a.askReinstallMarked()
		return
	}
	p, ok := a.installed.selected()
	if !ok {
		return
	}
	act := action{title: "reinstall " + p.Name, args: kindArgs("reinstall", p.Kind, p.FullName), refresh: diskQueries}
	a.confirm = &confirmBox{
		title: fmt.Sprintf("Reinstall %s %s?", p.Name, p.Version),
		body:  []string{"brew removes it and installs it again."},
		choices: []choice{
			{"y", "reinstall", func() tea.Cmd { return a.run(act) }},
			{"n", "cancel", nil},
		},
	}
}

// askReinstallMarked confirms reinstalling the marked packages: one job
// per kind, formulae first. The marks clear once it starts.
func (a *App) askReinstallMarked() {
	pkgs := a.installed.markedPackages()
	names := make([]string, len(pkgs))
	ts := make([]target, len(pkgs))
	for i, p := range pkgs {
		names[i], ts[i] = p.Name, target{p.FullName, p.Kind}
	}
	run := func() tea.Cmd {
		a.clearMarks()
		return a.byKind(ts, func(kind brew.Kind, names []string) action {
			return action{title: "reinstall " + strings.Join(names, " "), args: kindArgs("reinstall", kind, names...), refresh: diskQueries}
		})
	}
	a.confirm = &confirmBox{
		title:   fmt.Sprintf("Reinstall %d packages?", len(pkgs)),
		body:    []string{strings.Join(names, ", ")},
		choices: []choice{{"y", "reinstall", run}, {"n", "cancel", nil}},
	}
}

// linkKey links the selected formula, or unlinks it if it's linked.
// Unlinking asks first, and so does forcing a keg-only formula's link.
func (a *App) linkKey() tea.Cmd {
	p, ok := a.installed.selected()
	switch {
	case !ok:
		return nil
	case p.Kind == brew.KindCask:
		return a.note("casks aren't linked")
	case p.Linked:
		act := action{title: "unlink " + p.Name, args: []string{"unlink", p.FullName}, refresh: []query{qInstalled}}
		a.confirm = &confirmBox{
			title: "Unlink " + p.Name + "?",
			body:  []string{"Its commands leave your PATH until you link it again."},
			choices: []choice{
				{"y", "unlink", func() tea.Cmd { return a.run(act) }},
				{"n", "cancel", nil},
			},
		}
		return nil
	case p.KegOnly:
		act := action{title: "link --force " + p.Name, args: []string{"link", "--force", p.FullName}, refresh: []query{qInstalled}}
		a.confirm = &confirmBox{
			title: p.Name + " is keg-only",
			body:  []string{"brew won't link it unless forced, as it", "can clash with macOS or another formula."},
			choices: []choice{
				{"f", "link --force", func() tea.Cmd { return a.run(act) }},
				{"n", "cancel", nil},
			},
		}
		return nil
	}
	return a.run(action{title: "link " + p.Name, args: []string{"link", p.FullName}, refresh: []query{qInstalled}})
}

// fetchAction downloads the newer version of names, all of kind. The
// download cache grows, so Cleanup's dry run is measured again.
func fetchAction(kind brew.Kind, names []string) action {
	return action{title: "fetch " + strings.Join(names, " "), args: kindArgs("fetch", kind, names...), refresh: []query{qCleanup}}
}

// fetchKey downloads the newer version of the marked packages, or else of
// the selected one, so a later upgrade needn't wait for it.
func (a *App) fetchKey() tea.Cmd {
	if a.marks() > 0 {
		return a.fetchMarked()
	}
	switch a.tab {
	case tabInstalled:
		p, ok := a.installed.selected()
		if !ok {
			return nil
		}
		if !p.Outdated {
			return a.note(p.Name + " is up to date")
		}
		return a.run(fetchAction(p.Kind, []string{p.FullName}))
	case tabOutdated:
		if i := a.outdated.cursor(); i >= 0 {
			o := a.snap.Outdated[i]
			return a.run(fetchAction(o.Kind, []string{o.Name}))
		}
	}
	return nil
}

// fetchMarked downloads the newer version of every marked package that is
// outdated, pinned or not, as a download upgrades nothing. The marks stay,
// so u can follow on the same set.
func (a *App) fetchMarked() tea.Cmd {
	var ts []target
	if a.tab == tabInstalled {
		for _, p := range a.installed.markedPackages() {
			if p.Outdated {
				ts = append(ts, target{p.FullName, p.Kind})
			}
		}
	} else {
		for _, i := range a.outdated.markedIdx() {
			o := a.snap.Outdated[i]
			ts = append(ts, target{o.Name, o.Kind})
		}
	}
	if len(ts) == 0 {
		return a.note("nothing marked is outdated")
	}
	return a.byKind(ts, fetchAction)
}
