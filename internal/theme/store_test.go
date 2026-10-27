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

func TestSaveThemeReplacesASymlink(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(dir, "theme")); err != nil {
		t.Fatal(err)
	}
	if err := SaveTheme(dir, "nord"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "original" {
		t.Errorf("saving wrote through the symlink: victim holds %q", b)
	}
	if got := LoadTheme(dir); got != "nord" {
		t.Errorf("LoadTheme = %q, want nord", got)
	}
}
