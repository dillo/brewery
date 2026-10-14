package brew

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
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
	cmd := exec.CommandContext(ctx, r.Bin, args...)
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
}

func NewClient(r Runner) *Client {
	return &Client{r: r, Timeout: 30 * time.Second}
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
