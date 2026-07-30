// Package opencodeplugin manages installation and uninstallation of
// OpenCode community plugins.
package opencodeplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/filemerge"
)

// Plugin describes a community OpenCode plugin.
type Plugin struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	NpmPackage  string `json:"npm_package"`
	InstallPath string `json:"install_path"`
}

// KnownPlugins returns the list of known community plugins.
func KnownPlugins() []Plugin {
	return []Plugin{
		{
			Name:        "sub-agent-statusline",
			Description: "Shows active sub-agent in the OpenCode statusline",
			NpmPackage:  "@opencode-ai/plugin-sub-agent-statusline",
		},
		{
			Name:        "sdd-engram-plugin",
			Description: "SDD change status in the OpenCode sidebar",
			NpmPackage:  "@gentle-ai/sdd-engram-plugin",
		},
		{
			Name:        "gentle-logo",
			Description: "Gentle AI logo in the OpenCode startup screen",
			NpmPackage:  "@gentle-ai/gentle-logo",
		},
	}
}

// installDir returns the OpenCode plugins directory.
func installDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "opencode", "plugins")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// tuiConfigPath returns the path to the OpenCode TUI config.
func tuiConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode", "tui.json"), nil
}

// Install installs a community plugin by name.
func Install(name string) error {
	// Find the plugin
	plugins := KnownPlugins()
	var target *Plugin
	for _, p := range plugins {
		if p.Name == name {
			target = &p
			break
		}
	}
	if target == nil {
		return fmt.Errorf("unknown plugin %q", name)
	}

	// Install npm package
	cmd := exec.Command("npm", "install", "-g", target.NpmPackage)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("npm install failed: %s: %w", string(out), err)
	}

	// Register in opencode tui.json
	cfgPath, err := tuiConfigPath()
	if err != nil {
		return err
	}

	var existingData []byte
	if _, err := os.Stat(cfgPath); err == nil {
		existingData, err = os.ReadFile(cfgPath)
		if err != nil {
			return err
		}
	}
	if len(existingData) == 0 {
		existingData = []byte("{}")
	}

	overlay := map[string]any{
		"plugins": []map[string]any{
			{"name": target.NpmPackage, "enabled": true},
		},
	}
	overlayData, _ := json.Marshal(overlay)

	merged, err := filemerge.MergeJSONC(existingData, overlayData)
	if err != nil {
		return fmt.Errorf("merge plugin config: %w", err)
	}

	return os.WriteFile(cfgPath, merged, 0644)
}

// Uninstall removes a community plugin.
func Uninstall(name string) error {
	plugins := KnownPlugins()
	var target *Plugin
	for _, p := range plugins {
		if p.Name == name {
			target = &p
			break
		}
	}
	if target == nil {
		return fmt.Errorf("unknown plugin %q", name)
	}

	// Uninstall npm package
	cmd := exec.Command("npm", "uninstall", "-g", target.NpmPackage)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("npm uninstall failed: %s: %w", string(out), err)
	}

	// Remove from tui.json
	cfgPath, err := tuiConfigPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil // file gone = nothing to clean up
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}

	if plugins, ok := cfg["plugins"].([]any); ok {
		var filtered []any
		for _, p := range plugins {
			if pm, ok := p.(map[string]any); ok {
				if pm["name"] == target.NpmPackage {
					continue
				}
			}
			filtered = append(filtered, p)
		}
		cfg["plugins"] = filtered
	}

	out, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(cfgPath, out, 0644)
}

// ListInstalled returns names of installed plugins.
func ListInstalled() ([]string, error) {
	cfgPath, err := tuiConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, nil
	}
	var result []string
	if plugins, ok := cfg["plugins"].([]any); ok {
		for _, p := range plugins {
			if pm, ok := p.(map[string]any); ok {
				if name, ok := pm["name"].(string); ok {
					result = append(result, name)
				}
			}
		}
	}
	return result, nil
}

// FormatPluginList returns a formatted string of all known plugins.
func FormatPluginList() string {
	var b strings.Builder
	b.WriteString("Available OpenCode community plugins:\n\n")
	for _, p := range KnownPlugins() {
		b.WriteString(fmt.Sprintf("  %s\n", p.Name))
		b.WriteString(fmt.Sprintf("    %s\n", p.Description))
		b.WriteString(fmt.Sprintf("    npm: %s\n", p.NpmPackage))
		b.WriteString("\n")
	}
	b.WriteString("Usage:\n")
	b.WriteString("  biggz plugin install <name>    — Install a plugin\n")
	b.WriteString("  biggz plugin uninstall <name>  — Uninstall a plugin\n")
	b.WriteString("  biggz plugin list              — List installed plugins\n")
	return b.String()
}
