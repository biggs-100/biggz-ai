# Archive Report: wire-pi-review-relay

**Change**: wire-pi-review-relay
**Archived**: 2026-09-12 → `openspec/changes/archive/2026-09-12-wire-pi-review-relay/`
**Store mode**: hybrid — filesystem artifacts (canonical, untracked audit trail) + BigMem mirrors `sdd/wire-pi-review-relay/*` (explore, research, preproposal, ln, proposal, spec, design, tasks, apply-progress, verify-report, sync-report, this report)
**Verdict at close**: PASS WITH WARNINGS — 21/21 tasks, 3/3 requirements, 11/11 scenarios, 0 blockers, 0 CRITICAL. El smoke real-`pi` (task 4.6/3.1) PASÓ con `admission_decision: completed` y receipt finalizado; archive alcanzado con RDD habilitado y aprobación explícita del maintainer (checkpoint 2026-09-12, "Proceed — sync + archive").

## Destination decision (canonical location)

**Usado**: `openspec/changes/archive/2026-09-12-wire-pi-review-relay/` — forma fecha-prefijada (`YYYY-MM-DD-{change}`).

Razones, en orden de autoridad:

1. **Spec canónica del repo** — `openspec/specs/sdd/spec.md`, REQ "Archive Step 3b Post-Archive Hygiene": los escenarios describen el movimiento como `archive/YYYY-MM-DD-{change}` vía `os.Rename` (el mecanismo exacto de `ArchiveChange`). Es el source-of-truth SDD del propio comportamiento.
2. **Código CLI downstream** — `cmd/biggz/cli_cleanup.go` (`collectChangeNames`) parsea exactamente `archive/YYYY-MM-DD-change` (comentario textual `// archive/YYYY-MM-DD-change` + strip de fecha): la forma fechada es el layout que el código del CLI espera al reconstruir nombres de change.
3. **Precedente más reciente** — `openspec/changes/archive/2026-09-11-fix-rdd-receipt-collection/` (archivado 2026-09-11, aceptado por el maintainer con el mismo procedimiento: sync nativo → mv → report + mirror BigMem) usa la forma fechada. Todos los archives desde 2026-09-01 (21 directorios) usan la forma fechada.
4. **`internal/sdd/archive.go` (`ArchiveChange`)** — es agnóstico al nombre: mueve `archive/<name>` con `os.Rename` y solo agrega el sufijo de colisión UTC `-YYYYMMDD-HHMMSS` si el destino existe. Pasar el nombre canónico fechado produce exactamente `archive/<named>`; el código no contradice la forma fechada (no tiene callers de producción; los tests solo verifican preservación de mtime/`.git/biggz/rdd-mode` y RDD-enabled).
5. **Operación verificable** — con el nombre fechado en filesystem y el topic bare `sdd/wire-pi-review-relay/archive-report` en BigMem, el merge de `applyStoreRouting` reconoce el change como archived vía BigMem — el mismo mecanismo sancionado por el cierre anterior (fix-rdd: "the marker the merged dispatcher view reads to report the change as archived").

**Alternativa rechazada**: `archive/wire-pi-review-relay` (sin fecha). Su único sustento era la nota del report `sdd-parity-rescope-grant-ledger` (2026-08-31) — "non-date is canonical per `internal/sdd/archive.go:ArchiveChange` (date prefix used for historical docs)" — lectura que quedó desactualizada: los 21 archives posteriores al 2026-09-01 (incluido el precedente más reciente) usan la fecha. La forma sin fecha sigue siendo tolerada como lectura por `status.go` (cualquier nombre de directorio) y por `cli_cleanup` (pasa sin strip), pero es histórica, no canónica.

## Final State (terminal record — outranks intermediate snapshots)

`capture-result --agent pi` ahora **ejecuta** al reviewer y captura sus bytes crudos: cierra el hueco donde `PiAdapter` no tenía caller y el verbo solo validaba el handshake.

- **Entregado**: `--execute` (exclusivo con `--input`/`--preflight`/`--materialize`, solo `--agent pi`) ruteado `Preflight → MaterializeReviewerTask → PiAdapter.Review → Capture`; el CLI compone el prompt (`shared` ⊕ rol ⊕ **output contract** ⊕ separador ⊕ task byte-idéntica); timeout acotado (`DefaultReviewerTimeout=10m`, `--timeout` 1..7200, `MaxReviewerTimeout=2h`); cancelación mata; fallos tipados `ReviewerFailure{Kind,Stage,Elapsed,ExitCode,StdoutBytes,StderrBytes,Cause}` con **captura cero** (empty-output, nonzero-exit, output-over-cap, timeout, canceled, launch, role-unavailable); stdout crudo llega a `Capture` sin modificar; admission/CAS intactos. PR3 añadió el asset `internal/assets/prompts/review-output-contract.md` (42 L / 3,620 B) y la clasificación CLI de `*ArtifactAdmissionError`.
- **C33 resuelto empíricamente**: `exec.LookPath("pi")` resuelve el shim npm de Windows (`pi.cmd`) y `PiAdapter` lo spawnea correctamente — el escape hatch `BIGGZ_PI_REVIEW_RELAY_EXECUTABLE` **no fue necesario**.
- **Smoke real-`pi` (4.6/3.1) — PASS** (repo disposable `%TEMP%\biggz-smoke-relay`): lineage `review-d8e94d2af673b09d`, candidate `a29f2fadbea5357e1e9ab27f31512a79b48756e4`, lens `risk`; `capture-result --agent pi --execute --timeout 180` → exit 0, `admission_decision: completed`, artifact revision `ae997825dd38c2c032c4dd2d54c1520ce30bf4a6c4335ff223edf7655cc4749d`; `review finalize` → receipt `sha256:f0ee681b64e00da44ae5b2f0c70e6f633effe4755efda24a6dcefb60f7d3f088`; `review gate pre-pr` → `allowed: true`, `delivery: burned/unmanaged`. **Re-verificado durante este archive** contra el store del repo disposable: 5 eventos, `chain_valid: true`, gate `allowed: true`.
- **Regresiones**: `go test ./... -count=1 -timeout 900s` → exit 0, 60 paquetes `ok`, 0 FAIL (task 4.7/3.2).
- **Verificación final**: `pass_with_warnings` — 3/3 requirements, 11/11 scenarios, 0 blockers, 0 CRITICAL; `verify-report.md` 16,924 B, `sha256:7a5d0faa76d5779155de95817b1ed462be51353ad5a1fb0d70eca7ca16305dce`; envelope `evidence_revision sha256:6f44c5010dc49ebc69ccd6eacce94961653598106591708335dc1949f997fa91`.
- **Ledger** (`biggz sdd-attempt status wire-pi-review-relay`, verificado a este cierre): `Revision e4bb396ac21d01acb09a5a00ceec21a043b0999e86d2e3e4c4482167aa2efa99`, `Next action: complete`, `Attempts: 1`, `Active attempt: 0`, `Decision needed: false`, `Complete: true`, `Blocked reason: corrupt_authority` ("ledger is complete; reset required to continue"). Los slices settlearon `passed` (PR3 revision `12c5537f…`; verify revision `e4bb396a…`). **Fricción documentada**: fueron necesarios cuatro resets autorizados por el maintainer porque un settle `passed` cierra el ledger del change (registrado en `apply-progress.md` y memoria).
- **Slice budgets (apply)**: PR1 **582** vs 400 (+182), PR2 **430** vs 400 (+30), PR3 **145** vs cap 200 (dentro). PR1/PR2 requieren `size:exception` en delivery (ya previsto: estrategia `auto-chain`, `stacked-to-main`).
- **Snapshot attribution**: `verify-report.md` fue autorado contra HEAD `d0cd87c6` + working tree dirty; sus claims "done" siguen válidos y sus "pending" (commit, consentimiento de smoke) siguen vigentes. `apply-progress.md` es historial de apply. No se encontraron contradicciones no-rankables.
- **Cycle context**: este run solo movió artefactos del change (mv de filesystem; el directorio estaba 100% untracked `??`) — sin cambios de código en archive; el sync de specs, el move y este report quedan **uncommitted** para el paso de delivery del orchestrator.

## Gates at close

| Gate | Result | Evidence |
|------|--------|----------|
| Task Completion | ✅ PASS — 21/21 checked, 0 unchecked; sin reconciliación | `tasks.md` persistido (`grep -c "- [x]"` = 21, `"- [ ]"` = 0); dispatcher `taskProgress 21/21 allComplete true` |
| CRITICAL issues | ✅ PASS — 0 CRITICAL / 0 blockers | `verify-report.md` `pass_with_warnings` (`critical_findings: 0`) |
| Sync | ✅ PASS — ya aplicado por la fase sync nativa; archive verificó, no re-aplicó | `openspec/specs/review/spec.md` `sha256:5c2d86a9cafdcfdfd54793cfa5ec1ffef27a514e3a255735a98e24592e39acdf`, 21 requirements / 62 scenarios; `dependencies.sync: all_done` |
| Native review receipt (RDD) | ✅ PASS con aprobación explícita — RDD habilitado, cero `blockedReasons`; sin `reviewGate` en este store (biggz no lo emite — `sdd-status-derivation.md`); receipt del feature verificado en el repo disposable: `allowed:true`, `burned/unmanaged` | `biggz rdd status` → enabled/default; `review gate pre-pr review-d8e94d2af673b09d` (smoke repo) → `allowed: true`; checkpoint maintainer 2026-09-12 "Proceed — sync + archive" |
| Dispatcher (autoridad final) | ✅ en archive: `archive: ready`, `apply/verify/sync: all_done`, `nextRecommended: archive`, `review_disabled: false`, sin `blockedReasons` | `biggz sdd-status --json`; `phaseInstructions.archive`: "Archive only when verify-report.md exists and every task checkbox is complete" — ambas satisfechas |
| Session-summary guard | ✅ sin bloqueo (ningún `blocked(session_summary_missing)` en status) | `biggz sdd-status --json` (sin `blockedReasons`) |
| Action context | ✅ repo-local; escrituras confinadas a `openspec/changes/**` | `actionContext.mode: repo-local`, `allowedEditRoots: [repo root]` |

## Specs synced (Step 2 — aplicado por la fase sync nativa; verificado aquí, no re-aplicado)

| Domain | Delta action | Main spec | Numbers |
|--------|-------------|-----------|---------|
| review | 2 ADDED — `Pi Reviewer Execute Mode` (3 sc), `Bounded Reviewer Execution and Typed Transport Failures` (4 sc); 1 MODIFIED — `Verbatim Transport and Tool-Less Reviewer` (4 sc reemplazan 2) | `openspec/specs/review/spec.md` | `+69/−1`, 401 → 469 líneas |

**Totals**: 2 requirements agregados + 1 modificado, 0 removidos; requirements 19 → 21, scenarios 53 → 62. **Verificación**: los 3 requirements y 11 scenarios del delta presentes en la spec canónica (parser del repo: `ApplyDeltas(main, deltas) == main`, idempotente); sin headings duplicados ni residuos del bloque reemplazado; dominios fuera del delta intactos. **Merge destructivo aprobado**: el bloque MODIFIED (30 líneas > umbral 20 de `largeMutationThreshold`) fue aprobado por el maintainer (checkpoint 2026-09-12, "Proceed — sync + archive", dominio `review` únicamente). Archive **no modificó** `openspec/specs/**` — el sync ya estaba `all_done` antes de este paso (ediciones pendientes de commit para el orchestrator).

## Archived contents

- `exploration.md`, `research.md` (49,377 B, `biggz-ai.sdd-research/v1` rev 1, 37 fuentes, C1–C33), `ln.md` (`biggz-ai.sdd-ln/v1` rev 1), `preproposal.md` (`biggz-ai.sdd-preproposal/v1` rev 3, `proposal_ready: true`) ✅
- `proposal.md`, `specs/review/spec.md` (delta: 2 ADDED + 1 MODIFIED), `design.md` ✅
- `tasks.md` (21/21 `[x]`), `apply-progress.md` (slices PR1–PR3 + smoke orquestado + regresiones), `verify-report.md` (`pass_with_warnings`, 0 CRITICAL), `sync-report.md` (`applied`, dominio review) ✅
- `review-subject.json` (`{"repository":"C:/Users/USER/Desktop/biggz-ai","commit_sha":"HEAD"}`), `_meta.yaml` (metadata del change: `phase: explore`, `status: active`, `type: feature`) ✅
- `archive-report.md` ✅ (este archivo; también espejado a BigMem `sdd/wire-pi-review-relay/archive-report`)
- **Sin `state.yaml`** — ninguno existía en el directorio activo (el checkpoint aprobado quedó en sesión/BigMem; no se fabricó uno). **Sin `.biggz-instance`** — no existía marker en el change; el `mv` preservó todo byte a byte (13/13 hashes idénticos pre/post).
- Integridad: hashes SHA-256 pre-move == post-move para los 13 archivos (incl. `verify-report.md` `7a5d0faa…` y el delta `816e523c…`).

## Post-archive hygiene (Step 3b)

- **Non-TTY (sub-agente) → no se eliminó nada** (contrato: exit 0 sin borrado).
- Candidatos: `git branch -vv` → **0** ramas con upstream `[gone]`; `git worktree list` → **1** worktree (raíz del repo, `d0cd87c6 [master]`). No se ejecutó `fetch --prune` (contrato non-TTY; sin candidatos que podar).
- `.biggz-instance`: no existía en el change; nada que preservar (el `mv` preserva todo lo que exista — no aplica).

## Follow-ups for future work (not blockers)

- **Delivery (orchestrator, siguiente paso, requiere autorización explícita del maintainer)**: el código + asset + normalización de docs + sync de la spec canónica + este move + report están **uncommitted** (HEAD `d0cd87c6` + dirty). PR1/PR2 llevan `size:exception` (582/430 vs 400); cadena `stacked-to-main`; hace falta abrir una issue para el `closes #N` del body del PR.
- **W-180s (preexistente, carry)**: el runner configurado `-timeout 180s` flakea en `internal/review` en esta máquina (~178–180 s de duración de paquete); usar `-timeout 900s` aquí.
- **Kill POSIX-only**: `TestPiAdapter_Review_DeadlineFailsClosed` se SKIPea en Windows; la ruta deadline/cancelación queda cubierta cross-platform vía seams del executor.
- **Smoke re-run**: repetir el reviewer real requiere consentimiento fresco; la política burn borró el receipt efímero (solo su hash es re-verificable contra `burned.json`).
- **Binario stale en PATH**: el `biggz` de PATH (Sep-7) precede `--materialize`; usar `./biggz.exe` hasta reemplazarlo. Además: cualquier cambio en `internal/assets/**` exige rebuild del CLI (FS embebido congelado al build — causa del primer smoke post-PR3 fallido).
- **Clasificación PR3 (4.5)**: solo `*ArtifactAdmissionError` se clasifica tipado; errores envueltos de `captureValidatePayload` conservan la línea `error:` plana que el ladder de diagnóstico del design espera.

## Evidence refs (raw)

- `biggz sdd-status --json` (pre-move) → `taskProgress {total:21, completed:21, allComplete:true}`, `dependencies {proposal…verify: all_done, sync: all_done, archive: ready}`, `nextRecommended: "archive"`, `review_disabled: false`, sin `blockedReasons`
- `biggz sdd-status --json --instructions` → `phaseInstructions.archive`: "Archive only when verify-report.md exists and every task checkbox is complete."
- `biggz rdd status` → `RDD Status: enabled` (Global enabled, Source default)
- Smoke repo (`%TEMP%\biggz-smoke-relay`): `biggz review gate pre-pr review-d8e94d2af673b09d --json` → `{"allowed":true,"delivery":"burned/unmanaged","reason":"review burned: receipt is ephemeral and burned after finalize; delivery via ordinary repository policy"}`; `review status` → 5 eventos, `chain_valid: true`, receipt persistido
- `biggz sdd-attempt status wire-pi-review-relay` → revision `e4bb396a…`, `Attempts: 1`, `Active: 0`, `Complete: true`
- Spec canónica: `sha256sum openspec/specs/review/spec.md` → `5c2d86a9…`; `grep -c '^### Requirement:'` → 21; `grep -c '^#### Scenario:'` → 62
- Delta: `grep -c` → 3 requirements / 11 scenarios (`specs/review/spec.md` `816e523c…`)
- Tasks: `grep -c "- [x]"` → 21; `grep -c "- [ ]"` → 0
- Move: `mv openspec/changes/wire-pi-review-relay openspec/changes/archive/2026-09-12-wire-pi-review-relay` — sin `git mv` (directorio untracked `??`); 13/13 SHA-256 idénticos pre/post
- Verificación post-move: `ls openspec/changes/` → solo `archive`; `git status --porcelain` → `?? openspec/changes/archive/2026-09-12-wire-pi-review-relay/`
- BigMem: `biggz bigmem save "sdd/wire-pi-review-relay/archive-report" <content> --type architecture --topic-key sdd/wire-pi-review-relay/archive-report --project biggz-ai` — el marker que la vista mergeada del dispatcher lee para reportar el change como archived
