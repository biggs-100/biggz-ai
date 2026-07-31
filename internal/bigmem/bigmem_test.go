package bigmem

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestOpen_MigratesLegacySchema proves that a database created by an older
// release (without provenance/lifecycle columns) is migrated on Open so that
// index creation and inserts do not fail with "no such column".
func TestOpen_MigratesLegacySchema(t *testing.T) {
	dir := t.TempDir()

	// Simulate a legacy database: observations without the newer columns.
	db, err := sql.Open("sqlite", filepath.Join(dir, "bigmem.db"))
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE observations (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			type TEXT DEFAULT '',
			content TEXT DEFAULT '',
			topic_key TEXT DEFAULT '',
			project TEXT DEFAULT '',
			scope TEXT DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			deleted_at TEXT
		);
		CREATE TABLE memory_relations (
			id TEXT PRIMARY KEY,
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			relation TEXT NOT NULL DEFAULT 'pending',
			reason TEXT DEFAULT '',
			evidence TEXT DEFAULT '',
			confidence REAL DEFAULT 0.0,
			judgment_status TEXT NOT NULL DEFAULT 'pending',
			marked_by TEXT DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		INSERT INTO observations (id, title, type, content, project, created_at, updated_at)
		VALUES ('legacy-1', 'Old note', 'discovery', 'legacy content', 'legacy', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	`)
	if err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	db.Close()

	// Open must migrate the legacy schema instead of failing.
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() on legacy db: %v", err)
	}
	defer s.Close()

	// The legacy row is still readable with migrated defaults.
	got, err := s.Get("legacy-1")
	if err != nil {
		t.Fatalf("Get(legacy row) error: %v", err)
	}
	if got.Title != "Old note" {
		t.Errorf("legacy Title = %q, want %q", got.Title, "Old note")
	}
	if got.SessionID != "" {
		t.Errorf("legacy SessionID = %q, want empty default", got.SessionID)
	}

	// New writes with the new columns work after migration.
	obs := &Observation{Title: "New note", Type: "decision", Content: "fresh", Project: "test", SessionID: "sess-1", ToolName: "cli"}
	if err := s.Save(obs); err != nil {
		t.Fatalf("Save() with new columns: %v", err)
	}
	saved, err := s.Get(obs.ID)
	if err != nil {
		t.Fatalf("Get(new row) error: %v", err)
	}
	if saved.SessionID != "sess-1" || saved.ToolName != "cli" {
		t.Errorf("SessionID/ToolName = %q/%q, want sess-1/cli", saved.SessionID, saved.ToolName)
	}
}

func TestSaveAndGet(t *testing.T) {
	s := openTestStore(t)
	obs := &Observation{Title: "Test decision", Type: "decision", Content: "**What**: Testing", Project: "test"}

	if err := s.Save(obs); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if obs.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	got, err := s.Get(obs.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Title != "Test decision" {
		t.Errorf("Title = %q, want %q", got.Title, "Test decision")
	}
}

func TestSave_UpdatesByTopicKey(t *testing.T) {
	s := openTestStore(t)
	obs1 := &Observation{Title: "Original", Type: "architecture", Content: "v1", TopicKey: "test/topic", Project: "test"}
	s.Save(obs1)

	obs2 := &Observation{Title: "Updated", Type: "architecture", Content: "v2", TopicKey: "test/topic", Project: "test"}
	s.Save(obs2)

	if obs1.ID != obs2.ID {
		t.Errorf("expected same ID: %s vs %s", obs1.ID, obs2.ID)
	}
	got, _ := s.Get(obs2.ID)
	if got.Title != "Updated" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated")
	}
}

func TestSearch(t *testing.T) {
	s := openTestStore(t)
	s.Save(&Observation{Title: "Auth design", Type: "architecture", Content: "JWT", Project: "biggz", TopicKey: "auth"})
	s.Save(&Observation{Title: "Bug fix", Type: "bugfix", Content: "Fixed NPE", Project: "biggz", TopicKey: "bug/npe"})
	s.Save(&Observation{Title: "Config", Type: "config", Content: "Timeout 30s", Project: "other", TopicKey: "config"})

	// Give SQLite a moment to index
	time.Sleep(10 * time.Millisecond)

	results, err := s.Search("auth", SearchOptions{})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) < 1 {
		t.Errorf("expected at least 1 result for 'auth', got %d", len(results))
	}

	results, err = s.Search("", SearchOptions{Type: "bugfix"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 bugfix, got %d", len(results))
	}

	results, err = s.Search("", SearchOptions{Project: "other"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 for 'other', got %d", len(results))
	}
}

func TestDelete(t *testing.T) {
	s := openTestStore(t)
	obs := &Observation{Title: "To delete", Type: "discovery", Content: "Will be removed"}
	s.Save(obs)

	if err := s.Delete(obs.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	_, err := s.Get(obs.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestUpdate(t *testing.T) {
	s := openTestStore(t)
	obs := &Observation{Title: "Original", Type: "decision", Content: "First"}
	s.Save(obs)

	updated, err := s.Update(obs.ID, map[string]any{"title": "Updated", "content": "New version"})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated.Title != "Updated" {
		t.Errorf("Title = %q", updated.Title)
	}
}

func TestStats(t *testing.T) {
	s := openTestStore(t)
	s.Save(&Observation{Title: "D1", Type: "decision"})
	time.Sleep(time.Millisecond)
	s.Save(&Observation{Title: "D2", Type: "decision"})
	time.Sleep(time.Millisecond)
	s.Save(&Observation{Title: "B1", Type: "bugfix"})

	stats, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats() error: %v", err)
	}
	if stats.TotalObservations != 3 {
		t.Errorf("expected 3, got %d", stats.TotalObservations)
	}
	if stats.ByType["decision"] != 2 {
		t.Errorf("expected 2 decisions, got %d", stats.ByType["decision"])
	}
}

func TestSession(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.SessionStart("sess-1", "test"); err != nil {
		t.Fatalf("SessionStart 1 error: %v", err)
	}
	if _, err := s.SessionStart("sess-2", "test"); err != nil {
		t.Fatalf("SessionStart 2 error: %v", err)
	}

	sessions, err := s.SessionContext(5)
	if err != nil {
		t.Fatalf("SessionContext() error: %v", err)
	}
	t.Logf("Got %d sessions", len(sessions))
	for _, se := range sessions {
		t.Logf("  Session: %s", se.ID)
	}
	if len(sessions) != 2 {
		// Debug: run same query directly
		rows, err := s.db.Query("SELECT id, start_time FROM sessions ORDER BY start_time DESC LIMIT 5")
		if err != nil {
			t.Logf("Direct query error: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var id, st string
				rows.Scan(&id, &st)
				t.Logf("Direct row: %s / %s", id, st)
			}
		}
		var count int
		s.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
		t.Logf("Direct SQL count: %d", count)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestCompare(t *testing.T) {
	s := openTestStore(t)
	a := &Observation{Title: "A", Type: "decision", TopicKey: "topic1", Project: "proj"}
	b := &Observation{Title: "B", Type: "decision", TopicKey: "topic1", Project: "proj"}
	s.Save(a)
	s.Save(b)

	r, err := s.Compare(a.ID, b.ID)
	if err != nil {
		t.Fatalf("Compare() error: %v", err)
	}
	if !r.SameTopic {
		t.Error("expected same topic")
	}
	if !r.SameProject {
		t.Error("expected same project")
	}
}

func TestMergeProjects(t *testing.T) {
	s := openTestStore(t)
	s.Save(&Observation{Title: "T1", Project: "src"})
	s.Save(&Observation{Title: "T2", Project: "src"})

	n, err := s.MergeProjects("src", "dst")
	if err != nil {
		t.Fatalf("MergeProjects() error: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 merged, got %d", n)
	}
}

func TestTimeline(t *testing.T) {
	s := openTestStore(t)
	s.Save(&Observation{Title: "First", Type: "decision"})
	time.Sleep(5 * time.Millisecond)
	s.Save(&Observation{Title: "Second", Type: "bugfix"})

	entries, err := s.Timeline(TimelineOptions{})
	if err != nil {
		t.Fatalf("Timeline() error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestSuggestTopicKey(t *testing.T) {
	key := SuggestTopicKey("Fixed auth bug in middleware", "", "bugfix")
	if !strings.HasPrefix(key, "bugfix/") {
		t.Errorf("expected bugfix/ prefix, got %s", key)
	}
}

func TestCapturePassive(t *testing.T) {
	content := "## Key Learnings\n- Found a race condition\n- Fixed it with mutex"
	obs, err := CapturePassive(content, "test")
	if err != nil {
		t.Fatalf("CapturePassive() error: %v", err)
	}
	if len(obs) != 2 {
		t.Errorf("expected 2 learnings, got %d", len(obs))
	}
}

func TestOpen_DefaultDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	s, err := Open("")
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer s.Close()

	if !strings.Contains(s.RootDir(), ".biggz") {
		t.Errorf("expected .biggz in path, got %s", s.RootDir())
	}
}

func TestEmptySearch(t *testing.T) {
	s := openTestStore(t)
	results, err := s.Search("nonexistent", SearchOptions{})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSavePrompt(t *testing.T) {
	s := openTestStore(t)
	p, err := s.SavePrompt("Test prompt content", "sess-1")
	if err != nil {
		t.Fatalf("SavePrompt() error: %v", err)
	}
	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
}
