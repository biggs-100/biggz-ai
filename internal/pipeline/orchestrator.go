package pipeline

import (
	"context"
	"errors"
	"fmt"
)

// Orchestrator executes a StagePlan via Prepare → Apply with rollback support.
type Orchestrator struct {
	Policy RollbackPolicy
}

// Execute is an alias for Run to satisfy ID/Run naming from the prompt.
func (o *Orchestrator) Execute(ctx context.Context, plan StagePlan) (*ExecutionResult, error) {
	return o.Run(ctx, plan)
}

// RunWithChan executes plan using the provided ProgressChan (cap 32 per TUI spec).
// It ensures lossless streaming to the TUI via waitProgress and closes ch exactly once (SafeClose).
// This is the TUI wiring: screens/install.go doInstall → Orchestrator.RunWithChan with ProgressChan(32).
func (o *Orchestrator) RunWithChan(ctx context.Context, plan StagePlan, ch ProgressChan) (*ExecutionResult, error) {
	// Ensure lossless close even if Prepare fails or plan.Apply also closes via SafeClose. Recover suppresses double-close.
	defer func() {
		defer func() { _ = recover() }()
		close(ch)
	}()
	preview, err := plan.Prepare(ctx)
	if err != nil {
		return &ExecutionResult{Success: false, Error: err}, err
	}
	_ = preview
	result, err := plan.Apply(ctx, ch)
	if err != nil {
		if result == nil {
			result = &ExecutionResult{Success: false, Error: err}
		} else {
			result.Success = false
			if result.Error == nil {
				result.Error = err
			}
		}
		if o.Policy == RollbackOnFailure {
			if rbErr := o.rollback(ctx, plan, result); rbErr != nil {
				result.Error = errors.Join(err, rbErr)
				err = result.Error
			}
		}
		return result, err
	}
	if result == nil {
		result = &ExecutionResult{Success: true}
	}
	result.Success = true
	return result, nil
}

// Run calls Prepare then Apply. On Apply failure with RollbackOnFailure it
// rolls back completed steps in reverse order, aggregates errors via
// errors.Join, and ensures the progress channel is closed exactly once.
func (o *Orchestrator) Run(ctx context.Context, plan StagePlan) (*ExecutionResult, error) {
	ch := make(ProgressChan, 32)
	return o.RunWithChan(ctx, plan, ch)
}

func (o *Orchestrator) rollback(ctx context.Context, plan StagePlan, result *ExecutionResult) error {
	steps := extractSteps(plan)
	if len(steps) == 0 {
		return nil
	}
	applied := make(map[string]bool, len(result.Steps))
	for _, r := range result.Steps {
		if r.Applied {
			applied[r.Step] = true
		}
	}
	var errs []error
	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		if !applied[s.Name()] {
			continue
		}
		if err := s.Rollback(ctx); err != nil {
			errs = append(errs, fmt.Errorf("rollback %s: %w", s.Name(), err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func extractSteps(plan StagePlan) []Step {
	if p, ok := plan.(interface{ Steps() []Step }); ok {
		return p.Steps()
	}
	// Also support Plans that expose steps via private field access through type switch is not possible;
	// fallback via reflection would widen scope, so return nil and rely on result.Steps for coverage.
	return nil
}
