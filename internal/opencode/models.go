// Package opencode reads the OpenCode model catalog cache and the biggz
// model-variants plugin cache, and resolves per-agent model assignments.
package opencode

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
// The file stores sorted Record<provider,Record<model,string[]>> written via tmp→rename atomic.
func LoadVariants(variantsPath string) (map[string]map[string][]string, error) {
	data, err := os.ReadFile(variantsPath)
	if err != nil {
		return nil, err
	}
	var variants map[string]map[string][]string
	if err := json.Unmarshal(data, &variants); err != nil {
		return nil, err
	}
	// ensure variants are sorted per model for determinism
	for prov, models := range variants {
		for model, levels := range models {
			sorted := append([]string(nil), levels...)
			sort.Strings(sorted)
			variants[prov][model] = sorted
		}
	}
	return variants, nil
}

// LoadVariantsOrEmpty returns empty map when file is absent or invalid, never error.
func LoadVariantsOrEmpty(variantsPath string) map[string]map[string][]string {
	v, err := LoadVariants(variantsPath)
	if err != nil {
		return map[string]map[string][]string{}
	}
	return v
}

// LoadVariantsSortedKeys returns sorted provider keys for deterministic iteration.
func LoadVariantsSortedKeys(variants map[string]map[string][]string) []string {
	keys := make([]string, 0, len(variants))
	for k := range variants {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
		"review-validator",
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

// --- Model Routing v1 (gentle-pi parity) ---

const ModelExportKind = "biggz-ai.agent_model_routing"
const ModelExportVersion = 1

// THINKING_LEVELS is the canonical ordered set for picker validation.
var THINKING_LEVELS = []ThinkingLevel{ThinkingOff, ThinkingLow, ThinkingMedium, ThinkingHigh, ThinkingInherit}

type ThinkingLevel string

const (
	ThinkingOff     ThinkingLevel = "off"
	ThinkingLow     ThinkingLevel = "low"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
	ThinkingInherit ThinkingLevel = "inherit"
)

func IsValidThinkingLevel(v ThinkingLevel) bool {
	switch v {
	case ThinkingOff, ThinkingLow, ThinkingMedium, ThinkingHigh, ThinkingInherit:
		return true
	default:
		return false
	}
}

type AgentRoutingEntry struct {
	Model    string        `json:"model,omitempty"`
	Thinking ThinkingLevel `json:"thinking,omitempty"`
}
type AgentModelConfig map[string]AgentRoutingEntry
type ModelRoutingEnvelope struct {
	Kind    string           `json:"kind"`
	Version int              `json:"version"`
	Agents  AgentModelConfig `json:"agents"`
}

var safeModelRe = regexp.MustCompile(`^[A-Za-z0-9._~:@/+%-]+$`)
var safeAgentRe = regexp.MustCompile(`^[A-Za-z0-9._:@/+%-]+$`)

func normalizeModelID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || !safeModelRe.MatchString(s) {
		return ""
	}
	return s
}
func isValidAgentName(s string) bool { return safeAgentRe.MatchString(s) }
func normalizeThinking(v string) ThinkingLevel {
	t := ThinkingLevel(strings.TrimSpace(v))
	if IsValidThinkingLevel(t) {
		return t
	}
	return ""
}
func NormalizeRoutingEntry(raw any) (*AgentRoutingEntry, bool) {
	if raw == nil {
		return nil, false
	}
	if s, ok := raw.(string); ok {
		m := normalizeModelID(s)
		if m == "" {
			return nil, false
		}
		return &AgentRoutingEntry{Model: m}, true
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	var e AgentRoutingEntry
	has := false
	if ms, ok := m["model"].(string); ok {
		if mm := normalizeModelID(ms); mm != "" {
			e.Model = mm
			has = true
		}
	}
	if ts, ok := m["thinking"].(string); ok {
		if tt := normalizeThinking(ts); tt != "" {
			e.Thinking = tt
			has = true
		}
	}
	if !has {
		if len(m) == 0 {
			return &AgentRoutingEntry{}, true
		}
		return nil, false
	}
	return &e, true
}
func NormalizeModelConfig(raw map[string]any) AgentModelConfig {
	out := make(AgentModelConfig)
	for k, v := range raw {
		if !isValidAgentName(k) {
			continue
		}
		if e, ok := NormalizeRoutingEntry(v); ok && e != nil {
			out[k] = *e
		}
	}
	return out
}
func DefaultModelConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".biggz", "models.json")
}
func ReadModelConfig(path string) (AgentModelConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(AgentModelConfig), nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return make(AgentModelConfig), nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse models.json: %w", err)
	}
	return NormalizeModelConfig(raw), nil
}
func WriteModelConfig(path string, cfg AgentModelConfig) error {
	clean := make(AgentModelConfig)
	for k, e := range cfg {
		if !isValidAgentName(k) {
			continue
		}
		me := normalizeModelID(e.Model)
		te := normalizeThinking(string(e.Thinking))
		if me == "" && te == "" {
			continue
		}
		clean[k] = AgentRoutingEntry{Model: me, Thinking: te}
	}
	// sorted MarshalIndent via ordered keys: encode via json.MarshalIndent is already sorted by key for maps
	// but ensure deterministic output by marshaling cleaned sorted representation
	data, err := json.MarshalIndent(clean, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// atomic tmp→rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func MergeModelConfigs(layers ...AgentModelConfig) AgentModelConfig {
	out := make(AgentModelConfig)
	keys := map[string]bool{}
	for _, l := range layers {
		for k := range l {
			keys[k] = true
		}
	}
	for k := range keys {
		for _, l := range layers {
			if e, ok := l[k]; ok && (e.Model != "" || e.Thinking != "") {
				out[k] = e
				break
			}
		}
	}
	return out
}
func EffectiveThinking(entry, global ThinkingLevel) ThinkingLevel {
	if entry == ThinkingInherit || entry == "" {
		if global == "" {
			return ThinkingInherit
		}
		return global
	}
	return entry
}
func SetThinking(cfg AgentModelConfig, agent string, level ThinkingLevel) {
	e := cfg[agent]
	if level == "" {
		e.Thinking = ""
	} else {
		e.Thinking = level
	}
	if e.Model == "" && e.Thinking == "" {
		delete(cfg, agent)
	} else {
		cfg[agent] = e
	}
}
func MarshalModelEnvelope(cfg AgentModelConfig) ([]byte, error) {
	env := ModelRoutingEnvelope{Kind: ModelExportKind, Version: ModelExportVersion, Agents: cfg}
	return json.MarshalIndent(env, "", "  ")
}
func ParseModelEnvelope(data []byte) (AgentModelConfig, error) {
	var env ModelRoutingEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	if env.Kind != ModelExportKind || env.Version != ModelExportVersion {
		return nil, fmt.Errorf("invalid envelope kind/version")
	}
	if env.Agents == nil {
		return make(AgentModelConfig), nil
	}
	clean := make(AgentModelConfig)
	for k, e := range env.Agents {
		if !isValidAgentName(k) {
			continue
		}
		me := normalizeModelID(e.Model)
		te := normalizeThinking(string(e.Thinking))
		if me == "" && te == "" && (e.Model != "" || e.Thinking != "") {
			continue
		}
		clean[k] = AgentRoutingEntry{Model: me, Thinking: te}
	}
	return clean, nil
}
func UpdateFrontmatterRouting(content string, entry *AgentRoutingEntry) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	end := strings.Index(content[4:], "\n---")
	if end == -1 {
		return content
	}
	end += 4
	front := content[4:end]
	body := content[end:]
	lines := strings.Split(front, "\n")
	filtered := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.HasPrefix(l, "model:") || strings.HasPrefix(l, "thinking:") {
			continue
		}
		filtered = append(filtered, l)
	}
	if entry != nil && (entry.Model != "" || entry.Thinking != "") {
		toInsert := []string{}
		if entry.Model != "" {
			toInsert = append(toInsert, "model: "+entry.Model)
		}
		if entry.Thinking != "" {
			toInsert = append(toInsert, "thinking: "+string(entry.Thinking))
		}
		idx := -1
		for i, l := range filtered {
			if strings.HasPrefix(l, "description:") {
				idx = i
				break
			}
		}
		ins := 0
		if idx >= 0 {
			ins = idx + 1
		} else if len(filtered) > 0 {
			ins = 1
			if ins > len(filtered) {
				ins = len(filtered)
			}
		}
		n := make([]string, 0, len(filtered)+len(toInsert))
		n = append(n, filtered[:ins]...)
		n = append(n, toInsert...)
		n = append(n, filtered[ins:]...)
		filtered = n
	}
	return "---\n" + strings.Join(filtered, "\n") + body
}
func PickerAgentFiles() []string {
	base := ConfigurableAgentPhases()
	if len(base) >= 30 {
		return base[:30]
	}
	out := make([]string, 0, 30)
	out = append(out, base...)
	for i := len(base); len(out) < 30; i++ {
		out = append(out, fmt.Sprintf("agent-%02d", i+1))
	}
	return out
}
