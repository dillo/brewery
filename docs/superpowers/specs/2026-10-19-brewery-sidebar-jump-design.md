# Jump in the sidebar: design

Reaching an entry low in the sidebar, such as Maintenance, takes many
`j` presses. `/` in the sidebar now searches its entries, so `/maint`
then `enter` opens Maintenance.

## Behaviour

- With the focus on the sidebar, `/` opens a prompt in the status bar,
  badged `JUMP`, with the hint `enter open · esc back`.
- **While typing:** the selection moves to the first entry, from the top,
  whose label contains the text, ignoring case. The tab follows, as it
  does for `j`.
  - The sidebar keeps every entry; only the selection moves.
  - An empty prompt goes back to the entry the search started on.
- **No match:** the selection stays where the last match left it, and the
  hint becomes `no match · esc back`.
- **`enter`:**
  - The prompt closes.
  - The focus moves into the selected entry's list, as `l` does in the
    sidebar. Home starts on its first tile, as it does for `l`.
  - With no match, `enter` just closes the prompt and the focus stays in
    the sidebar.
- **`esc`:** the prompt closes, and the selection goes back to the entry
  the search started on.
- **Paste:** works as in the list filter. If brew asks for input while the
  prompt is open, the answer waits until it closes, also as in the list
  filter.

**Change:** until now, `/` in the sidebar moved into the list and
filtered it. From the list, `/` still filters the list, unchanged.

## Code shape

- The jump reuses the filter's mode and text input (`modeFilter`,
  `a.filterIn`), with a flag on `App`: `jumping bool` and `jumpFrom int`
  (the entry it started on).
- `filterKey` hands keys to a new `jumpKey` while `jumping`.
- The status bar shows the `JUMP` badge and hint when `jumping`.
- `sidebarKey`'s `/` opens the jump instead of moving to the list.
- **Matching:** `jumpMatch(text string) (int, bool)` is the first entry
  whose label contains `text`, ignoring case.

## Help and hints

- **Help's Sidebar group** gains `/` `jump to an entry`, after `gg G`.
- **The sidebar's hint** becomes `j/k choose · / jump · l open · 1-5 sections`.

## Testing

- `/maint` selects Maintenance as it's typed, and `enter` opens its list
  with the focus there.
- Matching ignores case, and the first match from the top wins: `/s`
  selects Casks, the first label containing "s".
- No match keeps the last match and shows `no match`. `enter` then
  leaves the focus in the sidebar.
- `esc` goes back to the starting entry.
- Clearing the text goes back to the starting entry.
- `/` from the list still filters the list. Update any test that relied on
  `/` from the sidebar filtering the list.
- Help and the hint.
- `golangci-lint` reports `0 issues.`, and `make cover` stays at 90% or
  above. No goldens change, except a golden that shows the sidebar's
  hint, if there is one.
