package review

import (
	"fmt"
	"time"
)

// Snapshot captures the state of files being reviewed at a point in time.
// It records the git tree hash and the list of changed files.
type Snapshot struct {
	ID          string    `json:"id"`
	ReviewID    string    `json:"review_id"`
	BaseTree    string    `json:"base_tree"`     // git tree hash of base commit
	Candidate   string    `json:"candidate"`     // git tree hash of candidate commit
	Paths       []string  `json:"paths"`          // changed files
	ChangedLines int      `json:"changed_lines"`  // total lines changed
	RiskLevel   string    `json:"risk_level,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// SnapshotManager handles review snapshots.
type SnapshotManager struct {
	snapshots []Snapshot
}

// NewSnapshotManager creates an empty snapshot manager.
func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{snapshots: make([]Snapshot, 0)}
}

// Record creates and stores a new snapshot.
func (sm *SnapshotManager) Record(reviewID, baseTree, candidate string, paths []string, changedLines int) Snapshot {
	s := Snapshot{
		ID:           fmt.Sprintf("snap-%d", len(sm.snapshots)+1),
		ReviewID:     reviewID,
		BaseTree:     baseTree,
		Candidate:    candidate,
		Paths:        paths,
		ChangedLines: changedLines,
		CreatedAt:    time.Now(),
	}
	sm.snapshots = append(sm.snapshots, s)
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
