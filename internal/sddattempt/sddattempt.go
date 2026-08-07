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
	ActiveAttempt    int  `json:"active_attempt,omitempty"`
	DecisionRequired bool `json:"decision_required,omitempty"`
	Complete         bool `json:"complete,omitempty"`
	NextAction       string `json:"next_action,omitempty"` // "begin", "continue", "finish", "complete", ""

	// Objective scope
	ObjectiveID string `json:"objective_id,omitempty"`
	MaxAttempts int    `json:"max_attempts,omitempty"`
	MaxLines    int    `json:"max_changed_lines,omitempty"`

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

	// Requests records idempotency receipts for supplied request IDs: the
	// operation, the digest of the original request, and the recorded
	// outcome. A replay of the same request ID returns the SAME result
	// without mutating the ledger.
	Requests map[string]RuntimeRequestRecord `json:"requests,omitempty"`

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
	WorkUnit    string `json:"work_unit,omitempty"`

	// Timing
	BeganAt string `json:"began_at"`
	EndedAt string `json:"ended_at,omitempty"`

	// Outcome: "passed", "failed", "interrupted"
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
}

// RuntimeReset records ledger reset provenance.
type RuntimeReset struct {
	Reason       string `json:"reason"`
	ResetBy      string `json:"reset_by"`
	ResetAt      string `json:"reset_at"`
	PrevRevision string `json:"prev_revision"`
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
	requestDigestDomainBegin  = "biggz-ai.sdd-runtime-begin-request/v1"
	requestDigestDomainFinish = "biggz-ai.sdd-runtime-finish-request/v1"
	requestDigestDomainReset  = "biggz-ai.sdd-runtime-reset-request/v1"
	requestDigestDomainGrant  = "biggz-ai.sdd-runtime-grant-request/v1"

	opBegin  = "begin"
	opFinish = "finish"
	opReset  = "reset"
	opGrant  = "grant"
)

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

// RuntimeStatus is the public-facing status response.
type RuntimeStatus struct {
	ChangeName       string `json:"change_name"`
	Revision         string `json:"revision"`
	ActiveAttempt    int    `json:"active_attempt"`
	DecisionRequired bool   `json:"decision_required"`
	Complete         bool   `json:"complete"`
	NextAction       string `json:"next_action"`
	AttemptCount     int    `json:"attempt_count"`

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
			Migrated:         migrated,
			Scope:            s.Scope,
			GrantedRoots:     grantedRootsFor(store, instance),
			BindingRevision:  store.BindingRevision,
			BindingLineage:   store.BindingLineage,
			EvidenceRevision: store.EvidenceRevision,
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
			store := &RuntimeStore{
				ChangeName:   params.ChangeName,
				ObjectiveID:  params.ObjectiveID,
				MaxAttempts:  params.MaxAttempts,
				MaxLines:     params.MaxLines,
				WorkUnit:     params.WorkUnit,
				EvidenceGoal: params.EvidenceGoal,
				CreatedAt:    time.Now().UTC().Format(time.RFC3339),
				UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
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

		// Start new attempt
		nextOrdinal := store.ActiveAttempt + 1
		if nextOrdinal > store.MaxAttempts && store.MaxAttempts > 0 {
			store.DecisionRequired = true
			store.NextAction = "decision-required"
			store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := s.commit(store); err != nil {
				return fmt.Errorf("save store: %w", err)
			}
			return errors.New("max attempts reached, decision required")
		}

		store.ActiveAttempt = nextOrdinal
		store.NextAction = "continue"
		store.DecisionRequired = false
		store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		store.Attempts = append(store.Attempts, RuntimeAttempt{
			Ordinal:     nextOrdinal,
			ObjectiveID: params.ObjectiveID,
			WorkUnit:    params.WorkUnit,
			BeganAt:     time.Now().UTC().Format(time.RFC3339),
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

		// Update state based on outcome
		store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		if params.Outcome == "passed" {
			store.Complete = true
			store.NextAction = "complete"
			store.ActiveAttempt = 0
		} else {
			// Failed or interrupted — check if more attempts allowed
			if len(store.Attempts) >= store.MaxAttempts && store.MaxAttempts > 0 {
				store.DecisionRequired = true
				store.NextAction = "decision-required"
			} else {
				store.NextAction = "begin"
			}
		}

		remaining := store.MaxAttempts - len(store.Attempts)
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
	Revision      string `json:"revision"`
	AttemptsReset int    `json:"attempts_reset"`
	NewStore      bool   `json:"new_store,omitempty"`
	Migrated      bool   `json:"migrated,omitempty"`
	Scope         string `json:"scope,omitempty"`
}

// Reset clears the attempt history and creates a fresh ledger for a new
// objective. Requires an explicit maintainer scope decision.
// If change doesn't exist yet, creates a minimal fresh ledger.
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
		attemptCount := len(store.Attempts)

		// Record reset in provenance
		store.Resets = append(store.Resets, RuntimeReset{
			Reason:       params.Reason,
			ResetBy:      params.ResetBy,
			ResetAt:      time.Now().UTC().Format(time.RFC3339),
			PrevRevision: prevRev,
		})

		// Reset state
		store.ActiveAttempt = 0
		store.DecisionRequired = false
		store.Complete = false
		store.NextAction = "begin"
		store.Attempts = nil // Clear attempt history
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
		outcome := &ResetResult{AttemptsReset: attemptCount, Scope: s.Scope}
		recordRequest(store, params.RequestID, opReset, digest, outcome)
		if params.RequestID != "" {
			setRequestOutcomeRevision(store, params.RequestID, recordRevision(store))
		}
		if err := s.commit(store); err != nil {
			return fmt.Errorf("save store: %w", err)
		}

		result = &ResetResult{
			Revision:      store.Revision,
			AttemptsReset: attemptCount,
			Scope:         s.Scope,
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

Flags:
  --expected-revision <hash>    CAS guard: fail if current revision differs
                                (optional for grant: empty on a fresh
                                pre-attempt ledger, otherwise sha256)
  --objective-id <id>           Objective identifier for scope tracking
  --request-id <id>             Idempotency key (begin/finish/reset/grant):
                                replaying the same request id returns the
                                recorded result without mutating the ledger
  --outcome <passed|failed|interrupted>  Result of the attempt (finish)
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
`
