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

	"github.com/biggs-100/biggz-ai/internal/sddattempt"
)

// StatusOptions configures status output.
type StatusOptions struct {
	ReviewDisabled bool
}

// ChangeStatus represents the state of an SDD change.
type ChangeStatus struct {
	Name        string
	HasProposal bool
	HasSpecs    bool
	HasDesign   bool
	HasTasks    bool
	TasksTotal  int
	TasksDone   int
	HasApply    bool
	HasVerify   bool
	IsArchived  bool

	// Edit-authority surface (all omitempty / zero-value empty): a change
	// whose task plan targets repository roots outside the authorized edit
	// roots reports the block, the missing roots, the granted roots the
	// ledger projects for its change-instance identity, and the typed
	// consent envelope naming the runnable grant invocation.
	GrantedRoots         []string                    `json:"granted_roots,omitempty"`
	EditAuthorityBlocked bool                        `json:"edit_authority_blocked,omitempty"`
	MissingRoots         []string                    `json:"missing_roots,omitempty"`
	Consent              *EditAuthorityConsentResult `json:"consent,omitempty"`
}

// Status scans the openspec/changes directory and returns the status
// of all active (non-archived) changes, plus the most recent archived ones.
func Status(openspecRoot string) (active []ChangeStatus, archived []ChangeStatus, err error) {
	workspaceRoot := filepath.Dir(openspecRoot)
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
		cs, err := readChange(filepath.Join(changesDir, entry.Name()), entry.Name(), false, workspaceRoot)
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
		cs, err := readChange(filepath.Join(archiveDir, entry.Name()), entry.Name(), true, workspaceRoot)
		if err != nil {
			continue
		}
		archived = append(archived, cs)
	}

	return active, archived, nil
}

func readChange(dir, name string, isArchived bool, workspaceRoot string) (ChangeStatus, error) {
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
	tasksText := ""
	cs.HasTasks = fileExists(filepath.Join(dir, "tasks.md"))
	if cs.HasTasks {
		data, err := os.ReadFile(filepath.Join(dir, "tasks.md"))
		if err == nil {
			tasksText = string(data)
			lines := strings.Split(tasksText, "\n")
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

	if tasksText != "" {
		// The change-instance marker is never minted for an ordinary status:
		// a change without a marker has no identity to project grants for,
		// so the ledger is not even read (zero footprint).
		instance, err := readChangeInstanceMarker(dir)
		if err != nil {
			return cs, fmt.Errorf("read change-instance marker for %s: %w", name, err)
		}
		var granted []string
		var expectedRevision string
		readGranted := func() {
			if instance == "" {
				return
			}
			status, statusErr := sddattempt.StatusWithInstance(name, workspaceRoot, instance)
			if statusErr != nil {
				// A ledger that cannot be read projects nothing: detection
				// falls back to the conservative planning-only authority.
				return
			}
			granted = status.GrantedRoots
			expectedRevision = status.Revision
		}
		readGranted()

		allowed := make([]string, 0, 1+len(granted))
		allowed = append(allowed, workspaceRoot)
		allowed = append(allowed, granted...)
		missing := detectUnauthorizedEditRoots(tasksText, workspaceRoot, allowed)
		if len(missing) > 0 {
			// A blocked status needs a token to embed: mint (or reuse) the
			// change-instance marker, then re-read the ledger scoped to the
			// real identity so the envelope chains the exact ledger head.
			if instance == "" {
				instance, err = ensureChangeInstanceMarker(dir)
				if err != nil {
					return cs, fmt.Errorf("mint change-instance marker for %s: %w", name, err)
				}
				readGranted()
			}
			cs.EditAuthorityBlocked = true
			cs.MissingRoots = missing
			cs.Consent = newEditAuthorityConsent(name, workspaceRoot, missing, instance, expectedRevision)
		}
		cs.GrantedRoots = granted
	}

	return cs, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// FormatStatus returns a human-readable summary of SDD status.
// If opts.ReviewDisabled is true, the output includes an RDD status header.
func FormatStatus(active, archived []ChangeStatus, opts StatusOptions) string {
	var b strings.Builder

	if opts.ReviewDisabled {
		b.WriteString("RDD status: disabled (unmanaged)\n\n")
	}

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
	if cs.EditAuthorityBlocked {
		status += fmt.Sprintf(" — [%s]\n    %s\n", phase, editAuthorityBlockedReason(cs.MissingRoots))
		if cs.Consent != nil && len(cs.Consent.Choices) > 0 {
			status += fmt.Sprintf("    consent grant: %s\n", cs.Consent.Choices[0].Invocation)
		}
		return status
	}
	return fmt.Sprintf("%s — [%s]\n", status, phase)
}
