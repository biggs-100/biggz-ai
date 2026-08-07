// Package review implements the review transaction system for biggz-ai.
//
// It wraps the core model types (ReviewState, Evidence, FSM) with lifecycle
// methods that coordinate lens execution, finding management, corrections,
// receipt generation, and gate validation.
//
// The lifecycle uses the 13-state FSM with role-based guard table and budget
// counters. Every transition is optionally persisted to the content-addressed
// event store for chain-level integrity.
package review

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/pipeline"
)

// Review wraps a ReviewState with lifecycle coordination.
// When a Store is attached, every lifecycle transition persists an event.
type Review struct {
	State     *model.ReviewState
	Findings  []Finding
	store     *Store
	fsm       model.FSM
	startPlan *StartEventPayload
}

// New creates a new Review in Unreviewed state with a fresh ReviewState.
func New(subject model.ReviewSubject) *Review {
	state := model.NewReviewState(subject)
	state.Status = model.StatusUnreviewed
	return &Review{
		State:    state,
		Findings: make([]Finding, 0),
		fsm:      model.FSM{},
	}
}

// WithStore attaches an event store for persistence.
// Returns the Review for chaining.
func (r *Review) WithStore(store *Store) *Review {
	r.store = store
	return r
}

// FreezeStartPlan attaches the derived start plan (correction budget, base
// ref, lens selection) to the genesis event payload. Returns the Review for
// chaining.
func (r *Review) FreezeStartPlan(plan StartEventPayload) *Review {
	r.startPlan = &plan
	return r
}

// ---------------------------------------------------------------------------
// Event persistence helpers
// ---------------------------------------------------------------------------

// appendEvent serialises and persists a transition event to the store.
// Returns the new revision hash, or empty string if no store is attached.
func (r *Review) appendEvent(prevRev, operation string) (string, error) {
	if r.store == nil {
		return "", nil
	}
	rec := Record{
		Operation: operation,
		Role:      string(r.State.Role),
		Actor:     string(r.State.Role),
		Timestamp: time.Now().Format(time.RFC3339Nano),
	}
	// Attach subject as payload for genesis events.
	if operation == "start_review" {
		var payload []byte
		if r.startPlan != nil {
			payload, _ = json.Marshal(r.startPlan)
		} else {
			payload, _ = json.Marshal(r.State.Subject)
		}
		rec.Payload = payload
	}
	// Attach MerkleRoot for completion events.
	if operation == "complete_review" {
		payload, _ := json.Marshal(map[string]string{
			"merkle_root": r.State.MerkleRoot,
		})
		rec.Payload = payload
	}
	return r.store.Append(prevRev, rec)
}

// currentEventRev returns the current head revision from the store.
func (r *Review) currentEventRev() string {
	if r.store == nil {
		return ""
	}
	chain, err := r.store.LoadChain()
	if err != nil || chain.Count == 0 {
		return ""
	}
	return chain.HeadHash
}

// ---------------------------------------------------------------------------
// Lifecycle methods
// ---------------------------------------------------------------------------

// Start begins the review: Unreviewed → InReview.
// Appends a genesis event and an in_review event if a store is attached.
func (r *Review) Start(ctx context.Context) error {
	if err := r.fsm.Transition(r.State.Status, model.StatusInReview, r.State.Role, r.State.BudgetCounters); err != nil {
		return err
	}

	if r.store != nil {
		// Append genesis event.
		rev, err := r.appendEvent("", "start_review")
		if err != nil {
			return fmt.Errorf("start: append genesis: %w", err)
		}
		if r.State.LineageID == "" {
			r.State.LineageID = r.store.LineageID
		}
		// Append InReview event.
		if _, err := r.appendEvent(rev, "in_review"); err != nil {
			return fmt.Errorf("start: append in_review: %w", err)
		}
	}

	r.State.Status = model.StatusInReview
	r.State.UpdatedAt = time.Now()
	return nil
}

// Complete transitions the review InReview → Completed.
// Computes the MerkleRoot from the evidence chain and appends a
// complete_review event if a store is attached.
func (r *Review) Complete(ctx context.Context) error {
	if err := r.fsm.Transition(r.State.Status, model.StatusCompleted, r.State.Role, r.State.BudgetCounters); err != nil {
		return err
	}

	r.State.MerkleRoot = model.MerkleRoot(r.State.Evidence)
	r.State.Status = model.StatusCompleted
	r.State.UpdatedAt = time.Now()

	if r.store != nil {
		rev := r.currentEventRev()
		if _, err := r.appendEvent(rev, "complete_review"); err != nil {
			return fmt.Errorf("complete: append event: %w", err)
		}
	}

	return nil
}

// Fail transitions the review to Failed status (exceptional path).
func (r *Review) Fail(ctx context.Context, reason error) {
	r.State.MerkleRoot = model.MerkleRoot(r.State.Evidence)
	r.State.Status = model.StatusFailed
	r.State.UpdatedAt = time.Now()
}

// AddCorrection records a correction and transitions InReview → NeedsChanges.
// Checks budget counters via ValidateCorrectionBudget before allowing.
func (r *Review) AddCorrection(correction Correction) error {
	// Allow from InReview (new FSM) or Completed (legacy compat).
	if r.State.Status != model.StatusCompleted && r.State.Status != model.StatusInReview {
		return fmt.Errorf("can only add corrections to completed or in-review reviews, current: %s", r.State.Status)
	}

	// Check correction budget.
	if err := ValidateCorrectionBudget(r.State.BudgetCounters); err != nil {
		return err
	}

	// Transition via new FSM if starting from InReview.
	if r.State.Status == model.StatusInReview {
		if err := r.fsm.Transition(r.State.Status, model.StatusNeedsChanges, r.State.Role, r.State.BudgetCounters); err != nil {
			return fmt.Errorf("correction transition: %w", err)
		}
		r.State.Status = model.StatusNeedsChanges
	} else {
		// Legacy compat: Completed → NeedsChanges (skip FSM, old flow).
		r.State.Status = model.StatusInProgress
	}

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
	r.State.BudgetCounters = IncrementFixRound(r.State.BudgetCounters)
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

// Receipt generates a review receipt. If a store is attached and has a
// valid chain, it returns a chain-bound receipt via NewReceipt. Otherwise
// falls back to a state-based receipt via GenerateReceipt.
func (r *Review) Receipt() *Receipt {
	if r.store != nil {
		chain, err := r.store.LoadChain()
		if err == nil && chain.Count > 0 {
			receipt := NewReceipt(chain)
			return &receipt
		}
	}
	return GenerateReceipt(r.State)
}

// ErrNotCompleted is returned when a receipt or gate is requested on a
// non-completed review.
var ErrNotCompleted = fmt.Errorf("review is not completed")
