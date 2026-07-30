// Package review — compact state for parallel review transactions.
//
// The compact store enables multiple review lineages to operate concurrently
// without conflicts by maintaining a content-addressed, append-only event log
// alongside compacted snapshots of each lineage's terminal state.
//
// Architecture:
//
//	Transaction log  ──►  Append-only, content-addressed events
//	Compact state    ──►  Merged terminal state per lineage
//	Reconciliation   ──►  Verify event log matches compact state
package review

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/biggz-ai/biggz/model"
)

// CompactStateSchema identifies the compact state format version.
const CompactStateSchema = "biggz.review-compact-state/v1"

// CompactCorrectionBudget is the max correction rounds for a compact lineage.
const CompactCorrectionBudget = 1

// CompactStateError identifies a validation failure with lineage context.
type CompactStateError struct {
	LineageID string
	State     model.ReviewStatus
	Problem   string
}

func (e *CompactStateError) Error() string {
	return fmt.Sprintf("compact lineage %q state %q invalid: %s", e.LineageID, e.State, e.Problem)
}

// CompactRecord is a single compacted state record for one lineage.
type CompactRecord struct {
	LineageID    string            `json:"lineage_id"`
	Schema       string            `json:"schema"`
	State        model.ReviewStatus `json:"state"`
	MerkleRoot   string            `json:"merkle_root"`
	ReceiptHash  string            `json:"receipt_hash,omitempty"`
	EventCount   int               `json:"event_count"`
	Corrections  int               `json:"corrections"`
	FirstEvent   time.Time         `json:"first_event"`
	LastEvent    time.Time         `json:"last_event"`
	EvidenceHash string            `json:"evidence_hash,omitempty"`
}

// CompactStore manages compacted review state for parallel lineages.
type CompactStore struct {
	mu       sync.RWMutex
	records  map[string]*CompactRecord // lineageID → record
	events   map[string][]CompactEvent // lineageID → events
	eventLog []CompactEvent            // global append-only event log
}

// CompactEvent is a single event in the compact transaction log.
type CompactEvent struct {
	ID        string            `json:"id"`
	LineageID string            `json:"lineage_id"`
	Type      string            `json:"type"`
	State     model.ReviewStatus `json:"state"`
	Timestamp time.Time         `json:"timestamp"`
	Hash      string            `json:"hash"`
	PrevHash  string            `json:"prev_hash"`
}

// NewCompactStore creates an empty compact store.
func NewCompactStore() *CompactStore {
	return &CompactStore{
		records: make(map[string]*CompactRecord),
		events:  make(map[string][]CompactEvent),
	}
}

// BeginTransaction opens a new event for a lineage. Returns the event ID.
func (cs *CompactStore) BeginTransaction(lineageID string) (string, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Check existing record
	if rec, ok := cs.records[lineageID]; ok {
		if rec.Corrections >= CompactCorrectionBudget {
			return "", fmt.Errorf("lineage %s has exhausted its correction budget", lineageID)
		}
	}

	eventID := fmt.Sprintf("evt-%s-%d", lineageID[:min(8, len(lineageID))], time.Now().UnixNano())
	prevHash := ""
	if events, ok := cs.events[lineageID]; ok && len(events) > 0 {
		prevHash = events[len(events)-1].Hash
	}

	evt := CompactEvent{
		ID:        eventID,
		LineageID: lineageID,
		Type:      "begin",
		State:     model.StatusInProgress,
		Timestamp: time.Now().UTC(),
		PrevHash:  prevHash,
	}
	evt.Hash = hashCompactEvent(evt)

	cs.events[lineageID] = append(cs.events[lineageID], evt)
	cs.eventLog = append(cs.eventLog, evt)

	return eventID, nil
}

// CommitTransaction finalizes a transaction with the given state and evidence.
func (cs *CompactStore) CommitTransaction(lineageID, merkleRoot, receiptHash string, evidence []model.Evidence) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	events, ok := cs.events[lineageID]
	if !ok || len(events) == 0 {
		return fmt.Errorf("no active transaction for lineage %s", lineageID)
	}

	lastEvent := events[len(events)-1]
	evt := CompactEvent{
		ID:        fmt.Sprintf("evt-%s-%d", lineageID[:min(8, len(lineageID))], time.Now().UnixNano()),
		LineageID: lineageID,
		Type:      "commit",
		State:     model.StatusCompleted,
		Timestamp: time.Now().UTC(),
		PrevHash:  lastEvent.Hash,
	}
	evt.Hash = hashCompactEvent(evt)

	// Build evidence hash
	eh := hashEvidence(evidence)

	rec := &CompactRecord{
		LineageID:    lineageID,
		Schema:       CompactStateSchema,
		State:        model.StatusCompleted,
		MerkleRoot:   merkleRoot,
		ReceiptHash:  receiptHash,
		EventCount:   len(events) + 1,
		Corrections:  0,
		FirstEvent:   events[0].Timestamp,
		LastEvent:    evt.Timestamp,
		EvidenceHash: eh,
	}

	cs.events[lineageID] = append(cs.events[lineageID], evt)
	cs.eventLog = append(cs.eventLog, evt)
	cs.records[lineageID] = rec
	return nil
}

// FailTransaction marks a transaction as failed.
func (cs *CompactStore) FailTransaction(lineageID, reason string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	events, ok := cs.events[lineageID]
	if !ok || len(events) == 0 {
		return fmt.Errorf("no active transaction for lineage %s", lineageID)
	}

	lastEvent := events[len(events)-1]
	evt := CompactEvent{
		ID:        fmt.Sprintf("evt-%s-%d", lineageID[:min(8, len(lineageID))], time.Now().UnixNano()),
		LineageID: lineageID,
		Type:      "fail:" + reason,
		State:     model.StatusFailed,
		Timestamp: time.Now().UTC(),
		PrevHash:  lastEvent.Hash,
	}
	evt.Hash = hashCompactEvent(evt)

	cs.events[lineageID] = append(cs.events[lineageID], evt)
	cs.eventLog = append(cs.eventLog, evt)
	return nil
}

// GetRecord returns the compact record for a lineage, if any.
func (cs *CompactStore) GetRecord(lineageID string) (*CompactRecord, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	rec, ok := cs.records[lineageID]
	if !ok {
		return nil, false
	}
	return rec, true
}

// ListLineages returns all lineage IDs with their terminal state.
func (cs *CompactStore) ListLineages() []CompactRecord {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	result := make([]CompactRecord, 0, len(cs.records))
	for _, rec := range cs.records {
		result = append(result, *rec)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastEvent.After(result[j].LastEvent)
	})
	return result
}

// Reconcile verifies that the event log for every lineage is consistent
// with its compact record. Returns any lineages where events and records
// disagree.
func (cs *CompactStore) Reconcile() ([]string, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var issues []string
	for lineageID, rec := range cs.records {
		events := cs.events[lineageID]
		if len(events) != rec.EventCount {
			issues = append(issues, fmt.Sprintf(
				"%s: record claims %d events, log has %d", lineageID, rec.EventCount, len(events)))
			continue
		}
		// Verify hash chain
		var prevHash string
		for i, evt := range events {
			expectedHash := hashCompactEvent(evt)
			if evt.Hash != expectedHash {
				issues = append(issues, fmt.Sprintf(
					"%s: event %d hash mismatch", lineageID, i))
				break
			}
			if evt.PrevHash != prevHash {
				issues = append(issues, fmt.Sprintf(
					"%s: event %d prev_hash broken", lineageID, i))
				break
			}
			prevHash = evt.Hash
		}
	}
	return issues, nil
}

// ValidateRecord checks a single compact record for structural integrity.
func ValidateRecord(rec *CompactRecord) error {
	if rec.LineageID == "" {
		return &CompactStateError{Problem: "empty lineage_id"}
	}
	if rec.Schema != CompactStateSchema {
		return &CompactStateError{
			LineageID: rec.LineageID,
			Problem:   fmt.Sprintf("unexpected schema %q", rec.Schema),
		}
	}
	if rec.State != model.StatusCompleted && rec.State != model.StatusArchived {
		return &CompactStateError{
			LineageID: rec.LineageID,
			State:     rec.State,
			Problem:   "non-terminal state in compact record",
		}
	}
	if rec.MerkleRoot == "" && rec.State == model.StatusCompleted {
		return &CompactStateError{
			LineageID: rec.LineageID,
			State:     rec.State,
			Problem:   "completed compact record missing merkle root",
		}
	}
	return nil
}

// hashCompactEvent computes the SHA-256 hash of a compact event.
func hashCompactEvent(evt CompactEvent) string {
	h := sha256.New()
	h.Write([]byte(evt.ID))
	h.Write([]byte(evt.LineageID))
	h.Write([]byte(evt.Type))
	h.Write([]byte(string(evt.State)))
	h.Write([]byte(evt.Timestamp.Format(time.RFC3339Nano)))
	h.Write([]byte(evt.PrevHash))
	return hex.EncodeToString(h.Sum(nil))
}

// hashEvidence computes a hash over all evidence entries.
func hashEvidence(evidence []model.Evidence) string {
	if len(evidence) == 0 {
		return ""
	}
	h := sha256.New()
	for _, e := range evidence {
		h.Write([]byte(e.Kind))
		h.Write([]byte(e.Payload))
		h.Write([]byte(e.Hash))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// CompactStateStore is a durable persistence interface for compact records.
type CompactStateStore interface {
	SaveCompactRecord(rec *CompactRecord) error
	LoadCompactRecord(lineageID string) (*CompactRecord, error)
	ListCompactRecords() ([]CompactRecord, error)
	SaveCompactEvent(evt CompactEvent) error
	LoadCompactEvents(lineageID string) ([]CompactEvent, error)
}

// CompactRecoveryInfo holds metadata for recovering a compact lineage.
type CompactRecoveryInfo struct {
	LineageID      string    `json:"lineage_id"`
	ExpectedState  string    `json:"expected_state"`
	LastKnownGood  time.Time `json:"last_known_good"`
	RecoveryAction string    `json:"recovery_action"` // "replay", "reset", "quarantine"
	Evidence       string    `json:"evidence,omitempty"`
}

// DiagnoseLineage checks a lineage's compact record for corruption.
func DiagnoseLineage(rec *CompactRecord, lastGood time.Time) *CompactRecoveryInfo {
	if rec == nil {
		return nil
	}
	diag := &CompactRecoveryInfo{
		LineageID:     rec.LineageID,
		ExpectedState: string(rec.State),
		LastKnownGood: lastGood,
	}

	if rec.EventCount == 0 {
		diag.RecoveryAction = "reset"
		diag.Evidence = "zero events in compact record"
		return diag
	}

	if rec.LastEvent.Before(lastGood) && rec.State == model.StatusCompleted {
		diag.RecoveryAction = "replay"
		diag.Evidence = fmt.Sprintf("record state is completed but last event %s is before last known good %s",
			rec.LastEvent.Format(time.RFC3339), lastGood.Format(time.RFC3339))
		return diag
	}

	if rec.Corrections > CompactCorrectionBudget {
		diag.RecoveryAction = "quarantine"
		diag.Evidence = fmt.Sprintf("corrections %d exceeds budget %d", rec.Corrections, CompactCorrectionBudget)
		return diag
	}

	return nil
}

// MergeCompactRecords merges two compact records for the same lineage,
// keeping the most recent state from each field.
func MergeCompactRecords(a, b *CompactRecord) *CompactRecord {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	merged := *a
	if b.LastEvent.After(a.LastEvent) {
		merged.State = b.State
		merged.MerkleRoot = b.MerkleRoot
		merged.ReceiptHash = b.ReceiptHash
		merged.EventCount = b.EventCount
		merged.Corrections = b.Corrections
		merged.LastEvent = b.LastEvent
		merged.EvidenceHash = b.EvidenceHash
	}
	if a.FirstEvent.Before(b.FirstEvent) {
		merged.FirstEvent = a.FirstEvent
	} else {
		merged.FirstEvent = b.FirstEvent
	}
	return &merged
}

// CompactStoreStats returns statistics about the compact store.
type CompactStoreStats struct {
	TotalLineages   int            `json:"total_lineages"`
	CompletedCount  int            `json:"completed_count"`
	FailedCount     int            `json:"failed_count"`
	TotalEvents     int            `json:"total_events"`
	ByState         map[string]int `json:"by_state"`
}

// Stats computes statistics from the compact store.
func (cs *CompactStore) Stats() *CompactStoreStats {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	stats := &CompactStoreStats{
		TotalLineages: len(cs.records),
		TotalEvents:   len(cs.eventLog),
		ByState:       make(map[string]int),
	}
	for _, rec := range cs.records {
		stats.ByState[string(rec.State)]++
		if rec.State == model.StatusCompleted {
			stats.CompletedCount++
		} else if rec.State == model.StatusFailed {
			stats.FailedCount++
		}
	}
	return stats
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CompactStoreJSON is the JSON-serializable form of the compact store.
type CompactStoreJSON struct {
	Records  map[string]*CompactRecord `json:"records"`
	EventLog []CompactEvent             `json:"event_log"`
	Schema   string                     `json:"schema"`
}

// MarshalJSON serializes the compact store to JSON.
func (cs *CompactStore) MarshalJSON() ([]byte, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	j := CompactStoreJSON{
		Records:  cs.records,
		EventLog: cs.eventLog,
		Schema:   CompactStateSchema,
	}
	return json.MarshalIndent(j, "", "  ")
}

// UnmarshalJSON deserializes the compact store from JSON.
func (cs *CompactStore) UnmarshalJSON(data []byte) error {
	var j CompactStoreJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.records = j.Records
	cs.eventLog = j.EventLog
	cs.events = make(map[string][]CompactEvent)
	for _, evt := range cs.eventLog {
		cs.events[evt.LineageID] = append(cs.events[evt.LineageID], evt)
	}
	if cs.records == nil {
		cs.records = make(map[string]*CompactRecord)
	}
	return nil
}

// CompactStoreFormatError records show-stopping schema/parse errors in the
// compact store, distinct from per-lineage semantic validation failures.
type CompactStoreFormatError struct {
	Message string
	Path    string
}

func (e *CompactStoreFormatError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("compact store format error at %s: %s", e.Path, e.Message)
	}
	return fmt.Sprintf("compact store format error: %s", e.Message)
}

// ErrQuarantinedLineage is returned when attempting to operate on a lineage
// that has been quarantined due to unrecoverable corruption.
var ErrQuarantinedLineage = fmt.Errorf("lineage is quarantined")

// QuarantineLineage marks a lineage as quarantined in the compact store.
func (cs *CompactStore) QuarantineLineage(lineageID, reason string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	evt := CompactEvent{
		ID:        fmt.Sprintf("evt-%s-%d", lineageID[:min(8, len(lineageID))], time.Now().UnixNano()),
		LineageID: lineageID,
		Type:      "quarantine:" + reason,
		State:     model.StatusFailed,
		Timestamp: time.Now().UTC(),
	}
	evt.Hash = hashCompactEvent(evt)

	cs.events[lineageID] = append(cs.events[lineageID], evt)
	cs.eventLog = append(cs.eventLog, evt)
	cs.records[lineageID] = &CompactRecord{
		LineageID:  lineageID,
		Schema:     CompactStateSchema,
		State:      model.StatusFailed,
		EventCount: len(cs.events[lineageID]),
		LastEvent:  evt.Timestamp,
	}
	return nil
}

// RecoverLineage attempts to recover a quarantined lineage by verifying its
// event chain and rebuilding the compact record from events.
func (cs *CompactStore) RecoverLineage(lineageID string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	events, ok := cs.events[lineageID]
	if !ok || len(events) == 0 {
		return fmt.Errorf("no events for lineage %s", lineageID)
	}

	// Verify hash chain
	var prevHash string
	for i, evt := range events {
		expected := hashCompactEvent(evt)
		if evt.Hash != expected {
			return fmt.Errorf("hash mismatch at event %d", i)
		}
		if evt.PrevHash != prevHash {
			return fmt.Errorf("broken chain at event %d", i)
		}
		prevHash = evt.Hash
	}

	// Rebuild record
	lastEvent := events[len(events)-1]
	terminalState := lastEvent.State
	corrections := 0
	for _, evt := range events {
		if strings.HasPrefix(evt.Type, "correction:") {
			corrections++
		}
	}

	cs.records[lineageID] = &CompactRecord{
		LineageID:    lineageID,
		Schema:       CompactStateSchema,
		State:        terminalState,
		EventCount:   len(events),
		Corrections:  corrections,
		FirstEvent:   events[0].Timestamp,
		LastEvent:    lastEvent.Timestamp,
	}
	return nil
}
