//go:build live

package brew

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// Runs the real brew, read-only, to catch output-format drift.
// Run with: go test -tags live ./internal/brew/
func TestLiveQueries(t *testing.T) {
	bin, err := Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	c := NewClient(ExecRunner{Bin: bin, Env: QueryEnv})
	c.Prefix = PrefixOf(bin)
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
	sizes, err := c.Sizes(ctx, pkgs)
	if err != nil {
		t.Error(err)
	}
	measured := 0
	for _, n := range sizes {
		if n > 0 {
			measured++
		}
	}
	if measured < len(pkgs)/2 {
		t.Errorf("only %d of %d packages have a size", measured, len(pkgs))
	}
	if at, err := c.DataUpdated(ctx); err != nil || at.After(time.Now()) {
		t.Errorf("brew's data updated %v, %v", at, err)
	}
	if _, err := c.Outdated(ctx); err != nil {
		t.Error(err)
	}
	services, err := c.Services(ctx)
	if err != nil {
		t.Error(err)
	}
	for _, s := range services {
		if s.Name == "" || s.Status == "" {
			t.Errorf("incomplete service: %+v", s)
		}
	}
	if _, err := c.Taps(ctx); err != nil {
		t.Error(err)
	}
	if _, err := c.Doctor(ctx); err != nil {
		t.Error(err)
	}
	if _, err := c.Cleanup(ctx); err != nil {
		t.Error(err)
	}
}

// Reads brew's catalogue and asks about packages that may not be installed.
func TestLiveCatalogue(t *testing.T) {
	bin, err := Locate()
	if err != nil {
		t.Skip("brew not installed")
	}
	c := NewClient(ExecRunner{Bin: bin, Env: QueryEnv})
	ctx := context.Background()
	es, err := c.Catalogue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	casks := 0
	for _, e := range es {
		if e.Kind == KindCask {
			casks++
			if strings.HasPrefix(e.Desc, "(") {
				t.Errorf("display names left in %+v", e)
			}
		}
	}
	if len(es) < 5000 || casks < 2000 {
		t.Errorf("only %d entries, %d casks", len(es), casks)
	}
	if _, err := c.NewSinceUpdate(ctx); err != nil {
		t.Error(err)
	}
	wget, err := c.Details(ctx, "wget", KindFormula)
	if err != nil || wget.Latest == "" || wget.License == "" || len(wget.Needs) == 0 {
		t.Errorf("wget: %+v, %v", wget, err)
	}
	firefox, err := c.Details(ctx, "firefox", KindCask)
	if err != nil || firefox.Kind != KindCask || firefox.Latest == "" {
		t.Errorf("firefox: %+v, %v", firefox, err)
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
