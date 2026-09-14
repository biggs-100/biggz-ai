// Package sddattempt implements the SDD runtime attempt ledger.
//
// The ledger tracks each apply/verify/remediation attempt for an SDD change.
// It lives in the git common directory at
// <git-common-dir>/biggz/sdd-runtime/v1/<change>/ as content-addressed CAS
// records (record-<sha>.json + HEAD + LOCK); see cas_store.go for the layout
// and the one-time migration from the legacy home-dir single-file store.
//
// Design follows gentle-ai's compact sdd-attempt acquire/settle flow:
//   - status: read the ledger, report next_action
//   - begin: start a new attempt (CAS-guarded)
//   - finish: close the current attempt with outcome + evidence (CAS-guarded)
//   - reset: explicitly reset the ledger for a new objective
package sddattempt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// ─── Constants ───────────────────────────────────────────────────────────────

const (
	// RuntimeDir is the directory name of the runtime ledger container.
	RuntimeDir = "sdd-runtime"
	// RuntimeNoGitDir is the directory name of the machine-scoped fallback
	// ledger container, used when no git repository is present.
	RuntimeNoGitDir = "sdd-runtime-nogit"
	// RuntimeVersion is the schema version.
	RuntimeVersion = "v1"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// RuntimeStore is the complete ledger for one SDD change.
type RuntimeStore struct {
	// Revision is the SHA-256 content address of the record carrying this
	// state (CAS token): every mutation reads the current revision and
	// publishes a new record. A mismatch means a concurrent write happened.
	Revision string `json:"-"`

	// ChangeName identifies the SDD change this ledger belongs to.
	ChangeName string `json:"change_name"`

	// Current attempt tracking
	ActiveAttempt    int    `json:"active_attempt,omitempty"`
	DecisionRequired bool   `json:"decision_required,omitempty"`
	Complete         bool   `json:"complete,omitempty"`
	NextAction       string `json:"next_action,omitempty"` // "begin", "continue", "finish", "complete", ""

	// Objective scope
	ObjectiveID            string `json:"objective_id,omitempty"`
	MaxAttempts            int    `json:"max_attempts,omitempty"`
	MaxLines               int    `json:"max_changed_lines,omitempty"`
	CumulativeChangedLines int    `json:"cumulative_changed_lines,omitempty"`

	// Generation is the live objective generation: 1 names the objective
	// opened by the first acquire/begin. Generation 1 is represented by
	// field ABSENCE — the zero value is never stamped, because records are
	// content-addressed full snapshots re-verified on load and any value on
	// a generation-1 record would change its canonical bytes and break
	// every pre-change ledger. Readers derive 0 as 1 (never persisted).
	Generation int `json:"generation,omitempty"`

	// Evidence tracking
	WorkUnit     string `json:"work_unit,omitempty"`
	EvidenceGoal string `json:"evidence_goal,omitempty"`

	// Binding for remediation
	EvidenceRevision string `json:"evidence_revision,omitempty"`
	BindingRevision  string `json:"binding_revision,omitempty"`
	BindingLineage   string `json:"binding_lineage,omitempty"`

	// Attempt history (immutable after finish)
	Attempts []RuntimeAttempt `json:"attempts"`

	// Reset history (provenance tracking)
	Resets []RuntimeReset `json:"resets,omitempty"`

	// Advances is the objective-generation provenance history, one entry
	// per successor advance. It follows the Resets append-only pattern but
	// is purely additive (an advance never removes anything). omitempty
	// keeps generation-1 ledgers byte-identical to records written before
	// the field existed.
	Advances []RuntimeAdvance `json:"advances,omitempty"`

	// Requests records idempotency receipts for supplied request IDs: the
	// operation, the digest of the original request, and the recorded
	// outcome. A replay of the same request ID returns the SAME result
	// without mutating the ledger.
	Requests map[string]RuntimeRequestRecord `json:"requests,omitempty"`

	// Tokens maps compact acquire tokens to attempt ordinals. Each acquire
	// mints one token (content-address independent, persisted here) that
	// settle later presents to close exactly that attempt. This is the
	// bounded-path counterpart to begin/finish's active-ordinal tracking.
	Tokens map[string]int `json:"tokens,omitempty"`

	// Grants is the append-only per-change edit-authority audit history.
	// GrantedRoots is never persisted: it is derived read-time by
	// grantedRootsFor, so the snapshot hash stays stable.
	Grants []RuntimeGrant `json:"grants,omitempty"`

	// created_at / updated_at
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// RuntimeAttempt records a single attempt lifecycle.
type RuntimeAttempt struct {
	Ordinal     int    `json:"ordinal"`
	ObjectiveID string `json:"objective_id,omitempty"`

	// ObjectiveGeneration is the objective generation this attempt was
	// spent under. Generation 1 is represented by field ABSENCE (never an
	// explicit 1), so pre-change attempt records keep their content
	// address; readers derive 0 as 1.
	ObjectiveGeneration int `json:"objective_generation,omitempty"`

	WorkUnit string `json:"work_unit,omitempty"`

	// Timing
	BeganAt string `json:"began_at"`
	EndedAt string `json:"ended_at,omitempty"`

	// Outcome: "passed", "failed", "interrupted", "progress" (settle-only
	// non-terminal checkpoint: closes the attempt without completing the
	// ledger, so the next acquire in the same scope is admitted).
	Outcome string `json:"outcome,omitempty"`

	// Evidence
	EvidenceRevision string `json:"evidence_revision,omitempty"`

	// Diagnosis (human-readable)
	Diagnosis string `json:"diagnosis,omitempty"`

	// Harness disposition
	HarnessDisposition string `json:"harness_disposition,omitempty"`

	// Cleanup
	CleanupEvidence string `json:"cleanup_evidence,omitempty"`
	ProcessEvidence string `json:"process_evidence,omitempty"`

	// Remediation binding
	RemediatesEvidenceRevision string `json:"remediates_evidence_revision,omitempty"`

	ChangedLines int `json:"changed_lines,omitempty"`
}

// RuntimeReset records ledger reset provenance.
type RuntimeReset struct {
	Reason       string `json:"reason"`
	ResetBy      string `json:"reset_by"`
	ResetAt      string `json:"reset_at"`
	PrevRevision string `json:"prev_revision"`

	// ToGeneration is the budget epoch the reset opened: the generation every
	// preserved attempt stops counting as live under, for the budget, the 2x
	// cap, the refund allowance and the accumulator. omitempty keeps reset
	// entries written before the boundary existed byte-identical.
	ToGeneration int `json:"to_generation,omitempty"`
}

// RuntimeAdvance records one objective-generation advance: the admission of
// a successor objective after its predecessor completed. The store keeps
// one entry per advance (provenance only — Attempts already carries the full
// attempt chain); every field is omitempty and generation-1 ledgers never
// carry an entry, so the slice is invisible to pre-change canonical bytes.
type RuntimeAdvance struct {
	RequestID      string `json:"request_id,omitempty"`
	FromWorkUnit   string `json:"from_work_unit,omitempty"`
	ToWorkUnit     string `json:"to_work_unit,omitempty"`
	FromGeneration int    `json:"from_generation,omitempty"`
	ToGeneration   int    `json:"to_generation,omitempty"`
	MaxAttempts    int    `json:"max_attempts,omitempty"`
	MaxLines       int    `json:"max_changed_lines,omitempty"`
	PrevRevision   string `json:"prev_revision,omitempty"`
	AdvancedAt     string `json:"advanced_at,omitempty"`
}

// RuntimeRequestRecord is the idempotency receipt for one request ID. It is
// written only when the operation actually applied, and it stores the
// recorded outcome so a convergent replay returns that outcome — not the
// current ledger state — no matter what happened in between. The outcome's
// embedded revision names the record that first applied the request.
type RuntimeRequestRecord struct {
	Operation  string          `json:"operation"` // "begin" | "finish" | "reset"
	Digest     string          `json:"digest"`    // sha256:... of the canonical request
	Outcome    json.RawMessage `json:"outcome"`   // recorded result of the first application
	RecordedAt string          `json:"recorded_at"`
}

// Request ID syntax mirrors gentle-ai's canonical lowercase identifier.
var requestIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

const (
	requestDigestDomainBegin   = "biggz-ai.sdd-runtime-begin-request/v1"
	requestDigestDomainFinish  = "biggz-ai.sdd-runtime-finish-request/v1"
	requestDigestDomainReset   = "biggz-ai.sdd-runtime-reset-request/v1"
	requestDigestDomainGrant   = "biggz-ai.sdd-runtime-grant-request/v1"
	requestDigestDomainAcquire = "biggz-ai.sdd-runtime-acquire-request/v1"
	requestDigestDomainSettle  = "biggz-ai.sdd-runtime-settle-request/v1"

	opBegin   = "begin"
	opFinish  = "finish"
	opReset   = "reset"
	opGrant   = "grant"
	opAcquire = "acquire"
	opSettle  = "settle"
)

// Blocked reasons for the compact acquire/settle admission probe.
// They mirror gentle-ai's CompactBlockReason values used by RuntimeStatus.
const (
	BlockedReasonBudgetExhausted     = "budget_exhausted"
	BlockedReasonCorruptAuthority    = "corrupt_authority"
	BlockedReasonActiveAttempt       = "active_attempt"
	BlockedReasonInvalidContinuation = "invalid_continuation"
	// BlockedReasonWorkUnitComplete classifies a repeat of the work unit a
	// passed settle already completed: that objective is done, so the change
	// continues through a successor work unit (a different --work-unit) and
	// never through this one again. corrupt_authority stays reserved for a
	// completion that is genuinely anomalous — no passed attempt to name.
	BlockedReasonWorkUnitComplete = "work_unit_complete"
)

// budgetRecoveryHint is the remedy every exhausted-budget exit names: a reset
// opens a fresh budget epoch (the generation advances), so the preserved
// attempt chain stops counting as live while every recorded outcome, its
// evidence and the refund history stay in the audit. The exits append it
// verbatim, because an exit that advertises a remedy that does not work
// strands the ledger it describes.
const budgetRecoveryHint = "; run sdd-attempt reset to open a fresh budget — the preserved attempt chain and its evidence stay in the audit"

// RuntimeRecordRejectedError is the single typed error for all record rejections.
type RuntimeRecordRejectedError struct {
	Cause error
}

func (e *RuntimeRecordRejectedError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "runtime record rejected"
}

func (e *RuntimeRecordRejectedError) Unwrap() error { return e.Cause }

// runtimeChangedLineBudgetExceeded owns the line-budget check.
func runtimeChangedLineBudgetExceeded(s *RuntimeStore, delta int) bool {
	if s == nil {
		return false
	}
	if s.MaxLines == 0 {
		return false
	}
	return s.CumulativeChangedLines+delta > s.MaxLines
}

func runtimeAttemptDeliveredIncrement(attempt RuntimeAttempt) int {
	if attempt.Outcome == "interrupted" && attempt.ChangedLines == 0 {
		return 0
	}
	return 1
}

func runtimeAttemptDeliveredIncrementSlice(attempts []RuntimeAttempt) int {
	n := 0
	for _, a := range attempts {
		n += runtimeAttemptDeliveredIncrement(a)
	}
	return n
}

func runtimeRefundedAttempts(store *RuntimeStore) int {
	if store == nil {
		return 0
	}
	// The refund cap belongs to the generation currently open: attempts of a
	// generation that already succeeded stay in the chain but are never
	// charged against the successor's fresh 2x allowance.
	live := liveGenerationAttempts(store)
	delivered := runtimeAttemptDeliveredIncrementSlice(live)
	refunded := len(live) - delivered
	if refunded < 0 {
		refunded = 0
	}
	if refunded > store.MaxAttempts {
		refunded = store.MaxAttempts
	}
	return refunded
}

// derivedGeneration renders the persisted objective generation: the wire
// represents generation 1 by field ABSENCE, so every reader derives the same
// value (0 reads as 1) without touching canonical bytes.
func derivedGeneration(store *RuntimeStore) int {
	if store == nil {
		return 1
	}
	return max(store.Generation, 1)
}

// liveGenerationAttempts returns the attempts recorded under the store's
// live objective generation. A succeeded generation's attempts stay in the
// chain for the audit but must not charge the open generation's budget,
// refund cap or accumulator.
func liveGenerationAttempts(store *RuntimeStore) []RuntimeAttempt {
	if store == nil {
		return nil
	}
	generation := derivedGeneration(store)
	live := make([]RuntimeAttempt, 0, len(store.Attempts))
	for _, attempt := range store.Attempts {
		if max(attempt.ObjectiveGeneration, 1) == generation {
			live = append(live, attempt)
		}
	}
	return live
}

// lifetimeChangedLines totals the changed lines of the whole preserved chain:
// the lifetime view survives advances and resets because it is derived from
// the attempts themselves.
func lifetimeChangedLines(store *RuntimeStore) int {
	if store == nil {
		return 0
	}
	total := 0
	for _, attempt := range store.Attempts {
		total += attempt.ChangedLines
	}
	return total
}

// lastAdvanceEntry returns the most recent advance provenance entry, if any.
func lastAdvanceEntry(store *RuntimeStore) *RuntimeAdvance {
	if store == nil || len(store.Advances) == 0 {
		return nil
	}
	return &store.Advances[len(store.Advances)-1]
}

// lastAttemptPassed reports whether the chain's last recorded attempt is a
// passed one — the only shape a completed objective can carry.
func lastAttemptPassed(store *RuntimeStore) bool {
	return len(store.Attempts) > 0 && store.Attempts[len(store.Attempts)-1].Outcome == "passed"
}

// requestDigest hashes the canonical JSON of a request under a schema domain,
// mirroring gentle-ai's runtimeValueHash.
func requestDigest(domain string, params any) string {
	payload, _ := json.Marshal(params)
	sum := sha256.Sum256(append(append([]byte(domain), '\n'), payload...))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// recordRequest persists the idempotency receipt for a request ID after the
// operation applied. It is a no-op when no request ID was supplied. The
// embedded outcome revision is patched to the committed revision right
// before commit (see setRequestOutcomeRevision).
func recordRequest(store *RuntimeStore, requestID, operation, digest string, outcome any) {
	if requestID == "" {
		return
	}
	payload, _ := json.Marshal(outcome)
	if store.Requests == nil {
		store.Requests = map[string]RuntimeRequestRecord{}
	}
	store.Requests[requestID] = RuntimeRequestRecord{
		Operation:  operation,
		Digest:     digest,
		Outcome:    payload,
		RecordedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// ─── Status ──────────────────────────────────────────────────────────────────

// SettleObligation describes what a passing settle will owe for the
// current ledger head. It mirrors gentle-ai's runtimeSettleObligation
// derivation: the unremediated failed evidence that a future passing
// settle must name via RemediatesEvidenceRevision.
type SettleObligation struct {
	EvidenceRevision           string `json:"evidence_revision"`
	RemediatesEvidenceRevision string `json:"remediates_evidence_revision,omitempty"`
}

// RuntimeStatus is the public-facing status response.
type RuntimeStatus struct {
	ChangeName             string `json:"change_name"`
	Revision               string `json:"revision"`
	ActiveAttempt          int    `json:"active_attempt"`
	DecisionRequired       bool   `json:"decision_required"`
	Complete               bool   `json:"complete"`
	NextAction             string `json:"next_action"`
	AttemptCount           int    `json:"attempt_count"`
	CumulativeChangedLines int    `json:"cumulative_changed_lines,omitempty"`

	// Generation is the DERIVED live objective generation: the wire
	// represents generation 1 by field absence, so readers render 0 as 1.
	// LifetimeAttempts, LifetimeChangedLines and LastAdvance are read-time
	// projections of the preserved attempt chain and the advance provenance —
	// none of them is ever persisted.
	Generation           int             `json:"generation,omitempty"`
	LifetimeAttempts     int             `json:"lifetime_attempts,omitempty"`
	LifetimeChangedLines int             `json:"lifetime_changed_lines,omitempty"`
	LastAdvance          *RuntimeAdvance `json:"last_advance,omitempty"`

	// Migrated is true when this access imported the legacy home-dir
	// ledger into the clone-scoped store (reported once).
	Migrated bool `json:"migrated,omitempty"`

	// Scope is the ledger scope: "clone" (git common dir) or "machine"
	// (no-git home fallback, see ScopeMachine).
	Scope string `json:"scope,omitempty"`

	// GrantedRoots projects the canonical granted edit roots for the
	// change-instance identity the read declared (see StatusWithInstance).
	// omitempty is load-bearing: grant-free chains and undeclared reads
	// serialize byte-identically to before the field existed.
	GrantedRoots []string `json:"granted_roots,omitempty"`

	// Optional binding info (only when a binding revision is set)
	BindingRevision  string `json:"binding_revision,omitempty"`
	BindingLineage   string `json:"binding_lineage,omitempty"`
	EvidenceRevision string `json:"evidence_revision,omitempty"`

	// Admission probe: answers "would this acquire be admitted?" so
	// verify/archive never get stranded by a stale decision-required.
	// Only StatusWithInstance populates them; plain reads remain empty
	// for backward compatibility, matching gentle-ai's AdmissionStatus
	// contract.
	BlockedReason    string            `json:"blocked_reason,omitempty"`
	BlockedExit      string            `json:"blocked_exit,omitempty"`
	SettleObligation *SettleObligation `json:"settle_obligation,omitempty"`
}

// Status reads the runtime ledger and returns a status response.
// Returns an empty status (NextAction="begin") if no ledger exists yet.
func Status(changeName, repoRoot string) (*RuntimeStatus, error) {
	return StatusWithInstance(changeName, repoRoot, "")
}

// StatusWithInstance reads the runtime ledger like Status, scoped to one
// change-instance identity: granted roots are projected only when they were
// recorded for exactly this instance, in grant order, deduplicated. An empty
// instance (a reader that declared no identity) behaves exactly like today's
// Status: no granted roots are projected and the response serializes
// byte-identically.
func StatusWithInstance(changeName, repoRoot, instance string) (*RuntimeStatus, error) {
	s, err := resolveStore(changeName, repoRoot)
	if err != nil {
		return nil, err
	}
	var status *RuntimeStatus
	err = s.withStoreLock(func() error {
		store, migrated, err := s.replay()
		if err != nil {
			return err
		}
		if store == nil {
			status = &RuntimeStatus{
				ChangeName: changeName,
				NextAction: "begin",
				Migrated:   false,
				Scope:      s.Scope,
			}
			return nil
		}
		status = &RuntimeStatus{
			ChangeName:       store.ChangeName,
			Revision:         store.Revision,
			ActiveAttempt:    store.ActiveAttempt,
			DecisionRequired: store.DecisionRequired,
			Complete:         store.Complete,
			NextAction:       deriveNextAction(store),
			AttemptCount:     len(store.Attempts),

			CumulativeChangedLines: store.CumulativeChangedLines,
			Migrated:               migrated,
			Scope:                  s.Scope,
			GrantedRoots:           grantedRootsFor(store, instance),
			BindingRevision:        store.BindingRevision,
			BindingLineage:         store.BindingLineage,
			EvidenceRevision:       store.EvidenceRevision,
			Generation:             derivedGeneration(store),
			LifetimeAttempts:       len(store.Attempts),
			LifetimeChangedLines:   lifetimeChangedLines(store),
			LastAdvance:            lastAdvanceEntry(store),
		}
		// Admission probe: would an acquire be admitted against this ledger
		// head? Populates BlockedReason/BlockedExit/SettleObligation so
		// sdd status can free verify/archive from a stale decision-required,
		// mirroring gentle-ai's AdmissionStatus / runtimeReadiness. The
		// classification comes exclusively from deriveScopeAdmission.
		if decision := deriveScopeAdmission(store, ScopeRequest{}); decision.Reason != "" {
			status.BlockedReason = decision.Reason
			status.BlockedExit = decision.Exit
			status.SettleObligation = deriveSettleObligation(store)
		} else {
			// Even when not blocked, expose the settle obligation so a
			// successful acquire's caller knows what its passing settle will
			// owe (gentle-ai's SettleObligation is always derived from the
			// chain, not only when blocked).
			if obligation := deriveSettleObligation(store); obligation != nil {
				status.SettleObligation = obligation
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return status, nil
}

func deriveNextAction(store *RuntimeStore) string {
	if store.Complete {
		return "complete"
	}
	if store.DecisionRequired {
		return "decision-required"
	}
	if store.ActiveAttempt > 0 {
		// Check if the active attempt has an outcome (needs finish)
		for i := len(store.Attempts) - 1; i >= 0; i-- {
			if store.Attempts[i].Ordinal == store.ActiveAttempt {
				if store.Attempts[i].Outcome == "" {
					return "continue"
				}
			}
		}
		return "finish"
	}
	return "begin"
}

// deriveSettleObligation returns the unremediated failed evidence the chain
// still holds, if any. It mirrors gentle-ai's runtimeSettleObligation: the
// last failed attempt whose evidence has not been discharged by a later
// passing correction that names it via RemediatesEvidenceRevision.
func deriveSettleObligation(store *RuntimeStore) *SettleObligation {
	if store == nil || len(store.Attempts) == 0 {
		return nil
	}
	// Find the most recent failed attempt with evidence that is still
	// unremediated.
	for i := len(store.Attempts) - 1; i >= 0; i-- {
		attempt := store.Attempts[i]
		if attempt.Outcome != "failed" || attempt.EvidenceRevision == "" {
			continue
		}
		remediated := false
		for j := i + 1; j < len(store.Attempts); j++ {
			later := store.Attempts[j]
			if later.Outcome == "passed" && later.RemediatesEvidenceRevision == attempt.EvidenceRevision {
				remediated = true
				break
			}
		}
		if !remediated {
			return &SettleObligation{
				EvidenceRevision:           attempt.EvidenceRevision,
				RemediatesEvidenceRevision: attempt.EvidenceRevision,
			}
		}
	}
	return nil
}

// ScopeRequest is the scope an admission wants to open or continue: the
// work unit and evidence goal plus the budget the caller requests. It gives
// Acquire, Begin and the status probe one admission shape, so the
// successor-advance branch (a later slice) has a single place to compare
// the requested scope against the live objective.
type ScopeRequest struct {
	WorkUnit     string
	EvidenceGoal string
	MaxAttempts  int
	MaxLines     int
}

// ScopeDecision is the single admission verdict. An empty Reason admits;
// Advance reports that an admission opens a successor generation over a
// completed objective. When blocked, Reason and Exit are exactly what
// BlockedError and the status probe project.
type ScopeDecision struct {
	Advance bool
	Reason  string
	Exit    string
}

// deriveScopeAdmission is the SINGLE owner of scope admission: it answers
// "would this acquire/begin be admitted?" for the current ledger head and is
// the one classification consumed by Acquire, Begin and the
// StatusWithInstance probe.
//
// Classification order:
//   - complete: a passed objective hands the change to a successor that
//     names a DIFFERENT work unit (Advance); repeating the settled work unit
//     — or probing it passively with no work unit named — is refused with
//     work_unit_complete and the successor route, so the status projection
//     tells the same story as the acquire path. A completion that is
//     genuinely anomalous (decision-required, a dangling active attempt, or
//     no passed attempt to name) stays corrupt_authority;
//   - decision-required: budget_exhausted (advance never launders it);
//   - an open attempt: active_attempt.
//
// The unified scope guard over the four scope fields is enforced by the
// compact acquire path (scopeChangeRefusal), not here: begin records the
// scope the caller declares, which is the behavior its callers rely on.
func deriveScopeAdmission(store *RuntimeStore, req ScopeRequest) ScopeDecision {
	if store == nil {
		return ScopeDecision{}
	}
	if store.Complete {
		// Completion is scoped to one work unit: a passed objective is
		// terminal for its own scope while remaining an ordinary predecessor
		// for the distinct work unit the SDD graph still owes. The passive
		// probe (a request that names no work unit) tells the SAME story as
		// the acquire path — the work unit is complete and continues through
		// a successor — so status never claims a corrupt authority nor offers
		// reset as the only exit. corrupt_authority stays reserved for a
		// genuinely anomalous completion: decision-required, a dangling
		// active attempt, or no passed attempt to name.
		if !store.DecisionRequired && store.ActiveAttempt == 0 && lastAttemptPassed(store) {
			if req.WorkUnit != "" && req.WorkUnit != store.WorkUnit {
				return ScopeDecision{Advance: true}
			}
			return ScopeDecision{Reason: BlockedReasonWorkUnitComplete, Exit: workUnitCompleteExit(store.WorkUnit)}
		}
		return ScopeDecision{Reason: BlockedReasonCorruptAuthority, Exit: "ledger is complete; reset required to continue"}
	}
	if store.DecisionRequired {
		// Minimal budget-exhausted mapping: decision-required with
		// attempts at or beyond the objective's budget is the stale
		// decision that verify/archive should be freed from when the
		// probe reports a settle obligation.
		if store.MaxAttempts > 0 && len(liveGenerationAttempts(store)) >= store.MaxAttempts {
			return ScopeDecision{Reason: BlockedReasonBudgetExhausted, Exit: "attempt budget exhausted; decision required" + budgetRecoveryHint}
		}
		return ScopeDecision{Reason: BlockedReasonBudgetExhausted, Exit: "decision required; run sdd-attempt reset to continue"}
	}
	if store.ActiveAttempt > 0 && slices.IndexFunc(store.Attempts, func(a RuntimeAttempt) bool {
		return a.Ordinal == store.ActiveAttempt && a.Outcome == ""
	}) >= 0 {
		return ScopeDecision{Reason: BlockedReasonActiveAttempt, Exit: "an attempt is already active; settle it before acquiring"}
	}
	return ScopeDecision{}
}

// workUnitCompleteExit is the exact refusal for repeating the work unit a
// passed settle already completed: it names the successor route (a different
// --work-unit) instead of pointing at reset as the only way out.
func workUnitCompleteExit(workUnit string) string {
	return fmt.Sprintf("work unit %q is complete; it continues through a successor work unit: run `biggz sdd-attempt acquire <change> --work-unit \"<a different label>\" --request-id \"<unique-id>\"` with a different --work-unit; reset discards this scope instead of succeeding it", workUnit)
}

// scopeChangeRefusal is the ONE unified scope guard over the four scope
// fields: while an objective is open, changing --work-unit, --evidence-goal,
// --max-attempts or --max-lines without an explicit reset is an invalid
// continuation. An empty request value means "unchanged", and a zero stored
// value records no scope to preserve. It replaces the two narrow guards the
// acquire path carried (one field each, silently incomplete).
//
// The budget fields are compared only while the live generation holds at
// least one attempt: an objective that has spent nothing yet — the budget
// epoch a reset just opened, a successor the advance just opened, or a
// pre-attempt ledger — has no consumed scope to preserve, so its opening
// acquire DECLARES the budget instead of inheriting it. Refusing it there
// told a caller that had just reset to reset. The work unit and evidence
// goal guards stay unconditional: a live objective's label is never cleared
// while its attempts exist.
func scopeChangeRefusal(store *RuntimeStore, req ScopeRequest) (reason, exit string) {
	objectiveOpen := len(liveGenerationAttempts(store)) > 0
	switch {
	case req.WorkUnit != "" && store.WorkUnit != "" && req.WorkUnit != store.WorkUnit:
		return BlockedReasonInvalidContinuation, fmt.Sprintf("work unit scope changed without reset: have %q, acquire wants %q", store.WorkUnit, req.WorkUnit)
	case req.EvidenceGoal != "" && store.EvidenceGoal != "" && req.EvidenceGoal != store.EvidenceGoal:
		return BlockedReasonInvalidContinuation, fmt.Sprintf("evidence goal changed without reset: have %q, acquire wants %q", store.EvidenceGoal, req.EvidenceGoal)
	case objectiveOpen && req.MaxAttempts > 0 && store.MaxAttempts > 0 && req.MaxAttempts != store.MaxAttempts:
		return BlockedReasonInvalidContinuation, fmt.Sprintf("attempt budget changed without reset: have %d, acquire wants %d", store.MaxAttempts, req.MaxAttempts)
	case objectiveOpen && req.MaxLines > 0 && store.MaxLines > 0 && req.MaxLines != store.MaxLines:
		return BlockedReasonInvalidContinuation, fmt.Sprintf("line budget changed without reset: have %d, acquire wants %d", store.MaxLines, req.MaxLines)
	}
	return "", ""
}

// applyScopeAdvance records the admission of a successor objective over a
// completed one: the live scope becomes the request's (its own budget, with
// the predecessor's only as the fallback when the request omits it), the
// generation advances with one provenance entry, and every prior attempt
// stays untouched. Completion, the decision, the active pointer and the live
// evidence are cleared so the caller can open the successor's first attempt;
// the accumulator restarts while the lifetime view keeps the whole chain.
// The caller appends the successor's attempt and sets the next action.
func applyScopeAdvance(store *RuntimeStore, req ScopeRequest, requestID string) {
	fromGeneration := derivedGeneration(store)
	toGeneration := fromGeneration + 1
	if req.MaxAttempts > 0 {
		store.MaxAttempts = req.MaxAttempts
	}
	if req.MaxLines > 0 {
		store.MaxLines = req.MaxLines
	}
	store.Advances = append(store.Advances, RuntimeAdvance{
		RequestID:      requestID,
		FromWorkUnit:   store.WorkUnit,
		ToWorkUnit:     req.WorkUnit,
		FromGeneration: fromGeneration,
		ToGeneration:   toGeneration,
		MaxAttempts:    store.MaxAttempts,
		MaxLines:       store.MaxLines,
		PrevRevision:   store.Revision,
		AdvancedAt:     time.Now().UTC().Format(time.RFC3339),
	})
	store.Generation = toGeneration
	store.WorkUnit = req.WorkUnit
	if req.EvidenceGoal != "" {
		store.EvidenceGoal = req.EvidenceGoal
	}
	store.Complete = false
	store.DecisionRequired = false
	store.ActiveAttempt = 0
	store.CumulativeChangedLines = 0
	store.EvidenceRevision = ""
	store.BindingRevision = ""
	store.BindingLineage = ""
	store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

// isStaleDecisionRequired reports whether a ledger that is in
// decision-required should no longer block verify/archive because the probe
// says the block is stale (corrupt_authority or budget_exhausted with a
// settle obligation), mirroring gentle-ai's applyNativeRuntimeRouting.
func isStaleDecisionRequired(status *RuntimeStatus) bool {
	if status == nil {
		return false
	}
	if !status.DecisionRequired {
		return false
	}
	if status.BlockedReason != BlockedReasonBudgetExhausted && status.BlockedReason != BlockedReasonCorruptAuthority {
		return false
	}
	return status.SettleObligation != nil && status.SettleObligation.EvidenceRevision != ""
}

// BlockedError is returned by Acquire when the ledger would refuse the
// request. It carries the same BlockedReason/BlockedExit/SettleObligation
// that StatusWithInstance projects, so callers can route without re-reading.
type BlockedError struct {
	Reason           string
	Exit             string
	SettleObligation *SettleObligation
}

func (e *BlockedError) Error() string {
	if e.Exit != "" {
		return fmt.Sprintf("blocked(%s): %s", e.Reason, e.Exit)
	}
	return fmt.Sprintf("blocked(%s)", e.Reason)
}

func (e *BlockedError) Unwrap() error { return ErrBlocked }

// IsBlocked reports whether err is a BlockedError.
func IsBlocked(err error) bool {
	var blocked *BlockedError
	return errors.As(err, &blocked)
}

// ErrBlocked is the sentinel for blocked ledger state.
var ErrBlocked = errors.New("runtime ledger blocked")

// RemediationComplete reports whether the ledger's last attempt is a passed
// correction of exactly the given failed evidence revision: the attempt was
// finished as passed and its --remediates-evidence-revision binding names
// the failed evidence the status derives. When true, the status clears its
// remediation state so dependencies route Verify → ready and next →
// verify. The instance parameter is accepted for caller symmetry with
// StatusWithInstance; the immutable attempt chain is change-scoped, so it
// does not affect the answer. A nil/missing ledger or an empty evidence
// revision never completes remediation.
func RemediationComplete(changeName, repoRoot, instance, evidenceRevision string) bool {
	if evidenceRevision == "" {
		return false
	}
	store, err := LoadStore(changeName, repoRoot)
	if err != nil || len(store.Attempts) == 0 {
		return false
	}
	last := store.Attempts[len(store.Attempts)-1]
	return last.Outcome == "passed" && last.RemediatesEvidenceRevision == evidenceRevision
}

// ─── Begin ───────────────────────────────────────────────────────────────────

// BeginParams define a new attempt.
type BeginParams struct {
	ChangeName   string
	RepoRoot     string
	ExpectedRev  string // CAS: expected current revision (empty if new or force)
	ObjectiveID  string
	WorkUnit     string
	EvidenceGoal string
	MaxAttempts  int
	MaxLines     int
	ChangedLines int    `json:"changed_lines,omitempty"`
	RequestID    string // idempotency key: if the same request_id is already recorded, return it
}

// BeginResult describes the result of beginning an attempt.
type BeginResult struct {
	Revision      string `json:"revision"`
	ActiveAttempt int    `json:"active_attempt"`
	AlreadyActive bool   `json:"already_active,omitempty"`
	Migrated      bool   `json:"migrated,omitempty"`
	Scope         string `json:"scope,omitempty"`
}

// Begin starts a new attempt on the change. Returns the attempt ordinal.
// CAS-guarded: if ExpectedRev doesn't match the current revision, the
// operation fails with a conflict error.
//
// When a RequestID is supplied, the operation is idempotent: if the ledger
// already holds a receipt for (change, request-id), the recorded outcome of
// the first application is returned without mutating the ledger — a replay
// is convergent even after an intervening different operation. A request ID
// reused with different inputs fails. When no RequestID is supplied the
// ledger behaves exactly as before.
func Begin(params BeginParams) (*BeginResult, error) {
	if params.RequestID != "" && !requestIDPattern.MatchString(params.RequestID) {
		return nil, errors.New("request_id must be a canonical lowercase identifier")
	}
	digest := requestDigest(requestDigestDomainBegin, params)

	s, err := resolveStore(params.ChangeName, params.RepoRoot)
	if err != nil {
		return nil, err
	}

	var result *BeginResult
	var migrated bool
	err = s.withStoreLock(func() error {
		loaded, mig, err := s.replay()
		if err != nil {
			return err
		}
		migrated = mig

		// First access: create a fresh store with attempt 1 already active
		// (the record's content address is its revision).
		if loaded == nil {
			if runtimeChangedLineBudgetExceeded(&RuntimeStore{MaxLines: params.MaxLines, CumulativeChangedLines: 0}, params.ChangedLines) {
				return &BlockedError{Reason: BlockedReasonBudgetExhausted, Exit: fmt.Sprintf("changed lines %d would exceed budget %d (cumulative %d)", params.ChangedLines, params.MaxLines, 0), SettleObligation: nil}
			}
			store := &RuntimeStore{
				ChangeName:    params.ChangeName,
				ObjectiveID:   params.ObjectiveID,
				MaxAttempts:   params.MaxAttempts,
				MaxLines:      params.MaxLines,
				WorkUnit:      params.WorkUnit,
				EvidenceGoal:  params.EvidenceGoal,
				CreatedAt:     time.Now().UTC().Format(time.RFC3339),
				UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
				ActiveAttempt: 1,
				NextAction:    "begin",
				Attempts: []RuntimeAttempt{{
					Ordinal:     1,
					ObjectiveID: params.ObjectiveID,
					WorkUnit:    params.WorkUnit,
					BeganAt:     time.Now().UTC().Format(time.RFC3339),
				}},
			}
			outcome := &BeginResult{ActiveAttempt: 1, Scope: s.Scope}
			recordRequest(store, params.RequestID, opBegin, digest, outcome)
			if params.RequestID != "" {
				setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
			}
			if err := s.commit(store); err != nil {
				return fmt.Errorf("save store: %w", err)
			}
			result = &BeginResult{Revision: store.Revision, ActiveAttempt: 1, Scope: s.Scope}
			return nil
		}
		store := loaded

		// Idempotent replay: the request ID already applied once. Return the
		// recorded outcome for that request without touching the current state.
		if params.RequestID != "" && store.Requests != nil {
			if record, exists := store.Requests[params.RequestID]; exists {
				if record.Operation != opBegin || record.Digest != digest {
					return fmt.Errorf("request_id %q was reused with different inputs", params.RequestID)
				}
				var replayed BeginResult
				if err := json.Unmarshal(record.Outcome, &replayed); err != nil {
					return fmt.Errorf("replay request %q: %w", params.RequestID, err)
				}
				result = &replayed
				return nil
			}
		}

		// CAS check
		if params.ExpectedRev != "" && store.Revision != params.ExpectedRev {
			return fmt.Errorf("CAS conflict: expected revision %s, got %s", params.ExpectedRev, store.Revision)
		}

		// Idempotency: check if this request_id/ordinal is already recorded
		if store.ActiveAttempt > 0 {
			for i := len(store.Attempts) - 1; i >= 0; i-- {
				if store.Attempts[i].Ordinal == store.ActiveAttempt && store.Attempts[i].Outcome == "" {
					result = &BeginResult{
						Revision:      store.Revision,
						ActiveAttempt: store.ActiveAttempt,
						AlreadyActive: true,
						Scope:         s.Scope,
					}
					return nil
				}
			}
		}

		// Admission seam: the same single classification that gates Acquire
		// and the status probe gates Begin too. A completed objective that a
		// distinct work unit names opens its successor generation here exactly
		// as the compact acquire path does; a repeat or a genuinely anomalous
		// completion is refused with the seam's reason and exit.
		request := ScopeRequest{
			WorkUnit:     params.WorkUnit,
			EvidenceGoal: params.EvidenceGoal,
			MaxAttempts:  params.MaxAttempts,
			MaxLines:     params.MaxLines,
		}
		decision := deriveScopeAdmission(store, request)
		if decision.Reason != "" {
			return &BlockedError{Reason: decision.Reason, Exit: decision.Exit, SettleObligation: deriveSettleObligation(store)}
		}
		if decision.Advance {
			applyScopeAdvance(store, request, params.RequestID)
		}

		if runtimeChangedLineBudgetExceeded(store, params.ChangedLines) {
			return &BlockedError{Reason: BlockedReasonBudgetExhausted, Exit: fmt.Sprintf("changed lines %d would exceed budget %d (cumulative %d)", params.ChangedLines, store.MaxLines, store.CumulativeChangedLines), SettleObligation: deriveSettleObligation(store)}
		}
		// Budget and refund bookkeeping is scoped to the live generation, so
		// an advanced ledger starts its successor with a fresh allowance.
		live := liveGenerationAttempts(store)
		delivered := runtimeAttemptDeliveredIncrementSlice(live)
		refunded := runtimeRefundedAttempts(store)
		if store.MaxAttempts > 0 {
			if delivered >= store.MaxAttempts {
				if refunded >= store.MaxAttempts || len(live) >= 2*store.MaxAttempts {
					return &BlockedError{Reason: BlockedReasonBudgetExhausted, Exit: "attempt budget exhausted (2x cap with refunds)" + budgetRecoveryHint, SettleObligation: deriveSettleObligation(store)}
				}
				store.DecisionRequired = true
				store.NextAction = "decision-required"
				store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				if err := s.commit(store); err != nil {
					return fmt.Errorf("save store: %w", err)
				}
				return &BlockedError{Reason: BlockedReasonBudgetExhausted, Exit: "max attempts reached, decision required" + budgetRecoveryHint, SettleObligation: deriveSettleObligation(store)}
			}
			if len(live) >= 2*store.MaxAttempts {
				return &BlockedError{Reason: BlockedReasonBudgetExhausted, Exit: "attempt budget exhausted (2x cap)" + budgetRecoveryHint, SettleObligation: deriveSettleObligation(store)}
			}
		}
		// Start new attempt: derive ordinal from history when no active attempt (cumulative never reset).
		nextOrdinal := store.ActiveAttempt + 1
		if store.ActiveAttempt == 0 {
			nextOrdinal = len(store.Attempts) + 1
			if nextOrdinal == 1 && len(store.Attempts) > 0 {
				nextOrdinal = store.Attempts[len(store.Attempts)-1].Ordinal + 1
			}
		}

		store.ActiveAttempt = nextOrdinal
		store.NextAction = "continue"
		store.DecisionRequired = false
		store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		store.Attempts = append(store.Attempts, RuntimeAttempt{
			Ordinal:             nextOrdinal,
			ObjectiveID:         params.ObjectiveID,
			ObjectiveGeneration: store.Generation,
			WorkUnit:            params.WorkUnit,
			BeganAt:             time.Now().UTC().Format(time.RFC3339),
		})
		outcome := &BeginResult{ActiveAttempt: nextOrdinal, Scope: s.Scope}
		recordRequest(store, params.RequestID, opBegin, digest, outcome)
		if params.RequestID != "" {
			setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
		}
		if err := s.commit(store); err != nil {
			return fmt.Errorf("save store: %w", err)
		}
		result = &BeginResult{Revision: store.Revision, ActiveAttempt: nextOrdinal, Scope: s.Scope}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.Migrated = migrated
	result.Scope = s.Scope
	return result, nil
}

// ─── Finish ──────────────────────────────────────────────────────────────────

// FinishParams define the end of an attempt.
type FinishParams struct {
	ChangeName  string
	RepoRoot    string
	ExpectedRev string // CAS: expected current revision
	Outcome     string // "passed", "failed", "interrupted"
	RequestID   string // idempotency key: replaying the same request id returns the recorded result

	// Evidence
	EvidenceRevision string

	// Diagnostics
	Diagnosis          string
	HarnessDisposition string
	CleanupEvidence    string
	ProcessEvidence    string

	// Remediation binding (only for remediation paths)
	ExpectedBindingRevision    string
	SuccessorLineageID         string
	RemediatesEvidenceRevision string
	ChangedLines               int `json:"changed_lines,omitempty"`
}

// FinishResult describes the result of finishing an attempt.
type FinishResult struct {
	Revision          string `json:"revision"`
	RemainingAttempts int    `json:"remaining_attempts,omitempty"`
	DecisionRequired  bool   `json:"decision_required,omitempty"`
	Complete          bool   `json:"complete,omitempty"`
	Migrated          bool   `json:"migrated,omitempty"`
	Scope             string `json:"scope,omitempty"`
}

// Finish closes the current attempt.
// Returns a conflict error if ExpectedRev doesn't match.
//
// When a RequestID is supplied, the operation is idempotent: a replayed
// request ID returns the recorded outcome without mutating the ledger, even
// when the current state has moved on. A request ID reused with different
// inputs fails.
func Finish(params FinishParams) (*FinishResult, error) {
	if params.RequestID != "" && !requestIDPattern.MatchString(params.RequestID) {
		return nil, errors.New("request_id must be a canonical lowercase identifier")
	}
	digest := requestDigest(requestDigestDomainFinish, params)

	s, err := resolveStore(params.ChangeName, params.RepoRoot)
	if err != nil {
		return nil, err
	}

	var result *FinishResult
	var migrated bool
	err = s.withStoreLock(func() error {
		loaded, mig, err := s.replay()
		if err != nil {
			return err
		}
		migrated = mig
		if loaded == nil {
			return errors.New("no runtime ledger for this change — has sdd-attempt begin been run?")
		}
		store := loaded

		// Idempotent replay: the request ID already applied once. Return the
		// recorded outcome for that request without touching the current state.
		if params.RequestID != "" && store.Requests != nil {
			if record, exists := store.Requests[params.RequestID]; exists {
				if record.Operation != opFinish || record.Digest != digest {
					return fmt.Errorf("request_id %q was reused with different inputs", params.RequestID)
				}
				var replayed FinishResult
				if err := json.Unmarshal(record.Outcome, &replayed); err != nil {
					return fmt.Errorf("replay request %q: %w", params.RequestID, err)
				}
				result = &replayed
				return nil
			}
		}

		// CAS check
		if store.Revision != params.ExpectedRev {
			return fmt.Errorf("CAS conflict: expected revision %s, got %s", params.ExpectedRev, store.Revision)
		}

		if runtimeChangedLineBudgetExceeded(store, params.ChangedLines) {
			return &BlockedError{Reason: BlockedReasonBudgetExhausted, Exit: fmt.Sprintf("changed lines %d would exceed budget %d (cumulative %d)", params.ChangedLines, store.MaxLines, store.CumulativeChangedLines), SettleObligation: deriveSettleObligation(store)}
		}
		// Find active attempt
		var found bool
		for i := len(store.Attempts) - 1; i >= 0; i-- {
			if store.Attempts[i].Ordinal == store.ActiveAttempt {
				store.Attempts[i].Outcome = params.Outcome
				store.Attempts[i].EndedAt = time.Now().UTC().Format(time.RFC3339)
				store.Attempts[i].EvidenceRevision = params.EvidenceRevision
				store.Attempts[i].Diagnosis = params.Diagnosis
				store.Attempts[i].HarnessDisposition = params.HarnessDisposition
				store.Attempts[i].CleanupEvidence = params.CleanupEvidence
				store.Attempts[i].ProcessEvidence = params.ProcessEvidence
				store.Attempts[i].RemediatesEvidenceRevision = params.RemediatesEvidenceRevision
				store.Attempts[i].ChangedLines = params.ChangedLines

				// Update binding for remediation
				if params.ExpectedBindingRevision != "" {
					store.BindingRevision = params.ExpectedBindingRevision
				}
				if params.SuccessorLineageID != "" {
					store.BindingLineage = params.SuccessorLineageID
				}
				if params.RemediatesEvidenceRevision != "" {
					store.EvidenceRevision = params.RemediatesEvidenceRevision
				}

				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no active attempt %d found", store.ActiveAttempt)
		}
		store.CumulativeChangedLines += params.ChangedLines

		// Update state based on outcome
		store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		if params.Outcome == "passed" {
			store.Complete = true
			store.NextAction = "complete"
			store.ActiveAttempt = 0
		} else {
			// Failed or interrupted — check if more attempts allowed (refund-aware)
			store.ActiveAttempt = 0
			live := liveGenerationAttempts(store)
			delivered := runtimeAttemptDeliveredIncrementSlice(live)
			if store.MaxAttempts > 0 && (delivered >= store.MaxAttempts || len(live) >= 2*store.MaxAttempts) {
				store.DecisionRequired = true
				store.NextAction = "decision-required"
			} else {
				store.NextAction = "begin"
			}
		}

		delivered := runtimeAttemptDeliveredIncrementSlice(liveGenerationAttempts(store))
		remaining := store.MaxAttempts - delivered
		if remaining < 0 {
			remaining = 0
		}
		outcome := &FinishResult{
			RemainingAttempts: remaining,
			DecisionRequired:  store.DecisionRequired,
			Complete:          store.Complete,
			Scope:             s.Scope,
		}
		recordRequest(store, params.RequestID, opFinish, digest, outcome)
		if params.RequestID != "" {
			setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
		}
		if err := s.commit(store); err != nil {
			return fmt.Errorf("save store: %w", err)
		}

		result = &FinishResult{
			Revision:          store.Revision,
			RemainingAttempts: remaining,
			DecisionRequired:  store.DecisionRequired,
			Complete:          store.Complete,
			Scope:             s.Scope,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.Migrated = migrated
	result.Scope = s.Scope
	return result, nil
}

// ─── Reset ───────────────────────────────────────────────────────────────────

// ResetParams define a ledger reset.
type ResetParams struct {
	ChangeName  string
	RepoRoot    string
	ExpectedRev string // CAS: expected current revision
	RequestID   string // idempotency key: replaying the same request id returns the recorded result
	Reason      string
	ResetBy     string
	MaxAttempts int
	MaxLines    int
	ObjectiveID string
}

// ResetResult describes the result of resetting the ledger.
type ResetResult struct {
	Revision string `json:"revision"`
	// AttemptsPreserved is the attempt-chain length at the moment of the
	// reset: reset clears only the LIVE objective, so the chain — and its
	// count — survives as the audit. The field states what the operation
	// KEEPS, never what it destroys, and the printed CLI text says the same.
	AttemptsPreserved int    `json:"attempts_preserved"`
	NewStore          bool   `json:"new_store,omitempty"`
	Migrated          bool   `json:"migrated,omitempty"`
	Scope             string `json:"scope,omitempty"`
}

// Reset clears the LIVE objective — completion, the decision, the active
// attempt and its evidence, plus the live scope — so a new objective can
// open, while the attempt chain stays as the audit: attempts are never
// discarded, so len(Attempts) never decreases and every prior attempt keeps
// its outcome and evidence. The reset opens a FRESH budget epoch: it advances
// the generation, so the preserved attempts stop counting as live for the
// budget, the 2x cap, the refund allowance and the accumulator, while the
// lifetime views keep the whole chain. Requires an explicit maintainer scope
// decision. If change doesn't exist yet, creates a minimal fresh ledger.
//
// When a RequestID is supplied, the operation is idempotent: a replayed
// request ID returns the recorded outcome without mutating the ledger.
func Reset(params ResetParams) (*ResetResult, error) {
	if params.RequestID != "" && !requestIDPattern.MatchString(params.RequestID) {
		return nil, errors.New("request_id must be a canonical lowercase identifier")
	}
	digest := requestDigest(requestDigestDomainReset, params)

	s, err := resolveStore(params.ChangeName, params.RepoRoot)
	if err != nil {
		return nil, err
	}

	var result *ResetResult
	var migrated bool
	err = s.withStoreLock(func() error {
		loaded, mig, err := s.replay()
		if err != nil {
			return err
		}
		migrated = mig

		if loaded == nil {
			// Create fresh
			store := &RuntimeStore{
				ChangeName:  params.ChangeName,
				ObjectiveID: params.ObjectiveID,
				MaxAttempts: params.MaxAttempts,
				MaxLines:    params.MaxLines,
				CreatedAt:   time.Now().UTC().Format(time.RFC3339),
				UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
				NextAction:  "begin",
			}
			outcome := &ResetResult{NewStore: true, Scope: s.Scope}
			recordRequest(store, params.RequestID, opReset, digest, outcome)
			if params.RequestID != "" {
				setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
			}
			if err := s.commit(store); err != nil {
				return fmt.Errorf("save store: %w", err)
			}
			result = &ResetResult{Revision: store.Revision, NewStore: true, Scope: s.Scope}
			return nil
		}
		store := loaded

		// Idempotent replay: the request ID already applied once. Return the
		// recorded outcome for that request without touching the current state.
		if params.RequestID != "" && store.Requests != nil {
			if record, exists := store.Requests[params.RequestID]; exists {
				if record.Operation != opReset || record.Digest != digest {
					return fmt.Errorf("request_id %q was reused with different inputs", params.RequestID)
				}
				var replayed ResetResult
				if err := json.Unmarshal(record.Outcome, &replayed); err != nil {
					return fmt.Errorf("replay request %q: %w", params.RequestID, err)
				}
				result = &replayed
				return nil
			}
		}

		// CAS check
		if params.ExpectedRev != "" && store.Revision != params.ExpectedRev {
			return fmt.Errorf("CAS conflict: expected revision %s, got %s", params.ExpectedRev, store.Revision)
		}

		prevRev := store.Revision
		// The chain survives the reset untouched, so the recorded count is
		// the number of attempts PRESERVED, never the number cleared.
		preservedAttempts := len(store.Attempts)

		// The reset opens a FRESH budget epoch: advancing the generation makes
		// every preserved attempt stop counting as live for the budget, the 2x
		// cap, the refund allowance and the accumulator. Without the boundary
		// the closed generation's 2x cap would gate every later objective
		// forever, so the remedy the blocked exits advertise would not work.
		nextGeneration := derivedGeneration(store) + 1

		// Record reset in provenance, including the budget epoch it opened.
		store.Resets = append(store.Resets, RuntimeReset{
			Reason:       params.Reason,
			ResetBy:      params.ResetBy,
			ResetAt:      time.Now().UTC().Format(time.RFC3339),
			PrevRevision: prevRev,
			ToGeneration: nextGeneration,
		})

		// Reset clears the live objective and nothing else: the attempt chain
		// is the audit, so it survives untouched (count never decreases, every
		// attempt keeps its outcome and evidence). The generation advances so
		// the preserved attempts cannot charge the re-opened objective, and the
		// line accumulator restarts exactly as it does for a successor.
		store.Generation = nextGeneration
		store.CumulativeChangedLines = 0
		store.ActiveAttempt = 0
		store.DecisionRequired = false
		store.Complete = false
		store.NextAction = "begin"
		store.EvidenceRevision = ""
		store.BindingRevision = ""
		store.BindingLineage = ""
		store.WorkUnit = ""
		store.EvidenceGoal = ""

		// Update objective if provided
		if params.ObjectiveID != "" {
			store.ObjectiveID = params.ObjectiveID
		}
		if params.MaxAttempts > 0 {
			store.MaxAttempts = params.MaxAttempts
		}
		if params.MaxLines > 0 {
			store.MaxLines = params.MaxLines
		}

		store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		outcome := &ResetResult{AttemptsPreserved: preservedAttempts, Scope: s.Scope}
		recordRequest(store, params.RequestID, opReset, digest, outcome)
		if params.RequestID != "" {
			setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
		}
		if err := s.commit(store); err != nil {
			return fmt.Errorf("save store: %w", err)
		}

		result = &ResetResult{
			Revision:          store.Revision,
			AttemptsPreserved: preservedAttempts,
			Scope:             s.Scope,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.Migrated = migrated
	result.Scope = s.Scope
	return result, nil
}

// ─── Acquire / Settle (compact bounded path) ───────────────────────────────

// AcquireParams define a compact acquire request. They mirror BeginParams
// but are bounded to the token-continuation contract: the returned Token
// identifies exactly the attempt Settle must close.
type AcquireParams struct {
	ChangeName   string
	RepoRoot     string
	RequestID    string
	WorkUnit     string
	EvidenceGoal string
	MaxAttempts  int
	MaxLines     int
	ChangedLines int    `json:"changed_lines,omitempty"`
	Token        string // optional ownership proof: continue the active attempt that owns this token
	// RemediatesEvidenceRevision declares the failed evidence this acquire
	// intends to correct, so an unsatisfiable remediation can be refused
	// before it spends an attempt (mirrors gentle-ai's acquire-time
	// remediation_unsatisfiable check).
	RemediatesEvidenceRevision string
}

// AcquireResult is the bounded orchestration projection returned by Acquire.
type AcquireResult struct {
	Token    string `json:"token"`
	Revision string `json:"revision"`
	// SettleObligation mirrors gentle-ai's acquire SettleObligation: what a
	// passing settle of this token will already owe.
	SettleObligation *SettleObligation `json:"settle_obligation,omitempty"`
	Scope            string            `json:"scope,omitempty"`
	Migrated         bool              `json:"migrated,omitempty"`
}

// SettleParams close the attempt selected by Token. They mirror FinishParams
// but select by token, not by active ordinal.
type SettleParams struct {
	ChangeName                 string
	RepoRoot                   string
	Token                      string
	RequestID                  string
	Outcome                    string
	EvidenceRevision           string
	Diagnosis                  string
	HarnessDisposition         string
	CleanupEvidence            string
	ProcessEvidence            string
	RemediatesEvidenceRevision string
	ChangedLines               int `json:"changed_lines,omitempty"`
	// Strict restores lock semantics: unknown or non-active tokens block with
	// invalid_continuation instead of admitting with a warning. Default (false)
	// is admissible: the ledger logs faithfully without authorizing. Strict is
	// reserved for flows where mutual exclusion matters (prod apply).
	Strict bool
}

// SettleResult describes the result of settling an attempt.
type SettleResult struct {
	Revision          string `json:"revision"`
	RemainingAttempts int    `json:"remaining_attempts,omitempty"`
	DecisionRequired  bool   `json:"decision_required,omitempty"`
	Complete          bool   `json:"complete,omitempty"`
	Migrated          bool   `json:"migrated,omitempty"`
	Scope             string `json:"scope,omitempty"`
	// Warning is set when the settlement was admitted without a prior claim
	// (unknown token recorded as unclaimed attempt) or against a non-active
	// attempt. Empty on the strict happy path.
	Warning string `json:"warning,omitempty"`
}

// mintAcquireToken derives a deterministic token for an acquire. It is
// content-address independent (does not include the snapshot revision) so the
// token can be stored in Tokens without circular hashing, while remaining
// deterministic for a given request ID and ordinal so idempotent replays
// converge.
func mintAcquireToken(changeName, requestID string, ordinal int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|biggz-ai.acquire-token/v1", changeName, requestID, ordinal)))
	return "tok-" + hex.EncodeToString(h[:])[:24]
}

// Acquire claims one bounded attempt and mints a token that identifies that
// exact begin record for Settle. It shares the same snapshot store as
// Begin/Finish but with token-based idempotency, mirroring gentle-ai's
// requestDigestDomainAcquire/Settle pattern.
//
// When RequestID is supplied the operation is idempotent: replaying the same
// request ID with identical inputs returns the same token without mutating
// the ledger. A reused request ID with different inputs fails.
//
// Readiness is checked via deriveScopeAdmission (the minimal
// runtimeReadiness predicate): a completed work unit is continued by a
// distinct --work-unit (successor advance) and refused with
// BlockedReasonWorkUnitComplete when the settled label repeats; a
// decision-required ledger is blocked with BlockedReasonBudgetExhausted and
// the current SettleObligation; only a genuinely anomalous completion stays
// BlockedReasonCorruptAuthority. An already-active attempt is blocked with
// BlockedReasonActiveAttempt unless the caller presents the active token as
// ownership proof (gentle-ai's Token continuation).
func Acquire(params AcquireParams) (*AcquireResult, error) {
	if params.RequestID != "" && !requestIDPattern.MatchString(params.RequestID) {
		return nil, errors.New("request_id must be a canonical lowercase identifier")
	}
	if params.Token != "" {
		trimmed := strings.TrimSpace(params.Token)
		if trimmed == "" {
			return nil, errors.New("token must be non-empty when supplied")
		}
		params.Token = trimmed
	}
	if params.RemediatesEvidenceRevision != "" {
		// Minimal sha256:... shape validation, mirroring gentle-ai's
		// remediation revision check: must be sha256:<64 lowercase hex>.
		if !strings.HasPrefix(params.RemediatesEvidenceRevision, "sha256:") || len(params.RemediatesEvidenceRevision) != 7+64 {
			return nil, errors.New("remediates_evidence_revision must be sha256:<64 lowercase hex>")
		}
		for _, c := range params.RemediatesEvidenceRevision[7:] {
			if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
				return nil, errors.New("remediates_evidence_revision must be sha256:<64 lowercase hex>")
			}
		}
	}
	// Defaults mirror Begin/Finish CLI defaults (task constraint).
	if params.MaxAttempts == 0 {
		params.MaxAttempts = 3
	}
	if params.MaxLines == 0 {
		params.MaxLines = 400
	}
	digest := requestDigest(requestDigestDomainAcquire, params)

	s, err := resolveStore(params.ChangeName, params.RepoRoot)
	if err != nil {
		return nil, err
	}

	var result *AcquireResult
	var migrated bool
	err = s.withStoreLock(func() error {
		loaded, mig, err := s.replay()
		if err != nil {
			return err
		}
		migrated = mig

		// Idempotent replay: same request ID already applied.
		if params.RequestID != "" && loaded != nil && loaded.Requests != nil {
			if rec, exists := loaded.Requests[params.RequestID]; exists {
				if rec.Operation != opAcquire || rec.Digest != digest {
					return fmt.Errorf("request_id %q was reused with different inputs", params.RequestID)
				}
				var replayed AcquireResult
				if err := json.Unmarshal(rec.Outcome, &replayed); err != nil {
					return fmt.Errorf("replay request %q: %w", params.RequestID, err)
				}
				result = &replayed
				result.Migrated = migrated
				result.Scope = s.Scope
				return nil
			}
		}

		// Fresh ledger: create store with first attempt and mint token.
		if loaded == nil {
			if runtimeChangedLineBudgetExceeded(&RuntimeStore{MaxLines: params.MaxLines, CumulativeChangedLines: 0}, params.ChangedLines) {
				return &BlockedError{Reason: BlockedReasonBudgetExhausted, Exit: fmt.Sprintf("changed lines %d would exceed budget %d (cumulative %d)", params.ChangedLines, params.MaxLines, 0), SettleObligation: nil}
			}
			ordinal := 1
			token := mintAcquireToken(params.ChangeName, params.RequestID, ordinal)
			if params.RequestID == "" {
				// Without a request ID the token must still be unique; include
				// a time-derived component so concurrent empty-ID acquires
				// do not collide.
				h := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d", params.ChangeName, ordinal, time.Now().UnixNano())))
				token = "tok-" + hex.EncodeToString(h[:])[:24]
			}
			store := &RuntimeStore{
				ChangeName:    params.ChangeName,
				MaxAttempts:   params.MaxAttempts,
				MaxLines:      params.MaxLines,
				WorkUnit:      params.WorkUnit,
				EvidenceGoal:  params.EvidenceGoal,
				CreatedAt:     time.Now().UTC().Format(time.RFC3339),
				UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
				ActiveAttempt: ordinal,
				NextAction:    "continue",
				Attempts: []RuntimeAttempt{{
					Ordinal:      ordinal,
					WorkUnit:     params.WorkUnit,
					BeganAt:      time.Now().UTC().Format(time.RFC3339),
					ChangedLines: params.ChangedLines,
				}},
				Tokens: map[string]int{token: ordinal},
			}
			// Remediation satisfiability pre-check: if the acquire declares
			// a correction, the chain must still hold that failed evidence
			// unremediated (minimal failedEvidenceRemediationSettleable).
			if params.RemediatesEvidenceRevision != "" {
				// No failed evidence yet on a fresh ledger -> unsatisfiable.
				return &BlockedError{
					Reason: BlockedReasonInvalidContinuation,
					Exit:   "this acquire declares a correction for failed evidence the attempt chain does not hold unremediated",
				}
			}
			outcome := &AcquireResult{Token: token, SettleObligation: deriveSettleObligation(store), Scope: s.Scope}
			recordRequest(store, params.RequestID, opAcquire, digest, outcome)
			if params.RequestID != "" {
				setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
			}
			if err := s.commit(store); err != nil {
				return fmt.Errorf("save store: %w", err)
			}
			result = &AcquireResult{Token: token, Revision: store.Revision, SettleObligation: deriveSettleObligation(store), Scope: s.Scope, Migrated: migrated}
			if params.RequestID != "" {
				// Patch the embedded revision on the persisted outcome.
				// The canonical hash excludes it, so this does not invalidate
				// the committed address; the replay path returns the same
				// token/revision pair.
				store.Revision = result.Revision
			}
			return nil
		}
		store := loaded

		// Token continuation: presenting the active attempt's own token proves
		// ownership and short-circuits to proceed with zero mutation, matching
		// gentle-ai's runtimeReadiness PresentedToken path.
		if params.Token != "" && store.ActiveAttempt > 0 {
			if ordinal, ok := store.Tokens[params.Token]; ok && ordinal == store.ActiveAttempt {
				// Verify the active attempt is still running (outcome == "").
				for _, attempt := range store.Attempts {
					if attempt.Ordinal == store.ActiveAttempt && attempt.Outcome == "" {
						result = &AcquireResult{Token: params.Token, Revision: store.Revision, SettleObligation: deriveSettleObligation(store), Scope: s.Scope, Migrated: migrated}
						return nil
					}
				}
			}
			// Non-matching token that claims ownership but is not the active
			// token is still a block that names the real active token.
			if store.Tokens != nil {
				for tok, ord := range store.Tokens {
					if ord == store.ActiveAttempt {
						return &BlockedError{
							Reason:           BlockedReasonActiveAttempt,
							Exit:             fmt.Sprintf("a distinct attempt token %s is already active; settle it before acquiring (presented token %s does not match)", tok, params.Token),
							SettleObligation: deriveSettleObligation(store),
						}
					}
				}
			}
		}

		// Admission check: complete or decision-required or active-attempt,
		// classified by the single deriveScopeAdmission seam. An admitted
		// advance opens the successor generation FIRST, so every budget check
		// below measures the successor's own budget and accumulator.
		request := ScopeRequest{
			WorkUnit:     params.WorkUnit,
			EvidenceGoal: params.EvidenceGoal,
			MaxAttempts:  params.MaxAttempts,
			MaxLines:     params.MaxLines,
		}
		decision := deriveScopeAdmission(store, request)
		if decision.Reason != "" {
			return &BlockedError{Reason: decision.Reason, Exit: decision.Exit, SettleObligation: deriveSettleObligation(store)}
		}
		if decision.Advance {
			applyScopeAdvance(store, request, params.RequestID)
		} else if reason, exit := scopeChangeRefusal(store, request); reason != "" {
			// The one unified scope guard: an open objective admits only its
			// exact recorded scope (empty request values mean "unchanged").
			// A completed objective's change of work unit is not drift — it is
			// the successor advance handled above.
			return &BlockedError{Reason: reason, Exit: exit, SettleObligation: deriveSettleObligation(store)}
		}

		// Remediation satisfiability pre-check (acquire-time fail-fast,
		// mirrors gentle-ai's CompactBlockRemediationUnsatisfiable).
		if params.RemediatesEvidenceRevision != "" {
			chainHasFailed := false
			chainEvidence := ""
			for i := len(store.Attempts) - 1; i >= 0; i-- {
				a := store.Attempts[i]
				if a.Outcome == "failed" && a.EvidenceRevision != "" {
					remediated := false
					for j := i + 1; j < len(store.Attempts); j++ {
						later := store.Attempts[j]
						if later.Outcome == "passed" && later.RemediatesEvidenceRevision == a.EvidenceRevision {
							remediated = true
							break
						}
					}
					if !remediated {
						chainHasFailed = true
						chainEvidence = a.EvidenceRevision
						break
					}
				}
			}
			if !chainHasFailed || chainEvidence != params.RemediatesEvidenceRevision {
				return &BlockedError{
					Reason:           BlockedReasonInvalidContinuation,
					Exit:             fmt.Sprintf("this acquire declares a correction for failed evidence %s but the chain's unremediated failure is %q", params.RemediatesEvidenceRevision, chainEvidence),
					SettleObligation: deriveSettleObligation(store),
				}
			}
		}

		if runtimeChangedLineBudgetExceeded(store, params.ChangedLines) {
			return &BlockedError{Reason: BlockedReasonBudgetExhausted, Exit: fmt.Sprintf("changed lines %d would exceed budget %d (cumulative %d)", params.ChangedLines, store.MaxLines, store.CumulativeChangedLines), SettleObligation: deriveSettleObligation(store)}
		}
		// The 2x cap and the delivered budget belong to the LIVE generation:
		// a succeeded predecessor may neither block its successor nor load
		// its refunds against the successor's fresh allowance.
		live := liveGenerationAttempts(store)
		if store.MaxAttempts > 0 && len(live) >= 2*store.MaxAttempts {
			return &BlockedError{Reason: BlockedReasonBudgetExhausted, Exit: "attempt budget exhausted (2x cap)" + budgetRecoveryHint, SettleObligation: deriveSettleObligation(store)}
		}
		deliveredEarly := runtimeAttemptDeliveredIncrementSlice(live)
		if store.MaxAttempts > 0 && deliveredEarly >= store.MaxAttempts {
			// Check if refund cap already allows: if refunded < Max, we could still allow one more? But delivered at limit means budget exhausted regardless
			// For refund case, delivered would be < Max because refunded not counted, so this branch not taken
			store.DecisionRequired = true
			store.NextAction = "decision-required"
			store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := s.commit(store); err != nil {
				return fmt.Errorf("save store: %w", err)
			}
			return &BlockedError{Reason: BlockedReasonBudgetExhausted, Exit: "max attempts reached, decision required" + budgetRecoveryHint, SettleObligation: deriveSettleObligation(store)}
		}
		// Budget check already covered by deriveScopeAdmission, but also
		// enforce maxAttempts guard for the new attempt ordinal.
		nextOrdinal := store.ActiveAttempt + 1
		// For a store that had an active attempt previously closed, ActiveAttempt
		// may be 0 after a finish; the nextOrdinal should be len(Attempts)+1 in
		// that case. Derive from history length instead.
		if store.ActiveAttempt == 0 {
			nextOrdinal = len(store.Attempts) + 1
			if nextOrdinal == 1 && len(store.Attempts) > 0 {
				nextOrdinal = store.Attempts[len(store.Attempts)-1].Ordinal + 1
			}
		}
		// The objective-opening acquire declares the live budget: when the live
		// generation holds no attempt yet — the budget epoch a reset just
		// opened, a successor the advance just opened, or a pre-attempt ledger
		// — the request's budget becomes the objective's, exactly as the first
		// acquire of a ledger does. Once an attempt is spent, scopeChangeRefusal
		// owns the invariant and the stored budget stays untouched.
		if len(live) == 0 || store.MaxAttempts == 0 {
			store.MaxAttempts = params.MaxAttempts
		}
		if len(live) == 0 || store.MaxLines == 0 {
			store.MaxLines = params.MaxLines
		}

		// Mint token and start new attempt.
		ordinal := nextOrdinal
		token := mintAcquireToken(params.ChangeName, params.RequestID, ordinal)
		if params.RequestID == "" {
			h := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d", params.ChangeName, ordinal, time.Now().UnixNano())))
			token = "tok-" + hex.EncodeToString(h[:])[:24]
		}

		if store.Tokens == nil {
			store.Tokens = map[string]int{}
		}
		store.Tokens[token] = ordinal
		store.ActiveAttempt = ordinal
		store.NextAction = "continue"
		store.DecisionRequired = false
		store.Complete = false
		store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if params.WorkUnit != "" {
			store.WorkUnit = params.WorkUnit
		}
		if params.EvidenceGoal != "" {
			store.EvidenceGoal = params.EvidenceGoal
		}
		store.Attempts = append(store.Attempts, RuntimeAttempt{
			Ordinal:             ordinal,
			ObjectiveGeneration: store.Generation,
			WorkUnit:            params.WorkUnit,
			BeganAt:             time.Now().UTC().Format(time.RFC3339),
			ChangedLines:        params.ChangedLines,
		})
		outcome := &AcquireResult{Token: token, SettleObligation: deriveSettleObligation(store), Scope: s.Scope}
		recordRequest(store, params.RequestID, opAcquire, digest, outcome)
		if params.RequestID != "" {
			setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
		}
		if err := s.commit(store); err != nil {
			return fmt.Errorf("save store: %w", err)
		}
		result = &AcquireResult{Token: token, Revision: store.Revision, SettleObligation: deriveSettleObligation(store), Scope: s.Scope, Migrated: migrated}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.Migrated = migrated
	result.Scope = s.Scope
	return result, nil
}

// Settle closes the attempt selected by Token. It looks up token → ordinal,
// validates the ordinal is the live active attempt, and records the outcome
// with the same state derivation as Finish (Complete / DecisionRequired /
// NextAction). It is the bounded-path counterpart to Finish.
//
// Outcome "progress" is a non-terminal checkpoint for multi-unit scopes:
// it closes the attempt like a failure (NextAction "begin", ledger stays
// open) without completing the ledger, so ONE acquire scope can cover N
// work units with a single final settle(passed). "progress" consumes one
// delivered attempt of budget, exactly like "failed".
func Settle(params SettleParams) (*SettleResult, error) {
	if params.RequestID != "" && !requestIDPattern.MatchString(params.RequestID) {
		return nil, errors.New("request_id must be a canonical lowercase identifier")
	}
	if strings.TrimSpace(params.Token) == "" {
		return nil, errors.New("token is required for settle")
	}
	params.Token = strings.TrimSpace(params.Token)
	if params.Outcome == "" {
		return nil, errors.New("outcome is required for settle")
	}
	switch params.Outcome {
	case "passed", "failed", "interrupted", "progress":
	default:
		return nil, fmt.Errorf("invalid outcome %q; want passed, failed, interrupted, or progress (progress is settle-only)", params.Outcome)
	}
	if params.Outcome == "interrupted" && params.EvidenceRevision != "" {
		return nil, errors.New("interrupted attempts must omit evidence_revision")
	}
	if params.RemediatesEvidenceRevision != "" {
		if !strings.HasPrefix(params.RemediatesEvidenceRevision, "sha256:") || len(params.RemediatesEvidenceRevision) != 7+64 {
			return nil, errors.New("remediates_evidence_revision must be sha256:<64 lowercase hex>")
		}
		for _, c := range params.RemediatesEvidenceRevision[7:] {
			if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
				return nil, errors.New("remediates_evidence_revision must be sha256:<64 lowercase hex>")
			}
		}
	}
	digest := requestDigest(requestDigestDomainSettle, params)

	s, err := resolveStore(params.ChangeName, params.RepoRoot)
	if err != nil {
		return nil, err
	}

	var result *SettleResult
	var migrated bool
	err = s.withStoreLock(func() error {
		loaded, mig, err := s.replay()
		if err != nil {
			return err
		}
		migrated = mig
		if loaded == nil {
			return errors.New("no runtime ledger for this change — has sdd-attempt acquire been run?")
		}
		store := loaded

		// Idempotent replay: same request ID already applied.
		if params.RequestID != "" && store.Requests != nil {
			if rec, exists := store.Requests[params.RequestID]; exists {
				if rec.Operation != opSettle || rec.Digest != digest {
					return fmt.Errorf("request_id %q was reused with different inputs", params.RequestID)
				}
				var replayed SettleResult
				if err := json.Unmarshal(rec.Outcome, &replayed); err != nil {
					return fmt.Errorf("replay request %q: %w", params.RequestID, err)
				}
				result = &replayed
				result.Migrated = migrated
				result.Scope = s.Scope
				return nil
			}
		}

		// Lookup token → ordinal.
		// Admissible settle (default): an unresolvable token is recorded as an
		// unclaimed attempt with a warning instead of blocking with
		// invalid_continuation. Strict restores the old lock semantics.
		var admitWarning string
		ordinal, ok := store.Tokens[params.Token]
		if !ok {
			// Fallback: token may be the revision itself (legacy path where
			// token == revision). Accept it if it matches the current head's
			// active attempt.
			if params.Token == store.Revision && store.ActiveAttempt > 0 {
				ordinal = store.ActiveAttempt
				ok = true
			} else if params.Strict {
				return &BlockedError{
					Reason:           BlockedReasonInvalidContinuation,
					Exit:             fmt.Sprintf("token %q does not continue the attempt currently on record; run sdd-attempt status to see the live token", params.Token),
					SettleObligation: deriveSettleObligation(store),
				}
			} else {
				// No prior claim: append an unclaimed attempt so the work is
				// logged faithfully instead of rejected.
				ordinal = maxAttemptOrdinal(store.Attempts) + 1
				store.Attempts = append(store.Attempts, RuntimeAttempt{
					Ordinal:   ordinal,
					BeganAt:   time.Now().UTC().Format(time.RFC3339),
					Diagnosis: fmt.Sprintf("admitted without prior acquire (unclaimed token %q)", params.Token),
				})
				if store.Tokens == nil {
					store.Tokens = map[string]int{}
				}
				store.Tokens[params.Token] = ordinal
				admitWarning = fmt.Sprintf("settled without prior acquire (token %q unclaimed); recorded as attempt %d", params.Token, ordinal)
				ok = true
			}
		}

		// The ordinal must be the active attempt and still running.
		// Skip this check for freshly admitted unclaimed attempts (their
		// warning is already set above).
		if admitWarning == "" && store.ActiveAttempt != ordinal {
			if params.Strict {
				return &BlockedError{
					Reason:           BlockedReasonInvalidContinuation,
					Exit:             fmt.Sprintf("token %q maps to attempt %d but active attempt is %d", params.Token, ordinal, store.ActiveAttempt),
					SettleObligation: deriveSettleObligation(store),
				}
			}
			admitWarning = fmt.Sprintf("token %q maps to non-active attempt %d (active: %d); settled with supersede note", params.Token, ordinal, store.ActiveAttempt)
		}
		var found bool
		var idx int
		for i := len(store.Attempts) - 1; i >= 0; i-- {
			if store.Attempts[i].Ordinal == ordinal {
				if store.Attempts[i].Outcome != "" {
					return &BlockedError{
						Reason:           BlockedReasonInvalidContinuation,
						Exit:             fmt.Sprintf("attempt %d is already finished with outcome %q", ordinal, store.Attempts[i].Outcome),
						SettleObligation: deriveSettleObligation(store),
					}
				}
				found = true
				idx = i
				break
			}
		}
		if !found {
			return &BlockedError{
				Reason:           BlockedReasonInvalidContinuation,
				Exit:             fmt.Sprintf("no active attempt %d found for token %q", ordinal, params.Token),
				SettleObligation: deriveSettleObligation(store),
			}
		}

		// Validate remediation binding (mirrors Finish's evidence remediation
		// checks, minimal).
		chainFailedAttempt, chainHasFailedEvidence := findChainFailedAttempt(store.Attempts)
		chainFailedEvidence := ""
		if chainHasFailedEvidence {
			chainFailedEvidence = chainFailedAttempt.EvidenceRevision
		}
		if params.RemediatesEvidenceRevision != "" {
			if !chainHasFailedEvidence {
				return &BlockedError{
					Reason:           BlockedReasonInvalidContinuation,
					Exit:             fmt.Sprintf("this correction names failed verification %s, but the attempt chain records no failed verification", params.RemediatesEvidenceRevision),
					SettleObligation: deriveSettleObligation(store),
				}
			}
			if chainFailedEvidence != params.RemediatesEvidenceRevision {
				return &BlockedError{
					Reason:           BlockedReasonInvalidContinuation,
					Exit:             fmt.Sprintf("this correction names failed verification %s, but the chain's unremediated failure is %s", params.RemediatesEvidenceRevision, chainFailedEvidence),
					SettleObligation: deriveSettleObligation(store),
				}
			}
		}
		if params.Outcome == "passed" && chainHasFailedEvidence && params.RemediatesEvidenceRevision == "" {
			return &BlockedError{
				Reason:           BlockedReasonInvalidContinuation,
				Exit:             fmt.Sprintf("passing correction for failed verification %q requires --remediates-evidence-revision", chainFailedEvidence),
				SettleObligation: deriveSettleObligation(store),
			}
		}

		if runtimeChangedLineBudgetExceeded(store, params.ChangedLines) {
			return &BlockedError{Reason: BlockedReasonBudgetExhausted, Exit: fmt.Sprintf("changed lines %d would exceed budget %d (cumulative %d)", params.ChangedLines, store.MaxLines, store.CumulativeChangedLines), SettleObligation: deriveSettleObligation(store)}
		}
		// Apply settlement.
		store.Attempts[idx].Outcome = params.Outcome
		store.Attempts[idx].EndedAt = time.Now().UTC().Format(time.RFC3339)
		store.Attempts[idx].EvidenceRevision = params.EvidenceRevision
		store.Attempts[idx].Diagnosis = params.Diagnosis
		store.Attempts[idx].HarnessDisposition = params.HarnessDisposition
		store.Attempts[idx].CleanupEvidence = params.CleanupEvidence
		store.Attempts[idx].ProcessEvidence = params.ProcessEvidence
		store.Attempts[idx].RemediatesEvidenceRevision = params.RemediatesEvidenceRevision
		store.Attempts[idx].ChangedLines = params.ChangedLines
		store.CumulativeChangedLines += params.ChangedLines

		if params.RemediatesEvidenceRevision != "" {
			store.EvidenceRevision = params.RemediatesEvidenceRevision
		} else if params.EvidenceRevision != "" && params.Outcome == "failed" {
			store.EvidenceRevision = params.EvidenceRevision
		}

		store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		if params.Outcome == "passed" {
			store.Complete = true
			store.NextAction = "complete"
			store.ActiveAttempt = 0
		} else {
			live := liveGenerationAttempts(store)
			delivered := runtimeAttemptDeliveredIncrementSlice(live)
			if store.MaxAttempts > 0 && (delivered >= store.MaxAttempts || len(live) >= 2*store.MaxAttempts) {
				store.DecisionRequired = true
				store.NextAction = "decision-required"
			} else {
				store.NextAction = "begin"
				store.ActiveAttempt = 0
			}
		}

		delivered := runtimeAttemptDeliveredIncrementSlice(liveGenerationAttempts(store))
		remaining := store.MaxAttempts - delivered
		if remaining < 0 {
			remaining = 0
		}
		outcome := &SettleResult{
			RemainingAttempts: remaining,
			DecisionRequired:  store.DecisionRequired,
			Complete:          store.Complete,
			Scope:             s.Scope,
			Warning:           admitWarning,
		}
		recordRequest(store, params.RequestID, opSettle, digest, outcome)
		if params.RequestID != "" {
			setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
		}
		if err := s.commit(store); err != nil {
			return fmt.Errorf("save store: %w", err)
		}
		result = &SettleResult{
			Revision:          store.Revision,
			RemainingAttempts: remaining,
			DecisionRequired:  store.DecisionRequired,
			Complete:          store.Complete,
			Scope:             s.Scope,
			Migrated:          migrated,
			Warning:           admitWarning,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.Migrated = migrated
	result.Scope = s.Scope
	return result, nil
}

// maxAttemptOrdinal returns the highest attempt ordinal on record (0 when empty).
func maxAttemptOrdinal(attempts []RuntimeAttempt) int {
	max := 0
	for _, a := range attempts {
		if a.Ordinal > max {
			max = a.Ordinal
		}
	}
	return max
}

// findChainFailedAttempt mirrors gentle-ai's runtimeChainFailedAttempt: the
// most recent unremediated failed attempt, if any.
func findChainFailedAttempt(attempts []RuntimeAttempt) (RuntimeAttempt, bool) {
	for i := len(attempts) - 1; i >= 0; i-- {
		a := attempts[i]
		if a.Outcome == "failed" && a.EvidenceRevision != "" {
			remediated := false
			for j := i + 1; j < len(attempts); j++ {
				later := attempts[j]
				if later.Outcome == "passed" && later.RemediatesEvidenceRevision == a.EvidenceRevision {
					remediated = true
					break
				}
			}
			if !remediated {
				return a, true
			}
		}
	}
	return RuntimeAttempt{}, false
}

// ─── Grant ───────────────────────────────────────────────────────────────────

// maximumGrantRoots bounds one grant's root list.
const maximumGrantRoots = 32

// GrantParams record a per-change edit-authority grant: maintainer-authorized
// permission for this change's apply actor to edit the named repository
// roots. Roots are canonicalized (absolute, symlink-evaluated) before the
// request digest is computed, so the digest binds the exact identities the
// record carries. ChangeInstance is required and is the single source of the
// grant's instance identity — the CLI always passes the persisted change
// marker token.
type GrantParams struct {
	ChangeName     string
	RepoRoot       string
	ExpectedRev    string // CAS: expected current revision; empty on a fresh pre-attempt ledger
	Roots          []string
	Reason         string
	Actor          string
	RequestID      string // idempotency key: replaying the same request id returns the recorded result
	ChangeInstance string // required; the change-instance identity this grant authorizes
}

// GrantResult describes the result of a grant.
type GrantResult struct {
	Revision     string   `json:"revision"`
	Scope        string   `json:"scope,omitempty"`
	GrantedRoots []string `json:"granted_roots,omitempty"`
	Migrated     bool     `json:"migrated,omitempty"`
}

// Grant records a per-change edit-authority grant and commits a new snapshot
// with the grant appended to the audit history. It has NO structural
// precondition: authorizing roots is orthogonal to attempt state, and on a
// fresh pre-attempt ledger the grant creates the store exactly like Begin
// does. The CAS guard is re-checked as every sibling mutation: an
// ExpectedRev that does not match the current revision refuses.
//
// When a RequestID is supplied, the operation is idempotent: a replay
// returns the recorded outcome without mutating the ledger. A request ID
// reused with different inputs fails.
func Grant(params GrantParams) (*GrantResult, error) {
	if err := validateChangeInstance(params.ChangeInstance); err != nil {
		return nil, err
	}
	params, err := normalizeGrantRootsRequest(params)
	if err != nil {
		return nil, err
	}
	if params.RequestID != "" && !requestIDPattern.MatchString(params.RequestID) {
		return nil, errors.New("request_id must be a canonical lowercase identifier")
	}
	digest := requestDigest(requestDigestDomainGrant, params)
	grantedAt := grantClock()

	s, err := resolveStore(params.ChangeName, params.RepoRoot)
	if err != nil {
		return nil, err
	}

	var result *GrantResult
	var migrated bool
	err = s.withStoreLock(func() error {
		loaded, mig, err := s.replay()
		if err != nil {
			return err
		}
		migrated = mig

		// First access: create a fresh store carrying the grant (the record's
		// content address is its revision), exactly like Begin's fresh-store
		// path but with no attempt started.
		if loaded == nil {
			store := &RuntimeStore{
				ChangeName: params.ChangeName,
				CreatedAt:  time.Now().UTC().Format(time.RFC3339),
				UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
				NextAction: "begin",
				Grants: []RuntimeGrant{{
					Roots:     params.Roots,
					Actor:     params.Actor,
					Reason:    params.Reason,
					GrantedAt: grantedAt,
					Instance:  params.ChangeInstance,
				}},
			}
			outcome := &GrantResult{GrantedRoots: grantedRootsFor(store, params.ChangeInstance), Scope: s.Scope}
			recordRequest(store, params.RequestID, opGrant, digest, outcome)
			if params.RequestID != "" {
				setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
			}
			if err := s.commit(store); err != nil {
				return fmt.Errorf("save store: %w", err)
			}
			result = &GrantResult{
				Revision:     store.Revision,
				GrantedRoots: grantedRootsFor(store, params.ChangeInstance),
				Scope:        s.Scope,
			}
			return nil
		}
		store := loaded

		// Idempotent replay: the request ID already applied once. Return the
		// recorded outcome for that request without touching the current state.
		if params.RequestID != "" && store.Requests != nil {
			if record, exists := store.Requests[params.RequestID]; exists {
				if record.Operation != opGrant || record.Digest != digest {
					return fmt.Errorf("request_id %q was reused with different inputs", params.RequestID)
				}
				var replayed GrantResult
				if err := json.Unmarshal(record.Outcome, &replayed); err != nil {
					return fmt.Errorf("replay request %q: %w", params.RequestID, err)
				}
				result = &replayed
				return nil
			}
		}

		// CAS check
		if params.ExpectedRev != "" && store.Revision != params.ExpectedRev {
			return fmt.Errorf("CAS conflict: expected revision %s, got %s", params.ExpectedRev, store.Revision)
		}

		// Append the grant to the audit history and commit a new snapshot.
		store.Grants = append(store.Grants, RuntimeGrant{
			Roots:     params.Roots,
			Actor:     params.Actor,
			Reason:    params.Reason,
			GrantedAt: grantedAt,
			Instance:  params.ChangeInstance,
		})
		store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		outcome := &GrantResult{GrantedRoots: grantedRootsFor(store, params.ChangeInstance), Scope: s.Scope}
		recordRequest(store, params.RequestID, opGrant, digest, outcome)
		if params.RequestID != "" {
			setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
		}
		if err := s.commit(store); err != nil {
			return fmt.Errorf("save store: %w", err)
		}
		result = &GrantResult{
			Revision:     store.Revision,
			GrantedRoots: grantedRootsFor(store, params.ChangeInstance),
			Scope:        s.Scope,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.Migrated = migrated
	result.Scope = s.Scope
	return result, nil
}

// normalizeGrantRootsRequest mirrors the sibling operations' CAS/audit-field
// validation and canonicalizes every requested root: absolute, then
// symlink-evaluated, so a link and its target record one identity. Canonical
// duplicates collapse before the request digest is computed, keeping the
// digest identical to the event.
func normalizeGrantRootsRequest(params GrantParams) (GrantParams, error) {
	if len(params.Roots) < 1 || len(params.Roots) > maximumGrantRoots {
		return GrantParams{}, fmt.Errorf("grant requires between 1 and %d roots", maximumGrantRoots)
	}
	canonical := make([]string, 0, len(params.Roots))
	seen := make(map[string]struct{}, len(params.Roots))
	for _, root := range params.Roots {
		if err := validateBoundedText(root, 4096); err != nil {
			return GrantParams{}, fmt.Errorf("invalid grant root: %w", err)
		}
		resolved, err := filepath.Abs(root)
		if err == nil {
			resolved, err = filepath.EvalSymlinks(resolved)
		}
		if err != nil {
			return GrantParams{}, fmt.Errorf("resolve grant root %q: %w", root, err)
		}
		if err := validateBoundedText(resolved, 4096); err != nil {
			return GrantParams{}, fmt.Errorf("invalid canonical grant root: %w", err)
		}
		if _, duplicate := seen[resolved]; duplicate {
			continue
		}
		seen[resolved] = struct{}{}
		canonical = append(canonical, resolved)
	}
	params.Roots = canonical
	if err := validateBoundedText(params.Reason, 500); err != nil {
		return GrantParams{}, fmt.Errorf("invalid grant reason: %w", err)
	}
	if err := validateBoundedText(params.Actor, 128); err != nil {
		return GrantParams{}, fmt.Errorf("invalid grant actor: %w", err)
	}
	if err := validateChangeInstance(params.ChangeInstance); err != nil {
		return GrantParams{}, err
	}
	return params, nil
}

// ─── Store I/O ───────────────────────────────────────────────────────────────

// storeRootOverride redirects the ledger store root; used by tests to keep
// the ledger out of the real home directory and real git dirs. Empty means
// the default clone-scoped path.
var storeRootOverride = ""

// loadStore loads the current ledger state under the store lock, migrating
// the legacy home-dir ledger on first access. Returns os.ErrNotExist when no
// ledger exists.
func loadStore(changeName, repoRoot string) (*RuntimeStore, bool, error) {
	s, err := resolveStore(changeName, repoRoot)
	if err != nil {
		return nil, false, err
	}
	var store *RuntimeStore
	var migrated bool
	err = s.withStoreLock(func() error {
		loaded, mig, err := s.replay()
		if err != nil {
			return err
		}
		store, migrated = loaded, mig
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if store == nil {
		return nil, false, os.ErrNotExist
	}
	return store, migrated, nil
}

// saveStore persists a mutated ledger state under the store lock, refusing
// on a CAS mismatch against the revision the caller loaded.
func saveStore(store *RuntimeStore, repoRoot string) error {
	s, err := resolveStore(store.ChangeName, repoRoot)
	if err != nil {
		return err
	}
	return s.withStoreLock(func() error {
		current, _, err := s.replay()
		if err != nil {
			return err
		}
		if current != nil && store.Revision != current.Revision {
			return fmt.Errorf("CAS conflict: expected revision %s, got %s", store.Revision, current.Revision)
		}
		return s.commit(store)
	})
}

// LoadStore loads the runtime store from disk under the store lock (public
// version for other packages). Returns os.ErrNotExist when no ledger exists.
func LoadStore(changeName, repoRoot string) (*RuntimeStore, error) {
	store, _, err := loadStore(changeName, repoRoot)
	return store, err
}

// SaveStore persists a runtime store to disk (public version for other
// packages). The store's Revision must match the current ledger revision
// (CAS); it is overwritten with the committed revision on success.
func SaveStore(store *RuntimeStore, repoRoot string) error {
	return saveStore(store, repoRoot)
}

// computeRevision verifies the legacy single-file ledger's self-consistency
// (migration only): the legacy revision is the SHA-256 of the snapshot
// serialized without the revision field.
func computeRevision(store *RuntimeStore) string {
	rev := store.Revision
	store.Revision = ""
	data, _ := json.Marshal(store)
	store.Revision = rev
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// LedgerExists reports whether a ledger exists for the given change without
// creating any filesystem state. It checks both the clone-scoped HEAD and the
// legacy home-dir fallback, which is exactly what replay checks before
// creating anything.
func LedgerExists(changeName, repoRoot string) bool {
	s, err := resolveStore(changeName, repoRoot)
	if err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "HEAD")); err == nil {
		return true
	}
	if _, err := os.Stat(s.LegacyPath); err == nil {
		return true
	}
	return false
}

// ─── Rescope (cumulative-preserving narrow) ─────────────────────────────────

var (
	ErrRuntimeRescopeWidened    = errors.New("rescope widened budget")
	ErrRuntimeRescopeExhausted  = errors.New("rescope budget exhausted")
	ErrRuntimeRescopeNotAllowed = errors.New("rescope not allowed")
)

type RescopeParams struct {
	ChangeName   string
	RepoRoot     string
	ExpectedRev  string
	RequestID    string
	WorkUnit     string
	EvidenceGoal string
	MaxAttempts  int
	MaxLines     int
	Reason       string
	Actor        string
}

const requestDigestDomainRescope = "biggz-ai.sdd-runtime-rescope-request/v1"
const opRescope = "rescope"

func Rescope(params RescopeParams) (*ResetResult, error) {
	if params.RequestID != "" && !requestIDPattern.MatchString(params.RequestID) {
		return nil, errors.New("request_id must be a canonical lowercase identifier")
	}
	if err := validateBoundedText(params.Reason, 500); err != nil {
		return nil, fmt.Errorf("invalid rescope reason: %w", err)
	}
	if err := validateBoundedText(params.Actor, 128); err != nil {
		return nil, fmt.Errorf("invalid rescope actor: %w", err)
	}
	digest := requestDigest(requestDigestDomainRescope, params)
	s, err := resolveStore(params.ChangeName, params.RepoRoot)
	if err != nil {
		return nil, err
	}
	var result *ResetResult
	var migrated bool
	err = s.withStoreLock(func() error {
		loaded, mig, err := s.replay()
		if err != nil {
			return err
		}
		migrated = mig
		if loaded == nil {
			return fmt.Errorf("no runtime ledger for this change — has sdd-attempt been run?")
		}
		store := loaded
		if params.RequestID != "" && store.Requests != nil {
			if rec, exists := store.Requests[params.RequestID]; exists {
				if rec.Operation != opRescope || rec.Digest != digest {
					return fmt.Errorf("request_id %q was reused with different inputs", params.RequestID)
				}
				var replayed ResetResult
				if err := json.Unmarshal(rec.Outcome, &replayed); err != nil {
					return fmt.Errorf("replay request %q: %w", params.RequestID, err)
				}
				result = &replayed
				return nil
			}
		}
		if params.ExpectedRev != "" && store.Revision != params.ExpectedRev {
			return fmt.Errorf("CAS conflict: expected revision %s, got %s", params.ExpectedRev, store.Revision)
		}
		// Verbatim narrowing predicate: Active==0 && ObjectiveID!="" && !DecisionRequired && !Complete && len>0 && last.Outcome!="" && !drifted
		// Drift stub is false until CandidateTree is wired (TODO), so !drift is always true.
		if store.ActiveAttempt != 0 {
			return fmt.Errorf("%w: rescope requires no active attempt", ErrRuntimeRescopeNotAllowed)
		}
		if store.ObjectiveID == "" {
			return fmt.Errorf("%w: rescope requires non-empty objective_id", ErrRuntimeRescopeNotAllowed)
		}
		if store.DecisionRequired {
			return fmt.Errorf("%w: rescope blocked while decision required", ErrRuntimeRescopeNotAllowed)
		}
		if store.Complete {
			return fmt.Errorf("%w: rescope blocked when complete", ErrRuntimeRescopeNotAllowed)
		}
		if len(store.Attempts) == 0 {
			return fmt.Errorf("%w: rescope requires prior terminal attempt", ErrRuntimeRescopeNotAllowed)
		}
		last := store.Attempts[len(store.Attempts)-1]
		if last.Outcome == "" {
			return fmt.Errorf("%w: last attempt not finished", ErrRuntimeRescopeNotAllowed)
		}
		// Drift stub false: candidateTree not wired, treat as not drifted.
		drifted := false // TODO: wire candidateTree wiring for drift stub
		if drifted {
			return fmt.Errorf("%w: rescope blocked when drifted", ErrRuntimeRescopeNotAllowed)
		}
		oldMaxAttempts := store.MaxAttempts
		oldMaxLines := store.MaxLines
		newMaxAttempts := params.MaxAttempts
		if newMaxAttempts == 0 {
			newMaxAttempts = oldMaxAttempts
		}
		newMaxLines := params.MaxLines
		if newMaxLines == 0 {
			newMaxLines = oldMaxLines
		}
		cumAttempts := len(store.Attempts)
		cumLines := store.CumulativeChangedLines
		// Narrowing check first: new <= old => Widened
		if newMaxAttempts <= oldMaxAttempts || newMaxLines <= oldMaxLines {
			return fmt.Errorf("%w: rescope widened budget requires newMaxAttempts > oldMaxAttempts (%d) && newMaxLines > oldMaxLines (%d), got %d->%d attempts %d->%d lines", ErrRuntimeRescopeWidened, oldMaxAttempts, oldMaxLines, oldMaxAttempts, newMaxAttempts, oldMaxLines, newMaxLines)
		}
		// Wedge violation: new <= cumulative => Exhausted (must be after Widened)
		if newMaxAttempts <= cumAttempts || newMaxLines <= cumLines {
			return fmt.Errorf("%w: rescope wedge requires newMaxAttempts > cumAttempts (%d) && newMaxLines > cumLines (%d), got %d->%d attempts %d->%d lines", ErrRuntimeRescopeExhausted, cumAttempts, cumLines, oldMaxAttempts, newMaxAttempts, oldMaxLines, newMaxLines)
		}
		// Cumulative never reset — preserve attempts slice
		store.MaxAttempts = newMaxAttempts
		store.MaxLines = newMaxLines
		if params.WorkUnit != "" {
			store.WorkUnit = params.WorkUnit
		}
		if params.EvidenceGoal != "" {
			store.EvidenceGoal = params.EvidenceGoal
		}
		store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		store.NextAction = "begin"
		store.DecisionRequired = false
		store.Complete = false
		out := &ResetResult{AttemptsPreserved: len(store.Attempts), Scope: s.Scope}
		recordRequest(store, params.RequestID, opRescope, digest, out)
		if params.RequestID != "" {
			setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
		}
		if err := s.commit(store); err != nil {
			return fmt.Errorf("save store: %w", err)
		}
		result = &ResetResult{Revision: store.Revision, AttemptsPreserved: len(store.Attempts), Scope: s.Scope}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.Migrated = migrated
	result.Scope = s.Scope
	return result, nil
}

// ─── CLI helpers ─────────────────────────────────────────────────────────────

// ParseChangeName extracts the change name from args or returns an error.
func ParseChangeName(args []string) (string, error) {
	for i, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a, nil
		}
		// Skip flag values
		if strings.Contains(a, "=") {
			continue
		}
		if i+1 < len(args) && strings.HasPrefix(args[i+1], "-") {
			continue
		}
		i++ // skip value
	}
	return "", fmt.Errorf("change name required")
}

// RepoRoot returns the current working directory as repo root.
func RepoRoot() (string, error) {
	return os.Getwd()
}

// ─── Help text ───────────────────────────────────────────────────────────────

const HelpText = `SDD Attempt Runtime Ledger — manage attempt budgets for SDD changes.

The ledger lives in the git common directory (biggz/sdd-runtime/v1/<change>/)
as content-addressed CAS records; a legacy home-dir ledger is migrated
automatically on first access (the old file is kept untouched). Outside a git
repository the ledger falls back to a machine-scoped store
(~/.biggz/sdd-runtime-nogit/v1/<change>/) with the same semantics.

Usage:
  biggz sdd-attempt status <change> [--change-instance <token>] — show current attempt state
  biggz sdd-attempt begin <change> [flags]       — start a new attempt
  biggz sdd-attempt finish <change> [flags]      — finish current attempt
  biggz sdd-attempt reset <change> [flags]       — reset ledger (requires reason)
  biggz sdd-attempt grant <change> [flags]       — record per-change edit authority for roots
  biggz sdd-attempt acquire <change> [flags]     — claim a bounded attempt and return its token
  biggz sdd-attempt settle <change> [flags]      — complete the attempt selected by its token

  <change> is positional (the word right after the operation); there are no
  --change or --cwd flags. The workspace root is always os.Getwd().

Flags:
  --expected-revision <hash>    CAS guard: fail if current revision differs
                                (optional for grant: empty on a fresh
                                pre-attempt ledger, otherwise sha256)
  --objective-id <id>           Objective identifier for scope tracking
  --request-id <id>             Idempotency key (begin/finish/reset/grant):
                                replaying the same request id returns the
                                recorded result without mutating the ledger
  --outcome <passed|failed|interrupted>  Result of the attempt (finish)
  --outcome <passed|failed|interrupted|progress>  Result of the attempt (settle;
                                progress is a non-terminal checkpoint: it
                                closes the attempt without completing the
                                ledger, so one scope can cover N work units
                                with a single final passed settle)
  --diagnosis <text>            Human-readable diagnosis (finish/reset)
  --reason <text>               Reset reason (reset, required) or grant reason
                                (grant, required)
  --reset-by <name>             Who authorized the reset
  --actor <name>                Who authorized the grant (grant, required)
  --change-instance <token>     Change-instance identity: scopes the granted
                                roots projection of status; required for grant
                                (pass the change's persisted marker token)
  --root <path>                 Repository root to grant edit authority over
                                (grant, required and repeatable, 1..32 roots)
  --max-attempts <n>            Maximum allowed attempts
  --max-lines <n>               Maximum changed lines
  --work-unit <id>              Work unit identifier
  --evidence-revision <hash>    Evidence revision hash
  --binding-revision <hash>     Binding revision for remediation
  --binding-lineage <id>        Successor lineage for remediation
  --token <token>               Compact token returned by acquire (acquire optional continuation, settle required)
  --remediates-evidence-revision <hash>  Repaired failed evidence (acquire/settle)
`
