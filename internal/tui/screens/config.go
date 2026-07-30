package screens

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/filemerge"
	"github.com/biggz-ai/biggz/internal/tui/styles"
	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/plugin"
	tea "github.com/charmbracelet/bubbletea"
)

// sddPhases lists all configurable SDD phases.
var sddPhases = []string{
	"biggz-orchestrator", "sdd-init", "sdd-explore", "sdd-propose",
	"sdd-spec", "sdd-design", "sdd-tasks", "sdd-apply",
	"sdd-verify", "sdd-archive", "sdd-onboard",
}

var personaItems = []string{"gentleman", "neutral"}
var deliveryItems = []string{"ask-on-risk", "auto-chain", "single-pr", "exception-ok"}

type configTab int

const (
	tabConfigModels configTab = iota
	tabConfigPersona
	tabConfigDelivery
	tabConfigDone
)

var configTabLabels = []string{" Models ", " Persona ", " Delivery ", " Save "}

// ConfigModel handles all configuration options with real persistence.
type ConfigModel struct {
	tab      configTab
	cursor   int
	models   map[string]string // phase -> model
	persona  int
	delivery int
	budget   int
	saved    bool
	status   string // status message after save
	err      string // error message
}

// NewConfigModel creates the config screen.
func NewConfigModel() ConfigModel {
	m := ConfigModel{
		tab:      tabConfigModels,
		models:   make(map[string]string),
		persona:  0,
		delivery: 0,
		budget:   400,
	}
	m.loadFromConfig()
	return m
}

// loadFromConfig reads current settings from opencode.json.
func (m *ConfigModel) loadFromConfig() {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	// Read model assignments
	if agents, ok := cfg["agent"].(map[string]any); ok {
		for _, phase := range sddPhases {
			if a, ok := agents[phase].(map[string]any); ok {
				if model, ok := a["model"].(string); ok {
					m.models[phase] = model
				}
			}
		}
	}
}

func (m ConfigModel) Init() tea.Cmd { return nil }

// saveConfig writes changes to opencode.json.
func (m *ConfigModel) saveConfig() error {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Write model assignments
	agents, _ := cfg["agent"].(map[string]any)
	if agents == nil {
		agents = make(map[string]any)
		cfg["agent"] = agents
	}
	for phase, model := range m.models {
		a, _ := agents[phase].(map[string]any)
		if a == nil {
			a = make(map[string]any)
			agents[phase] = a
		}
		if model == "" {
			delete(a, "model")
		} else {
			a["model"] = model
		}
	}

	// Write delivery strategy
	cfg["delivery_strategy"] = deliveryItems[m.delivery]

	// Write review budget
	cfg["review_budget_lines"] = m.budget

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if _, err := filemerge.WriteFileAtomic(configPath, out, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// SavePersonaMsg triggers persona injection.
type SavePersonaMsg struct {
	Persona string
}

// Update handles input.
func (m ConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if m.tab > tabConfigModels {
				m.tab--
				m.cursor = 0
			}
		case "right", "l":
			if m.tab < tabConfigDone {
				m.tab++
				m.cursor = 0
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			max := m.itemsLen() - 1
			if m.cursor < max {
				m.cursor++
			}
		case "enter", " ":
			switch m.tab {
			case tabConfigPersona:
				m.persona = m.cursor
			case tabConfigDelivery:
				m.delivery = m.cursor
			case tabConfigDone:
				if err := m.saveConfig(); err != nil {
					m.err = err.Error()
					return m, nil
				}
				m.saved = true
				m.status = fmt.Sprintf("Saved — persona: %s, delivery: %s, budget: %d",
					personaItems[m.persona], deliveryItems[m.delivery], m.budget)
			case tabConfigModels:
				// Toggle model selection (cycle through common models)
				m.cycleModel(sddPhases[m.cursor])
			}
		}
	case SavePersonaMsg:
		// handled
	}
	return m, nil
}

// cycleModel cycles through common models for a phase.
func (m *ConfigModel) cycleModel(phase string) {
	models := []string{
		"", // default
		"anthropic:claude-sonnet-4-20250514",
		"anthropic:claude-haiku-4-20250514",
		"openai:gpt-4o",
		"openai:gpt-4o-mini",
		"google:gemini-2.5-flash",
		"google:gemini-2.5-pro",
	}
	current := m.models[phase]
	for i, mm := range models {
		if mm == current {
			next := (i + 1) % len(models)
			if next == 0 {
				delete(m.models, phase)
			} else {
				m.models[phase] = models[next]
			}
			return
		}
	}
	// Not found, set first non-empty
	m.models[phase] = models[1]
}

func (m ConfigModel) itemsLen() int {
	switch m.tab {
	case tabConfigModels:
		return len(sddPhases)
	case tabConfigPersona:
		return len(personaItems)
	case tabConfigDelivery:
		return len(deliveryItems)
	case tabConfigDone:
		return 1
	}
	return 0
}

// detectAgent creates a minimal adapter for persona injection.
func (m ConfigModel) detectAgent() plugin.AgentAdapter {
	home, _ := os.UserHomeDir()
	_, err := os.Stat(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err == nil {
		return &minimalAgent{}
	}
	return nil
}

// minimalAgent implements plugin.AgentAdapter minimally for persona injection.
type minimalAgent struct{}

func (a *minimalAgent) ID() model.AgentID                         { return "opencode" }
func (a *minimalAgent) Name() string                              { return "OpenCode" }
func (a *minimalAgent) Tier() model.SupportTier                   { return model.TierFull }
func (a *minimalAgent) Capabilities() []string                    { return nil }
func (a *minimalAgent) Detect(ctx context.Context, homeDir string) (bool, string, string, bool, error) { return false, "", "", false, nil }
func (a *minimalAgent) InstallCommand(profile interface{}) ([][]string, error) { return nil, nil }
func (a *minimalAgent) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error { return nil }
func (a *minimalAgent) SupportsAutoInstall() bool                 { return false }
func (a *minimalAgent) SupportsSkills() bool                      { return false }
func (a *minimalAgent) SupportsSystemPrompt() bool                { return true }
func (a *minimalAgent) SupportsMCP() bool                         { return false }
func (a *minimalAgent) SupportsOutputStyles() bool                { return false }
func (a *minimalAgent) SupportsSlashCommands() bool               { return false }
func (a *minimalAgent) SupportsSubAgents() bool                   { return false }
func (a *minimalAgent) SystemPromptStrategy() model.SystemPromptStrategy { return model.StrategyFileReplace }
func (a *minimalAgent) MCPStrategy() model.MCPStrategy            { return model.StrategyMergeIntoSettings }
func (a *minimalAgent) GlobalConfigDir(homeDir string) string     { return filepath.Join(homeDir, ".config", "opencode") }
func (a *minimalAgent) SystemPromptDir(homeDir string) string     { return a.GlobalConfigDir(homeDir) }
func (a *minimalAgent) SystemPromptFile(homeDir string) string    { return filepath.Join(a.GlobalConfigDir(homeDir), "AGENTS.md") }
func (a *minimalAgent) SkillsDir(homeDir string) string           { return filepath.Join(a.GlobalConfigDir(homeDir), "skills") }
func (a *minimalAgent) CommandsDir(homeDir string) string         { return filepath.Join(a.GlobalConfigDir(homeDir), "commands") }
func (a *minimalAgent) SubAgentsDir(homeDir string) string        { return "" }
func (a *minimalAgent) EmbeddedSubAgentsDir() string              { return "" }
func (a *minimalAgent) OutputStyleDir(homeDir string) string      { return "" }
func (a *minimalAgent) SettingsPath(homeDir string) string        { return filepath.Join(a.GlobalConfigDir(homeDir), "opencode.json") }
func (a *minimalAgent) MCPConfigPath(homeDir string, serverName string) string { return a.SettingsPath(homeDir) }

// View renders the config screen.
func (m ConfigModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Configuration"))
	b.WriteString("\n")

	// Tabs
	for i, label := range configTabLabels {
		if i == int(m.tab) {
			b.WriteString(styles.ButtonActive.Render(label))
		} else {
			b.WriteString(styles.ButtonInactive.Render(label))
		}
	}
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render("Error: "+m.err))
		b.WriteString("\n\n")
		m.err = ""
	}
	if m.saved {
		b.WriteString(styles.SuccessBox.Render(m.status))
		b.WriteString("\n\n")
	}

	switch m.tab {
	case tabConfigModels:
		b.WriteString(styles.Section.Render("Model Assignments"))
		b.WriteString("\n")
		b.WriteString(styles.StatusInfo.Render("Select a phase and press ENTER to cycle through models."))
		b.WriteString("\n\n")
		for i, phase := range sddPhases {
			cur := "  "
			if i == m.cursor {
				cur = "▸ "
			}
			model := m.models[phase]
			if model == "" {
				model = "(default)"
			}
			b.WriteString(fmt.Sprintf("%s%-25s %s\n", cur, phase, model))
		}

	case tabConfigPersona:
		b.WriteString(styles.Section.Render("Persona"))
		b.WriteString("\n")
		b.WriteString(styles.StatusInfo.Render("Choose the agent's personality. Gentleman is warm and direct, Neutral is professional."))
		b.WriteString("\n\n")
		for i, p := range personaItems {
			cur := "  "
			if i == m.cursor {
				cur = "▸ "
			}
			sel := styles.CheckboxEmpty.String()
			if i == m.persona {
				sel = styles.CheckboxSelected.String()
			}
			b.WriteString(fmt.Sprintf("%s%s%s\n", cur, sel, p))
		}

	case tabConfigDelivery:
		b.WriteString(styles.Section.Render("Delivery Strategy"))
		b.WriteString("\n")
		b.WriteString(styles.StatusInfo.Render("Controls how large changes are split into PRs."))
		b.WriteString("\n\n")
		for i, d := range deliveryItems {
			cur := "  "
			if i == m.cursor {
				cur = "▸ "
			}
			sel := styles.CheckboxEmpty.String()
			if i == m.delivery {
				sel = styles.CheckboxSelected.String()
			}
			b.WriteString(fmt.Sprintf("%s%s%s\n", cur, sel, d))
		}
		b.WriteString(fmt.Sprintf("\nReview budget: %d lines\n", m.budget))

	case tabConfigDone:
		b.WriteString(styles.Section.Render("Save"))
		b.WriteString("\n")
		b.WriteString("Press ENTER to save all changes:\n\n")
		b.WriteString(fmt.Sprintf("  • Persona:         %s\n", personaItems[m.persona]))
		b.WriteString(fmt.Sprintf("  • Delivery:        %s\n", deliveryItems[m.delivery]))
		b.WriteString(fmt.Sprintf("  • Review budget:   %d lines\n", m.budget))
		b.WriteString(fmt.Sprintf("  • Model overrides: %d phases\n", len(m.models)))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("← → tabs · ↑↓ navigate · ENTER select · esc back"))
	return styles.AppStyle.Render(b.String())
}
