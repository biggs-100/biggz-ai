# Apply Progress: sdd-fast-lane

**Slice**: the whole change (Phases 1-5 of `tasks.md`).
**Work unit**: `slice-fast-lane` · ledger token `tok-5a76825b8a672e03e933dbb7` · request-id `req-fastlane-apply`.
**Mode**: Standard (no `strict_tdd`); repo convention honored — RED test first, then GREEN.
**Store**: hybrid — this file + BigMem mirror `sdd/sdd-fast-lane/apply-progress`.
**Provenance (honest)**: the delegated apply run was cut off by the harness timeout (20 min) while it was writing this artifact, after it had completed Phases 1-4 and most of Phase 5 (23 of 24 checkboxes) and had the package suite green. Its own captured output was lost with the timeout. The orchestrator completed task 5.6, ran the end-to-end dogfood and re-verified the package; those orchestrator runs are the evidence of record below.

### Tasks

- [x] 1.1-1.5 RED lane core: plan-only change through the real dispatcher reaches apply → all_done → verify → archive readiness; per-slot precedence; in-place graduation; plan checklist drives progress; canonical headings satisfy the verify totals.
- [x] 2.1-2.4 RED parity, legacy and dogfood: BigMem/filesystem parity for the same plan-only change; the legacy four-artifact path keeps its behaviour and blocked reasons; the CLI status path reports the lane; the legacy suites stay green.
- [x] 3.1-3.4 GREEN alias in **both** resolvers: `resolveArtifactPaths` aliases `plan.md` per slot (a real artifact wins, apply/verify never alias) and `bigmemArtifactPaths` gains the plan topic plus a content helper for the three direct `bySuffix["tasks"]` reads, so the filesystem and BigMem derivations agree.
- [x] 4.1-4.5 GREEN text surfaces: `instructions.go` read lists no longer demand four artifacts, `_shared/sdd-status-contract.md` states the lane-ready rule, `biggz-orchestrator-workflow.md:338` now says **never skip gates**, and `sdd-ff.md` gained the lane mode with the canonical `### Requirement:` / `#### Scenario:` scaffold.
- [x] 5.1-5.5 Apply-time checks: `deriveRoute` tolerates a plan-only change (no verdict persisted), the legacy renderers were checked, the engram branch resolves `bigmem:sdd/{name}/plan`, the CLI was **rebuilt** (assets are embedded) and the dogfood ran.
- [x] 5.6 Final: `go test ./... -count=1 -timeout 900s` green.

### Files changed

| File | Action | + | − | Changed |
|---|---|---|---|---|
| `internal/sdd/status.go` | modify | 22 | 4 | 26 |
| `internal/sdd/engram_status.go` | modify | 64 | 20 | 84 |
| `internal/sdd/instructions.go` | modify | 2 | 2 | 4 |
| `internal/assets/prompts/sdd/sdd-ff.md` | modify | 28 | 1 | 29 |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | modify | 1 | 1 | 2 |
| `internal/assets/skills/_shared/sdd-status-contract.md` | modify | 1 | 1 | 2 |
| `cmd/biggz/sdd_status_cli_test.go` | modify | 65 | 0 | 65 |
| **Tracked subtotal** | | 183 | 29 | **212** |
| `internal/sdd/status_lane_test.go` | create | 379 | 0 | 379 |
| **Test subtotal** | | 379 | 0 | **379** |
| **Total changed** | | **562** | **29** | **591** |

### Budget

Slice budget: 400 changed lines. Actual: **591** (+191, ≈48% over) — the lane test file alone (379 lines) exceeds the plan's 180-260 estimate because the RED matrix covers nine scenarios (lane, precedence, graduation, checklist, headings, parity, legacy, CLI, and the gate invariance check). Production-side the change is 212 lines. Resolution options: **`size:exception` for a single coherent PR** (recommended: the alias, its parity helper and their tests are one unit) or a split into alias vs text/docs — the second would separate the contract text from the behaviour it documents.

### Evidence

| Evidence | Command | Result |
|---|---|---|
| Package suite | `go test ./internal/sdd -count=1` | `ok` 25.583s (orchestrator re-run) |
| Full sweep (5.6) | `go test ./... -count=1 -timeout 900s` | **exit 0** — 60 packages `ok`, 0 `FAIL`, 0 panics |
| CLI rebuild (5.4) | `go build -o biggz.exe ./cmd/biggz` | build OK (required: assets are embedded) |
| **End-to-end dogfood (5.5)** | `./biggz.exe sdd-status --json --cwd <fixture>` over a change whose **only** artifact is `plan.md` | `active: 1` · **`nextRecommended: apply`** · slots `proposal=True specs=True design=True tasks=True` · `taskProgress {total: 2, completed: 1}` read from the plan · `dependencies.proposal/specs/design/tasks = all_done`, `apply = ready` |
| Legacy path | the full sweep includes the existing dispatcher suites (`status_test.go`, `status_v2_test.go`, `engram_status_test.go`, `topology_parity_test.go`) | green — the alias is additive |

### Deviations

- The 591/400 budget overrun (see Budget); nothing was skipped to compensate.
- The delegated run's own proof-of-run output was lost with the harness timeout; the orchestrator re-ran the package suite, the full sweep and the dogfood, and those runs are recorded above.
- `apply-progress.md` itself was written by the orchestrator, not by the interrupted run.
