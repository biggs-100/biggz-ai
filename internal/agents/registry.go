// Package agents provides a factory-based registry for creating and
// discovering AgentAdapter implementations. It is separate from the
// build-time registry.Registry, which holds pre-wired singleton instances
// for the pipeline.
package agents

import (
	"fmt"

	"github.com/biggs-100/biggz-ai/internal/agents/capabilitymanifest"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// Factory is a function type that creates a new AgentAdapter on demand.
type Factory func() plugin.AgentAdapter

// Registry holds a map of agent ID → Factory for lazy adapter creation.
// Unlike registry.Registry (pre-wired singletons), this supports
// install-time discovery and CLI adapter creation.
type Registry struct {
	adapters map[model.AgentID]Factory
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{adapters: make(map[model.AgentID]Factory)}
}

// Register adds a factory under the given agent ID. It validates that the
// agent ID is known in the canonical capability manifest before registering.
// Duplicate IDs overwrite the previous entry (last registration wins).
func (r *Registry) Register(id model.AgentID, factory Factory) error {
	if _, ok := capabilitymanifest.ForAgent(id); !ok {
		return fmt.Errorf("unknown agent ID: %s", id)
	}
	r.adapters[id] = factory
	return nil
}

// Get returns the adapter for the given ID by invoking its factory.
// Returns nil, false if no factory is registered for that ID.
func (r *Registry) Get(id model.AgentID) (plugin.AgentAdapter, bool) {
	f, ok := r.adapters[id]
	if !ok {
		return nil, false
	}
	return f(), true
}

// ListAll returns CatalogEntry values built from each registered adapter.
func (r *Registry) ListAll() []plugin.CatalogEntry {
	entries := make([]plugin.CatalogEntry, 0, len(r.adapters))
	for id, f := range r.adapters {
		a := f()
		entries = append(entries, plugin.CatalogEntry{
			ID:   string(id),
			Name: a.Name(),
			Type: "agent",
		})
	}
	return entries
}

// NewDefaultRegistry creates a Registry pre-wired with all adapters that
// registered themselves via the package-level Register function.
// Adapter packages must be imported (directly or transitively) for their
// init() functions to run before calling this.
// Returns an error if any registered adapter has an unknown agent ID.
func NewDefaultRegistry() (*Registry, error) {
	r := NewRegistry()
	for id, fn := range globalFactories {
		if err := r.Register(id, fn); err != nil {
			return nil, fmt.Errorf("register %s: %w", id, err)
		}
	}
	return r, nil
}
