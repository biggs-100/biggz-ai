package review

// Single lineage identity — design D1. One derivation serves the review
// offer, the gate lookup, and the terminal receipt:
//
//	review-<hex(sha256("biggz-ai.review-start-lineage/v1" NUL
//	     {"worktree_identity":<canonical git common dir>,
//	      "target_identity":<full subject commit SHA>}))[:16]>
//
// The worktree identity is the canonical git common dir (the store scope),
// never the worktree path and never the caller's cwd; the target identity is
// the full object SHA of the subject commit. Both are resolved from the
// repository, so the same repo + same target derive the same identity across
// runs and from any working directory.
//
// Canonicalization accepts an absent target (legacy organic subjects without
// a commit SHA bind to the current HEAD, as risk derivation documents),
// abbreviated and symbolic targets (`HEAD`); a non-empty subject that does
// not resolve refuses typed and callers persist nothing.

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// lineageIdentityDomain is the literal domain string hashed into every
	// lineage identity. It is part of the D1 contract: changing it changes
	// every derived id.
	lineageIdentityDomain = "biggz-ai.review-start-lineage/v1"
	// lineageIdentityPrefix opens every lineage id.
	lineageIdentityPrefix = "review-"
	// lineageIdentityHexLen is the number of digest hex characters kept.
	lineageIdentityHexLen = 16
)

// Lineage identity refusal codes.
const (
	// LineageIdentityUnresolvableCode refuses a non-empty subject that does
	// not resolve to a commit object (unknown, ambiguous, or not a commit).
	LineageIdentityUnresolvableCode = "unresolvable_subject_commit"
	// LineageIdentityRepoScopeCode refuses a derivation whose store scope
	// (the canonical git common dir) cannot be resolved.
	LineageIdentityRepoScopeCode = "unresolvable_repository_scope"
)

// LineageIdentityRefusal is the typed refusal for a lineage identity that
// cannot be derived: an unresolvable subject commit or an unresolvable
// repository scope. On refusal the caller persists nothing.
type LineageIdentityRefusal struct {
	Code   string
	Detail string
	err    error
}

// Error implements error.
func (r *LineageIdentityRefusal) Error() string {
	if r.err != nil {
		return fmt.Sprintf("review lineage identity: %s: %s: %v", r.Code, r.Detail, r.err)
	}
	return fmt.Sprintf("review lineage identity: %s: %s", r.Code, r.Detail)
}

// Unwrap exposes the underlying cause for errors.Is/errors.As.
func (r *LineageIdentityRefusal) Unwrap() error { return r.err }

// lineageIdentityMaterial is the exact JSON hashed into the identity. The
// field names, their order, and the NUL domain separator are the contract.
type lineageIdentityMaterial struct {
	WorktreeIdentity string `json:"worktree_identity"`
	TargetIdentity   string `json:"target_identity"`
}

// CanonicalSubjectSHA resolves raw to the full commit object SHA via
// `rev-parse <raw>^{commit}`. Full SHAs are returned unchanged; abbreviated
// and symbolic values (`HEAD`) resolve to their full SHA, and an absent value
// ("" or whitespace) binds to the current `HEAD` — the documented legacy
// organic subject contract (subjects without a commit SHA bind to HEAD), so
// the resolved full SHA is what callers persist. A non-empty unknown,
// ambiguous, or non-commit value refuses typed.
func CanonicalSubjectSHA(repo, raw string) (string, error) {
	target := cmp.Or(strings.TrimSpace(raw), "HEAD")
	out, err := gitIn(repo, "rev-parse", "--verify", "--quiet", target+"^{commit}")
	if err != nil || !validCommitSHA(out) {
		return "", &LineageIdentityRefusal{
			Code:   LineageIdentityUnresolvableCode,
			Detail: fmt.Sprintf("subject commit %q does not resolve to a commit object", raw),
			err:    err,
		}
	}
	return out, nil
}

// DeriveLineageID derives the single lineage identity for a subject commit:
// canonicalize the target to its full SHA, resolve the store scope (canonical
// git common dir), then hash the domain-separated material. The derivation is
// deterministic: same repo + same target ⇒ same id, from any cwd.
func DeriveLineageID(repo, subjectSHA string) (string, error) {
	target, err := CanonicalSubjectSHA(repo, subjectSHA)
	if err != nil {
		return "", err
	}
	commonDir, err := resolveGitCommonDir(repo)
	if err != nil {
		return "", &LineageIdentityRefusal{
			Code:   LineageIdentityRepoScopeCode,
			Detail: fmt.Sprintf("cannot resolve the canonical git common dir for %s", lineageIdentityRepoLabel(repo)),
			err:    err,
		}
	}
	material, err := json.Marshal(lineageIdentityMaterial{WorktreeIdentity: commonDir, TargetIdentity: target})
	if err != nil {
		// Unreachable for two strings; never panic on provider input.
		return "", fmt.Errorf("review lineage identity: encode material: %w", err)
	}
	sum := sha256.Sum256(append([]byte(lineageIdentityDomain+"\x00"), material...))
	return lineageIdentityPrefix + hex.EncodeToString(sum[:])[:lineageIdentityHexLen], nil
}

// lineageIdentityRepoLabel names the repository in refusal detail.
func lineageIdentityRepoLabel(repo string) string {
	if trimmed := strings.TrimSpace(repo); trimmed != "" {
		return trimmed
	}
	return "the current directory"
}
