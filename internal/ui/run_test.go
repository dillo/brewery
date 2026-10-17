package ui

import (
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dillo/brewery/internal/jobs"
)

// fakeProc is a scripted brew process. Output chunks are delivered in
// order; after the last one, Read blocks until the test calls exit (or the
// process is interrupted or killed), then returns io.EOF.
type fakeProc struct {
	mu          sync.Mutex
	out         chan []byte
	in          strings.Builder
	code        int
	exited      bool
	interrupted bool
	killed      bool
	cols, rows  int
}

func newFakeProc(code int, chunks ...string) *fakeProc {
	p := &fakeProc{out: make(chan []byte, len(chunks)+8), code: code}
	for _, c := range chunks {
		p.out <- []byte(c)
	}
	return p
}

// exit lets Read return io.EOF once the queued output is read.
func (p *fakeProc) exit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.exited {
		p.exited = true
		close(p.out)
	}
}

func (p *fakeProc) Read(b []byte) (int, error) {
	chunk, ok := <-p.out
	if !ok {
		return 0, io.EOF
	}
	return copy(b, chunk), nil
}

func (p *fakeProc) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.in.Write(b)
}

func (p *fakeProc) input() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.in.String()
}

func (p *fakeProc) Resize(cols, rows int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cols, p.rows = cols, rows
	return nil
}

func (p *fakeProc) Interrupt() error {
	p.mu.Lock()
	p.interrupted, p.code = true, 130
	p.mu.Unlock()
	p.exit()
	return nil
}

func (p *fakeProc) Kill() error {
	p.mu.Lock()
	p.killed, p.code = true, -1
	p.mu.Unlock()
	p.exit()
	return nil
}

func (p *fakeProc) Wait() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.code, nil
}

// fakeStarter hands out scripted processes in order and records each
// command it was asked to start.
type fakeStarter struct {
	mu    sync.Mutex
	procs []*fakeProc
	calls [][]string
	envs  [][]string
	err   error
}

func (f *fakeStarter) start(args, env []string, cols, rows int) (Proc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, args)
	f.envs = append(f.envs, env)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.procs) == 0 {
		p := newFakeProc(0)
		p.exit()
		return p, nil
	}
	p := f.procs[0]
	f.procs = f.procs[1:]
	_ = p.Resize(cols, rows)
	return p, nil
}

// newJobApp builds an app whose jobs run on fs, with the spinner off and no
// waiting between lock retries.
func newJobApp(t *testing.T, fs *fakeStarter) *App {
	t.Helper()
	a := New(newFakeSource(t), fs.start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.tickEvery = 0
	a.backoff = func(int) time.Duration { return 0 }
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return a
}

// parked holds the results of commands still running when settle gave up
// waiting, typically a read waiting for more output.
type parked []chan tea.Msg

// settle runs cmd and everything it leads to, like drain, but parks any
// command that takes longer than 100ms instead of blocking the test.
func settle(t *testing.T, a *App, cmd tea.Cmd) parked {
	t.Helper()
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		return deliver(t, a, msg)
	case <-time.After(100 * time.Millisecond):
		return parked{ch}
	}
}

// deliver feeds one message to the app and settles what follows.
func deliver(t *testing.T, a *App, msg tea.Msg) parked {
	t.Helper()
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		var out parked
		for i := range v.Len() {
			out = append(out, settle(t, a, v.Index(i).Interface().(tea.Cmd))...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	_, next := a.Update(msg)
	return settle(t, a, next)
}

// resume waits for parked commands (after the test let them continue) and
// settles what follows.
func resume(t *testing.T, a *App, p parked) parked {
	t.Helper()
	var out parked
	for _, ch := range p {
		select {
		case msg := <-ch:
			out = append(out, deliver(t, a, msg)...)
		case <-time.After(2 * time.Second):
			t.Fatal("a parked command never finished")
		}
	}
	return out
}

func TestJobRunsAndShowsOutput(t *testing.T) {
	p := newFakeProc(0, "==> Upgrading fd\r\n", "🍺 fd 10.6.0 installed\r\n")
	fs := &fakeStarter{procs: []*fakeProc{p}}
	a := newJobApp(t, fs)

	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}, refresh: []query{qInstalled, qOutdated}}))
	if !reflect.DeepEqual(fs.calls, [][]string{{"upgrade", "fd"}}) {
		t.Fatalf("started %v", fs.calls)
	}
	if !reflect.DeepEqual(fs.envs[0], []string{"HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1"}) {
		t.Errorf("pane actions must not auto-update: env %v", fs.envs[0])
	}
	out := stripANSI(a.render())
	for _, want := range []string{"brew upgrade fd", "fd 10.6.0 installed", "RUNNING", "upgrade fd"} {
		if !strings.Contains(out, want) {
			t.Errorf("while running, missing %q:\n%s", want, out)
		}
	}
	if lipgloss.Height(a.render()) != 40 {
		t.Errorf("frame is %d lines with the drawer open, want 40", lipgloss.Height(a.render()))
	}

	p.exit()
	resume(t, a, waiting)
	if j := a.queue.Current(); j != nil {
		t.Fatalf("job still current: %+v", j)
	}
	out = stripANSI(a.render())
	if !strings.Contains(out, "✓ upgrade fd") || !strings.Contains(out, "NORMAL") {
		t.Errorf("after exit 0, missing the done flash or NORMAL:\n%s", out)
	}
	if !strings.Contains(out, "✓ brew upgrade fd · exit 0") {
		t.Errorf("the drawer should stay open showing the result:\n%s", out)
	}
}

func TestJobPTYSizeMatchesDrawer(t *testing.T) {
	p := newFakeProc(0)
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
	if p.cols != 116 || p.rows != 12 {
		t.Errorf("pty is %dx%d, want the drawer's inner 116x12", p.cols, p.rows)
	}
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if p.cols != 96 || p.rows != 10 {
		t.Errorf("after resize pty is %dx%d, want 96x10", p.cols, p.rows)
	}
	p.exit()
	resume(t, a, waiting)
}

func TestJobsQueueBehindTheRunningOne(t *testing.T) {
	first, second := newFakeProc(0), newFakeProc(0)
	fs := &fakeStarter{procs: []*fakeProc{first, second}}
	a := newJobApp(t, fs)
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	waiting = append(waiting, settle(t, a, a.run(action{title: "pin git", args: []string{"pin", "git"}}))...)
	if len(fs.calls) != 1 {
		t.Fatalf("the second job must wait: started %v", fs.calls)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "+1 queued") {
		t.Errorf("status bar should count the queued job: %q", bar)
	}
	first.exit()
	waiting = resume(t, a, waiting)
	if len(fs.calls) != 2 || fs.calls[1][0] != "pin" {
		t.Fatalf("the queued job should start next: %v", fs.calls)
	}
	second.exit()
	resume(t, a, waiting)
}

func TestFinishedJobRefreshesWhatItTouched(t *testing.T) {
	src := newCountingSource(t)
	p := newFakeProc(0)
	p.exit()
	a := New(src, (&fakeStarter{procs: []*fakeProc{p}}).start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	settle(t, a, a.run(action{title: "restart redis", args: []string{"services", "restart", "redis"}, refresh: []query{qServices}}))
	if src.calls["services"] != 1 || src.calls["installed"] != 0 || src.calls["doctor"] != 0 {
		t.Errorf("a services action should refresh only services: %v", src.calls)
	}
}

func TestFailedJobStillRefreshesAndSaysSo(t *testing.T) {
	p := newFakeProc(1, "Error: No such keg: /opt/homebrew/Cellar/fd\r\n")
	p.exit()
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	settle(t, a, a.run(action{title: "uninstall fd", args: []string{"uninstall", "fd"}, refresh: []query{qInstalled}}))
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "✗ uninstall fd failed (exit 1)") {
		t.Errorf("status bar: %q", bar)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "No such keg") || !strings.Contains(out, "failed, exit 1") {
		t.Errorf("the drawer should keep the error visible:\n%s", out)
	}
}

func TestStartFailureIsReported(t *testing.T) {
	a := newJobApp(t, &fakeStarter{err: errors.New("fork/exec /opt/homebrew/bin/brew: no such file")})
	settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "couldn't start brew") {
		t.Errorf("status bar: %q", bar)
	}
	if a.queue.Current() != nil {
		t.Error("a job that could not start must not stay current")
	}
}

func TestLockedJobWaitsAndRetries(t *testing.T) {
	locked := newFakeProc(1, "Error: A `brew update` process has already locked /opt/homebrew/var/homebrew/locks/update.\r\n")
	locked.exit()
	ok := newFakeProc(0, "done\r\n")
	ok.exit()
	fs := &fakeStarter{procs: []*fakeProc{locked, ok}}
	a := newJobApp(t, fs)
	settle(t, a, a.run(action{title: "upgrade", args: []string{"upgrade"}}))
	if len(fs.calls) != 2 {
		t.Fatalf("the job should be retried after the lock error: %v", fs.calls)
	}
	j := a.queue.Current()
	if j != nil {
		t.Fatalf("job should be finished: %+v", j)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "✓ upgrade") {
		t.Errorf("status bar: %q", bar)
	}
}

func TestPasswordPromptSwitchesToInsert(t *testing.T) {
	p := newFakeProc(0, "==> Installing docker-desktop\r\nPassword:")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	a.drawer.open = false
	waiting := settle(t, a, a.run(action{title: "upgrade docker-desktop", args: []string{"upgrade", "--cask", "docker-desktop"}}))
	if a.mode != modeInsert || !a.drawer.open {
		t.Fatalf("a password prompt should open the drawer in insert mode (mode %v, open %v)", a.mode, a.drawer.open)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "INSERT") {
		t.Errorf("status bar should show INSERT: %q", bar)
	}
	for _, k := range []string{"h", "u", "n", "t", "e", "r", "q", "enter"} {
		a.Update(press(k))
	}
	if got := p.input(); got != "hunterq\r" {
		t.Errorf("typed %q, want the keys passed straight to brew (q included)", got)
	}
	if a.confirm != nil {
		t.Error("q in insert mode must not ask to quit")
	}
	a.Update(press("esc"))
	if a.mode != modeNormal {
		t.Error("esc leaves insert mode")
	}
	p.exit()
	resume(t, a, waiting)
}

func TestPromptWaitsForTheCommandLine(t *testing.T) {
	p := newFakeProc(0, "==> Installing docker-desktop\r\n")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade docker-desktop", args: []string{"upgrade", "--cask", "docker-desktop"}}))
	keys(t, a, ":")
	typeText(t, a, "info")
	p.out <- []byte("Password:")
	waiting = resume(t, a, waiting)
	if a.mode != modeCommand || a.cmd.value() != "info" {
		t.Fatalf("a prompt must not take over the command line (mode %v, line %q)", a.mode, a.cmd.value())
	}
	if got := p.input(); got != "" {
		t.Errorf("nothing typed at the prompt may reach brew, got %q", got)
	}
	keys(t, a, "esc")
	if a.mode != modeInsert || !a.drawer.open {
		t.Errorf("closing the line should answer the waiting prompt (mode %v, open %v)", a.mode, a.drawer.open)
	}
	p.exit()
	resume(t, a, waiting)
}

func TestBrewsOwnConfirmationSwitchesToInsert(t *testing.T) {
	p := newFakeProc(0, "==> Upgrading fd\r\n  fd 10.5.0 -> 10.6.0\r\n==> Do you want to proceed with the upgrade? [y/n]\r\n")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	if a.mode != modeInsert {
		t.Fatalf("brew's [y/n] question should switch to insert, mode %v", a.mode)
	}
	keys(t, a, "y")
	if p.input() != "y" {
		t.Errorf("brew got %q, want y", p.input())
	}
	p.exit()
	resume(t, a, waiting)
}

func TestInsertModeEndsWithTheJob(t *testing.T) {
	p := newFakeProc(0, "Proceed? [y/N] ")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
	p.exit()
	resume(t, a, waiting)
	if a.mode != modeNormal {
		t.Error("insert mode should end when the job exits")
	}
}

func TestCtrlCAsksThenStopsTheJob(t *testing.T) {
	p := newFakeProc(0, "downloading…")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))

	a.Update(press("ctrl+c"))
	if a.confirm == nil || !strings.Contains(a.confirm.title, "Stop upgrade fd?") {
		t.Fatalf("ctrl+c should ask before stopping: %+v", a.confirm)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "[y] stop it") {
		t.Errorf("the confirmation should be drawn:\n%s", out)
	}
	a.Update(press("n"))
	if p.interrupted || a.confirm != nil {
		t.Fatal("n keeps the job running")
	}
	a.Update(press("ctrl+c"))
	_, cmd := a.Update(press("y"))
	if !p.interrupted {
		t.Fatal("y should interrupt brew")
	}
	waiting = append(waiting, settle(t, a, cmd)...)
	resume(t, a, waiting)
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "stopped upgrade fd") {
		t.Errorf("status bar: %q", bar)
	}
}

func TestStopDialogOnlyStopsTheJobItNamed(t *testing.T) {
	first, second := newFakeProc(0, "a"), newFakeProc(0, "b")
	fs := &fakeStarter{procs: []*fakeProc{first, second}}
	a := newJobApp(t, fs)
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	waiting = append(waiting, settle(t, a, a.run(action{title: "pin git", args: []string{"pin", "git"}}))...)

	a.Update(press("ctrl+c"))
	if a.confirm == nil || !strings.Contains(a.confirm.title, "Stop upgrade fd?") {
		t.Fatalf("ctrl+c should ask about the running job: %+v", a.confirm)
	}
	first.exit() // finishes while the dialog is open, so the queued job starts
	waiting = resume(t, a, waiting)
	if len(fs.calls) != 2 || fs.calls[1][0] != "pin" {
		t.Fatalf("the queued job should have started: %v", fs.calls)
	}
	_, cmd := a.Update(press("y"))
	waiting = append(waiting, settle(t, a, cmd)...)
	if a.confirm != nil {
		t.Error("y closes the dialog")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "upgrade fd already finished") {
		t.Errorf("y on a finished job should say so: %q", bar)
	}
	if second.interrupted || a.canceled {
		t.Error("y must not stop a job the dialog did not name")
	}
	if j := a.queue.Current(); j == nil || j.Title != "pin git" {
		t.Errorf("the queued job should still be running: %+v", j)
	}
	second.exit()
	resume(t, a, waiting)
}

func TestStoppingAJobWaitingForTheLock(t *testing.T) {
	locked := newFakeProc(1, "Error: A `brew update` process has already locked /x.\r\n")
	locked.exit()
	fs := &fakeStarter{procs: []*fakeProc{locked}}
	a := newJobApp(t, fs)
	a.backoff = func(int) time.Duration { return time.Hour } // keep it waiting
	waiting := settle(t, a, a.run(action{title: "upgrade", args: []string{"upgrade"}}))
	if j := a.queue.Current(); j == nil || j.State != jobs.Waiting {
		t.Fatalf("job should be waiting for the lock: %+v", j)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "waiting for brew's lock") {
		t.Errorf("status bar: %q", bar)
	}
	a.Update(press("ctrl+c"))
	_, cmd := a.Update(press("y"))
	settle(t, a, cmd)
	if a.queue.Current() != nil {
		t.Error("a stopped waiting job is done")
	}
	_ = waiting // the hour-long retry timer is abandoned with the test
}

// quits reports whether cmd, or a batch or sequence it returns, quits.
// Commands other than tea.Quit are not run, so a long tick can't block.
func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	if reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.Quit).Pointer() {
		return true
	}
	msg := cmd()
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		for i := range v.Len() {
			if c := v.Index(i).Interface().(tea.Cmd); c != nil && reflect.ValueOf(c).Pointer() == reflect.ValueOf(tea.Quit).Pointer() {
				return true
			}
		}
	}
	_, ok := msg.(tea.QuitMsg)
	return ok
}

// quitWithJob starts a job (and queues a second), then quits with q, y.
func quitWithJob(t *testing.T) (*App, *fakeProc, *fakeStarter, tea.Cmd) {
	t.Helper()
	p := newFakeProc(0, "working")
	fs := &fakeStarter{procs: []*fakeProc{p}}
	a := newJobApp(t, fs)
	a.quitGrace = time.Hour
	settle(t, a, a.run(action{title: "upgrade", args: []string{"upgrade"}}))
	a.run(action{title: "pin git", args: []string{"pin", "git"}})
	_, cmd := a.Update(press("q"))
	if cmd != nil || a.confirm == nil {
		t.Fatal("q with a running job should ask first")
	}
	_, cmd = a.Update(press("y"))
	return a, p, fs, cmd
}

func TestQuitStopsTheRunningJobFirst(t *testing.T) {
	a, p, fs, cmd := quitWithJob(t)
	if !p.interrupted || p.killed || cmd == nil {
		t.Fatalf("y should send brew ctrl+c and wait (interrupted=%v killed=%v)", p.interrupted, p.killed)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "stopping upgrade… · ctrl+c quits now") {
		t.Errorf("status bar: %q", bar)
	}
	a.Update(press("2"))
	if a.tab != tabHome {
		t.Error("keys do nothing while brewery is quitting")
	}
	j := a.queue.Current()
	_, cmd = a.Update(jobExitMsg{j.ID, j.Attempts, 130})
	if !quits(cmd) {
		t.Error("once the job has stopped, brewery quits")
	}
	if len(a.history) != 1 || a.history[0].State != jobs.Canceled {
		t.Errorf("the stopped job is recorded: %+v", a.history)
	}
	if len(fs.calls) != 1 {
		t.Errorf("the queued job must not start: %v", fs.calls)
	}
}

func TestQuitKillsAJobThatWontStop(t *testing.T) {
	a, p, _, _ := quitWithJob(t)
	_, cmd := a.Update(quitNowMsg{})
	if !p.killed || !quits(cmd) {
		t.Errorf("after the grace period brew is killed and brewery quits (killed=%v)", p.killed)
	}
}

func TestCtrlCWhileQuittingQuitsNow(t *testing.T) {
	a, p, _, _ := quitWithJob(t)
	_, cmd := a.Update(press("ctrl+c"))
	if !p.killed || !quits(cmd) {
		t.Errorf("a second ctrl+c doesn't wait (killed=%v)", p.killed)
	}
}

func TestDrawerKeys(t *testing.T) {
	p := newFakeProc(0)
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
	a.Update(press("esc"))
	if a.drawer.open {
		t.Fatal("esc hides the drawer")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "o output") {
		t.Errorf("a hidden drawer should be findable: %q", bar)
	}
	if h := lipgloss.Height(a.render()); h != 40 {
		t.Errorf("frame is %d lines with the drawer hidden", h)
	}
	a.Update(press("o"))
	if !a.drawer.open {
		t.Error("o shows it again")
	}
	a.Update(press("i"))
	if a.mode != modeInsert {
		t.Error("i types into the running job")
	}
	a.Update(press("esc"))
	p.exit()
	resume(t, a, waiting)
}

func TestKeyBytes(t *testing.T) {
	tests := map[string]string{"a": "a", "enter": "\r", "backspace": "\x7f", "ctrl+c": "\x03", "up": "\x1b[A"}
	for k, want := range tests {
		if got := string(keyBytes(press(k))); got != want {
			t.Errorf("keyBytes(%s) = %q, want %q", k, got, want)
		}
	}
}

func TestPasteGoesToBrewInInsertOnly(t *testing.T) {
	p := newFakeProc(0, "Password:")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade docker-desktop", args: []string{"upgrade", "--cask", "docker-desktop"}}))
	if a.mode != modeInsert {
		t.Fatalf("a password prompt should switch to insert mode, got %v", a.mode)
	}
	a.Update(tea.PasteMsg{Content: "hun ter2"})
	if got := p.input(); got != "hun ter2" {
		t.Errorf("pasted text should reach brew raw, got %q", got)
	}
	a.Update(press("esc"))
	a.Update(tea.PasteMsg{Content: "more"})
	if got := p.input(); got != "hun ter2" {
		t.Errorf("a paste in normal mode must write nothing, got %q", got)
	}
	p.exit()
	resume(t, a, waiting)
}

func TestFailureFlashOmitsOutputHintWhenNextJobStarts(t *testing.T) {
	first, second := newFakeProc(1, "boom"), newFakeProc(0, "next")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{first, second}})
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	waiting = append(waiting, settle(t, a, a.run(action{title: "pin git", args: []string{"pin", "git"}}))...)
	first.exit()
	waiting = resume(t, a, waiting)
	bar := stripANSI(a.statusBar())
	if !strings.Contains(bar, "✗ upgrade fd failed (exit 1)") || strings.Contains(bar, "o shows output") {
		t.Errorf("the next job replaces the output, so the flash must not point at it: %q", bar)
	}
	second.exit()
	resume(t, a, waiting)
}

func TestFailureFlashSurvivesTheCommandLine(t *testing.T) {
	p := newFakeProc(1, "boom")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	keys(t, a, ":")
	typeText(t, a, "inf")
	p.exit()
	resume(t, a, waiting)
	keys(t, a, "esc")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "✗ upgrade fd failed (exit 1)") {
		t.Errorf("a failure while the line was open should show once it closes: %q", bar)
	}
}

func TestInsertCtrlCCountsAsStop(t *testing.T) {
	p := newFakeProc(0, "Proceed? [y/N] ")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	if a.mode != modeInsert {
		t.Fatalf("the prompt should switch to insert mode, got %v", a.mode)
	}
	a.Update(press("ctrl+c"))
	if got := p.input(); got != "\x03" {
		t.Fatalf("ctrl+c should reach brew as 0x03, got %q", got)
	}
	if err := p.Interrupt(); err != nil { // brew dies of the 0x03, exit 130
		t.Fatal(err)
	}
	resume(t, a, waiting)
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "stopped upgrade fd") {
		t.Errorf("ctrl+c in insert should count as a stop: %q", bar)
	}
}

func TestStaleAttemptMessagesAreIgnored(t *testing.T) {
	locked := newFakeProc(1, "Error: A `brew update` process has already locked /opt/homebrew/var/homebrew/locks/update.\r\n")
	locked.exit()
	retry := newFakeProc(0, "retrying\r\n")
	fs := &fakeStarter{procs: []*fakeProc{locked, retry}}
	a := newJobApp(t, fs)
	waiting := settle(t, a, a.run(action{title: "upgrade", args: []string{"upgrade"}}))
	j := a.queue.Current()
	if j == nil || j.Attempts != 2 {
		t.Fatalf("the job should be on its second attempt: %+v", j)
	}
	before := a.drawer.output()
	deliver(t, a, jobOutputMsg{id: j.ID, attempt: 1, data: []byte("stale output\r\n")})
	deliver(t, a, jobExitMsg{id: j.ID, attempt: 1, code: 1})
	if got := a.drawer.output(); got != before {
		t.Errorf("stale output reached the drawer: %q -> %q", before, got)
	}
	if a.queue.Current() != j || j.State != jobs.Running || a.proc == nil {
		t.Errorf("a stale exit must not end the current attempt: %+v", j)
	}
	retry.exit()
	resume(t, a, waiting)
}

func TestQuitTimerAfterTheJobStoppedDoesNothing(t *testing.T) {
	a, _, _, _ := quitWithJob(t)
	j := a.queue.Current()
	a.Update(jobExitMsg{j.ID, j.Attempts, 130}) // stopped: brewery saves it, then quits
	if _, cmd := a.Update(quitNowMsg{}); cmd != nil {
		t.Error("a late grace timer mustn't quit before the save is done")
	}
}
