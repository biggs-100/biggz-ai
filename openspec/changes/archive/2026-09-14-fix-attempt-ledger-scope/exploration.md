# Exploration: fix-attempt-ledger-scope

> **Change:** `fix-attempt-ledger-scope` · **Repo:** biggz-ai (Go, `github.com/biggs-100/biggz-ai`) · **Fase:** explore
> Todo lo que sigue se leyó del código en HEAD; cada afirmación lleva `path:line`. El transcript de la probe (acquire/settle/reset con binario real) se toma como hecho dado, no se reproduce aquí.

## 1. Estado actual

El attempt ledger vive en `internal/sddattempt/`: núcleo `sddattempt.go` y CAS en `cas_store.go`.

- Raíces: clone scope `<git-common-dir>/biggz/sdd-runtime/v1/<change>/`, machine scope `<home>/.biggz/sdd-runtime-nogit/v1/<change>/` cuando no hay repo git (`cas_store.go:244-281`, `:282-315`, `:327-337`).
- Layout CAS: `HEAD` + `record-<sha256>.json` (`cas_store.go:554-557`, `:560-591`); cada mutación lee HEAD, carga el record, muta en memoria y publica un record nuevo con `commit` (`cas_store.go:394-448`).
- La revisión es el sha256 del payload canónico: se excluye el campo `Revision` (`json:"-"`) y las revisiones embebidas en receipts de requests (`cas_store.go:521-553`, `:606-610`). `loadRecord` re-verifica la dirección de contenido (`cas_store.go:489-510`).
- Migración legacy: `~/.biggz/sdd-runtime/v1/<change>.json` se importa en el primer acceso, con self-check de revisión y fail-closed (`cas_store.go:339-391`, `sddattempt.go:2016-2026`); el archivo viejo queda intacto.

## 2. A — Máquina de estados

Campos que definen el estado (`RuntimeStore`, `sddattempt.go:44-100`):

| Campo | Línea | Significado |
|---|---|---|
| `ActiveAttempt` | `:54` | ordinal del attempt abierto (0 = ninguno) |
| `DecisionRequired` | `:55` | presupuesto agotado; requiere decisión humana |
| `Complete` | `:56` | generación cerrada |
| `NextAction` | `:57` | pista de proyección; el comment documenta `"begin"\|"continue"\|"finish"\|"complete"\|""` pero también se escribe `"decision-required"` (ver abajo) |
| `ObjectiveID` | `:60` | identificador del objetivo (solo informativo en la práctica, ver B) |
| `WorkUnit` | `:66` | etiqueta de la generación (ver B) |

Por attempt (`RuntimeAttempt`): `WorkUnit` (`:106`), `Outcome` (`:115`), `RemediatesEvidenceRevision` (`:131`).

Escritores de estado:

| Transición | Efecto | Línea |
|---|---|---|
| `Finish(passed)` | `Complete=true`, `NextAction="complete"`, `ActiveAttempt=0` | `sddattempt.go:857-860` |
| `Finish(failed\|interrupted)` | `ActiveAttempt=0`; `DecisionRequired=true` + `"decision-required"` si `delivered>=Max` o `len>=2*Max`, si no `"begin"` | `:861-873` |
| `Settle(passed)` | `Complete=true`, `NextAction="complete"`, `ActiveAttempt=0` | `:1660-1663` |
| `Settle(failed\|interrupted\|progress)` | `DecisionRequired` o `NextAction="begin"`, `ActiveAttempt=0` | `:1664-1672` |
| `Acquire` (ledger fresco) | `ActiveAttempt=ordinal`, `NextAction="continue"` | `:1243-1245` |
| `Acquire` (continuación) | `ActiveAttempt=ordinal`, `NextAction="continue"`, `DecisionRequired=false`, `Complete=false` | `:1413-1417` |
| `Acquire` (budget agotado) | persiste `DecisionRequired=true` + `"decision-required"` y bloquea | `:1355-1364` |
| `Begin` (budget agotado) | ídem | `:678-686` |
| `Reset` | limpia `ActiveAttempt`, `DecisionRequired`, `Complete`, `NextAction="begin"`, `Attempts=nil` | `:1015-1024` |
| `Rescope` | `NextAction="begin"`, `DecisionRequired=false`, `Complete=false` | `:2166-2168` |

`NextAction` derivado en lectura: `deriveNextAction` (`:390-413`) con precedencia `Complete→"complete"` (`:391`), `DecisionRequired→"decision-required"` (`:394`), intent abierto→`"continue"`/`"finish"` (`:397-409`), si no `"begin"` (`:411`).

Los dos sitios de `store.Complete = true`:
- `:857-859` pertenece a **`Finish`** — alcanzable por CLI diagnóstica (`cmd/biggz/cli_sdd.go:779`), que el contrato del workflow degrada a "diagnostic" (`internal/assets/biggz/biggz-orchestrator-workflow.md:187`).
- `:1660-1663` pertenece a **`Settle`** — el camino obligatorio (`biggz-orchestrator-workflow.md:184-187`); es el que reproduce la probe.

**Hallazgo estructural:** `Complete=true` es terminal en la práctica. `Acquire` bloquea (`:1312-1313` vía `:453-455`), `Rescope` se niega (`:2121-2123`), y `Begin` no lo consulta pero tampoco lo limpia (`:576-731`, sin escritura de `Complete`). Los únicos limpiadores (`Reset :1018`, `Acquire :1417`, `Rescope :2168`) son inalcanzables o inútiles cuando `Complete=true`: `:1417` solo se ejecuta si la admisión no bloqueó, y `:2168` solo si el guard `:2121` no rechazó. **La única salida de una generación completa es `Reset`, que borra el audit (`:1018-1020`).** Esto explica los 5 resets del maintainer.

## 3. B — Work-unit como identidad vs. objetivo

- `store.WorkUnit` se escribe en: acquire fresco (`:1239`), acquire de continuación (`:1425-1427`), rescope (`:2159-2161`); en `Begin` solo en la creación (`:607`) — la continuación de `Begin` agrega el label al attempt (`:703-708`) pero no actualiza el store.
- **Guard 1 — `invalid_continuation`:** `sddattempt.go:1392-1401`. Condición: `store.WorkUnit != "" && params.WorkUnit != "" && store.WorkUnit != params.WorkUnit`. Exit: `work unit scope changed without reset: have %q, acquire wants %q`. Solo lo posee `Acquire`; `Begin`/`Finish` no tienen guard equivalente.
- Guard gemelo sobre el goal: `:1402-1410` (`evidence goal changed without reset`). Avanzar de slice cambia label Y goal, así que ambos guards aplican.
- **Guard 2 — `corrupt_authority`:** `deriveAdmissionBlocked` (`:448-476`), rama `if store.Complete` → `BlockedReasonCorruptAuthority` (`:179`) con exit `ledger is complete; reset required to continue` (`:453-455`). Se invoca desde `Acquire` (`:1312-1313`) y desde la proyección de status (`:369-372`).
- `progress` cierra el attempt sin completar el ledger (`HelpText`, entrada `--outcome ... progress`: "non-terminal checkpoint ... so one scope can cover N work units with a single final passed settle"), pero **no limpia `store.WorkUnit`**, así que el siguiente acquire con otro label sigue chocando con el guard de `:1395`. La probe lo confirma.
- `ObjectiveID`: campo del store (`:60`) y del attempt (`:105`). Se escribe solo en `Begin` (`:604` creación, `:615` attempt) y en `Reset` (`:963`, `:1028-1029`). **`AcquireParams` no tiene `ObjectiveID`** (`:1068-1084`): el flujo acquire/settle nunca lo setea. `Rescope` exige `ObjectiveID != ""` (`:2115`) y además presupuestos estrictamente crecientes (`:2148-2155` "Widened"/"Exhausted"), por lo que en el flujo bounded `Rescope` es inalcanzable.
- Secuencia de unidades dentro de un objetivo: **no existe**. `Attempts` es un log lineal que registra el label de cada attempt (`:1431-1435`) — historia, no admisión. No hay índice de unidad, cola, ni campo de "siguiente unidad".

## 4. C — Consumidores de `Complete`

Lecturas del flag (grep repo completo de `.Complete`; solo sitios de producción):

| Sitio | Consecuencia si `true` |
|---|---|
| `deriveNextAction` `sddattempt.go:391-393` | `NextAction="complete"` (precede a decision-required/active) |
| `StatusWithInstance` `sddattempt.go:355` | `RuntimeStatus.Complete=true`; CLI imprime `Complete: true` (`cmd/biggz/cli_sdd.go:688`) |
| `deriveAdmissionBlocked` `sddattempt.go:453-455` | `blocked(corrupt_authority): ledger is complete; reset required to continue`; todo `Acquire` falla (`:1312-1313`) y el status proyecta `BlockedReason`/`BlockedExit` (`:369-372`) |
| `Rescope` `sddattempt.go:2121-2123` | `ErrRuntimeRescopeNotAllowed: rescope blocked when complete` |
| `BindApprovedReview` `internal/sdd/binding.go:21-24` | `change %q is already complete, cannot bind review` |
| `isStaleDecisionRequired` `internal/sdd/status.go:1255-1268` | Clasifica `corrupt_authority` como "stale decision" **solo si** `SettleObligation != nil` (`:1267-1268`); con obligación libera verify/archive (`status.go:720-726`, `:748-758`, `applyStaleDecisionRouting :1270-1286`) |
| `SettleResult.Complete` (`sddattempt.go:1119-1135`, JSON `complete`) | El orquestador enruta por `proceed`/`blocked`/`complete` (`biggz-orchestrator-workflow.md:187`; `internal/assets/skills/_shared/sdd-status-contract.md:86-93`) |
| Assets | `internal/assets/opencode/commands/sdd-verify.md:22` exige "an active or complete attempt"; `sdd-archive.md:20` exige ledger entry |

Sin lectores: RDD gates y `internal/sdd` no consultan `Complete` directamente (solo vía `corrupt_authority`); `sddattempt.isStaleDecisionRequired` (`:476-495`) no tiene referencias de producción (grep). `RemediationComplete` **no lee `Complete`** (ver D).

## 5. D — Acoplamiento de remediation (riesgo máximo)

Predicados:

- Obligación: `deriveSettleObligation` (`:415-446`) devuelve el último attempt `failed` con `EvidenceRevision` no remediado; "remediado" = existe un posterior con `Outcome=="passed" && RemediatesEvidenceRevision == failed.EvidenceRevision` (`:428-433`). El par derivado es `{EvidenceRevision: X, RemediatesEvidenceRevision: X}`.
- Admisión de un acquire correctivo: `:1316-1345` exige que la cadena mantenga ESE fallo sin remediar; si no, `invalid_continuation` (`:1339-1344`). En ledger fresco con `--remediates` → insatisfacible (`:1253-1262`). **No depende de ningún `passed` previo.**
- Guard de settle: `params.Outcome == "passed" && chainHasFailedEvidence && params.RemediatesEvidenceRevision == ""` → `invalid_continuation` "passing correction ... requires --remediates-evidence-revision" (`:1629-1633`).
- `RemediationComplete` (`:529-542`): `last.Outcome == "passed" && last.RemediatesEvidenceRevision == evidenceRevision`; pura proyección sobre la cadena, sin leer `store.Complete`. Consumido en `internal/sdd/status.go:677` y `internal/sdd/engram_status.go:324-325`.

Respuesta precisa a la pregunta: **la admisión de remediation depende únicamente de que la evidencia fallida siga presente y sin remediar en la cadena; jamás de un `passed` previo.** `Complete` solo la bloquea de forma genérica y anticipada vía `corrupt_authority` (`:453-455`).

Qué rompería si un `passed` settle dejara de setear `Complete=true` incondicionalmente:
1. No rompe remediation: el flujo correctivo vive tras settles `failed`, estado en el que `Complete` ya era `false`; los guards `:1316-1345` y `:1629-1633` no cambian y deben permanecer intactos.
2. Rompe las aserciones actuales que fijan el cierre implícito: `acquire_settle_test.go:69`, `:176-223` (`TestAcquire_BlockedWhenComplete`), `:328`; `cas_store_test.go:80`, `:118`, `:227`, `:346`, `:354`; `parity_test.go:133`, `:221`; `sddattempt_test.go:177`.
3. Rompe contratos de assets que asumen cierre: `sdd-verify.md:22` ("active or complete") y la semántica de `staleDecision` que usa `corrupt_authority` como señal.
4. Deja sin definición el cierre del change: hoy no existe ninguna señal terminal explícita; si `passed` deja de cerrar y no se añade otra, un ledger tras la última slice queda `NextAction="begin"`, sin bloqueo, y nada lo cierra.

Conclusión: el fix es seguro **solo si** el cierre pasa a ser una señal explícita y explicable (nunca "passage silencioso = generación abierta para siempre"), y si los guards de remediation (`:1316-1345`, `:1629-1633`) y la derivación de obligación (`:415-446`) no se tocan.

## 6. E — Quién provee `--work-unit` en el flujo SDD real

- Mandato del orquestador: `biggz-orchestrator-workflow.md:184` — `acquire ... --work-unit <label> --evidence-goal <goal> ...`; `:187` — `status|begin|finish|reset` son diagnósticas y `reset` nunca es automático. El label es un placeholder sin regla de granularidad.
- Verify: `internal/assets/prompts/sdd/sdd-verify.md:44` y `:82` fijan literal `--work-unit verify`; `internal/assets/skills/sdd-verify/SKILL.md:36` exige bracket acquire→settle (o reutilizar el token del orquestador sin re-adquirir).
- Apply: `internal/assets/prompts/sdd/sdd-apply.md` no contiene ninguna instrucción de ledger (el acquire lo hace el orquestador antes de lanzar, por `:184`). Sus menciones de "work unit" (`:92`, `:149-159`, `:248-249`) son el concepto de slice de tareas/PR, no el label del ledger.
- Contrato compartido: `internal/assets/skills/_shared/sdd-status-contract.md:86-93` exige el bracket acquire/settle y que ante `settle_obligation` se relaye verbatim (`:229`), sin definir granularidad.

Conclusión: **ningún asset define si el label es por slice o por change**. El ledger lo trata como identidad de scope inmutable para toda la generación (`:1395-1401`), mientras el `HelpText` vende `progress` como la forma de cubrir "N work units" dentro de un scope — N unidades invisibles bajo un mismo label (test `TestSettle_ProgressMultiUnitSingleFinalSettle`, `acquire_settle_test.go:274-331`, usa el mismo label `"units-multi"` en ambos acquires).

## 7. F — Persistencia y compatibilidad

- Contenido direccionado: cada record es inmutable; la revisión cubre el JSON canónico completo del `RuntimeStore` (menos `Revision` y las revisiones embebidas de receipts, `cas_store.go:521-553`). La cadena de revisiones es el historial de records en el directorio; HEAD nombra el actual (`:560-591`).
- Regla de oro para tocar el esquema: cualquier campo nuevo debe ser `omitempty` y con valor cero semánticamente igual al viejo. `loadRecord` re-canonicaliza el record parseado y falla con "does not match its content address" (`cas_store.go:503-508`) si los bytes canónicos cambian — un campo no-omitempty o un default aplicado en lectura invalidaría **todos** los records existentes.
- Cambiar la *regla* de completitud (derivaciones, guards) no invalida records: los históricos se siguen leyendo. Pero un ledger viejo con `"complete": true` seguirá siendo terminal (no hay ruta de reapertura) hasta que se decida lo contrario en la fase de diseño.
- Migración legacy: los tests `TestMigration_*` (`cas_store_test.go:40-160`) fijan que el estado legacy (incluido `Complete=true`, `:80`) se preserve y que un ledger roto no migre (`:122-160`).

## 8. G — Blast radius

Fuente (esperada):
- `internal/sddattempt/sddattempt.go` — máquina de estados, guards `:1392-1410`, rama de completitud, `deriveAdmissionBlocked`, `deriveSettleObligation`, `HelpText`.
- `internal/sddattempt/cas_store.go` — solo si cambia el esquema persistido (regla omitempty).
- `cmd/biggz/cli_sdd.go` — print de status `:676-700`, rama Finish `:805`, mensajes de settle/acquire.
- `internal/sdd/status.go` (`:1255-1268`, `:677`), `internal/sdd/binding.go:21-24`, `internal/sdd/engram_status.go:324-325` — si cambia la clasificación de `corrupt_authority` o el cierre.

Assets/prompts:
- `internal/assets/biggz/biggz-orchestrator-workflow.md:184-187`
- `internal/assets/skills/_shared/sdd-status-contract.md:86-93`, `:229`
- `internal/assets/skills/sdd-verify/SKILL.md:36`; `internal/assets/prompts/sdd/sdd-verify.md:44`, `:82`
- `internal/assets/opencode/commands/sdd-verify.md:22`; `internal/assets/opencode/commands/sdd-archive.md:20`

Specs canónicas:
- `openspec/specs/runtime/spec.md:60-78` (Ledger Verify-Before-Commit CAS), `:81-105` (Dual Budget Single Owner), `:107-130` (Interrupted Refund 2×), `:166-183` (**Admissible Settle**).
- `openspec/specs/sdd/spec.md:164-170` (**REQ-G2-01**) y `:179-186` (**REQ-G2-02**): mencionan `!Complete` únicamente como precondición de rescope.
- Grep de `work-unit|work_unit|corrupt_authority|remaining_attempts` en `openspec/specs/**`: **sin coincidencias**. No existe requisito canónico que documente la transición de completitud, el mapeo `corrupt_authority`, ni la identidad de scope del work-unit → hay que añadir REQs nuevos.

Tests que fijan la semántica ACTUAL (deben actualizarse conscientemente):
- `internal/sddattempt/acquire_settle_test.go` — `:69` (`settle.Complete == true`), `:176-223` (`TestAcquire_BlockedWhenComplete`: passed→complete→`corrupt_authority`), `:225-273` (budget + obligation), `:274-331` (`TestSettle_ProgressMultiUnitSingleFinalSettle`: `:304` mid.Complete false, `:308` estado open/begin/unblocked, `:328` final.Complete true), `:335-395` (token admissible/strict), `:424-460` (non-active admite con warning).
- `internal/sddattempt/cas_store_test.go` — `:80`, `:118`, `:227`, `:346`, `:354` (status/result `Complete`).
- `internal/sddattempt/parity_test.go` — `:128-141` (Finish passed→Complete; rescope completo NotAllowed), `:221`.
- `internal/sddattempt/sddattempt_test.go:177` (replay `Complete`).
- `internal/sddattempt/rescope_test.go` (guards de rescope).
- `internal/sdd/remediation_derive_test.go:57-80`, `:83-140` (Begin/Finish correctivo y reason de remediation).
- **Sin test**: el mensaje `work unit scope changed without reset` (`:1395-1401`) no está cubierto por ningún test (grep sin coincidencias). El cambio debe añadir cobertura de avance de slices.

## 9. H — Preguntas abiertas (sin decidir)

1. **¿Completitud explícita o consecuencia implícita de `passed`?** Hoy: implícita (`:1660-1663`). Tradeoff observado: implícita fuerza 1 label por generación → N resets que destruyen el audit (5 en una sesión); explícita preserva el audit pero exige que el orquestador sepa cuál es la última slice (nueva señal tipo `--complete`/`settle --close`), y hoy ningún asset lo sabe.
2. **¿Work-unit como identidad o como unidad secuenciada?** Opciones: mover `WorkUnit` fuera de la identidad de scope (índice/cola de unidades en el objetivo); relajar el guard `:1395` a solo "no cambiar mientras hay attempt activo"; o dejar el guard pero permitir que `progress` avance el label. Compatibilidad: `store.WorkUnit` está persistido — el cambio de significado afecta admisión de ledgers viejos.
3. **¿Cómo mantener `corrupt_authority` significativo?** Hoy significa a la vez "generación completa" y "estado corrupto real" (`:453-455`), y es consumido como string en `internal/sdd/status.go:1263` y en el contrato (`sdd-status-contract.md:229`). ¿Nuevo `BlockedReason` propio para completitud (p. ej. `complete`) manteniendo `corrupt_authority` para corrupción genuina? Impacto en wire/story de assets.
4. **¿Compatibilidad hacia atrás?** Los records son inmutables (`cas_store.go:489-510`): los ledgers con `complete:true` existentes seguirán terminales salvo que se diseñe una ruta de reapertura explícita. ¿Se resignan como terminales (más simple) o se migran?
5. **¿Presupuesto por slice?** `progress` consume 1 delivered attempt (`runtimeAttemptDeliveredIncrement`, `:209-232`); con `MaxAttempts=3` (default y usado en la probe) una generación de 4 slices entra en `DecisionRequired`. ¿El modelo es N slices ≤ MaxAttempts o presupuesto por objetivo?
6. **¿Y el guard de `evidence-goal` (`:1402-1410`)?** Avanzar de slice también cambia el goal; cualquier diseño que solo toque el guard de `WorkUnit` seguirá bloqueando.

## 10. Approaches (orientativo, el propose decide)

| Approach | Pros | Cons | Effort |
|---|---|---|---|
| 1. Señal de cierre explícita, guard de scope intacto | Cambio mínimo; conserva `corrupt_authority` para estados raros; assets "active or complete" siguen funcionando si el cierre es real | Sigue 1 label por generación; no resuelve slices con evidencia propia | Low |
| 2. Units secuenciadas dentro del objetivo (nuevo campo + guard monotónico) | Modela la realidad; N evidencias preservadas y auditables | Esquema persistido (regla omitempty), guards y tests; requiere la señal de cierre del approach 1 igualmente | High |
| 3. Híbrido: cierre explícito + guard relajado a "no retroceder" (nuevo label permitido solo sin attempt activo y último terminal) | Resuelve el caso reproduce sin inflar presupuestos; mantiene `invalid_continuation` para retrocesos | Diseño de la regla de avance + compat de `store.WorkUnit` | Medium |

## 11. Ready for Proposal

Sí. El mapa de estados, guards, consumidores y el análisis de remediation (D) están establecidos con evidencia `path:line`; las decisiones de diseño quedan explicitadas como preguntas abiertas (H) para `sdd-propose`.

## 12. Upstream reference: gentle-ai and gentle-pi

**Método**: lectura read-only de ambos clones; los tests upstream se tratan como especificación. Rutas relativas: `GA/` = `C:/Users/USER/Desktop/herramientas/gentle-ai`, `GP/` = `.../gentle-pi`, `BA/` = biggz-ai. Afirmaciones de ausencia verificadas con búsquedas explícitas: `Advance|Lifetime|Handoff|untracked|Consent` en `BA/internal/sddattempt`, `handoff|rescope|advance|repair` en `BA/cmd/biggz/cli_sdd.go`, `sdd-attempt close` en ambos clones.

### 12.1 Objetivo vs work unit (Q1)

- Upstream modela `RuntimeObjective` como entidad persistida (`GA/internal/sddstatus/runtime_ledger.go:238-266`): `ID`, `Generation`, `WorkUnit`, `EvidenceGoal`, `InitialCandidateIdentity/Tree`, `MaxAttempts`, `MaxChangedLines`, `PredecessorObjectiveID/Generation`, `Relation`, `MaxChangedLinesSource`.
- NO existe "objetivo que contiene una secuencia de work units": el objetivo ES el scope de UN work unit; la secuencia de work units del change vive como generaciones sucesivas de objetivos (`generation+1`).
- Cada intento denormaliza la identidad del objetivo: `RuntimeAttempt.ObjectiveID` y `ObjectiveGeneration` (`GA/...:283-285`), porque "a superseded objective vanishes from RuntimeStatus" (`:247-250`).
- Enforcement: `runtimeObjectiveID(change, workUnit, evidenceGoal, candidateIdentity, generation)` deriva el ID (`:1046-1050`); el replay exige igualdad de ID/generation/WorkUnit/EvidenceGoal/budgets al continuar y deriva identidad nueva al abrir (`applyRuntimeBeginEvent` `:2402-2448`).
- Test de propiedad: `BeginAttemptRequest` es el ÚNICO dueño del scope de work unit; `CompactAcquireRequest` debe ser un embed, nunca un struct paralelo (`GA/internal/sddstatus/runtime_objective_owner_test.go:29-52`).

### 12.2 Avance de work units (Q2)

- Predicado de admisión: `runtimeBeginAdmission` (`GA/...runtime_admission.go:72-156`). Con `status.Complete` (`:83`): si `runtimeObjectiveAdvanceAdmissible` → `advancing = true` (`:84-88`, comment "When the request names a distinct work unit, the ordinary continuation is the successor objective"); si no → `runtimeObjectiveCompleteRefusal` (`:89`).
- `runtimeObjectiveAdvanceAdmissible` (`GA/...runtime_ledger.go:1695-1710`) exige: Complete ∧ ¬DecisionRequired ∧ sin intento activo ∧ objetivo ≠ nil ∧ intentos > 0 ∧ `request.WorkUnit != status.Objective.WorkUnit` (`:1700`) ∧ último intento de este objetivo con outcome `passed`, sin changed-line budget excedido y con finish candidate registrado.
- Repeated begin tras passed: `ErrRuntimeObjectiveDone` (`:88-94`) — "SDD runtime objective is complete; it continues through a successor objective, so run `gentle-ai sdd-attempt acquire ... --work-unit \"<a different label>\" ...` with a different --work-unit (rescope applies only to an objective that is not complete)". Wrapper `:1492-1499`: "it passed within budget, so this change continues through a SUCCESSOR objective, not a repeat of this one".
- Twin de scope: NO hay guards separados ni sentinel `invalid_continuation`; `runtimeObjectiveScopeChanged` (`GA/...runtime_admission.go:160-167`) compara en UN predicado WorkUnit ∨ EvidenceGoal ∨ MaxAttempts ∨ MaxChangedLines, evaluado en las dos ramas de continuación (`:102`, `:139`) → `ErrRuntimeObjectiveChange` = "SDD runtime objective changed without an explicit reset" (`GA/...runtime_ledger.go:87`), envuelto por `runtimeObjectiveChangeRefusal` (`:1601-1627`) que nombra reset/rescope/begin según el estado.
- Avance real: UN registro `runtimeOperationAdvance` = `"objective/advance"` (`:45`, `:1063-1069`); `RuntimeAdvance` (`:353-368`) guarda `PreviousObjectiveID/Generation/WorkUnit/EvidenceRevision`; el replay abre la generación sucesora conservando los intentos previos (`:2648-2691`); invariante de forma: el begin del advance debe nombrar work unit distinto del anterior (`:2945-2951`).
- Por caso (tests como spec, `GA/...runtime_objective_advance_test.go`): repeated (mismo label, incluso evidence-goal re-formulado) → `ErrRuntimeObjectiveDone`, estado intacto, cero registros (`:139-166`, `:130-137`); advance (label distinto) → objetivo generation 2 con presupuesto PROPIO, `CumulativeAttempts==1`, `LifetimeAttempts==2`, `Attempts[0]` intacto (passed, label y evidence originales), 1 registro, replay idempotente (`:68-116`); compact acquire → `proceed` con token (`:180-206`) y `complete` sin token para el label ya settled (`:208-226`); backwards (label viejo con otro objetivo vivo) → `ErrRuntimeObjectiveChange`, nunca reabre vía begin.
- Superficie compacta: `ErrRuntimeObjectiveDone` → `state: complete` con exit que nombra el sucesor (`GA/...runtime_compact.go:596-597`); `ErrRuntimeObjectiveChange` → `blocked(maintainer_decision)` (`:598-599`).

### 12.3 Completion (Q3)

- `Complete = true` es consecuencia del settle `passed` sin presupuesto excedido: `applyRuntimeFinishEvent` (`GA/...runtime_ledger.go:2769-2773`), con `NextAction = RuntimeActionComplete`. No hay verbo/flag separado — mismo disparador que BA (que además no evalúa budget en ese branch; matiz menor).
- Terminal SÓLO para el scope: "A passed objective terminates its own scope, not the change" (`GA/...runtime_admission.go:70-76`).
- Complete BLOQUEA: begin con el mismo work unit (`ErrRuntimeObjectiveDone`), advance con DecisionRequired (`ErrRuntimeBudgetExhausted`, test `:169-190`), rescope (`runtime_ledger.go:1845-1849` → `ErrRuntimeObjectiveDone`).
- Complete NO bloquea: begin/acquire con work unit distinto (registro advance), y `Reset` (`runtimeObjectiveResetAdmissible` devuelve true con Complete/DecisionRequired, `:1640`); el refusal de complete nombra el reset como salida para descartar el scope (`:1492-1499`).
- Paths exactos tras un objetivo completo: (a) `begin|acquire` con `--work-unit` distinto → `objective/advance`; (b) `reset --reason --actor` y nuevo begin; (c) rescope NO (refused).

### 12.4 Granularidad de presupuesto (Q4)

- Presupuesto por OBJETIVO: `RuntimeObjective.MaxAttempts/MaxChangedLines` (`GA/...runtime_ledger.go:244-245`); `RuntimeStatus` proyecta cumulativos de la generación viva más `LifetimeAttempts/LifetimeChangedLines` a nivel change, nunca reembolsados (`:432-435`).
- El sucesor abre presupuesto fresco desde SU request ("The successor opens a fresh per-objective budget", `runtime_admission.go:150-152`; test "successor inherited the predecessor budget" falla, `:93-100`), con `Lifetime*` conservado (`:101-104`).
- Consume presupuesto: una llamada begin/acquire (+1 `CumulativeAttempts`, `runtime_ledger.go:2481-2483`). Reembolsos: interrupted con incremento entregado (#3815) y failed+invalidated+cero-líneas con marcador persistido `InvalidatedWithoutDelivery` (#3152), cap de reembolsos = MaxAttempts por objetivo → gasto máximo 2x (`:2790-2855`; tests `GA/...runtime_budget_granularity_test.go:54-110,143-170`).
- Consent envelope: se dispara cuando `AdmissionStatus` deriva `blocked(maintainer_decision)` (`GA/...runtime_admission.go:181-215`). Textos (`GA/...budget_consent.go:92-107`): headline "This work unit has spent its attempt budget." (variante harness "This work unit spent its attempt budget without ever running your work."), reason "the objective's bounded attempts are gone, so no further work unit can open until a maintainer decides.", value "Resetting opens a fresh bounded budget for this objective and keeps every recorded attempt in the immutable chain." Vocabulario mixto deliberado: presupuesto del objetivo, work unit como etiqueta de su scope.

### 12.5 Reset / reopen / repair (Q5)

- Reset upstream limpia objetivo, cumulativos, evidence y completion, pero CONSERVA la cadena de intentos (`GA/...runtime_ledger.go:2361-2386`); test: tras reset + begin nuevo, `len(third.Attempts) == 3` y los dos primeros conservan `oldObjectiveID` + `LifetimeAttempts` (`GA/...runtime_ledger_reset_test.go:70-95`). Reset se niega además para presupuesto intacto sin drift (`:1765-1774`, `ErrRuntimeResetNotAllowed`).
- Continuaciones que preservan intentos: `objective/advance` (tras passed, presupuesto fresco) y `objective/rescope` (narrowing zero-drift que arrastra cumulativos SIN cambiarlos, `:369-397`, `:1829-1970`). `repair` NO es recuperación genérica: sólo el defecto del escritor consecutive-rescope (`:545-551`).
- BA: `Reset` ejecuta `store.Attempts = nil // Clear attempt history` (`BA/internal/sddattempt/sddattempt.go:1020`). BA SÍ tiene `Rescope` en librería con semántica cumulative-preserving (`BA/...:2043-2170`, `BA/internal/sddattempt/cas_store.go:2-3`, `rescope_test.go:31-58`) pero SIN verbo CLI; no tiene Advance/Handoff/Lifetime (grep sin coincidencias).

### 12.6 CLI diff (Q6)

Gentle-ai (`GA/internal/cli/sdd_attempt.go:312-440`) vs biggz-ai (`BA/internal/sddattempt/sddattempt.go:2215-2262` HelpText; dispatcher `BA/cmd/biggz/cli_sdd.go:489+`):

| Item | gentle-ai | biggz-ai | Estado |
|---|---|---|---|
| status | + `--work-unit/--evidence-goal/--max-attempts/--max-changed-lines` opcionales = veredicto de acquire (`:312-324`) | `status <change> [--change-instance]` | falta el veredicto |
| begin/finish | existen; "diagnostic/compatibility surfaces" (assets `:19`) | ruta normal | equivalente |
| acquire/settle | ruta primaria acotada (`:393-429`) | sí | equivalente |
| outcomes | failed\|interrupted\|passed | finish igual; settle añade `progress` (`BA/...:1461-1465`) | BA-extra |
| handoff | sí (`:352-357`) | ABSENT (sin verbo ni flag) | absent |
| reset | `--reason --actor` + `--objective-relation remediation\|independent` (`:358-365`) | `--reason --reset-by`, `--objective-id` opcional; sin relation | shape distinto |
| rescope | verbo con narrowing + relation + untracked (`:366-385`) | librería sin CLI | absent en CLI |
| repair | sí (`:386-392`) | ABSENT | absent |
| grant | sí (`:430-439`) | sí | equivalente |
| flags comunes | `--cwd/--change` requeridos; `--max-changed-lines` | `<change>` posicional, cwd=os.Getwd(); `--max-lines` | shape distinto |
| untracked | `--untracked-scope/--expected-untracked-inventory/--intended-untracked` | ABSENT (help + paquete sin coincidencias) | absent |
| BA-extras sin upstream | — | `--objective-id`, `--reset-by`, `--binding-revision`, `--binding-lineage`, `--strict` | BA-extra |

Sin verbo `close` en ninguno de los dos (búsqueda explícita).

### 12.7 Qué se les dice a los agentes (Q7)

- Granularidad = por continuación runtime-bearing de fase, no por change ni por task: "Use the provider-owned Git-common-dir runtime ledger for every runtime-bearing `sdd-apply`, `sdd-verify`, or remediation continuation" (`GA/internal/assets/skills/_shared/sdd-orchestrator-sections.md:13`; gemelo `GA/internal/assets/claude/sdd-orchestrator-workflow.md:98`).
- El label viaja como `acquire ... --work-unit <label> --evidence-goal <goal> ...` y el mismo work unit se re-adquiere con el `--token` del padre: "A matching token proves the actor is continuing that SAME attempt" (`workflow.md:99`; `GA/internal/assets/skills/_shared/sdd-status-contract.md:22-23`).
- "Full `status|begin|finish|reset` operations are diagnostic/compatibility surfaces; reset requires an explicit maintainer scope decision and is never automatic" (`orchestrator-sections.md:19`).
- Verify: "For the final OpenSpec `verify` work unit, persist ... verify-report.md before settlement" (`GA/internal/assets/skills/sdd-verify/SKILL.md:46,152`).
- Labels reales de test: `migration-schema-implementation` (apply) y `requirements-runtime-verification` (verify) (`advance test :17-20`) — el label nombra fase+scope. Ojo: `sdd-tasks` usa "work units" para slices de PR encadenados (`GA/internal/assets/skills/sdd-tasks/SKILL.md:153,258`), granularidad de delivery, no del ledger.

### 12.8 gentle-pi (Q8)

Sólo proxy/display: `quiet-tools.ts` reconoce `sdd-attempt` como comando rutinario únicamente para verbos {acquire, settle, grant} (`GP/extensions/quiet-tools.ts:241,346-359`; grant especial por roots `:391`); no escribe el ledger. Los contract tests pinnean acquire/settle en assets (`GP/tests/native-sdd-attempt-authority.test.ts:68-96,127-143`) y afirman que el estado Pi NO persiste contadores de intentos (`:220`). `GP/lib/review-integration-v2.ts:1917` sólo menciona en un comentario el digest de `sdd-attempt finish`. Work units fuera del ledger: `GP/skills/work-unit-commits/SKILL.md` (commits como unidades revisables — convención de delivery).

### 12.9 Tabla de divergencias (Q9)

| Concepto | gentle-ai | biggz-ai | Veredicto |
|---|---|---|---|
| Entidad Objective | `RuntimeObjective` ID+Generation+WorkUnit+budgets (`runtime_ledger.go:238-266`) | flat `RuntimeStore.ObjectiveID`, sin generation (`sddattempt.go:60`) | regressed/lost |
| Linaje por intento | `ObjectiveID`+`ObjectiveGeneration` (`:283-285`) | sólo `ObjectiveID` (`:105`) | regressed/lost |
| Guard de scope | 1 predicado con budgets (`runtime_admission.go:160-167`) | 2 guards, sin budgets, salta valores vacíos (`:1397-1413`) | intentionally different (más estrecho) |
| passed → Complete | `:2769-2773` | `:858`, `:1661` | ported correctly |
| Terminalidad de Complete | scope-only; sucesor vía advance (`runtime_admission.go:70-76`) | change-terminal: `blocked(corrupt_authority)` "ledger is complete; reset required to continue" (`:453-454`) | regressed/lost |
| Registro advance | `objective/advance` preserva intentos (`:45`, `:1063-1069`, `:2648-2691`) | no existe | absent |
| Reset | conserva `Attempts`/`Lifetime*` (`:2361-2386`; reset test `:91-95`) | `Attempts = nil` (`:1020`) | regressed/lost |
| Contadores | cumulative (objetivo) + lifetime (change) (`:432-435`) | refund-aware pero sin lifetime/generation | regressed (parcial) |
| Consent de presupuesto | envelope tipado (`budget_consent.go:92-107`) | ABSENT | absent |
| Rescope | verbo + narrowing auditado (`:366-385`, `:1829-1970`) | librería sí, CLI no (`:2043-2170`) | parcialmente portado |
| Clasificación scope-change | `blocked(maintainer_decision)` con continuación (`runtime_compact.go:598-599`) | `invalid_continuation` (`:1397-1398`) | intentionally different |
| `progress` outcome | no existe; análogo = refund interrupted+incremento (`:2827-2855`) | settle-only, cobra como failed (`:1461-1465`) | BA-extra |

### 12.10 Candidatos de port y partes no portables (Q10 — análisis, no decisión)

- Portables (semántica): identidad de objetivo con generation persistida en cada intento; avance por work unit distinto tras passed-en-presupuesto como registro nuevo que preserva la cadena; Complete des-terminalizado a nivel change; rechazos tipo `ErrRuntimeObjectiveDone` que nombran el sucesor; presupuesto por objetivo con contadores lifetime separados; guard de scope unificado (incluidos budgets); presupuesto del sucesor tomado de su propio request.
- NO portables tal cual: shape del CLI (GA usa `--cwd/--change` y verbos handoff/rescope/repair/`--objective-relation`; BA usa `<change>` posicional — portar exige adaptar invocaciones, no semántica); el esquema CAS de BA (`biggz/sdd-runtime/v1`, request digests propios) — el advance debe ser un nuevo tipo de registro CAS en esa codificación, no el `runtimeRecord` upstream; el fallback machine-scoped sin git de BA (HelpText) que upstream no tiene; los campos de acoplamiento review/RDD de BA (`--binding-revision/--binding-lineage`, `--strict`, settle obligations) que upstream no conoce; la maquinaria untracked-inventory upstream (#4195) y handoff/worktree binding, ajenas al CLI de BA.
- Advertencias: el `Rescope` de BA ya difiere del upstream (reglas widened/exhausted propias); cualquier port debe preservar la semántica cumulative-never-reset ya testeada (`rescope_test.go:9-58`). La lista de asserts del advance test upstream es la spec mínima a replicar.
