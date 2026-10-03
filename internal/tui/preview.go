package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type previewState struct {
	content string
	err     error
	loading bool
	offset  int
	request uint64
	cancel  context.CancelFunc
}
type previewReadyMsg struct{ request uint64 }
type previewMsg struct {
	request uint64
	content string
	err     error
}

func (m Model) previewVisible() bool {
	return m.mode == historyView && m.width >= 110 && m.height >= 10
}
func (m Model) listWidth() int {
	if m.previewVisible() {
		return (m.width - 1) * 45 / 100
	}
	return m.width
}
func (m Model) previewWidth() int { return max(1, m.width-m.listWidth()-1) }
func (m *Model) invalidatePreview() {
	if m.preview.cancel != nil {
		m.preview.cancel()
		m.preview.cancel = nil
	}
	m.preview.request++
	m.preview.loading = false
}
func (m *Model) schedulePreview() tea.Cmd {
	m.invalidatePreview()
	m.preview.content, m.preview.err, m.preview.offset = "", nil, 0
	if !m.previewVisible() || m.loading || m.err != nil || len(m.history.Versions) == 0 || m.ctx.Err() != nil {
		return nil
	}
	m.preview.loading = true
	ctx, cancel := context.WithCancel(m.ctx)
	m.preview.cancel = cancel
	id := m.preview.request
	return func() tea.Msg {
		timer := time.NewTimer(100 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			return previewReadyMsg{id}
		}
	}
}
func (m *Model) fetchPreview() tea.Cmd {
	if m.preview.cancel != nil {
		m.preview.cancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.preview.cancel = cancel
	id, version, service := m.preview.request, m.history.Versions[m.selected], m.service
	return func() tea.Msg {
		defer cancel()
		if ctx.Err() != nil {
			return nil
		}
		diff, err := service.GetDiff(ctx, version)
		if diff.Truncated {
			diff.Patch = limitedNotice + diff.Patch
		}
		if err == nil && diff.Patch == "" {
			diff.Patch = "No changes relative to the first parent."
		}
		return previewMsg{id, diff.Patch, err}
	}
}

func (m Model) previewLines() []string {
	return m.cachedLines(m.preview.content, m.previewWidth(), false, true)
}
func (m *Model) clampPreview() {
	m.preview.offset = max(0, min(m.preview.offset, max(0, len(m.previewLines())-m.bodyHeight())))
}
func (m Model) previewRows() []string {
	rows := make([]string, m.bodyHeight())
	if !m.previewVisible() {
		return rows
	}
	switch {
	case m.preview.loading:
		rows[0] = paint("33", "Loading…")
	case m.preview.err != nil:
		rows[0] = paint("31", "Error: "+single(m.preview.err.Error()))
	case len(m.history.Versions) > 0:
		lines := m.previewLines()
		start := min(m.preview.offset, len(lines))
		copy(rows, lines[start:min(len(lines), start+len(rows))])
	}
	for i := range rows {
		rows[i] = ansi.Truncate(rows[i], m.previewWidth(), "")
	}
	return rows
}
func panelCell(s string, width int) string {
	s = ansi.Truncate(s, width, "")
	return s + strings.Repeat(" ", max(0, width-ansi.StringWidth(s)))
}
func (m Model) separator() string {
	if m.previewFocus {
		return paint("1;36", "│")
	}
	return paint("90", "│")
}
func (m Model) mouse(mouse tea.Mouse, wheel bool) (tea.Model, tea.Cmd) {
	if m.width < 30 || m.height < 10 || mouse.X < 0 || mouse.X >= m.width || mouse.Y < 5 || mouse.Y >= m.height-2 {
		return m, nil
	}
	if !wheel && mouse.Button != tea.MouseLeft {
		return m, nil
	}
	if m.mode != historyView {
		if wheel {
			if mouse.Button == tea.MouseWheelUp {
				return m.key("up")
			}
			if mouse.Button == tea.MouseWheelDown {
				return m.key("down")
			}
		}
		return m, nil
	}
	if len(m.history.Versions) == 0 {
		return m, nil
	}
	if m.previewVisible() && mouse.X == m.listWidth() {
		return m, nil
	}
	inPreview := m.previewVisible() && mouse.X > m.listWidth()
	if !wheel {
		m.previewFocus = inPreview
	}
	if wheel {
		delta := 0
		if mouse.Button == tea.MouseWheelUp {
			delta = -1
		}
		if mouse.Button == tea.MouseWheelDown {
			delta = 1
		}
		if inPreview {
			m.preview.offset += delta
			m.clampPreview()
			return m, nil
		}
		cmd := m.focusHistoryRow(m.historyCursor + delta)
		return m, cmd
	} else if !inPreview && mouse.Y >= 6 {
		next := m.historyOffset + mouse.Y - 6
		rows := m.historyRows()
		if next < len(rows) {
			cmd := m.focusHistoryRow(next)
			if rows[next].header {
				m.toggleHistoryGroup("toggle")
			}
			return m, cmd
		}
	}
	return m, nil
}
