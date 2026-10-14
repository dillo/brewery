// Command brewery is a terminal dashboard for Homebrew.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/jobs"
	"brewery/internal/state"
	"brewery/internal/ui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// locateBrew is swapped out in tests.
var locateBrew = brew.Locate

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("brewery", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, "brewery", version)
		return 0
	}

	bin, err := locateBrew()
	if errors.Is(err, brew.ErrNotFound) {
		fmt.Fprintln(stderr, "brewery: Homebrew isn't installed (looked in $HOMEBREW_PREFIX, /opt/homebrew, /usr/local and $PATH).")
		fmt.Fprintln(stderr, "Install it from https://brew.sh, then run brewery again.")
		return 1
	}

	cachePath := state.DefaultCachePath()
	snap, _ := state.Load(cachePath) // no cache yet is fine: the first refresh fills it
	client := brew.NewClient(brew.ExecRunner{Bin: bin, Env: brew.QueryEnv})
	start := func(args, env []string, cols, rows int) (ui.Proc, error) {
		return brew.StartPTY(bin, args, env, cols, rows)
	}

	app := ui.New(client, start, snap, cachePath)
	hist := jobs.History{Dir: jobs.DefaultHistoryDir()}
	recs, _ := hist.Load() // no history yet is fine
	app.UseHistory(hist, recs)
	app.UseCompletions(client.Completions)
	app.UseTakeover(func(args []string) *exec.Cmd { return exec.Command(bin, args...) })
	// less +F follows the log as it grows; ctrl+c stops following, q returns.
	app.UsePager(func(files []string) *exec.Cmd {
		return exec.Command("less", append([]string{"+F", "--"}, files...)...)
	})

	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
		return 1
	}
	return 0
}
