package theme

import (
	"fmt"
	"image/color"
	"testing"
)

func TestModeColorsAreDistinct(t *testing.T) {
	st := New(Macchiato)
	seen := map[string]string{}
	for _, mode := range []string{"NORMAL", "COMMAND", "FILTER", "RUNNING", "INSERT"} {
		bg := st.Mode(mode).GetBackground()
		r, g, b, _ := bg.RGBA()
		key := fmt.Sprint(r, g, b)
		if other, dup := seen[key]; dup {
			t.Errorf("%s and %s share a badge color", mode, other)
		}
		seen[key] = mode
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
