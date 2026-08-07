package review

import (
	"fmt"
	"time"

	"github.com/biggs-100/biggz-ai/model"
)

// Correction represents a single fix applied during a review cycle.
// This is intentionally minimal — budget, retry limits, and policy rules
// are evaluated externally via the FSM guard table and budget counters.
type Correction struct {
	ID           string    `json:"id"`
	Files        []string  `json:"files"`
	LinesChanged int       `json:"lines_changed"`
	Reason       string    `json:"reason"`
	BeforeHash   string    `json:"before_hash"`  // SHA of evidence chain before correction
	AfterHash    string    `json:"after_hash"`   // SHA of evidence chain after correction
	CreatedAt    time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------------
// Correction budget enforcement
// ---------------------------------------------------------------------------
//
// The budget counters (FixRounds, ScopedValidations) are tracked on the
// ReviewState and enforced by the FSM guard table. These functions provide
// direct budget validation for use in lifecycle methods.

// ValidateCorrectionBudget checks whether the fix-rounds budget allows
// a new correction cycle. Returns nil if the budget is not exhausted.
func ValidateCorrectionBudget(counters model.BudgetCounters) error {
	if counters.FixRounds >= model.MaxFixRounds {
		return fmt.Errorf("correction budget exceeded: fix rounds exhausted (%d/%d)",
			counters.FixRounds, model.MaxFixRounds)
	}
	return nil
}

// ValidateReReviewBudget checks whether the scoped-validations budget
// allows a re-review. Returns nil if the budget is not exhausted.
func ValidateReReviewBudget(counters model.BudgetCounters) error {
	if counters.ScopedValidations >= model.MaxScopedValidations {
		return fmt.Errorf("re-review budget exceeded: scoped validations exhausted (%d/%d)",
			counters.ScopedValidations, model.MaxScopedValidations)
	}
	return nil
}

// IncrementFixRound returns updated counters with FixRounds incremented.
func IncrementFixRound(counters model.BudgetCounters) model.BudgetCounters {
	return model.BudgetCounters{
		FixRounds:         counters.FixRounds + 1,
		ScopedValidations: counters.ScopedValidations,
	}
}

// IncrementScopedValidation returns updated counters with ScopedValidations
// incremented by one.
func IncrementScopedValidation(counters model.BudgetCounters) model.BudgetCounters {
	return model.BudgetCounters{
		FixRounds:         counters.FixRounds,
		ScopedValidations: counters.ScopedValidations + 1,
	}
}
