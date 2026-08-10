// Package opencode reads the OpenCode model catalog cache and the biggz
// model-variants plugin cache, and resolves per-agent model assignments.
package opencode

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultCachePath returns the default path to the OpenCode models cache file.
// OpenCode writes this file after its first startup; until then it may not
// exist and callers must treat it as empty.
func DefaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "opencode", "models.json")
}

// DefaultSettingsPath returns the default path to the OpenCode settings file.
func DefaultSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// DefaultVariantsCachePath returns the path to the biggz plugin-generated
// model-variants file (~/.biggz/cache/model-variants.json).
func DefaultVariantsCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".biggz", "cache", "model-variants.json")
}

// Model represents a single model within a provider. Cost is the input price
// per million tokens and Limit is the context window in tokens. The cache
// stores these as nested objects ({"input": .., "output": ..} / {"context": ..,
// "output": ..}); custom UnmarshalJSON flattens them into these scalars while
// also accepting flat numeric shapes.
type Model struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Family    string   `json:"family"`
	ToolCall  bool     `json:"tool_call"`
	Reasoning bool     `json:"reasoning"`
	Cost      float64  `json:"-"`
	Limit     float64  `json:"-"`
	Variants  []string `json:"-"` // populated by EnrichWithVariants from the plugin cache
}

// UnmarshalJSON tolerates both the nested cost/limit object shape written by
// OpenCode and plain numeric shapes.
func (m *Model) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Family    string `json:"family"`
		ToolCall  bool   `json:"tool_call"`
		Reasoning bool   `json:"reasoning"`
		Cost      any    `json:"cost"`
		Limit     any    `json:"limit"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*m = Model{
		ID:        raw.ID,
		Name:      raw.Name,
		Family:    raw.Family,
		ToolCall:  raw.ToolCall,
		Reasoning: raw.Reasoning,
	}
	switch v := raw.Cost.(type) {
	case float64:
		m.Cost = v
	case map[string]any:
		if in, ok := v["input"].(float64); ok {
			m.Cost = in
		}
	}
	switch v := raw.Limit.(type) {
	case float64:
		m.Limit = v
	case map[string]any:
		if context, ok := v["context"].(float64); ok {
			m.Limit = context
		}
	}
	return nil
}

// Provider represents a model provider. Env lists the environment variables
// OpenCode uses for authentication (flattened to a comma-separated string from
// the cache's array shape).
type Provider struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Env    string           `json:"-"`
	Models map[string]Model `json:"models"`
}

// UnmarshalJSON tolerates both the array env shape written by OpenCode and a
// flat string shape.
func (p *Provider) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID     string           `json:"id"`
		Name   string           `json:"name"`
		Env    any              `json:"env"`
		Models map[string]Model `json:"models"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*p = Provider{ID: raw.ID, Name: raw.Name, Models: raw.Models}
	switch v := raw.Env.(type) {
	case string:
		p.Env = v
	case []any:
		parts := make([]string, 0, len(v))
		for _, e := range v {
			if s, ok := e.(string); ok {
				parts = append(parts, s)
			}
		}
		p.Env = strings.Join(parts, ", ")
	}
	return nil
}

// LoadModels parses the OpenCode models cache JSON file and returns providers
// keyed by ID. Malformed provider entries are skipped so one bad provider
// never blanks the whole catalog.
func LoadModels(cachePath string) (map[string]Provider, error) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("read models cache %q: %w", cachePath, err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse models cache: %w", err)
	}

	providers := make(map[string]Provider, len(raw))
	for id, providerJSON := range raw {
		var p Provider
		if err := json.Unmarshal(providerJSON, &p); err != nil {
			continue
		}
		p.ID = id
		providers[id] = p
	}

	return providers, nil
}

// LoadModelsOrEmpty parses the OpenCode models cache when it exists and falls
// back to an empty provider set when OpenCode has not populated the cache yet.
func LoadModelsOrEmpty(cachePath string) (map[string]Provider, error) {
	providers, err := LoadModels(cachePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]Provider{}, nil
		}
		return nil, err
	}
	return providers, nil
}

// LoadVariants reads the plugin-generated model-variants.json file.
func LoadVariants(variantsPath string) (map[string]map[string][]string, error) {
	data, err := os.ReadFile(variantsPath)
	if err != nil {
		return nil, err
	}
	var variants map[string]map[string][]string
	if err := json.Unmarshal(data, &variants); err != nil {
		return nil, err
	}
	return variants, nil
}

// EnrichWithVariants merges variant data from the plugin cache file into
// cache-loaded providers. If the file is missing or invalid, models keep nil
// Variants (the effort picker is then skipped).
func EnrichWithVariants(cached map[string]Provider, variantsPath string) {
	variants, err := LoadVariants(variantsPath)
	if err != nil {
		return
	}

	// Pass 1: exact provider matches.
	for provID, models := range variants {
		cachedProv, ok := cached[provID]
		if !ok {
			continue
		}
		for modelID, levels := range models {
			if cachedModel, ok := cachedProv.Models[modelID]; ok {
				cachedModel.Variants = levels
				cachedProv.Models[modelID] = cachedModel
			}
		}
		cached[provID] = cachedProv
	}

	// Pass 2: deterministic fallback — match unassigned models by model ID
	// across providers (sorted iteration keeps results deterministic).
	variantKeys := make([]string, 0, len(variants))
	for provID := range variants {
		variantKeys = append(variantKeys, provID)
	}
	sort.Strings(variantKeys)

	cachedKeys := make([]string, 0, len(cached))
	for cachedID := range cached {
		cachedKeys = append(cachedKeys, cachedID)
	}
	sort.Strings(cachedKeys)

	for _, provID := range variantKeys {
		models := variants[provID]
		modelKeys := make([]string, 0, len(models))
		for modelID := range models {
			modelKeys = append(modelKeys, modelID)
		}
		sort.Strings(modelKeys)

		for _, modelID := range modelKeys {
			levels := models[modelID]
			for _, cachedID := range cachedKeys {
				p := cached[cachedID]
				if cachedModel, ok := p.Models[modelID]; ok && len(cachedModel.Variants) == 0 {
					cachedModel.Variants = levels
					p.Models[modelID] = cachedModel
					cached[cachedID] = p
				}
			}
		}
	}
}

// FilterModelsForSDD returns models from a provider that support tool_call
// (required for agent delegation). Results are sorted by model name.
func FilterModelsForSDD(provider Provider) []Model {
	var models []Model
	for _, m := range provider.Models {
		if m.ToolCall {
			models = append(models, m)
		}
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})

	return models
}

// EffortLevels returns the available reasoning effort levels for this model.
// Returns nil if the model has no variants (effort picker should be skipped).
func (m Model) EffortLevels() []string {
	if len(m.Variants) == 0 {
		return nil
	}
	return m.Variants
}

// SDDPhases returns the ordered list of SDD phase sub-agent names.
func SDDPhases() []string {
	return []string{
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
}

// JDPhases returns the ordered list of judgment-day workflow agent names.
// They support independent model configuration for diversity of perspective.
func JDPhases() []string {
	return []string{
		"jd-judge-a",
		"jd-judge-b",
		"jd-fix-agent",
	}
}

// ReviewPhases returns the ordered list of native bounded-review agents.
func ReviewPhases() []string {
	return []string{
		"review-risk",
		"review-readability",
		"review-reliability",
		"review-resilience",
		"review-refuter",
	}
}

// OrchestratorAgent is the biggz base coordinator agent key.
const OrchestratorAgent = "biggz-orchestrator"

// ConfigurableAgentPhases returns all agent names that support per-agent model
// configuration: the orchestrator, all SDD phases, judgment-day agents, and
// review agents. This is the whitelist used by ReadCurrentModelAssignments.
func ConfigurableAgentPhases() []string {
	phases := []string{OrchestratorAgent}
	phases = append(phases, SDDPhases()...)
	phases = append(phases, JDPhases()...)
	phases = append(phases, ReviewPhases()...)
	return phases
}
