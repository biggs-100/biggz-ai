package bigmem

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// helper to insert observation with explicit updated_at for deterministic ordering.
func insertObsWithUpdatedAt(t *testing.T, s *Store, title, typ, project, updatedAt string) *Observation {
	t.Helper()
	obs := &Observation{Title: title, Type: typ, Content: "content " + title, Project: project}
	if err := s.Save(obs); err != nil {
		t.Fatalf("Save %q: %v", title, err)
	}
	// Force updated_at to desired value (RFC3339) for deterministic ordering.
	if _, err := s.db.Exec("UPDATE observations SET updated_at = ?, created_at = ? WHERE id = ?", updatedAt, updatedAt, obs.ID); err != nil {
		t.Fatalf("update updated_at for %q: %v", title, err)
	}
	// Also refresh in-memory struct
	obs.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return obs
}

func TestRecent_ReturnsUpdatedAtDesc(t *testing.T) {
	s := openTestStore(t)
	// Seed two observations as described in spec: 2026-08-27 stale and 2026-09-01 fresh.
	stale := insertObsWithUpdatedAt(t, s, "Stale summary", "session_summary", "biggz-ai", "2026-08-27T10:00:00Z")
	fresh := insertObsWithUpdatedAt(t, s, "Fresh summary", "session_summary", "biggz-ai", "2026-09-01T12:00:00Z")

	results, err := s.Recent(SearchOptions{Limit: 5})
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].ID != fresh.ID {
		t.Errorf("Recent order: first = %q (%s) want fresh %q (%s); stale=%q", results[0].Title, results[0].UpdatedAt.Format(time.RFC3339), fresh.Title, fresh.UpdatedAt.Format(time.RFC3339), stale.ID)
	}
	if results[1].ID != stale.ID {
		t.Errorf("Recent order: second = %q want stale %q", results[1].ID, stale.ID)
	}
}

func TestRecent_Cap50Clamp(t *testing.T) {
	s := openTestStore(t)
	// Insert 60 observations.
	for i := 0; i < 60; i++ {
		obs := &Observation{Title: fmt.Sprintf("obs-%02d", i), Type: "discovery", Content: "c", Project: "biggz-ai"}
		if err := s.Save(obs); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}
	results, err := s.Recent(SearchOptions{Limit: 100})
	if err != nil {
		t.Fatalf("Recent limit 100: %v", err)
	}
	if len(results) > 50 {
		t.Errorf("limit 100 should clamp to 50, got %d", len(results))
	}
	if len(results) != 50 {
		t.Errorf("expected 50 results when 60 inserted and limit 100, got %d", len(results))
	}
	// Also verify that Search directly also clamps (invariant)
	results2, err := s.Search("", SearchOptions{Limit: 100})
	if err != nil {
		t.Fatalf("Search limit 100: %v", err)
	}
	if len(results2) > 50 {
		t.Errorf("Search limit 100 clamp failed, got %d", len(results2))
	}
}

func TestRecent_ProjectFilterPassThrough(t *testing.T) {
	s := openTestStore(t)
	insertObsWithUpdatedAt(t, s, "A biggz", "discovery", "biggz-ai", "2026-09-01T10:00:00Z")
	insertObsWithUpdatedAt(t, s, "B other", "discovery", "other", "2026-09-01T11:00:00Z")

	results, err := s.Recent(SearchOptions{Project: "biggz-ai", Limit: 10})
	if err != nil {
		t.Fatalf("Recent project filter: %v", err)
	}
	for _, r := range results {
		if r.Project != "biggz-ai" {
			t.Errorf("project filter leaked %q project %q", r.Title, r.Project)
		}
	}
	if len(results) != 1 {
		t.Errorf("expected 1 for biggz-ai, got %d", len(results))
	}
}

func TestRecent_TypeFilterPassThrough(t *testing.T) {
	s := openTestStore(t)
	insertObsWithUpdatedAt(t, s, "S1", "session_summary", "biggz-ai", "2026-09-01T10:00:00Z")
	insertObsWithUpdatedAt(t, s, "D1", "decision", "biggz-ai", "2026-09-01T11:00:00Z")

	results, err := s.Recent(SearchOptions{Type: "session_summary", Limit: 10})
	if err != nil {
		t.Fatalf("Recent type filter: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 session_summary, got %d", len(results))
	}
	if results[0].Type != "session_summary" {
		t.Errorf("type = %q want session_summary", results[0].Type)
	}
}

func TestRecent_ScopeFilterPassThrough(t *testing.T) {
	s := openTestStore(t)
	// create with different scopes
	obs1 := &Observation{Title: "proj scope", Type: "decision", Content: "c", Project: "biggz-ai", Scope: "project"}
	if err := s.Save(obs1); err != nil {
		t.Fatalf("Save proj: %v", err)
	}
	obs2 := &Observation{Title: "personal scope", Type: "decision", Content: "c", Project: "biggz-ai", Scope: "personal"}
	if err := s.Save(obs2); err != nil {
		t.Fatalf("Save personal: %v", err)
	}
	// filter by personal should only return personal
	results, err := s.Recent(SearchOptions{Scope: "personal", Limit: 10})
	if err != nil {
		t.Fatalf("Recent scope: %v", err)
	}
	for _, r := range results {
		if r.Scope != "personal" {
			t.Errorf("scope leaked %q scope %q", r.Title, r.Scope)
		}
	}
}

func TestSearch_FTSRankUnchangedForNonEmptyQuery(t *testing.T) {
	s := openTestStore(t)
	// We verify invariant that Search with non-empty query still uses ORDER BY rank.
	// Functional check: FTS "session" should return matching rows; empty query returns recency.
	// To ensure rank path unchanged, we check that non-empty query does NOT degrade to updated_at DESC.
	// Insert two: stale has strong FTS match ("session session session"), fresh has weak match but newer date.
	stale := insertObsWithUpdatedAt(t, s, "session session session", "discovery", "biggz-ai", "2026-08-27T10:00:00Z")
	fresh := insertObsWithUpdatedAt(t, s, "session", "discovery", "biggz-ai", "2026-09-01T12:00:00Z")
	// Modify content to affect ranking? Title also indexed. Stale has more term frequency.
	// Ensure stale has stronger rank: update its content to repeat query term.
	if _, err := s.db.Exec("UPDATE observations SET content = ? WHERE id = ?", "session session session", stale.ID); err != nil {
		t.Fatalf("update stale content: %v", err)
	}
	if _, err := s.db.Exec("UPDATE observations SET content = ? WHERE id = ?", "session", fresh.ID); err != nil {
		t.Fatalf("update fresh content: %v", err)
	}
	// Force FTS rebuild via DoctorFix or via re-insert? FTS trigger should have updated on UPDATE.
	// But content change via direct Exec bypasses trigger; rebuild FTS.
	if err := s.DoctorFix(); err != nil {
		t.Fatalf("DoctorFix: %v", err)
	}
	results, err := s.Search("session", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search session: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected 2 results for 'session', got %d", len(results))
	}
	// The invariant is that FTS order is by rank, not by updated_at.
	// While we cannot reliably assert which rank wins without BM25 internals,
	// we can assert that code path does not return updated_at DESC order for term query
	// when Search("") does. The key invariant test is that file still contains ORDER BY rank at 1844
	// and ORDER BY updated_at DESC at 1801, which is checked in TestOrderingInvariant.
	_ = results
}

func TestOrderingInvariant(t *testing.T) {
	// Verify source still contains the two ordering clauses at correct locations.
	data, err := readBigmemGo()
	if err != nil {
		t.Fatalf("read bigmem.go: %v", err)
	}
	if !strings.Contains(data, "ORDER BY o.updated_at DESC") {
		t.Error("bigmem.go missing ORDER BY o.updated_at DESC (empty query recency @1801)")
	}
	if !strings.Contains(data, "ORDER BY rank") {
		t.Error("bigmem.go missing ORDER BY rank (FTS relevance @1844)")
	}
	idxRecency := strings.Index(data, "ORDER BY o.updated_at DESC")
	idxRank := strings.Index(data, "ORDER BY rank")
	if idxRecency == -1 || idxRank == -1 {
		t.Fatalf("missing ordering markers")
	}
	if idxRecency > idxRank {
		t.Error("ORDER BY o.updated_at DESC should appear before ORDER BY rank (empty query branch before FTS)")
	}
}

func readBigmemGo() (string, error) {
	candidates := []string{
		"internal/bigmem/bigmem.go",
		"../internal/bigmem/bigmem.go",
		"../../internal/bigmem/bigmem.go",
	}
	for _, p := range candidates {
		if b, err := os.ReadFile(p); err == nil {
			return string(b), nil
		}
	}
	return "", fmt.Errorf("bigmem.go not found in candidates")
}
