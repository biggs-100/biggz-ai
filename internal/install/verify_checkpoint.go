package install

import (
	"strings"
)

// VerifyAskUserQuestionPrecededBySynthesis is a lightweight verification hook
// that detects when a checkpoint ask (irreversible action) is called without
// preceding full synthesis markers. It checks that the conversation buffer
// before the tool call contains the full-block header plus artifacts + risks
// + next. Routine quiet one-liners carry no full block by design, so this
// hook applies to checkpoint asks only.
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
		"Post-Delegation Report",
		"## Sub-agent Result",
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
