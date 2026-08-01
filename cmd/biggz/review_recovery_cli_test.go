package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggz-ai/biggz/internal/review"
)

// ─── review recover / schema / inspect / quarantine-legacy (Debt D3) ─────────

func TestReviewRecover_RestoresLostHEAD(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject})
	if code != 0 {
		t.Fatalf("start: exit code = %d (stderr: %s)", code, stderr)
	}
	lineageID := startedLineageID(t, stdout)

	headPath := filepath.Join(lineageStoreDir(repoDir), lineageID, "HEAD")
	if err := os.Remove(headPath); err != nil {
		t.Fatalf("remove HEAD: %v", err)
	}

	code, stdout, stderr = runReviewCommand(t, reviewRecoverRun,
		[]string{"biggz", "review", "recover", lineageID}, "")
	if code != 0 {
		t.Fatalf("recover: exit code = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Recovered: yes") {
		t.Errorf("recover stdout = %q, want Recovered: yes", stdout)
	}
	if !strings.Contains(stdout, "Events kept:      2") {
		t.Errorf("recover stdout = %q, want 2 events kept", stdout)
	}

	// A second recover on the restored authority is a no-op.
	code, stdout, stderr = runReviewCommand(t, reviewRecoverRun,
		[]string{"biggz", "review", "recover", lineageID}, "")
	if code != 0 {
		t.Fatalf("second recover: exit code = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Recovered: no (authority intact)") {
		t.Errorf("second recover stdout = %q, want authority intact no-op", stdout)
	}
}

func TestReviewSchema_ListAndEvent(t *testing.T) {
	code, stdout, stderr := runReviewCommand(t, reviewSchemaRun, []string{"biggz", "review", "schema"}, "")
	if code != 0 {
		t.Fatalf("schema: exit code = %d (stderr: %s)", code, stderr)
	}
	for _, name := range []string{"start_review", "lens_result", "refutation", "dispose", "reopen",
		"invalidate", "withdraw", "complete_review", "receipt", "manifest"} {
		if !strings.Contains(stdout, name) {
			t.Errorf("schema list must contain %q", name)
		}
	}
	if !strings.Contains(stdout, review.LensResultEventSchema) {
		t.Errorf("schema list must carry the lens_result schema id")
	}

	code, stdout, stderr = runReviewCommand(t, reviewSchemaRun,
		[]string{"biggz", "review", "schema", "--event", "lens_result"}, "")
	if code != 0 {
		t.Fatalf("schema --event: exit code = %d (stderr: %s)", code, stderr)
	}
	for _, field := range []string{"subject_hash", "selected_order", "manifest_path"} {
		if !strings.Contains(stdout, field) {
			t.Errorf("schema --event lens_result must document %q, got: %s", field, stdout)
		}
	}

	code, _, stderr = runReviewCommand(t, reviewSchemaRun,
		[]string{"biggz", "review", "schema", "--event", "nope"}, "")
	if code != 1 || !strings.Contains(stderr, "unknown schema") {
		t.Fatalf("unknown schema: code = %d, stderr = %q, want 1 with unknown-schema error", code, stderr)
	}
}

func TestReviewInspect_JSON(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject})
	if code != 0 {
		t.Fatalf("start: exit code = %d (stderr: %s)", code, stderr)
	}
	lineageID := startedLineageID(t, stdout)

	code, stdout, stderr = runReviewCommand(t, reviewInspectRun,
		[]string{"biggz", "review", "inspect", lineageID, "--json"}, "")
	if code != 0 {
		t.Fatalf("inspect: exit code = %d (stderr: %s)", code, stderr)
	}
	var result review.InspectResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("inspect --json is not JSON: %v\n%s", err, stdout)
	}
	if result.LineageID != lineageID || result.EventCount != 2 {
		t.Fatalf("inspect = %+v, want 2 events", result)
	}
	if result.Events[0].Operation != "start_review" || result.Events[1].Operation != "in_review" {
		t.Errorf("operations = %s, %s; want start_review, in_review",
			result.Events[0].Operation, result.Events[1].Operation)
	}
	for _, event := range result.Events {
		if len(event.Revision) != 64 || event.Size <= 0 {
			t.Errorf("event summary incomplete: %+v", event)
		}
	}
}

func TestReviewQuarantineLegacy_ExplainsOutOfScope(t *testing.T) {
	code, _, stderr := runReviewCommand(t, reviewQuarantineLegacyRun,
		[]string{"biggz", "review", "quarantine-legacy"}, "")
	if code != 1 {
		t.Fatalf("quarantine-legacy: exit code = %d, want 1", code)
	}
	for _, want := range []string{"not implemented", "preserved-results", "outside the CLI"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("quarantine-legacy stderr must explain %q, got: %s", want, stderr)
		}
	}
}

// ─── review dispose-result CLI cycle ─────────────────────────────────────────

func TestReviewDisposeResult_CLICycle(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	headSHA := runGitOutput(t, repoDir, "rev-parse", "HEAD")
	subject := filepath.Join(t.TempDir(), "subject.json")
	if err := os.WriteFile(subject, []byte(fmt.Sprintf(`{"repository":%q,"commit_sha":%q}`, filepath.ToSlash(repoDir), headSHA)), 0644); err != nil {
		t.Fatalf("write subject: %v", err)
	}

	// Low-risk docs change with declared lenses: silent start.
	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lenses", "risk"})
	if code != 0 {
		t.Fatalf("start: exit code = %d (stderr: %s)", code, stderr)
	}
	lineageID := startedLineageID(t, stdout)

	capture := func(expectedRevision string) {
		t.Helper()
		code, stdout, stderr := runReviewCapture(t, []string{"biggz", "review", "capture-result",
			"--lineage", lineageID, "--target", headSHA, "--lens", "risk", "--order", "0",
			"--expected-revision", expectedRevision, "--preflight"}, "")
		if code != 0 {
			t.Fatalf("preflight: exit code = %d (stderr: %s)", code, stderr)
		}
		var preflight review.PreflightResult
		if err := json.Unmarshal([]byte(stdout), &preflight); err != nil {
			t.Fatalf("preflight JSON: %v\n%s", err, stdout)
		}
		paths := review.ManifestPaths(preflight.ChangedPathManifest)
		payload, err := json.Marshal(map[string]any{
			"subject_hash": preflight.Subject.SubjectHash,
			"inspection":   map[string]any{"status": "completed", "paths": paths},
			"lens":         "risk",
			"findings": []any{
				map[string]any{"id": "R1-001", "lens": "risk", "location": paths[0] + ":1",
					"severity": "CRITICAL", "claim": "unbounded loop",
					"evidence_class": "deterministic", "causal_disposition": "introduced"},
			},
			"evidence": []any{"go test reproduced the hang"},
		})
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		code, stdout, stderr = runReviewCapture(t, []string{"biggz", "review", "capture-result",
			"--lineage", lineageID, "--target", headSHA, "--lens", "risk", "--order", "0",
			"--expected-revision", expectedRevision, "--input", "-"}, string(payload))
		if code != 0 {
			t.Fatalf("capture: exit code = %d (stderr: %s)", code, stderr)
		}
	}

	headHash := statusHeadHash(t, lineageID)
	capture(headHash)

	// Dispose the captured slot through the CLI.
	code, stdout, stderr = runReviewCommand(t, reviewDisposeResultRun,
		[]string{"biggz", "review", "dispose-result", lineageID, "--lens", "risk", "--order", "0",
			"--reason", "scope changed"}, "")
	if code != 0 {
		t.Fatalf("dispose-result: exit code = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Lens slot disposed: risk order 0") || !strings.Contains(stdout, "scope changed") {
		t.Errorf("dispose stdout = %q, want the slot and reason", stdout)
	}

	// Finalize refuses the disposed planned slot.
	code, _, stderr = runReviewFinalize(t, []string{"biggz", "review", "finalize", lineageID})
	if code != 1 || !strings.Contains(stderr, "disposed") {
		t.Fatalf("finalize after dispose: code = %d, stderr = %q, want disposed refusal", code, stderr)
	}

	// Re-capture at the new head supersedes the disposal; finalize succeeds.
	capture(statusHeadHash(t, lineageID))
	code, stdout, stderr = runReviewFinalize(t, []string{"biggz", "review", "finalize", lineageID})
	if code != 0 {
		t.Fatalf("finalize after re-capture: exit code = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Review finalized:") {
		t.Errorf("finalize stdout = %q", stdout)
	}
}

// statusHeadHash returns the lineage head hash via `review status --json`.
func statusHeadHash(t *testing.T, lineageID string) string {
	t.Helper()
	status := statusJSON(t, lineageID)
	var headHash string
	if err := json.Unmarshal(status["head_hash"], &headHash); err != nil {
		t.Fatalf("status head_hash: %v", err)
	}
	return headHash
}
