package main

import (
	"os"
	"path/filepath"
)

// crashLog keeps what brewery prints if it crashes. Bubble Tea catches a
// panic, restores the terminal, then writes the panic and its stack to
// os.Stderr; while the TUI runs, os.Stderr points at a file instead, so
// that report lands in crash.log. Nothing else writes to stderr while the
// TUI is up: it would garble the screen.
type crashLog struct {
	path   string
	f      *os.File
	stderr *os.File // the real stderr, put back by stop
}

// startCrashLog points os.Stderr at a new temp file next to path.
func startCrashLog(path string) (*crashLog, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".crash-*.log")
	if err != nil {
		return nil, err
	}
	c := &crashLog{path: path, f: f, stderr: os.Stderr}
	os.Stderr = f
	return c, nil
}

// stop puts os.Stderr back. If anything was written, the file becomes the
// crash log and stop reports true; otherwise the file is removed.
func (c *crashLog) stop() bool {
	os.Stderr = c.stderr
	info, err := c.f.Stat()
	c.f.Close()
	if err != nil || info.Size() == 0 {
		os.Remove(c.f.Name())
		return false
	}
	if err := os.Rename(c.f.Name(), c.path); err != nil {
		c.path = c.f.Name() // keep it where it is
	}
	return true
}
