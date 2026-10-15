package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

// Messages from opening a homepage and copying a name.
type (
	openedMsg struct{ err error }
	copiedMsg struct {
		text string
		err  error
	}
)

// UseOpener sets how the app opens a web page; UseClipboard how it copies
// text. The UI never starts programs itself.
func (a *App) UseOpener(open func(url string) error)     { a.opener = open }
func (a *App) UseClipboard(copy func(text string) error) { a.clipboard = copy }

// gKey handles the key after g: g goes to the top of the list, x opens
// the selected item's homepage. ok is false for any other key, which then
// does what it always does.
func (a *App) gKey(k string) (cmd tea.Cmd, ok bool) {
	switch k {
	case "g":
		home := tea.KeyPressMsg{Code: tea.KeyHome}
		if a.jobsOpen {
			return a.jobs.update(home), true
		}
		switch a.tab {
		case tabInstalled:
			return a.installed.update(home), true
		case tabOutdated:
			return a.outdated.update(home), true
		case tabServices:
			return a.services.update(home), true
		case tabTaps:
			return a.taps.update(home), true
		}
		return nil, true
	case "x":
		return a.openHomepage(), true
	}
	return nil, false
}

// packageNamed finds an installed package by name and kind.
func (a *App) packageNamed(name string, kind brew.Kind) (brew.Package, bool) {
	for _, p := range a.snap.Packages {
		if p.Name == name && p.Kind == kind {
			return p, true
		}
	}
	return brew.Package{}, false
}

// selection is what the cursor is on in the current list: a name to copy
// and, where there is one, a web page.
func (a *App) selection() (name, url string, ok bool) {
	if a.jobsOpen {
		if i := a.jobs.cursor(); i >= 0 {
			return "brew " + strings.Join(a.history[i].Args, " "), "", true
		}
		return "", "", false
	}
	switch a.tab {
	case tabInstalled:
		if p, ok := a.installed.selected(); ok {
			return p.Name, p.Homepage, true
		}
	case tabOutdated:
		if i := a.outdated.cursor(); i >= 0 {
			o := a.snap.Outdated[i]
			p, _ := a.packageNamed(o.Name, o.Kind)
			return o.Name, p.Homepage, true
		}
	case tabServices:
		if s, ok := a.selectedService(); ok {
			p, _ := a.packageNamed(s.Name, brew.KindFormula)
			return s.Name, p.Homepage, true
		}
	case tabTaps:
		if t, ok := a.selectedTap(); ok {
			url := ""
			if strings.HasPrefix(t.Remote, "https://") {
				url = strings.TrimSuffix(t.Remote, ".git")
			}
			return t.Name, url, true
		}
	}
	return "", "", false
}

// webPage reports whether url is a web address. Homepages come from tap
// metadata, and macOS open would also launch a local file or app, or read
// a leading "-" as an option, so nothing else is opened.
func webPage(url string) bool {
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
}

// openHomepage opens the selected item's web page in the browser.
func (a *App) openHomepage() tea.Cmd {
	name, url, ok := a.selection()
	switch {
	case !ok:
		return nil
	case !webPage(url):
		return a.note("no web page for " + name)
	case a.opener == nil:
		return a.note("opening web pages isn't available")
	}
	a.flash = a.st.Info.Render("opening " + url)
	open := a.opener
	return func() tea.Msg { return openedMsg{err: open(url)} }
}

// copyName copies the selected item's name.
func (a *App) copyName() tea.Cmd {
	name, _, ok := a.selection()
	switch {
	case !ok:
		return nil
	case a.clipboard == nil:
		return a.note("copying isn't available")
	}
	copy := a.clipboard
	return func() tea.Msg { return copiedMsg{text: name, err: copy(name)} }
}

// linkDone reports how opening a page or copying went.
func (a *App) linkDone(msg tea.Msg) {
	switch m := msg.(type) {
	case openedMsg:
		if m.err != nil {
			a.flash = a.st.Err.Render("✗ couldn't open the page: " + m.err.Error())
		}
	case copiedMsg:
		if m.err != nil {
			a.flash = a.st.Err.Render("✗ couldn't copy: " + m.err.Error())
		} else {
			a.flash = a.st.OK.Render("copied " + m.text)
		}
	}
}
