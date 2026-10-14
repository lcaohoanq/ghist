# ghist — File History

Explore how a file evolved through Git commits, following renames, directly in VS Code.

Right-click a file in Explorer or the editor and choose **ghist: File History**.
A single **ghist History** editor tab shows a timeline table, newest first, with
Message, Author, Time and Hash columns. Select a commit to preview its changes in
VS Code’s native diff editor beside History; focus stays in the table and later
previews reuse the adjacent editor group. Use **Open file** to read the complete
historical file. **Refresh** captures the latest HEAD and keeps the selection if
the commit still exists.
You can also run **ghist: File History** from the Command Palette.

History automatically follows the active working file after a 250 ms debounce.
**Pin file** keeps the current history; **Unpin file** follows the most recently
opened working file. Snapshot and diff editors do not change the tracked file.
Automatic loading starts only after you open History and pauses while its tab is
hidden. **Choose file** lets you browse to another file, including outside the
workspace. Loading another file clears the selection without opening a diff.

Use **↑ / ↓** to select commits, **Enter** to preview a diff, and **Ctrl/Cmd+F** to
focus local search. Search matches messages, authors, emails and hashes without
running Git again. **Newer / Older** move through the filtered results. Added,
deleted, renamed and merge commits have badges; merges compare with their first
parent. The timeline connects successive versions of this one file, rather than
showing a repository branch graph. Merge commits use a double-ring marker on the
same timeline; their other parent is not drawn as a branch.

The table uses VS Code’s theme and fonts, including high contrast. Below 700 px,
Author and Hash move into row tooltips. Long histories use virtual scrolling.
Avatars are author initials generated locally; all panel assets are bundled.

Selecting a commit also looks up associated GitHub pull requests or GitLab merge
requests and shows `PR #…` / `MR !…` links in that row. Click a link to open it
in your browser. This optional lookup uses the repository's `origin` and the
authenticated `gh` or `glab` CLI. A missing CLI, login, origin or network never
prevents local history and diffs from working.

History follows committed changes up to the HEAD captured when the list loads.
It does not show uncommitted edits. Opening history, diffs and old files never
checks out a revision or changes your working files. Rename diffs use each
side's historical path; merge diffs compare against the first parent.

## Requirements

- VS Code 1.100 or newer on Linux, macOS or Windows.
- Git available on PATH in the extension host.
- A trusted workspace and a Git working tree with commits.

Installable VSIX packages include the Go backend: Go and the terminal ghist
program are not needed to use the extension. No fzf, login or network is needed
to read history. With Remote SSH, WSL, Dev Containers or Codespaces, install the
extension on the remote side; Git and the backend run there. Browser-only
vscode.dev without a remote extension host and virtual workspaces are unsupported.

Binary or non-UTF-8 revisions cannot be opened as text. Revisions larger than
2 MiB are refused so the diff never compares incomplete content. History requests
can be cancelled; each backend request has a 120-second timeout and a 64 MiB
response limit.

## Development

From this directory, with Node.js 22+ and the Go version in the root `go.mod`:

```sh
npm ci
npm run build
npm test
npx playwright install chromium
npm run test:webview
```

Open `extensions/vscode` as the VS Code workspace and press **F5** to launch an
Extension Development Host. Open a Git repository in that window and use
**ghist: File History** on a tracked file.

Run the smoke test in a real VS Code extension host:

```sh
npm run test:extension
```

The smoke test clicks the real History webview with Playwright, checks keyboard
preview, pin/unpin and automatic following, verifies that focus stays in History
and exactly one adjacent diff group is reused, then opens historical files. The
DOM suite covers search, long text, empty/loading/error states, four themes and
10,000 commits. CI runs both suites and preserves theme preview screenshots.
Set `CHROMIUM_PATH=/path/to/chromium` to use a local browser for DOM tests.

The test downloads VS Code unless `VSCODE_EXECUTABLE_PATH` points to an existing
Electron executable (not its shell launcher). On headless Linux, run it with
`xvfb-run -a npm run test:extension`. Tests create isolated temporary repositories
and a temporary VS Code user profile.

## Package and install

```sh
npm run package
code --install-extension ghist-<target>-0.2.0.vsix
```

The package command builds for the current machine by default. To cross-compile:

```sh
npm run package -- linux-x64
npm run package -- linux-arm64
npm run package -- darwin-x64
npm run package -- darwin-arm64
npm run package -- win32-x64
npm run package -- win32-arm64
```

Each VSIX contains only the backend for its declared target. Building another
target replaces the development backend; run `npm run build` before pressing F5
or running tests on your own machine again. These commands create local artifacts;
they do not publish to the Marketplace. The manifest's publisher identifier must
be registered by the maintainer before a Marketplace release.

`ghist.backendPath` can point to an absolute path to a custom `ghist-backend`
executable, for development or unsupported platforms. It must implement the same
protocol version as the extension.

## Layout

- `src/`: TypeScript commands, History panel and controller, virtual documents and subprocess client.
- `../../cmd/ghist-backend/`: one-request JSON backend using the existing Git/history packages.
- `src/webview/` and `media/`: bundled TypeScript client, SVG timeline and theme CSS.
- `tests/browser/`: Playwright DOM tests.
- `scripts/`: backend builds, platform-specific VSIX packaging and extension-host tests.
- `src/test/`: subprocess/controller integration tests and VS Code smoke test.

The backend reads one protocol-versioned JSON request from stdin, writes one JSON
response to stdout and exits. It supports `history` (absolute `filePath`) and
`snapshot` (absolute repository `root`, commit hash `revision`, repository-relative
Git `path`). Errors are structured JSON. Snapshots address immutable commit hashes,
so an existing editor remains stable when history is refreshed.
