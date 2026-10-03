package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lcaohoanq/ghist/internal/history"
)

func groupedModel(t *testing.T) Model {
	t.Helper()
	m := ready()
	t.Cleanup(m.stop)
	m.width = 120
	m.history.Versions = nil
	for i, date := range []string{
		"2026-07-29T00:15:00+07:00", "2026-07-29T23:45:00-07:00",
		"2026-07-23T12:00:00+07:00", "2026-07-23T11:00:00+07:00",
		"2026-07-29T10:00:00+07:00",
	} {
		d, err := time.Parse(time.RFC3339, date)
		if err != nil {
			t.Fatal(err)
		}
		m.history.Versions = append(m.history.Versions, history.FileVersion{Commit: history.Commit{
			Date: d, ShortHash: fmt.Sprintf("%07d", i), Subject: fmt.Sprintf("commit-%d", i),
		}})
	}
	m.revealSelected()
	return m
}

func TestDayGroupsAndDefaultRendering(t *testing.T) {
	m := groupedModel(t)
	rows := m.historyRows()
	if len(rows) != 8 || !rows[0].header || rows[0].count != 2 || rows[3].count != 2 || rows[6].count != 1 {
		t.Fatalf("incorrect contiguous groups: %+v", rows)
	}
	for _, width := range []int{40, 109, 120, 180} {
		updateModel(&m, tea.WindowSizeMsg{Width: width, Height: 24})
		screen := ansi.Strip(m.View().Content)
		for _, want := range []string{"▾ 29-07-2026 · 2 commits", "▾ 23-07-2026 · 2 commits", "▾ 29-07-2026 · 1 commit"} {
			if !strings.Contains(screen, want) {
				t.Fatalf("width %d missing %q", width, want)
			}
		}
	}
}

func TestGroupToggleKeepsPreviewAndSelection(t *testing.T) {
	m := groupedModel(t)
	m.preview.content = "retained diff"
	request := m.preview.request
	if cmd := keyModel(&m, "up"); cmd != nil || m.historyCursor != 0 {
		t.Fatal("header focus loaded preview")
	}
	for _, key := range []string{"enter", "space", "left", "right"} {
		if cmd := keyModel(&m, key); cmd != nil {
			t.Fatal("toggle requested preview", key)
		}
		if m.selected != 0 || m.preview.content != "retained diff" || m.preview.request != request {
			t.Fatal("toggle changed preview", key)
		}
	}
	keyModel(&m, "down")
	keyModel(&m, "left")
	if !m.collapsed[0] || m.historyCursor != 0 || len(m.historyRows()) != 6 {
		t.Fatal("collapse from commit")
	}
	keyModel(&m, "down") // next header, not hidden commit
	if m.historyCursor != 1 || m.selected != 0 {
		t.Fatal("hidden rows navigable")
	}
	keyModel(&m, "down")
	if m.selected != 2 {
		t.Fatal("wrong commit after hidden rows")
	}
	keyModel(&m, "n")
	if m.selected != 1 || m.collapsed[0] || m.historyCursor != 2 {
		t.Fatal("n did not reveal destination")
	}
}

func TestGroupMouseAndAllCollapsed(t *testing.T) {
	m := groupedModel(t)
	for _, group := range []int{0, 2, 4} {
		m.setGroupCollapsed(group, true)
	}
	m.historyCursor = 0
	m.clampHistory()
	keyModel(&m, "end")
	if m.historyCursor != 2 || m.selected != 0 {
		t.Fatal("end on collapsed headers")
	}
	keyModel(&m, "home")
	cmd := updateModel(&m, tea.MouseClickMsg{X: 2, Y: 7, Button: tea.MouseLeft})
	if cmd != nil || m.collapsed[2] || m.historyCursor != 1 {
		t.Fatal("header click did not expand")
	}
	updateModel(&m, tea.MouseClickMsg{X: 2, Y: 8, Button: tea.MouseLeft})
	if m.selected != 2 {
		t.Fatal("commit click mapping")
	}
	keyModel(&m, "left")
	updateModel(&m, tea.MouseWheelMsg{X: 2, Y: 8, Button: tea.MouseWheelDown})
	if m.historyCursor != 2 || m.selected != 2 {
		t.Fatal("wheel should move to header without changing preview")
	}
	keyModel(&m, "p")
	if m.selected != 3 || m.collapsed[2] {
		t.Fatal("p did not reveal collapsed destination")
	}
}

func TestGroupScrollingResizeAndDetailReturn(t *testing.T) {
	m := groupedModel(t)
	updateModel(&m, tea.WindowSizeMsg{Width: 120, Height: 10})
	keyModel(&m, "end")
	if m.historyCursor != 7 || m.historyOffset != 6 || m.selected != 4 {
		t.Fatal("end scroll")
	}
	updateModel(&m, tea.MouseClickMsg{X: 2, Y: 6, Button: tea.MouseLeft})
	if !m.collapsed[4] || m.historyCursor != 6 {
		t.Fatal("scrolled header click")
	}
	keyModel(&m, "pgup")
	if m.historyCursor != 4 || m.selected != 2 {
		t.Fatal("page navigation")
	}
	keyModel(&m, "left")
	if !m.collapsed[2] {
		t.Fatal("collapse")
	}
	keyModel(&m, "d")
	if m.mode != diffView || m.selected != 2 {
		t.Fatal("d on header must open retained commit")
	}
	keyModel(&m, "esc")
	updateModel(&m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if !m.collapsed[2] || !m.collapsed[4] || m.historyOffset != 0 {
		t.Fatal("state lost on resize/detail")
	}
	if m.historyCursor != 3 {
		t.Fatal("header focus lost")
	}
	for _, line := range strings.Split(m.View().Content, "\n") {
		if ansi.StringWidth(line) > 80 {
			t.Fatal("overflow")
		}
	}
}
