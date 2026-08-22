// Package orchestrator manages the full review lifecycle.
//
// The Orchestrator coordinates pipeline execution of a complete review
// cycle from a ReviewSubject. It handles state creation, direct status
// updates on the ReviewState, and error recovery by marking the state
// Failed on pipeline errors.
package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/pipeline"
)

// Orchestrator manages the full review lifecycle.
type Orchestrator struct {
	pipeline *pipeline.Pipeline
	graph    *pipeline.Graph // optional DAG graph for parallel execution
}

// New creates an Orchestrator with sequential pipeline stages.
func New(stages ...pipeline.Stage) *Orchestrator {
	return &Orchestrator{
		pipeline: pipeline.New(stages...),
	}
}

// NewWithGraph creates an Orchestrator that uses a DAG graph for parallel execution.
// Nodes with no dependencies run concurrently. Falls back to sequential pipeline
// if graph is empty.
func NewWithGraph(g *pipeline.Graph) *Orchestrator {
	return &Orchestrator{
		pipeline: pipeline.New(g.Stages()...),
		graph:    g,
	}
}

// Execute runs a complete review cycle:
//  1. Creates a ReviewState from the subject (Status: InProgress)
//  2. Executes the pipeline
//  3. On success: marks the state Completed
//  4. On failure: marks the state Failed, returns partial state with error
//
// The caller can always inspect the returned ReviewState — even on failure —
// to examine partial evidence or execution state.
func (o *Orchestrator) Execute(ctx context.Context, subject model.ReviewSubject) (*model.ReviewState, error) {
	state := model.NewReviewState(subject)

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
		state.Status = model.StatusFailed
		state.UpdatedAt = time.Now()
		return state, fmt.Errorf("pipeline: %w", execErr)
	}

	state.Status = model.StatusCompleted
	state.UpdatedAt = time.Now()

	return state, nil
}
