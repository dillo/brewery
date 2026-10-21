package brew

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ErrNotFound means no brew executable could be located.
var ErrNotFound = errors.New("brew not found")

// Locate finds the brew executable: $HOMEBREW_PREFIX first, then the standard
// Apple Silicon and Intel prefixes, then $PATH.
func Locate() (string, error) {
	var candidates []string
	if prefix := os.Getenv("HOMEBREW_PREFIX"); prefix != "" {
		candidates = append(candidates, filepath.Join(prefix, "bin", "brew"))
	}
	candidates = append(candidates, "/opt/homebrew/bin/brew", "/usr/local/bin/brew")
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() { // #nosec G703 -- only stats where the user says brew is
			return c, nil
		}
	}
	if p, err := exec.LookPath("brew"); err == nil {
		return p, nil
	}
	return "", ErrNotFound
}

// Runner runs brew with the given arguments and returns its output.
type Runner interface {
	Run(ctx context.Context, args ...string) (stdout, stderr []byte, err error)
}

// QueryEnv keeps read-only queries fast and their output machine-readable.
var QueryEnv = []string{
	"HOMEBREW_NO_AUTO_UPDATE=1",
	"HOMEBREW_NO_ENV_HINTS=1",
	"HOMEBREW_NO_COLOR=1",
}

// ExecRunner runs the real brew binary.
type ExecRunner struct {
	Bin string
	Env []string // appended to os.Environ()
}

// Run starts brew in its own process group so that cancelling ctx kills
// brew and everything it spawned. WaitDelay bounds the wait for a child that
// escaped the kill but still holds the output pipes.
func (r ExecRunner) Run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, r.Bin, args...) // #nosec G204 -- Bin is the brew that Locate found
	cmd.Env = append(os.Environ(), r.Env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	return out.Bytes(), errb.Bytes(), err
}

// QueryError reports a failed brew query with enough detail to show the user.
type QueryError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *QueryError) Error() string {
	msg := fmt.Sprintf("brew %s: %v", strings.Join(e.Args, " "), e.Err)
	if s := strings.TrimSpace(e.Stderr); s != "" {
		msg += "\n" + s
	}
	return msg
}

func (e *QueryError) Unwrap() error { return e.Err }

// Client runs read-only brew queries.
type Client struct {
	r       Runner
	Timeout time.Duration
	// VulnsTimeout is Vulns' own: a scan of a large install, or a slow
	// OSV.dev, takes longer than the other queries are given.
	VulnsTimeout time.Duration
	Prefix       string // where brew is installed (/opt/homebrew): Cellar and Caskroom are in it

	AgentsDir string // where brew services keeps its launchd files

	dirsMu      sync.Mutex
	cache, repo string // brew --cache and --repository, once asked

	measured func(Package) // tests: called after Sizes measures each package
}

func NewClient(r Runner) *Client {
	c := &Client{r: r, Timeout: 30 * time.Second, VulnsTimeout: 2 * time.Minute}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		c.AgentsDir = filepath.Join(home, "Library", "LaunchAgents")
	}
	return c
}

func (c *Client) query(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	out, errb, err := c.r.Run(ctx, args...)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timed out after %s", c.Timeout)
		}
		return nil, &QueryError{Args: args, Stderr: string(errb), Err: err}
	}
	return out, nil
}

func (c *Client) Installed(ctx context.Context) ([]Package, error) {
	out, err := c.query(ctx, "info", "--json=v2", "--installed")
	if err != nil {
		return nil, err
	}
	return ParseInfo(out)
}

func (c *Client) Outdated(ctx context.Context) ([]Outdated, error) {
	out, err := c.query(ctx, "outdated", "--json=v2")
	if err != nil {
		return nil, err
	}
	return ParseOutdated(out)
}

func (c *Client) Services(ctx context.Context) ([]Service, error) {
	out, err := c.query(ctx, "services", "info", "--all", "--json")
	if err != nil {
		return nil, err
	}
	return ParseServices(out)
}

func (c *Client) Taps(ctx context.Context) ([]Tap, error) {
	out, err := c.query(ctx, "tap-info", "--json", "--installed")
	if err != nil {
		return nil, err
	}
	return ParseTaps(out)
}

// Cleanup asks brew what each cleanup would free, running both dry-runs
// at once.
func (c *Client) Cleanup(ctx context.Context) (Cleanup, error) {
	type result struct {
		e   CleanupEstimate
		err error
	}
	dryRun := func(args ...string) chan result {
		ch := make(chan result, 1)
		go func() {
			ctx, cancel := context.WithTimeout(ctx, c.Timeout)
			defer cancel()
			out, errb, err := c.r.Run(ctx, args...)
			if err != nil {
				if ctx.Err() != nil {
					err = fmt.Errorf("timed out after %s", c.Timeout)
				}
				ch <- result{err: &QueryError{Args: args, Stderr: string(errb), Err: err}}
				return
			}
			// Some lines go to stderr: read both.
			ch <- result{e: ParseCleanup(string(out) + "\n" + string(errb))}
		}()
		return ch
	}
	std, all := dryRun("cleanup", "--dry-run"), dryRun("cleanup", "--dry-run", "--prune=all")
	s, a := <-std, <-all
	if s.err != nil {
		return Cleanup{}, s.err
	}
	if a.err != nil {
		return Cleanup{}, a.err
	}
	return Cleanup{Standard: s.e, All: a.e}, nil
}

// Autoremove asks brew which formulae autoremove would uninstall: those
// installed only as dependencies that nothing needs any more.
func (c *Client) Autoremove(ctx context.Context) ([]string, error) {
	out, err := c.query(ctx, "autoremove", "--dry-run")
	if err != nil {
		return nil, err
	}
	return ParseAutoremove(string(out)), nil
}

// ServiceFiles lists brew's launchd files in AgentsDir, sorted, which
// brew services cleanup removes when their service isn't loaded. A
// missing directory has none.
func (c *Client) ServiceFiles(context.Context) ([]string, error) {
	entries, err := os.ReadDir(c.AgentsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() {
			continue
		}
		if (strings.HasPrefix(n, "homebrew.") || strings.HasPrefix(n, "sh.brew.")) && strings.HasSuffix(n, ".plist") {
			files = append(files, filepath.Join(c.AgentsDir, n))
		}
	}
	return files, nil
}

// Doctor runs `brew doctor`. It exits 1 whenever it finds warnings, so a
// non-zero exit that produced at least one warning is a normal result. A
// timeout, a failure to run brew, or a non-zero exit with no warnings parsed
// (a crash, or only an `Error:` on stderr) is reported as an error rather
// than as a clean result.
func (c *Client) Doctor(ctx context.Context) ([]DoctorWarning, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	out, errb, err := c.r.Run(ctx, "doctor")
	if err != nil && ctx.Err() != nil {
		return nil, &QueryError{Args: []string{"doctor"}, Stderr: string(errb), Err: fmt.Errorf("timed out after %s", c.Timeout)}
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return nil, &QueryError{Args: []string{"doctor"}, Stderr: string(errb), Err: err}
	}
	warnings := ParseDoctor(string(out) + "\n" + string(errb))
	if err != nil && len(warnings) == 0 {
		return nil, &QueryError{Args: []string{"doctor"}, Stderr: string(errb), Err: err}
	}
	return warnings, nil
}

// dataFiles are files brew rewrites when it fetches new package data, by
// path under its cache or its repository. Which exist depends on brew's
// version and whether it reads the API or a tapped homebrew/core.
var dataFiles = []struct {
	inCache bool
	path    string
}{
	{true, "api/formula_names.txt"},
	{true, "api/formula.jws.json"},
	{false, ".git/FETCH_HEAD"},
}

// DataUpdated is when brew last fetched package data (brew update, or its
// auto-update): the newest of its data files.
func (c *Client) DataUpdated(ctx context.Context) (time.Time, error) {
	cache, repo, err := c.dataDirs(ctx)
	if err != nil {
		return time.Time{}, err
	}
	var newest time.Time
	for _, f := range dataFiles {
		dir := repo
		if f.inCache {
			dir = cache
		}
		if info, err := os.Stat(filepath.Join(dir, f.path)); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	if newest.IsZero() {
		return time.Time{}, errors.New("found none of brew's data files")
	}
	return newest, nil
}

// dataDirs are brew's cache and repository. They don't move while brewery
// runs, so brew is asked once; a failure is asked again next time.
func (c *Client) dataDirs(ctx context.Context) (cache, repo string, err error) {
	c.dirsMu.Lock()
	defer c.dirsMu.Unlock()
	if c.cache != "" && c.repo != "" {
		return c.cache, c.repo, nil
	}
	cb, err := c.query(ctx, "--cache")
	if err != nil {
		return "", "", err
	}
	rb, err := c.query(ctx, "--repository")
	if err != nil {
		return "", "", err
	}
	c.cache, c.repo = strings.TrimSpace(string(cb)), strings.TrimSpace(string(rb))
	return c.cache, c.repo, nil
}
