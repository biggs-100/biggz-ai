package testutil

import (
	"context"
	"fmt"
	"sync"

	"github.com/biggs-100/biggz-ai/internal/extension"
	"github.com/biggs-100/biggz-ai/internal/policy"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
)

// FakeExtensionAPI is an in-memory ExtensionAPI for tests. It records
// lenses/commands/tools/fallback/On handlers and allows InvokeTool to trigger
// the handler. It supports t.Setenv isolation (no global state).
type FakeExtensionAPI struct {
	mu sync.RWMutex

	Lenses   []lens.Lens
	LensMap  map[string]lens.Lens
	Commands map[string]extension.CommandHandler
	Tools    map[string]extension.ToolHandler
	ToolDefs map[string]extension.ToolDef
	Fallback extension.FileWriteFallback

	ToolCallHandlers   []func(context.Context, extension.ToolCallRequest) (policy.ToolCallDecision, error)
	ToolResultHandlers []func(context.Context, extension.ToolCallRequest, extension.ToolCallResult)

	Invoked []extension.ToolCallRequest

	// OnCalls records raw On invocations for verification.
	OnCalls []struct {
		Event string
		H     any
	}
}

// NewFake creates a new FakeExtensionAPI.
func NewFake() *FakeExtensionAPI {
	return &FakeExtensionAPI{
		LensMap:  make(map[string]lens.Lens),
		Commands: make(map[string]extension.CommandHandler),
		Tools:    make(map[string]extension.ToolHandler),
		ToolDefs: make(map[string]extension.ToolDef),
	}
}

func (f *FakeExtensionAPI) On(event string, h any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.OnCalls = append(f.OnCalls, struct {
		Event string
		H     any
	}{Event: event, H: h})
	switch event {
	case "tool_call":
		if fn, ok := h.(func(context.Context, extension.ToolCallRequest) (policy.ToolCallDecision, error)); ok {
			f.ToolCallHandlers = append(f.ToolCallHandlers, fn)
		}
	case "tool_result":
		if fn, ok := h.(func(context.Context, extension.ToolCallRequest, extension.ToolCallResult)); ok {
			f.ToolResultHandlers = append(f.ToolResultHandlers, fn)
		}
	case "session_stop":
		// no-op for fake
	}
}

func (f *FakeExtensionAPI) RegisterLens(l lens.Lens) {
	if l == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.LensMap == nil {
		f.LensMap = make(map[string]lens.Lens)
	}
	f.LensMap[l.ID()] = l
	f.Lenses = append(f.Lenses, l)
}

func (f *FakeExtensionAPI) RegisterCommand(name string, h extension.CommandHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Commands == nil {
		f.Commands = make(map[string]extension.CommandHandler)
	}
	f.Commands[name] = h
}

func (f *FakeExtensionAPI) RegisterTool(def extension.ToolDef, h extension.ToolHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Tools == nil {
		f.Tools = make(map[string]extension.ToolHandler)
		f.ToolDefs = make(map[string]extension.ToolDef)
	}
	f.Tools[def.Name] = h
	f.ToolDefs[def.Name] = def
}

func (f *FakeExtensionAPI) RegisterFileWriteFallback(h extension.FileWriteFallback) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Fallback = h
}

func (f *FakeExtensionAPI) GetFallback() extension.FileWriteFallback {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.Fallback
}

func (f *FakeExtensionAPI) ToolCallHandlersCopy() []func(context.Context, extension.ToolCallRequest) (policy.ToolCallDecision, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]func(context.Context, extension.ToolCallRequest) (policy.ToolCallDecision, error)(nil), f.ToolCallHandlers...)
}

// RunToolCallMiddleware runs the tool_call chain for Runner integration.
func (f *FakeExtensionAPI) RunToolCallMiddleware(ctx context.Context, req extension.ToolCallRequest) (policy.ToolCallDecision, extension.ToolCallRequest, error) {
	f.mu.RLock()
	handlers := append([]func(context.Context, extension.ToolCallRequest) (policy.ToolCallDecision, error){}, f.ToolCallHandlers...)
	f.mu.RUnlock()
	revised := req
	for _, h := range handlers {
		dec, err := h(ctx, revised)
		if err != nil {
			return policy.ToolCallDecision{}, revised, err
		}
		switch dec.Kind {
		case policy.DecisionBlock:
			return dec, revised, nil
		case policy.DecisionRevise:
			if dec.RevisedArgs != nil {
				revised.Args = dec.RevisedArgs
			}
			return dec, revised, nil
		}
	}
	return policy.ToolCallDecision{Kind: policy.DecisionAllow}, revised, nil
}

// RunToolResultHandlers runs tool_result handlers observability-only.
func (f *FakeExtensionAPI) RunToolResultHandlers(ctx context.Context, req extension.ToolCallRequest, res extension.ToolCallResult) {
	f.mu.RLock()
	handlers := append([]func(context.Context, extension.ToolCallRequest, extension.ToolCallResult){}, f.ToolResultHandlers...)
	f.mu.RUnlock()
	for _, h := range handlers {
		func() {
			defer func() { _ = recover() }()
			h(ctx, req, res)
		}()
	}
}

func (f *FakeExtensionAPI) ToolCallHandlersCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.ToolCallHandlers)
}

func (f *FakeExtensionAPI) ToolResultHandlersCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.ToolResultHandlers)
}

func (f *FakeExtensionAPI) InvokeTool(ctx context.Context, req extension.ToolCallRequest) (extension.ToolCallResult, error) {
	f.mu.Lock()
	f.Invoked = append(f.Invoked, req)
	handlers := append([]func(context.Context, extension.ToolCallRequest) (policy.ToolCallDecision, error){}, f.ToolCallHandlers...)
	resultHandlers := append([]func(context.Context, extension.ToolCallRequest, extension.ToolCallResult){}, f.ToolResultHandlers...)
	toolHandler, ok := f.Tools[req.Tool]
	fallback := f.Fallback
	f.mu.Unlock()

	revised := req
	for _, h := range handlers {
		dec, err := h(ctx, revised)
		if err != nil {
			return extension.ToolCallResult{}, err
		}
		if dec.Kind == policy.DecisionBlock {
			return extension.ToolCallResult{Output: dec.Reason, Err: fmt.Errorf("blocked: %s", dec.Reason)}, nil
		}
		if dec.Kind == policy.DecisionRevise {
			if dec.RevisedArgs != nil {
				revised.Args = dec.RevisedArgs
			}
			break
		}
	}

	if !ok {
		if fallback != nil && isFileWriteTool(req.Tool) {
			dec, err := fallback(ctx, revised)
			if err != nil {
				return extension.ToolCallResult{}, err
			}
			if dec.Kind == policy.DecisionBlock {
				return extension.ToolCallResult{Output: dec.Reason, Err: fmt.Errorf("blocked by fallback: %s", dec.Reason)}, nil
			}
			return extension.ToolCallResult{Output: "fallback allow"}, nil
		}
		return extension.ToolCallResult{}, fmt.Errorf("tool not found: %s", req.Tool)
	}

	res, err := toolHandler(ctx, revised)
	for _, rh := range resultHandlers {
		func() {
			defer func() { _ = recover() }()
			rh(ctx, revised, res)
		}()
	}
	return res, err
}

func (f *FakeExtensionAPI) Ordered(ids []string) []lens.Lens {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]lens.Lens, 0, len(ids))
	for _, id := range ids {
		if l, ok := f.LensMap[id]; ok {
			out = append(out, l)
		}
	}
	return out
}

func isFileWriteTool(tool string) bool {
	switch tool {
	case "file_write", "write", "edit", "apply_patch":
		return true
	default:
		return false
	}
}

// Ensure FakeExtensionAPI implements extension.ExtensionAPI.
var _ extension.ExtensionAPI = (*FakeExtensionAPI)(nil)
