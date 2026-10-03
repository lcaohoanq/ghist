package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lcaohoanq/ghist/internal/history"
)

func updateModel(m *Model, msg tea.Msg) tea.Cmd {
	next, cmd := m.Update(msg)
	*m = next.(Model)
	return cmd
}
func keyModel(m *Model, key string) tea.Cmd { next, cmd := m.key(key); *m = next.(Model); return cmd }
func TestPreviewLifecycle(t *testing.T) {
	m := New(context.Background(), &fakeExplorer{}, "file")
	defer m.stop()
	updateModel(&m, tea.WindowSizeMsg{Width: 120, Height: 24})
	wait := updateModel(&m, m.Init()())
	if wait == nil || !m.preview.loading || m.loading {
		t.Fatal("initial preview not scheduled")
	}
	start := time.Now()
	fetch := updateModel(&m, wait())
	if time.Since(start) < 90*time.Millisecond {
		t.Fatal("missing debounce")
	}
	old := fetch()
	oldID := m.preview.request
	wait = keyModel(&m, "j")
	if m.selected != 1 || m.preview.content != "" || !m.preview.loading {
		t.Fatal("selection did not clear preview")
	}
	if cmd := updateModel(&m, previewReadyMsg{oldID}); cmd != nil {
		t.Fatal("stale timer started Git")
	}
	fetch = updateModel(&m, wait())
	updateModel(&m, fetch())
	updateModel(&m, old)
	if m.preview.content != "+old" || m.preview.loading {
		t.Fatal("stale response applied")
	}
	wait = keyModel(&m, "n")
	updateModel(&m, tea.WindowSizeMsg{Width: 109, Height: 24})
	if wait() != nil || m.preview.loading || m.previewFocus {
		t.Fatal("hidden preview not cancelled")
	}
	wait = updateModel(&m, tea.WindowSizeMsg{Width: 110, Height: 24})
	if wait == nil {
		t.Fatal("resize did not reload")
	}
	fetch = updateModel(&m, wait())
	keyModel(&m, "q")
	if fetch() != nil {
		t.Fatal("quit did not cancel fetch")
	}
}
func TestPreviewNavigationAndMouse(t *testing.T) {
	m := ready()
	defer m.stop()
	m.width = 120
	m.preview.content = strings.Repeat("+line\n", 100)
	keyModel(&m, "tab")
	keyModel(&m, "end")
	if !m.previewFocus || m.preview.offset == 0 || m.selected != 0 {
		t.Fatal("preview focus navigation")
	}
	keyModel(&m, "home")
	keyModel(&m, "pgdown")
	if m.preview.offset != m.bodyHeight() {
		t.Fatal("page scroll")
	}
	updateModel(&m, tea.MouseWheelMsg{X: 115, Y: 8, Button: tea.MouseWheelUp})
	if m.preview.offset != m.bodyHeight()-1 || m.selected != 0 {
		t.Fatal("preview wheel")
	}
	updateModel(&m, tea.MouseWheelMsg{X: 2, Y: 8, Button: tea.MouseWheelDown})
	if m.selected != 1 || m.preview.offset != 0 {
		t.Fatal("list wheel")
	}
	updateModel(&m, tea.MouseClickMsg{X: 2, Y: 7, Button: tea.MouseLeft})
	if m.selected != 0 || m.previewFocus {
		t.Fatal("list click")
	}
	updateModel(&m, tea.MouseClickMsg{X: 115, Y: 8, Button: tea.MouseLeft})
	if !m.previewFocus {
		t.Fatal("preview click")
	}
	keyModel(&m, "p")
	if m.selected != 1 {
		t.Fatal("p must select with preview focus")
	}
	keyModel(&m, "esc")
	if m.previewFocus {
		t.Fatal("escape focus")
	}
	for _, key := range []string{"enter", "f", "esc", "esc"} {
		keyModel(&m, key)
	}
	if m.mode != historyView || m.selected != 1 || !m.preview.loading {
		t.Fatal("detail return")
	}
}

type patchExplorer struct {
	fakeExplorer
	patch string
}

func (f *patchExplorer) GetDiff(context.Context, history.FileVersion) (history.FileDiff, error) {
	return history.FileDiff{Patch: f.patch}, f.err
}
func TestPreviewContentAndIsolation(t *testing.T) {
	for _, tc := range []struct {
		patch string
		err   error
		want  string
	}{
		{"", nil, "No changes"},
		{"Binary files a/file and b/file differ", nil, "Binary files"},
		{"deleted file mode 100644\n-old", nil, "deleted file"},
		{"", errors.New("Git failed"), "Git failed"},
	} {
		m := ready()
		m.width = 120
		m.service = &patchExplorer{patch: tc.patch, fakeExplorer: fakeExplorer{err: tc.err}}
		wait := m.schedulePreview()
		fetch := updateModel(&m, wait())
		updateModel(&m, fetch())
		if !strings.Contains(m.View().Content, tc.want) || m.err != nil || m.loading {
			t.Fatal("preview state leaked", m.View().Content)
		}
		keyModel(&m, "j")
		if m.selected != 1 {
			t.Fatal("list blocked")
		}
		m.stop()
	}
}
func TestPreviewLayoutUnicodeAndResize(t *testing.T) {
	m := ready()
	defer m.stop()
	m.history.Versions[0].Commit.Author = "山田太郎"
	m.history.Versions[1].Commit.Author = "Lưu Cao Hoàng"
	for _, width := range []int{109, 110, 120, 180} {
		m.width = width
		m.preview.content = "@@ -0,0 +1,1 @@\n+" + strings.Repeat("界🙂", 200) + "\n\x1b]52;c;malicious\a-old"
		screen := m.View().Content
		rows := strings.Split(screen, "\n")
		if len(rows) != m.height {
			t.Fatal("height overflow")
		}
		for _, row := range rows {
			if ansi.StringWidth(row) > width {
				t.Fatal("width overflow", width, row)
			}
		}
		if strings.Contains(screen, "malicious") {
			t.Fatal("unsafe escape")
		}
		if width >= 110 {
			if !strings.Contains(screen, "Diff · aaaaaaa") {
				t.Fatal("missing title")
			}
			for _, row := range rows[6 : m.height-2] {
				plain := ansi.Strip(row)
				i := strings.Index(plain, "│")
				if i < 0 || ansi.StringWidth(plain[:i]) != m.listWidth() {
					t.Fatal("divider misaligned")
				}
			}
			if !strings.Contains(rows[8], "\x1b["+addedStyle+"m") {
				t.Fatal("wrapped addition lost color")
			}
			c := m.columns()
			a := c.row("  ", "aaaaaaa", "2026-01-01", "山田太郎", "MESSAGE", false)
			b := c.row("  ", "bbbbbbb", "2026-01-01", "Lưu Cao Hoàng", "MESSAGE", false)
			if ansi.StringWidth(strings.Split(a, "MESSAGE")[0]) != ansi.StringWidth(strings.Split(b, "MESSAGE")[0]) {
				t.Fatal("author alignment")
			}
		} else if strings.Contains(screen, "Diff ·") {
			t.Fatal("preview visible at narrow width")
		}
	}
}

// Model updates must remain responsive while Git is running, and cancellation
// must reach the actual request context, not only its debounce timer.
type blockingExplorer struct {
	fakeExplorer
	started chan context.Context
	release chan struct{}
}

func (f *blockingExplorer) GetDiff(ctx context.Context, _ history.FileVersion) (history.FileDiff, error) {
	f.started <- ctx
	<-f.release
	return history.FileDiff{Patch: "+late"}, nil
}
func TestPreviewCancelsRunningGit(t *testing.T) {
	m := ready()
	defer m.stop()
	m.width = 120
	service := &blockingExplorer{started: make(chan context.Context, 1), release: make(chan struct{})}
	m.service = service
	wait := m.schedulePreview()
	fetch := updateModel(&m, wait())
	result := make(chan tea.Msg, 1)
	go func() { result <- fetch() }()
	ctx := <-service.started
	keyModel(&m, "j")
	if ctx.Err() != context.Canceled {
		t.Error("selection did not cancel running Git")
	}
	close(service.release)
	updateModel(&m, <-result)
	if m.preview.content != "" || !m.preview.loading {
		t.Fatal("cancelled Git response replaced current loading state")
	}
}
