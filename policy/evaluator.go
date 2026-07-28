// Package policy defines the Evaluator interface for checking review
// policies. Evaluators receive a ReviewState and return a PolicyVerdict
// indicating whether the policy passes or fails, along with a reason
// and severity level.
package policy

import (
	"context"

	"github.com/biggz-ai/biggz/model"
)

// Evaluator checks whether a review state satisfies a specific policy.
// Implementations are registered in the pipeline and called during
// policy evaluation stages.
type Evaluator interface {
	// Name returns the name of this policy evaluator (e.g. "minimum-evidence").
	Name() string

	// Evaluate examines the given ReviewState and returns a PolicyVerdict.
	// A nil error indicates the evaluation completed successfully; the
	// verdict itself determines whether the policy passed or failed.
	Evaluate(ctx context.Context, state *model.ReviewState) (*model.PolicyVerdict, error)
}
