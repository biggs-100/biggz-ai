// Package planner provides dependency graph resolution for component deployment
// ordering. It implements Kahn's algorithm for topological sorting and supports
// automatic dependency resolution from a Selection to a complete ordered Plan.
package planner

import "fmt"

// Selection describes a user's install choices — which components and skills
// to deploy for a given agent.
type Selection struct {
	AgentID    string
	Components []string
	Skills     []string
}

// Plan describes the ordered set of components to install, including
// automatically resolved transitive dependencies.
type Plan struct {
	Components   []string // topological order of all components to deploy
	Dependencies []string // components auto-added (not in the original selection)
}

// ComponentNode represents a single node in the dependency graph with its
// metadata. Used to build the graph from catalog data.
type ComponentNode struct {
	ID           string
	Dependencies []string
}

// BuildReviewPayload returns a human-readable summary of what the plan
// contains: the deployment order and which dependencies were auto-resolved.
func BuildReviewPayload(plan *Plan) string {
	out := "Deployment Plan\n"
	out += "━━━━━━━━━━━━━━\n"
	out += fmt.Sprintf("Components to deploy (%d):\n", len(plan.Components))
	for i, c := range plan.Components {
		mark := " "
		for _, d := range plan.Dependencies {
			if d == c {
				mark = "*"
				break
			}
		}
		out += fmt.Sprintf("  %2d. %s %s\n", i+1, mark, c)
	}
	if len(plan.Dependencies) > 0 {
		out += "\n* Auto-resolved dependency (not in original selection)\n"
	}
	return out
}
