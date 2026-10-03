package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lcaohoanq/ghist/internal/history"
)

type streamingExplorer struct {
	fakeExplorer
	finished chan struct{}
}

func (s *streamingExplorer) StreamHistory(ctx context.Context, emit func(history.FileVersion) error) error {
	defer close(s.finished)
	for i := 0; i < 10000; i++ {
		if err := emit(testHistory().Versions[0]); err != nil {
			return err
		}
	}
	return nil
}
func TestStreamingBackpressureCancels(t *testing.T) {
	s := &streamingExplorer{finished: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan historyBatchMsg, 1)
	stopped := make(chan struct{})
	go func() { streamHistory(ctx, s, out); close(stopped) }()
	select {
	case first := <-out:
		if len(first.versions) != 1 || first.done {
			t.Fatal("first result was delayed")
		}
	case <-time.After(time.Second):
		t.Fatal("no first batch")
	}
	// Stop consuming while the producer fills the bounded queue.
	cancel()
	for _, ch := range []chan struct{}{stopped, s.finished} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("stream did not terminate")
		}
	}
}
func TestHistoryBatchesPreserveNavigationAndContent(t *testing.T) {
	m := New(context.Background(), &fakeExplorer{}, "file")
	defer m.stop()
	h := testHistory()
	updateModel(&m, historyBatchMsg{versions: h.Versions[:1]})
	if m.loading || !m.historyLoading || !strings.Contains(m.View().Content, "Loaded 1") {
		t.Fatal("first batch not visible")
	}
	keyModel(&m, "d")
	request := m.request
	updateModel(&m, contentMsg{request: request, content: "chosen diff"})
	updateModel(&m, historyBatchMsg{versions: h.Versions[1:]})
	if m.mode != diffView || m.content != "chosen diff" || m.request != request || m.selected != 0 {
		t.Fatal("batch reset current view")
	}
	keyModel(&m, "esc")
	keyModel(&m, "p")
	cursor := m.historyCursor
	updateModel(&m, historyBatchMsg{done: true, elapsed: 1234 * time.Millisecond})
	if !strings.Contains(m.View().Content, "Fetched 2 commits in 1.234s") {
		t.Fatal("completion duration not shown")
	}
	if m.selected != 1 || m.historyCursor != cursor || m.historyLoading {
		t.Fatal("completion reset selection")
	}
}
func TestHistoryBatchesExtendCollapsedDay(t *testing.T) {
	m := New(context.Background(), &fakeExplorer{}, "file")
	defer m.stop()
	h := testHistory()
	updateModel(&m, historyBatchMsg{versions: h.Versions[:1]})
	m.setGroupCollapsed(0, true)
	if len(m.historyRows()) != 1 {
		t.Fatal("collapse")
	}
	updateModel(&m, historyBatchMsg{versions: h.Versions[1:], done: true})
	rows := m.historyRows()
	if len(rows) != 1 || rows[0].count != 2 || !m.collapsed[0] {
		t.Fatal("day boundary lost")
	}
}
func TestHistoryStreamErrorsAndLateResults(t *testing.T) {
	m := New(context.Background(), &fakeExplorer{}, "file")
	defer m.stop()
	updateModel(&m, historyBatchMsg{versions: testHistory().Versions})
	updateModel(&m, historyBatchMsg{done: true, err: errors.New("interrupted read")})
	if strings.Contains(m.View().Content, "Fetched") {
		t.Fatal("failed history shown as complete")
	}
	if !strings.Contains(m.View().Content, "History incomplete") || len(m.history.Versions) != 2 {
		t.Fatal("partial results discarded")
	}
	keyModel(&m, "q")
	updateModel(&m, historyBatchMsg{versions: testHistory().Versions})
	if len(m.history.Versions) != 2 {
		t.Fatal("accepted post-cancel batch")
	}
}
func TestRenderCacheInvalidation(t *testing.T) {
	m := ready()
	defer m.stop()
	m.mode = diffView
	m.content = "@@ -1 +1 @@\n-old\n+new\n"
	a := m.lines()
	b := m.lines()
	if &a[0] != &b[0] {
		t.Fatal("rendered again")
	}
	m.width = 35
	c := m.lines()
	if &a[0] == &c[0] {
		t.Fatal("resize did not invalidate")
	}
	m.content = "different"
	if strings.Join(m.lines(), "") == strings.Join(c, "") {
		t.Fatal("stale content")
	}
}

func TestIncrementalRowsMatchFullRebuild(t *testing.T) {
	m := New(context.Background(), &fakeExplorer{}, "file")
	defer m.stop()
	for i := 0; i < 150; i++ {
		v := testHistory().Versions[0]
		v.Commit.Subject = strings.Repeat("x", i)
		v.Commit.Date = time.Date(2026, 1, 1+i/17, 0, 0, 0, 0, time.UTC)
		updateModel(&m, historyBatchMsg{versions: []history.FileVersion{v}})
		if i%19 == 0 {
			m.setGroupCollapsed(0, i%2 == 0)
		}
		actual, expected := m.historyRows(), m.buildHistoryRows()
		if len(actual) != len(expected) {
			t.Fatalf("batch %d: row count mismatch", i)
		}
		for j := range actual {
			if actual[j] != expected[j] {
				t.Fatalf("batch %d row %d mismatch", i, j)
			}
		}
	}
}
