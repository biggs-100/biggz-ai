// Package registry provides a build-time, thread-safe Registry for
// LensPlugin and AgentAdapter instances.
//
// Registration happens at program startup via explicit wiring — there
// is no dynamic loading. The Registry enforces that each plugin ID is
// unique; duplicate registration returns an error.
package registry

import (
	"fmt"
	"sync"

	"github.com/biggs-100/biggz-ai/plugin"
)

// Registry holds the set of registered lens plugins and agent adapters.
// It is safe for concurrent use via a read-write mutex.
type Registry struct {
	mu     sync.RWMutex
	lenses map[string]plugin.LensPlugin
	agents map[string]plugin.AgentAdapter
}

// New creates an empty Registry ready for registration.
func New() *Registry {
	return &Registry{
		lenses: make(map[string]plugin.LensPlugin),
		agents: make(map[string]plugin.AgentAdapter),
	}
}

// RegisterLens adds a lens plugin to the registry. It returns an error
// if a lens with the same ID is already registered.
func (r *Registry) RegisterLens(p plugin.LensPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := p.ID()
	if _, exists := r.lenses[id]; exists {
		return fmt.Errorf("lens %q is already registered", id)
	}
	r.lenses[id] = p
	return nil
}

// RegisterAdapter adds an agent adapter to the registry. It returns an error
// if an agent with the same ID is already registered.
func (r *Registry) RegisterAdapter(a plugin.AgentAdapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := string(a.ID())
	if _, exists := r.agents[id]; exists {
		return fmt.Errorf("agent %q is already registered", id)
	}
	r.agents[id] = a
	return nil
}

// GetLens returns the lens plugin with the given ID, or nil if no lens
// with that ID is registered.
func (r *Registry) GetLens(id string) plugin.LensPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lenses[id]
}

// LensIDs returns the IDs of all registered lens plugins.
func (r *Registry) LensIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.lenses))
	for id := range r.lenses {
		ids = append(ids, id)
	}
	return ids
}

// GetAdapter returns the agent adapter with the given ID, or nil if
// no agent with that ID is registered.
func (r *Registry) GetAdapter(id string) plugin.AgentAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agents[id]
}

// ListAll returns CatalogEntry values built from each registered adapter.
func (r *Registry) ListAll() []plugin.CatalogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := make([]plugin.CatalogEntry, 0, len(r.agents))
	for _, a := range r.agents {
		entries = append(entries, plugin.CatalogEntry{
			ID:   string(a.ID()),
			Name: a.Name(),
			Type: "agent",
		})
	}
	return entries
}
