package review

// Pi reviewer execute — binary-owned prompt composition and bounded execution.
//
// ComposeReviewerPrompt binds the reviewer role assets and the output
// contract to the frozen task: shared bytes ⊕ lens role bytes ⊕ output
// contract ⊕ separator ⊕ MaterializeReviewerTask, with the task segment
// byte-identical and no channel for a caller-authored body.
// ExecutePiReview runs a ReviewerRunner under a finite deadline and enforces
// the artifact byte cap before Capture; raw stdout reaches Capture unmodified
// and no failure path captures or leaves a partial slot.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

const (
	// DefaultReviewerTimeout bounds one reviewer run when the caller does not
	// choose a timeout.
	DefaultReviewerTimeout = 10 * time.Minute
	// MaxReviewerTimeout is the largest reviewer timeout ExecutePiReview
	// accepts; anything above is refused before a reviewer can launch.
	MaxReviewerTimeout = 2 * time.Hour
)

// reviewerPromptDir is the embedded asset directory of the reviewer roles.
const reviewerPromptDir = "prompts/review"

// ReviewerOutputContractAsset is the embedded binary-owned output contract:
// inserted after the role text and before the materialized task, it states
// the admitted reviewer result shape.
const ReviewerOutputContractAsset = "prompts/review-output-contract.md"

// reviewerPromptSeparator divides the binary-owned role bytes from the
// byte-identical materialized task closing the prompt.
const reviewerPromptSeparator = "\n---\n"

// reviewerRoleAssets maps each executable lens to its role asset under
// reviewerPromptDir, always preceded by shared.md. performance and
// dependencies are valid lenses without a role asset and refuse typed.
var reviewerRoleAssets = map[string]string{
	"risk":        "r1-risk.md",
	"readability": "r2-readability.md",
	"reliability": "r3-reliability.md",
	"resilience":  "r4-resilience.md",
}

// ReviewerRunner executes one reviewer prompt and returns its raw stdout.
// *PiAdapter implements it; the executor never interprets the bytes.
type ReviewerRunner interface {
	Review(ctx context.Context, prompt string) ([]byte, error)
}

// ComposeReviewerPrompt composes the complete binary-owned reviewer prompt:
// the shared role asset, the lens role asset, the output contract, a
// separator, and the materialized task as the byte-identical final segment.
// Caller-authored bodies have no channel through this composition.
func ComposeReviewerPrompt(binding CaptureBinding) ([]byte, error) {
	if err := binding.validate(); err != nil {
		return nil, err
	}
	roleAsset, ok := reviewerRoleAssets[binding.Lens]
	if !ok {
		return nil, &ReviewerFailure{
			Kind: ReviewerFailureRoleUnavailable, Stage: ReviewerStageRole,
			Cause: fmt.Errorf("no reviewer role asset for lens %q", binding.Lens),
		}
	}
	shared, err := assets.FS.ReadFile(reviewerPromptDir + "/shared.md")
	if err != nil {
		return nil, fmt.Errorf("reviewer role asset: %w", err)
	}
	role, err := assets.FS.ReadFile(reviewerPromptDir + "/" + roleAsset)
	if err != nil {
		return nil, fmt.Errorf("reviewer role asset: %w", err)
	}
	contract, err := assets.FS.ReadFile(ReviewerOutputContractAsset)
	if err != nil {
		return nil, fmt.Errorf("reviewer output contract asset: %w", err)
	}
	task, err := MaterializeReviewerTask(binding)
	if err != nil {
		return nil, err
	}
	prompt := make([]byte, 0, len(shared)+1+len(role)+len(contract)+len(reviewerPromptSeparator)+len(task))
	prompt = append(prompt, shared...)
	prompt = append(prompt, '\n')
	prompt = append(prompt, role...)
	prompt = append(prompt, contract...)
	prompt = append(prompt, reviewerPromptSeparator...)
	return append(prompt, task...), nil
}

// ExecutePiReview runs the reviewer for one capture binding under a finite
// timeout and captures its raw stdout through the existing Capture path.
// Every failure — role, transport, empty output, over-cap, deadline or
// cancelation — returns before Capture: nothing captures and no partial slot
// can exist after a refusal.
func ExecutePiReview(ctx context.Context, binding CaptureBinding, timeout time.Duration, runner ReviewerRunner) (CaptureOutcome, error) {
	if runner == nil {
		return CaptureOutcome{}, errors.New("reviewer execute: no reviewer runner configured")
	}
	if timeout <= 0 {
		timeout = DefaultReviewerTimeout
	}
	if timeout > MaxReviewerTimeout {
		return CaptureOutcome{}, fmt.Errorf("reviewer execute: timeout %s exceeds the %s maximum", timeout, MaxReviewerTimeout)
	}
	prompt, err := ComposeReviewerPrompt(binding)
	if err != nil {
		return CaptureOutcome{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	raw, err := runner.Review(runCtx, string(prompt))
	if err != nil {
		return CaptureOutcome{}, classifyReviewerRunError(runCtx, err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return CaptureOutcome{}, &ReviewerFailure{
			Kind: ReviewerFailureEmptyOutput, Stage: ReviewerStagePi, StdoutBytes: len(raw),
		}
	}
	if len(raw) > ArtifactResultLimit {
		return CaptureOutcome{}, &ReviewerFailure{
			Kind: ReviewerFailureOutputOverCap, Stage: ReviewerStageOutput, StdoutBytes: len(raw),
		}
	}
	return Capture(binding, raw)
}

// classifyReviewerRunError types a reviewer run error: failures already typed
// by the runner are preserved, and untyped errors are classified against the
// run context so timeout and cancelation stay distinguishable.
func classifyReviewerRunError(ctx context.Context, err error) error {
	var failure *ReviewerFailure
	if errors.As(err, &failure) {
		return failure
	}
	classified := &ReviewerFailure{Kind: ReviewerFailureLaunch, Stage: ReviewerStagePi, Cause: err}
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		classified.Kind = ReviewerFailureTimeout
	case errors.Is(ctx.Err(), context.Canceled):
		classified.Kind = ReviewerFailureCanceled
	}
	return classified
}
