package screens

import (
	"context"
	"fmt"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/agents/claude"
	"github.com/biggs-100/biggz-ai/internal/agents/cursor"
	"github.com/biggs-100/biggz-ai/internal/agents/opencode"
	"github.com/biggs-100/biggz-ai/internal/agents/qwen"
	"github.com/biggs-100/biggz-ai/internal/agents/windsurf"
	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/biggs-100/biggz-ai/plugin"
	tea "github.com/charmbracelet/bubbletea"
)

// installStep tracks the guided installation flow.
type installStep int

const (
	stepInstallIdle      installStep = iota
	stepInstallDetect               // scanning for agents
	stepInstallSelect               // pick from multiple agents
	stepInstallReview               // review what will be installed
	stepInstallRunning              // installation in progress
	stepInstallDone                 // success
	stepInstallError                // failure
)

// InstallModel handles the guided installation wizard.
type InstallModel struct {
	step     installStep
	adapters []plugin.AgentAdapter
	cursor   int
	selected int            // which adapter is selected (-1 = not yet)
	result   *install.Result
	errMsg   string
	agent    string
}

// NewInstallModel creates the install screen.
func NewInstallModel() InstallModel {
	return InstallModel{step: stepInstallIdle, selected: -1}
}

func (m InstallModel) Init() tea.Cmd { return nil }

// installResultMsg carries the async result.
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
			switch m.step {
			case stepInstallIdle:
				m.step = stepInstallDetect
				m.adapters = m.detectAdapters()
				m.cursor = 0
				if len(m.adapters) == 0 {
					m.step = stepInstallError
					m.errMsg = "No AI agent detected (tried opencode, claude, qwen, cursor, windsurf)"
					return m, nil
				}
				if len(m.adapters) == 1 {
					m.selected = 0
					m.step = stepInstallReview
					m.agent = m.adapters[0].Name()
					return m, nil
				}
				m.step = stepInstallSelect
				return m, nil

			case stepInstallSelect:
				m.selected = m.cursor
				m.step = stepInstallReview
				m.agent = m.adapters[m.cursor].Name()
				return m, nil

			case stepInstallReview:
				if m.selected >= 0 && m.selected < len(m.adapters) {
					m.step = stepInstallRunning
					return m, func() tea.Msg { return doInstall(m.adapters[m.selected]) }
				}
			}
		case "up", "k":
			if m.step == stepInstallSelect && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.step == stepInstallSelect && m.cursor < len(m.adapters)-1 {
				m.cursor++
			}
		case "r":
			if m.step == stepInstallDone || m.step == stepInstallError {
				m.step = stepInstallIdle
				m.errMsg = ""
				m.result = nil
				m.selected = -1
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
	homeDir := ""
	for _, factory := range []struct {
		name string
		fn   func() plugin.AgentAdapter
	}{
		{"opencode", func() plugin.AgentAdapter { return opencode.NewAdapter() }},
		{"claude", func() plugin.AgentAdapter { return claude.NewAdapter() }},
		{"qwen", func() plugin.AgentAdapter { return qwen.NewAdapter() }},
		{"cursor", func() plugin.AgentAdapter { return cursor.NewAdapter() }},
		{"windsurf", func() plugin.AgentAdapter { return windsurf.NewAdapter() }},
	} {
		adapter := factory.fn()
		installed, _, _, _, _ := adapter.Detect(context.Background(), homeDir)
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
		b.WriteString("Welcome to biggz-ai installation.\n\n")
		b.WriteString("This wizard will:\n")
		b.WriteString("  1. Detect your AI coding agent\n")
		b.WriteString("  2. Review what will be installed\n")
		b.WriteString("  3. Deploy SDD orchestrator, skills, and BigMem\n\n")
		b.WriteString(styles.StatusInfo.Render("Components to install:\n"))
		b.WriteString("  • SDD orchestrator agent + 18 sub-agents\n")
		b.WriteString("  • 24 SDD skills in ~/.biggz/skills/\n")
		b.WriteString("  • 12 SDD slash commands\n")
		b.WriteString("  • BigMem MCP server (persistent memory)\n")
		b.WriteString("  • RDD kill switch (Review-Driven Development)\n")
		b.WriteString("  • biggz persona in AGENTS.md\n")
		b.WriteString("  • BigMem protocol in AGENTS.md\n")
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("Press ENTER to start"))

	case stepInstallDetect:
		b.WriteString(styles.Spinner.Render("Scanning for AI coding agents..."))
		b.WriteString("\n\n")
		b.WriteString(styles.StatusInfo.Render("Checking: opencode, claude, qwen, cursor, windsurf"))

	case stepInstallSelect:
		b.WriteString("Multiple AI coding agents detected.\n")
		b.WriteString("Choose which one to install biggz-ai into:\n\n")
		for i, a := range m.adapters {
			cur := "  "
			if i == m.cursor {
				cur = "▸ "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", cur, a.Name()))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ navigate · ENTER select"))

	case stepInstallReview:
		b.WriteString(styles.Section.Render("Installation Summary"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Agent: %s\n\n", m.agent))
		b.WriteString("The following will be installed:\n")
		b.WriteString("  ✅ SDD orchestrator + 18 sub-agents\n")
		b.WriteString("  ✅ 24 SDD skills\n")
		b.WriteString("  ✅ 12 slash commands\n")
		b.WriteString("  ✅ BigMem MCP server\n")
		b.WriteString("  ✅ RDD kill switch\n")
		b.WriteString("  ✅ Persona + BigMem protocol in AGENTS.md\n")
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("ENTER to confirm · esc back"))

	case stepInstallRunning:
		b.WriteString(styles.Spinner.Render(fmt.Sprintf("Installing biggz-ai in %s...\n\n", m.agent)))
		b.WriteString(styles.StatusInfo.Render("Deploying skills, configuring agents, setting up MCP...\n"))
		b.WriteString(styles.StatusInfo.Render("Installing BigMem protocol in AGENTS.md..."))

	case stepInstallDone:
		r := m.result
		b.WriteString(styles.SuccessBox.Render(fmt.Sprintf(
			"✅ biggz-ai installed successfully in %s",
			m.agent)))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Skills deployed:  %d\n", r.SkillsDeployed))
		b.WriteString(fmt.Sprintf("  Commands written: %d\n", r.CommandsWritten))
		b.WriteString(fmt.Sprintf("  Config merged:    %v\n", r.ConfigMerged))
		b.WriteString("\n")
		b.WriteString("Next steps:\n")
		b.WriteString("  • Run /sdd-new in OpenCode to start a change\n")
		b.WriteString("  • Run 'biggz sdd-status' to check SDD status\n")
		b.WriteString("  • Run 'biggz tdd enable' to enable Strict TDD\n")
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
