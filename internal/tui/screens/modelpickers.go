package screens

import (
	"fmt"
	"strings"

	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type mpView int

const (
	mpList mpView = iota
	mpConfig
)

// ModelPickerScreen lets the user pick and configure AI models per provider.
type ModelPickerScreen struct {
	view       mpView
	cursor     int
	scroll     int
	modelType  string // "primary", "small", "agent"
	config     ModelConfig
	err        string
}

// ModelConfig holds per-provider model assignments.
type ModelConfig struct {
	PrimaryModel string `json:"primary"`
	SmallModel   string `json:"small"`
	AgentModel   string `json:"agent"`
	Provider     string `json:"provider"`
	Temperature  float64 `json:"temperature,omitempty"`
	MaxTokens    int     `json:"maxTokens,omitempty"`
}

var modelProviders = []struct {
	name  string
	models []string
}{
	{"anthropic", []string{"claude-sonnet-4-20250514", "claude-haiku-4-20250514", "claude-opus-4-20250514"}},
	{"openai", []string{"gpt-5", "gpt-5-codex", "o3", "o4-mini"}},
	{"google", []string{"gemini-2.5-pro", "gemini-2.5-flash"}},
	{"opencode", []string{"zen", "gpt-5.1-codex", "claude-sonnet-4-5"}},
	{"local", []string{"llama-4", "mistral-3", "qwen-3"}},
}

var modelTypes = []string{"primary", "small", "agent"}

func NewModelPickerScreen() ModelPickerScreen {
	return ModelPickerScreen{config: ModelConfig{Provider: "anthropic", Temperature: 0.3, MaxTokens: 4096}}
}

func (m ModelPickerScreen) Init() tea.Cmd { return nil }

func (m ModelPickerScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 { m.cursor-- }
		case "down", "j":
			max := len(modelProviders) - 1
			if m.view == mpConfig {
				max = 5 // config fields
			}
			if m.cursor < max { m.cursor++ }
		case "enter":
			if m.view == mpList {
				m.modelType = modelTypes[0]
				m.view = mpConfig
				m.cursor = 0
			}
		case "tab":
			if m.view == mpConfig {
				m.modelType = nextModelType(m.modelType)
			}
		case "esc":
			if m.view == mpConfig {
				m.view = mpList
				m.cursor = 0
				return m, nil
			}
			return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
		}
	}
	return m, nil
}

func (m ModelPickerScreen) View() string {
	var b strings.Builder

	switch m.view {
	case mpConfig:
		b.WriteString(styles.Title.Render(fmt.Sprintf("Model Config — %s", m.config.Provider)))
		b.WriteString("\n\n")
		b.WriteString(styles.StatusInfo.Render(fmt.Sprintf("Editing: %s model  [TAB to switch]", m.modelType)))
		b.WriteString("\n\n")

		fields := []struct {
			label string
			value string
		}{
			{"Provider", m.config.Provider},
			{fmt.Sprintf("%s Model", m.modelType), currentModel(m)},
			{"Temperature", fmt.Sprintf("%.1f", m.config.Temperature)},
			{"Max Tokens", fmt.Sprintf("%d", m.config.MaxTokens)},
		}
		for i, f := range fields {
			cur := "  "
			if i == m.cursor { cur = "▸ " }
			b.WriteString(fmt.Sprintf("%s%s: %s\n", cur, f.label, f.value))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("TAB switch model type · ↑↓ select · ENTER edit · ESC back"))

	default:
		b.WriteString(styles.Title.Render("Model Configuration"))
		b.WriteString("\n\n")
		b.WriteString("Select a provider to configure:\n\n")
		for i, p := range modelProviders {
			cur := "  "
			if i == m.cursor { cur = "▸ " }
			b.WriteString(fmt.Sprintf("%s%s\n", cur, styles.MenuItemKey.Render(p.name)))
			b.WriteString(fmt.Sprintf("    %s\n", strings.Join(p.models, ", ")))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ navigate · ENTER configure · ESC back"))
	}
	return styles.AppStyle.Render(b.String())
}

func nextModelType(t string) string {
	for i, mt := range modelTypes {
		if mt == t && i+1 < len(modelTypes) {
			return modelTypes[i+1]
		}
	}
	return modelTypes[0]
}

func currentModel(m ModelPickerScreen) string {
	return m.config.PrimaryModel
}
