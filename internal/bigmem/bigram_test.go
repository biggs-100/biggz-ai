package bigmem

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestFindCandidates_BM25Floor verifies that BM25Floor filtering shrinks candidates.
// Creates 5 observations with varying title overlap and compares permissive
// (-2.0) vs strict (0.0) floors.
func TestFindCandidates_BM25Floor(t *testing.T) {
	s := openTestStore(t)

	saved := &Observation{
		Title:   "authentication middleware JWT token design",
		Type:    "decision",
		Content: "saved content for bm25 floor test",
		Project: "proj-bm25",
		Scope:   "project",
	}
	if err := s.Save(saved); err != nil {
		t.Fatalf("Save saved: %v", err)
	}

	// Candidates with decreasing overlap so BM25 scores spread.
	candidates := []struct {
		title    string
		topicKey string
	}{
		{"authentication middleware JWT token", "bm25-c1"},
		{"authentication middleware JWT", "bm25-c2"},
		{"authentication middleware", "bm25-c3"},
		{"authentication token", "bm25-c4"},
		{"middleware JWT token", "bm25-c5"},
	}
	for _, c := range candidates {
		obs := &Observation{
			Title:    c.title,
			Type:     "decision",
			Content:  "content " + c.title,
			Project:  "proj-bm25",
			Scope:    "project",
			TopicKey: c.topicKey,
		}
		if err := s.Save(obs); err != nil {
			t.Fatalf("Save candidate %q: %v", c.title, err)
		}
	}

	// FTS triggers are synchronous, but a tiny sleep avoids Windows clock colliding
	// rel IDs (not needed for FTS itself).
	time.Sleep(5 * time.Millisecond)

	floorPermissive := -2.0
	permissive, err := s.FindCandidatesWithOptions(saved.ID, saved.Title, FindOptions{
		Project:   "proj-bm25",
		Scope:     "project",
		Limit:     5,
		BM25Floor: &floorPermissive,
	})
	if err != nil {
		t.Fatalf("FindCandidatesWithOptions permissive: %v", err)
	}

	floorStrict := 0.0
	strict, err := s.FindCandidatesWithOptions(saved.ID, saved.Title, FindOptions{
		Project:   "proj-bm25",
		Scope:     "project",
		Limit:     5,
		BM25Floor: &floorStrict,
	})
	if err != nil {
		t.Fatalf("FindCandidatesWithOptions strict: %v", err)
	}

	t.Logf("permissive(%v) = %d candidates, strict(%v) = %d", floorPermissive, len(permissive), floorStrict, len(strict))
	for _, c := range permissive {
		t.Logf("  permissive candidate: %q score=%.4f judgment=%s", c.Title, c.Score, c.JudgmentID)
	}
	for _, c := range strict {
		t.Logf("  strict candidate: %q score=%.4f", c.Title, c.Score)
	}

	if len(permissive) == 0 {
		t.Fatalf("expected at least 1 candidate with permissive floor -2.0, got 0")
	}
	if len(strict) >= len(permissive) {
		t.Errorf("expected strict floor 0.0 to return fewer candidates than permissive -2.0: strict=%d permissive=%d", len(strict), len(permissive))
	}
	// Strict floor 0.0 should filter everything because BM25 scores are negative.
	if len(strict) != 0 {
		t.Errorf("expected 0 candidates with strict floor 0.0 (all BM25 negative), got %d", len(strict))
	}

	// Nil floor means no filtering — should return up to limit.
	nilCands, err := s.FindCandidatesWithOptions(saved.ID, saved.Title, FindOptions{
		Project: "proj-bm25",
		Scope:   "project",
		Limit:   5,
		// BM25Floor nil
	})
	if err != nil {
		t.Fatalf("FindCandidatesWithOptions nil floor: %v", err)
	}
	if len(nilCands) == 0 {
		t.Fatalf("expected >=1 candidate with nil floor, got 0")
	}
	if len(nilCands) < len(permissive) {
		t.Errorf("nil floor should be at least as permissive as -2.0: nil=%d permissive=%d", len(nilCands), len(permissive))
	}

	// Backward compat wrapper: FindCandidates(savedID,title,project,scope) should behave like -2.0.
	wrapped, err := s.FindCandidates(saved.ID, saved.Title, "proj-bm25", "project")
	if err != nil {
		t.Fatalf("FindCandidates wrapper: %v", err)
	}
	// Wrapper uses -2.0, so it should equal permissive count (within timing variance due to new rel inserts limiting).
	// We cannot assert exact equality because previous calls already inserted relations, but wrapper should still return candidates.
	if len(wrapped) == 0 {
		t.Errorf("expected wrapper FindCandidates to return candidates, got 0")
	}
}

// TestJudgeRelation_CrossProjectGuard verifies that judging a relation
// between observations in different projects fails with ErrCrossProjectRelation.
func TestJudgeRelation_CrossProjectGuard(t *testing.T) {
	s := openTestStore(t)

	// Two observations in different projects.
	obsA := &Observation{
		Title:    "Cross project A",
		Type:     "decision",
		Content:  "content A",
		Project:  "proj-alpha",
		Scope:    "project",
		TopicKey: "cross-a",
	}
	if err := s.Save(obsA); err != nil {
		t.Fatalf("Save A: %v", err)
	}
	obsB := &Observation{
		Title:    "Cross project B",
		Type:     "decision",
		Content:  "content B",
		Project:  "proj-beta",
		Scope:    "project",
		TopicKey: "cross-b",
	}
	if err := s.Save(obsB); err != nil {
		t.Fatalf("Save B: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	// Directly insert a pending relation between the two cross-project observations.
	relID := fmt.Sprintf("rel-%d-a", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`INSERT INTO memory_relations (id, source_id, target_id, relation, judgment_status, session_id, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', 'pending', '', ?, ?)`, relID, obsA.ID, obsB.ID, now, now); err != nil {
		t.Fatalf("insert cross-project relation: %v", err)
	}

	err := s.JudgeRelation(relID, "related", "test", "", 0.9)
	if err == nil {
		t.Fatalf("expected ErrCrossProjectRelation, got nil")
	}
	if !errors.Is(err, ErrCrossProjectRelation) {
		t.Fatalf("expected ErrCrossProjectRelation, got %v", err)
	}

	// Verify relation is still pending (not mutated).
	var status string
	if err := s.db.QueryRow(`SELECT judgment_status FROM memory_relations WHERE id = ?`, relID).Scan(&status); err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != "pending" {
		t.Errorf("expected status pending after blocked judge, got %q", status)
	}

	// Same-project relation should succeed.
	obsC := &Observation{
		Title:    "Same project C",
		Type:     "decision",
		Content:  "content C",
		Project:  "proj-alpha",
		Scope:    "project",
		TopicKey: "cross-c",
	}
	if err := s.Save(obsC); err != nil {
		t.Fatalf("Save C: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	relID2 := fmt.Sprintf("rel-%d-b", time.Now().UnixNano())
	if _, err := s.db.Exec(`INSERT INTO memory_relations (id, source_id, target_id, relation, judgment_status, session_id, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', 'pending', '', ?, ?)`, relID2, obsA.ID, obsC.ID, now, now); err != nil {
		t.Fatalf("insert same-project relation: %v", err)
	}

	if err := s.JudgeRelation(relID2, "related", "same project ok", "", 0.8); err != nil {
		t.Fatalf("expected same-project JudgeRelation to succeed, got %v", err)
	}
	var status2 string
	if err := s.db.QueryRow(`SELECT judgment_status FROM memory_relations WHERE id = ?`, relID2).Scan(&status2); err != nil {
		t.Fatalf("query status2: %v", err)
	}
	if status2 != "judged" {
		t.Errorf("expected status judged after same-project judge, got %q", status2)
	}

	// Different direction (target -> source) with cross projects should also be blocked.
	relID3 := fmt.Sprintf("rel-%d-c", time.Now().UnixNano())
	if _, err := s.db.Exec(`INSERT INTO memory_relations (id, source_id, target_id, relation, judgment_status, session_id, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', 'pending', '', ?, ?)`, relID3, obsB.ID, obsA.ID, now, now); err != nil {
		t.Fatalf("insert cross-project reverse relation: %v", err)
	}
	if err := s.JudgeRelation(relID3, "compatible", "", "", 0.5); !errors.Is(err, ErrCrossProjectRelation) {
		t.Fatalf("expected ErrCrossProjectRelation for reverse cross-project, got %v", err)
	}
}
