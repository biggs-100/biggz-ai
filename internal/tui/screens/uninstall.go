package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type uninstallStep int

const (
	uninstallIdle uninstallStep = iota
	uninstallConfirm
	uninstallRunning
	uninstallDone
	uninstallError
)

// UninstallModel handles cleaning up biggz-ai.
type UninstallModel struct {
	step   uninstallStep
	err    string
	status string
}

// NewUninstallModel creates the uninstall screen.
func NewUninstallModel() UninstallModel {
	return UninstallModel{step: uninstallIdle}
}

func (m UninstallModel) Init() tea.Cmd { return nil }

// uninstallResultMsg carries the result.
type uninstallResultMsg struct {
	status string
	err    string
}

// doUninstall removes biggz-ai artifacts.
func doUninstall() tea.Msg {
	home, _ := os.UserHomeDir()
	var removed []string

	// Remove ~/.biggz/ directory
	biggzDir := filepath.Join(home, ".biggz")
	if info, err := os.Stat(biggzDir); err == nil && info.IsDir() {
		if err := os.RemoveAll(biggzDir); err != nil {
			return uninstallResultMsg{err: fmt.Sprintf("remove .biggz: %v", err)}
		}
		removed = append(removed, "~/.biggz/")
	}

	// Reset opencode.json — remove biggz-orchestrator agent entry
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if data, err := os.ReadFile(configPath); err == nil {
		// Remove biggz-orchestrator entry
		overlay := []byte(`{"agent":{"biggz-orchestrator":{"__replace__":null}},"mcp":{"biggz":{"__replace__":null}}}`)
		merged, err := filemerge.MergeJSONC(data, overlay)
		if err == nil {
			if _, err := filemerge.WriteFileAtomic(configPath, merged, 0644); err == nil {
				removed = append(removed, "opencode.json (biggs entries cleaned)")
			}
		}
	}

	// Remove biggz persona from AGENTS.md
	agentsPath := filepath.Join(home, ".config", "opencode", "AGENTS.md")
	if data, err := os.ReadFile(agentsPath); err == nil {
		content := string(data)
		openMarker := "<!-- biggz:persona -->"
		closeMarker := "<!-- /biggz:persona -->"
		openIdx := strings.Index(content, openMarker)
		if openIdx >= 0 {
			closeIdx := strings.Index(content, closeMarker)
			if closeIdx >= 0 {
				newContent := content[:openIdx] + content[closeIdx+len(closeMarker):]
				if _, err := filemerge.WriteFileAtomic(agentsPath, []byte(newContent), 0644); err == nil {
					removed = append(removed, "AGENTS.md (biggs persona removed)")
				}
			}
		}
	}

	return uninstallResultMsg{
		status: fmt.Sprintf("Uninstalled. Removed:\n  • %s", strings.Join(removed, "\n  • ")),
	}
}

// Update handles input.
func (m UninstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			if m.step == uninstallIdle {
				m.step = uninstallConfirm
				return m, nil
			}
			if m.step == uninstallConfirm {
				m.step = uninstallRunning
				return m, doUninstall
			}
		case "y":
			if m.step == uninstallConfirm {
				m.step = uninstallRunning
				return m, doUninstall
			}
		case "n":
			if m.step == uninstallConfirm {
				m.step = uninstallIdle
			}
		case "r":
			if m.step == uninstallDone || m.step == uninstallError {
				m.step = uninstallIdle
				m.err = ""
			}
		}

	case uninstallResultMsg:
		if msg.err != "" {
			m.step = uninstallError
			m.err = msg.err
			return m, nil
		}
		m.status = msg.status
		m.step = uninstallDone
	}

	return m, nil
}

// View renders.
func (m UninstallModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Uninstall"))
	b.WriteString("\n\n")

	switch m.step {
	case uninstallIdle:
		b.WriteString("This will remove biggz-ai from your system.\n\n")
		b.WriteString(styles.StatusInfo.Render("What gets removed:\n"))
		b.WriteString("  • ~/.biggz/ directory (skills, BigMem, config)\n")
		b.WriteString("  • biggz-orchestrator from opencode.json\n")
		b.WriteString("  • MCP server config for biggz\n")
		b.WriteString("  • biggz persona from AGENTS.md\n")
		b.WriteString("\nNote: gentle-ai and its data are NOT affected.\n")
		b.WriteString("\nPress ENTER to continue.\n")

	case uninstallConfirm:
		b.WriteString(styles.WarningBox.Render("Are you sure? This cannot be undone."))
		b.WriteString("\n\n")
		b.WriteString(styles.Help.Render("[Y]es to uninstall · [N]o to cancel"))

	case uninstallRunning:
		b.WriteString(styles.Spinner.Render("Uninstalling..."))

	case uninstallDone:
		b.WriteString(styles.SuccessBox.Render(m.status))
		b.WriteString("\n\nPress [R] to restart.")

	case uninstallError:
		b.WriteString(styles.ErrorBox.Render(fmt.Sprintf("Uninstall failed: %s", m.err)))
		b.WriteString("\n\nPress [R] to retry.")
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("ESC back · ? help"))
	return styles.AppStyle.Render(b.String())
}
