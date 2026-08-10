package screens

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/agents"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/biggs-100/biggz-ai/internal/uninstall"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
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

// defaultAdapters builds the adapter map from the init-registered factories.
// All adapter packages are imported elsewhere in the binary, so their init()
// functions have run before this screen can be reached.
func defaultAdapters() (map[string]plugin.AgentAdapter, error) {
	reg, err := agents.NewDefaultRegistry()
	if err != nil {
		return nil, err
	}
	adapters := make(map[string]plugin.AgentAdapter)
	for _, entry := range reg.ListAll() {
		if a, ok := reg.Get(model.AgentID(entry.ID)); ok {
			adapters[entry.ID] = a
		}
	}
	return adapters, nil
}

// doUninstall removes biggz-ai artifacts through the surgical per-agent
// inventory (internal/uninstall), which keeps user data (BigMem memory,
// backups, custom agents) intact. The TUI owns the confirmation step, so the
// run is authorized with Yes=true and never prompts again.
func doUninstall() tea.Msg {
	home, _ := os.UserHomeDir()
	adapters, err := defaultAdapters()
	if err != nil {
		return uninstallResultMsg{err: fmt.Sprintf("uninstall: %v", err)}
	}
	res, err := uninstall.Run(context.Background(), adapters, uninstall.Config{
		HomeDir: home,
		Yes:     true,
	})
	if err != nil {
		return uninstallResultMsg{err: fmt.Sprintf("uninstall: %v", err)}
	}
	status, reportErr := uninstallReport(res)
	return uninstallResultMsg{status: status, err: reportErr}
}

// uninstallReport renders the uninstall Result for the TUI. It returns the
// success status and, when any operation failed, the failure report as the
// error string (partial failures are not reverted; re-running only removes
// leftovers).
func uninstallReport(res *uninstall.Result) (status string, reportErr string) {
	var removed []string
	for _, ar := range res.AgentResults {
		if ar.RemovedFiles == 0 && ar.RewrittenConfigs == 0 {
			continue
		}
		removed = append(removed, fmt.Sprintf("%s: removed %d files, %d configs rewritten",
			ar.AgentID, ar.RemovedFiles, ar.RewrittenConfigs))
	}
	if len(removed) == 0 && len(res.Failed) == 0 {
		removed = append(removed, "nothing to remove")
	}
	status = fmt.Sprintf("Uninstalled. Removed:\n  • %s\n%s\n\nKept: BigMem memory data, backups, custom agents — use `biggz uninstall --purge` for a full wipe.",
		strings.Join(removed, "\n  • "), res.Summary)
	if len(res.Failed) > 0 {
		var failures []string
		for _, f := range res.Failed {
			failures = append(failures, fmt.Sprintf("%s: FAILED %s: %v", f.Agent, f.Op, f.Err))
		}
		reportErr = fmt.Sprintf("%s\n\n%s\n\nPress [R] to retry (successes are not reverted; re-running only removes leftovers).",
			strings.Join(failures, "\n"), res.Summary)
	}
	return status, reportErr
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
			m.status = msg.status
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
		b.WriteString("  • Agent skills, prompts, commands, and plugins\n")
		b.WriteString("  • biggz settings keys and MCP config\n")
		b.WriteString("  • biggz persona / memory-protocol sections in AGENTS.md\n")
		b.WriteString("  • Shared ~/.biggz store (skills, binaries)\n")
		b.WriteString("  • Windows PATH entry\n")
		b.WriteString("\nKept: BigMem memory data, backups, custom agents.\n")
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
		b.WriteString(styles.ErrorBox.Render(fmt.Sprintf("Uninstall finished with failures:\n%s", m.err)))
		b.WriteString("\n\nPress [R] to retry.")
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("ESC back · ? help"))
	return styles.AppStyle.Render(b.String())
}
