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
	"time"
)

// piReviewerWaitDelay forcibly releases Wait shortly after a context kill so
// a grandchild holding the inherited stdout pipe cannot outlive the deadline.
const piReviewerWaitDelay = 5 * time.Second

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
	lookPath := a.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	binary, err := lookPath("pi")
	if err != nil {
		return nil, fmt.Errorf("pi reviewer transport unavailable: %w", err)
	}
	scratch, err := os.MkdirTemp("", "biggz-pi-review-*")
	if err != nil {
		return nil, fmt.Errorf("pi reviewer transport unavailable: create scratch directory: %w", err)
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
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, fmt.Errorf("pi reviewer transport failed: %w: %s", err, stderr.String())
	}
	if len(bytes.TrimSpace(stdout.Bytes())) == 0 {
		return nil, errors.New("pi reviewer transport produced no final message")
	}
	return stdout.Bytes(), nil
}
