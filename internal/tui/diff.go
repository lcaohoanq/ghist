package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Keep code legible on subtle, explicitly paired foreground/background colors.
// The signs and line numbers also communicate changes without color.
const (
	addedStyle   = "38;2;222;230;226;48;2;24;48;38"
	removedStyle = "38;2;235;225;227;48;2;52;30;36"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

type diffRow struct {
	kind     byte
	text     string
	old, new int
}

func parseDiff(patch string) []diffRow {
	var rows []diffRow
	old, next := 0, 0
	inHunk := false
	for _, line := range strings.Split(strings.TrimSuffix(safe(patch), "\n"), "\n") {
		if match := hunkHeader.FindStringSubmatch(line); match != nil {
			old, _ = strconv.Atoi(match[1])
			next, _ = strconv.Atoi(match[2])
			inHunk = true
			rows = append(rows, diffRow{kind: '@', text: line})
			continue
		}
		if strings.HasPrefix(line, "diff --git ") {
			inHunk = false
		}
		row := diffRow{kind: 'm', text: line}
		if inHunk && len(line) > 0 {
			row.kind = line[0]
			row.text = line[1:]
			switch row.kind {
			case ' ':
				row.old, row.new = old, next
				old++
				next++
			case '-':
				row.old = old
				old++
			case '+':
				row.new = next
				next++
			default:
				row.kind = 'm'
				row.text = line
			}
		}
		rows = append(rows, row)
	}
	return rows
}
func lineNumber(n, digits int) string {
	if n == 0 {
		return strings.Repeat(" ", digits)
	}
	return fmt.Sprintf("%*d", digits, n)
}
func diffStyle(kind byte) string {
	switch kind {
	case '+':
		return addedStyle
	case '-':
		return removedStyle
	}
	return ""
}

// wrapDiffText preserves every source character while preferring code-token
// boundaries. Hard wrapping is reserved for tokens wider than the content area.
func wrapDiffText(text string, width int) []string {
	width = max(1, width)
	var lines []string
	var line strings.Builder
	used := 0
	appendToken := func(token string) {
		for _, part := range strings.Split(ansi.Hardwrap(token, width, true), "\n") {
			size := ansi.StringWidth(part)
			if used > 0 && used+size > width {
				lines = append(lines, line.String())
				line.Reset()
				used = 0
			}
			line.WriteString(part)
			used += size
		}
	}
	start := 0
	for i, r := range text {
		if strings.ContainsRune(" .,;:()[]{}=", r) {
			appendToken(text[start : i+1])
			start = i + 1
		}
	}
	appendToken(text[start:])
	lines = append(lines, line.String())
	return lines
}

func wrapDiffRow(row diffRow, width, digits int, side bool) []string {
	prefix := lineNumber(row.old, digits) + " " + lineNumber(row.new, digits) + " "
	if side {
		prefix = lineNumber(max(row.old, row.new), digits) + " "
	}
	sign := " "
	if row.kind == '+' || row.kind == '-' {
		sign = string(row.kind)
	}
	prefix += sign + " "
	gutter := ansi.StringWidth(prefix)
	parts := wrapDiffText(row.text, max(1, width-gutter))
	for i, part := range parts {
		lead := prefix
		if i > 0 {
			lead = strings.Repeat(" ", gutter-2) + "↪ "
		}
		if style := diffStyle(row.kind); style != "" {
			parts[i] = paint(style, panelCell(lead+part, width))
		} else {
			parts[i] = paint("90", lead) + part
		}
	}
	return parts
}
func renderDiff(patch string, width int, unified bool) []string {
	width = max(1, width)
	rows := parseDiff(patch)
	digits := 4
	for _, row := range rows {
		digits = max(digits, len(strconv.Itoa(max(row.old, row.new))))
	}
	side := width >= 100 && !unified
	var out []string
	leftWidth := (width - 1) / 2
	rightWidth := width - leftWidth - 1
	if side {
		out = append(out, paint("1;90", panelCell(" BEFORE · parent", leftWidth)+"│ AFTER · selected"))
	}
	for i := 0; i < len(rows); {
		row := rows[i]
		if row.kind == 'm' || row.kind == '@' {
			style := "90"
			if row.kind == '@' {
				style = "1;36"
			}
			for _, part := range strings.Split(ansi.Hardwrap(row.text, width, true), "\n") {
				out = append(out, paint(style, part))
			}
			i++
			continue
		}
		if !side {
			out = append(out, wrapDiffRow(row, width, digits, false)...)
			i++
			continue
		}
		var left, right []diffRow
		if row.kind == ' ' {
			a, b := row, row
			a.new = 0
			b.old = 0
			left = []diffRow{a}
			right = []diffRow{b}
			i++
		} else {
			// Pair adjacent removed/added runs by position. Unmatched lines get an
			// empty opposite cell; wrapping never shifts the next logical pair.
			for i < len(rows) && (rows[i].kind == '-' || rows[i].kind == '+') {
				if rows[i].kind == '-' {
					left = append(left, rows[i])
				} else {
					right = append(right, rows[i])
				}
				i++
			}
		}
		for j := 0; j < max(len(left), len(right)); j++ {
			var a, b []string
			if j < len(left) {
				a = wrapDiffRow(left[j], leftWidth, digits, true)
			}
			if j < len(right) {
				b = wrapDiffRow(right[j], rightWidth, digits, true)
			}
			for k := 0; k < max(len(a), len(b)); k++ {
				l, r := "", ""
				if k < len(a) {
					l = a[k]
				}
				if k < len(b) {
					r = b[k]
				}
				out = append(out, panelCell(l, leftWidth)+paint("90", "│")+panelCell(r, rightWidth))
			}
		}
	}
	return out
}
