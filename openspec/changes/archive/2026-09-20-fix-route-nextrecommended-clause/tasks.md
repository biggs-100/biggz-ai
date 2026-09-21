# Tasks: fix-route-nextrecommended-clause

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | Code+spec ~90–120 (delta ~39, guard test ~50–70); SDD trail adds ~450–550 (proposal, design, tasks, apply-progress, verify/archive reports) |
| 400-line budget risk | Medium — code+delta fits one PR; trail-inclusive PR exceeds 400 |
| Chained PRs recommended | No — one deliverable; trail is pipeline record, not reviewable slices |
| Suggested split | Single PR; maintainer-applied `size:exception` (PRs #112, #123, #159) |
| Delivery strategy | auto-chain |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: Medium

Delta+guard fit one PR alone; verify/archive records push the delivery PR past 400 under the accepted `size:exception` precedent. Auto-chain proceeds with PR 1.

### Suggested Work Units

| Unit | Goal | PR | Focused test | Runtime harness | Rollback boundary |
|------|------|----|--------------|-----------------|-------------------|
| 1 `routing-inert-guard` | Equality guard + delta | PR 1 | `go test ./internal/sdd/ -run TestSubrouteRoutingInert -count=1` | In-process temp-workspace derivation; no CLI/process surface — N/A external command | Delete test hunk; no production code |
| 2 `record-sync-delivery` | Sync, issue, archive, PR | PR 1 | `go test ./internal/sdd/ -run 'TestSubroute&#124;TestRoute&#124;TestStatus' -count=1` | `rg -n 'nextRecommended.*MUST be empty' openspec/specs/` → no orchestrator hit | Revert spec-of-record hunk; delta stays in folder |

## Phase 1 — Guard (PR 1)

- [x] 1.1 `TestSubrouteRoutingInert` in `internal/sdd/subroute_test.go`, reusing `isolatedSubrouteHome`/`seedSubrouteChange`/`deriveSubrouteChange`: seed `subroute: direct-inline` vs no key (D3); assert `NextRecommended` equality (D2.1), `Subroute == "direct-inline"` (D2.2), `Route == "organic"` both (D2.3). Run `go test ./internal/sdd/ -run TestSubrouteRoutingInert -count=1`.
- [x] 1.2 Comment: Standard mode, no RED — guard locks shipped behavior; falsifiable only by divergence if routing reads `subroute`; fallback + no-`active` case prove the old clause unsatisfiable.
- [x] 1.3 `gofmt -l internal/sdd/subroute_test.go` → empty (CI tool: `.github/workflows/ci.yml:33`, `scripts/gofmtcheck.sh`); evidence in `apply-progress.md`.

## Phase 2 — Keep-passing (PR 1)

- [x] 2.1 `go test ./internal/sdd/ -run 'TestSubroute|TestRoute|TestStatus' -count=1`.
- [x] 2.2 `go test ./internal/sdd/ -run TestEvaluateRoute -count=1`.
- [x] 2.3 `go test ./internal/sdd/ -count=1`; `go build ./...`; `go vet ./...`.
- [x] 2.4 Record harness: both variants derived in-process over `t.TempDir()`; N/A external command.

## Post-verify gated steps (NOT apply tasks)

> **Restructured after `sdd-verify` (2026-09-20).** Steps 3.x–4.x were originally written as task checkboxes. Because verify readiness requires `taskProgress.AllComplete` (`internal/sdd/status.go:1192-1197`), the unchecked boxes kept the native dispatcher at `nextRecommended: apply` and `dependencies.verify: blocked` after the implementation unit was already done — recorded by verification as WARNING-2. They are not apply work: sync requires verify PASS first (`internal/assets/skills/sdd-sync/SKILL.md`) and archive requires the change to close. They are therefore listed below as gated steps **without checkboxes**, matching the repo convention (archived `tasks.md` files reach verify with zero pending boxes, e.g. `archive/2026-09-16-fix-checkpoint-ask-context/tasks.md`). Content is unchanged; only the checkbox form differs. The verification snapshot still reports `7/13` because it was taken before this restructure.

### Phase 3 — Destructive sync (gated)

- **3.1 GATE** — explicit human `allow-destructive` authorization before sync — REQ-OR-003 block ~39 lines > `largeMutationThreshold = 20` (`internal/sdd/openspec-deltas.go:13`). No authorization, no sync.
- **3.2 Merge delta** into `openspec/specs/orchestrator/spec.md`: one `REQ-OR-003` heading, invariant clause present, four preserved scenarios intact.
- **3.3 Post-sync check** — `rg -n 'nextRecommended.*MUST be empty' openspec/specs/` → no orchestrator hit; then `go test ./internal/sdd/ -count=1`.

### Phase 4 — Delivery & archive

- **4.1 PREREQUISITE** (orchestrator confirms with human, never silent): `bug` issue via `issue-creation` skill; PR needs `Closes #<N>` + exactly one `type:*` label (`.github/workflows/pr-check.yml`).
- **4.2 Archive** after verify PASS (`sdd-archive`): move folder to `openspec/changes/archive/`; archive report.
- **4.3 Open PR 1** with verify evidence; request maintainer `size:exception` (#112/#123/#159 precedent).

## Dependencies

1.1 → 1.2 → 1.3 → 2.x → 3.1 → 3.2 → 3.3 → 4.2 → 4.3; 4.1 precedes 4.3. Unit 1 independent; 3.2 consumes the already-written delta.

## Verification Map (5)

| Scenario | Proof |
|----------|-------|
| S1 organic direct + routing-inertness invariant | 1.1 (three assertions); clause text 3.2 |
| S2 organic route for delegated work | `TestSubrouteOrganicDeclared` (2.1) |
| S3 subroute omitted when undeclared | `TestSubrouteUndeclaredOmitted` (2.1) |
| S4 route for SDD work | `TestSubrouteSDDRouteSuppresses` (2.1); SDD `nextRecommended` conjunct textual-only |
| S5 continue includes route context | `TestSddContinue_*` (`go test ./cmd/biggz/ -run TestSddContinue -count=1`); no edit |

## Rollback

- Unit 1: delete `TestSubrouteRoutingInert` — independent of the sync.
- Unit 2: revert the spec-of-record hunk (delta returns with the folder); reverting the delivery commit restores the unsatisfiable `nextRecommended MUST be empty` clause as documented status quo; no runtime behavior.
