# Apply Progress: fix-route-nextrecommended-clause

## Phase 1 — Routing-inert guard (PR 1)

- [x] 1.1 `TestSubrouteRoutingInert` added to `internal/sdd/subroute_test.go`, reusing `isolatedSubrouteHome`/`seedSubrouteChange`/`deriveSubrouteChange`: two proposal-only organic seeds — declared (`subroute: direct-inline`) vs no-key — assert `NextRecommended` equality (D2.1), `Subroute == "direct-inline"` on the declared derivation (D2.2), and `Route == "organic"` on both (D2.3). Focused run PASS (evidence below).
- [x] 1.2 Comment on the test states: Standard mode, no RED step because no production fix exists (the behavior is already correct); the guard locks shipped behavior and fails only if routing ever starts reading the declared subroute (`resolveNextRecommended`/`deriveRoute` branching on `cs.Subroute`); the replaced clause ("`nextRecommended` MUST be empty") is unsatisfiable because an active proposal-only change always resolves to a non-empty token (planning next or `resolve-blockers` fallback) and change-less organic work has no `active` entry exposing `route`/`subroute` at all.
- [x] 1.3 `gofmt -l internal/sdd/subroute_test.go` → exit 0, empty (CI enforces `gofmt -l .` at `.github/workflows/ci.yml` "Check formatting" step and `scripts/gofmtcheck.sh`).

## Phase 2 — Keep-passing (PR 1)

- [x] 2.1 `go test ./internal/sdd/ -run 'TestSubroute|TestRoute|TestStatus' -count=1` → `ok github.com/biggs-100/biggz-ai/internal/sdd 2.086s`, exit 0.
- [x] 2.2 `go test ./internal/sdd/ -run TestEvaluateRoute -count=1` → `ok github.com/biggs-100/biggz-ai/internal/sdd 0.101s`, exit 0.
- [x] 2.3 `go test ./internal/sdd/ -count=1` raw → only the two `PI_SUBAGENT_CHILD` ownership tests fail (`TestCheckCheckpointAsk`, `TestValidateCheckpointSubstance`, message `checkpoint asks may only be emitted by orchestrator, not sub-agent (ownership)`); with `PI_SUBAGENT_CHILD=` cleared → `ok github.com/biggs-100/biggz-ai/internal/sdd 24.684s`, exit 0. `go build ./...` exit 0, no output. `go vet ./...` exit 0, no output.
- [x] 2.4 Harness recorded below: both variants derived in-process over `t.TempDir()` workspaces; N/A external command.

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/subroute_test.go` | Modified | +35 lines: `TestSubrouteRoutingInert` (two seeds, three assertions) with Standard-mode/no-RED comment. |
| `openspec/changes/fix-route-nextrecommended-clause/tasks.md` | Modified | Ticked 1.1–1.3, 2.1–2.4; phases 3–4 left unticked with their gate text intact. |
| `openspec/changes/fix-route-nextrecommended-clause/apply-progress.md` | Created | This evidence file. |

Unchanged: `deriveRoute`, `resolveNextRecommended`, `declaredOrganicSubroute`, every other `.go` file, `openspec/specs/**`, `openspec/changes/archive/**`, `AGENTS.md`, `.github/**`, other changes. No commit, no push, no attempt-ledger action.

## Work Unit Evidence (Unit 1 — `routing-inert-guard`)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/sdd/ -run TestSubrouteRoutingInert -count=1 -v` → `=== RUN TestSubrouteRoutingInert`, `--- PASS: TestSubrouteRoutingInert (0.10s)`, `ok github.com/biggs-100/biggz-ai/internal/sdd 0.207s`, exit 0. |
| Runtime harness command/scenario and exact result | In-process integration path: `StatusWithOptions` over two `t.TempDir()` workspaces seeded by `seedSubrouteChange` (proposal-only change, `state.yaml` with and without `subroute: direct-inline`); both derivations return `Route == "organic"`, declared `Subroute == "direct-inline"`, and equal `NextRecommended`. N/A external command — no CLI/process surface is modified. |
| Rollback boundary | Delete the `TestSubrouteRoutingInert` block (35 lines) in `internal/sdd/subroute_test.go`; no production code is involved, so removal restores pre-change behavior alone. |

## Keep-Passing / Build / Vet

| Check | Result |
|---|---|
| `go test ./internal/sdd/ -run 'TestSubroute|TestRoute|TestStatus' -count=1` | `ok github.com/biggs-100/biggz-ai/internal/sdd 2.086s`, exit 0 |
| `go test ./internal/sdd/ -run TestEvaluateRoute -count=1` | `ok github.com/biggs-100/biggz-ai/internal/sdd 0.101s`, exit 0 |
| `go test ./internal/sdd/ -count=1` (raw, `PI_SUBAGENT_CHILD=1` inherited) | FAIL, exit 1, 25.263s — only `TestCheckCheckpointAsk` + `TestValidateCheckpointSubstance` fail with `checkpoint asks may only be emitted by orchestrator, not sub-agent (ownership)`; expected by design for sub-agent sessions |
| `PI_SUBAGENT_CHILD= go test ./internal/sdd/ -count=1` (env cleared) | `ok github.com/biggs-100/biggz-ai/internal/sdd 24.684s`, exit 0 |
| `go build ./...` | exit 0, no output |
| `go vet ./...` | exit 0, no output |
| `gofmt -l internal/sdd/subroute_test.go` (go1.25 toolchain) | exit 0, no files listed |
| Full suite | `go test ./...` not run/claimed: `internal/review` runs ~163s against the 180s budget under full-suite load on this Windows box and passes in isolation. |

## Modern Go Guidance

Consulted `sh "C:/Users/USER/.pi/agent/skills/use-modern-go/scripts/run-tool.sh" list --file-path internal/sdd/subroute_test.go` (go.mod targets Go 1.25.0); full list read. Relevant guidance evaluated: `testing_t_context` (no context use in this test — N/A), `loopvar_capture` (no loop-variable closures added), `slices_contains`/`slices_sorted` (no slice scans added). The test uses existing fixture style (`t.Helper`, `t.Setenv`, `t.TempDir`, `t.Fatalf`) and Go 1.22+ range semantics already in the file; no flagged idiom applies.

## Diff Summary (authored, Unit 1)

```
internal/sdd/subroute_test.go | 35 +++++++++++++++++++++++++++++++++++
1 file changed, 35 insertions(+)
```

35 authored additions, 0 deletions — well inside the 400-line review budget. No commit was created; changes remain in the working tree for the orchestrator-owned delivery.

## Remaining Work (gated — not started)

- **Phase 3 (destructive sync) — BLOCKED on a human gate**: 3.1 requires explicit `allow-destructive` authorization because the REQ-OR-003 block (~39 lines) exceeds `largeMutationThreshold = 20` (`internal/sdd/openspec-deltas.go:13`). No authorization, no sync. 3.2 (merge delta into `openspec/specs/orchestrator/spec.md`) and 3.3 (`rg` check + package rerun) are unticked.
- **Phase 4 (delivery & archive) — unticked**: 4.1 `bug` issue via `issue-creation` (needs `Closes #<N>` + exactly one `type:*` label); 4.2 archive only after verify PASS; 4.3 open PR 1 with verify evidence and maintainer `size:exception` request.
- No commit, no push, no branch operation, no attempt-ledger command was performed by this apply run.
