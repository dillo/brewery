//go:build unix

package brew

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A timed-out query must return even when brew left a child process
// holding its output pipe open.
func TestExecRunnerKillsProcessGroupOnTimeout(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "brew")
	script := "#!/bin/sh\nsleep 30 &\nsleep 30\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := ExecRunner{Bin: bin}.Run(ctx, "info")
	if took := time.Since(start); took > 7*time.Second {
		t.Errorf("Run took %s after a 200ms timeout", took)
	}
	if err == nil {
		t.Error("want an error from a killed query")
	}
}
