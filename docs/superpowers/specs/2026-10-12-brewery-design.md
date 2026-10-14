# brewery: design spec

> **Status: approved design (2026-10-13), pending the user's final read of this file.** Next step: implementation plan (writing-plans).

## Goal

brewery is a terminal UI that wraps Homebrew. It shows what the plain `brew` CLI keeps hidden and lets you run any brew command, with a much better experience: it feels like Neovim, with touches of Claude Code. It starts as a personal tool and will later be published through a tap.

- Command: `brewery`. The `homebrew` alias lives only in the user's dotfiles. Publishing under the name "Homebrew" is avoided for trademark reasons.
- The name `brewery` was checked on 2026-10-12: no formula, cask or binary in Homebrew core uses it.

## Decisions

| Topic | Decision |
|---|---|
| Scope order | See (A), then Maintain (C), then Discover (B) |
| Audience | The user first; published later through a tap |
| Stack | Go + Bubble Tea / Lip Gloss / Bubbles (Charm) |
| Interaction | Panes + Vim keys + `:` command line (runs any brew command) + `/` filter + `Ctrl-P` palette |
| Layout | Numbered tabs + wide table + preview pane (gh-dash style). The Home tab holds health tiles |
| Command output | A Claude Code-style output drawer that slides up and streams live, plus a `:jobs` history |
| Theme | Catppuccin Macchiato |

Mockups are in `.superpowers/brainstorm/33153-1791860435/content/` (`layout.html`, `command-output.html`).

## v1 scope

| v1: "See + Maintain" | Later |
|---|---|
| Tabs: Home, Installed, Outdated, Services, Taps | Search/Discover (catalog search + install), Disk breakdown, Brewfile/bundle |
| Preview: info, dependency tree, used-by, caveats, size, on-request vs. dependency | Changelogs, analytics |
| Actions: upgrade, uninstall (orphan warning), pin/unpin, services start/stop/restart, cleanup (shows space freed), update, doctor | Install from Search, tap/untap UI, autoremove wizard |
| `:` runs any brew command with completion | Context-aware suggestions |
| macOS | Linux, theme flavors, config file |

## 1. Architecture (approved)

```
cmd/brewery          main: flags, --version, starts the UI
internal/ui/         Bubble Tea views (no brew knowledge)
  app.go             root model: tabs, focus, mode (NORMAL/COMMAND/FILTER/RUNNING/INSERT)
  tabs/              home, installed, outdated, services, taps
  components/        table, preview, cmdline, drawer, statusbar, help
  theme/             semantic tokens → Lip Gloss styles
internal/state/      Store: brew snapshot + derived data (reverse deps, orphans, leaves)
internal/brew/       the only package that runs brew
  query.go           read-only calls
  exec.go            mutating calls through a PTY
internal/jobs/       one-at-a-time job queue + history (~/.local/state/brewery/jobs/)
```

Rules: the UI never shells out. `brew` is the only package that knows brew's CLI or JSON formats. `state` builds the "used by" map from `runtime_dependencies` in the info JSON, so it needs no `brew deps` calls.

## 2. Data loading and freshness (approved)

Measured on the user's machine (138 formulae, 11 casks, 23 leaves):

| Source | Time |
|---|---|
| `brew info --json=v2 --installed` | 0.75s, 690KB |
| `brew outdated --json=v2` | 1.37s |
| `brew services list --json` | 0.64s |
| `brew tap-info --json --installed` | 1.00s |
| `brew doctor` (text, exits 1 when it finds warnings) | 1.74s |

- **Startup:** the first frame comes from `~/.cache/brewery/snapshot.json` in under 100ms. The four JSON queries then run **in parallel** in the background, and each tab updates as its query returns. The status bar shows `⠹ refreshing` and then when it last refreshed. The first run, with no cache, shows skeleton rows.
- **Doctor:** runs lazily once the JSON queries finish. Each `Warning:` block becomes one item. If parsing fails, the raw text is shown.
- **Targeted refresh after jobs:** upgrade or uninstall re-runs info + outdated. A services action re-runs services. `r` refreshes everything.
- **`brew update`** runs only when you ask, with `U`. The Outdated tab shows how old the data is, based on the modification time of brew's repository data. Actions launched from a pane set `HOMEBREW_NO_AUTO_UPDATE=1`. Commands typed at `:` run unchanged.
- **Disk sizes:** brew's JSON has none, so brewery measures `Cellar/<name>` and `Caskroom/<name>` in the background for visible rows, and caches the results.
- **Query failure or 30s timeout:** that tab shows an inline error with the command and stderr. Other tabs keep working.

## 3. Running commands (approved)

- **Job** = argv (no shell), start and end time, exit code, and the end of its output (kept in its history record). Mutating jobs run **one at a time** because of brew's global lock. Extra jobs show as `queued`. Read-only queries skip the queue.
- **PTY** through `creack/pty`. The drawer contains a terminal emulator (Charm `x/vt` or a minimal custom renderer) that handles `\r` redraws and ANSI colors. When the process asks for input (`Password:`, `[y/N]`), the drawer switches to INSERT mode and keys go to the PTY. Input is masked and never logged.
- **Full-screen takeover** through Bubble Tea `ExecProcess` for `edit`, `sh`, `irb` and `bundle edit`. A `:!` prefix forces takeover.
- **Confirmations for destructive actions:** uninstall shows dependents and the orphans it would leave, with an option to remove them. Cleanup runs `--dry-run` first to show space freed. Untap, services stop, `^c` on a running job, and `A` (upgrade all) also ask first. Commands typed at `:` don't ask.
- **History:** `:jobs` lists jobs, and `Enter` replays a job's output in the drawer. The last 100 jobs are kept, each with the last 128 KiB of its output, in `~/.local/state/brewery/jobs/`.
- **Completion:** a popup above the prompt. Commands, subcommands, flags (with descriptions) and the kind of argument each command takes come from the fish completion script brew ships (`<brew --repository>/completions/fish/brew.fish`), read once at start. Package, tap and service names come from the snapshot.

## 4. UI, keymap and theme (approved)

- **Modes** (colored block in the status bar): NORMAL (mauve), COMMAND (blue), FILTER (teal), RUNNING (peach + spinner), INSERT (green). `esc` goes back one step.
- **Global keys:** `1`–`5` and `[`/`]` switch tabs. `Tab` moves focus. `:` opens the command line, `/` filters, `Ctrl-P` opens the palette, `r` refreshes, `U` runs update, `?` shows help, and `q` quits (with a confirmation if a job is running).
- **List keys:** `j/k`, `gg/G`, `Ctrl-d/u`, `space` to multi-select, `Enter` to focus the preview. `u` upgrade, `x` uninstall, `p` pin, `o` open homepage, `y` copy name. On Services: `s` start, `S` stop, `R` restart, `l` show log. On Outdated: `A` upgrades all.
- **Home:** tiles (Installed, Outdated, Services with error count, Doctor, reclaimable cache = size of `brew --cache`, measured in the background) + recent jobs. `Enter` on a tile jumps to its tab.
- **Installed:** a view bar above the list shows every view with its count, the active one highlighted: **Top-level** (default; nothing else needs these, so leaves plus apps), **All**, **Apps** (casks) and **Dependencies** (installed only because something needed them). A line under the bar says what the active view shows; `f`/`F` step forward and back. Narrow panes use short names (Top, All, Apps, Deps). Columns: name, version, type, pin, outdated ↑, size. Preview: description, dependency tree, used-by, caveats, tap, install date.
- **Outdated:** current → latest, multi-select upgrade.
- **Services:** name, status (colored), user, plist. The preview shows the end of the log.
- **Taps:** name, formula and cask counts, remote, official or not.
- **Theme:** semantic tokens (`accent`, `warn`, `err`, `muted`, …) → Catppuccin Macchiato. Other flavors later only need a new palette file. Nerd Font icons with an ASCII fallback (`--no-icons` or automatic detection).
- **Responsive layout:** below 100 columns the preview stacks under the list. Below 60 columns the preview hides and `Enter` toggles it.

## 5. Errors, testing and shipping (approved)

**Errors:**
- brew not found (checks `$HOMEBREW_PREFIX`, `/opt/homebrew`, `/usr/local`): print a message and exit without starting the TUI.
- A tab's query fails: show an inline error in that tab, and `r` retries.
- A job fails: the drawer stays open with `✗ exit N`, the job is recorded, and data refreshes anyway.
- Brew's lock is held by another process: show "waiting for brew lock…" and retry with backoff (cancelable).
- Panic: restore the terminal first, then write `~/.local/state/brewery/crash.log`.

**Testing:**
- `brew`: golden tests against fixture JSON captured from real brew output, plus `make test-live`, which runs the real brew read-only.
- `state`: table tests for reverse deps, orphans and leaves.
- `ui`: tests that send key messages and check model state, plus `teatest` snapshot tests of `View()` with a fake brew interface.
- `jobs`: run a fake script under the PTY that prints `\r` progress, asks `Password:`, then exits 1.
- CI: GitHub Actions running `go test ./...` and `golangci-lint` on macOS.

**Shipping:** goreleaser builds darwin arm64/amd64 binaries and publishes a formula to `<user>/homebrew-tap`. The `homebrew` alias belongs in the user's dotfiles only.

## Open items

- GitHub username/org for the tap (needed before release, not before building).
- Drawer terminal emulator: the first plan task is a time-boxed spike on Charm `x/vt`. If it can't handle brew's `\r` progress bars and ANSI output, use a minimal custom renderer (it only needs line buffering, `\r` overwrite, SGR colors and erase-line).
