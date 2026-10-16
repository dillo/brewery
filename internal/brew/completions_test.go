package brew

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func parseFixtureCompletions(t *testing.T) Completions {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "brew.fish"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c, err := ParseFish(f)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestParseFishCommands(t *testing.T) {
	c := parseFixtureCompletions(t)
	var names []string
	for _, cmd := range c.Commands {
		names = append(names, cmd.Name)
	}
	want := []string{"--cache", "bundle", "edit", "info", "pin", "services", "uninstall", "untap", "upgrade"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("commands %v, want %v", names, want)
	}
	cache, _ := c.Command("--cache")
	if cache.Desc != "Display Homebrew's download cache" {
		t.Errorf("escaped quote not undone: %q", cache.Desc)
	}
	services, _ := c.Command("services")
	if !strings.HasPrefix(services.Desc, "Manage background services") {
		t.Errorf("the first description should win: %q", services.Desc)
	}
	if _, ok := c.Command("release"); ok {
		t.Error("release has options but no __fish_brew_complete_cmd line, so it isn't a command")
	}
}

func TestParseFishFlagsAndArgs(t *testing.T) {
	c := parseFixtureCompletions(t)
	up, _ := c.Command("upgrade")
	flags := map[string]string{}
	for _, f := range up.Flags {
		if _, dup := flags[f.Name]; dup {
			t.Errorf("flag %s listed twice", f.Name)
		}
		flags[f.Name] = f.Desc
	}
	if flags["--dry-run"] != "Show what would be upgraded, but do not actually upgrade anything" {
		t.Errorf("--dry-run: %q", flags["--dry-run"])
	}
	for _, name := range []string{"--cask", "--formula", "--greedy", "--verbose"} {
		if _, ok := flags[name]; !ok {
			t.Errorf("upgrade is missing %s", name)
		}
	}
	for _, tt := range []struct {
		cmd  string
		want []string
	}{
		{"upgrade", []string{"formulae_installed", "casks_installed"}},
		{"uninstall", []string{"formulae_installed", "casks_installed"}},
		{"untap", []string{"taps_installed"}},
		{"info", []string{"formulae_all", "casks_all"}},
	} {
		cmd, _ := c.Command(tt.cmd)
		if !reflect.DeepEqual(cmd.Args, tt.want) {
			t.Errorf("%s args %v, want %v", tt.cmd, cmd.Args, tt.want)
		}
	}
}

func TestParseFishSubcommands(t *testing.T) {
	c := parseFixtureCompletions(t)
	services, _ := c.Command("services")
	var names []string
	for _, s := range services.Subs {
		names = append(names, s.Name)
	}
	if want := []string{"list", "run", "start", "stop"}; !reflect.DeepEqual(names, want) {
		t.Errorf("subcommands %v, want %v", names, want)
	}
	start, ok := services.Sub("start")
	if !ok || start.Desc != "Start the service formula immediately and register it to launch at login (or boot)" {
		t.Errorf("start: %+v", start)
	}
	if !reflect.DeepEqual(start.Args, []string{"services"}) {
		t.Errorf("start args %v", start.Args)
	}
	var all bool
	for _, f := range start.Flags {
		all = all || f.Name == "--all"
	}
	if !all {
		t.Errorf("start should have --all: %+v", start.Flags)
	}
	for _, alias := range []string{"unload", "t"} {
		if _, ok := services.Sub(alias); ok {
			t.Errorf("%s is an alias of stop and should be skipped", alias)
		}
	}
	stop, _ := services.Sub("stop")
	if len(stop.Flags) == 0 {
		t.Errorf("stop lost its flags: %+v", stop)
	}
	if _, ok := services.Sub("launch"); ok {
		t.Error("an alias without its own sub_cmd line should not become a subcommand")
	}

	// brew describes bundle upgrade exactly like bundle install, so it folds
	// into install and is not offered. This is the accepted trade-off
	// documented on ParseFish; typing "bundle upgrade" still runs.
	bundle, _ := c.Command("bundle")
	if _, ok := bundle.Sub("install"); !ok {
		t.Errorf("bundle should have install: %+v", bundle.Subs)
	}
	if _, ok := bundle.Sub("upgrade"); ok {
		t.Error("bundle upgrade shares install's description and should be folded into it")
	}
}

func TestFishTokens(t *testing.T) {
	for _, tt := range []struct {
		line string
		want []string
	}{
		{`__fish_brew_complete_cmd '--cache' 'Display Homebrew\'s download cache'`,
			[]string{"__fish_brew_complete_cmd", "--cache", "Display Homebrew's download cache"}},
		{`__fish_brew_complete_cmd 'services' "Integrates macOS's \"launchctl\""`,
			[]string{"__fish_brew_complete_cmd", "services", `Integrates macOS's "launchctl"`}},
		{`__fish_brew_complete_arg 'pin' -l debug -d 'x\\y'`,
			[]string{"__fish_brew_complete_arg", "pin", "-l", "debug", "-d", `x\y`}},
	} {
		if got := fishTokens(tt.line); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("fishTokens(%s) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestClientCompletionsReadsBrewsFishFile(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "completions", "fish")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join("testdata", "brew.fish"))
	if err := os.WriteFile(filepath.Join(dir, "brew.fish"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewClient(fakeRunner{out: map[string][]byte{"--repository": []byte(repo + "\n")}})
	got, err := c.Completions(context.Background())
	if err != nil || len(got.Commands) != 9 {
		t.Fatalf("got %d commands, err %v", len(got.Commands), err)
	}

	empty := NewClient(fakeRunner{out: map[string][]byte{"--repository": []byte(t.TempDir())}})
	if _, err := empty.Completions(context.Background()); err == nil {
		t.Error("a missing completion file should be an error")
	}
}
