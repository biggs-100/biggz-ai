package planner

import "fmt"

// Graph represents a directed graph of component dependencies.
// An edge from → to means "from depends on to": to must be deployed
// before from. Nodes must be added via AddNode before AddEdge.
type Graph struct {
	edges map[string][]string // from → list of dependencies
}

// NewGraph creates an empty Graph ready for use.
func NewGraph() *Graph {
	return &Graph{edges: make(map[string][]string)}
}

// AddNode ensures a node exists in the graph. No-op if the node
// is already present.
func (g *Graph) AddNode(id string) {
	if _, ok := g.edges[id]; !ok {
		g.edges[id] = []string{}
	}
}

// AddEdge adds a dependency edge: from depends on to.
// Returns an error if either node hasn't been added via AddNode.
func (g *Graph) AddEdge(from, to string) error {
	if _, ok := g.edges[from]; !ok {
		return fmt.Errorf("unknown node %q: must call AddNode first", from)
	}
	if _, ok := g.edges[to]; !ok {
		return fmt.Errorf("unknown node %q: must call AddNode first", to)
	}
	g.edges[from] = append(g.edges[from], to)
	return nil
}

// DependenciesOf returns all transitive dependencies of the given node
// in no particular order. The node itself is not included.
func (g *Graph) DependenciesOf(node string) []string {
	visited := make(map[string]bool)
	var deps []string
	var walk func(n string)
	walk = func(n string) {
		for _, dep := range g.edges[n] {
			if !visited[dep] {
				visited[dep] = true
				deps = append(deps, dep)
				walk(dep)
			}
		}
	}
	walk(node)
	return deps
}

// TopologicalSort returns all nodes in dependency order (dependencies
// first) using Kahn's algorithm. Returns an error with the partial
// result if a cycle is detected — callers can still inspect the
// ordered prefix.
func (g *Graph) TopologicalSort() ([]string, error) {
	// inDegree maps node → number of dependencies (incoming edges)
	inDegree := make(map[string]int, len(g.edges))
	// dependents maps dependency → list of nodes that depend on it
	dependents := make(map[string][]string)

	for from, deps := range g.edges {
		if _, ok := inDegree[from]; !ok {
			inDegree[from] = 0
		}
		for _, to := range deps {
			dependents[to] = append(dependents[to], from)
			inDegree[from]++
		}
	}

	// Seed queue with nodes that have zero dependencies
	queue := make([]string, 0, len(inDegree))
	for node, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, node)
		}
	}

	result := make([]string, 0, len(inDegree))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)
		for _, dependent := range dependents[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(result) != len(inDegree) {
		return result, fmt.Errorf(
			"cycle detected: %d of %d nodes ordered",
			len(result), len(inDegree),
		)
	}

	return result, nil
}
