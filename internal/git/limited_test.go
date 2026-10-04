package git

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestLargeContentIsLimitedAndCanLoadFull(t *testing.T) {
	dir := fixture(t)
	// The byte cap splits a multibyte rune.
	content := strings.Repeat("界", contentLimit/3+3) + "\n"
	write(t, dir, "large", content)
	commit(t, dir, "large")
	r, h := explore(t, filepath.Join(dir, "large"))
	v := h.Versions[0]
	ctx := context.Background()
	s, err := r.Snapshot(ctx, v)
	if err != nil || !s.Truncated || s.Binary || len(s.Content) > contentLimit {
		t.Fatalf("snapshot: size=%d truncated=%v binary=%v err=%v", len(s.Content), s.Truncated, s.Binary, err)
	}
	full, err := r.FullSnapshot(ctx, v)
	if err != nil || full.Truncated || full.Content != content {
		t.Fatalf("full snapshot: %v", err)
	}
	d, err := r.Diff(ctx, v)
	if err != nil || !d.Truncated || len(d.Patch) > contentLimit {
		t.Fatalf("diff: %v", err)
	}
	fullDiff, err := r.FullDiff(ctx, v)
	if err != nil || fullDiff.Truncated || !strings.Contains(fullDiff.Patch, content) {
		t.Fatalf("full diff: %v", err)
	}
}
