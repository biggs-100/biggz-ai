package review

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Snapshot captures the state of files being reviewed at a point in time.
type Snapshot struct {
	ID           string    `json:"id"`
	ReviewID     string    `json:"review_id"`
	BaseTree     string    `json:"base_tree"`
	Candidate    string    `json:"candidate"`
	Paths        []string  `json:"paths"`
	ChangedLines int       `json:"changed_lines"`
	RiskLevel    string    `json:"risk_level,omitempty"`
	Hash         string    `json:"hash,omitempty"`
	ParentHash   string    `json:"parent_hash,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// SnapshotManager handles review snapshots with integrity verification.
type SnapshotManager struct {
	snapshots []Snapshot
	store     SnapshotStore
}

// SnapshotStore is the persistence interface for snapshots.
type SnapshotStore interface {
	SaveSnapshot(s Snapshot) error
	LoadSnapshot(id string) (*Snapshot, error)
	ListSnapshots(reviewID string) ([]Snapshot, error)
	DeleteSnapshot(id string) error
}

// NewSnapshotManager creates a snapshot manager with optional store.
func NewSnapshotManager(store SnapshotStore) *SnapshotManager {
	return &SnapshotManager{
		snapshots: make([]Snapshot, 0),
		store:     store,
	}
}

// Record creates and stores a new snapshot with integrity hash.
func (sm *SnapshotManager) Record(reviewID, baseTree, candidate string, paths []string, changedLines int) Snapshot {
	parentHash := ""
	if len(sm.snapshots) > 0 {
		last := sm.snapshots[len(sm.snapshots)-1]
		parentHash = last.Hash
	}

	s := Snapshot{
		ID:           fmt.Sprintf("snap-%d", time.Now().UnixNano()),
		ReviewID:     reviewID,
		BaseTree:     baseTree,
		Candidate:    candidate,
		Paths:        paths,
		ChangedLines: changedLines,
		ParentHash:   parentHash,
		CreatedAt:    time.Now().UTC(),
	}
	s.Hash = computeSnapshotHash(s)
	sm.snapshots = append(sm.snapshots, s)

	if sm.store != nil {
		if err := sm.store.SaveSnapshot(s); err != nil {
			// Log but don't fail — in-memory still works
		}
	}
	return s
}

// Latest returns the most recent snapshot.
func (sm *SnapshotManager) Latest() *Snapshot {
	if len(sm.snapshots) == 0 {
		return nil
	}
	return &sm.snapshots[len(sm.snapshots)-1]
}

// All returns all snapshots.
func (sm *SnapshotManager) All() []Snapshot {
	result := make([]Snapshot, len(sm.snapshots))
	copy(result, sm.snapshots)
	return result
}

// Get returns a snapshot by ID.
func (sm *SnapshotManager) Get(id string) *Snapshot {
	for _, s := range sm.snapshots {
		if s.ID == id {
			return &s
		}
	}
	if sm.store != nil {
		s, err := sm.store.LoadSnapshot(id)
		if err == nil {
			return s
		}
	}
	return nil
}

// VerifyChain checks the integrity of the snapshot hash chain.
func (sm *SnapshotManager) VerifyChain() ([]string, error) {
	var issues []string
	var prevHash string
	for i, s := range sm.snapshots {
		expected := computeSnapshotHash(s)
		if s.Hash != expected {
			issues = append(issues, fmt.Sprintf("snapshot %d (%s): hash mismatch", i, s.ID))
		}
		if s.ParentHash != prevHash {
			issues = append(issues, fmt.Sprintf("snapshot %d (%s): broken parent chain", i, s.ID))
		}
		prevHash = s.Hash
	}
	return issues, nil
}

// RecoverFromStore reloads all snapshots from the store, rebuilding the
// in-memory chain. Returns the number of snapshots recovered.
func (sm *SnapshotManager) RecoverFromStore(reviewID string) (int, error) {
	if sm.store == nil {
		return 0, fmt.Errorf("no snapshot store configured")
	}
	snapshots, err := sm.store.ListSnapshots(reviewID)
	if err != nil {
		return 0, fmt.Errorf("list snapshots: %w", err)
	}
	sm.snapshots = snapshots

	// Verify chain integrity
	issues, err := sm.VerifyChain()
	if err != nil {
		return len(snapshots), fmt.Errorf("chain verification: %w", err)
	}
	if len(issues) > 0 {
		return len(snapshots), fmt.Errorf("chain issues: %v", issues)
	}
	return len(snapshots), nil
}

// RestoreToPoint replays the state to a specific snapshot, discarding
// snapshots after it.
func (sm *SnapshotManager) RestoreToPoint(id string) error {
	for i, s := range sm.snapshots {
		if s.ID == id {
			sm.snapshots = sm.snapshots[:i+1]
			return nil
		}
	}
	return fmt.Errorf("snapshot %s not found", id)
}

// Export serializes the snapshot chain to JSON.
func (sm *SnapshotManager) Export() ([]byte, error) {
	return json.MarshalIndent(sm.snapshots, "", "  ")
}

// Import loads snapshots from JSON, verifying chain integrity.
func (sm *SnapshotManager) Import(data []byte) error {
	var snapshots []Snapshot
	if err := json.Unmarshal(data, &snapshots); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	// Verify chain before accepting
	var prevHash string
	for i, s := range snapshots {
		expected := computeSnapshotHash(s)
		if s.Hash != expected {
			return fmt.Errorf("snapshot %d: hash mismatch", i)
		}
		if s.ParentHash != prevHash {
			return fmt.Errorf("snapshot %d: broken parent chain", i)
		}
		prevHash = s.Hash
	}
	sm.snapshots = snapshots
	return nil
}

const SnapshotDomain = "biggz-ai.review-snapshot/v1"

func computeSnapshotHash(s Snapshot) string {
	fields := [][]byte{
		[]byte(s.BaseTree),
		[]byte(s.Candidate),
		[]byte(s.ParentHash),
		[]byte(strconv.Itoa(s.ChangedLines)),
	}
	for _, p := range s.Paths {
		fields = append(fields, []byte(p))
	}
	payload := writeLengthPrefixed(fields...)
	return domainHash(SnapshotDomain, payload)
}

// DiagnosisReport describes the health of a snapshot chain.
type DiagnosisReport struct {
	TotalSnapshots  int      `json:"total_snapshots"`
	ChainValid      bool     `json:"chain_valid"`
	Issues          []string `json:"issues,omitempty"`
	OldestSnapshot  string   `json:"oldest_snapshot,omitempty"`
	NewestSnapshot  string   `json:"newest_snapshot,omitempty"`
	RecoveryNeeded  bool     `json:"recovery_needed,omitempty"`
	RecoveryActions []string `json:"recovery_actions,omitempty"`
}

// Diagnose performs a full health check on the snapshot chain.
func (sm *SnapshotManager) Diagnose() *DiagnosisReport {
	r := &DiagnosisReport{
		TotalSnapshots: len(sm.snapshots),
	}
	if len(sm.snapshots) == 0 {
		r.ChainValid = true
		return r
	}

	issues, _ := sm.VerifyChain()
	r.Issues = issues
	r.ChainValid = len(issues) == 0
	r.OldestSnapshot = sm.snapshots[0].ID
	r.NewestSnapshot = sm.snapshots[len(sm.snapshots)-1].ID

	if !r.ChainValid {
		r.RecoveryNeeded = true
		r.RecoveryActions = append(r.RecoveryActions,
			"Verify store integrity",
			"Rebuild from event log if available",
			"Consider reverting to last known good snapshot")
	}
	return r
}
