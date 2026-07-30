// Package sddattempt implements the SDD runtime attempt ledger.
//
// The ledger tracks each apply/verify/remediation attempt for an SDD change.
// It is stored in ~/.biggz/sdd-runtime/v1/<change>/ and uses CAS (compare-and-swap)
// via SHA-256 revision hashes to prevent concurrent mutation conflicts.
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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─── Constants ───────────────────────────────────────────────────────────────

const (
	// RuntimeDir is the root directory for all runtime ledgers.
	RuntimeDir = "sdd-runtime"
	// RuntimeVersion is the schema version.
	RuntimeVersion = "v1"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// RuntimeStore is the complete ledger for one SDD change.
type RuntimeStore struct {
	// Revision is the SHA-256 of the JSON-serialized store (excluding revision).
	// Used for CAS: every mutation reads the current revision and writes the
	// new one atomically. A mismatch means a concurrent write happened.
	Revision string `json:"revision"`

	// ChangeName identifies the SDD change this ledger belongs to.
	ChangeName string `json:"change_name"`

	// Current attempt tracking
	ActiveAttempt    int  `json:"active_attempt,omitempty"`
	DecisionRequired bool `json:"decision_required,omitempty"`
	Complete         bool `json:"complete,omitempty"`
	NextAction       string `json:"next_action,omitempty"` // "begin", "continue", "finish", "complete", ""

	// Objective scope
	ObjectiveID  string `json:"objective_id,omitempty"`
	MaxAttempts  int    `json:"max_attempts,omitempty"`
	MaxLines     int    `json:"max_changed_lines,omitempty"`

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

	// Optional binding info (only when a binding revision is set)
	BindingRevision string `json:"binding_revision,omitempty"`
	BindingLineage  string `json:"binding_lineage,omitempty"`
	EvidenceRevision string `json:"evidence_revision,omitempty"`
}

// Status reads the runtime ledger and returns a status response.
// Returns an empty status (NextAction="begin") if no ledger exists yet.
func Status(changeName, repoRoot string) (*RuntimeStatus, error) {
	store, err := loadStore(changeName, repoRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return &RuntimeStatus{
				ChangeName: changeName,
				NextAction: "begin",
			}, nil
		}
		return nil, err
	}

	// Derive next_action
	nextAction := deriveNextAction(store)

	return &RuntimeStatus{
		ChangeName:       store.ChangeName,
		Revision:         store.Revision,
		ActiveAttempt:    store.ActiveAttempt,
		DecisionRequired: store.DecisionRequired,
		Complete:         store.Complete,
		NextAction:       nextAction,
		AttemptCount:     len(store.Attempts),
		BindingRevision:  store.BindingRevision,
		BindingLineage:   store.BindingLineage,
		EvidenceRevision: store.EvidenceRevision,
	}, nil
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

// ─── Begin ───────────────────────────────────────────────────────────────────

// BeginParams define a new attempt.
type BeginParams struct {
	ChangeName    string
	RepoRoot      string
	ExpectedRev   string // CAS: expected current revision (empty if new or force)
	ObjectiveID   string
	WorkUnit      string
	EvidenceGoal  string
	MaxAttempts   int
	MaxLines      int
	RequestID     string // idempotency key: if the same request_id is already recorded, return it
}

// BeginResult describes the result of beginning an attempt.
type BeginResult struct {
	Revision      string `json:"revision"`
	ActiveAttempt int    `json:"active_attempt"`
	AlreadyActive bool   `json:"already_active,omitempty"`
}

// Begin starts a new attempt on the change. Returns the attempt ordinal.
// CAS-guarded: if ExpectedRev doesn't match the current revision, the
// operation fails with a conflict error.
func Begin(params BeginParams) (*BeginResult, error) {
	// Load or create store
	store, err := loadStore(params.ChangeName, params.RepoRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("load store: %w", err)
		}
		// First attempt: create new store
		store = &RuntimeStore{
			ChangeName:   params.ChangeName,
			ObjectiveID:  params.ObjectiveID,
			MaxAttempts:  params.MaxAttempts,
			MaxLines:     params.MaxLines,
			WorkUnit:     params.WorkUnit,
			EvidenceGoal: params.EvidenceGoal,
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		}
		store.Revision = computeRevision(store)
		store.NextAction = "begin"
		store.ActiveAttempt = 1
		store.Attempts = append(store.Attempts, RuntimeAttempt{
			Ordinal:     1,
			ObjectiveID: params.ObjectiveID,
			WorkUnit:    params.WorkUnit,
			BeganAt:     time.Now().UTC().Format(time.RFC3339),
		})
		if err := saveStore(store, params.RepoRoot); err != nil {
			return nil, fmt.Errorf("save store: %w", err)
		}
		return &BeginResult{Revision: store.Revision, ActiveAttempt: 1}, nil
	}

	// CAS check
	if params.ExpectedRev != "" && store.Revision != params.ExpectedRev {
		return nil, fmt.Errorf("CAS conflict: expected revision %s, got %s", params.ExpectedRev, store.Revision)
	}

	// Idempotency: check if this request_id/ordinal is already recorded
	if store.ActiveAttempt > 0 {
		for i := len(store.Attempts) - 1; i >= 0; i-- {
			if store.Attempts[i].Ordinal == store.ActiveAttempt && store.Attempts[i].Outcome == "" {
				return &BeginResult{
					Revision:      store.Revision,
					ActiveAttempt: store.ActiveAttempt,
					AlreadyActive: true,
				}, nil
			}
		}
	}

	// Start new attempt
	nextOrdinal := store.ActiveAttempt + 1
	if nextOrdinal > store.MaxAttempts && store.MaxAttempts > 0 {
		store.DecisionRequired = true
		store.NextAction = "decision-required"
		store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		store.Revision = computeRevision(store)
		if err := saveStore(store, params.RepoRoot); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("max attempts (%d) reached, decision required", store.MaxAttempts)
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
	store.Revision = computeRevision(store)
	if err := saveStore(store, params.RepoRoot); err != nil {
		return nil, err
	}
	return &BeginResult{Revision: store.Revision, ActiveAttempt: nextOrdinal}, nil
}

// ─── Finish ──────────────────────────────────────────────────────────────────

// FinishParams define the end of an attempt.
type FinishParams struct {
	ChangeName  string
	RepoRoot    string
	ExpectedRev string // CAS: expected current revision
	Outcome     string // "passed", "failed", "interrupted"

	// Evidence
	EvidenceRevision string

	// Diagnostics
	Diagnosis         string
	HarnessDisposition string
	CleanupEvidence   string
	ProcessEvidence   string

	// Remediation binding (only for remediation paths)
	ExpectedBindingRevision  string
	SuccessorLineageID       string
	RemediatesEvidenceRevision string
}

// FinishResult describes the result of finishing an attempt.
type FinishResult struct {
	Revision         string `json:"revision"`
	RemainingAttempts int   `json:"remaining_attempts,omitempty"`
	DecisionRequired bool  `json:"decision_required,omitempty"`
	Complete         bool  `json:"complete,omitempty"`
}

// Finish closes the current attempt.
// Returns a conflict error if ExpectedRev doesn't match.
func Finish(params FinishParams) (*FinishResult, error) {
	store, err := loadStore(params.ChangeName, params.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("load store: %w", err)
	}

	// CAS check
	if store.Revision != params.ExpectedRev {
		return nil, fmt.Errorf("CAS conflict: expected revision %s, got %s", params.ExpectedRev, store.Revision)
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
		return nil, fmt.Errorf("no active attempt %d found", store.ActiveAttempt)
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

	store.Revision = computeRevision(store)
	if err := saveStore(store, params.RepoRoot); err != nil {
		return nil, err
	}

	remaining := store.MaxAttempts - len(store.Attempts)
	if remaining < 0 {
		remaining = 0
	}

	return &FinishResult{
		Revision:          store.Revision,
		RemainingAttempts: remaining,
		DecisionRequired:  store.DecisionRequired,
		Complete:          store.Complete,
	}, nil
}

// ─── Reset ───────────────────────────────────────────────────────────────────

// ResetParams define a ledger reset.
type ResetParams struct {
	ChangeName    string
	RepoRoot      string
	ExpectedRev   string // CAS: expected current revision
	Reason        string
	ResetBy       string
	MaxAttempts   int
	MaxLines      int
	ObjectiveID   string
}

// ResetResult describes the result of resetting the ledger.
type ResetResult struct {
	Revision      string `json:"revision"`
	AttemptsReset int    `json:"attempts_reset"`
	NewStore      bool   `json:"new_store,omitempty"`
}

// Reset clears the attempt history and creates a fresh ledger for a new
// objective. Requires an explicit maintainer scope decision.
// If change doesn't exist yet, creates a minimal fresh ledger.
func Reset(params ResetParams) (*ResetResult, error) {
	store, err := loadStore(params.ChangeName, params.RepoRoot)
	if err != nil {
		if os.IsNotExist(err) {
			// Create fresh
			store = &RuntimeStore{
				ChangeName:  params.ChangeName,
				ObjectiveID: params.ObjectiveID,
				MaxAttempts: params.MaxAttempts,
				MaxLines:    params.MaxLines,
				CreatedAt:   time.Now().UTC().Format(time.RFC3339),
				UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
			}
			store.Revision = computeRevision(store)
			store.NextAction = "begin"
			if err := saveStore(store, params.RepoRoot); err != nil {
				return nil, err
			}
			return &ResetResult{Revision: store.Revision, NewStore: true}, nil
		}
		return nil, fmt.Errorf("load store: %w", err)
	}

	// CAS check
	if params.ExpectedRev != "" && store.Revision != params.ExpectedRev {
		return nil, fmt.Errorf("CAS conflict: expected revision %s, got %s", params.ExpectedRev, store.Revision)
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
	store.Revision = computeRevision(store)
	if err := saveStore(store, params.RepoRoot); err != nil {
		return nil, err
	}

	return &ResetResult{
		Revision:      store.Revision,
		AttemptsReset: attemptCount,
	}, nil
}

// ─── Store I/O ───────────────────────────────────────────────────────────────

func storeDir(repoRoot string) string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".biggz", RuntimeDir, RuntimeVersion)
}

func storePath(changeName, repoRoot string) string {
	dir := storeDir(repoRoot)
	return filepath.Join(dir, changeName+".json")
}

func loadStore(changeName, repoRoot string) (*RuntimeStore, error) {
	path := storePath(changeName, repoRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var store RuntimeStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &store, nil
}

func saveStore(store *RuntimeStore, repoRoot string) error {
	dir := storeDir(repoRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := storePath(store.ChangeName, repoRoot)
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	// Atomic write via temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// LoadStore loads a runtime store from disk (public version for other packages).
func LoadStore(changeName, repoRoot string) (*RuntimeStore, error) {
	return loadStore(changeName, repoRoot)
}

// SaveStore persists a runtime store to disk (public version for other packages).
func SaveStore(store *RuntimeStore, repoRoot string) error {
	return saveStore(store, repoRoot)
}

func computeRevision(store *RuntimeStore) string {
	// Serialize without revision to avoid circular dependency
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

Usage:
  biggz sdd-attempt status <change>              — show current attempt state
  biggz sdd-attempt begin <change> [flags]       — start a new attempt
  biggz sdd-attempt finish <change> [flags]      — finish current attempt
  biggz sdd-attempt reset <change> [flags]       — reset ledger (requires reason)

Flags:
  --expected-revision <hash>    CAS guard: fail if current revision differs
  --objective-id <id>           Objective identifier for scope tracking
  --outcome <passed|failed|interrupted>  Result of the attempt (finish)
  --diagnosis <text>            Human-readable diagnosis (finish/reset)
  --reason <text>               Reset reason (reset, required)
  --reset-by <name>             Who authorized the reset
  --max-attempts <n>            Maximum allowed attempts
  --max-lines <n>               Maximum changed lines
  --work-unit <id>              Work unit identifier
  --evidence-revision <hash>    Evidence revision hash
  --binding-revision <hash>     Binding revision for remediation
  --binding-lineage <id>        Successor lineage for remediation
`
