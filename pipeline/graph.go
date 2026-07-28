package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/biggz-ai/biggz/model"
)

// Node is a single unit in a DAG execution graph.
// Each node wraps a Stage and may depend on other nodes.
type Node struct {
	Stage      Stage
	DependsOn  []string // names of stages this node depends on
}

// Graph executes stages as a DAG, running independent stages in parallel.
// This implements SGH's multi-ready-unit scheduling: at any point, all nodes
// whose dependencies are satisfied are dispatched concurrently.
//
// The orchestrator uses this to delegate independent tasks in parallel
// (e.g., running all 4 lenses simultaneously instead of sequentially).
type Graph struct {
	nodes map[string]*Node
}

// NewGraph creates an empty execution graph.
func NewGraph() *Graph {
	return &Graph{nodes: make(map[string]*Node)}
}

// AddNode registers a stage with optional dependencies.
// Dependencies are referenced by stage name.
func (g *Graph) AddNode(stage Stage, dependsOn ...string) {
	name := stage.Name()
	g.nodes[name] = &Node{
		Stage:     stage,
		DependsOn: dependsOn,
	}
}

// Execute runs all nodes in dependency order. Nodes with all dependencies
// satisfied run concurrently via goroutines. If any node fails, all running
// nodes are cancelled, and completed nodes are rolled back in reverse order.
func (g *Graph) Execute(ctx context.Context, state *model.ReviewState) error {
	// Build dependency tracking
	completed := make(map[string]bool)
	completedMu := sync.Mutex{}
	var completedOrder []string

	// Track failed nodes
	var failedErr error
	failedMu := sync.Mutex{}
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Count total nodes
	totalNodes := len(g.nodes)

	for len(completed) < totalNodes {
		if failedErr != nil {
			break
		}

		// Compute ready set: nodes where all deps are completed
		ready := g.readyNodes(completed)
		if len(ready) == 0 && len(completed) < totalNodes {
			// Detect cycles: ready set empty but not all completed
			remaining := totalNodes - len(completed)
			return fmt.Errorf("graph cycle detected: %d nodes remaining but none ready", remaining)
		}

		// Dispatch ready nodes in parallel
		var wg sync.WaitGroup
		type nodeResult struct {
			name string
			err  error
		}
		results := make(chan nodeResult, len(ready))

		for _, name := range ready {
			wg.Add(1)
			go func(nodeName string) {
				defer wg.Done()
				node := g.nodes[nodeName]

				select {
				case <-cancelCtx.Done():
					results <- nodeResult{name: nodeName, err: ctx.Err()}
					return
				default:
				}

				err := node.Stage.Execute(cancelCtx, state)

				completedMu.Lock()
				completedOrder = append(completedOrder, nodeName)
				completedMu.Unlock()

				results <- nodeResult{name: nodeName, err: err}
			}(name)
		}

		wg.Wait()
		close(results)

		// Process results
		for r := range results {
			if r.err != nil {
				failedMu.Lock()
				if failedErr == nil {
					failedErr = fmt.Errorf("node %s failed: %w", r.name, r.err)
					cancel() // cancel remaining nodes
				}
				failedMu.Unlock()

				// Rollback the failed node
				g.nodes[r.name].Stage.Rollback(ctx, state)
			} else {
				completedMu.Lock()
				completed[r.name] = true
				completedMu.Unlock()
			}
		}
	}

	if failedErr != nil {
		// Rollback all completed nodes in reverse order
		completedMu.Lock()
		rollbackOrder := make([]string, len(completedOrder))
		copy(rollbackOrder, completedOrder)
		completedMu.Unlock()

		for i := len(rollbackOrder) - 1; i >= 0; i-- {
			g.nodes[rollbackOrder[i]].Stage.Rollback(ctx, state)
		}
		return failedErr
	}

	return nil
}

// readyNodes returns the names of nodes whose dependencies are all satisfied.
func (g *Graph) readyNodes(completed map[string]bool) []string {
	var ready []string
	for name, node := range g.nodes {
		if completed[name] {
			continue
		}
		allDepsMet := true
		for _, dep := range node.DependsOn {
			if !completed[dep] {
				allDepsMet = false
				break
			}
		}
		if allDepsMet {
			ready = append(ready, name)
		}
	}
	return ready
}

// Stages returns the underlying stages in registration order.
func (g *Graph) Stages() []Stage {
	var stages []Stage
	for _, node := range g.nodes {
		stages = append(stages, node.Stage)
	}
	return stages
}
