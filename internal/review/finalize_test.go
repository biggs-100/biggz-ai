package review

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// finalizeFixtureRepo builds a git repo whose head commit adds a.txt (3 lines)
// and b.txt (2 lines) on top of a base commit: 5 original changed lines →
// correction budget min(200, ceil(5/2)) = 3.
func finalizeFixtureRepo(t *testing.T) (string, string, string) {
	t.Helper()
	repo := t.TempDir()
	gitInit(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatalf("write base.txt: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "base")
	baseCommit := runGitInDir(t, repo, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("x\ny\n"), 0644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "candidate")
	head := runGitInDir(t, repo, "rev-parse", "HEAD")
	return repo, baseCommit, head
}

// finalizeStart starts a lineage with a derived start plan and returns the
// store and the frozen plan.
func finalizeStart(t *testing.T, repo, commitSHA, lineageID string, lenses []string, baseRef string) (*Store, StartEventPayload) {
	t.Helper()
	store, err := Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	base, lines, err := DeriveOriginalChangedLines(repo, commitSHA, baseRef)
	if err != nil {
		t.Fatalf("DeriveOriginalChangedLines: %v", err)
	}
	budget, err := DeriveCorrectionBudget(lines)
	if err != nil {
		t.Fatalf("DeriveCorrectionBudget: %v", err)
	}
	plan := StartEventPayload{
		Schema: ReviewStartEventSchema, Repository: repo, CommitSHA: commitSHA,
		BaseRef: base, OriginalChangedLines: lines, CorrectionBudget: budget,
		MaxCorrectionAttempts: MaxCompactCorrectionAttempts, SelectedLenses: lenses,
	}
	review := New(model.ReviewSubject{Repository: repo, CommitSHA: commitSHA})
	review.State.Role = model.RoleReviewer
	review.WithStore(store).FreezeStartPlan(plan)
	if err := review.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return store, plan
}

// captureLens captures one reviewer result for the given lens/order at the
// current chain head and returns the new binding context.
func captureLens(t *testing.T, repo, lineageID, commitSHA string, lens string, order int) {
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
	payload := captureResultJSON(t, binding, paths, preflight.Subject.SubjectHash)
	outcome, err := Capture(binding, payload)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if outcome.Artifact.AdmissionDecision != AdmissionCompleted {
		t.Fatalf("decision = %q, want completed", outcome.Artifact.AdmissionDecision)
	}
}

// ---------------------------------------------------------------------------
// Finalize: missing lens slots
// ---------------------------------------------------------------------------

func TestFinalize_RejectsMissingDeclaredLensSlots(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "finalize-missing", []string{"risk", "readability"}, "")

	captureLens(t, repo, "finalize-missing", head, "risk", 0)

	_, err := Finalize(repo, "finalize-missing")
	if err == nil {
		t.Fatal("expected missing-slot rejection")
	}
	if !strings.Contains(err.Error(), "readability") || !strings.Contains(err.Error(), "missing") {
		t.Errorf("want missing captured lens slot error naming readability, got: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Count != 3 {
		t.Errorf("event count = %d, want 3 (rejection must not append)", chain.Count)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, ReceiptsDirName)); !os.IsNotExist(err) {
		t.Error("finalize rejection must not create the receipts directory")
	}
}

func TestFinalize_RejectsZeroCapturedLenses(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "finalize-zero", nil, "")

	_, err := Finalize(repo, "finalize-zero")
	if err == nil {
		t.Fatal("expected rejection for zero captured lenses")
	}
	if !strings.Contains(err.Error(), "no captured lens slots") {
		t.Errorf("want no-captured-slots error, got: %v", err)
	}
}

func TestFinalize_RejectsExtraLensOutsideSelection(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "finalize-extra", []string{"risk"}, "")

	captureLens(t, repo, "finalize-extra", head, "risk", 0)
	captureLens(t, repo, "finalize-extra", head, "readability", 1)

	_, err := Finalize(repo, "finalize-extra")
	if err == nil {
		t.Fatal("expected rejection for a lens outside the frozen selection")
	}
	if !strings.Contains(err.Error(), "outside the frozen selection") {
		t.Errorf("want outside-selection error, got: %v", err)
	}
	if chain, _ := store.LoadChain(); chain.Count != 4 {
		t.Errorf("event count = %d, want 4 (rejection must not append)", chain.Count)
	}
}

// ---------------------------------------------------------------------------
// Finalize: happy path + persisted receipt
// ---------------------------------------------------------------------------

func TestFinalize_HappyPathPersistsReceipt(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "finalize-happy", []string{"risk", "readability"}, "")

	captureLens(t, repo, "finalize-happy", head, "risk", 0)
	captureLens(t, repo, "finalize-happy", head, "readability", 1)

	before, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	outcome, err := Finalize(repo, "finalize-happy")
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if outcome.Idempotent {
		t.Error("first finalize must not be idempotent")
	}
	if outcome.ReceiptPath == "" || outcome.ReceiptHash == "" || outcome.Revision == "" {
		t.Fatalf("outcome is incomplete: %+v", outcome)
	}

	// Receipt is ephemeral: after burn the file is deleted, but the outcome
	// still carries the path/hash and the burned marker exists.
	abs := filepath.Join(store.Dir, outcome.ReceiptPath)
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Errorf("receipt file %q should be deleted after burn (ephemeral), stat err: %v", abs, err)
	}
	marker := filepath.Join(store.Dir, BurnedMarkerFile)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("burned marker %q should exist after finalize: %v", marker, err)
	}
	if !store.IsBurned() {
		t.Error("store should be burned after finalize")
	}
	// Verify the burned receipt via the outcome hash binding (file is gone,
	// so we validate the hash binding indirectly via the complete_review event).
	// Re-derive receipt fields to ensure the outcome hash is well-formed.
	if !validSHA256Identity(outcome.ReceiptHash) {
		t.Errorf("outcome receipt hash invalid: %q", outcome.ReceiptHash)
	}

	// The complete_review + burn_review events must be appended.
	after, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if after.Count != before.Count+2 {
		t.Errorf("event count = %d, want %d (complete_review + burn_review)", after.Count, before.Count+2)
	}
	if !after.Valid {
		t.Error("chain must stay valid after finalize")
	}
	if len(after.Records) < 2 {
		t.Fatalf("chain too short after finalize: %d", after.Count)
	}
	completeRec := after.Records[after.Count-2]
	if completeRec.Operation != CompleteReviewOperation {
		t.Errorf("second-last operation = %q, want complete_review", completeRec.Operation)
	}
	var evt completeEventPayload
	if err := json.Unmarshal(completeRec.Payload, &evt); err != nil {
		t.Fatalf("parse complete event: %v", err)
	}
	if evt.ReceiptPath != outcome.ReceiptPath || evt.ReceiptHash != outcome.ReceiptHash {
		t.Error("complete_review event does not reference the burned receipt")
	}
	burnRec := after.Records[after.Count-1]
	if burnRec.Operation != BurnOperation {
		t.Errorf("last operation = %q, want burn_review", burnRec.Operation)
	}
	var burnEvt burnEventPayload
	if err := json.Unmarshal(burnRec.Payload, &burnEvt); err != nil {
		t.Fatalf("parse burn event: %v", err)
	}
	if burnEvt.ReceiptHash != outcome.ReceiptHash {
		t.Errorf("burn event hash %q != outcome hash %q", burnEvt.ReceiptHash, outcome.ReceiptHash)
	}
	if verdict := store.Validate(); !verdict.Valid {
		t.Errorf("store integrity after finalize: %s", verdict.Reason)
	}
}

func TestFinalize_RejectsAlreadyCompletedWithoutReceipt(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "finalize-legacy", []string{"risk"}, "")
	captureLens(t, repo, "finalize-legacy", head, "risk", 0)

	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	// Simulate a legacy completion that never materialized a receipt.
	legacyPayload, _ := json.Marshal(map[string]string{"merkle_root": "deadbeef"})
	if _, err := store.Append(chain.HeadHash, Record{
		Operation: CompleteReviewOperation,
		Role:      string(model.RoleLead),
		Actor:     string(model.RoleLead),
		Timestamp: "2026-01-01T00:00:00Z",
		Payload:   legacyPayload,
	}); err != nil {
		t.Fatalf("append legacy complete: %v", err)
	}

	_, err = Finalize(repo, "finalize-legacy")
	if err == nil {
		t.Fatal("expected rejection for completed lineage without a receipt artifact")
	}
	if !strings.Contains(err.Error(), "no persisted receipt artifact") {
		t.Errorf("want no-persisted-receipt error, got: %v", err)
	}
}

func TestFinalize_RejectsChainIntegrityFailure(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "finalize-tampered", []string{"risk"}, "")
	captureLens(t, repo, "finalize-tampered", head, "risk", 0)

	// Tamper with an event file; the content hash no longer matches the name.
	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	tampered := false
	for _, entry := range entries {
		if entry.IsDir() || len(entry.Name()) != 64 {
			continue
		}
		path := filepath.Join(store.Dir, entry.Name())
		original, _ := os.ReadFile(path)
		if err := os.WriteFile(path, append(original, []byte("tamper")...), 0644); err != nil {
			t.Fatalf("tamper: %v", err)
		}
		tampered = true
		break
	}
	if !tampered {
		t.Fatal("no event file found to tamper")
	}

	_, err = Finalize(repo, "finalize-tampered")
	if err == nil {
		t.Fatal("expected rejection for tampered chain")
	}
	if !strings.Contains(err.Error(), "chain integrity failed") && !strings.Contains(err.Error(), "load chain") {
		t.Errorf("want chain-integrity error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Finalize: idempotency
// ---------------------------------------------------------------------------

func TestFinalize_IdempotentSecondFinalize(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "finalize-idem", []string{"risk"}, "")

	captureLens(t, repo, "finalize-idem", head, "risk", 0)

	first, err := Finalize(repo, "finalize-idem")
	if err != nil {
		t.Fatalf("first Finalize: %v", err)
	}
	countAfterFirst, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	_ = first

	second, err := Finalize(repo, "finalize-idem")
	if err == nil {
		t.Fatalf("second Finalize should fail with burned error, got outcome %+v", second)
	}
	if !errors.Is(err, ErrAlreadyBurned) && !strings.Contains(strings.ToLower(err.Error()), "burned") {
		t.Fatalf("second finalize error should be ErrAlreadyBurned, got: %v", err)
	}
	countAfterSecond, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if countAfterSecond.Count != countAfterFirst.Count {
		t.Errorf("event count changed across burned second finalize: %d → %d", countAfterFirst.Count, countAfterSecond.Count)
	}
	if countAfterSecond.HeadHash != countAfterFirst.HeadHash {
		t.Error("head changed across burned second finalize")
	}
}

// ---------------------------------------------------------------------------
// Budget derivation + freeze at start
// ---------------------------------------------------------------------------

func TestBudget_DerivationAndFreezeAtStart(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, plan := finalizeStart(t, repo, head, "budget-freeze", nil, "")

	// 5 changed lines (a.txt +3, b.txt +2) → min(200, ceil(5/2)) = 3.
	if plan.OriginalChangedLines != 5 {
		t.Errorf("original changed lines = %d, want 5", plan.OriginalChangedLines)
	}
	if plan.CorrectionBudget != 3 {
		t.Errorf("correction budget = %d, want 3", plan.CorrectionBudget)
	}
	if plan.MaxCorrectionAttempts != MaxCompactCorrectionAttempts {
		t.Errorf("max attempts = %d, want %d", plan.MaxCorrectionAttempts, MaxCompactCorrectionAttempts)
	}

	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	var frozen StartEventPayload
	if err := json.Unmarshal(chain.Records[0].Payload, &frozen); err != nil {
		t.Fatalf("parse genesis: %v", err)
	}
	if frozen.CorrectionBudget != 3 || frozen.OriginalChangedLines != 5 || frozen.MaxCorrectionAttempts != 1 {
		t.Errorf("genesis payload does not carry the frozen budget: %+v", frozen)
	}
	if frozen.BaseRef == "" || !validCommitSHA(frozen.BaseRef) {
		t.Errorf("genesis base ref = %q, want the resolved base tree", frozen.BaseRef)
	}

	auth := NewAuthority(repo)
	st, err := auth.Status("budget-freeze")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Budget == nil {
		t.Fatal("status must surface the frozen budget")
	}
	if st.Budget.CorrectionLines != 3 || st.Budget.MaxAttempts != 1 || st.Budget.OriginalChangedLines != 5 {
		t.Errorf("status budget = %+v, want correction 3 / attempts 1 / original 5", st.Budget)
	}
	if st.ReceiptArtifact != nil {
		t.Error("no receipt artifact before finalize")
	}
}

func TestBudget_DerivationCap(t *testing.T) {
	budget, err := DeriveCorrectionBudget(10)
	if err != nil || budget != 5 {
		t.Errorf("budget(10) = %d, %v; want 5", budget, err)
	}
	budget, err = DeriveCorrectionBudget(401)
	if err != nil || budget != 200 {
		t.Errorf("budget(401) = %d, %v; want capped 200", budget, err)
	}
	budget, err = DeriveCorrectionBudget(1)
	if err != nil || budget != 2 {
		t.Errorf("budget(1) = %d, %v; want 2 (floor_two)", budget, err)
	}
	if _, err := DeriveCorrectionBudget(-1); err == nil {
		t.Error("expected rejection for negative changed lines")
	}
}

func TestBudget_FloorTwo(t *testing.T) {
	cases := []struct {
		lines int
		want  int
	}{
		{0, 2},
		{1, 2},
		{2, 2},
		{3, 2},
		{4, 2},
		{5, 3},
		{6, 3},
		{400, 200},
		{401, 200},
	}
	for _, tt := range cases {
		tt := tt
		t.Run("", func(t *testing.T) {
			budget, err := DeriveCorrectionBudget(tt.lines)
			if err != nil {
				t.Fatalf("DeriveCorrectionBudget(%d): %v", tt.lines, err)
			}
			if budget != tt.want {
				t.Errorf("DeriveCorrectionBudget(%d) = %d, want %d", tt.lines, budget, tt.want)
			}
		})
	}
}

func TestBudget_ExplicitBaseRef(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatalf("write base.txt: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "base")
	commit1 := runGitInDir(t, repo, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("x\ny\n"), 0644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "candidate")
	runGitInDir(t, repo, "rev-parse", "HEAD") // commit2
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\nmore\n"), 0644); err != nil {
		t.Fatalf("write base.txt: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "amend")
	commit3 := runGitInDir(t, repo, "rev-parse", "HEAD")

	// Default derivation uses the parent (commit2): only base.txt changed (+1).
	_, lines, err := DeriveOriginalChangedLines(repo, commit3, "")
	if err != nil {
		t.Fatalf("default derivation: %v", err)
	}
	if lines != 1 {
		t.Errorf("default lines = %d, want 1 (base.txt +1)", lines)
	}
	// Explicit --base-ref commit1 widens the diff: base.txt +1, a.txt +3, b.txt +2.
	base, lines, err := DeriveOriginalChangedLines(repo, commit3, commit1)
	if err != nil {
		t.Fatalf("explicit base derivation: %v", err)
	}
	if base != runGitInDir(t, repo, "rev-parse", commit1+"^{tree}") {
		t.Errorf("base = %q, want commit1 tree", base)
	}
	if lines != 6 {
		t.Errorf("explicit base lines = %d, want 6", lines)
	}
}

// ---------------------------------------------------------------------------
// Forecast + cumulative enforcement
// ---------------------------------------------------------------------------

func TestResumeForecastGate_WithinBudget(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "gate-ok", nil, "")
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if err := ResumeForecastGate(chain, 3); err != nil {
		t.Fatalf("forecast 3 <= budget 3 must pass, got: %v", err)
	}
	if err := ResumeForecastGate(chain, 1); err != nil {
		t.Fatalf("forecast 1 <= budget 3 must pass, got: %v", err)
	}
}

func TestResumeForecastGate_OverBudgetRejected(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "gate-over", nil, "")
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	err = ResumeForecastGate(chain, 4)
	if err == nil {
		t.Fatal("expected rejection for forecast 4 > budget 3")
	}
	if !strings.Contains(err.Error(), "3") || !strings.Contains(err.Error(), "budget") {
		t.Errorf("error must name the budget, got: %v", err)
	}
	if err := ResumeForecastGate(chain, 0); err == nil {
		t.Error("expected rejection for non-positive forecast")
	}
}

func TestResumeForecastGate_NoFrozenBudget(t *testing.T) {
	// A legacy start (bare subject payload, no plan) has no frozen budget.
	repo, _, head := finalizeFixtureRepo(t)
	store, err := Open(repo, "gate-legacy")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	review := New(model.ReviewSubject{Repository: repo, CommitSHA: head})
	review.State.Role = model.RoleReviewer
	review.WithStore(store)
	if err := review.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if err := ResumeForecastGate(chain, 1); err == nil {
		t.Fatal("expected rejection for lineage without a frozen budget")
	} else if !strings.Contains(err.Error(), "no frozen correction budget") {
		t.Errorf("want no-frozen-budget error, got: %v", err)
	}
}

func TestCorrectionAccounting_Unit(t *testing.T) {
	if MaxCompactCorrectionAttempts != 1 {
		t.Errorf("MaxCompactCorrectionAttempts = %d, want 1", MaxCompactCorrectionAttempts)
	}
	if CorrectionAttemptConsumed(0) || !CorrectionAttemptConsumed(1) || !CorrectionAttemptConsumed(2) {
		t.Error("CorrectionAttemptConsumed must exhaust at the single attempt")
	}
	if err := ValidateCorrectionActual(2, 1, 3); err != nil {
		t.Fatalf("cumulative 1 + actual 2 <= budget 3 must pass, got: %v", err)
	}
	err := ValidateCorrectionActual(2, 2, 3)
	if err == nil {
		t.Fatal("expected rejection for cumulative 2 + actual 2 > budget 3")
	}
	if !strings.Contains(err.Error(), "3") || !strings.Contains(err.Error(), "budget") {
		t.Errorf("error must name the budget, got: %v", err)
	}
	if err := ValidateCorrectionActual(-1, 0, 3); err == nil {
		t.Error("expected rejection for negative actual lines")
	}
}

// ---------------------------------------------------------------------------
// Status surfaces the persisted receipt after finalize
// ---------------------------------------------------------------------------

func TestStatus_SurfacesPersistedReceiptAfterFinalize(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "status-receipt", []string{"risk"}, "")
	captureLens(t, repo, "status-receipt", head, "risk", 0)

	outcome, err := Finalize(repo, "status-receipt")
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	auth := NewAuthority(repo)
	st, err := auth.Status("status-receipt")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.ReceiptArtifact == nil {
		t.Fatal("status must surface the persisted receipt artifact")
	}
	if st.ReceiptArtifact.Path != outcome.ReceiptPath || st.ReceiptArtifact.Hash != outcome.ReceiptHash {
		t.Errorf("status receipt artifact = %+v, want %+v", st.ReceiptArtifact, outcome)
	}
	// Receipt is ephemeral: file is deleted after burn, but the artifact
	// reference remains in the complete_review event. The burned marker proves
	// the receipt was created and then burned.
	if _, err := os.Stat(filepath.Join(store.Dir, st.ReceiptArtifact.Path)); !os.IsNotExist(err) {
		t.Errorf("receipt file %q should be deleted after burn (ephemeral), got err: %v", st.ReceiptArtifact.Path, err)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, BurnedMarkerFile)); err != nil {
		t.Errorf("burned marker should exist after finalize: %v", err)
	}
}

func TestCanStopSession_Allowed(t *testing.T) {
	if !CanStopSession(SessionStopState{PendingFindings: 0, PendingLenses: 0}) {
		t.Error("empty state should allow stop")
	}
}

func TestCanStopSession_BlockedIdempotent(t *testing.T) {
	s := SessionStopState{PendingFindings: 2, PendingLenses: 1}
	first := CanStopSession(s)
	second := CanStopSession(s)
	if first || second {
		t.Fatalf("blocked state must return false both times, got %v %v", first, second)
	}
	if s.PendingFindings != 2 || s.PendingLenses != 1 {
		t.Error("CanStopSession must not mutate state")
	}
}

func TestCanStopSession_PartialPending(t *testing.T) {
	if CanStopSession(SessionStopState{PendingFindings: 1, PendingLenses: 0}) {
		t.Error("pending findings must block")
	}
	if CanStopSession(SessionStopState{PendingFindings: 0, PendingLenses: 1}) {
		t.Error("pending lenses must block")
	}
}

func TestFixDeltaBinding(t *testing.T) {
	if got := FixDeltaHashForSnapshot("a", "b", "c", 0, nil); got != EmptyFixDeltaHash {
		t.Errorf("cumulative 0 must return EmptyFixDeltaHash, got %q", got)
	}
	if got := FixDeltaHashForSnapshot("a", "b", "c", -1, nil); got != EmptyFixDeltaHash {
		t.Errorf("negative must return EmptyFixDeltaHash, got %q", got)
	}
	hash := FixDeltaHashForSnapshot("baseTreeAAAA", "candidateBBBB", "pathsDigestCCCC", 2, nil)
	if hash == EmptyFixDeltaHash {
		t.Error("cumulative 2 must not be Empty")
	}
	flat := payloadSHA256([]byte("fix-delta:2"))
	if hash == flat {
		t.Errorf("FixDelta hash must differ from flat payloadSHA256, both %q", hash)
	}
	if len(hash) < 7 || hash[:7] != "sha256:" {
		t.Errorf("FixDelta hash must be sha256: prefix, got %q", hash)
	}
	// Determinism
	hash2 := FixDeltaHashForSnapshot("baseTreeAAAA", "candidateBBBB", "pathsDigestCCCC", 2, nil)
	if hash != hash2 {
		t.Error("FixDelta hash must be deterministic")
	}
	// Different trees produce different hash
	hashDiff := FixDeltaHashForSnapshot("otherBase", "candidateBBBB", "pathsDigestCCCC", 2, nil)
	if hash == hashDiff {
		t.Error("different baseTree must produce different hash")
	}
	// LedgerIDs affect hash
	hashLedger := FixDeltaHashForSnapshot("baseTreeAAAA", "candidateBBBB", "pathsDigestCCCC", 2, []string{"ledger1"})
	if hash == hashLedger {
		t.Error("ledgerIDs must affect hash")
	}
	// Validate rejects flat delta when receipt expects domain-bound hash:
	// Build a receipt with flat hash and ensure it would not match recomputed domain hash
	recomputed := FixDeltaHashForSnapshot("baseTreeAAAA", "candidateBBBB", "pathsDigestCCCC", 2, nil)
	if flat == recomputed {
		t.Error("flat must not equal recomputed domain hash")
	}
}
