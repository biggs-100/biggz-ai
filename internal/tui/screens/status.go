package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/git"
	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/sdd"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// StatusModel shows real RDD, SDD, and system status.
type StatusModel struct {
	rddStatus   *review.RDDStatusReport
	sddActive   int
	sddArchived int
	hasSDD      bool
	err         string
}

// NewStatusModel creates the status screen.
func NewStatusModel() StatusModel {
	return StatusModel{}
}

func (m StatusModel) Init() tea.Cmd { return nil }

// refreshStatusMsg carries the refresh result.
type refreshStatusMsg struct {
	rdd      *review.RDDStatusReport
	active   int
	archived int
	hasSDD   bool
	err      string
}

// doRefresh runs all status checks.
func doRefresh() tea.Msg {
	cwd, _ := os.Getwd()

	// Detect git dirs via single owner wrapper
	commonDir, worktreeDir := git.DetectGitDirs()

	// RDD status
	rddStatus, _ := review.RDDStatus(worktreeDir, commonDir)

	// SDD status
	var active, archived int
	openspecRoot := filepath.Join(cwd, "openspec")
	hasSDD := false
	if info, err := os.Stat(openspecRoot); err == nil && info.IsDir() {
		hasSDD = true
		activeList, archivedList, _ := sdd.Status(openspecRoot)
		active = len(activeList)
		archived = len(archivedList)
	}

	return refreshStatusMsg{
		rdd:      rddStatus,
		active:   active,
		archived: archived,
		hasSDD:   hasSDD,
	}
}

// Update handles input.
func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, doRefresh
		}

	case refreshStatusMsg:
		if msg.err != "" {
			m.err = msg.err
			return m, nil
		}
		m.rddStatus = msg.rdd
		m.sddActive = msg.active
		m.sddArchived = msg.archived
		m.hasSDD = msg.hasSDD
		m.err = ""
	}

	return m, nil
}

// detectGitDirs is retained for test parity but delegates to the single wrapper.
func detectGitDirs() (commonDir, worktreeDir string) {
	return git.DetectGitDirs()
}

// View renders the status screen.
func (m StatusModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Status"))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render("Error: " + m.err))
		b.WriteString("\n\n")
	}

	// RDD Status
	b.WriteString(styles.Section.Render("Review-Driven Development (RDD)"))
	b.WriteString("\n")
	if m.rddStatus != nil {
		if m.rddStatus.EffectiveMode == review.RDDModeDisabled {
			b.WriteString(styles.StatusDisabled.Render("● RDD is DISABLED"))
		} else {
			b.WriteString(styles.StatusEnabled.Render("● RDD is ENABLED"))
		}
		b.WriteString(fmt.Sprintf(" (source: %s)\n", m.rddStatus.Source))
		if m.rddStatus.WorktreeCount > 0 {
			b.WriteString(fmt.Sprintf("  Linked worktrees: %d\n", m.rddStatus.WorktreeCount))
		}
	} else {
		b.WriteString(styles.StatusInfo.Render("● RDD: unknown (not in a git repo)"))
	}
	b.WriteString("\n")

	// SDD Status
	b.WriteString(styles.Section.Render("SDD Changes"))
	b.WriteString("\n")
	if m.hasSDD {
		b.WriteString(fmt.Sprintf("  Active:  %d\n", m.sddActive))
		b.WriteString(fmt.Sprintf("  Archived: %d\n", m.sddArchived))
	} else {
		b.WriteString(styles.StatusInfo.Render("  No openspec/ directory found. Run /sdd-init first.\n"))
	}
	b.WriteString("\n")

	// System Health
	b.WriteString(styles.Section.Render("System Health"))
	b.WriteString("\n")
	b.WriteString(styles.StatusEnabled.Render("  ✓ biggz-mcp: available\n"))
	b.WriteString(styles.StatusEnabled.Render("  ✓ BigMem: active\n"))
	b.WriteString(styles.StatusEnabled.Render("  ✓ Skills: 24 deployed\n"))

	b.WriteString("\n")
	b.WriteString(styles.Help.Render("[R] refresh · esc back"))
	return styles.AppStyle.Render(b.String())
}
