package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/theme"
)

// filterable is a list the "/" filter can narrow. Each list keeps its own
// filter, so switching tabs doesn't lose it.
type filterable interface {
	matchText() string
	setMatch(string)
}

// matches reports whether s contains the filter text, ignoring case.
func matches(s, match string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(match))
}

func newFilterInput(st theme.Styles) textinput.Model {
	in := textinput.New()
	in.Prompt = "/"
	s := in.Styles()
	s.Cursor.Blink = false
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(st.P.Teal).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(st.P.Text)
	in.SetStyles(s)
	return in
}

// filterTarget is the list "/" narrows now: the jobs view when it's open,
// otherwise the tab's list. Home has none.
func (a *App) filterTarget() filterable {
	if a.jobsOpen {
		return &a.jobs
	}
	switch a.tab {
	case tabInstalled:
		return &a.installed
	case tabOutdated:
		return &a.outdated
	case tabServices:
		return &a.services
	case tabTaps:
		return &a.taps
	case tabDiscover:
		return &a.discover
	}
	return nil
}

// openFilter switches to filter mode, starting from the list's filter.
func (a *App) openFilter() tea.Cmd {
	t := a.filterTarget()
	if t == nil {
		return a.note("/ filters a list · Home has none")
	}
	a.mode = modeFilter
	a.filterIn.SetValue(t.matchText())
	a.filterIn.CursorEnd()
	a.filterIn.Focus()
	return nil
}

// filterKey handles a key in filter mode: the list follows as you type,
// enter keeps the filter, and esc clears it.
func (a *App) filterKey(msg tea.Msg) tea.Cmd {
	t := a.filterTarget()
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "enter":
			a.mode = modeNormal
			return nil
		case "esc":
			a.mode = modeNormal
			t.setMatch("")
			return nil
		}
	}
	var cmd tea.Cmd
	a.filterIn, cmd = a.filterIn.Update(msg)
	t.setMatch(a.filterIn.Value())
	return cmd
}
