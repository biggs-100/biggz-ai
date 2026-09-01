package sdd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type domainInfo struct {
	domain     string
	deltas     []RequirementDelta
	hasRenamed bool
}

type syncGuardrails struct {
	legacyDomains      []string
	destructiveDomains []string
	collisionInfo      []string
}

func syncCheckStoreGate(workspaceRoot string) (ArtifactStore, SyncResult, string) {
	store := declaredArtifactStore(workspaceRoot)
	if store == "" || IsEngramStore(store) || store == ArtifactStoreNone {
		return store, SyncNotApplicable, "sync not applicable for store " + string(store)
	}
	return store, "", ""
}

func syncResolveChangeRoot(change, workspaceRoot string) (string, SyncResult, string, error) {
	changeRoot := filepath.Join(workspaceRoot, "openspec", "changes", change)
	if _, err := os.Stat(changeRoot); err != nil {
		if os.IsNotExist(err) {
			return "", SyncNotApplicable, "change not found", nil
		}
		return "", "", "", fmt.Errorf("stat change root: %w", err)
	}
	if !hasSyncDeltas(changeRoot) {
		return changeRoot, SyncNotApplicable, "no delta specs for change " + change, nil
	}
	return changeRoot, "", "", nil
}

func syncVerifyMustPass(changeRoot string, store ArtifactStore) (SyncResult, string, error) {
	_, artifacts, _, _, _, verifyResult, err := collectArtifactDerivation(changeRoot, store)
	if err != nil {
		return "", "", err
	}
	if artifacts["verifyReport"] != ArtifactDone || !verifyResult.Passing {
		reason := verifyResult.Reason
		if reason == "" {
			reason = "verify result is missing or not PASS"
		}
		return SyncBlocked, "sync blocked: verify must be PASS before sync (" + reason + ")", nil
	}
	return "", "", nil
}

func domainFromSpecPath(specPath string) string {
	domain := filepath.Base(filepath.Dir(specPath))
	if domain == "specs" {
		return "unknown"
	}
	return domain
}

func syncParseDomainInfos(changeRoot string) ([]domainInfo, SyncResult, string, error) {
	files := findSpecFiles(filepath.Join(changeRoot, "specs"))
	if len(files) == 0 {
		return nil, SyncNotApplicable, "no delta specs", nil
	}
	domainToDeltas := map[string][]RequirementDelta{}
	domainHasRenamed := map[string]bool{}
	for _, f := range files {
		contentBytes, err := os.ReadFile(f)
		if err != nil {
			return nil, "", "", fmt.Errorf("read delta %s: %w", f, err)
		}
		pr, err := ParseDeltaSpec(string(contentBytes))
		if err != nil {
			return nil, "", "", fmt.Errorf("parse delta %s: %w", f, err)
		}
		domain := domainFromSpecPath(f)
		domainToDeltas[domain] = append(domainToDeltas[domain], pr.Deltas...)
		if pr.HasRenamed {
			domainHasRenamed[domain] = true
		}
	}
	var infos []domainInfo
	for domain, deltas := range domainToDeltas {
		infos = append(infos, domainInfo{domain: domain, deltas: deltas, hasRenamed: domainHasRenamed[domain]})
	}
	return infos, "", "", nil
}

func syncGuardRenamed(infos []domainInfo, hasResolveViaEngram bool) (SyncResult, string) {
	for _, info := range infos {
		if info.hasRenamed && !hasResolveViaEngram {
			return SyncBlocked, fmt.Sprintf("sync blocked: delta contains ## RENAMED for domain %q; rewrite as ADDED+REMOVED", info.domain)
		}
	}
	return "", ""
}

func syncIsDestructive(deltas []RequirementDelta, mainContent string) bool {
	for _, d := range deltas {
		if d.Kind == DeltaRemoved {
			return true
		}
		if d.Kind == DeltaModified {
			_, _, blocks := parseMainSpec(mainContent)
			if oldBody, ok := blocks[d.Name]; ok {
				if isLargeModification(oldBody, d.Body) {
					return true
				}
				continue
			}
			if len(strings.Split(strings.TrimSpace(d.Body), "\n")) > largeMutationThreshold {
				return true
			}
		}
	}
	return false
}

func syncCheckCollision(change, workspaceRoot, domain, promptLower string, hasResolveViaEngram bool) (bool, string) {
	if hasResolveViaEngram {
		return false, ""
	}
	collides, other := detectCollision(change, workspaceRoot, domain)
	if !collides {
		return false, ""
	}
	ordered := strings.Contains(promptLower, "ordered") || strings.Contains(promptLower, "allow-collision")
	if ordered {
		return false, ""
	}
	return true, fmt.Sprintf("%s (collides with %s)", domain, other)
}

func syncCollectGuardrails(infos []domainInfo, workspaceRoot, change, promptLower string, hasResolveViaEngram bool) syncGuardrails {
	var g syncGuardrails
	for _, info := range infos {
		mainPath := filepath.Join(workspaceRoot, "openspec", "specs", info.domain, "spec.md")
		var mainContent string
		if data, err := os.ReadFile(mainPath); err == nil {
			mainContent = string(data)
			if isLegacyFlat(mainContent) && !hasResolveViaEngram {
				g.legacyDomains = append(g.legacyDomains, info.domain)
			}
		}
		if syncIsDestructive(info.deltas, mainContent) {
			g.destructiveDomains = append(g.destructiveDomains, info.domain)
		}
		if collides, infoStr := syncCheckCollision(change, workspaceRoot, info.domain, promptLower, hasResolveViaEngram); collides {
			g.collisionInfo = append(g.collisionInfo, infoStr)
		}
	}
	return g
}

func syncBlockOnGuardrails(g syncGuardrails, promptLower string, hasResolveViaEngram bool) (SyncResult, string) {
	if len(g.legacyDomains) > 0 && !hasResolveViaEngram {
		return SyncBlocked, fmt.Sprintf("sync blocked: main spec legacy flat for domain(s) %s; convert to use ### Requirement: headings", strings.Join(g.legacyDomains, ", "))
	}
	if len(g.destructiveDomains) > 0 && !hasResolveViaEngram {
		allowDestructive := strings.Contains(promptLower, "allow-destructive")
		if !allowDestructive {
			return SyncBlocked, fmt.Sprintf("sync blocked: destructive change (REMOVED or large MODIFIED) for domain(s) %s without explicit approval; add allow-destructive to prompt", strings.Join(g.destructiveDomains, ", "))
		}
	}
	if len(g.collisionInfo) > 0 && !hasResolveViaEngram {
		return SyncBlocked, fmt.Sprintf("sync blocked: collision without order for domain(s) %s; resolve ordering or add resolve-via-engram", strings.Join(g.collisionInfo, ", "))
	}
	return "", ""
}

func syncApplyDeltas(infos []domainInfo, workspaceRoot string) (SyncResult, string, error) {
	for _, info := range infos {
		mainPath := filepath.Join(workspaceRoot, "openspec", "specs", info.domain, "spec.md")
		var mainContent string
		if data, err := os.ReadFile(mainPath); err == nil {
			mainContent = string(data)
		} else if !os.IsNotExist(err) {
			return "", "", fmt.Errorf("read main spec %s: %w", mainPath, err)
		}
		newContent, err := ApplyDeltas(mainContent, info.deltas)
		if err != nil {
			return SyncBlocked, fmt.Sprintf("sync blocked: apply deltas for domain %q failed: %v", info.domain, err), nil
		}
		if err := os.MkdirAll(filepath.Dir(mainPath), 0755); err != nil {
			return "", "", fmt.Errorf("mkdir for %s: %w", mainPath, err)
		}
		absTarget, _ := filepath.Abs(mainPath)
		absRoot, _ := filepath.Abs(workspaceRoot)
		if !strings.HasPrefix(absTarget, absRoot) {
			return SyncBlocked, fmt.Sprintf("sync blocked: target %s outside allowed edit roots", mainPath), nil
		}
		if err := os.WriteFile(mainPath, []byte(newContent), 0644); err != nil {
			return "", "", fmt.Errorf("write main spec %s: %w", mainPath, err)
		}
	}
	return "", "", nil
}

func syncEnsureNotDisappeared(changeRoot string) error {
	if _, err := os.Stat(changeRoot); err != nil {
		return fmt.Errorf("change dir disappeared after sync: %w", err)
	}
	return nil
}
