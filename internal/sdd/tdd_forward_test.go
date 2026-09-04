package sdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTDDConfig_FileBased(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create .biggz/tdd.json
	configDir := filepath.Join(tmpDir, ".biggz")
	os.MkdirAll(configDir, 0755)
	
	data := []byte(`{"strict_tdd":true,"test_command":"go test ./...","test_runner":"go"}`)
	os.WriteFile(filepath.Join(configDir, "tdd.json"), data, 0644)
	
	// Load config
	loaded, err := LoadTDDConfig(tmpDir, "test-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !loaded.StrictTDD {
		t.Error("expected strict_tdd to be true")
	}
	if loaded.TestCommand != "go test ./..." {
		t.Errorf("expected 'go test ./...', got %q", loaded.TestCommand)
	}
}

func TestLoadTDDConfig_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Load config - should return default
	loaded, err := LoadTDDConfig(tmpDir, "test-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.StrictTDD {
		t.Error("expected strict_tdd to be false by default")
	}
}

func TestGetTDDForwardingResult_Disabled(t *testing.T) {
	config := &TDDConfig{StrictTDD: false}
	result := GetTDDForwardingResult(config)
	if result.Enabled {
		t.Error("expected Enabled to be false")
	}
}

func TestGetTDDForwardingResult_Enabled(t *testing.T) {
	config := &TDDConfig{
		StrictTDD:   true,
		TestCommand: "cargo test",
	}
	result := GetTDDForwardingResult(config)
	if !result.Enabled {
		t.Error("expected Enabled to be true")
	}
	if result.TestCommand != "cargo test" {
		t.Errorf("expected 'cargo test', got %q", result.TestCommand)
	}
	if result.Instructions == "" {
		t.Error("expected non-empty instructions")
	}
}

func TestBuildTDDInstructions(t *testing.T) {
	config := &TDDConfig{
		StrictTDD:   true,
		TestCommand: "npm test",
	}
	instructions := buildTDDInstructions(config)
	
	if instructions == "" {
		t.Error("expected non-empty instructions")
	}
	if !contains(instructions, "STRICT TDD MODE IS ACTIVE") {
		t.Error("expected 'STRICT TDD MODE IS ACTIVE' in instructions")
	}
	if !contains(instructions, "npm test") {
		t.Error("expected test command in instructions")
	}
}

func TestForwardTDDToSubAgent_Apply(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create config
	configDir := filepath.Join(tmpDir, ".biggz")
	os.MkdirAll(configDir, 0755)
	data := []byte(`{"strict_tdd":true,"test_command":"go test ./..."}`)
	os.WriteFile(filepath.Join(configDir, "tdd.json"), data, 0644)
	
	// Forward for apply phase
	instructions := ForwardTDDToSubAgent(tmpDir, "test", "apply")
	if instructions == "" {
		t.Error("expected non-empty instructions for apply phase")
	}
}

func TestForwardTDDToSubAgent_Verify(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create config
	configDir := filepath.Join(tmpDir, ".biggz")
	os.MkdirAll(configDir, 0755)
	data := []byte(`{"strict_tdd":true,"test_command":"go test ./..."}`)
	os.WriteFile(filepath.Join(configDir, "tdd.json"), data, 0644)
	
	// Forward for verify phase
	instructions := ForwardTDDToSubAgent(tmpDir, "test", "verify")
	if instructions == "" {
		t.Error("expected non-empty instructions for verify phase")
	}
}

func TestForwardTDDToSubAgent_OtherPhase(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create config
	configDir := filepath.Join(tmpDir, ".biggz")
	os.MkdirAll(configDir, 0755)
	data := []byte(`{"strict_tdd":true}`)
	os.WriteFile(filepath.Join(configDir, "tdd.json"), data, 0644)
	
	// Forward for spec phase - should be empty
	instructions := ForwardTDDToSubAgent(tmpDir, "test", "spec")
	if instructions != "" {
		t.Error("expected empty instructions for non-apply/verify phases")
	}
}

func TestForwardTDDToSubAgent_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create config with strict_tdd: false
	configDir := filepath.Join(tmpDir, ".biggz")
	os.MkdirAll(configDir, 0755)
	data := []byte(`{"strict_tdd":false}`)
	os.WriteFile(filepath.Join(configDir, "tdd.json"), data, 0644)
	
	// Forward for apply phase - should be empty
	instructions := ForwardTDDToSubAgent(tmpDir, "test", "apply")
	if instructions != "" {
		t.Error("expected empty instructions when TDD is disabled")
	}
}

func TestSaveTDDConfig(t *testing.T) {
	tmpDir := t.TempDir()
	
	config := &TDDConfig{
		StrictTDD:   true,
		TestCommand: "pytest",
		TestRunner:  "python",
	}
	
	err := SaveTDDConfig(tmpDir, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Verify file exists
	configPath := filepath.Join(tmpDir, ".biggz", "tdd.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("expected config file to exist")
	}
	
	// Verify content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	var loaded TDDConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !loaded.StrictTDD {
		t.Error("expected strict_tdd to be true")
	}
}

func TestTDDForwardingSummary(t *testing.T) {
	tests := []struct {
		name   string
		result *TDDForwardingResult
		contains string
	}{
		{
			name: "enabled",
			result: &TDDForwardingResult{Enabled: true, Source: "config"},
			contains: "ENABLED",
		},
		{
			name: "disabled",
			result: &TDDForwardingResult{Enabled: false},
			contains: "DISABLED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := TDDForwardingSummary(tt.result)
			if summary == "" {
				t.Error("expected non-empty summary")
			}
		})
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
