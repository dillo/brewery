# Sidebar Jump Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `/` in the sidebar opens a `JUMP` prompt. Typing moves the selection to the first entry whose label contains the text, `enter` opens that entry's list, and `esc` goes back to where the jump started.

**Architecture:** the jump reuses the list filter's mode (`modeFilter`) and its text input (`a.filterIn`), with a `jumping` flag on `App`. Because of that, paste and brew's held-back prompts already behave as they do for the filter. `filterKey` hands keys to `jumpKey` while jumping, and the status bar swaps the badge and hint.

**Tech Stack:** Go 1.26, Bubble Tea v2, golangci-lint v2.15.0.

**Spec:** `docs/superpowers/specs/2026-10-19-brewery-sidebar-jump-design.md`

## Global Constraints

- **Lint:** `.golangci.yml` is fixed (gocognit and gocyclo `min-complexity: 20`, tests excluded). Don't edit it, and don't add `//nolint`. `golangci-lint run ./...` must report `0 issues.`
- **Coverage:** `make cover` stays at 90% or above in total and per package.
- **Goldens:** none should change. If one does, read `git diff internal/ui/testdata/golden` first: only the sidebar hint may change.
- **Style:** match the surrounding code's comment density, naming and idioms. Every function gets a short doc comment.
- **Exact texts:** user-facing strings are exactly as written here: `JUMP`, `enter open · esc back`, `no match · esc back`, `jump to an entry`, `j/k choose · / jump · l open · 1-5 sections`.
- **Commits:** one commit, with the message given at the end of the task.

---

### Task 1: Jump to a sidebar entry with `/`

**Files:**
- Create: `internal/ui/jump.go`
- Modify: `internal/ui/app.go`:
  - add `jumping bool`, `jumpFrom int` and `jumpMiss bool` to `App`, next to `filterIn`;
  - `statusBar`'s `modeFilter` case.
- Modify: `internal/ui/filter.go` (`filterKey` hands keys to `jumpKey` while jumping)
- Modify: `internal/ui/sidebar.go` (`sidebarKey`'s `/` and `l` cases)
- Modify: `internal/ui/help.go` (the Sidebar group)
- Modify: `internal/ui/actions.go` (`actionHints`' sidebar hint)
- Test: `internal/ui/sidebar_test.go`:
  - replace `TestSlashFromTheSidebarOnHomeKeepsTheFocus` and `TestSlashFromTheSidebarFiltersTheList`, whose behaviour this changes;
  - add the tests below.

- [ ] **Step 1: Write the failing tests.**
  - In `internal/ui/sidebar_test.go`, delete `TestSlashFromTheSidebarOnHomeKeepsTheFocus` and `TestSlashFromTheSidebarFiltersTheList`, then add the tests below.
  - `typeText` (cmdline_test.go) and `label` already exist in the package.
  - Add `tea "charm.land/bubbletea/v2"` to the imports if the file lacks it.

```go
func TestSlashInTheSidebarJumps(t *testing.T) {
	a := newJobApp(t, &fakeStarter{}) // the focus starts on the sidebar, on Home
	keys(t, a, "/")
	if a.mode != modeFilter || !a.jumping || a.focus != colSidebar {
		t.Fatalf("/ in the sidebar should open the jump: mode %d, jumping %v, focus %v", a.mode, a.jumping, a.focus)
	}
	typeText(t, a, "maint")
	if label(a) != "Maintenance" {
		t.Errorf("/maint selects %q, want Maintenance", label(a))
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "JUMP") || !strings.Contains(bar, "enter open · esc back") {
		t.Errorf("status bar while jumping: %q", bar)
	}
	keys(t, a, "enter")
	if a.mode != modeNormal || a.jumping || a.focus != colList || label(a) != "Maintenance" {
		t.Errorf("enter opens the entry: mode %d, jumping %v, focus %v, entry %q", a.mode, a.jumping, a.focus, label(a))
	}
}

func TestJumpMatchesTheFirstEntryIgnoringCase(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "/")
	typeText(t, a, "S")
	if label(a) != "Casks" {
		t.Errorf("/S selects %q, want Casks, the first label with an s", label(a))
	}
	typeText(t, a, "zz")
	if label(a) != "Casks" || !strings.Contains(stripANSI(a.statusBar()), "no match · esc back") {
		t.Errorf("no match keeps the last match and says so: %q, %q", label(a), stripANSI(a.statusBar()))
	}
	keys(t, a, "enter")
	if a.mode != modeNormal || a.focus != colSidebar {
		t.Errorf("enter with no match only closes the prompt: mode %d, focus %v", a.mode, a.focus)
	}
}

func TestJumpGoesBack(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "j", "j") // Top-level
	keys(t, a, "/")
	typeText(t, a, "hist")
	keys(t, a, "esc")
	if label(a) != "Top-level" || a.mode != modeNormal || a.jumping || a.focus != colSidebar {
		t.Errorf("esc goes back to where the jump started: %q, mode %d, focus %v", label(a), a.mode, a.focus)
	}
	keys(t, a, "/")
	typeText(t, a, "taps")
	keys(t, a, "backspace", "backspace", "backspace", "backspace")
	if label(a) != "Top-level" {
		t.Errorf("clearing the text goes back: %q", label(a))
	}
	keys(t, a, "esc")
}

func TestJumpTakesAPaste(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "/")
	_, cmd := a.Update(tea.PasteMsg{Content: "history"})
	settle(t, a, cmd)
	if label(a) != "History" {
		t.Errorf("a pasted jump selects %q, want History", label(a))
	}
}

func TestSidebarHelpAndHintMentionTheJump(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "j/k choose · / jump · l open · 1-5 sections") {
		t.Errorf("sidebar hint: %q", bar)
	}
	keys(t, a, "?")
	if out := stripANSI(a.render()); !strings.Contains(out, "jump to an entry") {
		t.Errorf("help lacks the jump:\n%s", out)
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/ui -run 'Jump|SlashInTheSidebar' -count=1`. Expected: it fails to compile (`a.jumping` is undefined).

- [ ] **Step 3: Add the App fields.** In `internal/ui/app.go`, after `filterIn`:

```go
	jumping    bool            // the "/" prompt is the sidebar's jump, not a list's filter
	jumpFrom   int             // the entry the jump started on
	jumpMiss   bool            // the jump's text matches no entry
```

(Let gofmt align them with their neighbours.)

- [ ] **Step 4: Write `internal/ui/jump.go`:**

```go
package ui

import tea "charm.land/bubbletea/v2"

// openJump opens the sidebar's "/" prompt: as the user types, the
// selection moves to the first entry whose label contains the text.
func (a *App) openJump() tea.Cmd {
	a.mode, a.jumping, a.jumpFrom, a.jumpMiss = modeFilter, true, a.sel, false
	a.filterIn.SetValue("")
	a.filterIn.Focus()
	return nil
}

// jumpKey handles a key while jumping: the selection follows the text,
// enter opens the entry, and esc goes back to where the jump started.
func (a *App) jumpKey(msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "enter":
			a.mode, a.jumping = modeNormal, false
			if !a.jumpMiss {
				a.openEntry()
			}
			return nil
		case "esc":
			a.mode, a.jumping = modeNormal, false
			return a.selectEntry(a.jumpFrom)
		}
	}
	var cmd tea.Cmd
	a.filterIn, cmd = a.filterIn.Update(msg)
	text := a.filterIn.Value()
	if text == "" {
		a.jumpMiss = false
		return tea.Batch(cmd, a.selectEntry(a.jumpFrom))
	}
	i, ok := jumpMatch(text)
	a.jumpMiss = !ok
	if !ok {
		return cmd // the selection stays on the last match
	}
	return tea.Batch(cmd, a.selectEntry(i))
}

// jumpMatch is the first entry, from the top, whose label contains text,
// ignoring case.
func jumpMatch(text string) (int, bool) {
	for i, e := range entries {
		if matches(e.label, text) {
			return i, true
		}
	}
	return 0, false
}
```

- [ ] **Step 5: Route the keys.**

**`internal/ui/filter.go`:** make `filterKey` start with the jump check, before it reads `filterTarget`:

```go
func (a *App) filterKey(msg tea.Msg) tea.Cmd {
	if a.jumping {
		return a.jumpKey(msg)
	}
	t := a.filterTarget()
```

Update its doc comment to add: "While jumping, the keys are the sidebar's jump's."

**`internal/ui/sidebar.go`:**
- In `sidebarKey`, replace the `"/"` case with:

```go
	case "/":
		return a.openJump(), true
```

- Replace the `"l", "right", "enter", "tab"` case's body with `a.openEntry(); return nil, true`, and add this helper below `sidebarKey`:

```go
// openEntry moves the focus from the sidebar into the selected entry's
// list. Home starts on its first tile, whichever had the focus before.
func (a *App) openEntry() {
	a.focus = colList
	if a.tab == tabHome {
		a.home.enter()
	}
}
```

**`internal/ui/app.go`, `statusBar`:** replace the `modeFilter` case with:

```go
	case modeFilter:
		badge, hint := "FILTER", "enter keep · esc clear"
		if a.jumping {
			badge, hint = "JUMP", "enter open · esc back"
			if a.jumpMiss {
				hint = "no match · esc back"
			}
		}
		return a.promptBar(badge, hint, func(w int) string {
			a.filterIn.SetWidth(max(w-2, 1)) // the prompt and the cursor cell
			return a.filterIn.View()
		})
```

If `statusBar` goes over a complexity limit, move this case into a helper `filterBar() string`.

- [ ] **Step 6: Help and hint.**
  - **`internal/ui/help.go`:** set the Sidebar group to
    `helpGroup{"Sidebar", [][2]string{{"j k", "choose"}, {"gg G", "first, last"}, {"/", "jump to an entry"}, {"l enter", "go to the list"}}}`.
  - **`internal/ui/actions.go`, `actionHints`:** change the sidebar's hint to `"j/k choose · / jump · l open · 1-5 sections"`.

- [ ] **Step 7: Run the tests.**
  - `go test ./internal/ui -run 'Jump|SlashInTheSidebar' -count=1`. Expected: PASS.
  - Then `go test ./internal/ui -count=1`. A test that asserted the old sidebar hint text needs the new text. A test that used `/` from the sidebar to filter a list should press `l` first, which keeps its intent.
  - Check the goldens with `go test ./internal/ui -run TestScreens -count=1`. If the sidebar hint shows on a golden, update it with `-update` and confirm the diff is only that hint.

- [ ] **Step 8: Check.** `go test ./... -count=1`, `golangci-lint run ./...` (`0 issues.`), `make cover`.

- [ ] **Step 9: Commit.** `git add -A internal && git commit -m "feat(ui): / in the sidebar jumps to an entry"`
