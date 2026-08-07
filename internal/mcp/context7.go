// Package mcp provides MCP server configuration helpers.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

// Context7Config holds the configuration for the Context7 MCP server.
type Context7Config struct {
	// UserAPIKey is the Context7 API key.
	UserAPIKey string `json:"userApiKey,omitempty"`
	// Enabled controls whether the MCP server is active.
	Enabled bool `json:"enabled"`
}

// DefaultContext7Config returns the default Context7 config.
func DefaultContext7Config() Context7Config {
	return Context7Config{Enabled: true}
}

// InjectContext7ToSettings adds the Context7 MCP server configuration
// into the agent's settings file (opencode.json for OpenCode).
func InjectContext7ToSettings(settingsPath string, cfg Context7Config) error {
	overlay := map[string]any{
		"mcp": map[string]any{
			"context7": map[string]any{
				"command":    []string{"npx", "-y", "@context7/context7-mcp"},
				"type":       "local",
				"enabled":    cfg.Enabled,
				"userApiKey": cfg.UserAPIKey,
			},
		},
	}
	overlayData, err := json.Marshal(overlay)
	if err != nil {
		return fmt.Errorf("marshal context7 overlay: %w", err)
	}

	var existingData []byte
	if _, err := os.Stat(settingsPath); err == nil {
		existingData, err = os.ReadFile(settingsPath)
		if err != nil {
			return err
		}
	}
	if len(existingData) == 0 {
		existingData = []byte("{}")
	}

	merged, err := filemerge.MergeJSONC(existingData, overlayData)
	if err != nil {
		return fmt.Errorf("merge context7: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(settingsPath, merged, 0644)
}

// RemoveContext7FromSettings removes the Context7 MCP server entry.
func RemoveContext7FromSettings(settingsPath string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if mcp, ok := cfg["mcp"].(map[string]any); ok {
		delete(mcp, "context7")
		cfg["mcp"] = mcp
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, out, 0644)
}

// IsContext7Installed checks if Context7 MCP is configured.
func IsContext7Installed(settingsPath string) bool {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false
	}
	if mcp, ok := cfg["mcp"].(map[string]any); ok {
		if _, ok := mcp["context7"]; ok {
			return true
		}
	}
	return false
}
