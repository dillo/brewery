# Action menu, with reinstall, link and fetch: design

The last practical item from the Brewy backlog was keys for `reinstall`,
`link`, `unlink` and `fetch`, which can only be typed at `:` today. Each
new key is one more letter to remember, though, so this adds an action
menu instead: `enter` on a row lists what can be done to it. Reinstall,
link and fetch live in that menu, and keep direct keys (`R`, `L`, `f`)
that the menu shows beside them.

## The menu

### Opening and closing

- `enter` opens the menu on Installed, Outdated, Services, Taps, Discover
  and History, from the list or from the details.
- On an empty list (or one filtered to nothing), `enter` does nothing.
- `enter` no longer moves from the list to the details. `l`, `right` and
  `tab` still do.
- Home, the sidebar and Maintenance keep their `enter`: Home opens the
  tile, the sidebar goes to the list, and Maintenance runs the row (each
  row is a single action already).

### Inside the menu

| Key | Does |
|---|---|
| `j`, `down` | next item, wrapping to the first |
| `k`, `up` | previous item, wrapping to the last |
| `enter` | runs the item under the cursor |
| an item's key | runs that item; `gx` is `g` then `x` |
| `esc`, `q` | closes the menu |

Any other key is ignored while the menu is open. Running an item closes
the menu first, then does exactly what the item's direct key does,
including its notes and prompts.

### Looks

A box over the screen, drawn with the same `overlay` as the confirm box,
with a rounded border.

```
╭ fd 10.2.0 ─────────────────────╮
│ › Upgrade to 10.3.0          u │
│   Fetch 10.3.0               f │
│   Reinstall…                 R │
│   Unlink…                    L │
│   Pin                        p │
│   Uninstall…                 x │
│   Copy the name              y │
│   Open the homepage         gx │
│                                │
│ j/k move · enter run · esc     │
╰────────────────────────────────╯
```

- **Title:** what the menu acts on: `<name> <version>` for a package,
  the name for a service or tap, `brew <args>` for a History job
  (truncated to fit), and `N marked` with marks.
- **Items:** the label on the left; its key on the right, muted. The
  selected item is marked `›` and drawn in the accent colour.
- A label ends in `…` when picking it asks first.
- **Footer:** `j/k move · enter run · esc close`, muted.
- The cursor starts on the first item.

### Items per tab

Only the items that apply to the selected row are listed, in this order.

**Installed**

| Item | Key | Listed when |
|---|---|---|
| `Upgrade to <newer>` | `u` | outdated and not pinned |
| `Fetch <newer>` | `f` | outdated |
| `Reinstall…` | `R` | always |
| `Link` or `Unlink…` | `L` | a formula; `Link…` for an unlinked keg-only one |
| `Pin` or `Unpin` | `p` | always |
| `Uninstall…` | `x` | always |
| `Copy the name` | `y` | always |
| `Open the homepage` | `gx` | it has a web page |

**Outdated**

| Item | Key | Listed when |
|---|---|---|
| `Upgrade to <newer>` | `u` | not pinned |
| `Fetch <newer>` | `f` | always |
| `Upgrade all…` | `A` | always |
| `Copy the name` | `y` | always |
| `Open the homepage` | `gx` | it has a web page |

**Services**

| Item | Key | Listed when |
|---|---|---|
| `Start` | `s` | status isn't `started` |
| `Stop…` | `S` | status is `started` |
| `Restart` | `R` | status is `started` |
| `Follow the log` | `L` | it has a log file |
| `Copy the name` | `y` | always |
| `Open the homepage` | `gx` | its formula has a web page |

**Taps**

| Item | Key | Listed when |
|---|---|---|
| `Untap…` | `x` | always |
| `Copy the name` | `y` | always |
| `Open the homepage` | `gx` | its remote is a web page |

**Discover**

| Item | Key | Listed when |
|---|---|---|
| `Install…` | `a` | always |
| `Copy the name` | `y` | always |
| `Open the homepage` | `gx` | it has a web page |

**History**

| Item | Key | Listed when |
|---|---|---|
| `Run again…` | `R` | the job has arguments |
| `Copy the command` | `y` | always |

### With marks

On Installed and Outdated, when packages are marked, the menu acts on
the marked set, titled `N marked`, and lists only the items that work on
several:

| Tab | Items |
|---|---|
| Installed | `Upgrade` (`u`), `Fetch` (`f`), `Reinstall…` (`R`), `Uninstall…` (`x`) |
| Outdated | `Upgrade` (`u`), `Fetch` (`f`) |

Each runs what its key does with marks today, checks included.

## Data

`brew.Package` gains two fields, read from `brew info --json=v2`. Both
apply to formulae only; casks leave them false.

| Field | JSON | Meaning |
|---|---|---|
| `Linked bool` | `linked_keg` (a version string or null) | true when `linked_keg` is a non-empty string |
| `KegOnly bool` | `keg_only` | true when brew doesn't link the formula into the prefix by default |

The JSON names of the snapshot fields are `linked` and `keg_only`.
`snapshotVersion` goes from 4 to 5, so the old cache is ignored once.

Installed's details for a formula gain a `linked` row:

| State | Text |
|---|---|
| linked | `yes` |
| not linked | `no` |
| not linked, keg-only | `no · keg-only` |

Casks don't get the row.

## The new actions

These are reached from the menu, or directly by their keys.

### `R`: reinstall (Installed)

**Selected package** (no marks):
- A confirm box opens.
  - Title: `Reinstall <name> <version>?`
  - Body: `brew removes it and installs it again.`
  - Choices: `[y] reinstall` and `[n] cancel`.
- On yes it runs
  `action{title: "reinstall <name>", args: kindArgs("reinstall", kind, fullName), refresh: diskQueries}`.

**Marked packages:**
- One confirm box opens.
  - Title: `Reinstall N packages?`
  - Body: their names, comma-separated, wrapped like uninstall's.
- On yes it runs one job per kind, formulae first, as marked upgrade
  does. Each job is titled `reinstall <names>`, uses
  `kindArgs("reinstall", kind, names...)` and refreshes `diskQueries`.
- Marks clear as they do after a marked upgrade.

### `L`: link or unlink (Installed)

`L` acts on the selected package only; marks are ignored.

| Selected package | What `L` does |
|---|---|
| a cask | the note `casks aren't linked` |
| a linked formula | a confirm box (below), then `unlink <name>` |
| an unlinked keg-only formula | a confirm box (below), then `link --force <name>` |
| an unlinked formula | runs `link <name>` straight away |

The confirm boxes:

| For | Title | Body | Choices |
|---|---|---|---|
| unlink | `Unlink <name>?` | `Its commands leave your PATH until you link it again.` | `[y] unlink`, `[n] cancel` |
| keg-only link | `<name> is keg-only` | `brew won't link it unless forced, as it can clash with macOS or another formula.` | `[f] link --force`, `[n] cancel` |

The actions:

| Verb | Title | Args |
|---|---|---|
| link | `link <name>` | `link <fullName>` |
| unlink | `unlink <name>` | `unlink <fullName>` |
| forced link | `link --force <name>` | `link --force <fullName>` |

All three refresh `[qInstalled]`.

### `f`: fetch the newer version (Installed and Outdated)

**Selected package** (no marks):

| Package | What `f` does |
|---|---|
| up to date | the note `<name> is up to date` |
| outdated | runs the action below straight away |

The action is
`action{title: "fetch <name>", args: kindArgs("fetch", kind, fullName), refresh: []query{qCleanup}}`.
The cache grows, so Cleanup's dry run is measured again.

**Marked packages:**
- `f` fetches the outdated ones, one job per kind, formulae first.
- Packages that are up to date are skipped. Pinned ones are fetched too,
  since a download never upgrades anything.
- If none of the marked packages is outdated, it shows the note
  `nothing marked is outdated`.
- Marks are kept, so `u` can follow on the same set.

On Outdated, every row is outdated, so `f` always fetches.

## Code shape

- A new `internal/ui/menu.go` holds the menu:
  - `menuItem{key, label string; run func() tea.Cmd}`;
  - `menuBox{title string; items []menuItem; cursor int; gPending bool}`
    with `key(k string) (run func() tea.Cmd, done bool)` and
    `view(st theme.Styles, maxW int) string`;
  - `(a *App) openMenu()`, which builds the items for the current tab
    and selection, one helper per tab (`installedMenu`, `outdatedMenu`,
    `servicesMenu`, `tapsMenu`, `discoverMenu`, `historyMenu`).
- Each item's `run` calls the same function as its key, so the key and
  the menu can't drift apart.
- `App` gains `menu *menuBox`. `modalKey` gives it keys after `confirm`,
  and `render` draws it as it draws `confirm`.
- `tabKey` sends `enter` to `openMenu` on the six tabs above, from the
  list or the details.
- Every function stays under the complexity limits (gocognit and gocyclo
  at 20).

## Help and hints

**Help:**
- Each of the six tabs gains `enter` `actions` as its first row.
- `l tab enter` becomes `l tab` wherever it appears.
- New rows, after `p` on Installed:

| Group | Key | Text |
|---|---|---|
| Installed | `R` | `reinstall` |
| Installed | `L` | `link or unlink` |
| Installed | `f` | `fetch the newer version` |
| Outdated | `f` | `fetch the newer version` |

- The `space` text on Installed becomes `mark for several`, and on
  Outdated the same.

**Hints:** they lead with the menu and drop most single letters.

| Where | Hint |
|---|---|
| Installed list | `enter actions · space mark · l details` |
| Installed details | `enter actions · j/k scroll · h back to the list` |
| Outdated | `enter actions · space mark · A upgrade all` |
| Services list | `enter actions · l details` |
| Services details | `enter actions · h back to the list`, with `j/k scroll the log · ` in front when the log scrolls |
| Taps | `enter actions` |
| Discover list | `enter actions · / search · l details · r refetch` |
| Discover details | `enter actions · j/k scroll · h back to the list` |
| History list | `enter actions · l details · / filter by command` |
| History details | `enter actions · j/k scroll · h back to the list` |
| Menu open | `j/k move · enter run · esc close` |

Maintenance, Home and the sidebar keep their hints.

## Testing

**brew:**
- `ParseInfo` reads `linked_keg` (string or null) and `keg_only`.
- The fixture `internal/brew/testdata/info.json` gets `"linked_keg"` on
  fd, and `"keg_only": true` with a null `linked_keg` on one formula.
  Check that the other tests still pass.

**ui, the menu:**
- `enter` opens it on each of the six tabs, and not on an empty list.
- `enter` on Home, the sidebar and Maintenance does what it did.
- `enter` no longer opens the details; `l` and `tab` still do.
- The items listed for each tab, including the conditional ones:
  outdated or not, pinned, linked, keg-only, cask, service started or
  not, no web page, marks on Installed and Outdated.
- `j`/`k` wrap; `enter` runs the item under the cursor; an item's key
  runs it; `g` then `x` opens the homepage; `esc` and `q` close it; any
  other key is ignored.
- Picking an item that asks opens its confirm box.

**ui, the actions:**
- `R`, single: prompt text, then args and refresh.
- `R`, marked: the prompt, then one job per kind.
- `L`:
  - on a cask: the note;
  - on a linked formula: the prompt, then unlink;
  - on an unlinked formula: links without a prompt;
  - on a keg-only formula: the prompt, then `link --force`.
- `f`:
  - on an up-to-date package: the note;
  - on an outdated package: the args, and the refresh is `qCleanup`;
  - on marks: only the outdated ones, and the "nothing marked is
    outdated" note;
  - on Outdated.
- The details' `linked` row in each of its three states, and no row for
  a cask.
- Help and hints.

**Existing tests** that press `enter` to reach the details switch to `l`
or `tab`.

**Goldens and checks:**
- Add a golden of the Installed menu open on fd.
- The other goldens change only for the details row and the hints. Check
  the diff before updating.
- `make cover` stays at 90% or above in total and per package.
- `golangci-lint` reports `0 issues.` with the complexity limits at 20.
