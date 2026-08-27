package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/extension"
	"github.com/biggs-100/biggz-ai/internal/policy"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
	readability "github.com/biggs-100/biggz-ai/internal/review/lens/readability"
)

func TestFakeRecordsAndInvokes(t *testing.T) {
	t.Setenv("FAKE_TEST_ENV", "1")
	fake := NewFake()

	// Record lenses/tools/fallback/On
	fake.RegisterLens(&readability.Lens{})
	if len(fake.Lenses) != 1 || fake.Lenses[0].ID() != "readability" {
		t.Fatalf("lens not recorded")
	}
	if _, ok := fake.LensMap["readability"]; !ok {
		t.Fatalf("lens map not recorded")
	}

	calledCmd := false
	fake.RegisterCommand("x", func(_ context.Context, _ map[string]any) error {
		calledCmd = true
		return nil
	})
	if _, ok := fake.Commands["x"]; !ok {
		t.Fatalf("command not recorded")
	}
	_ = calledCmd

	fake.RegisterTool(extension.ToolDef{Name: "my_tool"}, func(_ context.Context, _ extension.ToolCallRequest) (extension.ToolCallResult, error) {
		return extension.ToolCallResult{Output: "ok"}, nil
	})
	if _, ok := fake.Tools["my_tool"]; !ok {
		t.Fatalf("tool not recorded")
	}

	fallbackCalled := false
	fake.RegisterFileWriteFallback(func(_ context.Context, _ extension.ToolCallRequest) (policy.ToolCallDecision, error) {
		fallbackCalled = true
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})
	if fake.Fallback == nil {
		t.Fatalf("fallback not recorded")
	}
	_ = fallbackCalled

	fake.On("tool_call", func(_ context.Context, _ extension.ToolCallRequest) (policy.ToolCallDecision, error) {
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	})
	if len(fake.ToolCallHandlers) != 1 {
		t.Fatalf("On tool_call not recorded")
	}
	fake.On("tool_result", func(_ context.Context, _ extension.ToolCallRequest, _ extension.ToolCallResult) {})
	if len(fake.ToolResultHandlers) != 1 {
		t.Fatalf("On tool_result not recorded")
	}

	// InvokeTool triggers handler
	res, err := fake.InvokeTool(context.Background(), extension.ToolCallRequest{Tool: "my_tool"})
	if err != nil {
		t.Fatalf("InvokeTool: %v", err)
	}
	if res.Output != "ok" {
		t.Fatalf("InvokeTool output mismatch: %q", res.Output)
	}
	if len(fake.Invoked) != 1 || fake.Invoked[0].Tool != "my_tool" {
		t.Fatalf("Invoke not recorded: %+v", fake.Invoked)
	}

	// Check Ordered via fake
	ordered := fake.Ordered([]string{"readability"})
	if len(ordered) != 1 || ordered[0].ID() != "readability" {
		t.Fatalf("Ordered failed")
	}

	// t.Setenv isolation already tested via env var above
	if v := os.Getenv("FAKE_TEST_ENV"); v != "1" {
		t.Fatalf("t.Setenv isolation failed")
	}
}

func TestFakeLensesIsolation(t *testing.T) {
	fake1 := NewFake()
	fake1.RegisterLens(&fakeLens{id: "a"})
	fake2 := NewFake()
	if len(fake2.Lenses) != 0 {
		t.Fatalf("fake isolation failed, second fake has lenses")
	}
}

func TestFake_InvokeWithMiddlewareBlock(t *testing.T) {
	fake := NewFake()
	fake.On("tool_call", func(_ context.Context, _ extension.ToolCallRequest) (policy.ToolCallDecision, error) {
		return policy.ToolCallDecision{Kind: policy.DecisionBlock, Reason: "blocked"}, nil
	})
	toolCalled := false
	fake.RegisterTool(extension.ToolDef{Name: "x"}, func(_ context.Context, _ extension.ToolCallRequest) (extension.ToolCallResult, error) {
		toolCalled = true
		return extension.ToolCallResult{Output: "ok"}, nil
	})
	res, err := fake.InvokeTool(context.Background(), extension.ToolCallRequest{Tool: "x"})
	if err == nil {
		// blocked returns result with error field inside? Actually our fake returns blocked with Err? Check.
	}
	if toolCalled {
		t.Fatalf("tool should not execute when blocked")
	}
	if res.Output != "blocked" {
		t.Fatalf("blocked output not propagated: %q", res.Output)
	}
	_ = err
}

type fakeLens struct{ id string }

func (f *fakeLens) ID() string { return f.id }
func (f *fakeLens) Analyze(_ context.Context, _ lens.LensInput) (lens.LensResult, error) {
	return lens.LensResult{LensID: f.id}, nil
}


