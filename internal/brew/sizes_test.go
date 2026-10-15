package brew

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSizes(t *testing.T) {
	prefix, apps := t.TempDir(), t.TempDir()
	write(t, filepath.Join(prefix, "Cellar", "fd", "10.5.0", "bin", "fd"), 1000)
	write(t, filepath.Join(prefix, "Cellar", "fd", "10.4.0", "bin", "fd"), 500) // an old keg counts too
	write(t, filepath.Join(prefix, "Caskroom", "docker-desktop", "4.39.0", ".metadata"), 10)
	write(t, filepath.Join(apps, "Docker.app", "Contents", "MacOS", "Docker"), 4000)
	if err := os.Symlink("/etc/hosts", filepath.Join(apps, "Docker.app", "link")); err != nil {
		t.Fatal(err)
	}
	c := NewClient(fakeRunner{})
	c.Prefix = prefix
	pkgs := []Package{
		{Name: "fd", FullName: "fd", Kind: KindFormula},
		{Name: "docker-desktop", FullName: "docker-desktop", Kind: KindCask, Targets: []string{filepath.Join(apps, "Docker.app")}},
		{Name: "gone", FullName: "user/tap/gone", Kind: KindFormula},
	}
	got, err := c.Sizes(context.Background(), pkgs)
	if err != nil {
		t.Fatal(err)
	}
	link, _ := os.Lstat(filepath.Join(apps, "Docker.app", "link"))
	want := map[string]int64{
		"formula/fd":            1500,
		"cask/docker-desktop":   4010 + link.Size(),
		"formula/user/tap/gone": 0,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %d, want %d", k, got[k], w)
		}
	}
}

func TestSizesStopsWhenCancelled(t *testing.T) {
	prefix := t.TempDir()
	write(t, filepath.Join(prefix, "Cellar", "fd", "1", "f"), 1)
	c := NewClient(fakeRunner{})
	c.Prefix, c.Timeout = prefix, time.Minute
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Sizes(ctx, []Package{{Name: "fd", FullName: "fd"}}); err == nil {
		t.Error("a cancelled measure should fail")
	}
}

func TestPrefixOf(t *testing.T) {
	if got := PrefixOf("/opt/homebrew/bin/brew"); got != "/opt/homebrew" {
		t.Errorf("got %s", got)
	}
}
