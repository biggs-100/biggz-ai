package main

// Lineage identity CLI tests (tasks 3.1–3.3, design D1): `review start`
// canonicalizes the subject commit to a full SHA before persisting, defaults
// the lineage id to the single derived identity, keeps an explicit --lineage,
// and rejects unresolvable subjects with a typed refusal that persists nothing.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
)

// lineageIdentitySubjectFile writes a ReviewSubject fixture with an explicit
// commit_sha value (abbreviated, symbolic, or unresolvable).
func lineageIdentitySubjectFile(t *testing.T, repoDir, commitSHA string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "subject.json")
	subject := fmt.Sprintf(`{"repository":%q,"commit_sha":%q}`, filepath.ToSlash(repoDir), commitSHA)
	if err := os.WriteFile(path, []byte(subject), 0644); err != nil {
		t.Fatalf("write subject: %v", err)
	}
	return path
}

// lineageIdentityStoreEntries lists the repo's lineage store directories;
// a missing store yields no entries.
func lineageIdentityStoreEntries(t *testing.T, repoDir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(lineageStoreDir(repoDir))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read lineage store: %v", err)
	}
	return entries
}

func TestReviewStartLineageIdentityCanonicalizesAbbreviatedSubject(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	full := runGitOutput(t, repoDir, "rev-parse", "HEAD")
	abbrev := runGitOutput(t, repoDir, "rev-parse", "--short=8", "HEAD")
	subject := lineageIdentitySubjectFile(t, repoDir, abbrev)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	id := startedLineageID(t, stdout)
	want, err := review.DeriveLineageID(repoDir, full)
	if err != nil {
		t.Fatalf("DeriveLineageID: %v", err)
	}
	if id != want {
		t.Errorf("lineage id = %q, want the derived identity %q", id, want)
	}

	plan := frozenStartPlan(t, repoDir, id)
	if plan.CommitSHA != full {
		t.Errorf("persisted subject commit = %q, want full SHA %q", plan.CommitSHA, full)
	}
	if plan.CommitSHA == abbrev {
		t.Error("an abbreviated subject must never be persisted")
	}
	if entries := lineageIdentityStoreEntries(t, repoDir); len(entries) != 1 {
		t.Errorf("lineage store entries = %d, want 1", len(entries))
	}
}

func TestReviewStartLineageIdentitySymbolicHEADSubject(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	full := runGitOutput(t, repoDir, "rev-parse", "HEAD")
	subject := lineageIdentitySubjectFile(t, repoDir, "HEAD")

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	id := startedLineageID(t, stdout)
	want, err := review.DeriveLineageID(repoDir, "HEAD")
	if err != nil {
		t.Fatalf("DeriveLineageID: %v", err)
	}
	if id != want {
		t.Errorf("lineage id = %q, want the derived identity %q", id, want)
	}
	plan := frozenStartPlan(t, repoDir, id)
	if plan.CommitSHA != full {
		t.Errorf("persisted subject commit = %q, want full SHA %q (HEAD canonicalized)", plan.CommitSHA, full)
	}
}

func TestReviewStartLineageIdentityUnresolvableRejectedTyped(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	subject := lineageIdentitySubjectFile(t, repoDir, strings.Repeat("f", 40))

	code, _, stderr := runReviewStart(t, []string{"--subject", subject})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "unresolvable_subject_commit") {
		t.Errorf("stderr must carry the typed refusal code, got: %s", stderr)
	}
	if entries := lineageIdentityStoreEntries(t, repoDir); len(entries) != 0 {
		t.Errorf("rejection persisted %d lineage entries, want 0", len(entries))
	}
}

func TestReviewStartLineageIdentityExplicitLineageStillWins(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	full := runGitOutput(t, repoDir, "rev-parse", "HEAD")
	abbrev := runGitOutput(t, repoDir, "rev-parse", "--short=8", "HEAD")
	subject := lineageIdentitySubjectFile(t, repoDir, abbrev)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lineage", "explicit-lineage-id"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if id := startedLineageID(t, stdout); id != "explicit-lineage-id" {
		t.Errorf("lineage id = %q, want explicit-lineage-id", id)
	}
	plan := frozenStartPlan(t, repoDir, "explicit-lineage-id")
	if plan.CommitSHA != full {
		t.Errorf("persisted subject commit = %q, want full SHA %q", plan.CommitSHA, full)
	}
}

func TestReviewLineageIdentityRuntimeHarness(t *testing.T) {
	// Real CLI starts over real temp repositories: abbreviated and symbolic
	// subjects canonicalize to the full SHA, the derived identity is the one
	// persisted (stable across derivations), and an unresolvable subject
	// persists nothing.
	repoA := gitRepoWithCommit(t)
	chdir(t, repoA)
	fullA := runGitOutput(t, repoA, "rev-parse", "HEAD")
	abbrevA := runGitOutput(t, repoA, "rev-parse", "--short=8", "HEAD")
	subjectA := lineageIdentitySubjectFile(t, repoA, abbrevA)
	codeA, stdoutA, stderrA := runReviewStart(t, []string{"--subject", subjectA})
	if codeA != 0 {
		t.Fatalf("start(abbrev) exit = %d (stderr: %s)", codeA, stderrA)
	}
	idA := startedLineageID(t, stdoutA)
	wantA, err := review.DeriveLineageID(repoA, fullA)
	if err != nil {
		t.Fatalf("DeriveLineageID: %v", err)
	}
	planA := frozenStartPlan(t, repoA, idA)
	againA, err := review.DeriveLineageID(repoA, abbrevA)
	if err != nil {
		t.Fatalf("re-derive: %v", err)
	}
	fmt.Printf("harness: repo=%s full=%s abbrev=%s\n", repoA, fullA, abbrevA)
	fmt.Printf("harness: start(abbrev) lineage=%s derived=%s identical=%t exit=%d\n", idA, wantA, idA == wantA, codeA)
	fmt.Printf("harness: genesis_subject=%s full_sha_persisted=%t\n", planA.CommitSHA, planA.CommitSHA == fullA)
	fmt.Printf("harness: derived_again=%s stable=%t\n", againA, againA == idA)

	repoB := gitRepoWithCommit(t)
	chdir(t, repoB)
	fullB := runGitOutput(t, repoB, "rev-parse", "HEAD")
	subjectB := lineageIdentitySubjectFile(t, repoB, "HEAD")
	codeB, stdoutB, stderrB := runReviewStart(t, []string{"--subject", subjectB})
	if codeB != 0 {
		t.Fatalf("start(HEAD) exit = %d (stderr: %s)", codeB, stderrB)
	}
	idB := startedLineageID(t, stdoutB)
	wantB, err := review.DeriveLineageID(repoB, "HEAD")
	if err != nil {
		t.Fatalf("DeriveLineageID(HEAD): %v", err)
	}
	planB := frozenStartPlan(t, repoB, idB)
	fmt.Printf("harness: start(HEAD) lineage=%s derived=%s identical=%t genesis_subject=%s full_sha_persisted=%t\n",
		idB, wantB, idB == wantB, planB.CommitSHA, planB.CommitSHA == fullB)

	repoC := gitRepoWithCommit(t)
	chdir(t, repoC)
	subjectC := lineageIdentitySubjectFile(t, repoC, strings.Repeat("e", 40))
	codeC, _, stderrC := runReviewStart(t, []string{"--subject", subjectC})
	fmt.Printf("harness refusal: exit=%d stderr=%s\n", codeC, strings.TrimSpace(stderrC))
	fmt.Printf("harness: store_entries_after_refusal=%d (expect 0)\n", len(lineageIdentityStoreEntries(t, repoC)))
	if codeC != 1 {
		t.Errorf("unresolvable start exit = %d, want 1", codeC)
	}
	if entries := lineageIdentityStoreEntries(t, repoC); len(entries) != 0 {
		t.Errorf("unresolvable start persisted %d entries, want 0", len(entries))
	}
}
