package agentbuilder

import (
	"context"
	"errors"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/agents"
	"github.com/biggs-100/biggz-ai/model"
)

// ─── MockEngine tests ────────────────────────────────────────────────────────

func TestMockEngine_Agent_ReturnsConfiguredID(t *testing.T) {
	mock := &MockEngine{AgentIDVal: agents.AgentClaudeCode}
	if got := mock.Agent(); got != agents.AgentClaudeCode {
		t.Errorf("Agent() = %q, want %q", got, agents.AgentClaudeCode)
	}
}

func TestMockEngine_Available_ReturnsConfiguredValue(t *testing.T) {
	tests := []struct {
		available bool
	}{
		{true},
		{false},
	}
	for _, tt := range tests {
		mock := &MockEngine{IsAvailable: tt.available}
		if got := mock.Available(); got != tt.available {
			t.Errorf("Available() = %v, want %v", got, tt.available)
		}
	}
}

func TestMockEngine_Generate_ReturnsConfiguredResponse(t *testing.T) {
	mock := &MockEngine{
		Output:      "generated content",
		IsAvailable: true,
	}
	out, err := mock.Generate(context.Background(), "some prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "generated content" {
		t.Errorf("Generate() = %q, want %q", out, "generated content")
	}
}

func TestMockEngine_Generate_ReturnsConfiguredError(t *testing.T) {
	expectedErr := errors.New("generation failed")
	mock := &MockEngine{
		Err:         expectedErr,
		IsAvailable: true,
	}
	_, err := mock.Generate(context.Background(), "some prompt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

// ─── NewEngine tests ─────────────────────────────────────────────────────────

func TestNewEngine_ClaudeCode_ReturnsClaudeEngine(t *testing.T) {
	engine := NewEngine(agents.AgentClaudeCode)
	if engine == nil {
		t.Fatal("expected non-nil engine for claude-code")
	}
	if engine.Agent() != agents.AgentClaudeCode {
		t.Errorf("Agent() = %q, want %q", engine.Agent(), agents.AgentClaudeCode)
	}
}

func TestNewEngine_OpenCode_ReturnsOpenCodeEngine(t *testing.T) {
	engine := NewEngine(agents.AgentOpenCode)
	if engine == nil {
		t.Fatal("expected non-nil engine for opencode")
	}
	if engine.Agent() != agents.AgentOpenCode {
		t.Errorf("Agent() = %q, want %q", engine.Agent(), agents.AgentOpenCode)
	}
}

func TestNewEngine_GeminiCLI_ReturnsGeminiEngine(t *testing.T) {
	engine := NewEngine(agents.AgentGeminiCLI)
	if engine == nil {
		t.Fatal("expected non-nil engine for gemini-cli")
	}
	if engine.Agent() != agents.AgentGeminiCLI {
		t.Errorf("Agent() = %q, want %q", engine.Agent(), agents.AgentGeminiCLI)
	}
}

func TestNewEngine_CodexCLI_ReturnsCodexEngine(t *testing.T) {
	engine := NewEngine(agents.AgentCodexCLI)
	if engine == nil {
		t.Fatal("expected non-nil engine for codex-cli")
	}
	if engine.Agent() != agents.AgentCodexCLI {
		t.Errorf("Agent() = %q, want %q", engine.Agent(), agents.AgentCodexCLI)
	}
}

func TestNewEngine_Unknown_ReturnsNil(t *testing.T) {
	engine := NewEngine(model.AgentID("unknown-agent"))
	if engine != nil {
		t.Errorf("expected nil engine for unknown agent, got %v", engine)
	}
}

func TestNewEngine_Cursor_ReturnsNil(t *testing.T) {
	// cursor, vscode-copilot, etc. are not supported generation engines.
	engine := NewEngine(agents.AgentCursor)
	if engine != nil {
		t.Errorf("expected nil engine for cursor (not a generation engine), got %v", engine)
	}
}

// ─── Engine command construction tests ───────────────────────────────────────
// These test the Agent() method (which identifies the CLI binary used)
// without actually executing anything.

func TestClaudeEngine_AgentID(t *testing.T) {
	e := &ClaudeEngine{}
	if e.Agent() != agents.AgentClaudeCode {
		t.Errorf("ClaudeEngine.Agent() = %q, want %q", e.Agent(), agents.AgentClaudeCode)
	}
}

func TestOpenCodeEngine_AgentID(t *testing.T) {
	e := &OpenCodeEngine{}
	if e.Agent() != agents.AgentOpenCode {
		t.Errorf("OpenCodeEngine.Agent() = %q, want %q", e.Agent(), agents.AgentOpenCode)
	}
}

func TestGeminiEngine_AgentID(t *testing.T) {
	e := &GeminiEngine{}
	if e.Agent() != agents.AgentGeminiCLI {
		t.Errorf("GeminiEngine.Agent() = %q, want %q", e.Agent(), agents.AgentGeminiCLI)
	}
}

func TestCodexEngine_AgentID(t *testing.T) {
	e := &CodexEngine{}
	if e.Agent() != agents.AgentCodexCLI {
		t.Errorf("CodexEngine.Agent() = %q, want %q", e.Agent(), agents.AgentCodexCLI)
	}
}

// TestAllSupportedEngines verifies that all supported engine IDs produce non-nil engines.
func TestAllSupportedEngines(t *testing.T) {
	supported := []model.AgentID{
		agents.AgentClaudeCode,
		agents.AgentOpenCode,
		agents.AgentGeminiCLI,
		agents.AgentCodexCLI,
	}
	for _, id := range supported {
		t.Run(string(id), func(t *testing.T) {
			engine := NewEngine(id)
			if engine == nil {
				t.Errorf("NewEngine(%q) returned nil", id)
			}
		})
	}
}
