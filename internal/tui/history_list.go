package tui

import tea "charm.land/bubbletea/v2"

// group is the original version index at the start of a contiguous day.
// Keeping version indices preserves Git's topological order and p/n semantics.
type historyRow struct {
	group, version, count int
	header                bool
}

func (m Model) historyRows() []historyRow {
	if m.renderCache == nil || len(m.history.Versions) == 0 {
		return m.buildHistoryRows()
	}
	c := m.renderCache
	c.mu.Lock()
	defer c.mu.Unlock()
	first := &m.history.Versions[0]
	// Append batches incrementally, including a day group split across batches.
	// Rebuild only when collapse state changes or a different history replaces it.
	if c.first == nil || *c.first != *first || c.count > len(m.history.Versions) || c.collapseRevision != m.collapseRevision {
		c.rows = nil
		c.count = 0
		c.lastHeader = 0
	}
	for i := c.count; i < len(m.history.Versions); i++ {
		day := m.history.Versions[i].Commit.Date.Format("2006-01-02")
		if len(c.rows) == 0 || m.history.Versions[c.rows[c.lastHeader].group].Commit.Date.Format("2006-01-02") != day {
			c.lastHeader = len(c.rows)
			c.rows = append(c.rows, historyRow{group: i, header: true})
		}
		c.rows[c.lastHeader].count++
		group := c.rows[c.lastHeader].group
		if !m.collapsed[group] {
			c.rows = append(c.rows, historyRow{group: group, version: i})
		}
	}
	c.first, c.count, c.collapseRevision = first, len(m.history.Versions), m.collapseRevision
	return c.rows
}
func (m Model) buildHistoryRows() []historyRow {
	var rows []historyRow
	for start := 0; start < len(m.history.Versions); {
		day := m.history.Versions[start].Commit.Date.Format("2006-01-02")
		end := start + 1
		for end < len(m.history.Versions) && m.history.Versions[end].Commit.Date.Format("2006-01-02") == day {
			end++
		}
		rows = append(rows, historyRow{group: start, count: end - start, header: true})
		if !m.collapsed[start] {
			for i := start; i < end; i++ {
				rows = append(rows, historyRow{group: start, version: i})
			}
		}
		start = end
	}
	return rows
}

func (m *Model) clampHistory() {
	rows := m.historyRows()
	m.historyCursor = max(0, min(m.historyCursor, len(rows)-1))
	m.historyOffset = max(0, min(m.historyOffset, max(0, len(rows)-m.bodyHeight())))
	if m.historyCursor < m.historyOffset {
		m.historyOffset = m.historyCursor
	}
	if m.historyCursor >= m.historyOffset+m.bodyHeight() {
		m.historyOffset = m.historyCursor - m.bodyHeight() + 1
	}
}

func (m *Model) focusHistoryRow(index int) tea.Cmd {
	rows := m.historyRows()
	if len(rows) == 0 {
		return nil
	}
	m.historyCursor = max(0, min(index, len(rows)-1))
	m.clampHistory()
	row := rows[m.historyCursor]
	if !row.header && row.version != m.selected {
		m.selected = row.version
		return m.schedulePreview()
	}
	return nil
}

func (m *Model) setGroupCollapsed(group int, collapsed bool) {
	// Copy state because Bubble Tea models are passed by value.
	next := make(map[int]bool, len(m.collapsed)+1)
	for k, v := range m.collapsed {
		next[k] = v
	}
	next[group] = collapsed
	m.collapsed = next
	m.collapseRevision++
}

func (m *Model) toggleHistoryGroup(key string) {
	rows := m.historyRows()
	if len(rows) == 0 {
		return
	}
	row := rows[min(m.historyCursor, len(rows)-1)]
	collapsed := !m.collapsed[row.group]
	if key == "left" {
		collapsed = true
	} else if key == "right" {
		collapsed = false
	}
	m.setGroupCollapsed(row.group, collapsed)
	if collapsed || row.header {
		for i, r := range m.historyRows() {
			if r.header && r.group == row.group {
				m.historyCursor = i
				break
			}
		}
	}
	m.clampHistory()
}

func (m *Model) revealSelected() {
	for _, row := range m.historyRows() {
		if row.header && m.selected >= row.group && m.selected < row.group+row.count {
			m.setGroupCollapsed(row.group, false)
			break
		}
	}
	for i, row := range m.historyRows() {
		if !row.header && row.version == m.selected {
			m.historyCursor = i
			break
		}
	}
	m.clampHistory()
}
