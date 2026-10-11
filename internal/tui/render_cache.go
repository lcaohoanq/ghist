package tui

import (
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/lcaohoanq/ghist/internal/history"
)

type lineCache struct {
	content       string
	width         int
	unified, file bool
	lines         []string
}
type renderCache struct {
	authorFirst              *history.FileVersion
	authorCount, authorWidth int
	mu                       sync.Mutex
	content, preview         lineCache
	first                    *history.FileVersion
	count                    int
	lastHeader               int
	collapseRevision         uint64
	rows                     []historyRow
	graphFirst               *history.FileVersion
	graphCount               int
	graphCells               []graphCell
}

func (m Model) cachedLines(content string, width int, file, preview bool) []string {
	build := func() []string {
		if file {
			return strings.Split(ansi.Hardwrap(safe(content), max(1, width), true), "\n")
		}
		return renderDiff(content, width, m.diffUnified)
	}
	if m.renderCache == nil {
		return build()
	}
	c := m.renderCache
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := &c.content
	if preview {
		entry = &c.preview
	}
	if entry.lines != nil && entry.content == content && entry.width == width && entry.file == file && entry.unified == m.diffUnified {
		return entry.lines
	}
	lines := build()
	// Avoid retaining a second large representation after explicit full loading.
	if len(content) <= (2<<20)+len(limitedNotice) {
		*entry = lineCache{content, width, m.diffUnified, file, lines}
	} else {
		*entry = lineCache{}
	}
	return lines
}

func (m Model) historyAuthorWidth() int {
	measure := func() int {
		n := 6
		for _, v := range m.history.Versions {
			n = max(n, ansi.StringWidth(single(v.Commit.Author)))
		}
		return n
	}
	if m.renderCache == nil || len(m.history.Versions) == 0 {
		return measure()
	}
	c := m.renderCache
	c.mu.Lock()
	defer c.mu.Unlock()
	first := &m.history.Versions[0]
	if c.authorFirst == nil || c.authorFirst.Commit.Hash != first.Commit.Hash || c.authorCount > len(m.history.Versions) {
		c.authorCount, c.authorWidth = 0, 6
	}
	for _, v := range m.history.Versions[c.authorCount:] {
		c.authorWidth = max(c.authorWidth, ansi.StringWidth(single(v.Commit.Author)))
	}
	c.authorFirst, c.authorCount = first, len(m.history.Versions)
	return c.authorWidth
}

// graphCells returns pre-built graph cells for the current version list,
// building incrementally as new batches arrive (same pattern as authorWidth).
func (m Model) graphCells() []graphCell {
	if m.renderCache == nil || len(m.history.Versions) == 0 {
		return buildGraphCells(m.history.Versions)
	}
	c := m.renderCache
	c.mu.Lock()
	defer c.mu.Unlock()
	first := &m.history.Versions[0]
	// Rebuild from scratch when the history identity changes.
	if c.graphFirst == nil || c.graphFirst.Commit.Hash != first.Commit.Hash || c.graphCount > len(m.history.Versions) {
		c.graphCells = nil
		c.graphCount = 0
	}
	if c.graphCount < len(m.history.Versions) {
		// Append cells for newly arrived versions. buildGraphCells is called on
		// the full slice so connectors are correct; we only expose the slice view.
		c.graphCells = buildGraphCells(m.history.Versions)
		c.graphFirst, c.graphCount = first, len(m.history.Versions)
	}
	return c.graphCells
}
