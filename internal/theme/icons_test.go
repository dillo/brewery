package theme

import "testing"

func TestWith(t *testing.T) {
	if got := With("", "cask"); got != "cask" {
		t.Errorf("no icon: %q", got)
	}
	if got := With(Nerd.Cask, "cask"); got != "\U000f003b cask" {
		t.Errorf("with an icon: %q", got)
	}
}
