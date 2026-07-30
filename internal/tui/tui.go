// Package tui provides a terminal UI for biggz-ai configuration and management.
package tui

import (
	"fmt"
	"os"

	"github.com/biggz-ai/biggz/internal/tui/screens"
	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// Screen IDs
const (
	screenWelcome = iota
	screenInstall
	screenConfig
	screenStatus
	screenMemory
	screenBackup
	screenProfile
	screenUpgrade
	screenUninstall
	screenStrictTDD
	screenReview
	screenSessions
	screenCount
)

// Model is the top-level TUI model.
type Model struct {
	currentScreen int
	showHelp      bool
	welcome       screens.WelcomeModel
	install       screens.InstallModel
	config        screens.ConfigModel
	status        screens.StatusModel
	memory        screens.MemoryModel
	backup        screens.BackupModel
	profile       screens.ProfileModel
	upgrade       screens.UpgradeModel
	uninstall     screens.UninstallModel
	strictTDD     screens.StrictTDDModel
	reviewScr     screens.ReviewModel
	sessions      screens.SessionsModel
	width, height int
	err           error
}

// New creates the initial TUI model.
func New() Model {
	return Model{
		currentScreen: screenWelcome,
		welcome:       screens.NewWelcomeModel(),
		install:       screens.NewInstallModel(),
		config:        screens.NewConfigModel(),
		status:        screens.NewStatusModel(),
		memory:        screens.NewMemoryModel(),
		backup:        screens.NewBackupModel(),
		profile:       screens.NewProfileModel(),
		upgrade:       screens.NewUpgradeModel(),
		uninstall:     screens.NewUninstallModel(),
		strictTDD:     screens.NewStrictTDDModel(),
		reviewScr:     screens.NewReviewModel(),
		sessions:      screens.NewSessionsModel(),
	}
}

// Init initializes the TUI.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Help toggle — works on any screen
		if msg.String() == "?" {
			m.showHelp = !m.showHelp
			return m, nil
		}

		// When help is shown, only ESC or ? close it
		if m.showHelp {
			if msg.String() == "?" || msg.String() == "esc" {
				m.showHelp = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.currentScreen == screenWelcome {
				return m, tea.Quit
			}
			m.currentScreen = screenWelcome
			m.showHelp = false
			return m, nil
		}

	case screens.NavigateMsg:
		m.currentScreen = int(msg.Screen)
		m.showHelp = false
		return m, nil

	case screens.QuitMsg:
		return m, tea.Quit

	case error:
		m.err = msg
		return m, nil
	}

	// Route message to current screen
	switch m.currentScreen {
	case screenWelcome:
		u, cmd := m.welcome.Update(msg)
		m.welcome = u.(screens.WelcomeModel)
		return m, cmd
	case screenInstall:
		u, cmd := m.install.Update(msg)
		m.install = u.(screens.InstallModel)
		return m, cmd
	case screenConfig:
		u, cmd := m.config.Update(msg)
		m.config = u.(screens.ConfigModel)
		return m, cmd
	case screenStatus:
		u, cmd := m.status.Update(msg)
		m.status = u.(screens.StatusModel)
		return m, cmd
	case screenMemory:
		u, cmd := m.memory.Update(msg)
		m.memory = u.(screens.MemoryModel)
		return m, cmd
	case screenBackup:
		u, cmd := m.backup.Update(msg)
		m.backup = u.(screens.BackupModel)
		return m, cmd
	case screenProfile:
		u, cmd := m.profile.Update(msg)
		m.profile = u.(screens.ProfileModel)
		return m, cmd
	case screenUpgrade:
		u, cmd := m.upgrade.Update(msg)
		m.upgrade = u.(screens.UpgradeModel)
		return m, cmd
	case screenUninstall:
		u, cmd := m.uninstall.Update(msg)
		m.uninstall = u.(screens.UninstallModel)
		return m, cmd
	case screenStrictTDD:
		u, cmd := m.strictTDD.Update(msg)
		m.strictTDD = u.(screens.StrictTDDModel)
		return m, cmd
	case screenReview:
		u, cmd := m.reviewScr.Update(msg)
		m.reviewScr = u.(screens.ReviewModel)
		return m, cmd
	case screenSessions:
		u, cmd := m.sessions.Update(msg)
		m.sessions = u.(screens.SessionsModel)
		return m, cmd
	}

	return m, nil
}

// View renders the current screen or help overlay.
func (m Model) View() string {
	if m.err != nil {
		return styles.ErrorBox.Render(fmt.Sprintf("Error: %v\n\nPress esc to quit.", m.err))
	}

	if m.showHelp {
		return screens.HelpOverlay(m.currentScreen)
	}

	switch m.currentScreen {
	case screenWelcome:
		return m.welcome.View()
	case screenInstall:
		return m.install.View()
	case screenConfig:
		return m.config.View()
	case screenStatus:
		return m.status.View()
	case screenMemory:
		return m.memory.View()
	case screenBackup:
		return m.backup.View()
	case screenProfile:
		return m.profile.View()
	case screenUpgrade:
		return m.upgrade.View()
	case screenUninstall:
		return m.uninstall.View()
	case screenStrictTDD:
		return m.strictTDD.View()
	case screenReview:
		return m.reviewScr.View()
	case screenSessions:
		return m.sessions.View()
	}
	return ""
}

// Run starts the TUI.
func Run() {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
