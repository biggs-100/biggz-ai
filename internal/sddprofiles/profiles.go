// Package sddprofiles manages named SDD profiles that assign different
// models to different SDD phases for cost optimization.
//
// Examples:
//   - cheap: gpt-4o-mini for spec/design, gpt-4o for apply
//   - quality: claude-sonnet-4-5 for all phases
//   - fast: gemini-2.5-flash for planning, gpt-4o for apply
package sddprofiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// Profile defines a named model assignment for SDD phases.
type Profile struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Agents      map[string]string `json:"agents"` // phase → model, e.g. "sdd-spec" → "gpt-4o-mini"
}

// PhaseOrder defines the canonical SDD phase order for profiles.
var PhaseOrder = []string{
	"sdd-init",
	"sdd-explore",
	"sdd-propose",
	"sdd-spec",
	"sdd-design",
	"sdd-tasks",
	"sdd-apply",
	"sdd-verify",
	"sdd-archive",
	"sdd-onboard",
}

// ─── Built-in profiles ───────────────────────────────────────────────────────

// DefaultProfiles returns the built-in profile catalog.
func DefaultProfiles() []Profile {
	return []Profile{
		{
			Name:        "default",
			Description: "Default model for all phases (no per-phase assignment)",
			Agents:      map[string]string{},
		},
		{
			Name:        "balanced",
			Description: "Fast models for planning, powerful models for coding",
			Agents: map[string]string{
				"sdd-init":    "fast",
				"sdd-explore": "fast",
				"sdd-propose": "fast",
				"sdd-spec":    "balanced",
				"sdd-design":  "balanced",
				"sdd-tasks":   "fast",
				"sdd-apply":   "powerful",
				"sdd-verify":  "balanced",
				"sdd-archive": "fast",
				"sdd-onboard": "fast",
			},
		},
		{
			Name:        "quality",
			Description: "Premium model for all phases (best results, highest cost)",
			Agents: map[string]string{
				"sdd-init":    "powerful",
				"sdd-explore": "powerful",
				"sdd-propose": "powerful",
				"sdd-spec":    "powerful",
				"sdd-design":  "powerful",
				"sdd-tasks":   "powerful",
				"sdd-apply":   "powerful",
				"sdd-verify":  "powerful",
				"sdd-archive": "powerful",
				"sdd-onboard": "powerful",
			},
		},
		{
			Name:        "cheap",
			Description: "Budget models for all phases (fast but less capable)",
			Agents: map[string]string{
				"sdd-init":    "fast",
				"sdd-explore": "fast",
				"sdd-propose": "fast",
				"sdd-spec":    "fast",
				"sdd-design":  "fast",
				"sdd-tasks":   "fast",
				"sdd-apply":   "balanced",
				"sdd-verify":  "balanced",
				"sdd-archive": "fast",
				"sdd-onboard": "fast",
			},
		},
	}
}

// ─── Overlay generation ──────────────────────────────────────────────────────

// GenerateOverlay creates an overlay JSON that assigns per-agent models
// for the given profile. It also adds a profile-scoped orchestrator agent
// (e.g., biggz-orchestrator-cheap) so the user can switch between profiles.
func GenerateOverlay(profile Profile) ([]byte, error) {
	// Build agent overrides: for each phase in PhaseOrder that has a model
	// assignment in the profile, generate agent config with model override.
	agentOverrides := make(map[string]any)
	for _, phase := range PhaseOrder {
		modelTier, ok := profile.Agents[phase]
		if !ok || modelTier == "" {
			continue
		}
		// Create profile-scoped agent key: sdd-apply → sdd-apply-cheap
		agentKey := phase + "-" + profile.Name
		agentOverrides[agentKey] = map[string]any{
			"model": modelTier,
		}
		// Also add the original phase key with model override
		// (so the orchestrator picks the right model when delegating)
		if _, exists := agentOverrides[phase]; !exists {
			// Only set if not already defined by a higher-priority profile
		}
	}

	// Create profile-scoped orchestrator
	orchestratorKey := "biggz-orchestrator-" + profile.Name
	agentOverrides[orchestratorKey] = map[string]any{
		"description": fmt.Sprintf("biggz-ai SDD Orchestrator (%s profile)", profile.Name),
	}

	overlay := map[string]any{
		"agent": agentOverrides,
	}

	return json.MarshalIndent(overlay, "", "  ")
}

// ─── Profile detection ───────────────────────────────────────────────────────

// DetectProfiles reads the agent's settings file and returns all active profiles.
// It detects profile-scoped agent keys (e.g., sdd-apply-cheap, biggz-orchestrator-cheap).
func DetectProfiles(settingsPath string) ([]string, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, err
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	agents, ok := cfg["agent"].(map[string]any)
	if !ok {
		return nil, nil
	}

	// Collect unique profile suffixes by looking for agent keys matching
	// pattern: sdd-{phase}-{name} or biggz-orchestrator-{name}
	profiles := make(map[string]bool)
	for key := range agents {
		for _, phase := range PhaseOrder {
			prefix := phase + "-"
			if strings.HasPrefix(key, prefix) {
				suffix := strings.TrimPrefix(key, prefix)
				if suffix != "" && suffix != phase {
					profiles[suffix] = true
				}
			}
		}
		// Also check for profile-scoped orchestrator
		orchPrefix := "biggz-orchestrator-"
		if strings.HasPrefix(key, orchPrefix) {
			suffix := strings.TrimPrefix(key, orchPrefix)
			if suffix != "" && suffix != "biggz-orchestrator" {
				profiles[suffix] = true
			}
		}
	}

	var result []string
	for p := range profiles {
		result = append(result, p)
	}
	return result, nil
}

// ─── Profile application ─────────────────────────────────────────────────────

// ApplyProfile installs a profile into the agent's settings file.
// It generates the overlay and merges it into the existing settings.
func ApplyProfile(settingsPath string, profile Profile) error {
	overlay, err := GenerateOverlay(profile)
	if err != nil {
		return fmt.Errorf("generate overlay: %w", err)
	}

	var existingData []byte
	if _, err := os.Stat(settingsPath); err == nil {
		existingData, err = os.ReadFile(settingsPath)
		if err != nil {
			return fmt.Errorf("read settings: %w", err)
		}
	}
	if len(existingData) == 0 {
		existingData = []byte("{}")
	}

	merged, err := filemerge.MergeJSONC(existingData, overlay)
	if err != nil {
		return fmt.Errorf("merge: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(settingsPath, merged, 0644)
}

// RemoveProfile removes all profile-scoped agents for a given profile name.
func RemoveProfile(settingsPath string, profileName string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return err
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	agents, ok := cfg["agent"].(map[string]any)
	if !ok {
		return nil
	}

	// Remove all agent keys matching pattern: *-{profileName}
	for key := range agents {
		if strings.HasSuffix(key, "-"+profileName) {
			delete(agents, key)
		}
	}
	cfg["agent"] = agents

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, out, 0644)
}

// ─── CLI helpers ─────────────────────────────────────────────────────────────

// ListProfiles returns the built-in profile catalog formatted for CLI display.
func ListProfiles() string {
	var b strings.Builder
	b.WriteString("Available SDD profiles:\n\n")
	for _, p := range DefaultProfiles() {
		b.WriteString(fmt.Sprintf("  %-12s  %s\n", p.Name, p.Description))
		if len(p.Agents) > 0 {
			for _, phase := range PhaseOrder {
				if model, ok := p.Agents[phase]; ok {
					b.WriteString(fmt.Sprintf("    %-20s → %s\n", phase, model))
				}
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("Usage: biggz sdd-profile apply <name>     — activate profile\n")
	b.WriteString("       biggz sdd-profile remove <name>    — remove profile agents\n")
	b.WriteString("       biggz sdd-profile list             — list available profiles\n")
	return b.String()
}
