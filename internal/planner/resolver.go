package planner

import "fmt"

// Resolver takes a pre-built dependency Graph and resolves a Selection
// into an ordered Plan with all transitive dependencies included.
type Resolver struct {
	graph *Graph
}

// NewResolver creates a Resolver backed by the given Graph.
// The graph should be pre-populated with all known components and
// their dependency edges before calling Resolve.
func NewResolver(g *Graph) *Resolver {
	return &Resolver{graph: g}
}

// Resolve computes a Plan from the given Selection. It:
//  1. Collects all selected components and their transitive dependencies
//  2. Topologically sorts the full set
//  3. Identifies which components were auto-added (not in the selection)
//
// Returns an error if any selected component is unknown or if the graph
// contains a cycle.
func (r *Resolver) Resolve(sel Selection) (*Plan, error) {
	// Validate selected components exist in the graph
	for _, c := range sel.Components {
		if _, ok := r.graph.edges[c]; !ok {
			return nil, fmt.Errorf("unknown component %q", c)
		}
	}
	for _, s := range sel.Skills {
		if _, ok := r.graph.edges[s]; !ok {
			return nil, fmt.Errorf("unknown skill %q", s)
		}
	}

	// Collect the set of explicitly selected nodes
	selected := make(map[string]bool)
	for _, c := range sel.Components {
		selected[c] = true
	}
	for _, s := range sel.Skills {
		selected[s] = true
	}

	// Collect all transitive dependencies of selected nodes
	allNeeded := make(map[string]bool)
	for node := range selected {
		allNeeded[node] = true
		for _, dep := range r.graph.DependenciesOf(node) {
			allNeeded[dep] = true
		}
	}

	// Sort the full graph and filter to only what's needed
	sorted, err := r.graph.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("resolve: %w", err)
	}

	ordered := make([]string, 0, len(allNeeded))
	for _, node := range sorted {
		if allNeeded[node] {
			ordered = append(ordered, node)
		}
	}

	// Auto-added = needed but not explicitly selected
	var autoAdded []string
	for node := range allNeeded {
		if !selected[node] {
			autoAdded = append(autoAdded, node)
		}
	}

	return &Plan{
		Components:   ordered,
		Dependencies: autoAdded,
	}, nil
}
