package screens

import (
	"context"
	"fmt"
	"strings"

	"github.com/biggz-ai/biggz/internal/agents/claude"
	"github.com/biggz-ai/biggz/internal/agents/opencode"
	"github.com/biggz-ai/biggz/internal/agents/qwen"
	"github.com/biggz-ai/biggz/internal/install"
	"github.com/biggz-ai/biggz/internal/tui/styles"
	"github.com/biggz-ai/biggz/plugin"
	tea "github.com/charmbracelet/bubbletea"
)

// step tracks install progress.
type installStep int

const (
	stepInstallIdle    installStep = iota
	stepInstallDetect
	stepInstallRunning
	stepInstallDone
	stepInstallError
)

// InstallModel handles agent detection and installation.
type InstallModel struct {
	step     installStep
	adapters []plugin.AgentAdapter
	cursor   int
	result   *install.Result
	errMsg   string
	agent    string
}

// NewInstallModel creates the install screen.
func NewInstallModel() InstallModel {
	return InstallModel{step: stepInstallIdle}
}

func (m InstallModel) Init() tea.Cmd { return nil }

// installResultMsg carries the result.
type installResultMsg struct {
	result *install.Result
	err    error
}

// doInstall runs the install in a goroutine.
func doInstall(adapter plugin.AgentAdapter) tea.Msg {
	ctx := context.Background()
	r, err := install.Run(ctx, adapter, install.Config{})
	return installResultMsg{result: r, err: err}
}

// Update handles input.
func (m InstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			if m.step == stepInstallIdle {
				m.step = stepInstallDetect
				m.adapters = m.detectAdapters()
				if len(m.adapters) == 0 {
					m.step = stepInstallError
					m.errMsg = "No AI agent detected (tried opencode, claude, qwen)"
					return m, nil
				}
				if len(m.adapters) == 1 {
					m.cursor = 0
					m.step = stepInstallRunning
					m.agent = m.adapters[0].Name()
					return m, func() tea.Msg { return doInstall(m.adapters[0]) }
				}
				return m, nil
			}
			if m.step == stepInstallDetect && len(m.adapters) > 1 {
				adapter := m.adapters[m.cursor]
				m.step = stepInstallRunning
				m.agent = adapter.Name()
				return m, func() tea.Msg { return doInstall(adapter) }
			}
		case "up", "k":
			if m.step == stepInstallDetect && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.step == stepInstallDetect && m.cursor < len(m.adapters)-1 {
				m.cursor++
			}
		case "r":
			if m.step == stepInstallDone || m.step == stepInstallError {
				m.step = stepInstallIdle
				m.errMsg = ""
				m.result = nil
			}
		}

	case installResultMsg:
		if msg.err != nil {
			m.step = stepInstallError
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.result = msg.result
		m.step = stepInstallDone
		return m, nil
	}

	return m, nil
}

// detectAdapters tries to detect available AI coding agents.
func (m InstallModel) detectAdapters() []plugin.AgentAdapter {
	var found []plugin.AgentAdapter
	homeDir := "" // use real home
	for _, factory := range []struct {
		name string
		fn   func() plugin.AgentAdapter
	}{
		{"opencode", func() plugin.AgentAdapter { return opencode.NewAdapter() }},
		{"claude", func() plugin.AgentAdapter { return claude.NewAdapter() }},
		{"qwen", func() plugin.AgentAdapter { return qwen.NewAdapter() }},
	} {
		// Check via the adapter registry
		// Note: agents.Register is called by each adapter's init()
		adapter := factory.fn()
		ctx := context.Background()
		installed, _, _, _, _ := adapter.Detect(ctx, homeDir)
		if installed {
			found = append(found, adapter)
		}
	}
	return found
}

// View renders the install screen.
func (m InstallModel) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Install biggz-ai"))
	b.WriteString("\n\n")

	switch m.step {
	case stepInstallIdle:
		b.WriteString("This will install biggz-ai in your AI coding agent.\n\n")
		b.WriteString(styles.StatusInfo.Render("What gets installed:\n"))
		b.WriteString("  • SDD orchestrator agent + 18 sub-agents\n")
		b.WriteString("  • 24 SDD skills in ~/.biggz/skills/\n")
		b.WriteString("  • 12 SDD slash commands\n")
		b.WriteString("  • BigMem MCP server (persistent memory)\n")
		b.WriteString("  • RDD kill switch\n")
		b.WriteString("  • biggz persona in AGENTS.md\n")
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("Press ENTER to start installation"))

	case stepInstallDetect:
		if len(m.adapters) > 1 {
			b.WriteString("Multiple agents detected. Choose one:\n\n")
			for i, a := range m.adapters {
				cur := "  "
				if i == m.cursor {
					cur = "▸ "
				}
				b.WriteString(fmt.Sprintf("%s%s\n", cur, a.Name()))
			}
			b.WriteString("\n")
			b.WriteString(styles.Help.Render("↑↓ navigate · ENTER select"))
		} else {
			b.WriteString(styles.Spinner.Render("Detecting agents..."))
		}

	case stepInstallRunning:
		b.WriteString(styles.Spinner.Render(fmt.Sprintf("Installing in %s...\n\n", m.agent)))
		b.WriteString(styles.StatusInfo.Render("Deploying skills, configuring agents, setting up MCP..."))

	case stepInstallDone:
		r := m.result
		b.WriteString(styles.SuccessBox.Render(fmt.Sprintf(
			"✅ biggz-ai installed successfully in %s\n\n"+
				"  • Skills deployed:  %d\n"+
				"  • Commands written: %d\n"+
				"  • Config merged:    %v\n",
			r.BinaryPath, r.SkillsDeployed, r.CommandsWritten, r.ConfigMerged)))
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("Press [R] to reinstall · esc for menu"))

	case stepInstallError:
		b.WriteString(styles.ErrorBox.Render(fmt.Sprintf("❌ Install failed: %s", m.errMsg)))
		b.WriteString("\n\n")
		b.WriteString(styles.Help.Render("Press [R] to retry · esc for menu"))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("esc back · ctrl+c quit"))
	return styles.AppStyle.Render(b.String())
}
