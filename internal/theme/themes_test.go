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

func TestIndex(t *testing.T) {
	if i, ok := Index("dracula"); !ok || Themes[i].Name != "dracula" {
		t.Errorf("Index(dracula) = %d, %v", i, ok)
	}
	if i, ok := Index("catppuccin-macchiato"); !ok || i != 0 {
		t.Errorf("Index(catppuccin-macchiato) = %d, %v; want 0, true", i, ok)
	}
	for _, name := range []string{"", "nope", "Dracula"} {
		if i, ok := Index(name); ok {
			t.Errorf("Index(%q) = %d, true; there is no such theme", name, i)
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
