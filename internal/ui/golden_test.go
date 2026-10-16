package ui

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
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

// TestScreens snapshots brewery's main screens at 120×40, icons off, from
// the test fixtures.
func TestScreens(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.UseCatalog(newFakeCatalog(t))
	a.UseAnalytics(newFakeStats(t))
	a.detailsDelay = 0
	for _, s := range []struct {
		name string
		keys []string
	}{
		{"home", []string{"1"}},
		{"installed", []string{"2", "j", "j", "j"}}, // git, with its preview
		{"outdated", []string{"3"}},
		{"services", []string{"4"}},
		{"discover-all", []string{"6", "F"}},
		{"help", []string{"2", "?"}},
	} {
		keys(t, a, s.keys...)
		golden(t, s.name, stripANSI(a.render()))
		if a.help {
			keys(t, a, "esc") // any key closes help
		}
	}
}
