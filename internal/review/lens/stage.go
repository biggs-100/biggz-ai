package lens

import (
	"context"
	"fmt"

	"github.com/biggs-100/biggz-ai/model"
)

// LensStage adapts a Lens to pipeline.Stage for sequential execution.
// Each lens is one Stage in PlanLenses order (risk→resilience→readability→
// reliability). The pipeline executes Stages sequentially with reverse
// rollback on failure; no graph.go/DAG is involved.
//
// Input is the frozen LensInput derived once via DeriveRiskInput (single
// --numstat -z) plus hunk bytes (≤8MiB, Truncated). The stage is stateless
// beyond the last result for inspection.
type LensStage struct {
	lens   Lens
	input  LensInput
	result *LensResult
}

// NewLensStage creates a Stage for the given lens and frozen input.
// The input must be derived once upstream; the stage never re-derives.
func NewLensStage(l Lens, input LensInput) *LensStage {
	return &LensStage{lens: l, input: input}
}

// Name returns the lens ID as the stage name (e.g., "resilience").
func (s *LensStage) Name() string {
	if s.lens == nil {
		return "lens:unknown"
	}
	return s.lens.ID()
}

// Execute runs the lens Analyze against the frozen input. The result is
// cached for inspection via Result(). Failures wrap with stage context for
// pipeline error attribution.
func (s *LensStage) Execute(ctx context.Context, _ *model.ReviewState) error {
	if s.lens == nil {
		return fmt.Errorf("lens stage: nil lens")
	}
	result, err := s.lens.Analyze(ctx, s.input)
	if err != nil {
		return fmt.Errorf("lens %s: %w", s.lens.ID(), err)
	}
	// Ensure ResultHash is populated for native heuristic results.
	if result.ResultHash == "" {
		result.ResultHash = LensResultHash(result)
	}
	s.result = &result
	return nil
}

// Rollback is a no-op: lenses are stateless heuristics with no persistent
// side effects beyond the cached result. The pipeline still calls it in
// reverse order on failure to satisfy the Stage contract.
func (s *LensStage) Rollback(_ context.Context, _ *model.ReviewState) error {
	return nil
}

// Result returns the last Analyze result, or nil if Execute has not yet
// succeeded. Non-mutating; callers must not modify the returned pointer.
func (s *LensStage) Result() *LensResult {
	return s.result
}

// Ensure LensStage implements pipeline.Stage at compile time.
// Avoid importing pipeline package here to keep lens→pipeline one-way;
// verification is via the test's interface assertion using a local copy of
// the Stage interface shape.
var _ interface {
	Name() string
	Execute(context.Context, *model.ReviewState) error
	Rollback(context.Context, *model.ReviewState) error
} = (*LensStage)(nil)
