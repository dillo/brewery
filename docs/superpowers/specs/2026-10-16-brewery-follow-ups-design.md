# brewery: follow-ups

Date: 2026-10-16. Status: approved in brainstorming; becomes Plan 7½ on `feat/follow-ups`. The release stays Plan 8, which still waits until the user asks.

## Goal

Close the deferred items still open in `.superpowers/sdd/progress.md` before the release. Plans 2–6 already took care of the items Plans 1–5 deferred. Still open: the Plan 6 and Plan 7 deferred lists, Plan 5's leftover help and focus items, the crash log, and Plan 3's hard-link save. Per-task Minor notes are out of scope unless listed here.

## 1. Discover correctness

- **Tap dependencies.** `setHave` records each installed package under its `FullName` as well as its short name. `missing()` then treats `user/tap/foo` as installed when `foo` from that tap is, so it isn't listed under "Would also install".
- **Already queued.** Pressing `a` on a package whose install is queued or running flashes "<name> is already queued" and opens no dialog. Same for `:install <name>` when it would start the same install.
- **`triedOn` persists.** The analytics store keeps a `tried` file in its directory holding the day of the last fetch attempt (`2006-01-02`). On start, Discover reads it, so a fetch that failed earlier today isn't retried just because brewery restarted. `r` still forces a fetch. A missing or unreadable file counts as "not tried".

## 2. Analytics errors

`analytics.Store.Fetch` reports what went wrong, and the Discover title says it:

| Cause | Title text |
|---|---|
| network error (DNS, connect, timeout) | couldn't reach formulae.brew.sh · r retries |
| non-200 status | formulae.brew.sh answered 404 · r retries (the actual code) |
| body that doesn't parse | couldn't read formulae.brew.sh's lists · r retries |

The store returns `*StatusError{Code int}` for a bad status and wraps parse failures in `ErrBadList`. Anything else counts as unreachable.

## 3. Sizes and refresh cost

- **Partial sizes.** `Client.Sizes` checks the context between packages. When the 30s deadline passes, it returns the sizes measured so far along with the context error. The UI stores those sizes, keeps any older sizes for the rest, and doesn't show an error.
- **`DataUpdated`** asks `brew --cache` and `brew --repository` once per `Client` (on first use, `sync.Once`-style, and a failure is retried next time) and reuses the paths afterwards.
- **`fetch`** gets a copy of the package list only for `qSizes`. The other queries take none.

## 4. Looks

- **`Tint`** ends with the palette's text colour (`p.Text`), not the terminal's default foreground, so tinted cells inside a styled row don't turn the rest of the row plain. `Tint` takes the colour to go back to.
- **Leftover `pvOpen`.** On every resize, if the Services layout isn't "preview alone", `pvOpen` is cleared. Shrinking later shows the list first.
- **Keys while the preview stands alone.** When the preview stands alone, the list's keys (movement, marks, actions on the selected row) do nothing. `tab`/`enter`/`esc` go back to the list, and global keys (tabs, `:`, `?`, `q`, `r`) still work.
- **Services STATUS** is wide enough for `error (exit 127)` plus an icon and a space.
- **`--no-icons` help** says: set `BREWERY_NO_ICONS` to any value.

## 5. Keys and focus

- Leaving a tab drops preview focus there (Installed, Discover), so coming back starts on the list.
- If brew starts a prompt while help is open, help closes, so the next key goes to brew instead of being eaten.

## 6. Crash log

- `startCrashLog` also points file descriptor 2 at the temp file with `syscall.Dup2`, keeping a dup of the real fd 2 that `stop` puts back. Go runtime fatal errors (concurrent map writes, out of memory) write to fd 2 directly, so they now land in the log too.
- Logs are named `crash-<20061016-150405>.log`. After saving one, the oldest beyond the newest 5 are removed. brewery's exit message names the new file.

## 7. History without hard links

`jobs.History` saves by `os.Link` so it never overwrites. Some state directories (network shares, FUSE) don't support hard links. When the link fails with `EPERM`, `ENOTSUP` or `EXDEV`, the save falls back to Lstat-then-Rename, which isn't atomic but works. The link function is a field so tests can make it fail.

## 8. Tests and comments

- `TestSelectedRowStaysHighlighted` fails if the selected row is never found (a `found` flag).
- The `TestSizeTableDropsTrailingColumns` comment gives the widths as 8 + 8.
- New: `r` before Discover was ever opened starts no fetch and doesn't panic; it fetches once Discover opens.
- New: the `:` line completes catalogue names when typed key by key (`:install ripgr` then `tab`), not only through `wantCatalogue`.

## Testing

Each section gets unit tests in its package, written first. At the end, the controller runs race, live, vet, gofmt and `go mod tidy -diff`. Then the controller drives the branch binary through a PTY with scratch XDG dirs: Discover with an unreachable analytics base URL (wording), Services preview alone then a resize, help open while a stand-in brew prompts, and a forced panic to check the crash log name.
