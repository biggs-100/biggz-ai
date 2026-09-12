package review

// Lineage resolution tests (tasks 4.1–4.2, design D1): the candidate-lineage
// resolver is derived-first, reads legacy lineages (abbreviated subjects,
// UUIDv7 ids) without ever rewriting them, orders matches newest-first, and
// fails closed with a typed refusal when nothing resolves. It never falls
// back to a bare change name.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
)

// resolverRepo creates a git repository with a base and a candidate commit.
func resolverRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitInit(t, repo)
	writeLineageIdentityFile(t, filepath.Join(repo, "base.txt"), "base\n")
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "base")
	writeLineageIdentityFile(t, filepath.Join(repo, "candidate.txt"), "candidate\n")
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "candidate")
	return repo
}

// seedResolverLineage appends a start_review genesis carrying commitSHA for
// lineageID at timestamp and returns the lineage store directory. The subject
// bytes are persisted verbatim, exactly like a legacy `review start` wrote
// them (abbreviated or symbolic values included).
func seedResolverLineage(t *testing.T, repo, lineageID, commitSHA, timestamp string) string {
	t.Helper()
	store, err := Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open(%s): %v", lineageID, err)
	}
	payload, err := json.Marshal(StartEventPayload{
		Schema:     ReviewStartEventSchema,
		Repository: repo,
		CommitSHA:  commitSHA,
	})
	if err != nil {
		t.Fatalf("marshal genesis: %v", err)
	}
	if _, err := store.Append("", Record{
		Operation: "start_review",
		Role:      string(model.RoleReviewer),
		Actor:     string(model.RoleReviewer),
		Timestamp: timestamp,
		Payload:   payload,
	}); err != nil {
		t.Fatalf("append genesis for %s: %v", lineageID, err)
	}
	return store.Dir
}

// resolverStoreRoot returns the review-transactions root for repo.
func resolverStoreRoot(t *testing.T, repo string) string {
	t.Helper()
	return filepath.Join(lineageIdentityCommonDir(t, repo), "biggz", "review-transactions")
}

// lineageBytesSnapshot fingerprints every file under root by relative path
// and content hash so a resolution can be proven read-only.
func lineageBytesSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		payload, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		sum := sha256.Sum256(payload)
		snapshot[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snapshot
}

// lineageSnapshotsEqual reports whether two store snapshots are identical.
func lineageSnapshotsEqual(before, after map[string]string) bool {
	if len(before) != len(after) {
		return false
	}
	for path, hash := range before {
		if after[path] != hash {
			return false
		}
	}
	return true
}

// assertLineageBytesUnchanged fails when resolution rewrote, removed, or
// added any store byte.
func assertLineageBytesUnchanged(t *testing.T, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("store file count changed: %d → %d\nbefore=%v\nafter=%v", len(before), len(after), before, after)
	}
	for path, hash := range before {
		got, ok := after[path]
		if !ok {
			t.Errorf("store file %s disappeared during resolution", path)
			continue
		}
		if got != hash {
			t.Errorf("store file %s was rewritten during resolution", path)
		}
	}
}

func TestResolveCandidateLineageDerivedFirst(t *testing.T) {
	repo := resolverRepo(t)
	full := runGitInDir(t, repo, "rev-parse", "HEAD")
	derived, err := DeriveLineageID(repo, full)
	if err != nil {
		t.Fatalf("DeriveLineageID: %v", err)
	}
	assertLineageIDFormat(t, derived)

	// A competing legacy lineage for the same subject exists AND is newer by
	// timestamp; the derived identity must still win (derived-first).
	legacyID := "01932d0a-7f4e-7c31-9a6b-2f6f6b6c0001"
	seedResolverLineage(t, repo, legacyID, full, "2026-09-11T12:00:00Z")
	seedResolverLineage(t, repo, derived, full, "2026-09-10T12:00:00Z")

	got, err := ResolveCandidateLineage(repo, "HEAD^{commit}")
	if err != nil {
		t.Fatalf("ResolveCandidateLineage: %v", err)
	}
	if got != derived {
		t.Fatalf("resolved %s, want the derived identity %s (derived-first)", got, derived)
	}

	// Abbreviated candidates canonicalize to the same resolution.
	abbrev := runGitInDir(t, repo, "rev-parse", "--short=8", "HEAD")
	again, err := ResolveCandidateLineage(repo, abbrev)
	if err != nil {
		t.Fatalf("ResolveCandidateLineage(abbreviated): %v", err)
	}
	if again != derived {
		t.Fatalf("abbreviated candidate resolved %s, want %s", again, derived)
	}
}

func TestResolveCandidateLineageLegacyAbbreviatedReadOnly(t *testing.T) {
	repo := resolverRepo(t)
	full := runGitInDir(t, repo, "rev-parse", "HEAD")
	abbrev := runGitInDir(t, repo, "rev-parse", "--short=8", "HEAD")
	if abbrev == full {
		t.Fatalf("abbreviated %q equals full %q; canonicalization cannot be proven", abbrev, full)
	}
	derived, err := DeriveLineageID(repo, full)
	if err != nil {
		t.Fatalf("DeriveLineageID: %v", err)
	}

	// A pre-derivation lineage: UUIDv7-style id, abbreviated subject persisted.
	legacyID := "01932d0a-7f4e-7c31-9a6b-2f6f6b6c0002"
	seedResolverLineage(t, repo, legacyID, abbrev, "2026-09-11T09:00:00Z")

	root := resolverStoreRoot(t, repo)
	if _, err := os.Stat(filepath.Join(root, derived)); err == nil {
		t.Fatalf("derived store %s unexpectedly present; the scan path would not be exercised", derived)
	}
	before := lineageBytesSnapshot(t, root)

	got, err := ResolveCandidateLineage(repo, "HEAD^{commit}")
	if err != nil {
		t.Fatalf("ResolveCandidateLineage: %v", err)
	}
	if got != legacyID {
		t.Fatalf("resolved %s, want legacy %s (abbreviated subject must stay readable)", got, legacyID)
	}

	after := lineageBytesSnapshot(t, root)
	assertLineageBytesUnchanged(t, before, after)

	// A second resolution is stable and still read-only.
	second, err := ResolveCandidateLineage(repo, full)
	if err != nil || second != legacyID {
		t.Fatalf("second resolution = %q err %v, want %s", second, err, legacyID)
	}
	assertLineageBytesUnchanged(t, before, lineageBytesSnapshot(t, root))
}

func TestResolveCandidateLineageNewestFirst(t *testing.T) {
	repo := resolverRepo(t)
	full := runGitInDir(t, repo, "rev-parse", "HEAD")
	base := runGitInDir(t, repo, "rev-parse", "HEAD~1")

	// A non-matching lineage with the newest timestamp must never shadow the
	// matching pair: matching is by canonical subject, ordering only breaks
	// ties between matches.
	seedResolverLineage(t, repo, "01932d0a-7f4e-7c31-9a6b-2f6f6b6c0009", base, "2026-09-11T23:59:00Z")
	older := "01932d0a-7f4e-7c31-9a6b-2f6f6b6c0003"
	newer := "01932d0a-7f4e-7c31-9a6b-2f6f6b6c0004"
	seedResolverLineage(t, repo, older, full, "2026-09-10T10:00:00Z")
	seedResolverLineage(t, repo, newer, full, "2026-09-11T10:00:00Z")

	got, err := ResolveCandidateLineage(repo, full)
	if err != nil {
		t.Fatalf("ResolveCandidateLineage: %v", err)
	}
	if got != newer {
		t.Fatalf("resolved %s, want the newest matching lineage %s", got, newer)
	}

	// Stable across repeated resolutions.
	again, err := ResolveCandidateLineage(repo, "HEAD^{commit}")
	if err != nil || again != newer {
		t.Fatalf("repeated resolution = %q err %v, want %s", again, err, newer)
	}
}

func TestResolveCandidateLineageRefusesTyped(t *testing.T) {
	repo := resolverRepo(t)
	full := runGitInDir(t, repo, "rev-parse", "HEAD")
	base := runGitInDir(t, repo, "rev-parse", "HEAD~1")

	// Nothing in the store: typed resolution refusal.
	_, err := ResolveCandidateLineage(repo, "HEAD^{commit}")
	var refusal *LineageResolutionRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err %v is not *LineageResolutionRefusal", err)
	}
	if refusal.Code != LineageResolutionUnresolvedCode {
		t.Errorf("code = %q, want %q", refusal.Code, LineageResolutionUnresolvedCode)
	}

	// A lineage for a different subject never matches: no bare-name fallback.
	seedResolverLineage(t, repo, "legacy-other-subject", base, "2026-09-11T10:00:00Z")
	if _, err := ResolveCandidateLineage(repo, full); !errors.As(err, &refusal) {
		t.Fatalf("mismatched lineage err = %v, want *LineageResolutionRefusal", err)
	}

	// An unresolvable candidate refuses through the typed identity refusal.
	_, err = ResolveCandidateLineage(repo, strings.Repeat("f", 40))
	var identityRefusal *LineageIdentityRefusal
	if !errors.As(err, &identityRefusal) {
		t.Fatalf("unresolvable candidate err %v is not *LineageIdentityRefusal", err)
	}
	if identityRefusal.Code != LineageIdentityUnresolvableCode {
		t.Errorf("identity code = %q, want %q", identityRefusal.Code, LineageIdentityUnresolvableCode)
	}

	// Outside a git repository the resolver refuses instead of guessing.
	if _, err := ResolveCandidateLineage(t.TempDir(), "HEAD^{commit}"); err == nil {
		t.Error("resolution outside a git repository must refuse")
	}
}

func TestResolveCandidateLineageUnreadableStoreRefusesTyped(t *testing.T) {
	repo := resolverRepo(t)
	root := resolverStoreRoot(t, repo)
	if err := os.MkdirAll(filepath.Dir(root), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(root), err)
	}
	// A regular file where the store root belongs makes the scan unreadable
	// on every platform (ENOTDIR / not-a-directory).
	const sentinel = "not a directory\n"
	if err := os.WriteFile(root, []byte(sentinel), 0644); err != nil {
		t.Fatalf("write %s: %v", root, err)
	}

	_, err := ResolveCandidateLineage(repo, "HEAD^{commit}")
	var refusal *LineageResolutionRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("unreadable store err %v is not *LineageResolutionRefusal", err)
	}
	if refusal.Code != LineageResolutionUnresolvedCode {
		t.Errorf("code = %q, want %q", refusal.Code, LineageResolutionUnresolvedCode)
	}
	payload, readErr := os.ReadFile(root)
	if readErr != nil || string(payload) != sentinel {
		t.Errorf("resolution must not touch the unreadable store: err=%v payload=%q", readErr, payload)
	}
}

func TestResolveCandidateLineageRuntimeHarness(t *testing.T) {
	// (a) derived id present → resolves.
	repo := resolverRepo(t)
	full := runGitInDir(t, repo, "rev-parse", "HEAD")
	derived, err := DeriveLineageID(repo, full)
	if err != nil {
		t.Fatalf("DeriveLineageID: %v", err)
	}
	seedResolverLineage(t, repo, derived, full, "2026-09-11T10:00:00Z")
	fmt.Printf("harness: repo=%s full=%s derived=%s\n", repo, full, derived)
	fromDerived, err := ResolveCandidateLineage(repo, "HEAD^{commit}")
	fmt.Printf("harness: derived-present resolved=%s err=%v\n", fromDerived, err)
	if err != nil || fromDerived != derived {
		t.Fatalf("derived-present resolution = %q err %v, want %s", fromDerived, err, derived)
	}

	// (b) legacy lineage with an abbreviated subject and a UUIDv7-style id →
	// still resolves AND its bytes are unchanged afterwards.
	legacyRepo := resolverRepo(t)
	legacyFull := runGitInDir(t, legacyRepo, "rev-parse", "HEAD")
	legacyAbbrev := runGitInDir(t, legacyRepo, "rev-parse", "--short=8", "HEAD")
	legacyID := "01932d0a-7f4e-7c31-9a6b-2f6f6b6c0005"
	seedResolverLineage(t, legacyRepo, legacyID, legacyAbbrev, "2026-09-11T09:30:00Z")
	root := resolverStoreRoot(t, legacyRepo)
	before := lineageBytesSnapshot(t, root)
	fromLegacy, err := ResolveCandidateLineage(legacyRepo, "HEAD^{commit}")
	after := lineageBytesSnapshot(t, root)
	unchanged := lineageSnapshotsEqual(before, after)
	fmt.Printf("harness: legacy_id=%s full=%s subject=%s resolved=%s err=%v bytes_unchanged=%t\n",
		legacyID, legacyFull, legacyAbbrev, fromLegacy, err, unchanged)
	if err != nil || fromLegacy != legacyID {
		t.Fatalf("legacy resolution = %q err %v, want %s", fromLegacy, err, legacyID)
	}
	if !unchanged {
		t.Fatalf("resolution rewrote the legacy store: before=%v after=%v", before, after)
	}

	// (c) nothing present in the store → typed refusal.
	emptyRepo := resolverRepo(t)
	_, err = ResolveCandidateLineage(emptyRepo, "HEAD^{commit}")
	var refusal *LineageResolutionRefusal
	typed := errors.As(err, &refusal)
	fmt.Printf("harness: empty-store refusal err=%v typed=%t\n", err, typed)
	if !typed {
		t.Fatalf("empty-store err %v is not *LineageResolutionRefusal", err)
	}
	if refusal.Code != LineageResolutionUnresolvedCode {
		t.Errorf("refusal code = %q, want %q", refusal.Code, LineageResolutionUnresolvedCode)
	}
}
