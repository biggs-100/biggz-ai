package extension

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestShim_DelegatesRegisterToolToFake(t *testing.T) {
	api := New()
	shim := &Shim{API: api}
	def := ToolDef{Name: "shim_tool", Description: "via shim"}
	called := false
	shim.RegisterTool(def, func(_ context.Context, req ToolCallRequest) (ToolCallResult, error) {
		called = true
		return ToolCallResult{Output: "ok"}, nil
	})
	res, err := api.InvokeTool(context.Background(), ToolCallRequest{Tool: "shim_tool"})
	if err != nil {
		t.Fatalf("InvokeTool: %v", err)
	}
	if !called || res.Output != "ok" {
		t.Fatalf("handler not invoked via shim: called=%v output=%q", called, res.Output)
	}

	// AgentAdapterShim hook → RegisterTool via same API
	adapterShim := &AgentAdapterShim{API: api}
	adapterShim.HookToTool("my_hook", func(_ context.Context, _ ToolCallRequest) (ToolCallResult, error) {
		return ToolCallResult{Output: "hook"}, nil
	})
	res2, err := api.InvokeTool(context.Background(), ToolCallRequest{Tool: "my_hook"})
	if err != nil {
		t.Fatalf("HookToTool Invoke: %v", err)
	}
	if res2.Output != "hook" {
		t.Fatalf("HookToTool not delegated, got %q", res2.Output)
	}
}

func TestShim_DeprecatedAnnotation(t *testing.T) {
	data, err := os.ReadFile("shim.go")
	if err != nil {
		t.Fatalf("read shim.go: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "// Deprecated: use ExtensionAPI") {
		t.Fatalf("shim.go must contain // Deprecated: use ExtensionAPI")
	}
}

func TestShim_NoLensPlugin(t *testing.T) {
	// Check this package and plugin package for LensPlugin
	needle := "type " + "LensPlugin"
	data, err := os.ReadFile("shim.go")
	if err != nil {
		t.Fatalf("read shim.go: %v", err)
	}
	if strings.Contains(string(data), needle) {
		t.Fatalf("shim.go must not contain %s", needle)
	}
	// Also check plugin/interfaces.go
	if b, err := os.ReadFile("../../plugin/interfaces.go"); err == nil {
		if strings.Contains(string(b), needle) {
			t.Fatalf("plugin/interfaces.go must not contain %s", needle)
		}
	}
	// Check internal/extension files via glob
	files := []string{"api.go", "runner.go", "shim.go"}
	for _, f := range files {
		if b, err := os.ReadFile(f); err == nil {
			if strings.Contains(string(b), needle) {
				t.Fatalf("%s contains %s", f, needle)
			}
		}
	}
}

func TestAgentAdapterShim_Deprecated(t *testing.T) {
	data, err := os.ReadFile("shim.go")
	if err != nil {
		t.Fatalf("read shim.go: %v", err)
	}
	// Ensure both shim types are marked deprecated
	content := string(data)
	// At least two deprecated markers (Shim and AgentAdapterShim)
	count := strings.Count(content, "// Deprecated: use ExtensionAPI")
	if count < 2 {
		t.Fatalf("expected at least 2 deprecated annotations, got %d", count)
	}
}
