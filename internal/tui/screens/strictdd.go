package screens

import (
	"fmt"
	"strings"

	"github.com/biggz-ai/biggz/internal/state"
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

// loadTDDState reads strict_tdd from ~/.biggz/config.json.
func loadTDDState() tea.Msg {
	enabled, err := state.GetBigMemConfig("strict_tdd")
	if err != nil {
		return tddLoadMsg{enabled: false}
	}
	return tddLoadMsg{enabled: enabled}
}

// saveTDDState writes strict_tdd to ~/.biggz/config.json.
func saveTDDState(enabled bool) tea.Msg {
	if err := state.SetBigMemConfig("strict_tdd", enabled); err != nil {
		return tddResultMsg{err: fmt.Sprintf("save: %v", err)}
	}
	s := "enabled"
	if !enabled {
		s = "disabled"
	}
	return tddResultMsg{status: fmt.Sprintf("Strict TDD %s", s)}
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
