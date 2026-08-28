// Package sdd implements native SDD status derivation for the hybrid
// filesystem + BigMem (Engram) store.
// Alias invariant: engram == bigmem — both topic prefixes refer to the same
// BigMem store; drift that renames one without the other must be detected.

//
// This file ports gentle-ai's resolveEngramStatus / collectEngramChanges /
// engramObservation machinery as a minimal native derivation that makes
// StatusWithOptions authoritative for both stores without breaking
// filesystem-only users.
//
// Design notes (intentional minimal wire):
//   - BigMem observations are read via direct SQLite on the BigMem store
//     file (modernc.org/sqlite), topic_key LIKE 'sdd/%', mirroring
//     gentle's sdd/{change}/tasks etc convention. Using SQLite avoids an
//     import cycle if internal/bigmem ever imports internal/sdd and keeps
//     the derivation testable with t.TempDir() stores.
//   - When the store file is absent or has no sdd/ observations, the
//     collector returns (nil,nil,nil) and StatusWithOptions falls back to
//     filesystem only — no breakage for filesystem-only users.
//   - Tests inject a temporary store via SetBigMemStoreRootForTest /
//     bigmemStoreRootOverride. When the override is set, project filtering
//     is disabled so mocked observations are always visible regardless of
//     inferred project. In production (override == ""), observations are
//     filtered by inferred project (ENGRAM_PROJECT / BIGMEM_PROJECT /
//     git remote / base dir) and scope != personal, matching gentle's
//     engramObservationMatchesProject.
//   - Filesystem wins on conflict (documented in mergeFilesystemAndBigMem):
//     a change name present in both stores keeps the filesystem
//     ChangeStatus and the BigMem duplicate is discarded. This keeps the
//     local file trail the audit authority for hybrid mode while still
//     surfacing BigMem-only changes.
package sdd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
	"github.com/biggs-100/biggz-ai/internal/sddattempt"
	_ "modernc.org/sqlite"
)

// bigmemStoreRootOverride allows tests to inject a temporary BigMem store
// directory (the directory that contains bigmem.db). Empty means the default
// ~/.biggz/bigmem location. When the DB file does not exist, collection is
// a no-op.
var bigmemStoreRootOverride string

// SetBigMemStoreRootForTest sets the BigMem store root used by
// collectBigMemChanges. Pass "" to restore the default. Exported for tests.
func SetBigMemStoreRootForTest(root string) { bigmemStoreRootOverride = root }

// bigmemTitlePattern matches the BigMem topic_key convention
// sdd/{change}/{artifact}. The artifact set is the minimal slice needed
// for status derivation: proposal/spec/design/tasks/apply-progress/
// verify-report plus archive-report (closure signal) and state/explore
// (ignored for existence). It mirrors gentle-ai's engramTitlePattern.
var bigmemTitlePattern = regexp.MustCompile(`^sdd/([^/]+)/(proposal|spec|design|tasks|apply-progress|verify-report|archive-report|state|explore)$`)

// bigmemDBPath returns the bigmem.db file path for a store root using
// the unified bigmem.ResolveDBPath helper. Empty root means the default
// ~/.biggz/bigmem. This ensures Store.Open and engram_status share ghost
// WAL/SHM handling, checkpoint-before-copy, and max(updated_at) merge.
func bigmemDBPath(root string) (string, error) {
	return bigmem.ResolveDBPath(root)
}

// inferBigMemProject mirrors gentle-ai's inferEngramProject for BigMem:
// ENGRAM_PROJECT / BIGMEM_PROJECT / BIGGZ_PROJECT env, then git common-dir
// config remote URL, then base dir. Lower-cased.
func inferBigMemProject(workspaceRoot string) string {
	for _, env := range []string{"BIGMEM_PROJECT", "BIGGZ_PROJECT", "ENGRAM_PROJECT"} {
		if p := strings.TrimSpace(os.Getenv(env)); p != "" {
			return strings.ToLower(p)
		}
	}
	if path := gitConfigPathFor(workspaceRoot); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			if p := projectFromGitConfig(string(data)); p != "" {
				return p
			}
		}
	}
	return strings.ToLower(filepath.Base(workspaceRoot))
}

// gitConfigPathFor resolves the config file for workspaceRoot, handling
// worktrees (where .git is a file with gitdir: pointer). Copied from
// gentle-ai's gitConfigPathFor.
func gitConfigPathFor(workspaceRoot string) string {
	gitEntry := filepath.Join(workspaceRoot, ".git")
	info, err := os.Stat(gitEntry)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return filepath.Join(gitEntry, "config")
	}
	pointer, err := os.ReadFile(gitEntry)
	if err != nil {
		return ""
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(pointer)), "gitdir:"))
	if gitDir == "" {
		return ""
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(workspaceRoot, gitDir)
	}
	commonDir := gitDir
	if content, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		if trimmed := strings.TrimSpace(string(content)); trimmed != "" {
			commonDir = trimmed
			if !filepath.IsAbs(commonDir) {
				commonDir = filepath.Join(gitDir, commonDir)
			}
		}
	}
	return filepath.Join(commonDir, "config")
}

func projectFromGitConfig(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "url =") {
			continue
		}
		url := strings.TrimSpace(strings.TrimPrefix(line, "url ="))
		url = strings.TrimSuffix(url, ".git")
		if idx := strings.LastIndexAny(url, "/:"); idx >= 0 && idx+1 < len(url) {
			return strings.ToLower(url[idx+1:])
		}
	}
	return ""
}

// bigmemArtifactState maps presence + content to ArtifactState.
func bigmemArtifactState(m map[string]string, suffix string) ArtifactState {
	content, ok := m[suffix]
	if !ok {
		// alias: spec <-> specs
		if suffix == "spec" {
			if c, ok2 := m["specs"]; ok2 {
				content = c
				ok = true
			}
		} else if suffix == "specs" {
			if c, ok2 := m["spec"]; ok2 {
				content = c
				ok = true
			}
		}
		if !ok {
			return ArtifactMissing
		}
	}
	if strings.TrimSpace(content) == "" {
		return ArtifactPartial
	}
	return ArtifactDone
}

func bigmemArtifactPaths(changeName string, m map[string]string) ArtifactPaths {
	paths := ArtifactPaths{}
	if _, ok := m["proposal"]; ok {
		paths.Proposal = []string{fmt.Sprintf("bigmem:sdd/%s/proposal", changeName)}
	}
	// spec topic is singular in BigMem
	if _, ok := m["spec"]; ok {
		paths.Specs = []string{fmt.Sprintf("bigmem:sdd/%s/spec", changeName)}
	} else if _, ok := m["specs"]; ok {
		paths.Specs = []string{fmt.Sprintf("bigmem:sdd/%s/spec", changeName)}
	}
	if _, ok := m["design"]; ok {
		paths.Design = []string{fmt.Sprintf("bigmem:sdd/%s/design", changeName)}
	}
	if _, ok := m["tasks"]; ok {
		paths.Tasks = []string{fmt.Sprintf("bigmem:sdd/%s/tasks", changeName)}
	}
	if _, ok := m["apply-progress"]; ok {
		paths.ApplyProgress = []string{fmt.Sprintf("bigmem:sdd/%s/apply-progress", changeName)}
	}
	if _, ok := m["verify-report"]; ok {
		paths.VerifyReport = []string{fmt.Sprintf("bigmem:sdd/%s/verify-report", changeName)}
	}
	return paths
}

// deriveBigMemChangeStatus reconstructs a ChangeStatus from BigMem artifact
// content (not filesystem). It reuses the same derivation authority as
// deriveChangeStatus: taskProgress, spec counts, verify evaluation,
// applyState, edit-authority, remediation, dependencies, nextRecommended
// and blockedReasons. BigMem changes have no change-instance marker and no
// granted roots, so the runtime read is instance-less and the apply authority
// check uses only workspaceRoot.
func deriveBigMemChangeStatus(name string, bySuffix map[string]string, workspaceRoot string, includeInstructions bool, isArchived bool) ChangeStatus {
	cs := ChangeStatus{Name: name, IsArchived: isArchived}
	// Legacy Has* based on presence (even partial counts as Has)
	_, hasProposal := bySuffix["proposal"]
	_, hasSpec := bySuffix["spec"]
	if !hasSpec {
		_, hasSpec = bySuffix["specs"]
	}
	_, hasDesign := bySuffix["design"]
	_, hasTasks := bySuffix["tasks"]
	_, hasApply := bySuffix["apply-progress"]
	_, hasVerify := bySuffix["verify-report"]
	cs.HasProposal = hasProposal
	cs.HasSpecs = hasSpec
	cs.HasDesign = hasDesign
	cs.HasTasks = hasTasks
	cs.HasApply = hasApply
	cs.HasVerify = hasVerify

	if isArchived {
		// Archived changes are done; still compute minimal derived fields for
		// audit display but keep routing terminal. We derive artifact states
		// for display, but NextRecommended is done regardless.
		artifacts := map[string]ArtifactState{
			"proposal":      bigmemArtifactState(bySuffix, "proposal"),
			"specs":         bigmemArtifactState(bySuffix, "spec"),
			"design":        bigmemArtifactState(bySuffix, "design"),
			"tasks":         bigmemArtifactState(bySuffix, "tasks"),
			"applyProgress": bigmemArtifactState(bySuffix, "apply-progress"),
			"verifyReport":  bigmemArtifactState(bySuffix, "verify-report"),
		}
		taskProgress := countTaskProgressText(bySuffix["tasks"])
		cs.TasksTotal = taskProgress.Total
		cs.TasksDone = taskProgress.Completed
		cs.SchemaName = StatusSchemaName
		cs.SchemaVersion = StatusSchemaVersion
		cs.ChangeRoot = fmt.Sprintf("bigmem:sdd/%s", name)
		cs.PlanningHome = PlanningHome{Mode: "repo-local", Path: "bigmem:sdd"}
		cs.ArtifactStore = ArtifactStoreEngram
		cs.ArtifactPaths = bigmemArtifactPaths(name, bySuffix)
		cs.ContextFiles = cs.ArtifactPaths
		cs.Artifacts = artifacts
		cs.TaskProgress = taskProgress
		cs.Dependencies = Dependencies{
			Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone,
			Tasks: DependencyAllDone, Apply: DependencyAllDone, Verify: DependencyAllDone, Archive: DependencyAllDone,
		}
		cs.ApplyState = ApplyAllDone
		cs.ActionContext = ActionContext{Mode: "repo-local", WorkspaceRoot: workspaceRoot, AllowedEditRoots: []string{workspaceRoot}}
		cs.Relationships = Relationships{}
		cs.RemediationState = RemediationState{}
		cs.ReviewOffer = nil
		cs.NextRecommended = "done"
		cs.BlockedReasons = []string{}
		if includeInstructions {
			instructions := renderPhaseInstructions(cs)
			cs.PhaseInstructions = &instructions
		}
		return cs
	}

	artifacts := map[string]ArtifactState{
		"proposal":      bigmemArtifactState(bySuffix, "proposal"),
		"specs":         bigmemArtifactState(bySuffix, "spec"),
		"design":        bigmemArtifactState(bySuffix, "design"),
		"tasks":         bigmemArtifactState(bySuffix, "tasks"),
		"applyProgress": bigmemArtifactState(bySuffix, "apply-progress"),
		"verifyReport":  bigmemArtifactState(bySuffix, "verify-report"),
	}
	taskProgress := countTaskProgressText(bySuffix["tasks"])
	cs.TasksTotal = taskProgress.Total
	cs.TasksDone = taskProgress.Completed

	// Spec counts from in-memory spec content
	var specContents []string
	if c, ok := bySuffix["spec"]; ok {
		specContents = append(specContents, c)
	} else if c, ok := bySuffix["specs"]; ok {
		specContents = append(specContents, c)
	}
	specCounts := countSpecRequirementsAndScenarios(specContents)
	verifyResult := parseVerifyResult(bySuffix["verify-report"], specCounts)

	coreReady := artifacts["proposal"] == ArtifactDone && artifacts["specs"] == ArtifactDone &&
		artifacts["design"] == ArtifactDone && artifacts["tasks"] == ArtifactDone && taskProgress.Total > 0
	applyState := resolveApplyState(coreReady, taskProgress)
	blockedReasons := artifactBlockedReasons(artifacts, taskProgress)

	allowedEditRoots := []string{workspaceRoot}
	applyState = applyEditAuthorityBlock(applyState, &blockedReasons, bySuffix["tasks"], workspaceRoot, allowedEditRoots)

	verifyReportCurrent := artifacts["verifyReport"] == ArtifactDone
	// BigMem changes have no change-instance marker file; use empty instance
	// like gentle-ai's Engram path.
	instance := ""
	remediationComplete := sddattempt.RemediationComplete(name, workspaceRoot, instance, verifyResult.EvidenceRevision)
	staleDecision := isStaleDecisionRequired(name, workspaceRoot, instance)
	remediationState := RemediationState{}
	if verifyReportCurrent && !verifyResult.Passing && applyState == ApplyAllDone && !remediationComplete && !staleDecision {
		reason := fmt.Sprintf(
			"verify evidence requires unmanaged remediation for %s: %s; receipt-driven review is disabled, so this correction is bounded by the native runtime attempt budget alone",
			verifyResult.EvidenceRevision, verifyResult.Reason,
		)
		if store, err := sddattempt.LoadStore(name, workspaceRoot); err == nil && len(store.Attempts) > 0 {
			last := store.Attempts[len(store.Attempts)-1]
			if last.RemediatesEvidenceRevision == verifyResult.EvidenceRevision {
				switch last.Outcome {
				case "interrupted":
					reason += " (last correction interrupted — original failure still bindable)"
				case "failed":
					if last.EvidenceRevision != "" && last.EvidenceRevision != verifyResult.EvidenceRevision {
						reason += fmt.Sprintf(" (last correction failed — new failure %s now bindable)", last.EvidenceRevision)
					} else {
						reason += " (last correction failed — new failure now bindable)"
					}
				}
			}
		}
		remediationState = RemediationState{
			Required:               true,
			FailedEvidenceRevision: verifyResult.EvidenceRevision,
			Reason:                 reason,
		}
	}
	if remediationState.Reason != "" {
		blockedReasons.genuine = append(blockedReasons.genuine, remediationState.Reason)
	}
	effectiveRemediationComplete := remediationComplete || staleDecision
	dependencies := resolveDependencies(artifacts, taskProgress, applyState, coreReady, verifyReportCurrent, verifyResult.Passing, effectiveRemediationComplete)
	dependencies = applyStaleDecisionRouting(dependencies, staleDecision)
	nextRecommended := resolveNextRecommended(dependencies, applyState, verifyReportCurrent, remediationState)

	cs.SchemaName = StatusSchemaName
	cs.SchemaVersion = StatusSchemaVersion
	cs.ChangeRoot = fmt.Sprintf("bigmem:sdd/%s", name)
	cs.PlanningHome = PlanningHome{Mode: "repo-local", Path: "bigmem:sdd"}
	cs.ArtifactStore = ArtifactStoreEngram
	cs.ArtifactPaths = bigmemArtifactPaths(name, bySuffix)
	cs.ContextFiles = cs.ArtifactPaths
	cs.Artifacts = artifacts
	cs.TaskProgress = taskProgress
	cs.Dependencies = dependencies
	cs.ApplyState = applyState
	cs.ActionContext = ActionContext{Mode: "repo-local", WorkspaceRoot: workspaceRoot, AllowedEditRoots: allowedEditRoots}
	cs.Relationships = Relationships{}
	cs.RemediationState = remediationState
	cs.ReviewOffer = nil
	cs.NextRecommended = nextRecommended
	cs.BlockedReasons = blockedReasons.finalize(nextRecommended)
	if includeInstructions {
		instructions := renderPhaseInstructions(cs)
		cs.PhaseInstructions = &instructions
	}
	return cs
}

// collectBigMemChanges is the spec-compliant entry point: it returns only the
// active BigMem changes (archive-report changes are filtered out, matching
// gentle's collectEngramChanges). Archived BigMem changes are still readable
// via collectBigMemChangesWithArchive.
func collectBigMemChanges(workspaceRoot string) ([]ChangeStatus, error) {
	active, _, err := collectBigMemChangesWithArchive(workspaceRoot, false)
	return active, err
}

// collectBigMemChangesWithArchive is the hybrid authority used by
// StatusWithOptions. It returns both active and archived BigMem changes,
// reconstructing ChangeStatus via deriveBigMemChangeStatus. When the BigMem
// DB is absent or has no sdd/ observations, it returns (nil,nil,nil).
func collectBigMemChangesWithArchive(workspaceRoot string, includeInstructions bool) (active []ChangeStatus, archived []ChangeStatus, err error) {
	dbPath, err := bigmemDBPath(bigmemStoreRootOverride)
	if err != nil {
		return nil, nil, nil
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, nil, nil
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, nil, nil
	}
	defer db.Close()

	rows, err := db.Query(`SELECT topic_key, content, project, scope FROM observations WHERE topic_key LIKE 'sdd/%' AND deleted_at IS NULL`)
	if err != nil {
		return nil, nil, nil
	}
	defer rows.Close()

	inferredProject := ""
	// Only apply project filtering in production (override == ""). Tests use
	// t.TempDir() stores with arbitrary project values; filtering would hide
	// mocked observations.
	if bigmemStoreRootOverride == "" {
		inferredProject = inferBigMemProject(workspaceRoot)
	}

	byChange := map[string]map[string]string{}
	archivedSet := map[string]bool{}
	seenSet := map[string]bool{}

	for rows.Next() {
		var topicKey, content, project, scope sql.NullString
		if err := rows.Scan(&topicKey, &content, &project, &scope); err != nil {
			continue
		}
		scopeStr := ""
		if scope.Valid {
			scopeStr = scope.String
		}
		if strings.EqualFold(strings.TrimSpace(scopeStr), "personal") {
			continue
		}
		if inferredProject != "" {
			obsProject := ""
			if project.Valid {
				obsProject = strings.TrimSpace(project.String)
			}
			if !strings.EqualFold(obsProject, inferredProject) {
				continue
			}
		}
		topic := ""
		if topicKey.Valid {
			topic = strings.TrimSpace(topicKey.String)
		}
		matches := bigmemTitlePattern.FindStringSubmatch(topic)
		if len(matches) != 3 {
			continue
		}
		changeName := matches[1]
		suffix := matches[2]
		if _, ok := byChange[changeName]; !ok {
			byChange[changeName] = map[string]string{}
		}
		c := ""
		if content.Valid {
			c = content.String
		}
		byChange[changeName][suffix] = c
		if suffix == "archive-report" {
			archivedSet[changeName] = true
		} else if suffix != "state" {
			seenSet[changeName] = true
		}
	}
	if err := rows.Err(); err != nil {
		// treat query error as no BigMem changes (fallback)
		return nil, nil, nil
	}

	// Build sorted change list for determinism
	var allNames []string
	seenNames := map[string]bool{}
	for n := range seenSet {
		if !seenNames[n] {
			seenNames[n] = true
			allNames = append(allNames, n)
		}
	}
	for n := range archivedSet {
		if !seenNames[n] {
			seenNames[n] = true
			allNames = append(allNames, n)
		}
		// also include archived names that have seenSet (already added)
	}
	// Also include any change that only has archive-report but not seen (edge)
	for n := range byChange {
		if !seenNames[n] {
			if archivedSet[n] {
				seenNames[n] = true
				allNames = append(allNames, n)
			}
		}
	}
	sort.Strings(allNames)

	for _, name := range allNames {
		m := byChange[name]
		if m == nil {
			continue
		}
		isArchived := archivedSet[name]
		hasSeen := seenSet[name]
		// A change with only state is not considered a valid change
		if !isArchived && !hasSeen {
			continue
		}
		cs := deriveBigMemChangeStatus(name, m, workspaceRoot, includeInstructions, isArchived)
		if isArchived {
			archived = append(archived, cs)
		} else {
			active = append(active, cs)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Name < active[j].Name })
	sort.Slice(archived, func(i, j int) bool { return archived[i].Name < archived[j].Name })
	return active, archived, nil
}

// mergeFilesystemAndBigMem merges filesystem and BigMem changes by change name.
// Filesystem wins on conflict: a name present in the filesystem keeps the
// filesystem ChangeStatus and the BigMem duplicate is discarded. This keeps
// the local file trail the audit authority for hybrid mode while still
// surfacing BigMem-only changes. Archived and active are merged independently,
// and a name that is archived on filesystem is not re-added as active from
// BigMem (and vice versa).
func mergeFilesystemAndBigMem(fsActive, fsArchived, memActive, memArchived []ChangeStatus) (active, archived []ChangeStatus) {
	fsActiveByName := map[string]bool{}
	for _, cs := range fsActive {
		fsActiveByName[cs.Name] = true
	}
	fsArchivedByName := map[string]bool{}
	for _, cs := range fsArchived {
		fsArchivedByName[cs.Name] = true
	}

	active = append([]ChangeStatus{}, fsActive...)
	for _, cs := range memActive {
		if fsActiveByName[cs.Name] {
			continue
		}
		if fsArchivedByName[cs.Name] {
			continue
		}
		active = append(active, cs)
	}

	archived = append([]ChangeStatus{}, fsArchived...)
	memArchivedByName := map[string]bool{}
	for _, cs := range memArchived {
		_ = memArchivedByName // not needed; dedup via fsArchivedByName and active
		if fsArchivedByName[cs.Name] {
			continue
		}
		if fsActiveByName[cs.Name] {
			continue
		}
		// also avoid duplicates within memArchived vs already-merged active
		alreadyActive := false
		for _, a := range active {
			if a.Name == cs.Name {
				alreadyActive = true
				break
			}
		}
		if alreadyActive {
			continue
		}
		archived = append(archived, cs)
	}

	sort.Slice(active, func(i, j int) bool { return active[i].Name < active[j].Name })
	sort.Slice(archived, func(i, j int) bool { return archived[i].Name < archived[j].Name })
	return active, archived
}
