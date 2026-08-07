package screens

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
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

// modelPresets defines named model assignment presets.
// Each preset maps phase → model tier.
type modelPreset struct {
	Name        string
	Description string
	Models      map[string]string // phase → model tier hint
}

var modelPresets = []modelPreset{
	{
		Name:        "Balanced",
		Description: "Default — mid-range models for all phases",
		Models: map[string]string{
			"biggz-orchestrator": "anthropic:claude-sonnet-4-20250514",
			"sdd-init":           "anthropic:claude-haiku-4-20250514",
			"sdd-explore":        "anthropic:claude-haiku-4-20250514",
			"sdd-propose":        "anthropic:claude-haiku-4-20250514",
			"sdd-spec":           "anthropic:claude-sonnet-4-20250514",
			"sdd-design":         "anthropic:claude-sonnet-4-20250514",
			"sdd-tasks":          "anthropic:claude-haiku-4-20250514",
			"sdd-apply":          "anthropic:claude-sonnet-4-20250514",
			"sdd-verify":         "anthropic:claude-sonnet-4-20250514",
			"sdd-archive":        "anthropic:claude-haiku-4-20250514",
			"sdd-onboard":        "anthropic:claude-haiku-4-20250514",
		},
	},
	{
		Name:        "Performance",
		Description: "Fastest models — speed over capability",
		Models: map[string]string{
			"biggz-orchestrator": "anthropic:claude-haiku-4-20250514",
			"sdd-init":           "openai:gpt-4o-mini",
			"sdd-explore":        "openai:gpt-4o-mini",
			"sdd-propose":        "openai:gpt-4o-mini",
			"sdd-spec":           "anthropic:claude-haiku-4-20250514",
			"sdd-design":         "anthropic:claude-haiku-4-20250514",
			"sdd-tasks":          "openai:gpt-4o-mini",
			"sdd-apply":          "anthropic:claude-sonnet-4-20250514",
			"sdd-verify":         "anthropic:claude-haiku-4-20250514",
			"sdd-archive":        "openai:gpt-4o-mini",
			"sdd-onboard":        "openai:gpt-4o-mini",
		},
	},
	{
		Name:        "Economy",
		Description: "Cheapest models — budget-friendly",
		Models: map[string]string{
			"biggz-orchestrator": "openai:gpt-4o-mini",
			"sdd-init":           "google:gemini-2.5-flash",
			"sdd-explore":        "google:gemini-2.5-flash",
			"sdd-propose":        "google:gemini-2.5-flash",
			"sdd-spec":           "openai:gpt-4o-mini",
			"sdd-design":         "openai:gpt-4o-mini",
			"sdd-tasks":          "google:gemini-2.5-flash",
			"sdd-apply":          "openai:gpt-4o",
			"sdd-verify":         "openai:gpt-4o-mini",
			"sdd-archive":        "google:gemini-2.5-flash",
			"sdd-onboard":        "google:gemini-2.5-flash",
		},
	},
	{
		Name:        "Quality",
		Description: "Best models — highest quality, highest cost",
		Models: map[string]string{
			"biggz-orchestrator": "anthropic:claude-sonnet-4-20250514",
			"sdd-init":           "anthropic:claude-sonnet-4-20250514",
			"sdd-explore":        "anthropic:claude-sonnet-4-20250514",
			"sdd-propose":        "anthropic:claude-sonnet-4-20250514",
			"sdd-spec":           "anthropic:claude-sonnet-4-20250514",
			"sdd-design":         "anthropic:claude-sonnet-4-20250514",
			"sdd-tasks":          "anthropic:claude-sonnet-4-20250514",
			"sdd-apply":          "anthropic:claude-sonnet-4-20250514",
			"sdd-verify":         "anthropic:claude-sonnet-4-20250514",
			"sdd-archive":        "anthropic:claude-sonnet-4-20250514",
			"sdd-onboard":        "anthropic:claude-sonnet-4-20250514",
		},
	},
	{
		Name:        "Custom",
		Description: "Per-phase manual configuration",
		Models:      map[string]string{},
	},
}

// commonModels is the cycle list for per-phase model selection.
var commonModels = []string{
	"", // default (no override)
	"anthropic:claude-sonnet-4-20250514",
	"anthropic:claude-haiku-4-20250514",
	"openai:gpt-4o",
	"openai:gpt-4o-mini",
	"google:gemini-2.5-flash",
	"google:gemini-2.5-pro",
}

type configTab int

const (
	tabConfigPreset  configTab = iota // new: preset selection first
	tabConfigModels                   // per-phase model selection
	tabConfigPersona
	tabConfigDelivery
	tabConfigDone
)

var configTabLabels = []string{" Preset ", " Models ", " Persona ", " Delivery ", " Save "}

// ConfigModel handles all configuration options with real persistence.
type ConfigModel struct {
	tab      configTab
	cursor   int
	preset   int          // selected preset index
	models   map[string]string // phase -> model
	persona  int
	delivery int
	budget   int
	saved    bool
	status   string
	err      string
}

// NewConfigModel creates the config screen.
func NewConfigModel() ConfigModel {
	m := ConfigModel{
		tab:      tabConfigPreset,
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
	if agents, ok := cfg["agent"].(map[string]any); ok {
		for _, phase := range sddPhases {
			if a, ok := agents[phase].(map[string]any); ok {
				if model, ok := a["model"].(string); ok {
					m.models[phase] = model
				}
			}
		}
	}
	if d, ok := cfg["delivery_strategy"].(string); ok {
		for i, item := range deliveryItems {
			if d == item {
				m.delivery = i
				break
			}
		}
	}
	if b, ok := cfg["review_budget_lines"].(float64); ok {
		m.budget = int(b)
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

	cfg["delivery_strategy"] = deliveryItems[m.delivery]
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
			if m.tab > tabConfigPreset {
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
			case tabConfigPreset:
				// Apply preset models
				m.preset = m.cursor
				if m.cursor < len(modelPresets)-1 {
					// Non-Custom preset: apply all models
					preset := modelPresets[m.cursor]
					for phase, model := range preset.Models {
						m.models[phase] = model
					}
				}
			case tabConfigModels:
				m.cycleModel(sddPhases[m.cursor])
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
				m.status = fmt.Sprintf("Saved — %d model overrides, persona: %s, delivery: %s",
					len(m.models), personaItems[m.persona], deliveryItems[m.delivery])
			}
		}
	}
	return m, nil
}

// cycleModel cycles through common models for a phase.
func (m *ConfigModel) cycleModel(phase string) {
	current := m.models[phase]
	for i, mm := range commonModels {
		if mm == current {
			next := (i + 1) % len(commonModels)
			if next == 0 {
				delete(m.models, phase)
			} else {
				m.models[phase] = commonModels[next]
			}
			return
		}
	}
	m.models[phase] = commonModels[1]
}

func (m ConfigModel) itemsLen() int {
	switch m.tab {
	case tabConfigPreset:
		return len(modelPresets)
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

	for i, label := range configTabLabels {
		if i == int(m.tab) {
			b.WriteString(styles.ButtonActive.Render(label))
		} else {
			b.WriteString(styles.ButtonInactive.Render(label))
		}
	}
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render("Error: " + m.err))
		b.WriteString("\n\n")
		m.err = ""
	}
	if m.saved {
		b.WriteString(styles.SuccessBox.Render(m.status))
		b.WriteString("\n\n")
	}

	switch m.tab {
	case tabConfigPreset:
		b.WriteString(styles.Section.Render("Model Presets"))
		b.WriteString("\n")
		b.WriteString(styles.StatusInfo.Render("Choose a preset or select Custom for per-phase configuration."))
		b.WriteString("\n\n")
		for i, p := range modelPresets {
			cur := "  "
			if i == m.cursor {
				cur = "▸ "
			}
			sel := styles.CheckboxEmpty.String()
			if i == m.preset {
				sel = styles.CheckboxSelected.String()
			}
			b.WriteString(fmt.Sprintf("%s%s %-12s %s\n", cur, sel, p.Name, p.Description))
			if i == m.preset && i < len(modelPresets)-1 {
				// Show what would be applied
				for phase, model := range p.Models {
					b.WriteString(fmt.Sprintf("    %-25s → %s\n", phase, model))
				}
			}
		}

	case tabConfigModels:
		b.WriteString(styles.Section.Render("Per-Phase Model Assignments"))
		b.WriteString("\n")
		b.WriteString(styles.StatusInfo.Render("Select a phase and press ENTER to cycle through models. Custom preset only."))
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
		b.WriteString(styles.StatusInfo.Render("Choose the agent's personality."))
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
		b.WriteString(styles.Section.Render("Save & Apply"))
		b.WriteString("\n")
		b.WriteString("Press ENTER to save all changes:\n\n")
		b.WriteString(fmt.Sprintf("  • Preset:          %s\n", modelPresets[m.preset].Name))
		b.WriteString(fmt.Sprintf("  • Model overrides: %d phases\n", len(m.models)))
		b.WriteString(fmt.Sprintf("  • Persona:         %s\n", personaItems[m.persona]))
		b.WriteString(fmt.Sprintf("  • Delivery:        %s\n", deliveryItems[m.delivery]))
		b.WriteString(fmt.Sprintf("  • Review budget:   %d lines\n", m.budget))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("← → tabs · ↑↓ navigate · ENTER select · esc back"))
	return styles.AppStyle.Render(b.String())
}
