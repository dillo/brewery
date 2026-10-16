package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

var crashAt = time.Date(2026, 10, 16, 15, 4, 5, 0, time.Local)

// crashFiles lists the names in dir.
func crashFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestCrashLogKeepsWhatWasWritten(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	real := os.Stderr
	c, err := startCrashLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if os.Stderr == real {
		t.Fatal("stderr should point at the crash file while the TUI runs")
	}
	fmt.Fprint(os.Stderr, "Caught panic:\r\n\r\nboom\r\n")
	// The runtime's fatal errors write to file descriptor 2 directly.
	unix.Write(2, []byte("fatal error: concurrent map writes\n"))
	path := c.stop(crashAt)
	if path != filepath.Join(dir, "crash-20261016-150405.log") {
		t.Fatalf("stop returned %q", path)
	}
	if os.Stderr != real {
		t.Error("stop must put stderr back")
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "Caught panic:\r\n\r\nboom\r\nfatal error: concurrent map writes\n" {
		t.Errorf("crash log = %q, %v", b, err)
	}
}

func TestCrashLogLeavesNothingWithoutACrash(t *testing.T) {
	dir := t.TempDir()
	c, err := startCrashLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path := c.stop(crashAt); path != "" {
		t.Errorf("nothing was written: no crash, got %q", path)
	}
	if left := crashFiles(t, dir); len(left) != 0 {
		t.Errorf("files left behind: %v", left)
	}
}

func TestCrashLogsKeepTheNewestFive(t *testing.T) {
	dir := t.TempDir()
	for i := range 7 {
		c, err := startCrashLog(dir)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "crash %d\n", i)
		c.stop(crashAt.Add(time.Duration(i) * time.Minute))
	}
	want := []string{
		"crash-20261016-150605.log", "crash-20261016-150705.log", "crash-20261016-150805.log",
		"crash-20261016-150905.log", "crash-20261016-151005.log",
	}
	if got := crashFiles(t, dir); !slices.Equal(got, want) {
		t.Errorf("kept %v, want %v", got, want)
	}
}

func TestCrashLogAdoptsWhatADeadRunLeft(t *testing.T) {
	dir := t.TempDir()
	// A run killed by a fatal error left its temp file; pid 1 is launchd,
	// whose pid stays taken, so a file naming it counts as in use.
	dead := filepath.Join(dir, ".crash-2147483646-1.log")
	live := filepath.Join(dir, ".crash-1-2.log")
	empty := filepath.Join(dir, ".crash-2147483645-3.log")
	for path, body := range map[string]string{dead: "fatal error: out of memory\n", live: "", empty: ""} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(dead, crashAt, crashAt); err != nil {
		t.Fatal(err)
	}
	c, err := startCrashLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	c.stop(crashAt)
	adopted := filepath.Join(dir, "crash-20261016-150405.log")
	if !slices.Equal(c.earlier, []string{adopted}) {
		t.Errorf("earlier = %v", c.earlier)
	}
	if b, err := os.ReadFile(adopted); err != nil || string(b) != "fatal error: out of memory\n" {
		t.Errorf("adopted log = %q, %v", b, err)
	}
	if got := crashFiles(t, dir); !slices.Equal(got, []string{".crash-1-2.log", "crash-20261016-150405.log"}) {
		t.Errorf("left %v: the empty leftover goes, the live one stays", got)
	}
}
