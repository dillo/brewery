package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"brewery/internal/jobs"
)

var jobCols = []col{{"STATUS", 9}, {"COMMAND", 0}, {"STARTED", 12}, {"TOOK", 6}}

// jobKey identifies a jobs row by its command and start time.
func jobKey(r table.Row) string { return r[1] + "\x00" + r[2] }

// jobRows lists finished jobs, newest first as the history keeps them.
// Start times show the clock for today and the date before that.
func jobRows(recs []jobs.Record, now time.Time) []table.Row {
	rows := make([]table.Row, len(recs))
	for i, r := range recs {
		status := "✓ done"
		switch r.State {
		case jobs.Failed:
			status = fmt.Sprintf("✗ exit %d", r.ExitCode)
		case jobs.Canceled:
			status = "stopped"
		}
		started := r.Started.In(now.Location())
		when := started.Format("15:04")
		if y, m, d := now.Date(); started.Year() != y || started.Month() != m || started.Day() != d {
			when = started.Format("Jan 2 15:04")
		}
		rows[i] = table.Row{status, "brew " + strings.Join(r.Args, " "), when, elapsed(r.Ended.Sub(r.Started))}
	}
	return rows
}

// openJobs shows the jobs view over the current tab.
func (a *App) openJobs() tea.Cmd {
	a.jobsOpen = true
	a.jobs.setRows(jobRows(a.history, a.now()))
	return nil
}

// jobsKey handles a key while the jobs view is open. ok is false for keys
// that still do what they do everywhere (tabs, :, /, ?, y, o, r, ctrl+c); tab keys
// also close the view.
func (a *App) jobsKey(msg tea.KeyPressMsg) (cmd tea.Cmd, ok bool) {
	switch msg.String() {
	case "esc", "q":
		a.jobsOpen = false
		return nil, true
	case "enter":
		return a.showRecord(), true
	case "1", "2", "3", "4", "5", "[", "]":
		a.jobsOpen = false
		return nil, false
	case ":", "/", "?", "y", "o", "r", "ctrl+c":
		return nil, false
	}
	return a.jobs.update(msg), true
}

// showRecord replays the selected job's output in the drawer. A running
// job keeps the drawer until it finishes.
func (a *App) showRecord() tea.Cmd {
	i := a.jobs.cursor()
	if i < 0 {
		return nil
	}
	if cur := a.queue.Current(); cur != nil {
		return a.note(cur.Title + " is running · its output stays in the drawer until it finishes")
	}
	r := a.history[i]
	a.drawer.replay(r.Job(), r.Output)
	return nil
}
