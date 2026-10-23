# Themes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users switch brewery between twelve dark themes with a live-preview picker (`t` or `:theme`), and remember the choice across runs.

**Architecture:** `internal/theme` gains a list of named palettes, a lookup that falls back to Macchiato, and load/save of the chosen name in brewery's state directory. `internal/ui` gains `App.UseTheme`, which rebuilds the styles every part keeps its own copy of (the way `UseIcons` already walks them), re-applies the styles baked into tables and text inputs, and rebuilds pre-coloured rows. A small `themeBox` overlay previews themes as its cursor moves; `:theme <name>` switches directly. `cmd/brewery` loads the saved theme at startup.

**Tech Stack:** Go, Bubble Tea v2 (`charm.land/bubbletea/v2`), Bubbles v2 (`charm.land/bubbles/v2/table`, `textinput`), Lip Gloss v2.

**Spec:** `docs/superpowers/specs/2026-10-23-brewery-themes-design.md`

## Global Constraints

- Macchiato (`catppuccin-macchiato`) is the default and the first theme in the list.
- Only dark themes; themes set foreground colours and highlights only, never a painted background.
- Theme names are lower-case words joined by `-`, exactly: `catppuccin-macchiato`, `catppuccin-mocha`, `catppuccin-frappe`, `dracula`, `nord`, `gruvbox-dark`, `tokyo-night`, `one-dark`, `rose-pine`, `kanagawa`, `everforest-dark`, `solarized-dark`.
- The saved file is `theme` in `jobs.DefaultStateDir()`, one line holding the name. Directories are created `0o700`, the file written `0o600`, as the rest of brewery's files are.
- A missing or unreadable file, or an unknown name, means Macchiato, with no error.
- The file is written only on `enter` in the picker and by `:theme <name>`, never while previewing.
- Status messages: `theme: <Title>` on a kept theme; a failed save says the theme couldn't be saved and the theme still applies.
- Picker footer, exactly: `j/k preview · enter keep · esc cancel`. Box title: `Theme`.
- Work on a branch (`feat/themes`), never on `master`. Commits use the repo's existing author (`Dillo Raju <4197+dillo@users.noreply.github.com>`), conventional prefixes as in `git log` (`feat(theme):`, `feat(ui):`, `docs:`).
- `make lint` (golangci-lint v2 with gosec, gocognit ≤ 20, gocyclo ≤ 20) must stay clean.

## File Structure

- Create `internal/theme/themes.go` — the `Theme` type, the `Themes` list with all twelve palettes, `Lookup`.
- Create `internal/theme/themes_test.go` — list and lookup tests.
- Create `internal/theme/store.go` — `LoadTheme`, `SaveTheme`.
- Create `internal/theme/store_test.go` — load/save tests.
- Modify `internal/theme/theme_test.go` — the mode-badge test runs for every theme.
- Create `internal/ui/themes.go` — `restyle`, `UseTheme`, `UseThemeDir`, `keepTheme`, the `themeBox` picker, `:theme`, theme completion.
- Create `internal/ui/themes_test.go` — switching, picker, command tests.
- Modify `internal/ui/app.go` — new `App` fields, `UseIcons` uses `restyle`, the picker in `modalKey`, `globalKey`, `typeIntoJob`, `render`, `statusBar`.
- Modify `internal/ui/components.go` — `tableStyles`, `styleInput`.
- Modify `internal/ui/cmdline.go` — `newCmdline` uses `styleInput`; `execute` handles `theme`.
- Modify `internal/ui/filter.go` — `newFilterInput` uses `styleInput`.
- Modify `internal/ui/complete.go` — `theme` builtin and its argument completion.
- Modify `internal/ui/actions.go` — `actionHints` shows the picker footer.
- Modify `internal/ui/help.go` — `t` in Everywhere.
- Modify `internal/ui/golden_test.go` — a `themes` snapshot.
- Modify `cmd/brewery/main.go` — load and apply the saved theme.
- Modify `README.md` — Features, Keys, Files.

---

### Task 1: The theme list

**Files:**
- Create: `internal/theme/themes.go`
- Create: `internal/theme/themes_test.go`
- Modify: `internal/theme/theme_test.go` (`TestModeColorsAreDistinct`)

**Interfaces:**
- Produces: `type Theme struct { Name, Title string; Palette Palette }`; `var Themes []Theme` (Macchiato first); `func Lookup(name string) Theme` (unknown or empty → `Themes[0]`).

- [ ] **Step 1: Create the branch**

```bash
git switch -c feat/themes
```

- [ ] **Step 2: Write the failing tests**

Create `internal/theme/themes_test.go`:

```go
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
```

In `internal/theme/theme_test.go`, replace `TestModeColorsAreDistinct` with a version that checks every theme (the five badges must stay apart in all of them):

```go
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
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `go test ./internal/theme`
Expected: build failure, `undefined: Themes` and `undefined: Lookup`.

- [ ] **Step 4: Write the theme list**

Create `internal/theme/themes.go`. Each palette is the theme's published colours. Where a theme has no colour for a slot, the slot takes the colour the theme's own terminal (ANSI) mapping uses for it, else its nearest shade: the darker backgrounds for Mantle and Crust, and the main text for Subtext0.

```go
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
```

- [ ] **Step 5: Run the tests to see them pass**

Run: `go test ./internal/theme`
Expected: `ok  	github.com/dillo/brewery/internal/theme`

- [ ] **Step 6: Commit**

```bash
git add internal/theme/themes.go internal/theme/themes_test.go internal/theme/theme_test.go
git commit -m "feat(theme): twelve dark themes by name, Macchiato first and the fallback"
```

---

### Task 2: Remembering the chosen theme

**Files:**
- Create: `internal/theme/store.go`
- Create: `internal/theme/store_test.go`

**Interfaces:**
- Consumes: `Themes`, `Lookup` from Task 1.
- Produces: `func LoadTheme(dir string) string` (always a known theme's name); `func SaveTheme(dir, name string) error`.

- [ ] **Step 1: Write the failing tests**

Create `internal/theme/store_test.go`:

```go
package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveThenLoadTheme(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "brewery") // not there yet: SaveTheme makes it
	if err := SaveTheme(dir, "nord"); err != nil {
		t.Fatal(err)
	}
	if got := LoadTheme(dir); got != "nord" {
		t.Errorf("LoadTheme = %q, want nord", got)
	}
}

func TestLoadThemeFallsBackToTheDefault(t *testing.T) {
	missing := t.TempDir()
	junk := t.TempDir()
	if err := os.WriteFile(filepath.Join(junk, "theme"), []byte("\x00\x01 not a theme\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	renamed := t.TempDir()
	if err := os.WriteFile(filepath.Join(renamed, "theme"), []byte("solarized-darker\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{"missing": missing, "junk": junk, "unknown name": renamed} {
		if got := LoadTheme(dir); got != "catppuccin-macchiato" {
			t.Errorf("%s: LoadTheme = %q, want the default", name, got)
		}
	}
}

func TestSaveThemeReportsAFailedWrite(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SaveTheme(filepath.Join(file, "brewery"), "nord"); err == nil {
		t.Error("a directory under a file can't be made, so saving should fail")
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/theme -run 'Theme$|Default|Failed'`
Expected: build failure, `undefined: SaveTheme` and `undefined: LoadTheme`.

- [ ] **Step 3: Write load and save**

Create `internal/theme/store.go`:

```go
package theme

import (
	"os"
	"path/filepath"
	"strings"
)

// themeFile holds the chosen theme's name, in brewery's state directory.
const themeFile = "theme"

// LoadTheme returns the name of the theme saved in dir. With nothing
// saved, or a name brewery doesn't know (a theme a later version renamed),
// it returns the default's.
func LoadTheme(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, themeFile)) // #nosec G304 -- brewery's own state directory
	if err != nil {
		return Themes[0].Name
	}
	return Lookup(strings.TrimSpace(string(b))).Name
}

// SaveTheme keeps name in dir for the next run, making dir if needed.
func SaveTheme(dir, name string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, themeFile), []byte(name+"\n"), 0o600)
}
```

- [ ] **Step 4: Run the tests to see them pass**

Run: `go test ./internal/theme`
Expected: `ok  	github.com/dillo/brewery/internal/theme`

- [ ] **Step 5: Lint**

Run: `golangci-lint run ./internal/theme/...`
Expected: `0 issues.` If gosec doesn't flag G304 on the `ReadFile`, drop the `#nosec` comment (an unused one is noise); run again.

- [ ] **Step 6: Commit**

```bash
git add internal/theme/store.go internal/theme/store_test.go
git commit -m "feat(theme): save the chosen theme's name, and load it back or the default"
```

---

### Task 3: Switching theme in the app

**Files:**
- Create: `internal/ui/themes.go`
- Create: `internal/ui/themes_test.go`
- Modify: `internal/ui/app.go` (`App` fields near `menu`; `New`; `UseIcons`)
- Modify: `internal/ui/components.go` (`newTable`; add `tableStyles`, `styleInput`)
- Modify: `internal/ui/cmdline.go` (`newCmdline`)
- Modify: `internal/ui/filter.go` (`newFilterInput`)

**Interfaces:**
- Consumes: `theme.Lookup`, `theme.Themes` (Task 1).
- Produces: `func (a *App) UseTheme(name string)`; `func (a *App) restyle(f func(*theme.Styles))`; `func tableStyles(st theme.Styles) table.Styles`; `func styleInput(in *textinput.Model, st theme.Styles, prompt color.Color)`; field `App.themeName string` (the theme in use).

- [ ] **Step 1: Write the failing test**

Create `internal/ui/themes_test.go`:

```go
package ui

import (
	"fmt"
	"image/color"
	"reflect"
	"strings"
	"testing"

	"github.com/dillo/brewery/internal/jobs"
	"github.com/dillo/brewery/internal/theme"
)

// code is how c appears in the escape codes the screen is drawn with,
// as a foreground (38;2;…) or a background (48;2;…).
func code(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("2;%d;%d;%d", r>>8, g>>8, b>>8)
}

// colors lists every slot of p.
func colors(p theme.Palette) []color.Color {
	v := reflect.ValueOf(p)
	cs := make([]color.Color, v.NumField())
	for i := range cs {
		cs[i] = v.Field(i).Interface().(color.Color)
	}
	return cs
}

// noMacchiato fails when the screen still uses one of Macchiato's colours.
func noMacchiato(t *testing.T, where, screen string) {
	t.Helper()
	for _, c := range colors(theme.Macchiato) {
		if strings.Contains(screen, code(c)) {
			t.Errorf("%s: still drawn in Macchiato's %s", where, code(c))
		}
	}
}

func TestUseThemeReachesEveryPart(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseHistory(jobs.History{Dir: t.TempDir()}, recentJobs(a.now())) // History's rows are coloured when set
	a.UseTheme("dracula")
	if a.themeName != "dracula" {
		t.Fatalf("themeName = %q", a.themeName)
	}
	dracula := theme.Lookup("dracula").Palette
	for _, label := range []string{"Home", "Top-level", "Outdated", "Services", "History", "Maintenance"} {
		show(t, a, label)
		screen := a.render()
		noMacchiato(t, label, screen)
		if !strings.Contains(screen, code(dracula.Mauve)) {
			t.Errorf("%s: the focused pane should be in Dracula's accent", label)
		}
	}
	keys(t, a, ":")
	screen := a.render()
	noMacchiato(t, "the : prompt", screen)
	if !strings.Contains(screen, code(dracula.Blue)) {
		t.Error("the : prompt should be in Dracula's blue")
	}
	keys(t, a, "esc", "/")
	noMacchiato(t, "the / prompt", a.render())
}

func TestUseThemeUnknownIsTheDefault(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseTheme("dracula")
	a.UseTheme("nope")
	if a.themeName != "catppuccin-macchiato" {
		t.Errorf("themeName = %q, want the default", a.themeName)
	}
}

func TestUseThemeKeepsIcons(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseIcons(true)
	a.UseTheme("nord")
	if a.st.Icons != theme.Nerd || a.installed.st.Icons != theme.Nerd {
		t.Error("switching theme should keep the Nerd Font icons")
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/ui -run UseTheme`
Expected: build failure, `a.UseTheme undefined` and `a.themeName undefined`.

- [ ] **Step 3: Pull out the styles baked in at construction**

In `internal/ui/components.go`, change `newTable`'s last option to use a new function, and add `styleInput` below `newTable`. Add imports `"image/color"` and `"charm.land/bubbles/v2/textinput"` (and `"charm.land/lipgloss/v2"` if it's not already imported there).

```go
		table.WithStyles(tableStyles(st)),
	)
}

// tableStyles are the styles every table is drawn with.
func tableStyles(st theme.Styles) table.Styles {
	return table.Styles{Header: st.TableHeader, Cell: st.TableCell, Selected: st.TableSelected}
}

// styleInput colours a prompt: its first character in prompt, what's typed
// in the text colour, and a cursor that doesn't blink.
func styleInput(in *textinput.Model, st theme.Styles, prompt color.Color) {
	s := in.Styles()
	s.Cursor.Blink = false
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(prompt).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(st.P.Text)
	in.SetStyles(s)
}
```

In `internal/ui/cmdline.go`, `newCmdline` becomes:

```go
func newCmdline(st theme.Styles) cmdline {
	in := textinput.New()
	in.Prompt = ":"
	styleInput(&in, st, st.P.Blue)
	return cmdline{input: in}
}
```

In `internal/ui/filter.go`, `newFilterInput` becomes:

```go
func newFilterInput(st theme.Styles) textinput.Model {
	in := textinput.New()
	in.Prompt = "/"
	styleInput(&in, st, st.P.Teal)
	return in
}
```

Remove imports these files no longer use (`go build ./...` names them).

- [ ] **Step 4: Add the App fields**

In `internal/ui/app.go`, add below `menu *menuBox` (the picker's own field comes in Task 4, with its type):

```go
	themeName  string          // the theme in use
	themeDir   string          // where the chosen theme is saved; "" saves nothing
```

In `New`, add `themeName: theme.Themes[0].Name,` to the `a := &App{…}` literal (next to `tab: tabHome`).

Replace `UseIcons`'s body loop with `restyle`:

```go
func (a *App) UseIcons(on bool) {
	ic := theme.Icons{}
	if on {
		ic = theme.Nerd
	}
	a.restyle(func(st *theme.Styles) { st.Icons = ic })
	a.sync()
}
```

- [ ] **Step 5: Write UseTheme**

Create `internal/ui/themes.go`:

```go
package ui

import (
	"charm.land/bubbles/v2/table"

	"github.com/dillo/brewery/internal/theme"
)

// restyle changes every part's copy of the styles with f. Each part keeps
// its own copy, so changing a.st alone would show in only some places.
func (a *App) restyle(f func(*theme.Styles)) {
	for _, st := range []*theme.Styles{&a.st, &a.installed.st, &a.outdated.st, &a.services.st, &a.taps.st, &a.jobs.st, &a.maint.st, &a.home.st, &a.drawer.st, &a.discover.st} {
		f(st)
	}
}

// UseTheme switches to the theme called name, or to the default when
// brewery has none by that name. It isn't saved: keepTheme does that.
func (a *App) UseTheme(name string) {
	t := theme.Lookup(name)
	st := theme.New(t.Palette)
	st.Icons = a.st.Icons
	a.restyle(func(s *theme.Styles) { *s = st })
	a.themeName = t.Name
	// Styles set when a part was built, and rows coloured when they were set.
	for _, tb := range []*table.Model{&a.installed.table, &a.outdated.table, &a.services.table, &a.taps.table, &a.jobs.table, &a.maint.table, &a.discover.table} {
		tb.SetStyles(tableStyles(st))
	}
	styleInput(&a.cmd.input, st, st.P.Blue)
	styleInput(&a.filterIn, st, st.P.Teal)
	a.jobs.setRows(jobRows(st, a.history, a.now()))
	a.discover.apply()
	a.sync()
	a.syncMaint()
}
```

- [ ] **Step 6: Run the tests to see them pass**

Run: `go test ./internal/ui -run 'UseTheme|Icons'`
Expected: `ok`. If `TestUseThemeReachesEveryPart` names a screen still drawn in a Macchiato colour, find the part that keeps it (`grep -n 'theme.Styles' internal/ui/*.go` lists every holder) and add it to `restyle`, or its baked-in style to `UseTheme`; don't loosen the test.

- [ ] **Step 7: Run the whole suite**

Run: `go test ./...`
Expected: every package `ok`; no golden changes (the default is still Macchiato and goldens are compared without colours).

- [ ] **Step 8: Commit**

```bash
git add internal/ui/themes.go internal/ui/themes_test.go internal/ui/app.go internal/ui/components.go internal/ui/cmdline.go internal/ui/filter.go
git commit -m "feat(ui): UseTheme restyles every part, its tables, prompts and coloured rows"
```

---

### Task 4: The theme picker

**Files:**
- Modify: `internal/ui/themes.go` (add `themeBox`, `openThemes`, `themesKey`, `keepTheme`, `UseThemeDir`)
- Modify: `internal/ui/themes_test.go`
- Modify: `internal/ui/app.go` (`modalKey`, `globalKey`, `typeIntoJob`, `render`, `statusBar`)
- Modify: `internal/ui/actions.go` (`actionHints`)
- Modify: `internal/ui/help.go` (Everywhere group)
- Modify: `internal/ui/golden_test.go` (`TestScreens`)
- Create: `internal/ui/testdata/golden/themes.txt` (written by `-update`)

**Interfaces:**
- Consumes: `UseTheme`, `themeName`, `themeDir` (Task 3); `theme.SaveTheme`, `theme.LoadTheme` (Task 2).
- Produces: `type themeBox struct { cursor int; was string; gPending bool }`; `func (b *themeBox) key(k string) (done, keep bool)`; `func (b *themeBox) view(st theme.Styles, maxW int) string`; `const themesFooter`; `func (a *App) openThemes()`; `func (a *App) keepTheme(name string) tea.Cmd`; `func (a *App) UseThemeDir(dir string)`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/themes_test.go` (add imports `"os"`, `"path/filepath"`):

```go
func TestThemeBoxKeys(t *testing.T) {
	n := len(theme.Themes)
	b := &themeBox{}
	for _, tt := range []struct {
		keys       []string
		cursor     int
		done, keep bool
	}{
		{[]string{"k"}, n - 1, false, false}, // wraps to the last
		{[]string{"j"}, 0, false, false},     // and back round
		{[]string{"down", "down", "up"}, 1, false, false},
		{[]string{"G"}, n - 1, false, false},
		{[]string{"g", "g"}, 0, false, false},
		{[]string{"j", "enter"}, 1, true, true},
		{[]string{"esc"}, 1, true, false},
		{[]string{"q"}, 1, true, false},
		{[]string{"g", "esc"}, 1, true, false},
	} {
		var done, keep bool
		for _, k := range tt.keys {
			done, keep = b.key(k)
		}
		if b.cursor != tt.cursor || done != tt.done || keep != tt.keep {
			t.Errorf("%v: cursor %d, done %v, keep %v; want %d, %v, %v", tt.keys, b.cursor, done, keep, tt.cursor, tt.done, tt.keep)
		}
	}
}

// pickerApp is an app that saves its theme to a temp directory.
func pickerApp(t *testing.T) (*App, string) {
	t.Helper()
	a := newJobApp(t, &fakeStarter{})
	dir := t.TempDir()
	a.UseThemeDir(dir)
	show(t, a, "Home")
	return a, dir
}

// saved reports whether a theme file was written to dir.
func saved(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "theme"))
	return err == nil
}

func TestPickerPreviewsAndCancels(t *testing.T) {
	a, dir := pickerApp(t)
	keys(t, a, "t")
	if a.themes == nil || a.themes.cursor != 0 || a.themes.was != "catppuccin-macchiato" {
		t.Fatalf("t opens the picker on the theme in use: %+v", a.themes)
	}
	keys(t, a, "j")
	if a.themeName != theme.Themes[1].Name {
		t.Errorf("j previews the next theme: in use %q", a.themeName)
	}
	if !strings.Contains(a.render(), "j/k preview · enter keep · esc cancel") {
		t.Error("the picker's footer should show")
	}
	keys(t, a, "esc")
	if a.themes != nil || a.themeName != "catppuccin-macchiato" {
		t.Errorf("esc closes the picker and puts the old theme back: in use %q", a.themeName)
	}
	if saved(dir) {
		t.Error("previewing and cancelling mustn't save anything")
	}
}

func TestPickerKeepsAndSaves(t *testing.T) {
	a, dir := pickerApp(t)
	keys(t, a, "t", "j", "j", "enter")
	want := theme.Themes[2]
	if a.themes != nil || a.themeName != want.Name {
		t.Errorf("enter keeps the theme under the cursor: in use %q", a.themeName)
	}
	if got := theme.LoadTheme(dir); got != want.Name {
		t.Errorf("saved %q, want %q", got, want.Name)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "theme: "+want.Title) {
		t.Errorf("the status bar should say which theme: %q", stripANSI(a.statusBar()))
	}
	keys(t, a, "t")
	if a.themes.cursor != 2 || a.themes.was != want.Name {
		t.Errorf("the picker opens on the kept theme: %+v", a.themes)
	}
}

func TestPickerSaveFailureStillApplies(t *testing.T) {
	a, _ := pickerApp(t)
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	a.UseThemeDir(filepath.Join(file, "brewery")) // can't make a dir under a file
	keys(t, a, "t", "j", "enter")
	if a.themeName != theme.Themes[1].Name {
		t.Errorf("the theme applies even when it can't be saved: in use %q", a.themeName)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "couldn't save") {
		t.Errorf("the status bar should say it couldn't be saved: %q", stripANSI(a.statusBar()))
	}
}

func TestTypingIntoAJobCancelsThePicker(t *testing.T) {
	a, _ := pickerApp(t)
	keys(t, a, "t", "j")
	a.typeIntoJob()
	if a.themes != nil || a.themeName != "catppuccin-macchiato" {
		t.Errorf("brew asking for input closes the picker and puts the old theme back: in use %q", a.themeName)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/ui -run 'ThemeBox|Picker|TypingInto'`
Expected: build failure, `undefined: themeBox` and `a.UseThemeDir undefined`.

- [ ] **Step 3: Write the picker**

In `internal/ui/app.go`, add above `themeName string`:

```go
	themes     *themeBox       // open theme picker, or nil
```

Add to `internal/ui/themes.go`, with imports `"strings"`, `tea "charm.land/bubbletea/v2"`, `"charm.land/lipgloss/v2"`, `"github.com/charmbracelet/x/ansi"`:

```go
// themeBox is the theme picker. The theme under the cursor is in use
// while it's open: enter keeps it, esc puts back the one from before.
type themeBox struct {
	cursor   int    // index into theme.Themes
	was      string // the theme in use when the picker opened
	gPending bool   // g was pressed: g next goes to the top
}

// themesFooter says how to use the picker.
const themesFooter = "j/k preview · enter keep · esc cancel"

// key handles a key press while the picker is open. done is true when it
// should close, and keep then says whether the theme under the cursor
// stays. Keys it doesn't use do nothing.
func (b *themeBox) key(k string) (done, keep bool) {
	n := len(theme.Themes)
	if b.gPending {
		b.gPending = false
		if k == "g" {
			b.cursor = 0
			return false, false
		}
	}
	switch k {
	case "esc", "q":
		return true, false
	case "enter":
		return true, true
	case "j", "down":
		b.cursor = (b.cursor + 1) % n
	case "k", "up":
		b.cursor = (b.cursor + n - 1) % n
	case "G", "end":
		b.cursor = n - 1
	case "home":
		b.cursor = 0
	case "g":
		b.gPending = true
	}
	return false, false
}

// view renders the picker, at most maxW columns wide: each theme by its
// title, a ✓ on the one in use when it opened, and how to use it.
func (b *themeBox) view(st theme.Styles, maxW int) string {
	inner := min(lipgloss.Width(themesFooter), max(maxW-4, 10))
	lines := []string{st.Title.Render("Theme"), ""}
	for i, t := range theme.Themes {
		label := t.Title
		if t.Name == b.was {
			label += " ✓"
		}
		mark := "  "
		if i == b.cursor {
			mark, label = st.Accent.Render("› "), st.Accent.Bold(true).Render(label)
		}
		lines = append(lines, ansi.Truncate(mark+label, inner, "…"))
	}
	lines = append(lines, "", st.Muted.Render(ansi.Truncate(themesFooter, inner, "…")))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(st.P.Mauve).
		Padding(0, 1).Render(strings.Join(lines, "\n"))
}

// UseThemeDir sets where the chosen theme is saved. Without it, a theme
// lasts until brewery quits.
func (a *App) UseThemeDir(dir string) { a.themeDir = dir }

// openThemes opens the picker on the theme in use.
func (a *App) openThemes() {
	b := &themeBox{was: a.themeName}
	for i, t := range theme.Themes {
		if t.Name == a.themeName {
			b.cursor = i
		}
	}
	a.themes = b
}

// themesKey handles a key while the picker is open: a move switches to the
// theme under the cursor, enter keeps it, esc puts the old one back.
func (a *App) themesKey(k string) tea.Cmd {
	b := a.themes
	before := b.cursor
	done, keep := b.key(k)
	switch {
	case done && keep:
		a.themes = nil
		return a.keepTheme(theme.Themes[b.cursor].Name)
	case done:
		a.cancelThemes()
	case b.cursor != before:
		a.UseTheme(theme.Themes[b.cursor].Name)
	}
	return nil
}

// cancelThemes closes the picker and puts back the theme in use when it
// opened.
func (a *App) cancelThemes() {
	if a.themes != nil {
		a.UseTheme(a.themes.was)
		a.themes = nil
	}
}

// keepTheme switches to the theme called name and saves it for next time.
// A theme that can't be saved still applies until brewery quits.
func (a *App) keepTheme(name string) tea.Cmd {
	a.UseTheme(name)
	t := theme.Lookup(name)
	if a.themeDir != "" {
		if err := theme.SaveTheme(a.themeDir, t.Name); err != nil {
			a.flash = a.st.Err.Render("✗ theme: " + t.Title + " for now; couldn't save it: " + err.Error())
			return nil
		}
	}
	a.flash = a.st.OK.Render("theme: " + t.Title)
	return nil
}
```

- [ ] **Step 4: Wire the picker into the app**

In `internal/ui/app.go`:

`modalKey`, add a case right after the `case a.menu != nil:` block:

```go
	case a.themes != nil:
		return a.themesKey(k), true
```

`globalKey`, add a case after `case "?":` … (before `case "y":`):

```go
	case "t":
		a.openThemes()
```

`typeIntoJob` becomes (brew's question must get the keys, so the picker closes as the menu does, putting the old theme back):

```go
func (a *App) typeIntoJob() {
	a.cancelThemes()
	a.mode, a.drawer.open, a.gPending, a.help, a.menu = modeInsert, true, false, false, nil
}
```

Update its doc comment's last sentence to: `Help, the action menu and the theme picker close, so they can't eat the first key of the answer.`

`render`: the tiny-window condition becomes

```go
	if bodyH == 0 && (a.confirm != nil || a.menu != nil || a.themes != nil || a.help) {
```

and after the `if a.menu != nil { … }` overlay add:

```go
	if a.themes != nil {
		body = overlay(body, a.themes.view(a.st, a.width), a.width, bodyH)
	}
```

`statusBar`: `if a.menu == nil { // the menu's footer stands alone` becomes

```go
	if a.menu == nil && a.themes == nil { // the menu's and picker's footers stand alone
```

In `internal/ui/actions.go`, `actionHints` starts:

```go
func (a *App) actionHints() string {
	if a.themes != nil {
		return themesFooter
	}
	if a.menu != nil {
		return menuFooter
	}
```

In `internal/ui/help.go`, the Everywhere group gets `{"t", "change the theme"}` after `{":", "run any brew command"}`:

```go
		{"[ ]", "previous or next entry"}, {":", "run any brew command"}, {"t", "change the theme"}, {"r", "refresh"}, {"U", "brew update"},
```

- [ ] **Step 5: Run the picker tests to see them pass**

Run: `go test ./internal/ui -run 'ThemeBox|Picker|TypingInto|UseTheme'`
Expected: `ok`

- [ ] **Step 6: Add the snapshot**

In `internal/ui/golden_test.go` `TestScreens`, add after the `{"help", …}` entry:

```go
		{"themes", []string{"Home"}, []string{"t"}},
```

and in the loop, after the `if a.help { … }` block:

```go
		if a.themes != nil {
			keys(t, a, "esc") // close it: the screens after are in the default theme
		}
```

Run: `go test ./internal/ui -run TestScreens -update && git diff --stat internal/ui/testdata/golden && git status --short internal/ui/testdata/golden`
Expected: `themes.txt` is new; `help.txt` changes by the one `t  change the theme` line (and whatever that line pushes down); nothing else changes. Open `internal/ui/testdata/golden/themes.txt` and check the box is centred over Home, titled `Theme`, lists the twelve titles with `› Catppuccin Macchiato ✓` first, and ends with the footer.

- [ ] **Step 7: Run the whole suite and lint**

Run: `go test ./... && golangci-lint run ./...`
Expected: every package `ok`; `0 issues.` If a help test (`internal/ui/help_test.go`) fails only because the Everywhere group now has the `t` row, update that test's expectation to include `t  change the theme`; nothing else should fail.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/themes.go internal/ui/themes_test.go internal/ui/app.go internal/ui/actions.go internal/ui/help.go internal/ui/golden_test.go internal/ui/testdata/golden/
git commit -m "feat(ui): t opens a theme picker that previews as it moves; enter keeps and saves"
```

---

### Task 5: `:theme`

**Files:**
- Modify: `internal/ui/themes.go` (add `themeCommand`, `themeCandidates`)
- Modify: `internal/ui/cmdline.go` (`execute`)
- Modify: `internal/ui/complete.go` (`builtins`, `everyday`, `candidates`)
- Modify: `internal/ui/themes_test.go`

**Interfaces:**
- Consumes: `openThemes`, `keepTheme` (Task 4).
- Produces: `func (a *App) themeCommand(args []string) tea.Cmd`; `func themeCandidates() []candidate`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/themes_test.go`:

```go
func TestThemeCommand(t *testing.T) {
	a, dir := pickerApp(t)
	keys(t, a, ":")
	typeText(t, a, "theme dracula")
	keys(t, a, "enter")
	if a.themeName != "dracula" || theme.LoadTheme(dir) != "dracula" {
		t.Errorf(":theme dracula switches and saves: in use %q, saved %q", a.themeName, theme.LoadTheme(dir))
	}
	if a.themes != nil {
		t.Error(":theme with a name doesn't open the picker")
	}

	keys(t, a, ":")
	typeText(t, a, "theme nope")
	keys(t, a, "enter")
	if a.themeName != "dracula" {
		t.Errorf("an unknown name changes nothing: in use %q", a.themeName)
	}
	if !strings.Contains(stripANSI(a.statusBar()), "no theme called nope") {
		t.Errorf("an unknown name says so: %q", stripANSI(a.statusBar()))
	}

	keys(t, a, ":")
	typeText(t, a, "theme")
	keys(t, a, "enter")
	if a.themes == nil || a.themes.was != "dracula" {
		t.Errorf(":theme alone opens the picker on the theme in use: %+v", a.themes)
	}
}

func TestThemeCompletion(t *testing.T) {
	a := newCompletingApp(t, &fakeStarter{})
	for _, tt := range []struct{ line, word, want string }{
		{"th", "th", "theme"},
		{"theme d", "d", "dracula"},
		{"theme catppuccin-m", "catppuccin-m", "catppuccin-macchiato catppuccin-mocha"},
		{"theme nord ", "", ""}, // one name only
	} {
		word, cs := a.candidates(tt.line)
		if word != tt.word || candidateTexts(cs) != tt.want {
			t.Errorf("candidates(%q) = %q, %q; want %q, %q", tt.line, word, candidateTexts(cs), tt.word, tt.want)
		}
	}
	if _, cs := a.candidates("theme d"); cs[0].desc != "Dracula" {
		t.Errorf("a theme's title comes along as its description: %q", cs[0].desc)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/ui -run 'ThemeCommand|ThemeCompletion'`
Expected: FAIL. `:theme dracula` is run as `brew theme dracula` (the fake starter records it) so `themeName` stays `catppuccin-macchiato`; `candidates("th")` is `""`.

- [ ] **Step 3: Write `:theme`**

Add to `internal/ui/themes.go`:

```go
// themeCommand is :theme. With no name it opens the picker; with one it
// switches to that theme and saves it.
func (a *App) themeCommand(args []string) tea.Cmd {
	if len(args) == 0 {
		a.openThemes()
		return nil
	}
	for _, t := range theme.Themes {
		if t.Name == args[0] {
			return a.keepTheme(t.Name)
		}
	}
	return a.note("no theme called " + args[0] + " · :theme lists them")
}

// themeCandidates are the names :theme takes, each with its title.
func themeCandidates() []candidate {
	cs := make([]candidate, len(theme.Themes))
	for i, t := range theme.Themes {
		cs[i] = candidate{t.Name, t.Title}
	}
	return cs
}
```

In `internal/ui/cmdline.go` `execute`, add to the `switch args[0]` with `q` and `jobs`:

```go
	case "theme":
		return a.themeCommand(args[1:])
```

In `internal/ui/complete.go`:

```go
var builtins = []candidate{
	{"jobs", "Show History: finished jobs and their output"},
	{"theme", "Change brewery's colours"},
}
```

add `"theme": true,` to `everyday` (keep its alphabetical order: after `"tap": true,`), and in `candidates`, right after the `if len(words) == 0 { … }` block:

```go
	if words[0] == "theme" {
		if len(words) > 1 {
			return word, nil // it takes one name
		}
		return word, matching(themeCandidates(), word)
	}
```

- [ ] **Step 4: Run the tests to see them pass**

Run: `go test ./internal/ui -run 'ThemeCommand|ThemeCompletion|Candidates'`
Expected: `ok`

- [ ] **Step 5: Run the whole suite and lint**

Run: `go test ./... && golangci-lint run ./...`
Expected: every package `ok`; `0 issues.`

- [ ] **Step 6: Commit**

```bash
git add internal/ui/themes.go internal/ui/themes_test.go internal/ui/cmdline.go internal/ui/complete.go
git commit -m "feat(ui): :theme opens the picker, :theme <name> switches and saves, with completion"
```

---

### Task 6: Load the saved theme at startup, and document it

**Files:**
- Modify: `cmd/brewery/main.go` (`newApp`)
- Modify: `README.md` (Features › Getting around; Keys › Everywhere; Files)

**Interfaces:**
- Consumes: `theme.LoadTheme` (Task 2); `App.UseThemeDir`, `App.UseTheme` (Tasks 3–4); `jobs.DefaultStateDir()` (existing).

- [ ] **Step 1: Apply the saved theme in `newApp`**

In `cmd/brewery/main.go`, add the import `"github.com/dillo/brewery/internal/theme"`, and in `newApp` right after `app.UseIcons(…)`:

```go
	// The theme the user last kept; Macchiato when there's none.
	stateDir := jobs.DefaultStateDir()
	app.UseThemeDir(stateDir)
	app.UseTheme(theme.LoadTheme(stateDir))
```

- [ ] **Step 2: Build and run the tests**

Run: `go build ./... && go test ./...`
Expected: builds; every package `ok`.

- [ ] **Step 3: Check it by hand**

Run with a throwaway state directory so your real one isn't touched:

```bash
make build && XDG_STATE_HOME="$(mktemp -d)" sh -c './brewery; cat "$XDG_STATE_HOME/brewery/theme"'
```

In brewery: press `t`, move with `j` and watch the whole screen change, press `enter` on Nord, quit with `q`. Expected: the `cat` prints `nord`. Run the same command line again but keep the directory (`d=$(mktemp -d); XDG_STATE_HOME=$d ./brewery` twice): the second start opens in Nord.

- [ ] **Step 4: Update the README**

Under **Features › Getting around**, add a bullet after the Vim-style keys bullet:

```markdown
- Twelve dark themes, from Catppuccin to Gruvbox, Nord and Dracula. `t`
  previews them live; brewery remembers the one you keep.
```

In **Keys › Everywhere**, add a row after the `:` row:

```markdown
| `t` | change the theme; `:theme <name>` switches straight to one |
```

In **Files**, the state directory's line becomes:

```markdown
- `$XDG_STATE_HOME/brewery` (default `~/.local/state/brewery`): the history
  of jobs brewery ran, the theme you chose, and crash logs.
```

- [ ] **Step 5: Lint and commit**

Run: `golangci-lint run ./...`
Expected: `0 issues.`

```bash
git add cmd/brewery/main.go README.md
git commit -m "feat: brewery starts in the theme you last kept; README covers themes"
```
