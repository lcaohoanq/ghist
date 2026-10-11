package tui

import (
	"fmt"
	"strings"
	"time"
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
	return m.cachedLines(m.content, m.width, m.mode != diffView, false)
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
	author := m.historyAuthorWidth()
	width := m.listWidth()
	showDate := width >= 60
	fixed := 15 // selection, hash and column spacing
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
		hash = paint(styleYellow, hash)
		date = paint(styleMuted, date)
		author = paint(styleCyan, author)
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
	rows := []string{paint(styleBoldCyan, "ghist") + " · " + paint(styleBold, mode) + " · " + paint(styleCyan, single(m.path))}
	metadata := m.metadata()
	reviewRows, _ := m.reviewLayout()
	for i, line := range metadata {
		code := styleMuted
		if i == 0 {
			code = styleBoldYellow
		}
		// Review rows contain only application-generated OSC 8 hyperlinks whose
		// URLs were validated by the hosting package. Repository text still goes
		// through single(), which removes control sequences.
		if m.reviewer != nil && i > 0 && i <= len(reviewRows) {
			reviewCode := styleBoldCyan
			reviewRow := i - 1
			if m.review.err != nil || (m.review.openErr != nil && reviewRow == len(reviewRows)-1) {
				reviewCode = styleBoldRed
			}
			rows = append(rows, paint(reviewCode, line))
		} else {
			rows = append(rows, paint(code, single(line)))
		}
	}
	rows = append(rows, paint(styleMuted, strings.Repeat("─", m.width)))
	columns := m.columns()
	if m.mode == historyView {
		header := paint(styleBoldMuted, columns.row("    ", "COMMIT", "DATE", "AUTHOR", "MESSAGE", false))
		if m.previewVisible() {
			marker, previewMarker := "> ", "  "
			if m.previewFocus {
				marker, previewMarker = "  ", "> "
			}
			header = paint(styleBoldCyan, columns.row("  "+marker, "COMMIT", "DATE", "AUTHOR", "MESSAGE", false))
			title := previewMarker + "Diff"
			if len(m.history.Versions) > 0 {
				title += " · " + single(m.history.Versions[m.selected].Commit.ShortHash)
			}
			header = panelCell(header, m.listWidth()) + m.separator() + paint(styleBoldCyan, title)
		}
		rows = append(rows, header)
	}
	var body []string
	switch {
	case m.loading:
		body = []string{paint(styleYellow, "Loading…")}
	case m.mode == historyView && m.historyErr != nil && len(m.history.Versions) == 0:
		body = []string{paint(styleRed, "Error: "+single(m.historyErr.Error()))}
	case m.err != nil:
		body = []string{paint(styleRed, "Error: "+single(m.err.Error()))}
	case m.mode == historyView:
		visible := m.historyRows()
		for i := m.historyOffset; i < len(visible) && len(body) < m.bodyHeight(); i++ {
			row := visible[i]
			var line string
			if row.header {
				arrow := "▾"
				if m.collapsed[row.group] {
					arrow = "▸"
				}
				label := "commits"
				if row.count == 1 {
					label = "commit"
				}
				line = fmt.Sprintf("%s %s · %d %s", arrow, m.history.Versions[row.group].Commit.Date.Format("02-01-2006"), row.count, label)
				if i != m.historyCursor {
					line = paint(styleBoldCyan, line)
				}
			} else {
				c := m.history.Versions[row.version].Commit
				marker := "    "
				if i == m.historyCursor {
					marker = "  > "
				}
				line = columns.row(marker, c.ShortHash, c.Date.Format("02-01-2006"), c.Author, c.Subject, i != m.historyCursor)
			}
			if i == m.historyCursor {
				line = paint(styleSelected, cell(line, m.listWidth()))
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
	rows = append(rows, paint(styleMuted, strings.Repeat("─", m.width)))
	help := "←/→ fold · Enter toggle/inspect · ↑↓ select · d/f view · o review · p/n version · q quit"
	if m.previewVisible() {
		help = "←/→ fold · Enter toggle/inspect · Tab focus · ↑↓ scroll · d/f view · o review · s layout · p/n version · q quit"
	}
	if m.mode == historyView && m.pickerEnabled {
		help = "Esc files · " + help
	}
	if m.mode != historyView {
		help = "↑↓/jk/PgUp/PgDn scroll · s split/unified · d diff · f file · o review · p/n version · Esc back · q quit"
	}
	if m.historyLoading {
		help = fmt.Sprintf("Loaded %d commits · loading more… · ", len(m.history.Versions)) + help
	} else if m.historyComplete && m.historyErr == nil {
		help = fmt.Sprintf("Fetched %d commits in %s · ", len(m.history.Versions), m.historyElapsed.Round(time.Millisecond)) + help
	}
	if m.historyErr != nil && len(m.history.Versions) > 0 {
		help = "History incomplete: " + single(m.historyErr.Error()) + " · " + help
	}
	rows = append(rows, paint(styleCyan, help))
	for i := range rows {
		rows[i] = ansi.Truncate(rows[i], m.width, "")
	}
	v := tea.NewView(strings.Join(rows, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
