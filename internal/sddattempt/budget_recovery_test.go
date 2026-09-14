package sddattempt

// Budget recovery after an exhausted live generation (fix-attempt-ledger-scope
// corrective slice). Reset preserves the attempt chain as the audit and opens
// a FRESH budget epoch, so the 2x cap of the closed generation can neither
// gate the re-opened objective nor a successor that names a different work
// unit, and the blocked exits name a remedy that actually works. These tests
// are the regression pin for the defect where Reset preserved the chain but
// left every preserved attempt counting as live, stranding the ledger with no
// exit once the chain reached 2*MaxAttempts.

import (
	"errors"
	"strings"
	"testing"
)

// budgetEvidence renders one deterministic sha256-shaped evidence revision.
func budgetEvidence(digit byte) string {
	return "sha256:" + strings.Repeat(string(digit), 64)
}

// exhaustLiveGenerationBudget drives one change into the exact state the
// defect describes: the live generation reaches 2*maxAttempts recorded
// attempts and the ledger lands in decision-required. The first attempt is a
// delivered failure with evidence (so the preserved chain keeps a meaningful
// audit), the remaining ones are refund-eligible interrupted attempts.
func exhaustLiveGenerationBudget(t *testing.T, change string, maxAttempts int) {
	t.Helper()
	for i := 1; i <= 2*maxAttempts; i++ {
		acq, err := Acquire(AcquireParams{
			ChangeName: change, RepoRoot: "r",
			RequestID:    change + "-exhaust-acquire-" + string(rune('0'+i)),
			WorkUnit:     "w",
			EvidenceGoal: "goal",
			MaxAttempts:  maxAttempts, MaxLines: 400,
		})
		if err != nil {
			t.Fatalf("exhaust acquire %d: %v", i, err)
		}
		settle := SettleParams{
			ChangeName: change, RepoRoot: "r", Token: acq.Token,
			RequestID: change + "-exhaust-settle-" + string(rune('0'+i)),
			Outcome:   "interrupted", Diagnosis: "interrupted without lines",
		}
		if i == 1 {
			settle.Outcome = "failed"
			settle.EvidenceRevision = budgetEvidence('e')
			settle.Diagnosis = "first pass failed"
			settle.ChangedLines = 1
		}
		if _, err := Settle(settle); err != nil {
			t.Fatalf("exhaust settle %d: %v", i, err)
		}
	}
	store, _, err := loadStore(change, "r")
	if err != nil {
		t.Fatalf("loadStore after exhaustion: %v", err)
	}
	if !store.DecisionRequired {
		t.Fatalf("fixture did not exhaust the live generation: decision_required=%v attempts=%d",
			store.DecisionRequired, len(store.Attempts))
	}
	if got := len(liveGenerationAttempts(store)); got != 2*maxAttempts {
		t.Fatalf("live attempts at exhaustion = %d, want %d (the 2x cap)", got, 2*maxAttempts)
	}
}

// TestReset_OpensFreshBudgetForTheLiveObjective is the defect's core
// assertion: an exhausted live generation must be recoverable through the
// remedy its own blocked exit advertises. Reset preserves the whole chain as
// the audit and stamps a fresh budget epoch, so a same-label acquire is
// ADMITTED with a full fresh allowance instead of dying on the preserved
// chain's 2x cap.
func TestReset_OpensFreshBudgetForTheLiveObjective(t *testing.T) {
	setStoreRoot(t)
	const change = "ch-budget-recovery"
	const maxAttempts = 2
	exhaustLiveGenerationBudget(t, change, maxAttempts)

	// Exhausted: the ledger refuses the acquire and its exit must name the
	// remedy — reset — that this test then proves actually works.
	_, err := Acquire(AcquireParams{
		ChangeName: change, RepoRoot: "r", RequestID: "recovery-blocked-acquire",
		WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: maxAttempts, MaxLines: 400,
	})
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonBudgetExhausted {
		t.Fatalf("exhausted acquire = %v, want blocked(%s)", err, BlockedReasonBudgetExhausted)
	}
	if !strings.Contains(blocked.Exit, "reset") {
		t.Fatalf("exhausted exit %q must name a remedy that works (reset)", blocked.Exit)
	}

	// Reset preserves the audit and opens a fresh budget epoch.
	reset, err := Reset(ResetParams{
		ChangeName: change, RepoRoot: "r", RequestID: "recovery-reset",
		Reason: "recover the exhausted objective", ResetBy: "maintainer",
	})
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if reset.AttemptsPreserved != 2*maxAttempts {
		t.Fatalf("reset preserved %d attempts, want %d", reset.AttemptsPreserved, 2*maxAttempts)
	}

	status, err := StatusWithInstance(change, "r", "")
	if err != nil {
		t.Fatalf("StatusWithInstance after reset: %v", err)
	}
	if status.Complete || status.DecisionRequired || status.ActiveAttempt != 0 || status.NextAction != "begin" {
		t.Fatalf("live state after reset = %+v, want cleared", status)
	}
	if status.Generation != 2 {
		t.Fatalf("generation after reset = %d, want 2 (a fresh budget epoch)", status.Generation)
	}
	if status.AttemptCount != 2*maxAttempts || status.LifetimeAttempts != 2*maxAttempts {
		t.Fatalf("attempt audit after reset = %d/%d, want %d preserved",
			status.AttemptCount, status.LifetimeAttempts, 2*maxAttempts)
	}
	if status.LifetimeChangedLines != 1 {
		t.Fatalf("lifetime lines after reset = %d, want 1", status.LifetimeChangedLines)
	}

	// The advertisement must be true: reset really does free the budget. A
	// same-label acquire is admitted with the fresh allowance.
	acq, err := Acquire(AcquireParams{
		ChangeName: change, RepoRoot: "r", RequestID: "recovery-acquire-1",
		WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: maxAttempts, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("post-reset same-label acquire was refused: %v", err)
	}
	if acq.Token == "" {
		t.Fatal("post-reset acquire returned no token")
	}

	// The fresh allowance is real: one delivered settle leaves the full
	// remainder, because the predecessor's spent attempts are not charged.
	settled, err := Settle(SettleParams{
		ChangeName: change, RepoRoot: "r", Token: acq.Token,
		RequestID: "recovery-settle-1", Outcome: "failed", ChangedLines: 3,
		Diagnosis: "recovery attempt failed",
	})
	if err != nil {
		t.Fatalf("post-reset settle: %v", err)
	}
	if settled.DecisionRequired {
		t.Fatalf("one delivered settle exhausted the fresh allowance: %+v", settled)
	}
	if settled.RemainingAttempts != maxAttempts-1 {
		t.Fatalf("remaining attempts after one delivered settle = %d, want %d",
			settled.RemainingAttempts, maxAttempts-1)
	}

	// A further continuation is admitted too, and the re-opened objective is
	// guarded again while it is open: changing the budget still needs reset.
	next, err := Acquire(AcquireParams{
		ChangeName: change, RepoRoot: "r", RequestID: "recovery-acquire-2",
		WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: maxAttempts, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("second post-reset acquire was refused: %v", err)
	}
	if _, err := Settle(SettleParams{
		ChangeName: change, RepoRoot: "r", Token: next.Token,
		RequestID: "recovery-settle-2", Outcome: "interrupted",
		Diagnosis: "interrupted without lines",
	}); err != nil {
		t.Fatalf("second post-reset settle: %v", err)
	}
	_, err = Acquire(AcquireParams{
		ChangeName: change, RepoRoot: "r", RequestID: "recovery-guard-acquire",
		WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: maxAttempts + 3, MaxLines: 400,
	})
	var guarded *BlockedError
	if !errors.As(err, &guarded) || guarded.Reason != BlockedReasonInvalidContinuation {
		t.Fatalf("budget change under the re-opened objective = %v, want blocked(%s)",
			err, BlockedReasonInvalidContinuation)
	}

	// The audit survives intact: every pre-reset attempt is still in the
	// chain with its outcome and evidence, attributed to the closed
	// generation, and the lifetime views never decreased.
	store, _, err := loadStore(change, "r")
	if err != nil {
		t.Fatalf("loadStore after recovery: %v", err)
	}
	if len(store.Attempts) != 2*maxAttempts+2 {
		t.Fatalf("attempt chain = %d, want %d (preserved + recovered)",
			len(store.Attempts), 2*maxAttempts+2)
	}
	preserved := store.Attempts[:2*maxAttempts]
	if preserved[0].Outcome != "failed" || preserved[0].EvidenceRevision != budgetEvidence('e') ||
		preserved[0].ChangedLines != 1 {
		t.Fatalf("preserved failure was mutated by the reset: %+v", preserved[0])
	}
	for i, attempt := range preserved {
		if attempt.ObjectiveGeneration != 0 {
			t.Fatalf("preserved attempt %d moved generations: %+v", i, attempt)
		}
		if i > 0 && attempt.Outcome != "interrupted" {
			t.Fatalf("preserved attempt %d lost its outcome: %+v", i, attempt)
		}
	}
	for i := 2 * maxAttempts; i < len(store.Attempts); i++ {
		if store.Attempts[i].ObjectiveGeneration != 2 {
			t.Fatalf("recovered attempt %d generation = %d, want 2", i, store.Attempts[i].ObjectiveGeneration)
		}
	}
	if len(store.Resets) != 1 || store.Resets[0].ToGeneration != 2 {
		t.Fatalf("reset provenance = %+v, want one entry stamped with the generation it opened", store.Resets)
	}
	// The accumulator restarted with the epoch: only the recovered 3 lines
	// count, never the predecessor's 1.
	if store.CumulativeChangedLines != 3 {
		t.Fatalf("cumulative lines after recovery = %d, want 3 (fresh accumulator)", store.CumulativeChangedLines)
	}
	if status.LifetimeChangedLines >= lifetimeChangedLines(store) {
		t.Fatalf("lifetime lines decreased across the reset: %d -> %d",
			status.LifetimeChangedLines, lifetimeChangedLines(store))
	}
}

// TestSuccessorNotGatedByPredecessorExhaustedBudget is the probe's second
// symptom: an acquire naming a DIFFERENT work unit (and its own budget) must
// never be charged with, or blocked by, the predecessor's exhausted cap. The
// successor is admitted after the reset, with its own budget, and the
// preserved predecessor chain stays untouched in the audit.
func TestSuccessorNotGatedByPredecessorExhaustedBudget(t *testing.T) {
	setStoreRoot(t)
	const change = "ch-successor-recovery"
	exhaustLiveGenerationBudget(t, change, 2)

	// While the predecessor epoch is exhausted, even a different work unit
	// is refused — that is the state the reset must be able to leave.
	_, err := Acquire(AcquireParams{
		ChangeName: change, RepoRoot: "r", RequestID: "successor-blocked-acquire",
		WorkUnit: "verify-work", EvidenceGoal: "verify the change", MaxAttempts: 2, MaxLines: 400,
	})
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonBudgetExhausted {
		t.Fatalf("exhausted different-work-unit acquire = %v, want blocked(%s)", err, BlockedReasonBudgetExhausted)
	}

	reset, err := Reset(ResetParams{
		ChangeName: change, RepoRoot: "r", RequestID: "successor-reset",
		Reason: "succeed the exhausted objective", ResetBy: "maintainer",
		MaxAttempts: 2,
	})
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if reset.AttemptsPreserved != 4 {
		t.Fatalf("reset preserved %d attempts, want 4", reset.AttemptsPreserved)
	}

	// The successor names a different work unit and its own budget: admitted,
	// never charged with the predecessor's spent attempts.
	acq, err := Acquire(AcquireParams{
		ChangeName: change, RepoRoot: "r", RequestID: "successor-acquire-1",
		WorkUnit: "verify-work", EvidenceGoal: "verify the change", MaxAttempts: 5, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("successor was gated by the predecessor's exhausted budget: %v", err)
	}
	if acq.Token == "" {
		t.Fatal("successor acquire returned no token")
	}

	status, err := StatusWithInstance(change, "r", "")
	if err != nil {
		t.Fatalf("StatusWithInstance: %v", err)
	}
	if status.Generation != 2 || status.LifetimeAttempts != 5 || status.AttemptCount != 5 {
		t.Fatalf("successor status = generation %d / lifetime %d / attempts %d, want 2 / 5 / 5",
			status.Generation, status.LifetimeAttempts, status.AttemptCount)
	}
	if status.Complete || status.DecisionRequired {
		t.Fatalf("successor status claims completion or a decision: %+v", status)
	}

	store, _, err := loadStore(change, "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if store.MaxAttempts != 5 {
		t.Fatalf("successor budget = %d, want 5 (its own request)", store.MaxAttempts)
	}
	if len(store.Attempts) != 5 {
		t.Fatalf("attempt chain = %d, want 5", len(store.Attempts))
	}
	successor := store.Attempts[4]
	if successor.WorkUnit != "verify-work" || successor.ObjectiveGeneration != 2 {
		t.Fatalf("successor attempt = %+v, want verify-work at generation 2", successor)
	}
	predecessor := store.Attempts[0]
	if predecessor.Outcome != "failed" || predecessor.EvidenceRevision != budgetEvidence('e') {
		t.Fatalf("preserved predecessor mutated: %+v", predecessor)
	}
}
