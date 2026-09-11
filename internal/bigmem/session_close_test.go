package bigmem

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestSessionEnd_DualWriteExactlyOnce proves REQ-SC1 (searchable close).
// The defect: SessionEnd wrote only the sessions row, so the close was
// invisible to empty-query recency (Search(""), ORDER BY updated_at DESC).
func TestSessionEnd_DualWriteExactlyOnce(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.SessionStart("sess-close-1", "biggz-ai"); err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	if _, err := s.SessionEnd("sess-close-1", "summary v1"); err != nil {
		t.Fatalf("SessionEnd first: %v", err)
	}

	// Runtime evidence: row count + what recency actually returns.
	count := countSessionSummaries(t, s, "sess-close-1")
	t.Logf("after first close: session_summary rows for sess-close-1 = %d", count)
	if count != 1 {
		t.Fatalf("after first close want exactly 1 session_summary row, got %d", count)
	}

	results, err := s.Search("", SearchOptions{Type: "session_summary", Limit: 5, Project: "biggz-ai"})
	if err != nil {
		t.Fatalf("Search empty: %v", err)
	}
	for _, r := range results {
		t.Logf("recall: id=%s type=%s session_id=%s updated_at=%s content=%q",
			r.ID, r.Type, r.SessionID, r.UpdatedAt.Format(time.RFC3339), r.Content)
	}
	if len(results) != 1 {
		t.Fatalf("recency must find exactly 1 session_summary, got %d", len(results))
	}
	if results[0].Content != "summary v1" || results[0].SessionID != "sess-close-1" {
		t.Fatalf("recall mismatch: content=%q session=%q", results[0].Content, results[0].SessionID)
	}

	// Repeat close: exactly-once — update in place, never duplicate.
	if _, err := s.SessionEnd("sess-close-1", "summary v2"); err != nil {
		t.Fatalf("SessionEnd second: %v", err)
	}
	count = countSessionSummaries(t, s, "sess-close-1")
	t.Logf("after repeat close: session_summary rows for sess-close-1 = %d (content=summary v2)", count)
	if count != 1 {
		t.Fatalf("repeat close must keep exactly 1 row, got %d", count)
	}
	results, err = s.Search("", SearchOptions{Type: "session_summary", Limit: 5, Project: "biggz-ai"})
	if err != nil {
		t.Fatalf("Search empty after repeat: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("repeat close must not duplicate, got %d", len(results))
	}
	if results[0].Content != "summary v2" {
		t.Fatalf("repeat close must update in place, content=%q", results[0].Content)
	}
}

// TestSessionEnd_SameContentTwoSessionsStaySeparate guards the hash-dedup
// phase: two sessions closing with identical text must still get one
// observation each — a window merge would leave the second session
// unsearchable (REQ-SC1 exactly-once per session id).
func TestSessionEnd_SameContentTwoSessionsStaySeparate(t *testing.T) {
	s := openTestStore(t)
	const identical = "identical close text"
	for _, id := range []string{"sess-a", "sess-b"} {
		if _, err := s.SessionStart(id, "biggz-ai"); err != nil {
			t.Fatalf("SessionStart %s: %v", id, err)
		}
		if _, err := s.SessionEnd(id, identical); err != nil {
			t.Fatalf("SessionEnd %s: %v", id, err)
		}
	}
	for _, id := range []string{"sess-a", "sess-b"} {
		if n := countSessionSummaries(t, s, id); n != 1 {
			t.Fatalf("session %s must have exactly 1 session_summary, got %d", id, n)
		}
	}
	results, err := s.Search("", SearchOptions{Type: "session_summary", Limit: 5, Project: "biggz-ai"})
	if err != nil {
		t.Fatalf("Search empty: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 rows (one per session), got %d", len(results))
	}
}

// TestSessionEnd_ObservationWriteRetriesOnceThenFailsVisibly proves the
// REQ-SC1 locked-store scenario: retry once after the delay, and on persistent
// failure surface an explicit error while the sessions row stays intact.
// The package-level upsert seam forces the failure without a real file lock.
func TestSessionEnd_ObservationWriteRetriesOnceThenFailsVisibly(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.SessionStart("sess-locked", "biggz-ai"); err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	origUpsert := sessionSummaryUpsert
	origDelay := sessionSummaryRetryDelay
	t.Cleanup(func() {
		sessionSummaryUpsert = origUpsert
		sessionSummaryRetryDelay = origDelay
	})
	sessionSummaryRetryDelay = time.Millisecond

	// (a) fail once, then succeed — retried exactly once, success reported.
	attempts := 0
	sessionSummaryUpsert = func(st *Store, sessionID, project, summary string) error {
		attempts++
		if attempts == 1 {
			return errors.New("database is locked")
		}
		return origUpsert(st, sessionID, project, summary)
	}
	if _, err := s.SessionEnd("sess-locked", "retry summary"); err != nil {
		t.Fatalf("retry-once must succeed on second attempt: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("want exactly 2 attempts, got %d", attempts)
	}
	if n := countSessionSummaries(t, s, "sess-locked"); n != 1 {
		t.Fatalf("retried close must persist 1 row, got %d", n)
	}

	// (b) persistent failure — explicit error, sessions row kept and updated.
	sessionSummaryUpsert = func(st *Store, sessionID, project, summary string) error {
		return errors.New("database is locked")
	}
	if _, err := s.SessionEnd("sess-locked", "still locked"); err == nil {
		t.Fatal("persistent observation failure must surface an explicit error")
	} else if !strings.Contains(err.Error(), "after retry") {
		t.Fatalf("error must name the retry exhaustion, got %v", err)
	}
	var endTime, summary string
	if qErr := s.db.QueryRow(
		"SELECT COALESCE(end_time, ''), COALESCE(summary, '') FROM sessions WHERE id = ?",
		"sess-locked").Scan(&endTime, &summary); qErr != nil {
		t.Fatalf("sessions row must survive the observation failure: %v", qErr)
	}
	if endTime == "" || summary != "still locked" {
		t.Fatalf("sessions row not updated: end_time=%q summary=%q", endTime, summary)
	}
}

// TestSaveCtx_SessionSummaryRoutesToDeterministicID proves the SaveCtx route:
// session_summary + session_id uses the stable id and updates in place.
func TestSaveCtx_SessionSummaryRoutesToDeterministicID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	var lastID string
	for i := 0; i < 2; i++ {
		obs := &Observation{Title: "Session summary", Type: "session_summary", Content: "routed", SessionID: "sess-routed", Project: "biggz-ai"}
		if err := s.SaveCtx(ctx, obs); err != nil {
			t.Fatalf("SaveCtx %d: %v", i, err)
		}
		lastID = obs.ID
	}
	if lastID != SessionSummaryObsID("sess-routed") {
		t.Fatalf("id = %q want %q", lastID, SessionSummaryObsID("sess-routed"))
	}
	if n := countSessionSummaries(t, s, "sess-routed"); n != 1 {
		t.Fatalf("repeated SaveCtx must keep 1 row, got %d", n)
	}
}

// countSessionSummaries counts persisted session_summary observations for a session id.
func countSessionSummaries(t *testing.T, s *Store, sessionID string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM observations WHERE type = 'session_summary' AND session_id = ? AND deleted_at IS NULL`,
		sessionID).Scan(&n); err != nil {
		t.Fatalf("count session_summary %s: %v", sessionID, err)
	}
	return n
}
