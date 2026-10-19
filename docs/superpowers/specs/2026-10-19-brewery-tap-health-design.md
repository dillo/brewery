# Tap health: design

The last item from the Brewy backlog. A tap whose GitHub repository is
archived, moved or deleted doesn't break `brew upgrade`: its formulae just
stop getting updates, quietly. Brewery's jobs run without brew's
auto-update, so not even `brew update`'s one error line shows. This adds a
check you run yourself from Maintenance, and shows what it found there and
on Taps.

## When it runs

Only when the user starts it: `enter` on the new Maintenance row. Brewery
never calls GitHub otherwise. The check only reads, so it doesn't ask
first.

## What is checked

- Every installed tap that isn't official and whose remote is a GitHub
  address: `https://github.com/<owner>/<repo>`, with or without `.git`.
- brew's official taps are skipped silently.
- Other third-party taps (a remote elsewhere, or none) are listed as
  `not on GitHub` and not checked.
- One request per tap, one after another: `GET
  https://api.github.com/repos/<owner>/<repo>`, with the headers
  `Accept: application/vnd.github+json` and `User-Agent: brewery`, no token,
  and a 10-second timeout per request.
- Without a token GitHub allows 60 requests an hour, which is plenty for a
  run by hand.

## Results

| State | Meaning | Shown as |
|---|---|---|
| fine | 200; not archived; same `owner/repo` (case doesn't matter) | `fine` |
| archived | 200 with `"archived": true` | `archived` |
| moved | 200 after a redirect, with a different `full_name` | `moved to <full_name>` |
| gone | 404 | `gone` |
| unknown | anything else | `couldn't check: <reason>` |

- Renamed or transferred repositories answer with a 301 to
  `/repositories/<id>`. Go's HTTP client follows it, and the final body's
  `full_name` is the new name. Archived wins over moved when both apply.
- **Rate limit:** a 403 or 429 whose `X-RateLimit-Remaining` header is
  `0`. The reason is `rate limit reached`, and every tap not yet checked
  gets the same state without a request.
- **Other failures** (no network, a timeout, another status): the reason
  is the error, shortened to fit. The check carries on with the next tap.

Results are kept by tap name with the time the check finished, in the
snapshot cache: `TapChecks []github.TapCheck` and `TapsCheckedAt
time.Time`. `snapshotVersion` goes from 5 to 6. Results for taps no longer
installed are ignored when shown.

## Maintenance row: Check taps

It sits between Services cleanup and Update Homebrew.

**Status column:**

| Situation | Status |
|---|---|
| no taps to check (no third-party GitHub tap installed) | `no taps to check` (muted) |
| never run | `not checked yet` (muted) |
| running | `checking…` |
| problems found | the counts of archived, moved and gone that aren't zero, in that order, joined with ` · `, e.g. `1 archived · 1 gone` (warning colour) |
| no problems | `N fine` (OK colour) |

Third-party GitHub taps with no result, such as one added after the
check, count as not checked: when none has a result the status is `not
checked yet` (muted), otherwise ` · K not checked yet` closes it, e.g.
`2 fine · 1 not checked yet`.

When some taps couldn't be checked, ` · M couldn't check` is added to
either of the last two, e.g. `3 fine · 1 couldn't check`. When none was
fine and none had a problem, the status is just `M couldn't check`, in
the warning colour.

**Details column:**
- Title `Check taps`.
- `checked <ago>` (the `ago` helper Maintenance already uses), or `not
  checked yet`.
- One line per third-party tap, by name: the name, then its state as
  shown above, in the warning colour for archived, moved, gone and
  couldn't check. Taps not on GitHub say `not on GitHub`.
- Closing hint: `enter checks them on GitHub`.

**`enter`:**
- Starts the check and notes `checking taps on GitHub…`.
- While a check is running, `enter` notes `already checking taps`.
- With no taps to check, it notes `no taps to check`.

Help's and the hints' Maintenance texts don't change: `enter` already
"runs the selected task".

## Taps tab

- A new column, `GITHUB`, between `OFFICIAL` and `REMOTE`, shows the
  tap's last result as in the table above (`moved to …` truncated to
  fit), in the warning colour unless it's `fine`.
- It's empty for official taps, for taps never checked, and for taps not
  on GitHub.
- Nothing else changes. The action menu already offers `Untap…`.

## Code shape

- **New package `internal/github`:**
  - `type State string` with `Fine`, `Archived`, `Moved`, `Gone` and
    `Unknown`;
  - `type TapCheck struct{ Tap string; State State; MovedTo, Reason string }`
    (JSON `tap`, `state`, `moved_to`, `reason`);
  - `type Client struct{ BaseURL string; HTTP *http.Client }`, with
    `NewClient()` set to `https://api.github.com`;
  - `(c *Client) Repo(ctx, owner, repo string) (TapCheck, error)` makes one
    request. It returns `ErrRateLimited` for the rate limit, and an error
    for other failures;
  - `RepoOf(remote string) (owner, repo string, ok bool)` parses a GitHub
    remote.
- **UI:**
  - The UI depends on an interface, `TapChecker{ Repo(ctx, owner, repo) (github.TapCheck, error) }`,
    set with `a.UseTapChecker(c)`, as Discover's analytics are set with
    `UseAnalytics`.
  - `cmd/brewery` passes `github.NewClient()`.
  - With no checker set, `enter` on the row notes `checking taps isn't available`.
- **The check:** one `tea.Cmd` checks the taps in order and returns one
  message with all the results and the time. The App stores them in the
  snapshot and saves it.

## Testing

**github** (with `httptest.Server`):
- 200 fine; 200 archived; a 301 to another path whose body has another
  `full_name` (moved); 404 (gone); 403 with `X-RateLimit-Remaining: 0`
  (`ErrRateLimited`); 500 (error).
- The request has the `Accept` and `User-Agent` headers.
- `RepoOf` with `.git`, without `.git`, `http://`, another host, and an
  empty remote.

**ui** (with a fake checker):
- the row's status in every situation above, with its colours;
- the details' lines;
- `enter` runs it once, and a second `enter` while running only notes;
- official taps are skipped and non-GitHub taps listed;
- after a rate limit, the remaining taps make no request and show
  `couldn't check: rate limit reached`;
- results survive a save and a load of the snapshot;
- the Taps tab's `GITHUB` column.

**Goldens and checks:**
- The Maintenance golden gains the row (`not checked yet`). There's no
  Taps golden; the Taps test covers the column. Check the diff before
  updating.
- `make cover` stays at 90% or above in total and per package.
- `golangci-lint` reports `0 issues.` with the complexity limits at 20.
