package extension

// Deprecated: use ExtensionAPI
type Shim struct {
	API ExtensionAPI
}

// Deprecated: use ExtensionAPI
type AgentAdapterShim struct {
	API ExtensionAPI
}

// NewShim creates a deprecated shim that forwards to ExtensionAPI.
// Deprecated: use ExtensionAPI
func NewShim(api ExtensionAPI) *Shim {
	return &Shim{API: api}
}

// Deprecated: use ExtensionAPI
func (s *Shim) RegisterTool(def ToolDef, h ToolHandler) {
	if s == nil || s.API == nil {
		return
	}
	s.API.RegisterTool(def, h)
}

// Deprecated: use ExtensionAPI
func (s *Shim) RegisterCommand(name string, h CommandHandler) {
	if s == nil || s.API == nil {
		return
	}
	s.API.RegisterCommand(name, h)
}

// Deprecated: use ExtensionAPI
func (s *AgentAdapterShim) HookToTool(hookName string, handler ToolHandler) {
	if s == nil || s.API == nil {
		return
	}
	def := ToolDef{Name: hookName, Description: "shim hook " + hookName}
	s.API.RegisterTool(def, handler)
}

// Deprecated: use ExtensionAPI
func (s *AgentAdapterShim) RegisterTool(def ToolDef, h ToolHandler) {
	if s == nil || s.API == nil {
		return
	}
	s.API.RegisterTool(def, h)
}

// Ensure no LensPlugin is reintroduced in this package.
