package screens

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/opencode"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// CodexPickerBackground identifies the Codex model background in the
// installer wizard (REQ-WIZ-002).
const CodexPickerBackground = "codex"

// CodexDefaultAgents returns the agent-layer routing default for the Codex
// background. It is the highest-precedence layer fed into
// opencode.MergeModelConfigs and only covers the orchestrator key; every
// other agent falls through to user > builtin.
func CodexDefaultAgents() opencode.AgentModelConfig {
	return opencode.AgentModelConfig{
		opencode.OrchestratorAgent: {
			Model:    "openai/gpt-5",
			Thinking: opencode.ThinkingMedium,
		},
	}
}

// CodexPicker is a thin wrapper over ModelPickerScreen for the Codex
// background. Rendering uses biggz styles tokens only; persistence reuses the
// existing model-routing precedence (agents > user > builtin) via
// opencode.MergeModelConfigs. No gentle-ai catalog/model/planner port.
type CodexPicker struct {
	Inner ModelPickerScreen
	// Agents is the agent-layer override. Defaults to CodexDefaultAgents.
	Agents opencode.AgentModelConfig
}

// NewCodexPicker creates a Codex picker with default cache paths.
func NewCodexPicker() CodexPicker {
	return CodexPicker{Inner: NewModelPickerScreen(), Agents: CodexDefaultAgents()}
}

// NewCodexPickerWithPaths creates a Codex picker with explicit paths
// (used by tests so the real home directory is never touched).
func NewCodexPickerWithPaths(cachePath, variantsPath, settingsPath string) CodexPicker {
	return CodexPicker{
		Inner:  NewModelPickerScreenWithPaths(cachePath, variantsPath, settingsPath),
		Agents: CodexDefaultAgents(),
	}
}

// Background reports the model background this picker serves.
func (p CodexPicker) Background() string { return CodexPickerBackground }

// ResolveEffective merges agents > user > builtin per the existing
// model-routing precedence.
func (p CodexPicker) ResolveEffective(user, builtin opencode.AgentModelConfig) opencode.AgentModelConfig {
	return opencode.MergeModelConfigs(p.Agents, user, builtin)
}

// Init implements tea.Model.
func (p CodexPicker) Init() tea.Cmd { return p.Inner.Init() }

// Update delegates key handling to the wrapped ModelPickerScreen.
func (p CodexPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := p.Inner.Update(msg)
	if inner, ok := updated.(ModelPickerScreen); ok {
		p.Inner = inner
	}
	return p, cmd
}

// View renders the Codex picker header plus the wrapped picker view.
// Styles come from internal/tui/styles only.
func (p CodexPicker) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Codex model background"))
	b.WriteString("\n\n")
	b.WriteString(styles.Section.Render("Pick the Codex model for this agent"))
	b.WriteString("\n\n")
	b.WriteString(p.Inner.View())
	b.WriteString("\n")
	b.WriteString(styles.Help.Render("↑↓ navigate · ENTER select · ESC back"))
	return wizardGuardView(b.String())
}
