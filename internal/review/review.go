// Package review implements the review transaction system for biggz-ai.
//
// It wraps the core model types (ReviewState, Evidence, FSM) with lifecycle
// methods that coordinate lens execution, finding management, corrections,
// receipt generation, and gate validation.
//
// Design decisions (vs gentle-ai):
// - Single Review struct (no parallel CompactState)
// - Evidence chain as source of truth (individual hashes are derivable)
// - 5 coarse FSM states + external PolicyEvaluator
// - Minimal Correction model (budget/policy is external)
package review

import (
	"context"
	"fmt"
	"time"

	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/pipeline"
)

// Review wraps a ReviewState with lifecycle coordination.
type Review struct {
	State    *model.ReviewState
	Findings []Finding
}

// New creates a new Review with a fresh ReviewState.
func New(subject model.ReviewSubject) *Review {
	return &Review{
		State:    model.NewReviewState(subject),
		Findings: make([]Finding, 0),
	}
}

// Start begins the review by validating the transition to InProgress.
func (r *Review) Start(ctx context.Context) error {
	if err := model.Transition(r.State.Status, model.StatusInProgress); err != nil {
		return err
	}
	r.State.Status = model.StatusInProgress
	r.State.UpdatedAt = time.Now()
	return nil
}

// Complete transitions the review to Completed after successful pipeline execution.
// Computes the MerkleRoot from the evidence chain.
func (r *Review) Complete(ctx context.Context) error {
	if err := model.Transition(r.State.Status, model.StatusCompleted); err != nil {
		return err
	}
	r.State.MerkleRoot = model.MerkleRoot(r.State.Evidence)
	r.State.Status = model.StatusCompleted
	r.State.UpdatedAt = time.Now()
	return nil
}

// Fail transitions the review to Failed status.
func (r *Review) Fail(ctx context.Context, reason error) {
	r.State.MerkleRoot = model.MerkleRoot(r.State.Evidence)
	_ = model.Transition(r.State.Status, model.StatusFailed)
	r.State.Status = model.StatusFailed
	r.State.UpdatedAt = time.Now()
}

// AddCorrection records a correction and reopens the review.
func (r *Review) AddCorrection(correction Correction) error {
	if r.State.Status != model.StatusCompleted {
		return fmt.Errorf("can only add corrections to completed reviews, current: %s", r.State.Status)
	}
	if err := model.Transition(r.State.Status, model.StatusInProgress); err != nil {
		return fmt.Errorf("reopen for correction: %w", err)
	}
	r.State.Status = model.StatusInProgress
	payload := fmt.Sprintf("correction: %s (%d files, %d lines)", correction.Reason, len(correction.Files), correction.LinesChanged)
	r.State.Evidence = model.AppendEvidence(r.State.Evidence, "correction", payload)
	r.State.Corrections = append(r.State.Corrections, model.Correction{
		ID:        correction.ID,
		Field:     correction.Reason,
		OldValue:  correction.BeforeHash,
		NewValue:  correction.AfterHash,
		Reason:    correction.Reason,
		CreatedAt: time.Now(),
	})
	r.State.UpdatedAt = time.Now()
	return nil
}

// RunPipeline executes the pipeline stages against the review's state.
// Coordinates Start → pipeline → Complete (or Fail on error).
func (r *Review) RunPipeline(ctx context.Context, p *pipeline.Pipeline) error {
	if err := r.Start(ctx); err != nil {
		return err
	}
	if err := p.Execute(ctx, r.State); err != nil {
		r.Fail(ctx, err)
		return fmt.Errorf("pipeline: %w", err)
	}
	return r.Complete(ctx)
}

// Receipt generates a review receipt from the current state.
func (r *Review) Receipt() *Receipt {
	return &Receipt{
		ReviewID:   r.State.ID,
		MerkleRoot: r.State.MerkleRoot,
		Status:     r.State.Status,
		Completed:  r.State.UpdatedAt,
	}
}

// ErrNotCompleted is returned when a receipt or gate is requested on a non-completed review.
var ErrNotCompleted = fmt.Errorf("review is not completed")
