schema: biggz-ai.sdd-research/v1
revision: 1
change: wire-pi-review-relay
lane: upstream-relay-design
outcome: done
artifact_store: hybrid

# Research: wire-pi-review-relay — upstream relay design

Carril seleccionado: **diseño del relay upstream** de `Gentleman-Programming/gentle-pi` (repo público): cómo el producto TypeScript hostea sus lentes de review, y la evidencia dura detrás de los síntomas de fallo del relay reportados contra él (incluido el síntoma Windows de stdout vacío).

Convenciones de evidencia:

- Toda cita de código upstream proviene del snapshot local de `upstream/main` extraído en `C:\Users\USER\AppData\Local\Temp\gentle-pi-main` (paths relativos a esa raíz, con líneas cuando aplica).
- Las citas de issues provienen de las páginas públicas de GitHub; cada fuente registra `URL`, `accessed_at` y un `excerpt` exacto.
- El código propio aparece SOLO como contraste y está etiquetado `owned-code`.
- Todo claim validado mapea a uno o más `source ids`.

## Questions

1. **Reviewer hosting mechanism** — ¿cómo hostea upstream una lente de review? ¿Qué host, cómo se definen los reviewers (overlays con tools restringidas?) y en qué archivos?
2. **Prompt binding** — ¿cómo liga upstream el rol de la lente a la task materializada? ¿La task materializada ES el prompt, o se compone un rol alrededor?
3. **Transport & contract** — ¿qué identificadores/versiones de contrato existen, qué se transporta (stdout crudo? JSON?) y qué validación corre antes de aceptar un resultado?
4. **Timeouts, cancelación, procesos** — ¿qué política de timeout/wait existe, cómo se cancela y cómo se manejan hijos (pipelines, shims, `.cmd`) en Windows?
5. **El síntoma stdout vacío en Windows (#943)** y el clúster relacionado (#941, #940, #937, #936, #935, #926, #924, #893, #871): mecanismo real, causa raíz probada o desconocida, con pasajes decisivos citados.
6. **Transfer assessment** — para cada hallazgo: ¿aplica a NUESTRO diseño Go (CLI ejecuta al reviewer vía `PiAdapter`, rol embebido por el CLI, timeout acotado, stdout del proceso `pi --print`)? Sin paridad asumida.

## Admission and observed grants

- Capability contract: `biggz-ai.sdd-research-capability/v1` — admitido para este carril.
- Grants observados: **`documentation`** (read/grep/find/ls sobre workspace y snapshot) y **`open-web`** (`web_search`/`web_fetch`).
- Uso: 12 páginas de issues de GitHub + 1 fetch crudo de `raw.githubusercontent.com` servidos por T1 (TLS directo), sin denegaciones ni fallback DDG. 4 fuentes Go/Windows vía índice de búsqueda (secundarias).
- Clases solicitadas: `local-worktree-snapshot`, `upstream-repo`, `github-issues`. Adicionales: `open-web`, `owned-code` (contraste).

## Sources

### S1 — `lib/review-host-relay.ts`
- class: local-worktree-snapshot
- title: Thin Pi host relay — coordinator, reviewer bound, typed failures, submission binding
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/lib/review-host-relay.ts
- accessed_at: 2026-09-12
- excerpt: "Pass those prompt bytes to the pure opaque Pi adapter, which owns its locked-down print-mode subprocess and fresh empty scratch directory; take its stdout as raw final bytes."

### S2 — `lib/opaque-pi-reviewer-adapter.ts`
- class: local-worktree-snapshot
- title: Opaque Pi reviewer adapter — frozen argv, win32 launcher branch, SIGKILL, empty-output refusal
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/lib/opaque-pi-reviewer-adapter.ts
- accessed_at: 2026-09-12
- excerpt: "A bare `pi` on Windows resolves to pi.cmd, pi.ps1, or a POSIX shim, none of which Node can spawn with shell:false (EINVAL or ENOENT). This adapter already runs inside Pi, so on win32 the host's own JavaScript entry is spawned through the host's process.execPath instead; a shell is never enabled."

### S3 — `lib/review-relay-contract.ts`
- class: local-worktree-snapshot
- title: Pi host relay handshake declaration (contract constant + env + session-scope injection)
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/lib/review-relay-contract.ts
- accessed_at: 2026-09-12
- excerpt: "gentle-ai admits `pi` as a host-mediated runtime identity only when the launcher that relays the invocation declares this exact contract through this exact environment variable."

### S4 — `extensions/gentle-ai.ts`
- class: local-worktree-snapshot
- title: Pi extension wiring of the host relay (slot selection, agent probe, group capture tools)
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/extensions/gentle-ai.ts
- accessed_at: 2026-09-12
- excerpt: "the thin Pi host relay. The provider decides which capture slots the host satisfies by issuing the --materialize token on a pi-bound `review.capture-result` collect input; nothing is ever inferred."

### S5 — `assets/agents/review-risk.md` (+ readability/reliability/resilience)
- class: local-worktree-snapshot
- title: Manual-lane lens agent overlays with restricted tools (native relay path never loads them)
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/assets/agents/review-risk.md
- accessed_at: 2026-09-12
- excerpt: "Manual/compat-lane only: the provider host-relay capture path never loads this agent definition; native lens capture materializes the Go-issued opaque prompt through the gentle-pi host relay." (nota línea 12; frontmatter `tools: "*": false, read, grep, find, gentle_review_scope`)

### S6 — `assets/chains/4r-review.chain.md`
- class: local-worktree-snapshot
- title: Manual 4R review chain (compat lane, cuatro pasos con salidas a archivo)
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/assets/chains/4r-review.chain.md
- accessed_at: 2026-09-12
- excerpt: "Manual/compat-lane only: the provider host-relay capture path never loads this chain; it exists solely for explicit manual 4R invocation."

### S7 — `docs/review-integration.md`
- class: local-worktree-snapshot
- title: Review integration architecture — ownership boundary and transport behavior
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/docs/review-integration.md
- accessed_at: 2026-09-12
- excerpt: "A pure opaque adapter: `Buffer → Buffer/error`. It accepts a Go-materialized prompt as bytes, invokes Pi, and returns raw final bytes or a typed transport error."

### S8 — `contracts/review-provider-contract-mirror/v1.2.0/bundle/orchestration/pi.md`
- class: local-worktree-snapshot
- title: Mirrored provider orchestration contract for Pi sessions
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/contracts/review-provider-contract-mirror/v1.2.0/bundle/orchestration/pi.md
- accessed_at: 2026-09-12
- excerpt: "Reviewers inspect only the provider-bound immutable trees, and the gentle-pi relay owns the reviewer prompt."

### S9 — `lib/native-review-cli.ts`
- class: local-worktree-snapshot
- title: Native review CLI client — operation map, single handshake injection point
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/lib/native-review-cli.ts
- accessed_at: 2026-09-12
- excerpt: "The one central runner for every gentle-ai CLI invocation the extension makes. It declares the Pi host relay handshake on each spawn: gentle-ai refuses pi admission pre-authority without it (gentle-pi#311 P4), and a single injection point keeps the declaration impossible to forget on any individual operation."

### S10 — `lib/review-integration-v2.ts`
- class: local-worktree-snapshot
- title: Negotiated contract v2 types — ReviewCollectInputV3, ReviewCaptureSubmissionV1
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/lib/review-integration-v2.ts
- accessed_at: 2026-09-12
- excerpt: "tokens that submit the captured bytes; the host substitutes only the artifact location into the declared {{value}} slot and never synthesizes or filters the form itself."

### S11 — `docs/native-authority-architecture.md`
- class: local-worktree-snapshot
- title: Windows Evidence section — upstream's own platform-support stance
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/docs/native-authority-architecture.md
- accessed_at: 2026-09-12
- excerpt: "Current code contains Windows-aware paths but does not prove complete Windows support" … "No end-to-end Windows support claim"

### S12 — `package.json` (snapshot)
- class: local-worktree-snapshot
- title: Snapshot package identity/version (freshness anchor)
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/package.json
- accessed_at: 2026-09-12
- excerpt: "\"name\": \"gentle-pi\", \"version\": \"2.5.0\""

### S13 — `tests/opaque-pi-reviewer-adapter.test.ts`
- class: local-worktree-snapshot
- title: win32 launch-shape test for the reviewer adapter
- publisher: Gentleman-Programming/gentle-pi @ upstream/main (snapshot local)
- URL: C:/Users/USER/AppData/Local/Temp/gentle-pi-main/tests/opaque-pi-reviewer-adapter.test.ts
- accessed_at: 2026-09-12
- excerpt: "test(\"the Pi launch shape spawns the host entry through process.execPath on win32 and stays byte-identical elsewhere\", ...)"

### S14 — `package.json` on `main` (raw.githubusercontent.com)
- class: upstream-repo
- title: Upstream main branch package identity (freshness cross-check)
- publisher: raw.githubusercontent.com
- URL: https://raw.githubusercontent.com/Gentleman-Programming/gentle-pi/main/package.json
- accessed_at: 2026-09-12T17:36:07.596Z
- excerpt: "\"version\": \"2.5.0\""

### S15 — GitHub issue #943
- class: github-issues
- title: "Windows reviewer relay exits successfully with empty stdout and stderr"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/943
- accessed_at: 2026-09-12T17:34:00.770Z
- excerpt: "Actual: the facade reports empty output without a verdict. Pi print-mode source permits exit 0 when the terminal state lacks an assistant message or text blocks, but which case occurred remains unknown." (abierto 2026-09-12; reporter DaniloMValladarez)

### S16 — GitHub issue #863
- class: github-issues
- title: "an intermittent upstream reviewer refusal is reported as a terminal transport failure"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/863
- accessed_at: 2026-09-12T17:34:20.508Z
- excerpt: "Replaying the relay's own invocation by hand never reproduced the refusal: 9 of 9 successes, using `OPAQUE_PI_REVIEWER_ARGV` verbatim … The relay's spawn shape is therefore faithful; the refusal simply comes and goes."

### S17 — GitHub issue #757
- class: github-issues
- title: "opaque Pi reviewer freezes --no-extensions, unloading the extension that provides the selected model → silent reviewer model substitution"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/757
- accessed_at: 2026-09-12T17:34:20.308Z
- excerpt: "Pi does not fail on the missing provider. It falls back to another authenticated provider from `auth.json` and answers normally, exit code 0, so the review completes green — but under a model the user never selected"

### S18 — GitHub issue #937
- class: github-issues
- title: "review-readability lens is reproducibly refused by an upstream policy block, leaving the lineage unclosable"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/937
- accessed_at: 2026-09-12T17:34:11.640Z
- excerpt: "The `review-readability` lens fails **reproducibly** with an upstream policy refusal: This request was blocked as it seems to violate Anthropic's Terms of Service restrictions on reverse engineering or duplicating model outputs"

### S19 — GitHub issue #936
- class: github-issues
- title: "targeted validator subagent is sometimes spawned without a shell, so it can never inspect the frozen candidate"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/936
- accessed_at: 2026-09-12T17:34:11.583Z
- excerpt: "The targeted validator subagent is sometimes spawned **without a shell tool**. Since its whole job is to inspect the immutable frozen trees by running `gentle-ai review inspect-candidate`, it then cannot inspect anything"

### S20 — GitHub issue #935
- class: github-issues
- title: "targeted validator result decoder rejects proof_refs, a field the provider emits in its own validation_request"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/935
- accessed_at: 2026-09-12T17:34:12.130Z
- excerpt: "The targeted validator's result decoder rejects `proof_refs` with: decode provider targeted validator result: json: unknown field \"proof_refs\""

### S21 — GitHub issue #926
- class: github-issues
- title: "correction-plan capture rejects provider-issued binding (lineage/target token mismatch)"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/926
- accessed_at: 2026-09-12T17:34:16.047Z
- excerpt: "`gentle_review_capture` (correction-plan slot) rejects the exact provider-issued `collectBinding` from a fresh `STATUS`, twice in a row, leaving the lineage stuck in `correction_required` with no valid forward route."

### S22 — GitHub issue #924
- class: github-issues
- title: "review.capture-validation rejects its own admitted evidence when the corrected finding is deterministic"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/924
- accessed_at: 2026-09-12T17:34:16.119Z
- excerpt: "The bounded-review correction route cannot close when the corrected finding's `evidence_class` is `deterministic`. `review.capture-validation` fails at `phase: \"preflight\"` with `code: \"invalid_request\"`"

### S23 — GitHub issue #893
- class: github-issues
- title: "RDD relay rejects its own capture binding and fresh START fails"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/893
- accessed_at: 2026-09-12T17:34:16.065Z
- excerpt: "The RDD relay accepts a provider-issued materialize `collectBinding` during the forecast phase, but rejects the exact same binding when it is immediately resubmitted with `reviewerRunAcknowledged: true`."

### S24 — GitHub issue #871
- class: github-issues
- title: "select-intended-untracked deterministically rejects an exact inspect selectionBinding"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/871
- accessed_at: 2026-09-12T17:34:20.441Z
- excerpt: "deterministically rejects a `selectionBinding` that was copied byte-for-byte from a fresh `inspect` response, blocking ordinary review START for any workspace whose candidate contains intended untracked files."

### S25 — GitHub issue #941
- class: github-issues
- title: "select-intended-untracked rejects provider-issued selection binding, blocking START for fully-tracked candidates"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/941
- accessed_at: 2026-09-12T17:34:07.885Z
- excerpt: "Native ordinary review cannot START when the candidate has zero untracked files but the repo has a large unrelated untracked inventory (here: 426 files - QA screenshots plus MCP-server logs; candidate = 3 tracked modified files, 352 changed lines)."

### S26 — GitHub issue #940
- class: github-issues
- title: "admission reads a reviewer's statement of contract compliance as an inspection failure and refuses a valid result"
- publisher: github.com (Gentleman-Programming/gentle-pi)
- URL: https://github.com/Gentleman-Programming/gentle-pi/issues/940
- accessed_at: 2026-09-12T17:34:07.683Z
- excerpt: "Reviewer artifact admission rejected a lens result because it read the reviewer's **correct description of its own method** as a confession that inspection had failed"

### S27 — golang/go issue #69939
- class: open-web
- title: "syscall: special case `cmd.exe /c ` in StartProcess"
- publisher: github.com/golang/go
- URL: https://github.com/golang/go/issues/69939
- accessed_at: 2026-09-12
- excerpt: "It is well known that os/exec doesn't correctly escape nor quote `*.bat`, `*.cmd`, `cmd /c *` arguments." (secundaria, vía índice)

### S28 — `src/os/exec/lp_windows.go`
- class: open-web
- title: Go standard library Windows LookPath source comment
- publisher: github.com/golang/go
- URL: https://github.com/golang/go/blob/go1.19.3/src/os/exec/lp_windows.go
- accessed_at: 2026-09-12
- excerpt: "LookPath also uses PATHEXT environment variable to match a suitable candidate." (secundaria, vía índice)

### S29 — BatBadBut research
- class: open-web
- title: "BatBadBut: You can't securely execute commands on Windows"
- publisher: flatt.tech
- URL: https://flatt.tech/research/posts/batbadbut-you-cant-securely-execute-commands-on-windows/
- accessed_at: 2026-09-12
- excerpt: "CreateProcess() implicitly spawns cmd.exe when executing batch files (.bat, .cmd, etc.), even if the application didn’t specify them in the command line." (secundaria, vía índice)

### S30 — golang/go issue #66586
- class: open-web
- title: "os/exec: Cmd.Start with an absolute path no longer implicitly adds .exe in Go 1.22"
- publisher: github.com/golang/go
- URL: https://github.com/golang/go/issues/66586
- accessed_at: 2026-09-12
- excerpt: "On Windows, `Command` and `Cmd.Start` no longer call `LookPath` if the path to the executable is already absolute and has an executable file extension." (secundaria, vía índice)

### S31 — `internal/review/pi_adapter.go` (OURS)
- class: owned-code
- title: biggz-ai PiAdapter — no production caller today
- publisher: biggz-ai workspace (owned)
- URL: C:/Users/USER/Desktop/biggz-ai/internal/review/pi_adapter.go
- accessed_at: 2026-09-12
- excerpt: "if len(bytes.TrimSpace(stdout.Bytes())) == 0 { return nil, errors.New(\"pi reviewer transport produced no final message\") }" (más: flags `--print --mode text --no-session --no-tools --no-extensions --no-skills --no-prompt-templates --no-themes --no-context-files --no-approve`, `cmd.Stdin`, scratch dir, `WaitDelay` 5s)

### S32 — `internal/review/pi_relay.go` (OURS)
- class: owned-code
- title: biggz-ai host relay handshake (naming/env/two-way compat)
- publisher: biggz-ai workspace (owned)
- URL: C:/Users/USER/Desktop/biggz-ai/internal/review/pi_relay.go
- accessed_at: 2026-09-12
- excerpt: "The host-mediated immutable relay keeps Go as the sole authority for prompt materialization, admission, budgets, receipts and gates."

### S33 — `internal/review/materialize.go` (OURS)
- class: owned-code
- title: biggz-ai reviewer task materializer (binding/context/name-status/numstat/patches)
- publisher: biggz-ai workspace (owned)
- URL: C:/Users/USER/Desktop/biggz-ai/internal/review/materialize.go
- accessed_at: 2026-09-12
- excerpt: "composes the complete provider-owned reviewer task for a collect transition from the frozen trees: binding, preflight context, name-status, numstat, and per-path frozen patch bytes with explicit delimiters."

### S34 — `cmd/biggz/cli_review.go:1055-1066` (OURS)
- class: owned-code
- title: capture-result pi admission comment — the gap this change closes
- publisher: biggz-ai workspace (owned)
- URL: C:/Users/USER/Desktop/biggz-ai/cmd/biggz/cli_review.go
- accessed_at: 2026-09-12
- excerpt: "Host relay available: the capture path could materialize via PiAdapter.Review with the Go-issued opaque prompt and then submit the raw bytes through the existing --input path. The current minimal port keeps the existing file/stdin input path unchanged; the adapter is available for future materialize/execute routing when the provider prompt binding is added."

### S35 — `internal/assets/prompts/review/*.md` (OURS)
- class: owned-code
- title: Reviewer role prompts (r1-risk, r2-readability, r3-reliability, r4-resilience, shared, external)
- publisher: biggz-ai workspace (owned)
- URL: C:/Users/USER/Desktop/biggz-ai/internal/assets/prompts/review/r1-risk.md
- accessed_at: 2026-09-12
- excerpt: "You are the R1 Risk classifier. Analyze the authored change for risk tier."

### S36 — `openspec/changes/wire-pi-review-relay/preproposal.md` (change context)
- class: owned-code
- title: Confirmed product decisions and research selection state (rev 2)
- publisher: biggz-ai workspace (change artifact)
- URL: C:/Users/USER/Desktop/biggz-ai/openspec/changes/wire-pi-review-relay/preproposal.md
- accessed_at: 2026-09-12
- excerpt: "Research is now SELECTED (lane: upstream relay design), so its completion is mandatory before the proposal phase; `proposal_ready` stays false until the lane lands `done` with valid evidence references."

### S37 — `openspec/changes/wire-pi-review-relay/exploration.md` (change context)
- class: owned-code
- title: Exploration — the execute gap, invariants, and the Windows/.cmd residual risk note
- publisher: biggz-ai workspace (change artifact)
- URL: C:/Users/USER/Desktop/biggz-ai/openspec/changes/wire-pi-review-relay/exploration.md
- accessed_at: 2026-09-12
- excerpt: "Riesgo residual: `pi` en Windows resuelto por `LookPath` a un shim `.cmd`; ningún test ejercita un binario `pi` real (todos son fakes) → smoke real requerido en apply."

## Validated claims

### Q1 — Reviewer hosting mechanism

- **C1** — El ejecutor de review upstream es una **extensión de Pi** (`extensions/gentle-ai.ts`) que hospeda un *thin relay*: la extensión nunca decide qué revisar; satisface solo slots emitidos por el provider. Importa `runReviewHostRelayReviewerGroup`, `runReviewHostRelaySlot`, `reviewHostRelaySlots` desde `../lib/review-host-relay.ts` (S4:109–129); el cableado vive tras el comentario "the provider decides which capture slots the host satisfies by issuing the --materialize token on a pi-bound `review.capture-result` collect input; nothing is ever inferred" (S4:5748–5752). Nota de campo: sin `--agent pi`, "reviewHostRelaySlots() saw zero materialize slots, the relay was unreachable, and no lens was ever launched" (S4:6176–6183). **Sources: S4, S1.**
- **C2** — Por cada slot de lente el relay ejecuta **tres procesos en secuencia**: (1) el CLI provider `gentle-ai review capture-result <tokens>` con el env del relay inyectado, cuyo stdout es el prompt opaco; (2) un `pi` print-mode fresco en un scratch dir vacío vía el adapter opaco, con el prompt por stdin; (3) la submission `gentle-ai review <operationToken> <tokens>` con el resultado staged en un temp file y solo el slot de artifact sustituido. El relay rechaza sintetizar el paso 3: "The host never synthesizes or filters the completing form; a materialize slot without a provider submission is a typed contract mismatch, never a rebuilt invocation" (S1 header). **Sources: S1.**
- **C3** — La forma del proceso reviewer está congelada en el adapter: `OPAQUE_PI_REVIEWER_ARGV` es constante congelada (`--print --mode text --no-session --no-tools --no-extensions --no-skills --no-prompt-templates --no-themes --no-context-files --no-approve`, S2:6–17); spawn `shell: false, windowsHide: true`; cwd `mkdtemp` chmod `0o700`; prompt a stdin (`child.stdin.end(prompt)`); stdout/stderr como Buffers; superficie pura "`Buffer → Buffer/error`" (S7). **Sources: S2, S7.**
- **C4** — Los "agentes" de lente existen como **overlays de Pi con tools restringidas** (`assets/agents/review-{risk,readability,reliability,resilience}.md`, frontmatter `tools: "*": false` más `read, grep, find, gentle_review_scope`) y como cadena 4R manual (`assets/chains/4r-review.chain.md`), pero ambos están marcados lane manual/compat: "the provider host-relay capture path never loads this agent definition; native lens capture materializes the Go-issued opaque prompt through the gentle-pi host relay" (S5:12; cadena S6:4). **Sources: S5, S6.**

### Q2 — Prompt binding

- **C5** — En la ruta nativa **los bytes materializados por el provider SON el prompt completo del reviewer**: "Run the exact provider-issued capture binding … and take stdout as opaque prompt BYTES, verbatim" → esos bytes van al adapter sin cambios, prompt por stdin (S1 + `prepareReviewHostRelaySlot`). El host NO compone rol: el adapter "accepts a Go-materialized prompt as bytes" y "does not parse bindings, select work, rebuild prompts, inspect repository state, retry, classify results, or create authority" (S7). **Sources: S1, S7.**
- **C6** — El rol de la lente, en la ruta nativa, es **Go-issued dentro del prompt materializado**; el relay nunca liga un rol. Evidencia: la nota del overlay (S5) y el contrato espejado: "Reviewers inspect only the provider-bound immutable trees, and the gentle-pi relay owns the reviewer prompt" (S8). **Sources: S5, S8.**
- **C7** — Una materialización vacía es **refusal tipada antes de lanzar al reviewer**: `EMPTY_PROMPT: "empty-prompt"`, mensaje "gentle-ai prompt materialization produced no bytes" (S1:58). **Sources: S1.**

### Q3 — Transport & contract

- **C8** — Handshake del relay: env `GENTLE_PI_REVIEW_RELAY_CONTRACT` con valor exacto `gentle-pi.review-relay/v1` (constantes compiladas, "versioned by release, never by configuration", S3:15–16). La extensión lo declara en **cada** invocación al CLI vía el runner único ("a single injection point keeps the declaration impossible to forget on any individual operation", S9) y también a nivel de sesión con `declareReviewRelayHandshake` ("Without it the first `gentle-ai review status --agent pi` typed in the session shell fails closed", S3). Sin declaración, Pi queda fail-closed en admission, pre-authority (S3). **Sources: S3, S9.**
- **C9** — Lanes de contrato: `gentle-ai.review-integration/v1` y `/v2`; el relay consume collect inputs tipados `ReviewCollectInputV3` (`name`, `schema`, `captureOperation`, `arguments[]`, `artifactSubject` opcional, `baseTree`) con `token` por argumento (S10:537–543); el formulario de cierre es `ReviewCaptureSubmissionV1` (`operationToken`, `argumentTokens`, `values[]` con `substitutionLocation`, S10:419–439), y el host sustituye **solo** la ubicación del artifact en el slot `{{value}}` (S10:416–418; constante `REVIEW_HOST_RELAY_SUBMISSION_VALUE_SLOT = "{{value}}"`, S1:69). Versiones observadas en logs de campo: status `gentle-ai.review-integration.status/v7` (S24), failure `gentle-ai.review-integration.failure/v2` (S22). **Sources: S10, S1, S24, S22.**
- **C10** — Qué se transporta: **stdout crudo en ambas direcciones**. Prompt: stdout materializado → stdin de pi, sin modificar. Resultado: Buffer crudo → `result.raw` (modo `0o600`) en staging `mkdtemp` `0o700` → path sustituido en el único token `{{value}}` → submission ejecutada; el stdout de la submission se devuelve opaco como "the provider's admitted-manifest JSON" (S1, `submitReviewHostRelayPreparedResult`). **Sources: S1.**
- **C11** — La validación previa a aceptar tiene dos capas. Relay-side (probado en código): el slot requiere `captureOperation === "review.capture-result" && materialize === "true" && agent === "pi"` (S1:201–205); el formulario debe bindear **exactamente un** valor de artifact con ubicación en rango y token con `{{value}}`, o es `submission-contract-mismatch` — "the relay never repairs, filters, or synthesizes it" (S1). Provider-side: la admission Go es dueña de aceptar el resultado (el relay nunca lo parsea, S7); ejemplo de refusal en campo: "reviewer artifact admission incomplete: reviewer evidence reports the candidate could not be inspected even though inspection.status is \"completed\"… [invalid_request]" (S26). **Sources: S1, S7, S26.**

### Q4 — Timeouts, cancellation, process handling

- **C12** — Política del bound del reviewer (gentle-pi#367, main): no es un número fijo sino **derivado** — "floor + ceil(promptBytes / MiB * perMebibyte), clamped to the ceiling": `REVIEW_HOST_RELAY_PI_TIMEOUT_FLOOR_MS = 900_000`, `REVIEW_HOST_RELAY_PI_TIMEOUT_PER_MEBIBYTE_MS = 900_000`, `REVIEW_HOST_RELAY_PI_TIMEOUT_MAX_MS = 7_200_000` (S1:357–371). Override por env `GENTLE_PI_REVIEW_RELAY_PI_TIMEOUT_MS`: "replaces the derived bound entirely … a positive decimal, silently ignored when malformed, and clamped to the same hard ceiling so no configuration can turn a foreground FINALIZE into an unbounded child process" (S1). Al vencer: `child.kill("SIGKILL")`, fallo tipado `pi-timed-out` con `elapsedMs`/`timeoutMs`; mensaje "Relaunching the same slot unchanged reaches the same wall." (S1). **Sources: S1.**
- **C13** — Razón histórica (comentario S1): "The previous bound was a single hardcoded 600_000 ms … A field-measured lens legitimately needed 478s against a ~1.58 MB materialized prompt: it survived by hand and was killed under the relay". El bound derivado da ~4.7× de margen porque "the reviewer model and provider are user-owned and the relay cannot know their throughput" (S1). El default propio del adapter cuando no se pasa timeout es 600 000 ms (`DEFAULT_OPAQUE_PI_TIMEOUT_MS`, S2:86), pero el relay siempre pasa el valor derivado. **Sources: S1, S2.**
- **C14** — Cancelación: un `AbortSignal` atraviesa materialize → reviewer → submit ("The supplied AbortSignal intentionally stays live across materialize, reviewer, and submit, preserving the established cancellation behavior", S1); cancelar al reviewer mata con `SIGKILL`; una señal pre-abortada produce `cancelled` tipado sin lanzar (`CANCELLED`, S2). Los grupos de reviewers **corren concurrentes**: "Starts every reviewer before awaiting any result. If one or more reviewers fail, it rejects only after every started transport has settled and reports the earliest failed request in provider order." (S1). **Sources: S1, S2.**
- **C15** — Los hijos del CLI provider (materialize y submit) usan el mismo endurecimiento: `shell: false, windowsHide: true`, `SIGKILL` en timeout, stdin cerrado/`end(buffer)`, stdout/stderr Buffer concat, `timer?.unref()` (S1 `collectGentleAiProcess`, líneas 390–430). **Sources: S1.**
- **C16** — Manejo de hijos en Windows: el adapter tiene rama win32 explícita. Un `pi` pelado en Windows "resolves to pi.cmd, pi.ps1, or a POSIX shim, none of which Node can spawn with shell:false (EINVAL or ENOENT)". Por eso, en win32 y sin launcher explícito, spawnea **la entrada JavaScript del propio host vía `process.execPath`** (`resolvePiLaunch`, S2:106–116); si la entrada no es resoluble, falla cerrado ("a bare pi launcher cannot be spawned on Windows without a shell"); un `piExecutable` explícito conserva su forma en toda plataforma; `shell` nunca se habilita. Cubierto por test: "the Pi launch shape spawns the host entry through process.execPath on win32 and stays byte-identical elsewhere" (S13:240–255). **Sources: S2, S13.**

### Q5 — Síntoma stdout vacío en Windows (#943) y clúster relacionado

- **C17** — #943 (abierto 2026-09-12, Windows, gentle-pi 2.5.0 / Pi 0.85.1 / gentle-ai nativo 2.7.0): "A native reviewer capture on Windows ended with exit code 0 and empty stdout and stderr, leaving the review without a verdict." Log reportado (exacto): `outcome: pi-host-relay-transport-failure / kind: pi-empty-output / stage: pi / exit_code: 0 / timed_out: false / elapsed_ms: 31733 / timeout_ms: 1344130 / stdout_bytes: 0 / stderr_bytes: 0 / mutation_performed: false`. El reporter agrega: "The Node/bundle CLI entrypoint was verified and a benign `--version` probe succeeded." **Sources: S15.**
- **C18** — **La causa raíz de #943 sigue desconocida según el propio reporte**: "The relay uses saved defaults (`openai-codex/gpt-5.3-codex-spark`, `xhigh`)… No configuration mismatch or root cause is proven." La única hipótesis de mecanismo — "Pi print-mode source permits exit 0 when the terminal state lacks an assistant message or text blocks, but which case occurred remains unknown" — está explícitamente no probada y no hay reproducción mínima estable. #943 declara además que #757 y #863 "describe different symptoms". **Sources: S15.**
- **C19** — El mapeo host-side que produce el síntoma es **por diseño y probado en código**: el adapter lanza `EMPTY_OUTPUT` cuando `processResult.stdout.length === 0` ("Pi process produced no output bytes", S2:~257) y el relay lo mapea a `PI_EMPTY_OUTPUT: "pi-empty-output"` ("pi subprocess produced no output bytes", S1:62,467), surfaced como `pi-host-relay-transport-failure`. Clave: `stderr_bytes: 0` (S15) significa que el hijo no emitió nada clasificable — el capture no contiene material para distinguir "el modelo no produjo texto" de "el proceso no escribió nada". **Sources: S2, S1, S15.**
- **C20** *(analysis, no aserción de fuente)* — El `timeout_ms: 1344130` de #943 es consistente con la fórmula derivada de C12 para un prompt materializado de ~505 KiB (900 000 + ceil(bytes/MiB × 900 000)); `timed_out: false` **excluye el bound de timeout como causa**. **Sources: S1 (constantes), S15 (log).**
- **C21** — #863 (evidencia más cercana de transporte): todo exit no-cero del reviewer mapea a `REVIEW_HOST_RELAY_FAILURE.PI_FAILED` vía `relayPiTransportError`, reportado como `outcome: "pi-host-relay-transport-failure"` — "There is no reason code that distinguishes a transient upstream refusal from a genuine transport fault". El fallo fue **intermitente** sobre candidato y binding byte-idénticos ("refused once and approved minutes later"), y el reporter falsó tanto la hipótesis de umbral por tamaño como la de forma del spawn: "Replaying the relay's own invocation by hand never reproduced the refusal: 9 of 9 successes, using `OPAQUE_PI_REVIEWER_ARGV` verbatim … The relay's spawn shape is therefore faithful; the refusal simply comes and goes." El stderr observado en el caso no-cero fue un bloqueo de política upstream ("This request was blocked as it seems to violate Anthropic's Terms of Service restrictions on reverse engineering or duplicating model outputs"); resubmitir el binding idéntico tras STATUS fresco devolvió `closed / approved`. Tasa desconocida: "I do not know whether this affects 5% or 40% of captures, and I make no such claim." **Sources: S16.**
- **C22** — #757 (mismo argv, otro síntoma): el flag congelado `--no-extensions` descarga los providers de modelo aportados por extensiones; pi "does not fail on the missing provider. It falls back to another authenticated provider from `auth.json` and answers normally, exit code 0" — sustitución silenciosa de modelo (observado `gpt-5-mini` en lugar del claude elegido) — o, sin credencial de fallback, "No API key found for the selected model". La comparación controlada varía solo `--no-extensions` (exit 0 en ambos lados). **Sources: S17.**
- **C23** — #937 (clase política relevante): `review-readability` falla **reproduciblemente** (2/2, resultado idéntico) con el mismo texto de política ToS de Anthropic, bloqueando el cierre de linaje; el reporter lo separa explícitamente de #863: "That issue describes an *intermittent* upstream refusal … This one is different on both axes: the message is a distinct, explicit policy block rather than a generic transport failure, and it is reproducible rather than transient." **Sources: S18.**
- **C24** — El resto del clúster **no es el síntoma de stdout vacío**; cada issue tiene su sujeto, establecido desde su página: #936 validator subagent sin shell ("The targeted validator subagent is sometimes spawned **without a shell tool**", S19); #935 decoder rechaza `proof_refs` ("decode provider targeted validator result: json: unknown field \"proof_refs\"", S20); #926 correction-plan capture rechaza el binding del propio provider ("rejects the exact provider-issued `collectBinding` from a fresh `STATUS`, twice in a row", S21); #924 `review.capture-validation` rechaza su propia evidencia admitida para findings deterministas (`invalid_request` / `cause: "admitted targeted-validator evidence does not match its request and digests"`, S22); #893 binding aceptado en forecast y rechazado al ack ("rejects the exact same binding when it is immediately resubmitted with `reviewerRunAcknowledged: true`", S23); #871 selección untracked pre-linaje ("deterministically rejects a `selectionBinding` that was copied byte-for-byte from a fresh `inspect` response", S24); #941 el mismo seam para candidato fully-tracked con inventario untracked grande (S25); #940 admission lee la descripción correcta del método del reviewer como confesión de fallo ("read the reviewer's **correct description of its own method** as a confession that inspection had failed", S26). Solo #863 y #943 son reportes de *transporte* del relay; #757 y #937 son clases upstream de refusal/sustitución que afloran por el relay. **Sources: S19, S20, S21, S22, S23, S24, S25, S26.**
- **C25** — La postura de plataforma de upstream limita cualquier claim de Windows: "Current code contains Windows-aware paths but does not prove complete Windows support", con fila final "No end-to-end Windows support claim" (S11). **Sources: S11.**

### Q6 — Transfer assessment (analysis anclado a las fuentes; sin paridad asumida)

- **C26 (transfiere — mecanismo ya espejado)** — Ejecutar el reviewer como subproceso print-mode fresco y bloqueado, con scratch cwd, prompt por stdin y stdout crudo capturado es exactamente la forma que nuestro `PiAdapter` ya implementa con paridad de flags (S31), y el contrato upstream describe la misma superficie (S2, S7). Transfieren los conceptos porque nuestro adapter se escribió contra ese boundary exacto; no transfiere el código. **Sources: S2, S7, S31.**
- **C27 (transfiere — clase de guarda de salida vacía)** — El `EMPTY_OUTPUT` upstream tiene análogo directo: stdout vacío/whitespace → error tipado "pi reviewer transport produced no final message" (S31). El síntoma #943 en nuestra ruta **refusaría tipado** en vez de capturar basura; la pregunta abierta es de upstream (por qué pi no emitió nada). **Sources: S2, S31, S15.**
- **C28 (transfiere — semántica de timeout + datapoints de cautela)** — Timeout como fallo tipado con kill transfiere directo (S31 tiene `CommandContext` + `WaitDelay`; el diseño añade default acotado + `--timeout` según decisión confirmada S36). Dos datapoints upstream son load-bearing para cualquier default fijo: una lente legítima necesitó 478 s contra un prompt de ~1.58 MB bajo el bound previo de 600 s (S1), y en #943 el bound derivado (~22 min) **no** fue el disparador (`timed_out: false`, S15), mientras #863 muestra que los desenlaces de transporte pueden ser intermitentes con independencia del tamaño (S16). Esto constriñe la justificación, no la decisión (la decisión es del maintainer, S36). **Sources: S1, S15, S16, S31, S36.**
- **C29 (NO transfiere — arquitectura del relay)** — El relay tri-party (extensión spawnea CLI provider → adapter spawnea pi → extensión spawnea submit; el host nunca parsea) es específico del runtime upstream: existe porque gentle-pi es *host* que relega a un binario provider externo. Nuestro diseño confirmado hace al CLI el ejecutor (`PiAdapter` dentro de `capture-result --execute`, S36) con Go dueño de materialización/admission/receipts (S32, S33, S34). Transfieren la forma del proceso y la taxonomía de fallos; NO el boundary host-relay, ni las dos invocaciones extra al CLI, ni la indirección del formulario de submission. **Sources: S1, S7, S32, S33, S34, S36.**
- **C30 (paridad de riesgo, no de código — `--no-extensions`)** — Nuestro adapter congela el mismo `--no-extensions` (S31), así que la *clase* de riesgo documentada en #757 (modelos aportados por extensión descargados → sustitución silenciosa con exit 0, o fallo duro sin credencial de fallback, S17) aplica donde nuestro `pi` obtenga modelos de extensiones. Ni upstream ni nosotros fijamos modelo en el argv del reviewer: upstream mantiene "Model/provider/profile selection … user-owned: no --model, no --provider, environment untouched" (S1). Si el `pi` objetivo tiene una superficie de modelos por extensión **no está establecido por estas fuentes**. **Sources: S1, S17, S31.**
- **C31 (NO transfiere — clases de rechazo de ledger/binding)** — #871/#926/#924/#940/#893/#941 son semántica del ledger del provider (selection bindings, tokens de correction-plan, digests de validación, heurísticas de prosa en admission); nuestro ledger Go implementa sus propios invariantes de binding/preflight/admission/CAS (S33, S37) y esos defectos upstream no formulan claim alguno contra nuestra implementación. Son relevantes solo como *warnings de forma de fallo* sobre staleness de bindings y sensibilidad al fraseo en admission. **Sources: S21, S22, S23, S24, S25, S26, S33, S37.**
- **C32 (no puede transferir — causa raíz desconocida)** — Para #943 la causa raíz es desconocida (C18); nada puede portarse salvo la lección: cuando el hijo sale 0 con cero bytes en ambos streams, el capture no lleva material de diagnóstico (S15), así que cualquier obligación de diagnóstico debe satisfacerla el *padre* con su fallo tipado (lo que el diseño ya contempla, S36). **Sources: S15, S36.**
- **C33 (no resuelto — resolución `.cmd` en Windows en NUESTRA ruta Go)** — Upstream necesitó una rama especial de launcher porque Node no puede spawnear shims `.cmd`/`.ps1` con `shell:false` (EINVAL/ENOENT, S2). Si nuestra ruta Go sufre una clase equivalente **no está establecido**: `exec.LookPath` honra `PATHEXT` en Windows (S28), el proyecto Go documenta que `os/exec` no escapa/quotea correctamente argumentos `*.bat`/`*.cmd` (S27), `CreateProcess` spawnea `cmd.exe` implícitamente para batch files (S29), y las semánticas de StartProcess/LookPath cambiaron entre versiones de Go (S30). Nuestro repo solo ejercita binarios `pi` falsos y no existe corrida de aceptación real en Windows (S37). Se registra como incertidumbre, no como claim validado. **Sources: S27, S28, S29, S30, S31, S37.**

## Contradictions

1. **#863 vs #937 (reproducibilidad de refusals de política).** #863: refusal intermitente, resubmitir el binding idéntico funciona, tasa desconocida (S16). #937: refusal reproducible 2/2 sobre el mismo candidato congelado, y el reporter separa ambos explícitamente: "they are not the same observation" (S18). Ambos coinciden en que no hay reason code dedicado para refusals de política. No resuelto por ninguna fuente; sin fix/PR upstream enlazado al momento del acceso.
2. **#943 vs #863 (clase de fallo).** #943 es exit 0 con **stderr vacío**; #863 es exit 1 con stderr de política. #943 declara que #757 y #863 "describe different symptoms" (S15). Ninguna fuente establece mecanismo compartido; cualquier claim de mecanismo común es rechazado por el encuadre de los reporters (S15, S18).
3. **Doc de Windows upstream vs #943.** `docs/native-authority-architecture.md` afirma "No end-to-end Windows support claim" (S11) mientras #943 es un fallo Windows vivo. No es contradicción lógica — es consistente con una superficie Windows no probada — pero implica que upstream no tiene baseline Windows probado para clasificar #943.
4. **#757 sustitución silenciosa vs #943 "no configuration mismatch".** #757 prueba sustitución silenciosa de modelo bajo el argv congelado en un setup de modelos por extensión (S17); el reporter de #943 dice que el modelo por defecto es built-in y "No configuration mismatch or root cause is proven" (S15). Setups distintos; sin contradicción establecida, pero tampoco exclusión.

## Uncertainty

1. **Causa raíz de #943** — explícitamente no probada ("No configuration mismatch or root cause is proven"; la hipótesis de estado terminal de print-mode "remains unknown", S15). Sin reproducción mínima; sin PR enlazado; nada en el snapshot referencia el issue.
2. **Mecanismo de la escritura vacía** — si el hijo pi no produjo texto de asistente, perdió su stdio o nunca corrió el modelo no se puede distinguir desde el capture (`stdout_bytes: 0`, `stderr_bytes: 0`; S15).
3. **Tasa de refusals de la clase #863** — el reporter declina explícitamente cuantificar (S16).
4. **Si #863 y #937 comparten causa** — "They may well share a fix — a reason code that names upstream policy refusals as a category — but they are not the same observation" (S18).
5. **Comportamiento Go/Windows `.cmd` para nuestro adapter** — ver C33: las fuentes describen las clases de shim de Windows y los caveats de quoting de Go (S27–S30), pero ninguna establece el comportamiento end-to-end de `exec.CommandContext(ctx, LookPath("pi"), flags…)` contra un shim `pi` estilo npm; nuestros tests usan solo fakes (S37). No validado para nuestro diseño.
6. **Comportamiento del bound derivado en el release** — el release npm 2.5.0 vs el snapshot `main`: el snapshot coincide con `package.json` de main (`2.5.0`, S12/S14) y el `timeout_ms` de #943 cuadra con la fórmula derivada (C20, analysis), pero ninguna fuente garantiza que binario de release y `main` sean idénticos en todo lo demás.
7. **Estados de los issues** — los 12 issues consultados estaban Open, sin ramas/PRs enlazadas al momento del acceso (S15–S26).

## Freshness

- **Snapshot**: `upstream/main` extraído a `C:\Users\USER\AppData\Local\Temp\gentle-pi-main`; cross-check el mismo día contra `raw.githubusercontent.com/.../main/package.json` → `"version": "2.5.0"` (S12, S14). Consistente con `main` al momento del acceso (2026-09-12).
- **Issues**: abiertos entre 2026-09-09 y 2026-09-12; accedidos 2026-09-12T17:34Z (S15–S26). Entornos reportados: gentle-pi 2.5.0, Pi 0.85.1, gentle-ai nativo 2.7.0.
- **Fuentes Go/Windows secundarias**: recuperadas 2026-09-12 vía snippets de índice de búsqueda (S27–S30).
- **Números de línea del snapshot** válidos para la copia extraída citada; un `main` futuro puede derivar.

## Product choices (non-authoritative)

Decisiones confirmadas por el maintainer (2026-09-12, rev 2 de `preproposal.md`, S36): superficie = modo execute en `capture-result --agent pi --execute`; prompt binding = rol embebido por el CLI desde `internal/assets/prompts/review/*.md` + task materializada, por stdin del adapter; timeout = default acotado (10 min) con override `--timeout` y cancelación que mata al reviewer sin captura parcial. Esta investigación NO modifica, recomienda ni infiere consentimiento sobre ellas; se reproducen solo para separar evidencia de decisiones.

## Open questions for the orchestrator (evidence-derived; NOT product decisions)

1. **La composición de rol no tiene precedente en la ruta nativa upstream.** La ruta nativa upstream deliberadamente NO compone rol host-side — los bytes materializados son todo el prompt, y los overlays existen solo para la lane manual (S1, S5, S7). Nuestra elección confirmada (rol embebido por el CLI, S36) diverge; design debe especificar orden y contenido de la composición, ya que las únicas fuentes de rol in-tree son artefactos de la lane manual (nuestros prompts S35 están hoy sin binding, según S37).
2. **Estrategia de resolución en Windows.** Upstream tuvo que evitar el `pi` pelado en win32 (S2); nuestra ruta Go confía en `LookPath` (S31) cuyo comportamiento con `.cmd` sigue sin establecer (C33, incertidumbre 5). Si design debe añadir una regla de resolución Windows o un smoke de aceptación es pregunta abierta de design.
3. **Divergencia de política de timeout.** Upstream combina bound derivado por escala con override de env clampeado a techo de 2 h, precisamente para que ninguna configuración haga unbounded una operación foreground (S1). Nuestra decisión confirmada usa default fijo de 10 min + flag (S36). El datapoint upstream de 478 s/1.58 MB (S1) y el fallo de #943 a los 31.7 s sin timeout (S15) son la evidencia relevante si design quiere justificar o revisar el default.
4. **Obligación de diagnóstico con salida vacía.** En #943 el capture lleva cero bytes de diagnóstico (S15); si nuestro diseño promete diagnósticos seguros ante fallo del reviewer, el padre debe generarlos, porque el hijo puede no emitir nada (C19, C32).

## Gaps

Ninguno que bloquee el outcome `done`: las seis preguntas del carril tienen claims mapeados a fuentes. Las incógnitas restantes están listadas en **Uncertainty** y **Open questions** y no convierten claims no soportados en claims válidos.
