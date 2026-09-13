# Design: wire-pi-review-relay

> **Revisión (remediation)** — el smoke real-`pi` invalidó **D1**: el prompt compuesto (2,936 B) no declara la forma de salida, así que el reviewer responde la pregunta del rol ("classify … for risk tier") y `Capture` lo rechaza con `error: reviewer artifact admission incomplete: reviewer payload contains no complete JSON object` (`artifact.go:689` vía `capture.go:321-324`). D1 se revisa abajo y se añade **D5 (output contract)**. Las decisiones de producto confirmadas (surface `--execute`, rol embebido, timeout con `--timeout`) NO se re-abren.

## Technical Approach

`--execute`, quinto modo de `capture-result`, reutiliza la binding (`Preflight` `capture.go:294` → `MaterializeReviewerTask` `materialize.go:74` → `Capture` `capture.go:417`) e inserta la ejecución: prompt binary-owned (rol + contrato de salida + task byte-idéntica) por stdin a `PiAdapter.Review` (`pi_adapter.go:107`), stdout crudo directo a `Capture`. Handshake pi (`ValidatePiAgent` `cli_review.go:1105` → `IsPiRelayAvailable` `pi_relay.go:54-69`) y RDD (`cli_review.go:1121`, `:1175`) son precondiciones; ningún fallo captura.

## Architecture Decisions

### D1 (revised) — Role + task composition

**Qué invalidó el smoke**: con `shared.md` ⊕ `r1-risk.md` ⊕ separador ⊕ task (2,936 B), el prompt ordena *clasificar*, no *emitir*: nada describe la salida admitida. `shared.md`/`r1-risk.md` son plantillas `text/template` que también consumen las heurísticas in-process (`lens/prompt.go:26,46`; `readability/lens.go:51`, `reliability/lens.go:52`, `resilience/lens.go:43`), y `MaterializeReviewerTask` emite sólo datos (`materialize.go:141-161`: `GENTLE_AI_REVIEW_BINDING` / `_CONTEXT` / `_NAME_STATUS` / `_NUMSTAT` / `_PATCH`).

| Opción | Tradeoff | Decisión |
|---|---|---|
| (A) rol verbatim + **contrato** + separador + task exacta | el contrato es la última instrucción antes de la task; cero transformación | ✅ |
| (B) render de las plantillas (inventario en cero o completo) | evidencia falsa o ~2× prompt (research: 478 s @1.58 MB) | ✗ |
| (C) strip de assets / assets nuevos de rol | prosa frágil; rompe el conteo de 6 plantillas | ✗ |

Mapping sin cambios (`shared` primero): `risk`→`r1-risk`, `readability`→`r2-readability`, `reliability`→`r3-reliability`, `resilience`→`r4-resilience`; `performance`/`dependencies` → refusal `role-unavailable` sin launch; `external`/unknown inalcanzables (`isSupportedLens` `artifact.go:50`, `ValidateArtifactSubject` `artifact.go:148-150`).

`prompt = shared ⊕ "\n" ⊕ role ⊕ contrato ⊕ "\n---\n" ⊕ MaterializeReviewerTask(binding)` — el rol conserva su partición opencode (overlay `sdd-overlay-single.json:300` + task reemplazada verbatim `review-result-artifacts.ts:21-22`), el contrato va **después** del rol (recencia) y la task cierra el prompt **byte-idéntica** (`HasSuffix`, invariante ya verde `reviewer_execute_test.go:95-97`). El contrato abre declarando que las secciones previas son material de reglas y cierra con "your entire response is that object": la única salida que satisface el prompt es el objeto admitido, y cualquier otra cosa es un run fallido que `Capture` rechaza sin capturar.

### D5 — Output contract: texto, forma y ubicación

**Forma admitida (target de enforcement, derivada de `ReviewerResult` `artifact.go:287-293` + `Admit` `artifact.go:459`)**: exactamente **un** objeto JSON en stdout, keys cerradas:

```json
{"subject_hash":"sha256:…","inspection":{"status":"completed","paths":["a/b.go","c.go"]},
 "lens":"risk","findings":[{"id":"R1-001","lens":"risk","location":"a/b.go:42","severity":"CRITICAL",
 "claim":"…","proof_refs":["…"],"evidence_class":"inferential","causal_disposition":"introduced"}],
 "evidence":["…concrete…"]}
```

- `subject_hash`: echo literal del `subject_hash` de la línea `GENTLE_AI_REVIEW_BINDING`; ausente → `incomplete` (`:514`), distinto → `binding_mismatch` (`:517`).
- `inspection.status` = `"completed"` (`:520`); `inspection.paths` = **todos** los `path` de `GENTLE_AI_REVIEW_CONTEXT.changed_path_manifest`, canónicos y en orden ascendente (`admitValidateInspection` `:525-541`: no canónico/desordenado/duplicado → `out_of_scope`, faltante/extra → `incomplete`).
- `lens` opcional; si se incluye DEBE igualar la lente seleccionada (`capture.go:333-334`); la lente efectiva sale del binding (`capture.go:344`).
- `findings`: array explícito (nunca omitido; vacío = clean) y `evidence`: array **no vacío** de líneas concretas (`:313`; placeholders `none|n/a|tbd|pass|…` → `incomplete` `canonicalizeEvidenceItems` `:377` · `isConcreteEvidence` `:935`, y frases "could not inspect"/"not inspected" → `incomplete` `admitCheckUnavailableEvidence` `:545-552`). Caso clean canónico: `findings: []` + evidencia del barrido (`fixtures/lens-result-event.fixture.json`: `canonical_payload.findings: []`).
- Finding: `id` contra `^R[1-6]-…$` (`artifactFindingID` `:642`) con prefijo de la lente (`:574`), `severity` ∈ BLOCKER|CRITICAL|WARNING|SUGGESTION, `claim` no vacío, `location` `repo/relative/path:<línea>` dentro del manifiesto (`admitValidateSingleFinding` `:572-590`); IDs duplicados → `ambiguous`; **cada finding severo (BLOCKER/CRITICAL) exige `evidence_class` ∈ deterministic|inferential|insufficient y `causal_disposition` ∈ introduced|behavior-activated|worsened|pre-existing|base-only|unknown** (`:592`); `candidate_causal_finding_ids` se derivan de severos con disposición causal.
- Rechazos: sin objeto JSON completo / llaves sin cerrar → `incomplete` "no complete JSON object" (`:689`, el fallo actual); >1 objeto (p. ej. re-declarar la forma) → `ambiguous` (`:692`); campos desconocidos y múltiples valores → decode error (`decodeReviewerResult` `:758-761`) — por eso `status`, `summary`, `result_hash`, `selected_order` del overlay quedan prohibidos; `<task_result>` no-JSON → `nested_envelope` (`:662-679`).

| Ubicación del contrato | Tradeoff | Decisión |
|---|---|---|
| (a) asset nuevo `internal/assets/prompts/review/output-contract.md` | denso con las plantillas, pero **rompe dos conteos de 6** en dos paquetes (`review/lens/prompt_test.go:17`; `skillregistry/resolver_prompt_test.go:16,25`) → tocar un paquete ajeno | ✗ |
| (a′) asset nuevo **`internal/assets/prompts/review-output-contract.md`** (hermano de `review/`, con precedente en `prompts/review-validator.md`, que el overlay referencia) | el composer lee dos rutas; conserva el invariante "`prompts/review/` = 6 plantillas"; **cero ediciones de test por ubicación**; ya cubierto por `//go:embed all:prompts` (`embed.go:7`) | ✅ |
| (b) constante Go en `reviewer_execute.go` | sin blast radius, pero saca prosa del patrón de assets y del seam `assets.FS` | ✗ |
| (c) reusar el texto del overlay (`sdd-overlay-single.json` → `agent["review-risk"].prompt`) | es data JSON del plugin opencode, no un asset Go; su ledger incluye `status` (campo desconocido → rechazo) y termina "If clean, say exactly: No findings." — prosa que reproduce el fallo actual | ✗ |
| (d) extender los assets de rol | el texto entra al render de las heurísticas in-process (`lens/*/lens.go`) y cambiaría comportamiento verde | ✗ |

Blast radius: **opencode path intacto** (ninguna ref `{file:./prompts/...}` nueva; el test de overlay sólo verifica refs existentes y no tiene chequeo inverso de huérfanos, `prompt_overlay_contract_test.go:12-37`; `internal/components/prompts.go:35` camina sólo `prompts/sdd`). En Go sólo cambia `ComposeReviewerPrompt` (+ su test): nada en CLI, admission, `Capture` ni heurísticas.

### D2 — Windows `pi` resolution (sin cambios)

`LookPath("pi")` (`pi_adapter.go:162-174`, default `:99`): Go 1.25 spawnea shims `.cmd`/`.bat`; argv fijo sin metacaracteres (`:125-126`). Override `BIGGZ_PI_REVIEW_RELAY_EXECUTABLE` (absoluto, nunca shell). Smoke en apply: `where pi` + `pi --version`, luego execute real; `kind=launch` → reintentar con override; `kind=empty-output` → registrar, nunca capturar.

### D3 — Timeout (default intacto, +1 nota de tamaño)

Default 600 s; `--timeout <seconds>` entero 1..7200 (techo C12); `0`, negativo, no entero, >7200 o sin `--execute` → usage error. Kill al vencer (`CommandContext` + `WaitDelay`, `pi_adapter.go:128` · `piReviewerWaitDelay` `:25`); ctrl-c vía `signal.NotifyContext`. **Interacción con D5**: el contrato añade ≈1.2–1.8 KB; el prompt del smoke pasa 2,936 B → ~4.5 KB, y sobre el datapoint de campo (478 s @1.58 MB) el crecimiento es ~0.1%. El default confirmado NO cambia y no hay evidencia nueva que lo contradiga.

### D4 — Diagnostics on zero bytes (sin cambios)

`ReviewerFailure{Kind, Stage, Elapsed, ExitCode, StdoutBytes, StderrBytes, Cause}`; kinds `launch|timeout|canceled|empty-output|nonzero-exit|output-over-cap|role-unavailable`. MUST NOT: capturar, slot parcial, resultado fabricado, truncar.

## Data Flow

```
--agent pi --execute [--timeout s]
 ├ usage matrix → handshake (`:1105`) → RDD (`:1121`,`:1175`)
 └ ExecutePiReview → ComposeReviewerPrompt (shared ⊕ role ⊕ CONTRACT ⊕ sep ⊕ task)
    → WithTimeout+NotifyContext → adapter.Review (stdin=prompt, scratch)
    ├ raw > ArtifactResultLimit (artifact.go:35) → output-over-cap
    └ Capture(binding, raw) :417 — validate→extract→strict decode→Admit→CAS→append [sin cambios]
        └ rechazo de admisión → error plano (executor no reclasifica) → exit 1
```

## File Changes (remediation slice)

| File | Action | Description | Size |
|---|---|---|---|
| `internal/assets/prompts/review-output-contract.md` | Create | contrato literal (forma JSON + reglas de rechazo), sin sintaxis de plantilla | ~45 L |
| `internal/review/reviewer_execute.go` | Modify | const del asset, lectura vía `assets.FS`, tercer segmento + capacidad (`:31-86`: `:32` dir, `:69-73` lecturas, `:81-86` append) | +~12 |
| `internal/review/reviewer_execute_test.go` | Modify | `composeReviewerPromptWant` a 4 segmentos + test de marcadores/orden (:49-56, :75-106) | +~45 |

Total ≈ **100 líneas**: tracked ≈ 57 (asset nuevo ~45 L + `reviewer_execute.go` +~12), test ≈ 45 → **400-line budget risk: Low**, sin `size:exception`. Se preservan las filas de D1 para PR1/PR2 (ya entregadas).

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit (RED) | contrato presente entre rol y separador; orden `shared`<`role`<`contract`<`sep`<`task`; `HasSuffix` + igualdad byte a byte de 4 segmentos; marcadores estables (`"subject_hash"`, `"inspection"`, `"findings"`, `"evidence"`, `"evidence_class"`, `"causal_disposition"`, "exactly one JSON object"); asset sin `{{` | `reviewerAsset(t, "review-output-contract.md")` (`reviewer_execute_test.go:40`) + fake runner |
| Integration | sin cambios: shim `pi` replay golden `subject_hash`; cada kind captura cero | `cmd/biggz/review_execute_test.go` |
| E2E | smoke real-`pi` re-run (orchestrator, repo desechable): `review start` → `capture-result --execute --timeout 180` | éxito = exit 0 + `CapturedArtifact{admission_decision: completed, subject_hash, result_hash, manifest_path}` (`capture.go:238-250`) |

**Ladder de diagnóstico del smoke** (el orchestrator clasifica desde la salida tipada):

| Observación | Lectura |
|---|---|
| `kind=nonzero-exit` + `exit_code=N` (línea tipada `cli_review.go:1193`) | transporte/modelo, no admisión: revisar stderr del cause, flags, `--no-tools` |
| `error: reviewer artifact admission incomplete: reviewer payload contains no complete JSON object` | el contrato no se siguió (prosa, no objeto, o llaves sin cerrar) → endurecer D5, re-run |
| `error: reviewer artifact admission ambiguous: … multiple JSON objects` | el reviewer re-declaró la forma → prohibición explícita de repetirla |
| `error: reviewer artifact admission <decision>: <diagnostic>` | regla exacta violada (echo de `subject_hash`, cobertura de manifiesto, evidencia severa) — el diagnostic nombra la regla |
| `error: decode reviewer result: json: unknown field "…"` | el reviewer copió campos del overlay (`status`, `summary`, `result_hash`) → cerrar keys |
| `kind=output-over-cap` / `kind=empty-output` | presupuesto de stdout / ausencia de salida final |

Éxito parcial (transporte OK, admisión rechazando) se diagnostica por esa tabla: el error de `Capture` no es `ReviewerFailure`, así que sale como línea `error:` plana (`cli_review.go:1197-1199`) — precisamente la firma del fallo actual.

## Interfaces / Contracts

`internal/review` — sin cambios de firma:

```go
const ReviewerOutputContractAsset = "prompts/review-output-contract.md" // nuevo
func ComposeReviewerPrompt(binding CaptureBinding) ([]byte, error)      // segmento extra
```

CLI, admission, CAS y slot: sin cambios. `prompts/review/` sigue siendo el set de 6 plantillas.

## Threat Matrix

| Boundary | Applicability | Reason |
|---|---|---|
| Documentation-like paths | N/A | tool-less |
| Git repository selection | N/A | sin selectores |
| Commit state | N/A | sin commits |
| Push state | N/A | sin push |
| PR commands | N/A | sin PR |

Límite de proceso (shim/argv/stdin/kill): D2/D4 + RED.

## Proposed Spec Delta (para la fase spec — no aplicado aquí)

Sí hace falta delta: `REQ-VERBATIM` lista `internal/assets/prompts/review/*.md`, glob que **no** cubre `prompts/review-output-contract.md`, así que el texto vigente prohíbe el fix. Delta mínimo (MODIFIED, con los escenarios existentes preservados):

```markdown
### Requirement: Verbatim Transport and Tool-Less Reviewer

Hosts MUST forward the materialized bytes to the reviewer unchanged; in the execute route the materialized task MUST travel byte-identical inside the runtime-composed prompt. The reviewer MUST run without repository tools (no live worktree inspection), and the prompt MUST be composed from binary-owned reviewer prompt assets — the reviewer role assets (`internal/assets/prompts/review/*.md`) plus the binary-owned output-contract asset (`internal/assets/prompts/review-output-contract.md`) — together with the materialized task, never caller-authored. The output-contract asset MUST state the admitted result shape (subject echo, completed inspection of the frozen manifest, findings, concrete evidence, severe-finding evidence class and causal disposition), MUST NOT be a Go template, and MUST appear after the role text and before the materialized task. Model/provider selection MUST remain ambient, never pinned by the binary.

(Previously: only caller-authorship was prohibited; no output contract.)

#### Scenario: Verbatim transport and tool-less run
(unchanged)
#### Scenario: Caller-authored prompt discarded
(unchanged)
#### Scenario: Composed prompt keeps the segment byte-identical
(unchanged)

#### Scenario: Composed prompt carries the output contract before the task

- GIVEN a runtime-composed reviewer prompt for any executable lens
- WHEN the prompt is inspected
- THEN it MUST contain the binary-owned output-contract section naming the admitted result shape (subject echo, completed inspection, findings, concrete evidence)
- AND that section MUST appear between the role text and the materialized task segment
```

## Migration / Rollout

Sin migración: opt-in, gateado por el handshake. Rollback: revertir el commit — aditivo (un asset + un segmento de prompt), sin esquema ni estado nuevo; los eventos siguen siendo `lens_result`.

## Open Questions

- **Orden de `inspection.paths`** (pre-existente): `admitValidateInspection` exige orden ascendente (`artifact.go:527-528`) **y** igualdad con el orden del manifiesto (`:539-540`); con manifiestos no lexicográficos ninguna submission pasaría. En la práctica el tree-diff de git es lexicográfico (`TestAdmit_HappyPath` usa `a.txt,b.txt`). El contrato instruye orden ascendente; si el smoke devuelve `incomplete: did not cover the complete frozen path manifest`, es esto y escapa a esta remediación.
- Especialización del contrato por lente (hoy un asset único) y su relación con el overlay opencode: diferido; no condiciona el smoke.
