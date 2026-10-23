package theme

import (
	"reflect"
	"regexp"
	"testing"
)

func TestThemes(t *testing.T) {
	if len(Themes) != 12 {
		t.Errorf("%d themes, want 12", len(Themes))
	}
	if first := Themes[0]; first.Name != "catppuccin-macchiato" || !reflect.DeepEqual(first.Palette, Macchiato) {
		t.Errorf("the first theme is the default, Macchiato: got %s", first.Name)
	}
	name := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	names, titles := map[string]bool{}, map[string]bool{}
	for _, th := range Themes {
		if !name.MatchString(th.Name) {
			t.Errorf("%q: names are lower-case words joined by -", th.Name)
		}
		if names[th.Name] || titles[th.Title] {
			t.Errorf("%s (%s) is in the list twice", th.Name, th.Title)
		}
		names[th.Name], titles[th.Title] = true, true
		p := reflect.ValueOf(th.Palette)
		for i := range p.NumField() {
			if p.Field(i).IsNil() {
				t.Errorf("%s: %s isn't set", th.Name, p.Type().Field(i).Name)
			}
		}
	}
}

func TestLookup(t *testing.T) {
	if got := Lookup("dracula"); got.Name != "dracula" || got.Title != "Dracula" {
		t.Errorf("Lookup(dracula) = %s (%s)", got.Name, got.Title)
	}
	for _, name := range []string{"", "nope", "Dracula"} {
		if got := Lookup(name); got.Name != "catppuccin-macchiato" {
			t.Errorf("Lookup(%q) = %s, want the default", name, got.Name)
		}
	}
}
