package bigmem

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGraph_BuildAndRender(t *testing.T) {
	s := openTestStore(t)

	// Seed 3 observations with distinct topic_keys.
	obs1 := &Observation{Title: "Auth model", Type: "architecture", Content: "auth v1", TopicKey: "architecture/auth-model", Project: "proj"}
	obs2 := &Observation{Title: "Cache", Type: "architecture", Content: "cache v1", TopicKey: "architecture/cache", Project: "proj"}
	obs3 := &Observation{Title: "API design", Type: "decision", Content: "api v1", TopicKey: "decision/api-design", Project: "proj"}
	for _, o := range []*Observation{obs1, obs2, obs3} {
		if err := s.Save(o); err != nil {
			t.Fatalf("Save %s: %v", o.TopicKey, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	// Insert 2 relations directly (supersedes, related) with confidence.
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO memory_relations (id, source_id, target_id, relation, judgment_status, confidence, created_at, updated_at) VALUES (?, ?, ?, ?, 'judged', ?, ?, ?)`,
		"rel-1", obs1.ID, obs2.ID, "supersedes", 0.9, now, now)
	if err != nil {
		t.Fatalf("insert rel1: %v", err)
	}
	_, err = s.db.Exec(`INSERT INTO memory_relations (id, source_id, target_id, relation, judgment_status, confidence, created_at, updated_at) VALUES (?, ?, ?, ?, 'judged', ?, ?, ?)`,
		"rel-2", obs2.ID, obs3.ID, "related", 0.85, now, now)
	if err != nil {
		t.Fatalf("insert rel2: %v", err)
	}

	nodes, edges, err := s.BuildGraph("proj", 10)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if len(nodes) != 3 {
		t.Errorf("nodes = %d, want 3", len(nodes))
	}
	if len(edges) != 2 {
		t.Errorf("edges = %d, want 2", len(edges))
	}

	// ASCII should contain hierarchy and edge.
	ascii := RenderASCII(nodes, edges)
	if !strings.Contains(ascii, "architecture") {
		t.Errorf("ascii missing 'architecture': %q", ascii)
	}
	if !strings.Contains(ascii, "auth-model") {
		t.Errorf("ascii missing 'auth-model': %q", ascii)
	}
	if !strings.Contains(ascii, "--supersedes-->") {
		t.Errorf("ascii missing edge supersedes: %q", ascii)
	}
	if !strings.Contains(ascii, "0.9") {
		t.Errorf("ascii missing confidence 0.9: %q", ascii)
	}
	// Check tree connectors.
	if !strings.Contains(ascii, "├─") && !strings.Contains(ascii, "└─") {
		t.Errorf("ascii missing tree connectors: %q", ascii)
	}

	// DOT should contain digraph and arrow.
	dot := RenderDOT(nodes, edges)
	if !strings.Contains(dot, "digraph") {
		t.Errorf("dot missing 'digraph': %q", dot)
	}
	if !strings.Contains(dot, "->") {
		t.Errorf("dot missing '->': %q", dot)
	}
	if !strings.Contains(dot, "supersedes") {
		t.Errorf("dot missing supersedes label: %q", dot)
	}
	// Nodes as topic_keys
	if !strings.Contains(dot, "architecture/auth-model") {
		t.Errorf("dot missing topic_key node: %q", dot)
	}

	// JSON should contain nodes and edges.
	js, err := RenderJSON(nodes, edges)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	if !strings.Contains(js, "\"nodes\"") {
		t.Errorf("json missing nodes: %q", js)
	}
	if !strings.Contains(js, "\"edges\"") {
		t.Errorf("json missing edges: %q", js)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(js), &payload); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	var jNodes []GraphNode
	if err := json.Unmarshal(payload["nodes"], &jNodes); err != nil {
		t.Fatalf("nodes unmarshal: %v", err)
	}
	if len(jNodes) != 3 {
		t.Errorf("json nodes len = %d, want 3", len(jNodes))
	}
	var jEdges []GraphEdge
	if err := json.Unmarshal(payload["edges"], &jEdges); err != nil {
		t.Fatalf("edges unmarshal: %v", err)
	}
	if len(jEdges) != 2 {
		t.Errorf("json edges len = %d, want 2", len(jEdges))
	}
}

func TestGraph_Empty(t *testing.T) {
	s := openTestStore(t)
	nodes, edges, err := s.BuildGraph("", 10)
	if err != nil {
		t.Fatalf("BuildGraph empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
	ascii := RenderASCII(nodes, edges)
	if ascii != "No graph data" {
		t.Errorf("empty ascii = %q, want 'No graph data'", ascii)
	}
	dot := RenderDOT(nodes, edges)
	if dot != "No graph data" {
		t.Errorf("empty dot = %q, want 'No graph data'", dot)
	}
	js, _ := RenderJSON(nodes, edges)
	if !strings.Contains(js, "\"nodes\"") || !strings.Contains(js, "\"edges\"") {
		t.Errorf("empty json missing keys: %q", js)
	}
}

func TestGraph_ProjectFilter(t *testing.T) {
	s := openTestStore(t)
	a := &Observation{Title: "A", Type: "decision", Content: "a", TopicKey: "topic/a", Project: "proj-a"}
	b := &Observation{Title: "B", Type: "decision", Content: "b", TopicKey: "topic/b", Project: "proj-b"}
	if err := s.Save(a); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := s.Save(b); err != nil {
		t.Fatalf("save b: %v", err)
	}
	nodes, _, _ := s.BuildGraph("proj-a", 10)
	if len(nodes) != 1 {
		t.Errorf("filtered nodes = %d, want 1", len(nodes))
	}
	if len(nodes) == 1 && nodes[0].Project != "proj-a" {
		t.Errorf("filtered project = %q, want proj-a", nodes[0].Project)
	}
	nodesAll, _, _ := s.BuildGraph("", 10)
	if len(nodesAll) != 2 {
		t.Errorf("all nodes = %d, want 2", len(nodesAll))
	}
}

func TestGraph_RenderASCIIHierarchy(t *testing.T) {
	nodes := []GraphNode{
		{ID: "1", TopicKey: "architecture/auth-model", Title: "A", Type: "architecture"},
		{ID: "2", TopicKey: "architecture/cache", Title: "B", Type: "architecture"},
		{ID: "3", TopicKey: "decision/api-design", Title: "C", Type: "decision"},
	}
	edges := []GraphEdge{
		{SourceID: "1", TargetID: "2", Relation: "supersedes", Confidence: 0.9},
	}
	ascii := RenderASCII(nodes, edges)
	// Hierarchy: architecture/ with two children, decision/ with one.
	if !strings.Contains(ascii, "architecture/") {
		t.Errorf("missing architecture/: %q", ascii)
	}
	if !strings.Contains(ascii, "cache") {
		t.Errorf("missing cache: %q", ascii)
	}
	if !strings.Contains(ascii, "decision/") {
		t.Errorf("missing decision/: %q", ascii)
	}
}

func TestGraph_Limit(t *testing.T) {
	s := openTestStore(t)
	for i := 0; i < 5; i++ {
		o := &Observation{Title: "T", Type: "decision", Content: "c", TopicKey: "topic/" + string(rune('a'+i)), Project: "p"}
		// Ensure unique hash to avoid dedup collision.
		o.Content = "content-" + string(rune('a'+i)) + time.Now().String()
		if err := s.Save(o); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
		time.Sleep(1 * time.Millisecond)
	}
	nodes, _, _ := s.BuildGraph("p", 2)
	if len(nodes) != 2 {
		t.Errorf("limit nodes = %d, want 2", len(nodes))
	}
}
