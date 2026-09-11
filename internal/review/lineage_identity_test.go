package review

// Lineage identity tests (tasks 3.1–3.3, design D1): the single-lineage
// derivation is exact and deterministic, canonicalizes abbreviated/symbolic
// targets to full SHAs, and rejects unresolvable subjects with a typed
// refusal that persists nothing.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// lineageIdentityRepo creates a git repository with one commit and a README.
func lineageIdentityRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitInit(t, repo)
	writeLineageIdentityFile(t, filepath.Join(repo, "README.md"), "# lineage identity\n")
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "initial")
	return repo
}

// writeLineageIdentityFile writes one file into a test repository.
func writeLineageIdentityFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// lineageIdentityCommonDir recomputes the canonical store scope independently
// of resolveGitCommonDir: a direct git read plus stdlib path canonicalization.
func lineageIdentityCommonDir(t *testing.T, repo string) string {
	t.Helper()
	commonDir := runGitInDir(t, repo, "rev-parse", "--git-common-dir")
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(repo, commonDir)
	}
	resolved, err := filepath.EvalSymlinks(commonDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", commonDir, err)
	}
	return filepath.Clean(resolved)
}

// expectedLineageIDFromFormula recomputes the D1 derivation independently of
// the implementation: domain + NUL + {"worktree_identity","target_identity"}
// → sha256 → first 16 lowercase hex, prefixed review-.
func expectedLineageIDFromFormula(t *testing.T, repo, fullSHA string) string {
	t.Helper()
	material, err := json.Marshal(struct {
		WorktreeIdentity string `json:"worktree_identity"`
		TargetIdentity   string `json:"target_identity"`
	}{WorktreeIdentity: lineageIdentityCommonDir(t, repo), TargetIdentity: fullSHA})
	if err != nil {
		t.Fatalf("marshal material: %v", err)
	}
	sum := sha256.Sum256(append([]byte("biggz-ai.review-start-lineage/v1\x00"), material...))
	return "review-" + hex.EncodeToString(sum[:])[:16]
}

var lineageIDPattern = regexp.MustCompile(`^review-[0-9a-f]{16}$`)

// assertLineageIDFormat asserts review-<16 lowercase hex>.
func assertLineageIDFormat(t *testing.T, id string) {
	t.Helper()
	if !lineageIDPattern.MatchString(id) {
		t.Fatalf("lineage id %q does not match review-<16 lowercase hex>", id)
	}
}

func TestLineageIdentityDerivationIsExactAndDeterministic(t *testing.T) {
	repo := lineageIdentityRepo(t)
	full := runGitInDir(t, repo, "rev-parse", "HEAD")
	abbrev := runGitInDir(t, repo, "rev-parse", "--short=8", "HEAD")
	if abbrev == full {
		t.Fatalf("abbreviated %q equals full %q; canonicalization cannot be proven", abbrev, full)
	}
	want := expectedLineageIDFromFormula(t, repo, full)

	for name, raw := range map[string]string{"full": full, "abbreviated": abbrev, "symbolic HEAD": "HEAD"} {
		got, err := DeriveLineageID(repo, raw)
		if err != nil {
			t.Fatalf("DeriveLineageID(%s=%q): %v", name, raw, err)
		}
		assertLineageIDFormat(t, got)
		if got != want {
			t.Errorf("DeriveLineageID(%s) = %s, want %s (the exact D1 formula)", name, got, want)
		}
	}

	// The same repo + same target derives the same identity across runs.
	second, err := DeriveLineageID(repo, abbrev)
	if err != nil {
		t.Fatalf("second DeriveLineageID: %v", err)
	}
	if second != want {
		t.Errorf("second run derived %s, want %s", second, want)
	}

	// A foreign cwd must not change the identity (repo is explicit, store
	// scope is the git common dir).
	t.Chdir(t.TempDir())
	fromForeign, err := DeriveLineageID(repo, full)
	if err != nil {
		t.Fatalf("DeriveLineageID from foreign cwd: %v", err)
	}
	if fromForeign != want {
		t.Errorf("foreign cwd derived %s, want %s", fromForeign, want)
	}

	// A linked worktree shares the store scope (common dir), so it derives
	// the same identity for the same target.
	worktree := filepath.Join(t.TempDir(), "identity-wt")
	runGitInDir(t, repo, "worktree", "add", worktree, "-b", "identity-wt-branch")
	fromWorktree, err := DeriveLineageID(worktree, full)
	if err != nil {
		t.Fatalf("DeriveLineageID from linked worktree: %v", err)
	}
	if fromWorktree != want {
		t.Errorf("linked worktree derived %s, want %s (store scope is the common dir)", fromWorktree, want)
	}

	// A different target derives a different identity.
	writeLineageIdentityFile(t, filepath.Join(repo, "second.txt"), "second\n")
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "second")
	otherHead := runGitInDir(t, repo, "rev-parse", "HEAD")
	otherID, err := DeriveLineageID(repo, otherHead)
	if err != nil {
		t.Fatalf("DeriveLineageID(other target): %v", err)
	}
	if otherID == want {
		t.Errorf("different targets derived the same identity %s", want)
	}
}

func TestLineageIdentityCanonicalSubjectSHA(t *testing.T) {
	repo := lineageIdentityRepo(t)
	full := runGitInDir(t, repo, "rev-parse", "HEAD")
	abbrev := runGitInDir(t, repo, "rev-parse", "--short=10", "HEAD")
	runGitInDir(t, repo, "tag", "lineage-identity-v1")

	cases := []struct {
		name, raw, want string
	}{
		{"full sha is idempotent", full, full},
		{"abbreviated sha resolves to full", abbrev, full},
		{"symbolic HEAD resolves to full", "HEAD", full},
		{"tag resolves to full", "lineage-identity-v1", full},
		{"commit expression resolves to full", "HEAD^{commit}", full},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CanonicalSubjectSHA(repo, tc.raw)
			if err != nil {
				t.Fatalf("CanonicalSubjectSHA(%q): %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("CanonicalSubjectSHA(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}

	treeSHA := runGitInDir(t, repo, "rev-parse", "HEAD^{tree}")
	blobSHA := runGitInDir(t, repo, "hash-object", "-w", "README.md")
	refusals := []struct{ name, raw string }{
		{"missing commit", strings.Repeat("f", 40)},
		{"malformed ref", "lineage/no-such-ref"},
		{"empty", ""},
		{"whitespace only", "   "},
		{"tree object", treeSHA},
		{"blob object", blobSHA},
	}
	for _, tc := range refusals {
		t.Run("refusal: "+tc.name, func(t *testing.T) {
			got, err := CanonicalSubjectSHA(repo, tc.raw)
			if err == nil {
				t.Fatalf("CanonicalSubjectSHA(%q) = %q, want a typed refusal", tc.raw, got)
			}
			if got != "" {
				t.Errorf("refusal returned a value %q, want empty", got)
			}
			var refusal *LineageIdentityRefusal
			if !errors.As(err, &refusal) {
				t.Fatalf("err %v is not *LineageIdentityRefusal", err)
			}
			if refusal.Code != LineageIdentityUnresolvableCode {
				t.Errorf("code = %q, want %q", refusal.Code, LineageIdentityUnresolvableCode)
			}
			if _, derr := DeriveLineageID(repo, tc.raw); derr == nil {
				t.Error("DeriveLineageID must refuse the same unresolvable subject")
			}
		})
	}

	// Outside a git repository the derivation must refuse typed, never guess.
	notARepo := t.TempDir()
	if _, err := DeriveLineageID(notARepo, "HEAD"); err == nil {
		t.Error("DeriveLineageID outside a git repository must refuse")
	} else {
		var refusal *LineageIdentityRefusal
		if !errors.As(err, &refusal) {
			t.Errorf("non-repo err %v is not *LineageIdentityRefusal", err)
		}
	}
}

func TestLineageIdentityRuntimeHarness(t *testing.T) {
	repo := lineageIdentityRepo(t)
	full := runGitInDir(t, repo, "rev-parse", "HEAD")
	abbrev := runGitInDir(t, repo, "rev-parse", "--short=8", "HEAD")
	common := lineageIdentityCommonDir(t, repo)
	fmt.Printf("harness: repo=%s\n", repo)
	fmt.Printf("harness: full_sha=%s abbrev=%s symbolic=HEAD\n", full, abbrev)
	fmt.Printf("harness: common_dir=%s\n", common)

	fromAbbrev, err := DeriveLineageID(repo, abbrev)
	if err != nil {
		t.Fatalf("DeriveLineageID(abbrev): %v", err)
	}
	fromSymbolic, err := DeriveLineageID(repo, "HEAD")
	if err != nil {
		t.Fatalf("DeriveLineageID(HEAD): %v", err)
	}
	fromFull, err := DeriveLineageID(repo, full)
	if err != nil {
		t.Fatalf("DeriveLineageID(full): %v", err)
	}
	want := expectedLineageIDFromFormula(t, repo, full)
	allEqual := fromAbbrev == fromSymbolic && fromSymbolic == fromFull && fromFull == want
	fmt.Printf("harness: derived(abbrev)=%s derived(symbolic)=%s derived(full)=%s all_equal=%t\n",
		fromAbbrev, fromSymbolic, fromFull, allEqual)
	if !allEqual {
		t.Errorf("derivations disagree: abbrev=%s symbolic=%s full=%s want=%s", fromAbbrev, fromSymbolic, fromFull, want)
	}

	second, err := DeriveLineageID(repo, abbrev)
	if err != nil {
		t.Fatalf("second DeriveLineageID: %v", err)
	}
	fmt.Printf("harness: second_run=%s stable=%t\n", second, second == fromAbbrev)

	foreign := t.TempDir()
	t.Chdir(foreign)
	foreignID, err := DeriveLineageID(repo, abbrev)
	if err != nil {
		t.Fatalf("foreign cwd DeriveLineageID: %v", err)
	}
	fmt.Printf("harness: foreign_cwd_id=%s stable=%t (cwd=%s)\n", foreignID, foreignID == fromAbbrev, foreign)

	worktree := filepath.Join(t.TempDir(), "harness-wt")
	runGitInDir(t, repo, "worktree", "add", worktree, "-b", "identity-harness-wt-branch")
	worktreeID, err := DeriveLineageID(worktree, abbrev)
	if err != nil {
		t.Fatalf("linked worktree DeriveLineageID: %v", err)
	}
	fmt.Printf("harness: linked_worktree_id=%s stable=%t\n", worktreeID, worktreeID == fromAbbrev)

	missing := strings.Repeat("f", 40)
	_, refusalErr := DeriveLineageID(repo, missing)
	fmt.Printf("harness refusal: raw=%s err=%v\n", missing, refusalErr)
	var refusal *LineageIdentityRefusal
	if !errors.As(refusalErr, &refusal) || refusal.Code != LineageIdentityUnresolvableCode {
		t.Errorf("unresolvable target err = %v, want *LineageIdentityRefusal with code %q", refusalErr, LineageIdentityUnresolvableCode)
	}
	storeRoot := filepath.Join(common, "biggz", "review-transactions")
	entries, statErr := os.ReadDir(storeRoot)
	count := 0
	if statErr == nil {
		count = len(entries)
	}
	fmt.Printf("harness: store_root=%s entries_after_refusal=%d (expect 0)\n", storeRoot, count)
	if count != 0 {
		t.Errorf("derivation persisted %d store entries; it must persist nothing", count)
	}
}
