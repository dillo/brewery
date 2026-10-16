package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"golang.org/x/sys/unix"
)

// keepCrashes is how many crash logs are kept; older ones are removed.
const keepCrashes = 5

// crashLog keeps what brewery prints if it crashes. Bubble Tea catches a
// panic, restores the terminal, then writes the panic and its stack to
// os.Stderr; while the TUI runs, os.Stderr points at a file instead, so
// that report lands in a crash log. File descriptor 2 points there too, for
// the Go runtime's fatal errors (concurrent map writes, out of memory),
// which bypass os.Stderr and end the process at once. Nothing else writes
// to stderr while the TUI is up: it would garble the screen.
type crashLog struct {
	dir    string
	f      *os.File
	stderr *os.File // the real stderr, put back by stop
	// terminal is a copy of the real file descriptor 2, for the full-screen
	// commands that must print to the terminal; stop puts it back as fd 2
	// and closes it. It is not inherited by children unless handed over
	// as a cmd.Stderr.
	terminal *os.File
	earlier  []string // crash logs left by runs that died without stop
}

// startCrashLog points os.Stderr and file descriptor 2 at a new temp file
// in dir. A run killed by a fatal error never calls stop, so temp files
// left in dir by earlier runs become crash logs first (or are removed when
// empty), and earlier lists them.
func startCrashLog(dir string) (*crashLog, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	earlier := adoptLeftovers(dir)
	f, err := os.CreateTemp(dir, fmt.Sprintf(".crash-%d-*.log", os.Getpid()))
	if err != nil {
		return nil, err
	}
	// close-on-exec, so only a command given terminal as its stderr has it
	fd2, err := unix.FcntlInt(2, unix.F_DUPFD_CLOEXEC, 0)
	if err == nil {
		err = unix.Dup2(int(f.Fd()), 2)
		if err != nil {
			unix.Close(fd2)
		}
	}
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	c := &crashLog{dir: dir, f: f, stderr: os.Stderr, terminal: os.NewFile(uintptr(fd2), "/dev/stderr"), earlier: earlier}
	os.Stderr = f
	return c, nil
}

// stop puts stderr back. If anything was written, the file becomes a crash
// log named for now, and stop returns its path; otherwise the file is
// removed and stop returns "".
func (c *crashLog) stop(now time.Time) string {
	_ = unix.Dup2(int(c.terminal.Fd()), 2) // best effort: brewery is on its way out
	c.terminal.Close()
	os.Stderr = c.stderr
	info, err := c.f.Stat()
	c.f.Close()
	if err != nil || info.Size() == 0 {
		os.Remove(c.f.Name())
		return ""
	}
	return keepCrash(c.dir, c.f.Name(), now)
}

// keepCrash renames the temp file tmp to a crash log named for at, removes
// the oldest logs beyond keepCrashes, and returns where the log is.
func keepCrash(dir, tmp string, at time.Time) string {
	path := filepath.Join(dir, "crash-"+at.Format("20060102-150405")+".log")
	if err := os.Rename(tmp, path); err != nil {
		path = tmp // keep it where it is
	}
	logs, _ := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	slices.Sort(logs) // the names sort by time
	for _, old := range logs[:max(len(logs)-keepCrashes, 0)] {
		os.Remove(old)
	}
	return path
}

// adoptLeftovers turns temp files left by runs that never reached stop
// into crash logs, named for when they were last written, and returns
// them. Empty ones are removed. A temp file names its run's process, so
// one that another brewery is still using is left alone.
func adoptLeftovers(dir string) []string {
	tmps, _ := filepath.Glob(filepath.Join(dir, ".crash-*.log"))
	var kept []string
	for _, tmp := range tmps {
		if running(tmp) {
			continue
		}
		info, err := os.Stat(tmp)
		if err != nil {
			continue
		}
		if info.Size() == 0 {
			os.Remove(tmp)
			continue
		}
		kept = append(kept, keepCrash(dir, tmp, info.ModTime()))
	}
	return kept
}

// running reports whether the process named in the temp file's name
// (.crash-<pid>-….log) is still running.
func running(tmp string) bool {
	var pid int
	if _, err := fmt.Sscanf(filepath.Base(tmp), ".crash-%d-", &pid); err != nil || pid <= 0 {
		return false
	}
	err := unix.Kill(pid, 0)
	return err == nil || errors.Is(err, unix.EPERM)
}
