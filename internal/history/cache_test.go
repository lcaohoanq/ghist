package history

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestCacheEvictsByBytesAndRecency(t *testing.T) {
	c := newContentCache(2, 12)
	a, b, d := contentKey{hash: "a"}, contentKey{hash: "b"}, contentKey{hash: "d"}
	c.put(a, "aaa", 3)
	c.put(b, "bbb", 3)
	c.get(a)
	c.put(d, "ddd", 3)
	if _, ok := c.get(b); ok {
		t.Fatal("did not evict least recently used entry")
	}
	c.put(a, "large", 20)
	if _, ok := c.get(a); ok {
		t.Fatal("retained oversized replacement")
	}
	c = newContentCache(10, 8)
	c.put(a, "aaa", 3)
	c.put(b, "bbbb", 4)
	if c.bytes > 8 || c.order.Len() != 1 {
		t.Fatal("byte limit not enforced")
	}
}

type countingRepo struct {
	diffs, snapshots int
	err              error
}

func (r *countingRepo) FileHistory(context.Context) (FileHistory, error) { return FileHistory{}, nil }
func (r *countingRepo) Diff(context.Context, FileVersion) (FileDiff, error) {
	r.diffs++
	return FileDiff{Patch: "patch"}, r.err
}
func (r *countingRepo) Snapshot(context.Context, FileVersion) (FileSnapshot, error) {
	r.snapshots++
	return FileSnapshot{Content: "snapshot", Exists: true}, r.err
}
func TestServiceCachesContentAndSeparatesPaths(t *testing.T) {
	r := &countingRepo{}
	s := NewService(r)
	ctx := context.Background()
	v := FileVersion{Commit: Commit{Hash: strings.Repeat("a", 40)}, AfterPath: "a"}
	for i := 0; i < 2; i++ {
		s.GetDiff(ctx, v)
		s.GetSnapshot(ctx, v)
	}
	if r.diffs != 1 || r.snapshots != 1 {
		t.Fatal("cache misses on revisit")
	}
	v.AfterPath = "b"
	s.GetDiff(ctx, v)
	if r.diffs != 2 {
		t.Fatal("path collision")
	}
	v.Parent = "parent"
	r.err = errors.New("failed")
	s.GetDiff(ctx, v)
	s.GetDiff(ctx, v)
	if r.diffs != 4 {
		t.Fatal("cached failure")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.GetSnapshot(cancelled, v); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestCacheConcurrentAccess(t *testing.T) {
	c := newContentCache(4, 1024)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			for j := 0; j < 100; j++ {
				k := contentKey{kind: byte(j % 8)}
				c.put(k, "value", 5)
				c.get(k)
			}
		})
	}
	wg.Wait()
	if c.order.Len() > 4 || c.bytes > 1024 {
		t.Fatal("bounds exceeded")
	}
}
