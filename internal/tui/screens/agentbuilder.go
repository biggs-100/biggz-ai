package screens

import (
	"fmt"
	"strings"

	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type abView int

const (
	abEngine abView = iota
	abPrompt
	abSDD
	abPreview
	abGenerating
	abInstalling
	abComplete
)

// AgentBuilderScreen walks through creating a custom agent step by step.
type AgentBuilderScreen struct {
	view       abView
	cursor     int
	scroll     int
	agentName  string
	agentDesc  string
	model      string
	sddEnabled bool
	prompt     string
	err        string
}

func NewAgentBuilderScreen() AgentBuilderScreen {
	return AgentBuilderScreen{view: abEngine, sddEnabled: true}
}

func (m AgentBuilderScreen) Init() tea.Cmd { return nil }

func (m AgentBuilderScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 { m.cursor-- }
		case "down", "j":
			max := map[abView]int{
				abEngine: 2, abPrompt: 1, abSDD: 1, abPreview: 3,
				abGenerating: 0, abInstalling: 0, abComplete: 1,
			}[m.view]
			if m.cursor < max { m.cursor++ }
		case "enter":
			switch m.view {
			case abEngine:
				m.view = abPrompt; m.cursor = 0
			case abPrompt:
				m.view = abSDD; m.cursor = 0
			case abSDD:
				m.view = abPreview; m.cursor = 0
			case abPreview:
				m.view = abGenerating
			case abGenerating:
				m.view = abInstalling
			case abInstalling:
				m.view = abComplete
			case abComplete:
				return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
			}
		case "esc":
			if m.view > abEngine {
				m.view--
				m.cursor = 0
				return m, nil
			}
			return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
		}
	}
	return m, nil
}

func (m AgentBuilderScreen) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Agent Builder"))
	b.WriteString("\n\n")

	switch m.view {
	case abEngine:
		b.WriteString(styles.Section.Render("Step 1: Choose Engine"))
		b.WriteString("\n\n")
		engines := []string{"OpenCode", "Claude Code", "Qwen"}
		for i, e := range engines {
			cur := "  "
			if i == m.cursor { cur = "▸ " }
			b.WriteString(fmt.Sprintf("%s%s\n", cur, e))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("ENTER next · ESC back"))

	case abPrompt:
		b.WriteString(styles.Section.Render("Step 2: System Prompt"))
		b.WriteString("\n\n")
		b.WriteString("  Agent name: my-agent\n")
		b.WriteString("  Description: Custom agent for...\n")
		b.WriteString(fmt.Sprintf("\n  %s\n", styles.StatusInfo.Render("Prompt template loaded from prompts/default.md")))
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("ENTER next · ESC back"))

	case abSDD:
		b.WriteString(styles.Section.Render("Step 3: SDD Integration"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Enable SDD phases: %s\n", styles.StatusEnabled.Render("yes")))
		b.WriteString(fmt.Sprintf("  Enable Review:     %s\n", styles.StatusEnabled.Render("yes")))
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("ENTER next · ESC back"))

	case abPreview:
		b.WriteString(styles.Section.Render("Step 4: Preview"))
		b.WriteString("\n\n")
		b.WriteString("  Agent:    my-agent\n")
		b.WriteString("  Engine:   OpenCode\n")
		b.WriteString("  SDD:      enabled\n")
		b.WriteString("  Review:   enabled\n")
		b.WriteString("  Prompt:   prompts/my-agent.md\n")
		b.WriteString("\n  Press ENTER to generate.\n")
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("ENTER generate · ESC back"))

	case abGenerating:
		b.WriteString(styles.Section.Render("Generating..."))
		b.WriteString("\n\n")
		b.WriteString("  ⏳ Creating agent configuration...\n")
		b.WriteString("  ⏳ Writing prompt file...\n")

	case abInstalling:
		b.WriteString(styles.Section.Render("Installing..."))
		b.WriteString("\n\n")
		b.WriteString("  ✅ Agent config written\n")
		b.WriteString("  ⏳ Deploying to ~/.config/opencode/agents/...\n")

	case abComplete:
		b.WriteString(styles.Section.Render("Agent Created!"))
		b.WriteString("\n\n")
		b.WriteString(styles.StatusEnabled.Render("✓ Agent 'my-agent' installed successfully"))
		b.WriteString("\n\n")
		b.WriteString("  Location: ~/.config/opencode/agents/my-agent.md\n")
		b.WriteString("  Use @my-agent in OpenCode to invoke it.\n")
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("ENTER return to menu"))
	}
	return styles.AppStyle.Render(b.String())
}
