# History performance

## Reproduce

```sh
go test ./internal/git -run '^$' -bench BenchmarkFileHistory -benchtime=3x -count=1
go test -race ./...
go vet ./...
```

The benchmark builds one local repository with 120 file revisions, a rename,
100 unrelated files and unrelated changes alongside each revision. Setup is
excluded from timings. The legacy implementation is retained only in test code;
its complete history must equal the new implementation's result before timing.

Sample on an Intel i5-11400H, Linux amd64 (three iterations, warm local storage):

| Implementation | Complete traversal | Go allocated bytes/op | Allocations/op |
| --- | ---: | ---: | ---: |
| Legacy log + per-commit diff-tree | 313 ms | 6,998,061 | 18,326 |
| One streaming log | 5.1 ms | 131,725 | 1,465 |

First parsed version averaged 2.7 ms. This measures the repository callback,
not time to first terminal paint. Go allocation figures exclude Git subprocess
memory. These are synthetic measurements, not a promise for another repository.

## Implementation

- One `git log --follow --name-status -z` supplies metadata and paths. Metadata
  and rename path tokens are parsed positionally, preserving unusual filenames.
- The captured HEAD, topological ordering, rename detection and first-parent diff
  convention remain. Git determines file-history traversal; this is not a custom
  traversal of every possible rename lineage through a merge graph.
- The first version is sent immediately. Later UI batches contain up to 64
  versions, with partial batches flushed every 40 ms. The queues are bounded.
- The UI appends versions without resetting selection, collapse state, an open
  diff or a snapshot. Cancellation propagates through queue waits to Git, which
  is always reaped. Partial results remain visible on a later error.
- The service LRU stores at most 64 diff/snapshot entries and 32 MiB of string
  payload, including keys. Object/map overhead is additional. Cache scope is one
  repository service, and keys include content kind, commit, parent and paths.
  Errors and cancelled fetches are not cached.
- Initial diff/snapshot output is capped at 2 MiB, while remaining Git output is
  drained. `L` explicitly requests full output without retaining it in the LRU.
  This does not cap Git's own memory, CPU time or the full-output request.
- Current bounded content/preview render results are cached by content, width and
  layout. History rows and author widths are cached until their inputs change.

## Regression coverage

Tests cover one Git process regardless of revision count, streamed output before
EOF, malformed/truncated metadata, tab/newline paths, rename/delete/root/merge
history, captured HEAD, cancellation under backpressure, partial failures,
selection preservation, day groups spanning batches, LRU byte/entry eviction,
concurrent access, render invalidation and explicit full-content retrieval.

## Remaining limits

Git's first result may still be slow for expensive rename detection or graph
ordering. Streaming improves responsiveness once Git emits records; it does not
remove that work. All history metadata is eventually retained in memory. There
is no disk cache or resumable pagination yet. Large explicitly loaded content can
still be expensive to render. Measure on the affected real repository before
choosing another traversal mode or introducing pagination.
