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
	"github.com/dillo/brewery/internal/github"
	"github.com/dillo/brewery/internal/jobs"
	"github.com/dillo/brewery/internal/state"
	"github.com/dillo/brewery/internal/theme"
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

// run starts brewery and returns its exit code. Each step either hands the
// next one what it needs or says why brewery stops.
func run(args []string, stdout, stderr io.Writer) int {
	opts, code, ok := parseFlags(args, stdout, stderr)
	if !ok {
		return code
	}
	bin, v, ok := findBrew(stderr)
	if !ok {
		return 1
	}
	screen := &fullScreen{}
	app := newApp(bin, v, opts)
	useCommands(app, bin, screen)
	return runApp(app, screen, stderr)
}

// options are what the command line asks for.
type options struct {
	noIcons bool
}

// parseFlags reads args. When brewery should stop here, for --version or a
// bad flag, ok is false and code is the exit code.
func parseFlags(args []string, stdout, stderr io.Writer) (opts options, code int, ok bool) {
	fs := flag.NewFlagSet("brewery", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	noIcons := fs.Bool("no-icons", false, "draw plain text instead of Nerd Font icons (or set BREWERY_NO_ICONS to any value)")
	if err := fs.Parse(args); err != nil {
		return options{}, 2, false
	}
	if *showVersion {
		fmt.Fprintln(stdout, "brewery", version)
		return options{}, 0, false
	}
	return options{noIcons: *noIcons}, 0, true
}

// findBrew finds the brew to drive and reads its version. When there is
// none, or it is too old, it says so on stderr and ok is false.
func findBrew(stderr io.Writer) (bin string, v brew.Version, ok bool) {
	bin, err := locateBrew()
	if errors.Is(err, brew.ErrNotFound) {
		fmt.Fprintln(stderr, "brewery: Homebrew isn't installed (looked in $HOMEBREW_PREFIX, /opt/homebrew, /usr/local and $PATH).")
		fmt.Fprintln(stderr, "Install it from https://brew.sh, then run brewery again.")
		return "", brew.Version{}, false
	}
	v, msg := versionProblem(bin)
	if msg != "" {
		fmt.Fprintln(stderr, msg)
		return "", brew.Version{}, false
	}
	return bin, v, true
}

// newApp builds the TUI around the brew at bin, with what earlier runs
// left behind: the snapshot cache and the job history.
func newApp(bin string, v brew.Version, opts options) *ui.App {
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
	app.UseIcons(wantIcons(opts.noIcons, os.Getenv("BREWERY_NO_ICONS")))
	// The theme the user last kept; Macchiato when there's none.
	stateDir := jobs.DefaultStateDir()
	app.UseThemeDir(stateDir)
	app.UseTheme(theme.LoadTheme(stateDir))
	app.UseCompletions(client.Completions)
	app.UseCatalog(client)
	// Discover's analytics are kept beside the snapshot.
	app.UseAnalytics(analytics.NewStore(filepath.Join(filepath.Dir(cachePath), "analytics"), version))
	app.UseTapChecker(github.NewClient()) // only used when the user runs Check taps
	return app
}

// fullScreen gives full-screen commands the terminal's stderr: while the
// TUI runs, os.Stderr and file descriptor 2 are the crash log. terminal is
// set once the crash log starts, before the program runs any command.
type fullScreen struct {
	terminal *os.File
}

func (s *fullScreen) command(cmd *exec.Cmd) *exec.Cmd {
	if s.terminal != nil {
		cmd.Stderr = s.terminal
	} else {
		cmd.Stderr = os.Stderr
	}
	return cmd
}

// useCommands hands the app the programs it runs outside the TUI.
func useCommands(app *ui.App, bin string, screen *fullScreen) {
	app.UseTakeover(func(args []string) *exec.Cmd { return screen.command(exec.Command(bin, args...)) }) // #nosec G204 -- bin is the brew that Locate found
	// less +F follows the log as it grows; ctrl+c stops following, q returns.
	app.UsePager(func(files []string) *exec.Cmd {
		return screen.command(exec.Command("less", append([]string{"+F", "--"}, files...)...)) // #nosec G204 -- files follow --
	})

	app.UseOpener(func(url string) error { return exec.Command("open", url).Run() }) // #nosec G204 -- macOS's open; only ever given an http(s) URL
	app.UseClipboard(clipboard.WriteAll)
}

// runApp runs the TUI with stderr going to a crash log, then reports any
// crash, this run's or an earlier one's, and any error the program gave.
func runApp(app *ui.App, screen *fullScreen, stderr io.Writer) int {
	crash, crashErr := startCrashLog(jobs.DefaultStateDir())
	if crashErr == nil {
		screen.terminal = crash.terminal
	}
	err := runProgram(app)
	if crashErr == nil && reportCrashes(crash, stderr) {
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, "brewery:", err)
		return 1
	}
	return 0
}

// reportCrashes stops the crash log and points at what it kept. It reports
// whether this run crashed; if not, it mentions crashes from earlier runs.
func reportCrashes(crash *crashLog, stderr io.Writer) bool {
	if path := crash.stop(time.Now()); path != "" {
		fmt.Fprintf(stderr, "brewery crashed. What happened is in %s; please include it when reporting this.\n", path)
		return true
	}
	for _, path := range crash.earlier {
		fmt.Fprintf(stderr, "brewery crashed last time. What happened is in %s; please include it when reporting this.\n", path)
	}
	return false
}
