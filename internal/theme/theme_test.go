package theme

import (
	"fmt"
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
