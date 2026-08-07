package sddattempt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLegacyLedger writes a legacy-format home-dir ledger file (single JSON
// with an embedded "revision" field) for the given store state. The wrapper
// struct keeps every embedded RawMessage byte-exact (a map round-trip would
// reorder keys inside receipt outcomes and break the revision self-check).
func writeLegacyLedger(t *testing.T, root, changeName string, store *RuntimeStore) string {
	t.Helper()
	store.ChangeName = changeName
	rev := computeRevision(store)
	type legacyFile struct {
		Revision string `json:"revision"`
		RuntimeStore
	}
	payload, err := json.MarshalIndent(legacyFile{Revision: rev, RuntimeStore: *store}, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy file: %v", err)
	}
	path := filepath.Join(root, changeName+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}
	if err := os.WriteFile(path, payload, 0644); err != nil {
		t.Fatalf("write legacy ledger: %v", err)
	}
	return path
}

func TestMigration_ImportsLegacyLedgerOnce(t *testing.T) {
	root := setStoreRoot(t)

	// The legacy receipt's digest is computed from the exact params of the
	// request that will be replayed after migration (C1 semantics: a replay
	// with the same params converges; different params are rejected).
	replayParams := FinishParams{
		ChangeName: "ch-migrate", RepoRoot: "r",
		ExpectedRev: "legacy-head", Outcome: "passed", RequestID: "req-legacy",
	}
	legacyDigest := requestDigest(requestDigestDomainFinish, replayParams)

	// Pre-create a legacy ledger with an attempt and a recorded request.
	legacy := &RuntimeStore{
		ObjectiveID: "obj-legacy",
		MaxAttempts: 3,
		Attempts: []RuntimeAttempt{{
			Ordinal: 1, ObjectiveID: "obj-legacy", WorkUnit: "w",
			BeganAt: "2026-07-31T00:00:00Z", Outcome: "passed", EndedAt: "2026-07-31T01:00:00Z",
		}},
		ActiveAttempt: 0,
		Complete:      true,
		NextAction:    "complete",
		Requests: map[string]RuntimeRequestRecord{
			"req-legacy": {
				Operation:  opFinish,
				Digest:     legacyDigest,
				Outcome:    json.RawMessage(`{"revision":"legacy-head","complete":true}`),
				RecordedAt: "2026-07-31T01:00:00Z",
			},
		},
	}
	legacyPath := writeLegacyLedger(t, root, "ch-migrate", legacy)

	// First access migrates and reports it once.
	status, err := Status("ch-migrate", "r")
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if !status.Migrated {
		t.Fatal("first access must report migration")
	}
	if status.Complete != true || status.NextAction != "complete" {
		t.Fatalf("migrated status = %+v, want legacy state preserved", status)
	}

	// Records now live under the clone-scoped store dir.
	storeDir := filepath.Join(root, RuntimeVersion, "ch-migrate")
	if _, err := os.Stat(filepath.Join(storeDir, "HEAD")); err != nil {
		t.Fatalf("HEAD missing after migration: %v", err)
	}
	headData, err := os.ReadFile(filepath.Join(storeDir, "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headData))
	if _, err := os.Stat(filepath.Join(storeDir, "record-"+head+".json")); err != nil {
		t.Fatalf("initial record missing after migration: %v", err)
	}

	// Second access does not migrate again.
	status2, err := Status("ch-migrate", "r")
	if err != nil {
		t.Fatalf("Status() #2 error: %v", err)
	}
	if status2.Migrated {
		t.Fatal("second access must not re-report migration")
	}

	// The legacy file is kept untouched.
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy file must be kept untouched, got: %v", err)
	}

	// C1 idempotency map semantics survive migration: replaying the legacy
	// request with the same params returns its recorded outcome.
	result, err := Finish(replayParams)
	if err != nil {
		t.Fatalf("Finish(replay after migration) error: %v", err)
	}
	if !result.Complete {
		t.Fatalf("replayed legacy request = %+v, want recorded outcome", result)
	}
	if result.Migrated {
		t.Fatal("replay after migration must not re-report migration")
	}
}

func TestMigration_RejectsBrokenLegacyLedger(t *testing.T) {
	root := setStoreRoot(t)

	legacy := &RuntimeStore{ObjectiveID: "obj", MaxAttempts: 3}
	legacyPath := writeLegacyLedger(t, root, "ch-broken", legacy)

	// Corrupt the embedded revision so the self-check fails.
	var object map[string]any
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("read legacy: %v", err)
	}
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	object["revision"] = strings.Repeat("0", 64)
	payload, _ := json.MarshalIndent(object, "", "  ")
	if err := os.WriteFile(legacyPath, payload, 0644); err != nil {
		t.Fatalf("rewrite legacy: %v", err)
	}

	_, err = Status("ch-broken", "r")
	if err == nil || !strings.Contains(err.Error(), "revision check") {
		t.Fatalf("expected revision-check failure, got %v", err)
	}
	// Fail closed: nothing was migrated.
	if _, statErr := os.Stat(filepath.Join(root, RuntimeVersion, "ch-broken", "HEAD")); !os.IsNotExist(statErr) {
		t.Fatal("broken legacy ledger must not be migrated")
	}
}

func TestMigration_SkipsWhenStoreNonEmpty(t *testing.T) {
	root := setStoreRoot(t)

	// A store already exists under the new layout.
	first, err := Begin(BeginParams{ChangeName: "ch-existing", RepoRoot: "r", WorkUnit: "w"})
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	// Now a legacy file appears (e.g. restored from a backup).
	legacy := &RuntimeStore{ObjectiveID: "obj", MaxAttempts: 3}
	writeLegacyLedger(t, root, "ch-existing", legacy)

	status, err := Status("ch-existing", "r")
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if status.Migrated {
		t.Fatal("must not migrate over a non-empty store")
	}
	if status.Revision != first.Revision {
		t.Fatalf("revision changed: got %s, want %s", status.Revision, first.Revision)
	}
}

func TestCAS_RecordsAreContentAddressed(t *testing.T) {
	setStoreRoot(t)

	begin, err := Begin(BeginParams{ChangeName: "ch-cas", RepoRoot: "r", WorkUnit: "w"})
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	if _, err := Finish(FinishParams{ChangeName: "ch-cas", RepoRoot: "r", ExpectedRev: begin.Revision, Outcome: "passed"}); err != nil {
		t.Fatalf("Finish() error: %v", err)
	}

	storeDir := filepath.Join(storeRootOverride, RuntimeVersion, "ch-cas")
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		t.Fatalf("read store dir: %v", err)
	}
	recordCount := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "record-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		recordCount++
		revision := strings.TrimSuffix(strings.TrimPrefix(name, "record-"), ".json")
		data, err := os.ReadFile(filepath.Join(storeDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		// The canonical form must hash to the file name.
		var store RuntimeStore
		if err := json.Unmarshal(data, &store); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if got := sha256Hex(canonicalRecordPayload(&store)); got != revision {
			t.Fatalf("record %s does not match its content address (got %s)", name, got)
		}
	}
	if recordCount != 2 {
		t.Fatalf("record count = %d, want 2 (begin + finish)", recordCount)
	}

	// Replay matches the legacy semantics: status derives from the head record.
	status, err := Status("ch-cas", "r")
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if !status.Complete || status.NextAction != "complete" {
		t.Fatalf("status = %+v, want complete", status)
	}
}

func TestCAS_TamperedRecordFailsClosed(t *testing.T) {
	setStoreRoot(t)

	if _, err := Begin(BeginParams{ChangeName: "ch-tamper", RepoRoot: "r", WorkUnit: "w"}); err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	storeDir := filepath.Join(storeRootOverride, RuntimeVersion, "ch-tamper")
	headData, err := os.ReadFile(filepath.Join(storeDir, "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headData))
	recordPath := filepath.Join(storeDir, "record-"+head+".json")
	if err := os.WriteFile(recordPath, []byte("{tampered"), 0644); err != nil {
		t.Fatalf("tamper record: %v", err)
	}

	if _, err := Status("ch-tamper", "r"); err == nil {
		t.Fatal("tampered record must fail closed")
	}
}

func TestCAS_StaleExpectedRevisionConflicts(t *testing.T) {
	setStoreRoot(t)

	begin, err := Begin(BeginParams{ChangeName: "ch-cas2", RepoRoot: "r", WorkUnit: "w"})
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	if _, err := Finish(FinishParams{ChangeName: "ch-cas2", RepoRoot: "r", ExpectedRev: begin.Revision, Outcome: "passed"}); err != nil {
		t.Fatalf("Finish() error: %v", err)
	}

	// A stale expected revision must conflict.
	_, err = Begin(BeginParams{ChangeName: "ch-cas2", RepoRoot: "r", WorkUnit: "w2", ExpectedRev: begin.Revision})
	if err == nil || !strings.Contains(err.Error(), "CAS conflict") {
		t.Fatalf("expected CAS conflict, got %v", err)
	}
}

func TestCAS_EmbeddedReceiptRevisionMatchesRecord(t *testing.T) {
	setStoreRoot(t)

	first, err := Begin(BeginParams{ChangeName: "ch-receipt", RepoRoot: "r", WorkUnit: "w", RequestID: "req-embed"})
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}

	// The committed record's embedded receipt outcome must carry the record's
	// own revision (C1 convergent-replay guarantee).
	storeDir := filepath.Join(storeRootOverride, RuntimeVersion, "ch-receipt")
	headData, err := os.ReadFile(filepath.Join(storeDir, "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headData))
	data, err := os.ReadFile(filepath.Join(storeDir, "record-"+head+".json"))
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	var store RuntimeStore
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatalf("parse record: %v", err)
	}
	receipt, ok := store.Requests["req-embed"]
	if !ok {
		t.Fatal("request receipt missing from committed record")
	}
	var outcome BeginResult
	if err := json.Unmarshal(receipt.Outcome, &outcome); err != nil {
		t.Fatalf("parse receipt outcome: %v", err)
	}
	if outcome.Revision != first.Revision || outcome.Revision != head {
		t.Fatalf("embedded receipt revision %s != first result revision %s != record revision %s",
			outcome.Revision, first.Revision, head)
	}
}

func TestStore_OutsideGitRepoFallsBackToMachineScope(t *testing.T) {
	// No storeRootOverride: resolution uses the real git common dir.
	// Outside a git repository the runtime attempt authority must fall back
	// to the machine-scoped ledger instead of failing closed.
	redirectHome(t)
	dir := t.TempDir()

	status, err := Status("ch-nogit", dir)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if status.Scope != ScopeMachine {
		t.Fatalf("status scope = %q, want %q", status.Scope, ScopeMachine)
	}
	if status.NextAction != "begin" {
		t.Fatalf("status next action = %q, want begin", status.NextAction)
	}

	begin, err := Begin(BeginParams{ChangeName: "ch-nogit", RepoRoot: dir, WorkUnit: "w"})
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	if begin.Scope != ScopeMachine {
		t.Fatalf("begin scope = %q, want %q", begin.Scope, ScopeMachine)
	}
	if begin.ActiveAttempt != 1 {
		t.Fatalf("active attempt = %d, want 1", begin.ActiveAttempt)
	}

	finish, err := Finish(FinishParams{ChangeName: "ch-nogit", RepoRoot: dir, ExpectedRev: begin.Revision, Outcome: "passed"})
	if err != nil {
		t.Fatalf("Finish() error: %v", err)
	}
	if finish.Scope != ScopeMachine {
		t.Fatalf("finish scope = %q, want %q", finish.Scope, ScopeMachine)
	}
	if !finish.Complete {
		t.Fatal("finish must complete the ledger")
	}

	status2, err := Status("ch-nogit", dir)
	if err != nil {
		t.Fatalf("Status() after finish error: %v", err)
	}
	if !status2.Complete || status2.Scope != ScopeMachine {
		t.Fatalf("status after finish = %+v, want complete + machine scope", status2)
	}

	// The ledger lives under the machine-scoped fallback dir with the same
	// layout: HEAD + content-addressed record.
	storeDir := machineStoreDir(t, "ch-nogit")
	headData, err := os.ReadFile(filepath.Join(storeDir, "HEAD"))
	if err != nil {
		t.Fatalf("read machine-scoped HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headData))
	if _, err := os.Stat(filepath.Join(storeDir, "record-"+head+".json")); err != nil {
		t.Fatalf("machine-scoped record missing: %v", err)
	}
}
