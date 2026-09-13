// Package review — PiAdapter for immutable review.
//
// PiAdapter invokes a brand-new print-mode pi process with an opaque prompt
// and returns its raw final bytes without interpreting them. The process runs
// in an empty temporary scratch directory with every discovery surface disabled
// so the reviewer sees only the Go-issued bytes, exactly like gentle's
// reviewerprovider.PiAdapter. Go keeps prompt materialization, admission,
// budgets, receipts and gates.
package review

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// piReviewerWaitDelay forcibly releases Wait shortly after a context kill so
// a grandchild holding the inherited stdout pipe cannot outlive the deadline.
const piReviewerWaitDelay = 5 * time.Second

// PiReviewRelayExecutableEnv names the absolute-path override for the pi
// reviewer executable. The override is executed directly and is never routed
// through a shell; a non-absolute value refuses typed.
const PiReviewRelayExecutableEnv = "BIGGZ_PI_REVIEW_RELAY_EXECUTABLE"

// ReviewerFailureKind names the failing stage of one reviewer execution so
// callers never parse prose.
type ReviewerFailureKind string

const (
	ReviewerFailureLaunch          ReviewerFailureKind = "launch"           // the transport could not launch
	ReviewerFailureTimeout         ReviewerFailureKind = "timeout"          // the run hit its deadline
	ReviewerFailureCanceled        ReviewerFailureKind = "canceled"         // the caller canceled the run
	ReviewerFailureEmptyOutput     ReviewerFailureKind = "empty-output"     // no stdout bytes
	ReviewerFailureNonzeroExit     ReviewerFailureKind = "nonzero-exit"     // the reviewer exited non-zero
	ReviewerFailureOutputOverCap   ReviewerFailureKind = "output-over-cap"  // stdout exceeds ArtifactResultLimit
	ReviewerFailureRoleUnavailable ReviewerFailureKind = "role-unavailable" // no reviewer role asset
)

// ReviewerStage names where a ReviewerFailure happened.
type ReviewerStage string

const (
	ReviewerStagePi     ReviewerStage = "pi"     // pi transport
	ReviewerStageRole   ReviewerStage = "role"   // binary-owned role/prompt
	ReviewerStageOutput ReviewerStage = "output" // raw-stdout budget
)

// ReviewerFailure is the typed reviewer execution failure. Error preserves the
// legacy transport texts; no failure carries captured bytes and no failure
// path captures or leaves a partial slot.
type ReviewerFailure struct {
	Kind        ReviewerFailureKind
	Stage       ReviewerStage
	Elapsed     time.Duration
	ExitCode    int
	StdoutBytes int
	StderrBytes int
	Cause       error
}

// Error renders the typed failure while preserving the legacy transport texts.
func (f *ReviewerFailure) Error() string {
	message := "pi reviewer transport failed"
	switch f.Kind {
	case ReviewerFailureLaunch:
		message = "pi reviewer transport unavailable"
	case ReviewerFailureEmptyOutput:
		message = "pi reviewer transport produced no final message"
	case ReviewerFailureOutputOverCap:
		message = fmt.Sprintf("pi reviewer output is %d bytes, exceeding the %d-byte artifact cap (refused whole; no capture was performed)", f.StdoutBytes, ArtifactResultLimit)
	case ReviewerFailureRoleUnavailable:
		message = "pi reviewer role asset is unavailable"
	}
	if f.Cause != nil {
		message += ": " + f.Cause.Error()
	}
	return message
}

// Unwrap exposes the underlying cause for errors.Is and errors.As.
func (f *ReviewerFailure) Unwrap() error { return f.Cause }

// PiAdapter invokes a brand-new print-mode pi process with an opaque prompt
// and returns its raw final bytes without interpreting them.
type PiAdapter struct {
	LookPath       func(string) (string, error)
	CommandContext func(context.Context, string, ...string) *exec.Cmd
}

// NewPiAdapter returns an adapter using the pi binary resolved from PATH.
func NewPiAdapter() *PiAdapter {
	return &PiAdapter{LookPath: exec.LookPath, CommandContext: exec.CommandContext}
}

// Review runs pi in an empty temporary directory with every discovery surface
// disabled. The prompt is delivered through stdin so command arguments never
// carry provider material. It matches gentle's flags byte-for-byte:
// --print --mode text --no-session --no-tools --no-extensions --no-skills
// --no-prompt-templates --no-themes --no-context-files --no-approve
func (a *PiAdapter) Review(ctx context.Context, prompt string) ([]byte, error) {
	started := time.Now()
	binary, err := a.resolveBinary()
	if err != nil {
		return nil, &ReviewerFailure{Kind: ReviewerFailureLaunch, Stage: ReviewerStagePi, Cause: err}
	}
	scratch, err := os.MkdirTemp("", "biggz-pi-review-*")
	if err != nil {
		return nil, &ReviewerFailure{Kind: ReviewerFailureLaunch, Stage: ReviewerStagePi,
			Cause: fmt.Errorf("create scratch directory: %w", err)}
	}
	defer os.RemoveAll(scratch)

	commandContext := a.CommandContext
	if commandContext == nil {
		commandContext = exec.CommandContext
	}
	cmd := commandContext(ctx, binary,
		"--print", "--mode", "text", "--no-session", "--no-tools", "--no-extensions",
		"--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve")
	cmd.Dir = scratch
	cmd.WaitDelay = piReviewerWaitDelay
	cmd.Stdin = bytes.NewReader([]byte(prompt))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		failure := &ReviewerFailure{Kind: ReviewerFailureNonzeroExit, Stage: ReviewerStagePi,
			Elapsed: time.Since(started), StdoutBytes: stdout.Len(), StderrBytes: stderr.Len()}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			failure.ExitCode = exitErr.ExitCode()
		}
		cause := err
		if ctxErr := ctx.Err(); ctxErr != nil {
			cause = ctxErr
			switch {
			case errors.Is(ctxErr, context.DeadlineExceeded):
				failure.Kind = ReviewerFailureTimeout
			case errors.Is(ctxErr, context.Canceled):
				failure.Kind = ReviewerFailureCanceled
			}
		}
		failure.Cause = fmt.Errorf("%w: %s", cause, stderr.String())
		return nil, failure
	}
	if len(bytes.TrimSpace(stdout.Bytes())) == 0 {
		return nil, &ReviewerFailure{Kind: ReviewerFailureEmptyOutput, Stage: ReviewerStagePi,
			Elapsed: time.Since(started), StdoutBytes: stdout.Len(), StderrBytes: stderr.Len()}
	}
	return stdout.Bytes(), nil
}

// resolveBinary picks the reviewer executable: the absolute-path
// BIGGZ_PI_REVIEW_RELAY_EXECUTABLE override when declared — executed directly,
// never through a shell — otherwise `pi` resolved through PATH.
func (a *PiAdapter) resolveBinary() (string, error) {
	override := strings.TrimSpace(os.Getenv(PiReviewRelayExecutableEnv))
	if override != "" {
		if !filepath.IsAbs(override) {
			return "", fmt.Errorf("%s must name an absolute path", PiReviewRelayExecutableEnv)
		}
		return override, nil
	}
	lookPath := a.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	return lookPath("pi")
}
