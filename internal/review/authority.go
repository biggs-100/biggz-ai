package review

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/biggz-ai/biggz/model"
)

// ---------------------------------------------------------------------------
// Authority — Store Facade
// ---------------------------------------------------------------------------
//
// Authority is the public API over the content-addressed event store.
// It coordinates Store (persistence), FSM (validation), and Receipt
// (chain binding) into a single facade.

// Authority provides store facade operations for the review event store.
type Authority struct {
	repo string
}

// LineageInfo describes a single review lineage for inventory output.
type LineageInfo struct {
	LineageID string `json:"lineage_id"`
	State     string `json:"state,omitempty"`
	LastEvent string `json:"last_event,omitempty"`
}

// LineageStatus provides detailed status for a single lineage.
type LineageStatus struct {
	LineageID        string               `json:"lineage_id"`
	HeadHash         string               `json:"head_hash"`
	EventCount       int                  `json:"event_count"`
	ChainValid       bool                 `json:"chain_valid"`
	Receipt          *Receipt             `json:"receipt,omitempty"`
	IntegrityVerdict *IntegrityVerdict    `json:"integrity_verdict,omitempty"`
	BudgetCounters   model.BudgetCounters `json:"budget_counters"`
	Lenses           []CapturedLens       `json:"lenses"`
	Budget           *FrozenBudgetInfo    `json:"budget,omitempty"`
	ReceiptArtifact  *ReceiptArtifactRef  `json:"receipt_artifact,omitempty"`
	NextTransition   *NextTransition      `json:"next_transition,omitempty"`
}

// NewAuthority creates an Authority for the given repo root.
// If repo is empty, auto-detects from the current working directory.
func NewAuthority(repo string) *Authority {
	return &Authority{repo: repo}
}

// Open opens or creates a Store for the given lineage.
func (a *Authority) Open(lineageID string) (*Store, error) {
	return Open(a.repo, lineageID)
}

// Append appends an event to the lineage's event store.
func (a *Authority) Append(lineageID, prevRevision string, rec Record) (string, error) {
	store, err := a.Open(lineageID)
	if err != nil {
		return "", err
	}
	return store.Append(prevRevision, rec)
}

// LoadChain loads and validates the event chain for a lineage.
func (a *Authority) LoadChain(lineageID string) (ValidatedChain, error) {
	store, err := a.Open(lineageID)
	if err != nil {
		return ValidatedChain{}, err
	}
	return store.LoadChain()
}

// Validate performs a full integrity check on a lineage's event chain.
func (a *Authority) Validate(lineageID string) IntegrityVerdict {
	store, err := a.Open(lineageID)
	if err != nil {
		return IntegrityVerdict{Valid: false, Reason: err.Error()}
	}
	return store.Validate()
}

// Inventory lists all lineages in the event store by scanning
// .git/biggz/review-transactions/ directories. Returns an empty slice
// if the store root does not exist yet.
func (a *Authority) Inventory() ([]LineageInfo, error) {
	gitDir, err := resolveGitDir(a.repo)
	if err != nil {
		return nil, fmt.Errorf("inventory: %w", err)
	}
	storeRoot := filepath.Join(gitDir, "biggz", "review-transactions")

	entries, err := os.ReadDir(storeRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []LineageInfo{}, nil
		}
		return nil, fmt.Errorf("inventory: read store root: %w", err)
	}

	var lineages []LineageInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		linfo := LineageInfo{LineageID: e.Name()}

		// Try to get last event state/timestamp from HEAD.
		store := OpenWithDir(filepath.Join(storeRoot, e.Name()), e.Name())
		chain, err := store.LoadChain()
		if err == nil && chain.Count > 0 {
			last := chain.Records[chain.Count-1]
			linfo.LastEvent = last.Timestamp
			linfo.State = last.Operation
		}

		lineages = append(lineages, linfo)
	}

	// Sort by lineage ID for deterministic output.
	sort.Slice(lineages, func(i, j int) bool {
		return lineages[i].LineageID < lineages[j].LineageID
	})

	return lineages, nil
}

// Status returns detailed status for a single lineage:
// head hash, event count, chain integrity, receipt, and budget counters.
func (a *Authority) Status(lineageID string) (*LineageStatus, error) {
	store, err := a.Open(lineageID)
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}

	chain, err := store.LoadChain()
	if err != nil {
		return nil, fmt.Errorf("status: load chain: %w", err)
	}

	verdict := store.Validate()

	st := &LineageStatus{
		LineageID:        chain.LineageID,
		HeadHash:         chain.HeadHash,
		EventCount:       chain.Count,
		ChainValid:       verdict.Valid,
		IntegrityVerdict: &verdict,
		BudgetCounters:   model.BudgetCounters{},
		Lenses:           CapturedLenses(chain),
		Budget:           frozenBudgetOf(chain),
		ReceiptArtifact:  receiptArtifactOf(chain),
	}

	// Derived routing envelope (Phase C2): the orchestrator's ONLY routing
	// authority. Derived from persisted bytes and the RDD kill switch; all
	// existing fields are unchanged.
	st.NextTransition = deriveNextTransition(store, a.repo, chain, verdict)

	// Create receipt if chain is valid and has events.
	if verdict.Valid && chain.Count > 0 {
		receipt := NewReceipt(chain)
		st.Receipt = &receipt
	}

	return st, nil
}

// ---------------------------------------------------------------------------
// Deprecated: AuthorityVerifier (kept for backward compat with gate.go — PR 3)
// ---------------------------------------------------------------------------

// AuthorityVerifier validates that a review receipt matches the repository state.
// Deprecated: use Authority for new code.
type AuthorityVerifier struct {
	RepoRoot string
}

// AuthorityResult describes the outcome of an authority check.
type AuthorityResult struct {
	Valid      bool   `json:"valid"`
	Reason     string `json:"reason"`
	ReviewID   string `json:"review_id,omitempty"`
	MerkleRoot string `json:"merkle_root,omitempty"`
	GitCommit  string `json:"git_commit,omitempty"`
	GitBranch  string `json:"git_branch,omitempty"`
}

// Verify checks that the receipt is valid for the given review state.
func (av *AuthorityVerifier) Verify(receipt *Receipt, state *model.ReviewState) *AuthorityResult {
	if receipt == nil {
		return &AuthorityResult{Valid: false, Reason: "no receipt provided"}
	}
	if state == nil {
		return &AuthorityResult{Valid: false, Reason: "no review state provided"}
	}
	if !VerifyReceipt(receipt, state) {
		return &AuthorityResult{
			Valid:      false,
			Reason:     "receipt does not match state",
			ReviewID:   state.ID,
			MerkleRoot: state.MerkleRoot,
		}
	}
	return &AuthorityResult{
		Valid:      true,
		ReviewID:   state.ID,
		MerkleRoot: state.MerkleRoot,
		Reason:     "receipt is valid",
	}
}
