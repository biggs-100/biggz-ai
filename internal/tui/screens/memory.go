package screens

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/bigmem"
	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type memView int

const (
	memViewList memView = iota
	memViewSearch
	memViewDetail
	memViewTimeline
)

// MemoryModel browses BigMem persistent memory.
type MemoryModel struct {
	store    *bigmem.Store
	items    []*bigmem.Observation
	cursor   int
	view     memView
	searchQ  string
	detail   *bigmem.Observation
	loaded   bool
	err      string
	copied   bool // shows "Copied!" feedback
	session  string
	tlEntries []bigmem.TimelineEntry
}

// NewMemoryModel creates the memory screen.
func NewMemoryModel() MemoryModel {
	return MemoryModel{view: memViewList}
}

func (m MemoryModel) Init() tea.Cmd { return nil }

// openStore opens BigMem.
func (m *MemoryModel) openStore() error {
	if m.store != nil {
		return nil
	}
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".biggz", "bigmem")
	s, err := bigmem.Open(dbPath)
	if err != nil {
		return err
	}
	m.store = s
	return nil
}

// memoryListMsg, searchMsg, detailMsg, resultMsg, clipboardMsg
type memoryListMsg struct {
	items []*bigmem.Observation
	err   string
}
type memSearchMsg struct {
	items []*bigmem.Observation
	err   string
}
type memDetailMsg struct {
	obs *bigmem.Observation
	err string
}
type memResultMsg struct {
	status string
	err    string
}
type memTimelineMsg struct {
	entries []bigmem.TimelineEntry
	err     string
}

func loadTimelineData(focusID string) tea.Msg {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".biggz", "bigmem")
	s, err := bigmem.Open(dbPath)
	if err != nil {
		return memTimelineMsg{err: err.Error()}
	}
	defer s.Close()
	entries, err := s.Timeline(bigmem.TimelineOptions{FocusID: focusID, Before: 5, After: 5})
	if err != nil {
		return memTimelineMsg{err: err.Error()}
	}
	return memTimelineMsg{entries: entries}
}

func loadRecent() tea.Msg {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".biggz", "bigmem")
	s, err := bigmem.Open(dbPath)
	if err != nil {
		return memoryListMsg{err: err.Error()}
	}
	defer s.Close()
	items, err := s.Search("", bigmem.SearchOptions{Limit: 15})
	if err != nil {
		return memoryListMsg{err: err.Error()}
	}
	return memoryListMsg{items: items}
}

func doSearch(query string) tea.Msg {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".biggz", "bigmem")
	s, err := bigmem.Open(dbPath)
	if err != nil {
		return memSearchMsg{err: err.Error()}
	}
	defer s.Close()
	items, err := s.Search(query, bigmem.SearchOptions{Limit: 20})
	if err != nil {
		return memSearchMsg{err: err.Error()}
	}
	return memSearchMsg{items: items}
}

func loadDetail(id string) tea.Msg {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".biggz", "bigmem")
	s, err := bigmem.Open(dbPath)
	if err != nil {
		return memDetailMsg{err: err.Error()}
	}
	defer s.Close()
	obs, err := s.Get(id)
	if err != nil {
		return memDetailMsg{err: err.Error()}
	}
	return memDetailMsg{obs: obs}
}

// Update handles input.
func (m MemoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.view = memViewList
			return m, loadRecent
		case "/":
			if m.view != memViewDetail {
				m.view = memViewSearch
				m.searchQ = ""
				return m, nil
			}
		case "enter":
			if m.view == memViewSearch && m.searchQ != "" {
				return m, func() tea.Msg { return doSearch(m.searchQ) }
			}
			if m.view == memViewList && len(m.items) > 0 {
				return m, func() tea.Msg { return loadDetail(m.items[m.cursor].ID) }
			}
		case "t":
			if m.view == memViewList && len(m.items) > 0 {
				m.view = memViewTimeline
				focusID := m.items[m.cursor].ID
				return m, func() tea.Msg { return loadTimelineData(focusID) }
			}
		case "c":
			if m.view == memViewDetail && m.detail != nil {
				// OSC 52 clipboard copy
				content := fmt.Sprintf("%s\n\n%s\n\nType: %s | Project: %s | Created: %s",
					m.detail.Title, m.detail.Content, m.detail.Type,
					m.detail.Project, m.detail.CreatedAt.Format("Jan 2, 2006 15:04"))
				b64 := base64.StdEncoding.EncodeToString([]byte(content))
				seq := fmt.Sprintf("\x1b]52;c;%s\x07", b64)
				// tea.Println sends to stdout outside the view
				return m, func() tea.Msg {
					fmt.Print(seq)
					return nil
				}
			}
		case "up", "k":
			if m.view == memViewList && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.view == memViewList && m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "backspace":
			if m.view == memViewSearch && len(m.searchQ) > 0 {
				m.searchQ = m.searchQ[:len(m.searchQ)-1]
			}
		case "esc":
			if m.view == memViewDetail {
				m.view = memViewList
				m.detail = nil
				return m, nil
			}
			if m.view == memViewSearch {
				m.view = memViewList
				return m, nil
			}
		default:
			if m.view == memViewSearch && len(msg.String()) == 1 && msg.String() != "r" {
				m.searchQ += msg.String()
			}
		}

	case memoryListMsg:
		if msg.err != "" { m.err = msg.err; return m, nil }
		m.items = msg.items
		if m.items == nil { m.items = []*bigmem.Observation{} }
		m.loaded = true
		m.view = memViewList
		m.err = ""

	case memSearchMsg:
		if msg.err != "" { m.err = msg.err; return m, nil }
		m.items = msg.items
		if m.items == nil { m.items = []*bigmem.Observation{} }
		m.view = memViewList
		m.err = ""

	case memDetailMsg:
		if msg.err != "" { m.err = msg.err; return m, nil }
		m.detail = msg.obs
		m.view = memViewDetail
		m.err = ""
	case memTimelineMsg:
		if msg.err != "" { m.err = msg.err; return m, nil }
		m.tlEntries = msg.entries
		m.view = memViewTimeline
		m.err = ""
	}

	return m, nil
}

// View renders.
func (m MemoryModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("BigMem Persistent Memory"))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render(m.err))
		b.WriteString("\n\n")
	}

	switch m.view {
	case memViewSearch:
		b.WriteString(styles.Section.Render("Search"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Query: %s█\n", m.searchQ))
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("Type query, ENTER to search, ESC to cancel"))

	case memViewDetail:
		b.WriteString(styles.Section.Render(m.detail.Title))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Type:    %s\n", m.detail.Type))
		b.WriteString(fmt.Sprintf("  Created: %s\n", m.detail.CreatedAt.Format("Jan 2, 2006 15:04")))
		if m.detail.Project != "" {
			b.WriteString(fmt.Sprintf("  Project: %s\n", m.detail.Project))
		}
		if m.detail.TopicKey != "" {
			b.WriteString(fmt.Sprintf("  Topic:   %s\n", m.detail.TopicKey))
		}
		b.WriteString(fmt.Sprintf("\n%s\n", m.detail.Content))
		b.WriteString("\n\n")
		b.WriteString(styles.Help.Render("[C] copy · ESC back"))

	case memViewTimeline:
		b.WriteString(styles.Section.Render("Timeline"))
		b.WriteString("\n\n")
		if len(m.tlEntries) == 0 {
			b.WriteString(styles.StatusInfo.Render("No timeline data."))
		} else {
			for _, e := range m.tlEntries {
				timeStr := e.CreatedAt.Format("15:04")
				if e.IsFocus {
					b.WriteString(styles.StatusEnabled.Render(fmt.Sprintf("\n  >>> [%s] %s\n", e.Type, e.Title)))
					b.WriteString(fmt.Sprintf("      %s\n", e.CreatedAt.Format("Jan 2, 2006 15:04:05")))
				} else if e.IsBefore {
					b.WriteString(fmt.Sprintf("  ↑ %s  [%s] %s\n", timeStr, e.Type, e.Title))
				} else {
					b.WriteString(fmt.Sprintf("  ↓ %s  [%s] %s\n", timeStr, e.Type, e.Title))
				}
			}
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("[T] toggle timeline · ESC back"))

	case memViewList:
		if !m.loaded {
			b.WriteString("Press [R] to load recent memories.\n\n")
			b.WriteString(styles.StatusInfo.Render("BigMem stores decisions, bugs, discoveries, and\nconventions across sessions."))
		} else if len(m.items) == 0 {
			b.WriteString(styles.StatusInfo.Render("No memories found."))
		} else {
			b.WriteString(styles.Section.Render(fmt.Sprintf("Memories (%d)", len(m.items))))
			b.WriteString("\n\n")
			for i, item := range m.items {
				cur := "  "
				if i == m.cursor { cur = "▸ " }
				b.WriteString(fmt.Sprintf("%s[%s] %s\n", cur, item.Type, item.Title))
			}
			b.WriteString("\n")
			b.WriteString(styles.StatusInfo.Render("ENTER detail · / search"))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("[R] refresh · / search · T timeline · C copy · ESC back · ? help"))
	return styles.AppStyle.Render(b.String())
}
