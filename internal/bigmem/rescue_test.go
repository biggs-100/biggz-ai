package bigmem

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

// helper to insert orphan session directly
func insertOrphanSession(t *testing.T, s *Store, id string) {
	t.Helper()
	// Ensure sessions table exists
	s.db.Exec(`CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT, parent_id TEXT REFERENCES sessions(id) ON DELETE SET NULL, leaf_id TEXT, branch_summary TEXT)`)
	_, err := s.db.Exec(`INSERT INTO sessions (id, start_time, project, leaf_id) VALUES (?, datetime('now'), NULL, ?)`, id, id)
	if err != nil {
		t.Fatalf("insert orphan session %q: %v", id, err)
	}
}

func insertSessionWithProject(t *testing.T, s *Store, id, project string) {
	t.Helper()
	s.db.Exec(`CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT, parent_id TEXT REFERENCES sessions(id) ON DELETE SET NULL, leaf_id TEXT, branch_summary TEXT)`)
	if project == "" {
		_, err := s.db.Exec(`INSERT INTO sessions (id, start_time, project, leaf_id) VALUES (?, datetime('now'), NULL, ?)`, id, id)
		if err != nil {
			t.Fatalf("insert session %q: %v", id, err)
		}
		return
	}
	_, err := s.db.Exec(`INSERT INTO sessions (id, start_time, project, leaf_id) VALUES (?, datetime('now'), ?, ?)`, id, project, id)
	if err != nil {
		t.Fatalf("insert session %q project %q: %v", id, project, err)
	}
}

func sessionProject(t *testing.T, s *Store, id string) string {
	t.Helper()
	var p sql.NullString
	err := s.db.QueryRow(`SELECT project FROM sessions WHERE id = ?`, id).Scan(&p)
	if err != nil {
		t.Fatalf("query session project %q: %v", id, err)
	}
	if !p.Valid {
		return ""
	}
	return p.String
}

// ─── 1.1-1.5 Foundation ─────────────────────────────────────────────────

func TestResolveWrite_AdoptsOrphan(t *testing.T) {
	s := openTestStore(t)
	insertOrphanSession(t, s, "sess-orphan")
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := s.resolveWriteProjectTx(tx, "sess-orphan", "projA"); err != nil {
		_ = tx.Rollback()
		t.Fatalf("resolveWriteProjectTx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := sessionProject(t, s, "sess-orphan"); got != "proja" {
		t.Errorf("adopted project = %q, want %q", got, "proja")
	}
}

func TestResolveWrite_NoOp(t *testing.T) {
	s := openTestStore(t)
	insertSessionWithProject(t, s, "sess-owned", "projA")
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := s.resolveWriteProjectTx(tx, "sess-owned", "projA"); err != nil {
		_ = tx.Rollback()
		t.Fatalf("no-op should not error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := sessionProject(t, s, "sess-owned"); got != "projA" {
		t.Errorf("project after no-op = %q, want projA", got)
	}
	// Also test same normalized different case
	tx2, _ := s.db.Begin()
	if err := s.resolveWriteProjectTx(tx2, "sess-owned", "ProjA"); err != nil {
		_ = tx2.Rollback()
		t.Fatalf("case-insensitive no-op failed: %v", err)
	}
	_ = tx2.Commit()
}

func TestForeign_BlocksAmbiguous(t *testing.T) {
	s := openTestStore(t)
	insertOrphanSession(t, s, "sess-foreign")
	// Add foreign observation
	obs := &Observation{Title: "Foreign", Type: "decision", Content: "foreign content", Project: "other", SessionID: "sess-foreign"}
	if err := s.Save(obs); err != nil {
		t.Fatalf("save foreign obs: %v", err)
	}
	// Ensure session still orphan before resolve
	// Reset session to orphan (Save may have adopted? But our Save would have tried to adopt via resolveWriteProjectTx before saving foreign obs?
	// For this test, we need orphan session with foreign observation where requested project != foreign.
	// We created foreign observation with project "other" and session "sess-foreign" — Save would have adopted sess-foreign to "other" earlier, so we need to reset orphan.
	s.db.Exec(`UPDATE sessions SET project = NULL WHERE id = ?`, "sess-foreign")
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	err = s.resolveWriteProjectTx(tx, "sess-foreign", "projA")
	if err == nil {
		_ = tx.Rollback()
		t.Fatal("expected ErrProjectOwnershipAmbiguous, got nil")
	}
	if !errors.Is(err, ErrProjectOwnershipAmbiguous) {
		_ = tx.Rollback()
		t.Fatalf("expected ErrProjectOwnershipAmbiguous, got %v", err)
	}
	if want := "biggz bigmem rescue-ownership --project proja --session sess-foreign"; err != nil && !contains(err.Error(), want) {
		_ = tx.Rollback()
		t.Errorf("error %q should contain %q", err.Error(), want)
	}
	_ = tx.Rollback()
	// Verify no mutation
	if got := sessionProject(t, s, "sess-foreign"); got != "" {
		t.Errorf("foreign blocked should not mutate project, got %q", got)
	}
}

func TestAdopt_SyncProbe(t *testing.T) {
	s := openTestStore(t)
	insertOrphanSession(t, s, "sess-sync")
	// Without sync_mutations table, adopt should succeed no-op
	tx, _ := s.db.Begin()
	if err := s.adoptSessionOwnershipTx(tx, "sess-sync", "projA"); err != nil {
		_ = tx.Rollback()
		t.Fatalf("adopt without sync table: %v", err)
	}
	_ = tx.Commit()
	if got := sessionProject(t, s, "sess-sync"); got != "projA" {
		t.Errorf("adopt without sync = %q, want projA", got)
	}
	// With sync_mutations table, adopt should also enqueue
	s2 := openTestStore(t)
	insertOrphanSession(t, s2, "sess-sync2")
	// Create sync_mutations with expected columns
	_, err := s2.db.Exec(`CREATE TABLE sync_mutations (id TEXT PRIMARY KEY, entity TEXT, entity_key TEXT, op TEXT, project TEXT, created_at TEXT)`)
	if err != nil {
		t.Fatalf("create sync_mutations: %v", err)
	}
	tx2, _ := s2.db.Begin()
	if err := s2.adoptSessionOwnershipTx(tx2, "sess-sync2", "projA"); err != nil {
		_ = tx2.Rollback()
		t.Fatalf("adopt with sync: %v", err)
	}
	_ = tx2.Commit()
	var cnt int
	_ = s2.db.QueryRow(`SELECT COUNT(*) FROM sync_mutations WHERE entity_key = ?`, "sess-sync2").Scan(&cnt)
	if cnt != 1 {
		t.Errorf("sync_mutations enqueue count = %d, want 1", cnt)
	}
	if got := sessionProject(t, s2, "sess-sync2"); got != "projA" {
		t.Errorf("adopt with sync project = %q, want projA", got)
	}
	// Without columns, probe tolerates absence
	s3 := openTestStore(t)
	insertOrphanSession(t, s3, "sess-sync3")
	_, _ = s3.db.Exec(`CREATE TABLE sync_mutations (id TEXT PRIMARY KEY, nonsense TEXT)`)
	tx3, _ := s3.db.Begin()
	if err := s3.adoptSessionOwnershipTx(tx3, "sess-sync3", "projA"); err != nil {
		_ = tx3.Rollback()
		t.Fatalf("adopt with weird sync schema should not error: %v", err)
	}
	_ = tx3.Commit()
	if got := sessionProject(t, s3, "sess-sync3"); got != "projA" {
		t.Errorf("adopt with weird sync = %q, want projA", got)
	}
}

// ─── 2.x Bulk Rescue ───────────────────────────────────────────────────

func TestPlan_DryRunMatchesApply(t *testing.T) {
	s := openTestStore(t)
	insertOrphanSession(t, s, "plan-1")
	insertOrphanSession(t, s, "plan-2")
	insertSessionWithProject(t, s, "plan-owned", "projA")
	// Add foreign for one orphan
	insertOrphanSession(t, s, "plan-amb")
	fObs := &Observation{Title: "Amb foreign", Type: "note", Content: "amb", Project: "other", SessionID: "plan-amb"}
	_ = s.Save(fObs)
	// Reset amb to orphan (since Save adopted to other)
	s.db.Exec(`UPDATE sessions SET project = NULL WHERE id = ?`, "plan-amb")
	plan, err := s.PlanRescue("projA")
	if err != nil {
		t.Fatalf("PlanRescue: %v", err)
	}
	if len(plan.Adoptable) != 2 {
		t.Errorf("Plan adoptable = %d, want 2 (plan-1, plan-2) got %v", len(plan.Adoptable), plan.Adoptable)
	}
	if len(plan.Ambiguous) != 1 {
		t.Errorf("Plan ambiguous = %d, want 1 got %v", len(plan.Ambiguous), plan.Ambiguous)
	}
	// Dry-run result should match plan
	dry, err := s.RescueNullProjectOwnership("projA", RescueOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if dry.Adopted != len(plan.Adoptable) {
		t.Errorf("dry Adopted %d != plan adoptable %d", dry.Adopted, len(plan.Adoptable))
	}
	if len(dry.Ambiguous) != len(plan.Ambiguous) {
		t.Errorf("dry ambiguous %d != plan %d", len(dry.Ambiguous), len(plan.Ambiguous))
	}
	// Apply
	res, err := s.RescueNullProjectOwnership("projA", RescueOptions{})
	if err != nil {
		t.Fatalf("Rescue: %v", err)
	}
	if res.Adopted != len(plan.Adoptable) {
		t.Errorf("apply Adopted %d != plan %d", res.Adopted, len(plan.Adoptable))
	}
	if len(res.Ambiguous) != len(plan.Ambiguous) {
		t.Errorf("apply ambiguous %d != plan %d", len(res.Ambiguous), len(plan.Ambiguous))
	}
	// Verify DB
	if got := sessionProject(t, s, "plan-1"); got != "proja" {
		t.Errorf("plan-1 project %q, want proja", got)
	}
	if got := sessionProject(t, s, "plan-2"); got != "proja" {
		t.Errorf("plan-2 project %q, want proja", got)
	}
	if got := sessionProject(t, s, "plan-amb"); got != "" {
		t.Errorf("ambiguous should stay orphan, got %q", got)
	}
	// Plan after apply should have 0 adoptable (already adopted) + 1 ambiguous
	plan2, _ := s.PlanRescue("projA")
	if len(plan2.Adoptable) != 0 {
		t.Errorf("after apply plan adoptable %d, want 0", len(plan2.Adoptable))
	}
}

func TestRescue_BulkAdoptsN(t *testing.T) {
	s := openTestStore(t)
	const n = 5
	for i := 0; i < n; i++ {
		insertOrphanSession(t, s, "bulk-"+string(rune('0'+i)))
	}
	res, err := s.RescueNullProjectOwnership("projA", RescueOptions{})
	if err != nil {
		t.Fatalf("bulk rescue: %v", err)
	}
	if res.Adopted != n {
		t.Errorf("bulk adopted %d, want %d", res.Adopted, n)
	}
	for i := 0; i < n; i++ {
		id := "bulk-" + string(rune('0'+i))
		if got := sessionProject(t, s, id); got != "proja" {
			t.Errorf("bulk %q project %q, want proja", id, got)
		}
	}
}

func TestRescue_UnknownExcluded(t *testing.T) {
	s := openTestStore(t)
	insertOrphanSession(t, s, "unknown-orphan")
	insertSessionWithProject(t, s, "sess-unknown", "unknown")
	// Ensure orphan counted, unknown not
	plan, err := s.PlanRescue("projA")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, id := range plan.Adoptable {
		if id == "sess-unknown" {
			t.Errorf("unknown session should not be adoptable")
		}
	}
	for _, a := range plan.Ambiguous {
		if a.SessionID == "sess-unknown" {
			t.Errorf("unknown should not be ambiguous either")
		}
	}
	res, err := s.RescueNullProjectOwnership("projA", RescueOptions{})
	if err != nil {
		t.Fatalf("rescue: %v", err)
	}
	if got := sessionProject(t, s, "sess-unknown"); got != "unknown" {
		t.Errorf("unknown session project = %q, want unknown", got)
	}
	// Orphan should be adopted
	if got := sessionProject(t, s, "unknown-orphan"); got != "proja" {
		t.Errorf("orphan should be adopted, got %q", got)
	}
	// Adopted count should be 1, not 2
	if res.Adopted != 1 {
		t.Errorf("adopted %d, want 1 (unknown excluded)", res.Adopted)
	}
}

func TestRescue_ScopedSession(t *testing.T) {
	s := openTestStore(t)
	insertOrphanSession(t, s, "scoped-a")
	insertOrphanSession(t, s, "scoped-b")
	res, err := s.RescueNullProjectOwnership("projA", RescueOptions{SessionID: "scoped-a"})
	if err != nil {
		t.Fatalf("scoped rescue: %v", err)
	}
	if res.Adopted != 1 {
		t.Errorf("scoped adopted %d, want 1", res.Adopted)
	}
	if got := sessionProject(t, s, "scoped-a"); got != "proja" {
		t.Errorf("scoped-a %q, want proja", got)
	}
	if got := sessionProject(t, s, "scoped-b"); got != "" {
		t.Errorf("scoped-b should remain orphan, got %q", got)
	}
	// Dry-run scoped should not mutate
	s2 := openTestStore(t)
	insertOrphanSession(t, s2, "scoped-dry")
	dry, err := s2.RescueNullProjectOwnership("projA", RescueOptions{SessionID: "scoped-dry", DryRun: true})
	if err != nil {
		t.Fatalf("scoped dry: %v", err)
	}
	if dry.Adopted != 1 {
		t.Errorf("scoped dry adopted %d, want 1", dry.Adopted)
	}
	if got := sessionProject(t, s2, "scoped-dry"); got != "" {
		t.Errorf("dry-run should not mutate, got %q", got)
	}
	// Scoped with foreign should be ambiguous
	s3 := openTestStore(t)
	insertOrphanSession(t, s3, "scoped-amb")
	fObs := &Observation{Title: "X", Type: "note", Content: "x", Project: "other", SessionID: "scoped-amb"}
	_ = s3.Save(fObs)
	s3.db.Exec(`UPDATE sessions SET project = NULL WHERE id = ?`, "scoped-amb")
	res3, err := s3.RescueNullProjectOwnership("projA", RescueOptions{SessionID: "scoped-amb"})
	if err != nil {
		t.Fatalf("scoped amb: %v", err)
	}
	if len(res3.Ambiguous) != 1 || res3.Adopted != 0 {
		t.Errorf("scoped amb result adopted %d ambiguous %d, want 0,1", res3.Adopted, len(res3.Ambiguous))
	}
}

// ─── 3.x Save integration ────────────────────────────────────────────────

func TestSave_Resolves(t *testing.T) {
	s := openTestStore(t)
	insertOrphanSession(t, s, "sess-save")
	obs := &Observation{Title: "Save resolves", Type: "decision", Content: "content for save resolves", Project: "projA", SessionID: "sess-save"}
	if err := s.Save(obs); err != nil {
		t.Fatalf("Save resolves: %v", err)
	}
	if got := sessionProject(t, s, "sess-save"); got != "proja" {
		t.Errorf("Save should adopt session, got %q want proja", got)
	}
	// Verify observation persisted
	gotObs, err := s.Get(obs.ID)
	if err != nil {
		t.Fatalf("Get after Save: %v", err)
	}
	if gotObs.Project != "proja" {
		t.Errorf("saved obs project %q, want proja", gotObs.Project)
	}
	// Ambiguous Save should fail with hint and not mutate
	s2 := openTestStore(t)
	insertOrphanSession(t, s2, "sess-save-amb")
	fObs := &Observation{Title: "Foreign2", Type: "note", Content: "foreign2", Project: "other", SessionID: "sess-save-amb"}
	_ = s2.Save(fObs)
	s2.db.Exec(`UPDATE sessions SET project = NULL WHERE id = ?`, "sess-save-amb")
	obs2 := &Observation{Title: "Should fail", Type: "decision", Content: "fail content", Project: "projA", SessionID: "sess-save-amb"}
	err = s2.Save(obs2)
	if err == nil {
		t.Fatal("ambiguous Save should error")
	}
	if !errors.Is(err, ErrProjectOwnershipAmbiguous) {
		t.Fatalf("expected ErrProjectOwnershipAmbiguous, got %v", err)
	}
	if !contains(err.Error(), "biggz bigmem rescue-ownership --project proja --session sess-save-amb") {
		t.Errorf("ambiguous error %q missing hint", err.Error())
	}
	if got := sessionProject(t, s2, "sess-save-amb"); got != "" {
		t.Errorf("ambiguous Save should not mutate session, got %q", got)
	}
}

func TestSave_ConcurrentSerialized(t *testing.T) {
	s := openTestStore(t)
	insertOrphanSession(t, s, "sess-conc")
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			defer wg.Done()
			obs := &Observation{Title: "Concurrent", Type: "decision", Content: "conc content", Project: "projA", SessionID: "sess-conc", TopicKey: ""}
			// Ensure unique ID to avoid dedup collision; Save generates ID if empty, but we give distinct titles?
			// Use non-deterministic ID by not setting ID, but content same would trigger hash dedup? Use unique content per goroutine
			obs.Content = "conc content " + string(rune('0'+idx))
			obs.Title = "Conc " + string(rune('0'+idx))
			errs[idx] = s.Save(obs)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("concurrent Save %d error: %v", i, err)
		}
	}
	if got := sessionProject(t, s, "sess-conc"); got != "proja" {
		t.Errorf("concurrent final project %q, want proja", got)
	}
}

func TestCLI_JSON(t *testing.T) {
	s := openTestStore(t)
	insertOrphanSession(t, s, "cli-1")
	insertOrphanSession(t, s, "cli-2")
	res, err := s.RescueNullProjectOwnership("projA", RescueOptions{})
	if err != nil {
		t.Fatalf("rescue: %v", err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out RescueResult
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Adopted != 2 {
		t.Errorf("JSON adopted %d, want 2", out.Adopted)
	}
	if !json.Valid(data) {
		t.Error("JSON invalid")
	}
}

func TestCLI_DryRunNoMutation(t *testing.T) {
	s := openTestStore(t)
	insertOrphanSession(t, s, "dry-1")
	insertOrphanSession(t, s, "dry-2")
	res, err := s.RescueNullProjectOwnership("projA", RescueOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if res.Adopted != 2 {
		t.Errorf("dry adopted %d, want 2", res.Adopted)
	}
	// Verify no mutation
	for _, id := range []string{"dry-1", "dry-2"} {
		if got := sessionProject(t, s, id); got != "" {
			t.Errorf("dry-run should not mutate %q, got %q", id, got)
		}
	}
	data, _ := json.Marshal(res)
	if !json.Valid(data) {
		t.Error("dry JSON invalid")
	}
	// Apply after dry should adopt same count
	res2, _ := s.RescueNullProjectOwnership("projA", RescueOptions{})
	if res2.Adopted != res.Adopted {
		t.Errorf("apply after dry adopted %d != dry %d", res2.Adopted, res.Adopted)
	}
}

func TestCLI_Scoped(t *testing.T) {
	s := openTestStore(t)
	insertOrphanSession(t, s, "scoped-cli-a")
	insertOrphanSession(t, s, "scoped-cli-b")
	res, err := s.RescueNullProjectOwnership("projA", RescueOptions{SessionID: "scoped-cli-a", DryRun: true})
	if err != nil {
		t.Fatalf("scoped dry: %v", err)
	}
	if res.Adopted != 1 {
		t.Errorf("scoped dry adopted %d, want 1", res.Adopted)
	}
	if got := sessionProject(t, s, "scoped-cli-a"); got != "" {
		t.Errorf("scoped dry should not mutate, got %q", got)
	}
	data, _ := json.Marshal(res)
	var out RescueResult
	_ = json.Unmarshal(data, &out)
	if out.Adopted != 1 {
		t.Errorf("scoped JSON adopted %d, want 1", out.Adopted)
	}
}

// helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
