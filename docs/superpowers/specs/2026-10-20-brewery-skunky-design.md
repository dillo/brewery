# Skunky: design

A user asked for "the skunky beer feature": a way to see when something
installed has a known security problem. Homebrew 7 has `brew vulns`, which
looks up installed formulae in the OSV.dev database (CVEs among them). This
runs it now and then, and adds a sidebar view of the formulae it flags, with
the findings in the details.

It covers known vulnerabilities only. A compromised package isn't something
OSV lists for Homebrew; when that happens brew deprecates or disables the
package, which the Deprecated view already shows.

## Data

`brew vulns --json` prints, with no arguments, a scan of every installed
formula:

- `findings`: one entry per formula looked up, with `formula`, `version`,
  `vulnerabilities` (open findings) and `patched` (findings one of brew's
  patches already fixes). Each finding has `id`, `severity` (`CRITICAL`,
  `HIGH`, `MEDIUM`, `LOW` or `UNKNOWN`), `summary` and `fixed_versions`.
- `checked`: how many formulae were looked up.
- `skipped_formulae`: the ones brew couldn't map to an upstream source, and
  so didn't look up.
- `complete`: false when any were skipped.

It exits 1 when it finds anything, as doctor does. Casks aren't scanned.
A scan of 138 formulae took about 10 seconds; 49 were skipped.

**brew package.** `(c *Client) Vulns(ctx) (VulnScan, error)` runs
`brew vulns --json` with the client's timeout.

```go
type VulnScan struct {
	Findings []VulnFinding `json:"findings"` // formulae with open findings
	Checked  int           `json:"checked"`  // how many formulae were looked up
	Skipped  []string      `json:"skipped"`  // ones brew couldn't scan
}

type VulnFinding struct {
	Formula string `json:"formula"`
	Version string `json:"version"` // the version scanned
	Vulns   []Vuln `json:"vulns"`
}

type Vuln struct {
	ID       string   `json:"id"`
	Severity string   `json:"severity"` // as brew gives it
	Summary  string   `json:"summary"`
	Fixed    []string `json:"fixed"`    // fixed_versions: versions or commit hashes
}
```

- `patched` isn't kept, and a formula whose `vulnerabilities` is empty is
  dropped.
- `complete` isn't kept: it's false exactly when `Skipped` isn't empty.
- **Exit status.**
  - Exit 0 or 1 with JSON that parses is a result.
  - A timeout, a failure to run brew, or output that doesn't parse is a
    `QueryError`.
  - brew's `Unknown command` on stderr is `ErrNoVulns`, so the UI can say
    brew is too old.

**Snapshot.** It gains `Vulns *brew.VulnScan` (`vulns`) and `VulnsAt
time.Time` (`vulns_at`), and `snapshotVersion` goes from 7 to 8. `nil`
means never scanned, which isn't the same as a scan that found nothing.

**Which formulae are skunky.** An installed formula is skunky when the scan
has a finding for its name at the version installed now, less brew's
revision: brew vulns reports `jpeg-xl 0.12.0` when `0.12.0_1` is installed,
so a trailing `_<digits>` is dropped before comparing. After an upgrade
the installed version no longer matches, so the formula leaves the view
and its count straight away. The next scan judges the new version.

## When it runs

- A new query, `qVulns`, joins the slow pass that runs after the JSON
  queries (with doctor, cleanup and sizes). It joins only when `Vulns` is
  `nil` or `VulnsAt` is 24 hours or more before now, so most refreshes
  don't contact OSV.dev.
- **On demand.** On the Skunky view, `r` refreshes as it does everywhere
  and also adds `qVulns` to the slow pass whatever its age. `r` on any
  other view keeps the 24-hour rule.
- After an upgrade, the refresh that follows keeps the 24-hour rule too.
  The version rule above is what clears an upgraded formula.

## Sidebar: Skunky

- A new Installed view, **Skunky** (`filterSkunky`), sits after Deprecated
  under PACKAGES. Its description: `Installed formulae with known security
  problems, worst first.`
- **Badge:**
  - the number of skunky formulae (formulae, not findings);
  - a red chip, `!N`, when any of them has a HIGH or CRITICAL finding;
    otherwise a muted count, like Pinned's;
  - nothing before the first scan;
  - the stale `⚠` chip when the last scan failed, as other entries show for
    a failed query.

## The view

**Rows.** The skunky formulae, sorted by their worst finding, then by name.
Severities rank `CRITICAL`, `HIGH`, `MEDIUM`, `UNKNOWN`, `LOW`. UNKNOWN
sits above LOW because it means not scored yet, not minor. Anything else
brew prints ranks with UNKNOWN.

**SEVERITY column.** On this view only, the TYPE column is headed
`SEVERITY` and shows the worst finding's level: red for CRITICAL and HIGH,
yellow for MEDIUM, muted for the rest. Every row is a formula, so TYPE
would add nothing there. The other columns don't change.

**Coverage note.** A muted line under the list, like Outdated's "brew's
data from …":

| Situation | Note |
|---|---|
| scanned | `scanned <ago> · <checked> of <formulae installed> formulae checked · casks aren't scanned` |
| last scan failed, earlier result kept | `couldn't scan: <reason> · showing the scan from <ago>` |
| last scan failed, no earlier result | `couldn't scan: <reason>` |
| brew too old | `brew vulns isn't available · U updates brew` |

`<ago>` is the helper Maintenance uses. The reason is shortened to fit.

**Empty view.**

| Situation | Note |
|---|---|
| never scanned | `not scanned yet · r scans now` |
| scanning, no result yet | `checking with brew vulns…` |
| scanned, nothing found | `no known vulnerabilities in the <checked> formulae checked` |
| `/` filter | the existing `nothing in this view matches /…` |

The nothing-found note says what was checked, never that anything is safe.

## Details

A `Vulnerabilities` section, on every Installed view, for a formula with
open findings at its installed version. It sits after the key/value rows
and before the dependency tree, headed like `Dependencies` and `Caveats`.

```
Vulnerabilities
  HIGH      OSV-2021-777
            Heap-use-after-free in xmlAddNextSibling
  MEDIUM    CVE-2026-1234
            Integer overflow in xmlParseQName
            fixed in 2.15.5
  2.15.6 is available · u upgrades
```

- Findings are sorted worst first, then by ID. Severity is coloured as in
  the SEVERITY column.
- Summaries wrap to the pane's width.
- **Fixed line:**
  - versions: `fixed in <versions>`, joined with `, `;
  - when every entry is a 40-character hex commit hash: `fixed upstream,
    not yet released`;
  - a mix of the two: only the versions are shown;
  - none: no line.
- **Upgrade hint:** when the formula is outdated, the block closes with
  `<latest> is available · u upgrades`, muted. It doesn't claim the upgrade
  fixes anything: brew's versions and OSV's fixes are written differently
  (tags, commits, ranges), and a wrong "fixed" is worse than none.

## Help

When the view is Skunky, Help's Installed group adds `r` with `refresh, and
scan with brew vulns now`.

## Errors

- **A failed scan** keeps the last result: its rows, count and details
  stay. The badge shows the stale chip and the note says why.
- `VulnsAt` isn't updated, so the next refresh tries again.
- **`ErrNoVulns`** isn't an error: no failed-query mark in the status bar
  and no stale chip. The view shows its note, and `qVulns` isn't run again
  until a job that refreshes everything (such as `U`) finishes, since brew
  update may bring the command.
- **Quitting** cancels a running scan, as with other queries.

## Testing

**brew:**
- A fixture, `vulns.json`, trimmed from a real scan:
  - libxml2 with two HIGH findings;
  - a formula with MEDIUM and UNKNOWN findings, one fixed in a version and
    one in a commit;
  - openjpeg with only `patched` findings;
  - some `skipped_formulae`.
- Parsing:
  - keeps the open findings, `checked` and the skipped list;
  - drops `patched`, and openjpeg.
- Exit status:
  - exit 1 with JSON is a result;
  - exit 1 without JSON is an error;
  - a timeout is an error;
  - `Unknown command` is `ErrNoVulns`.

**ui:**
- **Rows:** a finding counts only at the installed version, and an
  upgraded formula drops out.
- **Order:** sorting by worst severity, with UNKNOWN above LOW and
  unrecognised severities with UNKNOWN.
- **Badge:** blank, muted count, red chip, stale chip.
- **List:** the SEVERITY column and its colours, every coverage note and
  every empty note.
- **Details:**
  - the block: versions, commit hashes, a mix, none;
  - the upgrade hint;
  - no block for a clean formula, and the block on other views.
- **Scheduling:**
  - nil scans, 23 hours ago doesn't, 25 hours ago does;
  - `r` on Skunky scans regardless, and `r` elsewhere doesn't;
  - after `ErrNoVulns`, nothing scans again until a full-refresh job (`U`)
    finishes.
- **Snapshot:** the scan survives a save and a load.

**Goldens:**
- These change, and nothing else may:
  - the sidebar on every screen (one more entry);
  - Help, if it shows the sidebar;
  - Installed's menu, whose details (lua-language-server) gain the
    Vulnerabilities section.
- There's a new golden for the Skunky view.
- Check the diff before updating.

`make cover` stays at 90% or above in total and per package, and
`golangci-lint` reports `0 issues.`
