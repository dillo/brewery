package brew

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// fakeRunner returns canned output keyed by the joined argument list.
type fakeRunner struct {
	out   map[string][]byte
	err   map[string]error
	delay time.Duration
}

func (f fakeRunner) Run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
	key := strings.Join(args, " ")
	if err := f.err[key]; err != nil {
		return f.out[key], []byte("Error: something broke"), err
	}
	return f.out[key], nil, nil
}

func TestClientInstalled(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{"info --json=v2 --installed": fixture(t, "info.json")}})
	pkgs, err := c.Installed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 7 {
		t.Errorf("got %d packages", len(pkgs))
	}
}

func TestClientCleanup(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{
		"cleanup --dry-run":             []byte("Would remove: /x (3.9KB)\n==> This operation would free approximately 3.9KB of disk space.\n"),
		"cleanup --dry-run --prune=all": fixture(t, "cleanup.txt"),
	}})
	got, err := c.Cleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := Cleanup{Standard: CleanupEstimate{1, "3.9KB"}, All: CleanupEstimate{4, "86.7MB"}}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	c = NewClient(fakeRunner{err: map[string]error{"cleanup --dry-run --prune=all": errors.New("exit status 1")}})
	if _, err := c.Cleanup(context.Background()); err == nil || !strings.Contains(err.Error(), "brew cleanup --dry-run --prune=all") {
		t.Errorf("a failed dry-run is an error naming it: %v", err)
	}
	c = NewClient(fakeRunner{err: map[string]error{"cleanup --dry-run": errors.New("exit status 1")}})
	if _, err := c.Cleanup(context.Background()); err == nil || !strings.Contains(err.Error(), "brew cleanup --dry-run: exit status 1") {
		t.Errorf("a failed standard dry-run is an error naming it: %v", err)
	}
}

func TestClientQueryErrorIncludesStderr(t *testing.T) {
	c := NewClient(fakeRunner{err: map[string]error{"services info --all --json": errors.New("exit status 1")}})
	_, err := c.Services(context.Background())
	var qe *QueryError
	if !errors.As(err, &qe) {
		t.Fatalf("want *QueryError, got %v", err)
	}
	if !strings.Contains(err.Error(), "brew services info --all --json") || !strings.Contains(err.Error(), "something broke") {
		t.Errorf("error message missing details: %q", err)
	}
}

func TestClientTimeout(t *testing.T) {
	c := NewClient(fakeRunner{delay: time.Second})
	c.Timeout = 10 * time.Millisecond
	_, err := c.Taps(context.Background())
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("want timeout error, got %v", err)
	}
}

func TestClientDoctorExitOneIsNotAnError(t *testing.T) {
	exitErr := exec.Command("false").Run() // a real *exec.ExitError with status 1
	c := NewClient(fakeRunner{
		out: map[string][]byte{"doctor": fixture(t, "doctor.txt")},
		err: map[string]error{"doctor": exitErr},
	})
	ws, err := c.Doctor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 {
		t.Errorf("got %d warnings, want 2", len(ws))
	}
}

func TestClientDoctorFailureWithoutWarningsIsAnError(t *testing.T) {
	exitErr := exec.Command("false").Run() // a real *exec.ExitError with status 1
	c := NewClient(fakeRunner{err: map[string]error{"doctor": exitErr}})
	_, err := c.Doctor(context.Background())
	var qe *QueryError
	if !errors.As(err, &qe) {
		t.Fatalf("want *QueryError, got %v", err)
	}
	if !strings.Contains(err.Error(), "something broke") {
		t.Errorf("error message missing stderr: %q", err)
	}
}

func TestClientDoctorTimeout(t *testing.T) {
	c := NewClient(fakeRunner{delay: time.Second})
	c.Timeout = 10 * time.Millisecond
	_, err := c.Doctor(context.Background())
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("want timeout error, got %v", err)
	}
}
