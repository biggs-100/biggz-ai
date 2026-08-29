package sdd

import (
	"encoding/json"
	"fmt"
	"strings"
)

// StatusContractV2 is the sole supported contract. Duplicate definition
// guard: status.go also defines it; this file re-exports for projection.
// (If status.go defines StatusContractV2, this will be duplicate; caller
// should not re-define. We keep projection helpers here.)

// ArtifactStore identifies the planning artifact store mode.
type ArtifactStore string

const (
	ArtifactStoreOpenSpec ArtifactStore = "openspec"
	ArtifactStoreEngram   ArtifactStore = "engram"
	ArtifactStoreBigMem   ArtifactStore = "bigmem"
	ArtifactStoreNone     ArtifactStore = "none"
)

// Alias invariant: engram == bigmem — both refer to the same BigMem store.
// Drift that renames one without the other must be detected by orchestrator.test.go.

func IsEngramStore(s ArtifactStore) bool { return s == ArtifactStoreEngram || s == ArtifactStoreBigMem }

func NormalizeArtifactStore(s ArtifactStore) ArtifactStore {
	if s == ArtifactStoreBigMem {
		return ArtifactStoreEngram
	}
	return s
}

// Relationships tracks cross-change links (empty by default).
type Relationships struct {
	DependsOn               []string `json:"dependsOn"`
	Supersedes              []string `json:"supersedes"`
	Amends                  []string `json:"amends"`
	ConflictsWith           []string `json:"conflictsWith"`
	SameDomainActiveChanges []string `json:"sameDomainActiveChanges"`
}

// ReviewOfferBlock is a fresh post-verification invitation. It contains no
// candidate identity or persisted review authority.
type ReviewOfferBlock struct {
	Available  bool   `json:"available"`
	Invocation string `json:"invocation"`
}

// StatusV2Projection is the complete public SDD status document. It projects
// only SDD planning, task, verification, action, and relationship truth.
type StatusV2Projection struct {
	SchemaName        string                      `json:"schemaName"`
	SchemaVersion     int                         `json:"schemaVersion"`
	ChangeName        *string                     `json:"changeName,omitempty"`
	ArtifactStore     ArtifactStore               `json:"artifactStore"`
	PlanningHome      PlanningHome                `json:"planningHome"`
	ChangeRoot        *string                     `json:"changeRoot,omitempty"`
	ArtifactPaths     ArtifactPaths               `json:"artifactPaths"`
	ContextFiles      ArtifactPaths               `json:"contextFiles"`
	Artifacts         map[string]ArtifactState    `json:"artifacts"`
	TaskProgress      TaskProgress                `json:"taskProgress"`
	Dependencies      Dependencies                `json:"dependencies"`
	ApplyState        ApplyState                  `json:"applyState"`
	ActionContext     ActionContext               `json:"actionContext"`
	Relationships     Relationships               `json:"relationships"`
	RemediationState  RemediationState            `json:"remediationState"`
	ReviewOffer       *ReviewOfferBlock           `json:"reviewOffer,omitempty"`
	Consent           *EditAuthorityConsentResult `json:"consent,omitempty"`
	PhaseInstructions *PhaseInstructions          `json:"phaseInstructions,omitempty"`
	NextRecommended   string                      `json:"nextRecommended"`
	BlockedReasons    []string                    `json:"blockedReasons"`
}

// CommandArgs carries parsed sdd-status CLI arguments.
type CommandArgs struct {
	ChangeName          string
	CWD                 string
	JSON                bool
	IncludeInstructions bool
	Contract            string
}

// ParseCommandArgs parses sdd-status arguments. Default contract is v2.
// Requests for v1 or unknown contracts fail read-only with the fresh
// instruction.
func ParseCommandArgs(args []string) (CommandArgs, error) {
	parsed := CommandArgs{Contract: StatusContractV2}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if handled, err := parseKnownFlag(arg, args, &i, &parsed); handled {
			if err != nil {
				return CommandArgs{}, err
			}
			continue
		}
		if isContractEqualsArg(arg) {
			if err := parseContractEqualsArg(arg, &parsed); err != nil {
				return CommandArgs{}, err
			}
			continue
		}
		if err := parsePositionalArg(arg, &parsed); err != nil {
			return CommandArgs{}, err
		}
	}
	if err := validateStatusContract(parsed.Contract); err != nil {
		return CommandArgs{}, err
	}
	return parsed, nil
}

func parseKnownFlag(arg string, args []string, idx *int, parsed *CommandArgs) (bool, error) {
	switch arg {
	case "--json":
		parsed.JSON = true
		return true, nil
	case "--instructions":
		parsed.IncludeInstructions = true
		return true, nil
	case "--cwd":
		if *idx+1 >= len(args) || startsWithDash(args[*idx+1]) {
			return true, fmt.Errorf("--cwd requires a value")
		}
		parsed.CWD = args[*idx+1]
		*idx++
		return true, nil
	case "--contract":
		if *idx+1 >= len(args) || startsWithDash(args[*idx+1]) {
			return true, fmt.Errorf("--contract requires a value")
		}
		parsed.Contract = args[*idx+1]
		*idx++
		if err := validateStatusContract(parsed.Contract); err != nil {
			return true, err
		}
		return true, nil
	default:
		return false, nil
	}
}

func isContractEqualsArg(arg string) bool { return len(arg) > 11 && arg[:11] == "--contract=" }

func parseContractEqualsArg(arg string, parsed *CommandArgs) error {
	parsed.Contract = arg[11:]
	return validateStatusContract(parsed.Contract)
}

func parsePositionalArg(arg string, parsed *CommandArgs) error {
	if len(arg) > 0 && arg[0] == '-' {
		return fmt.Errorf("unknown sdd-status argument %q", arg)
	}
	if parsed.ChangeName == "" {
		parsed.ChangeName = arg
		return nil
	}
	return fmt.Errorf("unexpected sdd-status argument %q", arg)
}

func startsWithDash(s string) bool { return len(s) > 0 && s[0] == '-' }

func validateStatusContract(contract string) error {
	if contract == StatusContractV2 {
		return nil
	}
	return fmt.Errorf("unsupported sdd-status contract %q. Start a fresh implementation state and rerun `biggz sdd-status --contract biggz-ai.sdd-status/v2`.", contract)
}

// ProjectStatusV2 projects a ChangeStatus to the authority-free V2 document.
// It rejects unsupported identities, stores, states, and artifact values
// rather than silently broadening the public document.
func ProjectStatusV2(status ChangeStatus) (StatusV2Projection, error) {
	if status.SchemaName != StatusSchemaName || status.SchemaVersion != StatusSchemaVersion {
		return StatusV2Projection{}, fmt.Errorf("unsupported SDD status identity %q@%d", status.SchemaName, status.SchemaVersion)
	}
	// Artifact store is derived from PlanningHome for backward compat:
	// repo-local => openspec; if status already carries explicit store, validate it.
	store := ArtifactStoreOpenSpec
	if status.ArtifactStore != "" {
		if !isValidArtifactStore(status.ArtifactStore) {
			return StatusV2Projection{}, fmt.Errorf("unsupported SDD v2 artifact store %q", status.ArtifactStore)
		}
		store = status.ArtifactStore
	}
	if !isValidApplyState(status.ApplyState) {
		return StatusV2Projection{}, fmt.Errorf("unsupported SDD v2 apply state %q", status.ApplyState)
	}
	if !isValidNextRecommended(status.NextRecommended) {
		return StatusV2Projection{}, fmt.Errorf("unsupported SDD v2 next action %q", status.NextRecommended)
	}
	artifacts, err := projectArtifactsV2(store, status.Artifacts)
	if err != nil {
		return StatusV2Projection{}, err
	}
	// Determine artifactStore for projection
	artifactStore := store
	var changeName *string
	if status.Name != "" {
		n := status.Name
		changeName = &n
	}
	var changeRoot *string
	if status.ChangeRoot != "" {
		r := status.ChangeRoot
		changeRoot = &r
	}
	// Authority-free: strip edit-authority blocked reasons and never report
	// resolve-blockers solely due to authority. sdd-status never blocks on
	// edit authority; sdd-apply warns via EditAuthorityBlocked/Consent.
	filteredReasons := make([]string, 0, len(status.BlockedReasons))
	for _, r := range status.BlockedReasons {
		if strings.Contains(r, "edit_authority_missing") || strings.Contains(r, "blocked(edit_authority_missing)") {
			continue
		}
		filteredReasons = append(filteredReasons, r)
	}
	nextRecommended := status.NextRecommended
	if nextRecommended == "resolve-blockers" && len(filteredReasons) == 0 {
		// Authority was the sole blocker; V2 never forces resolve-blockers for authority.
		// Map to apply when ApplyState is ready, otherwise preserve non-blocked next.
		if status.ApplyState == ApplyReady {
			nextRecommended = "apply"
		} else if status.ApplyState == ApplyAllDone {
			nextRecommended = "verify"
		} else {
			nextRecommended = "apply"
		}
	}
	if filteredReasons == nil {
		filteredReasons = []string{}
	}
	projected := StatusV2Projection{
		SchemaName:       status.SchemaName,
		SchemaVersion:    status.SchemaVersion,
		ChangeName:       changeName,
		ArtifactStore:    artifactStore,
		PlanningHome:     status.PlanningHome,
		ChangeRoot:       changeRoot,
		ArtifactPaths:    status.ArtifactPaths,
		ContextFiles:     status.ContextFiles,
		Artifacts:        artifacts,
		TaskProgress:     status.TaskProgress,
		Dependencies:     status.Dependencies,
		ApplyState:       status.ApplyState,
		ActionContext:    status.ActionContext,
		Relationships:    status.Relationships,
		RemediationState: status.RemediationState,
		ReviewOffer:      status.ReviewOffer,
		Consent:          status.Consent,
		NextRecommended:  nextRecommended,
		BlockedReasons:   filteredReasons,
	}
	if status.PhaseInstructions != nil {
		projected.PhaseInstructions = status.PhaseInstructions
	}
	return projected, nil
}

func projectArtifactsV2(store ArtifactStore, source map[string]ArtifactState) (map[string]ArtifactState, error) {
	keys := artifactStateKeys(store)
	projected := make(map[string]ArtifactState, len(keys))
	for _, key := range keys {
		state, ok := source[key]
		if !ok || !isValidArtifactState(state) {
			return nil, fmt.Errorf("unsupported SDD v2 artifact %q state %q", key, state)
		}
		projected[key] = state
	}
	return projected, nil
}

func artifactStateKeys(store ArtifactStore) []string {
	// V2 allowlist: only planning/tasks/verification keys
	return []string{"proposal", "specs", "design", "tasks", "applyProgress", "verifyReport"}
}

func isValidArtifactStore(value ArtifactStore) bool {
	return value == ArtifactStoreOpenSpec || value == ArtifactStoreEngram || value == ArtifactStoreBigMem || value == ArtifactStoreNone
}

func isValidArtifactState(value ArtifactState) bool {
	return value == ArtifactMissing || value == ArtifactPartial || value == ArtifactDone
}

func isValidApplyState(value ApplyState) bool {
	return value == ApplyBlocked || value == ApplyReady || value == ApplyAllDone
}

func isValidNextRecommended(value string) bool {
	switch value {
	case "apply", "verify", "remediate", "archive", "resolve-blockers", "sdd-new", "select-change", "propose", "spec", "design", "tasks", "done":
		return true
	default:
		return false
	}
}

func marshalStatusV2Indent(status ChangeStatus) ([]byte, error) {
	projected, err := ProjectStatusV2(status)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(projected, "", "  ")
}
