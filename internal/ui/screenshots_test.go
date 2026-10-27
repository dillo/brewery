//go:build screenshots || demo

package ui

import (
	"context"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/analytics"
	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/github"
	"github.com/dillo/brewery/internal/jobs"
	"github.com/dillo/brewery/internal/state"
)

// The README's screenshots: `make screenshots` runs this, then freeze turns
// each docs/screenshots/<name>.ansi into a PNG. The machine is made up
// (testdata/demo), so nothing about the machine running it shows. The demo
// video (demo_test.go) runs on the same machine.

// demoNow is the demo machine's clock.
var demoNow = time.Date(2026, 10, 16, 14, 32, 0, 0, time.UTC)

// demoFile reads one of the demo machine's files.
func demoFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "demo", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// demoSize is a package's size on disk: the big ones as they really are,
// the rest spread between 0.1 and 40MB.
func demoSize(key string) int64 {
	big := map[string]int64{
		"cask/docker-desktop": 2_362_232_012, "cask/visual-studio-code": 712_000_000, "cask/firefox": 431_000_000,
		"cask/raycast": 398_000_000, "cask/font-jetbrains-mono-nerd-font": 241_000_000, "cask/iterm2": 94_000_000,
		"formula/go": 309_000_000, "formula/node": 112_000_000, "formula/python@3.13": 87_000_000,
		"formula/icu4c@78": 83_000_000, "formula/postgresql@16": 77_000_000, "formula/git": 63_200_000,
	}
	if n, ok := big[key]; ok {
		return n
	}
	h := fnv.New32a()
	h.Write([]byte(key))
	return 100_000 + int64(h.Sum32()%40_000_000)
}

// demoSnapshot is everything brew would say about the demo machine.
func demoSnapshot(t *testing.T) state.Snapshot {
	t.Helper()
	var s state.Snapshot
	var err error
	if s.Packages, err = brew.ParseInfo(demoFile(t, "info.json")); err != nil {
		t.Fatal(err)
	}
	if s.Outdated, err = brew.ParseOutdated(demoFile(t, "outdated.json")); err != nil {
		t.Fatal(err)
	}
	if s.Services, err = brew.ParseServices(demoFile(t, "services.json")); err != nil {
		t.Fatal(err)
	}
	if s.Taps, err = brew.ParseTaps(demoFile(t, "taps.json")); err != nil {
		t.Fatal(err)
	}
	s.Doctor, s.DoctorRan = brew.ParseDoctor(string(demoFile(t, "doctor.txt"))), true
	s.Cleanup = brew.Cleanup{
		Standard: brew.CleanupEstimate{Items: 14, Size: "1.2GB"},
		All:      brew.CleanupEstimate{Items: 38, Size: "2.7GB"},
	}
	s.CleanupRan = true
	s.AutoremoveRan, s.ServiceFilesRan = true, true
	for _, sv := range s.Services {
		if sv.File != "" {
			s.ServiceFiles = append(s.ServiceFiles, sv.File)
		}
	}
	s.Sizes = map[string]int64{}
	for i, p := range s.Packages {
		s.Sizes[p.Key()] = demoSize(p.Key())
		s.Packages[i].Linked = p.Kind == brew.KindFormula // the demo's info.json predates linked_keg
		if p.Name == "python@3.13" {                      // superseded, for the Deprecated view
			s.Packages[i].Deprecated, s.Packages[i].Reason = true, "versioned_formula"
			s.Packages[i].DeprecationDate, s.Packages[i].Replacement = "2026-04-01", "python@3.15"
		}
	}
	// A second third-party tap, and what Check taps found yesterday.
	s.Taps = append(s.Taps, brew.Tap{Name: "oldfriend/tools", Remote: "https://github.com/oldfriend/homebrew-tools", Formulae: 3})
	s.TapChecks = []github.TapCheck{
		{Tap: "dillo/tap", State: github.Fine},
		{Tap: "oldfriend/tools", State: github.Archived},
	}
	s.TapsCheckedAt = demoNow.Add(-26 * time.Hour)
	// What brew vulns found this morning, made up like the rest: gh is
	// outdated, so its details offer the upgrade.
	s.Vulns = &brew.VulnScan{Checked: 51, Skipped: []string{"ca-certificates", "gettext", "ncurses"}, Findings: []brew.VulnFinding{
		{Formula: "gh", Version: "2.101.0", Vulns: []brew.Vuln{
			{ID: "OSV-2026-4521", Severity: "HIGH", Summary: "Token written to a debug log", Fixed: []string{"2.102.0"}},
			{ID: "OSV-2026-4388", Severity: "MEDIUM", Summary: "Unchecked redirect in browse", Fixed: []string{"2.102.0"}},
		}},
		{Formula: "libssh2", Version: "1.11.1", Vulns: []brew.Vuln{
			{ID: "OSV-2026-3170", Severity: "MEDIUM", Summary: "Out-of-bounds read in packet handling",
				Fixed: []string{"5d2f0c4e8a1b3c7d9e0f2a4b6c8d0e1f3a5b7c9d"}},
		}},
		{Formula: "c-ares", Version: "1.34.8", Vulns: []brew.Vuln{
			{ID: "OSV-2026-2904", Severity: "LOW", Summary: "Read past the end of a short reply"},
		}},
	}}
	s.VulnsAt = demoNow.Add(-5 * time.Hour)
	s.FetchedAt, s.DataAt = demoNow.Add(-2*time.Minute), demoNow.Add(-3*time.Hour)
	return s
}

// demoSource answers every query from the demo snapshot.
type demoSource struct{ snap state.Snapshot }

func (d demoSource) Installed(context.Context) ([]brew.Package, error) { return d.snap.Packages, nil }
func (d demoSource) Outdated(context.Context) ([]brew.Outdated, error) { return d.snap.Outdated, nil }
func (d demoSource) Services(context.Context) ([]brew.Service, error)  { return d.snap.Services, nil }
func (d demoSource) Taps(context.Context) ([]brew.Tap, error)          { return d.snap.Taps, nil }
func (d demoSource) Doctor(context.Context) ([]brew.DoctorWarning, error) {
	return d.snap.Doctor, nil
}
func (d demoSource) Cleanup(context.Context) (brew.Cleanup, error) { return d.snap.Cleanup, nil }
func (d demoSource) Sizes(context.Context, []brew.Package) (map[string]int64, error) {
	return d.snap.Sizes, nil
}
func (d demoSource) Vulns(context.Context) (brew.VulnScan, error)   { return *d.snap.Vulns, nil }
func (d demoSource) DataUpdated(context.Context) (time.Time, error) { return d.snap.DataAt, nil }

// demoStats serves Homebrew's analytics as of the demo machine's day.
type demoStats struct{ lists map[string]analytics.List }

func (d demoStats) Cached(list string) (analytics.List, time.Time, error) {
	return d.lists[list], demoNow.Add(-time.Hour), nil
}
func (d demoStats) Fetch(_ context.Context, list string) (analytics.List, error) {
	return d.lists[list], nil
}
func (d demoStats) Tried() string             { return demoNow.Format(time.DateOnly) }
func (d demoStats) SetTried(day string) error { return nil }

// pipx is what brew says about pipx (brew info and brew deps, Oct 2026).
var pipx = brew.Details{
	Package: brew.Package{
		Name: "pipx", FullName: "pipx", Kind: brew.KindFormula, Tap: "homebrew/core",
		Desc: "Execute binaries from Python packages in isolated environments", Homepage: "https://pipx.pypa.io",
		License: "MIT", Latest: "1.18.1",
	},
	Needs: []string{"ca-certificates", "lz4", "mpdecimal", "openssl@4", "python@3.15", "readline", "sqlite", "xz", "zstd"},
}

// upgradeGh is brew's output while it upgrades gh, as the drawer shows it.
const upgradeGh = "==> Upgrading 1 outdated package:\r\n" +
	"gh 2.101.0 -> 2.102.0\r\n" +
	"==> Fetching downloads for: gh\r\n" +
	"==> Downloading https://ghcr.io/v2/homebrew/core/gh/manifests/2.102.0\r\n" +
	"######################################################################### 100.0%\r\n" +
	"==> Fetching gh\r\n" +
	"==> Downloading https://ghcr.io/v2/homebrew/core/gh/blobs/sha256:4f1c2b…\r\n" +
	"######################################################################### 100.0%\r\n" +
	"==> Upgrading gh\r\n" +
	"  2.101.0 -> 2.102.0\r\n" +
	"==> Pouring gh--2.102.0.arm64_tahoe.bottle.tar.gz\r\n"

// demoApp is brewery on the demo machine at 140×36, icons on.
func demoApp(t *testing.T, fs *fakeStarter) *App {
	t.Helper()
	t.Setenv("HOME", "/Users/demo")
	local := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = local })

	snap := demoSnapshot(t)
	a := New(demoSource{snap}, fs.start, snap, filepath.Join(t.TempDir(), "snapshot.json"))
	a.now = func() time.Time { return demoNow }
	a.tickEvery = 0
	a.UseIcons(true)
	a.readLog = (&fakeLogs{text: map[string]string{"/opt/homebrew/var/log/postgresql@16.log": "" +
		"2026-10-16 09:12:03.418 UTC [4821] LOG:  starting PostgreSQL 16.10 on aarch64-apple-darwin25.0.0\n" +
		"2026-10-16 09:12:03.420 UTC [4821] LOG:  listening on IPv6 address \"::1\", port 5432\n" +
		"2026-10-16 09:12:03.420 UTC [4821] LOG:  listening on IPv4 address \"127.0.0.1\", port 5432\n" +
		"2026-10-16 09:12:03.431 UTC [4821] LOG:  database system is ready to accept connections\n"}}).read

	catalog := &fakeCatalog{
		entries: brew.ParseCatalogue(string(demoFile(t, "catalogue.txt"))),
		added:   []brew.Entry{{Name: "python@3.15", Kind: brew.KindFormula}, {Name: "ghostty", Kind: brew.KindCask}},
		details: map[string]brew.Details{"pipx": pipx},
	}
	a.UseCatalog(catalog)
	lists := map[string]analytics.List{}
	for _, name := range analytics.Lists {
		l, err := analytics.Parse(demoFile(t, strings.ReplaceAll(name, "/", "-")+".json"))
		if err != nil {
			t.Fatal(err)
		}
		lists[name] = l
	}
	a.UseAnalytics(demoStats{lists})
	a.detailsDelay = 0

	recs := []jobs.Record{
		{Title: "upgrade starship", Args: []string{"upgrade", "starship"}, State: jobs.Done,
			Started: demoNow.Add(-26 * time.Hour), Ended: demoNow.Add(-26*time.Hour + 9*time.Second)},
		{Title: "install lazygit", Args: []string{"install", "lazygit"}, State: jobs.Done,
			Started: demoNow.Add(-50 * time.Minute), Ended: demoNow.Add(-50*time.Minute + 6*time.Second)},
		{Title: "update", Args: []string{"update"}, State: jobs.Done,
			Started: demoNow.Add(-3 * time.Hour), Ended: demoNow.Add(-3*time.Hour + 4*time.Second)},
	}
	a.UseHistory(jobs.History{Dir: t.TempDir()}, []jobs.Record{recs[1], recs[2], recs[0]}) // newest first
	a.Update(tea.WindowSizeMsg{Width: 140, Height: 36})
	return a
}

// shoot writes the screen as docs/screenshots/<name>.ansi, colours and all.
func shoot(t *testing.T, a *App, name string) {
	t.Helper()
	screen := a.render()
	plain := stripANSI(screen)
	for _, bad := range []string{"loading…", "starting brewery…", "asking brew…"} {
		if strings.Contains(plain, bad) {
			t.Fatalf("%s is still loading (%q):\n%s", name, bad, plain)
		}
	}
	if strings.TrimSpace(plain) == "" {
		t.Fatalf("%s is empty", name)
	}
	dir := filepath.Join("..", "..", "docs", "screenshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(screen+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScreenshots(t *testing.T) {
	a := demoApp(t, &fakeStarter{})
	show(t, a, "Home")
	shoot(t, a, "home")

	selectInstalled(t, a, "git") // on All
	show(t, a, "Top-level")      // back to the tools, not their libraries
	shoot(t, a, "installed")

	show(t, a, "Popular tools") // Popular tools
	selectDiscover(t, a, "pipx")
	shoot(t, a, "discover")

	selectInstalled(t, a, "gh") // outdated, so its menu offers the upgrade
	show(t, a, "Top-level")
	keys(t, a, "enter")
	shoot(t, a, "menu")
	keys(t, a, "esc")

	selectInstalled(t, a, "python@3.13")
	show(t, a, "Deprecated")
	shoot(t, a, "deprecated")

	selectInstalled(t, a, "gh") // the worst, and outdated
	show(t, a, "Skunky")
	shoot(t, a, "skunky")

	show(t, a, "Services") // postgresql@16, with its log
	keys(t, a, "l")
	shoot(t, a, "services")

	show(t, a, "Maintenance")
	a.maint.table.SetCursor(maintTaps)
	keys(t, a, "l")
	shoot(t, a, "maintenance")

	show(t, a, "Home") // the sidebar's jump from Home, part-way through /maint
	keys(t, a, "h", "/")
	typeText(t, a, "mai")
	shoot(t, a, "jump")
	keys(t, a, "esc")

	p := newFakeProc(0, upgradeGh) // still running when the picture is taken
	fs := &fakeStarter{procs: []*fakeProc{p}}
	a = demoApp(t, fs)
	show(t, a, "Outdated")
	for range len(a.snap.Outdated) {
		if i := a.outdated.cursor(); i >= 0 && a.snap.Outdated[i].Name == "gh" {
			break
		}
		keys(t, a, "j")
	}
	if i := a.outdated.cursor(); i < 0 || a.snap.Outdated[i].Name != "gh" {
		t.Fatal("gh not found on Outdated")
	}
	waiting := settle(t, a, a.handleKey(press("u")))
	shoot(t, a, "upgrade")
	p.exit()
	resume(t, a, waiting)
}

func (d demoSource) Autoremove(context.Context) ([]string, error)   { return d.snap.Autoremove, nil }
func (d demoSource) ServiceFiles(context.Context) ([]string, error) { return d.snap.ServiceFiles, nil }
