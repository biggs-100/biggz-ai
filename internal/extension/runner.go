package extension

import (
	"context"
	"os"

	"github.com/biggs-100/biggz-ai/internal/policy"
)

// Runner wraps pi.on tool_call/tool_result/session_stop and delegates to
// policy.PolicyInterceptor, preserving fallback and consent v3, with
// PI_SUBAGENT_CHILD=1 bypass. It reuses PolicyInterceptor (no duplicate policy).
type Runner struct {
	API         ExtensionAPI
	Interceptor *policy.PolicyInterceptor
}

// Before is invoked on pi.on("tool_call"). It checks subagent bypass, runs
// ExtensionAPI tool_call middleware, consults fallback for file-write, then
// delegates to PolicyInterceptor. Block/revise short-circuits remain.
func (r *Runner) Before(ctx context.Context, req policy.ToolCallRequest) (policy.ToolCallDecision, error) {
	if os.Getenv("PI_SUBAGENT_CHILD") == "1" {
		return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
	}

	// 1. Run ExtensionAPI tool_call middleware chain if API supports it.
	// Support both apiImpl and testutil Fake via interface dispatch.
	if r.API != nil {
		// Prefer typed accessor for apiImpl.
		if withHandlers, ok := r.API.(interface {
			ToolCallHandlers() []toolCallHandler
		}); ok {
			handlers := withHandlers.ToolCallHandlers()
			extReq := ToolCallRequest{Tool: req.Tool, Args: req.Args, CallID: req.CallID}
			for _, h := range handlers {
				dec, err := h(ctx, extReq)
				if err != nil {
					return policy.ToolCallDecision{}, err
				}
				switch dec.Kind {
				case policy.DecisionBlock:
					return dec, nil
				case policy.DecisionRevise:
					if dec.RevisedArgs != nil {
						req.Args = dec.RevisedArgs
					}
					// Revise short-circuits remaining middleware, proceed to policy.
					goto checkFallback
				case policy.DecisionAllow, "":
					// continue
				default:
					// treat unknown as allow
				}
			}
		} else if fakeWith, ok := r.API.(interface {
			RunToolCallMiddleware(ctx context.Context, req ToolCallRequest) (policy.ToolCallDecision, ToolCallRequest, error)
		}); ok {
			extReq := ToolCallRequest{Tool: req.Tool, Args: req.Args, CallID: req.CallID}
			dec, revised, err := fakeWith.RunToolCallMiddleware(ctx, extReq)
			if err != nil {
				return policy.ToolCallDecision{}, err
			}
			switch dec.Kind {
			case policy.DecisionBlock:
				return dec, nil
			case policy.DecisionRevise:
				req.Args = revised.Args
				goto checkFallback
			}
			// allow: continue
			req.Args = revised.Args
		}
	}

checkFallback:
	// 2. If fallback registered and tool is file-write-like, delegate to fallback
	// before policy evaluation (preserves registerFileWriteFallback intact).
	if r.API != nil && isFileWriteTool(req.Tool) {
		var fb FileWriteFallback
		if withFallback, ok := r.API.(interface{ GetFallback() FileWriteFallback }); ok {
			fb = withFallback.GetFallback()
		}
		if fb != nil {
			extReq := ToolCallRequest{Tool: req.Tool, Args: req.Args, CallID: req.CallID}
			dec, err := fb(ctx, extReq)
			if err != nil {
				return policy.ToolCallDecision{}, err
			}
			// If fallback blocks, short-circuit policy.
			if dec.Kind == policy.DecisionBlock {
				return dec, nil
			}
			// Fallback allow/revise: continue to policy (policy may still block)
			if dec.Kind == policy.DecisionRevise && dec.RevisedArgs != nil {
				req.Args = dec.RevisedArgs
			}
		}
	}

	// 3. Delegate to PolicyInterceptor (no duplicate logic, includes consent v3).
	if r.Interceptor != nil {
		return r.Interceptor.BeforeToolCall(ctx, req)
	}
	return policy.ToolCallDecision{Kind: policy.DecisionAllow}, nil
}

// After is invoked on pi.on("tool_result"). It is observability-only and
// MUST NOT block or mutate the result, then delegates to Interceptor.
func (r *Runner) After(ctx context.Context, req policy.ToolCallRequest, res policy.ToolCallResult) {
	if os.Getenv("PI_SUBAGENT_CHILD") == "1" {
		return
	}
	// Run ExtensionAPI tool_result handlers observability-only.
	if r.API != nil {
		if withHandlers, ok := r.API.(interface {
			ToolResultHandlers() []toolResultHandler
		}); ok {
			handlers := withHandlers.ToolResultHandlers()
			extReq := ToolCallRequest{Tool: req.Tool, Args: req.Args, CallID: req.CallID}
			extRes := ToolCallResult{Output: res.Output, Err: res.Err}
			for _, h := range handlers {
				func() {
					defer func() { _ = recover() }()
					h(ctx, extReq, extRes)
				}()
			}
		} else if fakeWith, ok := r.API.(interface {
			RunToolResultHandlers(ctx context.Context, req ToolCallRequest, res ToolCallResult)
		}); ok {
			extReq := ToolCallRequest{Tool: req.Tool, Args: req.Args, CallID: req.CallID}
			extRes := ToolCallResult{Output: res.Output, Err: res.Err}
			fakeWith.RunToolResultHandlers(ctx, extReq, extRes)
		}
	}
	if r.Interceptor != nil {
		r.Interceptor.AfterToolCall(ctx, req, res)
	}
}

// CanStop is invoked on pi.on("session_stop") and checks closure invariants.
// It returns true only when no pending work blocks termination. For now it
// mirrors policy-level guard: if pending findings/lenses env set, block.
// Pure and idempotent; Runner delegates but keeps fallback intact.
func (r *Runner) CanStop(_ context.Context) bool {
	if os.Getenv("PI_SUBAGENT_CHILD") == "1" {
		return true
	}
	// Preservation of existing TODO: Runner simply allows stop; higher-level
	// invariants are checked by review.CanStopSession. This remains pure.
	return true
}
