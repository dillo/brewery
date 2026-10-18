package brew

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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
	if _, ok := errors.AsType[*QueryError](err); !ok {
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
	if _, ok := errors.AsType[*QueryError](err); !ok {
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

func TestClientDataUpdated(t *testing.T) {
	cache, repo := t.TempDir(), t.TempDir()
	older, newer := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	for path, at := range map[string]time.Time{
		filepath.Join(cache, "api", "formula_names.txt"): newer,
		filepath.Join(repo, ".git", "FETCH_HEAD"):        older,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	c := NewClient(fakeRunner{out: map[string][]byte{"--cache": []byte(cache + "\n"), "--repository": []byte(repo + "\n")}})
	got, err := c.DataUpdated(context.Background())
	if err != nil || !got.Equal(newer) {
		t.Errorf("got %v, %v; want the newest file's time %v", got, err, newer)
	}

	c = NewClient(fakeRunner{out: map[string][]byte{"--cache": []byte(t.TempDir()), "--repository": []byte(t.TempDir())}})
	if _, err := c.DataUpdated(context.Background()); err == nil {
		t.Error("with none of the files, it's an error")
	}
}

// countingRunner counts each command it runs.
type countingRunner struct {
	fakeRunner
	mu sync.Mutex
	n  map[string]int
}

func (c *countingRunner) Run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	c.mu.Lock()
	c.n[strings.Join(args, " ")]++
	c.mu.Unlock()
	return c.fakeRunner.Run(ctx, args...)
}

func TestClientDataUpdatedAsksForDirsOnce(t *testing.T) {
	cache := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cache, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "api", "formula_names.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r := &countingRunner{n: map[string]int{}, fakeRunner: fakeRunner{
		out: map[string][]byte{"--cache": []byte(cache + "\n"), "--repository": []byte(t.TempDir() + "\n")},
		err: map[string]error{"--repository": errors.New("exit status 1")},
	}}
	c := NewClient(r)
	if _, err := c.DataUpdated(context.Background()); err == nil {
		t.Fatal("brew --repository failed: want an error")
	}
	delete(r.err, "--repository")
	for range 3 {
		if _, err := c.DataUpdated(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if r.n["--cache"] != 2 || r.n["--repository"] != 2 {
		t.Errorf("ran %v: a failure asks again, then the dirs are kept", r.n)
	}
}

func TestClientAutoremove(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{
		"autoremove --dry-run": []byte("==> Would autoremove 2 unneeded formulae:\nlibyaml\nm4\n"),
	}})
	got, err := c.Autoremove(context.Background())
	if err != nil || !slices.Equal(got, []string{"libyaml", "m4"}) {
		t.Errorf("got %q, %v", got, err)
	}
	c = NewClient(fakeRunner{err: map[string]error{"autoremove --dry-run": errors.New("exit status 1")}})
	if _, err := c.Autoremove(context.Background()); err == nil || !strings.Contains(err.Error(), "brew autoremove --dry-run") {
		t.Errorf("a failed dry run is an error naming it: %v", err)
	}
}

func TestClientServiceFiles(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"homebrew.mxcl.redis.plist", "sh.brew.thing.plist", "com.apple.other.plist", "homebrew.notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "homebrew.mxcl.dir.plist"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := NewClient(fakeRunner{})
	c.AgentsDir = dir
	got, err := c.ServiceFiles(context.Background())
	want := []string{filepath.Join(dir, "homebrew.mxcl.redis.plist"), filepath.Join(dir, "sh.brew.thing.plist")}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("got %q, %v; want %q", got, err, want)
	}

	c.AgentsDir = filepath.Join(dir, "missing")
	if got, err := c.ServiceFiles(context.Background()); err != nil || len(got) != 0 {
		t.Errorf("a missing directory has no files: %q, %v", got, err)
	}

	if os.Geteuid() == 0 {
		t.Skip("root reads any directory")
	}
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	c.AgentsDir = locked
	if _, err := c.ServiceFiles(context.Background()); err == nil {
		t.Error("an unreadable directory is an error")
	}
}

func TestNewClientFindsLaunchAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got, want := NewClient(fakeRunner{}).AgentsDir, filepath.Join(home, "Library", "LaunchAgents"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
