package screens

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/opencode"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// OpenCodePickerBackground identifies the OpenCode+Pi model background in the
// installer wizard (REQ-WIZ-002).
const OpenCodePickerBackground = "opencode"

// OpenCodeDefaultAgents returns the agent-layer routing default for the
// OpenCode+Pi background. It is intentionally empty so resolution defers to
// user > builtin; callers set Agents when the Agents stage selects an
// explicit override.
func OpenCodeDefaultAgents() opencode.AgentModelConfig {
	return opencode.AgentModelConfig{}
}

// OpenCodePicker is a thin wrapper over ModelPickerScreen for the OpenCode+Pi
// background. Rendering uses biggz styles tokens only; persistence reuses the
// existing model-routing precedence (agents > user > builtin) via
// opencode.MergeModelConfigs. No gentle-ai catalog/model/planner port.
type OpenCodePicker struct {
	Inner ModelPickerScreen
	// Agents is the agent-layer override. Defaults to OpenCodeDefaultAgents.
	Agents opencode.AgentModelConfig
}

// NewOpenCodePicker creates an OpenCode+Pi picker with default cache paths.
func NewOpenCodePicker() OpenCodePicker {
	return OpenCodePicker{Inner: NewModelPickerScreen(), Agents: OpenCodeDefaultAgents()}
}

// NewOpenCodePickerWithPaths creates an OpenCode+Pi picker with explicit paths
// (used by tests so the real home directory is never touched).
func NewOpenCodePickerWithPaths(cachePath, variantsPath, settingsPath string) OpenCodePicker {
	return OpenCodePicker{
		Inner:  NewModelPickerScreenWithPaths(cachePath, variantsPath, settingsPath),
		Agents: OpenCodeDefaultAgents(),
	}
}

// Background reports the model background this picker serves.
func (p OpenCodePicker) Background() string { return OpenCodePickerBackground }

// ResolveEffective merges agents > user > builtin per the existing
// model-routing precedence.
func (p OpenCodePicker) ResolveEffective(user, builtin opencode.AgentModelConfig) opencode.AgentModelConfig {
	return opencode.MergeModelConfigs(p.Agents, user, builtin)
}

// Init implements tea.Model.
func (p OpenCodePicker) Init() tea.Cmd { return p.Inner.Init() }

// Update delegates key handling to the wrapped ModelPickerScreen.
func (p OpenCodePicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := p.Inner.Update(msg)
	if inner, ok := updated.(ModelPickerScreen); ok {
		p.Inner = inner
	}
	return p, cmd
}

// View renders the OpenCode+Pi picker header plus the wrapped picker view.
// Styles come from internal/tui/styles only.
func (p OpenCodePicker) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("OpenCode+Pi model background"))
	b.WriteString("\n\n")
	b.WriteString(styles.Section.Render("Pick the OpenCode model for this agent"))
	b.WriteString("\n\n")
	b.WriteString(p.Inner.View())
	b.WriteString("\n")
	b.WriteString(styles.Help.Render("↑↓ navigate · ENTER select · ESC back"))
	return wizardGuardView(b.String())
}
