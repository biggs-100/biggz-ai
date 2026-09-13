# Exploration: wire-pi-review-relay

## Current State

### Flujo end-to-end actual de un review hosteado en Pi (Q1)

1. **`review start --agent pi`** — gate del handshake en `cmd/biggz/cli_review.go:648-651`; sin `BIGGZ_PI_REVIEW_RELAY_CONTRACT` (o el compat gentle) refusa con `ErrPiRelayHandshake` antes de tocar repo/consent (`internal/review/pi_relay.go:48-51,82-87`).
2. **`review status <lineage> --contract biggz-ai.review-integration/v1 --next-transition`** — envelope `collect` con `lineage/target/lens/order/expected_revision` (`internal/review/contract.go:104-112`; `expected_revision = chain.HeadHash` en `:108`). Única autoridad de routing.
3. **`capture-result ... --materialize`** — imprime la reviewer task completa, read-only: markers `GENTLE_AI_REVIEW_BINDING`/`CONTEXT`/`NAME_STATUS`/`NUMSTAT`/`PATCH` (`internal/review/materialize.go:29-33`, composición `:74-160`). CLI: `cmd/biggz/cli_review.go:1153-1163`. Excluyente con `--input`/`--preflight` (`:1089-1102`).
4. **Ejecución del reviewer — EL GAP.** Ningún código de producción invoca `PiAdapter`: `NewPiAdapter()` no tiene callers fuera de tests (solo `internal/review/pi_adapter.go:33` y `pi_relay_test.go`). El operador/orquestador hand-hostea un proceso pi fresh bloqueado, le pasa los bytes y recupera su salida. La admisión está en el comentario `cmd/biggz/cli_review.go:1060-1065`: *"the capture path could materialize via PiAdapter.Review with the Go-issued opaque prompt and then submit the raw bytes through the existing --input path. The current minimal port keeps the existing file/stdin input path unchanged; the adapter is available for future materialize/execute routing when the provider prompt binding is added."*
5. **`capture-result ... --input <file>|-`** — `review.Capture(binding, payload)` (`internal/review/capture.go:417-466`; call site `cli_review.go:1171`): admission estricta + append del evento `lens_result`; re-lectura de stdin con cap `ArtifactResultLimit` (`cli_review.go:1241-1259`).
6. **`review finalize <lineage>`** (`cli_review.go:928-957`; **sin** flag `--agent`) → receipt (`internal/review/finalize.go:1-15`).
7. **`review gate pre-pr|pre-push <lineage>`** (RDD; `internal/review/gate.go`).

**Rollover CAS**: cada capture exitoso mueve el head; el próximo `collect` cambia `expected_revision` (`contract.go:108` re-lee `chain.HeadHash`); revision stale se rechaza (`capture.go` `captureAppendNewResult` compara `fresh.HeadHash != binding.ExpectedRevision`; tests `capture_test.go:236,303`).

**Intervención humana/orquestador hoy**: (a) declarar el env del handshake; (b) hostear al reviewer + aportar el rol/contrato del reviewer out-of-band + transportar bytes en ambos sentidos; (c) responder el consentimiento medium/high (`internal/review/consent_relay.go:158-196`); (d) finalize/gates. **Mecánico (ya lo hace el CLI)**: materialize, capture, status, finalize, gate. **No mecánico por diseño**: consentimiento (decisión humana). **No automatizado**: ejecutar al reviewer y capturar sus bytes (esto es el gap, no el consentimiento).

**Prior art dogfood (relay manual, real)**: `openspec/changes/archive/2026-09-11-fix-rdd-receipt-collection/apply-progress.md:109` (start → materialize 12,818 B → reviewer pi locked-down con flags `PiAdapter` → `--input` admission completed → finalize → gate allowed), nota honesta 2 en `:143`, y W-1 en `verify-report.md:120` / next step en `archive-report.md:69` ("add the production biggz-pi/gentle-pi host relay…").

### PiAdapter.Review y el "provider prompt binding" (Q2)

`internal/review/pi_adapter.go`:
- **Entradas**: `ctx`, `prompt string` opaco; seams de test `LookPath`/`CommandContext` (`:27-35`).
- **Ejecución**: resuelve `pi` por PATH (`:47-50`), scratch dir temporal borrado al salir (`:51-55`, `cmd.Dir = scratch` `:64`), flags discovery-disabled byte-for-byte (`:61-63`): `--print --mode text --no-session --no-tools --no-extensions --no-skills --no-prompt-templates --no-themes --no-context-files --no-approve`. Prompt por **stdin** para que argv nunca cargue material del provider (`:66`); stdout/stderr a buffers (`:67-68`).
- **Timeouts/exit**: `WaitDelay = 5s` (`:65`, const `:23`) para soltar `Wait` si un grandchild retiene el pipe de stdout; error de `ctx` preferido sobre el de exec (`:70-74`).
- **Salida**: bytes crudos de stdout **sin interpretar**; stdout vacío/whitespace → error tipado `"pi reviewer transport produced no final message"` (`:75-77`). Nunca trunca ni valida el contenido — eso queda a `Capture`.

**Qué es el "provider prompt binding" que falta** — la composición del prompt completo del reviewer pi y su liga a la captura:
- El artifact materializado **no lleva rol/instrucciones** del reviewer: solo binding + context + name-status + numstat + patches (`materialize.go:29-33,74-160`).
- En el host **opencode** el rol viene del overlay de agentes `review-risk|review-readability|review-reliability|review-resilience` con `"tools": {"*": false}` (`internal/assets/opencode/sdd-overlay-single.json:300-318`), y el plugin reemplaza el cuerpo de la task por los bytes materializados — *"the materialized task IS the reviewer prompt"* (`internal/assets/opencode/plugins/review-result-artifacts.ts`).
- En **pi** no existe overlay ni launcher equivalente (`internal/assets/pi/*` solo tiene extensiones TUI/session; `subagent-config.json` sin reviewers). El dogfood lo documenta: *"the materialized task carries only the binding/context/patches — production hosts supply the reviewer role from the agent overlay"* (`archive …/apply-progress.md:143`).
- Las plantillas `internal/assets/prompts/review/r{1..4}-*.md` existen pero hoy solo se renderizan como validación interna de las heurísticas in-process (`internal/review/lens/readability/lens.go:99`); **no hay binding a un modelo**.

Conclusión: el lado "binding" hacia la captura ya está definido por el literal `GENTLE_AI_REVIEW_BINDING` en los primeros bytes del artifact (`materialize.go:29`); lo no definido es **el rol/contrato del reviewer para pi y quién lo emite** en la ruta execute. Decisión de producto → proposal/design.

### Handshake (Q3)

- Constantes y env: `biggz-pi.review-relay/v1` + `gentle-pi.review-relay/v1` compat (`pi_relay.go:26,30`), vars `BIGGZ_PI_REVIEW_RELAY_CONTRACT` / `GENTLE_PI_REVIEW_RELAY_CONTRACT` (`:33,36`).
- Elegibilidad: `IsPiRelayAvailable()` acepta el valor exacto bajo cualquiera de las dos vars, en cruz, y rechaza cualquier otro valor o vacío (`:54-70`). `CurrentProducerHost()` devuelve `"pi"` solo si el handshake está declarado; si no, el host ambiental `opencode` (`internal/review/producers.go:149-153`; hosts en `:37`).
- Validación: `ValidatePiAgent` (`pi_relay.go:82-87`) invocado en `review start` (`cli_review.go:648`), `review status` (`:343`) y `review capture-result` (`:1055`). **`review finalize` no tiene `--agent`** (`cli_review.go:928-957`) y no gatea.
- Sin handshake: refusal tipada en esos 3 verbos; texto del error deliberadamente sin `=` ni `/` para sobrevivir el privacy gate (`pi_relay.go:44-51`; test `pi_relay_test.go:28-35`). El handshake lo exporta el host biggz-pi/gentle-pi "on every invocation it relays" (`pi_relay.go:12-17`) — host que **no existe en este repo** (W-1).

### Invariantes que una ruta execute NO puede cambiar (Q4)

1. **Envelope `biggz-ai.review-integration/v1`** (`contract.go:30-33,79-131`) como única autoridad; el `collect` input exacto (`ContractCaptureInput` `:54-62`) debe seguir siendo la fuente de binding.
2. **CAS `expected_revision`** re-leído tras cada capture; staleness rechazada.
3. **Materialización Go-owned determinista y read-only** (spec `review/spec.md:323-337`; `materialize.go:74-91`).
4. **Transporte verbatim + reviewer tool-less** (spec `review/spec.md:371-379`).
5. **Admission estricta**: `DisallowUnknownFields` + single-JSON (`artifact.go:662-707,758-770`), echo de `subject_hash`, inspección `completed` sobre todo el manifest, evidencia concreta, reglas de findings severos (`admit` `artifact.go:459+`).
6. **Slot inmutable + idempotencia**: recapture de bytes idénticos = no-op; bytes canónicos distintos = rechazo (`capture.go` sección occupied-slot; tests `capture_test.go:172,204`).
7. **Refusals no truncadas**: cap 4 MiB `ArtifactResultLimit` (`artifact.go:35`), `materialize_vacuous`/cap (`materialize.go:55-58`).
8. **Receipts/gates/budget/ledger** (`finalize.go:1-15`, `gate.go`, `ledger.go`) sin cambio de esquema.
9. **RDD kill-switch** en mutaciones (`cli_review.go:1068` + re-check contra `binding.Repo` antes de capturar) — execute es mutación.
10. **Producer parity + host opencode intactos** (`producers.go`, guard `internal/review/rdd_parity_test.go:14-100` incl. `PluginWiresCapture`).

## Affected Areas

- `cmd/biggz/cli_review.go` — routing de capture-result (`:1055-1183`), usage (`:1046,1050,1086,1103`), help (`:182-183`), gates start/status (`:343,648`).
- `internal/review/pi_adapter.go` — adapter sin caller productivo; seams `LookPath`/`CommandContext`.
- `internal/review/pi_relay.go` — handshake.
- `internal/review/materialize.go` / `capture.go` / `contract.go` — bytes, admission, envelopes.
- `internal/review/producers.go` — `SupportedReviewHosts` (`:37`), `CurrentProducerHost` (`:149`).
- `internal/assets/opencode/plugins/review-result-artifacts.ts` — transporte de referencia opencode (no regresar; guard `rdd_parity_test.go:102-123`).
- `internal/assets/pi/*` — sin launcher/agent de review hoy; `internal/assets/prompts/review/*.md` — plantillas de rol sin binding a modelo.
- Tests: `internal/review/pi_relay_test.go`, `materialize_test.go`, `lineage_identity_test.go`, `capture_test.go`, `rdd_parity_test.go`; `cmd/biggz/review_materialize_test.go`, `review_parity_test.go`.
- Specs: `openspec/specs/review/spec.md` (`:323,371,387`) — requiere delta nuevo.

## Approaches

| # | Approach | Pros | Cons | Effort |
|---|----------|------|------|--------|
| a | **`capture-result --agent pi --execute`** (modo explícito, exclusivo con `--input/--preflight/--materialize`): `Preflight → MaterializeReviewerTask → PiAdapter.Review(prompt) → Capture(binding, raw)` | Reutiliza binding/preflight/Capture exactos; menor superficie (un verbo); es la ruta ya insinuada por el comentario `cli_review.go:1060-1064`; PiAdapter y seams de test existen; simetría con `--materialize`; host opencode intacto | Amplía semántica de capture-result (hoy 4 modos excluyentes, `:1089-1102`); ejecución bloqueante dentro de un verbo de captura; necesita el prompt-binding (rol reviewer); stdout hoy sin cap en el adapter; tensión con el relato "host-mediated" de `pi_relay.go:12-17` (allí el ejecutor es el launcher del host) | Medium |
| b | **Nuevo verbo `biggz review collect --agent pi`** | No toca capture-result; nombre alineado al `collect` del contrato; extensible (futuro opencode); puede llevar selección explícita de rol/prompt | Superficie nueva (dispatch `reviewRun` `:89-150`, help, tests); duplicación del parseo de flags o refactor; dos rutas hacia `Capture` a mantener coherentes; colisión coloquial con "collect" | Medium |
| c | **Mantener relay manual; automatizar solo el handoff** (imprimir comando pi ejecutable / script / launcher externo) | Cambio mínimo; preserva el discurso host-mediated; cero ejecución bloqueante en CLI | **No cierra el gap** (el host no existe en este repo: `internal/assets/pi/*`, W-1); operador sigue moviendo bytes; rol sigue out-of-band; scripts no testables como Go | Low |

## Recommendation

**Approach (a)** — modo execute en `capture-result` (flag explícito `--execute`, condicionado al handshake `ValidatePiAgent`, con `ctx` + deadline + `WaitDelay` y cap de stdout a `ArtifactResultLimit`, reutilizando `Preflight/Materialize/Capture` sin tocar admission ni CAS). Motivos: es la ruta que el propio código declara como futura (`cli_review.go:1060-1064`), reutiliza toda la cadena de autoridad existente y no añade un verbo nuevo que duplique flags. (b) es el fallback si el maintainer prefiere cero creep semántico en capture-result; (c) no resuelve nada y deja el handshake ceremonial. En cualquiera de (a)/(b) **el prompt-binding del reviewer (rol/lens para pi) es una decisión de design obligatoria**.

## Risks

- **Ejecución bloqueante/foreground**: un review pi es un run de modelo largo; capturar dentro del CLI bloquea el verbo. `CommandContext` mata solo el proceso directo (no el árbol en Windows) y `WaitDelay=5s` solo libera `Wait` (`pi_adapter.go:21-23,65`). Definir timeout, cancelación (ctrl-c) y UX.
- **Windows / empty stdout**: en nuestra ruta Go existe el mismo mecanismo base (stdout por pipe a buffer) y el síntoma se refusa tipado (`pi_adapter.go:75-77`; test `pi_relay_test.go:92-94`). El síntoma upstream reportado **no pude verificarlo** (el repo TS upstream no está en este workspace; no asumo paridad). Riesgo residual: `pi` en Windows resuelto por `LookPath` a un shim `.cmd`; ningún test ejercita un binario `pi` real (todos son fakes) → smoke real requerido en apply.
- **Stdout sin cap**: el adapter bufferiza todo; el rechazo por cap ocurre recién en `Capture` (post-buffer). Acotar en la ruta nueva.
- **Decoder estricto vs provider**: `evidence` debe ser `[]string` (string pelado falla el decode), `evidence` vacía rechazada (`artifact.go:314`), all-clear exige evidencia concreta, findings severos exigen evidence+disposition. El reviewer pi debe emitir el JSON estricto; el rol prompt decide esto.
- **Idempotencia/retry**: bytes idénticos re-capturados jamás satisfacen un rechazo de admission — hay que relanzar reviewer; y cualquier capture exitoso consume la revision (CAS) → el retry debe re-consultar status, no reusar flags.
- **Tensión de diseño**: el comentario de `pi_relay.go:12-17` describe el relay como host-mediated (el launcher ejecuta). La ruta CLI-execute convierte al CLI en host/ejecutor para pi — decisión explícita, no accidental.
- **Blast radius en opencode**: ninguna de (a)/(b) toca el plugin; el guard `PluginWiresCapture` debe seguir verde.

## Test Surface & RED Seam (Q7)

- Hoy: handshake (`pi_relay_test.go:13-87`), adapter con fakes POSIX/Windows (`:89-206`, seam `LookPath/CommandContext` `:100-119`, `fakePiCommandContext` `:207+`), deadline fail-closed (`:147-162`, skip en Windows), flags completos (`:174-206`). Package: `materialize_test.go` (harness real con git, `:423+`), `lineage_identity_test.go:282+` (harness), `capture_test.go` (admission/CAS). CLI: `review_materialize_test.go` (bytes exactos + captures-nothing), `review_parity_test.go:66-108` (harness `gitRepoWithAuthCommit`/`chdir`/`runReviewStart`), `review_contract_cli_test.go`, y guard `rdd_parity_test.go`.
- RED propuesto: (i) unit en `internal/review` para la orquestación nueva (fake adapter vía seams): assert que el prompt que recibe el adapter == `MaterializeReviewerTask(binding)` byte a byte, que los bytes crudos llegan a `Capture` sin transformar, y refusals de stdout vacío/deadline; (ii) CLI con shim `pi` en PATH (script POSIX / `.cmd` en Windows vía PATHEXT) que emite el JSON estricto golden con el `subject_hash` derivado de `--preflight`, corriendo start→execute→finalize→gate en repo temporal con los harness existentes; (iii) regression: `review_materialize_test` (read-only/captures nothing), `capture_test` (admission), `rdd_parity PluginWiresCapture`.

## Prior Art (Q8)

- Repo: archived change `2026-09-11-fix-rdd-receipt-collection` (`apply-progress.md:109,143`; `verify-report.md:120,130`; `archive-report.md:21,24,69`). El gap está explícitamente registrado como W-1 diferido ("add the production biggz-pi/gentle-pi host relay … so CurrentProducerHost() reports pi and hosts drive the relay"). `docs/comparison-with-gentle.md:95,116` declara el relay como ✅ "host-mediated … PiAdapter scratch-dir".
- **Missing feature, no unmet requirement**: ningún spec en `openspec/specs/` promete la ejecución CLI-side ni el handshake (grep `BIGGZ_PI_REVIEW_RELAY|review-relay|--agent pi` sobre specs: solo coincidencias incidentales de install/state). `review/spec.md` cubre materialización (`:323`), verbatim transport (`:371`) y producer parity (`:387`), no la ruta execute. → El change debe **añadir** requisitos, no satisfacer uno existente.

## Ready for Proposal

**Yes.** Con dos decisiones abiertas para el maintainer (a resolver en proposal/design, no aquí): (1) **prompt-binding del reviewer pi** — ¿el rol/lens se embebe en el prompt del adapter (usando `internal/assets/prompts/review/*.md`), o se delega al host/overlay pi?; (2) superficie: `capture-result --execute` (recomendado) vs verbo `collect` nuevo. Nota de alcance: (a)/(b) hacen al CLI ejecutor para pi, lo que tensiona el relato host-mediated de `pi_relay.go:12-17` — declararlo explícitamente en el proposal.
