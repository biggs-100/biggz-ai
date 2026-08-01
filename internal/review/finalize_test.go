package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/biggz-ai/biggz/model"
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

	// The receipt file must exist under receipts/ and be content-addressed.
	abs := filepath.Join(store.Dir, outcome.ReceiptPath)
	payload, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("receipt file missing: %v", err)
	}
	name := filepath.Base(outcome.ReceiptPath)
	if sha256Hex(payload) != strings.TrimSuffix(name, ".json") {
		t.Error("receipt file name does not match its content hash")
	}

	// The receipt must parse, validate, and carry the full binding fields.
	var receipt PersistedReceipt
	if err := json.Unmarshal(payload, &receipt); err != nil {
		t.Fatalf("parse receipt: %v", err)
	}
	if err := receipt.Validate(); err != nil {
		t.Fatalf("Validate receipt: %v", err)
	}
	if receipt.Schema != ReviewReceiptSchema || receipt.LineageID != "finalize-happy" {
		t.Errorf("receipt identity mismatch: %+v", receipt)
	}
	if receipt.Generation != 1 {
		t.Errorf("generation = %d, want 1", receipt.Generation)
	}
	if receipt.GenesisRevision != before.GenesisHash {
		t.Errorf("genesis revision %s != chain genesis %s", receipt.GenesisRevision, before.GenesisHash)
	}
	if receipt.HeadRevision != before.HeadHash {
		t.Errorf("head revision %s != pre-finalize head %s", receipt.HeadRevision, before.HeadHash)
	}
	if receipt.BaseTree == "" || receipt.InitialReviewTree == "" || receipt.FinalCandidateTree == "" {
		t.Error("receipt trees are empty")
	}
	if receipt.InitialReviewTree != receipt.FinalCandidateTree {
		t.Error("initial and final candidate trees must be equal before correction")
	}
	if receipt.PathsDigest == "" || receipt.FixDeltaHash != EmptyFixDeltaHash || receipt.EvidenceHash == "" {
		t.Error("receipt digest/hash fields are incomplete")
	}
	if receipt.RiskTier != "medium" {
		t.Errorf("risk tier = %q, want medium (two lenses)", receipt.RiskTier)
	}
	if !reflect.DeepEqual(receipt.SelectedLenses, []string{"readability", "risk"}) {
		t.Errorf("selected lenses = %v, want [readability risk]", receipt.SelectedLenses)
	}
	if len(receipt.LensSubjects) != 2 {
		t.Fatalf("lens subjects = %d, want 2", len(receipt.LensSubjects))
	}
	for _, subject := range receipt.LensSubjects {
		if !validSHA256Identity(subject.SubjectHash) || !validSHA256Identity(subject.ResultHash) {
			t.Errorf("lens subject hashes invalid: %+v", subject)
		}
	}
	if !reflect.DeepEqual(receipt.ResolvedFindingIDs, []string{}) {
		t.Errorf("resolved finding IDs = %v, want [] (deterministic findings are auto-blocking and never resolved by the receipt)", receipt.ResolvedFindingIDs)
	}
	if !reflect.DeepEqual(receipt.StandingFindingIDs, []string{}) {
		t.Errorf("standing finding IDs = %v, want [] (no refuter batch was registered)", receipt.StandingFindingIDs)
	}
	if receipt.TerminalState != ReviewReceiptTerminalState {
		t.Errorf("terminal state = %q", receipt.TerminalState)
	}
	if !validSHA256Identity(receipt.ReceiptHash) || receipt.ReceiptHash != receipt.computeHash() {
		t.Error("receipt hash does not bind the receipt")
	}
	if receipt.ReceiptHash != outcome.ReceiptHash {
		t.Error("outcome hash does not match the persisted receipt hash")
	}

	// The complete_review event must be appended and reference the receipt.
	after, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if after.Count != before.Count+1 {
		t.Errorf("event count = %d, want %d", after.Count, before.Count+1)
	}
	if after.HeadHash != outcome.Revision {
		t.Errorf("head %s != outcome revision %s", after.HeadHash, outcome.Revision)
	}
	if !after.Valid {
		t.Error("chain must stay valid after finalize")
	}
	last := after.Records[after.Count-1]
	if last.Operation != CompleteReviewOperation {
		t.Errorf("last operation = %q, want complete_review", last.Operation)
	}
	var evt completeEventPayload
	if err := json.Unmarshal(last.Payload, &evt); err != nil {
		t.Fatalf("parse complete event: %v", err)
	}
	if evt.ReceiptPath != outcome.ReceiptPath || evt.ReceiptHash != outcome.ReceiptHash {
		t.Error("complete_review event does not reference the persisted receipt")
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

	second, err := Finalize(repo, "finalize-idem")
	if err != nil {
		t.Fatalf("second Finalize: %v", err)
	}
	if !second.Idempotent {
		t.Error("second finalize must be idempotent")
	}
	if second.ReceiptPath != first.ReceiptPath || second.ReceiptHash != first.ReceiptHash {
		t.Errorf("second finalize returned a different receipt: %+v vs %+v", second, first)
	}
	countAfterSecond, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if countAfterSecond.Count != countAfterFirst.Count {
		t.Errorf("event count changed across idempotent finalize: %d → %d", countAfterFirst.Count, countAfterSecond.Count)
	}
	if countAfterSecond.HeadHash != countAfterFirst.HeadHash {
		t.Error("head changed across idempotent finalize")
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
	if err != nil || budget != 1 {
		t.Errorf("budget(1) = %d, %v; want 1", budget, err)
	}
	if _, err := DeriveCorrectionBudget(-1); err == nil {
		t.Error("expected rejection for negative changed lines")
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
	if _, err := os.Stat(filepath.Join(store.Dir, st.ReceiptArtifact.Path)); err != nil {
		t.Errorf("status references a missing receipt file: %v", err)
	}
}
