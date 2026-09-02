package pipeline

import (
	"context"
	"fmt"
)

// Plan is a concrete StagePlan holding an ordered list of Steps.
type Plan struct {
	steps []Step
}

// NewPlan creates a Plan from the given steps in order.
func NewPlan(steps ...Step) *Plan { return &Plan{steps: steps} }

// Steps returns the underlying steps for Orchestrator rollback inspection.
func (p *Plan) Steps() []Step { return p.steps }

// ID compatibility helper: Plan implements StagePlan; Steps may also
// expose ID() alias via type switch in callers that expect ID/Run naming.
func (p *Plan) Prepare(ctx context.Context) (*PlanPreview, error) {
	names := make([]string, 0, len(p.steps))
	for _, s := range p.steps {
		if err := s.Prepare(ctx); err != nil {
			return nil, fmt.Errorf("%s: %w", s.Name(), err)
		}
		names = append(names, s.Name())
	}
	return &PlanPreview{Steps: names}, nil
}

// Apply executes steps sequentially, wrapping errors as "%s: %w".
// It owns close(ch) and is safe for Orchestrator double-close via SafeClose.
func (p *Plan) Apply(ctx context.Context, ch ProgressChan) (*ExecutionResult, error) {
	defer SafeClose(ch)
	var results []StepResult
	for _, s := range p.steps {
		select {
		case <-ctx.Done():
			wrapped := fmt.Errorf("%s: %w", s.Name(), ctx.Err())
			results = append(results, StepResult{Step: s.Name(), Applied: false, Error: wrapped})
			return &ExecutionResult{Success: false, Steps: results, Error: wrapped}, wrapped
		default:
		}
		if err := s.Apply(ctx, ch); err != nil {
			wrapped := fmt.Errorf("%s: %w", s.Name(), err)
			results = append(results, StepResult{Step: s.Name(), Applied: false, Error: wrapped})
			return &ExecutionResult{Success: false, Steps: results, Error: wrapped}, wrapped
		}
		results = append(results, StepResult{Step: s.Name(), Applied: true})
	}
	return &ExecutionResult{Success: true, Steps: results}, nil
}
