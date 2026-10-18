package ui

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/jobs"
)

// update rewrites the golden snapshots: go test ./internal/ui -run TestScreens -update
var update = flag.Bool("update", false, "rewrite the golden snapshots in testdata/golden")

// golden compares got with testdata/golden/<name>.txt, or rewrites the file
// with -update.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".txt")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (go test ./internal/ui -run TestScreens -update writes it)", err)
	}
	if got != string(want) {
		t.Errorf("%s changed (if that's intended, rewrite it with -update); got:\n%s", path, got)
	}
}

// TestScreens snapshots brewery's main screens at 140×40, icons off, from
// the test fixtures. The machine running it mustn't show through: paths
// under $HOME are shortened to ~ (the fixtures were captured under
// /Users/locutus), and dates are written in the local time zone.
func TestScreens(t *testing.T) {
	t.Setenv("HOME", "/Users/locutus")
	local := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = local })

	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.UseCatalog(newFakeCatalog(t))
	a.UseAnalytics(newFakeStats(t))
	a.detailsDelay = 0
	a.UseHistory(jobs.History{Dir: t.TempDir()}, recentJobs(a.now())) // the clock is fixed, so the times are too
	a.UseBrewVersion("4.6.3")
	for _, s := range []struct {
		name    string
		entries []string // shown in turn: the last one is the screen
		keys    []string
	}{
		{"home", []string{"Home"}, nil},
		{"installed", []string{"Top-level"}, []string{"j", "j", "j"}}, // git, with its preview
		{"outdated", []string{"Outdated"}, nil},
		{"services", []string{"Services"}, nil},
		{"discover-all", []string{"Popular tools", "Search all"}, nil}, // the selection (gh) carries over from Tools
		{"history", []string{"History"}, []string{"l"}},
		{"maintenance", []string{"Maintenance"}, nil},
		{"help", []string{"Top-level"}, []string{"?"}},
	} {
		for _, e := range s.entries {
			show(t, a, e)
		}
		keys(t, a, s.keys...)
		golden(t, s.name, stripANSI(a.render()))
		if a.help {
			keys(t, a, "esc") // any key closes help
		}
	}
	a.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	show(t, a, "Top-level")
	golden(t, "narrow-list", stripANSI(a.render()))
	keys(t, a, "l")
	golden(t, "narrow-details", stripANSI(a.render()))
}
