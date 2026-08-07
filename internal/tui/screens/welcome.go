package screens

import (
	"fmt"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

var welcomeItems = []MenuItem{
	{Key: "i", Label: "[I]nstall", Description: "Install or update biggz-ai in your AI agent", Screen: 1},
	{Key: "c", Label: "[C]onfigure", Description: "Model assignments, persona, delivery strategy", Screen: 2},
	{Key: "l", Label: "Mode[l] picker", Description: "Select AI models by provider", Screen: 15},
	{Key: "a", Label: "[A]gent builder", Description: "Create custom agents step by step", Screen: 16},
	{Key: "o", Label: "C[o]mmunity", Description: "Plugins, tools, and skills", Screen: 17},
	{Key: "s", Label: "[S]tatus", Description: "RDD status, SDD changes, system health", Screen: 3},
	{Key: "m", Label: "[M]emory", Description: "Browse BigMem persistent memory", Screen: 4},
	{Key: "b", Label: "[B]ackup", Description: "Create and restore snapshots", Screen: 5},
	{Key: "p", Label: "[P]rofiles", Description: "Save and load named configurations", Screen: 6},
	{Key: "u", Label: "[U]pdate", Description: "Check for and install updates", Screen: 7},
	{Key: "t", Label: "Stric[t] TDD", Description: "Toggle strict TDD mode", Screen: 9},
	{Key: "v", Label: "Re[v]iew", Description: "Review lineage timeline", Screen: 10},
	{Key: "r", Label: "[R]ecovery", Description: "Browse recovery trace ledgers", Screen: 14},
	{Key: "e", Label: "S[e]ssions", Description: "Browse BigMem sessions", Screen: 11},
	{Key: "x", Label: "Uninsta[x]", Description: "Remove biggz-ai from your system", Screen: 8},
	{Key: "d", Label: "[D]ashboard", Description: "Return to dashboard", Screen: 0},
	{Key: "q", Label: "[Q]uit", Description: "Exit biggz-ai", Screen: -1},
}

// WelcomeModel is the welcome/main menu screen.
type WelcomeModel struct {
	nav     NavHelper
	version string
}

// NewWelcomeModel creates the welcome screen.
func NewWelcomeModel() WelcomeModel {
	return WelcomeModel{
		nav:     NewNavHelper(welcomeItems),
		version: "dev", // would be set at build time
	}
}

func (m WelcomeModel) Init() tea.Cmd { return nil }

// Update handles input.
func (m WelcomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "i":
			return m, func() tea.Msg { return NavigateMsg{Screen: 1} }
		case "c":
			return m, func() tea.Msg { return NavigateMsg{Screen: 2} }
		case "l":
			return m, func() tea.Msg { return NavigateMsg{Screen: 15} }
		case "a":
			return m, func() tea.Msg { return NavigateMsg{Screen: 16} }
		case "o":
			return m, func() tea.Msg { return NavigateMsg{Screen: 17} }
		case "s":
			return m, func() tea.Msg { return NavigateMsg{Screen: 3} }
		case "m":
			return m, func() tea.Msg { return NavigateMsg{Screen: 4} }
		case "b":
			return m, func() tea.Msg { return NavigateMsg{Screen: 5} }
		case "p":
			return m, func() tea.Msg { return NavigateMsg{Screen: 6} }
		case "u":
			return m, func() tea.Msg { return NavigateMsg{Screen: 7} }
		case "t":
			return m, func() tea.Msg { return NavigateMsg{Screen: 9} }
		case "v":
			return m, func() tea.Msg { return NavigateMsg{Screen: 10} }
		case "r":
			return m, func() tea.Msg { return NavigateMsg{Screen: 14} }
		case "e":
			return m, func() tea.Msg { return NavigateMsg{Screen: 11} }
		case "x":
			return m, func() tea.Msg { return NavigateMsg{Screen: 8} }
		case "d":
			return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
		case "q":
			return m, func() tea.Msg { return QuitMsg{} }
		}

		screen, activated, quit := m.nav.Update(msg)
		if quit {
			return m, func() tea.Msg { return QuitMsg{} }
		}
		if activated {
			return m, func() tea.Msg { return NavigateMsg{Screen: screen} }
		}
	}
	return m, nil
}

// View renders the welcome screen.
func (m WelcomeModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("biggz-ai"))
	b.WriteString(fmt.Sprintf("  %s\n\n", styles.StatusInfo.Render("v"+m.version)))
	b.WriteString(styles.Section.Render("Main Menu"))
	b.WriteString("\n\n")

	for i, item := range welcomeItems {
		cursor := "  "
		if i == m.nav.Cursor {
			cursor = "▸ "
		}
		line := fmt.Sprintf("%s%s  %s", cursor, styles.MenuItemKey.Render(item.Key), item.Description)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.Help.Render("↑↓ navigate · enter select · ? help · esc quit · ctrl+c force"))

	return styles.AppStyle.Render(b.String())
}
