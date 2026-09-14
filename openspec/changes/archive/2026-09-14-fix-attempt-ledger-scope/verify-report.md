```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:3fc55d07630e90063d2b6d4c73ab854680ed11194ad89221a147608906086ea7
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 11/11
scenarios: 26/26
test_command: go test ./... -count=1 -timeout 900s
test_exit_code: 0
test_output_hash: sha256:3fc55d07630e90063d2b6d4c73ab854680ed11194ad89221a147608906086ea7
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: fix-attempt-ledger-scope
**Version**: N/A (delta spec `specs/runtime/spec.md`; `openspec/specs/runtime/spec.md` se sincroniza en `sdd-sync`)
**Mode**: Standard (Strict TDD: false — `openspec/config.yaml:11`; no se cargó `strict-tdd-verify.md`)

Re-verificación completa del estado corregido (regresión W1 arreglada + spec enmendada). El reporte anterior describía un estado superado; este documento lo reemplaza. Al arrancar, `sdd-status` reportaba `nextRecommended: remediate` con razón «verify result total 10 does not match actual requirement count 11» — la evidencia persistida del verify previo ya no conciliaba con el delta enmendado; esta corrida reconstruye la evidencia desde cero.

### Completeness

| Metric | Value |
|--------|-------|
| Tasks total | 19 |
| Tasks complete | 19 |
| Tasks incomplete | 0 |

`biggz sdd-status --json` → `taskProgress.allComplete: true`, `dependencies.verify: ready` (artefactos proposal/specs/design/tasks/apply todos `done`). Verificación completa de las tres dimensiones (completeness + correctness + coherencia de design).

### Build & Tests Execution

**Build**: ✅ Passed

```text
go build ./...                              → exit 0, output vacío (sha256 e3b0c442…7852b855)
go vet ./internal/sddattempt/ ./internal/sdd/ ./cmd/biggz/  → exit 0, output vacío
gofmt -l internal/sddattempt/ internal/sdd/ cmd/biggz/      → sin archivos
```

**Tests**: ✅ 60 paquetes `ok` / ❌ 0 / ⚠️ 0 (comando autoritativo)

```text
go test ./... -count=1 -timeout 900s
→ EXIT=0 · 83 líneas capturadas · 60 "ok" · sin FAIL, panic ni DATA RACE
→ internal/sddattempt 16.5s · internal/sdd 51.1s · cmd/biggz 131.2s
→ sha256 del output capturado: 3fc55d07630e90063d2b6d4c73ab854680ed11194ad89221a147608906086ea7
```

Suites focalizadas (todas PASS por nombre): congeladas → `TestDualBudget`, `TestRefund`, `TestRecordRejected`, `TestForInstance`, `TestRescopeGuards`, `TestRescopeNarrowWedge`, `TestRescopeCumulativeNeverReset`, `TestRescopeFiveFiveToThreeVsFive`, `TestFinish_RequestIDReplayWithoutActiveAttempt` (`sddattempt_test.go:152-184`; `:177` es el assert de replay sin mutación), `TestMachineScope_*` (5, una SKIP por portabilidad Windows documentada in-file), `TestCASContract_*` (3), `TestLegacyCompat_*` (2); nuevas/corregidas → `TestReset_OpensFreshBudgetForTheLiveObjective`, `TestSuccessorNotGatedByPredecessorExhaustedBudget` (`budget_recovery_test.go`), `TestStaleDecisionCompletedLedgerRoutesUnstranded` y `TestRemediationCorrectiveAdmittedAfterGenerationAdvance` (`internal/sdd/remediation_derive_test.go`, PASS).

**Coverage**: ➖ No disponible — esta fase no ejecutó comando de cobertura; se usó matriz escenario→test + ejecución real. Dimensión registrada como omitida.

**Ledger evidence**: work unit `verify-fresh-epoch`, token del orquestador `tok-7c12c60250520b11dc3641d3` (reutilizado; sin `acquire|reset`); settle `--request-id verify-fresh-epoch-settle-1 --outcome passed --evidence-revision sha256:3fc55d07…86ea7 --harness-disposition reused` → `complete: true`, `remaining_attempts: 2`. El `evidence_revision` de este reporte ES el hash settleado.

### Spec Compliance Matrix

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Successor Advance | Advance | `internal/sddattempt/advance_test.go > TestAdvance_DistinctWorkUnitAfterPassedObjective` | ✅ COMPLIANT |
| Successor Advance | Fresh budget | `advance_test.go > TestAdvance_FreshBudget` (1→5, `CumulativeChangedLines=0`, lifetime 40) | ✅ COMPLIANT |
| Repeated Work Unit Refusal | Repeat refused | `advance_test.go > TestAdvance_SameWorkUnitRefused` (2 subtests) + `acquire_settle_test.go > TestAcquire_BlockedWhenComplete` | ✅ COMPLIANT |
| Repeated Work Unit Refusal | Ledger intact | mismos dos tests (revision, `Complete`, record count, bytes del store sin mutar) | ✅ COMPLIANT |
| Scope-Change Guard | Any field refused | `advance_test.go > TestScopeGuard_AnyFieldRefusedAndUnchangedAdmitted` (4 subtests: work unit/evidence goal/max attempts/max lines → `invalid_continuation`) | ✅ COMPLIANT |
| Scope-Change Guard | Unchanged scope | mismo test (rama admitida sin reset; attempt 2 anexado) | ✅ COMPLIANT |
| Reset Preserves the Audit | Live state cleared | `advance_test.go > TestReset_PreservesTheAttemptChain` + `budget_recovery_test.go > TestReset_OpensFreshBudgetForTheLiveObjective` (`Complete=false`, `DecisionRequired=false`, `ActiveAttempt=0`, evidence vacía) | ✅ COMPLIANT |
| Reset Preserves the Audit | Attempts survive | mismos dos (outcome/evidence/lines de cada preservado; `AttemptsPreserved=4`) + probe CLI A (4 intentos con evidencia en bytes canónicos) | ✅ COMPLIANT |
| Reset Opens a Fresh Budget Epoch | Fresh epoch | `budget_recovery_test.go > TestReset_OpensFreshBudgetForTheLiveObjective` (generación 2; live reinicia: settle post-reset `RemainingAttempts=1`; cadena/evidencia intactas) + probe CLI A | ✅ COMPLIANT |
| Reset Opens a Fresh Budget Epoch | Same work unit admitted | mismo test (post-reset same-label acquire ADMITIDO; nunca `invalid_continuation`) + probe CLI A (token emitido) | ✅ COMPLIANT |
| Reset Opens a Fresh Budget Epoch | Successor not gated | `budget_recovery_test.go > TestSuccessorNotGatedByPredecessorExhaustedBudget` + probe CLI B (sucesor `--max-attempts 5` → `remaining_attempts: 4`) | ✅ COMPLIANT |
| Reset Opens a Fresh Budget Epoch | Never automatic | `TestReset_OpensFreshBudgetForTheLiveObjective` (tras agotar: el acquire inmediato SIGUE bloqueado `budget_exhausted`; solo el `Reset` explícito lo admite — un auto-reset habría admitido ese acquire) + code path: único call site de `Reset` es el verbo CLI `reset` (`cmd/biggz/cli_sdd.go:825`), ningún path de `Acquire/Settle/Begin/Finish` lo invoca | ✅ COMPLIANT |
| Generation Attribution | New generation | `advance_test.go > TestAdvance_GenerationAttribution` (`"generation":2`, `"objective_generation":2` en bytes) + `budget_recovery_test.go` (intentos post-reset con `ObjectiveGeneration=2`) | ✅ COMPLIANT |
| Generation Attribution | Kept generation | `TestAdvance_GenerationAttribution` (predecesor 0 = ausencia) + `legacy_compat_test.go > TestLegacyCompat_FrozenRecordAndDerivedGeneration` | ✅ COMPLIANT |
| Lifetime Accounting | Survives advance | `advance_test.go > TestAdvance_LifetimeSurvives` (`LifetimeAttempts=3`, `LifetimeChangedLines=10`) | ✅ COMPLIANT |
| Lifetime Accounting | Survives reset | `TestReset_PreservesTheAttemptChain` (1/20) + `TestReset_OpensFreshBudgetForTheLiveObjective` (lifetime no decrece; 4 intentos/1 línea) + probe CLI A (`Lifetime attempts: 4` → 6) | ✅ COMPLIANT |
| Pre-Change Ledger Compatibility | Legacy advances | `legacy_compat_test.go > TestLegacyCompat_AdvancesWithDifferentWorkUnit` + `cas_store_test.go > TestMigration_ImportsLegacyLedgerOnce` + probe CAS con fixture pre-cambio + `resets` (carga y clasifica) | ✅ COMPLIANT |
| Pre-Change Ledger Compatibility | Derived default | `TestLegacyCompat_FrozenRecordAndDerivedGeneration` + `cas_contract_test.go > TestCASContract_PreChangeRecordReCanonicalizesUnchanged` + `TestCASContract_GenerationOneRecordsStampNoNewFields` (vista 1, bytes 0) | ✅ COMPLIANT |
| Remediation Admission | Corrective admitted | `internal/sdd/remediation_derive_test.go > TestRemediationCorrectiveAdmittedAfterGenerationAdvance` + `advance_test.go > TestAdvance_ClearsLiveEvidence` | ✅ COMPLIANT |
| Remediation Admission | Remediated refused | mismo test (reclamo repetido → `invalid_continuation`) | ✅ COMPLIANT |
| No New CLI Surface | Existing surface | probe CLI A/B con la forma posicional documentada `biggz sdd-attempt acquire <change> --work-unit <otro>` (avance y reset admitidos sin verbo nuevo); `cmd/biggz/cli_sdd.go` diff = solo prints y mensaje de reset | ✅ COMPLIANT |
| No New CLI Surface | No new verb | code path: `git diff cmd/biggz/cli_sdd.go` (+8/−1) no toca el dispatch `switch operation` ni el parser de flags; `default` sigue listando los mismos 7 verbos (`status, begin, finish, reset, grant, acquire, settle`); sin test dedicado de enumeración (inspección del dispatch real + probes) | ✅ COMPLIANT |
| Interrupted Refund Capped at 2× | Interrupted with lines counts | `budget_refund_test.go > TestRefund` (congelado: `interrupted` 20 líneas → delivered 1) | ✅ COMPLIANT |
| Interrupted Refund Capped at 2× | Interrupted without lines refunded | `budget_refund_test.go > TestRefund` (congelado: `interrupted` 0 → refund-eligible) | ✅ COMPLIANT |
| Interrupted Refund Capped at 2× | Blocks after 2× cap | `budget_refund_test.go > TestRefund` (congelado: 7º acquire → `blocked(budget_exhausted)`) + probe CLI A (5º acquire bloqueado) | ✅ COMPLIANT |
| Interrupted Refund Capped at 2× | Successor starts with full refund budget | `advance_test.go > TestAdvance_SuccessorStartsWithFullRefundBudget` (predecesor agota 2×; sucesor admitido; refunds del sucesor = 0) + `TestSuccessorNotGatedByPredecessorExhaustedBudget` | ✅ COMPLIANT |

**Compliance summary**: 26/26 escenarios con evidencia de ejecución que pasa (0 UNTESTED, 0 FAILING, 0 PARTIAL). Los recuentos autoritativos 11/26 coinciden con el delta spec (10 ADDED + 1 MODIFIED; 22 + 4 escenarios), incluido el requirement nuevo `Reset Opens a Fresh Budget Epoch` con sus 4 escenarios.

### Correctness (Static Evidence)

| Requirement | Status | Notes |
|------------|--------|-------|
| Successor Advance | ✅ Implemented | `deriveScopeAdmission` (`sddattempt.go:615`) clasifica `Advance` con `Complete ∧ ¬DecisionRequired ∧ ActiveAttempt==0 ∧ lastAttemptPassed ∧ req.WorkUnit≠store.WorkUnit`; `applyScopeAdvance` (`:717`) abre la generación con presupuesto del request y `CumulativeChangedLines=0`. |
| Repeated Work Unit Refusal | ✅ Implemented | `BlockedReasonWorkUnitComplete` (`:228`) + `workUnitCompleteExit` (`:663`); mensaje exacto del design verificado carácter a carácter por tests y en ambos probes CLI. |
| Scope-Change Guard | ✅ Implemented | `scopeChangeRefusal` (`:678`): regla única sobre los 4 campos con `objectiveOpen := len(liveGenerationAttempts(store)) > 0` para los presupuestos; invocada por `Acquire` (`:1649`). Ver desviación (i). |
| Reset Preserves the Audit | ✅ Implemented | `Reset` (`:1239`) conserva `Attempts`/`Advances`; limpia solo el objetivo vivo (`Complete`, `DecisionRequired`, `ActiveAttempt`, evidencia, binding, work unit, evidence goal) — `len(Attempts)` nunca decrece. |
| Reset Opens a Fresh Budget Epoch | ✅ Implemented | `Reset` avanza `store.Generation = derivedGeneration(store)+1`, estampa `RuntimeReset.ToGeneration` (`omitempty`) y reinicia `CumulativeChangedLines=0`; `liveGenerationAttempts` (`:311`) deja a los preservados fuera del presupuesto/cap/refunds/acumulador. `budgetRecoveryHint` (`:239`) anexado a los exits de agotamiento; eliminada la cláusula muerta «or settle the obligated evidence». |
| Generation Attribution | ✅ Implemented | `Generation` (`:74`) y `RuntimeAttempt.ObjectiveGeneration` (`:127`) `omitempty`; sucesores y attempts post-reset estampan la generación vigente (`store.Generation` al anexar); predecesores quedan en ausencia (=1). |
| Lifetime Accounting | ✅ Implemented | `LifetimeAttempts=len(store.Attempts)`, `LifetimeChangedLines=lifetimeChangedLines(store)` (`:337`), `LastAdvance=lastAdvanceEntry` (`:348`) — derivados read-time en `StatusWithInstance` (`:503-508`), nunca persistidos. |
| Pre-Change Ledger Compatibility | ✅ Implemented | Gen-1 = ausencia de campo (fixtures `d1060a4c…`, `b2ac4a5f…` re-verifican content address por el load path real); `cas_store.go` fuera del diff; probe con fixture pre-cambio que ya contiene un entry `resets` legacy (ver CAS abajo). |
| Remediation Admission | ✅ Implemented | Guards de remediación intactos (ver Regresiones); `applyScopeAdvance` limpia `EvidenceRevision`/`Binding*` del objetivo vivo; la cadena preservada mantiene la falla remediable y el pre-check de `Acquire` sigue rechazando el reclamo ya remediado. |
| No New CLI Surface | ✅ Implemented | `cmd/biggz/cli_sdd.go` +8/−1: prints `Generation:`/`Lifetime attempts:`/`Lifetime lines:`, mensaje `Previous attempts preserved: %d`, y los exits de bloqueo provienen del paquete; sin verbo, flag ni HelpText nuevos. |
| Interrupted Refund Capped at 2× | ✅ Implemented | `runtimeRefundedAttempts` (`:281`) y todos los chequeos de cap (`Begin`, `Finish`, `Acquire`, `Settle`) usan `liveGenerationAttempts`; el sucesor arranca con `2×MaxAttempts` frescos. |

### Coherence (Design)

| Decision | Followed? | Notes |
|-----------|-----------|-------|
| Clasificación de la completitud (`work_unit_complete`; `corrupt_authority` solo anomalía) | ✅ Yes (con desviación ii) | `deriveScopeAdmission` + `TestCASContract_CompleteLedgerClassification` (3 anomalías → `corrupt_authority`); `internal/sdd/status.go` SÍ cambió — desviación (ii), ahora CON test. |
| Seam único de admisión (`deriveScopeAdmission`) consumido por `Acquire`, `Begin`, `StatusWithInstance` | ⚠️ Parcial (declarado) | El seam existe y lo consumen los tres; el guard de 4 campos vive en `scopeChangeRefusal` y lo invoca SOLO `Acquire` — desviación (i), aceptada por test congelado. |
| Codificación CAS (`omitempty`, gen-1 por AUSENCIA, `Lifetime*` derivado, `cas_store.go` intacto) | ✅ Yes | Verificado en código, en fixtures congelados y en un probe independiente con entry `resets` legacy. |
| Cap de reembolsos 2× por generación viva | ✅ Yes | `runtimeRefundedAttempts` + checks sobre `liveGenerationAttempts`; el delta MODIFIED corrige el «total» canónico. |
| Presupuesto del sucesor (fresco del request; `CumulativeChangedLines=0`; fallback solo si el request omite) | ✅ Yes | `applyScopeAdvance` + adopción del budget en el acquire que abre (`len(live)==0 || store.Max*==0`); verificado 1→5 y 2→5 con `remaining_attempts` en probe. |
| Mensaje de rechazo exacto (`work_unit_complete`) | ✅ Yes | `workUnitCompleteExit` reproduce el texto del design; verificado en tests y en ambos probes CLI. |
| Assets (4 superficies; mirror condicional) | ✅ Yes | Los 4 assets cambiados exactamente en los rangos previstos; `skills/sdd-verify/SKILL.md` y `internal/assets/skills/sdd-verify/SKILL.md` NO se tocaron y siguen byte-idénticos (sha256 `bf8e210f…` en ambos) → mirror por omisión. |

**Desviaciones documentadas — juicio independiente**

1. **Guard unificado invocado por `Acquire`, no dentro de `deriveScopeAdmission`**: **ACEPTABLE con reserva**. El test congelado `budget_refund_test.go:163 > TestRecordRejected` ejecuta `Begin(..., WorkUnit:"w2")` sobre un objetivo ABIERTO y espera admisión; mover el guard al seam rompería ese contrato. La permissividad de `Begin` es además comportamiento histórico, no una regresión de este change (los guards previos también vivían solo en `Acquire`). El escenario «Any field refused» del spec se ejerce por el path `acquire`. Reserva: la asimetría deja `begin` capaz de cambiar los 4 campos sobre un objetivo abierto — ver SUGGESTION S1.
2. **`internal/sdd/status.go` SÍ necesitaba cambios**: **DE ACUERDO, justificado y ahora cubierto**. `isStaleDecisionRequired` extendió la clasificación a `work_unit_complete`; sin eso, un ledger completo con obligación abierta habría quedado varado. La brecha W2 del verify previo está cerrada por `TestStaleDecisionCompletedLedgerRoutesUnstranded` (routing `Dependencies.Verify=ready`, `RemediationState` cero; y `complete+decision-required` sigue `corrupt_authority`). PASS ejecutado en esta corrida.

**Correcciones del ciclo previo — confirmadas**: (a) proyección de status `work_unit_complete` + exit que nombra al sucesor (tests + probes); (b) mensaje `Previous attempts preserved: %d` con `AttemptsPreserved` reemplazando a `AttemptsReset` (sin consumidores vivos de `attempts_reset`; verificado por grep).

### Regresiones y suites congeladas

`git diff --shortstat`: **11 archivos modificados, +945/−131** = `cmd/biggz/cli_sdd.go` (+8/−1), `cmd/biggz/sdd_attempt_grant_cli_test.go` (+99), `internal/assets/biggz/biggz-orchestrator-workflow.md` (+2/−2), `internal/assets/opencode/commands/sdd-verify.md` (+1/−1), `internal/assets/prompts/sdd/sdd-verify.md` (+2/−2), `internal/assets/skills/_shared/sdd-status-contract.md` (+7/−2), `internal/sdd/remediation_derive_test.go` (+199), `internal/sdd/status.go` (+9/−5), `internal/sddattempt/acquire_settle_test.go` (+25/−7), `internal/sddattempt/cas_store_test.go` (+157), `internal/sddattempt/sddattempt.go` (+436/−111). Untracked nuevos: `advance_test.go` (691), `cas_contract_test.go` (349), `legacy_compat_test.go` (193), `budget_recovery_test.go` (301) + `openspec/changes/fix-attempt-ledger-scope/`.

- **Suites congeladas VERDES y SIN MODIFICAR** (no aparecen en `git diff --stat`, mtimes previos al fix): `budget_refund_test.go` (1–3: `TestDualBudget`/`TestRefund`/`TestRecordRejected`), `parity_test.go` (`TestForInstance`/`TestRescopeGuards`/`TestRescopeNarrowWedge`), `rescope_test.go` (`TestRescopeCumulativeNeverReset`/`TestRescopeFiveFiveToThreeVsFive`), `sddattempt_test.go:177` (`TestFinish_RequestIDReplayWithoutActiveAttempt`), `machine_scope_test.go` (5 tests; `TestMachineScope_UnwritableFallbackFailsLoudly` SKIP por portabilidad Windows, documentado in-file), `cas_contract_test.go` y `legacy_compat_test.go` (nuevos del change; mtime 18:27/17:38, anteriores a la ronda correctiva de 19:15+ — no tocados por el fix).
- **`cas_store.go`**: diff vacío. **`Rescope`** (`sddattempt.go:2392`+): sin cambios de lógica; únicamente 2 líneas de construcción del resultado renombradas (`AttemptsReset→AttemptsPreserved: len(store.Attempts)`), presentes desde antes del fix (ya reportadas en el verify previo). **Guards de remediación** HEAD `:1316-1345` y `:1629-1633`: sin hunks del diff. **`deriveSettleObligation`** (HEAD `:415-446`, hoy `:552`): sin hunks; solo se reemplazó `deriveAdmissionBlocked` (que lo consumía) por `deriveScopeAdmission`. **`openspec/specs/**`**: intacto (`git status` limpio en esa ruta; el texto canónico "2×MaxAttempts total" de `:106` es el estado esperado hasta `sdd-sync`).
- Sin evidencia de regresión en ningún paquete: `go test ./...` verde completo.

### CAS byte guarantee

- **Generación 1 = ausencia de campo**: tests `TestCASContract_PreChangeRecordReCanonicalizesUnchanged` (fixture sin campos nuevos re-canonicaliza byte-idéntico y verifica `d1060a4c…` por el load path real) y `TestCASContract_GenerationOneRecordsStampNoNewFields` (records gen-1 de ambas familias de mutación sin `generation`/`advances`/`objective_generation`, hash canónico == content address). Scan independiente sobre los 12 records de un repo throwaway: los 6 records pre-reset tienen 0 ocurrencias de `"generation"`, `"objective_generation"`, `"advances"`, `"to_generation"`; tras el reset, `"generation":2` + `"to_generation":2` aparecen solo en los records post-reset.
- **`ToGeneration` es `omitempty`** (`RuntimeReset`, diff `:165`): confirmado en código.
- **Prueba independiente del caso más riesgoso (records pre-cambio que YA contienen `resets`)**: fixture JSON escrito a mano con un entry `resets` de forma legacy (`reason`/`reset_by`/`reset_at`/`prev_revision`, sin `to_generation`), publicado como `record-ac069386b017cfd00afd0a4bb465573989385c9985b70024d0672874456dc865.json` + HEAD en un repo git throwaway. `biggz sdd-attempt status` lo cargó verificando su content address recomputado (`Revision: ac069386…`), renderizó `Generation: 1` y clasificó `work_unit_complete` — o sea, la serialización actual de un entry `resets` legacy es byte-idéntica (un `omitempty` ausente habría añadido `"to_generation":0` y el load habría fallado closed). **Sin evidencia en contrario.**

### Fresh-epoch CLI probe (independiente, repos throwaway + `./biggz.exe`)

Binario del repo `biggz.exe` (mtime 19:22, construido después del fix 19:15). Dos repos git desechables bajo `/tmp`; nunca el ledger de este repositorio.

**Probe A — same-label fresh epoch** (`probe-same-label`, agotamiento `--max-attempts 2` = 4 intentos: 1 `failed` con evidencia + 3 `interrupted`):

```text
$ biggz sdd-attempt acquire probe-same-label --work-unit apply --max-attempts 2 --max-lines 400 --request-id probe-same-label-acq-5
error: blocked(budget_exhausted): attempt budget exhausted; decision required; run sdd-attempt reset to open a fresh budget — the preserved attempt chain and its evidence stay in the audit
EXIT=1   # la cláusula muerta "or settle the obligated evidence" ya no existe
$ biggz sdd-attempt acquire probe-same-label --work-unit verify-probe --max-attempts 2 ...   # pre-reset
error: blocked(budget_exhausted): attempt budget exhausted; decision required; run sdd-attempt reset ...   EXIT=1
$ biggz sdd-attempt status probe-same-label
Generation:        1 · Attempts: 4 · Lifetime attempts: 4 · Decision needed: true · Next action: decision-required
$ biggz sdd-attempt reset probe-same-label --reason "probe the fresh budget epoch" --request-id probe-same-label-reset
Ledger reset (revision: e19516f2…46a) · Previous attempts preserved: 4   EXIT=0
$ biggz sdd-attempt status probe-same-label
Generation:        2 · Attempts: 4 · Lifetime attempts: 4 · Decision needed: false · Next action: begin
$ biggz sdd-attempt acquire probe-same-label --work-unit apply --max-attempts 2 --max-lines 400 --request-id probe-same-label-acq-post
{"token": "tok-632a823a695b3352fd9a7935", "settle_obligation": {…}}}   EXIT=0   # ADMITIDO: epoch fresco
$ biggz sdd-attempt settle probe-same-label --token tok-632a… --outcome failed ...
{"remaining_attempts": 1}   EXIT=0   # allowance fresca: el predecesor (4 intentos) no se carga
$ biggz sdd-attempt acquire probe-same-label --work-unit apply --max-attempts 3 ...
error: blocked(invalid_continuation): attempt budget changed without reset: have 2, acquire wants 3   EXIT=1
$ biggz sdd-attempt acquire probe-same-label --work-unit apply --max-attempts 2 ... (mismo scope)
{"token": "tok-254ddfa2b4e7be9fd8b171af"}   EXIT=0
$ biggz sdd-attempt status probe-same-label
Generation: 2 · Attempts: 6 · Lifetime attempts: 6 · Blocked reason: active_attempt
```

Bytes canónicos del record HEAD: intento 1 preservado con `"outcome":"failed"` + `"evidence_revision":"sha256:ccc…"`, intentos 2-4 `"outcome":"interrupted"`, intento 5+ con `"objective_generation":2`, `"resets":[{"reason":…,"prev_revision":"1f8bef…","to_generation":2}]`, `"attempts_preserved":4` — cadena, outcomes y evidencia intactos.

**Probe B — successor not gated** (`probe-successor`, mismo agotamiento; pre-reset el sucesor fue rechazado `budget_exhausted`):

```text
$ biggz sdd-attempt reset probe-successor ... → Previous attempts preserved: 4   EXIT=0
$ biggz sdd-attempt acquire probe-successor --work-unit verify-probe --max-attempts 5 --max-lines 400 --request-id probe-successor-acq-succ
{"token": "tok-406531f0f0f5639cc2602959"}   EXIT=0   # ADMITIDO con su propio presupuesto
$ biggz sdd-attempt settle probe-successor --token tok-4065… --outcome failed --evidence-revision sha256:ddd… ...
{"remaining_attempts": 4}   EXIT=0   # 5-1: cap/presupuesto del predecesor NO heredado
$ biggz sdd-attempt status probe-successor
Generation: 2 · Attempts: 5 · Lifetime attempts: 5 · Complete: false
$ grep sobre el record HEAD: "generation":2 · "max_attempts":5 · "objective_generation":2
```

### Modern Go

`use-modern-go list --file-path internal/sddattempt/sddattempt.go` consultado (output completo leído; go.mod → go 1.25). Guideline relevantes ya aplicadas en el código del fix: `min_max` (`derivedGeneration`, `liveGenerationAttempts`), `slices_index_func` (`deriveScopeAdmission`), `any` (`requestDigest`). `json_omitzero` explicado con `explain` y desviado intencionalmente en los ints nuevos (`omitempty`, uniformidad con la familia content-addressed; para enteros los bytes resultantes son idénticos) — ver S2.

### Issues Found

**CRITICAL**: None

**WARNING**:
- **W1 — Presupuesto de review 6,5×**: la superficie autorada es de **2.610 líneas** medidas (11 trackeados +945/−131 = 1.076 + 4 tests nuevos = 1.534) contra el presupuesto de 400 líneas por PR; solo el código de producción suma 556 (`sddattempt.go` 547 + `cli_sdd.go` 9). El forecast de `tasks.md` era High (~650–850) y el real lo supera con creces. El change sigue sin slices entregados (working tree único, sin commit), así que la decisión `size:exception` explícita vs split por capas queda abierta para el mantenedor ANTES de abrir PR.
- **W2 — Delta spec sobre presupuesto**: `specs/runtime/spec.md` = **1.011 palabras** contra el «Size budget: Spec artifact MUST be under 650 words» de `sdd-spec` SKILL.md:233 (1,56×); el delta es la fuente autoritativa hasta `sdd-sync` y su tamaño encarece revisión y consumo. Recomendación: comprimir prosa/tablas al sincronizar, sin perder escenarios.

**SUGGESTION**:
- **S1 — Asimetría `begin`/`acquire`**: `begin` sigue admitiendo deriva de los 4 campos sobre un objetivo abierto (pinneado por `budget_refund_test.go:163`). Opción: un test que fije la asimetría como contrato explícito, o un `ScopeRequest.GuardScope` para que el seam aplique el guard solo al path `acquire`.
- **S2 — `omitempty` vs `omitzero`**: adoptar `json_omitzero` en los ints nuevos al sincronizar el guideline no cambiaría bytes (comportamiento idéntico en enteros) pero rompería la uniformidad con la familia content-addressed; decisión menor ya documentada y evaluada con `explain`.
- **S3 — `RuntimeStatus.CumulativeChangedLines`** ahora se publica con el valor real (antes siempre 0). Beneficio, pero un consumidor JSON externo lo verá cambiar; conviene mencionarlo en las notas del change.
- **S4 — `openspec/specs/runtime/spec.md:106`** sigue diciendo «2×MaxAttempts **total**»; es el estado esperado hasta `sdd-sync` (el delta MODIFIED es la única fuente válida mientras tanto).

### Skipped Dimensions / Honest Gaps

- **Coverage %**: no medida (sin comando de cobertura en esta fase; matriz escenario→test + ejecución real usadas como evidencia).
- **Strict TDD**: inactivo (`openspec/config.yaml:11` → `strict_tdd: false`); sin secciones TDD.
- **RED-by-reversion**: la tabla de reversión temporal de `apply-progress` es narrativa; verificar no modifica fuente, así que no se re-ejecutó. La mecánica del defecto W1 quedó corroborada por los tests de regresión y ambos probes CLI sobre el binario corregido.
- **`review-subject.json`**: NO escrito — la superficie de edición está restringida por el orquestador a `verify-report.md`; queda para el orquestador antes del gate RDD.
- **`TestMachineScope_UnwritableFallbackFailsLoudly`**: SKIP en Windows (simulación read-only no portable, documentado in-file); archivo congelado sin cambios.
- **UNPROVEN**: **ninguno**. Las 11 requirements y los 26 escenarios tienen evidencia de ejecución que pasa, o un code path verificable directamente (los dos escenarios de superficie CLI y el «Never automatic», explicados en la matriz). Ningún escenario queda solo con narrativa.

### Delivery Risk

Change multi-slice (~2.610 líneas autoradas, 1.534 de ellas tests nuevos) contra un presupuesto de revisión de 400 líneas/PR: 6,5×. Recomendación para el mantenedor antes de abrir PR: (a) `size:exception` explícita para el PR semántico, o (b) split por capas (campos/seam; avance/reset/cap; assets/CLI) aceptando que 2 de 3 PRs queden por encima de 400 por sus tests. Ningún slice se entregó todavía; el árbol está sin commit.

### Verdict

**PASS WITH WARNINGS**
26/26 escenarios verificados con ejecución real (0 UNPROVEN, 0 FAILING) y suite autoritativa verde (`go test ./... -count=1 -timeout 900s`, EXIT=0, 60 paquetes ok, hash `3fc55d07…` conforme al hash settleado); regresión W1 arreglada y probada end-to-end con probe CLI independiente (same-label admitido con allowance fresca, sucesor no gateado, cadena/evidencia/lifetime preservados); W2 previo cerrado con test; CAS byte-stable verificado incluso para records pre-cambio con `resets`. Design implementado con las dos desviaciones documentadas — ambas juzgadas aceptables (guard en `Acquire` pinneado por test congelado; `status.go` justificado y ahora testeado). Quedan 2 warnings materiales (presupuesto de review 6,5×; delta spec 1,56×) y 4 sugerencias, ninguno bloqueante.
