package ui

import tea "charm.land/bubbletea/v2"

// openJump opens the sidebar's "/" prompt: as the user types, the
// selection moves to the first entry whose label contains the text.
func (a *App) openJump() {
	a.mode, a.jumping, a.jumpFrom, a.jumpMiss = modeFilter, true, a.sel, false
	a.filterIn.SetValue("")
	a.filterIn.Focus()
}

// jumpKey handles a key while jumping: the selection follows the text,
// enter opens the entry, and esc goes back to where the jump started.
func (a *App) jumpKey(msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "enter":
			a.mode, a.jumping = modeNormal, false
			if !a.jumpMiss {
				a.openEntry()
			}
			return nil
		case "esc":
			a.mode, a.jumping = modeNormal, false
			return a.selectEntry(a.jumpFrom)
		}
	}
	var cmd tea.Cmd
	a.filterIn, cmd = a.filterIn.Update(msg)
	text := a.filterIn.Value()
	if text == "" {
		a.jumpMiss = false
		return tea.Batch(cmd, a.selectEntry(a.jumpFrom))
	}
	i, ok := jumpMatch(text)
	a.jumpMiss = !ok
	if !ok {
		return cmd // the selection stays on the last match
	}
	return tea.Batch(cmd, a.selectEntry(i))
}

// jumpMatch is the first entry, from the top, whose label contains text,
// ignoring case.
func jumpMatch(text string) (int, bool) {
	for i, e := range entries {
		if matches(e.label, text) {
			return i, true
		}
	}
	return 0, false
}
