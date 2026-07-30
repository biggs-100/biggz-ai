package screens

import (
	"fmt"
	"strings"

	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type syncStep int

const (
	syncIdle     syncStep = iota
	syncRunning
	syncDone
	syncError
)

// SyncModel handles sync + combined upgrade operations.
type SyncModel struct {
	step      syncStep
	status    string
	err       string
	combined  bool // if true, also runs upgrade
}

func NewSyncModel() SyncModel { return SyncModel{step: syncIdle} }

func (m SyncModel) Init() tea.Cmd { return nil }

type syncResultMsg struct {
	status string
	err    error
}

func doSync(combined bool) tea.Msg {
	// In production, this would run biggz sync / biggz install etc.
	status := "Configuration synced successfully."
	if combined {
		status = "Upgrade completed. Configuration synced successfully."
	}
	return syncResultMsg{status: status}
}

func (m SyncModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			if m.step == syncIdle {
				m.step = syncRunning
				return m, func() tea.Msg { return doSync(m.combined) }
			}
			if m.step == syncDone || m.step == syncError {
				return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
			}
		case "c":
			m.combined = !m.combined
		case "esc":
			return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
		}
	case syncResultMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.step = syncError
		} else {
			m.status = msg.status
			m.step = syncDone
		}
	}
	return m, nil
}

func (m SyncModel) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Sync Configuration"))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render(m.err))
		b.WriteString("\n\n")
	}
	if m.status != "" {
		b.WriteString(styles.SuccessBox.Render(m.status))
		b.WriteString("\n\n")
	}

	switch m.step {
	case syncIdle:
		b.WriteString("This will sync your biggz-ai configuration:\n\n")
		b.WriteString("  • Re-deploy skills and commands\n")
		b.WriteString("  • Re-apply config overlays\n")
		b.WriteString("  • Re-inject persona and BigMem protocol\n")
		b.WriteString("  • Update MCP configuration\n\n")
		cb := styles.CheckboxEmpty.String()
		if m.combined {
			cb = styles.CheckboxSelected.String()
		}
		b.WriteString(fmt.Sprintf("%s [C]ombined with upgrade (check for updates first)\n\n", cb))
		b.WriteString(styles.Help.Render("ENTER to sync · [C] toggle combined · ESC back"))

	case syncRunning:
		if m.combined {
			b.WriteString(styles.Spinner.Render("Checking for updates..."))
			b.WriteString("\n\n")
		}
		b.WriteString(styles.Spinner.Render("Syncing configuration..."))

	case syncDone:
		b.WriteString(styles.Help.Render("Press ENTER to return"))

	case syncError:
		b.WriteString(styles.Help.Render("Press ENTER to return"))
	}
	return styles.AppStyle.Render(b.String())
}
