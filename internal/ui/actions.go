package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

// action is something the user asked brew to do from a pane.
type action struct {
	title   string   // shown in the drawer and status bar, e.g. "upgrade fd"
	args    []string // brew's arguments
	refresh []query  // queries to re-run when it finishes
	full    bool     // re-run every query, doctor included (after brew update)
	env     []string // added to the job's environment
	typed   bool     // typed at ":": runs as brew would, auto-update included
}

// kindArgs builds brew arguments for verb on packages of one kind.
func kindArgs(verb string, kind brew.Kind, names ...string) []string {
	args := []string{verb}
	if kind == brew.KindCask {
		args = append(args, "--cask")
	}
	return append(args, names...)
}

// packageQueries are re-run after anything that installs, removes or pins.
var packageQueries = []query{qInstalled, qOutdated}

// actionKey handles the keys that ask brew to do something. ok is false
// when k is not an action on the current tab.
func (a *App) actionKey(k string) (cmd tea.Cmd, ok bool) {
	switch {
	case k == "U":
		return a.run(action{title: "update", args: []string{"update"}, full: true}), true
	case k == "u" && (a.tab == tabInstalled || a.tab == tabOutdated):
		return a.upgradeSelected(), true
	case k == "x" && a.tab == tabInstalled:
		return a.askUninstall(), true
	case k == "p" && a.tab == tabInstalled:
		return a.togglePin(), true
	case k == "A" && a.tab == tabOutdated:
		return a.askUpgradeAll(), true
	case a.tab == tabServices && (k == "s" || k == "S" || k == "R"):
		return a.serviceKey(k), true
	case a.tab == tabServices && k == "l":
		return a.openLog(), true
	}
	return nil, false
}

// note shows a one-off hint in the status bar.
func (a *App) note(msg string) tea.Cmd {
	a.flash = a.st.Warn.Render(msg)
	return nil
}

// upgradeSelected upgrades the package under the cursor.
func (a *App) upgradeSelected() tea.Cmd {
	var name string
	var kind brew.Kind
	switch a.tab {
	case tabInstalled:
		p, ok := a.installed.selected()
		if !ok {
			return nil
		}
		if !p.Outdated {
			return a.note(p.Name + " is up to date")
		}
		if p.Pinned {
			return a.note(p.Name + " is pinned · p unpins it")
		}
		name, kind = p.FullName, p.Kind
	case tabOutdated:
		i := a.outdated.cursor()
		if i < 0 {
			return nil
		}
		o := a.snap.Outdated[i]
		if o.Pinned {
			return a.note(o.Name + " is pinned · unpin it on Installed")
		}
		name, kind = o.Name, o.Kind
	}
	return a.run(action{title: "upgrade " + name, args: kindArgs("upgrade", kind, name), refresh: packageQueries})
}

// askUninstall confirms uninstalling the selected package. If something
// still needs it, it explains why brew would refuse instead. If it leaves
// dependencies nothing else needs, it offers to remove those too.
func (a *App) askUninstall() tea.Cmd {
	p, ok := a.installed.selected()
	if !ok {
		return nil
	}
	if used := a.ix.UsedBy(p.FullName); len(used) > 0 {
		a.confirm = &confirmBox{
			title:   "Can't uninstall " + p.Name,
			body:    []string{"Required by " + strings.Join(used, ", ") + ".", "Uninstall those first, or brew will refuse."},
			choices: []choice{{"n", "close", nil}},
		}
		return nil
	}
	refresh := append(slices.Clone(packageQueries), qServices) // its service goes too
	one := action{title: "uninstall " + p.Name, args: kindArgs("uninstall", p.Kind, p.FullName), refresh: refresh}
	box := &confirmBox{
		title:   fmt.Sprintf("Uninstall %s %s?", p.Name, p.Version),
		choices: []choice{{"y", "uninstall", func() tea.Cmd { return a.run(one) }}},
	}
	if orphans := a.ix.Orphans(p.FullName); len(orphans) > 0 {
		noun := "dependency"
		if len(orphans) > 1 {
			noun = "dependencies"
		}
		box.body = []string{fmt.Sprintf("That leaves %d %s nothing else needs:", len(orphans), noun), strings.Join(orphans, ", ")}
		all := action{
			title:   fmt.Sprintf("uninstall %s + %d orphaned", p.Name, len(orphans)),
			args:    append(kindArgs("uninstall", p.Kind, p.FullName), orphans...),
			refresh: refresh,
		}
		box.choices = append(box.choices, choice{"o", "uninstall + orphans", func() tea.Cmd { return a.run(all) }})
	}
	box.choices = append(box.choices, choice{"n", "cancel", nil})
	a.confirm = box
	return nil
}

// togglePin pins the selected package, or unpins it if it is pinned.
func (a *App) togglePin() tea.Cmd {
	p, ok := a.installed.selected()
	if !ok {
		return nil
	}
	verb := "pin"
	if p.Pinned {
		verb = "unpin"
	}
	return a.run(action{title: verb + " " + p.Name, args: kindArgs(verb, p.Kind, p.FullName), refresh: packageQueries})
}

// askUpgradeAll confirms upgrading every outdated package.
func (a *App) askUpgradeAll() tea.Cmd {
	if len(a.snap.Outdated) == 0 {
		return a.note("everything is up to date")
	}
	var names []string
	pinned := 0
	for _, o := range a.snap.Outdated {
		if o.Pinned {
			pinned++ // brew skips these
			continue
		}
		names = append(names, o.Name)
	}
	n := len(names)
	if n == 0 {
		return a.note("everything outdated is pinned")
	}
	body := []string{strings.Join(names, ", ")}
	if pinned > 0 {
		body = append(body, fmt.Sprintf("%d pinned will be skipped.", pinned))
	}
	// This dialog is the confirmation, so brew 7 needn't ask again. The
	// variable, unlike --no-ask, is ignored by older brews.
	all := action{title: "upgrade all", args: []string{"upgrade"}, env: []string{"HOMEBREW_NO_ASK=1"}, refresh: packageQueries}
	a.confirm = &confirmBox{
		title: fmt.Sprintf("Upgrade all %d outdated?", n),
		body:  body,
		choices: []choice{
			{"y", "upgrade all", func() tea.Cmd { return a.run(all) }},
			{"n", "cancel", nil},
		},
	}
	return nil
}

// serviceKey starts (s), stops (S, after asking) or restarts (R) the
// selected service.
func (a *App) serviceKey(k string) tea.Cmd {
	s, ok := a.selectedService()
	if !ok {
		return nil
	}
	name := s.Name
	verb := map[string]string{"s": "start", "S": "stop", "R": "restart"}[k]
	act := action{title: verb + " " + name, args: []string{"services", verb, name}, refresh: []query{qServices}}
	if verb != "stop" {
		return a.run(act)
	}
	a.confirm = &confirmBox{
		title: "Stop " + name + "?",
		body:  []string{"It won't start at login again until you start it."},
		choices: []choice{
			{"y", "stop", func() tea.Cmd { return a.run(act) }},
			{"n", "cancel", nil},
		},
	}
	return nil
}

// actionHints are the status-bar hints for the current tab's actions.
func (a *App) actionHints() string {
	if a.jobsOpen {
		return "enter show output · esc close"
	}
	switch a.tab {
	case tabInstalled:
		return "u upgrade · x uninstall · p pin · f/F view"
	case tabOutdated:
		return "u upgrade · A upgrade all"
	case tabServices:
		return "s start · S stop · R restart · l log"
	}
	return "1-5 tabs · r refresh · U update"
}
