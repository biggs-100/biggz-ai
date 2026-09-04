package screens

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/opencode"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// KiroPickerBackground identifies the Kiro model background in the
// installer wizard (REQ-WIZ-002).
const KiroPickerBackground = "kiro"

// KiroDefaultAgents returns the agent-layer routing default for the Kiro
// background. The model cache has no dedicated Kiro provider, so the default
// is a lightweight Anthropic model on the orchestrator key only; every other
// agent falls through to user > builtin via opencode.MergeModelConfigs.
func KiroDefaultAgents() opencode.AgentModelConfig {
	return opencode.AgentModelConfig{
		opencode.OrchestratorAgent: {
			Model:    "anthropic/claude-haiku-4-20250514",
			Thinking: opencode.ThinkingLow,
		},
	}
}

// KiroPicker is a thin wrapper over ModelPickerScreen for the Kiro
// background. Rendering uses biggz styles tokens only; persistence reuses the
// existing model-routing precedence (agents > user > builtin) via
// opencode.MergeModelConfigs. No gentle-ai catalog/model/planner port.
type KiroPicker struct {
	Inner ModelPickerScreen
	// Agents is the agent-layer override. Defaults to KiroDefaultAgents.
	Agents opencode.AgentModelConfig
}

// NewKiroPicker creates a Kiro picker with default cache paths.
func NewKiroPicker() KiroPicker {
	return KiroPicker{Inner: NewModelPickerScreen(), Agents: KiroDefaultAgents()}
}

// NewKiroPickerWithPaths creates a Kiro picker with explicit paths
// (used by tests so the real home directory is never touched).
func NewKiroPickerWithPaths(cachePath, variantsPath, settingsPath string) KiroPicker {
	return KiroPicker{
		Inner:  NewModelPickerScreenWithPaths(cachePath, variantsPath, settingsPath),
		Agents: KiroDefaultAgents(),
	}
}

// Background reports the model background this picker serves.
func (p KiroPicker) Background() string { return KiroPickerBackground }

// ResolveEffective merges agents > user > builtin per the existing
// model-routing precedence.
func (p KiroPicker) ResolveEffective(user, builtin opencode.AgentModelConfig) opencode.AgentModelConfig {
	return opencode.MergeModelConfigs(p.Agents, user, builtin)
}

// Init implements tea.Model.
func (p KiroPicker) Init() tea.Cmd { return p.Inner.Init() }

// Update delegates key handling to the wrapped ModelPickerScreen.
func (p KiroPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := p.Inner.Update(msg)
	if inner, ok := updated.(ModelPickerScreen); ok {
		p.Inner = inner
	}
	return p, cmd
}

// View renders the Kiro picker header plus the wrapped picker view.
// Styles come from internal/tui/styles only.
func (p KiroPicker) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Kiro model background"))
	b.WriteString("\n\n")
	b.WriteString(styles.Section.Render("Pick the Kiro model for this agent"))
	b.WriteString("\n\n")
	b.WriteString(p.Inner.View())
	b.WriteString("\n")
	b.WriteString(styles.Help.Render("↑↓ navigate · ENTER select · ESC back"))
	return wizardGuardView(b.String())
}
