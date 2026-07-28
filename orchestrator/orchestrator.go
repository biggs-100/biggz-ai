// Package orchestrator manages the full review lifecycle.
//
// The Orchestrator coordinates the registry, pipeline, and FSM transitions
// to execute a complete review cycle from a ReviewSubject. It handles
// state creation, status transitions, pipeline execution, and error
// recovery by transitioning to Failed on pipeline errors.
package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/pipeline"
	"github.com/biggz-ai/biggz/registry"
)

// Orchestrator manages the full review lifecycle. It holds a Registry for
// plugin access and a Pipeline for staged execution.
type Orchestrator struct {
	registry *registry.Registry
	pipeline *pipeline.Pipeline
}

// New creates an Orchestrator with the given registry and pipeline stages.
func New(reg *registry.Registry, stages ...pipeline.Stage) *Orchestrator {
	return &Orchestrator{
		registry: reg,
		pipeline: pipeline.New(stages...),
	}
}

// Execute runs a complete review cycle:
//  1. Creates a ReviewState from the subject (Status: Pending)
//  2. Transitions to InProgress
//  3. Executes the pipeline
//  4. On success: transitions to Completed
//  5. On failure: transitions to Failed, returns partial state with error
//
// The caller can always inspect the returned ReviewState — even on failure —
// to examine partial evidence or execution state.
func (o *Orchestrator) Execute(ctx context.Context, subject model.ReviewSubject) (*model.ReviewState, error) {
	state := model.NewReviewState(subject)

	if err := model.Transition(state.Status, model.StatusInProgress); err != nil {
		return nil, fmt.Errorf("start review: %w", err)
	}
	state.Status = model.StatusInProgress
	state.UpdatedAt = time.Now()

	if err := o.pipeline.Execute(ctx, state); err != nil {
		// On pipeline failure, transition to Failed and return partial
		// state so the caller can inspect what was completed.
		_ = model.Transition(state.Status, model.StatusFailed)
		state.Status = model.StatusFailed
		state.UpdatedAt = time.Now()
		return state, fmt.Errorf("pipeline: %w", err)
	}

	if err := model.Transition(state.Status, model.StatusCompleted); err != nil {
		return nil, fmt.Errorf("complete review: %w", err)
	}
	state.Status = model.StatusCompleted
	state.UpdatedAt = time.Now()

	return state, nil
}
