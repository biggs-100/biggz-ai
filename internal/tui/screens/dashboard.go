package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/bigmem"
	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// DashboardModel shows stats, projects, and quick actions (like engram TUI).
type DashboardModel struct {
	store    *bigmem.Store
	stats    *bigmem.StoreStats
	projects []bigmem.ProjectSummary
	loaded   bool
	err      string
	cursor   int
}

// dashboardActions mirrors engram's main menu.
var dashboardActions = []struct {
	key  string
	desc string
	view int
}{
	{"s", "Search memories", 12},      // screenMemSearch
	{"r", "Recent observations", 4},    // screenMemory (already has list)
	{"e", "Browse sessions", 11},       // screenSessions
	{"i", "Install agent plugin", 1},   // screenInstall
	{"q", "Quit", -1},
}

// NewDashboardModel creates the dashboard.
func NewDashboardModel() DashboardModel {
	return DashboardModel{cursor: 0}
}

func (m DashboardModel) Init() tea.Cmd { return loadDashboard }

type dashboardStatsMsg struct {
	stats    *bigmem.StoreStats
	projects []bigmem.ProjectSummary
	err      string
}

func loadDashboard() tea.Msg {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".biggz", "bigmem")
	s, err := bigmem.Open(dbPath)
	if err != nil {
		return dashboardStatsMsg{err: err.Error()}
	}
	defer s.Close()

	stats, err := s.Stats()
	if err != nil {
		return dashboardStatsMsg{err: err.Error()}
	}
	projects, err := s.ListProjects()
	if err != nil {
		projects = nil
	}
	return dashboardStatsMsg{stats: stats, projects: projects}
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 { m.cursor-- }
		case "down", "j":
			if m.cursor < len(dashboardActions)-1 { m.cursor++ }
		case "enter", " ":
			view := dashboardActions[m.cursor].view
			if view == -1 {
				return m, func() tea.Msg { return QuitMsg{} }
			}
			return m, func() tea.Msg { return NavigateMsg{Screen: view} }
		case "s":
			return m, func() tea.Msg { return NavigateMsg{Screen: 12} }
		case "r":
			return m, func() tea.Msg { return NavigateMsg{Screen: 4} }
		case "e":
			return m, func() tea.Msg { return NavigateMsg{Screen: 11} }
		case "i":
			return m, func() tea.Msg { return NavigateMsg{Screen: 1} }
		case "q":
			return m, func() tea.Msg { return QuitMsg{} }
		case "esc":
			return m, tea.Quit
		}

	case dashboardStatsMsg:
		if msg.err != "" { m.err = msg.err; return m, nil }
		m.stats = msg.stats
		m.projects = msg.projects
		m.loaded = true
		m.err = ""
	}

	return m, nil
}

func (m DashboardModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("biggz-ai"))
	b.WriteString("\n\n")

	// Stats box (like engram)
	b.WriteString("┌─ Stats ─────────────────────────────────────────────┐\n")
	if m.stats != nil {
		b.WriteString(fmt.Sprintf("│  %-10s %-6d  %-10s %-6d  │\n", "Sessions", m.stats.TotalSessions, "Obs", m.stats.TotalObservations))
		b.WriteString(fmt.Sprintf("│  %-10s %-6d  %-10s %-6d  │\n", "Projects", len(m.projects), "Prompts", 0))
	} else if !m.loaded {
		b.WriteString("│  Loading...                                          │\n")
	} else {
		b.WriteString(fmt.Sprintf("│  %-30s  │\n", m.err))
	}
	b.WriteString("└──────────────────────────────────────────────────────┘\n\n")

	// Projects section
	b.WriteString(styles.Section.Render("Projects"))
	b.WriteString("\n")
	if len(m.projects) == 0 {
		b.WriteString("  (none)\n")
	} else {
		maxShow := 5
		if len(m.projects) < maxShow { maxShow = len(m.projects) }
		for _, p := range m.projects[:maxShow] {
			b.WriteString(fmt.Sprintf("  • %s (%d obs, %d sessions)\n", p.Name, p.Observations, p.Sessions))
		}
		if len(m.projects) > maxShow {
			b.WriteString(fmt.Sprintf("  ...and %d more projects\n", len(m.projects)-maxShow))
		}
	}
	b.WriteString("\n")

	// Actions (like engram's main menu)
	b.WriteString(styles.Section.Render("Actions"))
	b.WriteString("\n\n")
	for i, a := range dashboardActions {
		cur := "  "
		if i == m.cursor { cur = "▸ " }
		b.WriteString(fmt.Sprintf("%s%s  %s\n", cur, styles.MenuItemKey.Render("["+a.key+"]"), a.desc))
	}

	b.WriteString("\n")
	b.WriteString(styles.Help.Render("↑↓ navigate · enter select · q quit · ? help"))
	return styles.AppStyle.Render(b.String())
}
