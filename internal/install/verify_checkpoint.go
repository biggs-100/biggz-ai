package install

import (
	"strings"
)

// VerifyAskUserQuestionPrecededBySynthesis is a lightweight verification hook
// that detects when ask_user_question/question is called without preceding
// synthesis markers. It checks that the conversation buffer before the tool
// call contains artifacts/paths + risks + next and the checkpoint header.
//
// Intended for tests: pass the concatenated assistant output preceding the
// tool call. Returns nil when synthesis is present, error otherwise.
//
// Pi extension hook TODO: a runtime pi extension could intercept
// ask_user_question execute and verify the preceding synthesis buffer
// contains these markers before allowing the call. Currently this is a
// test-only hook; runtime enforcement remains via the orchestrator prompt.
func VerifyAskUserQuestionPrecededBySynthesis(precedingOutput string) error {
	required := []string{
		"Post-Delegation Human Checkpoint",
		"Synthesize a concise summary",
		"artifacts",
		"risks",
		"next",
	}
	for _, want := range required {
		if !strings.Contains(precedingOutput, want) {
			return &CheckpointMissingError{Missing: want}
		}
	}
	return nil
}

// CheckpointMissingError indicates a required checkpoint marker is missing.
type CheckpointMissingError struct {
	Missing string
}

func (e *CheckpointMissingError) Error() string {
	return "checkpoint synthesis missing marker: " + e.Missing
}
