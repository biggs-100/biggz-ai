package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type sesView int

const (
	sesList sesView = iota
	sesDetail
)

// SessionsModel browses BigMem sessions.
type SessionsModel struct {
	sessions []bigmem.Session
	items    []*bigmem.Observation
	prompts  []bigmem.SavedPrompt
	cursor   int
	scroll   int
	view     sesView
	sesID    string
	err      string
}

// NewSessionsModel creates the sessions screen.
func NewSessionsModel() SessionsModel {
	return SessionsModel{view: sesList}
}

func (m SessionsModel) Init() tea.Cmd { return nil }

type sesListMsg struct {
	sessions []bigmem.Session
	err      string
}
type sesObsMsg struct {
	items   []*bigmem.Observation
	prompts []bigmem.SavedPrompt
	err     string
}

func loadSessions() tea.Msg {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".biggz", "bigmem")
	s, err := bigmem.Open(dbPath)
	if err != nil {
		return sesListMsg{err: err.Error()}
	}
	defer s.Close()
	// Use AllSessions via full.go if available
	// For now, read from session_context which gives recent sessions
	sessions, err := s.SessionContext(20)
	if err != nil {
		return sesListMsg{err: err.Error()}
	}
	return sesListMsg{sessions: sessions}
}

func loadSessionObs(sessionID string) tea.Msg {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".biggz", "bigmem")
	s, err := bigmem.Open(dbPath)
	if err != nil {
		return sesObsMsg{err: err.Error()}
	}
	defer s.Close()
	// Get prompts for this session
	prompts, err := s.ListPromptsBySession(sessionID)
	if err != nil {
		prompts = nil
	}
	// Get observations via search (filter by session reference in content)
	items, err := s.Search("", bigmem.SearchOptions{Limit: 50})
	if err != nil {
		return sesObsMsg{err: err.Error()}
	}
	var filtered []*bigmem.Observation
	for _, item := range items {
		if strings.Contains(item.Content, sessionID) || strings.Contains(item.TopicKey, sessionID) {
			filtered = append(filtered, item)
		}
	}
	return sesObsMsg{items: filtered, prompts: prompts}
}

// Update handles input.
func (m SessionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.view = sesList
			return m, loadSessions
		case "enter", " ":
			if m.view == sesList {
				if !m.sessionsIsLoaded() {
					return m, loadSessions
				}
				if len(m.sessions) > 0 {
					ses := m.sessions[m.cursor]
					m.sesID = ses.ID
					m.view = sesDetail
					m.scroll = 0
					return m, func() tea.Msg { return loadSessionObs(ses.ID) }
				}
			}
		case "d":
			if m.view == sesDetail {
				m.view = sesList
				return m, nil
			}
		case "up", "k":
			if m.view == sesList && m.cursor > 0 {
				m.cursor--
			}
			if m.view == sesDetail && m.scroll > 0 {
				m.scroll--
			}
		case "down", "j":
			if m.view == sesList && m.cursor < len(m.sessions)-1 {
				m.cursor++
			}
			if m.view == sesDetail {
				m.scroll++
			}
		case "esc":
			if m.view == sesDetail {
				m.view = sesList
				m.items = nil
				m.prompts = nil
				return m, nil
			}
		}

	case sesListMsg:
		if msg.err != "" {
			m.err = msg.err
			return m, nil
		}
		m.sessions = msg.sessions
		if m.sessions == nil {
			m.sessions = []bigmem.Session{}
		}
		m.err = ""

	case sesObsMsg:
		if msg.err != "" {
			m.err = msg.err
			return m, nil
		}
		m.items = msg.items
		if m.items == nil {
			m.items = []*bigmem.Observation{}
		}
		m.prompts = msg.prompts
		if m.prompts == nil {
			m.prompts = []bigmem.SavedPrompt{}
		}
	}

	return m, nil
}

func (m SessionsModel) sessionsIsLoaded() bool {
	return m.sessions != nil
}

// View renders.
func (m SessionsModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Sessions"))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render(m.err))
		b.WriteString("\n\n")
	}

	switch m.view {
	case sesDetail:
		b.WriteString(styles.Section.Render(fmt.Sprintf("Session Detail: %s", m.sesID)))
		b.WriteString("\n\n")

		// Find session metadata
		var currentSession *bigmem.Session
		for i := range m.sessions {
			if m.sessions[i].ID == m.sesID {
				currentSession = &m.sessions[i]
				break
			}
		}
		if currentSession != nil {
			b.WriteString(fmt.Sprintf("  Started: %s\n", currentSession.StartTime.Format("Jan 2, 2006 15:04")))
			if !currentSession.EndTime.IsZero() {
				b.WriteString(fmt.Sprintf("  Ended:   %s\n", currentSession.EndTime.Format("15:04")))
			}
			if currentSession.Summary != "" {
				b.WriteString(fmt.Sprintf("  Summary: %s\n", currentSession.Summary))
			}
			if currentSession.Project != "" {
				b.WriteString(fmt.Sprintf("  Project: %s\n", currentSession.Project))
			}
			b.WriteString("\n")
		}

		// Prompts (user messages)
		if len(m.prompts) > 0 {
			b.WriteString(styles.StatusEnabled.Render(fmt.Sprintf("Prompts (%d):", len(m.prompts))))
			b.WriteString("\n")
			start := m.scroll
			end := start + 8
			if start > len(m.prompts) {
				start = len(m.prompts)
			}
			if end > len(m.prompts) {
				end = len(m.prompts)
			}
			for _, p := range m.prompts[start:end] {
				content := p.Content
				if len(content) > 80 {
					content = content[:80] + "..."
				}
				b.WriteString(fmt.Sprintf("  • %s\n", content))
			}
			if len(m.prompts) > 8 {
				b.WriteString(fmt.Sprintf("  (%d more... scroll with ↑↓)\n", len(m.prompts)-8))
			}
			b.WriteString("\n")
		}

		// Observations
		if len(m.items) > 0 {
			b.WriteString(styles.StatusEnabled.Render(fmt.Sprintf("Observations (%d):", len(m.items))))
			b.WriteString("\n")
			for _, item := range m.items {
				b.WriteString(fmt.Sprintf("  [%s] %s\n", item.Type, item.Title))
			}
		} else if len(m.prompts) == 0 {
			b.WriteString(styles.StatusInfo.Render("No data for this session yet."))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ scroll · ESC back"))

	default:
		if !m.sessionsIsLoaded() {
			b.WriteString("Press ENTER or [R] to load sessions.\n\n")
			b.WriteString(styles.StatusInfo.Render("Sessions group observations by conversation."))
		} else if len(m.sessions) == 0 {
			b.WriteString(styles.StatusInfo.Render("No sessions found."))
		} else {
			b.WriteString(styles.Section.Render(fmt.Sprintf("Recent Sessions (%d)", len(m.sessions))))
			b.WriteString("\n\n")
			for i, s := range m.sessions {
				cur := "  "
				if i == m.cursor {
					cur = "▸ "
				}
				b.WriteString(fmt.Sprintf("%s%s  %s\n", cur, s.ID, s.StartTime.Format("Jan 2 15:04")))
				if s.Summary != "" {
					b.WriteString(fmt.Sprintf("    %s\n", s.Summary))
				}
			}
			b.WriteString("\n")
			b.WriteString(styles.StatusInfo.Render("ENTER view details"))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("[R] refresh · ENTER view · ESC back"))
	return styles.AppStyle.Render(b.String())
}
