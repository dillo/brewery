package theme

import (
	"os"
	"path/filepath"
	"strings"
)

// themeFile holds the chosen theme's name, in brewery's state directory.
const themeFile = "theme"

// LoadTheme returns the name of the theme saved in dir. With nothing
// saved, or a name brewery doesn't know (a theme a later version renamed),
// it returns the default's.
func LoadTheme(dir string) string {
	if dir == "" { // no state dir: nothing was saved
		return Themes[0].Name
	}
	b, err := os.ReadFile(filepath.Join(dir, themeFile)) // #nosec G304 -- brewery's own state directory
	if err != nil {
		return Themes[0].Name
	}
	return Lookup(strings.TrimSpace(string(b))).Name
}

// SaveTheme keeps name in dir for the next run, making dir if needed. It
// writes a temp file and renames it into place, so a symlink planted where
// the file goes is replaced rather than written through.
func SaveTheme(dir, name string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".theme-*") // 0600
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // fails harmlessly once renamed
	if _, err := f.WriteString(name + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, themeFile))
}
