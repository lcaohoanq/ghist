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

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Update an existing installation:

```sh
git -C ~/.ghist pull --ff-only
~/.ghist/install
```

Running ghist requires Git (core tests use 2.55) and an interactive terminal.
[fzf](https://github.com/junegunn/fzf#installation) is optional for file selection;
passing a file works without it. No remote Git service, credentials, or network
connection is needed to load history.

## Usage

```sh
ghist                            # fuzzy-find a file, then Enter to open history
ghist internal/order/service.go
ghist /absolute/path/to/repo/file.go
ghist -- -filename
ghist --help
```

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

## Development

For interactive hot reload on Linux/macOS, install
[Air](https://github.com/air-verse/air) (tested with v1.67.4):

```sh
go install github.com/air-verse/air@v1.67.4
make dev                              # open the file picker
make dev FILE=internal/tui/view.go
make dev PROJECT=/path/to/repo        # open another project's file picker
make dev PROJECT=/path/to/repo FILE=src/main.go
# Paths containing spaces also work:
make dev FILE="/path/to/repo/my file.go"
```

```sh
make build
make check
```

- `internal/git`: Git CLI adapter and repository integration tests.
- `internal/history`: UI-independent data, adapter interface and service.
- `internal/tui`: Bubble Tea v2 state, asynchronous loading and rendering.
- `cmd/ghist`: argument parsing, terminal checks and composition.
