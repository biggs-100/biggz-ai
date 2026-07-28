package sdd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AttemptState tracks review/correction attempts for an SDD change.
// Stored as a JSON file in the change directory.
type AttemptState struct {
	ChangeName      string `json:"change_name"`
	ActiveAttempt   int    `json:"active_attempt"`
	TotalAttempts   int    `json:"total_attempts"`
	MaxAttempts     int    `json:"max_attempts"`
	CorrectionLines int    `json:"correction_lines"`
	MaxChangedLines int    `json:"max_changed_lines"`
	Status          string `json:"status"` // "idle", "in_progress", "completed", "exhausted"
	UpdatedAt       string `json:"updated_at"`
}

// attemptPath returns the path to the attempt state file for a change.
func attemptPath(openspecRoot, changeName string) string {
	return filepath.Join(openspecRoot, "changes", changeName, ".attempt.json")
}

// AttemptStatus returns the current attempt state for a change.
func AttemptStatus(openspecRoot, changeName string) (*AttemptState, error) {
	state, err := readAttemptState(openspecRoot, changeName)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// AttemptBegin starts a new attempt for a change.
// Returns error if max attempts exceeded or an attempt is already in progress.
func AttemptBegin(openspecRoot, changeName string, budgetLines int) (*AttemptState, error) {
	state, err := readOrCreateAttemptState(openspecRoot, changeName)
	if err != nil {
		return nil, err
	}

	if state.Status == "in_progress" {
		return nil, fmt.Errorf("attempt already in progress for change %q", changeName)
	}

	if state.TotalAttempts >= state.MaxAttempts {
		return nil, fmt.Errorf("max attempts (%d) exhausted for change %q", state.MaxAttempts, changeName)
	}

	state.ActiveAttempt = state.TotalAttempts + 1
	state.TotalAttempts++
	state.Status = "in_progress"
	if budgetLines > 0 {
		state.MaxChangedLines = budgetLines
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := saveAttemptState(openspecRoot, changeName, state); err != nil {
		return nil, err
	}
	return state, nil
}

// AttemptFinish marks the current attempt as completed or failed.
func AttemptFinish(openspecRoot, changeName string, success bool, linesChanged int) (*AttemptState, error) {
	state, err := readAttemptState(openspecRoot, changeName)
	if err != nil {
		return nil, err
	}

	if state.Status != "in_progress" {
		return nil, fmt.Errorf("no active attempt for change %q", changeName)
	}

	state.CorrectionLines += linesChanged
	state.ActiveAttempt = 0

	if success {
		state.Status = "completed"
	} else if state.TotalAttempts >= state.MaxAttempts {
		state.Status = "exhausted"
	} else {
		state.Status = "idle"
	}

	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := saveAttemptState(openspecRoot, changeName, state); err != nil {
		return nil, err
	}
	return state, nil
}

// AttemptReset resets the attempt state for a change.
func AttemptReset(openspecRoot, changeName string) (*AttemptState, error) {
	state := &AttemptState{
		ChangeName:      changeName,
		MaxAttempts:     3,
		MaxChangedLines: 400,
		Status:          "idle",
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	if err := saveAttemptState(openspecRoot, changeName, state); err != nil {
		return nil, err
	}
	return state, nil
}

// --- internal helpers ---

func readAttemptState(openspecRoot, changeName string) (*AttemptState, error) {
	path := attemptPath(openspecRoot, changeName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err // raw error so os.IsNotExist works
	}
	var state AttemptState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse attempt state: %w", err)
	}
	return &state, nil
}

func readOrCreateAttemptState(openspecRoot, changeName string) (*AttemptState, error) {
	state, err := readAttemptState(openspecRoot, changeName)
	if err == nil {
		return state, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	// Create default state
	return &AttemptState{
		ChangeName:      changeName,
		MaxAttempts:     3,
		MaxChangedLines: 400,
		Status:          "idle",
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func saveAttemptState(openspecRoot, changeName string, state *AttemptState) error {
	path := attemptPath(openspecRoot, changeName)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}
