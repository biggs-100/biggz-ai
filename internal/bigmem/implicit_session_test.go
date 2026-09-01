package bigmem

import (
	"database/sql"
	"testing"
	"time"
)

func TestMostRecentActiveSession_ReturnsActive(t *testing.T) {
	s := openTestStore(t)
	if err := s.EnsureImplicitSession("uuid-active-1", "proj-a"); err != nil {
		t.Fatalf("EnsureImplicitSession: %v", err)
	}
	// Ensure Ensure created session; now set start_time deterministic
	// Query MostRecentActiveSession should find it
	id, ok, err := s.MostRecentActiveSession("proj-a")
	if err != nil {
		t.Fatalf("MostRecentActiveSession: %v", err)
	}
	if !ok || id != "uuid-active-1" {
		t.Fatalf("expected uuid-active-1 ok=true got id=%q ok=%v", id, ok)
	}
}

func TestMostRecentActiveSession_SkipsEnded(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.SessionStart("uuid-ended", "proj-b"); err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	if _, err := s.SessionEnd("uuid-ended", "done"); err != nil {
		t.Fatalf("SessionEnd: %v", err)
	}
	_, ok, err := s.MostRecentActiveSession("proj-b")
	if err != nil {
		t.Fatalf("MostRecentActiveSession: %v", err)
	}
	if ok {
		t.Fatalf("expected no active session after ended")
	}
}

func TestMostRecentActiveSession_EmptyProject(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.SessionStart("sess-x", "proj"); err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	_, ok, err := s.MostRecentActiveSession("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for empty project")
	}
}

func TestMostRecentActiveSession_PicksMostRecent(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.SessionStart("uuid-old", "proj-c"); err != nil {
		t.Fatalf("old: %v", err)
	}
	// backdate old
	if _, err := s.db.Exec(`UPDATE sessions SET start_time = ? WHERE id = ?`, "2025-01-01T00:00:00Z", "uuid-old"); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := s.SessionStart("uuid-new", "proj-c"); err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE sessions SET start_time = ? WHERE id = ?`, "2025-06-01T00:00:00Z", "uuid-new"); err != nil {
		t.Fatalf("set new: %v", err)
	}
	id, ok, err := s.MostRecentActiveSession("proj-c")
	if err != nil {
		t.Fatalf("MostRecentActiveSession: %v", err)
	}
	if !ok || id != "uuid-new" {
		t.Fatalf("expected uuid-new got %q ok=%v", id, ok)
	}
}

func TestMostRecentActiveSession_ExcludesManualSave(t *testing.T) {
	s := openTestStore(t)
	if err := s.EnsureImplicitSession("manual-save-proj-d", "proj-d"); err != nil {
		t.Fatalf("ensure manual: %v", err)
	}
	_, ok, err := s.MostRecentActiveSession("proj-d")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("manual-save should be excluded, expected ok=false")
	}
	// But with a real active session also present, it should be returned not manual
	if _, err := s.SessionStart("real-1", "proj-d"); err != nil {
		t.Fatalf("real: %v", err)
	}
	id, ok, err := s.MostRecentActiveSession("proj-d")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok || id != "real-1" {
		t.Fatalf("expected real-1 got %q ok=%v", id, ok)
	}
}

func TestEnsureImplicitSession_Idempotent(t *testing.T) {
	s := openTestStore(t)
	if err := s.EnsureImplicitSession("imp-1", "proj-e"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := s.EnsureImplicitSession("imp-1", "proj-e"); err != nil {
		t.Fatalf("second idempotent: %v", err)
	}
	var cnt int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, "imp-1").Scan(&cnt); err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected 1 row, got %d", cnt)
	}
	var dir sql.NullString
	if err := s.db.QueryRow(`SELECT directory FROM sessions WHERE id = ?`, "imp-1").Scan(&dir); err != nil {
		t.Fatalf("dir: %v", err)
	}
	// directory should be set (may be empty in test temp dir but column exists)
	_ = dir
}

func TestSaveUsesImplicitSessionViaDirectStore(t *testing.T) {
	s := openTestStore(t)
	// Create active session first
	if _, err := s.SessionStart("active-uuid", "proj-f"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Simulate MCP fallback: empty session_id should resolve to active
	sid, ok, _ := s.MostRecentActiveSession("proj-f")
	if !ok || sid != "active-uuid" {
		t.Fatalf("fallback: %q ok=%v", sid, ok)
	}
	_ = s.EnsureImplicitSession(sid, "proj-f")
	obs := &Observation{Title: "Implicit save test", Type: "decision", Content: "**What**: test", Project: "proj-f", SessionID: sid}
	if err := s.Save(obs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if obs.SessionID != "active-uuid" {
		t.Fatalf("session_id %q want active-uuid", obs.SessionID)
	}
	var storedSID string
	if err := s.db.QueryRow(`SELECT session_id FROM observations WHERE id = ?`, obs.ID).Scan(&storedSID); err != nil {
		t.Fatalf("query: %v", err)
	}
	if storedSID != "active-uuid" {
		t.Fatalf("stored session_id %q want active-uuid", storedSID)
	}
}

func TestSaveWithEmptySessionCreatesManualSave(t *testing.T) {
	s := openTestStore(t)
	proj := "proj-g"
	// No active session; fallback should be manual-save
	sid := ""
	if id, ok, _ := s.MostRecentActiveSession(proj); ok {
		sid = id
	} else {
		sid = defaultSessionID(proj)
	}
	if sid != "manual-save-proj-g" {
		t.Fatalf("expected manual-save-proj-g got %q", sid)
	}
	_ = s.EnsureImplicitSession(sid, proj)
	obs := &Observation{Title: "Manual fallback", Type: "discovery", Content: "hello", Project: proj, SessionID: sid}
	if err := s.Save(obs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	var cnt int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, sid).Scan(&cnt); err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("manual session not created")
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatalf("total sessions: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 session total got %d", n)
	}
}
