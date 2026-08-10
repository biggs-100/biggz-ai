package opencode

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/model"
)

// configurableAgentSet is the set of valid agent names that may appear in
// opencode.json with a model assignment. It includes the orchestrator, SDD,
// Judgment Day, and review agents, plus the backward-compatible
// "sdd-orchestrator" alias.
var configurableAgentSet = buildConfigurableAgentSet()

func buildConfigurableAgentSet() map[string]bool {
	set := make(map[string]bool, len(ConfigurableAgentPhases())+1)
	for _, p := range ConfigurableAgentPhases() {
		set[p] = true
	}
	// Backward-compatible read alias for configs that have not been synced yet.
	set["sdd-orchestrator"] = true
	return set
}

// ReadCurrentModelAssignments reads the agent definitions from opencode.json at
// settingsPath and extracts the "model" and "variant" fields for each
// configurable agent.
//
// Only agents whose names match a configurable agent (orchestrator, SDD phases,
// JD agents, review agents — see ConfigurableAgentPhases) are included. The
// legacy "sdd-orchestrator" key is mapped to OrchestratorAgent. Agents without a
// valid model spec are silently skipped.
//
// Returns an empty map (no error) when the file does not exist, contains no
// "agent" key, is not parseable JSON, or has no matching agents with a valid
// model field. The file is read with the JSONC-safe filemerge helpers so
// comments and trailing commas never break the parse.
func ReadCurrentModelAssignments(settingsPath string) (map[string]model.ModelAssignment, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]model.ModelAssignment{}, nil
		}
		return nil, err
	}

	root, err := filemerge.UnmarshalJSONObject(data)
	if err != nil {
		// Unparseable JSON — return empty map, no error.
		return map[string]model.ModelAssignment{}, nil
	}

	agentRaw, ok := root["agent"]
	if !ok {
		return map[string]model.ModelAssignment{}, nil
	}
	agentMap, ok := agentRaw.(map[string]any)
	if !ok {
		return map[string]model.ModelAssignment{}, nil
	}

	result := make(map[string]model.ModelAssignment)
	for name, defRaw := range agentMap {
		if !configurableAgentSet[name] {
			continue
		}
		defMap, ok := defRaw.(map[string]any)
		if !ok {
			continue
		}
		modelStr, ok := defMap["model"].(string)
		if !ok || modelStr == "" {
			continue
		}
		providerID, modelID, ok := model.SplitModelSpec(modelStr)
		if !ok {
			continue
		}
		assignmentKey := name
		if name == "sdd-orchestrator" {
			assignmentKey = OrchestratorAgent
			if _, hasOrchestrator := result[assignmentKey]; hasOrchestrator {
				continue
			}
		}
		effort, _ := defMap["variant"].(string)
		result[assignmentKey] = model.ModelAssignment{
			ProviderID: providerID,
			ModelID:    modelID,
			Effort:     effort,
		}
	}

	return result, nil
}

// jdAgentSet is a set for O(1) judgment-day agent membership checks.
var jdAgentSet = buildJDAgentSet()

func buildJDAgentSet() map[string]bool {
	set := make(map[string]bool, len(JDPhases()))
	for _, p := range JDPhases() {
		set[p] = true
	}
	return set
}

// isJDAgent reports whether the agent name is a judgment-day workflow agent.
// JD agents are excluded from root model fallback to preserve independent
// model configuration for diversity of perspective between judges.
func isJDAgent(name string) bool {
	return jdAgentSet[name]
}

// InjectModelAssignments injects "model" and "variant" fields into sub-agent
// definitions within the overlay JSON before it is merged into the settings
// file.
//
// Decision tree for EACH sub-agent:
//  1. TUI assignment exists for this agent → use it (always wins); variant is
//     set to Effort or "" to stay symmetric and prevent stale variant leakage.
//  2. Agent already exists as a key in the user's existing opencode.json
//     (existingAgentKeys) → skip; the deep merge preserves whatever the user
//     already has (including no model at all — that's intentional).
//  3. Neither of the above AND rootModelID is set → inject rootModelID so the
//     agent does not silently inherit the orchestrator model at runtime, and
//     write variant="" for symmetry with case 1. JD agents are excluded from
//     root propagation.
//
// If none of the above conditions apply, nothing is written for that agent.
func InjectModelAssignments(overlayBytes []byte, assignments map[string]model.ModelAssignment, rootModelID string, existingAgentKeys []string) ([]byte, error) {
	assignments = normalizeModelAssignments(assignments)

	existing := make(map[string]bool, len(existingAgentKeys))
	for _, k := range existingAgentKeys {
		existing[k] = true
	}

	var overlay map[string]any
	if err := json.Unmarshal(overlayBytes, &overlay); err != nil {
		return nil, fmt.Errorf("unmarshal overlay for model injection: %w", err)
	}

	agentsRaw, ok := overlay["agent"]
	if !ok {
		return overlayBytes, nil
	}
	agents, ok := agentsRaw.(map[string]any)
	if !ok {
		return overlayBytes, nil
	}

	for phase, agentDef := range agents {
		agentMap, ok := agentDef.(map[string]any)
		if !ok {
			continue
		}

		assignment, hasExplicitAssignment := assignments[phase]

		switch {
		case hasExplicitAssignment && assignment.ProviderID != "" && assignment.ModelID != "":
			// 1. TUI choice always wins.
			agentMap["model"] = assignment.FullID()
			agentMap["variant"] = assignment.Effort
		case existing[phase]:
			// 2. Agent already exists in the user's config — let the merge
			// preserve whatever they have.
		case rootModelID != "":
			// 3. Fresh install or new agent: use root model as default to break
			// inheritance. Clear variant explicitly so a stale variant cannot
			// leak through the deep merge. JD agents are excluded to support
			// independent model configuration between judges.
			if !isJDAgent(phase) {
				agentMap["model"] = rootModelID
				agentMap["variant"] = ""
			}
		}
	}

	result, err := json.MarshalIndent(overlay, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal overlay after model injection: %w", err)
	}
	return append(result, '\n'), nil
}

// normalizeModelAssignments accepts the historical "sdd-orchestrator"
// assignment key as an input alias, but maps it to the current coordinator key
// (biggz-orchestrator).
func normalizeModelAssignments(assignments map[string]model.ModelAssignment) map[string]model.ModelAssignment {
	if len(assignments) == 0 {
		return assignments
	}
	legacyAssignment, hasLegacy := assignments["sdd-orchestrator"]
	if !hasLegacy {
		return assignments
	}
	if _, hasOrchestrator := assignments[OrchestratorAgent]; hasOrchestrator {
		return assignments
	}

	normalized := make(map[string]model.ModelAssignment, len(assignments))
	for key, assignment := range assignments {
		if key == "sdd-orchestrator" {
			continue
		}
		normalized[key] = assignment
	}
	normalized[OrchestratorAgent] = legacyAssignment
	return normalized
}
