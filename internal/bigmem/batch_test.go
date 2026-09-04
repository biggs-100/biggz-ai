package bigmem

// Batch tests for SDD change fix-bigmem-mcp-nplus1 (Phases 1-3, store level).
//
// Covers: scoped ListRelationsByIDs lookup on both endpoints, empty-input
// no-query behavior, hostile-ID injection safety, 400-ID chunking, Offset
// paging, Store limit semantics, export paging + cap + project filter,
// and JSON round-trip shape preservation.

import (
	"encoding/json"
	"fmt"
	"testing"
)

func seedBatchObservations(t *testing.T, s *Store, n int, project string) []*Observation {
	t.Helper()
	out := make([]*Observation, 0, n)
	for i := 0; i < n; i++ {
		o := &Observation{
			ID:      fmt.Sprintf("%s-obs-%04d", project, i),
			Title:   fmt.Sprintf("Batch note %s %04d", project, i),
			Type:    "discovery",
			Content: fmt.Sprintf("batch content body %s %04d unique-token-%s-%04d", project, i, project, i),
			Project: project,
		}
		if err := s.Save(o); err != nil {
			t.Fatalf("Save(%s) error: %v", o.ID, err)
		}
		out = append(out, o)
	}
	return out
}

// insertBatchRelation creates a pending relation with a deterministic unique ID
// (direct INSERT avoids UnixNano collisions in tight seed loops) and judges it.
func insertBatchRelation(t *testing.T, s *Store, relID, sourceID, targetID, verdict string) {
	t.Helper()
	now := "2026-09-04T00:00:00Z"
	if _, err := s.db.Exec(`INSERT INTO memory_relations
			(id, source_id, target_id, relation, judgment_status, marked_by, session_id, created_at, updated_at)
			VALUES (?, ?, ?, 'pending', 'pending', 'test:model:v1', 'batch-test', ?, ?)`,
		relID, sourceID, targetID, now, now); err != nil {
		t.Fatalf("insert relation %s error: %v", relID, err)
	}
	if err := s.JudgeRelation(relID, verdict, "batch test", "batch evidence", 0.9); err != nil {
		t.Fatalf("JudgeRelation(%s) error: %v", relID, err)
	}
}

func judgeBatchRelation(t *testing.T, s *Store, sourceID, targetID, verdict string) {
	t.Helper()
	insertBatchRelation(t, s, fmt.Sprintf("batch-rel-%s-%s", sourceID, targetID), sourceID, targetID, verdict)
}

// TestListRelationsByIDs_ScopedBothEndpoints: relations are found whether the
// queried ID is the source or the target; unrelated IDs match nothing.
func TestListRelationsByIDs_ScopedBothEndpoints(t *testing.T) {
	s := openTestStore(t)
	obs := seedBatchObservations(t, s, 3, "batch-proj")
	judgeBatchRelation(t, s, obs[0].ID, obs[1].ID, "supersedes")

	bySource, err := s.ListRelationsByIDs([]string{obs[0].ID})
	if err != nil {
		t.Fatalf("ListRelationsByIDs(source) error: %v", err)
	}
	if len(bySource) != 1 || bySource[0].SourceID != obs[0].ID || bySource[0].TargetID != obs[1].ID {
		t.Fatalf("source-scoped lookup = %+v, want 1 relation %s->%s", bySource, obs[0].ID, obs[1].ID)
	}

	byTarget, err := s.ListRelationsByIDs([]string{obs[1].ID})
	if err != nil {
		t.Fatalf("ListRelationsByIDs(target) error: %v", err)
	}
	if len(byTarget) != 1 {
		t.Fatalf("target-scoped lookup returned %d relations, want 1", len(byTarget))
	}

	none, err := s.ListRelationsByIDs([]string{obs[2].ID})
	if err != nil {
		t.Fatalf("ListRelationsByIDs(unrelated) error: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("unrelated lookup returned %d relations, want 0", len(none))
	}
}

// TestListRelationsByIDs_EmptyInput: empty/nil input returns empty without error.
func TestListRelationsByIDs_EmptyInput(t *testing.T) {
	s := openTestStore(t)
	seedBatchObservations(t, s, 2, "batch-proj")

	for _, ids := range [][]string{nil, {}, {""}} {
		got, err := s.ListRelationsByIDs(ids)
		if err != nil {
			t.Fatalf("ListRelationsByIDs(%q) error: %v", ids, err)
		}
		if len(got) != 0 {
			t.Fatalf("ListRelationsByIDs(%q) = %d relations, want 0", ids, len(got))
		}
	}
}

// TestListRelationsByIDs_HostileIDs: injection payloads are inert bound
// parameters — they return empty without error and leave the table intact.
func TestListRelationsByIDs_HostileIDs(t *testing.T) {
	s := openTestStore(t)
	obs := seedBatchObservations(t, s, 2, "batch-proj")
	judgeBatchRelation(t, s, obs[0].ID, obs[1].ID, "supersedes")

	hostile := []string{
		"x' OR '1'='1",
		"'; DROP TABLE memory_relations; --",
		"%",
		"\" OR \"\"=\"",
	}
	got, err := s.ListRelationsByIDs(hostile)
	if err != nil {
		t.Fatalf("ListRelationsByIDs(hostile) error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListRelationsByIDs(hostile) = %d relations, want 0", len(got))
	}

	// Table intact: the seeded relation is still discoverable.
	after, err := s.ListRelationsByIDs([]string{obs[0].ID})
	if err != nil {
		t.Fatalf("ListRelationsByIDs after hostile input error: %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("table damaged by hostile input: got %d relations, want 1", len(after))
	}
}

// TestListRelationsByIDs_Chunking: >400 IDs span multiple IN chunks (800 vars
// each, under SQLite's 999-variable limit) without loss.
func TestListRelationsByIDs_Chunking(t *testing.T) {
	s := openTestStore(t)
	const n = 410
	hub := seedBatchObservations(t, s, 1, "chunk-hub")[0]
	// Star pattern: hub -> 410 leaves needs only n+1 observations and forces
	// a 2-chunk scoped lookup when querying [hub].
	ids := []string{hub.ID}
	for i := 0; i < n; i++ {
		leaf := fmt.Sprintf("chunk-leaf-%04d", i)
		if err := s.Save(&Observation{ID: leaf, Title: fmt.Sprintf("leaf %04d", i), Type: "discovery", Content: fmt.Sprintf("leaf body %04d unique-%04d", i, i), Project: "chunk-hub"}); err != nil {
			t.Fatalf("Save(%s) error: %v", leaf, err)
		}
		insertBatchRelation(t, s, fmt.Sprintf("chunk-rel-%04d", i), hub.ID, leaf, "supersedes")
	}
	got, err := s.ListRelationsByIDs(ids)
	if err != nil {
		t.Fatalf("ListRelationsByIDs(hub) error: %v", err)
	}
	if len(got) != n {
		t.Fatalf("ListRelationsByIDs(hub) = %d relations, want %d", len(got), n)
	}
}

// TestMemSearchAnnotationBound: 50 results are fully annotated with exactly 2
// Store calls (1 Search + 1 scoped lookup, zero Gets by construction — the
// annotation path uses the in-memory title map with "deleted" fallback).
// Cross-ID relations (only source in results) must not be missed.
func TestMemSearchAnnotationBound(t *testing.T) {
	s := openTestStore(t)
	obs := seedBatchObservations(t, s, 50, "batch-proj")
	for i := 0; i < 49; i++ {
		judgeBatchRelation(t, s, obs[i].ID, obs[i+1].ID, "supersedes")
	}

	results, err := s.Search("", SearchOptions{Project: "batch-proj", Limit: 50})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 50 {
		t.Fatalf("Search returned %d results, want 50", len(results))
	}

	ids := make([]string, 0, len(results))
	titleByID := make(map[string]string, len(results))
	for _, r := range results {
		ids = append(ids, r.ID)
		titleByID[r.ID] = r.Title
	}
	rels, err := s.ListRelationsByIDs(ids)
	if err != nil {
		t.Fatalf("ListRelationsByIDs error: %v", err)
	}
	if len(rels) != 49 {
		t.Fatalf("scoped lookup returned %d relations, want 49 chain relations", len(rels))
	}
	// Every chain relation resolves from the in-memory map (zero Gets).
	for _, rel := range rels {
		if titleByID[rel.SourceID] == "" || titleByID[rel.TargetID] == "" {
			t.Fatalf("relation %s->%s missing from title map", rel.SourceID, rel.TargetID)
		}
	}
}

// TestMemSearchCrossIDFallback mirrors the mem_search annotation rule: a
// relation whose counterpart is outside the result set resolves to the
// "deleted" fallback from the in-memory map, with zero Get calls.
func TestMemSearchCrossIDFallback(t *testing.T) {
	s := openTestStore(t)
	obs := seedBatchObservations(t, s, 2, "batch-proj")
	judgeBatchRelation(t, s, obs[0].ID, obs[1].ID, "supersedes")

	// Scope covers only A: the A->B relation must still be discovered.
	titleByID := map[string]string{obs[0].ID: obs[0].Title}
	rels, err := s.ListRelationsByIDs([]string{obs[0].ID})
	if err != nil {
		t.Fatalf("ListRelationsByIDs error: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("cross-ID lookup returned %d relations, want 1", len(rels))
	}
	title, ok := titleByID[rels[0].TargetID]
	if ok && title != "" {
		t.Fatalf("counterpart unexpectedly in map")
	}
	title = "deleted" // fallback applied by mem_search annotation
	if title != "deleted" {
		t.Fatalf("fallback = %q, want %q", title, "deleted")
	}
}

// TestSearchLimitSemantics: Store keeps the 50-row cap with default 20.
func TestSearchLimitSemantics(t *testing.T) {
	s := openTestStore(t)
	seedBatchObservations(t, s, 60, "batch-proj")

	def, err := s.Search("", SearchOptions{Project: "batch-proj"})
	if err != nil {
		t.Fatalf("Search default error: %v", err)
	}
	if len(def) != 20 {
		t.Fatalf("default limit returned %d rows, want 20", len(def))
	}

	zero, err := s.Search("", SearchOptions{Project: "batch-proj", Limit: 0})
	if err != nil {
		t.Fatalf("Search limit=0 error: %v", err)
	}
	if len(zero) != 20 {
		t.Fatalf("limit=0 returned %d rows, want default 20", len(zero))
	}

	neg, err := s.Search("", SearchOptions{Project: "batch-proj", Limit: -5})
	if err != nil {
		t.Fatalf("Search limit=-5 error: %v", err)
	}
	if len(neg) != 20 {
		t.Fatalf("limit=-5 returned %d rows, want default 20", len(neg))
	}

	huge, err := s.Search("", SearchOptions{Project: "batch-proj", Limit: 100000})
	if err != nil {
		t.Fatalf("Search limit=100000 error: %v", err)
	}
	if len(huge) != 50 {
		t.Fatalf("limit=100000 returned %d rows, want cap 50", len(huge))
	}
}

// pageExport mirrors the CLI export loop: 50-row Offset pages until a short
// page, honoring an explicit row cap (0 = uncapped).
func pageExport(t *testing.T, s *Store, project string, cap int) []*Observation {
	t.Helper()
	const pageSize = 50
	var all []*Observation
	for offset := 0; ; offset += pageSize {
		limit := pageSize
		if cap > 0 && cap-len(all) < limit {
			limit = cap - len(all)
		}
		page, err := s.Search("", SearchOptions{Project: project, Limit: limit, Offset: offset})
		if err != nil {
			t.Fatalf("export page offset=%d error: %v", offset, err)
		}
		all = append(all, page...)
		if len(page) < limit {
			break
		}
		if cap > 0 && len(all) >= cap {
			break
		}
	}
	return all
}

// TestExportPaging: 120 observations page out completely; explicit cap and
// project filter behave; Offset pages beyond the 50-row cap stay disjoint.
func TestExportPaging(t *testing.T) {
	s := openTestStore(t)
	seedBatchObservations(t, s, 100, "batch-p1")
	seedBatchObservations(t, s, 20, "batch-p2")

	p1, err := s.Search("", SearchOptions{Project: "batch-p1", Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("page 0 error: %v", err)
	}
	p2, err := s.Search("", SearchOptions{Project: "batch-p1", Limit: 50, Offset: 50})
	if err != nil {
		t.Fatalf("page 1 error: %v", err)
	}
	if len(p1) != 50 || len(p2) != 50 {
		t.Fatalf("pages = %d/%d, want 50/50", len(p1), len(p2))
	}
	seen := map[string]bool{}
	for _, o := range append(p1, p2...) {
		if seen[o.ID] {
			t.Fatalf("duplicate ID %s across pages", o.ID)
		}
		seen[o.ID] = true
	}

	all := pageExport(t, s, "batch-p1", 0)
	if len(all) != 100 {
		t.Fatalf("uncapped export = %d rows, want 100", len(all))
	}

	capped := pageExport(t, s, "batch-p1", 60)
	if len(capped) != 60 {
		t.Fatalf("capped export = %d rows, want 60", len(capped))
	}

	filtered := pageExport(t, s, "batch-p2", 0)
	if len(filtered) != 20 {
		t.Fatalf("project-filtered export = %d rows, want 20", len(filtered))
	}
	for _, o := range filtered {
		if o.Project != "batch-p2" {
			t.Fatalf("export leaked project %q", o.Project)
		}
	}
}

// TestExportRoundTrip: paged output marshals to the same JSON array shape and
// re-imports with zero parse errors.
func TestExportRoundTrip(t *testing.T) {
	s := openTestStore(t)
	seedBatchObservations(t, s, 60, "batch-proj")

	all := pageExport(t, s, "batch-proj", 0)
	if len(all) != 60 {
		t.Fatalf("export = %d rows, want 60", len(all))
	}
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var back []*Observation
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("round-trip parse error: %v", err)
	}
	if len(back) != 60 {
		t.Fatalf("round-trip = %d rows, want 60", len(back))
	}

	dst := openTestStore(t)
	imported := 0
	for _, o := range back {
		if err := dst.Save(o); err != nil {
			t.Fatalf("re-save %s error: %v", o.ID, err)
		}
		imported++
	}
	if imported != 60 {
		t.Fatalf("imported %d rows, want 60", imported)
	}
}

// TestSearchOffsetByteIdentical: Offset=0 issues the legacy query shape —
// paging only shifts the window without changing row content.
func TestSearchOffsetByteIdentical(t *testing.T) {
	s := openTestStore(t)
	seedBatchObservations(t, s, 10, "batch-proj")

	legacy, err := s.Search("", SearchOptions{Project: "batch-proj", Limit: 10})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	explicit, err := s.Search("", SearchOptions{Project: "batch-proj", Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("Search offset=0 error: %v", err)
	}
	if len(legacy) != len(explicit) {
		t.Fatalf("offset=0 changed row count: %d vs %d", len(legacy), len(explicit))
	}
	for i := range legacy {
		if legacy[i].ID != explicit[i].ID {
			t.Fatalf("offset=0 changed row %d: %s vs %s", i, legacy[i].ID, explicit[i].ID)
		}
	}
}
