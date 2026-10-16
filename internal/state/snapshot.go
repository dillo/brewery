package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dillo/brewery/internal/brew"
)

// snapshotVersion changes whenever Snapshot's shape changes; older cache
// files are ignored rather than half-loaded.
const snapshotVersion = 3

// Snapshot is everything brewery knows about the installation at one time.
type Snapshot struct {
	Version    int                  `json:"version"`
	Packages   []brew.Package       `json:"packages"`
	Outdated   []brew.Outdated      `json:"outdated"`
	Services   []brew.Service       `json:"services"`
	Taps       []brew.Tap           `json:"taps"`
	Doctor     []brew.DoctorWarning `json:"doctor"`
	DoctorRan  bool                 `json:"doctor_ran"`
	Cleanup    brew.Cleanup         `json:"cleanup"`
	CleanupRan bool                 `json:"cleanup_ran"`
	Sizes      map[string]int64     `json:"sizes"`   // bytes on disk, by Package.Key
	DataAt     time.Time            `json:"data_at"` // when brew last fetched package data
	FetchedAt  time.Time            `json:"fetched_at"`
}

// DefaultCachePath is $XDG_CACHE_HOME/brewery/snapshot.json, falling back
// to ~/.cache/brewery/snapshot.json when it's unset or not absolute. Without a home directory it uses the
// temp directory rather than a path relative to wherever brewery started.
func DefaultCachePath() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(dir) { // unset, or relative, which the XDG spec says to ignore
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = os.TempDir()
		}
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "brewery", "snapshot.json")
}

// Load reads a cached snapshot. A missing, corrupt or outdated file returns
// an empty Snapshot and an error the caller may ignore.
func Load(path string) (Snapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot cache: %w", err)
	}
	if s.Version != snapshotVersion {
		return Snapshot{}, fmt.Errorf("snapshot cache version %d, want %d", s.Version, snapshotVersion)
	}
	return s, nil
}

// Save writes the snapshot atomically (temp file + rename).
func Save(path string, s Snapshot) error {
	s.Version = snapshotVersion
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "snapshot-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
