// Command brewery is a terminal dashboard for Homebrew.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/dillo/brewery/internal/analytics"
	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/jobs"
	"github.com/dillo/brewery/internal/state"
	"github.com/dillo/brewery/internal/ui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// wantIcons reports whether to draw Nerd Font icons: yes, unless
// --no-icons was given or BREWERY_NO_ICONS is set to anything.
func wantIcons(noIcons bool, env string) bool { return !noIcons && env == "" }

// locateBrew and brewVersion are swapped out in tests.
var (
	locateBrew  = brew.Locate
	brewVersion = readBrewVersion
)

// runProgram runs the TUI until it quits; tests swap it for one that needs
// no terminal.
var runProgram = func(m tea.Model) error {
	_, err := tea.NewProgram(m).Run()
	return err
}

// readBrewVersion asks the brew at bin for its version. It takes about
// 50ms; a brew that hangs is given 5 seconds.
func readBrewVersion(bin string) (brew.Version, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return brew.NewClient(brew.ExecRunner{Bin: bin, Env: brew.QueryEnv}).BrewVersion(ctx)
}

// versionProblem reads the version of the brew at bin, and says why it is
// too old for brewery, or returns "". A version brewery can't read doesn't
// stop it, and comes back empty: anything really broken shows in the tabs.
func versionProblem(bin string) (brew.Version, string) {
	v, err := brewVersion(bin)
	if err != nil {
		return brew.Version{}, ""
	}
	if !v.TooOld() {
		return v, ""
	}
	return v, fmt.Sprintf("brewery needs Homebrew %d.%d or newer (you have %s). Update it with `brew update`.", brew.MinMajor, brew.MinMinor, v.Text)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("brewery", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	noIcons := fs.Bool("no-icons", false, "draw plain text instead of Nerd Font icons (or set BREWERY_NO_ICONS to any value)")
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
	v, msg := versionProblem(bin)
	if msg != "" {
		fmt.Fprintln(stderr, msg)
		return 1
	}

	cachePath := state.DefaultCachePath()
	snap, _ := state.Load(cachePath) // no cache yet is fine: the first refresh fills it
	client := brew.NewClient(brew.ExecRunner{Bin: bin, Env: brew.QueryEnv})
	client.Prefix = brew.PrefixOf(bin)
	start := func(args, env []string, cols, rows int) (ui.Proc, error) {
		return brew.StartPTY(bin, args, env, cols, rows)
	}

	app := ui.New(client, start, snap, cachePath)
	hist := jobs.History{Dir: jobs.DefaultHistoryDir()}
	recs, _ := hist.Load() // no history yet is fine
	app.UseHistory(hist, recs)
	app.UseBrewVersion(v.Text)
	app.UseIcons(wantIcons(*noIcons, os.Getenv("BREWERY_NO_ICONS")))
	app.UseCompletions(client.Completions)
	app.UseCatalog(client)
	// Discover's analytics are kept beside the snapshot.
	app.UseAnalytics(analytics.NewStore(filepath.Join(filepath.Dir(cachePath), "analytics"), version))
	// Full-screen commands get the terminal's stderr: while the TUI runs,
	// os.Stderr and file descriptor 2 are the crash log. terminal is set
	// once the crash log starts, before the program runs any command.
	var terminal *os.File
	fullScreen := func(cmd *exec.Cmd) *exec.Cmd {
		if terminal != nil {
			cmd.Stderr = terminal
		} else {
			cmd.Stderr = os.Stderr
		}
		return cmd
	}
	app.UseTakeover(func(args []string) *exec.Cmd { return fullScreen(exec.Command(bin, args...)) })
	// less +F follows the log as it grows; ctrl+c stops following, q returns.
	app.UsePager(func(files []string) *exec.Cmd {
		return fullScreen(exec.Command("less", append([]string{"+F", "--"}, files...)...))
	})

	app.UseOpener(func(url string) error { return exec.Command("open", url).Run() }) // macOS's open
	app.UseClipboard(clipboard.WriteAll)

	crash, crashErr := startCrashLog(jobs.DefaultStateDir())
	if crashErr == nil {
		terminal = crash.terminal
	}
	err = runProgram(app)
	if crashErr == nil {
		if path := crash.stop(time.Now()); path != "" {
			fmt.Fprintf(stderr, "brewery crashed. What happened is in %s; please include it when reporting this.\n", path)
			return 1
		}
		for _, path := range crash.earlier {
			fmt.Fprintf(stderr, "brewery crashed last time. What happened is in %s; please include it when reporting this.\n", path)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
		return 1
	}
	return 0
}
