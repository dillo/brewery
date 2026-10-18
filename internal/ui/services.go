package ui

import (
	"errors"
	"fmt"
	"image/color"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/theme"
)

// logTailBytes is how much of the end of a log the services preview reads.
const logTailBytes = 16 << 10

// logTail is the end of the log the services preview shows.
type logTail struct {
	path   string // the log shown, or being read
	text   string
	err    error
	loaded bool // the read of path has come back
	stale  bool // services were refreshed: read it again
	back   int  // lines the view is scrolled back from the newest
	most   int  // the most back can be, as of the last render
	page   int  // half a pane of log lines, as of the last render
}

// logMsg carries the end of a log file back to the App.
type logMsg struct {
	path, text string
	err        error
}

// readLogTail returns the last lines of the file at path as plain text: a
// log can hold colors and progress redraws that would garble the pane.
func readLogTail(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	off := max(info.Size()-logTailBytes, 0)
	b := make([]byte, info.Size()-off)
	n, err := f.ReadAt(b, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	s := strings.ToValidUTF8(string(b[:n]), "�")
	if off > 0 {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:] // the first line was cut
		}
	}
	lines := strings.Split(strings.TrimRight(ansi.Strip(s), "\r\n"), "\n")
	for i, l := range lines {
		l = strings.TrimSuffix(l, "\r")
		if j := strings.LastIndexByte(l, '\r'); j >= 0 {
			l = l[j+1:] // a redrawn line: keep what was drawn last
		}
		lines[i] = strings.ReplaceAll(l, "\t", "    ")
	}
	return strings.Join(lines, "\n"), nil
}

// selectedService is the service under the cursor, found by name.
func (a *App) selectedService() (brew.Service, bool) {
	row := a.services.table.SelectedRow()
	if a.services.n == 0 || row == nil {
		return brew.Service{}, false
	}
	for _, s := range a.snap.Services {
		if s.Name == row[0] {
			return s, true
		}
	}
	return brew.Service{}, false
}

// wantLog reads the selected service's log if the preview doesn't have it
// yet, or has an older copy from before the last refresh.
func (a *App) wantLog() tea.Cmd {
	s, ok := a.selectedService()
	if !ok || s.Log == "" {
		a.log = logTail{} // nothing to show: coming back to a log reads it afresh
		return nil
	}
	if s.Log == a.log.path && !a.log.stale {
		return nil
	}
	if s.Log != a.log.path {
		a.log = logTail{path: s.Log}
	}
	a.log.stale = false
	read, path := a.readLog, s.Log
	return func() tea.Msg {
		text, err := read(path)
		return logMsg{path, text, err}
	}
}

// gotLog keeps a log read, unless the selection has moved to another log.
func (a *App) gotLog(m logMsg) {
	if m.path == a.log.path {
		a.log.text, a.log.err, a.log.loaded = m.text, m.err, true
	}
}

// tilde shortens a path in the home directory to start with ~.
func tilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if rest, ok := strings.CutPrefix(path, home+"/"); ok {
		return "~/" + rest
	}
	return path
}

// pagerDoneMsg reports that the log viewer has exited.
type pagerDoneMsg struct{ err error }

// UsePager sets how the app builds the command that shows log files full
// screen, following what gets added. The UI never starts programs itself.
func (a *App) UsePager(cmd func(files []string) *exec.Cmd) {
	a.pagerCmd = cmd
}

// openLog shows the selected service's log full screen, and its error log
// when that is another file. Like a takeover, it waits for a running job.
func (a *App) openLog() tea.Cmd {
	s, ok := a.selectedService()
	if !ok {
		return nil
	}
	if a.pagerCmd == nil {
		return a.note("the log viewer isn't available")
	}
	var files []string
	for _, f := range []string{s.Log, s.ErrorLog} {
		if f != "" && !slices.Contains(files, f) {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return a.note("brew knows no log for " + s.Name)
	}
	files = slices.DeleteFunc(files, func(f string) bool {
		_, err := os.Stat(f)
		return err != nil
	})
	if len(files) == 0 {
		return a.note(s.Name + " hasn't logged anything yet")
	}
	if cur := a.queue.Current(); cur != nil {
		return a.note("wait for " + cur.Title + " to finish: the log takes over the terminal")
	}
	return tea.ExecProcess(a.pagerCmd(files), func(err error) tea.Msg { return pagerDoneMsg{err} })
}

// pagerDone reads the log again, since more may have been written while it
// was open, and says if the viewer failed.
func (a *App) pagerDone(m pagerDoneMsg) {
	a.log.stale = true
	if m.err != nil {
		a.flash = a.st.Err.Render("✗ couldn't show the log: " + m.err.Error())
	}
}

// serviceColor is the color of a service's state: green when it runs,
// red when it failed, muted when it's off, yellow for anything else.
func serviceColor(st theme.Styles, status string) color.Color {
	switch status {
	case "started", "scheduled":
		return st.P.Green
	case "error":
		return st.P.Red
	case "none", "stopped":
		return st.P.Overlay0
	}
	return st.P.Yellow
}

// serviceIcon is the icon for a service in this state.
func serviceIcon(ic theme.Icons, status string) string {
	switch status {
	case "started", "scheduled":
		return ic.Started
	case "error":
		return ic.Failed
	}
	return ic.Stopped
}

// serviceStatus is a service's status, with its exit code if it failed.
func serviceStatus(s brew.Service) string {
	if s.ExitCode != nil && *s.ExitCode != 0 {
		return fmt.Sprintf("%s (exit %d)", s.Status, *s.ExitCode)
	}
	return s.Status
}

// servicePreview shows the selected service's details over as much of the
// end of its log as fits a w×h pane.
func (a *App) servicePreview(w, h int) (title, body string) {
	st := a.st
	s, ok := a.selectedService()
	if !ok {
		return "Preview", ""
	}
	style := st.Warn
	switch s.Status {
	case "started", "scheduled":
		style = st.OK
	case "error":
		style = st.Err
	case "none", "stopped":
		style = st.Muted
	}
	head := style.Render(theme.With(serviceIcon(st.Icons, s.Status), serviceStatus(s)))
	if s.PID > 0 {
		head += st.Muted.Render(fmt.Sprintf(" · pid %d", s.PID))
	}
	errLog := ""
	if s.ErrorLog != s.Log {
		errLog = s.ErrorLog
	}
	top := head + "\n" + kv(st, "command", s.Command, "user", s.User, "file", tilde(s.File), "log", tilde(s.Log), "errors", tilde(errLog)) +
		"\n"

	var lines []string
	switch {
	case s.Log == "":
		lines = []string{st.Muted.Render("brew knows no log for this service")}
	case a.log.path != s.Log || !a.log.loaded:
		lines = []string{st.Muted.Render("loading…")}
	case errors.Is(a.log.err, fs.ErrNotExist):
		lines = []string{st.Muted.Render("nothing logged yet")}
	case a.log.err != nil:
		lines = []string{st.Err.Render(a.log.err.Error())}
	case a.log.text == "":
		lines = []string{st.Muted.Render("the log is empty")}
	default:
		lines = strings.Split(a.log.text, "\n")
	}
	_, ih := paneInner(w, h)
	room := max(ih-strings.Count(top, "\n")-1, 1) // the heading takes a line
	a.log.most, a.log.page = max(len(lines)-room, 0), max(room/2, 1)
	a.log.back = min(a.log.back, a.log.most)
	heading := "End of the log"
	if a.log.back > 0 {
		heading = fmt.Sprintf("The log · %d lines back", a.log.back)
	}
	end := len(lines) - a.log.back
	lines = lines[max(end-room, 0):end] // the lines that fit, which pane would cut
	return s.Name, top + st.Accent.Render(heading) + "\n" + strings.Join(lines, "\n")
}

// scrollLog scrolls the log in Services' details by a key. A log that fits
// the pane has nowhere to go, so the keys do nothing.
func (a *App) scrollLog(k string) {
	l := &a.log
	switch k {
	case "j", "down":
		l.back--
	case "k", "up":
		l.back++
	case "ctrl+d":
		l.back -= l.page
	case "ctrl+u":
		l.back += l.page
	case "home":
		l.back = l.most
	case "end", "G":
		l.back = 0
	}
	l.back = min(max(l.back, 0), l.most)
}
