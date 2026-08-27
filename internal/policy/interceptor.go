package policy

import (
	"context"
	"os"
)

type ToolCallRequest struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args"`
	CallID string         `json:"call_id"`
}
type DecisionKind string

const (
	DecisionAllow  DecisionKind = "allow"
	DecisionBlock  DecisionKind = "block"
	DecisionRevise DecisionKind = "revise"
	DecisionAsk    DecisionKind = "ask"
)

type ToolCallDecision struct {
	Kind        DecisionKind   `json:"kind"`
	Reason      string         `json:"reason,omitempty"`
	RevisedArgs map[string]any `json:"revised_args,omitempty"`
}
type ToolCallResult struct {
	Output string `json:"output"`
	Err    error  `json:"-"`
}

type ToolCallInterceptor interface {
	BeforeToolCall(ctx context.Context, req ToolCallRequest) (ToolCallDecision, error)
	AfterToolCall(ctx context.Context, req ToolCallRequest, res ToolCallResult)
}
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, req ToolCallRequest) (ToolCallDecision, error)
}

type ApprovalMode string
const (
	ApprovalModeAuto ApprovalMode = "auto"
	ApprovalModeAsk  ApprovalMode = "ask"
)
const ConsentSchema = "biggz-ai.review-integration.consent/v3"
type PolicyInterceptor struct {
	Evaluator PolicyEvaluator
	Approval  ApprovalMode
}

func (p *PolicyInterceptor) BeforeToolCall(ctx context.Context, req ToolCallRequest) (ToolCallDecision, error) {
	if p == nil || p.Evaluator == nil {
		return ToolCallDecision{Kind: DecisionAllow}, nil
	}
	dec, err := p.Evaluator.Evaluate(ctx, req)
	if err != nil {
		return ToolCallDecision{}, err
	}
	if dec.Kind == DecisionAsk && p.Approval == ApprovalModeAsk {
		resolved := os.Getenv("BIGGZ_TOOL_CONSENT")
		if resolved == "deny" {
			return ToolCallDecision{Kind: DecisionBlock, Reason: "consent denied"}, nil
		}
		if resolved == "allow" {
			return ToolCallDecision{Kind: DecisionAllow}, nil
		}
		return ToolCallDecision{Kind: DecisionBlock, Reason: "awaiting consent"}, nil
	}
	if dec.Kind == "" {
		return ToolCallDecision{Kind: DecisionAllow}, nil
	}
	return dec, nil
}

func (p *PolicyInterceptor) AfterToolCall(_ context.Context, _ ToolCallRequest, _ ToolCallResult) {
}
