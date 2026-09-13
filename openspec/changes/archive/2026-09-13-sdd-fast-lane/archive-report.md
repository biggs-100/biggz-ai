# Archive Report: sdd-fast-lane

**Change**: sdd-fast-lane
**Archived**: 2026-09-13 → `openspec/changes/archive/2026-09-13-sdd-fast-lane/`
**Store mode**: hybrid — artifacts en filesystem (`openspec/`) con espejo BigMem bajo `sdd/sdd-fast-lane/*` (11 topics + este `archive-report`); `openspec/config.yaml` no declara `artifact_store`, así que `declaredArtifactStore` resuelve al default `openspec` y el camino de sync es idéntico al de `hybrid`.
**Verdict at close**: PASS WITH WARNINGS — 24/24 tasks, 0 CRITICAL, 0 blockers, 2/2 requirements, 11/11 scenarios. El change aterrizó en el árbol de trabajo **sin commitear** (HEAD `d0cd87c6` + dirty) y la entrega (issue, branch, commit, PR único con `size:exception`) queda como siguiente paso del orchestrator con autorización explícita. Aprobación del maintainer para el archive: checkpoint 2026-09-13.

## Final State (terminal record — outranks intermediate snapshots)

El fast lane queda operativo, verificado y archivado: un change cuya **única** fuente de planning es `plan.md` alcanza apply→verify→archive sin exigir los cuatro artifacts, y ningún gate se relajó.

- **Alias por-slot en ambos resolvers**: `resolveArtifactPaths` (`internal/sdd/status.go:994`) sirve `plan.md` como candidato de los slots proposal/specs/design/tasks tras cada artifact real; `ApplyProgress`/`VerifyReport` **jamás** aliasan. BigMem replica con `bigmemArtifactPaths` (`engram_status.go:168`) y el helper `bigmemSlotContent(m, suffix)` en las tres lecturas directas de `bySuffix["tasks"]`.
- **Modo lane en `sdd-ff`** (`internal/assets/prompts/sdd/sdd-ff.md`): scaffold canónico (`### Requirement:` / `#### Scenario:` + checklist `- [ ]`), profundidad según artifacts, graduación in-place; sin comando nuevo, sin overlay, sin `AGENTS.md`.
- **Text surfaces**: `instructions.go:19,:25` lane-aware; `_shared/sdd-status-contract.md:223` (apply ready = planning set resuelto); `biggz-orchestrator-workflow.md:338` ahora dice **"Never skip gates"** (ya no "Never skip phases").
- **Gates intactos (los cinco)**: RDD receipt, `sdd-verify-validate`, workload guard, session-summary y edit authority — diff vacío en sus archivos + suites verdes; `TestLaneTerminalGatesNotRelaxed` compara lane vs full con los mismos blocked reasons.

### Decisions at close

| Decision (design.md) | Choice | Estado al cierre |
|----------------------|--------|------------------|
| Filesystem alias seam | `resolveArtifactPaths`: real primero, `plan.md` después; `Specs` → `[plan]` solo sin `specs/**/spec.md`; apply/verify nunca aliasan | ✅ Seguida — fijada por `TestLanePerSlotPrecedence`, `TestLaneInPlaceGraduation` y el dogfood |
| BigMem alias seam | Extender `spec↔specs` con fallback `plan`; `bigmemSlotContent` para `:221/:258/:312` (progress archivado, activo y edit authority) | ✅ Seguida — `TestLaneCrossStoreParity` compara las dos derivaciones reales |
| Staleness/ambiguity policy | Precedencia por-slot; un `plan.md` scratch junto a artifacts reales no altera routing | ✅ Seguida — `TestLaneTerminalGatesNotRelaxed` (caso `full-gates` con plan scratch) |
| Parity guarantee | Mismo plan-only en FS y BigMem → idéntico artifact set, applyState, next y progreso | ✅ Seguida — `TestLaneCrossStoreParity` + `TestMergeFilesystemAndBigMem_*` verdes |
| Text surfaces | Las cuatro superficies de texto + reconstrucción del CLI antes del dogfood | ✅ Seguida — grep determinista + rebuild ejecutado |
| Gates unchanged | Ningún validator/guard editado; la rama BigMem del edit authority ahora **lee** el plan (el guard sigue aplicando, no se relaja) | ✅ Seguida — diff vacío en `verify.go`, `cli_sdd.go`, `workload_guard.go`, `session_guard.go`, `edit_authority.go` |
| Open questions | `deriveRoute` reporta `organic` en plan-only (etiqueta cosmética, sin persistencia); renderers legacy toleran; rama engram tolera `bigmem:…/plan` | ✅ Resueltas — S-2 registrada como sugerencia |

## Verification reference (verify-report)

| Campo | Valor |
|-------|-------|
| Verdict | `pass_with_warnings` (admisible como PASS por `checkVerifyVerdict`) |
| Blockers / CRITICAL | 0 / 0 |
| Requirements / scenarios | **2/2** / **11/11** |
| Admission | `biggz sdd-verify-validate --requirements 2 --scenarios 11` → admitido; **re-check a archive time** → `Verify report is valid.` (exit 0) |
| Report file | `verify-report.md` · sha256 `05141185499905b4ebe27e31e0c47638f7269c8a05d7ffdfc4fc82c9da710aaa` · 21,112 B |
| Envelope | `evidence_revision sha256:763ab5a06ebcc138781b54f83508713d7d0b42929a581e83ca6fad5ac94c6fdd` (= `test_output_hash`; ledger attempt 2) |
| Sweep | `go test ./... -count=1 -timeout 900s` → exit 0, 60 packages `ok`, 0 `FAIL`, 0 panics; `go build ./...` exit 0 |
| Nota de entorno | El `-timeout 180s` configurado flakea en `internal/review` en esta máquina (**pre-existente, ajeno al cambio**); el barrido usó `-timeout 900s` |

**Coverage matrix**: 8 lane tests + `TestSDDStatusJSONPlanOnlyLaneApplyReady` (CLI) + suites legacy verdes acreditan los 11 scenarios por-scenario (matriz completa en `verify-report.md`). 3 de los 5 guards se acreditan por invariación de diff vacío + suites, no por test específico de lane (S-4).

## End-to-End Dogfood (evidencia de aceptación del change)

`./biggz.exe sdd-status --json --cwd C:/Users/USER/AppData/Local/Temp/lane-demo` sobre un fixture cuyo **único** artifact es `plan.md` (headings canónicos + 2 checkboxes, 1 hecho):

- `active: 1` · `Name: lane-demo` · los **cuatro** slots de planning resuelven a `<fixture>/plan.md`; `applyProgress`/`verifyReport` quedan `[]` (nunca aliasan)
- `taskProgress {total: 2, completed: 1, pending: 1}` leído del checklist del plan
- `applyState: ready` · `nextRecommended: apply` · `dependencies.proposal/specs/design/tasks = all_done`, `apply = ready` · sin `blockedReasons`
- `route: "organic"` — etiqueta legacy cosmética (S-2), ningún consumidor rutea por ella

El verifier reprodujo esta corrida de forma independiente; **este run de archive la re-ejecutó una vez más** contra el mismo binario reconstruido (`go build -o biggz.exe ./cmd/biggz`, exit 0) con idéntico resultado. Recordatorio operativo (S-1): los assets van embebidos — **sin rebuild del CLI, cualquier corrida asset-dependiente usa textos viejos**.

## Ledger & RDD state

- **Ledger** (`sdd-attempt status sdd-fast-lane`): 2 attempts, 0 activos, `complete: true`, `next_action: complete`, revision final `18aae8c0b2c57d19e39a5c6a76ab00d94b052542cd0d508773f090abfdf086f4` ("Blocked reason: corrupt_authority" es el exit informativo de un ledger cerrado — `ledger is complete; reset required to continue`; no hay más acciones posibles, que es lo esperado).
  - **Attempt 1 — apply `interrupted`**: la corrida delegada murió por timeout del harness (20 min) escribiendo `apply-progress`; el orchestrator completó las tareas 5.4–5.6, reconstruyó el CLI, corrió el dogfood y escribió `apply-progress.md`; settle `req-fastlane-settle` → rev `545f3800…`; token `tok-5a76825b8a672e03e933dbb7`.
  - **Attempt 2 — verify `passed`** con `evidence_revision sha256:763ab5a0…` (el mismo del verify-report); settle `req-fastlane-verify-settle` → rev `18aae8c0…`; token `tok-5d5efb50ae02eab11294e79d`.
  - **Archive no abrió ni settleó ninguna transacción**; superficies de escritura: el move de artifacts, este report y el espejo BigMem.
- **RDD (native review authority)**: `rdd status` → `effective_mode: enabled` (global enabled, source default). El candidato gobernante (`ResolveCandidateLineage(HEAD^{commit})` = `d0cd87c6…`) resuelve a la legacy lineage **`01a092ef-b0d6-74e0-95fd-49293fa374ec`** (4 eventos, `chain_valid: true`): gate `post-apply` → **`allowed: true`**, `delivery: "burned/unmanaged"` ("receipt is ephemeral and burned after finalize; delivery via ordinary repository policy"); `VerifyPreflightAt(ws, sdd-fast-lane)` → `nil`. **No bloquea** por diseño (delivery burned, no deciding). Maintainer aprobó el archive en el checkpoint 2026-09-13; no se abrió lineage nueva (el change conserva su `review-subject.json` y su `reviewOffer` post-verify sin consumir, ambos consistentes con el expediente honesto del verify-report: "este report no reclama receipt").

## Budget

**591 changed lines vs el budget de 400** (≈+48%): 212 tracked (+183/−29, producción) + 379 de tests (`internal/sdd/status_lane_test.go`, nuevo). La entrega requiere **`size:exception`** (recomendado: el alias, su helper de paridad y sus tests son una sola unidad coherente) o un split alias vs texto/docs. Registrado desde `apply-progress.md`; no viola ninguna spec — es carga de reviewer.

## Gates at close (native dev build `./biggz.exe`)

| Gate | Result | Evidence |
|------|--------|----------|
| Task Completion | ✅ PASS — **24/24** checked, 0 unchecked; sin reconciliación | `tasks.md` persistido (`- [x]` 24 / `- [ ]` 0); dispatcher `taskProgress {total: 24, completed: 24, allComplete: true}` |
| CRITICAL issues | ✅ PASS — 0 CRITICAL / 0 blockers | `verify-report.md` — PASS WITH WARNINGS (3 WARNING, 5 SUGGESTION) |
| Native review receipt (RDD) | ✅ PASS — gate del candidato `allowed: true`, `delivery: "burned/unmanaged"`; chain valid; maint. approval 2026-09-13 | `review gate post-apply 01a092ef-… --json`; `verifyPreflight` → `nil`; `rdd status` enabled |
| Session-summary guard | ✅ PASS — `{"verified":true,"reason":"","fallback":""}` | `session-close --check-only --json` |
| Native dispatcher (final authority) | ✅ post-move + espejo: `active: []`, `sdd-fast-lane` en `archived` | `sdd-status --json`; pre-move: `archive: ready`, `nextRecommended: archive`, sin `blockedReasons` |
| Action-context guard | ✅ repo-local; ediciones confinadas a `openspec/changes/**` + el topic BigMem | `actionContext.allowedEditRoots: [repo root]`, mode `repo-local` |

## Specs synced (Step 2 — aplicado por la fase nativa sdd-sync; **verificado aquí, no re-aplicado**)

| Domain | Delta action | Main spec | Numbers |
|--------|--------------|-----------|---------|
| `sdd-status` | 1 ADDED — `Plan-Only Fast Lane Alias` (7 scenarios) | `openspec/specs/sdd-status/spec.md` | **+46/−0** · 250→296 líneas · req 12→**13** · scen 33→**40** |
| `orchestrator` | 1 ADDED — `Fast Lane Runs Inside sdd-ff` (4 scenarios) | `openspec/specs/orchestrator/spec.md` | **+28/−0** · 769→797 líneas · req 33→**34** · scen 113→**117** |

**Totales**: 2 agregados, 0 modificados, 0 removidos; **+74/−0**. Hashes post-sync verificados en esta corrida: `sdd-status` `5fb490ce7a3eee6837bcda8ee6ae2854da8ff087cb9a05febd6cc01a490d0265`; `orchestrator` `c9d98da31d592913068ab90172a9c3f905ec6dfb81c003bc1eb79615c72809c5`. Paridad delta↔spec comprobada: ambos requisitos presentes exactamente 1× y **los 11 scenarios del delta aparecen verbatim** en los specs canónicos; requisitos ajenos intactos (0 borrados); solo `ADDED` → sin merges destructivos (`sync-report.md` lo registra; la fase sync corrió con guardas PASS). El archive no escribió en `openspec/specs/**` — los edits del sync siguen sin commitear para revisión del orchestrator.

## Archived contents

13 archivos (12 carried + este report); **sha256 pre-move = post-move** en los 12 carried (diff vacío):

| Archivo | sha256 |
|---------|--------|
| `_meta.yaml` | `96f5b82833080789bcb306418eadcc33c07f2a50402bd1294c95cec3dfdd1858` |
| `exploration.md` | `c1b91633f9621f6ecc44a1cfde81bc28b76fc6003dc9be9f8b4471b648ed29f7` |
| `preproposal.md` | `9c56f43f9f37c47c4dde13384ded2e9a46585bf955329515f0f06fded02212c9` |
| `proposal.md` | `18076f48d79e60c9a0dfde29115fd8f8f628f590cc16c4e9221b054652a4638e` |
| `specs/sdd-status/spec.md` | `9369832ba427a87d9ab83f9adf35f62333a4b1458b61fcd11656279c33f9d393` |
| `specs/orchestrator/spec.md` | `5bb3f43721b56cbd57c9dfd89b895ae2d384d6a204c4cb8d71279ee8691113ab` |
| `design.md` | `1fe8c164fe26f04d6cb3920984d64012d3af681507feb885fff1dee6fc7597ac` |
| `tasks.md` (24/24) | `bd2f079dac67c27ff6eb009b0cd9a0124f96e8bd64866ac191e59a024cfaf251` |
| `apply-progress.md` | `8bf53c463cc52c0fb999ee48a6a27ca90d14308f50ae6462a9cc30e777a50d64` |
| `verify-report.md` | `05141185499905b4ebe27e31e0c47638f7269c8a05d7ffdfc4fc82c9da710aaa` |
| `sync-report.md` | `cfacfb2f79793d0d2f8cddee6bbbcb133fe5410671fd205e2b9cabe315ff8a8c` |
| `review-subject.json` | `2f76215c7c3aae853776b15d22d5d587477253641a72ad6d49ed98765971c34f` |
| `archive-report.md` | este archivo (espejado a BigMem `sdd/sdd-fast-lane/archive-report`) |

Sin `state.yaml` (nunca se creó en esta corrida) y sin `.biggz-instance` (renombrado no aplica; no existe). Ningún artifact quedó fuera.

## Post-archive hygiene (Step 3b)

- Ejecución **non-TTY** (sub-agente) → contrato: no se borra nada, exit 0. Nada eliminado.
- Candidatos: **cero** ramas `sdd-fast-lane*` (el change nunca se ramificó; todo vive en el árbol de trabajo) y cero upstreams `[gone]`; un solo worktree (repo root). No hubo `fetch --prune` que ejecutar.

## Warnings carried (no bloquean) y follow-ups

- **W-1** — Presupuesto: **591 vs 400** changed lines → la entrega necesita `size:exception` (o split alias vs texto/docs).
- **W-2** — Provenance del apply: corrida interrumpida por timeout; tail completado por el orchestrator; `apply-progress.md` fue escrito por él. El verify **re-ejecutó todos los claims** (ninguna evidencia heredada sin reproducir), pero el slice no terminó como una corrida única íntegra.
- **W-3** — La prosa de assets ("Never skip gates", modo lane de `sdd-ff.md`) se guarda con grep determinista, **sin test Go**; precedente barato: `internal/assets/biggz/orchestrator_test.go`.
- **S-1** — Rebuild obligatorio del CLI (`go build -o biggz.exe ./cmd/biggz`) en todo flujo de dogfood/assets (assets embebidos).
- **S-2** — `deriveRoute` reporta `route: "organic"` en plan-only (flags legacy que no ven el alias): etiqueta cosmética; evaluar mapear a `sdd`.
- **S-3** — `_meta.yaml` quedó con `phase: explore` y descripción del approach descartado (metadata engañosa, nada la parsea).
- **S-4** — 3 de los 5 guards se acreditan por diff vacío + suites, no por test de lane específico.
- **S-5** — El fixture de dogfood depende de la máquina; usar `TestSDDStatusJSONPlanOnlyLaneApplyReady` como smoke durable en CI.

## Pending después del archive (NO ejecutado en este run)

Todo sigue **sin commitear** (HEAD `d0cd87c6` + dirty): los edits del sync en los specs canónicos, los 7 archivos de código/assets modificados, `internal/sdd/status_lane_test.go`, y el move + este report. La **entrega** (issue, branch, commit, PR único con `size:exception`) es el siguiente paso del orchestrator **con autorización explícita**. Este run no hizo commits, no tocó los specs canónicos, no modificó el código ni ningún otro change.

## Evidence refs (raw)

- `./biggz.exe sdd-status --json --cwd <repo>` pre-move → `dependencies {apply: all_done, verify: all_done, sync: all_done, archive: ready}`, `taskProgress 24/24`, `nextRecommended: "archive"`, sin `blockedReasons`
- `./biggz.exe sdd-status --json` post-move + espejo → `active: []`; `sdd-fast-lane` presente en `archived`
- `./biggz.exe sdd-attempt status sdd-fast-lane` → `Attempts: 2`, `Active attempt: 0`, `Complete: true`, `Revision: 18aae8c0…`
- `.git/biggz/sdd-runtime/v1/sdd-fast-lane/record-18aae8c0….json` → attempt 1 `interrupted` (settle rev `545f3800…`), attempt 2 `passed` (`evidence_revision sha256:763ab5a0…`)
- `./biggz.exe review gate post-apply 01a092ef-b0d6-74e0-95fd-49293fa374ec --json` → `{"passed":false,"allowed":true,"delivery":"burned/unmanaged","reason":"review burned: receipt is ephemeral and burned after finalize; delivery via ordinary repository policy"}` (exit 0)
- `./biggz.exe review status 01a092ef-… --json` → `chain_valid: true`, 4 eventos, receipt artifact `sha256:04b25b9f…`, burned `2026-09-11T19:36:52-05:00`
- `./biggz.exe rdd status --json` → `effective_mode: enabled`
- `./biggz.exe session-close --check-only --json` → `{"verified":true,"reason":"","fallback":""}`
- `./biggz.exe sdd-verify-validate --requirements 2 --scenarios 11 openspec/changes/archive/2026-09-13-sdd-fast-lane/verify-report.md` → `Verify report is valid.` (exit 0)
- Dogfood re-run (archive time): `./biggz.exe sdd-status --json --cwd C:/Users/USER/AppData/Local/Temp/lane-demo` → `nextRecommended: apply`, 4 slots → `plan.md`, `taskProgress {2,1,1}`
- Move: `mv openspec/changes/sdd-fast-lane openspec/changes/archive/2026-09-13-sdd-fast-lane` → sha256 pre/post idénticos en los 12 archivos; `openspec/changes/` queda solo con `archive/`
- BigMem: `sdd/sdd-fast-lane/archive-report` saved via `./biggz.exe bigmem save … --type architecture --topic-key sdd/sdd-fast-lane/archive-report` (project `biggz-ai`) — el marcador que el dispatcher híbrido lee para reportar el change como archivado
