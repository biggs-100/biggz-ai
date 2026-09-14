package sddattempt

// Objective advance (fix-attempt-ledger-scope slice 2). A passed settle
// completes only its own work unit, not the change: a distinct --work-unit
// opens a successor generation with its own budget while every prior attempt
// stays preserved, the same work unit is refused with the successor-naming
// reason, an open objective admits only its exact scope, Reset preserves the
// attempt chain, and the refund cap belongs to the live generation.
// Mirrors gentle-ai's runtime_objective_advance_test.go assertion list.

import (
	"errors"
	"strings"
	"testing"
)

// advanceEvidence renders one deterministic sha256-shaped evidence revision.
func advanceEvidence(c byte) string {
	return "sha256:" + strings.Repeat(string(c), 64)
}

// settlePassed completes the attempt selected by token with a passing
// outcome, so each advance test starts from the terminal state a passed
// objective leaves behind.
func settlePassed(t *testing.T, change, token, requestID, evidence string, changedLines int) *SettleResult {
	t.Helper()
	result, err := Settle(SettleParams{
		ChangeName: change, RepoRoot: "r", Token: token, RequestID: requestID,
		Outcome: "passed", EvidenceRevision: evidence, Diagnosis: "gates passed",
		HarnessDisposition: "reused", CleanupEvidence: "cleanup completed",
		ProcessEvidence: "process scan found no descendants",
		ChangedLines:    changedLines,
	})
	if err != nil {
		t.Fatalf("Settle(passed): %v", err)
	}
	if !result.Complete {
		t.Fatalf("passed settle did not complete its work unit: %+v", result)
	}
	return result
}

// TestAdvance_DistinctWorkUnitAfterPassedObjective is the reported defect's
// core assertion: a passed apply objective must not make the change
// terminal for a distinct downstream work unit. The successor opens its own
// generation and budget while the completed apply attempts stay immutable.
func TestAdvance_DistinctWorkUnitAfterPassedObjective(t *testing.T) {
	setStoreRoot(t)

	apply, err := Acquire(AcquireParams{
		ChangeName: "ch-advance", RepoRoot: "r", RequestID: "advance-apply-acquire",
		WorkUnit: "migration-schema-implementation", EvidenceGoal: "implement the migration schema",
		MaxAttempts: 3, MaxLines: 20,
	})
	if err != nil {
		t.Fatalf("Acquire(apply): %v", err)
	}
	passed := settlePassed(t, "ch-advance", apply.Token, "advance-apply-settle", advanceEvidence('a'), 20)
	before := recordCount(t, ledgerDir("ch-advance"))

	// The successor names a DIFFERENT work unit: it opens its own generation
	// with the budget IT requests. Reset is not the continuation.
	advanced, err := Acquire(AcquireParams{
		ChangeName: "ch-advance", RepoRoot: "r", RequestID: "advance-verify-acquire",
		WorkUnit: "requirements-runtime-verification", EvidenceGoal: "independently verify implementation against spec tasks",
		MaxAttempts: 2, MaxLines: 30,
	})
	if err != nil {
		t.Fatalf("distinct verification work unit was refused after a passed apply: %v", err)
	}
	if advanced.Token == "" {
		t.Fatal("advance must mint the successor's own token")
	}
	if got := recordCount(t, ledgerDir("ch-advance")); got != before+1 {
		t.Fatalf("advance wrote %d records, want exactly one", got-before)
	}

	status, err := StatusWithInstance("ch-advance", "r", "")
	if err != nil {
		t.Fatalf("StatusWithInstance: %v", err)
	}
	if status.Complete || status.DecisionRequired {
		t.Fatalf("advanced status = %+v, want an open successor objective", status)
	}
	if status.ActiveAttempt != 2 || status.NextAction != "continue" {
		t.Fatalf("advanced status attempt/action = %d/%q, want 2/continue", status.ActiveAttempt, status.NextAction)
	}
	if status.BlockedReason != BlockedReasonActiveAttempt {
		t.Fatalf("advanced status blocked reason = %q, want %q (only the live attempt blocks)", status.BlockedReason, BlockedReasonActiveAttempt)
	}
	if status.Generation != 2 {
		t.Fatalf("status generation = %d, want 2", status.Generation)
	}
	if status.CumulativeChangedLines != 0 {
		t.Fatalf("successor cumulative lines = %d, want 0 (fresh accumulator)", status.CumulativeChangedLines)
	}
	if status.LifetimeAttempts != 2 || status.LifetimeChangedLines != 20 {
		t.Fatalf("lifetime view = %d attempts / %d lines, want 2 / 20", status.LifetimeAttempts, status.LifetimeChangedLines)
	}
	if status.LastAdvance == nil || status.LastAdvance.ToGeneration != 2 ||
		status.LastAdvance.ToWorkUnit != "requirements-runtime-verification" {
		t.Fatalf("status last advance = %+v, want the recorded succession", status.LastAdvance)
	}

	store, _, err := loadStore("ch-advance", "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	// The successor must hold the budget IT requested, not the one the passed
	// predecessor was bound to.
	if store.MaxAttempts != 2 || store.MaxLines != 30 {
		t.Fatalf("successor budget = %d/%d, want its own 2/30 (predecessor held 3/20)", store.MaxAttempts, store.MaxLines)
	}
	// Advance never records a maintainer reset.
	if len(store.Resets) != 0 {
		t.Fatalf("advance recorded %d resets, want none", len(store.Resets))
	}
	if len(store.Advances) != 1 {
		t.Fatalf("advances = %d entries, want 1", len(store.Advances))
	}
	advance := store.Advances[0]
	if advance.RequestID != "advance-verify-acquire" ||
		advance.FromWorkUnit != "migration-schema-implementation" ||
		advance.ToWorkUnit != "requirements-runtime-verification" ||
		advance.FromGeneration != 1 || advance.ToGeneration != 2 ||
		advance.MaxAttempts != 2 || advance.MaxLines != 30 ||
		advance.PrevRevision != passed.Revision {
		t.Fatalf("advance provenance = %+v", advance)
	}
	if len(store.Attempts) != 2 {
		t.Fatalf("attempt chain = %d, want 2 preserved", len(store.Attempts))
	}
	predecessor := store.Attempts[0]
	if predecessor.Outcome != "passed" || predecessor.WorkUnit != "migration-schema-implementation" ||
		predecessor.EvidenceRevision != advanceEvidence('a') || predecessor.ObjectiveGeneration != 0 {
		t.Fatalf("completed apply attempt mutated: %+v", predecessor)
	}
	if store.Attempts[1].Ordinal != 2 || store.Attempts[1].ObjectiveGeneration != 2 ||
		store.Attempts[1].WorkUnit != "requirements-runtime-verification" {
		t.Fatalf("successor attempt = %+v, want ordinal 2 stamped generation 2", store.Attempts[1])
	}

	// A replay of the same successor request converges without a second record.
	replayed, err := Acquire(AcquireParams{
		ChangeName: "ch-advance", RepoRoot: "r", RequestID: "advance-verify-acquire",
		WorkUnit: "requirements-runtime-verification", EvidenceGoal: "independently verify implementation against spec tasks",
		MaxAttempts: 2, MaxLines: 30,
	})
	if err != nil || replayed.Token != advanced.Token || recordCount(t, ledgerDir("ch-advance")) != before+1 {
		t.Fatalf("advance replay = %+v err=%v records=%d", replayed, err, recordCount(t, ledgerDir("ch-advance")))
	}
}

// TestAdvance_FreshBudget pins the budget decision: the successor takes its
// own requested budget and its accumulator restarts at zero, while the
// lifetime total keeps the predecessor's lines.
func TestAdvance_FreshBudget(t *testing.T) {
	setStoreRoot(t)

	apply, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-budget", RepoRoot: "r", RequestID: "fresh-apply-acquire",
		WorkUnit: "apply", EvidenceGoal: "goal", MaxAttempts: 1, MaxLines: 500,
	})
	if err != nil {
		t.Fatalf("Acquire(apply): %v", err)
	}
	settlePassed(t, "ch-advance-budget", apply.Token, "fresh-apply-settle", advanceEvidence('b'), 40)

	successor, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-budget", RepoRoot: "r", RequestID: "fresh-verify-acquire",
		WorkUnit: "verify", EvidenceGoal: "verify the change", MaxAttempts: 5, MaxLines: 500,
	})
	if err != nil {
		t.Fatalf("successor with its own budget was refused: %v", err)
	}
	if successor.Token == "" {
		t.Fatal("successor acquire returned no token")
	}

	status, err := StatusWithInstance("ch-advance-budget", "r", "")
	if err != nil {
		t.Fatalf("StatusWithInstance: %v", err)
	}
	if status.CumulativeChangedLines != 0 {
		t.Fatalf("successor cumulative lines = %d, want 0 (the accumulator restarts)", status.CumulativeChangedLines)
	}
	if status.LifetimeChangedLines != 40 {
		t.Fatalf("lifetime lines = %d, want 40 (the predecessor's lines survive)", status.LifetimeChangedLines)
	}
	store, _, err := loadStore("ch-advance-budget", "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if store.MaxAttempts != 5 {
		t.Fatalf("successor max attempts = %d, want 5 (the successor's own request, not the predecessor's 1)", store.MaxAttempts)
	}
}

// TestAdvance_GenerationAttribution pins where the generation lands: the
// successor attempt is stamped generation 2 on the wire, predecessors stay
// generation 1 by field absence, and generation-1 records never carry the
// new members at all.
func TestAdvance_GenerationAttribution(t *testing.T) {
	setStoreRoot(t)

	apply, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-gen", RepoRoot: "r", RequestID: "gen-apply-acquire",
		WorkUnit: "apply", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("Acquire(apply): %v", err)
	}
	settlePassed(t, "ch-advance-gen", apply.Token, "gen-apply-settle", advanceEvidence('c'), 0)
	requireNoGenerationFields(t, storeFileBytes(t, "ch-advance-gen"))

	if _, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-gen", RepoRoot: "r", RequestID: "gen-verify-acquire",
		WorkUnit: "verify", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	}); err != nil {
		t.Fatalf("successor acquire: %v", err)
	}
	raw := storeFileBytes(t, "ch-advance-gen")
	if !strings.Contains(string(raw), `"generation":2`) {
		t.Fatalf("successor record does not stamp generation 2: %s", raw)
	}
	if !strings.Contains(string(raw), `"objective_generation":2`) {
		t.Fatalf("successor record does not attribute its attempt to generation 2: %s", raw)
	}

	store, _, err := loadStore("ch-advance-gen", "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if store.Generation != 2 {
		t.Fatalf("store generation = %d, want 2", store.Generation)
	}
	if store.Attempts[0].ObjectiveGeneration != 0 {
		t.Fatalf("predecessor generation = %d, want 0 (generation 1 by absence)", store.Attempts[0].ObjectiveGeneration)
	}
	if store.Attempts[1].ObjectiveGeneration != 2 {
		t.Fatalf("successor generation = %d, want 2", store.Attempts[1].ObjectiveGeneration)
	}
}

// TestAdvance_SameWorkUnitRefused pins the refusal: repeating the settled
// work unit — even under a restated evidence goal — stays complete, names the
// successor route, and leaves the ledger untouched.
func TestAdvance_SameWorkUnitRefused(t *testing.T) {
	setStoreRoot(t)

	apply, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-repeat", RepoRoot: "r", RequestID: "repeat-apply-acquire",
		WorkUnit: "migration-schema-implementation", EvidenceGoal: "implement the migration schema",
		MaxAttempts: 3, MaxLines: 20,
	})
	if err != nil {
		t.Fatalf("Acquire(apply): %v", err)
	}
	passed := settlePassed(t, "ch-advance-repeat", apply.Token, "repeat-apply-settle", advanceEvidence('d'), 0)
	before := recordCount(t, ledgerDir("ch-advance-repeat"))

	for _, test := range []struct {
		name    string
		request AcquireParams
	}{
		{name: "identical scope", request: AcquireParams{
			ChangeName: "ch-advance-repeat", RepoRoot: "r", RequestID: "repeat-same-scope",
			WorkUnit: "migration-schema-implementation", EvidenceGoal: "implement the migration schema",
			MaxAttempts: 3, MaxLines: 20,
		}},
		{name: "restated evidence goal", request: AcquireParams{
			ChangeName: "ch-advance-repeat", RepoRoot: "r", RequestID: "repeat-restated-goal",
			WorkUnit: "migration-schema-implementation", EvidenceGoal: "implement the migration schema once more",
			MaxAttempts: 3, MaxLines: 20,
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Acquire(test.request)
			var blocked *BlockedError
			if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonWorkUnitComplete {
				t.Fatalf("repeat refusal = %v, want blocked(%s)", err, BlockedReasonWorkUnitComplete)
			}
			for _, want := range []string{
				`work unit "migration-schema-implementation" is complete`,
				`biggz sdd-attempt acquire`,
				`--work-unit`,
				`with a different --work-unit`,
				`reset discards this scope instead of succeeding it`,
			} {
				if !strings.Contains(blocked.Exit, want) {
					t.Fatalf("refusal exit %q must contain %q", blocked.Exit, want)
				}
			}
			// The refusal mutates nothing: same revision, same records, same chain.
			status, err := StatusWithInstance("ch-advance-repeat", "r", "")
			if err != nil {
				t.Fatalf("StatusWithInstance: %v", err)
			}
			if status.Revision != passed.Revision || !status.Complete ||
				recordCount(t, ledgerDir("ch-advance-repeat")) != before {
				t.Fatalf("refused repeat mutated authority: status=%+v records=%d",
					status, recordCount(t, ledgerDir("ch-advance-repeat")))
			}
		})
	}
}

// TestAdvance_RefusedWhenDecisionIsRequired pins that advance never launders
// a budget that decision-required is holding.
func TestAdvance_RefusedWhenDecisionIsRequired(t *testing.T) {
	setStoreRoot(t)

	apply, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-exhausted", RepoRoot: "r", RequestID: "exhausted-apply-acquire",
		WorkUnit: "apply", EvidenceGoal: "goal", MaxAttempts: 1, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("Acquire(apply): %v", err)
	}
	if _, err := Settle(SettleParams{
		ChangeName: "ch-advance-exhausted", RepoRoot: "r", Token: apply.Token,
		RequestID: "exhausted-apply-settle", Outcome: "failed",
		Diagnosis: "apply failed and exhausted its budget",
	}); err != nil {
		t.Fatalf("Settle(failed): %v", err)
	}
	status, err := StatusWithInstance("ch-advance-exhausted", "r", "")
	if err != nil {
		t.Fatalf("StatusWithInstance: %v", err)
	}
	if !status.DecisionRequired || status.Complete {
		t.Fatalf("fixture did not reach decision_required: %+v", status)
	}
	before := recordCount(t, ledgerDir("ch-advance-exhausted"))

	_, err = Acquire(AcquireParams{
		ChangeName: "ch-advance-exhausted", RepoRoot: "r", RequestID: "exhausted-verify-acquire",
		WorkUnit: "requirements-runtime-verification", EvidenceGoal: "goal", MaxAttempts: 2, MaxLines: 30,
	})
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonBudgetExhausted {
		t.Fatalf("advance past decision_required = %v, want blocked(%s)", err, BlockedReasonBudgetExhausted)
	}
	if got := recordCount(t, ledgerDir("ch-advance-exhausted")); got != before {
		t.Fatalf("refused advance wrote %d records", got-before)
	}
}

// TestAdvance_LifetimeSurvives pins the lifetime view across the generation
// boundary: the successor's attempt grows the lifetime total while the
// per-generation accumulator restarts.
func TestAdvance_LifetimeSurvives(t *testing.T) {
	setStoreRoot(t)

	first, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-lifetime", RepoRoot: "r", RequestID: "life-apply-1",
		WorkUnit: "apply", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("Acquire #1: %v", err)
	}
	if _, err := Settle(SettleParams{
		ChangeName: "ch-advance-lifetime", RepoRoot: "r", Token: first.Token,
		RequestID: "life-settle-1", Outcome: "failed", Diagnosis: "first pass failed",
		ChangedLines: 10,
	}); err != nil {
		t.Fatalf("Settle(failed): %v", err)
	}
	second, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-lifetime", RepoRoot: "r", RequestID: "life-apply-2",
		WorkUnit: "apply", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("Acquire #2: %v", err)
	}
	settlePassed(t, "ch-advance-lifetime", second.Token, "life-settle-2", advanceEvidence('e'), 0)

	successor, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-lifetime", RepoRoot: "r", RequestID: "life-verify-1",
		WorkUnit: "verify", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("successor acquire: %v", err)
	}
	if successor.Token == "" {
		t.Fatal("successor acquire returned no token")
	}

	status, err := StatusWithInstance("ch-advance-lifetime", "r", "")
	if err != nil {
		t.Fatalf("StatusWithInstance: %v", err)
	}
	if status.LifetimeAttempts != 3 {
		t.Fatalf("LifetimeAttempts = %d, want 3", status.LifetimeAttempts)
	}
	if status.AttemptCount != 3 {
		t.Fatalf("AttemptCount = %d, want 3 (the chain never shrinks)", status.AttemptCount)
	}
	if status.LifetimeChangedLines != 10 {
		t.Fatalf("LifetimeChangedLines = %d, want 10", status.LifetimeChangedLines)
	}
	if status.Generation != 2 {
		t.Fatalf("generation = %d, want 2", status.Generation)
	}
}

// TestAdvance_SuccessorStartsWithFullRefundBudget pins the modified refund
// requirement: the predecessor generation exhausted its 2x cap, and the
// successor must still be admitted with its own fresh refund budget.
func TestAdvance_SuccessorStartsWithFullRefundBudget(t *testing.T) {
	setStoreRoot(t)

	// Generation 1 spends its whole 2x allowance (3 refund-eligible
	// interrupted attempts plus 3 delivered ones) and still passes on its
	// last attempt.
	for i := 1; i <= 3; i++ {
		acq, err := Acquire(AcquireParams{
			ChangeName: "ch-advance-refund", RepoRoot: "r",
			RequestID: "refund-apply-" + string(rune('0'+i)),
			WorkUnit:  "apply", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 1000,
		})
		if err != nil {
			t.Fatalf("Acquire interrupted %d: %v", i, err)
		}
		if _, err := Settle(SettleParams{
			ChangeName: "ch-advance-refund", RepoRoot: "r", Token: acq.Token,
			RequestID: "refund-settle-" + string(rune('0'+i)), Outcome: "interrupted",
			Diagnosis: "interrupted without lines",
		}); err != nil {
			t.Fatalf("Settle interrupted %d: %v", i, err)
		}
	}
	for i := 4; i <= 5; i++ {
		acq, err := Acquire(AcquireParams{
			ChangeName: "ch-advance-refund", RepoRoot: "r",
			RequestID: "refund-apply-" + string(rune('0'+i)),
			WorkUnit:  "apply", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 1000,
		})
		if err != nil {
			t.Fatalf("Acquire delivered %d: %v", i, err)
		}
		if _, err := Settle(SettleParams{
			ChangeName: "ch-advance-refund", RepoRoot: "r", Token: acq.Token,
			RequestID: "refund-settle-" + string(rune('0'+i)), Outcome: "failed",
			Diagnosis: "delivered failure", ChangedLines: 10,
		}); err != nil {
			t.Fatalf("Settle failed %d: %v", i, err)
		}
	}
	last, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-refund", RepoRoot: "r", RequestID: "refund-apply-6",
		WorkUnit: "apply", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 1000,
	})
	if err != nil {
		t.Fatalf("Acquire final: %v", err)
	}
	settlePassed(t, "ch-advance-refund", last.Token, "refund-settle-6", advanceEvidence('f'), 0)

	store, _, err := loadStore("ch-advance-refund", "r")
	if err != nil {
		t.Fatalf("loadStore before advance: %v", err)
	}
	if len(store.Attempts) != 6 {
		t.Fatalf("predecessor chain = %d attempts, want 6 (the full 2x allowance)", len(store.Attempts))
	}
	if got := runtimeRefundedAttempts(store); got != 3 {
		t.Fatalf("predecessor refunds = %d, want 3 (capped)", got)
	}

	// The successor's first acquire must not be charged with the
	// predecessor's refunds or attempt count.
	if _, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-refund", RepoRoot: "r", RequestID: "refund-verify-1",
		WorkUnit: "verify", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 1000,
	}); err != nil {
		t.Fatalf("successor was charged with the predecessor's refund cap: %v", err)
	}
	advanced, _, err := loadStore("ch-advance-refund", "r")
	if err != nil {
		t.Fatalf("loadStore after advance: %v", err)
	}
	if got := runtimeRefundedAttempts(advanced); got != 0 {
		t.Fatalf("successor refunds = %d, want 0 (its own generation only)", got)
	}
	if got := len(liveGenerationAttempts(advanced)); got != 1 {
		t.Fatalf("successor live attempts = %d, want 1", got)
	}
	if got := len(advanced.Attempts); got != 7 {
		t.Fatalf("preserved chain = %d attempts, want 7", got)
	}
}

// TestAdvance_ClearsLiveEvidence pins that the successor objective does not
// inherit the predecessor's live evidence binding, while the preserved chain
// keeps the remediated failure auditable.
func TestAdvance_ClearsLiveEvidence(t *testing.T) {
	setStoreRoot(t)

	failed, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-evidence", RepoRoot: "r", RequestID: "evidence-apply-1",
		WorkUnit: "apply", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("Acquire #1: %v", err)
	}
	failedEvidence := advanceEvidence('7')
	if _, err := Settle(SettleParams{
		ChangeName: "ch-advance-evidence", RepoRoot: "r", Token: failed.Token,
		RequestID: "evidence-settle-1", Outcome: "failed", EvidenceRevision: failedEvidence,
		Diagnosis: "verification failed",
	}); err != nil {
		t.Fatalf("Settle(failed): %v", err)
	}
	corrective, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-evidence", RepoRoot: "r", RequestID: "evidence-apply-2",
		WorkUnit: "apply", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
		RemediatesEvidenceRevision: failedEvidence,
	})
	if err != nil {
		t.Fatalf("Acquire(corrective): %v", err)
	}
	corrected, err := Settle(SettleParams{
		ChangeName: "ch-advance-evidence", RepoRoot: "r", Token: corrective.Token,
		RequestID: "evidence-settle-2", Outcome: "passed",
		EvidenceRevision:           advanceEvidence('8'),
		RemediatesEvidenceRevision: failedEvidence,
		Diagnosis:                  "focused tests pass; rollback recorded",
	})
	if err != nil {
		t.Fatalf("Settle(corrective passed): %v", err)
	}
	if !corrected.Complete {
		t.Fatalf("corrective settle did not complete its work unit: %+v", corrected)
	}

	before, _, err := loadStore("ch-advance-evidence", "r")
	if err != nil {
		t.Fatalf("loadStore before advance: %v", err)
	}
	if before.EvidenceRevision != failedEvidence {
		t.Fatalf("predecessor live evidence = %q, want %q", before.EvidenceRevision, failedEvidence)
	}

	if _, err := Acquire(AcquireParams{
		ChangeName: "ch-advance-evidence", RepoRoot: "r", RequestID: "evidence-verify-1",
		WorkUnit: "verify", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	}); err != nil {
		t.Fatalf("successor acquire: %v", err)
	}
	after, _, err := loadStore("ch-advance-evidence", "r")
	if err != nil {
		t.Fatalf("loadStore after advance: %v", err)
	}
	if after.EvidenceRevision != "" {
		t.Fatalf("successor inherited predecessor evidence: %q", after.EvidenceRevision)
	}
	if obligation := deriveSettleObligation(after); obligation != nil {
		t.Fatalf("successor raised a settle obligation: %+v", obligation)
	}
	// The remediated failure stays auditable in the preserved chain.
	if len(after.Attempts) != 3 || after.Attempts[0].Outcome != "failed" ||
		after.Attempts[0].EvidenceRevision != failedEvidence ||
		after.Attempts[1].RemediatesEvidenceRevision != failedEvidence {
		t.Fatalf("preserved chain lost the remediated failure: %+v", after.Attempts)
	}
}

// TestScopeGuard_AnyFieldRefusedAndUnchangedAdmitted pins the one unified
// scope guard: on an open objective any of the four scope fields may only
// change under an explicit reset, while repeating the exact scope is a
// continuation.
func TestScopeGuard_AnyFieldRefusedAndUnchangedAdmitted(t *testing.T) {
	setStoreRoot(t)

	acq, err := Acquire(AcquireParams{
		ChangeName: "ch-scope-guard", RepoRoot: "r", RequestID: "guard-acquire-1",
		WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if _, err := Settle(SettleParams{
		ChangeName: "ch-scope-guard", RepoRoot: "r", Token: acq.Token,
		RequestID: "guard-settle-1", Outcome: "failed", Diagnosis: "still open",
	}); err != nil {
		t.Fatalf("Settle(failed): %v", err)
	}
	before := recordCount(t, ledgerDir("ch-scope-guard"))

	for _, test := range []struct {
		name    string
		request AcquireParams
	}{
		{name: "work unit", request: AcquireParams{
			WorkUnit: "w2", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400}},
		{name: "evidence goal", request: AcquireParams{
			WorkUnit: "w", EvidenceGoal: "goal2", MaxAttempts: 3, MaxLines: 400}},
		{name: "max attempts", request: AcquireParams{
			WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: 5, MaxLines: 400}},
		{name: "max lines", request: AcquireParams{
			WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 800}},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.request.ChangeName = "ch-scope-guard"
			test.request.RepoRoot = "r"
			test.request.RequestID = "guard-refused-" + strings.ReplaceAll(test.name, " ", "-")
			_, err := Acquire(test.request)
			var blocked *BlockedError
			if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonInvalidContinuation {
				t.Fatalf("changed %s = %v, want blocked(%s)", test.name, err, BlockedReasonInvalidContinuation)
			}
			if !strings.Contains(blocked.Exit, "without reset") {
				t.Fatalf("refusal exit %q must name the missing reset", blocked.Exit)
			}
			if got := recordCount(t, ledgerDir("ch-scope-guard")); got != before {
				t.Fatalf("refused %s wrote %d records", test.name, got-before)
			}
		})
	}

	// Repeating the identical scope is admitted without any reset.
	next, err := Acquire(AcquireParams{
		ChangeName: "ch-scope-guard", RepoRoot: "r", RequestID: "guard-admitted",
		WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("unchanged scope must be admitted: %v", err)
	}
	if next.Token == "" {
		t.Fatal("admitted continuation returned no token")
	}
	store, _, err := loadStore("ch-scope-guard", "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if len(store.Attempts) != 2 || store.Attempts[1].Ordinal != 2 {
		t.Fatalf("chain = %+v, want the continuation appended as attempt 2", store.Attempts)
	}
}

// TestReset_PreservesTheAttemptChain pins Reset as the maintainer discard
// path: the live objective is cleared and the attempt audit survives it.
func TestReset_PreservesTheAttemptChain(t *testing.T) {
	setStoreRoot(t)

	acq, err := Acquire(AcquireParams{
		ChangeName: "ch-reset-preserve", RepoRoot: "r", RequestID: "reset-acquire-1",
		WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	settlePassed(t, "ch-reset-preserve", acq.Token, "reset-settle-1", advanceEvidence('9'), 20)

	if _, err := Reset(ResetParams{
		ChangeName: "ch-reset-preserve", RepoRoot: "r", RequestID: "reset-preserve-1",
		Reason: "discard this scope", ResetBy: "maintainer",
	}); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	status, err := StatusWithInstance("ch-reset-preserve", "r", "")
	if err != nil {
		t.Fatalf("StatusWithInstance: %v", err)
	}
	if status.Complete || status.DecisionRequired || status.ActiveAttempt != 0 || status.NextAction != "begin" {
		t.Fatalf("live state after reset = %+v, want cleared", status)
	}
	if status.AttemptCount != 1 || status.LifetimeAttempts != 1 || status.LifetimeChangedLines != 20 {
		t.Fatalf("attempt audit after reset = %d/%d/%d, want 1 attempt and 20 lifetime lines preserved",
			status.AttemptCount, status.LifetimeAttempts, status.LifetimeChangedLines)
	}

	store, _, err := loadStore("ch-reset-preserve", "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if len(store.Attempts) != 1 {
		t.Fatalf("attempts after reset = %d, want the N prior attempts preserved", len(store.Attempts))
	}
	attempt := store.Attempts[0]
	if attempt.Outcome != "passed" || attempt.EvidenceRevision != advanceEvidence('9') || attempt.ChangedLines != 20 {
		t.Fatalf("reset mutated a prior attempt: %+v", attempt)
	}
	if store.EvidenceRevision != "" {
		t.Fatalf("reset must clear the live evidence, got %q", store.EvidenceRevision)
	}
	if len(store.Resets) != 1 {
		t.Fatalf("reset provenance = %d entries, want 1", len(store.Resets))
	}
}
