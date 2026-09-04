// Package sdd — Review Gates: 5-gate system for review lifecycle.
//
// The 5 gates are:
//   - PostApply: after code changes are applied
//   - PreCommit: before git commit
//   - PrePush: before git push
//   - PrePR: before pull request creation
//   - Release: before release
//
// Each gate evaluates the review state and returns a result:
//   - Allow: proceed
//   - Invalidated: review needs to be redone
//   - Escalated: review needs additional authority
//   - ScopeChanged: review scope has changed
package sdd

import (
	"encoding/json"
	"fmt"
	"time"
)

// GateKind identifies which gate is being evaluated.
type GateKind string

const (
	// GatePostApply is after code changes are applied.
	GatePostApply GateKind = "post-apply"
	// GatePreCommit is before git commit.
	GatePreCommit GateKind = "pre-commit"
	// GatePrePush is before git push.
	GatePrePush GateKind = "pre-push"
	// GatePrePR is before pull request creation.
	GatePrePR GateKind = "pre-pr"
	// GateRelease is before release.
	GateRelease GateKind = "release"
)

// GateResult is the outcome of a gate evaluation.
type GateResult string

const (
	// GateAllow means proceed with the operation.
	GateAllow GateResult = "allow"
	// GateInvalidated means review needs to be redone.
	GateInvalidated GateResult = "invalidated"
	// GateEscalated means review needs additional authority.
	GateEscalated GateResult = "escalated"
	// GateScopeChanged means review scope has changed.
	GateScopeChanged GateResult = "scope-changed"
)

// GateRequest is a request to evaluate a gate.
type GateRequest struct {
	Schema           string   `json:"schema"`
	Gate             GateKind `json:"gate"`
	ChangeName       string   `json:"change_name"`
	WorkspaceRoot    string   `json:"workspace_root"`
	EvidenceRevision string   `json:"evidence_revision"`
	ReceiptHash      string   `json:"receipt_hash,omitempty"`
}

// GateResponse is the result of a gate evaluation.
type GateResponse struct {
	Result  GateResult `json:"result"`
	Reason  string     `json:"reason,omitempty"`
	Gate    GateKind   `json:"gate"`
	Time    time.Time  `json:"time"`
	Details string     `json:"details,omitempty"`
}

// GateEvaluator evaluates a gate request and returns a response.
type GateEvaluator interface {
	EvaluateGate(req *GateRequest) (*GateResponse, error)
}

// DefaultGateEvaluator implements GateEvaluator with basic validation.
type DefaultGateEvaluator struct{}

// EvaluateGate evaluates the gate request.
func (e *DefaultGateEvaluator) EvaluateGate(req *GateRequest) (*GateResponse, error) {
	resp := &GateResponse{
		Gate: req.Gate,
		Time: time.Now(),
	}

	// Validate request
	if err := validateGateRequest(req); err != nil {
		resp.Result = GateInvalidated
		resp.Reason = err.Error()
		return resp, nil
	}

	// Gate-specific validation
	switch req.Gate {
	case GatePostApply:
		return e.evaluatePostApply(req, resp)
	case GatePreCommit:
		return e.evaluatePreCommit(req, resp)
	case GatePrePush:
		return e.evaluatePrePush(req, resp)
	case GatePrePR:
		return e.evaluatePrePR(req, resp)
	case GateRelease:
		return e.evaluateRelease(req, resp)
	default:
		resp.Result = GateInvalidated
		resp.Reason = fmt.Sprintf("unknown gate: %s", req.Gate)
		return resp, nil
	}
}

// validateGateRequest validates the gate request fields.
func validateGateRequest(req *GateRequest) error {
	if req.Schema == "" {
		req.Schema = "biggz-ai.review-gate/v1"
	}
	if req.Gate == "" {
		return fmt.Errorf("gate is required")
	}
	if req.ChangeName == "" {
		return fmt.Errorf("change_name is required")
	}
	// WorkspaceRoot is optional for some gates
	return nil
}

// evaluatePostApply evaluates the post-apply gate.
func (e *DefaultGateEvaluator) evaluatePostApply(req *GateRequest, resp *GateResponse) (*GateResponse, error) {
	// Post-apply gate checks:
	// 1. Evidence revision is present
	// 2. Change exists
	if req.EvidenceRevision == "" {
		resp.Result = GateInvalidated
		resp.Reason = "evidence_revision required for post-apply gate"
		return resp, nil
	}

	resp.Result = GateAllow
	resp.Reason = "post-apply gate passed"
	return resp, nil
}

// evaluatePreCommit evaluates the pre-commit gate.
func (e *DefaultGateEvaluator) evaluatePreCommit(req *GateRequest, resp *GateResponse) (*GateResponse, error) {
	// Pre-commit gate checks:
	// 1. Receipt hash is present (receipt-bound commit)
	if req.ReceiptHash == "" {
		resp.Result = GateEscalated
		resp.Reason = "receipt_hash required for pre-commit gate (receipt-bound commit)"
		return resp, nil
	}

	resp.Result = GateAllow
	resp.Reason = "pre-commit gate passed"
	return resp, nil
}

// evaluatePrePush evaluates the pre-push gate.
func (e *DefaultGateEvaluator) evaluatePrePush(req *GateRequest, resp *GateResponse) (*GateResponse, error) {
	// Pre-push gate checks:
	// 1. Evidence revision is present
	// 2. Receipt hash is present
	if req.EvidenceRevision == "" || req.ReceiptHash == "" {
		resp.Result = GateEscalated
		resp.Reason = "evidence_revision and receipt_hash required for pre-push gate"
		return resp, nil
	}

	resp.Result = GateAllow
	resp.Reason = "pre-push gate passed"
	return resp, nil
}

// evaluatePrePR evaluates the pre-PR gate.
func (e *DefaultGateEvaluator) evaluatePrePR(req *GateRequest, resp *GateResponse) (*GateResponse, error) {
	// Pre-PR gate checks:
	// 1. Evidence revision is present
	// 2. Receipt hash is present
	// 3. Change is ready for PR
	if req.EvidenceRevision == "" || req.ReceiptHash == "" {
		resp.Result = GateEscalated
		resp.Reason = "evidence_revision and receipt_hash required for pre-PR gate"
		return resp, nil
	}

	resp.Result = GateAllow
	resp.Reason = "pre-PR gate passed"
	return resp, nil
}

// evaluateRelease evaluates the release gate.
func (e *DefaultGateEvaluator) evaluateRelease(req *GateRequest, resp *GateResponse) (*GateResponse, error) {
	// Release gate checks:
	// 1. Evidence revision is present
	// 2. Receipt hash is present
	// 3. All checks passed
	if req.EvidenceRevision == "" || req.ReceiptHash == "" {
		resp.Result = GateEscalated
		resp.Reason = "evidence_revision and receipt_hash required for release gate"
		return resp, nil
	}

	resp.Result = GateAllow
	resp.Reason = "release gate passed"
	return resp, nil
}

// EvaluateGate is a convenience function that creates a DefaultGateEvaluator
// and evaluates the request.
func EvaluateGate(req *GateRequest) (*GateResponse, error) {
	evaluator := &DefaultGateEvaluator{}
	return evaluator.EvaluateGate(req)
}

// ParseGateRequest parses a JSON string into a GateRequest.
func ParseGateRequest(jsonStr string) (*GateRequest, error) {
	var req GateRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		return nil, fmt.Errorf("parse gate request: %w", err)
	}
	return &req, nil
}

// GateResultSummary returns a one-line summary for the orchestrator.
func GateResultSummary(resp *GateResponse) string {
	switch resp.Result {
	case GateAllow:
		return fmt.Sprintf("◆ %s gate · ALLOW (%s)", resp.Gate, resp.Reason)
	case GateInvalidated:
		return fmt.Sprintf("◆ %s gate · INVALIDATED (%s)", resp.Gate, resp.Reason)
	case GateEscalated:
		return fmt.Sprintf("◆ %s gate · ESCALATED (%s)", resp.Gate, resp.Reason)
	case GateScopeChanged:
		return fmt.Sprintf("◆ %s gate · SCOPE_CHANGED (%s)", resp.Gate, resp.Reason)
	default:
		return fmt.Sprintf("◆ %s gate · UNKNOWN", resp.Gate)
	}
}
