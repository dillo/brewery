package ui

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/jobs"
)

// takeoverDoneMsg reports that a full-screen command has exited.
type takeoverDoneMsg struct {
	job *jobs.Job
	err error
}

// UseTakeover sets how the app builds a brew command that takes over the
// whole terminal (an editor, a shell). The UI never runs brew itself.
func (a *App) UseTakeover(cmd func(args []string) *exec.Cmd) {
	a.takeoverCmd = cmd
}

// takesOver reports whether brew args need the whole terminal: they open
// an editor or an interactive shell.
func takesOver(args []string) bool {
	switch args[0] {
	case "edit", "sh", "irb":
		return true
	case "bundle":
		return len(args) > 1 && args[1] == "edit"
	}
	return false
}

// takeover suspends brewery and gives brew the terminal until it exits.
// It waits for a running job: while brewery is suspended, nothing reads
// the job's output.
func (a *App) takeover(args []string) tea.Cmd {
	if a.takeoverCmd == nil {
		return a.note("full-screen commands aren't available")
	}
	if cur := a.queue.Current(); cur != nil {
		return a.note("wait for " + cur.Title + " to finish: full-screen commands take over the terminal")
	}
	j := &jobs.Job{Title: strings.Join(args, " "), Args: args, State: jobs.Running, Started: a.now()}
	a.takingOver = j
	return tea.ExecProcess(a.takeoverCmd(args), func(err error) tea.Msg { return takeoverDoneMsg{j, err} })
}

// takeoverDone records how a full-screen command ended and refreshes
// everything, since an editor or shell could have changed anything.
func (a *App) takeoverDone(m takeoverDoneMsg) tea.Cmd {
	a.takingOver = nil
	j, output := m.job, "ran full screen"
	j.Ended, j.State = a.now(), jobs.Done
	var exitErr *exec.ExitError
	switch {
	case errors.As(m.err, &exitErr):
		j.State, j.ExitCode = jobs.Failed, exitErr.ExitCode()
	case m.err != nil:
		j.State, j.ExitCode, output = jobs.Failed, -1, "couldn't start brew: "+m.err.Error()
	}
	if j.State == jobs.Done {
		a.flash = a.st.OK.Render("✓ " + j.Title)
	} else {
		a.flash = a.st.Err.Render(fmt.Sprintf("✗ %s failed (exit %d)", j.Title, j.ExitCode))
	}
	return tea.Batch(a.record(j, output), a.refresh())
}
