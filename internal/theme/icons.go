package theme

// Icons are glyphs brewery draws before some words. The Nerd Font ones
// need a Nerd Font in the terminal; with the zero Icons, brewery draws the
// words alone.
type Icons struct {
	Formula, Cask, Pinned    string
	Started, Stopped, Failed string // a service's state
}

// Nerd are Nerd Font (v3) glyphs.
var Nerd = Icons{
	Formula: "\uf0fc",     // nf-fa-beer
	Cask:    "\U000f003b", // nf-md-apps
	Pinned:  "\U000f0403", // nf-md-pin
	Started: "\U000f040a", // nf-md-play
	Stopped: "\U000f04db", // nf-md-stop
	Failed:  "\U000f0028", // nf-md-alert_circle
}

// With puts icon before word, or gives word alone when icon is empty.
func With(icon, word string) string {
	if icon == "" {
		return word
	}
	return icon + " " + word
}
