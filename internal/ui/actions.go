package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
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

// packageQueries are re-run after anything that installs, removes or pins;
// diskQueries after anything that also changes what's on disk, and so what
// autoremove would remove.
var (
	packageQueries = []query{qInstalled, qOutdated}
	diskQueries    = []query{qInstalled, qOutdated, qSizes, qAutoremove}
)

// actionKey handles the keys that ask brew to do something. ok is false
// when k is not an action on the current tab.
func (a *App) actionKey(k string) (cmd tea.Cmd, ok bool) {
	switch {
	case k == "U":
		return a.brewUpdate(), true
	case k == "c":
		return a.askCleanup(), true
	case k == "space" && (a.tab == tabInstalled || a.tab == tabOutdated):
		return a.toggleMark(), true
	case k == "u" && (a.tab == tabInstalled || a.tab == tabOutdated):
		if a.marks() > 0 {
			return a.upgradeMarked(), true
		}
		return a.upgradeSelected(), true
	case k == "x" && a.tab == tabInstalled:
		if a.marks() > 0 {
			a.askUninstallMarked()
			return nil, true
		}
		a.askUninstall()
		return nil, true
	case k == "p" && a.tab == tabInstalled:
		return a.togglePin(), true
	case k == "A" && a.tab == tabOutdated:
		return a.askUpgradeAll(), true
	case a.tab == tabServices && (k == "s" || k == "S" || k == "R"):
		return a.serviceKey(k), true
	case a.tab == tabHistory && k == "R":
		a.askRunAgain()
		return nil, true
	case a.tab == tabServices && k == "L":
		return a.openLog(), true
	case a.tab == tabTaps && k == "x":
		a.askUntap()
		return nil, true
	case a.tab == tabDiscover && k == "a":
		return a.askInstall(), true
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
		if behind := a.pinnedBehind(p.FullName, p.Kind); len(behind) > 0 {
			return a.note(p.Name + " needs " + strings.Join(behind, ", ") + " unpinned first")
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
		if behind := a.pinnedBehind(o.Name, o.Kind); len(behind) > 0 {
			return a.note(o.Name + " needs " + strings.Join(behind, ", ") + " unpinned first")
		}
		name, kind = o.Name, o.Kind
	}
	return a.run(action{title: "upgrade " + name, args: kindArgs("upgrade", kind, name), refresh: diskQueries})
}

// askUninstall confirms uninstalling the selected package. If something
// still needs it, it explains why brew would refuse instead. If it leaves
// dependencies nothing else needs, it offers to remove those too.
func (a *App) askUninstall() {
	p, ok := a.installed.selected()
	if !ok {
		return
	}
	if used := a.ix.UsedBy(p.FullName); len(used) > 0 {
		a.confirm = &confirmBox{
			title:   "Can't uninstall " + p.Name,
			body:    []string{"Required by " + strings.Join(used, ", ") + ".", "Uninstall those first, or brew will refuse."},
			choices: []choice{{"n", "close", nil}},
		}
		return
	}
	refresh := append(slices.Clone(diskQueries), qServices, qServiceFiles) // its service goes too
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
	var refused, behind []string
	for _, o := range a.snap.Outdated {
		if b := a.pinnedBehind(o.Name, o.Kind); !o.Pinned && len(b) > 0 {
			refused, behind = append(refused, o.Name), append(behind, b...)
		}
	}
	if len(refused) > 0 {
		they := "they need"
		if len(refused) == 1 {
			they = "it needs"
		}
		behind = a.ix.PinnedBehind(behind...) // sorted, once each
		body = append(body, fmt.Sprintf("brew will refuse %s: %s %s unpinned", strings.Join(refused, ", "), they, strings.Join(behind, ", ")))
	}
	// This dialog is the confirmation, so brew 7 needn't ask again. The
	// variable, unlike --no-ask, is ignored by older brews.
	all := action{title: "upgrade all", args: []string{"upgrade"}, env: []string{"HOMEBREW_NO_ASK=1"}, refresh: diskQueries}
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

// askCleanup offers brew's standard cleanup and pruning the whole download
// cache, each with what its dry-run says it would free.
func (a *App) askCleanup() tea.Cmd {
	cl := a.snap.Cleanup
	switch {
	case a.errs[qCleanup] != nil:
		return a.note("couldn't measure the cleanup · r retries")
	case !a.loaded[qCleanup]:
		return a.note("still measuring what cleanup would free")
	case cl.Standard.Size == "" && cl.All.Size == "":
		return a.note("nothing to clean up")
	}
	box := &confirmBox{title: "Clean up Homebrew?", body: cleanupLines(cl)}
	if cl.Standard.Size != "" {
		std := action{title: "cleanup", args: []string{"cleanup"}, refresh: []query{qCleanup, qSizes}}
		box.choices = append(box.choices, choice{"y", "standard", func() tea.Cmd { return a.run(std) }})
	}
	if cl.All.Items > cl.Standard.Items {
		all := action{title: "cleanup --prune=all", args: []string{"cleanup", "--prune=all"}, refresh: []query{qCleanup, qSizes}}
		box.choices = append(box.choices, choice{"a", "whole cache", func() tea.Cmd { return a.run(all) }})
	}
	box.choices = append(box.choices, choice{"n", "cancel", nil})
	a.confirm = box
	return nil
}

// cleanupLines words what each cleanup would free, as the cleanup prompt
// and Maintenance's details show it.
func cleanupLines(cl brew.Cleanup) []string {
	var lines []string
	if cl.Standard.Size != "" {
		lines = append(lines, "The standard cleanup frees "+cl.Standard.Size+":", "  old versions, and downloads over 120 days old")
	} else {
		lines = append(lines, "The standard cleanup has nothing to remove.")
	}
	if cl.All.Items > cl.Standard.Items {
		lines = append(lines, "Clearing the download cache frees "+cl.All.Size+":", "  every download brew has kept")
	}
	return lines
}

// askAutoremove confirms uninstalling the formulae autoremove's dry run
// found, after naming them.
func (a *App) askAutoremove() tea.Cmd {
	names := a.snap.Autoremove
	switch {
	case a.errs[qAutoremove] != nil:
		return a.note("couldn't check · r retries")
	case !a.loaded[qAutoremove]:
		return a.note("still checking")
	case len(names) == 0:
		return a.note("nothing to remove")
	}
	act := action{title: "autoremove", args: []string{"autoremove"}, refresh: diskQueries}
	a.confirm = &confirmBox{
		title: "Remove unneeded formulae?",
		body:  autoremoveLines(names),
		choices: []choice{
			{"y", "remove", func() tea.Cmd { return a.run(act) }},
			{"n", "cancel", nil},
		},
	}
	return nil
}

// askServicesCleanup confirms brew services cleanup, after naming the
// files it would remove and the services it would stop.
func (a *App) askServicesCleanup() tea.Cmd {
	remove, stop := unusedServices(a.snap.ServiceFiles, a.snap.Services)
	switch {
	case a.errs[qServiceFiles] != nil || a.errs[qServices] != nil:
		return a.note("couldn't check · r retries")
	case !a.loaded[qServiceFiles] || !a.loaded[qServices]:
		return a.note("still checking")
	case len(remove)+len(stop) == 0:
		return a.note("nothing to clean")
	}
	act := action{title: "services cleanup", args: []string{"services", "cleanup"}, refresh: []query{qServices, qServiceFiles}}
	a.confirm = &confirmBox{
		title: "Clean up services?",
		body:  servicesCleanupLines(remove, stop),
		choices: []choice{
			{"y", "clean up", func() tea.Cmd { return a.run(act) }},
			{"n", "cancel", nil},
		},
	}
	return nil
}

// autoremoveLines names what autoremove would uninstall, as its prompt
// and Maintenance's details show it.
func autoremoveLines(names []string) []string {
	lines := []string{"Unneeded formulae, which only other formulae needed:"}
	for _, n := range names {
		lines = append(lines, "  "+n)
	}
	return lines
}

// servicesCleanupLines names the files services cleanup would remove and
// the services it would stop.
func servicesCleanupLines(remove, stop []string) []string {
	var lines []string
	if len(remove) > 0 {
		lines = append(lines, "Unused service files in "+tilde(filepath.Dir(remove[0]))+":")
		for _, f := range remove {
			lines = append(lines, "  "+filepath.Base(f))
		}
	}
	if len(stop) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "Running services whose file is gone:")
		for _, s := range stop {
			lines = append(lines, "  "+s)
		}
	}
	return lines
}

// formulae counts formulae: "1 formula", "2 formulae".
func formulae(n int) string {
	if n == 1 {
		return "1 formula"
	}
	return fmt.Sprintf("%d formulae", n)
}

// brewUpdate runs brew update, then refreshes everything: what U and
// Maintenance's Update Homebrew row do.
func (a *App) brewUpdate() tea.Cmd {
	return a.run(action{title: "update", args: []string{"update"}, full: true})
}

// selectedTap is the tap under the cursor, found by name.
func (a *App) selectedTap() (brew.Tap, bool) {
	row := a.taps.table.SelectedRow()
	if a.taps.n == 0 || row == nil {
		return brew.Tap{}, false
	}
	for _, t := range a.snap.Taps {
		if t.Name == row[0] {
			return t, true
		}
	}
	return brew.Tap{}, false
}

// askUntap confirms untapping the selected tap. If anything installed came
// from it, it explains that brew would refuse instead.
func (a *App) askUntap() {
	t, ok := a.selectedTap()
	if !ok {
		return
	}
	var from []string
	for _, p := range a.snap.Packages {
		if p.Tap == t.Name {
			from = append(from, p.Name)
		}
	}
	if len(from) > 0 {
		a.confirm = &confirmBox{
			title:   "Can't untap " + t.Name,
			body:    []string{"Installed from it: " + strings.Join(from, ", ") + ".", "Uninstall those first, or brew will refuse."},
			choices: []choice{{"n", "close", nil}},
		}
		return
	}
	untap := action{title: "untap " + t.Name, args: []string{"untap", t.Name}, refresh: []query{qTaps}}
	box := &confirmBox{
		title: "Untap " + t.Name + "?",
		choices: []choice{
			{"y", "untap", func() tea.Cmd { return a.run(untap) }},
			{"n", "cancel", nil},
		},
	}
	if what := tapContents(t); what != "" {
		box.body = []string{"Its " + what + " won't be available", "until you tap it again."}
	}
	a.confirm = box
}

// tapContents says what a tap holds, such as "2 formulae and 1 cask".
func tapContents(t brew.Tap) string {
	var parts []string
	switch t.Formulae {
	case 0:
	case 1:
		parts = append(parts, "1 formula")
	default:
		parts = append(parts, fmt.Sprintf("%d formulae", t.Formulae))
	}
	if t.Casks > 0 {
		parts = append(parts, plural(t.Casks, "cask"))
	}
	return strings.Join(parts, " and ")
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
	act := action{title: verb + " " + name, args: []string{"services", verb, name}, refresh: []query{qServices, qServiceFiles}}
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

// askRunAgain offers to run the selected History job again.
func (a *App) askRunAgain() {
	i := a.jobs.cursor()
	if i < 0 || len(a.history[i].Args) == 0 {
		return
	}
	r := a.history[i]
	args := slices.Clone(r.Args)
	a.confirm = &confirmBox{
		title: "Run again?",
		body:  []string{"brew " + strings.Join(args, " ")},
		choices: []choice{
			{"y", "run", func() tea.Cmd { return a.runAgain(args, r.Takeover) }},
			{"n", "cancel", nil},
		},
	}
}

// runAgain runs args as brewery runs its own jobs: full screen if the job
// took the terminal before, otherwise in the drawer without brew's
// auto-update, refreshing what the command touches.
func (a *App) runAgain(args []string, fullScreen bool) tea.Cmd {
	if fullScreen || takesOver(args) {
		return a.takeover(args)
	}
	if args[0] == "install" && a.queue.Pending(args) != nil {
		return a.note(strings.Join(args, " ") + " is already queued")
	}
	refresh, all := refreshAfter(args)
	return a.run(action{title: strings.Join(args, " "), args: args, refresh: refresh, full: all})
}

// actionHints are the status-bar hints for where the focus is.
func (a *App) actionHints() string {
	if a.focus == colSidebar {
		return "j/k choose · l open · 1-5 sections"
	}
	switch a.tab {
	case tabInstalled:
		if a.focus == colDetails {
			return "j/k scroll · h back to the list · u upgrade · x uninstall"
		}
		return "u upgrade · x uninstall · p pin · l details"
	case tabOutdated:
		return "u upgrade · A upgrade all"
	case tabServices:
		if a.focus == colDetails {
			hint := "h back to the list · s start · S stop · R restart · L log"
			if _, _, lines, room, ok := a.logWindow(a.services.pvW, a.services.h); ok && len(lines) > room {
				hint = "j/k scroll the log · " + hint // only a log longer than the pane scrolls
			}
			return hint
		}
		return "s start · S stop · R restart · L log · l details"
	case tabTaps:
		return "x untap"
	case tabDiscover:
		if a.focus == colDetails {
			return "j/k scroll · h back to the list"
		}
		return "a install · / search · l details · r refetch"
	case tabHistory:
		if a.focus == colDetails {
			return "j/k scroll · h back to the list · R run again"
		}
		return "l details · R run again · / filter by command · y copy the command"
	case tabMaintenance:
		if a.focus == colDetails {
			return "enter run · h back to the list"
		}
		return "enter run · l details · c clean up · U update"
	}
	return "h/l move · j/k down, up · enter open · r refresh · U update · c clean up"
}

// pinnedBehind lists the pinned, outdated dependencies of the installed
// formula with the full name name. brew refuses to upgrade it until they're
// unpinned. It goes by the installed version's dependencies, which an
// upgrade can change.
func (a *App) pinnedBehind(name string, kind brew.Kind) []string {
	if kind != brew.KindFormula {
		return nil // brew checks formulae only
	}
	p, ok := a.ix.Get(name)
	if !ok {
		return nil
	}
	return a.ix.PinnedBehind(p.Deps...)
}
