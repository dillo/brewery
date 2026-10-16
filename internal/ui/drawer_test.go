package ui

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dillo/brewery/internal/jobs"
)

// replies collects what the terminal sends back to the process.
type replies struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (r *replies) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(b)
}

func (r *replies) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

func runningJob() *jobs.Job {
	var q jobs.Queue
	j := q.Add("upgrade fd", []string{"upgrade", "fd"})
	q.Next(testNow)
	return j
}

func newTestDrawer(t *testing.T, j *jobs.Job) *drawer {
	d := newDrawer(testStyles)
	d.resize(60, 6)
	d.start(j, io.Discard)
	t.Cleanup(d.stop)
	return &d
}

func TestDrawerRendersLikeATerminal(t *testing.T) {
	d := newTestDrawer(t, runningJob())
	d.write([]byte("\x1b[34m==>\x1b[0m Upgrading fd\r\n"))
	d.write([]byte("####      20%"))
	d.write([]byte("\r##########100%\r\n"))
	view := d.view(64, 10, testNow.Add(42*time.Second), 2, false)
	out := stripANSI(view)
	for _, want := range []string{"⠹ brew upgrade fd · 0:42", "==> Upgrading fd", "##########100%", "esc hide · ctrl+c stop"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "20%") {
		t.Error("\\r should overwrite the progress line, as a terminal does")
	}
	if lipgloss.Height(view) != 10 || lipgloss.Width(view) != 64 {
		t.Errorf("drawer is %dx%d, want 64x10", lipgloss.Width(view), lipgloss.Height(view))
	}
}

func TestDrawerShowsTheNewestLines(t *testing.T) {
	d := newTestDrawer(t, runningJob())
	for i := range 20 {
		d.write([]byte("line " + string(rune('a'+i)) + "\r\n"))
	}
	out := stripANSI(d.view(64, 10, testNow, 0, false))
	if !strings.Contains(out, "line s") || strings.Contains(out, "line a") {
		t.Errorf("the drawer should follow the end of the output:\n%s", out)
	}
}

func TestDrawerPromptDetection(t *testing.T) {
	tests := []struct {
		output string
		want   bool
	}{
		{"==> Installing docker-desktop\r\nPassword:", true},
		{"[sudo] password for dillo: ", true},
		{"Do you want to continue? [y/N] ", true},
		{"Proceed (y/n)? ", true},
		{"==> Downloading https://example.com/x.dmg\r\n", false},
		{"Password:\r\nok, thanks\r\n", false}, // answered: the cursor moved on
		{"Password:\r\n", false},               // answered with enter
		// brew 7's own confirmation ends its line and waits for one key.
		{"==> Upgrading 2 outdated packages:\r\n==> Do you want to proceed with the upgrade? [y/n]\r\n", true},
		{"==> Do you want to proceed with the upgrade? [y/n]\r\ny\r\n", false},
	}
	for _, tt := range tests {
		d := newTestDrawer(t, runningJob())
		d.write([]byte(tt.output))
		if got := d.prompt(); got != tt.want {
			t.Errorf("prompt() after %q = %v, want %v", tt.output, got, tt.want)
		}
	}
}

func TestDrawerForwardsTerminalReplies(t *testing.T) {
	d := newDrawer(testStyles)
	r := &replies{}
	d.start(runningJob(), r)
	defer d.stop()
	d.write([]byte("\x1b[6n")) // "where is the cursor?" would block without a reader
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(r.String(), "\x1b[") {
		if time.Now().After(deadline) {
			t.Fatal("the cursor-position reply never reached the process")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDrawerTitles(t *testing.T) {
	var q jobs.Queue
	j := q.Add("upgrade fd", []string{"upgrade", "fd"})
	q.Next(testNow)
	d := newTestDrawer(t, j)

	if out := stripANSI(d.view(70, 8, testNow, 0, true)); !strings.Contains(out, "typing goes to brew") {
		t.Errorf("insert hint missing:\n%s", out)
	}
	q.Finish(j, 1, false, "Error: A `brew update` process has already locked /x.", testNow)
	d.retryAt = testNow.Add(4 * time.Second)
	if out := stripANSI(d.view(70, 8, testNow, 0, false)); !strings.Contains(out, "waiting for brew's lock") || !strings.Contains(out, "retry 2 of 5 in 0:04") {
		t.Errorf("waiting title wrong:\n%s", out)
	}
	j.State, j.ExitCode = jobs.Done, 0
	if out := stripANSI(d.view(70, 8, testNow, 0, false)); !strings.Contains(out, "✓ brew upgrade fd · exit 0") {
		t.Errorf("done title wrong:\n%s", out)
	}
	j.State, j.ExitCode = jobs.Failed, 1
	if out := stripANSI(d.view(70, 8, testNow, 0, false)); !strings.Contains(out, "✗ brew upgrade fd · failed, exit 1") {
		t.Errorf("failed title wrong:\n%s", out)
	}
}

func TestDrawerStartsEachRunOnACleanScreen(t *testing.T) {
	d := newTestDrawer(t, runningJob())
	d.write([]byte("\x1b[31mfirst run\r\n"))
	d.start(d.job, io.Discard)
	if d.output() != "" {
		t.Errorf("a new run starts with empty output, got %q", d.output())
	}
	d.write([]byte("second"))
	out := d.view(64, 10, testNow, 0, false)
	if strings.Contains(stripANSI(out), "first run") {
		t.Errorf("the previous run's text should be gone:\n%s", stripANSI(out))
	}
	if strings.Contains(out, "\x1b[31msecond") {
		t.Error("colors from the previous run must not leak into the next")
	}
}

func TestDrawerDropsRepliesAfterStop(t *testing.T) {
	d := newDrawer(testStyles)
	r := &replies{}
	d.start(runningJob(), r)
	d.stop()
	d.write([]byte("\x1b[6n"))
	time.Sleep(50 * time.Millisecond)
	if r.String() != "" {
		t.Errorf("a stopped run must not get replies: %q", r.String())
	}
}

func TestDrawerHeight(t *testing.T) {
	for _, tt := range []struct{ body, want int }{{40, 16}, {37, 16}, {21, 10}, {13, 7}, {12, 6}, {9, 5}, {7, 5}, {3, 3}} {
		if got := drawerHeight(tt.body); got != tt.want {
			t.Errorf("drawerHeight(%d) = %d, want %d", tt.body, got, tt.want)
		}
	}
}

func TestElapsed(t *testing.T) {
	if elapsed(65*time.Second) != "1:05" || elapsed(0) != "0:00" {
		t.Error("elapsed format wrong")
	}
}

// failingWriter is a process that can no longer take input, like a PTY
// whose child has exited. It reports each write attempt on called.
type failingWriter struct{ called chan struct{} }

func (w failingWriter) Write(b []byte) (int, error) {
	select {
	case w.called <- struct{}{}:
	default:
	}
	return 0, syscall.EIO
}

func TestDrawerKeepsDrainingAfterAFailedReply(t *testing.T) {
	d := newTestDrawer(t, runningJob())
	called := make(chan struct{}, 1)
	d.start(runningJob(), failingWriter{called: called})
	d.write([]byte("\x1b[6n")) // the reply goes to the failing writer
	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("the reply never reached the process")
	}

	// A later query must not block the emulator, even though that reply
	// could not be delivered.
	d.start(runningJob(), io.Discard)
	done := make(chan struct{})
	go func() {
		d.write([]byte("\x1b[6n"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("write blocked: the reply drain stopped after a failed reply")
	}
}

func TestFrameFitsSmallWindows(t *testing.T) {
	for _, h := range []int{6, 7, 8, 9, 10, 12, 16, 24, 30, 40} {
		a := newJobApp(t, &fakeStarter{})
		a.Update(tea.WindowSizeMsg{Width: 80, Height: h})
		if got := lipgloss.Height(a.render()); got != h {
			t.Errorf("height %d, no job: rendered %d lines", h, got)
		}
		p := newFakeProc(0, "working")
		a = newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
		a.Update(tea.WindowSizeMsg{Width: 80, Height: h})
		settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
		out := a.render()
		if got := lipgloss.Height(out); got != h {
			t.Errorf("height %d, drawer open: rendered %d lines", h, got)
		}
		if h >= 10 && !strings.Contains(stripANSI(out), "working") {
			t.Errorf("height %d: the drawer should show the job's output:\n%s", h, stripANSI(out))
		}
		if lines := strings.Split(stripANSI(out), "\n"); !strings.Contains(lines[len(lines)-1], "RUNNING") {
			t.Errorf("height %d: the status bar must stay at the bottom:\n%s", h, stripANSI(out))
		}
	}
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{newFakeProc(0)}})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	settle(t, a, a.run(action{title: "x", args: []string{"x"}}))
	if a.drawer.rows != 6 {
		t.Errorf("at 80x24 the drawer shows %d lines of output, want 6", a.drawer.rows)
	}
}

func TestQuestionsShowInTinyWindows(t *testing.T) {
	p := newFakeProc(0, "working")
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 7}) // the drawer takes the whole body
	settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	a.Update(press("ctrl+c"))
	out := stripANSI(a.render())
	if !strings.Contains(out, "Stop upgrade fd?") || lipgloss.Height(out) != 7 {
		t.Errorf("the stop question must show, in 7 lines:\n%s", out)
	}
}
