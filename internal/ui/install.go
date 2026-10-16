package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

// askInstall confirms installing the package selected on Discover, naming
// what else it would add.
func (a *App) askInstall() tea.Cmd {
	r, ok := a.discover.selected()
	if !ok {
		return nil
	}
	if a.discover.have[entryKey(r.Name, r.Kind)] {
		return a.note(r.Name + " is already installed")
	}
	if a.queue.Pending(kindArgs("install", r.Kind, r.Name)) != nil {
		return a.note(r.Name + " is already queued")
	}
	a.installing = r
	a.confirm = a.installBox(r)
	return nil
}

// installTag marks the install dialog for the package with key, so brew's
// answer about its dependencies can update it.
func installTag(key string) string { return "install " + key }

// installBox is the dialog for installing r. Until brew has said what r
// needs, it says it's checking; y works either way.
func (a *App) installBox(r discRow) *confirmBox {
	key := entryKey(r.Name, r.Kind)
	if det, ok := a.discover.details[key]; ok && r.Kind == brew.KindFormula {
		if behind := a.ix.PinnedBehind(det.Needs...); len(behind) > 0 {
			it, is := "it", "is"
			if len(behind) > 1 {
				it, is = "them", "are"
			}
			return &confirmBox{
				title:   "Can't install " + r.Name,
				body:    []string{fmt.Sprintf("It needs %s, which %s pinned and outdated.", strings.Join(behind, ", "), is), "Unpin " + it + " on Installed first, or brew will refuse."},
				choices: []choice{{"n", "close", nil}},
				tag:     installTag(key),
			}
		}
	}
	about := []string{r.Desc}
	if r.Kind == brew.KindCask {
		what := "app"
		if strings.HasPrefix(r.Name, "font-") {
			what = "font"
		}
		about = []string{what, r.Desc}
	}
	var body []string
	if line := strings.Join(slices.DeleteFunc(about, func(s string) bool { return s == "" }), " · "); line != "" {
		body = append(body, line)
	}
	switch missing, ok := a.discover.missing(key); {
	case !ok && a.discover.detailsErr[key] != nil:
		body = append(body, "couldn't check dependencies")
	case !ok:
		body = append(body, "checking dependencies…")
	case len(missing) > 0:
		body = append(body, alsoInstalls(missing))
	}
	// This dialog is the confirmation, so brew 7 needn't ask again.
	install := action{title: "install " + r.Name, args: kindArgs("install", r.Kind, r.Name), env: []string{"HOMEBREW_NO_ASK=1"}, refresh: diskQueries}
	return &confirmBox{
		title: "Install " + strings.TrimSpace(r.Name+" "+a.discover.details[key].Latest) + "?",
		body:  body,
		choices: []choice{
			{"y", "install", func() tea.Cmd { return a.run(install) }},
			{"n", "cancel", nil},
		},
		tag: installTag(key),
	}
}

// alsoInstalls says which dependencies come along, naming at most five.
func alsoInstalls(names []string) string {
	noun := "dependency"
	if len(names) > 1 {
		noun = "dependencies"
	}
	s := fmt.Sprintf("also installs %d %s: %s", len(names), noun, strings.Join(names[:min(len(names), 5)], ", "))
	if len(names) > 5 {
		s += fmt.Sprintf(" +%d more", len(names)-5)
	}
	return s
}
