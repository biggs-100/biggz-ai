// Package pipeline provides a staged execution model with rollback support.
//
// The Pipeline executes registered Stage implementations sequentially. If any
// stage fails, the Pipeline rolls back all previously completed stages in
// reverse order before returning the error.
package pipeline

import (
	"context"
	"fmt"

	"github.com/biggz-ai/biggz/model"
)

// Stage defines a single unit of work within a pipeline.
// Each stage has a name, an Execute method for performing work, and a Rollback
// method for undoing the work if a later stage fails.
type Stage interface {
	// Name returns a human-readable name for this stage.
	Name() string

	// Execute runs the stage logic against the given ReviewState.
	// It should return an error if the stage fails.
	Execute(ctx context.Context, state *model.ReviewState) error

	// Rollback undoes any changes made by this stage's Execute.
	// It is only called when a later stage in the pipeline fails.
	Rollback(ctx context.Context, state *model.ReviewState) error
}

// Pipeline runs a sequence of stages in order, with automatic rollback on
// failure. A Pipeline with no stages is valid and executes as a no-op.
type Pipeline struct {
	stages []Stage
}

// New creates a Pipeline with the given stages. Stages are executed in the
// order they are provided.
func New(stages ...Stage) *Pipeline {
	return &Pipeline{stages: stages}
}

// Execute runs each stage sequentially. If any stage returns an error, all
// previously completed stages are rolled back in reverse order, and the
// originating error is wrapped and returned. Stages that did not execute
// are not rolled back.
func (p *Pipeline) Execute(ctx context.Context, state *model.ReviewState) error {
	var completed []Stage
	for _, stage := range p.stages {
		if err := stage.Execute(ctx, state); err != nil {
			// Rollback the failed stage first, then all previously
			// completed stages in reverse execution order.
			stage.Rollback(ctx, state)
			for i := len(completed) - 1; i >= 0; i-- {
				completed[i].Rollback(ctx, state)
			}
			return fmt.Errorf("stage %s failed: %w", stage.Name(), err)
		}
		completed = append(completed, stage)
	}
	return nil
}
