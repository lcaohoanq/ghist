package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/lcaohoanq/ghist/internal/history"
)

func TestHistoryColumnsStayAligned(t *testing.T) {
	m := ready()
	m.history.Versions = nil
	for _, author := range []string{"lcaohoanq", "Lưu Cao Hoàng", "山田太郎", strings.Repeat("Long name ", 8)} {
		m.history.Versions = append(m.history.Versions, history.FileVersion{Commit: history.Commit{ShortHash: "abcdef0", Author: author, Subject: "MESSAGE-TEXT"}})
	}
	for _, width := range []int{30, 59, 60, 100} {
		m.width = width
		for selected := range m.history.Versions {
			m.selected = selected
			m.revealSelected()
			screen := m.View().Content
			position := -1
			count := 0
			for row, line := range strings.Split(screen, "\n") {
				if ansi.StringWidth(line) > width {
					t.Fatalf("width %d overflow: %q", width, line)
				}
				if row < 6 {
					continue
				}
				plain := ansi.Strip(line)
				// At narrow widths, only the beginning of the message remains visible.
				index := strings.Index(plain, "MESSAGE")
				if index < 0 {
					continue
				}
				column := ansi.StringWidth(plain[:index])
				if position >= 0 && position != column {
					t.Fatalf("width %d: messages at columns %d and %d", width, position, column)
				}
				position = column
				count++
			}
			if count != 4 {
				t.Fatalf("width %d: expected 4 visible messages, got %d", width, count)
			}
			if !strings.Contains(screen, "\x1b[1;7m") {
				t.Fatal("missing selected-row highlight")
			}
			if len(strings.Split(screen, "\n")) != m.height {
				t.Fatal("incorrect screen height")
			}
		}
	}
	// The widest author is offscreen: column sizing must still remain unchanged.
	m.width = 100
	m.height = 10
	m.selected = 0
	before := m.columns()
	m.selected = 3
	if m.columns() != before {
		t.Fatal("scrolling changed columns")
	}
}
