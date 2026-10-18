# Autoremove, services cleanup and retry — design

Three items from the Brewy backlog: `brew autoremove` and `brew services
cleanup` as Maintenance rows with a preview, and running a History job
again.

## Data

### Autoremove

- `brew.Client.Autoremove(ctx) ([]string, error)` runs
  `brew autoremove --dry-run` and returns the formula names it would
  uninstall.
- `brew.ParseAutoremove(out string) []string` reads the names listed after
  the `==> Would autoremove N unneeded formula(e):` header, one per line,
  up to a blank line or the next `==>` line. No header (brew prints nothing
  when there is nothing to remove) means none.
- A non-zero exit, a timeout or a failure to run brew is a `QueryError`, as
  for the other queries.

### Unused service files

`brew services cleanup` has no dry run. From Homebrew's source
(`Library/Homebrew/services/cli.rb`) it does two things:

1. **kill_orphaned_services:** stops each running service whose service
   file is gone.
2. **remove_unused_service_files:** deletes every
   `{homebrew.*,sh.brew.*}.{plist,service,timer}` file in the user's
   service directory whose service isn't running.

Brewery copies that rule:

- `brew.Client.ServiceFiles(ctx) ([]string, error)` lists the
  `homebrew.*.plist` and `sh.brew.*.plist` files in
  `~/Library/LaunchAgents`, as full paths, sorted. The directory is a field
  on `Client` (`AgentsDir`), so tests can use a temporary directory.
  - A directory that doesn't exist means no files.
  - Any other read error is returned.
- **What counts as running:** Homebrew treats every service in
  `launchctl list` as running, including a loaded service that has
  errored. `brew services info --json` reports this as `loaded`, and gives
  the launchd label as `service_name`. `brew.Service` gains
  `Label string \`json:"service_name"\`` and
  `Loaded bool \`json:"loaded"\``.
- The UI works out the preview from those files and the services list:
  - **Files to remove:** each file whose label (its base name without
    `.plist`) isn't the `Label` of a service with `Loaded` set.
  - **Services to stop:** each service with `Loaded` set whose `File` is
    set but isn't among the files.
- **Known gap:** a service that is still loaded after its formula was
  uninstalled isn't in brew's services list, so its file would show as
  "to remove" although brew keeps it. This is rare and accepted.
- Brewery only previews the user's services. Root services
  (`/Library/LaunchDaemons`) are out of scope, as they are for the rest of
  brewery's services handling.

### Queries and snapshot

- **New queries:** `qAutoremove` and `qServiceFiles`.
- **When they run:** they're requested with `qDoctor`, `qCleanup` and
  `qSizes`, after the JSON queries on a full refresh, and on their own when
  an action lists them in `refresh`.
- **Snapshot fields:** `Autoremove []string`, `AutoremoveRan bool`,
  `ServiceFiles []string`, `ServiceFilesRan bool`.
- **Snapshot version:** `snapshotVersion` goes from 3 to 4, so an older
  cache file is ignored.
- **When the app loads a snapshot:** `loaded[qAutoremove]` is set from
  `AutoremoveRan`, and `loaded[qServiceFiles]` from `ServiceFilesRan`.

**Refreshes:**

| After | Re-runs |
|---|---|
| install, uninstall, upgrade, upgrade all, autoremove | `qAutoremove` |
| service start, stop, restart, services cleanup | `qServiceFiles` |
| a typed command | what `refreshAfter` says: a full refresh, which now includes both, or for `services` subcommands `qServices` plus `qServiceFiles` |

## Maintenance

### Rows and statuses

Rows, in order: Doctor, Cleanup, Autoremove, Services cleanup, Update
Homebrew. The constants are `maintDoctor`, `maintCleanup`,
`maintAutoremove`, `maintServices` and `maintUpdate`.

| Row | Status | Text |
|---|---|---|
| Autoremove | error | `✗ failed` (red) |
| | not loaded | `…` (Overlay0) |
| | none | `nothing to remove` (green) |
| | some | `removes N formula/formulae` (blue) |
| Services cleanup | error | `✗ failed` (red) |
| | not loaded (either `qServiceFiles` or `qServices`) | `…` (Overlay0) |
| | none | `nothing to clean` (green) |
| | some | `N unused file(s)`, or `stops N service(s)` when there are no files; both joined with ` · ` (blue) |

### Details

- **Autoremove:** "Unneeded formulae, which only other formulae needed:",
  then one name per line, then muted "enter removes them". If none:
  "nothing to remove" (OK). Errors and loading read like Cleanup's.
- **Services cleanup:** "Unused service files in ~/Library/LaunchAgents:"
  (their directory, `$HOME` shown as `~`), then each file's name. If any services would be stopped: "Running
  services whose file is gone:", then their names. Then muted "enter
  cleans up". If none: "nothing to clean" (OK).

### Enter

| Row's state | Enter does |
|---|---|
| error | the note `couldn't check · r retries` |
| not loaded | the note `still checking` |
| nothing to do | the note `nothing to remove`, or `nothing to clean` |
| something to do | a `confirmBox` |

The confirm boxes:

- **Autoremove:**
  - Title: "Remove unneeded formulae?"
  - Body: the names.
  - Runs `action{title: "autoremove", args: ["autoremove"], refresh:
    [qInstalled, qOutdated, qSizes, qAutoremove]}`.
- **Services cleanup:**
  - Title: "Clean up services?"
  - Body: the files and services as in the details.
  - Runs `action{title: "services cleanup", args: ["services",
    "cleanup"], refresh: [qServices, qServiceFiles]}`.

**Help:** the Maintenance group's enter line reads "run the selected task"
(the longer list of tasks didn't fit the help box).

## History: run a job again

`R` on History (list or details focus) with a job selected opens a
confirm box:

- **Title:** "Run again?"
- **Body:** `brew <args>`.

**Remembering takeovers:** `jobs.Job` gains `Takeover bool`, and
`jobs.Record` gains `Takeover bool \`json:"takeover,omitempty"\``, which
`NewRecord` copies. `a.takeover` sets it on the job it starts, so a line
forced full screen with `!` is remembered too; its args alone don't say so.
Older history files read it as false.

On yes:

- If `r.Takeover || takesOver(args)`, the job runs through
  `a.takeover(args)`.
- Otherwise it runs
  `a.run(action{title: strings.Join(args, " "), args: args, refresh:
  refresh, full: all})`, where `refresh, all := refreshAfter(args)`.
  - `typed` stays false, so brewery's usual environment applies (no
    auto-update).
- An `install` already in the queue gets the same note as at the prompt
  (`… is already queued`).

**Empty history:** `R` with no rows does nothing.

**Help and hints:**

- **Help:** History's group gains `{"R", "run the job again"}`.
- **List hint:** History's hint gains `R run again`.
- **Details hint:** History's details hint gains `R run again`.

## Testing

### brew package

- `ParseAutoremove`:
  - several names;
  - one formula (singular header);
  - nothing;
  - a header followed by a later `==>` line.
- `Client.Autoremove`:
  - with the fake runner: the args are `autoremove --dry-run`;
  - an error exit is a `QueryError`.
- `Client.ServiceFiles`:
  - a temp dir with matching and non-matching files;
  - a missing directory is empty;
  - an unreadable directory is an error (root skip).

### ui

- **Maintenance:**
  - the five rows' statuses in each state;
  - each row's details;
  - Enter on each new row: note or confirm box, and the action it runs
    (args and refresh).
- **Service-file preview:**
  - a file whose service isn't loaded is listed;
  - a loaded service's file isn't, even one in error;
  - a loaded service with a missing file is "to stop".
- **Refreshes:** uninstall and service start request the new queries.
- **History `R`:**
  - confirm then run: the args, `typed` false, and the refresh;
  - a takeover job, by its args and by `Record.Takeover` (a `!` line);
  - an empty history;
  - an already-queued install.
- **Goldens:** Maintenance and help are updated.
- **Coverage:** the total and every package stay at 90% or above.
