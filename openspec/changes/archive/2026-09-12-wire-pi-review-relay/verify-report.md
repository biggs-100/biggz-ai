```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:6f44c5010dc49ebc69ccd6eacce94961653598106591708335dc1949f997fa91
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 3/3
scenarios: 11/11
test_command: go test ./... -count=1 -timeout 900s
test_exit_code: 0
test_output_hash: sha256:6f44c5010dc49ebc69ccd6eacce94961653598106591708335dc1949f997fa91
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: `wire-pi-review-relay` — verificación final (specs → design → tasks → code → tests) sobre el árbol de trabajo **sin commitear** (HEAD `d0cd87c6`, working tree `+dirty`).
**Version**: N/A — delta con 3 requirements / 11 scenarios contados desde `openspec/changes/wire-pi-review-relay/specs/review/spec.md`.
**Mode**: Standard — `strict_tdd: false`; no se cargó el módulo strict-TDD. Ledger: token del orquestador `tok-72fa0cb9c79d13db15190d14` (work unit `verify-change`); **esta corrida no adquirió ni settleó** y liga su evidencia a ese token. `evidence_revision` = SHA-256 del output canónico del sweep completo (el valor contra el que el orquestador settlea). RDD habilitado en el workspace (`review_disabled: false`); `review-subject.json` escrito para el producer command ofrecido. Sin commits, sin push, sin edición de código por esta corrida.

**Revision under verification**: HEAD `d0cd87c6cb861a4ecfa3665ac48652498212cf74`; superficies modificadas/nuevas: `internal/review/reviewer_execute.go` (+ test), `internal/review/pi_adapter.go` (+ `pi_relay_test.go`), `cmd/biggz/cli_review.go` (+ `review_execute_test.go`), `internal/assets/prompts/review-output-contract.md`, normalización de docs (`README.md`, `sdd-status-contract.md`, `report-format.md`).
**Evidence provenance (honest scope)**: esta corrida re-ejecutó build, vet, tests enfocados, regresiones nombradas, sweep completo y el runner configurado; re-verificó el smoke real-`pi` desde los bytes persistidos (una re-ejecución del modelo requiere consentimiento fresco y no se hizo); re-ejecutó el `gate pre-pr` del smoke. El baseline de runtime del paquete (~131.5 s) proviene de `apply-progress.md` y no fue re-ejecutado (requeriría un checkout limpio); se cita como provenance, no como observación propia.

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 21 |
| Tasks complete | 21 |
| Tasks incomplete | 0 |

Los 21 checkboxes de `tasks.md` están `[x]` (fases 1–4). El estado estructurado (`biggz sdd-status --json`) confirma `taskProgress: 21/21`, `allComplete: true`, `nextRecommended: verify`.

### Build & Tests Execution
**Build**: ✅ Passed — `go build ./...` exit 0 (salida vacía → hash del stream vacío); `go vet ./internal/review ./cmd/biggz` exit 0 sin hallazgos.
```text
go build ./...            → exit 0 (quiet: output hash = SHA-256 of empty bytes)
go vet ./internal/review ./cmd/biggz → exit 0 (no findings)
```

**Tests**: ✅ — sweep completo exit 0: 60 paquetes `ok`, 0 `FAIL`.
```text
go test ./... -count=1 -timeout 900s → exit 0; 60 ok; 0 FAIL
go test ./internal/review -run 'TestMaterialize|TestCapture|TestPluginWiresCapture|TestRDDParity' -count=1 → ok 12.327s
go test ./internal/review -run 'TestComposeReviewerPrompt|TestExecutePiReview|TestPiAdapter|TestPiRelayHandshake' -count=1 -v → ok 8.361s (18/18 top-level PASS; 1 SKIP POSIX-only)
go test ./cmd/biggz -run TestReviewExecute -count=1 -v → ok 7.295s (3/3 top-level; 17 subtests PASS)
```

| Command | Exit | Observed | Output hash |
|---------|------|----------|-------------|
| `go test ./... -count=1 -timeout 900s` | 0 | 60 paquetes `ok`, 0 `FAIL` | `sha256:6f44c5010dc49ebc69ccd6eacce94961653598106591708335dc1949f997fa91` |
| `go test ./internal/review -run 'TestMaterialize\|TestCapture\|TestPluginWiresCapture\|TestRDDParity' -count=1` | 0 | `ok 12.327s` | `sha256:0cbc8d4fec4ea315fde4a5e3a939f3641857ee5b9e3f2f387cdf5e6ff1d67c58` |
| `go test ./internal/review -run 'TestComposeReviewerPrompt\|TestExecutePiReview\|TestPiAdapter\|TestPiRelayHandshake' -count=1 -v` | 0 | `ok 8.361s`; 18 top-level PASS, 1 SKIP | `sha256:7f517289097801a907a0cd4df2391431de47174b191bacb779e39352cf0ef698` |
| `go test ./cmd/biggz -run TestReviewExecute -count=1 -v` | 0 | `ok 7.295s`; 3 top-level, 17 subtests PASS | `sha256:785927b7108449815be87164886f81b08a32b13e14769dfea9b94fa77eaa169c` |
| `go build ./...` | 0 | salida vacía | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `go vet ./internal/review ./cmd/biggz` | 0 | sin hallazgos | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `go test ./... -count=1 -timeout 180s` (runner configurado) | 1 | `panic: test timed out after 3m0s`; `FAIL …/internal/review 180.226s` | `sha256:a1eac1e9b8ca0ba8fed0ce11760f5f8b08af7f1b4bec757851e880ea30717357` |

**Coverage**: ➖ Not available — no hay gate de cobertura en este plano del change; no se midió.

**Timeout note (WARNING, detalle en Issues)**: con el runner configurado (`-timeout 180s`) el paquete `internal/review` excede el presupuesto por paquete y la alarma de `testing` vence en medio del paquete: `panic: test timed out after 3m0s`, `FAIL github.com/biggs-100/biggz-ai/internal/review 180.226s`; en el sweep completo (900 s) el mismo paquete corre `ok 178.050s`. El test nombrado por la alarma depende del timing (esta corrida: `TestStartFreezesPlannedSelection`; la corrida de apply observó `TestStoreGitCommonDir`); ambos casos caen en archivos preexistentes no tocados por el change (`internal/review/store_test.go` y otros), lo que respalda la lectura "duración de paquete vs presupuesto", no "test roto".

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Pi Reviewer Execute Mode | Execute runs the pipeline | `cmd/biggz > TestReviewExecuteShimPipeline`; `internal/review > TestExecutePiReview_RawStdoutReachesCapture` | ✅ COMPLIANT |
| Pi Reviewer Execute Mode | Conflicting flags or missing preconditions refuse | `cmd/biggz > TestReviewExecuteUsageMatrix` (10 subtests); `TestReviewExecuteRefusals/missing_handshake_refuses_before_materialization`; `…/disabled_RDD_refuses_before_materialization` | ✅ COMPLIANT |
| Pi Reviewer Execute Mode | Admission failure captures nothing | `cmd/biggz > TestReviewExecuteRefusals/transport_and_admission_refusals_capture_nothing/admission_rejection_refuses_like_--input` | ✅ COMPLIANT |
| Bounded Reviewer Execution and Typed Transport Failures | Timeout and cancelation terminate the reviewer | `internal/review > TestExecutePiReview_TypedFailuresCaptureNothing/timeout`; `…/cancelation` (adaptador POSIX `TestPiAdapter_Review_DeadlineFailsClosed` → SKIP en este host, ver WARNING) | ✅ COMPLIANT |
| Bounded Reviewer Execution and Typed Transport Failures | Empty stdout and non-zero exit are typed failures | `internal/review > TestExecutePiReview_TypedFailuresCaptureNothing/empty_stdout`; `…/nonzero_exit`; `TestPiAdapter_Review_WithFakeBinary`; `TestPiAdapter_Review_NonzeroExitIsTyped`; `cmd/biggz > TestReviewExecuteRefusals/…/nonzero_exit_is_typed`; `…/empty_stdout_is_typed` | ✅ COMPLIANT |
| Bounded Reviewer Execution and Typed Transport Failures | Raw stdout reaches Capture unmodified | `internal/review > TestExecutePiReview_RawStdoutReachesCapture`; `cmd/biggz > TestReviewExecuteShimPipeline` | ✅ COMPLIANT |
| Bounded Reviewer Execution and Typed Transport Failures | Over-cap output refuses before capture | `internal/review > TestExecutePiReview_TypedFailuresCaptureNothing/output_over_cap`; `cmd/biggz > TestReviewExecuteRefusals/…/over-cap_output_is_typed` | ✅ COMPLIANT |
| Verbatim Transport and Tool-Less Reviewer (MODIFIED) | Verbatim transport and tool-less run | `cmd/biggz > TestReviewExecuteShimPipeline` (stdin dump cierra con la task byte-idéntica); `internal/review > TestPiAdapter_Review_FlagsAreComplete` (argv congelado: `--no-tools`, sin `--model`/`--provider`) | ✅ COMPLIANT |
| Verbatim Transport and Tool-Less Reviewer (MODIFIED) | Caller-authored prompt discarded | `internal/review > TestComposeReviewerPrompt_SharedThenRoleThenByteIdenticalTask` (igualdad byte a byte: no hay canal caller-authored); `cmd/biggz > TestReviewExecuteUsageMatrix` (`--input` exclusivo con `--execute`) | ✅ COMPLIANT |
| Verbatim Transport and Tool-Less Reviewer (MODIFIED) | Composed prompt keeps the segment byte-identical | `internal/review > TestComposeReviewerPrompt_SharedThenRoleThenByteIdenticalTask`; `cmd/biggz > TestReviewExecuteShimPipeline` | ✅ COMPLIANT |
| Verbatim Transport and Tool-Less Reviewer (MODIFIED) | Composed prompt carries the output contract before the task | `internal/review > TestComposeReviewerPrompt_CarriesOutputContract`; orden `shared<role<contract<sep<task` en `TestComposeReviewerPrompt_SharedThenRoleThenByteIdenticalTask` | ✅ COMPLIANT |

**Compliance summary**: 11/11 scenarios compliant — cada scenario tiene al menos un test pasante ejecutado por esta corrida.

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|------------|--------|-------|
| Pi Reviewer Execute Mode | ✅ Implemented | `--execute` validado por uso inmediatamente tras el parseo (exclusivo con `--input`/`--preflight`/`--materialize`, solo `--agent pi`), luego handshake → RDD → binding → `ExecutePiReview`; imprime `kind/stage/elapsed/exit_code/stdout_bytes/stderr_bytes` + "no capture was performed" con exit 1. |
| Bounded Reviewer Execution and Typed Transport Failures | ✅ Implemented | `DefaultReviewerTimeout=10m`, `MaxReviewerTimeout=2h`; `--timeout` entero 1..7200; `CommandContext`+`WaitDelay`; cancelación ctrl-c vía `signal.NotifyContext`; cap `ArtifactResultLimit` aplicado antes de `Capture` y sin truncar; toda falla retorna antes de `Capture`. |
| Verbatim Transport and Tool-Less Reviewer (MODIFIED) | ✅ Implemented | `ComposeReviewerPrompt` = `shared ⊕ "\n" ⊕ role ⊕ contract ⊕ "\n---\n" ⊕ MaterializeReviewerTask`; contrato leído vía `assets.FS` desde `prompts/review-output-contract.md` (no-template, post-rol, pre-task); `prompts/review/` conserva sus 6 plantillas; el argv del adapter no fija modelo/proveedor. |

**Preterminal smoke real-`pi` (re-verificado, no re-ejecutado)**: lineage `review-d8e94d2af673b09d`, candidate `a29f2fadbea5357e1e9ab27f31512a79b48756e4`, lens `risk`, `admission_decision: completed`. El evento `lens_result` es content-addressed (`sha256(bytes) == ae997825dd38c2c032c4dd2d54c1520ce30bf4a6c4335ff223edf7655cc4749d`, verificado con `sha256sum`); payload con `canonical_payload_sha256: sha256:3f46ec572070051b5fc35b4d3e8c12a7dde84f02ac433a020a0d4a5f600bc84d` y `manifest_sha256` del manifest congelado. Receipt `sha256:f0ee681b64e00da44ae5b2f0c70e6f633effe4755efda24a6dcefb60f7d3f088` registrado en los eventos `complete_review` y `burn_review` y en `burned.json` — verificable solo contra los registros persistidos porque la política burn borró el archivo efímero del receipt (no hay bytes que re-hashear). `gate pre-pr` re-ejecutado por esta corrida: `allowed: true`, `delivery: burned/unmanaged`. Binario usado por el smoke: `./biggz.exe` construido 2026-09-12 20:09:53 (`go version -m`: HEAD `d0cd87c6`+dirty), posterior a la última mtime de fuente/asset (19:55:07).

### Modern Go Guidelines
`use-modern-go` `list` (v0.1.1) fue consultado para los tres archivos Go tocados (`internal/review/reviewer_execute.go`, `internal/review/pi_adapter.go`, `cmd/biggz/cli_review.go`; salida idéntica de 46 guidelines, versión resuelta desde `go.mod` 1.25). El delta conforma: `t.Context()` en tests, `errors.Is`/`errors.As`, `bytes.Clone`, sin `interface{}`, sin copias redundantes de loop-var; no hay modernización obvia omitida → sin WARNING por esta dimensión.

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| D1 (revised) — Role + output contract + byte-identical task | ✅ Yes | Fórmula exacta implementada y asertada por orden (`shared@0 < role < contract < separator < task`). |
| D1 — Map de lentes + `role-unavailable` | ✅ Yes | `risk→r1-risk` … `resilience→r4-resilience`; `performance`/`dependencies` refusan tipado (`kind=role-unavailable`, `stage=role`) con cero launch y cero captura (test dedicado). |
| D2 — Windows resolution + override | ✅ Yes | `LookPath("pi")` + `BIGGZ_PI_REVIEW_RELAY_EXECUTABLE` absoluto, nunca shell (refusa relativo tipado); el smoke real resolvió `pi.cmd` sin override (C33 resuelto empíricamente). |
| D3 — Timeout default/override/kill | ✅ Yes | Default 600 s, `--timeout` 1..7200, kill por `CommandContext`+`WaitDelay`, cancelación por señal. |
| D4 — Diagnósticos tipados con cero bytes | ✅ Yes | `ReviewerFailure{Kind,Stage,Elapsed,ExitCode,StdoutBytes,StderrBytes,Cause}`; kinds `launch\|timeout\|canceled\|empty-output\|nonzero-exit\|output-over-cap\|role-unavailable`; sin captura, sin slot parcial, sin truncar. |
| D5 — Output contract asset | ✅ Yes | Asset hermano `internal/assets/prompts/review-output-contract.md` (42 L / 3,620 B), leído por `assets.FS`, no-template, entre rol y task; `prompts/review/` sigue en 6 plantillas. |

**Desviaciones observadas (no rompen spec)**:
- **Tamaño del contrato**: el design estimó ≈1.2–1.8 KB de contrato (prompt resultante ≈4.5 KB); el asset real es 3,620 B y el prompt del smoke pasó de 2,936 B a ≈6.5 KB (+0.23% sobre el datapoint de campo 1.58 MB). La decisión de D3 (default 10 min) no cambia; impacto de timeout despreciable.
- **Diagnosis ladder del smoke**: el ladder del design no incluía el caso "binario stale": el primer smoke post-PR3 falló con la firma "no complete JSON object" (fila 2 del ladder) y la causa real fue un `./biggz.exe` construido antes de crear el asset (el FS embebido se congela al build); se resolvió reconstruyendo el CLI. `apply-progress.md` documenta la lección; el ladder queda incompleto como guía, sin efecto sobre el resultado final.
- **Budget de slices (apply)**: PR1 582 vs 400, PR2 430 vs 400, PR3 145 vs ≈100 estimado (cap 200) — desviaciones de apply registradas en `apply-progress.md`; no afectan esta verificación.
- **Tensión host-mediated**: `pi_relay.go`/`cli_review.go` declaran ahora el modo execute explícito (el CLI es ejecutor para pi en esta ruta); documentado como decisión, no como accidente.

### Issues Found
**CRITICAL**: None
**WARNING**:
1. **Runner configurado flaky (preexistente, no regresión)**: `go test ./... -count=1 -timeout 180s` → exit 1 con `panic: test timed out after 3m0s` y `FAIL …/internal/review 180.226s`; en el sweep de 900 s el paquete corre `ok 178.050s`. Es presupuesto de paquete (duración ~178–180 s bajo carga) contra un límite de 180 s; el test nombrado al vencer la alarma depende del timing y vive en archivos no tocados por el change. El change agrega tests al paquete (baseline ~131.5 s según apply-progress → 140–178 s observados) y estrecha el margen: en esta máquina el ceiling honesto es 900 s.
2. **Superficies sin commitear**: las cuatro superficies de código + asset + normalización de docs están uncommitted (HEAD `d0cd87c6` +dirty). La verificación es válida solo para esos bytes exactos del working tree; nada fue commiteado por esta corrida.
3. **Reproducibilidad del smoke**: re-ejecutar el reviewer real requiere consentimiento fresco (no ejecutado); además, la política burn borró el receipt efímero, así que solo su hash es re-verificable contra los registros persistidos.
4. **Kill de proceso real POSIX-only**: `TestPiAdapter_Review_DeadlineFailsClosed` se SKIPea en Windows (`deadline test uses POSIX sh`); en este host la ruta tipada de deadline/cancelación quedó cubierta por el seam del executor (`TestExecutePiReview_TypedFailuresCaptureNothing/timeout|cancelation`) y por `exec.CommandContext` en el adapter, pero la terminación de un proceso real no se observó en la suite de este host.

**SUGGESTION**:
1. Añadir la fila "rebuild obligatorio del CLI tras cambios en `internal/assets/**` (FS embebido congelado al build)" al diagnosis ladder del design o al checklist de apply.
2. Actualizar la estimación de tamaño del contrato (3,620 B reales vs 1.2–1.8 KB estimados) si el design se retoma como referencia viva.
3. Al citar el flake de 180 s en reportes futuros, nombrar el test como timing-dependiente en vez de un test fijo.

### Verdict
**PASS WITH WARNINGS**
3/3 requirements y 11/11 scenarios con tests pasantes ejecutados por esta corrida; sweep completo exit 0 (60 paquetes `ok`); los WARNINGs son ambientales/provenance (flake del runner configurado, superficies sin commitear, consentimiento del smoke, kill POSIX-only) y no bloquean el avance, sujeto al gate RDD y al settle del ledger por el orquestador.
