//go:build live

package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
)

// Runs a harmless brew command as a job through the real pseudo-terminal
// runner and checks it shows up in the drawer.
// Run with: go test -tags live ./internal/ui/
func TestLiveJob(t *testing.T) {
	bin, err := brew.Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	start := func(args, env []string, cols, rows int) (Proc, error) {
		return brew.StartPTY(bin, args, env, cols, rows)
	}
	a := New(newFakeSource(t), start, state.Snapshot{}, filepath.Join(t.TempDir(), "s.json"))
	a.tickEvery = 0
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	waiting := settle(t, a, a.run(action{title: "brew --version", args: []string{"--version"}}))
	for deadline := time.Now().Add(10 * time.Second); len(waiting) > 0; {
		if time.Now().After(deadline) {
			t.Fatal("brew --version did not finish")
		}
		waiting = resume(t, a, waiting)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "Homebrew") || !strings.Contains(out, "✓ brew --version · exit 0") {
		t.Errorf("drawer after brew --version:\n%s", out)
	}
}
