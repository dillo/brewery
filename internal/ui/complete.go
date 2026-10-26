package ui

import (
	"context"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/theme"
)

// candidate is one way to complete the word being typed.
type candidate struct{ text, desc string }

// builtins are brewery's own commands, completed alongside brew's.
var builtins = []candidate{
	{"jobs", "Show History: finished jobs and their output"},
	{"theme", "Change brewery's colours"},
}

// everyday are the commands most people use. They complete before brew's
// many developer commands, so "up" offers upgrade before update-report.
var everyday = map[string]bool{
	"autoremove": true, "bundle": true, "cleanup": true, "config": true, "deps": true,
	"desc": true, "doctor": true, "edit": true, "home": true, "info": true, "install": true,
	"jobs": true, "leaves": true, "link": true, "list": true, "outdated": true, "pin": true,
	"reinstall": true, "search": true, "services": true, "tap": true, "theme": true, "uninstall": true,
	"unlink": true, "unpin": true, "untap": true, "update": true, "upgrade": true, "uses": true,
}

// popupRows is how many candidates the popup shows at once.
const popupRows = 8

// completionsMsg carries brew's completions, read once at start.
type completionsMsg struct {
	comp brew.Completions
	err  error
}

// UseCompletions sets how the app reads brew's command completions; it
// reads them once when it starts.
func (a *App) UseCompletions(load func(context.Context) (brew.Completions, error)) {
	a.loadComp = load
}

func (a *App) loadCompletions() tea.Cmd {
	if a.loadComp == nil {
		return nil
	}
	load, ctx := a.loadComp, a.ctx
	return func() tea.Msg {
		c, err := load(ctx)
		return completionsMsg{c, err}
	}
}

// candidates lists what can complete the last word of line, and returns
// that word. A first word completes to a command; after it come the
// command's subcommands, its flags (once the word starts with "-"), or
// the installed packages, taps or services it takes.
func (a *App) candidates(line string) (word string, cs []candidate) {
	word, words := splitLine(line)
	if len(words) == 0 {
		return word, matching(a.commandCandidates(), word)
	}
	if words[0] == "theme" {
		if len(words) > 1 {
			return word, nil // it takes one name
		}
		return word, matching(themeCandidates(), word)
	}
	cmd, ok := a.comp.Command(words[0])
	if !ok {
		return word, nil
	}
	return word, matching(a.commandArgs(cmd, words, word), word)
}

// commandArgs lists what can follow cmd, given the finished words and the
// word being typed: its subcommands, its flags, or the values its
// arguments take that aren't on the line yet.
func (a *App) commandArgs(cmd brew.Command, words []string, word string) (cs []candidate) {
	flag := strings.HasPrefix(word, "-")
	if len(words) > 1 {
		if sub, ok := cmd.Sub(words[1]); ok {
			cmd = sub
		}
	} else if len(cmd.Subs) > 0 && !flag {
		for _, s := range cmd.Subs {
			cs = append(cs, candidate{s.Name, s.Desc})
		}
		return cs
	}
	if flag {
		for _, f := range cmd.Flags {
			cs = append(cs, candidate{f.Name, f.Desc})
		}
		return cs
	}
	for _, kind := range cmd.Args {
		cs = append(cs, a.argValues(kind)...)
	}
	return slices.DeleteFunc(cs, func(c candidate) bool { return slices.Contains(words, c.text) })
}

// splitLine splits line into the word being typed and the finished words
// before it. A leading "!" and "brew" are dropped: they complete like the
// command after them.
func splitLine(line string) (word string, words []string) {
	line = strings.TrimPrefix(line, "!")
	words = strings.Fields(line)
	if !strings.HasSuffix(line, " ") && len(words) > 0 {
		word, words = words[len(words)-1], words[:len(words)-1]
	}
	if len(words) > 0 && words[0] == "brew" {
		words = words[1:]
	}
	return word, words
}

// commandCandidates lists brewery's and brew's commands, the everyday ones
// first.
func (a *App) commandCandidates() []candidate {
	cs := slices.Clone(builtins)
	for _, c := range a.comp.Commands {
		cs = append(cs, candidate{c.Name, c.Desc})
	}
	slices.SortStableFunc(cs, func(x, y candidate) int {
		if everyday[x.text] != everyday[y.text] {
			if everyday[x.text] {
				return -1
			}
			return 1
		}
		return strings.Compare(x.text, y.text)
	})
	return cs
}

// argValues lists the values brewery knows for an argument kind from
// brew's completions. Any formula or cask comes from Discover's catalogue,
// once it has been read.
func (a *App) argValues(kind string) []candidate {
	var cs []candidate
	switch kind {
	case "formulae_all", "casks_all":
		cs = a.catalogValues(argKind(kind))
	case "formulae_installed", "casks_installed":
		cs = a.installedValues(argKind(kind))
	case "formulae_outdated", "casks_outdated":
		cs = a.outdatedValues(argKind(kind))
	case "taps_installed":
		for _, t := range a.snap.Taps {
			cs = append(cs, candidate{t.Name, t.Remote})
		}
	case "services":
		for _, s := range a.snap.Services {
			cs = append(cs, candidate{s.Name, s.Status})
		}
	case "commands":
		for _, c := range a.comp.Commands {
			cs = append(cs, candidate{c.Name, c.Desc})
		}
	}
	slices.SortStableFunc(cs, func(x, y candidate) int { return strings.Compare(x.text, y.text) })
	return cs
}

// argKind is the package kind an argument kind such as "casks_all" asks
// for: casks, or else formulae.
func argKind(kind string) brew.Kind {
	if strings.HasPrefix(kind, "casks_") {
		return brew.KindCask
	}
	return brew.KindFormula
}

// catalogValues lists the catalogue's entries of one kind.
func (a *App) catalogValues(want brew.Kind) []candidate {
	var cs []candidate
	for _, e := range a.discover.cat {
		if e.Kind == want {
			cs = append(cs, candidate{e.Name, e.Desc})
		}
	}
	return cs
}

// installedValues lists the installed packages of one kind.
func (a *App) installedValues(want brew.Kind) []candidate {
	var cs []candidate
	for _, p := range a.snap.Packages {
		if p.Kind == want {
			cs = append(cs, candidate{p.Name, p.Desc})
		}
	}
	return cs
}

// outdatedValues lists the outdated packages of one kind, with the move
// from the installed version to the current one.
func (a *App) outdatedValues(want brew.Kind) []candidate {
	var cs []candidate
	for _, o := range a.snap.Outdated {
		if o.Kind == want {
			cs = append(cs, candidate{o.Name, o.Installed + " → " + o.Current})
		}
	}
	return cs
}

// matching keeps the candidates that start with word.
func matching(cs []candidate, word string) []candidate {
	return slices.DeleteFunc(cs, func(c candidate) bool { return !strings.HasPrefix(c.text, word) })
}

// suggest refreshes the popup for the line as typed. Without force, an
// empty line shows nothing; tab forces the full list.
func (a *App) suggest(force bool) {
	line := a.cmd.value()
	if strings.TrimSpace(line) == "" && !force {
		a.cmd.items = nil
		return
	}
	word, cs := a.candidates(line)
	a.cmd.items, a.cmd.sel, a.cmd.base = cs, -1, line[:len(line)-len(word)]
}

// cycle moves the popup's selection by dir (1 or -1) and puts the
// selected candidate in place of the word being typed.
func (c *cmdline) cycle(dir int) {
	n := len(c.items)
	if n == 0 {
		return
	}
	if c.sel < 0 && dir < 0 {
		c.sel = 0
	}
	c.sel = (c.sel + dir + n) % n
	c.show(c.base + c.items[c.sel].text)
}

// popup renders the candidates as a list for above the prompt, or ""
// when there are none. Long lists scroll to keep the selection in view.
func (c *cmdline) popup(st theme.Styles, w int) string {
	if len(c.items) == 0 {
		return ""
	}
	nameW, descW := 0, 0
	for _, it := range c.items {
		nameW = max(nameW, lipgloss.Width(it.text))
		descW = max(descW, lipgloss.Width(it.desc))
	}
	nameW = min(nameW, 30)
	inner := min(nameW+2+descW, 72, max(w-4, 10))
	start := popupStart(len(c.items), c.sel, popupRows)
	var rows []string
	for i := start; i < min(start+popupRows, len(c.items)); i++ {
		it := c.items[i]
		name := ansi.Truncate(it.text, nameW, "…")
		desc := ansi.Truncate(it.desc, max(inner-nameW-2, 0), "…")
		row := name + strings.Repeat(" ", nameW-lipgloss.Width(name)+2)
		if i == c.sel {
			rows = append(rows, st.TableSelected.Foreground(st.P.Text).Width(inner).Render(row+desc))
		} else {
			rows = append(rows, st.Text.Render(row)+st.Muted.Render(desc))
		}
	}
	return boxStyle(st.P.Blue).Width(inner + 4).Render(strings.Join(rows, "\n"))
}

// popupStart is the first of rows visible candidates out of n, scrolled so
// the selection sel (-1 for none) is in view.
func popupStart(n, sel, rows int) int {
	if sel < rows {
		return 0
	}
	return min(sel-rows+1, n-rows)
}
