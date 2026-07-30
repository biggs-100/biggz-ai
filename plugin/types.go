package plugin

// CatalogEntry describes a discoverable item (agent, component, or skill)
// in the system catalog. It is the shared return type for ListAll methods
// on both the agent Registry and the build-time registry.Registry.
type CatalogEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tier        string `json:"tier"`
	Type        string `json:"type"` // "agent" | "component" | "skill"
}
