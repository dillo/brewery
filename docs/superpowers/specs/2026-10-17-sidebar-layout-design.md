# brewery: sidebar layout

Date: 2026-10-17. Status: approved in conversation; built on `feat/sidebar` in three steps, each usable on its own.

## Goal

Lay brewery out left to right, the way the Brewy Mac app does: a sidebar picks what to show, the list in the middle shows it, and the details of the selected row sit on the right. Keep what brewery already is: vim keys, the status bar with the mode and the keys for where you are, the `:` command line, the drawer, the colours and fonts, and the light feel. Unlike Brewy, Home stays: it's the sidebar's first entry.

## Approach

The sidebar is a layer on top of today's tabs, not a rewrite of them. Each sidebar entry names an existing tab and one of that tab's views; selecting an entry is what a number key and `f` do today. Installed, Outdated, Services, Taps and Discover keep their lists, previews and keys. Only Maintenance is a new tab.

Steps:

1. The sidebar replaces the tab bar, with the flattened entries, focus, and the `h`/`l` keys.
2. The sliding column layout replaces today's list and preview split (`split.go`).
3. History and Maintenance join the sidebar.

## 1. The sidebar and focus

### Entries

```
 Home

 PACKAGES
  All            69
  Top-level      24
  Formulae       63
  Casks           6
  Dependencies   45
  Outdated       ↑3
  Pinned          1

 MANAGE
  Services       ✗1
  Taps            5

 DISCOVER
  Popular tools
  Popular apps
  Rising
  New
  Fonts
  Search all

 TOOLS
  History
  Maintenance
```

| Entry | Tab and view |
|---|---|
| Home | Home |
| All, Top-level, Dependencies | Installed: today's All, Top-level and Dependencies views |
| Formulae | Installed: a new view of the formulae |
| Casks | Installed: today's Apps view, renamed |
| Outdated | Outdated |
| Pinned | Installed: a new view of the pinned packages |
| Services, Taps | Services, Taps |
| Popular tools … Search all | Discover: today's six views (`Search all` is today's `All`) |
| History, Maintenance | step 3 |

TOOLS appears in step 3. Until then `:jobs` opens the job list as it does today.

### Look

- The sidebar is a bordered column 20 characters wide, including its border, on the left. It replaces the tab bar and the rule under it, so the body gains two rows.
- Section headings are muted capitals. The selected entry is a solid block in the active tab's style (`TabActive`); the others are plain.
- Counts are right-aligned and muted. The tab bar's chips move to their entries: `↑N` on Outdated (`ChipWarn`), `✗N` on Services (`ChipErr`), and `⚠` (`ChipStale`) on every entry whose data failed to refresh. A chip takes the place of the count.
- The focused column has the purple border that the focused pane has today; the others have the plain border.
- The list's strip of views (`Top-level 24  All 69 …`) and its `f/F to switch` hint go. The title and the view's one-line description stay.

### Focus and keys

Focus is one of: sidebar, list, details.

| Where | Key | What it does |
|---|---|---|
| Sidebar | `j` `k`, `gg` `G`, `ctrl+d` `ctrl+u` | move between entries, skipping headings; the list follows at once |
| Sidebar | `l` `enter` | focus the list |
| List | `h` | focus the sidebar |
| List | `l` `tab` `enter` | focus the details |
| Details | `h` `esc` | focus the list |
| Details | `j` `k` … | scroll, as the focused preview does today |
| Anywhere | `[` `]` | previous or next entry, wrapping; the focus stays in its column |
| Anywhere | `1`–`4` (`5` from step 3) | jump to Home, Packages, Manage, Discover (Tools): the entry last used in that section, or its first entry |

- brewery opens on Home with the focus in the sidebar.
- `f` and `F` are removed.
- Services' "follow the log" moves from `l` to `L`.
- Home's tiles move with `j` `k` instead of `h` `l`; see Home below.
- `esc` in the list keeps today's order: clear marks, then close the drawer.
- Every other key keeps its meaning: `u`, `A`, `x`, `p`, `space`, `s` `S` `R`, `a`, `/`, `:`, `o`, `i`, `y`, `gx`, `r`, `U`, `c`, `ctrl+c`, `q`, `?`. Keys that act on a row act on the list's selected row from the list or the details. From the sidebar they do nothing, except the ones that act on everything (`A`, `U`, `c`, `r`, `:`, `?`, `q`, `o`, `i`, `ctrl+c`).

### What carries over between entries

- The entries that are views of one tab share its `/` filter and its marks, as `f` does today. Packages marked in All are still marked in Casks.
- The cursor stays on the same package when it's in the new view (`followSelection`, as today).

### Home

When Home is selected, today's dashboard fills the list and details columns: the tiles, doctor's warnings, and the recent jobs.

- With the focus on Home's content, `j` and `k` move between the tiles.
- `enter` on a tile selects the matching sidebar entry and focuses the list: Installed → All, Outdated → Outdated, Services → Services. Doctor and Cleanup: see step 3. Until then they keep today's behaviour.

### Status bar and help

- The status bar's trail names the entry instead of the tab: `Casks › bitwarden`, `Popular tools › pipx`.
- `?` help lists the new keys and drops `f/F`.
- The README's Tabs and Keys sections become Sidebar and Keys. The screenshots are regenerated.

## 2. The sliding columns

Minimum widths: sidebar 20, list 60, details 40. Home's content counts as one column with a minimum of 60, in the list's place, with no details column.

The rule:

1. The focused column is always shown, at full width if it's the only one.
2. Add the column to its right if the minimum widths still fit, then the column to its left.
3. The sidebar keeps 20. What's left is split 55/45 between the list and the details, as today, or goes to Home's content.

| Width | Focus: sidebar | Focus: list | Focus: details |
|---|---|---|---|
| ≥ 120 | sidebar, list, details | sidebar, list, details | sidebar, list, details |
| 100–119 | sidebar, list | list, details | list, details |
| 80–99 | sidebar, list | sidebar, list | details |
| < 80 | sidebar | list | details |

At the narrowest widths `h` and `l` turn pages, and the status bar's trail says where you are.

What goes:

- `splitFor`'s three layouts: side by side, stacked, and preview-only-when-asked.
- `pvOpen`, `previewAlone`, `previewShown`, `servicesPreviewAlone`, and the special cases they need in the key handling and `esc`. With the details hidden, focusing them is the asking.
- `sideBySideWidth`, `previewMinWidth`, `minPreviewH`.

Unchanged: the drawer spans the full width under the columns; confirm prompts, help and the `:` popup overlay the body; Discover's pause before asking brew for details.

## 3. History and Maintenance

Both go under TOOLS, which `5` jumps to.

### History

The `:jobs` view becomes a sidebar entry.

- **List**: finished jobs, newest first, with today's `:jobs` rows: ✓ or ✗, the title, and how long ago.
- **Details**: the brew command (`brew ` + args), the exit code, when it started and how long it took, then the saved output (`Record.Output`), scrollable.
- `enter` focuses the details, which already show what `enter` replays into the drawer today.
- `:jobs` selects History.
- Home's `:jobs shows them all` line becomes `5 History has them all`.

No new data: `jobs.Record` already keeps the args and output.

### Maintenance

A new tab with a list of three rows:

| Row | In the list | In the details | `enter` |
|---|---|---|---|
| Doctor | `N warnings`, or `all clear` | doctor's warnings, as Home shows them | run doctor again |
| Cleanup | what the standard cleanup frees | both dry-run estimates, as the cleanup prompt words them | the cleanup prompt, as `c` |
| Update Homebrew | how long since brewery last ran `brew update` | Homebrew's version, and when brewery last ran `brew update` (from History), or that it hasn't | `brew update`, as `U` |

- `c` and `U` still work from anywhere.
- Home's Doctor tile selects Maintenance › Doctor, and its Cleanup tile selects Maintenance › Cleanup, focusing the list. Cleaning up from Home takes one more `enter` than today; in return every tile works the same way.
- Loading, failed and not-yet-measured states read as they do on Home and in the cleanup prompt today.
- No new queries. Autoremove will be the fourth row when it's built (see the Brewy backlog).

## Tests

- **Step 1**: sidebar rendering (entries, counts, chips, selection, focus border); entry → tab and view; `j` `k` `[` `]` and number keys, including the section memory; `h`/`l`/`enter`/`esc` focus moves; `L` for the log; Home's tiles on `j` `k`; marks and `/` shared across Installed's entries; help text.
- **Step 2**: a table test of the column rule for each width band against each focus, including Home; the golden files regenerated at 140 columns plus one narrow width.
- **Step 3**: History's rows and details; Maintenance's rows and details in loaded, loading and failed states; the Home tiles selecting their Maintenance rows; `:jobs` selecting History.

## Not in scope

- New brew features from the Brewy comparison (autoremove, reinstall, link/unlink, tap health, and so on).
- Collapsing the sidebar by hand; the column rule decides.
- Mouse support.
