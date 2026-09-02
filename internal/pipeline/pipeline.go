package pipeline

import "context"

// ProgressEvent is emitted during Apply to report step progress.
type ProgressEvent struct {
	Step    string
	Percent int
	Message string
}

// ProgressChan carries ProgressEvent values from Apply to consumers.
// Buffered (cap >=16) and closed by Apply on completion.
type ProgressChan chan ProgressEvent

// RollbackPolicy controls post-failure rollback behavior.
type RollbackPolicy int

const (
	NoRollback RollbackPolicy = iota
	RollbackOnFailure
)

// StepResult reports per-step outcome.
type StepResult struct {
	Step    string
	Applied bool
	Error   error
}

// ExecutionResult is returned by StagePlan.Apply and Orchestrator.Run.
type ExecutionResult struct {
	Success bool
	Steps   []StepResult
	Error   error
}

// PlanPreview is returned by Prepare with ordered step names.
type PlanPreview struct {
	Steps []string
}

// Step is a single reversible unit of work.
type Step interface {
	Name() string
	Prepare(ctx context.Context) error
	Apply(ctx context.Context, ch ProgressChan) error
	Rollback(ctx context.Context) error
}

// StagePlan groups steps into Prepare (dry-run preview) and Apply (sequential).
type StagePlan interface {
	Prepare(ctx context.Context) (*PlanPreview, error)
	Apply(ctx context.Context, ch ProgressChan) (*ExecutionResult, error)
}
