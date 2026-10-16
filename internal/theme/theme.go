// Package theme maps semantic UI roles to colors. Views use Styles only, never
// raw colors, so adding a palette (another Catppuccin flavor) touches one file.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Palette holds the Catppuccin color slots brewery uses.
type Palette struct {
	Mantle, Crust            color.Color
	Surface0, Surface1       color.Color
	Overlay0, Subtext0, Text color.Color
	Mauve, Blue, Teal, Green color.Color
	Yellow, Peach, Red, Pink color.Color
}

// Macchiato is Catppuccin Macchiato (https://catppuccin.com/palette).
var Macchiato = Palette{
	Mantle: lipgloss.Color("#1e2030"), Crust: lipgloss.Color("#181926"),
	Surface0: lipgloss.Color("#363a4f"), Surface1: lipgloss.Color("#494d64"),
	Overlay0: lipgloss.Color("#6e738d"), Subtext0: lipgloss.Color("#a5adcb"), Text: lipgloss.Color("#cad3f5"),
	Mauve: lipgloss.Color("#c6a0f6"), Blue: lipgloss.Color("#8aadf4"), Teal: lipgloss.Color("#8bd5ca"), Green: lipgloss.Color("#a6da95"),
	Yellow: lipgloss.Color("#eed49f"), Peach: lipgloss.Color("#f5a97f"), Red: lipgloss.Color("#ed8796"), Pink: lipgloss.Color("#f5bde6"),
}

// Styles are the semantic styles views render with.
type Styles struct {
	P Palette

	Text, Muted, Accent, Title lipgloss.Style
	OK, Warn, Err, Info        lipgloss.Style

	Pane, PaneFocused           lipgloss.Style
	PaneTitle, PaneTitleFocused lipgloss.Style

	Tab, TabActive, TabRule      lipgloss.Style
	View, ViewActive             lipgloss.Style
	ChipWarn, ChipErr, ChipStale lipgloss.Style
	StatusBar                    lipgloss.Style

	TableHeader, TableCell, TableSelected lipgloss.Style

	Icons Icons // none until the app turns them on
}

func New(p Palette) Styles {
	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	pane := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Surface1).Padding(0, 1)
	return Styles{
		P:    p,
		Text: fg(p.Text), Muted: fg(p.Overlay0), Accent: fg(p.Mauve), Title: fg(p.Blue).Bold(true),
		OK: fg(p.Green), Warn: fg(p.Yellow), Err: fg(p.Red), Info: fg(p.Teal),

		Pane:             pane,
		PaneFocused:      pane.BorderForeground(p.Mauve),
		PaneTitle:        fg(p.Subtext0),
		PaneTitleFocused: fg(p.Mauve).Bold(true),

		Tab:        lipgloss.NewStyle().Foreground(p.Text).Background(p.Surface0),
		TabActive:  lipgloss.NewStyle().Foreground(p.Crust).Background(p.Mauve).Bold(true),
		TabRule:    fg(p.Surface1),
		View:       fg(p.Subtext0).Padding(0, 1),
		ViewActive: fg(p.Mauve).Background(p.Surface0).Bold(true).Padding(0, 1),
		ChipWarn:   chip(p.Yellow, p.Crust),
		ChipErr:    chip(p.Red, p.Crust),
		ChipStale:  chip(p.Peach, p.Crust),
		StatusBar:  lipgloss.NewStyle().Foreground(p.Subtext0).Background(p.Mantle),

		TableHeader: fg(p.Subtext0).Bold(true).Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(p.Surface1),
		// No colors on cells: a cell's closing reset would cut the selected
		// row's highlight short. Color inside a cell goes through Tint.
		TableCell: lipgloss.NewStyle().Padding(0, 1),
		// The selected row keeps the terminal's text color, as the other rows
		// do: Tint goes back to it, so text after a tinted cell matches.
		TableSelected: lipgloss.NewStyle().Background(p.Surface0).Bold(true),
	}
}

// chip is a small solid badge, such as the ↑2 on a tab.
func chip(bg, fg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(fg).Background(bg).Bold(true).Padding(0, 1)
}

// Mode returns the status-bar badge style for an editor-style mode name.
func (s Styles) Mode(mode string) lipgloss.Style {
	c := s.P.Mauve
	switch mode {
	case "COMMAND":
		c = s.P.Blue
	case "FILTER":
		c = s.P.Teal
	case "RUNNING":
		c = s.P.Peach
	case "INSERT":
		c = s.P.Green
	}
	return lipgloss.NewStyle().Foreground(s.P.Crust).Background(c).Bold(true).Padding(0, 1)
}

// Tint colors s with c and then resets only the foreground, so text in a
// table cell keeps the selected row's background and bold.
func Tint(c color.Color, s string) string {
	return ansi.NewStyle().ForegroundColor(c).String() + s + ansi.NewStyle().ForegroundColor(nil).String()
}
