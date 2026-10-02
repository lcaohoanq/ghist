package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestDiffLineNumbersAndMetadata(t *testing.T) {
	patch := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -10,3 +20,4 @@ function\n context\n-old\n+new\n+added\n tail\n\\ No newline at end of file\n@@ -100 +200 @@\n-before\n+after\n"
	rows := parseDiff(patch)
	for _, tc := range []struct{ index, old, new int }{{4, 10, 20}, {5, 11, 0}, {6, 0, 21}, {7, 0, 22}, {8, 12, 23}, {11, 100, 0}, {12, 0, 200}} {
		row := rows[tc.index]
		if row.old != tc.old || row.new != tc.new {
			t.Fatalf("row %d: %+v", tc.index, row)
		}
	}
	rendered := renderDiff(patch, 80, false)
	if strings.Contains(rendered[1], removedStyle) || strings.Contains(rendered[2], addedStyle) {
		t.Fatal("file headers colored as edits")
	}
	text := ansi.Strip(strings.Join(rendered, "\n"))
	for _, want := range []string{"  11      - old", "       21 + new", "No newline at end of file", " 100      - before"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}
func TestSplitDiffPairsAndWraps(t *testing.T) {
	patch := "@@ -1,2 +1,3 @@\n-" + strings.Repeat("界🙂", 40) + "\n+replacement\n+extra\n tail"
	for _, width := range []int{30, 65, 99, 100, 141} {
		rows := renderDiff(patch, width, false)
		for _, row := range rows {
			if ansi.StringWidth(row) > width {
				t.Fatalf("width %d overflow: %q", width, row)
			}
		}
		text := ansi.Strip(strings.Join(rows, "\n"))
		if width >= 100 {
			if !strings.Contains(text, "BEFORE") || !strings.Contains(text, "AFTER") {
				t.Fatal("missing split labels")
			}
			tail := rows[len(rows)-1]
			parts := strings.Split(ansi.Strip(tail), "│")
			if len(parts) != 2 || !strings.Contains(parts[0], "2   tail") || !strings.Contains(parts[1], "3   tail") {
				t.Fatal("context pair misaligned", tail)
			}
		} else if strings.Contains(text, "BEFORE") {
			t.Fatal("split shown in narrow panel")
		}
		if !strings.Contains(text, "↪") {
			t.Fatal("wrapped continuation not marked")
		}
	}
}
func TestDiffSpecialCasesAndToggle(t *testing.T) {
	for _, patch := range []string{"Binary files a/f and b/f differ", "No changes relative to the first parent.", "deleted file mode 100644\n@@ -1 +0,0 @@\n-removed", "@@ -0,0 +1 @@\n+created", "@@ -1 +1 @@\n-old\n\\ No newline at end of file\n+new\n\\ No newline at end of file"} {
		for _, width := range []int{60, 120} {
			text := ansi.Strip(strings.Join(renderDiff(patch, width, false), "\n"))
			if text == "" {
				t.Fatal("empty output")
			}
		}
	}
	m := ready()
	defer m.stop()
	m.width = 120
	m.mode = diffView
	m.content = "@@ -1 +1 @@\n-old\n+new"
	m.offset = 9
	if !strings.Contains(strings.Join(m.lines(), "\n"), "BEFORE") {
		t.Fatal("default not split")
	}
	keyModel(&m, "s")
	if m.offset != 0 || !m.diffUnified || strings.Contains(strings.Join(m.lines(), "\n"), "BEFORE") {
		t.Fatal("toggle did not reflow/reset")
	}
	m.width = 200
	m.mode = historyView
	m.preview.content = m.content
	if strings.Contains(strings.Join(m.previewLines(), "\n"), "BEFORE") {
		t.Fatal("preview ignores preference")
	}
}
