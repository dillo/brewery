# What's new: design

The Outdated tab says that `gh` can go from 2.101.0 to 2.104.0, but not
what changed in between. Upgrading, pinning or waiting is decided blind,
or after a trip to the browser for each package. This adds the upstream
release notes for the versions you'd be skipping over, fetched from GitHub
when you ask, in a details pane beside the Outdated list.

## When it runs

Only when the user asks, for one package at a time: `w` on an Outdated
row, or **What's new** in its action menu. Brewery never calls GitHub
otherwise, as with Check taps. The fetch only reads, so it doesn't ask
first. What it fetched is kept, so asking again costs nothing until
there's a newer version.

Marked rows don't fetch: `w` with rows marked notes `w shows one package
at a time`, and the marked menu doesn't offer it.

## Where the notes come from

**The repository.** brew's JSON already names where each package is
downloaded from. The first of these that is on GitHub is the package's
repository:

| Kind | Tried in order |
|---|---|
| formula | `urls.stable.url`, `urls.head.url`, `homepage` |
| cask | `url`, `homepage` |

An address is on GitHub when it starts `https://github.com/<owner>/<repo>`
or `https://codeload.github.com/<owner>/<repo>`. Only the first two path
segments count, so `…/archive/refs/tags/v2.104.0.tar.gz` and
`…/releases/download/v2.104.0/gh.zip` both give `cli/gh`. A trailing
`.git` is dropped. `brew.Package` gains `Source string`, the first
address in the table that is set (on GitHub or not), so the pane can name
the host when it isn't GitHub.

**The releases.** One request: `GET
https://api.github.com/repos/<owner>/<repo>/releases?per_page=100`, with
the headers and the 10-second timeout Check taps uses, and no token.
Drafts don't appear without a token. Pre-releases are dropped unless the
newer version is itself one.

**Which releases.** The ones newer than the installed version, up to and
including the version brew would install, newest first. To compare them:

- **brew's versions**: a formula's revision suffix goes (`2.101.0_1` →
  `2.101.0`), and so does everything after a cask's first comma
  (`1.4.2,abc123` → `1.4.2`).
- **Tags**: a leading `v`, `release-`, `<repo>-` or `<package name>-` goes
  (`v2.104.0`, `gh-2.104.0` → `2.104.0`). When the tag doesn't parse, the
  release's name is tried the same way.
- **Comparison**: split on `.` and `-`; numeric parts compare as numbers,
  others as text; a missing part is lower (`2.1` < `2.1.1`).

A release whose tag and name both fail to parse is left out.

**When nothing matches.** The repository has releases but none fall in
the range, or it has none. Brewery then makes a second request, `GET
…/repos/<owner>/<repo>/tags?per_page=100`. When both versions match a
tag, the pane offers the compare page,
`https://github.com/<owner>/<repo>/compare/<old tag>...<new tag>`.
Otherwise it offers the releases page. Either way the address is shown,
and `gx` opens it.

**Failures** work as in Check taps:
- A 403 or 429 with `X-RateLimit-Remaining: 0` is `rate limit reached`.
- A 404 is `the repository is gone`.
- Anything else is the error, shortened to fit.

Failures aren't kept, so `w` tries again.

## The Outdated details pane

Outdated gets a preview pane, as Services and History have. It's there
for every row, not only ones with notes.

**Title**: `<name> <installed> → <current>`.

**Body, by situation:**

| Situation | Body |
|---|---|
| no GitHub source | `release notes come from GitHub, and <name> comes from <host>` (muted), then `gx opens the homepage` if there is one |
| not fetched yet | `<owner>/<repo>` (muted), then `w shows what's new, from GitHub` |
| fetching | `asking GitHub…` |
| failed | `couldn't fetch: <reason>` (warning colour), then `w tries again` |
| no releases in range | `no release notes between these versions`, then the compare or releases address, then `gx opens it` |
| releases found | below |

**Releases found:**
- A first line: `N releases since <installed> · fetched <ago>`.
- When any of them flags a breaking change (see below), the next line is
  `mentions breaking changes` in the warning colour.
- Then, for each release:
  - its tag in the title style;
  - its date, muted;
  - its notes, rendered as below.
- A release with no notes says `no notes` (muted).

**Scrolling.** `j`/`k` scroll the pane when it has the focus (`l` or
`tab`), as the Services log does. Moving to another row, or new notes
arriving, scrolls it back to the top.

**Long notes.** Each release shows at most 40 lines. A longer one ends
with `… K more lines`, then the release's address, muted.

### Rendering the notes

GitHub's release notes are markdown. Brewery has no markdown renderer,
and this doesn't add one; a small one covers what release notes use:

- `#`–`######` headings: the text, bold.
- `-`, `*` and `+` list items: `•`, with their indent kept, up to two
  levels.
- `**bold**` and `__bold__`: bold. `*em*` and `_em_`: unchanged text.
- `` `code` ``: unchanged, in the accent colour.
- `[text](url)`: the text. Bare addresses are kept.
- Images, HTML tags and HTML comments are dropped. GitHub's
  auto-generated notes are full of these.
- Fenced code blocks: their lines unchanged, muted, without the fences.
- Runs of blank lines become one. Lines wrap to the pane's width.

**Breaking changes.** These words are drawn in the warning colour,
matched whole and ignoring case: *breaking*, *removed*, *deprecated*,
*incompatible*, *migration*. Any of them flags the release.

## Upgrade all

`A`'s confirmation gains one line when the notes already fetched flag a
breaking change in any package it would upgrade: `release notes mention
breaking changes: <names>`, warning colour, names truncated to fit. The
same goes for `u` on marked rows, which doesn't ask first: the line is
added to the note it leaves, as the pinned-behind warning is. Neither
fetches; they only read what's kept.

## The action menu

On an Outdated row, **What's new** (`w`) comes after Fetch and before
Upgrade all. The label is `What's new` when nothing is kept, and `What's
new (fetched)` when the notes for these two versions are. For a row with
no GitHub source it isn't offered.

Help's Outdated group gains `{"w", "what's new, from GitHub"}` and
`{"l tab", "details"}`.

## What's kept

The snapshot gains `ReleaseNotes map[string]notes.Notes`, by
`Package.Key()`:

```go
type Notes struct {
    Repo     string           // owner/repo
    From, To string           // the versions they're for
    Releases []github.Release // in range, newest first
    Link     string           // compare or releases page, when Releases is empty
    At       time.Time        // when they were fetched
}
```

- Kept notes are shown only while `From` and `To` match the row. When
  brew finds a newer version, the pane says `not fetched yet` again.
- Entries for packages no longer outdated are dropped when the snapshot
  is saved.
- Release bodies are kept as fetched, at most 16 KB each, so the cache
  stays small.
- `snapshotVersion` goes from 8 to 9.

## Code shape

**`internal/github`**, alongside `Repo`:
- `type Release struct{ Tag, Name, Body, URL string; Published time.Time; Prerelease bool }`
  (JSON `tag_name`, `name`, `body`, `html_url`, `published_at`,
  `prerelease`).
- `(c *Client) Releases(ctx, owner, repo string) ([]Release, error)` and
  `(c *Client) Tags(ctx, owner, repo string) ([]string, error)`.
- Both share `Repo`'s request setup and error handling, and return
  `ErrRateLimited` for the rate limit.
- `SourceRepo(addr string) (owner, repo string, ok bool)` parses the
  addresses above. `RepoOf` stays as it is for taps.

**`internal/brew`**: `formulaJSON` reads `urls.stable.url` and
`urls.head.url`, and `caskJSON` reads `url`. `toPackage` fills `Source`.

**New package `internal/notes`**, with no UI in it:
- `Version(s string) []part` and `Less(a, b string) bool` normalise and
  compare.
- `TagVersion(tag, repo, name string) (string, bool)` strips a tag's
  prefixes.
- `Pick(rels []github.Release, repo, name, from, to string) []github.Release`
  chooses the range.
- `CompareLink(tags []string, …) string`.
- `Breaking(body string) bool`.
- `Fetch(ctx, f Fetcher, owner, repo, name, from, to string) (Notes, error)`
  runs the one or two requests.

**`internal/ui`**:
- `ReleaseFetcher{ Releases(…); Tags(…) }`, set with
  `a.UseReleases(f)`, as `UseTapChecker` is. `cmd/brewery` passes the
  same `github.NewClient()`.
- With no fetcher set, `w` notes `release notes aren't available`.
- `notes.go` holds the preview, the markdown renderer and the `w`
  handler.
- The fetch is one `tea.Cmd` returning one message. Only one fetch runs
  at a time: `w` on another row while one runs notes `already asking
  GitHub about <name>`.

## Testing

**github** (with `httptest.Server`):
- Releases: 200 with releases, an empty list, 404, 403 rate-limited, 500,
  a body that isn't JSON.
- Tags: the same cases.
- The `Accept` and `User-Agent` headers are sent.
- `SourceRepo`: archive, release download, codeload, `.git`, a homepage,
  another host, an empty address, and a path with only an owner.

**notes:**
- Version comparison: numeric parts, mixed text, different lengths,
  formula revisions, cask commas.
- Tag prefixes: `v`, `release-`, `<repo>-`, `<name>-`, and a tag that
  doesn't parse but whose name does.
- `Pick`: an exclusive lower and inclusive upper bound, pre-releases
  dropped, and pre-releases kept when the newer version is one.
- `CompareLink` with both, one and neither version tagged.
- `Breaking`: whole words only (*irremovable* doesn't count) and case.
- `Fetch` with a fake: releases found, the tag fallback, and failures.

**ui** (with a fake fetcher):
- Every row of the situation table.
- `w` fetches once, while kept notes for the same versions fetch
  nothing.
- A newer version shows `not fetched yet` again.
- `w` with marks, and `w` while a fetch runs, only note.
- The menu items and their labels.
- Upgrade all's warning line, and that it never fetches.
- The renderer, on a sample of GitHub's auto-generated notes, with
  images and HTML comments.
- Truncation at 40 lines.
- Notes survive a save and load, and are dropped for packages no longer
  outdated.

**Goldens and checks:**
- A new Outdated golden with the pane showing `not fetched yet`, and one
  with fetched notes from the fake. Check the diff before updating.
- The demo data gains releases for one outdated package, so screenshots
  can show it.
- `make cover` stays at 90% or above in total and per package.
- `golangci-lint` reports `0 issues.` with the complexity limits at 15.

## Not in this

- A token, for example from `gh auth token`. 60 requests an hour is
  plenty for one package at a time, and a token would be the first
  credential brewery reads.
- Changelogs outside GitHub (GitLab, sourcehut, `CHANGELOG.md` files).
- What's new on the Installed tab. Its outdated rows already lead to
  Outdated.
- Reading the notes full screen in the pager.
- Fetching for every outdated package at once.
