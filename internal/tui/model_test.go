package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lcaohoanq/ghist/internal/history"
)

type fakeExplorer struct {
	ctx      context.Context
	snapshot history.FileSnapshot
	err      error
}

func (f *fakeExplorer) ExploreFile(context.Context) (history.FileHistory, error) {
	return testHistory(), nil
}
func (f *fakeExplorer) GetDiff(ctx context.Context, v history.FileVersion) (history.FileDiff, error) {
	f.ctx = ctx
	return history.FileDiff{Patch: "+" + v.Commit.Subject}, f.err
}
func (f *fakeExplorer) GetSnapshot(ctx context.Context, _ history.FileVersion) (history.FileSnapshot, error) {
	f.ctx = ctx
	return f.snapshot, f.err
}
func testHistory() history.FileHistory {
	return history.FileHistory{Path: "file", Versions: []history.FileVersion{
		{Commit: history.Commit{Hash: strings.Repeat("a", 40), ShortHash: "aaaaaaa", Subject: "new"}, AfterPath: "file"},
		{Commit: history.Commit{Hash: strings.Repeat("b", 40), ShortHash: "bbbbbbb", Subject: "old"}, AfterPath: "old-file"},
	}}
}
func ready() Model {
	m := New(context.Background(), &fakeExplorer{}, "file")
	m.history = testHistory()
	m.loading = false
	return m
}
func TestNavigationAndStaleResults(t *testing.T) {
	m := ready()
	next, first := m.key("enter")
	m = next.(Model)
	if m.mode != diffView || !m.loading {
		t.Fatal("did not open diff")
	}
	next, second := m.key("p")
	m = next.(Model)
	if m.selected != 1 {
		t.Fatal("did not move older")
	}
	next, _ = m.Update(second())
	m = next.(Model)
	next, _ = m.Update(first())
	m = next.(Model)
	if m.content != "+old" || m.loading {
		t.Fatalf("stale response applied: %+v", m)
	}
	next, cmd := m.key("p")
	m = next.(Model)
	if m.selected != 1 || cmd != nil {
		t.Fatal("older boundary")
	}
	next, cmd = m.key("f")
	m = next.(Model)
	if m.mode != fileView || cmd == nil {
		t.Fatal("file view")
	}
	next, cmd = m.key("esc")
	m = next.(Model)
	if m.mode != diffView || cmd == nil {
		t.Fatal("file back")
	}
	next, _ = m.key("esc")
	m = next.(Model)
	if m.mode != historyView || m.selected != 1 || m.loading {
		t.Fatal("history back")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.content != "" {
		t.Fatal("late result after going back")
	}
}
func TestLoadingCanQuitAndCancel(t *testing.T) {
	m := ready()
	next, load := m.key("f")
	m = next.(Model)
	next, quit := m.key("q")
	m = next.(Model)
	if quit == nil || m.loading {
		t.Fatal("quit while loading")
	}
	load()
	if !errors.Is(m.service.(*fakeExplorer).ctx.Err(), context.Canceled) {
		t.Fatal("request not cancelled")
	}
}
func TestErrorAndSnapshotStates(t *testing.T) {
	for _, tc := range []struct {
		s    history.FileSnapshot
		err  error
		want string
	}{
		{history.FileSnapshot{}, nil, "does not exist"},
		{history.FileSnapshot{Exists: true, Binary: true}, nil, "Binary file"},
		{history.FileSnapshot{Exists: true}, nil, "Empty file"},
		{history.FileSnapshot{}, errors.New("read failed"), "read failed"},
	} {
		m := ready()
		m.service = &fakeExplorer{snapshot: tc.s, err: tc.err}
		next, cmd := m.key("f")
		m = next.(Model)
		next, _ = m.Update(cmd())
		m = next.(Model)
		if !strings.Contains(m.View().Content, tc.want) {
			t.Fatal(m.View().Content)
		}
	}
}
func TestResizeScrollAndControlSequences(t *testing.T) {
	m := ready()
	m.mode = fileView
	m.content = strings.Repeat("line\n", 100) + "\x1b]52;c;malicious\a\x1b[31mtext\x1b[0m"
	next, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m = next.(Model)
	next, _ = m.key("end")
	m = next.(Model)
	if m.offset == 0 {
		t.Fatal("end did not scroll")
	}
	screen := m.View().Content
	if strings.Contains(screen, "malicious") || strings.Contains(safe(m.content), "\x1b") {
		t.Fatal("unsafe content", screen)
	}
	for _, line := range strings.Split(screen, "\n") {
		if ansi.StringWidth(line) > 40 {
			t.Fatal("overflow", line)
		}
	}
	if len(strings.Split(screen, "\n")) > 12 {
		t.Fatal("height overflow")
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 10, Height: 3})
	m = next.(Model)
	if ansi.StringWidth(m.View().Content) > 10 {
		t.Fatal("small terminal overflow")
	}
}

func TestQuitCancelsInitialHistory(t *testing.T) {
	m := New(context.Background(), &fakeExplorer{}, "file")
	next, quit := m.key("ctrl+c")
	if quit == nil || !errors.Is(next.(Model).ctx.Err(), context.Canceled) {
		t.Fatal("initial history context was not cancelled")
	}
}
