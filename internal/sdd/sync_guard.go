package sdd

import (
	"fmt"
	"os"
	"path/filepath"
)

// syncStateGate implements the store/verify/apply gate for deriveSyncState.
// Returns (state, true) when gate triggers, otherwise ("", false).
func syncStateGate(store ArtifactStore, artifacts map[string]ArtifactState, vr verifyResultEvaluation, applyState ApplyState) (DependencyState, bool) {
	if IsEngramStore(store) || store == "" || store == ArtifactStoreNone {
		return DependencyAllDone, true
	}
	if applyState != ApplyAllDone {
		return DependencyBlocked, true
	}
	if artifacts["verifyReport"] != ArtifactDone || !vr.Passing {
		return DependencyBlocked, true
	}
	return "", false
}

// syncStateNeedsDeltas checks hasSyncDeltas and isSyncNeeded gates.
func syncStateNeedsDeltas(change, workspaceRoot, changeDir string) (DependencyState, bool) {
	if !hasSyncDeltas(changeDir) {
		return DependencyAllDone, true
	}
	if !isSyncNeeded(change, workspaceRoot) {
		return DependencyAllDone, true
	}
	return "", false
}

// deriveSyncGuardReasons collects per-domain guardrail reasons for status sync.
func deriveSyncGuardReasons(change, workspaceRoot, changeDir string) []string {
	var reasons []string
	files := findSpecFiles(filepath.Join(changeDir, "specs"))
	for _, f := range files {
		contentBytes, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		pr, err := ParseDeltaSpec(string(contentBytes))
		if err != nil {
			continue
		}
		domain := domainFromSpecPath(f)
		if r := syncStateRenamedReason(pr.HasRenamed, domain); r != "" {
			reasons = append(reasons, r)
		}
		if r := syncStateLegacyReason(workspaceRoot, domain); r != "" {
			reasons = append(reasons, r)
		}
		if r := syncStateDestructiveReason(pr, workspaceRoot, domain); r != "" {
			reasons = append(reasons, r)
		}
		if r := syncStateCollisionReason(change, workspaceRoot, domain); r != "" {
			reasons = append(reasons, r)
		}
	}
	return reasons
}

func syncStateRenamedReason(hasRenamed bool, domain string) string {
	if !hasRenamed {
		return ""
	}
	return fmt.Sprintf("sync blocked: delta for domain %q contains ## RENAMED; rewrite as ADDED+REMOVED", domain)
}

func syncStateLegacyReason(workspaceRoot, domain string) string {
	mainPath := filepath.Join(workspaceRoot, "openspec", "specs", domain, "spec.md")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		return ""
	}
	if isLegacyFlat(string(data)) {
		return fmt.Sprintf("sync blocked: main spec for domain %q is legacy flat; convert to use ### Requirement: headings", domain)
	}
	return ""
}

func syncStateDestructiveReason(pr ParseResult, workspaceRoot, domain string) string {
	mainPath := filepath.Join(workspaceRoot, "openspec", "specs", domain, "spec.md")
	var mainContent string
	if data, err := os.ReadFile(mainPath); err == nil {
		mainContent = string(data)
	}
	if syncIsDestructive(pr.Deltas, mainContent) {
		return fmt.Sprintf("sync blocked: destructive change (REMOVED or large MODIFIED) for domain %q without explicit approval; add allow-destructive to prompt", domain)
	}
	return ""
}

func syncStateCollisionReason(change, workspaceRoot, domain string) string {
	collides, other := detectCollision(change, workspaceRoot, domain)
	if !collides {
		return ""
	}
	return fmt.Sprintf("sync blocked: collision without order for domain %q collides with %q", domain, other)
}
