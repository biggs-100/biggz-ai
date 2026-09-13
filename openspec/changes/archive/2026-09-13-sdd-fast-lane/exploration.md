# Exploration: sdd-fast-lane

## Current State

El grafo de fases NO se almacena: se deriva en cada lectura de status.

- `prepareCoreState` (`internal/sdd/status.go:868`) es el chokepoint: `coreReady = proposal ∧ specs ∧ design ∧ tasks ∧ taskProgress.Total > 0`; `resolveApplyState` (`status.go:1149`) y `resolveDependencies` (`status.go:1161`) cuelgan de ahí: `Verify=Ready` solo si `coreReady && applyState==ApplyAllDone` (`status.go:1179`), `Archive=Ready` si `Verify==AllDone && tasks.AllComplete` (`status.go:1184`).
- `nextRecommended` se resuelve con prioridad fija: apply > verify > remediate > sync > archive (`status_helpers2.go:67`), luego planning: propose > spec > design > tasks (`status_helpers2.go:94`).
- Los artifact states vienen de `collectArtifactDerivation` (`status.go:628`) sobre `resolveArtifactPaths` (`status.go:994`): `proposal.md`, `specs/**/spec.md`, `design.md`, `tasks.md`. El contenido de tasks sale de `readText(firstPath(paths.Tasks))` (`status.go:642`) y los checkboxes de `countTaskProgressText` (`status.go:1106`).
- Hay TRES entrypoints de derivación: filesystem `deriveChangeStatusCtx` (`status.go:703`), forced-store (`status.go:468`) y BigMem `deriveBigMemChangeStatus` (`internal/sdd/engram_status.go`), que DUPLICA el `coreReady` inline (`engram_status.go:271`) — toda lane debe enseñarse dos veces.
- `status_v2.go:308` limita las claves de artifacts emitidas a las seis existentes; `status_v2.go:325` permite ya todos los tokens de `nextRecommended` que la lane necesita (apply/verify/remediate/sync/archive). No hace falta token nuevo.

## Affected Areas

- `internal/sdd/status.go:994` (`resolveArtifactPaths`) — punto de alias del artifact fusionado hacia los 4 slots.
- `internal/sdd/status.go:628` / `:868` / `:1149` / `:1161` — derivación core (quedan intactas si el alias es correcto).
- `internal/sdd/engram_status.go:271` y `bigmemArtifactPaths` (`engram_status.go:168`) — paridad BigMem (`coreReady` duplicado).
- `internal/sdd/instructions.go:19,25` — texto hardcodeado "Read proposal, specs, design, and tasks…" (apply y verify).
- `internal/assets/skills/sdd-verify/SKILL.md:48-50,75-78` — tabla de degradación graciosa (verify NO exige proposal/design).
- `internal/assets/skills/_shared/sdd-status-contract.md` — regla "apply ready only when specs, design, and tasks are available".
- `internal/assets/prompts/sdd/sdd-ff.md` — meta-command existente con umbrales 400 líneas/nueva interfaz/dominio/dep (candidato a hospedar la lane).
- `internal/assets/prompts/sdd/sdd-new.md` + `internal/assets/biggz/biggz-orchestrator-workflow.md:338` ("Never skip phases — follow dependency graph").

## Approaches

**1. Inferir la lane del artifact presente (plan.md alias)** — si existe `plan.md`, `resolveArtifactPaths` lo aliasa a Proposal/Specs/Design/Tasks con precedencia por-slot (artifact real gana); los checkboxes y los headings `### Requirement:`/`#### Scenario:` se leen del mismo archivo.

- Pros: cero campos/tokens nuevos; respeta el modelo "derived, not stored" del dispatcher; promoción mid-flight gratuita (escribir `proposal.md`/`specs/`/`design.md`/`tasks.md` gradua el change in-place); `contextFiles` ya dirige a los skills (sdd-apply/SKILL.md:31 "never assume filenames"); precedente de alias ya existe (`spec`↔`specs`, `engram_status.go:146-152`).
- Cons: ambigüedad si `plan.md` es scratch junto a artifacts reales (se mitiga con precedencia + test); el punto de verdad es la forma del repo, no una declaración explícita.
- Effort: Low (≈20-35 líneas en `resolveArtifactPaths` + ≈10-15 en BigMem).

**2. Campo `lane:` en `_meta.yaml`** — declarado en sdd-new.

- Pros: explícito y auditable.
- Cons: HOY nada parsea `_meta.yaml` (solo se comprueba existencia: `internal/sdd/new.go:143`); exige parser YAML nuevo en derivación + sincronización en promoción + propagación a `prepareCoreState`/`resolveDependencies`/BigMem.
- Effort: Medium-High.

**3. Persistir el veredicto de `sdd-route` en sdd-new** — nace del route ya existente.

- Pros: reusa los umbrales que ya coinciden con `sdd-ff` (route.go:46-52; sdd-ff.md umbrales idénticos).
- Cons: mismatch semántico — el veredicto es `direct|ask-sdd` (¿hacer SDD o no?), no profundidad de pipeline; hoy NO se persiste en ningún sitio y su único consumidor es texto advisory (`biggz-orchestrator-delegation.md:13,66`); nuevo esquema de persistencia + lector.
- Effort: Medium.

## Recommendation

Approach **1** (inferencia por artifact), con la lane ejecutada desde `sdd-ff` extendido — no un meta-command nuevo.

Mecánica mínima exacta (Q1): aliasar `plan.md` en `resolveArtifactPaths` (`status.go:994`) y `bigmemArtifactPaths` (`engram_status.go:168`) para que `collectArtifactDerivation` llene las seis claves; entonces `coreReady` (`status.go:868`/`engram_status.go:271`) pasa, `resolveApplyState` marca ready/all_done con los checkboxes del propio plan, y `resolveDependencies` + `nextRecommended` (`status.go:1161,1202`) producen apply → verify → archive SIN cambios. Clave: `readSpecCounts` (`internal/sdd/verify.go:121`) leerá los headings del plan, y `checkVerifyExitAndTotals` (`verify.go:188`, con el match de totales en `:197` y `:201`) exigirá que el envelope de verify declare exactamente esos totales — el plan DEBE usar `### Requirement:` / `#### Scenario:`.

## Question answers (evidence)

- **Q2 (Router)**: implementado en `internal/sdd/route.go:93` (`EvaluateRoute`), CLI en `cmd/biggz/cli_sdd.go:1735-1830`. Consumidor real: NINGUNO en código (grep Go+JS: solo tests y el texto advisory del orchestrator). El veredicto no se persiste.
- **Q3 (verify y planning artifacts)**: verify NO requiere proposal/design — la SKILL degrada por artifact presente (`sdd-verify/SKILL.md:48-50,75-78`); el único hard-dep es matching de counts contra specs reales (`verify.go:188` → `:197,201`) y la admisión `biggz sdd-verify-validate` (`cmd/biggz/cli_sdd.go:351`; `_shared/persistence-contract.md` "Verify-report admission"). Lugares a enseñar la lane: `instructions.go:19,25`, `sdd-status-contract.md` (apply-ready rule), `biggz-orchestrator-workflow.md:338`, y opcionalmente la tabla de degradación de sdd-verify.
- **Q5 (mínimo viable)**: `resolveArtifactPaths` (≈25 líneas) + `bigmemArtifactPaths` (≈12) + tests: nuevo `internal/sdd/status_lane_test.go` (plan.md solo → `apply` → checkboxes → `verify` ready → envelope passing con counts del plan → `archive` ready; precedencia plan.md vs artifacts reales; sin plan.md el comportamiento legado intacto) manteniendo verdes `status_test.go`, `status_v2_test.go`, `engram_status_test.go`, `route_test.go`; dogfood CLI en `cmd/biggz/sdd_status_cli_test.go` (`biggz sdd-status --json` sobre fixture plan.md-only). Texto: extender `sdd-ff.md` (modo lane) — sin skill nueva, sin `sdd-overlay-multi.json`, sin `wizard_skills.go:16`, sin tabla AGENTS.md. Total ≈ 40-50 líneas de código + 150-250 de tests: cabe en el budget de 400.
- **Q6 (gates que NO cambian)**: RDD receipt pre-verify — `rddGateBlocked` (`status.go:942`) → `verifyPreflightAt` (`verify.go:894`), aplicado en `applyTerminalGateBlocks` (`status.go:905`); `biggz sdd-verify-validate` — `cmd/biggz/cli_sdd.go:351`; PR workload guard — `WorkloadGuard` (`workload_guard.go:121`) + `biggz sdd-workload` (`cli_sdd.go:1464`); session summary — `EnsureSessionSummary` (`session_guard.go:415`) en `status.go:905`; edit authority — `applyEditAuthorityBlock` (`edit_authority.go:407`) dentro de `prepareCoreState` + guard `biggz sdd-apply` (sdd-apply/SKILL.md:35). El alias mantiene `tasksContent` no vacío, así que el scan de edit-roots sigue funcionando.
- **Q7 (prior art)**: `sdd-ff` (colapsa fases sin saltarlas), `RemediationState`/`remediate` (`status_helpers2.go:86`, `buildRemediationState` `status.go:671`), research lane (`internal/sdd/research.go:13`), `applyStaleDecisionRouting` (`status.go:1255`), `syncStateGate` (BigMem fuerza Sync=AllDone) y el alias `spec`↔`specs` (`engram_status.go:146`). Todo apunta a reutilizar, no inventar.

**Respuesta de una línea**: la fast lane NO necesita vocabulario nuevo — reutiliza artifacts, tokens y gates existentes; solo materializa `plan.md` como alias de los cuatro slots y un modo "lane" en el `sdd-ff` existente.

## Risks

- `engram_status.go:271` duplica `coreReady`: si solo se enseña el alias filesystem, la lane diverge entre stores (guardar con `topology_parity_test.go`).
- Un `plan.md` obsoleto o scratch puede sombrear/misroutar; requiere regla de precedencia por-slot + test explícito.
- Si el plan no usa los headings canónicos, el envelope de verify fallará la admisión por counts (`verify.go:197,201`).
- Legado: `probeArtifacts` (`HasProposal`…) queda false en una lane — nunca rutear por esos campos (`sdd-status-contract.md` lo prohíbe) y avisar dónde se renderizan.
- `biggz-orchestrator-workflow.md:338` ("Never skip phases") debe enmendarse o los orquestadores rechazarán la lane.
- No verificable desde el código: si `biggz sdd-continue` (`cli_sdd.go:1084`) y los renderers legacy rotulan bien un change plan.md-only.

## Ready for Proposal

**Yes** — proposal con approach 1; decisiones abiertas para el humano: (i) nombre del artifact fusionado (`plan.md` vs reusar `tasks.md` como documento único), (ii) si el modo lane vive dentro de `sdd-ff` (recomendado) o como meta-command nuevo, (iii) si la skill layer exige re-ejecutar `sdd-route` (advisory) antes de entrar a la lane.
