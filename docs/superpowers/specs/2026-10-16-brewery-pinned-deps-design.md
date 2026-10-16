# brewery: pinned dependencies

Date: 2026-10-16. Status: approved in conversation; becomes Plan 7¾ on `feat/pinned-deps`. The release stays Plan 8.

## Goal

Say when a pinned dependency will make brew refuse an install or upgrade, before the user starts a job that fails.

## What brew does (Homebrew 7.0.9, read from its source)

- Every install and upgrade runs `FormulaInstaller#check_install_sanity` (`formula_installer.rb:372`, in `prelude`).
- It walks the formula's runtime dependencies recursively (`:478-484`). Any dependency that is **pinned** and **not satisfied** fails it (`:494-497`). With no minimum version, "satisfied" means the latest version is installed (`dependency.rb:82`), so a pinned dependency that is outdated always fails.
- The error (`:509-511`): ``You must `brew unpin openssl@3` as installing curl requires the latest version of pinned dependencies.``
- A pinned dependency that's up to date is no problem. `brew upgrade` with no arguments skips pinned packages, and anything depending on a pinned, outdated one hits the same check.
- This is the formula installer. Casks aren't checked this way, so brewery checks formulae only.

Not reproduced live: nothing on the user's machine is pinned and outdated, and reproducing it needs a real install.

## Terms

A **blocker** of a formula is an installed package that is in its dependencies (recursive), pinned, and outdated.

`state.Index.PinnedBehind(names ...string) []string` returns, sorted and without duplicates, those of `names` that are installed, pinned and outdated (looked up by full name).

## 1. Installed preview

- The Dependencies tree labels each pinned node `<name> <pin icon> pinned`, and a pinned, outdated node adds ` · <version> → <latest>` in the Warn style.
- For a formula with blockers, a Warn line follows the tree: `⚠ upgrading needs openssl@3 unpinned` (names joined with ", ").

## 2. Upgrades

- **`u`** on Installed or Outdated, for an outdated, unpinned formula with blockers: no job; it flashes `curl needs openssl@3 unpinned first` (names joined with ", ").
- **Marked upgrades** leave out formulae with blockers. If some are left out, the flash after starting says `skipped curl: needs openssl@3 unpinned`; if all are, nothing starts and it says that alone.
- **Upgrade all (`A` on Outdated)**: the dialog adds a line `brew will refuse curl, wget: they need openssl@3 unpinned` (`it needs` for one; all blocked formulae, and the union of their blockers). `y` still runs `brew upgrade`; brew upgrades the rest.

The check uses the dependencies of the installed version. An upgrade can add or drop dependencies, so for upgrades it's a strong hint, not a guarantee.

## 3. Discover install dialog

- Once brew has answered about a formula, if `PinnedBehind(details.Needs...)` isn't empty, the dialog is `Can't install <name>`, with the body `It needs openssl@3, which is pinned and outdated.` / `Unpin it on Installed first, or brew will refuse.` (`them`/`are` for several), and only `n close`.
- Before brew answers, the dialog is as today ("checking dependencies…"); when the answer arrives, the open dialog updates as it already does.
- `Needs` comes from `brew deps` for the version being installed, so a refusal is never wrong; `brew deps` without `--full-name` can name a tap formula's same-tap dependency by its short name, which the full-name lookup misses, so the worst case is a missed warning.

## Testing

Unit tests in `state` (PinnedBehind) and `ui` (preview labels and warning; `u` on both tabs; marked upgrade partly and fully blocked; the upgrade-all (`A`) dialog line; the Discover dialog before and after brew answers). Tests that need a pinned, outdated dependency build one from `fixtureSnapshot`, in both the app's snapshot and the fake source (a refresh after a job reloads from the source). No real installs.
