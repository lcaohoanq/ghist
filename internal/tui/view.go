package tui

import (
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Remove escape sequences before adding our own styles. Keep only line breaks
// and expanded tabs from control characters in repository-controlled text.
func safe(s string) string {
	s = ansi.Strip(strings.ToValidUTF8(s, "�"))
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, s)
}
func single(s string) string { return strings.ReplaceAll(safe(s), "\n", " ") }
func (m Model) lines() []string {
	if m.mode == diffView {
		return renderDiff(m.content, m.width, m.diffUnified)
	}
	return strings.Split(ansi.Hardwrap(safe(m.content), max(1, m.width), true), "\n")
}

// Styles use the terminal's palette so they adapt to its light/dark theme.
func paint(code, text string) string { return "\x1b[" + code + "m" + text + "\x1b[0m" }

func cell(text string, width int) string {
	text = ansi.Truncate(single(text), width, "…")
	return text + strings.Repeat(" ", max(0, width-ansi.StringWidth(text)))
}

type historyColumns struct {
	author int
	date   bool
}

func (m Model) columns() historyColumns {
	// Measure the entire history so scrolling never shifts the message column.
	author := 6
	for _, v := range m.history.Versions {
		author = max(author, ansi.StringWidth(single(v.Commit.Author)))
	}
	width := m.listWidth()
	showDate := width >= 60
	fixed := 13 // selection, hash and column spacing
	if showDate {
		fixed += 12
	}
	return historyColumns{author: max(6, min(author, 24, width-fixed-16)), date: showDate}
}
func (c historyColumns) row(marker, hash, date, author, subject string, styled bool) string {
	hash = cell(hash, 7)
	author = cell(author, c.author)
	date = cell(date, 10)
	if styled {
		hash = paint("33", hash)
		date = paint("90", date)
		author = paint("36", author)
	}
	row := marker + hash + "  "
	if c.date {
		row += date + "  "
	}
	return row + author + "  " + single(subject)
}
func (m Model) View() tea.View {
	if m.width < 30 || m.height < 10 {
		v := tea.NewView(ansi.Truncate("Terminal too small. Resize; q to quit.", m.width, ""))
		v.AltScreen = true
		return v
	}
	mode := "History"
	if m.mode == diffView {
		mode = "Diff"
	}
	if m.mode == fileView {
		mode = "File"
	}
	rows := []string{paint("1;36", "ghist") + " · " + paint("1", mode) + " · " + paint("36", single(m.path))}
	for i, line := range m.metadata() {
		code := "90"
		if i == 0 {
			code = "1;33"
		}
		rows = append(rows, paint(code, single(line)))
	}
	rows = append(rows, paint("90", strings.Repeat("─", m.width)))
	columns := m.columns()
	if m.mode == historyView {
		header := paint("1;90", columns.row("  ", "COMMIT", "DATE", "AUTHOR", "MESSAGE", false))
		if m.previewVisible() {
			marker, previewMarker := "> ", "  "
			if m.previewFocus {
				marker, previewMarker = "  ", "> "
			}
			header = paint("1;36", columns.row(marker, "COMMIT", "DATE", "AUTHOR", "MESSAGE", false))
			title := previewMarker + "Diff"
			if len(m.history.Versions) > 0 {
				title += " · " + single(m.history.Versions[m.selected].Commit.ShortHash)
			}
			header = panelCell(header, m.listWidth()) + m.separator() + paint("1;36", title)
		}
		rows = append(rows, header)
	}
	var body []string
	switch {
	case m.loading:
		body = []string{paint("33", "Loading…")}
	case m.err != nil:
		body = []string{paint("31", "Error: "+single(m.err.Error()))}
	case m.mode == historyView:
		start := max(0, m.selected-m.bodyHeight()+1)
		for i := start; i < len(m.history.Versions) && len(body) < m.bodyHeight(); i++ {
			c := m.history.Versions[i].Commit
			line := columns.row("  ", c.ShortHash, c.Date.Format("2006-01-02"), c.Author, c.Subject, true)
			if i == m.selected {
				line = columns.row("> ", c.ShortHash, c.Date.Format("2006-01-02"), c.Author, c.Subject, false)
				line = paint("1;7", cell(line, m.listWidth()))
			}
			body = append(body, line)
		}
	default:
		lines := m.lines()
		start := min(m.offset, len(lines))
		end := min(len(lines), start+m.bodyHeight())
		body = append(body, lines[start:end]...)
	}
	for len(body) < m.bodyHeight() {
		body = append(body, "")
	}
	preview := m.previewRows()
	for i, line := range body {
		if m.previewVisible() {
			line = panelCell(line, m.listWidth()) + m.separator() + preview[i]
		}
		line = ansi.Truncate(line, m.width, "")

		rows = append(rows, line)
	}
	rows = append(rows, paint("90", strings.Repeat("─", m.width)))
	help := "↑↓/jk select · Enter inspect · d diff · f file · p older · n newer · q quit"
	if m.previewVisible() {
		help = "Tab focus · ↑↓/PgUp/PgDn scroll · Enter/d expand · s split/unified · f file · p/n version · q quit"
	}
	if m.mode != historyView {
		help = "↑↓/jk/PgUp/PgDn scroll · s split/unified · d diff · f file · p/n version · Esc back · q quit"
	}
	rows = append(rows, paint("36", help))
	for i := range rows {
		rows[i] = ansi.Truncate(rows[i], m.width, "")
	}
	v := tea.NewView(strings.Join(rows, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
