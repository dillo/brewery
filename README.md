# brewery

A terminal dashboard for Homebrew. See what's installed, what's outdated and
what's running; upgrade, uninstall, pin, start and stop services, clean up,
and find new formulae, apps and fonts to install — with brew's own output
live in a drawer, and any brew command a `:` away.

![brewery's Home tab: counts of what's installed, outdated and running, doctor's warnings and recent jobs](docs/screenshots/home.png)

## Install

```sh
brew install dillo/tap/brewery
```

Then run `brewery`.

## A look around

Everything installed, with what it needs and what needs it.

![The Installed tab: a list of packages with versions, sizes and status, and a preview of git with its dependency tree](docs/screenshots/installed.png)

Find popular and rising packages, and install them.

![The Discover tab: Homebrew's most-installed tools, with a preview of pipx showing what installing it would add and its install counts](docs/screenshots/discover.png)

brew's own output, live — answer its questions right in the drawer.

![An upgrade of gh running, with brew's output in the drawer below the Outdated list](docs/screenshots/upgrade.png)

## Requirements

- macOS (Apple Silicon or Intel)
- Homebrew 7.0 or newer
- A [Nerd Font](https://www.nerdfonts.com) for icons — optional. Without one,
  run `brewery --no-icons`, or set `BREWERY_NO_ICONS` to any value.

## Tabs

1. **Home** — counts, doctor's warnings, recent jobs
2. **Installed** — everything installed, with a preview of each package
3. **Outdated** — what has a newer version
4. **Services** — `brew services`, with each service's log
5. **Taps** — the taps you've added; `x` untaps one
6. **Discover** — popular and rising packages from Homebrew's analytics, what's
   new, and the whole catalogue to search

## Keys

Press `?` anywhere for the keys that work there.

| Key | What it does |
|---|---|
| `1`–`6`, `[` `]` | switch tabs |
| `h` `l` | move between Home's tiles; `enter` opens one |
| `j` `k`, `gg` `G`, `ctrl+d` `ctrl+u` | move in a list |
| `/` | filter the list |
| `f` `F` | next or previous view (Installed, Discover) |
| `u` | upgrade (Installed, Outdated) |
| `A` | upgrade all (Outdated) |
| `x` | uninstall (Installed), untap (Taps) |
| `p` | pin or unpin (Installed) |
| `space` | mark several for `u` or `x` |
| `s` `S` `R` | start, stop, restart a service; `l` follows its log |
| `a` | install (Discover) |
| `tab` `enter` | preview, then `j` `k` scroll |
| `y` | copy the name |
| `gx` | open the homepage |
| `:` | run any brew command |
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
(`go install github.com/charmbracelet/freeze@latest`) and the JetBrains Mono
Nerd Font (`brew install --cask font-jetbrains-mono-nerd-font`).

## Licence

MIT. See [LICENSE](LICENSE).
