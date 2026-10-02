package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestWrapDiffTextPreservesTokensAndContent(t *testing.T) {
	for _, text := range []string{
		"                        workouts: day.workouts,",
		"                    const rect = event.currentTarget.getBoundingClientRect();",
		strings.Repeat(" ", 100) + "setActiveTooltip(null);",
		strings.Repeat("界🙂é", 100) + "END",
		strings.Repeat("a", 300), "", "   ",
	} {
		for _, width := range []int{18, 35, 43, 67} {
			lines := wrapDiffText(text, width)
			if strings.Join(lines, "") != text {
				t.Fatalf("source changed at %d: %q", width, lines)
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > width {
					t.Fatalf("overflow at %d: %q", width, line)
				}
			}
			if strings.Contains(text, "currentTarget") && !strings.Contains(strings.Join(lines, "\n"), "currentTarget") {
				t.Fatalf("identifier split at %d: %q", width, lines)
			}
			if strings.Contains(text, "workouts") && strings.Count(strings.Join(lines, "\n"), "workouts") != 2 {
				t.Fatalf("identifier split at %d: %q", width, lines)
			}
		}
	}
}

func TestPreviewWrapAndScrollAfterResize(t *testing.T) {
	for _, width := range []int{110, 120, 180, 200, 240} {
		for _, unified := range []bool{false, true} {
			m := ready()
			m.width = width
			m.height = 12
			m.diffUnified = unified
			m.preview.content = "@@ -1 +1 @@\n-old\n+" + strings.Repeat("    workouts: day.workouts, ", 30) + "END_OF_LINE"
			lines := m.previewLines()
			if len(lines) <= m.bodyHeight() {
				t.Fatal("long line not wrapped")
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > m.previewWidth() {
					t.Fatalf("preview overflow at %d", width)
				}
			}
			keyModel(&m, "tab")
			keyModel(&m, "end")
			if !strings.Contains(m.View().Content, "END_OF_LINE") {
				t.Fatalf("wrapped tail unreachable at %d", width)
			}
			updateModel(&m, tea.WindowSizeMsg{Width: 110, Height: 12})
			keyModel(&m, "end")
			if !strings.Contains(m.View().Content, "END_OF_LINE") {
				t.Fatal("wrapped tail unreachable after resize")
			}
			if m.selected != 0 {
				t.Fatal("scroll changed commit")
			}
			m.stop()
		}
	}
}
