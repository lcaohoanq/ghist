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
	if c.authorFirst == nil || *c.authorFirst != *first || c.authorCount > len(m.history.Versions) {
		c.authorCount, c.authorWidth = 0, 6
	}
	for _, v := range m.history.Versions[c.authorCount:] {
		c.authorWidth = max(c.authorWidth, ansi.StringWidth(single(v.Commit.Author)))
	}
	c.authorFirst, c.authorCount = first, len(m.history.Versions)
	return c.authorWidth
}
