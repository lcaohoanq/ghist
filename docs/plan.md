# ghist — Development Plan

> Explore how a piece of code evolved over time.

## Approved MVP

The project, executable, documentation and Go module use **ghist**.
Module: `github.com/lcaohoanq/ghist`; entrypoint: `cmd/ghist/main.go`.

Implement phases 0–4 as one MVP:

```text
ghist path/to/file
  → file history
  → selected commit and metadata
  → diff
  → file snapshot
  → older/newer version
```

Only committed data reachable from HEAD is included. HEAD is resolved once when
opening the repository. Comparing against uncommitted working-tree changes is
outside the MVP.

## Architecture

```text
cmd/ghist → internal/tui → internal/history ← internal/git
```

- Go 1.26, Git CLI, Bubble Tea v2.
- `internal/history` owns domain types, the repository interface and services.
- `internal/git` implements the interface using `exec.CommandContext`.
- `internal/tui` consumes domain data through an application service interface.
- Neither the Git adapter nor history service knows about Bubble Tea.
- No direct parsing of `.git`, external hosting APIs, cache or indexing.

### Domain and service contracts

- `Commit`: full/short hash, author, email, author date and subject.
- `FileVersion`: commit, first parent, merge indicator, paths before/after the
  change and Git change status. An absent path represents addition/deletion.
- `FileHistory`: requested path, captured HEAD and ordered versions.
- `FileSnapshot`: commit, historical path, content, existence and binary status.
- `FileDiff`: commit, path, comparison parent and patch.
- Services: `ExploreFile`, `GetDiff`, `GetSnapshot`; `FileHistory.Move` clamps
  previous/next navigation to the available versions.

## Git behavior

- Accept a relative or absolute file path. Relative paths use the caller's cwd.
- Discover the repository from the file's existing parent, including deleted
  files whose original parent directories no longer exist.
- Reject directories. Distinguish missing Git, no repository, no commits and no
  file history. Do not require the requested file to exist in the working tree.
- Use `git log --follow` with NUL-delimited metadata and topological ordering.
  Include merge changes relative to the first parent, without emitting patches
  into the metadata response.
- Track historical names through rename records. Git's rename heuristics define
  which renames can be followed.
- Compare each commit against its first parent; compare root commits against an
  empty tree. Include both rename paths in diff requests.
- Read snapshots using the path at the selected commit. Deleted snapshots show
  an explanation; empty files and binary files have separate states.
- Pass Git arguments directly, use literal pathspecs, disable pager, Git colors,
  external diff and text conversion, and keep diagnostics attached to errors.
- All exploration is read-only. Never checkout a revision or write user files.

## TUI behavior

Three views: History, Diff and File. Metadata identifies the commit, author,
date with timezone, historical path and comparison parent. Merges are labeled.

| Key | Behavior |
| --- | --- |
| `↑/k`, `↓/j` | Select commit or scroll content |
| `Enter` | History → Diff → File |
| `d`, `f` | Open Diff / File |
| `p`, `n` | Older / newer entry, preserving the current view |
| `PgUp`, `PgDn`, `Home`, `End` | Page / jump through list or content |
| `Esc` | File → Diff → History, preserving selection |
| `q`, `Ctrl+C` | Cancel work and quit cleanly |

Navigation stops at either end of the list. Previous/next means adjacent file
history entries, not the selected commit's parent pointer.

Git work runs asynchronously. Cancel superseded detail requests and ignore
results with stale request IDs. Show loading and error states. Wrap and scroll
content, handle window resizing, distinguish additions/deletions with colors,
and strip repository-controlled terminal sequences before rendering.

Require interactive stdin/stdout for exploration. `--help` works without a TTY.
No public debug-history command or noninteractive output format in the MVP.

## Implementation and validation

Implementation order:

1. Bootstrap module, CLI and friendly errors.
2. Build Git adapter, domain/service and integration tests.
3. Add history TUI and selection.
4. Add diff, snapshot and older/newer navigation.
5. Validate the complete terminal workflow.

Acceptance tests:

- Temporary repository: create → modify → rename file → modify → delete.
- Correct snapshot and diff on both sides of rename, including root/deletion.
- Empty files, binary files, untracked paths and invalid commit IDs.
- Merge commit diff against first parent, including rename across a merge.
- Relative paths from subdirectories, absolute paths, Unicode, spaces and
  literal pathspec characters; captured HEAD remains stable after new commits.
- Navigation boundaries, loading/errors, late asynchronous responses,
  cancellation, resize and terminal control-sequence removal.
- `go test ./...`, `go test -race ./...`, `go vet ./...`, binary build and
  interactive terminal smoke test.

The MVP is accepted only after the full workflow and checks pass. Writing the
TUI alone does not count as a successful build or terminal smoke test.

### Current validation status

- Git/history tests, race checks and vet pass locally.
- CLI and TUI implementation and tests are written.
- Full dependency resolution, `go.sum`, full-package checks, binary build and
  terminal smoke test are pending: the execution environment blocks Go's
  connection to `proxy.golang.org` and Bubble Tea is not cached.
- Once dependency access is available, run `go mod tidy`, `make check`,
  `make build`, then exercise the terminal workflow above.
- The workspace's `.git` directory is an empty read-only environment placeholder;
  it has not been modified or replaced. Publishing and release packaging are
  outside this task.

## Future roadmap

- Phase 5: blame, selecting a blamed line and jumping to its commit.
- Phase 6: investigate `git log -L` for line history and line evolution.
- Later: search, working-tree comparison, side-by-side diff, file tree,
  historical comparison and commit graph visualization.
- A future GUI reuses the Git adapter, domain and application service; only its
  presentation changes.
- Consider lazy loading/caching only after measurement on large repositories.
- GitHub/GitLab integration, authentication, AI explanations, multi-repository
  management, plugins and animations are not MVP requirements.

The goal is to make “How did this code get here?” cheap to answer.
