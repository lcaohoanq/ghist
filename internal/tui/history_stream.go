package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lcaohoanq/ghist/internal/history"
)

type historyBatchMsg struct {
	versions []history.FileVersion
	done     bool
	err      error
}

// One queued version and one queued batch bound the producer's lead over UI.
// Only this coordinator owns batches; versions are never mutated after sending.
func streamHistory(ctx context.Context, source history.HistoryStreamer, out chan<- historyBatchMsg) {
	defer close(out)
	versions := make(chan history.FileVersion, 1)
	completed := make(chan error, 1)
	go func() {
		completed <- source.StreamHistory(ctx, func(v history.FileVersion) error {
			select {
			case versions <- v:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		close(versions)
	}()
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	batch := make([]history.FileVersion, 0, 64)
	first := true
	send := func(done bool, err error) bool {
		select {
		case out <- historyBatchMsg{batch, done, err}:
			batch = make([]history.FileVersion, 0, 64)
			return true
		case <-ctx.Done():
			return false
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case v, ok := <-versions:
			if !ok {
				send(true, <-completed)
				return
			}
			batch = append(batch, v)
			if first || len(batch) >= 64 {
				if !send(false, nil) {
					return
				}
				first = false
			}
		case <-ticker.C:
			if len(batch) > 0 && !send(false, nil) {
				return
			}
		}
	}
}
func (m Model) awaitHistory() tea.Cmd {
	return func() tea.Msg {
		select {
		case <-m.ctx.Done():
			return nil
		case batch, ok := <-m.historyEvents:
			if !ok {
				return nil
			}
			return batch
		}
	}
}
