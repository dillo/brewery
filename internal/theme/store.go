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
	b, err := os.ReadFile(filepath.Join(dir, themeFile)) // #nosec G304 -- brewery's own state directory
	if err != nil {
		return Themes[0].Name
	}
	return Lookup(strings.TrimSpace(string(b))).Name
}

// SaveTheme keeps name in dir for the next run, making dir if needed.
func SaveTheme(dir, name string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, themeFile), []byte(name+"\n"), 0o600)
}
