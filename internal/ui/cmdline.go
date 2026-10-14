package ui

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"brewery/internal/theme"
)

// cmdline is the ":" prompt: a one-line editor that remembers the
// session's earlier commands.
type cmdline struct {
	input textinput.Model
	past  []string // earlier commands, oldest first
	back  int      // how far up into past the line is; 0 is the draft
	draft string   // what was typed before browsing past commands

	items []candidate // the completion popup, empty when hidden
	sel   int         // the selected item, -1 for none
	base  string      // the line before the word being completed
}

func newCmdline(st theme.Styles) cmdline {
	in := textinput.New()
	in.Prompt = ":"
	s := in.Styles()
	s.Cursor.Blink = false
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(st.P.Blue).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(st.P.Text)
	in.SetStyles(s)
	return cmdline{input: in}
}

// open starts an empty line.
func (c *cmdline) open() {
	c.input.Reset()
	c.input.Focus()
	c.back, c.draft, c.items = 0, "", nil
}

func (c *cmdline) value() string { return c.input.Value() }

// remember adds a run command to the past, skipping a repeat of the last.
func (c *cmdline) remember(line string) {
	if n := len(c.past); n == 0 || c.past[n-1] != line {
		c.past = append(c.past, line)
	}
}

// update edits the line. up and down step through past commands.
func (c *cmdline) update(msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "up":
			if c.back < len(c.past) {
				if c.back == 0 {
					c.draft = c.input.Value()
				}
				c.back++
				c.show(c.past[len(c.past)-c.back])
			}
			return nil
		case "down":
			if c.back > 0 {
				c.back--
				if c.back == 0 {
					c.show(c.draft)
				} else {
					c.show(c.past[len(c.past)-c.back])
				}
			}
			return nil
		}
	}
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

// show puts s on the line with the cursor at the end.
func (c *cmdline) show(s string) {
	c.input.SetValue(s)
	c.input.CursorEnd()
}

// view renders the prompt in w columns.
func (c *cmdline) view(w int) string {
	c.input.SetWidth(max(w-2, 1)) // the prompt and the cursor cell
	return c.input.View()
}

// openCmdline switches to command mode with an empty prompt.
func (a *App) openCmdline() tea.Cmd {
	a.mode = modeCommand
	a.cmd.open()
	return nil
}

// cmdKey handles a key in command mode: enter runs the line, esc cancels,
// and everything else edits it.
func (a *App) cmdKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		a.mode = modeNormal
		return nil
	case "tab", "shift+tab":
		if len(a.cmd.items) == 0 {
			a.suggest(true)
		}
		dir := 1
		if msg.String() == "shift+tab" {
			dir = -1
		}
		a.cmd.cycle(dir)
		return nil
	case "enter":
		a.mode = modeNormal
		line := strings.TrimSpace(a.cmd.value())
		if line == "" {
			return nil
		}
		a.cmd.remember(line)
		return a.execute(line)
	}
	cmd := a.cmd.update(msg)
	a.suggest(false)
	return cmd
}

// execute runs a line typed at the prompt. A leading "brew" is optional.
// Lines run exactly as typed: no confirmation, and brew may auto-update.
// Editors and shells take over the terminal, as does any line starting
// with "!".
func (a *App) execute(line string) tea.Cmd {
	full, force := strings.CutPrefix(line, "!")
	args, err := splitArgs(full)
	if err != nil {
		return a.note(err.Error())
	}
	if len(args) > 0 && args[0] == "brew" {
		args = args[1:]
	}
	if len(args) == 0 {
		return nil
	}
	if force || takesOver(args) {
		return a.takeover(args)
	}
	switch args[0] {
	case "q", "quit":
		return a.askQuit()
	case "jobs":
		return a.openJobs()
	}
	refresh, all := refreshAfter(args)
	return a.run(action{title: strings.Join(args, " "), args: args, refresh: refresh, full: all, typed: true})
}

// readOnly are brew commands that change nothing brewery shows, so no
// refresh follows them. Commands starting with "-" (--prefix, --cache,
// --version, …) only print paths and versions.
var readOnly = map[string]bool{
	"info": true, "abv": true, "list": true, "ls": true, "search": true, "deps": true,
	"uses": true, "desc": true, "home": true, "homepage": true, "config": true,
	"outdated": true, "leaves": true, "commands": true, "help": true, "options": true,
	"log": true, "cat": true, "formulae": true, "casks": true, "tap-info": true,
	"missing": true, "doctor": true, "dr": true, "shellenv": true, "docs": true,
}

// refreshAfter says what to re-run after a typed command: nothing after a
// read-only one, just services after a services change, else everything.
func refreshAfter(args []string) (refresh []query, full bool) {
	cmd := args[0]
	switch {
	case readOnly[cmd] || strings.HasPrefix(cmd, "-"):
		return nil, false
	case cmd == "services":
		if len(args) == 1 || args[1] == "list" || args[1] == "info" {
			return nil, false
		}
		return []query{qServices}, false
	}
	return nil, true
}

var errUnclosedQuote = errors.New("unclosed quote")

// splitArgs splits a line into arguments the way a shell would, without
// running one: spaces separate, quotes group, and a backslash escapes the
// next character (outside single quotes).
func splitArgs(line string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inArg, quote, escaped := false, rune(0), false
	for _, r := range line {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped, inArg = true, true
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			cur.WriteRune(r)
		case r == '"' || r == '\'':
			quote, inArg = r, true
		case r == ' ' || r == '\t':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if quote != 0 || escaped {
		return nil, errUnclosedQuote
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args, nil
}
