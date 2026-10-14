package ui

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/jobs"
)

// fakeTakeover records the commands handed the whole terminal.
type fakeTakeover struct{ calls [][]string }

func (f *fakeTakeover) cmd(args []string) *exec.Cmd {
	f.calls = append(f.calls, args)
	return exec.Command("true")
}

// runLine types line at the prompt and presses enter, returning the
// command enter produced without running it.
func runLine(t *testing.T, a *App, line string) tea.Cmd {
	t.Helper()
	keys(t, a, ":")
	typeText(t, a, line)
	_, cmd := a.Update(press("enter"))
	return cmd
}

func TestFullScreenCommandsTakeOver(t *testing.T) {
	for _, tt := range []struct {
		line string
		want []string // nil: runs in the drawer
	}{
		{"edit fd", []string{"edit", "fd"}},
		{"sh", []string{"sh"}},
		{"irb", []string{"irb"}},
		{"bundle edit", []string{"bundle", "edit"}},
		{"bundle sh", []string{"bundle", "sh"}},
		{"!info fd", []string{"info", "fd"}},
		{"! brew log git", []string{"log", "git"}},
		{"bundle list", nil},
		{"info fd", nil},
	} {
		fs, tk := &fakeStarter{}, &fakeTakeover{}
		a := newJobApp(t, fs)
		a.UseTakeover(tk.cmd)
		cmd := runLine(t, a, tt.line)
		if tt.want == nil {
			if len(tk.calls) != 0 {
				t.Errorf(":%s should run in the drawer, took over with %v", tt.line, tk.calls)
			}
			continue
		}
		if len(tk.calls) != 1 || !reflect.DeepEqual(tk.calls[0], tt.want) || cmd == nil {
			t.Errorf(":%s took over with %v (cmd %v), want %v", tt.line, tk.calls, cmd != nil, tt.want)
		}
		if len(fs.calls) != 0 {
			t.Errorf(":%s also started a drawer job: %v", tt.line, fs.calls)
		}
	}
}

func TestTakeoverWaitsForTheRunningJob(t *testing.T) {
	p := newFakeProc(0)
	tk := &fakeTakeover{}
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	a.UseTakeover(tk.cmd)
	waiting := settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	runLine(t, a, "edit fd")
	if len(tk.calls) != 0 {
		t.Error("a full-screen command must wait while a job runs")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "wait for upgrade fd to finish") {
		t.Errorf("status bar should say why: %q", bar)
	}
	p.exit()
	resume(t, a, waiting)
}

func TestTakeoverWithoutAProgram(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	if cmd := runLine(t, a, "edit fd"); cmd != nil {
		t.Error("without a takeover program nothing should run")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "full-screen commands aren't available") {
		t.Errorf("status bar: %q", bar)
	}
}

func TestFinishedTakeoverIsRecordedAndRefreshes(t *testing.T) {
	src := newCountingSource(t)
	tk := &fakeTakeover{}
	a := New(src, (&fakeStarter{}).start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	a.UseTakeover(tk.cmd)
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	runLine(t, a, "edit fd")
	drain(t, a, func() tea.Msg { return takeoverDoneMsg{job: a.takingOver, err: nil} })

	if len(a.history) != 1 || a.history[0].State != jobs.Done || a.history[0].Title != "edit fd" {
		t.Fatalf("history: %+v", a.history)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "✓ edit fd") {
		t.Errorf("status bar: %q", stripANSI(a.statusBar()))
	}
	for _, q := range []string{"installed", "outdated", "services", "taps", "doctor"} {
		if src.calls[q] != 1 {
			t.Errorf("after a full-screen command, %s ran %d times, want 1", q, src.calls[q])
		}
	}
}

func TestFailedTakeoverKeepsItsExitCode(t *testing.T) {
	tk := &fakeTakeover{}
	a := newJobApp(t, &fakeStarter{})
	a.UseTakeover(tk.cmd)
	runLine(t, a, "sh")
	exitErr := exec.Command("sh", "-c", "exit 3").Run()
	drain(t, a, func() tea.Msg { return takeoverDoneMsg{job: a.takingOver, err: exitErr} })
	if r := a.history[0]; r.State != jobs.Failed || r.ExitCode != 3 {
		t.Errorf("record: %+v", r)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "✗ sh failed (exit 3)") {
		t.Errorf("status bar: %q", bar)
	}
}
