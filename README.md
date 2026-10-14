<h1 align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="assets/hero-dark.webp"><img src="assets/hero.webp" alt="ghist: &quot;How did this code get here?&quot; A poster with the ghist wordmark and that question beside a tilted terminal window. The window lists the commits of one file with the second commit selected and shows its diff below. A sticker reads read only." width="100%"></picture></h1>

**ghist answers "How did this code get here?" from your terminal.** Pick a file, step back through every commit that touched it, and read what each one changed, renames included. It only reads committed history from HEAD: it never checks out a revision and never touches your files.

## What it looks like

<img width="1920" height="1080" alt="ghist in a terminal: the commits of one file on the left, and the diff of the selected commit on the right with the old version beside the new one" src="https://github.com/user-attachments/assets/e9c81cb2-6ccd-4cf8-84dd-647190168b77" />

## Operate the workflow

You enter on the history of a file, open the diff of any commit, open the file as it was then, and walk back and forth through its versions with one key each.

<picture><source media="(prefers-color-scheme: dark)" srcset="assets/workflow-dark.webp"><img src="assets/workflow.webp" alt="Workflow of ghist. Running ghist with a file, or running ghist alone and picking a file with fzf, both open the History screen. From History, Enter or d opens the Diff screen. From Diff, Enter or f opens the File screen. Esc steps back from File to Diff and from Diff to History. On any screen, p opens the older version, n the newer version, o opens the first pull request or merge request, and q quits." width="100%"></picture>

<details>
<summary>The workflow as text</summary>

1. Start with `ghist <file>`, or run `ghist` alone and choose a file with fzf. Both open the History screen.
2. On History, select a commit. `Enter` or `d` opens its diff. On Diff, `Enter` or `f` opens the whole file as it was at that commit.
3. `Esc` steps back: File to Diff, Diff to History. On History, `Esc` returns to the picker only when you started without a file.
4. On any screen, `p` moves to the older version of the file, `n` to the newer one, `o` opens the first pull request or merge request in a browser, and `q` quits.

</details>

## Follows renames

Git sees a renamed file as two paths; ghist lists one history. Pick a commit from before the rename and it reads the file under its old path.

<picture><source media="(prefers-color-scheme: dark)" srcset="assets/renames-dark.webp"><img src="assets/renames.webp" alt="A file is committed five times. It is called draft.go in the two oldest commits and service.go in the three newest, with a renamed marker between them. Below, the ghist History screen lists those five commits as one history, with an older commit selected and its path shown as draft.go. A note says Git's rename detection decides which renames can be followed. The file names in the picture are made up." width="100%"></picture>

## Look, do not touch

ghist only reads. It asks Git for the history, the diffs and the files, and it never checks out a revision or writes a file in your repository. It shows committed history up to the HEAD it read when it opened.

<picture><source media="(prefers-color-scheme: dark)" srcset="assets/readonly-dark.webp"><img src="assets/readonly.webp" alt="Two panels. ghist asks Git: git rev-parse for your repository and its HEAD, git log --follow for the history with renames, git diff-tree for what a commit changed, git show for the file as it was, git ls-tree for the files in the picker. ghist never: checks out a revision, writes a file in your repository, needs a network to load history. Below, a timeline of commits is marked shown up to HEAD, which is read once when ghist opens. Commits made later and changes you have not committed are marked not shown." width="100%"></picture>

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

PR/MR lookup is optional. Install and authenticate
[GitHub CLI (`gh`)](https://cli.github.com/) or
[GitLab CLI (`glab`)](https://gitlab.com/gitlab-org/cli) to show reviews for the
selected commit. GitHub.com, GitLab.com, and authenticated self-hosted instances
are supported. A missing CLI, login, `origin`, or network connection never
prevents local history, preview, diff, file, or picker features from working.

<picture><source media="(prefers-color-scheme: dark)" srcset="assets/reviews-dark.webp"><img src="assets/reviews.webp" alt="Optional PR and MR lookup in four steps. The commit selected in History. The host, which ghist finds by reading origin: GitHub or GitLab, including self-hosted. gh or glab, whichever is signed in for that host, asks it for the PRs or MRs that contain the commit. The first PR or MR is shown, and pressing o opens it in the browser. A note says that without gh or glab, a sign-in, an origin or a network, history, diff, file and picker work as before." width="100%"></picture>

## Usage

Prefer the editor? The [VS Code extension](extensions/vscode/README.md) adds a
wide **ghist History** tab with a file timeline, local search, PR/MR links, automatic file
following and pinning, native diffs beside it, and read-only historical files. See its README
for local VSIX packaging, installation, and development.

```sh
ghist                            # fuzzy-find a file, then Enter to open history
ghist internal/order/service.go
ghist /absolute/path/to/repo/file.go
ghist -- -filename
ghist --help
```

<details>
<summary>All keys</summary>

| Key | Action |
| --- | --- |
| `↑` / `k`, `↓` / `j` | Select a commit or scroll content |
| `Tab` | Switch focus between history and preview |
| `Enter` | Toggle a focused day header; commit → diff → file |
| `Space`, click day header | Collapse / expand that day group |
| `←`, `→` | Collapse / expand the focused day group |
| `d`, `f` | Open diff or full file |
| `o` | Open the first displayed pull request / merge request in a browser |
| `s` | Toggle split/unified diff (split requires 100 columns in the diff panel) |
| `L` | Load full output when a diff or file exceeds the initial 2 MiB limit |
| `p`, `n` | Older / newer file version |
| `PgUp`, `PgDn`, `Home`, `End` | Page / jump in the list or content |
| `Esc` | File → diff → history → picker (when started without a file); direct-file mode focuses history from preview |
| `q`, `Ctrl+C` | Cancel loading and quit |

</details>

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

GitHub Actions runs `make check` (vet and tests with the race detector) and
`make build` on Ubuntu for every push and pull request. The CI workflow uses the
Go version from `go.mod` and can also be started manually from the Actions tab.

<picture><source media="(prefers-color-scheme: dark)" srcset="assets/layers-dark.webp"><img src="assets/layers.webp" alt="How the code is organized. cmd/ghist reads the arguments, checks the terminal, builds the pieces and starts the interface. internal/tui holds the Bubble Tea state, asynchronous loading and rendering. internal/git is the Git CLI adapter that reads history, diffs and files. Both internal/tui and internal/git depend on internal/history, which has the data and the rules, no interface and no Git, and defines what an adapter must do. internal/tui also uses internal/hosting for the optional PR and MR lookup through gh or glab. internal/git runs your own git CLI, which only reads." width="100%"></picture>

- `internal/git`: Git CLI adapter and repository integration tests.
- `internal/history`: UI-independent data, adapter interface and service.
- `internal/tui`: Bubble Tea v2 state, asynchronous loading and rendering.
- `cmd/ghist`: argument parsing, terminal checks and composition.

The pictures in this README are drawn from HTML in `assets/source/`. The header comment of `assets/source/figures.html` has the command that renders them.
