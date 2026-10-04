# ghist

<img width="1920" height="1080" alt="image" src="https://github.com/user-attachments/assets/e9c81cb2-6ccd-4cf8-84dd-647190168b77" />

Explore how a file evolved, directly from your terminal.

## Installation

Install [Go 1.26+](https://go.dev/dl/) and [Git](https://git-scm.com/downloads), then
run the same commands on Linux, macOS, WSL, or **Git Bash on Windows**:

```sh
git clone --depth 1 --branch develop https://github.com/lcaohoanq/ghist.git ~/.ghist
~/.ghist/install
```

The installer is currently available on the `develop` branch.

The installer builds the checked-out source for your OS and CPU, downloads Go
dependencies as needed, and installs `ghist` into `~/.local/bin` (`ghist.exe` on
Windows). It needs neither Air nor sudo, and does not edit shell profiles. On
Windows, use Git Bash for these installation commands, not PowerShell or cmd.exe.

If that directory is not on PATH, add this to your shell configuration:

```sh
# bash, zsh, or Git Bash
export PATH="$HOME/.local/bin:$PATH"
```

For fish, use `fish_add_path "$HOME/.local/bin"`. Then run `ghist --help`.
Choose another destination with `~/.ghist/install --bin-dir /path/to/bin`.

Update an existing installation:

```sh
git -C ~/.ghist pull --ff-only
~/.ghist/install
```

The installed version follows your checkout. To install the release **before Fast
mode**, clone its tag instead (into a separate directory):

```sh
git clone --depth 1 --branch v0.1.0 https://github.com/lcaohoanq/ghist.git ~/.ghist-v0.1.0
```

That historical tag predates this installer. For it, use the prebuilt binaries
from [GitHub Releases](https://github.com/lcaohoanq/ghist/releases/tag/v0.1.0), or
build inside its checkout with `go build -o ghist ./cmd/ghist`. The v0.1.0 release
always follows renames and does not have the `--follow` option described below.

Running ghist requires Git (core tests use 2.55) and an interactive terminal.
[fzf](https://github.com/junegunn/fzf#installation) is optional for file selection;
passing a file works without it. No remote Git service, credentials, or network
connection is needed to load history.

## Usage

```sh
ghist                            # fuzzy-find a file, then Enter to open history
ghist internal/order/service.go       # fast: history at the current path
ghist --follow internal/order/service.go # include history before renames
ghist --follow                       # picker with rename tracking
ghist /absolute/path/to/repo/file.go
ghist -- -filename
ghist --help
```

With no arguments, ghist uses [fzf](https://github.com/junegunn/fzf#installation)
to fuzzy-find a file across the current repository, including when launched from
a subdirectory. Type part of a name or path, use the arrow keys to select, and
press `Enter` to open History. `Esc` / `Ctrl+C` cancels without an error.
Only this mode requires `fzf`; passing a file works without it. No `rg` is needed.
The picker uses its own options rather than `FZF_DEFAULT_OPTS` or its options file.

Candidates are files in the captured `HEAD`, including tracked dotfiles and
symlinks, excluding submodules and files added only to the index or working tree.
A file deleted locally is still selectable; a file deleted from `HEAD` can still
be explored by passing its path directly. In picker mode, `Esc` from History
returns to file selection (also when the preview has focus). From full-screen
File/Diff, `Esc` first goes back through Diff/History. `q` / `Ctrl+C` quits ghist.
Returning to the picker starts a fresh search and refreshes the files from HEAD;
opening a file starts a fresh History view. Passing a file directly keeps the
existing `Esc` behavior.

Paths are relative to your current directory. Run from a repository subdirectory
or pass an absolute path. Deleted files work if their path still has Git history.
The repository's HEAD is captured when you open the file; uncommitted changes are
not included. The tool never checks out a revision or modifies your repository.

| Key | Action |
| --- | --- |
| `↑` / `k`, `↓` / `j` | Select a commit or scroll content |
| `Tab` | Switch focus between history and preview |
| `Enter` | Toggle a focused day header; commit → diff → file |
| `Space`, click day header | Collapse / expand that day group |
| `←`, `→` | Collapse / expand the focused day group |
| `d`, `f` | Open diff or full file |
| `s` | Toggle split/unified diff (split requires 100 columns in the diff panel) |
| `L` | Load full output when a diff or file exceeds the initial 2 MiB limit |
| `p`, `n` | Older / newer file version |
| `PgUp`, `PgDn`, `Home`, `End` | Page / jump in the list or content |
| `Esc` | File → diff → history → picker (when started without a file); direct-file mode focuses history from preview |
| `q`, `Ctrl+C` | Cancel loading and quit |

At 110 columns or wider, History shows a 45% commit list and 55% diff preview
for the selected file. Selection automatically refreshes the preview after a
100 ms pause. Arrow/page keys scroll the focused panel; click to focus or select
a commit, and use the mouse wheel over either panel to navigate it. `p`/`n`
always changes the selected version. Narrower terminals show the full-width
list; `Enter` or `d` opens the full-screen diff.

History groups consecutive commits by their displayed author date, with a bold
`▾` / `▸` day header and commit count. All groups start expanded. Click a header
or focus it with the navigation keys and press `Enter` or `Space` to toggle it;
`←` collapses the current group and `→` expands it. Navigation and mouse scrolling
follow visible rows, including headers. Focusing a header keeps the last selected
commit's metadata and preview; `d` / `f` still opens that commit. `p` / `n` follows
the original version order and automatically expands the destination group.
Group state survives view changes and resizing, but resets on app restart.
Dates use each commit's recorded timezone. Git's ordering is preserved, so a date
that recurs later in history has a separate group.

Diffs use readable foreground text on muted addition/deletion backgrounds,
with `+`/`-` markers and old/new line numbers. At 100 columns of available diff
space, the default is Before/After columns; narrower panels use unified rows.
Press `Enter` or `d` to expand the preview, then `s` to switch layouts. Wrapped
continuations use `↪` and stay aligned across the two sides.

Diffs compare a commit with its first parent, or an empty tree for the root
commit. Merge commits are labeled accordingly. Previous/next follows the file
history list, not the commit's parent pointer. By default, history stays at the
selected path and rename detection is disabled. History before a rename is not
followed; a rename into this path appears as an addition. Use `--follow` to track
renames with Git's heuristics and read snapshots at their historical paths. The
header shows `Fast · Renames not followed` or `Following renames`. Place `--follow`
before the file argument; it also applies to files selected through the picker.

Deleted-file snapshots and binary files show explanatory messages. Empty files
are distinguished from deleted files. Content wraps to the terminal width and
can be scrolled. Resize a very small terminal to see the full interface.

## Large repositories

History streams into the UI as Git produces results. You can inspect loaded
commits while older history is still arriving; the footer shows progress. Once
the entire history finishes loading, it shows the commit count and total fetch
time (for example, `Fetched 120 commits in 1.234s`). This measures history loading,
including delivery of streamed batches, and does not wait for diff previews.
Returning to the picker or quitting cancels the traversal. If loading fails after
some commits arrive, they remain available with a “History incomplete” message.

Metadata and historical file paths come from one Git traversal. Rename statuses
are requested only with `--follow`. Diff and snapshot results share an in-memory LRU per opened file,
limited to 64 entries and 32 MiB of string payload. Reopening a file starts a new
session. Diff/file views initially retain up to 2 MiB of output; press `L` to load
the complete result explicitly. This limits retained output, not Git's work to
compute it. Full output bypasses the content cache.

Git still traverses history; `--follow` additionally detects renames and can be
much slower on large repositories. Fast mode keeps topological ordering while
disabling rename detection, even if Git configuration enables it. History is loaded progressively to
completion, not paginated. See [performance notes](docs/performance.md) for the
benchmark and regression checks.

## Development

For interactive hot reload on Linux/macOS, install
[Air](https://github.com/air-verse/air) (tested with v1.67.4):

```sh
go install github.com/air-verse/air@v1.67.4
make dev                              # open the file picker
make dev FILE=internal/tui/view.go
make dev PROJECT=/path/to/repo        # open another project's file picker
make dev PROJECT=/path/to/repo FILE=src/main.go
make dev PROJECT=/path/to/repo FOLLOW=1 # opt into rename tracking
# Paths containing spaces also work:
make dev FILE="/path/to/repo/my file.go"
```

Ensure Go's binary install directory (`go env GOBIN`, or `$(go env GOPATH)/bin`
when GOBIN is empty) is on PATH. Use a file with committed Git history.
`PROJECT` sets the app's working directory; it defaults to the ghist directory.
Relative `PROJECT` paths are resolved from the ghist directory, and relative
`FILE` paths are resolved from `PROJECT`. Quote paths containing spaces. Set
`FOLLOW=1` to pass `--follow`; otherwise development uses fast mode too.
Air watches Go sources and module files, builds into ignored `tmp/`, and
restarts the app after changes. Each restart resets the selected commit and
scroll position. `q` exits the TUI; then `Ctrl+C` stops the Air watcher.
The development helper reconnects the foreground terminal and restores its
state on restart. Build errors are recorded in `tmp/build-errors.log`.
You can also run `PROJECT=/path/to/repo FILE=path/to/file air -c .air.toml` directly.


```sh
make build
make check
```

- `internal/git`: Git CLI adapter and repository integration tests.
- `internal/history`: UI-independent data, adapter interface and service.
- `internal/tui`: Bubble Tea v2 state, asynchronous loading and rendering.
- `cmd/ghist`: argument parsing, terminal checks and composition.

Integration tests create local temporary Git repositories, including rename,
deletion, binary-file and merge scenarios. They require Git but no network.

The MVP includes file history, commit metadata, diffs, snapshots and navigation.
Blame, line history, search, working-tree comparisons, GUI and hosting-service
integrations are future work. See [plan.md](plan.md).
