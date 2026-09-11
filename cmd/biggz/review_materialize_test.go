package main

// Task 2.4: `capture-result --materialize` prints exactly the materialized
// reviewer-task bytes and captures nothing; it is mutually exclusive with
// --input and --preflight.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
)

// materializeCLISnapshot fingerprints every file below dir by relative path
// and content hash.
func materializeCLISnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		payload, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		sum := sha256.Sum256(payload)
		snapshot[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return snapshot
}

// materializeStartedLineage starts a review lineage for the current repo and
// returns its id, candidate SHA and head revision. The subject file carries
// the resolved candidate SHA (the genesis event freezes it verbatim).
func materializeStartedLineage(t *testing.T, repoDir string) (lineageID, targetSHA, headHash string) {
	t.Helper()
	targetSHA = runGitOutput(t, repoDir, "rev-parse", "HEAD")
	subject := filepath.Join(t.TempDir(), "subject.json")
	payload := fmt.Sprintf(`{"repository":%q,"commit_sha":%q}`, filepath.ToSlash(repoDir), targetSHA)
	if err := os.WriteFile(subject, []byte(payload), 0644); err != nil {
		t.Fatalf("write subject: %v", err)
	}
	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lenses", "risk", "--consent", "granted"})
	if code != 0 {
		t.Fatalf("start: exit code = %d (stderr: %s)", code, stderr)
	}
	lineageID = startedLineageID(t, stdout)
	if err := json.Unmarshal(statusJSON(t, lineageID)["head_hash"], &headHash); err != nil {
		t.Fatalf("head_hash: %v", err)
	}
	return lineageID, targetSHA, headHash
}

// materializeArgs builds the exact capture-result argv for one slot.
func materializeArgs(lineageID, targetSHA, headHash string) []string {
	return []string{"biggz", "review", "capture-result",
		"--lineage", lineageID, "--target", targetSHA, "--lens", "risk",
		"--order", "0", "--expected-revision", headHash, "--materialize"}
}

func TestReviewCaptureResultMaterializePrintsBytesAndCapturesNothing(t *testing.T) {
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	lineageID, targetSHA, headHash := materializeStartedLineage(t, repoDir)

	before := materializeCLISnapshot(t, lineageStoreDir(repoDir))
	code, first, stderr := runReviewCapture(t, materializeArgs(lineageID, targetSHA, headHash), "")
	if code != 0 {
		t.Fatalf("materialize: exit code = %d (stderr: %s)", code, stderr)
	}

	// The complete task: binding, context, name-status, numstat, per-path
	// patch delimiters.
	for _, marker := range []string{
		review.MaterializeBindingMarker, review.MaterializeContextMarker,
		review.MaterializeNameStatusMarker, review.MaterializeNumStatMarker,
		review.MaterializePatchMarker + " internal/auth/token.go",
	} {
		if !strings.Contains(first, marker) {
			t.Errorf("--materialize output is missing %q:\n%s", marker, first)
		}
	}
	// It prints the task bytes, never a capture artifact envelope.
	if strings.Contains(first, `"admission_decision"`) {
		t.Errorf("--materialize must not print a capture artifact:\n%s", first)
	}

	// Prints EXACTLY the materialized bytes.
	expected, err := review.MaterializeReviewerTask(review.CaptureBinding{
		Repo: repoDir, LineageID: lineageID, TargetIdentity: targetSHA,
		Lens: "risk", Order: 0, ExpectedRevision: headHash,
	})
	if err != nil {
		t.Fatalf("MaterializeReviewerTask: %v", err)
	}
	if first != string(expected) {
		t.Errorf("--materialize stdout does not match MaterializeReviewerTask bytes (stdout %d bytes, expected %d)", len(first), len(expected))
	}

	// Appending the same invocation again prints identical bytes.
	code, second, stderr := runReviewCapture(t, materializeArgs(lineageID, targetSHA, headHash), "")
	if code != 0 {
		t.Fatalf("second materialize: exit code = %d (stderr: %s)", code, stderr)
	}
	if second != first {
		t.Error("--materialize is not byte-identical across two runs")
	}

	// Captures nothing: chain events and receipts are unchanged.
	after := materializeCLISnapshot(t, lineageStoreDir(repoDir))
	if !maps.Equal(before, after) {
		t.Errorf("--materialize mutated the lineage store:\nbefore=%v\nafter=%v", before, after)
	}
}

func TestReviewCaptureResultMaterializeExclusiveFlags(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	base := []string{"biggz", "review", "capture-result",
		"--lineage", "some-lineage", "--target", strings.Repeat("a", 40),
		"--lens", "risk", "--order", "0", "--expected-revision", strings.Repeat("b", 64),
		"--materialize"}

	code, _, stderr := runReviewCapture(t, append(append([]string{}, base...), "--preflight"), "")
	if code != 1 {
		t.Fatalf("--materialize --preflight: exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Errorf("--materialize --preflight stderr should name the conflict, got: %s", stderr)
	}

	code, _, stderr = runReviewCapture(t, append(append([]string{}, base...), "--input", "-"), "")
	if code != 1 {
		t.Fatalf("--materialize --input: exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Errorf("--materialize --input stderr should name the conflict, got: %s", stderr)
	}
}
