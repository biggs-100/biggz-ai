package extension

import (
	"context"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/policy"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
	readability "github.com/biggs-100/biggz-ai/internal/review/lens/readability"
	"github.com/biggs-100/biggz-ai/internal/review"
)

// TestAPI covers On order, block/revise short-circuit, tool_result no-mutate.

func TestAPI_OnOrder(t *testing.T) {
	api := New()
	var order []int
	api.On("tool_call", func(_ context.Context, _ ToolCallRequest) (policy.ToolCallDecision, error) {
		order = append(order, 1)
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})
	api.On("tool_call", func(_ context.Context, _ ToolCallRequest) (policy.ToolCallDecision, error) {
		order = append(order, 2)
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})
	api.RegisterTool(ToolDef{Name: "my_tool"}, func(_ context.Context, _ ToolCallRequest) (ToolCallResult, error) {
		return ToolCallResult{Output: "ok"}, nil
	})
	_, err := api.InvokeTool(context.Background(), ToolCallRequest{Tool: "my_tool", Args: map[string]any{}})
	if err != nil {
		t.Fatalf("InvokeTool: %v", err)
	}
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("order mismatch: %v", order)
	}
}

func TestAPI_BlockingMiddlewareShortCircuits(t *testing.T) {
	api := New()
	var secondCalled bool
	api.On("tool_call", func(_ context.Context, _ ToolCallRequest) (policy.ToolCallDecision, error) {
		return policy.ToolCallDecision{Kind: policy.DecisionBlock, Reason: "blocked by first"}, nil
	})
	api.On("tool_call", func(_ context.Context, _ ToolCallRequest) (policy.ToolCallDecision, error) {
		secondCalled = true
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})
	toolExecuted := false
	api.RegisterTool(ToolDef{Name: "my_tool"}, func(_ context.Context, _ ToolCallRequest) (ToolCallResult, error) {
		toolExecuted = true
		return ToolCallResult{Output: "ok"}, nil
	})
	res, err := api.InvokeTool(context.Background(), ToolCallRequest{Tool: "my_tool"})
	if err == nil {
		// InvokeTool returns blocked as result with error; we check second not called.
	}
	if secondCalled {
		t.Fatalf("second handler must not run when first blocks")
	}
	if toolExecuted {
		t.Fatalf("tool must not execute when blocked")
	}
	if res.Output != "blocked by first" {
		t.Fatalf("blocked reason not propagated: %q", res.Output)
	}
}

func TestAPI_ReviseShortCircuits(t *testing.T) {
	api := New()
	var secondCalled bool
	api.On("tool_call", func(_ context.Context, _ ToolCallRequest) (policy.ToolCallDecision, error) {
		return policy.ToolCallDecision{Kind: policy.DecisionRevise, RevisedArgs: map[string]any{"command": "echo safe"}, Reason: "sanitized"}, nil
	})
	api.On("tool_call", func(_ context.Context, _ ToolCallRequest) (policy.ToolCallDecision, error) {
		secondCalled = true
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})
	var gotArgs map[string]any
	api.RegisterTool(ToolDef{Name: "my_tool"}, func(_ context.Context, req ToolCallRequest) (ToolCallResult, error) {
		gotArgs = req.Args
		return ToolCallResult{Output: "ok"}, nil
	})
	_, err := api.InvokeTool(context.Background(), ToolCallRequest{Tool: "my_tool", Args: map[string]any{"command": "rm -rf /"}})
	if err != nil {
		t.Fatalf("InvokeTool revise: %v", err)
	}
	if secondCalled {
		t.Fatalf("second handler must not run when first revises")
	}
	if gotArgs["command"] != "echo safe" {
		t.Fatalf("revised args not propagated: %+v", gotArgs)
	}
}

func TestAPI_ToolResultDoesNotMutate(t *testing.T) {
	api := New()
	api.On("tool_result", func(_ context.Context, _ ToolCallRequest, res ToolCallResult) {
		// try to mutate - but handler receives copy, cannot affect original
		res.Output = "mutated"
	})
	api.RegisterTool(ToolDef{Name: "my_tool"}, func(_ context.Context, _ ToolCallRequest) (ToolCallResult, error) {
		return ToolCallResult{Output: "original"}, nil
	})
	res, err := api.InvokeTool(context.Background(), ToolCallRequest{Tool: "my_tool"})
	if err != nil {
		t.Fatalf("InvokeTool: %v", err)
	}
	if res.Output != "original" {
		t.Fatalf("tool_result handler mutated result: %q", res.Output)
	}
}

func TestAPI_RegisterToolAndInvokeToolRoundTrip(t *testing.T) {
	api := New()
	called := false
	api.RegisterTool(ToolDef{Name: "my_tool", Description: "test"}, func(_ context.Context, req ToolCallRequest) (ToolCallResult, error) {
		called = true
		if req.Tool != "my_tool" {
			t.Errorf("tool name mismatch: %q", req.Tool)
		}
		return ToolCallResult{Output: "hello"}, nil
	})
	res, err := api.InvokeTool(context.Background(), ToolCallRequest{Tool: "my_tool", Args: map[string]any{"x": 1}})
	if err != nil {
		t.Fatalf("InvokeTool: %v", err)
	}
	if !called {
		t.Fatalf("handler not invoked")
	}
	if res.Output != "hello" {
		t.Fatalf("output mismatch: %q", res.Output)
	}
}

func TestAPI_RegisterLensViaExtensionAPI(t *testing.T) {
	lens.ResetRegistry()
	api := New()
	api.RegisterLens(&readability.Lens{})
	ordered := api.Ordered([]string{"readability"})
	if len(ordered) != 1 || ordered[0].ID() != "readability" {
		t.Fatalf("Ordered readability failed: %v", ordered)
	}
	// Analyze must remain pure (no extension import in readability/lens.go)
	in := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"pkg/foo.go"},
			DiffSummary: map[string]int{"pkg/foo.go": 10},
			BaseTree:    "abc",
		},
		Hunks: map[string][]byte{"pkg/foo.go": []byte("package foo\nfunc (\n")},
	}
	result, err := ordered[0].Analyze(context.Background(), in)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Findings) == 0 {
		t.Fatalf("expected deterministic parser finding")
	}
}

func TestAPI_FallbackRegistration(t *testing.T) {
	api := New()
	called := false
	api.RegisterFileWriteFallback(func(_ context.Context, req ToolCallRequest) (policy.ToolCallDecision, error) {
		called = true
		if req.Tool != "file_write" {
			t.Errorf("fallback tool mismatch: %q", req.Tool)
		}
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})
	// Invoke file_write with no tool handler should delegate to fallback
	res, err := api.InvokeTool(context.Background(), ToolCallRequest{Tool: "file_write", Args: map[string]any{"path": "a.txt"}})
	if err != nil {
		t.Fatalf("InvokeTool fallback: %v", err)
	}
	if !called {
		t.Fatalf("fallback not invoked")
	}
	if res.Output != "fallback allow" {
		t.Fatalf("fallback output mismatch: %q", res.Output)
	}
}
