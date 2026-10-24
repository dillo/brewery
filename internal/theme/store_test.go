package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveThenLoadTheme(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "brewery") // not there yet: SaveTheme makes it
	if err := SaveTheme(dir, "nord"); err != nil {
		t.Fatal(err)
	}
	if got := LoadTheme(dir); got != "nord" {
		t.Errorf("LoadTheme = %q, want nord", got)
	}
}

func TestLoadThemeFallsBackToTheDefault(t *testing.T) {
	missing := t.TempDir()
	junk := t.TempDir()
	if err := os.WriteFile(filepath.Join(junk, "theme"), []byte("\x00\x01 not a theme\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	renamed := t.TempDir()
	if err := os.WriteFile(filepath.Join(renamed, "theme"), []byte("solarized-darker\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{"missing": missing, "junk": junk, "unknown name": renamed} {
		if got := LoadTheme(dir); got != "catppuccin-macchiato" {
			t.Errorf("%s: LoadTheme = %q, want the default", name, got)
		}
	}
}

func TestSaveThemeReportsAFailedWrite(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SaveTheme(filepath.Join(file, "brewery"), "nord"); err == nil {
		t.Error("a directory under a file can't be made, so saving should fail")
	}
}
