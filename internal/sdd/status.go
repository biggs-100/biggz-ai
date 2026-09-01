// Package sdd implements native SDD commands for biggz-ai.
//
// These are the backend commands that SDD skills call to read status,
// validate reports, and manage attempts. They make biggz-ai
// self-sufficient without depending on external skills for basic ops.
package sdd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/pathquote"
	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/sddattempt"
	"github.com/charmbracelet/x/ansi"
	"gopkg.in/yaml.v3"
)

// StatusSchemaName identifies the SDD status document emitted by biggz-ai.
const StatusSchemaName = "biggz-ai.sdd-status"

// StatusSchemaVersion is the version of the StatusSchemaName document.
const StatusSchemaVersion = 2

// StatusContractV2 is the sole supported SDD status contract.
const StatusContractV2 = "biggz-ai.sdd-status/v2"

// StatusContractV1 is the retired contract. Requests for it MUST fail
// read-only with the fresh-v2 rerun instruction.
const StatusContractV1 = "biggz-ai.sdd-status/v1"

// ArtifactState describes the content state of one SDD artifact.
type ArtifactState string

const (
	ArtifactMissing ArtifactState = "missing"
	ArtifactPartial ArtifactState = "partial"
	ArtifactDone    ArtifactState = "done"
)

// DependencyState describes one artifact dependency's readiness.
type DependencyState string

const (
	DependencyBlocked DependencyState = "blocked"
	DependencyReady   DependencyState = "ready"
	DependencyAllDone DependencyState = "all_done"
)

// ApplyState describes the apply phase's readiness.
type ApplyState string

const (
	ApplyBlocked ApplyState = "blocked"
	ApplyReady   ApplyState = "ready"
	ApplyAllDone ApplyState = "all_done"
)

// TaskProgress summarizes the markdown task checklist of tasks.md.
type TaskProgress struct {
	Total       int  `json:"total"`
	Completed   int  `json:"completed"`
	Pending     int  `json:"pending"`
	AllComplete bool `json:"allComplete"`
}

// Dependencies reports each phase's readiness derived from artifact states.
type Dependencies struct {
	Proposal DependencyState `json:"proposal"`
	Specs    DependencyState `json:"specs"`
	Design   DependencyState `json:"design"`
	Tasks    DependencyState `json:"tasks"`
	Apply    DependencyState `json:"apply"`
	Verify   DependencyState `json:"verify"`
	Sync     DependencyState `json:"sync"`
	Archive  DependencyState `json:"archive"`
}

// ActionContext carries the edit-authority context an apply actor needs.
type ActionContext struct {
	Mode             string   `json:"mode"`
	WorkspaceRoot    string   `json:"workspaceRoot"`
	AllowedEditRoots []string `json:"allowedEditRoots"`
}

// PlanningHome identifies the planning directory mode and path.
type PlanningHome struct {
	Mode string `json:"mode"`
	Path string `json:"path"`
}

// ArtifactPaths maps each SDD artifact to its on-disk path(s).
type ArtifactPaths struct {
	Proposal      []string `json:"proposal"`
	Specs         []string `json:"specs"`
	Design        []string `json:"design"`
	Tasks         []string `json:"tasks"`
	ApplyProgress []string `json:"applyProgress"`
	VerifyReport  []string `json:"verifyReport"`
}

// RemediationState describes bounded correction eligibility for a failed
// verification verdict. Biggz runs without a review authority, so lineage,
// generation, fix-batch and budget fields are never fabricated: correction
// proceeds unmanaged, bounded by the native runtime attempt budget alone.
type RemediationState struct {
	Required               bool   `json:"required"`
	Complete               bool   `json:"complete"`
	FailedEvidenceRevision string `json:"failedEvidenceRevision"`
	Reason                 string `json:"reason"`
}

// PhaseInstructions renders the per-phase guidance an orchestrator hands a
// sub-agent. Present only when --instructions was requested.
type PhaseInstructions struct {
	Apply     []string `json:"apply"`
	Verify    []string `json:"verify"`
	Remediate []string `json:"remediate"`
	Archive   []string `json:"archive"`
}

// blockerReasons accumulates the two blocked-reason buckets the derivation
// finalizes into the emitted BlockedReasons list.
type blockerReasons struct {
	expectedPlanning []string
	genuine          []string
}

// finalize emits only genuine reasons for planning phases (missing planning
// artifacts are the expected output of planning, not blockers) and
// expectedPlanning + genuine for every other next recommendation.
func (reasons blockerReasons) finalize(nextRecommended string) []string {
	switch nextRecommended {
	case "propose", "spec", "design", "tasks":
		return append([]string{}, reasons.genuine...)
	default:
		return append(append([]string{}, reasons.expectedPlanning...), reasons.genuine...)
	}
}

// StatusOptions configures status output.
type StatusOptions struct {
	ReviewDisabled bool
	// IncludeInstructions requests the phaseInstructions block on every
	// derived ChangeStatus (renderPhaseInstructions).
	IncludeInstructions bool
}

// ChangeStatus represents the state of an SDD change.
//
// The untagged legacy fields (Name, HasProposal, HasSpecs, HasDesign,
// HasTasks, TasksTotal, TasksDone, HasApply, HasVerify, IsArchived) serialize
// under their PascalCase names exactly as before the derivation port; they
// remain the file-probe read-compatibility surface. The camelCase fields
// below are the derived authority (schemaName, artifacts, taskProgress,
// dependencies, applyState, actionContext, remediationState,
// nextRecommended, blockedReasons, phaseInstructions).
type ChangeStatus struct {
	SchemaName        string                   `json:"schemaName,omitempty"`
	SchemaVersion     int                      `json:"schemaVersion,omitempty"`
	ChangeRoot        string                   `json:"changeRoot,omitempty"`
	PlanningHome      PlanningHome             `json:"planningHome,omitempty"`
	ArtifactStore     ArtifactStore            `json:"artifactStore,omitempty"`
	ArtifactPaths     ArtifactPaths            `json:"artifactPaths,omitempty"`
	ContextFiles      ArtifactPaths            `json:"contextFiles,omitempty"`
	Artifacts         map[string]ArtifactState `json:"artifacts,omitempty"`
	TaskProgress      TaskProgress             `json:"taskProgress,omitempty"`
	Dependencies      Dependencies             `json:"dependencies,omitempty"`
	ApplyState        ApplyState               `json:"applyState,omitempty"`
	ActionContext     ActionContext            `json:"actionContext,omitempty"`
	Relationships     Relationships            `json:"relationships,omitempty"`
	RemediationState  RemediationState         `json:"remediationState,omitempty"`
	ReviewOffer       *ReviewOfferBlock        `json:"reviewOffer,omitempty"`
	NextRecommended   string                   `json:"nextRecommended,omitempty"`
	BlockedReasons    []string                 `json:"blockedReasons,omitempty"`
	PhaseInstructions *PhaseInstructions       `json:"phaseInstructions,omitempty"`

	Name        string
	HasProposal bool
	HasSpecs    bool
	HasDesign   bool
	HasTasks    bool
	TasksTotal  int
	TasksDone   int
	HasApply    bool
	HasVerify   bool
	IsArchived  bool

	// Edit-authority surface (all omitempty / zero-value empty): a change
	// whose task plan targets repository roots outside the authorized edit
	// roots reports the block, the missing roots, the granted roots the
	// ledger projects for its change-instance identity, and the typed
	// consent envelope naming the runnable grant invocation.
	GrantedRoots         []string                    `json:"granted_roots,omitempty"`
	EditAuthorityBlocked bool                        `json:"edit_authority_blocked,omitempty"`
	MissingRoots         []string                    `json:"missing_roots,omitempty"`
	Consent              *EditAuthorityConsentResult `json:"consent,omitempty"`
}

// Status scans the openspec/changes directory and returns the status
// of all active (non-archived) changes, plus the most recent archived ones.
func Status(openspecRoot string) (active []ChangeStatus, archived []ChangeStatus, err error) {
	return StatusWithOptions(openspecRoot, StatusOptions{})
}

// StatusWithOptions scans like Status, deriving the structured status
// (artifacts, taskProgress, dependencies, applyState, nextRecommended,
// blockedReasons) for every change. IncludeInstructions additionally renders
// the phaseInstructions block on each derived ChangeStatus.
func StatusWithOptions(openspecRoot string, opts StatusOptions) (active []ChangeStatus, archived []ChangeStatus, err error) {
	workspaceRoot := filepath.Dir(openspecRoot)
	changesDir := filepath.Join(openspecRoot, "changes")
	archiveDir := filepath.Join(changesDir, "archive")

	active, err = collectActiveChanges(changesDir, workspaceRoot, opts.IncludeInstructions)
	if err != nil {
		return nil, nil, err
	}
	archived, err = collectArchivedChanges(archiveDir, workspaceRoot, opts.IncludeInstructions)
	if err != nil {
		return active, nil, err
	}
	return applyStoreRouting(active, archived, workspaceRoot, opts.IncludeInstructions)
}

func collectActiveChanges(changesDir, workspaceRoot string, includeInstructions bool) ([]ChangeStatus, error) {
	entries, err := os.ReadDir(changesDir)
	if err != nil {
		return nil, fmt.Errorf("read changes dir: %w", err)
	}
	var active []ChangeStatus
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "archive" {
			continue
		}
		cs, err := readChange(filepath.Join(changesDir, entry.Name()), entry.Name(), false, workspaceRoot, includeInstructions)
		if err != nil {
			continue
		}
		active = append(active, cs)
	}
	return active, nil
}

func collectArchivedChanges(archiveDir, workspaceRoot string, includeInstructions bool) ([]ChangeStatus, error) {
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read archive dir: %w", err)
	}
	var archived []ChangeStatus
	for i := len(entries) - 1; i >= 0 && len(archived) < 3; i-- {
		entry := entries[i]
		if !entry.IsDir() {
			continue
		}
		cs, err := readChange(filepath.Join(archiveDir, entry.Name()), entry.Name(), true, workspaceRoot, includeInstructions)
		if err != nil {
			continue
		}
		archived = append(archived, cs)
	}
	return archived, nil
}

func applyStoreRouting(active, archived []ChangeStatus, workspaceRoot string, includeInstructions bool) ([]ChangeStatus, []ChangeStatus, error) {
	store := declaredArtifactStore(workspaceRoot)
	if store == "" {
		clearArtifactStoreFields(active, archived)
		return active, archived, nil
	}
	if IsEngramStore(store) {
		if memActive, memArchived, err := collectBigMemChangesWithArchive(workspaceRoot, includeInstructions); err == nil {
			return memActive, memArchived, nil
		}
		return nil, nil, nil
	}
	if memActive, memArchived, err := collectBigMemChangesWithArchive(workspaceRoot, includeInstructions); err == nil && len(memActive)+len(memArchived) > 0 {
		active, archived = mergeFilesystemAndBigMem(active, archived, memActive, memArchived)
	}
	return active, archived, nil
}

func clearArtifactStoreFields(active, archived []ChangeStatus) {
	for i := range active {
		active[i].ArtifactPaths = ArtifactPaths{}
		active[i].ContextFiles = ArtifactPaths{}
		active[i].ArtifactStore = ArtifactStore("")
	}
	for i := range archived {
		archived[i].ArtifactPaths = ArtifactPaths{}
		archived[i].ContextFiles = ArtifactPaths{}
		archived[i].ArtifactStore = ArtifactStore("")
	}
}

func probeArtifacts(dir, workspaceRoot, name string, cs *ChangeStatus) {
	cs.HasProposal = fileExists(filepath.Join(dir, "proposal.md"))
	cs.HasDesign = fileExists(filepath.Join(dir, "design.md"))
	cs.HasApply = fileExists(filepath.Join(dir, "apply-progress.md"))
	verifyProbe := filepath.Join(dir, verifyReportFileName)
	if canonicalRel, err := canonicalVerifyReportPaths(workspaceRoot, workspaceRoot, dir, name); err == nil && canonicalRel != "" {
		cs.HasVerify = fileExists(verifyProbe)
	} else {
		cs.HasVerify = fileExists(verifyProbe)
	}
	if specEntries, err := os.ReadDir(filepath.Join(dir, "specs")); err == nil && len(specEntries) > 0 {
		cs.HasSpecs = true
	}
}

func countTasksFromText(tasksText string) (total, done int) {
	for _, line := range strings.Split(tasksText, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- [") {
			continue
		}
		total++
		if strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]") {
			done++
		}
	}
	return total, done
}

func loadTasksInfo(dir string) (hasTasks bool, tasksText string, total, done int) {
	hasTasks = fileExists(filepath.Join(dir, "tasks.md"))
	if !hasTasks {
		return false, "", 0, 0
	}
	data, err := os.ReadFile(filepath.Join(dir, "tasks.md"))
	if err != nil {
		return true, "", 0, 0
	}
	tasksText = string(data)
	total, done = countTasksFromText(tasksText)
	return hasTasks, tasksText, total, done
}

func readGrantedRoots(name, workspaceRoot, instance string) (granted []string, expectedRevision string) {
	if instance == "" {
		return nil, ""
	}
	status, err := sddattempt.StatusWithInstance(name, workspaceRoot, instance)
	if err != nil {
		return nil, ""
	}
	return status.GrantedRoots, status.Revision
}

func applyEditAuthorityForChange(dir, name, workspaceRoot, tasksText string, cs *ChangeStatus) error {
	instance, err := readChangeInstanceMarker(dir)
	if err != nil {
		return fmt.Errorf("read change-instance marker for %s: %w", name, err)
	}
	granted, expectedRevision := readGrantedRoots(name, workspaceRoot, instance)
	allowed := make([]string, 0, 1+len(granted))
	allowed = append(allowed, workspaceRoot)
	allowed = append(allowed, granted...)
	missing := detectUnauthorizedEditRoots(tasksText, workspaceRoot, allowed)
	if len(missing) > 0 {
		if instance == "" {
			instance, err = ensureChangeInstanceMarker(dir)
			if err != nil {
				return fmt.Errorf("mint change-instance marker for %s: %w", name, err)
			}
			granted, expectedRevision = readGrantedRoots(name, workspaceRoot, instance)
		}
		cs.EditAuthorityBlocked = true
		cs.MissingRoots = missing
		cs.Consent = newEditAuthorityConsent(name, workspaceRoot, missing, instance, expectedRevision)
	}
	cs.GrantedRoots = granted
	return nil
}

func readChange(dir, name string, isArchived bool, workspaceRoot string, includeInstructions bool) (ChangeStatus, error) {
	cs := ChangeStatus{Name: name, IsArchived: isArchived}
	probeArtifacts(dir, workspaceRoot, name, &cs)
	hasTasks, tasksText, total, done := loadTasksInfo(dir)
	cs.HasTasks = hasTasks
	cs.TasksTotal = total
	cs.TasksDone = done
	if tasksText != "" {
		if err := applyEditAuthorityForChange(dir, name, workspaceRoot, tasksText, &cs); err != nil {
			return cs, err
		}
	}
	if err := deriveChangeStatus(&cs, dir, workspaceRoot, includeInstructions); err != nil {
		return cs, err
	}
	return cs, nil
}

func collectArtifactDerivation(changeDir string, store ArtifactStore) (ArtifactPaths, map[string]ArtifactState, TaskProgress, string, SpecCounts, verifyResultEvaluation, error) {
	artifactPaths := resolveArtifactPaths(changeDir, store)
	artifacts := map[string]ArtifactState{
		"proposal":      singleArtifactState(artifactPaths.Proposal),
		"specs":         multiArtifactState(artifactPaths.Specs, filepath.Join(changeDir, "specs")),
		"design":        singleArtifactState(artifactPaths.Design),
		"tasks":         singleArtifactState(artifactPaths.Tasks),
		"applyProgress": singleArtifactState(artifactPaths.ApplyProgress),
		"verifyReport":  singleArtifactState(artifactPaths.VerifyReport),
	}
	tasksContent := readText(firstPath(artifactPaths.Tasks))
	taskProgress := countTaskProgressText(tasksContent)
	specCounts, err := readSpecCounts(artifactPaths.Specs)
	if err != nil {
		return ArtifactPaths{}, nil, TaskProgress{}, "", SpecCounts{}, verifyResultEvaluation{}, err
	}
	verifyResult := readVerifyResult(firstPath(artifactPaths.VerifyReport), specCounts)
	return artifactPaths, artifacts, taskProgress, tasksContent, specCounts, verifyResult, nil
}

func polishRemediationReason(baseReason, changeName, workspaceRoot, instance, evidenceRevision string) string {
	reason := baseReason
	store, err := sddattempt.LoadStore(changeName, workspaceRoot)
	if err != nil || len(store.Attempts) == 0 {
		return reason
	}
	last := store.Attempts[len(store.Attempts)-1]
	if last.RemediatesEvidenceRevision != evidenceRevision {
		return reason
	}
	switch last.Outcome {
	case "interrupted":
		reason += " (last correction interrupted — original failure still bindable)"
	case "failed":
		if last.EvidenceRevision != "" && last.EvidenceRevision != evidenceRevision {
			reason += fmt.Sprintf(" (last correction failed — new failure %s now bindable)", last.EvidenceRevision)
		} else {
			reason += " (last correction failed — new failure now bindable)"
		}
	}
	return reason
}

func buildRemediationState(changeDir, changeName, workspaceRoot string, artifacts map[string]ArtifactState, applyState ApplyState, verifyResult verifyResultEvaluation, blockedReasons *blockerReasons) (RemediationState, bool, bool, string, error) {
	verifyReportCurrent := artifacts["verifyReport"] == ArtifactDone
	instance, err := readChangeInstanceMarker(changeDir)
	if err != nil {
		return RemediationState{}, false, false, "", fmt.Errorf("read change-instance marker for %s: %w", changeName, err)
	}
	remediationComplete := sddattempt.RemediationComplete(changeName, workspaceRoot, instance, verifyResult.EvidenceRevision)
	staleDecision := isStaleDecisionRequired(changeName, workspaceRoot, instance)
	remediationState := RemediationState{}
	if verifyReportCurrent && !verifyResult.Passing && applyState == ApplyAllDone && !remediationComplete && !staleDecision {
		baseReason := fmt.Sprintf("verify evidence requires unmanaged remediation for %s: %s; receipt-driven review is disabled, so this correction is bounded by the native runtime attempt budget alone", verifyResult.EvidenceRevision, verifyResult.Reason)
		reason := polishRemediationReason(baseReason, changeName, workspaceRoot, instance, verifyResult.EvidenceRevision)
		remediationState = RemediationState{Required: true, FailedEvidenceRevision: verifyResult.EvidenceRevision, Reason: reason}
	}
	if remediationState.Reason != "" {
		blockedReasons.genuine = append(blockedReasons.genuine, remediationState.Reason)
	}
	return remediationState, staleDecision, remediationComplete, instance, nil
}

// deriveChangeStatus ports gentle-ai's sdd-status derivation authority:
// artifact states, task progress, spec counts, verify evaluation, apply
// state, edit-authority blocking, reduced (unmanaged) remediation,
// dependencies, nextRecommended, blocked reasons, and phase instructions.
// It runs at the END of readChange, after the legacy file probe and the
// change-instance/grant read, and is skipped for archived changes.
func deriveChangeStatus(cs *ChangeStatus, changeDir, workspaceRoot string, includeInstructions bool) error {
	if cs.IsArchived {
		cs.NextRecommended = "done"
		return nil
	}
	store := declaredArtifactStore(workspaceRoot)
	artifactPaths, artifacts, taskProgress, tasksContent, _, verifyResult, err := collectArtifactDerivation(changeDir, store)
	if err != nil {
		return err
	}
	coreReady := artifacts["proposal"] == ArtifactDone && artifacts["specs"] == ArtifactDone &&
		artifacts["design"] == ArtifactDone && artifacts["tasks"] == ArtifactDone && taskProgress.Total > 0
	applyState := resolveApplyState(coreReady, taskProgress)
	blockedReasons := artifactBlockedReasons(artifacts, taskProgress)
	allowedEditRoots := make([]string, 0, 1+len(cs.GrantedRoots))
	allowedEditRoots = append(allowedEditRoots, workspaceRoot)
	allowedEditRoots = append(allowedEditRoots, cs.GrantedRoots...)
	applyState = applyEditAuthorityBlock(applyState, &blockedReasons, tasksContent, workspaceRoot, allowedEditRoots)
	// Topology guard: foreign common-dir check memoized per Status, block only apply/verify/remediate
	if tasksContent != "" && coreReady {
		memo := make(map[string]string)
		foreignRoots := foreignRuntimeTopologyRoots(tasksContent, workspaceRoot, allowedEditRoots, memo)
		if len(foreignRoots) > 0 {
			// Only block when eligible for apply/verify/remediate (i.e., not already blocked by missing artifacts)
			// coreReady ensures planning complete, so next would be apply/verify etc.
			if applyState != ApplyBlocked {
				applyState = ApplyBlocked
			}
			blockedReasons.genuine = append(blockedReasons.genuine, "cross_common_dir_runtime_target: tasks.md references repositories outside planning common dir: "+strings.Join(foreignRoots, ", "))
		}
	}
	verifyReportCurrent := artifacts["verifyReport"] == ArtifactDone
	remediationState, staleDecision, remediationComplete, _, err := buildRemediationState(changeDir, cs.Name, workspaceRoot, artifacts, applyState, verifyResult, &blockedReasons)
	if err != nil {
		return err
	}

	// When staleDecision is true, treat remediation as complete for
	// dependency routing so verify/archive are DependencyReady instead of
	// blocked, even though the ledger still reports DecisionRequired.
	effectiveRemediationComplete := remediationComplete || staleDecision
	dependencies := resolveDependencies(artifacts, taskProgress, applyState, coreReady, verifyReportCurrent, verifyResult.Passing, effectiveRemediationComplete)
	// Sync routing between verify and archive
	syncState, syncReasons := deriveSyncState(cs.Name, workspaceRoot, changeDir, store, verifyResult, tasksContent, artifacts, applyState)
	dependencies.Sync = syncState
	for _, r := range syncReasons {
		blockedReasons.genuine = append(blockedReasons.genuine, r)
	}
	// Apply the stale-decision native routing to dependencies: free
	// verify/archive when the probe says the decision block is stale.
	dependencies = applyStaleDecisionRouting(dependencies, staleDecision)
	nextRecommended := resolveNextRecommended(dependencies, applyState, verifyReportCurrent, remediationState)

	cs.SchemaName = StatusSchemaName
	cs.SchemaVersion = StatusSchemaVersion
	cs.ChangeRoot = changeDir
	cs.PlanningHome = PlanningHome{Mode: "repo-local", Path: filepath.Join(workspaceRoot, "openspec")}
	cs.ArtifactStore = store
	cs.ArtifactPaths = artifactPaths
	cs.ContextFiles = artifactPaths
	cs.Artifacts = artifacts
	cs.TaskProgress = taskProgress
	cs.Dependencies = dependencies
	cs.ApplyState = applyState
	cs.ActionContext = ActionContext{Mode: "repo-local", WorkspaceRoot: workspaceRoot, AllowedEditRoots: allowedEditRoots}
	cs.Relationships = Relationships{}
	cs.RemediationState = remediationState
	cs.ReviewOffer = deriveReviewOffer(cs.Name, workspaceRoot, applyState, artifacts, verifyResult)
	cs.NextRecommended = nextRecommended
	cs.BlockedReasons = blockedReasons.finalize(nextRecommended)
	if includeInstructions {
		instructions := renderPhaseInstructions(*cs)
		cs.PhaseInstructions = &instructions
	}
	return nil
}

// deriveReviewOffer emits a fresh post-verification invitation iff
// applyState==all_done && verifyReport==done && passing && RDD enabled.
// No lineage/binding/receipt is persisted, only the quoted invocation.
func deriveReviewOffer(changeName, workspaceRoot string, applyState ApplyState, artifacts map[string]ArtifactState, vr verifyResultEvaluation) *ReviewOfferBlock {
	if applyState != ApplyAllDone {
		return nil
	}
	if artifacts["verifyReport"] != ArtifactDone {
		return nil
	}
	if !vr.Passing {
		return nil
	}
	if !isRDDEnabled(workspaceRoot) {
		return nil
	}
	shortSHA := shortSHAForWorkspace(workspaceRoot)
	if shortSHA == "" {
		shortSHA = "unknown"
	}
	invocation := fmt.Sprintf("biggz review start --lineage %s", pathquote.Quote(changeName+"-"+shortSHA))
	return &ReviewOfferBlock{Available: true, Invocation: invocation}
}

func detectGitDirs(workspaceRoot string) (worktreeDir, commonDir string) {
	if out, err := exec.Command("git", "-C", workspaceRoot, "rev-parse", "--git-dir").Output(); err == nil {
		worktreeDir = strings.TrimSpace(string(out))
		if !filepath.IsAbs(worktreeDir) {
			worktreeDir = filepath.Join(workspaceRoot, worktreeDir)
		}
		worktreeDir = filepath.Clean(worktreeDir)
	}
	if out, err := exec.Command("git", "-C", workspaceRoot, "rev-parse", "--git-common-dir").Output(); err == nil {
		commonDir = strings.TrimSpace(string(out))
		if !filepath.IsAbs(commonDir) {
			commonDir = filepath.Join(workspaceRoot, commonDir)
		}
		commonDir = filepath.Clean(commonDir)
	}
	if commonDir == "" {
		commonDir = worktreeDir
	}
	return worktreeDir, commonDir
}

func isRDDEnabled(workspaceRoot string) bool {
	worktreeDir, commonDir := detectGitDirs(workspaceRoot)
	// When not in a git repo, fall back to global mode check via RDDStatus with empty dirs
	// RDDStatus handles empty dirs as global-only check.
	status, err := review.RDDStatus(worktreeDir, commonDir)
	if err != nil {
		// Corrupt or unreadable should be treated as managed (enabled) for offer gating
		// unless the report itself says disabled
		if status != nil {
			return status.EffectiveMode == review.RDDModeEnabled
		}
		// If we cannot determine, assume enabled (default ON)
		return true
	}
	return status.EffectiveMode == review.RDDModeEnabled
}

func shortSHAForWorkspace(workspaceRoot string) string {
	out, err := exec.Command("git", "-C", workspaceRoot, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// declaredArtifactStore resolves the declared artifact store by reading
// openspec/config.yaml key sdd.artifact_store (preferred) or artifact_store,
// normalized via NormalizeArtifactStore. Missing or unreadable file defaults
// to openspec; none disables planning I/O and returns empty string.
func declaredArtifactStore(ws string) ArtifactStore {
	configPath := filepath.Join(ws, "openspec", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ArtifactStoreOpenSpec
	}
	var cfg struct {
		SDD struct {
			ArtifactStore string `yaml:"artifact_store"`
		} `yaml:"sdd"`
		ArtifactStore string `yaml:"artifact_store"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ArtifactStoreOpenSpec
	}
	raw := strings.TrimSpace(cfg.SDD.ArtifactStore)
	if raw == "" {
		raw = strings.TrimSpace(cfg.ArtifactStore)
	}
	if raw == "" {
		return ArtifactStoreOpenSpec
	}
	store := ArtifactStore(strings.ToLower(raw))
	store = NormalizeArtifactStore(store)
	if store == ArtifactStoreNone || strings.EqualFold(raw, "none") {
		return ArtifactStore("")
	}
	if !isValidArtifactStore(store) {
		return ArtifactStoreOpenSpec
	}
	return store
}

// resolveArtifactPaths maps every SDD artifact to its location branched by store:
// openspec returns filesystem openspec/changes/{change}/… paths;
// engram/bigmem returns bigmem:sdd/{change}/… paths;
// hybrid merges (filesystem-wins, so returns filesystem paths here, merge done at Status level);
// none returns empty paths.
func resolveArtifactPaths(changeRoot string, store ArtifactStore) ArtifactPaths {
	if store == "" || store == ArtifactStoreNone {
		return ArtifactPaths{}
	}
	if IsEngramStore(store) {
		name := filepath.Base(changeRoot)
		return ArtifactPaths{
			Proposal:      []string{fmt.Sprintf("bigmem:sdd/%s/proposal", name)},
			Specs:         []string{fmt.Sprintf("bigmem:sdd/%s/spec", name)},
			Design:        []string{fmt.Sprintf("bigmem:sdd/%s/design", name)},
			Tasks:         []string{fmt.Sprintf("bigmem:sdd/%s/tasks", name)},
			ApplyProgress: []string{fmt.Sprintf("bigmem:sdd/%s/apply-progress", name)},
			VerifyReport:  []string{fmt.Sprintf("bigmem:sdd/%s/verify-report", name)},
		}
	}
	paths := ArtifactPaths{
		Proposal:      existingPath(filepath.Join(changeRoot, "proposal.md")),
		Design:        existingPath(filepath.Join(changeRoot, "design.md")),
		Tasks:         existingPath(filepath.Join(changeRoot, "tasks.md")),
		ApplyProgress: existingPath(filepath.Join(changeRoot, "apply-progress.md")),
		VerifyReport:  existingPath(filepath.Join(changeRoot, "verify-report.md")),
	}
	specFiles := findSpecFiles(filepath.Join(changeRoot, "specs"))
	paths.Specs = specFiles
	return paths
}

func existingPath(path string) []string {
	if _, err := os.Stat(path); err == nil {
		return []string{path}
	}
	return []string{}
}

func findSpecFiles(specsRoot string) []string {
	var files []string
	err := filepath.WalkDir(specsRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !entry.IsDir() && entry.Name() == "spec.md" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return []string{}
	}
	sort.Strings(files)
	return files
}

// singleArtifactState is missing when the path is absent, partial when the
// file exists but has no trimmed content, done otherwise.
func singleArtifactState(paths []string) ArtifactState {
	if len(paths) == 0 {
		return ArtifactMissing
	}
	if hasContent(paths[0]) {
		return ArtifactDone
	}
	return ArtifactPartial
}

// multiArtifactState is missing when no spec.md was found, done when every
// found spec.md has content, and partial when the specs directory is
// non-empty but some (or all) spec.md files are empty.
func multiArtifactState(paths []string, root string) ArtifactState {
	if len(paths) == 0 {
		if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
			return ArtifactPartial
		}
		return ArtifactMissing
	}
	for _, path := range paths {
		if !hasContent(path) {
			return ArtifactPartial
		}
	}
	return ArtifactDone
}

func hasContent(path string) bool {
	content, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(content)) != ""
}

func readText(path string) string {
	if path == "" {
		return ""
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

func firstPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// countTaskProgressText counts markdown task checkboxes with the unified
// taskCheckbox pattern (also used by edit-authority detection). Total>0 with
// zero pending means all complete; a missing tasks file yields the zero
// struct.
func countTaskProgressText(content string) TaskProgress {
	var progress TaskProgress
	for _, line := range strings.Split(content, "\n") {
		matches := taskCheckbox.FindStringSubmatch(line)
		if len(matches) == 0 {
			continue
		}
		progress.Total++
		if matches[1] == "x" || matches[1] == "X" {
			progress.Completed++
		} else {
			progress.Pending++
		}
	}
	progress.AllComplete = progress.Total > 0 && progress.Pending == 0
	return progress
}

// artifactBlockedReasons buckets the expected planning reasons (missing or
// partial artifacts) separately from genuine anomalies (no checkboxes, edit
// authority, remediation).
func artifactBlockedReasons(artifacts map[string]ArtifactState, taskProgress TaskProgress) blockerReasons {
	var reasons blockerReasons
	if artifacts["proposal"] != ArtifactDone {
		reasons.expectedPlanning = append(reasons.expectedPlanning, "proposal.md is missing or partial.")
	}
	if artifacts["specs"] != ArtifactDone {
		reasons.expectedPlanning = append(reasons.expectedPlanning, "specs/**/spec.md is missing or partial.")
	}
	if artifacts["design"] != ArtifactDone {
		reasons.expectedPlanning = append(reasons.expectedPlanning, "design.md is missing or partial.")
	}
	if artifacts["tasks"] != ArtifactDone {
		reasons.expectedPlanning = append(reasons.expectedPlanning, "tasks.md is missing or partial.")
	}
	if artifacts["tasks"] == ArtifactDone && taskProgress.Total == 0 {
		reasons.genuine = append(reasons.genuine, "tasks.md has no markdown task checkboxes.")
	}
	return reasons
}

// resolveApplyState: planning must be fully done with a non-empty task list
// before apply is ready; an all-complete checklist makes it all_done.
func resolveApplyState(coreReady bool, taskProgress TaskProgress) ApplyState {
	if !coreReady {
		return ApplyBlocked
	}
	if taskProgress.AllComplete {
		return ApplyAllDone
	}
	return ApplyReady
}

// resolveDependencies derives the phase dependency states from artifact
// states, apply state, and the verify evaluation.
func resolveDependencies(artifacts map[string]ArtifactState, taskProgress TaskProgress, applyState ApplyState, coreReady, verifyReportCurrent, verifyReportPassing, remediationComplete bool) Dependencies {
	dependencies := Dependencies{
		Proposal: artifactDependency(artifacts["proposal"]),
		Specs:    artifactDependency(artifacts["specs"]),
		Design:   artifactDependency(artifacts["design"]),
		Tasks:    artifactDependency(artifacts["tasks"]),
		Apply:    DependencyBlocked,
		Verify:   DependencyBlocked,
		Archive:  DependencyBlocked,
	}
	if applyState == ApplyReady {
		dependencies.Apply = DependencyReady
	} else if applyState == ApplyAllDone {
		dependencies.Apply = DependencyAllDone
	}

	if verifyReportCurrent && coreReady && taskProgress.AllComplete && verifyReportPassing {
		dependencies.Verify = DependencyAllDone
	} else if coreReady && applyState == ApplyAllDone && (!verifyReportCurrent || remediationComplete) {
		dependencies.Verify = DependencyReady
	}
	if dependencies.Verify == DependencyAllDone && taskProgress.AllComplete {
		dependencies.Archive = DependencyReady
	}
	return dependencies
}

func artifactDependency(state ArtifactState) DependencyState {
	if state == ArtifactDone {
		return DependencyAllDone
	}
	return DependencyBlocked
}

// resolveNextRecommended routes in exact priority order: apply over verify,
// verify over archive, planning artifacts in dependency order, and
// resolve-blockers only for genuine anomalies. Biggz divergence from
// gentle-ai: the apply-done-with-current-verify-but-not-all-done case has no
// review authority to resolve, so the resolve-review exit is skipped and the
// routing falls through to the remaining rules (archive / planning /
// resolve-blockers).
func resolveNextRecommended(dependencies Dependencies, applyState ApplyState, verifyReportDone bool, remediation RemediationState) string {
	if next := nextForImmediateReady(dependencies, applyState, verifyReportDone, remediation); next != "" {
		return next
	}
	if next := nextForPlanning(dependencies); next != "" {
		return next
	}
	return "resolve-blockers"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isStaleDecisionRequired checks the admission probe for a stale
// decision-required. It mirrors gentle-ai's applyNativeRuntimeRouting that
// frees verify/archive when the ledger is in decision-required but the
// probe says BlockedReason is corrupt_authority or budget_exhausted with a
// settle obligation, so verify/archive are not stranded by a stale
// decision-required.
func isStaleDecisionRequired(changeName, workspaceRoot, instance string) bool {
	if !sddattempt.LedgerExists(changeName, workspaceRoot) {
		return false
	}
	status, err := sddattempt.StatusWithInstance(changeName, workspaceRoot, instance)
	if err != nil {
		return false
	}
	if status.BlockedReason != sddattempt.BlockedReasonBudgetExhausted && status.BlockedReason != sddattempt.BlockedReasonCorruptAuthority {
		return false
	}
	return status.SettleObligation != nil && status.SettleObligation.EvidenceRevision != ""
}

// applyStaleDecisionRouting frees verify/archive from a stale block. When
// staleDecision is true, verify is forced to ready (and archive follows if
// verify is all_done), so the change can still make progress via the
// bounded correction path without manual reset.
func applyStaleDecisionRouting(deps Dependencies, staleDecision bool) Dependencies {
	if !staleDecision {
		return deps
	}
	if deps.Verify == DependencyBlocked {
		deps.Verify = DependencyReady
	}
	if deps.Verify == DependencyAllDone && deps.Archive == DependencyBlocked {
		deps.Archive = DependencyReady
	}
	return deps
}

// deriveSyncState determines sync dependency and guardrail blockedReasons.
// It mirrors the executor guardrails but operates at status derivation time
// without a prompt (so destructive/collision indicate need for approval).
func deriveSyncState(change, workspaceRoot, changeDir string, store ArtifactStore, verifyResult verifyResultEvaluation, tasksContent string, artifacts map[string]ArtifactState, applyState ApplyState) (DependencyState, []string) {
	if st, done := syncStateGate(store, artifacts, verifyResult, applyState); done {
		return st, nil
	}
	if st, done := syncStateNeedsDeltas(change, workspaceRoot, changeDir); done {
		return st, nil
	}
	if strings.Contains(strings.ToLower(tasksContent), "resolve-via-engram") {
		return DependencyReady, nil
	}
	reasons := deriveSyncGuardReasons(change, workspaceRoot, changeDir)
	if len(reasons) > 0 {
		return DependencyReady, reasons
	}
	return DependencyReady, nil
}

// FormatStatus returns a human-readable summary of SDD status.
// If opts.ReviewDisabled is true, the output includes an RDD status header.
// Scannable rendering: 4 blocks Status Overview / Artifact Progress / Next Action / Risks/Blockers
// each in Outcome + Quick path + Details shape with sanitized truncation and chunking <7.
func FormatStatus(active, archived []ChangeStatus, opts StatusOptions) string {
	var b strings.Builder

	if opts.ReviewDisabled {
		b.WriteString("RDD status: disabled (unmanaged)\n\n")
	}
	width := getTerminalWidth()

	if len(active) == 0 {
		b.WriteString("No active changes.\n")
	} else {
		b.WriteString("Active changes:\n")
		for _, cs := range active {
			b.WriteString(formatBanner(cs, width))
			b.WriteString(formatStatusBlocks(cs, width))
		}
	}

	if len(archived) > 0 {
		b.WriteString("\nRecent archived:\n")
		for _, cs := range archived {
			b.WriteString(formatOne(cs, true))
		}
	}

	return b.String()
}

func getTerminalWidth() int {
	if wStr := os.Getenv("COLUMNS"); wStr != "" {
		if w, err := strconv.Atoi(wStr); err == nil && w > 0 {
			return w
		}
	}
	if wStr := os.Getenv("TERM_WIDTH"); wStr != "" {
		if w, err := strconv.Atoi(wStr); err == nil && w > 0 {
			return w
		}
	}
	return 80
}

func formatBanner(cs ChangeStatus, width int) string {
	if width <= 0 {
		width = 80
	}
	name := sanitizePlain(cs.Name)
	// also strip ANSI from name if any
	name = ansi.Strip(name)
	label := sanitizePlain(phaseLabel(cs))
	raw := fmt.Sprintf("◆ %s · %s", name, label)
	// sanitize before width measure
	sanitized := strings.ReplaceAll(raw, "	", "    ")
	sanitized = stripOsc(sanitized)
	sanitized = ansi.Strip(sanitized)
	sanitized = stripControls(sanitized)
	sanitized = strings.Join(strings.Fields(sanitized), " ")
	if visibleWidth(sanitized) > width {
		sanitized = truncateToWidth(sanitized, width)
	}
	return sanitized + "\n"
}

func formatStatusBlocks(cs ChangeStatus, width int) string {
	var b strings.Builder
	// Status Overview
	b.WriteString("## Status Overview\n")
	b.WriteString("**Outcome:** " + outcomeForStatusOverview(cs) + "\n")
	b.WriteString("**Quick path:**\n")
	for i, step := range quickPathForStatusOverview(cs) {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, sanitizeForWidth(step, width)))
	}
	b.WriteString("**Details:**\n")
	b.WriteString(renderStatusTable(detailsForStatusOverview(cs), width))
	// Artifact Progress
	b.WriteString("## Artifact Progress\n")
	b.WriteString("**Outcome:** " + outcomeForArtifactProgress(cs) + "\n")
	b.WriteString("**Quick path:**\n")
	for i, step := range quickPathForArtifactProgress(cs) {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, sanitizeForWidth(step, width)))
	}
	b.WriteString("**Details:**\n")
	b.WriteString(renderStatusTable(detailsForArtifactProgress(cs), width))
	// Next Action
	b.WriteString("## Next Action\n")
	b.WriteString("**Outcome:** " + outcomeForNextAction(cs) + "\n")
	b.WriteString("**Quick path:**\n")
	for i, step := range quickPathForNextAction(cs) {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, sanitizeForWidth(step, width)))
	}
	b.WriteString("**Details:**\n")
	b.WriteString(renderStatusTable(detailsForNextAction(cs), width))
	// Risks/Blockers
	risksOutcome, risksRows := detailsForRisksBlock(cs)
	if len(risksRows) == 0 && risksOutcome == "" {
		b.WriteString("## Risks/Blockers\n")
		b.WriteString("**Outcome:** None\n")
		b.WriteString("**Quick path:**\n")
		b.WriteString("1. None\n")
		b.WriteString("**Details:**\n")
		b.WriteString("None\n")
	} else {
		if risksOutcome == "" {
			risksOutcome = "blocked"
		}
		b.WriteString("## Risks/Blockers\n")
		b.WriteString("**Outcome:** " + sanitizeForWidth(risksOutcome, width) + "\n")
		b.WriteString("**Quick path:**\n")
		steps := quickPathForRisks(cs)
		if len(steps) == 0 {
			steps = []string{"None"}
		}
		for i, step := range steps {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, sanitizeForWidth(step, width)))
		}
		b.WriteString("**Details:**\n")
		if len(risksRows) == 0 {
			b.WriteString("None\n")
		} else {
			b.WriteString(renderStatusTable(risksRows, width))
		}
	}
	return b.String()
}

func outcomeForStatusOverview(cs ChangeStatus) string {
	label := phaseLabel(cs)
	if cs.IsArchived {
		label = "archived"
	}
	return sanitizeForWidth(label, 80)
}

func quickPathForStatusOverview(cs ChangeStatus) []string {
	if cs.NextRecommended != "" && cs.NextRecommended != "done" && cs.NextRecommended != "resolve-blockers" {
		return []string{fmt.Sprintf("biggz sdd-%s %s", cs.NextRecommended, cs.Name)}
	}
	return []string{"biggz sdd-status"}
}

func detailsForStatusOverview(cs ChangeStatus) [][]string {
	rows := [][]string{
		{"Change", sanitizePlain(cs.Name)},
		{"State", sanitizePlain(phaseLabel(cs))},
		{"Tasks", fmt.Sprintf("%d/%d", cs.TasksDone, cs.TasksTotal)},
		{"Artifacts", fmt.Sprintf("proposal:%s specs:%s design:%s tasks:%s", cs.Artifacts["proposal"], cs.Artifacts["specs"], cs.Artifacts["design"], cs.Artifacts["tasks"])},
	}
	// sanitize and truncate per cell will be done in renderStatusTable
	return rows
}

func outcomeForArtifactProgress(cs ChangeStatus) string {
	if cs.Artifacts == nil {
		return "unknown"
	}
	done := 0
	total := 0
	for _, k := range []string{"proposal", "specs", "design", "tasks", "applyProgress", "verifyReport"} {
		total++
		if cs.Artifacts[k] == ArtifactDone {
			done++
		}
	}
	if done == total {
		return "all artifacts done"
	}
	return fmt.Sprintf("%d/%d artifacts done", done, total)
}

func quickPathForArtifactProgress(cs ChangeStatus) []string {
	var steps []string
	if cs.Artifacts["proposal"] != ArtifactDone {
		steps = append(steps, "create proposal.md")
	}
	if cs.Artifacts["specs"] != ArtifactDone {
		steps = append(steps, "add specs/**/spec.md")
	}
	if cs.Artifacts["design"] != ArtifactDone {
		steps = append(steps, "write design.md")
	}
	if cs.Artifacts["tasks"] != ArtifactDone {
		steps = append(steps, "define tasks.md")
	}
	if len(steps) == 0 {
		steps = []string{"review artifacts"}
	}
	return steps
}

func detailsForArtifactProgress(cs ChangeStatus) [][]string {
	keys := []string{"proposal", "specs", "design", "tasks", "applyProgress", "verifyReport"}
	var rows [][]string
	for _, k := range keys {
		state := "missing"
		if cs.Artifacts != nil {
			if v, ok := cs.Artifacts[k]; ok {
				state = string(v)
			}
		}
		rows = append(rows, []string{k, state})
	}
	return rows
}

func outcomeForNextAction(cs ChangeStatus) string {
	if cs.NextRecommended == "" {
		return "none"
	}
	return sanitizePlain(cs.NextRecommended)
}

func quickPathForNextAction(cs ChangeStatus) []string {
	next := cs.NextRecommended
	if next == "" {
		return []string{"biggz sdd-status"}
	}
	if next == "sync" {
		return []string{fmt.Sprintf("biggz sdd-sync %s", cs.Name), "verify"}
	}
	if next == "resolve-blockers" {
		return []string{"resolve blocked reasons"}
	}
	return []string{fmt.Sprintf("biggz sdd-%s %s", next, cs.Name)}
}

func detailsForNextAction(cs ChangeStatus) [][]string {
	var rows [][]string
	rows = append(rows, []string{"next", sanitizePlain(cs.NextRecommended)})
	// explicit dependencies order
	deps := []struct {
		name  string
		state DependencyState
	}{
		{"proposal", cs.Dependencies.Proposal},
		{"specs", cs.Dependencies.Specs},
		{"design", cs.Dependencies.Design},
		{"tasks", cs.Dependencies.Tasks},
		{"apply", cs.Dependencies.Apply},
		{"verify", cs.Dependencies.Verify},
		{"archive", cs.Dependencies.Archive},
	}
	for _, d := range deps {
		if d.state != "" {
			rows = append(rows, []string{d.name, string(d.state)})
		}
	}
	return rows
}

func detailsForRisksBlock(cs ChangeStatus) (string, [][]string) {
	if len(cs.BlockedReasons) == 0 {
		return "", nil
	}
	outcome := cs.BlockedReasons[0]
	if len(cs.BlockedReasons) > 1 {
		outcome = fmt.Sprintf("%d blockers", len(cs.BlockedReasons))
	}
	var rows [][]string
	for _, r := range cs.BlockedReasons {
		s := sanitizePlain(r)
		// split into topic/decision on colon if possible for table
		topic, decision := splitTopicDecision(s)
		if decision == "" {
			topic = s
			decision = "blocked"
		}
		rows = append(rows, []string{topic, decision})
	}
	return outcome, rows
}

func quickPathForRisks(cs ChangeStatus) []string {
	if len(cs.BlockedReasons) == 0 {
		return nil
	}
	return []string{"resolve blockers", fmt.Sprintf("biggz sdd-status %s --json", cs.Name)}
}

func renderStatusTable(rows [][]string, width int) string {
	if len(rows) == 0 {
		return "None\n"
	}
	if width <= 0 {
		width = 80
	}
	// sanitize and truncate per cell to budget
	budget := (width - 6) / 2
	if budget < 5 {
		budget = 5
	}
	// conservative: ensure narrow 40 still passes, cap budget at 17 if width>40 but we already compute; for width 80 budget 37, but narrow test will use width 40 => 17, we handle via caller width.
	// For ensure per-cell VisibleWidth <= budget on narrow, we truncate to budget.
	var sanitized [][]string
	for _, r := range rows {
		topic := sanitizePlain(r[0])
		decision := ""
		if len(r) > 1 {
			decision = sanitizePlain(r[1])
		}
		topic = truncateToWidth(topic, budget)
		decision = truncateToWidth(decision, budget)
		sanitized = append(sanitized, []string{topic, decision})
	}
	chunks := chunkTable(sanitized, 7)
	var b strings.Builder
	for idx, chunk := range chunks {
		b.WriteString("| Topic | Decision |\n")
		b.WriteString("|-------|----------|\n")
		for _, r := range chunk {
			b.WriteString("| " + r[0] + " | " + r[1] + " |\n")
		}
		if idx < len(chunks)-1 {
			remaining := len(sanitized) - (idx+1)*7
			if remaining > 0 {
				b.WriteString(fmt.Sprintf("… +%d more\n", remaining))
			}
		}
	}
	return b.String()
}

func formatOne(cs ChangeStatus, archived bool) string {
	icon := map[bool]string{true: "✅", false: "⬜"}

	status := fmt.Sprintf("  %s %s", icon[cs.HasProposal && cs.TasksDone == cs.TasksTotal], cs.Name)
	if archived {
		return fmt.Sprintf("  • %s (%s)\n", cs.Name, phaseLabel(cs))
	}
	if cs.EditAuthorityBlocked {
		status += fmt.Sprintf(" — [%s]\n    %s\n", phaseLabel(cs), editAuthorityBlockedReason(cs.MissingRoots))
		if cs.Consent != nil && len(cs.Consent.Choices) > 0 {
			status += fmt.Sprintf("    consent grant: %s\n", cs.Consent.Choices[0].Invocation)
		}
		return status
	}
	return fmt.Sprintf("%s — [%s]\n", status, phaseLabel(cs))
}

// phaseLabel renders the human phase bracket. When the derivation has run,
// it is nextRecommended-aware (e.g. "[next: spec]"); resolve-blockers lists
// the blocked reasons themselves. The legacy file-probe chain remains the
// fallback for ChangeStatus values built without derivation.
func phaseLabel(cs ChangeStatus) string {
	if cs.NextRecommended != "" {
		switch cs.NextRecommended {
		case "done":
			return "done"
		case "resolve-blockers":
			if len(cs.BlockedReasons) > 0 {
				return "resolve-blockers: " + strings.Join(cs.BlockedReasons, " ")
			}
			return "resolve-blockers"
		default:
			return "next: " + cs.NextRecommended
		}
	}
	switch {
	case !cs.HasProposal:
		return "explore/proposal"
	case !cs.HasSpecs:
		return "spec"
	case !cs.HasDesign:
		return "design"
	case !cs.HasTasks:
		return "tasks"
	case cs.TasksDone < cs.TasksTotal:
		return fmt.Sprintf("apply (%d/%d tasks)", cs.TasksDone, cs.TasksTotal)
	case !cs.HasVerify:
		return "verify"
	default:
		return "archive-ready"
	}
}

// ShouldEnforceScopedSurfaces reports whether the 4-file heuristic triggers.
// Strict at >=4: 3 files allow single writer without per-path guard, 4 enforces per-path.
func ShouldEnforceScopedSurfaces(fileCount int) bool { return fileCount >= 4 }

// ScopedSurfaceRejection is the local rejection type for sdd guard without
// importing orchestrator (avoids import cycle between sdd and orchestrator).
type ScopedSurfaceRejection struct {
	Block  bool
	Reason string
}

// ValidateBoundedWriterSurfaces checks writer dispatch surfaces when fileCount >=4.
// Returns nil when no enforcement needed; otherwise returns a rejection when
// task/context lack scoped surfaces. Keeps the 4-file strict boundary.
// Full validation: heading case-insensitive, any-level terminator, bullet/` strip,
// ≥1 entry per heading, each via IsTaskScopedRepositoryRelativePath, dedup/sort, all headings agree.
func ValidateBoundedWriterSurfaces(input map[string]any, fileCount int) *ScopedSurfaceRejection {
	if !ShouldEnforceScopedSurfaces(fileCount) {
		return nil
	}
	agent, _ := input["agent"].(string)
	if agent != "worker" && agent != "gentle-ai-worker" {
		return nil
	}
	task, _ := input["task"].(string)
	ctx, _ := input["context"].(string)
	if !sddHasTaskScopedAllowedEditSurfaces(task, ctx) {
		// log offending surface if any
		offending := sddFindOffendingSurface(task, ctx)
		if offending != "" {
			log.Printf("[sdd] ValidateBoundedWriterSurfaces Block=true agent=%s fileCount=%d offending=%s", agent, fileCount, offending)
		} else {
			log.Printf("[sdd] ValidateBoundedWriterSurfaces Block=true agent=%s fileCount=%d reason=missing_surfaces", agent, fileCount)
		}
		return &ScopedSurfaceRejection{Block: true, Reason: "Parent must derive or map narrow repository-relative allowed edit surfaces from the delegated task and relaunch the writer. Do not ask the human to author paths or globs."}
	}
	return nil
}

var sddAllowedHeadingRe = regexp.MustCompile(`(?mi)^## Allowed edit surfaces[ \t]*$`)
var sddHeadingRe = regexp.MustCompile(`(?m)^#{1,6}\s+`)
var sddListMarkerRe = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+`)
var sddWhitespaceRe = regexp.MustCompile(`\s`)

func sddIsTaskScopedRepositoryRelativePath(v string) bool {
	n := strings.ReplaceAll(v, "\\", "/")
	if len(n) == 0 || filepath.IsAbs(v) || filepath.IsAbs(n) {
		return false
	}
	if ok, _ := regexp.MatchString(`^(?:[A-Za-z]:|/|~)`, n); ok {
		return false
	}
	w := regexp.MustCompile(`^(?:\./)+`).ReplaceAllString(n, "")
	if len(w) == 0 || w == "." || strings.HasPrefix(w, "/") {
		return false
	}
	if sddWhitespaceRe.MatchString(w) {
		return false
	}
	for _, s := range strings.Split(w, "/") {
		if s == ".." {
			return false
		}
	}
	first := strings.Split(w, "/")[0]
	return !strings.ContainsAny(first, "?*[]{}")
}
func sddReadSurfaceEntry(line string) string {
	e := sddListMarkerRe.ReplaceAllString(line, "")
	if len(e) >= 2 && strings.HasPrefix(e, "`") && strings.HasSuffix(e, "`") {
		return e[1 : len(e)-1]
	}
	return e
}
func sddLooksLikeSurfaceEntry(line string) bool {
	if len(line) == 0 {
		return false
	}
	if sddHeadingRe.MatchString(line) {
		return false
	}
	return !sddWhitespaceRe.MatchString(sddReadSurfaceEntry(line))
}
func sddReadAllowedEntries(following string) []string {
	lines := normalizeSurfaceLines(following)
	section := sectionUntilNextHeading(lines)
	return extractAllowedEntries(section)
}
func sddHasTaskScopedAllowedEditSurfaces(values ...string) bool {
	var exp []string
	has := false
	for _, v := range values {
		matches := sddAllowedHeadingRe.FindAllStringIndex(v, -1)
		for _, loc := range matches {
			following := v[loc[1]:]
			entries := sddReadAllowedEntries(following)
			if len(entries) == 0 {
				return false
			}
			for _, e := range entries {
				if !sddIsTaskScopedRepositoryRelativePath(e) {
					return false
				}
			}
			uniq := sddDedupSort(entries)
			if exp != nil && (len(exp) != len(uniq) || !sddEqualSorted(exp, uniq)) {
				return false
			}
			exp = uniq
			has = true
		}
	}
	return has
}
func sddDedupSort(in []string) []string {
	m := map[string]struct{}{}
	for _, v := range in {
		m[v] = struct{}{}
	}
	o := make([]string, 0, len(m))
	for k := range m {
		o = append(o, k)
	}
	sort.Strings(o)
	return o
}
func sddEqualSorted(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func sddFindOffendingSurface(values ...string) string {
	for _, v := range values {
		matches := sddAllowedHeadingRe.FindAllStringIndex(v, -1)
		for _, loc := range matches {
			following := v[loc[1]:]
			entries := sddReadAllowedEntries(following)
			for _, e := range entries {
				if !sddIsTaskScopedRepositoryRelativePath(e) {
					return e
				}
			}
		}
		if strings.Contains(v, "..") || strings.Contains(v, "~") {
			return v
		}
	}
	return ""
}
