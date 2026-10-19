# Reinstall, link and fetch on Installed: design

The last practical item from the Brewy backlog. `reinstall`, `link`,
`unlink` and `fetch` can only be typed at `:` today. This adds keys for
them on Installed, and `f` on Outdated.

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

## Keys

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

## Help and hints

**Help** adds these rows, after `p` on Installed:

| Group | Key | Text |
|---|---|---|
| Installed | `R` | `reinstall` |
| Installed | `L` | `link or unlink` |
| Installed | `f` | `fetch the newer version` |
| Outdated | `f` | `fetch the newer version` |

The `space` text on Installed becomes `mark for u/x/R/f on several`, and
on Outdated `mark for u/f on several`.

**Hints:**

| Where | Hint |
|---|---|
| Installed list | `u upgrade · x uninstall · p pin · R reinstall · l details` |
| Installed details | `j/k scroll · h back to the list · u upgrade · x uninstall` (unchanged) |
| Outdated | `u upgrade · A upgrade all · f fetch` |

## Testing

**brew:**
- `ParseInfo` reads `linked_keg` (string or null) and `keg_only`.
- The fixture `internal/brew/testdata/info.json` gets `"linked_keg"` on
  fd, and `"keg_only": true` with a null `linked_keg` on one formula.
  Check that the other tests still pass.

**ui:**
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

**Goldens and checks:**
- The goldens change only for the details row and hints. Check the diff
  before updating.
- `make cover` stays at 90% or above in total and per package.
- `golangci-lint` reports `0 issues.` with the complexity limits at 20.
