package sddattempt

// CAS contract for the objective-generation fields (fix-attempt-ledger-scope
// slice 1): generation 1 is represented by field ABSENCE, records are
// content-addressed full snapshots re-verified on load, and any
// generation/lifetime value a reader exposes is derived read-time — never
// stamped on canonical bytes. Mirrors the style of cas_store_test.go.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// preChangeRecord is a generation-1 ledger record serialized exactly as the
// pre-change struct definition produced it: no "generation", no "advances",
// no "objective_generation" members. Re-canonicalizing these bytes through
// the current struct must be byte-stable, or every existing ledger on disk
// would fail content-address verification after this change.
const preChangeRecord = `{"change_name":"ch-cas-contract","complete":true,"next_action":"complete","objective_id":"obj-gen1","max_attempts":3,"max_changed_lines":400,"cumulative_changed_lines":120,"work_unit":"apply","evidence_goal":"goal","attempts":[{"ordinal":1,"objective_id":"obj-gen1","work_unit":"apply","began_at":"2026-01-02T03:04:05Z","ended_at":"2026-01-02T04:05:06Z","outcome":"passed","evidence_revision":"sha256:1111111111111111111111111111111111111111111111111111111111111111","diagnosis":"ok","harness_disposition":"reused","cleanup_evidence":"clean","process_evidence":"proc","changed_lines":120}],"created_at":"2026-01-02T03:04:05Z","updated_at":"2026-01-02T04:05:06Z"}
`

// preChangeRecordRevision freezes the content address of the fixture above
// as written before this change; it must never move, because an existing
// ledger's HEAD names exactly this hash.
const preChangeRecordRevision = "d1060a4cbf56e59c0ec84aa717d79934847ee6374a559fceca36441b82047fbe"

// requireNoGenerationFields fails when a generation-1 record carries any of
// the new members: generation 1 must be represented by field absence, never
// by an explicit 1 or an empty provenance slice.
func requireNoGenerationFields(t *testing.T, record []byte) {
	t.Helper()
	for _, key := range []string{`"generation"`, `"advances"`, `"objective_generation"`} {
		if bytes.Contains(record, []byte(key)) {
			t.Fatalf("generation-1 record carries %s: %s", key, record)
		}
	}
}

// TestCASContract_PreChangeRecordReCanonicalizesUnchanged pins the #1 risk of
// this change: a record serialized WITHOUT the new fields must
// re-canonicalize to identical bytes and still pass content-address
// verification through the real load path. The read-time generation and
// lifetime views are derived from the chain and must never reach canonical
// bytes.
func TestCASContract_PreChangeRecordReCanonicalizesUnchanged(t *testing.T) {
	setStoreRoot(t)
	frozen := []byte(preChangeRecord)

	var parsed RuntimeStore
	if err := json.Unmarshal(frozen, &parsed); err != nil {
		t.Fatalf("parse pre-change record: %v", err)
	}

	// (a) Re-canonicalization is byte-stable: the current struct definition
	// serializes the old record exactly as it was written.
	rec := canonicalRecordPayload(&parsed)
	if !bytes.Equal(rec, frozen) {
		t.Fatalf("pre-change record re-canonicalizes to different bytes:\ngot  %s\nwant %s", rec, frozen)
	}
	revision := sha256Hex(rec)

	// The record is published exactly as an existing ledger holds it:
	// record-<content-address>.json plus HEAD.
	s, err := resolveStore("ch-cas-contract", "r")
	if err != nil {
		t.Fatalf("resolveStore: %v", err)
	}
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		t.Fatalf("mkdir ledger store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, recordFileName(revision)), frozen, 0644); err != nil {
		t.Fatalf("write record: %v", err)
	}
	if err := writeLedgerHead(s.Dir, revision); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}

	// The real read path must verify the frozen bytes without rejection.
	status, err := StatusWithInstance("ch-cas-contract", "r", "")
	if err != nil {
		t.Fatalf("StatusWithInstance of pre-change record: %v", err)
	}
	if !status.Complete || status.NextAction != "complete" {
		t.Fatalf("digested status = %+v, want the preserved complete generation-1 state", status)
	}
	if status.AttemptCount != 1 {
		t.Fatalf("AttemptCount = %d, want 1 (derived from the attempt chain)", status.AttemptCount)
	}
	// The complete-ledger classification survives the admission seam: the
	// passive probe reads the SAME story as the acquire path — the completed
	// work unit names its successor route — and the frozen bytes stay
	// untouched by the read.
	if status.BlockedReason != BlockedReasonWorkUnitComplete || status.BlockedExit != workUnitCompleteExit("apply") {
		t.Fatalf("admission probe = %q/%q, want work_unit_complete with the successor route", status.BlockedReason, status.BlockedExit)
	}

	store, _, err := loadStore("ch-cas-contract", "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}

	// (c) Generation and lifetime views are derived read-time. The stored
	// generation is absent (0) and only the derived view renders it as 1;
	// lifetime totals are computed from the immutable attempt chain.
	if store.Generation != 0 {
		t.Fatalf("stored Generation = %d, want 0 (absent on the wire)", store.Generation)
	}
	if derived := max(store.Generation, 1); derived != 1 {
		t.Fatalf("derived generation = %d, want 1", derived)
	}
	if len(store.Advances) != 0 {
		t.Fatalf("stored Advances = %+v, want empty on generation 1", store.Advances)
	}
	lifetimeAttempts := len(store.Attempts)
	lifetimeLines := 0
	for _, attempt := range store.Attempts {
		lifetimeLines += attempt.ChangedLines
		if attempt.ObjectiveGeneration != 0 {
			t.Fatalf("attempt %d ObjectiveGeneration = %d, want 0 (absent)", attempt.Ordinal, attempt.ObjectiveGeneration)
		}
	}
	if lifetimeAttempts != status.AttemptCount {
		t.Fatalf("lifetime attempts %d != status.AttemptCount %d", lifetimeAttempts, status.AttemptCount)
	}
	if lifetimeLines != 120 {
		t.Fatalf("lifetime changed lines = %d, want 120", lifetimeLines)
	}

	// The derived views never reach canonical bytes: the record on disk is
	// still byte-identical and still hashes to its frozen content address.
	raw, err := os.ReadFile(filepath.Join(s.Dir, recordFileName(revision)))
	if err != nil {
		t.Fatalf("re-read record: %v", err)
	}
	if !bytes.Equal(raw, frozen) {
		t.Fatalf("record bytes changed after reads:\ngot  %s\nwant %s", raw, frozen)
	}
	if got := sha256Hex(canonicalRecordPayload(store)); got != preChangeRecordRevision {
		t.Fatalf("content address moved: got %s, want frozen %s", got, preChangeRecordRevision)
	}
	requireNoGenerationFields(t, raw)
}

// TestCASContract_CompleteLedgerClassification pins the classification of a
// complete ledger head. A legitimately completed work unit (complete, not
// decision-required, no active attempt, last attempt passed) reports
// work_unit_complete with the successor-naming exit — never corrupt_authority
// and never "reset required to continue" as the only way out — while a
// genuinely anomalous completion (decision-required, a dangling active
// attempt, or no passed attempt to name) keeps corrupt_authority. The
// acquire path and the passive status probe must tell the same story.
func TestCASContract_CompleteLedgerClassification(t *testing.T) {
	setStoreRoot(t)

	// Legitimate completion: one work unit settled passed.
	acq, err := Acquire(AcquireParams{
		ChangeName: "ch-probe-complete", RepoRoot: "r", RequestID: "probe-a",
		WorkUnit: "apply", EvidenceGoal: "goal",
	})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if _, err := Settle(SettleParams{
		ChangeName: "ch-probe-complete", RepoRoot: "r", Token: acq.Token,
		RequestID: "probe-a-settle", Outcome: "passed",
		EvidenceRevision: "sha256:" + strings.Repeat("a", 64), Diagnosis: "ok",
	}); err != nil {
		t.Fatalf("Settle: %v", err)
	}

	status, err := StatusWithInstance("ch-probe-complete", "r", "")
	if err != nil {
		t.Fatalf("StatusWithInstance: %v", err)
	}
	if status.BlockedReason != BlockedReasonWorkUnitComplete {
		t.Fatalf("complete probe reason = %q, want %q", status.BlockedReason, BlockedReasonWorkUnitComplete)
	}
	if status.BlockedExit != workUnitCompleteExit("apply") {
		t.Fatalf("complete probe exit = %q, want the successor-naming exit %q", status.BlockedExit, workUnitCompleteExit("apply"))
	}
	for _, want := range []string{
		`work unit "apply" is complete`,
		`--work-unit`,
		`with a different --work-unit`,
		`reset discards this scope instead of succeeding it`,
	} {
		if !strings.Contains(status.BlockedExit, want) {
			t.Fatalf("complete probe exit %q must name the successor route (%q)", status.BlockedExit, want)
		}
	}
	if status.SettleObligation != nil {
		t.Fatalf("complete probe obligation = %+v, want none", status.SettleObligation)
	}

	// Repeating the settled label gets the identical story from the acquire
	// path: same reason, same successor-naming exit as the probe.
	_, err = Acquire(AcquireParams{
		ChangeName: "ch-probe-complete", RepoRoot: "r", RequestID: "probe-b",
		WorkUnit: "apply", EvidenceGoal: "goal",
	})
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonWorkUnitComplete || blocked.Exit != status.BlockedExit {
		t.Fatalf("repeat refusal = %v, want the same %s story as the probe", err, BlockedReasonWorkUnitComplete)
	}

	// Anomalous completion 1: a dangling active attempt pointer.
	store, err := LoadStore("ch-probe-complete", "r")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	activeAnomaly := *store
	activeAnomaly.ActiveAttempt = 7
	if err := SaveStore(&activeAnomaly, "r"); err != nil {
		t.Fatalf("SaveStore dangling active attempt: %v", err)
	}
	assertCorruptAuthorityProbe(t, "ch-probe-complete", "r", "dangling active attempt")

	// Anomalous completion 2: a decision-required completion.
	store, err = LoadStore("ch-probe-complete", "r")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	decisionAnomaly := *store
	decisionAnomaly.DecisionRequired = true
	if err := SaveStore(&decisionAnomaly, "r"); err != nil {
		t.Fatalf("SaveStore decision-required completion: %v", err)
	}
	assertCorruptAuthorityProbe(t, "ch-probe-complete", "r", "decision-required completion")

	// Anomalous completion 3: no passed attempt to name.
	store, err = LoadStore("ch-probe-complete", "r")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	unpassedAnomaly := *store
	unpassedAnomaly.DecisionRequired = false
	unpassedAnomaly.Attempts = append([]RuntimeAttempt(nil), store.Attempts...)
	unpassedAnomaly.Attempts[len(unpassedAnomaly.Attempts)-1].Outcome = "failed"
	if err := SaveStore(&unpassedAnomaly, "r"); err != nil {
		t.Fatalf("SaveStore unpassed completion: %v", err)
	}
	assertCorruptAuthorityProbe(t, "ch-probe-complete", "r", "no passed attempt to name")
}

// assertCorruptAuthorityProbe requires the passive probe to keep the
// historical corrupt_authority classification for an anomalous completion.
func assertCorruptAuthorityProbe(t *testing.T, change, repoRoot, anomaly string) {
	t.Helper()
	status, err := StatusWithInstance(change, repoRoot, "")
	if err != nil {
		t.Fatalf("StatusWithInstance (%s): %v", anomaly, err)
	}
	if status.BlockedReason != BlockedReasonCorruptAuthority || status.BlockedExit != "ledger is complete; reset required to continue" {
		t.Fatalf("%s probe = %q/%q, want corrupt_authority with the historical exit", anomaly, status.BlockedReason, status.BlockedExit)
	}
}

// TestCASContract_GenerationOneRecordsStampNoNewFields pins the write side:
// every record a generation-1 ledger produces — through both canonical
// mutation families — carries none of the new members, so the ledger stays
// readable by binaries that predate the fields.
func TestCASContract_GenerationOneRecordsStampNoNewFields(t *testing.T) {
	setStoreRoot(t)

	acq, err := Acquire(AcquireParams{
		ChangeName: "ch-cas-gen1-acq", RepoRoot: "r", RequestID: "req-gen1-acq",
		WorkUnit: "apply", EvidenceGoal: "goal",
	})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if _, err := Settle(SettleParams{
		ChangeName: "ch-cas-gen1-acq", RepoRoot: "r", Token: acq.Token,
		RequestID: "req-gen1-settle", Outcome: "passed",
		EvidenceRevision: "sha256:" + strings.Repeat("1", 64), Diagnosis: "ok",
	}); err != nil {
		t.Fatalf("Settle: %v", err)
	}

	begin, err := Begin(BeginParams{
		ChangeName: "ch-cas-gen1-begin", RepoRoot: "r", RequestID: "req-gen1-begin",
		WorkUnit: "apply", EvidenceGoal: "goal",
	})
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if _, err := Finish(FinishParams{
		ChangeName: "ch-cas-gen1-begin", RepoRoot: "r",
		ExpectedRev: begin.Revision, Outcome: "passed", RequestID: "req-gen1-finish",
	}); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	for _, change := range []string{"ch-cas-gen1-acq", "ch-cas-gen1-begin"} {
		dir := filepath.Join(storeRootOverride, RuntimeVersion, change)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read store dir %s: %v", change, err)
		}
		records := 0
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, "record-") || !strings.HasSuffix(name, ".json") {
				continue
			}
			records++
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			requireNoGenerationFields(t, raw)

			var parsed RuntimeStore
			if err := json.Unmarshal(raw, &parsed); err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			// (b) The new members stay absent (omitempty) at generation 1, and
			// the canonical form still hashes to the record's content
			// address. Raw bytes are NOT compared to the canonical payload
			// here: a request receipt embeds its outcome revision, which the
			// content address deliberately excludes (see cas_store.go).
			if parsed.Generation != 0 || len(parsed.Advances) != 0 {
				t.Fatalf("record %s carries generation state: generation=%d advances=%d", name, parsed.Generation, len(parsed.Advances))
			}
			for _, attempt := range parsed.Attempts {
				if attempt.ObjectiveGeneration != 0 {
					t.Fatalf("record %s attempt %d carries objective_generation=%d", name, attempt.Ordinal, attempt.ObjectiveGeneration)
				}
			}
			revision := strings.TrimSuffix(strings.TrimPrefix(name, "record-"), ".json")
			if got := sha256Hex(canonicalRecordPayload(&parsed)); got != revision {
				t.Fatalf("record %s: canonical hash %s != content address %s", name, got, revision)
			}
			// (c) The derived read-time generation view renders 0 as 1
			// without touching the persisted bytes.
			if derived := max(parsed.Generation, 1); derived != 1 {
				t.Fatalf("record %s derived generation = %d, want 1", name, derived)
			}
		}
		if records != 2 {
			t.Fatalf("%s: record count = %d, want 2 (first mutation + terminal mutation)", change, records)
		}
	}
}
