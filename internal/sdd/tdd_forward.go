// Package sdd — TDD Forwarding: auto-inject strict TDD instructions.
//
// When launching sdd-apply or sdd-verify sub-agents, the orchestrator MUST:
//  1. Search for testing capabilities in BigMem
//  2. If strict_tdd: true, add TDD instructions to sub-agent prompt
//  3. If not found, sub-agent uses Standard Mode
//
// This ensures TDD compliance is automatically forwarded to sub-agents
// without manual intervention.
package sdd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TDDConfig represents the TDD configuration for a project.
type TDDConfig struct {
	// StrictTDD indicates whether strict TDD mode is enabled.
	StrictTDD bool `json:"strict_tdd"`
	// TestCommand is the command to run tests.
	TestCommand string `json:"test_command,omitempty"`
	// TestRunner is the test runner (go, cargo, npm, etc.).
	TestRunner string `json:"test_runner,omitempty"`
	// Project is the project name.
	Project string `json:"project,omitempty"`
}

// TDDForwardingResult is the result of TDD forwarding check.
type TDDForwardingResult struct {
	// Enabled indicates whether TDD is enabled for this project.
	Enabled bool `json:"enabled"`
	// TestCommand is the command to run tests.
	TestCommand string `json:"test_command,omitempty"`
	// Instructions are the TDD instructions to inject.
	Instructions string `json:"instructions,omitempty"`
	// Source indicates where the config was found (engram, file, etc.).
	Source string `json:"source,omitempty"`
}

// LoadTDDConfig attempts to load TDD configuration from various sources.
// Sources checked in order:
//  1. BigMem (engram) via topic key
//  2. File-based config (.biggz/tdd.json, openspec/tdd.json)
//  3. Default (not strict)
func LoadTDDConfig(workspaceRoot string, project string) (*TDDConfig, error) {
	// Try file-based config first (simpler, no external deps)
	config, err := loadTDDConfigFromFile(workspaceRoot)
	if err == nil && config != nil {
		return config, nil
	}

	// Try BigMem if available
	config, err = loadTDDConfigFromBigMem(project)
	if err == nil && config != nil {
		return config, nil
	}

	// Default: not strict
	return &TDDConfig{
		StrictTDD: false,
	}, nil
}

// loadTDDConfigFromFile loads TDD config from file system.
func loadTDDConfigFromFile(workspaceRoot string) (*TDDConfig, error) {
	// Check .biggz/tdd.json
	configPath := filepath.Join(workspaceRoot, ".biggz", "tdd.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var config TDDConfig
		if err := json.Unmarshal(data, &config); err == nil {
			return &config, nil
		}
	}

	// Check openspec/tdd.json
	configPath = filepath.Join(workspaceRoot, "openspec", "tdd.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var config TDDConfig
		if err := json.Unmarshal(data, &config); err == nil {
			return &config, nil
		}
	}

	return nil, fmt.Errorf("no TDD config file found")
}

// loadTDDConfigFromBigMem loads TDD config from BigMem.
// This is a placeholder - in production, it would query BigMem.
func loadTDDConfigFromBigMem(project string) (*TDDConfig, error) {
	// Placeholder: in production, query BigMem for sdd-init/{project}
	// For now, return nil to indicate not found
	return nil, fmt.Errorf("BigMem not available")
}

// GetTDDForwardingResult creates the forwarding result from config.
func GetTDDForwardingResult(config *TDDConfig) *TDDForwardingResult {
	if config == nil || !config.StrictTDD {
		return &TDDForwardingResult{
			Enabled: false,
		}
	}

	instructions := buildTDDInstructions(config)

	return &TDDForwardingResult{
		Enabled:      true,
		TestCommand:  config.TestCommand,
		Instructions: instructions,
		Source:       "config",
	}
}

// buildTDDInstructions creates the TDD instructions string.
func buildTDDInstructions(config *TDDConfig) string {
	var sb strings.Builder

	sb.WriteString("STRICT TDD MODE IS ACTIVE.\n\n")
	sb.WriteString("You MUST follow strict TDD workflow:\n")
	sb.WriteString("1. Write a failing test first\n")
	sb.WriteString("2. Run tests to confirm failure\n")
	sb.WriteString("3. Write minimal code to pass\n")
	sb.WriteString("4. Run tests to confirm pass\n")
	sb.WriteString("5. Refactor if needed\n\n")

	if config.TestCommand != "" {
		sb.WriteString(fmt.Sprintf("Test runner: %s\n", config.TestCommand))
	}

	sb.WriteString("\nDo NOT fall back to Standard Mode.\n")

	return sb.String()
}

// ForwardTDDToSubAgent creates the TDD instructions for a sub-agent prompt.
// Returns empty string if TDD is not enabled.
func ForwardTDDToSubAgent(workspaceRoot, project, phase string) string {
	// Only forward for apply and verify phases
	if phase != "apply" && phase != "verify" {
		return ""
	}

	config, err := LoadTDDConfig(workspaceRoot, project)
	if err != nil || !config.StrictTDD {
		return ""
	}

	result := GetTDDForwardingResult(config)
	return result.Instructions
}

// SaveTDDConfig saves TDD configuration to file.
func SaveTDDConfig(workspaceRoot string, config *TDDConfig) error {
	configDir := filepath.Join(workspaceRoot, ".biggz")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "tdd.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// TDDForwardingSummary returns a one-line summary for the orchestrator.
func TDDForwardingSummary(result *TDDForwardingResult) string {
	if result.Enabled {
		return fmt.Sprintf("◆ TDD forwarding · ENABLED (source: %s)", result.Source)
	}
	return "◆ TDD forwarding · DISABLED"
}
