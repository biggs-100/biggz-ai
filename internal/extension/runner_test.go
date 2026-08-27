package extension

import (
	"context"
	"os"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/policy"
)

type mockEval func(context.Context, policy.ToolCallRequest) (policy.ToolCallDecision, error)

func (m mockEval) Evaluate(ctx context.Context, req policy.ToolCallRequest) (policy.ToolCallDecision, error) {
	return m(ctx, req)
}

func TestRunner_BlocksInjectedBash(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	eval := mockEval(func(_ context.Context, req policy.ToolCallRequest) (policy.ToolCallDecision, error) {
		if req.Tool == "user_bash" {
			if cmd, ok := req.Args["command"].(string); ok {
				if contains(cmd, "rm -rf") || contains(cmd, "mkfs") || contains(cmd, ":(){:|:&};:") {
					return policy.ToolCallDecision{Kind: policy.DecisionBlock, Reason: "blocked by policy"}, nil
				}
			}
		}
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})
	pi := &policy.PolicyInterceptor{Evaluator: eval, Approval: policy.ApprovalModeAuto}
	// sanity: direct interceptor should block
	directReq := policy.ToolCallRequest{Tool: "user_bash", Args: map[string]any{"command": "rm -rf /tmp/x"}}
	directDec, _ := pi.BeforeToolCall(context.Background(), directReq)
	t.Logf("direct dec=%q PI_SUBAGENT_CHILD=%q", directDec.Kind, getPIEnv())
	r := &Runner{API: New(), Interceptor: pi}

	cases := []string{"rm -rf /tmp/x", "mkfs /dev/sda", ":(){:|:&};:"}
	for _, c := range cases {
		req := policy.ToolCallRequest{Tool: "user_bash", Args: map[string]any{"command": c}}
		dec, err := r.Before(context.Background(), req)
		if err != nil {
			t.Fatalf("Before %q: %v", c, err)
		}
		t.Logf("runner dec for %q = %q", c, dec.Kind)
		if dec.Kind != policy.DecisionBlock {
			t.Fatalf("injection %q should block, got %q", c, dec.Kind)
		}
	}
}

func TestRunner_Revise(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	revised := map[string]any{"command": "echo safe"}
	eval := mockEval(func(_ context.Context, req policy.ToolCallRequest) (policy.ToolCallDecision, error) {
		return policy.ToolCallDecision{Kind: policy.DecisionRevise, RevisedArgs: revised, Reason: "sanitized"}, nil
	})
	pi := &policy.PolicyInterceptor{Evaluator: eval}
	r := &Runner{API: New(), Interceptor: pi}
	req := policy.ToolCallRequest{Tool: "user_bash", Args: map[string]any{"command": "rm -rf /"}}
	dec, err := r.Before(context.Background(), req)
	if err != nil {
		t.Fatalf("Before: %v", err)
	}
	if dec.Kind != policy.DecisionRevise {
		t.Fatalf("want revise, got %q", dec.Kind)
	}
	if dec.RevisedArgs["command"] != "echo safe" {
		t.Fatalf("revised args not propagated: %+v", dec.RevisedArgs)
	}
	if req.Args["command"] != "rm -rf /" {
		t.Fatalf("original must be preserved")
	}
}

func TestRunner_ConsentDeny(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	eval := mockEval(func(_ context.Context, _ policy.ToolCallRequest) (policy.ToolCallDecision, error) {
		return policy.ToolCallDecision{Kind: policy.DecisionAsk, Reason: "needs approval"}, nil
	})
	pi := &policy.PolicyInterceptor{Evaluator: eval, Approval: policy.ApprovalModeAsk}
	r := &Runner{API: New(), Interceptor: pi}
	t.Setenv("BIGGZ_TOOL_CONSENT", "deny")
	dec, err := r.Before(context.Background(), policy.ToolCallRequest{Tool: "user_bash"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dec.Kind != policy.DecisionBlock {
		t.Fatalf("deny should block, got %q", dec.Kind)
	}
	if dec.Reason != "consent denied" {
		t.Fatalf("reason mismatch: %q", dec.Reason)
	}
}

func TestRunner_ConsentAllow(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	eval := mockEval(func(_ context.Context, _ policy.ToolCallRequest) (policy.ToolCallDecision, error) {
		return policy.ToolCallDecision{Kind: policy.DecisionAsk, Reason: "needs approval"}, nil
	})
	pi := &policy.PolicyInterceptor{Evaluator: eval, Approval: policy.ApprovalModeAsk}
	r := &Runner{API: New(), Interceptor: pi}
	t.Setenv("BIGGZ_TOOL_CONSENT", "allow")
	dec, err := r.Before(context.Background(), policy.ToolCallRequest{Tool: "user_bash"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dec.Kind != policy.DecisionAllow {
		t.Fatalf("allow should proceed, got %q", dec.Kind)
	}
}

func TestRunner_ToolResultNoMutate(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	api := New()
	// Register a tool_result handler that tries to mutate
	api.On("tool_result", func(_ context.Context, _ ToolCallRequest, res ToolCallResult) {
		res.Output = "mutated"
	})
	pi := &policy.PolicyInterceptor{Evaluator: mockEval(func(_ context.Context, _ policy.ToolCallRequest) (policy.ToolCallDecision, error) {
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})}
	r := &Runner{API: api, Interceptor: pi}
	req := policy.ToolCallRequest{Tool: "user_bash", Args: map[string]any{"command": "echo hi"}}
	res := policy.ToolCallResult{Output: "hi"}
	r.After(context.Background(), req, res)
	if res.Output != "hi" {
		t.Fatalf("After mutated result: %q", res.Output)
	}
	// Also test with error
	res2 := policy.ToolCallResult{Output: "hi", Err: errSentinel("fail")}
	r.After(context.Background(), req, res2)
	if res2.Output != "hi" {
		t.Fatalf("After mutated result2")
	}
}

func TestRunner_Fallback(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	api := New()
	called := false
	api.RegisterFileWriteFallback(func(_ context.Context, req ToolCallRequest) (policy.ToolCallDecision, error) {
		called = true
		if req.Tool != "file_write" {
			t.Errorf("fallback tool mismatch: %q", req.Tool)
		}
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})
	pi := &policy.PolicyInterceptor{Evaluator: mockEval(func(_ context.Context, _ policy.ToolCallRequest) (policy.ToolCallDecision, error) {
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})}
	r := &Runner{API: api, Interceptor: pi}
	dec, err := r.Before(context.Background(), policy.ToolCallRequest{Tool: "file_write", Args: map[string]any{"path": "a.txt"}})
	if err != nil {
		t.Fatalf("Before: %v", err)
	}
	if !called {
		t.Fatalf("fallback not called")
	}
	if dec.Kind != policy.DecisionAllow {
		t.Fatalf("fallback allow expected, got %q", dec.Kind)
	}
}

func TestRunner_SubagentBypass(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "1")
	eval := mockEval(func(_ context.Context, _ policy.ToolCallRequest) (policy.ToolCallDecision, error) {
		return policy.ToolCallDecision{Kind: policy.DecisionBlock, Reason: "should be bypassed"}, nil
	})
	pi := &policy.PolicyInterceptor{Evaluator: eval, Approval: policy.ApprovalModeAsk}
	r := &Runner{API: New(), Interceptor: pi}
	t.Setenv("BIGGZ_TOOL_CONSENT", "deny")
	dec, err := r.Before(context.Background(), policy.ToolCallRequest{Tool: "user_bash", Args: map[string]any{"command": "rm -rf /"}})
	if err != nil {
		t.Fatalf("Before: %v", err)
	}
	if dec.Kind != policy.DecisionAllow {
		t.Fatalf("subagent bypass should allow, got %q", dec.Kind)
	}
	// Also check fallback bypass
	api := New()
	api.RegisterFileWriteFallback(func(_ context.Context, _ ToolCallRequest) (policy.ToolCallDecision, error) {
		t.Fatalf("fallback should be bypassed in subagent child")
		return policy.ToolCallDecision{Kind: policy.DecisionBlock}, nil
	})
	r2 := &Runner{API: api, Interceptor: pi}
	dec2, _ := r2.Before(context.Background(), policy.ToolCallRequest{Tool: "file_write"})
	if dec2.Kind != policy.DecisionAllow {
		t.Fatalf("subagent bypass fallback: got %q", dec2.Kind)
	}
}

func TestRunner_BeforeAllow(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	eval := mockEval(func(_ context.Context, _ policy.ToolCallRequest) (policy.ToolCallDecision, error) {
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})
	pi := &policy.PolicyInterceptor{Evaluator: eval}
	r := &Runner{API: New(), Interceptor: pi}
	dec, err := r.Before(context.Background(), policy.ToolCallRequest{Tool: "file_write"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dec.Kind != policy.DecisionAllow {
		t.Fatalf("want allow, got %q", dec.Kind)
	}
}

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

func contains(s, substr string) bool { return len(s) >= len(substr) && indexOf(s, substr) >= 0 }

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func getPIEnv() string { return os.Getenv("PI_SUBAGENT_CHILD") }

var errSentinel2 = errSentinelRunner("tool failed")

type errSentinelRunner string

func (e errSentinelRunner) Error() string { return string(e) }
