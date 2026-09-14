# Proposal: fix-attempt-ledger-scope

## Intent

El maintainer pagó cinco resets autorizados del attempt ledger en una sola sesión; cada `reset` ejecuta `store.Attempts = nil` (`internal/sddattempt/sddattempt.go:1020`) y borra el audit de attempts. La probe empírica con el binario real fija el mecanismo: `settle --outcome passed` devuelve `complete: true` con `remaining_attempts: 2`; el siguiente `acquire` con otro `--work-unit` muere en `blocked(corrupt_authority)` (`:453-454`) y con `--outcome progress` en `blocked(invalid_continuation)` (`:1392-1410`). El segundo coste es la trampa `DecisionRequired`: `progress` cobra 1 attempt entregado (`:209-232`) y con `--max-attempts 3` un change de 4 slices agota la generación.

El diagnóstico es un port gap, no una decisión de diseño: biggz-ai trata `work-unit` como la identidad de scope de TODA la generación y `Complete` como terminal del change; upstream trata el objetivo como el scope de UN work unit, `Complete` termina solo ese scope, y el sucesor se abre con otro label preservando la cadena (`GA/internal/sddstatus/runtime_admission.go:78-88`). Objetivo: un change multi-slice llega a apply/verify con evidencia por slice y cero resets de mantenimiento.

## Scope

### In Scope

- **Avance por work unit distinto**: `sdd-attempt acquire <change> --work-unit <label distinto>` tras un settle `passed` abre una generación sucesora y preserva la cadena completa de attempts. Predicado de admisión: `Complete=true` ∧ ¬`DecisionRequired` ∧ sin attempt activo ∧ `len(Attempts)>0` ∧ último attempt `passed` ∧ `--work-unit` distinto del vigente (adaptado de `runtimeObjectiveAdvanceAdmissible`, `GA/internal/sddstatus/runtime_ledger.go:1695-1710`).
- **Rechazo que nombra el sucesor**: un `acquire` con el MISMO label tras `passed` no devuelve `proceed` y su mensaje nombra la continuación (`acquire` con un `--work-unit` distinto) al estilo de `ErrRuntimeObjectiveDone` (`GA/internal/sddstatus/runtime_ledger.go:88-95`; compact: `state: complete` con exit que nombra el sucesor, `GA/internal/sddstatus/runtime_compact.go:596-597`); deja de presentar `reset` como única salida.
- **`Reset` preserva la cadena**: `Reset` deja de ejecutar `Attempts = nil` y limpia solo el objetivo vivo (`Complete`, `DecisionRequired`, `ActiveAttempt`, evidencia); attempts y contadores lifetime quedan auditables (ref. upstream: tras reset + begin, la cadena sigue completa, `GA/internal/sddstatus/runtime_ledger_reset_test.go:70-95`).
- **`ObjectiveGeneration` en cada attempt**: denormalización que atribuye cada attempt a su generación aunque el objetivo anterior desaparezca del status (`GA/internal/sddstatus/runtime_ledger.go:283-285`).
- **Contadores `Lifetime*` a nivel change**: `LifetimeAttempts`/`LifetimeChangedLines` visibles en status, inmunes a `Reset`/`Advance` (`GA/internal/sddstatus/runtime_ledger.go:428-429`); el sucesor abre presupuesto fresco desde su propio request (`GA/internal/sddstatus/runtime_admission.go:152-153`).
- **Guard de scope unificado**: un solo predicado `WorkUnit ∨ EvidenceGoal ∨ MaxAttempts ∨ MaxChangedLines` para "scope cambiado sin reset explícito" (ref. `runtimeObjectiveScopeChanged`, `GA/internal/sddstatus/runtime_admission.go:160-167`), reemplazando la dupla actual (`sddattempt.go:1392-1410`); sobre un objetivo ABIERTO el rechazo se conserva.
- **Compatibilidad CAS**: todo campo nuevo persistido `omitempty`; `generation` cero se lee como 1 solo en derivación/vista, nunca en bytes canónicos; un ledger preexistente `complete: true` sin `generation` puede avanzar con un `--work-unit` distinto (D3; regla de content address en `internal/sddattempt/cas_store.go:489-510`).
- **Record CAS del avance**: nuevo tipo de registro snapshot en la codificación `biggz/sdd-runtime/v1` que incorpora el evento de avance a la provenance (patrón `Resets []RuntimeReset`, `sddattempt.go:78`), idempotente por request ID — no el `runtimeRecord` event-sourced de upstream.
- **Tests**: cobertura nueva (label distinto, label repetido, label hacia atrás, replay idempotente, presupuesto fresco del sucesor, ledger viejo `complete:true`) y actualización consciente de los tests que fijan la semántica actual (listados en exploration §8: `acquire_settle_test.go`, `cas_store_test.go`, `parity_test.go`, `sddattempt_test.go`, `rescope_test.go`, `internal/sdd/remediation_derive_test.go`).
- **Specs canónicas**: REQs nuevos en `openspec/specs/runtime/spec.md` (completitud scope-only, advance, reset preservador, generación, lifetime); hoy `openspec/specs/**` no documenta completitud ni work units (grep sin coincidencias). REQ-G2-01/02 de `openspec/specs/sdd/spec.md:164-186` se verifican consistentes, sin cambio de requisito.
- **Assets**: actualizar `internal/assets/biggz/biggz-orchestrator-workflow.md:184-187`, `internal/assets/skills/_shared/sdd-status-contract.md:86-93,229`, `internal/assets/opencode/commands/sdd-verify.md:22` y `internal/assets/prompts/sdd/sdd-verify.md:44,82` para que la continuación por slice sea explícita.

### Out of Scope

- **Verbos CLI nuevos**: el avance ES `acquire` con otro `--work-unit` (D2); shape posicional y flags actuales (`--request-id`, `--work-unit`, `--evidence-goal`, `--max-attempts`, `--max-lines`) intactos.
- **Verbos upstream `handoff` / `repair` / `rescope`**: no se portan; el `Rescope` de librería (`sddattempt.go:2043-2170`) queda intacto con sus reglas widened/exhausted fijadas en `rescope_test.go:9-58`, sin verbo CLI.
- **Envelope tipado de consentimiento de presupuesto** (`GA/internal/sddstatus/budget_consent.go:92-107`).
- **Maquinaria untracked-inventory** (`--untracked-scope`, `--expected-untracked-inventory`, `--intended-untracked`).
- **Binding de worktree/handoff**.
- **Flags de acoplamiento review/RDD** (`--binding-revision`, `--binding-lineage`, `--strict`): sin cambios, al igual que los gates RDD.
- **Guards de remediación y derivación de obligación** (`sddattempt.go:1316-1345`, `:1629-1633`, `:415-446`): no se tocan.

## Capabilities

### New Capabilities
None — la semántica aterriza en la capability existente `runtime`.

### Modified Capabilities
- `runtime`: REQs nuevos de completitud scope-only, avance por work unit, reset preservador de la cadena, generación por attempt y contadores lifetime; el rechazo de admisión por completitud pasa a nombrar el sucesor.

## Approach

- **Mapeo semántico, no copia estructural**: el `RuntimeObjective` con `Generation` de upstream se proyecta sobre el `RuntimeStore` plano de biggz-ai añadiendo la generación vigente y su denormalización por attempt; al avanzar se limpia el objetivo vivo y la historia permanece en `Attempts` y en la provenance del avance.
- **Seam de admisión**: el predicado de avance vive junto a `deriveAdmissionBlocked` y la admisión de `Acquire` (`sddattempt.go:453-454`, `:1312-1313`) como único dueño que consumen acquire, begin y la proyección de status, evitando reglas duplicadas divergentes. La clasificación exacta del rechazo en la proyección de status (reason y mensaje) es decisión de design (ver Risks).
- **Presupuesto del sucesor**: `MaxAttempts`/`MaxLines` se toman del request del avance; los cumulativos de la generación se reinician y `Lifetime*` continúa.
- **Codificación CAS**: el avance se publica como record snapshot nuevo en `biggz/sdd-runtime/v1` con la provenance del evento (patrón `Resets`), no un record event-sourced tipo upstream; idempotencia por `request_id` como el resto de operaciones.
- **Derivación vs persistencia**: todo lo derivable (`generation` cero ≡ 1, lifetime si la cadena preservada lo permite) se computa en lectura para no tocar bytes canónicos; solo se persiste lo no derivable, siempre `omitempty`.
- **Delivery forecast**: toca source + tests + assets + specs → chained/stacked sobre main en slices por capa (núcleo CAS/admisión; reset/avance; assets; specs), acotados al presupuesto de 400 líneas por PR; las guard lines exactas quedan para `sdd-tasks`.

## Rollback

- Revertir los commits del change restaura guards y mensajes previos; los strings de rechazo no afectan datos.
- Los records CAS son snapshots inmutables: un ledger que avanzó bajo el código nuevo contiene campos con valores nuevos (generación ≥2, provenance de avance) y el binario revertido falla fail-closed al re-verificar el content address (`cas_store.go:489-510`, "does not match its content address"). No hay downgrade limpio para esos changes.
- Recuperación: preferir fix-forward; si hay que volver atrás con un ledger ya avanzado, restaurar el puntero `HEAD` del change (`<git-common-dir>/biggz/sdd-runtime/v1/<change>/`) a la revisión previa al avance — los records anteriores siguen en disco y el binario viejo vuelve a leerlos; los records del sucesor quedan como archivos huérfanos y nunca se borran (forensia).
- Ningún record se reescribe ni se migra in-place; la importación legacy (`~/.biggz/sdd-runtime/v1/<change>.json`) queda intacta.

## Success Criteria

- [ ] En un change multi-slice (dogfood CLI y/o test de integración), la secuencia acquire → settle `passed` → acquire con `--work-unit` distinto → settle `passed` llega a apply/verify con evidencia por slice y CERO entradas nuevas en `Resets`.
- [ ] Un `acquire` repetido con el mismo `--work-unit` tras `passed` no devuelve `proceed` y su mensaje nombra el sucesor, sin presentar `reset` como única salida.
- [ ] Un `acquire` que cambia `--work-unit`/`--evidence-goal`/budgets sobre un objetivo ABIERTO sigue rechazado.
- [ ] Tras `Reset`, todos los attempts previos siguen presentes y auditables (`len(Attempts)` no decrece; cada uno conserva outcome y evidencia).
- [ ] Un ledger preexistente `complete: true` sin `generation` avanza con un `--work-unit` distinto, y todo fixture de record previo al cambio carga sin error de content address.
- [ ] La remediación sigue admitiendo un attempt correctivo keyed en evidencia fallida sin remediar, incluso con un avance de generación de por medio (`internal/sdd/remediation_derive_test.go` verde + caso nuevo).
- [ ] `go test ./... -count=1` verde, con los tests de la semántica vieja actualizados conscientemente (ningún assert eliminado sin equivalente).

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| CAS: un campo nuevo no-omitempty o un default aplicado en lectura/escritura invalidaría TODOS los records ("does not match its content address") | High | Regla `omitempty` + derivación read-time; test de compat que carga un record previo y re-canonicaliza; bytes canónicos congelados en test |
| Assets que asumen la semántica actual: `opencode/commands/sdd-verify.md:22` ("active or complete attempt"), `sdd-status-contract.md:86-93,229`, `biggz-orchestrator-workflow.md:184-187`, `prompts/sdd/sdd-verify.md:44,82` | High | Actualizar los cuatro surfaces en el mismo change y verificar el texto; regenerar goldens de assets si existen |
| ~10 archivos de test fijan la semántica vieja (exploration §8); actualizarlos puede enmascarar regresiones | High | Migración test por test listada en tasks; prohibido relajar asserts sin equivalente nuevo |
| El presupuesto fresco del sucesor puede ocultar un loop descontrolado del orquestador | Med | `Lifetime*` visible en status; el label repetido tras `passed` queda rechazado; el avance exige request nuevo (sin auto-advance) |
| Clasificación del rechazo por completitud: retener `corrupt_authority` o introducir un reason propio toca `internal/sdd/status.go:1255-1268` y el contrato | Med | Decisión explícita en design; si cambia el reason, actualizar `status.go`, `sdd-status-contract.md:229` y tests |
| Downgrade de ledgers ya avanzados es fail-closed para el binario viejo | Med | Fix-forward preferido; restauración documentada del puntero `HEAD`; records nunca borrados |

Contradicciones §12 vs §1-§11: ninguna detectada; se aplicó §12 → upstream y §1-§11 → biggz-ai.
