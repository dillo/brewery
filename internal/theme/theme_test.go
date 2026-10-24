package theme

import (
	"fmt"
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestModeColorsAreDistinct(t *testing.T) {
	for _, th := range Themes {
		st := New(th.Palette)
		seen := map[string]string{}
		for _, mode := range []string{"NORMAL", "COMMAND", "FILTER", "RUNNING", "INSERT"} {
			bg := st.Mode(mode).GetBackground()
			r, g, b, _ := bg.RGBA()
			key := fmt.Sprint(r, g, b)
			if other, dup := seen[key]; dup {
				t.Errorf("%s: %s and %s share a badge color", th.Name, mode, other)
			}
			seen[key] = mode
		}
	}
}

func TestUnknownModeFallsBackToNormal(t *testing.T) {
	st := New(Macchiato)
	if st.Mode("WHATEVER").GetBackground() != st.Mode("NORMAL").GetBackground() {
		t.Error("unknown modes should look like NORMAL")
	}
}

func TestTabChipsAreDistinct(t *testing.T) {
	st := New(Macchiato)
	chips := map[string]any{"warn": st.ChipWarn, "err": st.ChipErr, "stale": st.ChipStale}
	seen := map[string]string{}
	for name, c := range chips {
		r, g, b, _ := c.(interface{ GetBackground() color.Color }).GetBackground().RGBA()
		key := fmt.Sprint(r, g, b)
		if other, dup := seen[key]; dup {
			t.Errorf("%s and %s chips share a color", name, other)
		}
		seen[key] = name
	}
}

func TestSelectedRowKeepsTheTerminalText(t *testing.T) {
	// Tint resets to the terminal's text color, so the selected row must use
	// it too, or the cells after a tinted one would change color.
	if c := New(Macchiato).TableSelected.GetForeground(); c != (lipgloss.NoColor{}) {
		t.Errorf("TableSelected has foreground %v", c)
	}
}

func TestTintResetsOnlyTheForeground(t *testing.T) {
	got := Tint(Macchiato.Green, "started")
	if got != "\x1b[38;2;166;218;149mstarted\x1b[39m" {
		t.Errorf("Tint = %q", got)
	}
}
