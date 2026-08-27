# Tasks: tool-interception

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 220–300 authored (+ ~130 tests) ≈ 350 total |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Interceptor + consent/v3 + CanStopSession | PR 1 | `go test ./internal/policy -run TestPolicyInterceptor -count=1` + `go test ./internal/review -run TestCanStopSession -count=1` | `go test ./... -count=1 -timeout 180s`; `go vet ./...` via fake Runner | Revert `interceptor.go` + `finalize.go` guard + Runner hook; no FSM/RDD |

## Phase 1: Foundation & Contracts

- [x] 1.1 Spike `ExtensionRunner.On("tool_call")` in `internal/assets/pi/*` + `plugin.go` — record signature
- [x] 1.2 Define `CanStopSession` state shape & closure invariants — resolve open question
- [x] 1.3 Create `internal/policy/interceptor.go` types `ToolCallRequest`/`Decision`/`Result` + `ToolCallInterceptor` — no `model/fsm`, <80 lines

## Phase 2: RED Tests (Before GREEN)

- [x] 2.1 RED `interceptor_test.go` — injected `user_bash rm -rf` denied → `BeforeToolCall` returns `block`; `t.Setenv` table
- [x] 2.2 RED `interceptor_test.go` — tampered args `revise` returns `RevisedArgs`, exec uses revised, original recorded
- [x] 2.3 RED `interceptor_test.go` — `AfterToolCall` with `Err` does not retry/re-exec or mutate FSM
- [x] 2.4 RED `finalize_test.go` — `CanStopSession` blocked returns `false` twice, idempotent, no mutation
- [x] 2.5 RED `interceptor_test.go` — `ask` emits `ToolApprovalRequested` consent/v3; `deny` blocks, `allow` resumes + `tool_execution_start`

## Phase 3: Core Implementation (GREEN)

- [x] 3.1 Implement `PolicyInterceptor.BeforeToolCall` in `interceptor.go` — default `allow`; deny/ask only blocks; `ask` → consent `ToolApprovalRequested` → await `resolved`
- [x] 3.2 Implement `AfterToolCall` observe-only — record output/error, no block/revise/re-exec, no FSM import
- [x] 3.3 Add `CanStopSession(state) bool` pure/idempotent in `finalize.go` before `Finalize` — no FSM states
- [x] 3.4 Wire `Runner.On("tool_call", BeforeToolCall)` in `assets/pi/*` + `plugin.go` — emit `tool_execution_*`/`tool_approval_*`; keep `registerFileWriteFallback`

## Phase 4: Integration & Verification

- [x] 4.1 Integration fake `ExtensionAPI` — `tool_call` → `Before` → `tool_execution_start`/`ApprovalRequested` → `resolved` → exec → `After`; fallback intact
- [x] 4.2 Verify `model/fsm.go` unchanged — 13 states + table identical; `allow` still fails gate if policy/budget guards fail
- [x] 4.3 Verify coupling/size — `grep ToolSession` zero, no struct >20 fields, no `model/fsm` import, wraps only `PolicyEvaluator`+`ApprovalMode`
- [x] 4.4 Gates — `go test ./... -count=1 -timeout 180s` PASS + `go vet ./...` PASS; `t.Setenv`/`t.TempDir` isolation

## Phase 5: Polish & Size Budget

- [x] 5.1 Count diff `git diff --numstat` <250 authored, <800 total; update `internal/install/*` atomically if layout changed
- [x] 5.2 Sweep docs/comments — English only, no FSM edits, no God object; `go vet` clean
