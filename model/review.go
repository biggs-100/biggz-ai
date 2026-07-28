// Package model defines the core data types for biggz-ai reviews.
//
// It contains the ReviewState structure with its Merkle-rooted evidence
// chain, the 5-state FSM for review lifecycle, and the hash functions
// that guarantee evidence-chain integrity.
package model

import (
	"time"

	"github.com/google/uuid"
)

// ReviewStatus represents the current lifecycle state of a review.
type ReviewStatus string

const (
	StatusPending    ReviewStatus = "pending"
	StatusInProgress ReviewStatus = "in_progress"
	StatusCompleted  ReviewStatus = "completed"
	StatusArchived   ReviewStatus = "archived"
	StatusFailed     ReviewStatus = "failed"
)

// ReviewSubject identifies the subject of a code review.
type ReviewSubject struct {
	Repository string `json:"repository"`
	CommitSHA  string `json:"commit_sha"`
}

// ReviewState represents the full state of a review at a point in time.
// It carries a UUIDv7 identifier, schema version for forward compatibility,
// the current status, the review subject, an append-only evidence chain
// with Merkle integrity, corrections applied during the review, and
// timestamps tracking the review lifecycle.
type ReviewState struct {
	ID            string        `json:"id"`
	SchemaVersion string        `json:"schema_version"`
	Status        ReviewStatus  `json:"status"`
	Subject       ReviewSubject `json:"subject"`
	Evidence      []Evidence    `json:"evidence"`
	Corrections   []Correction  `json:"corrections"`
	MerkleRoot    string        `json:"merkle_root"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// NewReviewState creates a ReviewState with a UUIDv7 ID, SchemaVersion "1.0",
// Status Pending, and an empty evidence chain.
func NewReviewState(subject ReviewSubject) *ReviewState {
	now := time.Now()
	return &ReviewState{
		ID:            uuid.Must(uuid.NewV7()).String(),
		SchemaVersion: CurrentSchemaVersion,
		Status:        StatusPending,
		Subject:       subject,
		Evidence:      []Evidence{},
		Corrections:   []Correction{},
		MerkleRoot:    "",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// CurrentSchemaVersion is the schema version used by the current build.
const CurrentSchemaVersion = "1.0"

// Evidence is a single entry in the append-only evidence chain.
// Each entry carries its position, a timestamp, the kind of evidence,
// a JSON payload, the hash of the previous entry (PrevHash), and its
// own hash (SHA-256 of Position|Timestamp|Kind|Payload|PrevHash).
type Evidence struct {
	Position  int       `json:"position"`
	Timestamp time.Time `json:"timestamp"`
	Kind      string    `json:"kind"`
	Payload   string    `json:"payload"`
	PrevHash  string    `json:"prevHash"`
	Hash      string    `json:"hash"`
}

// PolicyVerdict represents the result of evaluating a single policy
// against a ReviewState.
type PolicyVerdict struct {
	Policy   string `json:"policy"`
	Passed   bool   `json:"passed"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
}

// Correction represents a single correction applied during a review.
type Correction struct {
	ID        string    `json:"id"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
