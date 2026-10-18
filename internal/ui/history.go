package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/jobs"
	"github.com/dillo/brewery/internal/theme"
)

var jobCols = []col{{"STATUS", 9, 0}, {"COMMAND", 0, 20}, {"STARTED", 12, 5}, {"TOOK", 6, 0}}

// jobKey identifies a jobs row by its command and start time.
func jobKey(r table.Row) string { return r[1] + "\x00" + r[2] }

// jobRows lists finished jobs, newest first as the history keeps them,
// their status colored. Start times show the clock for today and the date
// before that.
func jobRows(st theme.Styles, recs []jobs.Record, now time.Time) []table.Row {
	rows := make([]table.Row, len(recs))
	for i, r := range recs {
		status := theme.Tint(st.P.Green, "✓ done")
		switch r.State {
		case jobs.Failed:
			status = theme.Tint(st.P.Red, fmt.Sprintf("✗ exit %d", r.ExitCode))
		case jobs.Canceled:
			status = theme.Tint(st.P.Yellow, "stopped")
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

// plainOutput is a job's saved terminal output as plain lines: escape
// sequences dropped, and of a line brew rewrote with \r (a progress bar),
// only what it said last.
func plainOutput(s string) string {
	lines := strings.Split(strings.ReplaceAll(ansi.Strip(s), "\r\n", "\n"), "\n")
	for i, l := range lines {
		l = strings.TrimRight(l, "\r")
		if j := strings.LastIndex(l, "\r"); j >= 0 {
			l = l[j+1:]
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}

// recordPreview is History's details for the selected job: its command,
// how it ended, when it ran, and what it printed, wrapped to the pane so
// a long line reads whole.
func (a *App) recordPreview(w, h int) (title, body string) {
	i := a.jobs.cursor()
	if i < 0 {
		return "Details", ""
	}
	r, st := a.history[i], a.st
	ended := st.OK.Render("✓ done")
	switch r.State {
	case jobs.Failed:
		ended = st.Err.Render(fmt.Sprintf("✗ exit %d", r.ExitCode))
	case jobs.Canceled:
		ended = st.Warn.Render("stopped")
	}
	started := r.Started.In(a.now().Location()).Format("2006-01-02 15:04")
	body = kv(st, "command", "brew "+strings.Join(r.Args, " "), "ended", ended,
		"started", started, "took", elapsed(r.Ended.Sub(r.Started))) +
		"\n" + st.Accent.Render("Output") + "\n"
	if out := plainOutput(r.Output); strings.TrimSpace(out) != "" {
		iw, _ := paneInner(w, h)
		body += ansi.Wrap(out, iw, "")
	} else {
		body += st.Muted.Render("nothing printed")
	}
	return r.Title, body
}

// showHistory selects History with its list focused.
func (a *App) showHistory() tea.Cmd {
	cmd := a.selectEntry(entryIndex("History"))
	a.focus = colList
	return cmd
}
