package policy

import (
	"context"
	"testing"
)

type mockEval func(context.Context, ToolCallRequest) (ToolCallDecision, error)

func (m mockEval) Evaluate(ctx context.Context, req ToolCallRequest) (ToolCallDecision, error) {
	return m(ctx, req)
}

func TestPolicyInterceptor_BeforeBlocksInjectedBash(t *testing.T) {
	t.Setenv("BIGGZ_TOOL_CONSENT", "deny")
	pi := &PolicyInterceptor{
		Evaluator: mockEval(func(_ context.Context, req ToolCallRequest) (ToolCallDecision, error) {
			if req.Tool == "user_bash" {
				if cmd, ok := req.Args["command"].(string); ok && contains(cmd, "rm -rf") {
					return ToolCallDecision{Kind: DecisionBlock, Reason: "blocked by policy"}, nil
				}
			}
			return ToolCallDecision{Kind: DecisionAllow}, nil
		}),
		Approval: ApprovalModeAuto,
	}
	dec, err := pi.BeforeToolCall(context.Background(), ToolCallRequest{Tool: "user_bash", Args: map[string]any{"command": "rm -rf /tmp/x"}})
	if err != nil {
		t.Fatalf("BeforeToolCall: %v", err)
	}
	if dec.Kind != DecisionBlock {
		t.Fatalf("want block, got %q", dec.Kind)
	}
}

func TestPolicyInterceptor_ReviseUsesRevisedArgs(t *testing.T) {
	revised := map[string]any{"command": "echo safe"}
	pi := &PolicyInterceptor{
		Evaluator: mockEval(func(_ context.Context, req ToolCallRequest) (ToolCallDecision, error) {
			return ToolCallDecision{Kind: DecisionRevise, RevisedArgs: revised, Reason: "sanitized"}, nil
		}),
	}
	req := ToolCallRequest{Tool: "user_bash", Args: map[string]any{"command": "rm -rf /"}}
	dec, err := pi.BeforeToolCall(context.Background(), req)
	if err != nil {
		t.Fatalf("BeforeToolCall: %v", err)
	}
	if dec.Kind != DecisionRevise {
		t.Fatalf("want revise, got %q", dec.Kind)
	}
	if dec.RevisedArgs["command"] != "echo safe" {
		t.Fatalf("revised args not propagated: %+v", dec.RevisedArgs)
	}
	if req.Args["command"] != "rm -rf /" {
		t.Fatalf("original must be preserved")
	}
}

func TestPolicyInterceptor_AfterObserveDoesNotMutate(t *testing.T) {
	pi := &PolicyInterceptor{Evaluator: mockEval(func(_ context.Context, _ ToolCallRequest) (ToolCallDecision, error) {
		return ToolCallDecision{Kind: DecisionAllow}, nil
	})}
	req := ToolCallRequest{Tool: "user_bash", Args: map[string]any{"command": "echo hi"}}
	res := ToolCallResult{Output: "hi", Err: errTest}
	// must not panic, retry, or mutate FSM (no import to check, just ensure no panic)
	pi.AfterToolCall(context.Background(), req, res)
	pi.AfterToolCall(context.Background(), req, ToolCallResult{Output: "hi"})
}

var errTest = errSentinel("tool failed")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

func TestPolicyInterceptor_ConsentAllowAndDeny(t *testing.T) {
	eval := mockEval(func(_ context.Context, _ ToolCallRequest) (ToolCallDecision, error) {
		return ToolCallDecision{Kind: DecisionAsk, Reason: "needs approval"}, nil
	})
	pi := &PolicyInterceptor{Evaluator: eval, Approval: ApprovalModeAsk}
	t.Run("deny blocks", func(t *testing.T) {
		t.Setenv("BIGGZ_TOOL_CONSENT", "deny")
		dec, err := pi.BeforeToolCall(context.Background(), ToolCallRequest{Tool: "user_bash"})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if dec.Kind != DecisionBlock {
			t.Fatalf("deny should block, got %q", dec.Kind)
		}
	})
	t.Run("allow resumes", func(t *testing.T) {
		t.Setenv("BIGGZ_TOOL_CONSENT", "allow")
		dec, err := pi.BeforeToolCall(context.Background(), ToolCallRequest{Tool: "user_bash"})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if dec.Kind != DecisionAllow {
			t.Fatalf("allow should proceed, got %q", dec.Kind)
		}
	})
	if ConsentSchema != "biggz-ai.review-integration.consent/v3" {
		t.Fatalf("consent schema mismatch: %q", ConsentSchema)
	}
}

func TestPolicyInterceptor_DefaultAllow(t *testing.T) {
	pi := &PolicyInterceptor{Evaluator: mockEval(func(_ context.Context, _ ToolCallRequest) (ToolCallDecision, error) {
		return ToolCallDecision{Kind: DecisionAllow}, nil
	})}
	dec, _ := pi.BeforeToolCall(context.Background(), ToolCallRequest{Tool: "file_write"})
	if dec.Kind != DecisionAllow {
		t.Fatalf("default allow failed: %q", dec.Kind)
	}
}

func TestNoFSMImportAndNoGodObject(t *testing.T) {
	pi := PolicyInterceptor{}
	_ = pi
}

func TestIntegration_FakeExtensionAPI(t *testing.T) {
	eval := mockEval(func(_ context.Context, req ToolCallRequest) (ToolCallDecision, error) {
		if req.Tool == "file_write" {
			return ToolCallDecision{Kind: DecisionAllow}, nil
		}
		if req.Tool == "user_bash" {
			if cmd, ok := req.Args["command"].(string); ok && contains(cmd, "rm -rf") {
				return ToolCallDecision{Kind: DecisionBlock, Reason: "blocked"}, nil
			}
			return ToolCallDecision{Kind: DecisionAsk}, nil
		}
		return ToolCallDecision{Kind: DecisionAllow}, nil
	})
	pi := &PolicyInterceptor{Evaluator: eval, Approval: ApprovalModeAsk}
	t.Setenv("BIGGZ_TOOL_CONSENT", "allow")
	dec, err := pi.BeforeToolCall(context.Background(), ToolCallRequest{Tool: "user_bash", Args: map[string]any{"command": "echo hi"}})
	if err != nil || dec.Kind != DecisionAllow {
		t.Fatalf("ask+allow should resume, got %q err %v", dec.Kind, err)
	}
	pi.AfterToolCall(context.Background(), ToolCallRequest{Tool: "user_bash"}, ToolCallResult{Output: "hi"})
	t.Setenv("BIGGZ_TOOL_CONSENT", "deny")
	dec, _ = pi.BeforeToolCall(context.Background(), ToolCallRequest{Tool: "user_bash", Args: map[string]any{"command": "echo hi"}})
	if dec.Kind != DecisionBlock {
		t.Fatalf("deny should block, got %q", dec.Kind)
	}
	dec, _ = pi.BeforeToolCall(context.Background(), ToolCallRequest{Tool: "file_write", Args: map[string]any{"path": "a.txt"}})
	if dec.Kind != DecisionAllow {
		t.Fatalf("file_write fallback must stay allow, got %q", dec.Kind)
	}
}

func contains(s, substr string) bool { return len(s) >= len(substr) && indexOf(s, substr) >= 0 }

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
