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

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/pipeline"
	"github.com/biggs-100/biggz-ai/registry"
)

// Orchestrator manages the full review lifecycle.
type Orchestrator struct {
	registry *registry.Registry
	pipeline *pipeline.Pipeline
	graph    *pipeline.Graph // optional DAG graph for parallel execution
}

// New creates an Orchestrator with sequential pipeline stages.
func New(reg *registry.Registry, stages ...pipeline.Stage) *Orchestrator {
	return &Orchestrator{
		registry: reg,
		pipeline: pipeline.New(stages...),
	}
}

// NewWithGraph creates an Orchestrator that uses a DAG graph for parallel execution.
// Nodes with no dependencies run concurrently. Falls back to sequential pipeline
// if graph is empty.
func NewWithGraph(reg *registry.Registry, g *pipeline.Graph) *Orchestrator {
	return &Orchestrator{
		registry: reg,
		pipeline: pipeline.New(g.Stages()...),
		graph:    g,
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

	// Use Graph (parallel DAG) when available, otherwise sequential Pipeline
	var execErr error
	if o.graph != nil && len(o.graph.Stages()) > 0 {
		execErr = o.graph.Execute(ctx, state)
	} else {
		execErr = o.pipeline.Execute(ctx, state)
	}

	if execErr != nil {
		_ = model.Transition(state.Status, model.StatusFailed)
		state.Status = model.StatusFailed
		state.UpdatedAt = time.Now()
		return state, fmt.Errorf("pipeline: %w", execErr)
	}

	if err := model.Transition(state.Status, model.StatusCompleted); err != nil {
		return nil, fmt.Errorf("complete review: %w", err)
	}
	state.Status = model.StatusCompleted
	state.UpdatedAt = time.Now()

	return state, nil
}
