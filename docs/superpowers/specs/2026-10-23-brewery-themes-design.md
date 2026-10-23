# Themes: design

brewery's colours are Catppuccin Macchiato, fixed. Users can now choose
from twelve popular dark themes. `t` opens a picker that previews each one
live, and brewery remembers the choice the next time it starts.
Macchiato stays the default.

## Scope

- Themes set foreground colours and highlights only, as Macchiato does
  now. The terminal's own background shows through, so only dark themes
  are offered: a light theme would be unreadable on a dark terminal.
- No theme files written by users, no light themes, no painted
  background.

## The themes

In picker order, each with the name `:theme` and the saved file use:

| Theme | Name |
|---|---|
| Catppuccin Macchiato (default) | `catppuccin-macchiato` |
| Catppuccin Mocha | `catppuccin-mocha` |
| Catppuccin Frappé | `catppuccin-frappe` |
| Dracula | `dracula` |
| Nord | `nord` |
| Gruvbox Dark | `gruvbox-dark` |
| Tokyo Night | `tokyo-night` |
| One Dark | `one-dark` |
| Rosé Pine | `rose-pine` |
| Kanagawa | `kanagawa` |
| Everforest Dark | `everforest-dark` |
| Solarized Dark | `solarized-dark` |

Each fills all fifteen slots of `theme.Palette` from the theme's published
palette. Where a theme has no colour for a slot's role, the slot takes the
theme's nearest standard colour: Mauve, the accent, takes the theme's
purple or magenta (Gruvbox's purple, Nord's `#b48ead`); Peach takes its
orange; the Mantle, Crust and Surface slots take its darker background
shades.

## Behaviour

- **Opening the picker:** `t`, anywhere in the main screen, except while
  typing in the `:` or `/` prompt; or `:theme` with no name. `?` lists
  `t` with the keys that work everywhere.
- **The box:** a centred overlay drawn like the action menu, titled
  `Theme`, listing the twelve themes by their display names. The theme in
  use has a `✓`, and the cursor starts on it. The footer reads
  `j/k preview · enter keep · esc cancel`.
- **Preview:** `j`/`k`, the arrows, `gg` and `G` move the cursor, and the
  theme under it is applied at once to the whole app, the box included.
- **`enter`:** keeps the theme under the cursor, saves it, and closes the
  box. The status bar says `theme: Gruvbox Dark`.
- **`esc` or `q`:** switches back to the theme in use when the box opened,
  and closes it. Nothing is saved.
- **`:theme <name>`:** switches to that theme and saves it, without the
  box. Tab completes the names. An unknown name is an error on the `:`
  line, as other mistakes there are, and nothing changes.
- **Jobs:** a running job keeps streaming into the drawer while the
  picker is open or the theme changes; only its colours change.

## Remembering the choice

- The name is saved in `theme` in brewery's state directory,
  `jobs.DefaultStateDir()` (`$XDG_STATE_HOME/brewery`, else
  `~/.local/state/brewery`), as one line, such as `gruvbox-dark`.
- `cmd/brewery` reads it at startup, next to where the history is read,
  and applies it before the first frame.
- A missing or unreadable file, or a name brewery doesn't know (such as a
  theme a later version renamed), means Macchiato, with no error.
- The file is written only on `enter` in the picker and by
  `:theme <name>`, never while previewing. If the write fails, the theme
  still applies for this session, and the status bar says the choice
  couldn't be saved.

## Code shape

- **`internal/theme`:** a `Theme` type (`Name`, `Title`, `Palette`) and a
  `Themes` list in picker order, Macchiato first. `Lookup(name)` returns
  the theme with that name, or Macchiato. `Palette` and `New` don't
  change; the `Macchiato` variable stays as Macchiato's palette.
- **Saving:** `LoadTheme(dir) string` and `SaveTheme(dir, name) error`, in
  `internal/theme`, reading and writing `<dir>/theme`. `SaveTheme`
  creates `dir` if needed.
- **Switching:** `App.UseTheme(name)` builds `theme.New` from the
  palette, keeping the current icons, and hands the styles to every part
  that keeps its own copy (the same list `UseIcons` walks). It also
  re-applies styles that were set when a part was built: the tables'
  styles, the `:` and `/` inputs' styles, and the drawer's. It then
  rebuilds rows that were coloured when they were set (History,
  Outdated, Services) and calls `a.sync()`. `UseIcons` and `UseTheme`
  share the walk over the parts.
- **Where it's saved:** `App` gets the state directory to save to,
  through a `UseThemeDir(dir)` setter called from `cmd/brewery`, so tests
  can point it at a temp directory. With no directory set, nothing is
  saved.
- **The picker:** its own type, `themeBox` (cursor, the theme in use when
  it opened), in `internal/ui/themes.go`, with a `key` method and a
  `view` drawn like `menuBox.view`. It isn't a `menuBox`: the menu runs
  one action and closes, while the picker applies a theme on every move
  and puts the old one back on cancel. `g` then `g` goes to the top, as
  in the menu.
- **`:theme`:** handled in the `:` line alongside `q` and `jobs`, before
  anything is passed to brew. Completion offers `theme` and, after it,
  the theme names.

## Testing

- **Theme list:** every theme has a unique name and title, and every
  `Palette` slot is set; the first theme is Macchiato.
- **`Lookup`:** a known name returns that theme; an unknown or empty name
  returns Macchiato.
- **Saving:** in a temp directory, save then load gives the name back; a
  missing file, junk or an unknown name loads as Macchiato; a write that
  can't happen returns an error.
- **Switching:** after `UseTheme("dracula")`, a render with colours shows
  Dracula's colour codes, and not Macchiato's, in the sidebar title, a
  table header, the `:` prompt and a History row, so no part keeps old
  styles.
- **Picker keys:** `t` opens it on the theme in use; `j` previews the
  next one; `esc` puts the first back and saves nothing; `enter` saves the
  one under the cursor; `:theme dracula` switches and saves;
  `:theme nope` errors and changes nothing; a failed save shows the
  status-bar note.
- **Snapshots:** a new golden snapshot of the picker open over Home. The
  others don't change: they're compared without colours, and the default
  is still Macchiato.

## Docs

- README: themes in Features; `t` in the Keys tables; the `theme` file
  in Files.
