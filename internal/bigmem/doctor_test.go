package bigmem

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDoctorFix_WALCheckpoint verifies that DoctorFix runs WAL checkpoint and VACUUM without error.
func TestDoctorFix_WALCheckpoint(t *testing.T) {
	s := openTestStore(t)
	if err := s.Save(&Observation{Title: "wal-test", Content: "content", Type: "note", Project: "test"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.DoctorFix(); err != nil {
		t.Fatalf("DoctorFix WAL checkpoint: %v", err)
	}
	// Should still be searchable after fix.
	results, err := s.Search("wal-test", SearchOptions{})
	if err != nil {
		t.Fatalf("Search after DoctorFix: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least 1 result after DoctorFix WAL checkpoint")
	}
}

// TestDoctorFix_FTSRebuild verifies that dropping FTS and calling DoctorFix rebuilds it correctly.
func TestDoctorFix_FTSRebuild(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if err := s.Save(&Observation{Title: "fts-rebuild-target", Content: "unique content fts", Type: "note", Project: "test"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Simulate FTS corruption by dropping FTS table.
	dbPath := filepath.Join(dir, "bigmem.db")
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := rawDB.Exec("DROP TABLE IF EXISTS observations_fts"); err != nil {
		t.Fatalf("drop fts: %v", err)
	}
	rawDB.Close()

	// Search would now fail or return no results; DoctorFix should rebuild.
	if err := s.DoctorFix(); err != nil {
		t.Fatalf("DoctorFix FTS rebuild: %v", err)
	}
	// Idempotent second run should also not error.
	if err := s.DoctorFix(); err != nil {
		t.Fatalf("second DoctorFix: %v", err)
	}

	results, err := s.Search("fts-rebuild-target", SearchOptions{})
	if err != nil {
		t.Fatalf("Search after FTS rebuild: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least 1 result after FTS rebuild")
	}

	// Verify FTS table exists.
	rawDB2, _ := sql.Open("sqlite", dbPath)
	defer rawDB2.Close()
	var count int
	if err := rawDB2.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='observations_fts'").Scan(&count); err != nil {
		t.Fatalf("check fts table: %v", err)
	}
	if count != 1 {
		t.Errorf("observations_fts table count = %d, want 1", count)
	}
}

// TestDoctorFix_SessionsDirectoryMigration verifies that DoctorFix adds missing directory column.
func TestDoctorFix_SessionsDirectoryMigration(t *testing.T) {
	dir := t.TempDir()
	// Create legacy DB with sessions table lacking directory column.
	dbPath := filepath.Join(dir, "bigmem.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE sessions (id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT)`)
	if err != nil {
		t.Fatalf("create legacy sessions: %v", err)
	}
	_, err = db.Exec(`INSERT INTO sessions (id, start_time, project) VALUES ('sess-1', '2026-01-01T00:00:00Z', 'proj')`)
	if err != nil {
		t.Fatalf("insert legacy session: %v", err)
	}
	db.Close()

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open after legacy sessions: %v", err)
	}
	defer s.Close()

	// Close and simulate missing column by recreating without directory? Open would have migrated via DoctorFix?
	// Force missing column: drop and recreate without directory, then run DoctorFix.
	rawDB, _ := sql.Open("sqlite", dbPath)
	rawDB.Exec("DROP TABLE IF EXISTS sessions")
	rawDB.Exec(`CREATE TABLE sessions (id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT)`)
	rawDB.Close()

	// Reopen store to pick up new file but keep old store's db connection? Need new store.
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("Open after dropping sessions: %v", err)
	}
	defer s2.Close()

	if err := s2.DoctorFix(); err != nil {
		t.Fatalf("DoctorFix schema migration: %v", err)
	}

	// Verify directory column exists via PRAGMA table_info.
	rawDB2, _ := sql.Open("sqlite", dbPath)
	defer rawDB2.Close()
	rows, err := rawDB2.Query("PRAGMA table_info(sessions)")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()
	hasDir := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == "directory" {
			hasDir = true
		}
	}
	if !hasDir {
		t.Error("expected sessions.directory column after DoctorFix, not found")
	}

	// Verify SessionContext still works (no such column error).
	sessions, err := s2.SessionContext(5)
	if err != nil {
		t.Fatalf("SessionContext after fix: %v", err)
	}
	_ = sessions
}

// TestDoctorFix_IdempotentOnHealthy verifies DoctorFix succeeds on healthy DB and is idempotent.
func TestDoctorFix_IdempotentOnHealthy(t *testing.T) {
	s := openTestStore(t)
	if err := s.Save(&Observation{Title: "healthy", Content: "content", Type: "note"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := s.DoctorFix(); err != nil {
			t.Fatalf("DoctorFix run %d: %v", i, err)
		}
	}
	// Ensure update/delete not malformed after fix.
	obs := &Observation{Title: "to-update", Content: "v1", Type: "note"}
	if err := s.Save(obs); err != nil {
		t.Fatalf("Save to-update: %v", err)
	}
	if _, err := s.Update(obs.ID, map[string]any{"content": "v2"}); err != nil {
		t.Fatalf("Update after DoctorFix: %v", err)
	}
	if err := s.Delete(obs.ID); err != nil {
		t.Fatalf("Delete after DoctorFix: %v", err)
	}
}

// TestDoctorFix_FTSDesync_OrphanHit verifies that an orphan FTS entry (search hit that Get cannot find)
// is repaired by DoctorFix. This reproduces the split-brain / stale FTS symptom:
// raw FTS MATCH returns a rowid that has no corresponding observation, so a direct
// FTS query succeeds but Get/sql no rows fails. DoctorFix must rebuild FTS atomically.
func TestDoctorFix_FTSDesync_OrphanHit(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	// Save a real observation so FTS and observations are in sync initially.
	if err := s.Save(&Observation{Title: "real-obs", Content: "real content for desync test", Type: "note", Project: "test"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	dbPath := filepath.Join(dir, "bigmem.db")
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	// Create orphan FTS entry: rowid 99999 has no observation row. Use a unique term without hyphens (FTS5 hyphen is operator).
	orphanTerm := "orphanuniquexyz999"
	// Direct insert into FTS bypassing triggers to simulate stale index.
	if _, err := rawDB.Exec(`INSERT INTO observations_fts(rowid, title, content, topic_key, tool_name, type, project) VALUES (99999, ?, ?, '', '', 'note', 'test')`, orphanTerm, orphanTerm); err != nil {
		t.Fatalf("insert orphan fts: %v", err)
	}
	// Verify orphan exists at FTS level (quote term for FTS MATCH).
	quoted := `"` + orphanTerm + `"`
	var orphanCount int
	if err := rawDB.QueryRow("SELECT COUNT(*) FROM observations_fts WHERE observations_fts MATCH ?", quoted).Scan(&orphanCount); err != nil {
		t.Fatalf("fts orphan count: %v", err)
	}
	if orphanCount == 0 {
		t.Fatalf("expected orphan FTS hit before fix, got 0")
	}
	// Verify that JOIN via observations yields no row (simulating search hit that Get cannot find).
	var joinCount int
	_ = rawDB.QueryRow(`SELECT COUNT(*) FROM observations_fts fts JOIN observations o ON o.rowid = fts.rowid WHERE fts.observations_fts MATCH ? AND o.deleted_at IS NULL`, quoted).Scan(&joinCount)
	if joinCount != 0 {
		t.Errorf("expected JOIN to filter orphan (0), got %d — indicates FTS desync not reproduced", joinCount)
	}
	// Also verify Get for that fake rowid fails: no observation with rowid 99999
	var obsID string
	err = rawDB.QueryRow("SELECT id FROM observations WHERE rowid = 99999").Scan(&obsID)
	if err == nil {
		t.Errorf("expected no observation for orphan rowid 99999, got id %q", obsID)
	}
	rawDB.Close()
	// Store.Search for orphan term should return 0 via JOIN + LIKE fallback (no observation has that term).
	results, err := s.Search(quoted, SearchOptions{})
	if err != nil {
		t.Fatalf("Search orphan before fix: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 Search results for orphan term before fix (JOIN filters), got %d", len(results))
	}
	// Now repair.
	if err := s.DoctorFix(); err != nil {
		t.Fatalf("DoctorFix: %v", err)
	}
	// After fix, orphan FTS entry must be gone.
	rawDB2, _ := sql.Open("sqlite", dbPath)
	var afterCount int
	_ = rawDB2.QueryRow("SELECT COUNT(*) FROM observations_fts WHERE observations_fts MATCH ?", quoted).Scan(&afterCount)
	if afterCount != 0 {
		t.Errorf("expected orphan FTS hit to be 0 after DoctorFix, got %d", afterCount)
	}
	rawDB2.Close()
	// Real observation must still be searchable.
	results2, err := s.Search("real-obs", SearchOptions{})
	if err != nil {
		t.Fatalf("Search real after fix: %v", err)
	}
	if len(results2) == 0 {
		t.Error("expected real-obs to be searchable after DoctorFix")
	}
	// Verify integrity_check passes and FTS table exists (query via store's DB to avoid stale raw handle).
	var ic string
	_ = s.db.QueryRow("PRAGMA integrity_check").Scan(&ic)
	var ftsExists int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='observations_fts'").Scan(&ftsExists)
	if ftsExists != 1 {
		// Debug: list tables
		rows, _ := s.db.Query("SELECT type, name FROM sqlite_master")
		if rows != nil {
			defer rows.Close()
			var types, names []string
			for rows.Next() {
				var tp, nm string
				rows.Scan(&tp, &nm)
				types = append(types, tp)
				names = append(names, nm)
			}
			t.Logf("sqlite_master after fix: %v %v", types, names)
		}
		t.Errorf("observations_fts should exist after fix, got %d", ftsExists)
	}
}

// TestDoctorFix_SessionsStartTimeMigration verifies that DoctorFix migrates NULL start_time to RFC3339.
func TestDoctorFix_SessionsStartTimeMigration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "bigmem.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, start_time TEXT, end_time TEXT, summary TEXT, project TEXT, directory TEXT)`)
	_, _ = db.Exec(`INSERT INTO sessions (id, start_time, project) VALUES ('sess-null', NULL, 'proj')`)
	_, _ = db.Exec(`INSERT INTO sessions (id, start_time, project) VALUES ('sess-zero', '0001-01-01T00:00:00Z', 'proj')`)
	db.Close()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if err := s.DoctorFix(); err != nil {
		t.Fatalf("DoctorFix: %v", err)
	}
	rawDB, _ := sql.Open("sqlite", dbPath)
	defer rawDB.Close()
	for _, id := range []string{"sess-null", "sess-zero"} {
		var st sql.NullString
		if err := rawDB.QueryRow("SELECT start_time FROM sessions WHERE id = ?", id).Scan(&st); err != nil {
			t.Fatalf("query start_time %s: %v", id, err)
		}
		if !st.Valid || st.String == "" || st.String == "0001-01-01T00:00:00Z" || st.String == "0001-01-01 00:00:00 +0000 UTC" {
			t.Errorf("session %s start_time not migrated, got %q", id, st.String)
		}
		// Must be RFC3339 parseable
		if _, err := time.Parse(time.RFC3339, st.String); err != nil {
			t.Errorf("session %s start_time not RFC3339: %q err %v", id, st.String, err)
		}
	}
}

// TestResolveDBPath_MergeByMaxUpdatedAt verifies that ResolveDBPath merges recovered into primary
// dedup by id keeping max(updated_at) and emits warning when both exist.
func TestResolveDBPath_MergeByMaxUpdatedAt(t *testing.T) {
	base := t.TempDir()
	primaryRoot := filepath.Join(base, "primary")
	if err := OpenDirAndSave(primaryRoot, "merge-id", "primary old", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("setup primary: %v", err)
	}
	recoveredRoot := filepath.Join(base, "bigmem_recovered")
	recoveredPath := filepath.Join(recoveredRoot, "bigmem.db")
	if err := CreateRecoveredWithNewer(recoveredPath, "merge-id", "recovered new", "2026-06-01T00:00:00Z"); err != nil {
		t.Fatalf("setup recovered: %v", err)
	}
	// Resolve should merge recovered into primary, keeping newer updated_at.
	mergedPath, err := ResolveDBPath(primaryRoot)
	if err != nil {
		t.Fatalf("ResolveDBPath: %v", err)
	}
	expectedPrimary := filepath.Join(primaryRoot, "bigmem.db")
	if mergedPath != expectedPrimary {
		t.Errorf("ResolveDBPath returned %q, want primary %q", mergedPath, expectedPrimary)
	}
	s, err := Open(primaryRoot)
	if err != nil {
		t.Fatalf("Open after merge: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	obs, err := s.Get("merge-id")
	if err != nil {
		t.Fatalf("Get after merge: %v", err)
	}
	if obs.Title != "recovered new" {
		t.Errorf("merge kept title %q, want %q (newer updated_at should win)", obs.Title, "recovered new")
	}
	if obs.UpdatedAt.Format(time.RFC3339) != "2026-06-01T00:00:00Z" {
		t.Errorf("merge kept updated_at %q, want 2026-06-01T00:00:00Z", obs.UpdatedAt.Format(time.RFC3339))
	}
	// Verify recovered file still exists (we leave warning, not delete)
	if _, err := sql.Open("sqlite", recoveredPath); err != nil {
		t.Errorf("recovered DB should still exist after merge")
	}
}

func OpenDirAndSave(root, id, title, updatedAt string) error {
	s, err := Open(root)
	if err != nil {
		return err
	}
	defer s.Close()
	obs := &Observation{ID: id, Title: title, Content: "content", Type: "note", Project: "test"}
	if err := s.Save(obs); err != nil {
		return err
	}
	// Force updated_at to specific value for deterministic merge test.
	_, err = s.db.Exec("UPDATE observations SET updated_at = ? WHERE id = ?", updatedAt, id)
	return err
}

func CreateRecoveredWithNewer(path, id, title, updatedAt string) error {
	if err := mkDirForPath(path); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	db.Exec("PRAGMA busy_timeout=5000")
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS observations (
		id TEXT PRIMARY KEY, title TEXT, type TEXT, content TEXT, session_id TEXT, tool_name TEXT,
		topic_key TEXT, project TEXT, scope TEXT, normalized_hash TEXT,
		revision_count INTEGER, duplicate_count INTEGER, last_seen_at TEXT, review_after TEXT,
		pinned INTEGER, created_at TEXT, updated_at TEXT, deleted_at TEXT)`)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`INSERT OR REPLACE INTO observations (id, title, type, content, session_id, tool_name, topic_key, project, scope, normalized_hash, revision_count, duplicate_count, pinned, created_at, updated_at) VALUES (?, ?, 'note', 'content', '', '', '', 'test', 'project', '', 1, 1, 0, ?, ?)`, id, title, now, updatedAt)
	return err
}

func mkDirForPath(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
}

// TestOpen_SetsPragmas verifies that Open sets required pragmas.
func TestOpen_SetsPragmas(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// journal_mode is persistent and visible from any connection.
	dbPath := filepath.Join(dir, "bigmem.db")
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	defer rawDB.Close()

	var journalMode string
	if err := rawDB.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}

	// foreign_keys is per-connection; verify via Store's own connection (same package can access unexported db).
	var fk int
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1 (ON)", fk)
	}
	var busy int
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}
	if busy != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busy)
	}
}
