# Design: fix-attempt-ledger-scope

## Technical Approach

Port semántico de upstream (`GA/.../runtime_admission.go:70-88`): el objetivo es el scope de UN work unit; `passed` lo cierra y el sucesor abre con `acquire` y otro `--work-unit`, presupuesto fresco y cadena preservada. Añade generación, campos `omitempty`, admisión unificada, `Reset` sin borrar `Attempts`, lifetime derivado. Sin tocar `Rescope`/guards de remediación.

## Architecture Decisions

### Decision: Clasificación de la completitud

| Opción | Tradeoff | Decisión |
|---|---|---|
| `work_unit_complete` (no `corrupt_authority`) | Toca wire y un test | **Elegida**: `corrupt_authority` solo para completitud anómala; `status.go:1256-1268` intacto; fuera de `blockedReasons` |

### Decision: Seam único de admisión

| Opción | Tradeoff | Decisión |
|---|---|---|
| `deriveScopeAdmission(store, ScopeRequest) ScopeDecision` (`sddattempt.go:448`), no reglas dispersas | Refactor de 3 call sites | **Elegida**: consumido por `Acquire:1312`, `Begin` y `StatusWithInstance:369-372` (wrapper) |

Abierto: 4 campos (vacío = sin cambio) → `invalid_continuation`; guards `:1392-1410` fuera.

### Decision: Codificación CAS

| Opción | Tradeoff | Decisión |
|---|---|---|
| `omitempty` + cero≡1 + derivación, no persistir gen1/lifetime con valor | Regla sutil, exige test; evita romper el downgrade | **Elegida**: persistidos `Generation`, `RuntimeAttempt.ObjectiveGeneration`, `Advances` (patrón `Resets :78`), idempotente por receipt `Requests[request_id]`; derivados en lectura: 0≡1 y `Lifetime*`; `cas_store.go` sin cambios |

### Decision: Cap de reembolsos 2×

| Opción | Tradeoff | Decisión |
|---|---|---|
| Por generación, no por change | `spec.md:104` under-specified | **Elegida**: checks (`runtimeRefundedAttempts`, `2*Max`) sobre la generación viva; `Lifetime*` sobre la cadena; enmienda abajo |

### Decision: Presupuesto del sucesor

| Opción | Tradeoff | Decisión |
|---|---|---|
| Fresco del request (`>0`, si no hereda), no heredar | Oculta loops; mitigado por `Lifetime*` | **Elegida**: `CumulativeChangedLines=0`; generación cerrada íntegra en `Attempts`+`Advances` |

### Decision: Mensaje de rechazo (`work_unit_complete`)

```
work unit %q is complete; it continues through a successor work unit: run `biggz sdd-attempt acquire <change> --work-unit "<a different label>" --request-id "<unique-id>"` with a different --work-unit; reset discards this scope instead of succeeding it
```

### Decision: Assets

| Archivo | Línea | Cambio |
|---|---|---|
| `biggz-orchestrator-workflow.md` | 184-187 | `complete` cierra el work unit; continuación con otro `--work-unit`; reset solo descarta |
| `sdd-status-contract.md` | 86-93, 229 | `passed` completa su work unit; sucesor con presupuesto propio |
| `opencode/commands/sdd-verify.md` | 22 | "active or complete" → active attempt del work unit verify |
| `prompts/sdd/sdd-verify.md` | 44, 82 | Acquire de `verify` tras `passed` = avance; sin reset |

Mirror si se toca `skills/sdd-verify/SKILL.md`; rebuild del CLI.

## Data Flow

```
gen1 acquire apply-a ─► #1 ─► settle passed ─► Complete gen1 · Attempts[#1]
gen2 acquire verify ─► Advance: vivo:=request (Complete=false, Cum=0),
      Generation=2 · Advances+=evento · #2 gen2 ─► settle passed
mismo label ─► blocked(work_unit_complete), ledger intacto
reset ─► limpia vivo; Attempts/Lifetime intactos · status ─► BlockedExit
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/sddattempt/sddattempt.go` | Modify | Campos, seam, `Acquire`/`Begin`/`Reset`, presupuesto, mensajes |
| `cmd/biggz/cli_sdd.go` | Modify | Print de `Generation`/`Lifetime*` |
| 4 assets | Modify | Tabla anterior |
| `advance_test.go` | Create | Escenarios |
| `legacy_compat_test.go` | Create | Fixture + bytes congelados |

## Interfaces / Contracts

```go
type ScopeRequest struct{ WorkUnit, EvidenceGoal string; MaxAttempts, MaxLines int }
type ScopeDecision struct{ Advance bool; Reason, Exit string }
func deriveScopeAdmission(store *RuntimeStore, req ScopeRequest) ScopeDecision
const BlockedReasonWorkUnitComplete = "work_unit_complete"
// Advances []RuntimeAdvance omitempty: RequestID, From/ToWorkUnit, From/ToGeneration, MaxAttempts, MaxLines, PrevRevision, AdvancedAt
// Derivado (no persistido): Generation/ObjectiveGeneration==0⇒1; Lifetime* sobre Attempts
```

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | Avance (gen2, 1→5, Cum=0), repetido, guard 4 campos, reset preserva N, atribución, remediación post-avance | `advance_test.go` (18 escenarios) |
| Legacy CAS | Fixture re-verifica content address; 0≡1 en vista; bytes gen1 congelados; legacy avanza | `legacy_compat_test.go` |
| Migrados | `acquire_settle_test.go:176-223`→`work_unit_complete`; `cas_store_test.go:80`, `remediation_derive_test.go` extendidos; `parity_test.go`, `rescope_test.go`, `sddattempt_test.go:177` sin cambios | Equivalente nuevo por assert viejo |
| CLI | Sin verbo/flag nuevo | Harness `cmd/biggz` |

## Threat Matrix

| Boundary | Applicability | Design response / RED test |
|---|---|---|
| Documentation-like paths | N/A — sin clasificación/ejecución | — |
| Git repository selection | Applicable — record+HEAD nuevos bajo `<git-common-dir>`; selectores root, subdir, no-git | Escribe donde leyó (falla sin store nuevo). RED: avance desde root/subdir → mismo common-dir; machine-scope en su dir |
| Commit / Push / PR | N/A — sin index/refspec/PR | — |

CAS (fuera de matriz): fixture y bytes gen1 como RED.

## Migration / Rollout

Sin migración. Gen1 no estampa campos ⇒ bytes idénticos a hoy; ledger avanzado fail-closed para binario viejo (fix-forward o restaurar HEAD); rebuild del CLI.

## Open Questions

- [ ] **REQUIRED spec-delta (blocking):** `spec.md:104` cap "`2×MaxAttempts` total" → por generación; no editar `openspec/specs/**`.
- [ ] `ResetResult.AttemptsReset` sin limpiar: ¿reinterpretar o renombrar?
- [ ] ¿Status imprime el hint del sucesor?
