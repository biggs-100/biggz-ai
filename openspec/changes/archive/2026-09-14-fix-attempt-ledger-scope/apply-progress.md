# Apply Progress: fix-attempt-ledger-scope — Slice 1 (unidad 1 / PR 1)

## Summary

Slice 1 implementa la fundación de campos CAS y el seam único de admisión, **sin cambiar
ninguna decisión ni mensaje**. `RuntimeStore` gana `Generation int` y
`Advances []RuntimeAdvance` (ambos `omitempty`); `RuntimeAttempt` gana
`ObjectiveGeneration int` (`omitempty`). La generación 1 se representa por **ausencia de
campo**: ningún valor se estampa, de modo que todo registro pre-existente conserva sus
bytes canónicos y su content address (registros `record-<sha>.json` re-verificados al
cargar). La lógica de `deriveAdmissionBlocked` se movió a
`deriveScopeAdmission(store, req)`, el único dueño de la admisión, ahora consumido por
`Acquire`, `Begin` y `StatusWithInstance`; la clasificación reproduce la de hoy
(`corrupt_authority` + `"ledger is complete; reset required to continue"`,
`budget_exhausted` en sus dos variantes, `active_attempt`).

`cas_contract_test.go` congela el contrato CAS: un registro serializado sin los campos
nuevos re-canonicaliza byte-idéntico y su hash queda congelado
(`d1060a4cbf56e59c0ec84aa717d79934847ee6374a559fceca36441b82047fbe`), los campos nuevos
nunca aparecen en registros de generación 1 (ambas familias de mutación), y las vistas de
generación/vida se derivan en lectura sin tocar los bytes canónicos. Las suites existentes
de `internal/sddattempt/` e `internal/sdd/` pasan **sin modificación**.

## Tasks Completed

- [x] 1.1 Campos CAS-compatibles: `Generation`, `Advances`, `RuntimeAdvance`, `ObjectiveGeneration`, todos `omitempty`, no estampados en generación 1
- [x] 1.2 Seam `deriveScopeAdmission` consumido por `Acquire`, `Begin` y `StatusWithInstance`; `corrupt_authority` y mensajes existentes conservados; guards de scope intactos
- [x] 1.3 `cas_contract_test.go`: bytes pre-cambio estables, ausencia `omitempty` en generación 1, derivación read-time (generación y lifetime)

> Nota: `tasks.md` no se marca `[x]` en esta corrida — queda fuera de las superficies de
> edición permitidas para este slice (solo `sddattempt.go`, `cas_contract_test.go` y este
> documento). El orquestador debe marcar 1.1–1.3 al integrar.

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sddattempt/sddattempt.go` | Modified | +111/−28: campos `Generation`/`Advances`/`RuntimeAdvance`/`ObjectiveGeneration` (`omitempty`); `ScopeRequest`/`ScopeDecision` + `deriveScopeAdmission` (mueve `deriveAdmissionBlocked`, eliminado); wiring en `Acquire`, `Begin` y `StatusWithInstance`; `slices.IndexFunc` en el chequeo de intento activo (idiom moderno, comportamiento idéntico) |
| `internal/sddattempt/cas_contract_test.go` | Created | 232 líneas: fixture pre-cambio con hash congelado verificado por el load path real, ausencia de campos en registros generación 1, derivación read-time sin bytes |
| `openspec/changes/fix-attempt-ledger-scope/apply-progress.md` | Created | Este documento |

## Test Results

```
go test ./internal/sddattempt/ -count=1                        → ok  internal/sddattempt  4.035s
go test ./internal/sddattempt/ -run 'Legacy|CAS' -count=1      → ok  internal/sddattempt  1.156s   (focused command de la unidad 1)
go test ./internal/sdd/ -count=1                               → ok  internal/sdd         23.901s
go test ./cmd/biggz/ -run 'Attempt|Grant|SddStatus' -count=1   → ok  cmd/biggz            4.240s   (safety net, consumidor CLI)
go test ./internal/sddattempt/ -run 'TestCASContract' -count=1 -v → PASS (2/2)
go vet ./internal/sddattempt/                                  → exit 0
go vet -vettool=$(built tools/nosourcegrep) ./internal/sddattempt/ → exit 0
gofmt -l internal/sddattempt/                                  → sin archivos
go build ./cmd/biggz                                           → OK
```

Complejidad (delta medido contra HEAD con gocyclo/gocognit): `Acquire` 78→78 (cognit
163→163), `Begin` 34→35 (74→76), `deriveScopeAdmission` por debajo de umbral. Sin
funciones nuevas sobre umbral; `internal/sddattempt` no está en el gate CI
(`./internal/review ./internal/sdd ./internal/verification`).

## Work Unit Evidence (Unidad 1 — PR 1)

| Evidence | Required value |
|---|---|
| Focused test command + exact result | `go test ./internal/sddattempt/ -run 'Legacy\|CAS' -count=1` → exit 0, ok 1.156s; suite completa del paquete `go test ./internal/sddattempt/ -count=1` → exit 0, ok 4.035s |
| Runtime harness command/scenario + result | `cas_contract_test.go` publica el registro congelado en el store CAS real (`record-<rev>.json` + `HEAD`) y lo digiere vía `StatusWithInstance` (verificación de content address end-to-end); `go build ./cmd/biggz` OK. Sin cambio visible de CLI en este slice (N/A por la tabla de la unidad: sin cambio visible) |
| Rollback boundary | Revert de `internal/sddattempt/sddattempt.go` (seam + campos) y borrado de `cas_contract_test.go`: gen-1 sin estampar. Los registros escritos por este slice no contienen campos nuevos, por lo que un binario previo los sigue leyendo sin migración |

## Behavior Contract (sin cambios)

- Decisiones y mensajes idénticos: `corrupt_authority` / `"ledger is complete; reset required to continue"`; `budget_exhausted` (dos variantes); `active_attempt`.
- Guards de scope vivos en `Acquire` ("work unit scope changed without reset" / "evidence goal changed without reset") — slice 2 los reemplaza por el guard unificado.
- Intactos: `Rescope`, guards de remediación, `deriveSettleObligation`, `cas_store.go`, `openspec/specs/**`, `internal/assets/**`. Sin verbo/flag/HelpText nuevos.
- Consumo nuevo declarado: `Begin` ahora consulta el seam y rechaza un ledger completo con el mismo `corrupt_authority` (antes lo ignoraba) — es el wiring exigido por la tarea 1.2; ningún test existente lo cubría.

## Remaining Work (slices 2–3)

- **Slice 2 (avance/rechazo/cap):** `advance_test.go` (18 escenarios), amenaza git (root/subdir vs no-git), `work_unit_complete` + mensaje con sucesor, guard unificado de 4 campos (reemplaza los guards actuales), presupuesto fresco del sucesor, `Reset` que preserva `Attempts`/lifetime, cap de reembolsos `2×MaxAttempts` por generación viva, `legacy_compat_test.go`, migración de `acquire_settle_test.go:176-223`, extensiones de `cas_store_test.go` y `remediation_derive_test.go`.
- **Slice 3 (assets/CLI/dogfood):** 4 assets + mirror del skill, print de `Generation`/`Lifetime*` en `cmd/biggz/cli_sdd.go`, rebuild + dogfood multi-slice, `go test ./... -count=1` final.

## Notes / Deviations

- `deriveAdmissionBlocked` eliminado: su lógica vive ahora en `deriveScopeAdmission` (seam único; dos call sites reescritos, cero referencias restantes).
- El parámetro `req` del seam se acepta como shape acordado y aún no se consulta (el avance llega en slice 2).
- Etiquetas `omitempty` en todos los campos nuevos (no `omitzero`): restricción explícita del slice; para enteros la omisión del cero es idéntica, y mantiene la uniformidad de la familia de structs content-addressed (guideline `json_omitzero` evaluada vía `explain`).
- Espejo BigMem de este documento: ver reporte del sub-agente.


---

# Apply Progress: fix-attempt-ledger-scope — Slice 2 (unidad 2 / PR 2)

## Summary

Slice 2 implementa el núcleo semántico del cambio en orden TDD (RED verificado por
reversión, después GREEN): un `settle passed` completa SOLO su work unit, y un `acquire`
con otro `--work-unit` abre una generación sucesora —cadena de intentos intacta,
presupuesto fresco del propio request, sin `reset`—; repetir el mismo `--work-unit` se
rechaza con `blocked(work_unit_complete)` nombrando al sucesor; el guard de scope queda
unificado sobre los 4 campos; `Reset` deja de borrar `Attempts`; el cap de reembolsos
`2×MaxAttempts` se evalúa sobre la generación VIVA. Sin verbo/flag nuevos y sin campos
persistidos nuevos más allá de `Generation`, `Advances` y
`RuntimeAttempt.ObjectiveGeneration` (todos `omitempty`, ausentes en generación 1).

## Decisions (implementadas)

| Tema | Decisión |
|---|---|
| Clasificación complete | `Advance` cuando el request nombra un `--work-unit` distinto al del objetivo completo (sin attempt activo y último attempt `passed`); `work_unit_complete` cuando nombra el MISMO; `corrupt_authority` sólo para completitud anómala (sin attempt `passed` que nombrar). Un request SIN `--work-unit` (sonda pasiva de `status`) conserva la clasificación histórica `corrupt_authority` / `ledger is complete; reset required to continue`. |
| Guard unificado | `scopeChangeRefusal(store, req)`: UNA función sobre work-unit/evidence-goal/max-attempts/max-lines (vacío = sin cambio; cero almacenado = sin scope que preservar) → `invalid_continuation`. Se invoca desde `Acquire` (compact path), no desde el seam: `Begin` nunca tuvo guards de scope y `TestRecordRejected` (congelado) hace `Begin` con `--work-unit` distinto sobre objetivo abierto esperando admisión — meterlo en el seam rompía ese contrato. |
| Avance | `applyScopeAdvance`: `Generation = derived+1`; `Advances += evento` (RequestID, From/ToWorkUnit, From/ToGeneration, MaxAttempts/MaxLines del sucesor, PrevRevision, AdvancedAt); `CumulativeChangedLines = 0`; `Complete`/`DecisionRequired`/`EvidenceRevision`/`Binding*` limpiados; scope vivo = request (presupuesto propio, con el predecesor sólo como fallback si el request lo omite). Intentos previos intactos; el attempt sucesor estampa `ObjectiveGeneration` (0/ausencia en gen 1, 2+ en sucesores). |
| Reset | Sólo limpia el objetivo vivo (Complete, DecisionRequired, attempt activo, evidencia, scope). `Attempts`, `Generation` y `Advances` se preservan: `len(Attempts)` nunca decrece y cada attempt conserva outcome y evidencia. |
| Reembolsos / cap | `runtimeRefundedAttempts` y todos los chequeos de cap (`len`, `delivered`) se calculan sobre `liveGenerationAttempts(store)`; el sucesor arranca con `2×MaxAttempts` frescos y no se le carga nada del predecesor. |
| Vistas de status | `RuntimeStatus` gana `Generation` (derivada 0⇒1), `LifetimeAttempts`, `LifetimeChangedLines`, `LastAdvance`, y ahora también publica `CumulativeChangedLines` (antes quedaba siempre en 0). Nada de esto toca bytes canónicos. |

## Tasks Completed

- [x] 2.1 RED `advance_test.go` (10 tests: Advance, Fresh budget 1→5+Cum=0, New/Kept generation + bytes, `LastAdvance`, lifetime ×2, cap de reembolsos del sucesor, evidencia no heredada, guard 4 campos, Reset preserva)
- [x] 2.2 RED amenaza Git repository selection: avance root/subdir → mismo `<git-common-dir>` (registro + HEAD), no-git → dir máquina propia (`TestAdvance_GitRepositorySelection`, en `cas_store_test.go` extendiendo los selectores existentes)
- [x] 2.3 RED reescrito `acquire_settle_test.go` (familia `TestAcquire_BlockedWhenComplete`): mismo label → `work_unit_complete` + mensaje con sucesor; ledger intacto; sonda pasiva conserva el lock histórico (assert reemplazado, no borrado)
- [x] 3.1 GREEN sucesor admitido: cadena preservada, provenance registrada, presupuesto del request, sin reset (también por `Begin`, mismo seam)
- [x] 3.2 GREEN `BlockedReasonWorkUnitComplete = "work_unit_complete"` + mensaje EXACTO del design
- [x] 3.3 GREEN guards `:1392-1410` eliminados; guard unificado de 4 campos → `invalid_continuation`; scope idéntico admitido
- [x] 4.1 GREEN `Reset` preserva `Attempts` (sólo limpia el objetivo vivo)
- [x] 4.2 GREEN cap `2×MaxAttempts` por generación viva; sucesor no cargado por el predecesor
- [x] 5.1 `legacy_compat_test.go`: fixture PRE-CAMBIO re-verificado por el load path real, bytes/hash congelados (`b2ac4a5fd8a93cd032cc8d389e1d6f0fce392a8d6ce129e54dc9e90a50d34d6f`), default derivado 0⇒1 en vistas sin tocar bytes, y el ledger legacy completo avanzando con otro `--work-unit`
- [x] 5.2 `cas_store_test.go` extendido (migrado pre-cambio avanza en el MISMO store clonado) + `internal/sdd/remediation_derive_test.go` (correctivo admitido tras avance de generación; evidencia ya remediada rechazada)
- [x] 5.3 Verificado sin cambios: `parity_test.go`, `rescope_test.go`, `sddattempt_test.go:177`, `budget_refund_test.go` (y `cas_contract_test.go`, `machine_scope_test.go`)

## RED evidence (prueba por reversión temporal, sin tocar git)

| Revert temporal | Fallos observados | Restauración |
|---|---|---|
| rama `Advance` → `corrupt_authority` | 5 tests FAIL (DistinctWorkUnit, FreshBudget, SuccessorRefundBudget, GitRepositorySelection, LegacyCompat_Advances) | restaurado — suite verde |
| casos MaxAttempts/MaxLines fuera de `scopeChangeRefusal` + `live := store.Attempts` | FAIL SuccessorStartsWithFullRefundBudget + subtests max_attempts/max_lines del guard | restaurado — suite verde |
| `store.Attempts = nil` de vuelta en `Reset` | FAIL TestReset_PreservesTheAttemptChain | restaurado — suite verde |

## Files Changed

| File | Action | Líneas |
|------|--------|--------|
| `internal/sddattempt/sddattempt.go` | Modified | +353/−77: seam con `Advance`, `workUnitCompleteExit`, `scopeChangeRefusal`, `applyScopeAdvance`, helpers derivados (`derivedGeneration`, `liveGenerationAttempts`, `lifetimeChangedLines`, `lastAdvanceEntry`), cap vivo, `Reset` preservador, vistas de status |
| `internal/sddattempt/advance_test.go` | Created | 691 líneas (10 tests de avance/guard/reset) |
| `internal/sddattempt/legacy_compat_test.go` | Created | 193 líneas (bytes congelados + avance legacy) |
| `internal/sddattempt/acquire_settle_test.go` | Modified | +24/−7 (familia `:176-223` reescrita) |
| `internal/sddattempt/cas_store_test.go` | Modified | +157 (amenaza git + migrado que avanza) |
| `internal/sdd/remediation_derive_test.go` | Modified | +97 (correctivo tras avance / remediada rechazada) |
| `openspec/changes/fix-attempt-ledger-scope/apply-progress.md` | Modified | esta sección (merge con slice 1) |

## Test Results

```
go test ./internal/sddattempt/ -count=1                          -> ok  internal/sddattempt  5.053s
go test ./internal/sddattempt/ -run 'Legacy|CAS' -count=1        -> ok  internal/sddattempt  1.052s
go test ./internal/sddattempt/ -run 'Advance|Reset|Legacy|CAS' -count=1 -> ok internal/sddattempt 2.567s (focused unidad 2)
go test ./internal/sdd/ -count=1                                 -> ok  internal/sdd         25.310s
go build ./...                                                   -> OK
go vet ./internal/sddattempt/ ./internal/sdd/                    -> exit 0
go vet -vettool=<built tools/nosourcegrep/main.go> ./internal/sddattempt/ ./internal/sdd/ -> exit 0
gofmt -l internal/sddattempt/ internal/sdd/                      -> sin archivos
```

CLI dogfood real (repo git temporal via `mktemp`, binario `go build`, ledger real intacto):

```
1) acquire ch-dogfood --work-unit apply --max-attempts 1 --max-lines 400   -> token A
2) settle A passed                                                        -> complete
3) acquire ch-dogfood --work-unit verify --max-attempts 2 --max-lines 30  -> ADMITE (avance, sin reset)
4) settle B passed                                                        -> complete
5) acquire ... --work-unit verify (repetido)                              -> exit 1
   error: blocked(work_unit_complete): work unit "verify" is complete; it continues through a
   successor work unit: run `biggz sdd-attempt acquire <change> --work-unit "<a different label>"
   --request-id "<unique-id>"` with a different --work-unit; reset discards this scope instead of
   succeeding it
```

## Work Unit Evidence (Unidad 2 — PR 2)

| Evidence | Required value |
|---|---|
| Focused test command + exact result | `go test ./internal/sddattempt/ -run 'Advance\|Reset\|Legacy\|CAS' -count=1` → exit 0, ok 2.567s; suite completa del paquete → ok 5.053s; `go test ./internal/sdd/ -count=1` → ok 25.310s |
| Runtime harness command/scenario + result | CLI real sobre repo git temporal: `acquire apply → settle passed → acquire verify` (avance, sin reset) `→ settle passed → acquire verify` repetido → `blocked(work_unit_complete)` con el mensaje del sucesor (exit 1). Integración git en test: `TestAdvance_GitRepositorySelection` (root/subdir → mismo `<git-common-dir>` con registro+HEAD; no-git → dir máquina propia) |
| Rollback boundary | `internal/sddattempt/sddattempt.go` (avance + guard + reset + cap + vistas) junto con los tests nuevos: revertirlo devuelve la clasificación previa. Los registros emitidos en generación 1 no estampan campos nuevos, así que un binario previo los re-lee; sin cambios en `cas_store.go`, assets, CLI ni `openspec/specs/**` |

## Escenario → test (ADDED requirements)

| Escenario spec | Test |
|---|---|
| Successor Advance / Advance | `TestAdvance_DistinctWorkUnitAfterPassedObjective` |
| Successor Advance / Fresh budget | `TestAdvance_FreshBudget` |
| Repeated Work Unit Refusal / Repeat refused | `TestAdvance_SameWorkUnitRefused`, `TestAcquire_BlockedWhenComplete` |
| Repeated Work Unit Refusal / Ledger intact | `TestAdvance_SameWorkUnitRefused`, `TestAcquire_BlockedWhenComplete` |
| Scope-Change Guard / Any field refused | `TestScopeGuard_AnyFieldRefusedAndUnchangedAdmitted` |
| Scope-Change Guard / Unchanged scope | `TestScopeGuard_AnyFieldRefusedAndUnchangedAdmitted` |
| Reset Preserves the Audit / Live state cleared | `TestReset_PreservesTheAttemptChain` |
| Reset Preserves the Audit / Attempts survive | `TestReset_PreservesTheAttemptChain` |
| Generation Attribution / New generation | `TestAdvance_GenerationAttribution` |
| Generation Attribution / Kept generation | `TestAdvance_GenerationAttribution`, `TestLegacyCompat_*` |
| Lifetime Accounting / Survives advance | `TestAdvance_LifetimeSurvives`, `TestAdvance_DistinctWorkUnitAfterPassedObjective` |
| Lifetime Accounting / Survives reset | `TestReset_PreservesTheAttemptChain` |
| Pre-Change Ledger Compatibility / Legacy advances | `TestLegacyCompat_AdvancesWithDifferentWorkUnit`, `TestMigration_ImportsLegacyLedgerOnce` (extendido) |
| Pre-Change Ledger Compatibility / Derived default | `cas_contract_test.go` (slice 1) + `TestLegacyCompat_FrozenRecordAndDerivedGeneration` |
| Remediation Admission / Corrective admitted | `TestRemediationCorrectiveAdmittedAfterGenerationAdvance` |
| Remediation Admission / Remediated refused | `TestRemediationCorrectiveAdmittedAfterGenerationAdvance` |
| No New CLI Surface / Existing surface | dogfood CLI (arriba) + todos los tests de avance vía `Acquire` |
| No New CLI Surface / No new verb | slice 2 no agrega verbo ni flag (guard 6.3 queda para slice 3) |
| Interrupted Refund Capped / Successor full budget | `TestAdvance_SuccessorStartsWithFullRefundBudget` |
| (resto de la requirement MODIFIED) | `budget_refund_test.go` congelado (1–3), sin cambios |

## Guard Constraints (verificado)

- Sin campos persistidos nuevos fuera de `Generation`, `Advances`, `RuntimeAttempt.ObjectiveGeneration` (todos `omitempty`, ausentes en generación 1).
- No tocado: `Rescope`, guards de remediación, `deriveSettleObligation`, `cas_store.go`, `internal/assets/**`, `cmd/biggz/cli_sdd.go`, `openspec/specs/**`.
- Sin verbo ni flag nuevos; sin cambios de HelpText. El avance usa la forma posicional existente de `sdd-attempt acquire`.
- Tests congelados intactos y verdes: `parity_test.go`, `rescope_test.go`, `sddattempt_test.go:177`, `budget_refund_test.go`, `cas_contract_test.go`, `machine_scope_test.go`, `cas_store_grant_test.go`.

## Notes / Deviations

- `tasks.md` no se marca `[x]` en esta corrida: queda fuera de las superficies de edición permitidas. El orquestador debe marcar 2.1-2.3, 3.1-3.3, 4.1-4.2 y 5.1-5.3 al integrar.
- `ResetResult.AttemptsReset` conserva su valor (conteo al momento del reset) aunque los intentos ya no se borren: la pregunta abierta del design queda para el slice 3 (print del CLI).
- La sonda de status (request vacío) sobre un ledger completo mantiene `corrupt_authority` / `ledger is complete; reset required to continue`: es lo que congela `cas_contract_test.go` (slice 1) y lo que upstream documenta («un caller que no nombra work unit no nombró scope sucesor»). El acquire que nombra el mismo label obtiene `work_unit_complete`; el que nombra otro, avanza.
- Presupuesto del sucesor: para `Acquire` el request ya trae los defaults 3/400 aplicados antes del digest (contrato CAS del receipt intacto); el fallback «si el request omite, hereda» sólo actúa con valores 0 reales (path `Begin`).
- `Acquire` publica ahora `CumulativeChangedLines` en `RuntimeStatus` (antes siempre 0); sin efecto en bytes canónicos.
- Espejo BigMem de este documento: ver reporte del sub-agente.

## Remaining Work (slice 3)

- 4 assets + mirror del skill (`biggz-orchestrator-workflow.md`, `sdd-status-contract.md`, `opencode/commands/sdd-verify.md`, `prompts/sdd/sdd-verify.md`), print de `Generation`/`Lifetime*` en `cmd/biggz/cli_sdd.go`, rebuild + dogfood multi-slice, `go test ./... -count=1` final (6.1-6.5).

---

## Slice 3 (unidad 3 / PR 3) — surface, correcciones y aceptación

**Autoría mixta, declarada honestamente**: la corrida delegada de `sdd-apply` agotó el timeout del harness (20 min) mientras arrancaba la tarea 6.1. El árbol ya contenía: la corrección (a) en el ledger y su routing, `internal/sdd/status.go`, los cuatro assets, `cmd/biggz/cli_sdd.go` y los tests de CLI. El **orquestador completó la cola**: rebuild, dogfood de aceptación, suite completa y este registro.

### Tareas
| Tarea | Estado | Evidencia |
|---|---|---|
| 6.1 Assets | Hecha (por el subagente) | 4 superficies: `biggz-orchestrator-workflow.md:184-187`, `sdd-status-contract.md:86-93,229`, `opencode/commands/sdd-verify.md:22`, `prompts/sdd/sdd-verify.md:44,82`. `internal/assets/skills/sdd-verify/SKILL.md` NO se tocó → el mirror `skills/sdd-verify/SKILL.md` queda intacto (regla byte-idéntica respetada por omisión) |
| 6.2 Print CLI | Hecha | `cli_sdd.go`: `Generation:`, `Lifetime attempts:`, `Lifetime lines:` en `status` |
| 6.3 Guardas | Verificada | Sin cambios en `Rescope`, guards de remediación, `deriveSettleObligation` ni `cas_store.go`; sin verbo/flag nuevos; `openspec/specs/**` intacto (confirmado con `git status`) |
| 6.4 Rebuild + dogfood | Hecha (orquestador) | `go build -o biggz.exe ./cmd/biggz` + dogfood en repo desechable (abajo) |
| 6.5 Suite completa | Hecha (orquestador) | `go test ./... -count=1 -timeout 900s` → exit 0 |

### Corrección (a) — status pasivo
La proyección de status sobre un ledger legítimamente completo ahora reporta `work_unit_complete` con el exit que nombra al sucesor, en vez de `corrupt_authority` + "reset required to continue". `corrupt_authority` queda para completitud anómala. El pin congelado de `cas_contract_test.go` se actualizó (autorizado) y `isStaleDecisionRequired` (`internal/sdd/status.go`) extendió la clasificación de decisión obsoleta al nuevo motivo — **desviación del design, que afirmaba que `status.go` no necesitaba cambios**.

### Corrección (b) — mensaje del reset
`ResetResult.AttemptsReset` → `AttemptsPreserved` (`json:"attempts_preserved"`), y el CLI imprime `Previous attempts preserved: %d`. Cierra la open question del design: el mensaje ya no afirma destruir lo que preserva. Consumidores verificados: campo, tag, print del CLI y tests de CLI.

### Dogfood de aceptación (binario reconstruido, repo desechable `/tmp/dogfood`)
```
1) acquire apply-work                                  → token
2) settle --outcome passed                             → complete: true, remaining_attempts: 2
3) status                                              → Blocked reason: work_unit_complete + exit que nombra al sucesor
4) acquire --work-unit verify-work  (OTRO label)       → ADMITIDO (avance, sin reset)
5) settle --outcome passed                             → complete: true
6) acquire --work-unit verify-work  (label repetido)   → blocked(work_unit_complete) con el mensaje exacto del design
7) status                                              → Generation: 2 · Attempts: 2 · Lifetime attempts: 2
8) reset                                               → "Previous attempts preserved: 2"; status sigue con Attempts: 2
```
Criterio del proposal cumplido: el change multi-slice llega con evidencia por slice y **cero resets de mantenimiento** (el reset del paso 8 se ejecutó solo para probar el mensaje).

### Cola del change
Ciclo SDD de planificación + 3 slices de apply completos. Pendiente: `sdd-verify`, `sdd-sync` y `sdd-archive`. **Decisión de entrega pendiente del mantenedor**: la slice 2 sola son ~1.599 líneas autoradas (430 de producción + ~1.169 de tests) contra el presupuesto de revisión de 400; la fragmentación en PRs de ≤400 exige separar tests de código, así que hay que elegir entre `size:exception` en el PR semántico o un split artificial.

---

# Apply Progress: fix-attempt-ledger-scope — Slice correctiva (W1 + W2)

## Summary

Slice correctiva sobre el árbol apply ya verificado (19/19 tasks, `pass_with_warnings`). Cierra la regresión dura **W1** — `Reset` preservaba la cadena pero no abría presupuesto nuevo, dejando el ledger sin salida cuando la generación viva alcanzaba `2×MaxAttempts` — y la brecha de cobertura **W2** — la rama `work_unit_complete` de `isStaleDecisionRequired` sin test. Sin verbo ni flag nuevos; sin tocar `Rescope`, guards de remediación, `deriveSettleObligation`, `cas_store.go`, assets, `openspec/specs/**` ni la superficie CLI (solo textos de exit del ledger salen del paquete).

## El defecto (W1)

`liveGenerationAttempts(store)` filtra por `max(attempt.ObjectiveGeneration, 1) == derivedGeneration(store)`. Desde el cambio, `Reset` preserva la cadena DENTRO de la misma generación, así que los intentos preservados siguen contando como vivos: cuando `len(live)` llega a `2×MaxAttempts`, el guard (`sddattempt.go:952`; gemelas en el path `Begin` y en el cap del `Acquire`) queda permanentemente verdadero y no hay salida. La sonda del orquestador: `reset` → `acquire --max-attempts 1` → `invalid_continuation: attempt budget changed without reset` (decirle “reset” a quien acaba de resetear); `reset --max-attempts 1` → `acquire` → `budget_exhausted (2x cap)`; y un `acquire` con otro `--work-unit` (sucesor) también rechazado. El exit bloqueado publicitaba un remedio que no funcionaba.

## Mecanismo elegido (y por qué)

1. **Epoch de presupuesto = avance de generación estampado por `Reset`** (`store.Generation = derivedGeneration(store)+1`; además `RuntimeReset.ToGeneration` con `omitempty`). Rationale: la generación YA es «el objetivo vivo» del modelo y `liveGenerationAttempts` YA es el seam compartido de presupuesto, cap, reembolsos y acumuladores; `applyScopeAdvance` (sucesor) usa el mismo patrón. Un contador de epoch paralelo habría duplicado esos seams. `to_generation` con `omitempty` no altera los bytes canónicos de registros previos (los `Resets` viejos re-canonicalizan idénticos). Se añade `CumulativeChangedLines = 0` en `Reset` — mismo tratamiento que `applyScopeAdvance` — porque un epoch con la línea agotada quedaba igualmente sin salida («fresh bounded budget ... accumulator purposes»).
2. **El guard de presupuesto solo compara sobre objetivo abierto** (`objectiveOpen := len(liveGenerationAttempts(store)) > 0` en `scopeChangeRefusal`) y el acquire que ABRE el objetivo adopta el presupuesto del request (`len(live) == 0 || store.Max* == 0`). Un epoch fresco no tiene scope consumido que preservar: el acquire lo declara — exactamente como el primer acquire de un ledger — y el mensaje sin sentido desaparece por comportamiento, no solo por texto. Los cuatro casos del guard siguen rechazando mientras el objetivo está abierto (pin congelado intacto: `TestScopeGuard_AnyFieldRefusedAndUnchangedAdmitted`).
3. **Exits con remedio que funciona**: constante `budgetRecoveryHint` anexada a los cinco exits de presupuesto agotado del ledger (`(2x cap)`, `(2x cap with refunds)`, `max attempts reached, decision required`, y el `decision required` del seam); se elimina el «or settle the obligated evidence», que era una vía muerta (un settle exige token de un intento activo que el estado no tiene). El mensaje «changed without reset» sobre objetivo abierto queda igual porque ahí es verdadero y el reset sí funciona.

## RED primero (orden mandatorio)

`internal/sddattempt/budget_recovery_test.go` se escribió ANTES del fix y falló contra el código actual (prueba capturada, sin tocar git):

- `TestReset_OpensFreshBudgetForTheLiveObjective` → `generation after reset = 1, want 2 (a fresh budget epoch)` — y tras esa aserción, el acquire same-label caía en `budget_exhausted (2x cap)`.
- `TestSuccessorNotGatedByPredecessorExhaustedBudget` → `successor was gated by the predecessor's exhausted budget: blocked(invalid_continuation): attempt budget changed without reset: have 2, acquire wants 5` — el mensaje exacto de la sonda.

W2: test añadido y cobertura PROBADA por reversión temporal de la rama `work_unit_complete` en `internal/sdd/status.go` (revert → el test falla en `a complete work unit carrying a settle obligation must classify as a stale decision` → `status.go` restaurado byte-idéntico, diff vacío contra copia previa).

## Los tests de la corrección

1. `TestReset_OpensFreshBudgetForTheLiveObjective` — cadena `[failed con evidencia, 3×interrupted]` con `MaxAttempts=2` → decision-required en el cap 2×; acquire bloqueado cuyo exit nombra `reset`; `Reset` preserva 4; status `Generation=2`, lifetime intacto; acquire same-label **ADMITIDO**; settle failed deja `RemainingAttempts=1` (no 0, predecesor no cargado); segundo acquire+settle admitidos; cambio de presupuesto sobre el objetivo reabierto sigue `invalid_continuation` (límite del guard); auditoría íntegra (outcome+evidencia de los preservados, `ObjectiveGeneration=0` de la época cerrada, `Resets[0].ToGeneration=2`, acumulador fresco=3, lifetime no decrece).
2. `TestSuccessorNotGatedByPredecessorExhaustedBudget` — mismo agotamiento; acquire con OTRO `--work-unit` bloqueado pre-reset; `Reset`; acquire `--work-unit verify-work --max-attempts 5` **ADMITIDO** con presupuesto propio adoptado (5), generación 2, cadena de 5, predecesor intacto.
3. **W2** `TestStaleDecisionCompletedLedgerRoutesUnstranded` (en `internal/sdd/remediation_derive_test.go`) — ledger complete vía begin/finish con falla no remediada (obligación abierta): probe `work_unit_complete` + obligación; `isStaleDecisionRequired` true; `readChange` → `RemediationState` cero, `Dependencies.Verify=ready`, `NextRecommended=verify` (verify/archive no varados); completitud anómala (complete + decision-required) sigue clasificando `corrupt_authority`.

## Files changed (slice correctiva)

| File | Acción | Delta |
|---|---|---|
| `internal/sddattempt/sddattempt.go` | Modified | neto **+38 líneas** (12 hunks): `RuntimeReset.ToGeneration` (`omitempty`), const `budgetRecoveryHint`, `Reset` con epoch (generación+1, acumulador 0, provenance), `scopeChangeRefusal` con `objectiveOpen`, adopción de presupuesto del acquire que abre, 5 exits con remedio, bloque duplicado de presupuesto eliminado |
| `internal/sddattempt/budget_recovery_test.go` | Created | **301 líneas** (2 tests + fixture `exhaustLiveGenerationBudget`) |
| `internal/sdd/remediation_derive_test.go` | Modified | **+101 líneas** (test W2) |
| `internal/sdd/status.go` | touched, restaurado | revert temporal para probar la cobertura W2; restaurado byte-idéntico |

## Test results (comandos exactos)

```
RED (pre-fix): go test ./internal/sddattempt/ -run 'TestReset_OpensFreshBudgetForTheLiveObjective|TestSuccessorNotGatedByPredecessorExhaustedBudget' -count=1 -v
  → FAIL x2 (evidencia arriba)
GREEN (post-fix): mismo comando → PASS x2, ok 0.969s
focused unidad: go test ./internal/sddattempt/ -count=1                      → ok 5.691s
go test ./internal/sdd/ -count=1                                             → ok 22.915s
go test ./internal/sddattempt/ -run '<frozen set>' -count=1                  → ok 2.546s (18 tests nombrados PASS)
go test ./internal/sdd/ -run 'TestStaleDecisionCompletedLedgerRoutesUnstranded' -count=1 → ok
go test ./cmd/biggz/ -run 'TestSDDAttempt' -count=1                          → ok (safety net consumidor CLI)
gofmt -l internal/sddattempt/ internal/sdd/                                  → vacío
go vet ./internal/sddattempt/ ./internal/sdd/                               → exit 0
```

Suites congeladas verdes y **sin modificar**: `budget_refund_test.go` (TestRefund/TestDualBudget/TestRecordRejected), `parity_test.go` (TestForInstance/TestRescopeGuards/TestRescopeNarrowWedge), `rescope_test.go`, `sddattempt_test.go:177` (TestFinish_RequestIDReplayWithoutActiveAttempt), `machine_scope_test.go`, `cas_contract_test.go`, `legacy_compat_test.go`.

## Work Unit Evidence (corrección W1+W2)

| Evidence | Required value |
|---|---|
| Focused test command + exact result | `go test ./internal/sddattempt/ -run 'TestReset_OpensFreshBudgetForTheLiveObjective|TestSuccessorNotGatedByPredecessorExhaustedBudget' -count=1` → RED pre-fix (2 FAIL, mensajes transcriptos) → GREEN post-fix (2 PASS, ok 0.969s); paquete completo ok 5.691s; `go test ./internal/sdd/ -count=1` → ok 22.915s; test W2 ok |
| Runtime harness command/scenario + result | Path de integración real del ledger CAS: los tests de `budget_recovery_test.go` ejercen `Replay/Commit` sobre el store real con `Current`/`HEAD` y receipts idempotentes; el test W2 corre en un repo git temporal (`gitInitTemp`) con el ledger bajo `<git-common-dir>` y deriva `readChange` completo (routing verify/archive). Dogfood CLI `sdd-attempt reset|acquire` reservado al orquestador (aquí prohibido) |
| Rollback boundary | Revertir los hunks de `sddattempt.go` + borrar `budget_recovery_test.go` + revertir el test W2 devuelve el comportamiento previo exacto (reset sin epoch, guard de presupuesto incondicional, exits viejos); `status.go` ya está byte-idéntico y `cmd/biggz`, assets y specs intactos |

## Pendiente / no hecho

- `go test ./...` completo: reservado al orquestador (instrucción explícita del slice).
- Sonda CLI end-to-end `sdd-attempt reset|acquire`: reservada al orquestador (restricción explícita: no ejecutar `acquire|settle|reset` del CLI). La simulación paquete-nivel reproduce la sonda al detalle.
- `tasks.md` no se toca (fuera de las superficies de edición); el orquestador decide si el change necesita marcas nuevas.
- W3 (presupuesto de review 400 líneas) sigue siendo decisión del mantenedor: sin resolver.
