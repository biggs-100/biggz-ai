package pi

// ProgressState reports Pi install pipeline progress.
type ProgressState struct {
	Percent     int    `json:"percent"`
	CurrentStep string `json:"current_step"`
	HasFailures bool   `json:"has_failures"`
}

// ProgressStep is one step in the install pipeline.
type ProgressStep struct {
	ID     string
	Status string
}

const (
	ProgressStatusPending   = "pending"
	ProgressStatusRunning   = "running"
	ProgressStatusSucceeded = "succeeded"
	ProgressStatusFailed    = "failed"
)

// NewProgressState creates a pending progress state for the given step IDs.
func NewProgressState(steps []string) ProgressState {
	if len(steps) == 0 {
		return ProgressState{Percent: 100}
	}
	return ProgressState{Percent: 0, CurrentStep: steps[0]}
}

// ProgressFromExecution aggregates step results into a deterministic ProgressState.
// steps with succeeded or failed count as completed; running/pending do not.
func ProgressFromExecution(steps []ProgressStep) ProgressState {
	if len(steps) == 0 {
		return ProgressState{Percent: 100}
	}
	completed := 0
	hasFailures := false
	currentStep := ""
	for _, s := range steps {
		switch s.Status {
		case ProgressStatusSucceeded, ProgressStatusFailed:
			completed++
			if s.Status == ProgressStatusFailed {
				hasFailures = true
			}
		case ProgressStatusRunning:
			if currentStep == "" {
				currentStep = s.ID
			}
		}
	}
	if currentStep == "" {
		for _, s := range steps {
			if s.Status == ProgressStatusPending {
				currentStep = s.ID
				break
			}
		}
	}
	percent := (completed * 100) / len(steps)
	return ProgressState{Percent: percent, CurrentStep: currentStep, HasFailures: hasFailures}
}
