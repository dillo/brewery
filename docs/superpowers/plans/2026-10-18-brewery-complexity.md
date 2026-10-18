# Complexity Limits Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the 13 functions over the new complexity limits (gocognit and gocyclo at 20, tests excluded) under them, without changing behaviour.

**Architecture:** Behaviour-preserving refactors. Each over-limit function gets small, named helpers that each do one job. A helper is named for what it does, and has a one-line comment if the surrounding code comments its functions. No new features, and no changes to what users see.

**Tech Stack:** Go 1.26, Bubble Tea v2, golangci-lint v2.15.0.

## Global Constraints

- **Lint:**
  - `.golangci.yml` is fixed at commit 25d9530: gocognit and gocyclo `min-complexity: 20`, with both excluded for `_test.go`. Don't edit it.
  - Don't add `//nolint`.
- **Behaviour:**
  - Behaviour doesn't change, and every existing test passes unmodified.
  - The goldens in `internal/ui/testdata/golden/` must not change. Never run `-update`.
  - New tests are welcome where a new helper isn't already covered.
- **Coverage:** `make cover` stays at 90% or above in total and per package.
- **Style:** match the surrounding code: its comment density, naming and idioms. Comments are short, plain sentences. Prefer a helper that reads well over a clever table.
- **Scope:**
  - Don't move code between files unless the helper plainly belongs in another one.
  - Don't refactor functions that aren't on your task's list.
- **Commits:** one commit per task, conventional style (`refactor(ui): …`).

## Commands

| Check | Command |
|---|---|
| Lint, one file's functions | `golangci-lint run ./... 2>&1 \| grep -E 'gocognit\|gocyclo'` |
| Tests | `go test ./... -count=1` |
| Goldens unchanged | `go test ./internal/ui -run TestScreens -count=1` |
| Coverage | `make cover` |

At the end of each task, none of that task's functions appears in the lint output. Other tasks' functions may still appear until their task is done.

---

### Task 1: brew parser — `ParseInfo` (23) and `caskDeps` (21)

**Files:**
- Modify: `internal/brew/parse.go:77` (`ParseInfo`) and `:135` (`caskDeps`)
- Test: `internal/brew/parse_test.go`, only to cover new helpers if needed

- [ ] Read both functions and their tests.
- [ ] Extract the per-package conversion steps of `ParseInfo`, such as building one `Package` from one JSON formula or cask, into helpers.
- [ ] Extract `caskDeps`'s inner loops, such as building the name index and resolving one cask's dependencies, into helpers.
- [ ] Run lint (neither name appears) and `go test ./internal/brew -count=1`.
- [ ] Commit: `refactor(brew): split ParseInfo and caskDeps into steps`

### Task 2: command-line completion — `candidates` (26) and `argValues` (28)

**Files:**
- Modify: `internal/ui/complete.go:62` (`candidates`) and `:117` (`argValues`)
- Test: `internal/ui/complete_test.go`, or whichever test file covers completion

- [ ] Read both functions and the completion tests.
- [ ] `argValues` switches on an argument kind. Give each kind, or each group of kinds, its own helper, so the switch only dispatches.
- [ ] Split `candidates` into its stages: parse the line, decide what kind of word is being completed, and gather matches.
- [ ] Run lint (neither name appears) and `go test ./internal/ui -count=1`.
- [ ] Commit: `refactor(ui): split completion into steps`

### Task 3: key and message routing — `actionKey` (gocyclo 29), `update` (25), `tabKey` (22)

**Files:**
- Modify: `internal/ui/actions.go:43` (`actionKey`)
- Modify: `internal/ui/app.go:376` (`update`) and `:733` (`tabKey`)

- [ ] **`actionKey`:** group its cases by tab into per-tab helpers, such as `installedKey`, `outdatedKey`, `historyKey` and `discoverKey`, each returning `(tea.Cmd, bool)`. Mirror the existing `serviceKey`. `actionKey` keeps the keys that work everywhere (`U`, `c`) and dispatches on `a.tab`. The order of checks must stay the same wherever two cases could both match.
- [ ] **`update`:** move groups of message cases into helpers where a case has more than a line or two of logic, for example the window-size handling or the job messages. The `switch msg.(type)` stays the single dispatcher.
- [ ] **`tabKey`:** extract the details-focus scrolling into a `scrollDetails(k)` helper, and the list update into `listUpdate(msg)`.
- [ ] Run lint (none of the three names appears) and `go test ./... -count=1`.
- [ ] Commit: `refactor(ui): route keys and messages through per-tab helpers`

### Task 4: drawing — `render` (gocyclo 23), `statusBar` (23), `tiles` (21), `badge` (21)

**Files:**
- Modify: `internal/ui/app.go:815` (`render`) and `:895` (`statusBar`)
- Modify: `internal/ui/home.go:58` (`tiles`)
- Modify: `internal/ui/sidebar.go:112` (`badge`)

- [ ] **`render`:** extract the per-tab body switch into `tabBody(c columns, h int) string`, and the overlay or drawer assembly if needed.
- [ ] **`statusBar`:** extract its left part (mode and breadcrumb) and right part (flash, busy and hints) into helpers.
- [ ] **`tiles`:** extract the Outdated tile and the Services tile into `outdatedTile(snap)` and `servicesTile(snap)`.
- [ ] **`badge`:** extract the per-tab counting, for example `serviceBadge` for the failing-services chip.
- [ ] Run lint (none of the four names appears), `go test ./... -count=1`, and the goldens test, which must be unchanged.
- [ ] Commit: `refactor(ui): split rendering of the body, status bar, tiles and badges`

### Task 5: Maintenance and marks — `syncMaint` (gocyclo 21) and `upgradeMarked` (23)

**Files:**
- Modify: `internal/ui/maintenance.go:31` (`syncMaint`), and `maintPreview` in the same file
- Modify: `internal/ui/actions.go`: `askAutoremove` and `askServicesCleanup`
- Modify: `internal/ui/marks.go:102` (`upgradeMarked`)

- [ ] **Shared Maintenance state:**
  - Autoremove and Services cleanup each decide "failed / still checking / nothing to do / something to do" in three places: `syncMaint`, `maintPreview` and the `ask…` function. Add one small helper per row that returns that state, for example a `tidyState` int with the values `tidyFailed`, `tidyChecking`, `tidyNone` and `tidySome`.
  - The Services cleanup helper also returns `remove` and `stop`.
  - Use it in all three places.
  - The texts and their order of precedence stay exactly as they are.
- [ ] **`syncMaint`:** each row's status moves into its own helper (`doctorStatus`, `cleanupStatus`, `autoremoveStatus`, `servicesStatus`, `updateStatus`), and `syncMaint` only assembles the rows.
- [ ] **`upgradeMarked`:** split the checks (pinned, pinned dependencies, mixed kinds) from building the action(s).
- [ ] Run lint, which must now report `0 issues.` across the whole repo. Run `go test ./... -count=1` and `make cover`.
- [ ] Commit: `refactor(ui): one state per Maintenance row; split upgradeMarked`
