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
	if _, err := unix.Write(2, []byte("fatal error: concurrent map writes\n")); err != nil {
		t.Fatal(err)
	}
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

func TestCrashLogHandsOutTheTerminalsStderr(t *testing.T) {
	real, err := os.Stderr.Stat()
	if err != nil {
		t.Fatal(err)
	}
	c, err := startCrashLog(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.stop(crashAt)
	term, err := c.terminal.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(real, term) {
		t.Error("terminal should be the real stderr, the one full-screen commands print to")
	}
	crashFile, err := c.f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(crashFile, term) {
		t.Error("terminal must not be the crash file")
	}
}

func TestCrashLogNeedsAWritableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to a read-only directory")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	real := os.Stderr
	var before, after unix.Stat_t
	if err := unix.Fstat(2, &before); err != nil {
		t.Fatal(err)
	}
	if c, err := startCrashLog(dir); err == nil {
		c.stop(crashAt)
		t.Fatal("startCrashLog should fail when it can't create the file")
	}
	if err := unix.Fstat(2, &after); err != nil {
		t.Fatal(err)
	}
	if os.Stderr != real || before.Dev != after.Dev || before.Ino != after.Ino {
		t.Error("a crash log that couldn't start must leave stderr alone")
	}
}

func TestCrashLogStaysPutWhenItCantBeRenamed(t *testing.T) {
	dir := t.TempDir()
	// A non-empty directory holds the crash log's name: the rename fails.
	if err := os.MkdirAll(filepath.Join(dir, "crash-20261016-150405.log", "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	c, err := startCrashLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, "boom\n")
	path := c.stop(crashAt)
	if path != c.f.Name() {
		t.Errorf("stop returned %q, want the temp file %q, kept where it is", path, c.f.Name())
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "boom\n" {
		t.Errorf("crash log = %q, %v", b, err)
	}
}

func TestCrashLogAdoptsALeftoverWithoutAPid(t *testing.T) {
	dir := t.TempDir()
	odd := filepath.Join(dir, ".crash-x-1.log") // names no process, so none is using it
	if err := os.WriteFile(odd, []byte("fatal error\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(odd, crashAt, crashAt); err != nil {
		t.Fatal(err)
	}
	c, err := startCrashLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	c.stop(crashAt)
	if want := []string{filepath.Join(dir, "crash-20261016-150405.log")}; !slices.Equal(c.earlier, want) {
		t.Errorf("earlier = %v, want %v", c.earlier, want)
	}
}
