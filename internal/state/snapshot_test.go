package state

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/github"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "snapshot.json")
	in := Snapshot{
		Packages:      testPkgs,
		Outdated:      []brew.Outdated{{Name: "docker-desktop", Kind: brew.KindCask, Installed: "4.39.0", Current: "4.95.0"}},
		Taps:          []brew.Tap{{Name: "anomalyco/tap", Formulae: 2}},
		DoctorRan:     true,
		TapChecks:     []github.TapCheck{{Tap: "o/t", State: github.Moved, MovedTo: "n/t"}},
		TapsCheckedAt: time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC),
		FetchedAt:     time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC),
	}
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	in.Version = snapshotVersion
	if !reflect.DeepEqual(out, in) {
		t.Errorf("round trip changed snapshot:\n got %+v\nwant %+v", out, in)
	}
}

func TestLoadMissingFile(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil || len(s.Packages) != 0 {
		t.Errorf("want empty snapshot and error, got %+v, %v", s, err)
	}
}

func TestLoadIgnoresOtherVersions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := os.WriteFile(path, []byte(`{"version": 999, "packages": [{"name": "fd"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if s, err := Load(path); err == nil || len(s.Packages) != 0 {
		t.Errorf("want version mismatch error, got %+v, %v", s, err)
	}
}

func TestDefaultCachePathHonorsXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg")
	if got := DefaultCachePath(); got != "/tmp/xdg/brewery/snapshot.json" {
		t.Errorf("got %s", got)
	}
}

func TestDefaultCachePathWithoutHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	got := DefaultCachePath()
	if !filepath.IsAbs(got) || !strings.HasPrefix(got, os.TempDir()) {
		t.Errorf("without a home directory the cache goes in the temp directory, got %s", got)
	}
}

func TestSaveCleansUpWhenItCantReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	// A non-empty directory where the file should go: the rename fails.
	if err := os.MkdirAll(filepath.Join(path, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, Snapshot{}); err == nil {
		t.Fatal("Save should fail")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "snapshot-*.json")); len(left) != 0 {
		t.Errorf("temp file left behind: %v", left)
	}
}

func TestDefaultCachePathIgnoresARelativeXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "relative/cache")
	if got := DefaultCachePath(); !filepath.IsAbs(got) {
		t.Errorf("a relative XDG_CACHE_HOME is ignored, got %s", got)
	}
}

func TestLoadCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := os.WriteFile(path, []byte(`{"version": 3, "packages": [`), 0o644); err != nil {
		t.Fatal(err)
	}
	if s, err := Load(path); err == nil || !strings.Contains(err.Error(), "read snapshot cache") || len(s.Packages) != 0 {
		t.Errorf("want an empty snapshot and a read error, got %+v, %v", s, err)
	}
}

func TestSaveFails(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	readOnly := t.TempDir()
	if err := os.Chmod(readOnly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) })
	root := os.Geteuid() == 0 // root writes to a read-only directory
	good := filepath.Join(t.TempDir(), "snapshot.json")
	for _, tt := range []struct {
		name, path string
		snap       Snapshot
	}{
		{"its dir is under a file", filepath.Join(file, "brewery", "snapshot.json"), Snapshot{}},
		{"its dir is read-only", filepath.Join(readOnly, "snapshot.json"), Snapshot{}},
		// JSON times stop at year 9999.
		{"it can't be encoded", good, Snapshot{FetchedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}},
	} {
		if root && tt.path == filepath.Join(readOnly, "snapshot.json") {
			t.Log("skipping the read-only case: running as root")
			continue
		}
		if err := Save(tt.path, tt.snap); err == nil {
			t.Errorf("%s: Save should fail", tt.name)
		}
		if _, err := os.Stat(tt.path); err == nil {
			t.Errorf("%s: nothing should be written", tt.name)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(good), "*")); len(left) != 0 {
		t.Errorf("files left behind: %v", left)
	}
}

func TestSaveIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "snapshot.json")
	if err := Save(path, Snapshot{}); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Dir(path)); err != nil || fi.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, %v; want 0700", fi.Mode().Perm(), err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, %v; want 0600", fi.Mode().Perm(), err)
	}
}
