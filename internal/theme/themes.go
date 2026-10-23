package theme

import "charm.land/lipgloss/v2"

// Theme is a palette and the names brewery knows it by: Name is what
// :theme takes and the saved file holds, Title is what the picker shows.
type Theme struct {
	Name, Title string
	Palette     Palette
}

// Themes are the themes the picker offers, in its order; the first is the
// default. All are dark: brewery colours text, not the background, so the
// terminal's own background shows through. A slot a theme has no colour
// for takes the one its terminal colours use there, else its nearest
// shade.
var Themes = []Theme{
	{"catppuccin-macchiato", "Catppuccin Macchiato", Macchiato},
	{"catppuccin-mocha", "Catppuccin Mocha", Palette{
		Mantle: lipgloss.Color("#181825"), Crust: lipgloss.Color("#11111b"),
		Surface0: lipgloss.Color("#313244"), Surface1: lipgloss.Color("#45475a"),
		Overlay0: lipgloss.Color("#6c7086"), Subtext0: lipgloss.Color("#a6adc8"), Text: lipgloss.Color("#cdd6f4"),
		Mauve: lipgloss.Color("#cba6f7"), Blue: lipgloss.Color("#89b4fa"), Teal: lipgloss.Color("#94e2d5"), Green: lipgloss.Color("#a6e3a1"),
		Yellow: lipgloss.Color("#f9e2af"), Peach: lipgloss.Color("#fab387"), Red: lipgloss.Color("#f38ba8"), Pink: lipgloss.Color("#f5c2e7"),
	}},
	{"catppuccin-frappe", "Catppuccin Frappé", Palette{
		Mantle: lipgloss.Color("#292c3c"), Crust: lipgloss.Color("#232634"),
		Surface0: lipgloss.Color("#414559"), Surface1: lipgloss.Color("#51576d"),
		Overlay0: lipgloss.Color("#737994"), Subtext0: lipgloss.Color("#a5adce"), Text: lipgloss.Color("#c6d0f5"),
		Mauve: lipgloss.Color("#ca9ee6"), Blue: lipgloss.Color("#8caaee"), Teal: lipgloss.Color("#81c8be"), Green: lipgloss.Color("#a6d189"),
		Yellow: lipgloss.Color("#e5c890"), Peach: lipgloss.Color("#ef9f76"), Red: lipgloss.Color("#e78284"), Pink: lipgloss.Color("#f4b8e4"),
	}},
	{"dracula", "Dracula", Palette{
		Mantle: lipgloss.Color("#21222c"), Crust: lipgloss.Color("#191a21"),
		Surface0: lipgloss.Color("#44475a"), Surface1: lipgloss.Color("#6272a4"),
		Overlay0: lipgloss.Color("#6272a4"), Subtext0: lipgloss.Color("#f8f8f2"), Text: lipgloss.Color("#f8f8f2"),
		Mauve: lipgloss.Color("#ff79c6"), Blue: lipgloss.Color("#bd93f9"), Teal: lipgloss.Color("#8be9fd"), Green: lipgloss.Color("#50fa7b"),
		Yellow: lipgloss.Color("#f1fa8c"), Peach: lipgloss.Color("#ffb86c"), Red: lipgloss.Color("#ff5555"), Pink: lipgloss.Color("#ff79c6"),
	}},
	{"nord", "Nord", Palette{
		Mantle: lipgloss.Color("#2e3440"), Crust: lipgloss.Color("#2e3440"),
		Surface0: lipgloss.Color("#3b4252"), Surface1: lipgloss.Color("#434c5e"),
		Overlay0: lipgloss.Color("#616e88"), Subtext0: lipgloss.Color("#d8dee9"), Text: lipgloss.Color("#eceff4"),
		Mauve: lipgloss.Color("#b48ead"), Blue: lipgloss.Color("#81a1c1"), Teal: lipgloss.Color("#8fbcbb"), Green: lipgloss.Color("#a3be8c"),
		Yellow: lipgloss.Color("#ebcb8b"), Peach: lipgloss.Color("#d08770"), Red: lipgloss.Color("#bf616a"), Pink: lipgloss.Color("#b48ead"),
	}},
	{"gruvbox-dark", "Gruvbox Dark", Palette{
		Mantle: lipgloss.Color("#1d2021"), Crust: lipgloss.Color("#1d2021"),
		Surface0: lipgloss.Color("#3c3836"), Surface1: lipgloss.Color("#504945"),
		Overlay0: lipgloss.Color("#928374"), Subtext0: lipgloss.Color("#d5c4a1"), Text: lipgloss.Color("#ebdbb2"),
		Mauve: lipgloss.Color("#d3869b"), Blue: lipgloss.Color("#83a598"), Teal: lipgloss.Color("#8ec07c"), Green: lipgloss.Color("#b8bb26"),
		Yellow: lipgloss.Color("#fabd2f"), Peach: lipgloss.Color("#fe8019"), Red: lipgloss.Color("#fb4934"), Pink: lipgloss.Color("#d3869b"),
	}},
	{"tokyo-night", "Tokyo Night", Palette{
		Mantle: lipgloss.Color("#16161e"), Crust: lipgloss.Color("#15161e"),
		Surface0: lipgloss.Color("#292e42"), Surface1: lipgloss.Color("#414868"),
		Overlay0: lipgloss.Color("#565f89"), Subtext0: lipgloss.Color("#a9b1d6"), Text: lipgloss.Color("#c0caf5"),
		Mauve: lipgloss.Color("#bb9af7"), Blue: lipgloss.Color("#7aa2f7"), Teal: lipgloss.Color("#73daca"), Green: lipgloss.Color("#9ece6a"),
		Yellow: lipgloss.Color("#e0af68"), Peach: lipgloss.Color("#ff9e64"), Red: lipgloss.Color("#f7768e"), Pink: lipgloss.Color("#bb9af7"),
	}},
	{"one-dark", "One Dark", Palette{
		Mantle: lipgloss.Color("#21252b"), Crust: lipgloss.Color("#181a1f"),
		Surface0: lipgloss.Color("#3e4451"), Surface1: lipgloss.Color("#4b5263"),
		Overlay0: lipgloss.Color("#5c6370"), Subtext0: lipgloss.Color("#9da5b4"), Text: lipgloss.Color("#abb2bf"),
		Mauve: lipgloss.Color("#c678dd"), Blue: lipgloss.Color("#61afef"), Teal: lipgloss.Color("#56b6c2"), Green: lipgloss.Color("#98c379"),
		Yellow: lipgloss.Color("#e5c07b"), Peach: lipgloss.Color("#d19a66"), Red: lipgloss.Color("#e06c75"), Pink: lipgloss.Color("#c678dd"),
	}},
	{"rose-pine", "Rosé Pine", Palette{
		Mantle: lipgloss.Color("#191724"), Crust: lipgloss.Color("#191724"),
		Surface0: lipgloss.Color("#403d52"), Surface1: lipgloss.Color("#524f67"),
		Overlay0: lipgloss.Color("#6e6a86"), Subtext0: lipgloss.Color("#908caa"), Text: lipgloss.Color("#e0def4"),
		Mauve: lipgloss.Color("#c4a7e7"), Blue: lipgloss.Color("#9ccfd8"), Teal: lipgloss.Color("#ebbcba"), Green: lipgloss.Color("#31748f"),
		Yellow: lipgloss.Color("#f6c177"), Peach: lipgloss.Color("#f6c177"), Red: lipgloss.Color("#eb6f92"), Pink: lipgloss.Color("#ebbcba"),
	}},
	{"kanagawa", "Kanagawa", Palette{
		Mantle: lipgloss.Color("#16161d"), Crust: lipgloss.Color("#16161d"),
		Surface0: lipgloss.Color("#2d4f67"), Surface1: lipgloss.Color("#54546d"),
		Overlay0: lipgloss.Color("#727169"), Subtext0: lipgloss.Color("#c8c093"), Text: lipgloss.Color("#dcd7ba"),
		Mauve: lipgloss.Color("#957fb8"), Blue: lipgloss.Color("#7e9cd8"), Teal: lipgloss.Color("#7aa89f"), Green: lipgloss.Color("#98bb6c"),
		Yellow: lipgloss.Color("#e6c384"), Peach: lipgloss.Color("#ffa066"), Red: lipgloss.Color("#e46876"), Pink: lipgloss.Color("#d27e99"),
	}},
	{"everforest-dark", "Everforest Dark", Palette{
		Mantle: lipgloss.Color("#232a2e"), Crust: lipgloss.Color("#232a2e"),
		Surface0: lipgloss.Color("#3d484d"), Surface1: lipgloss.Color("#4f585e"),
		Overlay0: lipgloss.Color("#859289"), Subtext0: lipgloss.Color("#9da9a0"), Text: lipgloss.Color("#d3c6aa"),
		Mauve: lipgloss.Color("#d699b6"), Blue: lipgloss.Color("#7fbbb3"), Teal: lipgloss.Color("#83c092"), Green: lipgloss.Color("#a7c080"),
		Yellow: lipgloss.Color("#dbbc7f"), Peach: lipgloss.Color("#e69875"), Red: lipgloss.Color("#e67e80"), Pink: lipgloss.Color("#d699b6"),
	}},
	{"solarized-dark", "Solarized Dark", Palette{
		Mantle: lipgloss.Color("#002b36"), Crust: lipgloss.Color("#002b36"),
		Surface0: lipgloss.Color("#073642"), Surface1: lipgloss.Color("#586e75"),
		Overlay0: lipgloss.Color("#586e75"), Subtext0: lipgloss.Color("#839496"), Text: lipgloss.Color("#93a1a1"),
		Mauve: lipgloss.Color("#6c71c4"), Blue: lipgloss.Color("#268bd2"), Teal: lipgloss.Color("#2aa198"), Green: lipgloss.Color("#859900"),
		Yellow: lipgloss.Color("#b58900"), Peach: lipgloss.Color("#cb4b16"), Red: lipgloss.Color("#dc322f"), Pink: lipgloss.Color("#d33682"),
	}},
}

// Lookup returns the theme called name, or the default when brewery has
// none by that name.
func Lookup(name string) Theme {
	for _, t := range Themes {
		if t.Name == name {
			return t
		}
	}
	return Themes[0]
}
