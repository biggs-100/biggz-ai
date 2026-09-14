package sddattempt

// Pre-change ledger compatibility (fix-attempt-ledger-scope slice 2): a
// generation-1 ledger written before this change re-verifies its content
// address through the real load path, renders its derived generation as 1
// without touching canonical bytes, and still advances to a successor work
// unit without a reset.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// legacyCompleteRecord is a PRE-CHANGE ledger record: complete after one
// passed apply attempt, serialized exactly as the pre-change struct
// definition produced it — no "generation", no "advances", no
// "objective_generation" members anywhere.
const legacyCompleteRecord = `{"change_name":"ch-legacy-advance","complete":true,"next_action":"complete","objective_id":"obj-legacy","max_attempts":3,"max_changed_lines":400,"cumulative_changed_lines":40,"work_unit":"apply","evidence_goal":"goal","attempts":[{"ordinal":1,"objective_id":"obj-legacy","work_unit":"apply","began_at":"2026-01-02T03:04:05Z","ended_at":"2026-01-02T04:05:06Z","outcome":"passed","evidence_revision":"sha256:9999999999999999999999999999999999999999999999999999999999999999","diagnosis":"ok","changed_lines":40}],"created_at":"2026-01-02T03:04:05Z","updated_at":"2026-01-02T04:05:06Z"}` + "\n"

// legacyCompleteRevision freezes the content address of the fixture above as
// written before this change; an existing ledger's HEAD names exactly this
// hash, so it must never move.
const legacyCompleteRevision = "b2ac4a5fd8a93cd032cc8d389e1d6f0fce392a8d6ce129e54dc9e90a50d34d6f"

// publishLegacyRecord writes the frozen pre-change record exactly as an
// existing ledger holds it (record-<content-address>.json plus HEAD) and
// returns the store directory and the verified revision.
func publishLegacyRecord(t *testing.T, change, fixture string) (string, string) {
	t.Helper()
	frozen := []byte(fixture)
	s, err := resolveStore(change, "r")
	if err != nil {
		t.Fatalf("resolveStore: %v", err)
	}
	revision := sha256Hex([]byte(fixture))
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		t.Fatalf("mkdir ledger store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, recordFileName(revision)), frozen, 0644); err != nil {
		t.Fatalf("write record: %v", err)
	}
	if err := writeLedgerHead(s.Dir, revision); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}
	return s.Dir, revision
}

// TestLegacyCompat_FrozenRecordAndDerivedGeneration pins the compatibility
// contract: the frozen pre-change bytes re-canonicalize byte-identically,
// verify their content address through the real load path, and expose the
// derived generation default (0 reads as 1) without ever touching canonical
// bytes.
func TestLegacyCompat_FrozenRecordAndDerivedGeneration(t *testing.T) {
	setStoreRoot(t)
	frozen := []byte(legacyCompleteRecord)

	var parsed RuntimeStore
	if err := json.Unmarshal(frozen, &parsed); err != nil {
		t.Fatalf("parse pre-change record: %v", err)
	}
	rec := canonicalRecordPayload(&parsed)
	if !bytes.Equal(rec, frozen) {
		t.Fatalf("pre-change record re-canonicalizes to different bytes:\ngot  %s\nwant %s", rec, frozen)
	}
	revision := sha256Hex(rec)
	if revision != legacyCompleteRevision {
		t.Fatalf("content address moved: got %s, want frozen %s", revision, legacyCompleteRevision)
	}

	dir, published := publishLegacyRecord(t, "ch-legacy-advance", legacyCompleteRecord)
	if published != legacyCompleteRevision {
		t.Fatalf("published revision = %s, want the frozen %s", published, legacyCompleteRevision)
	}

	// The real read path verifies the frozen bytes instead of rejecting them.
	status, err := StatusWithInstance("ch-legacy-advance", "r", "")
	if err != nil {
		t.Fatalf("StatusWithInstance of the pre-change record: %v", err)
	}
	if !status.Complete || status.NextAction != "complete" {
		t.Fatalf("digested status = %+v, want the preserved complete generation-1 state", status)
	}
	if status.Generation != 1 {
		t.Fatalf("derived generation = %d, want 1 (generation 1 is field absence)", status.Generation)
	}
	if status.AttemptCount != 1 || status.LifetimeAttempts != 1 || status.LifetimeChangedLines != 40 {
		t.Fatalf("lifetime view = %d/%d/%d, want 1 attempt and 40 lines",
			status.AttemptCount, status.LifetimeAttempts, status.LifetimeChangedLines)
	}

	// The derived views never reach canonical bytes: the record on disk is
	// byte-identical and still hashes to its frozen content address.
	raw, err := os.ReadFile(filepath.Join(dir, recordFileName(revision)))
	if err != nil {
		t.Fatalf("re-read record: %v", err)
	}
	if !bytes.Equal(raw, frozen) {
		t.Fatalf("record bytes changed after reads:\ngot  %s\nwant %s", raw, frozen)
	}
	requireNoGenerationFields(t, raw)

	store, _, err := loadStore("ch-legacy-advance", "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if store.Generation != 0 || len(store.Advances) != 0 {
		t.Fatalf("stored generation state = %d/%d, want absent", store.Generation, len(store.Advances))
	}
	if store.Attempts[0].ObjectiveGeneration != 0 {
		t.Fatalf("stored attempt generation = %d, want absent", store.Attempts[0].ObjectiveGeneration)
	}
	if got := sha256Hex(canonicalRecordPayload(store)); got != legacyCompleteRevision {
		t.Fatalf("content address moved after reads: got %s, want %s", got, legacyCompleteRevision)
	}
}

// TestLegacyCompat_AdvancesWithDifferentWorkUnit pins the reported defect's
// original shape: a pre-change complete ledger loads, verifies and advances
// to a successor work unit instead of demanding a reset.
func TestLegacyCompat_AdvancesWithDifferentWorkUnit(t *testing.T) {
	setStoreRoot(t)
	frozen := []byte(legacyCompleteRecord)
	dir, revision := publishLegacyRecord(t, "ch-legacy-advance", legacyCompleteRecord)

	advanced, err := Acquire(AcquireParams{
		ChangeName: "ch-legacy-advance", RepoRoot: "r", RequestID: "legacy-verify-acquire",
		WorkUnit: "verify", EvidenceGoal: "verify the change", MaxAttempts: 2, MaxLines: 30,
	})
	if err != nil {
		t.Fatalf("a pre-change complete ledger must advance with a different work unit: %v", err)
	}
	if advanced.Token == "" {
		t.Fatal("advance over the pre-change ledger returned no token")
	}

	// The frozen predecessor record is still byte-identical on disk: the
	// advance appends, it never rewrites history.
	raw, err := os.ReadFile(filepath.Join(dir, recordFileName(revision)))
	if err != nil {
		t.Fatalf("re-read frozen record: %v", err)
	}
	if !bytes.Equal(raw, frozen) {
		t.Fatalf("advance rewrote the pre-change record:\ngot  %s\nwant %s", raw, frozen)
	}

	headData, err := os.ReadFile(filepath.Join(dir, "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headData))
	if head == revision {
		t.Fatal("advance must publish a new record and move HEAD")
	}
	if _, err := os.Stat(filepath.Join(dir, recordFileName(head))); err != nil {
		t.Fatalf("advanced record missing: %v", err)
	}

	store, _, err := loadStore("ch-legacy-advance", "r")
	if err != nil {
		t.Fatalf("loadStore after advance: %v", err)
	}
	if store.Generation != 2 || len(store.Advances) != 1 || len(store.Attempts) != 2 || len(store.Resets) != 0 {
		t.Fatalf("advanced pre-change ledger = generation %d, %d advances, %d attempts, %d resets",
			store.Generation, len(store.Advances), len(store.Attempts), len(store.Resets))
	}
	advance := store.Advances[0]
	if advance.FromWorkUnit != "apply" || advance.ToWorkUnit != "verify" ||
		advance.FromGeneration != 1 || advance.ToGeneration != 2 ||
		advance.MaxAttempts != 2 || advance.MaxLines != 30 || advance.PrevRevision != revision {
		t.Fatalf("advance provenance = %+v", advance)
	}
	if store.Attempts[0].ObjectiveGeneration != 0 {
		t.Fatalf("predecessor attempt generation = %d, want 0 (generation 1 by absence)", store.Attempts[0].ObjectiveGeneration)
	}
	if store.Attempts[1].ObjectiveGeneration != 2 {
		t.Fatalf("successor attempt generation = %d, want 2", store.Attempts[1].ObjectiveGeneration)
	}
	if store.MaxAttempts != 2 || store.MaxLines != 30 {
		t.Fatalf("successor budget = %d/%d, want its own 2/30", store.MaxAttempts, store.MaxLines)
	}

	status, err := StatusWithInstance("ch-legacy-advance", "r", "")
	if err != nil {
		t.Fatalf("StatusWithInstance after advance: %v", err)
	}
	if status.Complete || status.Generation != 2 || status.AttemptCount != 2 {
		t.Fatalf("status after advance = %+v, want the open successor generation", status)
	}
}
