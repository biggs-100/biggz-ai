package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// next_transition derivation
// ---------------------------------------------------------------------------

func TestNextTransition_CollectFirstMissingLensInOrder(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "nt-collect", []string{"risk", "readability", "reliability"}, "")

	captureLens(t, repo, "nt-collect", head, "risk", 0)
	captureLens(t, repo, "nt-collect", head, "reliability", 2)

	auth := NewAuthority(repo)
	st, err := auth.Status("nt-collect")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	nt := st.NextTransition
	if nt == nil {
		t.Fatal("next_transition missing")
	}
	// Declared lenses are canonical (sorted): [readability, reliability,
	// risk]. The first missing slot in that order is readability at index 0.
	if nt.Action != "collect" || nt.Lens != "readability" || nt.Order == nil || *nt.Order != 0 {
		t.Fatalf("next_transition = %+v, want collect readability order 0 (first missing in canonical order)", nt)
	}
}

func TestNextTransition_FinalizeWhenAllCaptured(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "nt-finalize", []string{"risk", "readability"}, "")

	captureLens(t, repo, "nt-finalize", head, "risk", 0)
	captureLens(t, repo, "nt-finalize", head, "readability", 1)

	st, err := NewAuthority(repo).Status("nt-finalize")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.NextTransition == nil || st.NextTransition.Action != "finalize" {
		t.Fatalf("next_transition = %+v, want finalize", st.NextTransition)
	}
}

// cleanResultJSON renders a reviewer payload WITHOUT findings (clean review).
func cleanResultJSON(t *testing.T, binding CaptureBinding, paths []string, subjectHash string) []byte {
	t.Helper()
	envelope := map[string]any{
		"subject_hash": subjectHash,
		"inspection":   map[string]any{"status": "completed", "paths": paths},
		"lens":         binding.Lens,
		"findings":     []any{},
		"evidence":     []any{"clean sweep"},
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return payload
}

// captureLensClean captures a clean (no-findings) reviewer result.
func captureLensClean(t *testing.T, repo, lineageID, commitSHA string, lens string, order int) {
	t.Helper()
	store, err := Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	binding := CaptureBinding{
		Repo: repo, LineageID: lineageID, TargetIdentity: commitSHA,
		Lens: lens, Order: order, ExpectedRevision: chain.HeadHash,
	}
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)
	payload := cleanResultJSON(t, binding, paths, preflight.Subject.SubjectHash)
	outcome, err := Capture(binding, payload)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if outcome.Artifact.AdmissionDecision != AdmissionCompleted {
		t.Fatalf("decision = %q, want completed", outcome.Artifact.AdmissionDecision)
	}
}

func TestNextTransition_GateWhenFinalizedClean(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "nt-gate", []string{"risk", "readability"}, "")

	captureLensClean(t, repo, "nt-gate", head, "risk", 0)
	captureLensClean(t, repo, "nt-gate", head, "readability", 1)
	if _, err := Finalize(repo, "nt-gate"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	st, err := NewAuthority(repo).Status("nt-gate")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	nt := st.NextTransition
	if nt == nil {
		t.Fatal("next_transition missing")
	}
	if nt.Action != "gate" {
		t.Fatalf("next_transition = %+v, want gate", nt)
	}
	want := []string{"post-apply", "pre-commit", "pre-push", "pre-pr", "release"}
	if strings.Join(nt.Gates, ",") != strings.Join(want, ",") {
		t.Fatalf("gates = %v, want %v", nt.Gates, want)
	}
}

func TestNextTransition_CorrectionWhenBlockingFindings(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "nt-correction", []string{"risk"}, "")

	// Capture with a deterministic blocking finding and finalize: the receipt
	// does NOT resolve it (deterministic findings are auto-blocking), so a
	// resume after finalize followed by a NEW capture leaves every
	// candidate-causal finding unresolved: the lineage must route to
	// correction, not gate.
	captureLens(t, repo, "nt-correction", head, "risk", 0)
	if _, err := Finalize(repo, "nt-correction"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	// A resume after finalize followed by a NEW capture introduces a
	// candidate-causal finding the existing receipt does NOT resolve: the
	// lineage must route to correction, not gate.
	if err := resumeLineage(t, repo, "nt-correction"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	captureLens(t, repo, "nt-correction", head, "readability", 1)

	st, err := NewAuthority(repo).Status("nt-correction")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	nt := st.NextTransition
	if nt == nil {
		t.Fatal("next_transition missing")
	}
	if nt.Action != "correction" {
		t.Fatalf("next_transition = %+v, want correction", nt)
	}
	// 5 original changed lines → budget min(200, ceil(5/2)) = 3.
	if nt.BudgetRemaining != 3 {
		t.Fatalf("budget_remaining = %d, want 3", nt.BudgetRemaining)
	}
}

// resumeLineage appends a resume event (admin) at the current chain head.
func resumeLineage(t *testing.T, repo, lineageID string) error {
	t.Helper()
	auth := NewAuthority(repo)
	chain, err := auth.LoadChain(lineageID)
	if err != nil {
		return err
	}
	store, err := auth.Open(lineageID)
	if err != nil {
		return err
	}
	_, err = store.Append(chain.HeadHash, Record{
		Operation: "resume",
		Role:      "Admin",
		Actor:     "Admin",
		Timestamp: time.Now().Format(time.RFC3339Nano),
	})
	return err
}

func TestNextTransition_ChainInvalidStops(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "nt-invalid", nil, "")

	// Tamper an event file with content that still parses (a trailing byte
	// changes the content address without breaking the JSON).
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	genesis := chain.GenesisHash
	path := filepath.Join(store.Dir, "v1", "events", genesis)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = filepath.Join(store.Dir, genesis)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read genesis: %v", err)
	}
	if err := os.WriteFile(path, append(data, ' '), 0644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	st, err := NewAuthority(repo).Status("nt-invalid")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	nt := st.NextTransition
	if nt == nil || nt.Action != "stop" || nt.Reason != "chain_invalid" {
		t.Fatalf("next_transition = %+v, want stop/chain_invalid", nt)
	}
}

func TestNextTransition_TerminalStateStops(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "nt-terminated", []string{"risk"}, "")
	if _, err := Invalidate(repo, "nt-terminated", "policy violation"); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}

	st, err := NewAuthority(repo).Status("nt-terminated")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	nt := st.NextTransition
	if nt == nil || nt.Action != "stop" || nt.Reason != "invalidated" {
		t.Fatalf("next_transition = %+v, want stop/invalidated", nt)
	}
}

func TestNextTransition_RDDDisabledStops(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "nt-rdd", []string{"risk"}, "")

	gitDir, err := gitIn(repo, "rev-parse", "--git-dir")
	if err != nil {
		t.Fatalf("git dir: %v", err)
	}
	gitDir = filepath.Join(repo, gitDir) // gitIn returns a relative path
	if _, err := RDDDisable(gitDir, gitDir, "clone"); err != nil {
		t.Fatalf("RDDDisable: %v", err)
	}

	st, err := NewAuthority(repo).Status("nt-rdd")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	nt := st.NextTransition
	if nt == nil || nt.Action != "stop" || nt.Reason != "rdd_disabled" {
		t.Fatalf("next_transition = %+v, want stop/rdd_disabled", nt)
	}
}

func TestNextTransition_EmptyLineageHasNoTransition(t *testing.T) {
	// A repo with no review started: status must carry no next_transition.
	repo := t.TempDir()
	gitInit(t, repo)
	st, err := NewAuthority(repo).Status("nt-empty")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.NextTransition != nil {
		t.Fatalf("next_transition = %+v, want nil for an empty lineage", st.NextTransition)
	}
}
