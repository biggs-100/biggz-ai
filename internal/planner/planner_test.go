package planner

import (
	"testing"
)

// mustAddEdge calls AddEdge and fails the test if it returns an error.
func mustAddEdge(t *testing.T, g *Graph, from, to string) {
	t.Helper()
	if err := g.AddEdge(from, to); err != nil {
		t.Fatalf("AddEdge(%q, %q): %v", from, to, err)
	}
}

// -- Graph: TopologicalSort ------------------------------------------------

func TestTopologicalSort_EmptyGraph(t *testing.T) {
	g := NewGraph()
	result, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("got %d nodes, want 0", len(result))
	}
}

func TestTopologicalSort_SingleNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("skills")
	result, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "skills" {
		t.Errorf("got %v, want [skills]", result)
	}
}

func TestTopologicalSort_LinearChain(t *testing.T) {
	g := NewGraph()
	// skills → config → prompts
	g.AddNode("prompts")
	g.AddNode("config")
	g.AddNode("skills")
	mustAddEdge(t, g, "prompts", "config")
	mustAddEdge(t, g, "config", "skills")

	result, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("got %d nodes, want 3", len(result))
	}
	// Dependencies must come first
	if result[0] != "skills" {
		t.Errorf("first should be skills, got %s", result[0])
	}
	if result[1] != "config" {
		t.Errorf("second should be config, got %s", result[1])
	}
	if result[2] != "prompts" {
		t.Errorf("third should be prompts, got %s", result[2])
	}
}

func TestTopologicalSort_Diamond(t *testing.T) {
	g := NewGraph()
	//     a
	//    / \
	//   b   c
	//    \ /
	//     d
	// d depends on b and c; b depends on a; c depends on a
	for _, n := range []string{"a", "b", "c", "d"} {
		g.AddNode(n)
	}
	mustAddEdge(t, g, "d", "b")
	mustAddEdge(t, g, "d", "c")
	mustAddEdge(t, g, "b", "a")
	mustAddEdge(t, g, "c", "a")

	result, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 4 {
		t.Fatalf("got %d nodes, want 4", len(result))
	}
	// a must come before b and c; b and c must come before d
	pos := make(map[string]int)
	for i, n := range result {
		pos[n] = i
	}
	if pos["a"] > pos["b"] || pos["a"] > pos["c"] {
		t.Error("a must come before b and c")
	}
	if pos["b"] > pos["d"] || pos["c"] > pos["d"] {
		t.Error("b and c must come before d")
	}
}

func TestTopologicalSort_CycleDetection(t *testing.T) {
	g := NewGraph()
	// a depends on b, b depends on c, c depends on a (cycle)
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", "c")
	mustAddEdge(t, g, "c", "a")

	result, err := g.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	// Pure cycle = no nodes have in-degree 0, so queue starts empty
	if len(result) != 0 {
		t.Errorf("expected 0 ordered nodes for pure cycle, got %d", len(result))
	}
}

func TestTopologicalSort_PartialCycle(t *testing.T) {
	g := NewGraph()
	// d is independent; a → b → c → a (cycle)
	for _, n := range []string{"a", "b", "c", "d"} {
		g.AddNode(n)
	}
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", "c")
	mustAddEdge(t, g, "c", "a")

	result, err := g.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	// d should be orderable before the cycle is detected
	if len(result) != 1 || result[0] != "d" {
		t.Errorf("expected partial result [d], got %v", result)
	}
}

func TestTopologicalSort_IndependentNodes(t *testing.T) {
	g := NewGraph()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")

	result, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("got %d nodes, want 3", len(result))
	}
}

// -- Graph: DependenciesOf -------------------------------------------------

func TestDependenciesOf_NoDeps(t *testing.T) {
	g := NewGraph()
	g.AddNode("skills")
	deps := g.DependenciesOf("skills")
	if len(deps) != 0 {
		t.Errorf("got %d deps, want 0", len(deps))
	}
}

func TestDependenciesOf_Transitive(t *testing.T) {
	g := NewGraph()
	g.AddNode("prompts")
	g.AddNode("config")
	g.AddNode("skills")
	mustAddEdge(t, g, "prompts", "config")
	mustAddEdge(t, g, "config", "skills")

	deps := g.DependenciesOf("prompts")
	if len(deps) != 2 {
		t.Fatalf("got %d deps, want 2", len(deps))
	}

	hasSkills, hasConfig := false, false
	for _, d := range deps {
		if d == "skills" {
			hasSkills = true
		}
		if d == "config" {
			hasConfig = true
		}
	}
	if !hasSkills {
		t.Error("missing transitive dep 'skills'")
	}
	if !hasConfig {
		t.Error("missing direct dep 'config'")
	}
}

func TestDependenciesOf_UnknownNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("skills")
	deps := g.DependenciesOf("unknown")
	if len(deps) != 0 {
		t.Errorf("got %d deps for unknown node, want 0", len(deps))
	}
}

// -- Graph: AddEdge / AddNode -----------------------------------------------

func TestAddEdge_UnknownNode(t *testing.T) {
	g := NewGraph()
	err := g.AddEdge("a", "b")
	if err == nil {
		t.Fatal("expected error for unknown node 'a'")
	}
}

func TestAddEdge_Valid(t *testing.T) {
	g := NewGraph()
	g.AddNode("a")
	g.AddNode("b")
	mustAddEdge(t, g, "a", "b")

	result, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d nodes, want 2", len(result))
	}
}

// -- Resolver ---------------------------------------------------------------

func TestResolver_ResolveSimple(t *testing.T) {
	g := NewGraph()
	g.AddNode("prompts")
	g.AddNode("config")
	g.AddNode("skills")
	mustAddEdge(t, g, "prompts", "config")
	mustAddEdge(t, g, "config", "skills")

	r := NewResolver(g)
	plan, err := r.Resolve(Selection{
		Components: []string{"prompts"},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Plan should include skills, config (auto-added) + prompts
	if len(plan.Components) != 3 {
		t.Fatalf("got %d components, want 3: %v", len(plan.Components), plan.Components)
	}
	if plan.Components[0] != "skills" || plan.Components[1] != "config" || plan.Components[2] != "prompts" {
		t.Errorf("wrong order: %v", plan.Components)
	}
	// Auto-added = skills + config
	if len(plan.Dependencies) != 2 {
		t.Errorf("got %d deps, want 2: %v", len(plan.Dependencies), plan.Dependencies)
	}
}

func TestResolver_ResolveNoDeps(t *testing.T) {
	g := NewGraph()
	g.AddNode("skills")
	g.AddNode("config")

	r := NewResolver(g)
	plan, err := r.Resolve(Selection{
		Components: []string{"skills", "config"},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(plan.Components) != 2 {
		t.Errorf("got %d, want 2: %v", len(plan.Components), plan.Components)
	}
	if len(plan.Dependencies) != 0 {
		t.Errorf("got %d auto-deps, want 0", len(plan.Dependencies))
	}
}

func TestResolver_ResolveUnknownComponent(t *testing.T) {
	g := NewGraph()
	g.AddNode("skills")

	r := NewResolver(g)
	_, err := r.Resolve(Selection{
		Components: []string{"nonexistent"},
	})
	if err == nil {
		t.Fatal("expected error for unknown component")
	}
}

func TestResolver_ResolveCycle(t *testing.T) {
	g := NewGraph()
	g.AddNode("a")
	g.AddNode("b")
	mustAddEdge(t, g, "a", "b")
	mustAddEdge(t, g, "b", "a")

	r := NewResolver(g)
	_, err := r.Resolve(Selection{
		Components: []string{"a"},
	})
	if err == nil {
		t.Fatal("expected error for cycle in graph")
	}
}

func TestResolver_ResolveMultipleSelected(t *testing.T) {
	g := NewGraph()
	// prompts → config → skills
	g.AddNode("prompts")
	g.AddNode("config")
	g.AddNode("skills")
	mustAddEdge(t, g, "prompts", "config")
	mustAddEdge(t, g, "config", "skills")

	r := NewResolver(g)
	plan, err := r.Resolve(Selection{
		Components: []string{"skills", "prompts"},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Both selected + auto-added config
	if len(plan.Components) != 3 {
		t.Fatalf("got %d, want 3: %v", len(plan.Components), plan.Components)
	}
	// skills and config before prompts
	if plan.Components[2] != "prompts" {
		t.Errorf("prompts should be last, got %v", plan.Components)
	}
	// Only config should be auto-added (skills was explicitly selected)
	if len(plan.Dependencies) != 1 || plan.Dependencies[0] != "config" {
		t.Errorf("auto-deps should be [config], got %v", plan.Dependencies)
	}
}

func TestResolver_ResolveWithSkills(t *testing.T) {
	g := NewGraph()
	g.AddNode("config")
	g.AddNode("code-review")
	g.AddNode("auto-fix")
	mustAddEdge(t, g, "auto-fix", "code-review")

	r := NewResolver(g)
	plan, err := r.Resolve(Selection{
		Components: []string{"config"},
		Skills:     []string{"auto-fix"},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Should include: config, code-review (auto), auto-fix
	if len(plan.Components) != 3 {
		t.Fatalf("got %d components, want 3: %v", len(plan.Components), plan.Components)
	}
	if len(plan.Dependencies) != 1 || plan.Dependencies[0] != "code-review" {
		t.Errorf("auto-deps should be [code-review], got %v", plan.Dependencies)
	}
}
