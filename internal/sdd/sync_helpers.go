package sdd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/pathidentity"
)

// domainSource is one read of a change's delta file for a domain: the raw
// bytes plus the flags the write rules need (empty, full-spec shaped) and the
// deltas parsed from that same read.
type domainSource struct {
	path     string
	bytes    []byte
	empty    bool
	fullSpec bool
	deltas   []RequirementDelta
}

type domainInfo struct {
	domain     string
	deltas     []RequirementDelta
	hasRenamed bool
	sources    []domainSource
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

// isFullSpecShaped reports whether content carries requirement headings but
// no ADDED/MODIFIED/REMOVED/RENAMED section: the full-spec dialect sdd-spec
// mandates for new domains (versus the delta dialect for existing ones).
func isFullSpecShaped(content []byte) bool {
	return requirementHeadingRe.Match(content) && !deltaSectionRe.Match(content)
}

func syncParseDomainInfos(changeRoot string) ([]domainInfo, SyncResult, string, error) {
	files := findSpecFiles(filepath.Join(changeRoot, "specs"))
	if len(files) == 0 {
		return nil, SyncNotApplicable, "no delta specs", nil
	}
	var infos []domainInfo
	domainIndex := map[string]int{}
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
		idx, seen := domainIndex[domain]
		if !seen {
			// findSpecFiles returns sorted paths, so infos stay deterministic.
			infos = append(infos, domainInfo{domain: domain})
			idx = len(infos) - 1
			domainIndex[domain] = idx
		}
		info := &infos[idx]
		info.sources = append(info.sources, domainSource{
			path:     f,
			bytes:    contentBytes,
			empty:    strings.TrimSpace(string(contentBytes)) == "",
			fullSpec: isFullSpecShaped(contentBytes),
			deltas:   pr.Deltas,
		})
		info.deltas = append(info.deltas, pr.Deltas...)
		if pr.HasRenamed {
			info.hasRenamed = true
		}
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

// syncWriteBlockedMessage renders the fixed blocked shape of the sync write
// path: "sync blocked: <diagnosis> (domain <D>, file <path>); <remedy>". The
// domain, the offending file and the remedy are always present.
func syncWriteBlockedMessage(reason string, info domainInfo, file, remedy string) string {
	return fmt.Sprintf("sync blocked: %s (domain %s, file %s); %s", reason, info.domain, file, remedy)
}

// syncResolveDomainWrite decides what the sync writer may do for one domain.
// Rules: (1) when the living spec is absent and exactly one non-empty
// full-spec source exists (requirement headings, no delta section), its raw
// bytes are returned for a byte-identical copy; (2) a non-empty source
// without requirement blocks fails closed; (3) sync never writes empty
// content over a non-empty source and a write that produced no content is
// never resolved as `applied`. An existing target that byte-equals the
// full-spec source is a skip (idempotent re-run); an existing target that
// differs from it blocks (D8, stricter than "MUST NOT be replaced").
//
// Return contract: non-nil bytes + `applied` means the caller writes them;
// nil bytes + `blocked` means fail closed; nil bytes + "" means skip.
func syncResolveDomainWrite(info domainInfo, mainPath string) ([]byte, SyncResult, string, error) {
	mainBytes, mainExists, err := syncReadMainSpec(mainPath)
	if err != nil {
		return nil, "", "", err
	}
	nonEmpty, fullSpecs := syncSplitSources(info.sources)
	if len(nonEmpty) == 0 {
		return syncBlockEmptySources(info, mainPath)
	}
	if len(fullSpecs) > 0 {
		return syncResolveFullSpecWrite(info, mainBytes, mainExists, nonEmpty, fullSpecs)
	}
	return syncResolveDeltaWrite(info, mainBytes, nonEmpty)
}

// syncReadMainSpec reads the living spec at mainPath. A missing file is not an
// error: exists=false lets the caller treat the target as absent, while any
// other read failure is returned wrapped.
func syncReadMainSpec(mainPath string) ([]byte, bool, error) {
	mainBytes, err := os.ReadFile(mainPath)
	if err == nil {
		return mainBytes, true, nil
	}
	if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("read main spec %s: %w", mainPath, err)
	}
	return nil, false, nil
}

// syncSplitSources partitions the reads of one domain into the non-empty
// sources and the full-spec shaped ones, both in input order.
func syncSplitSources(sources []domainSource) (nonEmpty, fullSpecs []domainSource) {
	for _, source := range sources {
		if source.empty {
			continue
		}
		nonEmpty = append(nonEmpty, source)
		if source.fullSpec {
			fullSpecs = append(fullSpecs, source)
		}
	}
	return nonEmpty, fullSpecs
}

// syncBlockEmptySources fails closed for a domain whose delta files all carry
// no content, reporting the first source path when one exists.
func syncBlockEmptySources(info domainInfo, mainPath string) ([]byte, SyncResult, string, error) {
	file := mainPath
	if len(info.sources) > 0 {
		file = info.sources[0].path
	}
	return nil, SyncBlocked, syncWriteBlockedMessage("delta file is empty", info, file, "add requirement content or delete the empty delta file"), nil
}

// syncResolveFullSpecWrite classifies a domain with at least one full-spec
// shaped source: two or more candidates and a full-spec mixed with delta
// sources both fail closed, otherwise the single candidate decides the write.
func syncResolveFullSpecWrite(info domainInfo, mainBytes []byte, mainExists bool, nonEmpty, fullSpecs []domainSource) ([]byte, SyncResult, string, error) {
	if len(fullSpecs) >= 2 {
		paths := make([]string, 0, len(fullSpecs))
		for _, source := range fullSpecs {
			paths = append(paths, source.path)
		}
		return nil, SyncBlocked, syncWriteBlockedMessage("ambiguous full-spec sources: "+strings.Join(paths, ", "), info, fullSpecs[0].path, "keep exactly one full-spec file per domain, or rewrite the extras as ADDED/MODIFIED/REMOVED deltas"), nil
	}
	source := fullSpecs[0]
	if len(nonEmpty) > 1 {
		return nil, SyncBlocked, syncWriteBlockedMessage("full-spec source mixed with delta sources", info, source.path, "keep either the full-spec file or the delta files for this domain, not both"), nil
	}
	return syncWriteFullSpecTarget(info, mainBytes, mainExists, source)
}

// syncWriteFullSpecTarget decides one full-spec write against the living spec:
// an absent target takes the verbatim copy, an equal target is a skip
// (idempotent re-run) and a differing target fails closed (D8).
func syncWriteFullSpecTarget(info domainInfo, mainBytes []byte, mainExists bool, source domainSource) ([]byte, SyncResult, string, error) {
	if !mainExists {
		return source.bytes, SyncApplied, "", nil
	}
	if bytes.Equal(mainBytes, source.bytes) {
		return nil, "", "", nil
	}
	return nil, SyncBlocked, syncWriteBlockedMessage("existing living spec differs from the full-spec source", info, source.path, "reconcile openspec/specs/"+info.domain+"/spec.md with the change file, or rewrite the delta as ADDED/MODIFIED/REMOVED"), nil
}

// syncResolveDeltaWrite applies the parsed deltas of a domain whose sources
// are all delta shaped. A contentful source without requirement blocks fails
// closed, and a write that produced no content is never resolved as applied.
func syncResolveDeltaWrite(info domainInfo, mainBytes []byte, nonEmpty []domainSource) ([]byte, SyncResult, string, error) {
	for _, source := range nonEmpty {
		if len(source.deltas) == 0 {
			return nil, SyncBlocked, syncWriteBlockedMessage("delta file has content but no requirement blocks", info, source.path, "add a `## ADDED Requirements` section with `### Requirement:` blocks, or convert the file to full-spec shape"), nil
		}
	}
	newContent, err := ApplyDeltas(string(mainBytes), info.deltas)
	if err != nil {
		return nil, SyncBlocked, syncWriteBlockedMessage("apply deltas failed: "+err.Error(), info, nonEmpty[0].path, "fix the delta file so every requirement applies cleanly"), nil
	}
	if strings.TrimSpace(newContent) == "" {
		return nil, SyncBlocked, syncWriteBlockedMessage("delta produces no writable content", info, nonEmpty[0].path, "add requirement content; sync never writes empty content over a non-empty source"), nil
	}
	return []byte(newContent), SyncApplied, "", nil
}

func syncApplyDeltas(infos []domainInfo, workspaceRoot string) (SyncResult, string, error) {
	type pendingWrite struct {
		path  string
		bytes []byte
	}
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", "", fmt.Errorf("abs workspace root: %w", err)
	}
	// Resolve every domain before writing anything: a domain that blocks must
	// not leave earlier writes behind, and `applied` only follows a content
	// write (`sync.go` maps the empty success result to applied).
	var writes []pendingWrite
	for _, info := range infos {
		mainPath := filepath.Join(workspaceRoot, "openspec", "specs", info.domain, "spec.md")
		absTarget, err := filepath.Abs(mainPath)
		if err != nil {
			return "", "", fmt.Errorf("abs main spec %s: %w", mainPath, err)
		}
		// Path traversal fix (P1): use inode-aware containment via pathidentity.Contains
		// instead of lexical strings.HasPrefix (bypass: /tmp/root vs /tmp/root-evil).
		// Mirrors edit_authority.withinAnyRoot pattern; blocks prefix-only containment.
		if !pathidentity.Contains(absRoot, absTarget) {
			return SyncBlocked, fmt.Sprintf("sync blocked: target %s outside allowed edit roots", mainPath), nil
		}
		resolved, res, msg, err := syncResolveDomainWrite(info, mainPath)
		if err != nil {
			return "", "", err
		}
		if res != "" && res != SyncApplied {
			return res, msg, nil
		}
		if resolved != nil {
			writes = append(writes, pendingWrite{path: mainPath, bytes: resolved})
		}
	}
	for _, write := range writes {
		if err := os.MkdirAll(filepath.Dir(write.path), 0755); err != nil {
			return "", "", fmt.Errorf("mkdir for %s: %w", write.path, err)
		}
		if err := os.WriteFile(write.path, write.bytes, 0644); err != nil {
			return "", "", fmt.Errorf("write main spec %s: %w", write.path, err)
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
