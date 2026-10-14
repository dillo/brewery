package state

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"brewery/internal/brew"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "snapshot.json")
	in := Snapshot{
		Packages:  testPkgs,
		Outdated:  []brew.Outdated{{Name: "docker-desktop", Kind: brew.KindCask, Installed: "4.39.0", Current: "4.95.0"}},
		Taps:      []brew.Tap{{Name: "anomalyco/tap", Formulae: 2}},
		DoctorRan: true,
		FetchedAt: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC),
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
	os.WriteFile(path, []byte(`{"version": 999, "packages": [{"name": "fd"}]}`), 0o644)
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
