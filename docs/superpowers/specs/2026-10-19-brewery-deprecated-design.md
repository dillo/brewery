# Deprecated and disabled packages: design

brew marks a formula or cask **deprecated** when it's on its way out (it
still installs), and **disabled** when it won't install any more (brew
removes it about a year later). Brewery only shows a `deprecated: yes`
row in Installed's details. This shows both states in the list, explains
them in the details the way brew does, and adds a sidebar view of them.

## Data

`brew info --json=v2` gives, for formulae and casks alike:
`deprecated`, `deprecation_date`, `deprecation_reason`,
`deprecation_replacement_formula`, `deprecation_replacement_cask`,
`disabled`, `disable_date`, `disable_reason`,
`disable_replacement_formula`, `disable_replacement_cask`. Dates are
`YYYY-MM-DD` or null; reasons are a brew keyword (such as `unmaintained`)
or free text, or null.

`brew.Package` keeps `Deprecated` and gains:

| Field | JSON (snapshot) | From |
|---|---|---|
| `Disabled bool` | `disabled` | `disabled` |
| `DeprecationDate string` | `deprecation_date` | `deprecation_date`, or "" |
| `DisableDate string` | `disable_date` | `disable_date`, or "" |
| `Reason string` | `reason` | `deprecation_reason` when deprecated, else `disable_reason` when disabled, else "" |
| `Replacement string` | `replacement` | the replacement from the `disable_replacement_*` fields when disabled, else the `deprecation_replacement_*` fields when deprecated (as brew does), written as brew writes it: the name when the formula and cask replacements are the same name, `--formula <name>` or `--cask <name>` otherwise, "" when none |

`snapshotVersion` goes from 6 to 7.

**State.** As in brew, a package that is deprecated counts as deprecated,
even if it is also disabled; otherwise a disabled one counts as disabled.

## The wording

Brewery says what brew says (`Library/Homebrew/deprecate_disable.rb`).

**Reason.** A known keyword becomes brew's phrase; anything else is shown
as it is.

| Formula keyword | Phrase |
|---|---|
| `does_not_build` | does not build |
| `no_license` | has no license |
| `repo_archived` | has an archived upstream repository |
| `repo_removed` | has a removed upstream repository |
| `unmaintained` | is not maintained upstream |
| `unreachable` | is no longer reliably reachable upstream |
| `unsupported` | is not supported upstream |
| `deprecated_upstream` | is deprecated upstream |
| `versioned_formula` | is a versioned formula |
| `checksum_mismatch` | has a source file whose checksum changed upstream |

| Cask keyword | Phrase |
|---|---|
| `discontinued` | is discontinued upstream |
| `moved_to_mas` | is now only on the Mac App Store |
| `no_longer_available` | is no longer available upstream |
| `no_longer_meets_criteria` | no longer meets the criteria for casks |
| `unmaintained` | is not maintained upstream |
| `fails_gatekeeper_check` | does not pass the macOS Gatekeeper check |
| `unreachable` | is no longer reliably reachable upstream |

(Three long phrases are shortened to fit the details: brew's
`checksum_mismatch`, `moved_to_mas` and `no_longer_meets_criteria`.)

**Disable date.** The disable date, or else the deprecation date plus 12
months, as brew does. Before today: `disabled on <date>`; today or later:
`will be disabled on <date>`. No date: no line.

## Installed list

The STATUS column, which already shows `pinned` and `↑`, adds `⚠`
(yellow) for deprecated or `✗` (red) for disabled after them. Symbols, not
words, so the columns keep their room; the words appear in the details and
the Deprecated view.

## Installed details

The `deprecated` row becomes up to three rows, only for a deprecated or
disabled package:

| Key | Value |
|---|---|
| `deprecated` or `disabled` | the reason phrase, or `yes` when there's none; yellow for deprecated, red for disabled |
| (blank) | the disable-date line, muted |
| `replacement` | `brew install <replacement>` |

## Sidebar view: Deprecated

- A new Installed view, **Deprecated**, sits after Pinned under PACKAGES.
  Its description: `On their way out: deprecated, or disabled and no
  longer installable.`
- It lists the packages that are deprecated or disabled.
- Its count shows in the sidebar like Pinned's.
- When it's empty it says `nothing installed is deprecated or disabled`.

## Testing

**brew:**
- Parsing reads every field above.
- The replacement is written as brew writes it in all three cases.
- The reason is taken from the matching state.
- Fixture `info.json`:
  - json-c: deprecated, reason `unmaintained`, deprecation date
    `2026-03-01`, replacement formula `json-c-ng`;
  - fontbase: disabled, reason `discontinued`, disable date
    `2026-09-01`.
  - Doctor's fixture already calls fontbase deprecated or disabled.

**ui:**
- the reason phrases (formula, cask, free text, none);
- the date line before, on and after today, and from a deprecation date
  alone;
- the STATUS marks;
- the details rows for each state, and none for a normal package;
- the Deprecated view's rows, its count and its empty note.

**Goldens:**
- These change, and nothing else may:
  - Installed (the marks, fontbase's in the list);
  - the sidebar on every screen (one more entry);
  - Help, if it shows the sidebar.
- Check the diff before updating.

`make cover` stays at 90% or above, and `golangci-lint` reports
`0 issues.`
