package agents

import (
	"fmt"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// globalFactories holds adapters registered via package-level Register.
// Adapter packages register themselves in init() functions.
var globalFactories = make(map[model.AgentID]Factory)

// Register registers a factory globally for the given agent ID.
// This is intended for use by adapter package init() functions.
func Register(id model.AgentID, factory Factory) {
	globalFactories[id] = factory
}

// NewAdapter creates an adapter by agent ID using the global registry.
// It returns an error if the agent ID is unknown.
func NewAdapter(agentID model.AgentID) (plugin.AgentAdapter, error) {
	f, ok := globalFactories[agentID]
	if !ok {
		return nil, fmt.Errorf("unknown agent: %s", agentID)
	}
	return f(), nil
}
