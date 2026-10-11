package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lcaohoanq/ghist/internal/hosting"
)

type fakeReviewer struct {
	provider hosting.Provider
	results  map[string][]hosting.ReviewLink
	errs     map[string]error
	fetched  []string
	opened   []hosting.ReviewLink
	openErr  error
}

func (f *fakeReviewer) Provider() hosting.Provider { return f.provider }
func (f *fakeReviewer) Fetch(_ context.Context, sha string) ([]hosting.ReviewLink, error) {
	f.fetched = append(f.fetched, sha)
	return f.results[sha], f.errs[sha]
}
func (f *fakeReviewer) Open(_ context.Context, link hosting.ReviewLink) error {
	f.opened = append(f.opened, link)
	return f.openErr
}

func reviewModel(reviewer *fakeReviewer) Model {
	m := New(context.Background(), &fakeExplorer{}, "file", reviewer)
	m.history = testHistory()
	m.loading = false
	return m
}

func finishReview(t *testing.T, m *Model, wait tea.Cmd) {
	t.Helper()
	if wait == nil {
		t.Fatal("missing debounce command")
	}
	ready := wait()
	next, fetch := m.Update(ready)
	*m = next.(Model)
	if fetch == nil {
		t.Fatal("missing fetch command")
	}
	next, _ = m.Update(fetch())
	*m = next.(Model)
}

func TestReviewLoadingSuccessEmptyAndError(t *testing.T) {
	sha := testHistory().Versions[0].Commit.Hash
	for _, tt := range []struct {
		name    string
		links   []hosting.ReviewLink
		err     error
		loading string
		want    string
	}{
		{"github", []hosting.ReviewLink{{Provider: hosting.GitHub, Number: 12, URL: "https://github.com/acme/repo/pull/12"}}, nil, "Pull requests: loading…", "PR #12"},
		{"gitlab empty", nil, nil, "Merge requests: loading…", "Merge requests: none"},
		{"error", nil, errors.New("authenticate gh"), "Pull requests: loading…", "authenticate gh"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := hosting.GitHub
			if strings.Contains(tt.name, "gitlab") {
				provider = hosting.GitLab
			}
			r := &fakeReviewer{provider: provider, results: map[string][]hosting.ReviewLink{sha: tt.links}, errs: map[string]error{sha: tt.err}}
			m := reviewModel(r)
			wait := m.scheduleReview()
			if got := ansi.Strip(m.View().Content); !strings.Contains(got, tt.loading) {
				t.Fatalf("loading view = %q", got)
			}
			finishReview(t, &m, wait)
			if got := m.View().Content; !strings.Contains(ansi.Strip(got), tt.want) {
				t.Fatalf("result view = %q", got)
			}
			if len(tt.links) > 0 && !strings.Contains(m.View().Content, "\x1b]8;;"+tt.links[0].URL) {
				t.Fatal("missing OSC 8 hyperlink")
			}
			style := "\x1b[1;36m"
			if tt.err != nil {
				style = "\x1b[1;31m"
			}
			if !strings.Contains(m.View().Content, style) {
				t.Fatal("review metadata has the wrong highlight")
			}
		})
	}
}

func TestReviewStaleResponseAndCache(t *testing.T) {
	h := testHistory()
	r := &fakeReviewer{provider: hosting.GitHub, results: map[string][]hosting.ReviewLink{
		h.Versions[0].Commit.Hash: {{Provider: hosting.GitHub, Number: 1, URL: "https://github.com/a/r/pull/1"}},
		h.Versions[1].Commit.Hash: {{Provider: hosting.GitHub, Number: 2, URL: "https://github.com/a/r/pull/2"}},
	}, errs: map[string]error{}}
	m := reviewModel(r)
	firstWait := m.scheduleReview()
	firstReady := firstWait()
	next, firstFetch := m.Update(firstReady)
	m = next.(Model)

	m.selected = 1
	secondWait := m.scheduleReview()
	finishReview(t, &m, secondWait)
	next, _ = m.Update(firstFetch())
	m = next.(Model)
	if len(m.review.links) != 1 || m.review.links[0].Number != 2 {
		t.Fatalf("stale response applied: %+v", m.review.links)
	}

	m.selected = 0
	wait := m.scheduleReview()
	if wait == nil {
		finishReview(t, &m, m.scheduleReview())
	} else {
		finishReview(t, &m, wait)
	}
	fetches := len(r.fetched)
	m.selected = 1
	if cmd := m.scheduleReview(); cmd != nil || len(r.fetched) != fetches {
		t.Fatalf("cached review scheduled fetch: cmd=%v fetched=%d", cmd != nil, len(r.fetched))
	}
}

func TestReviewWrapClickAndOpenKey(t *testing.T) {
	links := []hosting.ReviewLink{
		{Provider: hosting.GitHub, Number: 101, URL: "https://github.com/a/r/pull/101"},
		{Provider: hosting.GitHub, Number: 202, URL: "https://github.com/a/r/pull/202"},
		{Provider: hosting.GitHub, Number: 303, URL: "https://github.com/a/r/pull/303"},
	}
	r := &fakeReviewer{provider: hosting.GitHub, results: map[string][]hosting.ReviewLink{}, errs: map[string]error{}}
	m := reviewModel(r)
	m.width = 30
	m.review.sha = m.history.Versions[0].Commit.Hash
	m.review.links = links
	rows, hits := m.reviewLayout()
	if len(rows) < 2 || len(hits) != len(links) {
		t.Fatalf("review layout did not wrap: rows=%d hits=%+v", len(rows), hits)
	}
	second := hits[1]
	next, open := m.Update(tea.MouseClickMsg{X: second.start, Y: 2 + second.row, Button: tea.MouseLeft})
	m = next.(Model)
	if open == nil {
		t.Fatal("click did not open review")
	}
	next, _ = m.Update(open())
	m = next.(Model)
	if len(r.opened) != 1 || r.opened[0].Number != 202 {
		t.Fatalf("click opened %+v", r.opened)
	}

	next, open = m.key("o")
	m = next.(Model)
	if open == nil {
		t.Fatal("o did not open first review")
	}
	m.Update(open())
	if len(r.opened) != 2 || r.opened[1].Number != 101 {
		t.Fatalf("o opened %+v", r.opened)
	}
}

func TestReviewOpenErrorDoesNotBreakHistory(t *testing.T) {
	r := &fakeReviewer{provider: hosting.GitHub, results: map[string][]hosting.ReviewLink{}, errs: map[string]error{}, openErr: errors.New("browser failed")}
	m := reviewModel(r)
	m.review.sha = m.history.Versions[0].Commit.Hash
	m.review.links = []hosting.ReviewLink{{Provider: hosting.GitHub, Number: 1, URL: "https://github.com/a/r/pull/1"}}
	next, cmd := m.key("o")
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !strings.Contains(ansi.Strip(m.View().Content), "browser failed") || len(m.history.Versions) != 2 {
		t.Fatal("open error disrupted history", m.View().Content)
	}
}
