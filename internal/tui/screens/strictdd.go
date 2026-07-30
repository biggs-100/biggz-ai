package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/filemerge"
	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// StrictTDDModel toggles Strict TDD mode.
type StrictTDDModel struct {
	enabled bool
	loaded  bool
	err     string
	status  string
}

// NewStrictTDDModel creates the strict TDD screen.
func NewStrictTDDModel() StrictTDDModel {
	return StrictTDDModel{}
}

func (m StrictTDDModel) Init() tea.Cmd { return nil }

// tddLoadMsg carries the current state.
type tddLoadMsg struct {
	enabled bool
	err     string
}

// tddResultMsg carries the save result.
type tddResultMsg struct {
	status string
	err    string
}

// loadTDDState reads strict_tdd from opencode.json.
func loadTDDState() tea.Msg {
	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return tddLoadMsg{enabled: false}
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return tddLoadMsg{enabled: false}
	}
	enabled, _ := cfg["strict_tdd"].(bool)
	return tddLoadMsg{enabled: enabled}
}

// saveTDDState writes strict_tdd to opencode.json.
func saveTDDState(enabled bool) tea.Msg {
	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return tddResultMsg{err: fmt.Sprintf("read config: %v", err)}
	}

	overlay := fmt.Sprintf(`{"strict_tdd":%v}`, enabled)
	merged, err := filemerge.MergeJSONC(data, []byte(overlay))
	if err != nil {
		return tddResultMsg{err: fmt.Sprintf("merge: %v", err)}
	}
	if _, err := filemerge.WriteFileAtomic(cfgPath, merged, 0644); err != nil {
		return tddResultMsg{err: fmt.Sprintf("write: %v", err)}
	}

	state := "enabled"
	if !enabled {
		state = "disabled"
	}
	return tddResultMsg{status: fmt.Sprintf("Strict TDD %s", state)}
}

// Update handles input.
func (m StrictTDDModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, loadTDDState
		case "enter", " ":
			if !m.loaded {
				return m, loadTDDState
			}
			// Toggle
			m.enabled = !m.enabled
			return m, func() tea.Msg { return saveTDDState(m.enabled) }
		case "up", "k":
			if m.loaded {
				m.enabled = !m.enabled
			}
		case "down", "j":
			if m.loaded {
				m.enabled = !m.enabled
			}
		}

	case tddLoadMsg:
		if msg.err != "" {
			m.err = msg.err
			return m, nil
		}
		m.enabled = msg.enabled
		m.loaded = true
		m.err = ""

	case tddResultMsg:
		if msg.err != "" {
			m.err = msg.err
			return m, nil
		}
		m.status = msg.status
	}

	return m, nil
}

// View renders.
func (m StrictTDDModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Strict TDD"))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render(m.err))
		b.WriteString("\n\n")
	}
	if m.status != "" {
		b.WriteString(styles.SuccessBox.Render(m.status))
		b.WriteString("\n\n")
	}

	if !m.loaded {
		b.WriteString("Press ENTER to load current state.\n\n")
		b.WriteString(styles.StatusInfo.Render("Strict TDD mode requires tests to pass before\n"))
		b.WriteString(styles.StatusInfo.Render("code can be committed. Enforced during sdd-apply.\n"))
	} else {
		b.WriteString(styles.Section.Render("Mode"))
		b.WriteString("\n\n")
		enableSel := styles.CheckboxEmpty.String()
		disableSel := styles.CheckboxEmpty.String()
		if m.enabled {
			enableSel = styles.CheckboxSelected.String()
		} else {
			disableSel = styles.CheckboxSelected.String()
		}
		b.WriteString(fmt.Sprintf("  %sEnable  — tests must pass before apply\n", enableSel))
		b.WriteString(fmt.Sprintf("  %sDisable — normal mode\n", disableSel))
		b.WriteString("\n\n")
		b.WriteString(styles.Help.Render("ENTER to toggle · [R] refresh · ESC back"))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("? help"))
	return styles.AppStyle.Render(b.String())
}
