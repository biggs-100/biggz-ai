// Package sdd implements native SDD commands for biggz-ai.
//
// These are the backend commands that SDD skills call to read status,
// validate reports, and manage attempts. They make biggz-ai
// self-sufficient without depending on external skills for basic ops.
package sdd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChangeStatus represents the state of an SDD change.
type ChangeStatus struct {
	Name       string
	HasProposal bool
	HasSpecs   bool
	HasDesign  bool
	HasTasks   bool
	TasksTotal int
	TasksDone  int
	HasApply   bool
	HasVerify  bool
	IsArchived bool
}

// Status scans the openspec/changes directory and returns the status
// of all active (non-archived) changes, plus the most recent archived ones.
func Status(openspecRoot string) (active []ChangeStatus, archived []ChangeStatus, err error) {
	changesDir := filepath.Join(openspecRoot, "changes")
	archiveDir := filepath.Join(changesDir, "archive")

	entries, err := os.ReadDir(changesDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read changes dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "archive" {
			continue
		}
		cs, err := readChange(filepath.Join(changesDir, entry.Name()), entry.Name(), false)
		if err != nil {
			continue
		}
		active = append(active, cs)
	}

	// Archived
	archiveEntries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return active, nil, nil
		}
		return active, nil, fmt.Errorf("read archive dir: %w", err)
	}

	// Show last 3 archived
	for i := len(archiveEntries) - 1; i >= 0 && len(archived) < 3; i-- {
		entry := archiveEntries[i]
		if !entry.IsDir() {
			continue
		}
		cs, err := readChange(filepath.Join(archiveDir, entry.Name()), entry.Name(), true)
		if err != nil {
			continue
		}
		archived = append(archived, cs)
	}

	return active, archived, nil
}

func readChange(dir, name string, isArchived bool) (ChangeStatus, error) {
	cs := ChangeStatus{Name: name, IsArchived: isArchived}

	// Check artifacts
	cs.HasProposal = fileExists(filepath.Join(dir, "proposal.md"))
	cs.HasDesign = fileExists(filepath.Join(dir, "design.md"))
	cs.HasApply = fileExists(filepath.Join(dir, "apply-progress.md"))
	cs.HasVerify = fileExists(filepath.Join(dir, "verify-report.md"))

	// Check specs subdirectory
	specsDir := filepath.Join(dir, "specs")
	if specEntries, err := os.ReadDir(specsDir); err == nil && len(specEntries) > 0 {
		cs.HasSpecs = true
	}

	// Parse tasks
	cs.HasTasks = fileExists(filepath.Join(dir, "tasks.md"))
	if cs.HasTasks {
		data, err := os.ReadFile(filepath.Join(dir, "tasks.md"))
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "- [") {
					cs.TasksTotal++
					if strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]") {
						cs.TasksDone++
					}
				}
			}
		}
	}

	return cs, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// FormatStatus returns a human-readable summary of SDD status.
func FormatStatus(active, archived []ChangeStatus) string {
	var b strings.Builder

	if len(active) == 0 {
		b.WriteString("No active changes.\n")
	} else {
		b.WriteString("Active changes:\n")
		for _, cs := range active {
			b.WriteString(formatOne(cs, false))
		}
	}

	if len(archived) > 0 {
		b.WriteString("\nRecent archived:\n")
		for _, cs := range archived {
			b.WriteString(formatOne(cs, true))
		}
	}

	return b.String()
}

func formatOne(cs ChangeStatus, archived bool) string {
	icon := map[bool]string{true: "✅", false: "⬜"}

	phase := ""
	switch {
	case !cs.HasProposal:
		phase = "explore/proposal"
	case !cs.HasSpecs:
		phase = "spec"
	case !cs.HasDesign:
		phase = "design"
	case !cs.HasTasks:
		phase = "tasks"
	case cs.TasksDone < cs.TasksTotal:
		phase = fmt.Sprintf("apply (%d/%d tasks)", cs.TasksDone, cs.TasksTotal)
	case !cs.HasVerify:
		phase = "verify"
	default:
		phase = "archive-ready"
	}

	status := fmt.Sprintf("  %s %s", icon[cs.HasProposal && cs.TasksDone == cs.TasksTotal], cs.Name)
	if archived {
		return fmt.Sprintf("  • %s (%s)\n", cs.Name, phase)
	}
	return fmt.Sprintf("  %s — [%s]\n", status, phase)
}
