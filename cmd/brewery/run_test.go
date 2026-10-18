package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/ui"
	"golang.org/x/sys/unix"
)

// fakeMachine points brewery's home, cache and state at a temp dir, finds
// a brew that is new enough without running it, and runs program in place
// of the TUI. It returns brewery's state dir, where crash logs go.
func fakeMachine(t *testing.T, program func(tea.Model) error) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	locateBrew = func() (string, error) { return "/opt/homebrew/bin/brew", nil }
	brewVersion = func(string) (brew.Version, error) { return brew.Version{Major: 7, Minor: 0, Text: "7.0.9"}, nil }
	realProgram := runProgram
	runProgram = program
	t.Cleanup(func() { locateBrew, brewVersion, runProgram = brew.Locate, readBrewVersion, realProgram })
	return filepath.Join(home, "state", "brewery")
}

// logsIn lists the crash logs in dir.
func logsIn(t *testing.T, dir string) []string {
	t.Helper()
	logs, err := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	if err != nil {
		t.Fatal(err)
	}
	return logs
}

func TestRunStartsTheApp(t *testing.T) {
	real := os.Stderr
	var got tea.Model
	var stderrDuring *os.File
	dir := fakeMachine(t, func(m tea.Model) error {
		got, stderrDuring = m, os.Stderr
		return nil
	})
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if _, ok := got.(*ui.App); !ok {
		t.Errorf("the program ran %T, want *ui.App", got)
	}
	if stderrDuring == real {
		t.Error("while the TUI runs, stderr should be the crash log")
	}
	if os.Stderr != real {
		t.Error("run must put stderr back")
	}
	if out.Len() != 0 || errb.Len() != 0 {
		t.Errorf("a clean run prints nothing: stdout %q, stderr %q", out.String(), errb.String())
	}
	if logs := logsIn(t, dir); len(logs) != 0 {
		t.Errorf("a clean run leaves no crash log: %v", logs)
	}
}

func TestRunReportsTheProgramsError(t *testing.T) {
	fakeMachine(t, func(tea.Model) error { return errors.New("could not open a new TTY") })
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if want := "brewery: could not open a new TTY\n"; errb.String() != want {
		t.Errorf("stderr %q, want %q", errb.String(), want)
	}
}

func TestRunReportsACrash(t *testing.T) {
	real := os.Stderr
	dir := fakeMachine(t, func(tea.Model) error {
		// What Bubble Tea does with a panic: restore the terminal, then
		// print the panic to os.Stderr and return ErrProgramPanic.
		fmt.Fprint(os.Stderr, "Caught panic:\r\n\r\nboom\r\n")
		return tea.ErrProgramPanic
	})
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if os.Stderr != real {
		t.Error("run must put stderr back")
	}
	logs := logsIn(t, dir)
	if len(logs) != 1 {
		t.Fatalf("want one crash log, got %v", logs)
	}
	if b, err := os.ReadFile(logs[0]); err != nil || string(b) != "Caught panic:\r\n\r\nboom\r\n" {
		t.Errorf("crash log = %q, %v", b, err)
	}
	want := "brewery crashed. What happened is in " + logs[0] + "; please include it when reporting this.\n"
	if errb.String() != want {
		t.Errorf("stderr %q, want %q (and not the program's error too)", errb.String(), want)
	}
}

func TestRunMentionsLastTimesCrash(t *testing.T) {
	dir := fakeMachine(t, func(tea.Model) error { return nil })
	// A run killed by a fatal error left its temp file behind; no process
	// has the pid it names.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".crash-2147483646-1.log"), []byte("fatal error: out of memory\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr %q: the crash was last time, this run went fine", code, errb.String())
	}
	logs := logsIn(t, dir)
	if len(logs) != 1 {
		t.Fatalf("want the leftover kept as a crash log, got %v", logs)
	}
	want := "brewery crashed last time. What happened is in " + logs[0] + "; please include it when reporting this.\n"
	if errb.String() != want {
		t.Errorf("stderr %q, want %q", errb.String(), want)
	}
}

func TestRunWithoutACrashLog(t *testing.T) {
	ran := false
	fakeMachine(t, func(tea.Model) error { ran = true; return nil })
	// The state dir can't be made (its parent is a file), so there is no
	// crash log; brewery runs anyway.
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", blocker)
	real := os.Stderr
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 0 || !ran {
		t.Fatalf("exit %d, ran %v, stderr %q", code, ran, errb.String())
	}
	if os.Stderr != real {
		t.Error("without a crash log, stderr is never moved")
	}
}

// fakeBrew writes a brew that runs script for --version and fails for
// anything else.
func fakeBrew(t *testing.T, script string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "brew")
	body := "#!/bin/sh\n[ \"$1\" = --version ] || exit 64\n" + script + "\n"
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestReadBrewVersion(t *testing.T) {
	bin := fakeBrew(t, `echo "Homebrew 4.6.3"; echo "Homebrew/homebrew-core (git revision 1a2b3c)"`)
	v, err := readBrewVersion(bin)
	if err != nil {
		t.Fatal(err)
	}
	if want := (brew.Version{Major: 4, Minor: 6, Text: "4.6.3"}); v != want {
		t.Errorf("got %+v, want %+v", v, want)
	}
}

func TestReadBrewVersionFails(t *testing.T) {
	bin := fakeBrew(t, `echo "Error: broken" >&2; exit 1`)
	if v, err := readBrewVersion(bin); err == nil || !strings.Contains(err.Error(), "broken") {
		t.Errorf("got %+v, %v: want brew's error", v, err)
	}
}

func TestRunAsksTheBrewItFound(t *testing.T) {
	ran := false
	fakeMachine(t, func(tea.Model) error { ran = true; return nil })
	bin := fakeBrew(t, `echo "Homebrew 4.6.3"`)
	locateBrew = func() (string, error) { return bin, nil }
	brewVersion = readBrewVersion
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 1 || ran {
		t.Fatalf("exit %d, ran %v: a too-old brew stops brewery before the TUI", code, ran)
	}
	if want := "brewery needs Homebrew 7.0 or newer (you have 4.6.3). Update it with `brew update`.\n"; errb.String() != want {
		t.Errorf("stderr %q, want %q", errb.String(), want)
	}
}

// toFile points file descriptor 2 at a new file for the rest of the test,
// standing in for the terminal's stderr, and returns the file's path.
func toFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "terminal")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := unix.Dup(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Dup2(int(f.Fd()), 2); err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() {
		_ = unix.Dup2(saved, 2)
		unix.Close(saved)
	})
	return path
}

func TestFullScreenCommandsPrintToTheTerminal(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "edited")
	// The real program, typing ":edit fd" and quitting once brew is done.
	// Its input is an *os.File, which exec hands to brew as it is; with an
	// io.Pipe, exec would wait forever on a copy of the input.
	dir := fakeMachine(t, func(m tea.Model) error {
		in, keys, err := os.Pipe()
		if err != nil {
			return err
		}
		defer in.Close()
		defer keys.Close()
		p := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(io.Discard), tea.WithWindowSize(100, 30))
		go func() {
			_, _ = io.WriteString(keys, ":edit fd\r")
			for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
				if _, err := os.Stat(marker); err == nil {
					break
				}
			}
			p.Quit()
		}()
		_, err = p.Run()
		return err
	})
	// brew edit prints to stderr and leaves a mark when done; every
	// query the app makes on the way fails.
	bin := filepath.Join(t.TempDir(), "brew")
	script := "#!/bin/sh\nif [ \"$1\" = edit ]; then echo \"editing $2\" >&2; touch '" + marker + "'; exit 0; fi\nexit 64\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	locateBrew = func() (string, error) { return bin, nil }
	terminal := toFile(t)
	var out, errb bytes.Buffer
	code := run(nil, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	b, err := os.ReadFile(terminal)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "editing fd\n") {
		t.Errorf("the terminal's stderr got %q: brew edit's stderr belongs there", b)
	}
	if logs := logsIn(t, dir); len(logs) != 0 {
		t.Errorf("brew edit's stderr went to a crash log: %v", logs)
	}
}
