package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lcaohoanq/ghist/internal/hosting"
)

type reviewResult struct {
	links []hosting.ReviewLink
	err   error
}

type reviewState struct {
	links   []hosting.ReviewLink
	err     error
	openErr error
	loading bool
	request uint64
	sha     string
	cancel  context.CancelFunc
	cache   map[string]reviewResult
}

type reviewReadyMsg struct {
	request uint64
	sha     string
}
type reviewMsg struct {
	request uint64
	sha     string
	links   []hosting.ReviewLink
	err     error
}
type reviewOpenMsg struct {
	sha string
	err error
}

func (m *Model) invalidateReview() {
	if m.review.cancel != nil {
		m.review.cancel()
		m.review.cancel = nil
	}
	m.review.request++
	m.review.loading = false
}

func (m *Model) scheduleReview() tea.Cmd {
	if m.reviewer == nil || len(m.history.Versions) == 0 || m.ctx.Err() != nil {
		return nil
	}
	sha := m.history.Versions[m.selected].Commit.Hash
	if sha == "" {
		return nil
	}
	m.invalidateReview()
	m.review.sha, m.review.err, m.review.openErr = sha, nil, nil
	if cached, ok := m.review.cache[sha]; ok {
		m.review.links = append([]hosting.ReviewLink(nil), cached.links...)
		m.clampOffset()
		m.clampHistory()
		m.clampPreview()
		return nil
	}
	m.review.links = nil
	m.review.loading = true
	ctx, cancel := context.WithCancel(m.ctx)
	m.review.cancel = cancel
	id := m.review.request
	return func() tea.Msg {
		timer := time.NewTimer(100 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			return reviewReadyMsg{request: id, sha: sha}
		}
	}
}

func (m *Model) fetchReview(msg reviewReadyMsg) tea.Cmd {
	if m.review.cancel != nil {
		m.review.cancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.review.cancel = cancel
	reviewer := m.reviewer
	return func() tea.Msg {
		defer cancel()
		links, err := reviewer.Fetch(ctx, msg.sha)
		return reviewMsg{request: msg.request, sha: msg.sha, links: links, err: err}
	}
}

func (m *Model) openReview(index int) tea.Cmd {
	if m.reviewer == nil || index < 0 || index >= len(m.review.links) {
		return nil
	}
	link, sha, reviewer := m.review.links[index], m.review.sha, m.reviewer
	m.review.openErr = nil
	return func() tea.Msg {
		err := reviewer.Open(m.ctx, link)
		return reviewOpenMsg{sha: sha, err: err}
	}
}

func (m Model) reviewPrefix() string {
	if m.reviewer != nil && m.reviewer.Provider() == hosting.GitLab {
		return "Merge requests: "
	}
	if m.reviewer != nil && m.reviewer.Provider() == hosting.GitHub {
		return "Pull requests: "
	}
	return "Reviews: "
}

type reviewHit struct {
	row, start, end, index int
}

func (m Model) reviewLayout() ([]string, []reviewHit) {
	if m.reviewer == nil {
		return nil, nil
	}
	prefix := m.reviewPrefix()
	status := "none"
	switch {
	case m.review.loading:
		status = "loading…"
	case m.review.err != nil:
		status = single(m.review.err.Error())
	}
	if len(m.review.links) == 0 || m.review.loading || m.review.err != nil {
		rows := []string{ansi.Truncate(prefix+status, m.width, "…")}
		if m.review.openErr != nil {
			rows = append(rows, ansi.Truncate("Open review: "+single(m.review.openErr.Error()), m.width, "…"))
		}
		return rows, nil
	}

	rows := []string{prefix}
	hits := make([]reviewHit, 0, len(m.review.links))
	indent := strings.Repeat(" ", ansi.StringWidth(prefix))
	for i, link := range m.review.links {
		label := fmt.Sprintf("PR #%d", link.Number)
		if link.Provider == hosting.GitLab {
			label = fmt.Sprintf("MR !%d", link.Number)
		}
		sep := ""
		if i > 0 {
			sep = " · "
		}
		row := len(rows) - 1
		if ansi.StringWidth(rows[row])+ansi.StringWidth(sep+label) > m.width && ansi.StringWidth(rows[row]) > ansi.StringWidth(prefix) {
			rows = append(rows, indent)
			row++
			sep = ""
		}
		start := ansi.StringWidth(rows[row]) + ansi.StringWidth(sep)
		rows[row] += sep + hyperlink(link.URL, label)
		hits = append(hits, reviewHit{row: row, start: start, end: start + ansi.StringWidth(label), index: i})
	}
	if m.review.openErr != nil {
		rows = append(rows, ansi.Truncate("Open review: "+single(m.review.openErr.Error()), m.width, "…"))
	}
	return rows, hits
}

func hyperlink(url, label string) string {
	return "\x1b]8;;" + url + "\x1b\\" + label + "\x1b]8;;\x1b\\"
}
