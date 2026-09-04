package screens

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/opencode"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// ClaudePickerBackground identifies the Claude model background in the
// installer wizard (REQ-WIZ-002).
const ClaudePickerBackground = "claude"

// ClaudeDefaultAgents returns the agent-layer routing default for the Claude
// background. It is the highest-precedence layer fed into
// opencode.MergeModelConfigs and only covers the orchestrator key; every
// other agent falls through to user > builtin.
func ClaudeDefaultAgents() opencode.AgentModelConfig {
	return opencode.AgentModelConfig{
		opencode.OrchestratorAgent: {
			Model:    "anthropic/claude-sonnet-4-20250514",
			Thinking: opencode.ThinkingHigh,
		},
	}
}

// ClaudePicker is a thin wrapper over ModelPickerScreen for the Claude
// background. Rendering uses biggz styles tokens only; persistence reuses the
// existing model-routing precedence (agents > user > builtin) via
// opencode.MergeModelConfigs. No gentle-ai catalog/model/planner port.
type ClaudePicker struct {
	Inner ModelPickerScreen
	// Agents is the agent-layer override. Defaults to ClaudeDefaultAgents.
	Agents opencode.AgentModelConfig
}

// NewClaudePicker creates a Claude picker with default cache paths.
func NewClaudePicker() ClaudePicker {
	return ClaudePicker{Inner: NewModelPickerScreen(), Agents: ClaudeDefaultAgents()}
}

// NewClaudePickerWithPaths creates a Claude picker with explicit paths
// (used by tests so the real home directory is never touched).
func NewClaudePickerWithPaths(cachePath, variantsPath, settingsPath string) ClaudePicker {
	return ClaudePicker{
		Inner:  NewModelPickerScreenWithPaths(cachePath, variantsPath, settingsPath),
		Agents: ClaudeDefaultAgents(),
	}
}

// Background reports the model background this picker serves.
func (p ClaudePicker) Background() string { return ClaudePickerBackground }

// ResolveEffective merges agents > user > builtin per the existing
// model-routing precedence.
func (p ClaudePicker) ResolveEffective(user, builtin opencode.AgentModelConfig) opencode.AgentModelConfig {
	return opencode.MergeModelConfigs(p.Agents, user, builtin)
}

// Init implements tea.Model.
func (p ClaudePicker) Init() tea.Cmd { return p.Inner.Init() }

// Update delegates key handling to the wrapped ModelPickerScreen.
func (p ClaudePicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := p.Inner.Update(msg)
	if inner, ok := updated.(ModelPickerScreen); ok {
		p.Inner = inner
	}
	return p, cmd
}

// View renders the Claude picker header plus the wrapped picker view.
// Styles come from internal/tui/styles only.
func (p ClaudePicker) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Claude model background"))
	b.WriteString("\n\n")
	b.WriteString(styles.Section.Render("Pick the Claude model for this agent"))
	b.WriteString("\n\n")
	b.WriteString(p.Inner.View())
	b.WriteString("\n")
	b.WriteString(styles.Help.Render("↑↓ navigate · ENTER select · ESC back"))
	return wizardGuardView(b.String())
}
