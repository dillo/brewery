package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCrashLogKeepsWhatWasWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "crash.log")
	real := os.Stderr
	c, err := startCrashLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.Stderr == real {
		t.Fatal("stderr should point at the crash file while the TUI runs")
	}
	fmt.Fprint(os.Stderr, "Caught panic:\r\n\r\nboom\r\n")
	if !c.stop() {
		t.Fatal("something was written: stop should report a crash")
	}
	if os.Stderr != real {
		t.Error("stop must put stderr back")
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "Caught panic:\r\n\r\nboom\r\n" {
		t.Errorf("crash.log = %q, %v", b, err)
	}
}

func TestCrashLogLeavesNothingWithoutACrash(t *testing.T) {
	dir := t.TempDir()
	c, err := startCrashLog(filepath.Join(dir, "crash.log"))
	if err != nil {
		t.Fatal(err)
	}
	if c.stop() {
		t.Error("nothing was written: no crash")
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("files left behind: %v", left)
	}
}
