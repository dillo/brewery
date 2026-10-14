package ui

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"

	"brewery/internal/jobs"
	"brewery/internal/theme"
)

// promptRe matches a terminal line that is waiting for the user: a
// password prompt (sudo) or a yes/no question.
var promptRe = regexp.MustCompile(`(?i)(password|passphrase)[^:]*:\s*$|[\[(]y/n[\])]\??\s*$`)

// askRe matches brew's own confirmation ("==> Do you want to proceed with
// the upgrade? [y/n]"), which ends its line before waiting for one key.
var askRe = regexp.MustCompile(`(?i)\[y/n\]\s*$`)

// spinnerFrames animate a running job's title.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// drawer shows the current job's output live, under the tab. Output goes
// through a terminal emulator, so progress bars that redraw themselves
// with \r or cursor movement look as they would in a real terminal.
//
// One emulator serves every job and is reset between runs. It is never
// closed: in x/vt v0.1.0, Close races with the goroutine reading its
// replies, so that goroutine instead lives as long as the drawer.
type drawer struct {
	st         theme.Styles
	open       bool
	job        *jobs.Job
	term       *vt.SafeEmulator
	reply      *replyTo
	raw        strings.Builder // this run's output, for spotting lock errors
	cols, rows int
	retryAt    time.Time // when a lock-waiting job runs again
}

func newDrawer(st theme.Styles) drawer {
	d := drawer{st: st, cols: 80, rows: 10, reply: &replyTo{}}
	d.term = vt.NewSafeEmulator(d.cols, d.rows)
	// The emulator answers terminal queries ("where is the cursor?") on its
	// read side and blocks if nobody reads, so replies are always drained.
	go io.Copy(d.reply, d.term)
	return d
}

// replyTo forwards terminal replies to the running process. Replies are
// dropped when no process is running, or when the process can't take them
// (it has exited): the drain must never stop, or the emulator blocks.
type replyTo struct {
	mu sync.Mutex
	w  io.Writer
}

func (r *replyTo) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.w != nil {
		r.w.Write(b) // a write error only means the reply is lost
	}
	return len(b), nil
}

func (r *replyTo) set(w io.Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.w = w
}

// drawerHeight is how tall the drawer is for a body of height bodyH: about
// two fifths, between 6 and 14 lines, leaving the tab at least 3.
func drawerHeight(bodyH int) int {
	return max(min(bodyH*2/5, 14, bodyH-3), min(6, bodyH-3), 0)
}

// start shows a new run of j on a clean screen. Terminal replies go to
// reply, the running process.
func (d *drawer) start(j *jobs.Job, reply io.Writer) {
	d.job, d.open = j, true
	d.raw.Reset()
	d.term.Write([]byte("\x1bc")) // full terminal reset: screen, cursor, colors
	d.term.ClearScrollback()
	d.reply.set(reply)
}

// replay shows a finished job again from the output it printed.
func (d *drawer) replay(j *jobs.Job, output string) {
	d.start(j, nil)
	d.write([]byte(output))
}

// stop detaches the finished process: later replies are dropped.
func (d *drawer) stop() { d.reply.set(nil) }

// write feeds process output to the terminal.
func (d *drawer) write(b []byte) {
	d.term.Write(b)
	d.raw.Write(b)
}

// output is everything this run has printed.
func (d *drawer) output() string { return d.raw.String() }

// resize sets the terminal's size, which is the drawer's inner area.
func (d *drawer) resize(cols, rows int) {
	d.cols, d.rows = max(cols, 1), max(rows, 1)
	d.term.Resize(d.cols, d.rows)
}

// prompt reports whether the process is waiting for typed input: the text
// before the cursor looks like a password prompt or a y/n question, or the
// cursor starts an empty line under brew's own [y/n] question.
func (d *drawer) prompt() bool {
	pos := d.term.CursorPosition()
	lines := strings.Split(ansi.Strip(d.term.Render()), "\n")
	if pos.Y < 0 || pos.Y >= len(lines) {
		return false
	}
	before := ansi.Truncate(lines[pos.Y], pos.X, "")
	if strings.TrimSpace(before) == "" && pos.Y > 0 && askRe.MatchString(lines[pos.Y-1]) {
		return true
	}
	return promptRe.MatchString(before)
}

// view renders the drawer as a w×h pane. frame picks the spinner frame;
// insert says whether keys currently go to the process.
func (d *drawer) view(w, h int, now time.Time, frame int, insert bool) string {
	st := d.st
	j := d.job
	if j == nil {
		return ""
	}
	cmd := "brew " + strings.Join(j.Args, " ")
	var title, hint string
	switch j.State {
	case jobs.Running:
		title = spinnerFrames[frame%len(spinnerFrames)] + " " + cmd + " · " + elapsed(now.Sub(j.Started))
		hint = "esc hide · ctrl+c stop · i type into brew"
		if insert {
			hint = st.OK.Render("typing goes to brew") + st.Muted.Render(" · esc stop typing")
		}
	case jobs.Waiting:
		title = "⏸ " + cmd + " · waiting for brew's lock"
		hint = fmt.Sprintf("another brew is running · retry %d of %d in %s · ctrl+c stop",
			j.Attempts+1, jobs.MaxAttempts, elapsed(max(d.retryAt.Sub(now), 0)))
	case jobs.Done:
		title = st.OK.Render("✓ "+cmd) + " · exit 0"
		hint = "esc close"
	default: // Failed, Canceled
		title = st.Err.Render("✗ "+cmd) + fmt.Sprintf(" · %s, exit %d", j.State, j.ExitCode)
		hint = "esc close"
	}
	body := d.term.Render()
	inner := max(h-4, 0) // border, title and hint lines
	lines := strings.Split(body, "\n")
	if len(lines) > inner {
		lines = lines[len(lines)-inner:]
	}
	for len(lines) < inner {
		lines = append(lines, "")
	}
	lines = append(lines, st.Muted.Render(hint))
	return pane(st, title, strings.Join(lines, "\n"), w, h, true)
}

// elapsed formats a duration as m:ss.
func elapsed(d time.Duration) string {
	s := int(d.Round(time.Second).Seconds())
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
