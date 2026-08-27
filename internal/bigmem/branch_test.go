package bigmem

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ─── REQ-B1: Fresh schema ────────────────────────────────────────────────────

func TestBranchSchema_FreshDB(t *testing.T) {
	s := openTestStore(t)
	// PRAGMA table_info must include branching cols.
	rows, err := s.db.Query("PRAGMA table_info(sessions)")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		cols[name] = true
	}
	for _, want := range []string{"parent_id", "leaf_id", "branch_summary"} {
		if !cols[want] {
			t.Errorf("missing column %q in fresh DB sessions", want)
		}
	}
	// Indexes
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_sessions_parent_id'").Scan(&count)
	if count != 1 {
		t.Errorf("idx_sessions_parent_id missing, count=%d", count)
	}
	_ = s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_sessions_leaf_id'").Scan(&count)
	if count != 1 {
		t.Errorf("idx_sessions_leaf_id missing, count=%d", count)
	}
}

func TestBranchRoot_LeafSelf(t *testing.T) {
	s := openTestStore(t)
	root, err := s.CreateBranch("", "")
	if err != nil {
		t.Fatalf("CreateBranch root: %v", err)
	}
	if root.ParentID != nil {
		t.Errorf("root ParentID = %v, want nil", *root.ParentID)
	}
	if root.LeafID != root.ID {
		t.Errorf("root LeafID = %q, want %q", root.LeafID, root.ID)
	}
	// Verify DB state: parent_id IS NULL, leaf_id == id
	var parent sql.NullString
	var leaf string
	if err := s.db.QueryRow("SELECT parent_id, leaf_id FROM sessions WHERE id = ?", root.ID).Scan(&parent, &leaf); err != nil {
		t.Fatalf("query root: %v", err)
	}
	if parent.Valid {
		t.Errorf("DB parent_id = %q, want NULL", parent.String)
	}
	if leaf != root.ID {
		t.Errorf("DB leaf_id = %q, want %q", leaf, root.ID)
	}
}

// ─── REQ-B2: Legacy migration ────────────────────────────────────────────────

func TestBranchLegacyMigration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "bigmem.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE sessions (id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT)`)
	if err != nil {
		t.Fatalf("create legacy sessions: %v", err)
	}
	_, err = db.Exec(`INSERT INTO sessions (id, start_time, project) VALUES ('sess-legacy-1', '2026-01-01T00:00:00Z', 'proj'), ('sess-legacy-2', '2026-01-02T00:00:00Z', 'proj')`)
	if err != nil {
		t.Fatalf("insert legacy: %v", err)
	}
	db.Close()

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open legacy: %v", err)
	}
	defer s.Close()

	// Doctor should flag missing branching cols before fix? Open already migrated via migrateSchema, so not missing.
	// To test Doctor flag, create true legacy without reopen迁移: use raw DB.
	dir2 := t.TempDir()
	dbPath2 := filepath.Join(dir2, "bigmem.db")
	db2, _ := sql.Open("sqlite", dbPath2)
	_, _ = db2.Exec(`CREATE TABLE sessions (id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT)`)
	_, _ = db2.Exec(`INSERT INTO sessions (id, start_time, project) VALUES ('a', '2026-01-01T00:00:00Z', 'p'), ('b', '2026-01-02T00:00:00Z', 'p')`)
	_, _ = db2.Exec(`CREATE TABLE IF NOT EXISTS observations (id TEXT PRIMARY KEY, title TEXT, type TEXT, content TEXT, project TEXT, created_at TEXT, updated_at TEXT)`)
	db2.Close()
	// Open via raw Store without migrate? Directly test hasMissingBranchColumns
	rawDB, _ := sql.Open("sqlite", dbPath2)
	if !hasMissingBranchColumns(rawDB) {
		t.Error("hasMissingBranchColumns should be true for legacy DB")
	}
	rawDB.Close()

	// Now DoctorFix should backfill leaf_id=id
	if err := s.DoctorFix(); err != nil {
		t.Fatalf("DoctorFix: %v", err)
	}
	for _, id := range []string{"sess-legacy-1", "sess-legacy-2"} {
		var parent sql.NullString
		var leaf sql.NullString
		if err := s.db.QueryRow("SELECT parent_id, leaf_id FROM sessions WHERE id = ?", id).Scan(&parent, &leaf); err != nil {
			t.Fatalf("query %s: %v", id, err)
		}
		if parent.Valid && parent.String != "" {
			t.Errorf("%s parent_id = %q, want NULL", id, parent.String)
		}
		if !leaf.Valid || leaf.String != id {
			t.Errorf("%s leaf_id = %q, want %q", id, leaf.String, id)
		}
	}
}

func TestBranchMigrationIdempotent(t *testing.T) {
	s := openTestStore(t)
	// Insert two sessions via CreateBranch then DoctorFix twice
	_, _ = s.CreateBranch("", "root1")
	_, _ = s.CreateBranch("", "root2")
	var before int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&before)
	if err := s.DoctorFix(); err != nil {
		t.Fatalf("first DoctorFix: %v", err)
	}
	var mid int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&mid)
	if err := s.DoctorFix(); err != nil {
		t.Fatalf("second DoctorFix: %v", err)
	}
	var after int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&after)
	if before != mid || mid != after {
		t.Errorf("row count changed: before %d mid %d after %d, want unchanged", before, mid, after)
	}
}

// ─── REQ-B3: CRUD ────────────────────────────────────────────────────────────

func TestBranchCreateChild(t *testing.T) {
	s := openTestStore(t)
	root, _ := s.CreateBranch("", "")
	child, err := s.CreateBranch(root.ID, "fix")
	if err != nil {
		t.Fatalf("CreateBranch child: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Errorf("child ParentID = %v, want %q", child.ParentID, root.ID)
	}
	if child.BranchSummary != "fix" {
		t.Errorf("child BranchSummary = %q, want %q", child.BranchSummary, "fix")
	}
	// DB verify parent_id
	var pdb sql.NullString
	_ = s.db.QueryRow("SELECT parent_id FROM sessions WHERE id = ?", child.ID).Scan(&pdb)
	if !pdb.Valid || pdb.String != root.ID {
		t.Errorf("DB parent_id = %q, want %q", pdb.String, root.ID)
	}
}

func TestBranchCreateMissingParent(t *testing.T) {
	s := openTestStore(t)
	_, err := s.CreateBranch("missing-parent-xyz", "x")
	if err == nil {
		t.Fatal("expected error for missing parent, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want contains 'not found'", err.Error())
	}
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE parent_id = ?", "missing-parent-xyz").Scan(&count)
	if count != 0 {
		t.Errorf("count = %d, want 0 rows for missing parent", count)
	}
}

func TestBranchListGetChain(t *testing.T) {
	s := openTestStore(t)
	a, _ := s.CreateBranch("", "a")
	b, _ := s.CreateBranch(a.ID, "b")
	c, _ := s.CreateBranch(b.ID, "c")
	list, err := s.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}
	if len(list) < 3 {
		t.Fatalf("list len = %d, want >=3", len(list))
	}
	// Filter to our chain
	found := map[string]bool{}
	for _, sess := range list {
		if sess.ID == a.ID || sess.ID == b.ID || sess.ID == c.ID {
			found[sess.ID] = true
		}
	}
	if len(found) != 3 {
		t.Errorf("list missing chain members, found %v", found)
	}
	gotB, err := s.GetBranch(b.ID)
	if err != nil {
		t.Fatalf("GetBranch: %v", err)
	}
	if gotB.ParentID == nil || *gotB.ParentID != a.ID {
		t.Errorf("GetBranch parent = %v, want %q", gotB.ParentID, a.ID)
	}
}

// ─── REQ-B5: SetLeaf ─────────────────────────────────────────────────────────

func TestSetLeafRace(t *testing.T) {
	s := openTestStore(t)
	a, _ := s.CreateBranch("", "a")
	b, _ := s.CreateBranch("", "b")
	// Use wg.Go per modern guideline
	var wg sync.WaitGroup
	wg.Add(2)
	// Two concurrent writers
	go func() {
		defer wg.Done()
		_ = s.SetLeaf(a.ID)
	}()
	go func() {
		defer wg.Done()
		_ = s.SetLeaf(b.ID)
	}()
	wg.Wait()
	// Final leaf must be one of the values (last-writer-wins)
	var leaf string
	// Any row's leaf_id reflects global leaf (our implementation updates all rows)
	if err := s.db.QueryRow("SELECT leaf_id FROM sessions LIMIT 1").Scan(&leaf); err != nil {
		t.Fatalf("query leaf_id: %v", err)
	}
	if leaf != a.ID && leaf != b.ID {
		t.Errorf("final leaf = %q, want one of %q/%q", leaf, a.ID, b.ID)
	}
	// -race will detect unsynchronized access if mu missing
}

// Also test GetLeafPath SQL injection safe
func TestGetLeafPathSQLInjection(t *testing.T) {
	s := openTestStore(t)
	_, _ = s.CreateBranch("", "")
	inj := "' OR 1=1"
	path, err := s.GetLeafPath(inj)
	if err == nil {
		if len(path) != 0 {
			t.Errorf("SQLi leafID %q returned %d rows, want 0 (param ? safe)", inj, len(path))
		}
	} else {
		// Error is also acceptable as long as no rows leaked
		if strings.Contains(err.Error(), "syntax") {
			t.Errorf("SQL injection caused syntax error, not param-safe: %v", err)
		}
	}
	// Also try classic injection
	inj2 := "' OR '1'='1"
	path2, _ := s.GetLeafPath(inj2)
	if len(path2) != 0 {
		t.Errorf("SQLi %q returned %d, want 0", inj2, len(path2))
	}
}

// ─── REQ-B4: Leaf→Root traversal ──────────────────────────────────────────────

func TestGetLeafPathChain(t *testing.T) {
	s := openTestStore(t)
	r, _ := s.CreateBranch("", "root")
	b, _ := s.CreateBranch(r.ID, "branch")
	l, _ := s.CreateBranch(b.ID, "leaf")
	path, err := s.GetLeafPath(l.ID)
	if err != nil {
		t.Fatalf("GetLeafPath: %v", err)
	}
	if len(path) != 3 {
		t.Fatalf("path len = %d, want 3", len(path))
	}
	// Expect leaf→root order: [L, B, R]
	if path[0].ID != l.ID || path[1].ID != b.ID || path[2].ID != r.ID {
		t.Errorf("path order = [%s %s %s], want [%s %s %s]", path[0].ID, path[1].ID, path[2].ID, l.ID, b.ID, r.ID)
	}
}

func TestGetLeafPathCycleGuard(t *testing.T) {
	s := openTestStore(t)
	a, _ := s.CreateBranch("", "a")
	b, _ := s.CreateBranch(a.ID, "b")
	// Inject cycle: A.parent = B, B.parent = A
	_, _ = s.db.Exec("UPDATE sessions SET parent_id = ? WHERE id = ?", b.ID, a.ID)
	_, _ = s.db.Exec("UPDATE sessions SET parent_id = ? WHERE id = ?", a.ID, b.ID)
	path, err := s.GetLeafPath(a.ID)
	if err != nil {
		t.Fatalf("GetLeafPath cycle: %v", err)
	}
	if len(path) != 2 {
		t.Errorf("cycle path len = %d, want 2 (visited guard)", len(path))
	}
	// Must not hang — test timeout will catch infinite loop; also ensure no duplicate beyond cycle
	seen := map[string]bool{}
	for _, sess := range path {
		if seen[sess.ID] {
			t.Errorf("duplicate %q in cycle path", sess.ID)
		}
		seen[sess.ID] = true
	}
}

func TestGetLeafPathDepth100(t *testing.T) {
	s := openTestStore(t)
	ids := make([]string, 0, 110)
	prev := ""
	for i := 0; i < 110; i++ {
		sess, err := s.CreateBranch(prev, fmt.Sprintf("n-%d", i))
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		ids = append(ids, sess.ID)
		prev = sess.ID
	}
	leaf := ids[len(ids)-1]
	path, err := s.GetLeafPath(leaf)
	if err != nil {
		t.Fatalf("GetLeafPath depth: %v", err)
	}
	if len(path) != 100 {
		t.Errorf("depth path len = %d, want 100 (truncated)", len(path))
	}
	if path[0].ID != leaf {
		t.Errorf("first = %q, want leaf %q", path[0].ID, leaf)
	}
}

func TestSessionContextBranchedFallback(t *testing.T) {
	s := openTestStore(t)
	_, _ = s.CreateBranch("", "a")
	_, _ = s.CreateBranch("", "b")
	// Empty leafID must fallback to linear SessionContext
	linear, _ := s.SessionContext(5)
	branched, err := s.SessionContextBranched("", 5)
	if err != nil {
		t.Fatalf("SessionContextBranched empty: %v", err)
	}
	if len(branched) != len(linear) {
		t.Errorf("fallback len branched %d != linear %d", len(branched), len(linear))
	}
	// Non-empty leaf should return leaf→root
	r, _ := s.CreateBranch("", "r")
	b, _ := s.CreateBranch(r.ID, "b")
	l, _ := s.CreateBranch(b.ID, "l")
	p, err := s.SessionContextBranched(l.ID, 10)
	if err != nil {
		t.Fatalf("branched: %v", err)
	}
	if len(p) != 3 || p[0].ID != l.ID {
		t.Errorf("branched path unexpected: %v", p)
	}
}

// ─── REQ-B5: Save anchoring ──────────────────────────────────────────────────

func TestSaveAnchoring(t *testing.T) {
	s := openTestStore(t)
	leafSess, _ := s.CreateBranch("", "leaf")
	obs := &Observation{Title: "anchored", Type: "discovery", Content: "anchored content", Project: "test"}
	if err := s.Save(obs, leafSess.ID); err != nil {
		t.Fatalf("Save with parent: %v", err)
	}
	got, err := s.Get(obs.ID)
	if err != nil {
		t.Fatalf("Get after anchored Save: %v", err)
	}
	if got.SessionID != leafSess.ID {
		t.Errorf("anchored SessionID = %q, want %q", got.SessionID, leafSess.ID)
	}
	// Save without parent must preserve FTS/dedup (no-op)
	obs2 := &Observation{Title: "no-parent", Type: "discovery", Content: "no parent content", Project: "test"}
	if err := s.Save(obs2); err != nil {
		t.Fatalf("Save without parent: %v", err)
	}
	results, err := s.Search("no-parent", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	found := false
	for _, r := range results {
		if r.ID == obs2.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("FTS search did not find observation saved without parent")
	}
	// Dedup preserved: save same topic_key twice without parent should rev count
	obs3 := &Observation{Title: "dedup-test", Type: "note", Content: "v1", TopicKey: "test/dedup-anchor", Project: "test"}
	_ = s.Save(obs3)
	id1 := obs3.ID
	obs4 := &Observation{Title: "dedup-test-2", Type: "note", Content: "v2", TopicKey: "test/dedup-anchor", Project: "test"}
	_ = s.Save(obs4)
	if obs4.ID != id1 {
		t.Errorf("dedup failed: id1 %q != id4 %q", id1, obs4.ID)
	}
}

// ─── REQ-B6/B7: Compat and retention ─────────────────────────────────────────

func TestLegacyGetSearch(t *testing.T) {
	s := openTestStore(t)
	root, _ := s.CreateBranch("", "root")
	// Save legacy observation without branching context
	obs := &Observation{Title: "legacy", Type: "note", Content: "legacy content", Project: "test"}
	_ = s.Save(obs)
	// Get should work independent of branching cols
	if _, err := s.Get(obs.ID); err != nil {
		t.Fatalf("Get legacy: %v", err)
	}
	// Create branched observation
	obsB := &Observation{Title: "branched legacy", Type: "note", Content: "branched content", Project: "test"}
	_ = s.Save(obsB, root.ID)
	if _, err := s.Get(obsB.ID); err != nil {
		t.Fatalf("Get branched: %v", err)
	}
	res, err := s.Search("legacy", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) == 0 {
		t.Error("Search legacy returned 0, want >=1")
	}
}

func TestNoAutoGC(t *testing.T) {
	s := openTestStore(t)
	r, _ := s.CreateBranch("", "r")
	b, _ := s.CreateBranch(r.ID, "b")
	c, _ := s.CreateBranch(b.ID, "c")
	// No explicit delete; all three must remain
	for _, id := range []string{r.ID, b.ID, c.ID} {
		if _, err := s.GetBranch(id); err != nil {
			t.Errorf("expected branch %q retained, got error %v", id, err)
		}
	}
	// Also verify no background GC deleted after DoctorFix
	_ = s.DoctorFix()
	for _, id := range []string{r.ID, b.ID, c.ID} {
		if _, err := s.GetBranch(id); err != nil {
			t.Errorf("after DoctorFix branch %q missing, GC should not happen", id)
		}
	}
}

func TestBranchSessionStartCompatibility(t *testing.T) {
	s := openTestStore(t)
	_, err := s.SessionStart("sess-compat-1", "proj")
	if err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	var leaf sql.NullString
	_ = s.db.QueryRow("SELECT leaf_id FROM sessions WHERE id = ?", "sess-compat-1").Scan(&leaf)
	if !leaf.Valid || leaf.String != "sess-compat-1" {
		t.Errorf("SessionStart leaf_id = %q, want self", leaf.String)
	}
	sessions, err := s.SessionContext(5)
	if err != nil {
		t.Fatalf("SessionContext: %v", err)
	}
	if len(sessions) == 0 {
		t.Error("SessionContext empty after SessionStart")
	}
}
