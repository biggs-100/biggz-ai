package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/opencode"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiModelItem string

func (i tuiModelItem) Title() string       { return string(i) }
func (i tuiModelItem) Description() string { return "" }
func (i tuiModelItem) FilterValue() string { return string(i) }

var thinkingOptions = []opencode.ThinkingLevel{
	opencode.ThinkingOff,
	opencode.ThinkingLow,
	opencode.ThinkingMedium,
	opencode.ThinkingHigh,
	opencode.ThinkingInherit,
}

// ModelRoutingModel is a Bubbles modal for per-agent model+thinking routing.
// Precedence is agents > user > builtin. Thinking inherit resolves to global.
type ModelRoutingModel struct {
	agents         []string
	config         opencode.AgentModelConfig
	userConfig     opencode.AgentModelConfig
	builtinConfig  opencode.AgentModelConfig
	cursor         int
	thinkingCursor int
	pickerCursor   int
	view           string // "agents" | "thinking" | "picker"
	selectedAgent  string
	pickerFiles    []string
	globalThinking opencode.ThinkingLevel
	status         string
	err            string
	width, height  int
}

func NewModelRoutingModel() ModelRoutingModel {
	agents := opencode.PickerAgentFiles()
	builtin := make(opencode.AgentModelConfig)
	for _, a := range agents {
		builtin[a] = opencode.AgentRoutingEntry{Thinking: opencode.ThinkingInherit}
	}
	userCfg, _ := opencode.ReadModelConfig(opencode.DefaultModelConfigPath())
	if userCfg == nil {
		userCfg = make(opencode.AgentModelConfig)
	}
	cfg := opencode.MergeModelConfigs(userCfg, builtin)
	picker := opencode.PickerAgentFiles()
	return ModelRoutingModel{
		agents:         agents,
		config:         cfg,
		userConfig:     userCfg,
		builtinConfig:  builtin,
		pickerFiles:    picker,
		globalThinking: opencode.ThinkingHigh,
		view:           "agents",
	}
}

func (m ModelRoutingModel) Init() tea.Cmd { return nil }

func (m ModelRoutingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.view == "agents" {
				m.moveCursor(-1)
			} else if m.view == "thinking" {
				if m.thinkingCursor > 0 {
					m.thinkingCursor--
				}
			} else if m.view == "picker" {
				if m.pickerCursor > 0 {
					m.pickerCursor--
				}
			}
		case "down", "j":
			if m.view == "agents" {
				m.moveCursor(1)
			} else if m.view == "thinking" {
				if m.thinkingCursor < len(thinkingOptions)-1 {
					m.thinkingCursor++
				}
			} else if m.view == "picker" {
				if m.pickerCursor < len(m.pickerFiles)-1 {
					m.pickerCursor++
				}
			}
		case "enter":
			return m.handleEnter()
		case "esc":
			if m.view == "thinking" || m.view == "picker" {
				m.view = "agents"
				return m, nil
			}
			return m, tea.Quit
		case "t", "e":
			if m.view == "agents" && len(m.agents) > 0 {
				m.selectedAgent = m.agents[m.cursor]
				m.view = "thinking"
				m.thinkingCursor = 0
			}
		}
	}
	return m, nil
}

func (m *ModelRoutingModel) moveCursor(delta int) {
	n := len(m.agents)
	if n == 0 {
		return
	}
	next := m.cursor + delta
	if next < 0 || next >= n {
		return
	}
	m.cursor = next
}

func (m ModelRoutingModel) handleEnter() (tea.Model, tea.Cmd) {
	if m.view == "agents" {
		if len(m.agents) == 0 {
			return m, nil
		}
		m.selectedAgent = m.agents[m.cursor]
		m.view = "picker"
		m.pickerCursor = 0
		return m, nil
	}
	if m.view == "thinking" {
		if m.selectedAgent == "" {
			m.view = "agents"
			return m, nil
		}
		lvl := thinkingOptions[m.thinkingCursor]
		opencode.SetThinking(m.config, m.selectedAgent, lvl)
		opencode.SetThinking(m.userConfig, m.selectedAgent, lvl)
		_ = opencode.WriteModelConfig(opencode.DefaultModelConfigPath(), m.userConfig)
		m.status = fmt.Sprintf("Saved %s → %s", m.selectedAgent, lvl)
		m.view = "agents"
		return m, nil
	}
	if m.view == "picker" {
		if m.pickerCursor >= 0 && m.pickerCursor < len(m.pickerFiles) {
			m.selectedAgent = m.pickerFiles[m.pickerCursor]
			m.view = "thinking"
			m.thinkingCursor = 0
		}
		return m, nil
	}
	return m, nil
}

// ResolveEffective returns the effective routing entry for agent respecting agents>user>builtin.
func (m ModelRoutingModel) ResolveEffective(agent string) opencode.AgentRoutingEntry {
	merged := opencode.MergeModelConfigs(m.config, m.userConfig, m.builtinConfig)
	if e, ok := merged[agent]; ok {
		e.Thinking = opencode.EffectiveThinking(e.Thinking, m.globalThinking)
		return e
	}
	return opencode.AgentRoutingEntry{Thinking: opencode.EffectiveThinking(opencode.ThinkingInherit, m.globalThinking)}
}

// PickerFiles returns the 30-file picker list.
func (m ModelRoutingModel) PickerFiles() []string { return m.pickerFiles }

// SetModel sets model for agent and persists.
func (m *ModelRoutingModel) SetModel(agent, model string) {
	e := m.config[agent]
	e.Model = model
	m.config[agent] = e
	ue := m.userConfig[agent]
	ue.Model = model
	m.userConfig[agent] = ue
	_ = opencode.WriteModelConfig(opencode.DefaultModelConfigPath(), m.userConfig)
}

// SetThinking sets thinking for agent and persists.
func (m *ModelRoutingModel) SetThinking(agent string, lvl opencode.ThinkingLevel) {
	opencode.SetThinking(m.config, agent, lvl)
	opencode.SetThinking(m.userConfig, agent, lvl)
	_ = opencode.WriteModelConfig(opencode.DefaultModelConfigPath(), m.userConfig)
}

func (m ModelRoutingModel) View() string {
	var b strings.Builder
	title := styles.Title.Render("Model Routing")
	b.WriteString(title + "\n\n")
	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render(m.err) + "\n\n")
	}
	if m.status != "" {
		b.WriteString(styles.SuccessBox.Render(m.status) + "\n\n")
	}
	switch m.view {
	case "agents":
		b.WriteString(styles.Help.Render("Agents — agents > user > builtin · t/effort · enter picker · esc quit") + "\n\n")
		for i, a := range m.agents {
			e := m.ResolveEffective(a)
			line := fmt.Sprintf("%-22s model=%s thinking=%s", a, e.Model, e.Thinking)
			if e.Model == "" {
				line = fmt.Sprintf("%-22s model=inherit thinking=%s", a, e.Thinking)
			}
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = "▸ "
				style = styles.SelectedStyle
			}
			b.WriteString(cursor + style.Render(line) + "\n")
		}
	case "thinking":
		b.WriteString(styles.Help.Render("Select thinking for "+m.selectedAgent) + "\n\n")
		for i, lvl := range thinkingOptions {
			cursor := "  "
			if i == m.thinkingCursor {
				cursor = "▸ "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", cursor, lvl))
		}
	case "picker":
		b.WriteString(styles.Help.Render("Picker 30 files — select agent for model+thinking") + "\n\n")
		for i, f := range m.pickerFiles {
			cursor := "  "
			if i == m.pickerCursor {
				cursor = "▸ "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", cursor, f))
		}
	}
	help := styles.Help.Render("↑↓ navigate · enter select · t thinking · esc back")
	b.WriteString("\n" + help + "\n")
	return styles.AppStyle.Render(b.String())
}

// PickerCount returns number of picker files (must be 30).
func PickerCount() int { return 30 }

// ThinkingLevels returns available thinking levels.
func ThinkingLevels() []opencode.ThinkingLevel { return thinkingOptions }

// ResolvePrecedence is helper for tests: agents>user>builtin.
func ResolvePrecedence(agents, user, builtin opencode.AgentModelConfig, agent string, global opencode.ThinkingLevel) opencode.AgentRoutingEntry {
	merged := opencode.MergeModelConfigs(agents, user, builtin)
	e := merged[agent]
	e.Thinking = opencode.EffectiveThinking(e.Thinking, global)
	return e
}

func sortedAgents(cfg opencode.AgentModelConfig) []string {
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
