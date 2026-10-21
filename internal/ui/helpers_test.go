package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/state"
	"github.com/dillo/brewery/internal/theme"
)

var testStyles = theme.New(theme.Macchiato)

func stripANSI(s string) string { return ansi.Strip(s) }

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "brew", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// fixtureSnapshot is the brew package's test fixtures, parsed.
func fixtureSnapshot(t *testing.T) state.Snapshot {
	t.Helper()
	var s state.Snapshot
	var err error
	if s.Packages, err = brew.ParseInfo(readFixture(t, "info.json")); err != nil {
		t.Fatal(err)
	}
	if s.Outdated, err = brew.ParseOutdated(readFixture(t, "outdated.json")); err != nil {
		t.Fatal(err)
	}
	if s.Services, err = brew.ParseServices(readFixture(t, "services.json")); err != nil {
		t.Fatal(err)
	}
	if s.Taps, err = brew.ParseTaps(readFixture(t, "taps.json")); err != nil {
		t.Fatal(err)
	}
	s.Doctor, s.DoctorRan = brew.ParseDoctor(string(readFixture(t, "doctor.txt"))), true
	s.Cleanup = brew.Cleanup{
		Standard: brew.CleanupEstimate{Items: 1, Size: "3.9KB"},
		All:      brew.ParseCleanup(string(readFixture(t, "cleanup.txt"))),
	}
	s.CleanupRan = true
	s.Autoremove, s.AutoremoveRan = []string{"libyaml", "m4"}, true
	agents := "/Users/locutus/Library/LaunchAgents/"
	s.ServiceFiles = []string{agents + "homebrew.mxcl.mysql.plist", agents + "homebrew.mxcl.postgresql@16.plist", agents + "homebrew.mxcl.redis.plist"}
	s.ServiceFilesRan = true
	s.Sizes = map[string]int64{"formula/git": 52_480_000, "cask/docker-desktop": 2_362_232_012, "formula/fd": 3_100_000}
	scan, err := brew.ParseVulns(readFixture(t, "vulns.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.Vulns, s.VulnsAt = &scan, testNow.Add(-3*time.Hour)
	return s
}

func press(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	r := []rune(k)[0]
	return tea.KeyPressMsg{Code: r, Text: k}
}

// show selects the sidebar entry labelled label and focuses its list, as
// moving there in the sidebar and pressing l would.
func show(t *testing.T, a *App, label string) {
	t.Helper()
	settle(t, a, a.selectEntry(entryIndex(label)))
	a.focus = colList
	_, cmd := a.Update(nil) // what follows any key: Services' log, Discover's details, the layout
	settle(t, a, cmd)
}
