package screens

import (
	"fmt"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// UpdatePromptModel shows a pre-welcome update notification.
type UpdatePromptModel struct {
	version    string
	visible    bool
	dismissed  bool
}

func NewUpdatePromptModel() UpdatePromptModel {
	return UpdatePromptModel{visible: true}
}

func (m UpdatePromptModel) Init() tea.Cmd { return nil }

type updateCheckResultMsg struct {
	version string
	err     error
}

func (m UpdatePromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ", "c":
			m.dismissed = true
			m.visible = false
		case "esc":
			m.dismissed = true
			m.visible = false
		}
	}
	return m, nil
}

func (m UpdatePromptModel) View() string {
	if !m.visible {
		return ""
	}
	var b strings.Builder
	b.WriteString(styles.Title.Render("Update Available"))
	b.WriteString("\n\n")
	b.WriteString(styles.WarningBox.Render(fmt.Sprintf(
		"A new version of biggz-ai is available: %s\n\n"+
			"Run 'biggz update' to upgrade, or visit\n"+
			"github.com/biggs-100/biggz-ai/releases", m.version)))
	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("ENTER to dismiss · ESC continue"))
	return styles.AppStyle.Render(b.String())
}
