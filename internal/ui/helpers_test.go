package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/state"
	"brewery/internal/theme"
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
	return s
}

func press(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	r := []rune(k)[0]
	return tea.KeyPressMsg{Code: r, Text: k}
}
