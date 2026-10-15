# brewery: Discover

Date: 2026-10-15. Status: approved in brainstorming; becomes Plan 7. The release moves to Plan 8, which still waits until the user asks.

## Goal

Help the user find formulae, apps and fonts they didn't know to look for, and install them from inside brewery. This is browsing, not only search. It replaces the "Later: Browse and Discover" sketch in `2026-10-12-brewery-design.md`. "Related to yours" is dropped because Homebrew publishes no data on what gets installed together.

## 1. Shape

A sixth tab, **Discover**, on key `6`. Like Installed, it has views you switch with `f`/`F`:

| View | Contents | Columns |
|---|---|---|
| Popular tools | top 500 formulae by installs on request, last 30 days | #, NAME, DESCRIPTION, INSTALLS |
| Popular apps | top 500 casks by installs, last 30 days, without `font-…` | #, NAME, DESCRIPTION, INSTALLS |
| Rising | top 50 by growth (section 2), formulae and casks together | #, NAME, TYPE, GROWTH, DESCRIPTION |
| New | formulae and casks added since the user's last `brew update` | NAME, TYPE, DESCRIPTION |
| Fonts | casks named `font-…`, most installed first; fonts with no installs follow, by name | #, NAME, DESCRIPTION, INSTALLS |
| All | the whole catalogue, formulae then casks, by name | NAME, TYPE, DESCRIPTION |

- INSTALLS is shown as a short number (`217k`, `1.2M`). GROWTH is a multiplier (`×4.1`), or `new` for a package with (almost) no installs before.
- Rows you already have are marked `✓` before the name and dimmed. They stay in the lists.
- `/` filters the current view by name and description. In All, that makes it the search over the whole catalogue.
- On the four analytics views, the title shows how old the analytics are ("analytics from Oct 14") when they aren't from today, and "fetching analytics…" while a download runs.
- Preview pane, using the same split as the other tabs (`tab`/`enter` focus it, then `j`/`k` scroll). brew is asked about a package once the cursor has rested on it for 250ms, so holding `j` doesn't start a brew for every row; until it answers, the preview says "asking brew…". Answers are kept until the catalogue is read again.
  - name, version, description
  - type and tap
  - homepage and licence
  - "Would also install": dependencies the user doesn't have yet, or "nothing else"
  - installs over 30, 90 and 365 days (30 and 90 for casks)
  - whether it's installed
- Keys:
  - `a` installs (section 3)
  - `gx` opens the homepage; `y` copies the name
  - `/` filters
  - `r` refetches the analytics, besides refreshing brew's data and reading the catalogue again as `r` does everywhere
  - `?` shows help, which gains a Discover section
- Discover has no `space` marks in this version.

## 2. Data

| What | Source | Network |
|---|---|---|
| Catalogue: every name and description | `brew search --desc ""`, about 0.5s | none |
| Preview details | `brew info --json=v2 <name>`, for the selected row only | none |
| Dependencies it would add | `brew deps <name>`, minus installed packages | none |
| Popularity | formulae.brew.sh analytics: `install-on-request/30d.json`, `install-on-request/90d.json`, `cask-install/30d.json`, `cask-install/90d.json` | about 10 MB a day, all five files together |
| 365-day installs in the preview | `install-on-request/365d.json` (about 4 MB), fetched with the others. Casks have no 365-day list, so a cask's preview shows 30 and 90 days only | included above |
| New | `formula_names.before.txt` vs `formula_names.txt`, and the same for casks, in brew's API cache | none |
| Installed ✓ | brewery's existing Installed data | none |

**Catalogue format:**
- Output lines are `name: description` under `==> Formulae` and `==> Casks`.
- Cask lines can carry a display name in brackets, as in `0-ad: (0 A.D.) Real-time strategy game`. The bracketed part is dropped from the description.
- The catalogue is read the first time Discover opens (or the `:` line opens, for completion), and again on every refresh once it has been read, which includes the one after `U` (brew update).

**Rising:**
- For each package: growth = installs in the last 30 days ÷ ((90-day installs − 30-day installs) ÷ 2).
- Only packages with at least 300 installs in the last 30 days count.
- When the earlier figure is under 10 installs a month, the package shows `new` and ranks above every multiplier, with the `new` packages ordered by 30-day installs. (A package released this month often has a handful of test installs before; without this it would show as ×5000.)
- Packages whose growth is 1 or less are left out: Rising only lists what grew.
- Rising needs both the 30-day and the 90-day lists of a kind; until it has them, that kind is left out.
- Ties are broken by 30-day installs, then by name.
- The top 50 are shown.

**Analytics format:**
- Each file is `{start_date, end_date, items:[{number, formula|cask, count, percent}]}`.
- `count` is a string with thousands separators (`"217,104"`).
- Formula items can name a variant (`node@22`); these are kept as their own names.
- Items not in the catalogue, such as packages from other taps or removed packages, are skipped.

**Fetching and caching:**
- The first time Discover opens each day, the five files download in the background, one request after another.
- Each file is cached as `$XDG_CACHE_HOME/brewery/analytics/<name>.json`, with the time it was fetched, and written atomically.
- `r` refetches.
- A request has a 30-second timeout and sends `User-Agent: brewery/<version>`. Nothing else is sent.
- Until a download finishes, or when it fails, the views use the cached copy.
- With no cached copy:
  - Popular, Rising and Fonts say "fetching analytics…".
  - If the fetch fails, they say "couldn't reach formulae.brew.sh · r retries".
  - New and All still work, because they're local.
  - Fonts still lists every `font-…` cask by name.
- A failed fetch never deletes the cache.
- The network is used only when Discover opens (at most once a day) or on `r`. `HOMEBREW_NO_ANALYTICS` doesn't stop brewery from reading analytics, because that setting only stops sending them.

## 3. Installing

- `a` on a row the user already has puts "ripgrep is already installed" in the status bar and does nothing else.
- Otherwise `a` opens the confirm dialog:
  ```
  Install ripgrep 15.1.0?
  Search tool like grep and The Silver Searcher
  also installs 1 dependency: pcre2
  [y] install   [n] cancel
  ```
  - The version appears once brew has said it.
  - For a cask, the description line starts with "app · " or "font · ".
  - The dependency line is left out when nothing extra is needed. When there are more than five, it shows five and "+N more".
  - While the dependencies are still being worked out, the line says "checking dependencies…". `y` works anyway.
- `y` runs an ordinary job, the same as the other actions:
  - `brew install <name>`, or `brew install --cask <name>` for a cask
  - environment: `brew.JobEnv` plus `HOMEBREW_NO_ASK=1`
  - one job at a time, shown in the drawer
  - `i` types into it if brew asks for something, such as a password for a cask
- When the job succeeds, brewery refreshes what installing changes (installed packages, Home, disk sizes, like `diskQueries`). The row gets its ✓ and the status bar says "✓ install ripgrep", as other finished jobs do ("✓ upgrade fd").
- When it fails, the drawer keeps the output and the status bar shows the usual "✗ install failed (exit N) · o shows output".
- Commands that take any formula or cask (`:install`, `:info`, `:edit`…) complete from the whole catalogue, so `:install rip<tab>` suggests packages you don't have yet. Opening the `:` line reads the catalogue if Discover hasn't, and the suggestions catch up when it arrives.

## 4. Code

The existing layers stay as they are. The UI never runs brew or uses the network itself.

- **`internal/analytics`** (new):
  - `Parse([]byte) (List, error)`. A List holds the start date, end date, and items with name, rank and count.
  - `Rising(d30, d90 List) []Riser` and `Compare`, its order. Pure.
  - `Store`: `Fetch(ctx, list)` downloads a list and keeps a copy; `Cached(list)` reads the copy and when it was fetched. `NewStore(dir, version)` sets the timeout and user agent.
  - Uses only `net/http` and `encoding/json`, so no new dependencies.
- **`internal/brew`** (additions, read-only):
  - `Catalogue()` and `ParseCatalogue`
  - `NewSinceUpdate()`, which reads the before/current name lists
  - `Details(name, kind)`: `brew info --json=v2` and `brew deps` for one package, run at once
  - `Package.License`
- **`internal/ui`:**
  - `discover.go` (new): the tab. It has its own views, filter and split preview, like Installed, and shares Installed's bar of views (moved to `viewbar.go`).
  - `stats.go` (new): reading and fetching the analytics, behind an `Analytics` interface.
  - `details.go` (new): the preview, behind a `Catalog` interface that `*brew.Client` satisfies.
  - `install.go` (new): the install dialog. Install is an `action` (args, env, refresh) that uses the existing confirm dialog.
  - The catalogue, analytics and details arrive as their own messages, not through the query machinery: they belong to Discover alone.
- **`cmd/brewery`:** main injects the catalogue (the brew client) and an `analytics.Store` kept beside the snapshot, next to the other `Use…` hooks.
- **Snapshot:** unchanged. The analytics cache is separate and isn't read at startup.

## 5. Testing

- **Parsers, from fixtures cut from real output:**
  - `brew search --desc`, including the bracketed cask form and the two headers
  - the analytics JSON, including comma counts and `@` variants
  - the before/current name lists
- **Rising:** table tests for the 300 floor, `new`, packages that didn't grow, ties and ordering; the top-50 cut is the UI's.
- **Fetch and cache:**
  - `httptest` covering success, timeout, a non-200 status and broken JSON
  - the cache survives a failure; atomic write; age check
  - no test uses the real network, except a live test behind the `live` build tag (`go test -tags live`), like brew's
- **UI, with the existing fake Source plus fake catalogue and analytics:**
  - views and counts, `✓` and dimming, filter
  - preview fields, and brew asked only where the cursor rests
  - dialog wording, including the dependency count, "+N more" and "checking dependencies…"
  - "already installed"
  - the offline, "fetching…" and failed states, and the analytics-age line
  - the job's args and env, and the refresh after it succeeds
  - `:install` completion from the catalogue
- **Real run:** the real binary in a PTY with scratch XDG dirs, which must be absolute paths (brewery ignores relative ones and would use the real cache).
  - It opens Discover, does the real read-only analytics download, and goes through each view, the filter and the preview.
  - It opens the install dialog and answers `n`.
  - Nothing is installed for real without the user's explicit go-ahead.
