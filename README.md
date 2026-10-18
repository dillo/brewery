# brewery

A terminal dashboard for Homebrew. See what's installed, what's outdated and
what's running; upgrade, uninstall, pin, start and stop services, clean up,
and find new formulae, apps and fonts to install — with brew's own output
live in a drawer, and any brew command a `:` away.

![brewery's Home: counts of what's installed, outdated and running, doctor's warnings and recent jobs](docs/screenshots/home.png)

## Install

```sh
brew install dillo/tap/brewery
```

Then run `brewery`.

## A look around

Everything installed, with what it needs and what needs it.

![Top-level: a list of packages with versions, sizes and status, and a preview of git with its dependency tree](docs/screenshots/installed.png)

Find popular and rising packages, and install them.

![Popular tools: Homebrew's most-installed tools, with a preview of pipx showing what installing it would add and its install counts](docs/screenshots/discover.png)

brew's own output, live — answer its questions right in the drawer.

![An upgrade of gh running, with brew's output in the drawer below the Outdated list](docs/screenshots/upgrade.png)

## Requirements

- macOS (Apple Silicon or Intel)
- Homebrew 7.0 or newer
- A [Nerd Font](https://www.nerdfonts.com) for icons — optional. Without one,
  run `brewery --no-icons`, or set `BREWERY_NO_ICONS` to any value.

## Sidebar

- **Home**: counts, doctor's warnings, recent jobs
- **Packages**: All, Top-level, Formulae, Casks, Dependencies, Outdated, Pinned,
  each with the selected package's details beside it
- **Manage**: Services, with each service's log; Taps, where `x` untaps one
- **Discover**: Popular tools, Popular apps, Rising, New, Fonts, and Search all
  for the whole catalogue
- **Tools**: History, every job brewery ran, with its command and output;
  Maintenance, where `enter` runs doctor, a cleanup or `brew update`

From 122 columns the sidebar, list and details sit side by side. In a narrower
window the columns slide with the focus, so `h` and `l` turn pages.

## Keys

Press `?` anywhere for the keys that work there.

| Key | What it does |
|---|---|
| `h` `l` | move between the sidebar, the list and the details; in a narrow window, a page at a time |
| `1`–`5` | Home, Packages, Manage, Discover, Tools |
| `[` `]` | previous or next entry |
| `h` `l` on Home | along the tiles, and from Doctor to Recent jobs; `h` from the first tile or Doctor goes back to the sidebar |
| `j` `k` on Home | down from the tiles to Doctor (then scroll it), back up to the tile; `enter` opens a tile, or History from Recent jobs |
| `j` `k`, `gg` `G`, `ctrl+d` `ctrl+u` | move in a list |
| `/` | filter the list |
| `u` | upgrade (Packages) |
| `A` | upgrade all (Outdated) |
| `x` | uninstall (Packages, except Outdated), untap (Taps) |
| `p` | pin or unpin (Packages, except Outdated) |
| `space` | mark several for `u` or `x` |
| `s` `S` `R` | start, stop, restart a service; `L` follows its log |
| `a` | install (Discover) |
| `l` `tab` `enter` | details, then `j` `k` scroll (on Maintenance, `enter` runs the row) |
| `y` | copy the name |
| `gx` | open the homepage |
| `:` | run any brew command; `:jobs` opens History |
| `r` | refresh |
| `U` | `brew update` |
| `c` | clean up |
| `o` | show or hide brew's output |
| `i` | type into the running job (passwords, questions) |
| `ctrl+c` | stop the running job |
| `q` | quit |

## Files

- `$XDG_CACHE_HOME/brewery` (default `~/.cache/brewery`): the last snapshot
  of your packages, so brewery opens instantly, and Homebrew's analytics.
- `$XDG_STATE_HOME/brewery` (default `~/.local/state/brewery`): the history
  of jobs brewery ran, and crash logs.

## Building from source

```sh
make build   # ./brewery
make test
```

`make screenshots` regenerates the images above from a made-up machine
(`internal/ui/testdata/demo`). It needs Charm's
[freeze](https://github.com/charmbracelet/freeze)
(`go install github.com/charmbracelet/freeze@v0.2.2`) and the JetBrains Mono
Nerd Font (`brew install --cask font-jetbrains-mono-nerd-font`).

## Licence

MIT. See [LICENSE](LICENSE).
