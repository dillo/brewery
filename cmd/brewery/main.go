// Command brewery is a terminal dashboard for Homebrew.
package main

import (
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

	"brewery/internal/analytics"
	"brewery/internal/brew"
	"brewery/internal/jobs"
	"brewery/internal/state"
	"brewery/internal/ui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// wantIcons reports whether to draw Nerd Font icons: yes, unless
// --no-icons was given or BREWERY_NO_ICONS is set to anything.
func wantIcons(noIcons bool, env string) bool { return !noIcons && env == "" }

// locateBrew is swapped out in tests.
var locateBrew = brew.Locate

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
	_, err = tea.NewProgram(app).Run()
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
