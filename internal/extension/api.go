package extension

import (
	"context"
	"fmt"
	"sync"

	"github.com/biggs-100/biggz-ai/internal/policy"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
)

// ToolCallRequest is the unified tool invocation request (oh-my-pi parity).
type ToolCallRequest struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args"`
	CallID string         `json:"call_id"`
}

// ToolCallResult is the unified tool invocation result.
type ToolCallResult struct {
	Output string `json:"output"`
	Err    error  `json:"-"`
}

// ToolDef describes a tool registration.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
}

// ToolHandler is the handler for a registered tool.
type ToolHandler func(context.Context, ToolCallRequest) (ToolCallResult, error)

// CommandHandler is the handler for a registered command.
type CommandHandler func(context.Context, map[string]any) error

// FileWriteFallback is the handler for file-write fallback interception.
type FileWriteFallback func(context.Context, ToolCallRequest) (policy.ToolCallDecision, error)

// ExtensionAPI is the sole registration surface for lenses, commands, tools,
// middleware, fallback and invocation. It mirrors oh-my-pi ExtensionAPI parity.
type ExtensionAPI interface {
	On(event string, h any)
	RegisterLens(l lens.Lens)
	RegisterCommand(name string, h CommandHandler)
	RegisterTool(def ToolDef, h ToolHandler)
	RegisterFileWriteFallback(h FileWriteFallback)
	InvokeTool(ctx context.Context, req ToolCallRequest) (ToolCallResult, error)
	Ordered(ids []string) []lens.Lens
}

// toolCallHandler is internal middleware for tool_call.
type toolCallHandler func(context.Context, ToolCallRequest) (policy.ToolCallDecision, error)

// toolResultHandler is internal observer for tool_result.
type toolResultHandler func(context.Context, ToolCallRequest, ToolCallResult)

type toolEntry struct {
	def     ToolDef
	handler ToolHandler
}

type apiImpl struct {
	mu                 sync.RWMutex
	lensMap            map[string]lens.Lens
	commands           map[string]CommandHandler
	tools              map[string]toolEntry
	fallback           FileWriteFallback
	toolCallHandlers   []toolCallHandler
	toolResultHandlers []toolResultHandler
}

// New creates a new ExtensionAPI instance.
func New() ExtensionAPI {
	return &apiImpl{
		lensMap:  make(map[string]lens.Lens),
		commands: make(map[string]CommandHandler),
		tools:    make(map[string]toolEntry),
	}
}

// On registers middleware for events: tool_call, tool_result, session_stop.
// Handlers are appended in registration order. For tool_call the handler
// must be func(context.Context, ToolCallRequest) (policy.ToolCallDecision, error).
// For tool_result it must be func(context.Context, ToolCallRequest, ToolCallResult).
func (a *apiImpl) On(event string, h any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch event {
	case "tool_call":
		if fn, ok := h.(func(context.Context, ToolCallRequest) (policy.ToolCallDecision, error)); ok {
			a.toolCallHandlers = append(a.toolCallHandlers, toolCallHandler(fn))
			return
		}
		if fn, ok := h.(toolCallHandler); ok {
			a.toolCallHandlers = append(a.toolCallHandlers, fn)
			return
		}
		// Also accept policy request variant for Runner convenience.
		if fn, ok := h.(func(context.Context, policy.ToolCallRequest) (policy.ToolCallDecision, error)); ok {
			wrapped := func(ctx context.Context, req ToolCallRequest) (policy.ToolCallDecision, error) {
				return fn(ctx, policy.ToolCallRequest{Tool: req.Tool, Args: req.Args, CallID: req.CallID})
			}
			a.toolCallHandlers = append(a.toolCallHandlers, wrapped)
			return
		}
		// Silent ignore unknown handler types (no panic, per guard).
	case "tool_result":
		if fn, ok := h.(func(context.Context, ToolCallRequest, ToolCallResult)); ok {
			a.toolResultHandlers = append(a.toolResultHandlers, toolResultHandler(fn))
			return
		}
		if fn, ok := h.(toolResultHandler); ok {
			a.toolResultHandlers = append(a.toolResultHandlers, fn)
			return
		}
		if fn, ok := h.(func(context.Context, policy.ToolCallRequest, policy.ToolCallResult)); ok {
			wrapped := func(ctx context.Context, req ToolCallRequest, res ToolCallResult) {
				fn(ctx, policy.ToolCallRequest{Tool: req.Tool, Args: req.Args, CallID: req.CallID}, policy.ToolCallResult{Output: res.Output, Err: res.Err})
			}
			a.toolResultHandlers = append(a.toolResultHandlers, wrapped)
			return
		}
	case "session_stop":
		// session_stop handlers are not stored for middleware chain; they are
		// handled by Runner.CanStop path. Accept but no-op.
	default:
		// Unknown event: ignore, no panic.
	}
}

// RegisterLens registers a lens via ExtensionAPI and also populates the
// global lens registry so Ordered via global still works. Duplicate IDs
// last-win.
func (a *apiImpl) RegisterLens(l lens.Lens) {
	if l == nil {
		return
	}
	a.mu.Lock()
	a.lensMap[l.ID()] = l
	a.mu.Unlock()
	lens.RegisterLens(l)
}

// RegisterCommand registers a command handler.
func (a *apiImpl) RegisterCommand(name string, h CommandHandler) {
	if name == "" || h == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.commands[name] = h
}

// RegisterTool registers a tool with its handler.
func (a *apiImpl) RegisterTool(def ToolDef, h ToolHandler) {
	if def.Name == "" || h == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tools[def.Name] = toolEntry{def: def, handler: h}
}

// RegisterFileWriteFallback registers the file-write fallback handler.
func (a *apiImpl) RegisterFileWriteFallback(h FileWriteFallback) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fallback = h
}

// InvokeTool executes the tool_call middleware chain in registration order.
// Block/revise short-circuits remaining handlers and prevents tool execution
// (block) or revises args (revise). After tool execution, tool_result
// handlers run observability-only and MUST NOT mutate the result.
func (a *apiImpl) InvokeTool(ctx context.Context, req ToolCallRequest) (ToolCallResult, error) {
	a.mu.RLock()
	callHandlers := append([]toolCallHandler(nil), a.toolCallHandlers...)
	resultHandlers := append([]toolResultHandler(nil), a.toolResultHandlers...)
	entry, ok := a.tools[req.Tool]
	hasTool := ok
	handler := entry.handler
	// Copy fallback for potential file_write handling: if fallback exists and
	// tool is file_write-like, middleware chain already runs, but fallback
	// is consulted via Runner, not here. Still, if InvokeTool targets
	// file_write and fallback exists without a tool handler, delegate.
	fb := a.fallback
	a.mu.RUnlock()

	// Run tool_call middleware chain.
	revisedReq := req
	for _, h := range callHandlers {
		dec, err := h(ctx, revisedReq)
		if err != nil {
			return ToolCallResult{}, err
		}
		switch dec.Kind {
		case policy.DecisionBlock:
			return ToolCallResult{Output: dec.Reason, Err: fmt.Errorf("blocked: %s", dec.Reason)}, nil
		case policy.DecisionRevise:
			if dec.RevisedArgs != nil {
				revisedReq.Args = dec.RevisedArgs
			}
			// Revise short-circuits remaining middleware (per spec).
			goto execTool
		case policy.DecisionAsk:
			// Ask is treated as block unless caller handles consent; mirror
			// PolicyInterceptor ask->block behavior when not resolved.
			return ToolCallResult{Output: dec.Reason, Err: fmt.Errorf("ask: %s", dec.Reason)}, nil
		case policy.DecisionAllow, "":
			// Continue to next handler.
		default:
			// Unknown kind treated as allow.
		}
	}
execTool:
	_ = fb // fallback preserved for Runner; InvokeTool does not auto-invoke fallback here unless no tool handler.
	if !hasTool {
		// If fallback exists and tool is file_write-like, delegate to fallback.
		if fb != nil && isFileWriteTool(req.Tool) {
			dec, err := fb(ctx, revisedReq)
			if err != nil {
				return ToolCallResult{}, err
			}
			if dec.Kind == policy.DecisionBlock {
				return ToolCallResult{Output: dec.Reason, Err: fmt.Errorf("blocked by fallback: %s", dec.Reason)}, nil
			}
			return ToolCallResult{Output: "fallback allow"}, nil
		}
		return ToolCallResult{}, fmt.Errorf("tool not found: %s", req.Tool)
	}

	res, err := handler(ctx, revisedReq)
	// Run tool_result handlers observability-only: preserve original result.
	// Handlers receive copies and cannot mutate preserved res.
	for _, rh := range resultHandlers {
		// Defensive recover: handlers must not panic caller.
		func() {
			defer func() { _ = recover() }()
			rh(ctx, revisedReq, res)
		}()
	}
	return res, err
}

// Ordered returns lenses for the given IDs in exact input order, skipping
// unknown IDs. It delegates to the global lens registry populated via
// RegisterLens, but also checks local map for determinism in tests where
// global registry may have been reset.
func (a *apiImpl) Ordered(ids []string) []lens.Lens {
	// Prefer global registry (covers both API-registered and legacy lenses)
	// but ensure local lenses are visible even if global was reset after
	// registration (test isolation). Merge: global + local, local wins.
	a.mu.RLock()
	localCopy := make(map[string]lens.Lens, len(a.lensMap))
	for k, v := range a.lensMap {
		localCopy[k] = v
	}
	a.mu.RUnlock()

	globalOrdered := lens.Ordered(ids)
	// If global already contains all requested ids that are in local, return global
	// (preserves last-win global semantics). Otherwise merge.
	if len(globalOrdered) == len(ids) || len(localCopy) == 0 {
		// Check if any id missing from global but present locally.
		missing := false
		globalSet := make(map[string]bool, len(globalOrdered))
		for _, l := range globalOrdered {
			globalSet[l.ID()] = true
		}
		for _, id := range ids {
			if !globalSet[id] {
				if _, ok := localCopy[id]; ok {
					missing = true
					break
				}
			}
		}
		if !missing {
			return globalOrdered
		}
	}
	// Build from local + global fallback.
	out := make([]lens.Lens, 0, len(ids))
	for _, id := range ids {
		if l, ok := localCopy[id]; ok {
			out = append(out, l)
			continue
		}
		// Fallback to global for not-local entries (use global Ordered for that id)
		if gl := lens.Ordered([]string{id}); len(gl) > 0 {
			out = append(out, gl[0])
		}
	}
	return out
}

// isFileWriteTool reports whether the tool name is a file-write variant.
func isFileWriteTool(tool string) bool {
	switch tool {
	case "file_write", "write", "edit", "apply_patch":
		return true
	default:
		return false
	}
}

// GetFallback exposes the registered fallback for Runner integration.
// Not part of the public ExtensionAPI spec but used via type assertion.
func (a *apiImpl) GetFallback() FileWriteFallback {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.fallback
}

// ToolCallHandlers returns a copy of tool_call handlers for Runner.
func (a *apiImpl) ToolCallHandlers() []toolCallHandler {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]toolCallHandler(nil), a.toolCallHandlers...)
}

// ToolResultHandlers returns a copy of tool_result handlers for Runner.
func (a *apiImpl) ToolResultHandlers() []toolResultHandler {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]toolResultHandler(nil), a.toolResultHandlers...)
}
