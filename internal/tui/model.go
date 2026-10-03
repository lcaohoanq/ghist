// Package tui presents history services in an interactive terminal.
package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lcaohoanq/ghist/internal/history"
)

type Explorer interface {
	ExploreFile(context.Context) (history.FileHistory, error)
	GetDiff(context.Context, history.FileVersion) (history.FileDiff, error)
	GetSnapshot(context.Context, history.FileVersion) (history.FileSnapshot, error)
}

type viewMode int

const (
	historyView viewMode = iota
	diffView
	fileView
)

type Model struct {
	followRenames                   bool
	pickerEnabled, backToPicker     bool
	historyCursor, historyOffset    int
	historyEvents                   chan historyBatchMsg
	historyLoading                  bool
	historyComplete                 bool
	historyElapsed                  time.Duration
	historyErr                      error
	renderCache                     *renderCache
	collapsed                       map[int]bool
	collapseRevision                uint64
	preview                         previewState
	previewFocus                    bool
	diffUnified                     bool
	stop                            context.CancelFunc
	ctx                             context.Context
	service                         Explorer
	path                            string
	history                         history.FileHistory
	selected, offset, width, height int
	mode                            viewMode
	loading                         bool
	err                             error
	content                         string
	request                         uint64
	cancel                          context.CancelFunc
}

type historyMsg struct {
	elapsed time.Duration
	history history.FileHistory
	err     error
}
type contentMsg struct {
	request uint64
	content string
	err     error
}

func New(ctx context.Context, service Explorer, path string) Model {
	ctx, stop := context.WithCancel(ctx)
	return Model{historyEvents: make(chan historyBatchMsg, 1), renderCache: &renderCache{}, ctx: ctx, stop: stop, service: service, path: path, width: 80, height: 24, loading: true, historyCursor: 1}
}

// WithPicker enables returning to file selection with Esc from History.
func (m Model) WithPicker() Model {
	m.pickerEnabled = true
	return m
}

// BackToPicker distinguishes navigation from quitting the application.
func (m Model) BackToPicker() bool { return m.backToPicker }

func (m Model) Init() tea.Cmd {
	if source, ok := m.service.(history.HistoryStreamer); ok {
		return func() tea.Msg {
			go streamHistory(m.ctx, source, m.historyEvents)
			return m.awaitHistory()()
		}
	}
	return func() tea.Msg {
		started := time.Now()
		h, err := m.service.ExploreFile(m.ctx)
		return historyMsg{history: h, err: err, elapsed: time.Since(started)}
	}
}
func (m *Model) invalidate() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.request++
	m.loading = false
}
func (m *Model) load(mode viewMode) tea.Cmd { return m.loadContent(mode, false) }
func (m *Model) loadContent(mode viewMode, full bool) tea.Cmd {
	if len(m.history.Versions) == 0 {
		return nil
	}
	m.invalidate()
	m.invalidatePreview()
	m.previewFocus = false
	m.mode = mode
	m.offset = 0
	m.content = ""
	m.err = nil
	m.loading = true
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	id, version, service := m.request, m.history.Versions[m.selected], m.service
	return func() tea.Msg {
		defer cancel()
		if mode == diffView {
			var d history.FileDiff
			var err error
			if source, ok := service.(fullExplorer); full && ok {
				d, err = source.GetFullDiff(ctx, version)
			} else {
				d, err = service.GetDiff(ctx, version)
			}
			if d.Truncated {
				d.Patch = limitedNotice + d.Patch
			}
			if err == nil && d.Patch == "" {
				d.Patch = "No changes relative to the first parent."
			}
			return contentMsg{id, d.Patch, err}
		}
		var s history.FileSnapshot
		var err error
		if source, ok := service.(fullExplorer); full && ok {
			s, err = source.GetFullSnapshot(ctx, version)
		} else {
			s, err = service.GetSnapshot(ctx, version)
		}
		text := s.Content
		if err == nil {
			switch {
			case !s.Exists:
				text = "File does not exist at this commit. Press d to view the deletion diff."
			case s.Binary:
				text = "Binary file — snapshot is not displayed."
			case s.Content == "":
				text = "Empty file."
			}
		}
		if s.Truncated && !s.Binary {
			text = limitedNotice + text
		}
		return contentMsg{id, text, err}
	}
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case previewReadyMsg:
		if msg.request == m.preview.request && m.previewVisible() && m.ctx.Err() == nil {
			cmd := m.fetchPreview()
			return m, cmd
		}
	case previewMsg:
		if msg.request == m.preview.request && m.previewVisible() {
			m.preview.loading = false
			m.preview.content, m.preview.err = msg.content, msg.err
			m.clampPreview()
		}
	case tea.MouseClickMsg:
		return m.mouse(msg.Mouse(), false)
	case tea.MouseWheelMsg:
		return m.mouse(msg.Mouse(), true)
	case tea.WindowSizeMsg:
		wasVisible := m.previewVisible()
		m.width = max(1, msg.Width)
		m.height = max(1, msg.Height)
		m.clampOffset()
		m.clampHistory()
		m.clampPreview()
		if !m.previewVisible() {
			m.invalidatePreview()
			m.previewFocus = false
		} else if !wasVisible {
			cmd := m.schedulePreview()
			return m, cmd
		}
	case historyBatchMsg:
		if m.ctx.Err() != nil {
			return m, nil
		}
		first := len(m.history.Versions) == 0
		m.historyLoading = !msg.done
		m.historyComplete = msg.done && msg.err == nil
		m.historyElapsed = msg.elapsed
		m.historyErr = msg.err
		m.history.Versions = append(m.history.Versions, msg.versions...)
		if m.mode == historyView {
			m.loading = len(m.history.Versions) == 0 && !msg.done
		}
		var preview tea.Cmd
		if first && len(m.history.Versions) > 0 {
			m.revealSelected()
			preview = m.schedulePreview()
		}
		if msg.done {
			if len(m.history.Versions) == 0 && msg.err == nil {
				m.historyErr = history.ErrNoHistory
			}
			return m, preview
		}
		return m, tea.Batch(m.awaitHistory(), preview)
	case historyMsg:
		if m.ctx.Err() != nil {
			return m, nil
		}
		m.historyComplete = msg.err == nil && len(msg.history.Versions) > 0
		m.historyElapsed = msg.elapsed
		m.historyLoading = false
		m.renderCache = &renderCache{}
		m.loading = false
		m.history = msg.history
		m.selected = 0
		m.collapsed = nil
		m.historyOffset = 0
		m.revealSelected()
		m.err = msg.err
		if m.err == nil && len(m.history.Versions) == 0 {
			m.err = history.ErrNoHistory
		}
		cmd := m.schedulePreview()
		return m, cmd
	case contentMsg:
		if msg.request != m.request {
			return m, nil
		}
		m.loading = false
		m.err = msg.err
		m.content = msg.content
		m.clampOffset()
	case tea.KeyPressMsg:
		return m.key(msg.String())
	}
	return m, nil
}
func (m Model) key(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "ctrl+c":
		if m.stop != nil {
			m.stop()
		}
		m.invalidate()
		m.invalidatePreview()
		return m, tea.Quit
	case "s":
		if m.mode != fileView {
			m.diffUnified = !m.diffUnified
			m.offset = 0
			m.preview.offset = 0
		}
		return m, nil
	case "tab":
		if m.previewVisible() {
			m.previewFocus = !m.previewFocus
		}
		return m, nil
	case "esc":
		if m.mode == historyView && m.pickerEnabled {
			m.backToPicker = true
			return m.key("q")
		}
		m.previewFocus = false
		if m.mode == fileView {
			cmd := m.load(diffView)
			return m, cmd
		}
		if m.mode == diffView {
			m.invalidate()
			m.mode = historyView
			m.clampHistory()
			m.offset = 0
			m.err = nil
			m.content = ""
			cmd := m.schedulePreview()
			return m, cmd
		}
		return m, nil
	}
	if len(m.history.Versions) == 0 {
		return m, nil
	}
	if m.mode == historyView && !m.previewFocus {
		rows := m.historyRows()
		header := len(rows) > 0 && rows[min(m.historyCursor, len(rows)-1)].header
		if key == "left" || key == "right" || (header && (key == "enter" || key == "space" || key == " ")) {
			m.toggleHistoryGroup(key)
			return m, nil
		}
	}
	switch key {
	case "enter":
		if m.mode == historyView {
			cmd := m.load(diffView)
			return m, cmd
		}
		if m.mode == diffView {
			cmd := m.load(fileView)
			return m, cmd
		}
	case "L":
		mode := m.mode
		if mode == historyView {
			mode = diffView
		}
		cmd := m.loadContent(mode, true)
		return m, cmd
	case "d":
		cmd := m.load(diffView)
		return m, cmd
	case "f":
		cmd := m.load(fileView)
		return m, cmd
	case "p", "n":
		delta := 1
		if key == "n" {
			delta = -1
		}
		next := m.history.Move(m.selected, delta)
		if next != m.selected {
			m.selected = next
			m.revealSelected()
			if m.mode != historyView {
				cmd := m.load(m.mode)
				return m, cmd
			}
			cmd := m.schedulePreview()
			return m, cmd
		}
	case "up", "k", "down", "j", "pgup", "pgdown", "home", "end":
		delta := 1
		if key == "up" || key == "k" {
			delta = -1
		}
		if key == "pgup" {
			delta = -m.bodyHeight()
		}
		if key == "pgdown" {
			delta = m.bodyHeight()
		}
		if m.mode == historyView && m.previewFocus && m.previewVisible() {
			m.preview.offset += delta
			if key == "home" {
				m.preview.offset = 0
			}
			if key == "end" {
				m.preview.offset = len(m.previewLines())
			}
			m.clampPreview()
		} else if m.mode == historyView {
			next := m.historyCursor + delta
			if key == "home" {
				next = 0
			}
			if key == "end" {
				next = len(m.historyRows()) - 1
			}
			cmd := m.focusHistoryRow(next)
			return m, cmd
		} else {
			m.offset += delta
			if key == "home" {
				m.offset = 0
			}
			if key == "end" {
				m.offset = len(m.lines())
			}
			m.clampOffset()
		}
	}
	return m, nil
}
func (m Model) bodyHeight() int {
	chrome := 7
	if m.mode == historyView {
		chrome++
	}
	return max(1, m.height-chrome)
}
func (m *Model) clampOffset() {
	m.offset = max(0, min(m.offset, max(0, len(m.lines())-m.bodyHeight())))
}
func (m Model) metadata() []string {
	if len(m.history.Versions) == 0 {
		return []string{"", "", ""}
	}
	v := m.history.Versions[m.selected]
	c := v.Commit
	parent := "root commit (empty tree)"
	if v.Parent != "" {
		parent = "parent " + v.Parent[:min(7, len(v.Parent))]
	}
	if v.Merge {
		parent = "merge; first " + parent
	}
	return []string{
		fmt.Sprintf("%s  %s", c.ShortHash, c.Subject),
		fmt.Sprintf("%s <%s>  %s", c.Author, c.Email, c.Date.Format("02-01-2006 15:04:05 -07:00")),
		fmt.Sprintf("%s | %s | version %d/%d", v.Path(), parent, m.selected+1, len(m.history.Versions)),
	}
}

const limitedNotice = "Output limited to 2 MiB. Press L to load full output.\n"

type fullExplorer interface {
	GetFullDiff(context.Context, history.FileVersion) (history.FileDiff, error)
	GetFullSnapshot(context.Context, history.FileVersion) (history.FileSnapshot, error)
}

// WithFollowRenames labels the traversal mode configured on the repository.
func (m Model) WithFollowRenames(follow bool) Model { m.followRenames = follow; return m }
