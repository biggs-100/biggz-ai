package capabilitymanifest

import (
	"testing"

	"github.com/biggz-ai/biggz/model"
)

func TestCount_Exactly27(t *testing.T) {
	expected := 27
	if got := Count(); got != expected {
		t.Fatalf("featureClaimsByAgent has %d entries, want %d", got, expected)
	}
}

func TestForAgent_Existing(t *testing.T) {
	tests := []struct {
		id   model.AgentID
		name string
	}{
		{"opencode", "opencode"},
		{"claude-code", "claude-code"},
		{"qwen-code", "qwen-code"},
		{"cursor", "cursor"},
		{"windsurf", "windsurf"},
		{"github-copilot", "github-copilot"},
		{"cody", "cody"},
		{"aider", "aider"},
		{"continue", "continue"},
		{"codeium", "codeium"},
		{"tabby", "tabby"},
		{"marscode", "marscode"},
		{"comate", "comate"},
		{"codegeex", "codegeex"},
		{"melo", "melo"},
		{"lingma", "lingma"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, ok := ForAgent(tt.id)
			if !ok {
				t.Fatalf("ForAgent(%q) returned false, want true", tt.id)
			}
			if m.AgentID != tt.id {
				t.Errorf("manifest.AgentID = %q, want %q", m.AgentID, tt.id)
			}
			if m.SchemaVersion != "1.0" {
				t.Errorf("manifest.SchemaVersion = %q, want %q", m.SchemaVersion, "1.0")
			}
		})
	}
}

func TestForAgent_Unknown(t *testing.T) {
	m, ok := ForAgent("nonexistent")
	if ok {
		t.Fatal("ForAgent(\"nonexistent\") returned true, want false")
	}
	if m != nil {
		t.Fatal("ForAgent(\"nonexistent\") returned non-nil manifest, want nil")
	}
}

func TestForAgent_AllAgentIDs(t *testing.T) {
	ids := AllAgentIDs()
	if len(ids) != 16 {
		expected := 27
	if len(ids) != expected {
		t.Fatalf("AllAgentIDs() returned %d ids, want %d", len(ids), expected)
	}
	}
	// Every AllAgentIDs result must be findable via ForAgent
	for _, id := range ids {
		m, ok := ForAgent(id)
		if !ok {
			t.Errorf("ForAgent(%q) returned false for id from AllAgentIDs()", id)
		}
		if m.AgentID != id {
			t.Errorf("manifest.AgentID = %q, want %q", m.AgentID, id)
		}
	}
}

func TestValidate_Pass(t *testing.T) {
	for _, id := range AllAgentIDs() {
		m, ok := ForAgent(id)
		if !ok {
			t.Fatalf("ForAgent(%q) returned false", id)
		}
		if err := Validate(*m); err != nil {
			t.Errorf("Validate(%q) returned error: %v", id, err)
		}
	}
}

func TestValidate_UnknownAgent(t *testing.T) {
	m := AgentCapabilityManifest{
		SchemaVersion: "1.0",
		AgentID:       "unknown-agent",
		Features:      AgentFeatureClaims{},
	}
	if err := Validate(m); err == nil {
		t.Error("Validate() expected error for unknown agent, got nil")
	}
}

func TestValidate_Mismatch(t *testing.T) {
	m := AgentCapabilityManifest{
		SchemaVersion: "1.0",
		AgentID:       "opencode",
		Features: AgentFeatureClaims{
			AutoInstall: false, // mismatched — canonical has true
		},
	}
	if err := Validate(m); err == nil {
		t.Error("Validate() expected error for mismatched claims, got nil")
	}
}

func TestFeatureClaims_OpenCode(t *testing.T) {
	m, ok := ForAgent("opencode")
	if !ok {
		t.Fatal("ForAgent(\"opencode\") returned false")
	}
	if !m.Features.AutoInstall {
		t.Error("OpenCode: expected AutoInstall=true")
	}
	if !m.Features.Skills {
		t.Error("OpenCode: expected Skills=true")
	}
	if !m.Features.SystemPrompt {
		t.Error("OpenCode: expected SystemPrompt=true")
	}
	if !m.Features.MCP {
		t.Error("OpenCode: expected MCP=true")
	}
	if !m.Features.SlashCommands {
		t.Error("OpenCode: expected SlashCommands=true")
	}
	if !m.Features.Workflows {
		t.Error("OpenCode: expected Workflows=true")
	}
	if m.Features.OutputStyles {
		t.Error("OpenCode: expected OutputStyles=false")
	}
	if m.Features.FileSubAgents {
		t.Error("OpenCode: expected FileSubAgents=false")
	}
}

func TestFeatureClaims_ClaudeCode(t *testing.T) {
	m, ok := ForAgent("claude-code")
	if !ok {
		t.Fatal("ForAgent(\"claude-code\") returned false")
	}
	if !m.Features.AutoInstall {
		t.Error("Claude Code: expected AutoInstall=true")
	}
	if !m.Features.OutputStyles {
		t.Error("Claude Code: expected OutputStyles=true")
	}
	if !m.Features.SlashCommands {
		t.Error("Claude Code: expected SlashCommands=true")
	}
	if !m.Features.FileSubAgents {
		t.Error("Claude Code: expected FileSubAgents=true")
	}
	if !m.Features.Skills {
		t.Error("Claude Code: expected Skills=true")
	}
	if !m.Features.SystemPrompt {
		t.Error("Claude Code: expected SystemPrompt=true")
	}
	if !m.Features.MCP {
		t.Error("Claude Code: expected MCP=true")
	}
	if m.Features.Workflows {
		t.Error("Claude Code: expected Workflows=false")
	}
}

func TestFeatureClaims_QwenCode(t *testing.T) {
	m, ok := ForAgent("qwen-code")
	if !ok {
		t.Fatal("ForAgent(\"qwen-code\") returned false")
	}
	if m.Features.AutoInstall {
		t.Error("Qwen Code: expected AutoInstall=false")
	}
	if !m.Features.SlashCommands {
		t.Error("Qwen Code: expected SlashCommands=true")
	}
	if !m.Features.Skills {
		t.Error("Qwen Code: expected Skills=true")
	}
	if !m.Features.SystemPrompt {
		t.Error("Qwen Code: expected SystemPrompt=true")
	}
	if !m.Features.MCP {
		t.Error("Qwen Code: expected MCP=true")
	}
	if m.Features.OutputStyles {
		t.Error("Qwen Code: expected OutputStyles=false")
	}
	if m.Features.FileSubAgents {
		t.Error("Qwen Code: expected FileSubAgents=false")
	}
	if m.Features.Workflows {
		t.Error("Qwen Code: expected Workflows=false")
	}
}

func TestFeatureClaims_OutOfScopeAgents_AllFalse(t *testing.T) {
	outOfScope := []model.AgentID{
		"github-copilot",
		"cody", "aider", "continue", "codeium",
		"tabby", "marscode", "comate", "codegeex",
		"melo", "lingma",
	}
	for _, id := range outOfScope {
		t.Run(string(id), func(t *testing.T) {
			m, ok := ForAgent(id)
			if !ok {
				t.Fatalf("ForAgent(%q) returned false", id)
			}
			if m.Features != (AgentFeatureClaims{}) {
				t.Errorf("%s: expected all claims false, got %+v", id, m.Features)
			}
		})
	}
}
