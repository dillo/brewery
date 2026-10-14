//go:build live

package brew

import (
	"context"
	"io"
	"strings"
	"testing"
)

// Runs the real brew, read-only, to catch output-format drift.
// Run with: go test -tags live ./internal/brew/
func TestLiveQueries(t *testing.T) {
	bin, err := Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	c := NewClient(ExecRunner{Bin: bin, Env: QueryEnv})
	ctx := context.Background()

	pkgs, err := c.Installed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Error("no installed packages parsed")
	}
	for _, p := range pkgs {
		if p.Name == "" || p.Version == "" {
			t.Errorf("incomplete package: %+v", p)
		}
	}
	if _, err := c.Outdated(ctx); err != nil {
		t.Error(err)
	}
	if _, err := c.Services(ctx); err != nil {
		t.Error(err)
	}
	if _, err := c.Taps(ctx); err != nil {
		t.Error(err)
	}
	if _, err := c.Doctor(ctx); err != nil {
		t.Error(err)
	}
}

// Runs a read-only brew command through the pseudo-terminal runner.
func TestLivePTY(t *testing.T) {
	bin, err := Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	p, err := StartPTY(bin, []string{"--version"}, JobEnv, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(p)
	code, err := p.Wait()
	if err != nil || code != 0 || !strings.Contains(string(out), "Homebrew") {
		t.Errorf("brew --version via PTY: code %d, err %v, output %q", code, err, out)
	}
}

// TestLiveCompletions reads the fish completions the installed brew ships.
func TestLiveCompletions(t *testing.T) {
	bin, err := Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	c, err := NewClient(ExecRunner{Bin: bin, Env: QueryEnv}).Completions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Commands) < 50 {
		t.Errorf("only %d commands parsed", len(c.Commands))
	}
	up, ok := c.Command("upgrade")
	if !ok || up.Desc == "" || len(up.Flags) < 10 || len(up.Args) == 0 {
		t.Errorf("upgrade parsed as %+v", up)
	}
	for _, cmd := range c.Commands {
		if cmd.Desc == "" {
			t.Errorf("command %s has no description", cmd.Name)
		}
	}
	services, _ := c.Command("services")
	if _, ok := services.Sub("start"); !ok {
		t.Error("services start is missing")
	}
	if _, ok := services.Sub("t"); ok {
		t.Error("services t is an alias of stop and should be skipped")
	}
}
