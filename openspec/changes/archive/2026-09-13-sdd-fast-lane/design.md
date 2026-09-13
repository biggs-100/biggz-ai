# Design: sdd-fast-lane

## Technical Approach

Alias por-slot de `plan.md` en ambos resolvers de paths; la derivación (`collectArtifactDerivation` `internal/sdd/status.go:628` → `coreReady` `:868` → `resolveDependencies` `:1161`) queda intacta y el modo lane vive en `sdd-ff`.

## Architecture Decisions

### Decision: Filesystem alias seam

**Choice**: en `resolveArtifactPaths` (`status.go:994`), cada slot = real primero, plan después.

| Slot | Regla |
|---|---|
| `Proposal/Design/Tasks` | `existingPath(real)`, si vacío `existingPath(<changeRoot>/plan.md)` |
| `Specs` | `findSpecFiles` (`status.go:1028`), si vacío `[plan]` |
| `ApplyProgress/VerifyReport` | nunca aliasan |

Aguas abajo sin cambios: `tasksContent` (`status.go:638`) y `readSpecCounts` (`verify.go:121`) leen el path resuelto; checkboxes vía `countTaskProgressText` (`status.go:1106`).

### Decision: BigMem alias seam

**Choice** (`internal/sdd/engram_status.go`): extender el alias `spec↔specs` (`:146`) en `bigmemArtifactState` con fallback a `m["plan"]`; `bigmemArtifactPaths` (`:168`) añade el topic `plan` tras el real; helper `bigmemSlotContent(m, suffix)` (real, si no plan) en `:221`, `:258` (progress) y `:312` (edit authority), que leen `bySuffix[...]` directo. **Alternatives**: aliasar solo paths — insuficiente por `:221/:258/:312`. **Rationale**: precedente `spec↔specs`.

### Decision: Staleness/ambiguity policy

**Choice**: nada nuevo; precedencia por-slot ⇒ un `plan.md` scratch junto a artifacts reales no altera routing. **Alternatives**: warning — sin canal (`status.go:905` solo `blockedReasons`). El test de precedencia fija el caso.

### Decision: Parity guarantee

**Choice**: `status_lane_test.go`: mismo plan-only en FS (`seedDeriveChange`, `derive_test.go:14`) y BigMem (`seedBigMemChange`, `engram_status_test.go:18`) → idéntico artifact set y apply ready. Merge ya cubierto: `TestMergeFilesystemAndBigMem_*` (`engram_status_test.go:159`).

### Decision: Text surfaces

| Archivo | Cambio |
|---|---|
| `instructions.go:19,:25` | dejar de exigir cuatro artifacts; leer "planning artifacts — or the merged `plan.md` in the fast lane —" |
| `sdd-status-contract.md:223` | apply ready = planning set resuelto (reales o alias `plan.md`) y progress no all-done |
| `biggz-orchestrator-workflow.md:338` | "Never skip phases" → "Never skip gates" |
| `sdd-ff.md` | modo lane: scaffold de `plan.md` con `### Requirement:` / `#### Scenario:` (admission `verify.go:188,197,201`) + checklist `- [ ]` (`status.go:869` exige `Total > 0`); sin comando, overlay ni AGENTS.md |

**Nota assets**: `internal/assets/**` se embebe en el binario — **rebuild del CLI antes del dogfood y de los tests CLI**.

## Data Flow

```
plan.md ──alias──► slots{proposal,specs,design,tasks}=Done
                    tasksContent(:638) ─► progress(:1106)
coreReady(:868) ─► Apply ready ► all_done ─► Verify ready ► Archive ready
BigMem: bigmemArtifactPaths + bigmemSlotContent ─► misma ruta (:271)
counts del plan ─► admission(verify.go:188,197,201)
```

## File Changes

| File | Action | Descripción | Size |
|------|--------|-------------|------|
| `internal/sdd/status.go` | Modify | alias per-slot (`:994`) | S (~12) |
| `internal/sdd/engram_status.go` | Modify | alias `:146`, paths `:168`, helper `:221/:258/:312` | S (~15) |
| `internal/sdd/instructions.go` | Modify | read-lists (`:19,:25`) | S (~2) |
| `internal/assets/skills/_shared/sdd-status-contract.md` | Modify | apply-ready (`:223`) | S (~1) |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modify | `:338` | S (~1) |
| `internal/assets/prompts/sdd/sdd-ff.md` | Modify | modo lane | S (~15 doc) |
| `internal/sdd/status_lane_test.go` | Create | lane, precedencia, paridad, legado | M (~150-220) |
| `cmd/biggz/sdd_status_cli_test.go` | Modify | dogfood `sdd-status --json --cwd` | S (~30-40) |

Código ≈ 40-50 líneas; tests ≈ 180-260. Rollback: revertir el alias — `plan.md` deja de leerse, sin migración.

## Interfaces / Contracts

`bigmemSlotContent(m map[string]string, suffix string) string` — `m[suffix]`, si no `m["plan"]`, si no `""`. Interno.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit RED | plan-only → apply ready → all_done → verify ready → archive ready; precedencia; checklist desde plan; graduación in-place; legado intacto | `status_lane_test.go` vía `readChange`/`deriveChangeStatusCtx`, fixture `seedDeriveChange` |
| Parity | mismo plan-only FS vs BigMem → idéntico artifact set/apply | `seedBigMemChange` + `deriveBigMemChangeStatus` |
| E2E/CLI | `biggz sdd-status --json --cwd <fixture>` | harness `runSDDStatusCLIArgs` (`sdd_status_cli_test.go:13`) |

## Threat Matrix

N/A — solo alias de artifacts y texto; sin routing/executable/subprocess/VCS/process boundary nuevo.

## Gates Unchanged (must NOT change)

RDD receipt (`status.go:942` → `applyTerminalGateBlocks` `:905`), `biggz sdd-verify-validate` (`cmd/biggz/cli_sdd.go:351`), workload guard (`workload_guard.go:121`), session-summary (`session_guard.go:415`), edit authority (`edit_authority.go:407`). Verify count admission (`verify.go:188,197,201`) intacta: totals del envelope vs counts del plan.

## Migration / Rollout

Sin migración: escribir un artifact real gradúa in-place. Rebuild del CLI por assets embebidos.

## Open Questions

- [ ] `deriveRoute` (`status.go:1216-1223`) lee flags legacy `Has*`: un plan-only reportaría `organic`. Verificar consumidores de `route` en apply.
- [ ] Renderers legacy `sdd-continue`/status sobre plan-only — verificar en apply.
- [ ] Branch engram de `resolveArtifactPaths`: confirmar que `firstPath` tolera `bigmem:sdd/{name}/plan`.
