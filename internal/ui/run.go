package ui

import (
	"fmt"
	"io"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/jobs"
)

// Proc is a running brew command. *brew.Proc satisfies it; tests use a fake.
type Proc interface {
	io.Reader // output; io.EOF once the command has exited
	io.Writer // keystrokes
	Resize(cols, rows int) error
	Interrupt() error
	Kill() error
	Wait() (int, error)
}

// Starter starts brew with args in a pseudo-terminal of cols×rows, adding
// env to its environment.
type Starter func(args, env []string, cols, rows int) (Proc, error)

// Messages from a running job. id and attempt name the run they belong to,
// so a late message from an earlier run is ignored.
type (
	jobOutputMsg struct {
		id, attempt int
		data        []byte
	}
	jobEOFMsg   struct{ id, attempt int }
	jobExitMsg  struct{ id, attempt, code int }
	jobRetryMsg struct{ id int }
	tickMsg     struct{}
	quitNowMsg  struct{} // the running job had its chance to stop: quit anyway
)

// quitGrace is how long a running job gets to stop after ctrl+c when
// brewery quits, before it is killed.
const quitGrace = 3 * time.Second

// readOutput reads the next chunk of a job's output.
func readOutput(p Proc, id, attempt int) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 32*1024)
		n, err := p.Read(buf)
		if n > 0 || err == nil {
			return jobOutputMsg{id, attempt, buf[:n]}
		}
		return jobEOFMsg{id, attempt}
	}
}

// waitExit waits for a job's process to exit once its output is drained.
func waitExit(p Proc, id, attempt int) tea.Cmd {
	return func() tea.Msg {
		code, err := p.Wait()
		if err != nil {
			code = -1
		}
		return jobExitMsg{id, attempt, code}
	}
}

// run queues act and starts it unless another job is running.
func (a *App) run(act action) tea.Cmd {
	j := a.queue.Add(act.title, act.args)
	a.acts[j.ID] = act
	if cur := a.queue.Current(); cur != nil {
		a.flash = a.st.Info.Render(fmt.Sprintf("queued %s · runs after %s", act.title, cur.Title))
		return nil
	}
	return a.runNext()
}

// runNext starts the oldest queued job, if nothing is running.
func (a *App) runNext() tea.Cmd {
	j := a.queue.Next(a.now())
	if j == nil {
		return nil
	}
	return a.launch(j)
}

// launch starts a run of j and shows it in the drawer.
func (a *App) launch(j *jobs.Job) tea.Cmd {
	act := a.acts[j.ID]
	env := append(slices.Clone(brew.JobEnv), act.env...)
	if act.typed {
		env = nil
	}
	p, err := a.start(j.Args, env, a.drawer.cols, a.drawer.rows)
	if err != nil {
		a.queue.Finish(j, -1, false, err.Error(), a.now())
		a.flash = a.st.Err.Render(fmt.Sprintf("✗ %s: couldn't start brew: %v", j.Title, err))
		return tea.Batch(a.record(j, "couldn't start brew: "+err.Error()), a.runNext())
	}
	a.proc, a.canceled = p, false
	a.drawer.start(j, p)
	return tea.Batch(readOutput(p, j.ID, j.Attempts), a.tick())
}

// tick animates the spinner while a job is current.
func (a *App) tick() tea.Cmd {
	if a.ticking || a.tickEvery == 0 {
		return nil
	}
	a.ticking = true
	return tea.Tick(a.tickEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

// onScreen reports whether a message belongs to the run being shown.
func (a *App) onScreen(id, attempt int) bool {
	j := a.queue.Current()
	return a.proc != nil && j != nil && j.ID == id && j.Attempts == attempt
}

// handleJob reacts to a job message and returns what to do next.
func (a *App) handleJob(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case jobOutputMsg:
		if !a.onScreen(m.id, m.attempt) {
			return nil
		}
		a.drawer.write(m.data)
		switch {
		case a.mode == modeNormal && a.drawer.prompt():
			a.typeIntoJob() // brew is asking: let the user answer
		case (a.mode == modeCommand || a.mode == modeFilter) && a.drawer.prompt():
			a.promptPending = true // answer once the line being typed closes
		}
		return readOutput(a.proc, m.id, m.attempt)
	case jobEOFMsg:
		if !a.onScreen(m.id, m.attempt) {
			return nil
		}
		return waitExit(a.proc, m.id, m.attempt)
	case jobExitMsg:
		if !a.onScreen(m.id, m.attempt) {
			return nil
		}
		return a.finish(a.queue.Current(), m.code)
	case jobRetryMsg:
		j := a.queue.Current()
		if j == nil || j.ID != m.id || j.State != jobs.Waiting {
			return nil // stopped while waiting
		}
		a.queue.Retry(j)
		return a.launch(j)
	case tickMsg:
		a.ticking = false
		a.frame++
		if a.queue.Current() != nil {
			return a.tick()
		}
	}
	return nil
}

// finish records how a run ended. A run that hit brew's lock is retried
// after a backoff; anything else refreshes what the job changed (even a
// failure may have changed something) and starts the next job.
func (a *App) finish(j *jobs.Job, code int) tea.Cmd {
	a.drawer.stop()
	a.proc = nil
	if a.mode == modeInsert {
		a.mode = modeNormal
	}
	if a.quitting {
		// Stopped on the way out: record it, start nothing else, and go.
		a.queue.Finish(j, code, true, a.drawer.output(), a.now())
		return tea.Sequence(a.record(j, a.drawer.output()), tea.Quit)
	}
	if a.queue.Finish(j, code, a.canceled, a.drawer.output(), a.now()) {
		wait := a.backoff(j.Attempts)
		a.drawer.retryAt = a.now().Add(wait)
		return tea.Tick(wait, func(time.Time) tea.Msg { return jobRetryMsg{j.ID} })
	}
	return tea.Batch(a.afterJob(j), a.runNext())
}

// afterJob reports a finished job and refreshes what it touched.
func (a *App) afterJob(j *jobs.Job) tea.Cmd {
	switch j.State {
	case jobs.Done:
		a.flash = a.st.OK.Render("✓ " + j.Title)
	case jobs.Canceled:
		a.flash = a.st.Warn.Render("stopped " + j.Title)
	default:
		msg := fmt.Sprintf("✗ %s failed (exit %d)", j.Title, j.ExitCode)
		if a.queue.Queued() == 0 {
			msg += " · o shows output" // else the next job is about to take the drawer
		}
		a.flash = a.st.Err.Render(msg)
	}
	act := a.acts[j.ID]
	saving := a.record(j, a.drawer.output())
	if act.full {
		return tea.Batch(saving, a.refresh())
	}
	return tea.Batch(saving, a.request(act.refresh...))
}

// historySavedMsg reports whether a finished job was saved to the history.
type historySavedMsg struct{ err error }

// record adds a finished job to the history and saves it. The job's
// action is no longer needed.
func (a *App) record(j *jobs.Job, output string) tea.Cmd {
	delete(a.acts, j.ID)
	r := jobs.NewRecord(j, output)
	a.history = append([]jobs.Record{r}, a.history[:min(len(a.history), jobs.Keep-1)]...)
	a.jobs.setRows(jobRows(a.st, a.history, a.now()))
	if a.hist == nil {
		return nil
	}
	h := *a.hist
	return func() tea.Msg { return historySavedMsg{err: h.Save(r)} }
}

// stopJob interrupts the current job, as ctrl+c in a terminal would. A job
// waiting for brew's lock is simply dropped.
func (a *App) stopJob() tea.Cmd {
	j := a.queue.Current()
	if j == nil {
		return nil
	}
	a.canceled = true
	if a.proc != nil {
		_ = a.proc.Interrupt() // the exit message follows
		return nil
	}
	a.queue.Finish(j, -1, true, "", a.now())
	return tea.Batch(a.afterJob(j), a.runNext())
}

// askStop confirms before stopping the current job. Output and exits keep
// arriving while the dialog is open, so "y" only stops the job it named.
func (a *App) askStop() {
	j := a.queue.Current()
	if j == nil {
		return
	}
	a.confirm = &confirmBox{
		title: "Stop " + j.Title + "?",
		body:  []string{"brew gets ctrl+c and may leave this half done."},
		choices: []choice{
			{"y", "stop it", func() tea.Cmd {
				if a.queue.Current() != j {
					return a.note(j.Title + " already finished") // it finished meanwhile
				}
				return a.stopJob()
			}},
			{"n", "keep running", nil},
		},
	}
}

// askQuit confirms before quitting while a job is running.
func (a *App) askQuit() tea.Cmd {
	if a.queue.Current() == nil {
		return a.quit()
	}
	a.confirm = &confirmBox{
		title: "Quit brewery?",
		body:  []string{a.queue.Current().Title + " is still running and will be stopped."},
		choices: []choice{
			{"y", "quit", a.quit},
			{"n", "stay", nil},
		},
	}
	return nil
}

// keyBytes is what a terminal would send for a key press.
func keyBytes(msg tea.KeyPressMsg) []byte {
	switch k := msg.String(); k {
	case "enter":
		return []byte("\r")
	case "backspace":
		return []byte{0x7f}
	case "tab":
		return []byte("\t")
	case "up":
		return []byte("\x1b[A")
	case "down":
		return []byte("\x1b[B")
	case "right":
		return []byte("\x1b[C")
	case "left":
		return []byte("\x1b[D")
	default:
		if len(k) == 6 && k[:5] == "ctrl+" && k[5] >= 'a' && k[5] <= 'z' {
			return []byte{k[5] - 'a' + 1} // ctrl+c is 0x03, and so on
		}
		return []byte(msg.Text)
	}
}
