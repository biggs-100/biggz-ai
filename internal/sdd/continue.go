package sdd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NextPhase determines the next SDD phase to run for a change.
// It checks which artifacts exist in the change directory and returns
// the recommended next phase.
func NextPhase(openspecRoot, changeName string) (string, error) {
	changeDir := filepath.Join(openspecRoot, "changes", changeName)
	if _, err := os.Stat(changeDir); os.IsNotExist(err) {
		return "", fmt.Errorf("change %q not found", changeName)
	}

	// Check artifacts in order
	hasProposal := fileExists(filepath.Join(changeDir, "proposal.md"))
	hasDesign := fileExists(filepath.Join(changeDir, "design.md"))
	hasTasks := fileExists(filepath.Join(changeDir, "tasks.md"))

	// Check specs (either in change/specs/ or in openspec/specs/)
	hasSpecs := false
	specsDir := filepath.Join(changeDir, "specs")
	if entries, err := os.ReadDir(specsDir); err == nil && len(entries) > 0 {
		// Check if there are actual spec files (not empty dirs)
		for _, e := range entries {
			if e.IsDir() {
				specFile := filepath.Join(specsDir, e.Name(), "spec.md")
				if fileExists(specFile) {
					hasSpecs = true
					break
				}
			}
		}
	}

	// Check apply progress
	hasApply := fileExists(filepath.Join(changeDir, "apply-progress.md"))
	hasVerify := fileExists(filepath.Join(changeDir, "verify-report.md"))

	// Read tasks progress
	tasksDone, tasksTotal := countTaskProgress(filepath.Join(changeDir, "tasks.md"))

	// Determine next phase
	switch {
	case !hasProposal:
		return "proposal", nil
	case !hasSpecs:
		return "spec", nil
	case !hasDesign:
		return "design", nil
	case !hasTasks:
		return "tasks", nil
	case tasksDone < tasksTotal:
		return fmt.Sprintf("apply (%d/%d tasks)", tasksDone, tasksTotal), nil
	case !hasApply:
		return "apply", nil
	case !hasVerify:
		return "verify", nil
	default:
		return "archive", nil
	}
}

// NextPhaseDescription returns a human-readable description of the next phase.
func NextPhaseDescription(phase string) string {
	switch {
	case phase == "proposal":
		return "Explore and create a proposal for this change."
	case phase == "spec":
		return "Write specifications for the capabilities defined in the proposal."
	case phase == "design":
		return "Create a technical design based on the specs."
	case phase == "tasks":
		return "Break down the design into implementation tasks."
	case strings.HasPrefix(phase, "apply"):
		return "Implement the remaining tasks: " + phase
	case phase == "verify":
		return "Verify the implementation against specs."
	case phase == "archive":
		return "Archive the completed change."
	default:
		return "Unknown phase: " + phase
	}
}

func countTaskProgress(tasksPath string) (done, total int) {
	data, err := os.ReadFile(tasksPath)
	if err != nil {
		return 0, 0
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [") {
			total++
			if strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]") {
				done++
			}
		}
	}
	return
}
