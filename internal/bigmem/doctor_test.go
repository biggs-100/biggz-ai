package bigmem

import (
	"database/sql"
	"path/filepath"
	"testing"
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
