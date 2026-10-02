# ghist

Explore how a file evolved, directly from your terminal.

```sh
go mod tidy
go build -o bin/ghist ./cmd/ghist
./bin/ghist path/to/file.go
```

Requires Go 1.26+ to build, Git (core tests run with 2.55), and an interactive terminal. Run
`go mod tidy` to download dependencies before the first build. No remote Git
service, credentials, or network connection is needed at runtime.

## Usage

```sh
ghist internal/order/service.go
ghist /absolute/path/to/repo/file.go
ghist -- -filename
ghist --help
```

Paths are relative to your current directory. Run from a repository subdirectory
or pass an absolute path. Deleted files work if their path still has Git history.
The repository's HEAD is captured when you open the file; uncommitted changes are
not included. The tool never checks out a revision or modifies your repository.

| Key | Action |
| --- | --- |
| `↑` / `k`, `↓` / `j` | Select a commit or scroll content |
| `Tab` | Switch focus between history and preview |
| `Enter` | History → diff → file |
| `d`, `f` | Open diff or full file |
| `s` | Toggle split/unified diff (split requires 100 columns in the diff panel) |
| `p`, `n` | Older / newer file version |
| `PgUp`, `PgDn`, `Home`, `End` | Page / jump in the list or content |
| `Esc` | Focus history from preview; file → diff → history |
| `q`, `Ctrl+C` | Cancel loading and quit |

At 110 columns or wider, History shows a 45% commit list and 55% diff preview
for the selected file. Selection automatically refreshes the preview after a
100 ms pause. Arrow/page keys scroll the focused panel; click to focus or select
a commit, and use the mouse wheel over either panel to navigate it. `p`/`n`
always changes the selected version. Narrower terminals show the full-width
list; `Enter` or `d` opens the full-screen diff.

Diffs use readable foreground text on muted addition/deletion backgrounds,
with `+`/`-` markers and old/new line numbers. At 100 columns of available diff
space, the default is Before/After columns; narrower panels use unified rows.
Press `Enter` or `d` to expand the preview, then `s` to switch layouts. Wrapped
continuations use `↪` and stay aligned across the two sides.

Diffs compare a commit with its first parent, or an empty tree for the root
commit. Merge commits are labeled accordingly. Previous/next follows the file
history list, not the commit's parent pointer. File renames follow Git's rename
detection heuristics; snapshots use the name at the historical commit.

Deleted-file snapshots and binary files show explanatory messages. Empty files
are distinguished from deleted files. Content wraps to the terminal width and
can be scrolled. Resize a very small terminal to see the full interface.

## Development

For interactive hot reload on Linux/macOS, install
[Air](https://github.com/air-verse/air) (tested with v1.67.4):

```sh
go install github.com/air-verse/air@v1.67.4
make dev FILE=internal/tui/view.go
# Paths containing spaces also work:
make dev FILE="/path/to/repo/my file.go"
```

Ensure Go's binary install directory (`go env GOBIN`, or `$(go env GOPATH)/bin`
when GOBIN is empty) is on PATH. Use a file with committed Git history.
Air watches Go sources and module files, builds into ignored `tmp/`, and
restarts the app after changes. Each restart resets the selected commit and
scroll position. `q` exits the TUI; then `Ctrl+C` stops the Air watcher.
The development helper reconnects the foreground terminal and restores its
state on restart. Build errors are recorded in `tmp/build-errors.log`.
You can also run `FILE=path/to/file air -c .air.toml` directly.


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
