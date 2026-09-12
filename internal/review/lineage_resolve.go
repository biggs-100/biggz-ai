package review

// Candidate lineage resolution — design D1 (gate half). The RDD gate must
// find the lineage that actually governs the candidate commit instead of
// passing the bare change name (bug #60):
//
//  1. canonicalize the candidate (`HEAD^{commit}`, abbreviated, or full) to
//     its full commit SHA;
//  2. derived-first: the single derived `review-<16hex>` identity (slice 3)
//     wins whenever it exists in the store;
//  3. otherwise scan `.git/biggz/review-transactions/*` newest-first for a
//     lineage whose genesis subject canonicalizes to the same commit. Legacy
//     lineages (abbreviated subject SHA, UUIDv7 id) stay readable;
//  4. nothing resolves → fail closed with a typed refusal. The bare change
//     name is never a fallback.
//
// The scan is read-path only: no lineage is migrated, renamed, backfilled,
// or rewritten, and its bytes stay untouched.

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Lineage resolution refusal codes.
const (
	// LineageResolutionUnresolvedCode refuses a candidate commit that has no
	// lineage in the store: neither the derived identity nor any legacy
	// lineage whose genesis subject resolves to it.
	LineageResolutionUnresolvedCode = "unresolved_candidate_lineage"
)

// LineageResolutionRefusal is the typed fail-closed refusal for a candidate
// without a resolvable lineage. Callers must never fall back to the bare
// change name.
type LineageResolutionRefusal struct {
	Code   string
	Detail string
	err    error
}

// Error implements error.
func (r *LineageResolutionRefusal) Error() string {
	if r.err != nil {
		return fmt.Sprintf("review lineage resolution: %s: %s: %v", r.Code, r.Detail, r.err)
	}
	return fmt.Sprintf("review lineage resolution: %s: %s", r.Code, r.Detail)
}

// Unwrap exposes the underlying cause for errors.Is/errors.As.
func (r *LineageResolutionRefusal) Unwrap() error { return r.err }

// lineageScanHit is one read-only scan match: the lineage id and the
// timestamp of its most recent event, used to order matches newest-first.
type lineageScanHit struct {
	id        string
	updatedAt time.Time
}

// ResolveCandidateLineage resolves the lineage that governs the candidate
// commit: derived-first, then a read-only newest-first scan of legacy
// lineages, then a typed refusal. The candidate may be any rev-parse input
// (`HEAD^{commit}`, abbreviated, or full); an unresolvable candidate refuses
// typed through CanonicalSubjectSHA.
func ResolveCandidateLineage(repo, candidateSHA string) (string, error) {
	candidate, err := CanonicalSubjectSHA(repo, candidateSHA)
	if err != nil {
		return "", err
	}
	derived, err := DeriveLineageID(repo, candidate)
	if err != nil {
		return "", err
	}
	storeRoot, err := lineageStoreRootDir(repo)
	if err != nil {
		return "", err
	}
	if lineageStoreEntryPresent(storeRoot, derived) {
		return derived, nil
	}
	if legacy, found, err := scanLegacyLineages(repo, storeRoot, derived, candidate); err != nil {
		return "", err
	} else if found {
		return legacy, nil
	}
	return "", &LineageResolutionRefusal{
		Code: LineageResolutionUnresolvedCode,
		Detail: fmt.Sprintf(
			"no review lineage for candidate %q (resolved commit %s): derived identity %s has no store entry and no legacy lineage genesis subject resolves to it",
			candidateSHA, candidate, derived),
	}
}

// lineageStoreRootDir resolves the read-only lineage store root
// (<canonical git common dir>/biggz/review-transactions).
func lineageStoreRootDir(repo string) (string, error) {
	commonDir, err := resolveGitCommonDir(repo)
	if err != nil {
		return "", &LineageIdentityRefusal{
			Code:   LineageIdentityRepoScopeCode,
			Detail: fmt.Sprintf("cannot resolve the canonical git common dir for %s", lineageIdentityRepoLabel(repo)),
			err:    err,
		}
	}
	return filepath.Join(commonDir, "biggz", "review-transactions"), nil
}

// lineageStoreEntryPresent reports whether the lineage directory exists in
// the store. It is a read-only stat; nothing is created.
func lineageStoreEntryPresent(storeRoot, lineageID string) bool {
	info, err := os.Stat(filepath.Join(storeRoot, lineageID))
	return err == nil && info.IsDir()
}

// scanLegacyLineages walks the store read-only for lineages whose genesis
// subject canonicalizes to the candidate commit. Matches are ordered
// newest-first (most recent event timestamp wins; lineage id breaks ties).
// A missing store root has no matches.
func scanLegacyLineages(repo, storeRoot, derived, candidate string) (string, bool, error) {
	entries, err := os.ReadDir(storeRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, &LineageResolutionRefusal{
			Code:   LineageResolutionUnresolvedCode,
			Detail: fmt.Sprintf("cannot read the lineage store root %s", storeRoot),
			err:    err,
		}
	}
	hits := make([]lineageScanHit, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == derived {
			continue
		}
		if hit, ok := legacyLineageHit(repo, filepath.Join(storeRoot, entry.Name()), entry.Name(), candidate); ok {
			hits = append(hits, hit)
		}
	}
	if len(hits) == 0 {
		return "", false, nil
	}
	slices.SortFunc(hits, compareLineageScanHits)
	return hits[0].id, true, nil
}

// legacyLineageHit reads one lineage read-only and reports whether its
// genesis subject resolves to the candidate commit. Unreadable, empty, or
// unresolvable lineages are skipped, never rewritten.
func legacyLineageHit(repo, dir, lineageID, candidate string) (lineageScanHit, bool) {
	store := OpenWithDir(dir, lineageID)
	chain, err := store.LoadChain()
	if err != nil || chain.Count == 0 {
		return lineageScanHit{}, false
	}
	var plan StartEventPayload
	if err := json.Unmarshal(chain.Records[0].Payload, &plan); err != nil {
		return lineageScanHit{}, false
	}
	raw := strings.TrimSpace(plan.CommitSHA)
	if raw == "" {
		return lineageScanHit{}, false
	}
	resolved, err := CanonicalSubjectSHA(repo, raw)
	if err != nil || resolved != candidate {
		return lineageScanHit{}, false
	}
	return lineageScanHit{
		id:        lineageID,
		updatedAt: lastRecordTime(chain.Records[len(chain.Records)-1].Timestamp),
	}, true
}

// lastRecordTime parses an event timestamp; unparsable timestamps order
// oldest so a deterministic id tiebreak still applies.
func lastRecordTime(raw string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// compareLineageScanHits orders matches newest-first with a stable id
// tiebreak.
func compareLineageScanHits(a, b lineageScanHit) int {
	if order := b.updatedAt.Compare(a.updatedAt); order != 0 {
		return order
	}
	return cmp.Compare(a.id, b.id)
}
