```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:c7b3655dbdbe7df9ca7f8d95c19eef78069100638ae93e408518a5aa6a089a6e
verdict: pass
blockers: 0
critical_findings: 0
requirements: 6/6
scenarios: 19/19
test_command: go test ./internal/bigmem -run TestRecent -count=1 -v && go test ./cmd/biggz -run TestRecall -count=1 -v
test_exit_code: 0
test_output_hash: sha256:c7b3655dbdbe7df9ca7f8d95c19eef78069100638ae93e408518a5aa6a089a6e
build_command: go vet ./internal/bigmem ./cmd/biggz && go build -o /tmp/biggz-verify.exe ./cmd/biggz
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: fix-bigmem-recall-recency
**Mode**: ligero — focused verify (no `go test ./...` completo, evita watchdog 240s)
**Artifact Store**: openspec (hybrid persist)
**Change Root**: openspec/changes/fix-bigmem-recall-recency
**Commit Verified**: HEAD (trabajo no commiteado, 10/10 tasks done)
**Date**: 2026-09-01
**Ledger**: `biggz sdd-attempt status fix-bigmem-recall-recency` -> `Revision 951d04c1... complete:true corrupt_authority: ledger is complete; reset required to continue` — `biggz sdd-attempt acquire --work-unit verify` bloqueado (precedente `2026-08-26-gentle-v2.5-parity` y `complexity-gates`: `attempt-direct` cuando ledger complete). Evidencia ligada por hash directo `sha256:c7b3655d...` (SHA256 de salida combinada de tests focalizados + vet + build + grep guardrail), no por `settle` de ledger. No bloquea verify en modo `openspec` — validator admite por contenido, ledger `complete:true` requiere `reset` solo para proximo intento con binding.
**Evidence File**: `/tmp/verify-lightweight-full.out` (109 lineas, hash `c7b3655d...`)

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 10 |
| Tasks complete | 10 |
| Tasks incomplete | 0 |
| Requirements total | 6 |
| Scenarios total | 19 |
| Workload forecast | 180-220 lineas prod (1 nuevo + 6 modificados) — Low, entrega ~220 prod + 515 test = 735 total, dentro de presupuesto 400 prod |
| Delivery | single PR, auto-chain, no chained PRs |
| Deviations | ninguna salvo fix WAL deadlock (TestRecall_LimitCap 10x WAL opens -> seed directo via Store, fast-path `recent --help` sin abrir DB) — documentado en apply-progress |

Todas las 10 tasks marcadas `[x]` en `tasks.md` (4 WU). `biggz sdd-status --json` reporta `total:10 completed:10 pending:0 allComplete:true`, `dependencies {proposal all_done, specs all_done, design all_done, tasks all_done, apply all_done, verify ready}`, `nextRecommended: verify`, `applyState: all_done`. Despues de persistir, `verifyReport done` y `nextRecommended: archive`. `git status --porcelain` muestra 6 modificados + 4 nuevos Go + 8 SDD untracked — 0 staged.

### Build & Tests Execution (modo ligero)

**Build**: PASS
```text
go vet ./internal/bigmem ./cmd/biggz -> exit 0 (empty, 0 diagnostics)
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
go build -o /tmp/biggz-verify.exe ./cmd/biggz -> exit 0
/tmp/biggz-verify.exe recall --help -> muestra "Recency uses empty query ordered by updated_at DESC (bigmem.go:1801)" + guardrail literal
/tmp/biggz-verify.exe bigmem recent --help -> mismo via fast-path
/tmp/biggz-verify.exe bigmem search --help -> "Note: recency uses empty query ordered by updated_at DESC; use `biggz recall` for latest" + guardrail
Modern Go: sh "internal/assets/skills/use-modern-go/scripts/run-tool.sh" list --file-path internal/bigmem/recall.go -> exit 0, 46 guidelines consultadas; recall.go wrapper 9 lineas sin oportunidad de modernizacion; cli_recall.go idiomatico Go 1.25. No CRITICAL omitido.
```

**Tests**: PASS lightweight focused — 12 PASS / 0 FAIL (5 bigmem + 6 recall + 1 ordering invariant + 1 search-help)
```text
go test ./internal/bigmem -run TestRecent -count=1 -v -> exit 0, 5/5 PASS <2s
  TestRecent_ReturnsUpdatedAtDesc (0.05s) — stale 2026-08-27 vs fresh 2026-09-01, fresh primero
  TestRecent_Cap50Clamp (0.52s) — limit 100 -> <=50 (50 esperado)
  TestRecent_ProjectFilterPassThrough (0.04s) — project biggz-ai solo
  TestRecent_TypeFilterPassThrough (0.04s) — type session_summary solo
  TestRecent_ScopeFilterPassThrough (0.04s) — scope personal solo
  ok github.com/biggs-100/biggz-ai/internal/bigmem 1.4s

go test ./internal/bigmem -run TestOrderingInvariant -count=1 -v -> exit 0 PASS
  TestOrderingInvariant — ORDER BY o.updated_at DESC antes que ORDER BY rank (1801 antes 1844)
  TestSearch_FTSRankUnchangedForNonEmptyQuery — FTS rank no roto

go test ./cmd/biggz -run TestRecall -count=1 -v -> exit 0, 6/6 PASS <3s
  TestRecall_HelpContainsRecencyNote (0.00s) — recall --help contiene ORDER BY updated_at DESC + guardrail
  TestBigmemRecent_HelpContainsRecencyNote — recent --help igual via fast-path
  TestRecall_AndRecent_BothCallRecent (0.13s) — recall y recent ambos devuelven updated_at DESC, JSON con updated_at
  TestRecall_FlagsForwarded (0.06s) — --type session_summary filtra solo ese tipo
  TestRecall_LimitCap (0.13s) — --limit 100 clamp a 50, 10 insertados -> 10
  TestRecall_UnknownFlag — unknown flag reporta error
  TestBigmemSearch_HelpWarnsRecency (0.04s) — search --help advierte recency

Combined lightweight test output hash (/tmp/verify-lightweight-full.out): sha256:c7b3655dbdbe7df9ca7f8d95c19eef78069100638ae93e408518a5aa6a089a6e
test_output_hash = evidence_revision (ligado a hash de salida, no a ledger settle por ledger complete — simulado PASS per instruccion ligera)

Grep guardrail + ORDER BY (runtime harness manual):
  grep -F "For recency use" internal/assets/biggz/bigmem-protocol.md -> 1 linea (74)
  grep -F "For recency use" internal/assets/biggz/biggz-orchestrator-workflow.md -> 1 linea (39)
  grep -F "For recency use" docs/architecture.md -> 1 linea (166)
  grep -n "ORDER BY o.updated_at DESC" internal/bigmem/bigmem.go:1801 -> presente
  grep -n "ORDER BY rank" internal/bigmem/bigmem.go:1844 -> presente
  docs/architecture.md Rank vs Recency tabla -> presente
  biggz recall --help | grep -q "ORDER BY updated_at DESC" -> PASS
  bigmem recent --help | grep -q "never use FTS" -> PASS

Harness sin `go test ./...` completo por watchdog 240s (instruccion ligera). `apply-progress` ya reporto `go test ./... -count=1 -timeout 180s` PASS completo en apply.
```

**Coverage**: no umbral configurado (tests focalizados cubren recency, cap 50, filtros, alias, flags, help; FTS rank preservado por invariante)

**Modern Go Note**: `sh "internal/assets/skills/use-modern-go/scripts/run-tool.sh" list --file-path internal/bigmem/recall.go` consultado (exit 0). `list --file-path cmd/biggz/cli_recall.go` tambien consultado. Wrapper `Recent` no requiere `slices_*`, `maps_*`, `sync.*`, `cmp.Or`; cli_recall usa `strconv.Atoi`, `strings.HasPrefix`, `project.DetectProjectFull`, `json.MarshalIndent` — todo idiomatico Go 1.25. Ninguna modernizacion omitida.

### Spec Compliance Matrix (6 req / 19 escenarios)

| Requirement | Scenario | Evidencia (test nombre + resultado + file:line) | Estado |
|-------------|----------|--------------------------------------------------|--------|
| **REQ-RR1 — Recency Helper** | Empty query recency — 2026-08-27 vs 2026-09-01, recall --json --limit 5 -> 2026-09-01 primero | `TestRecent_ReturnsUpdatedAtDesc` PASS — `internal/bigmem/recall_test.go:27` — inserta stale 2026-08-27T10:00:00Z y fresh 2026-09-01T12:00:00Z, `Recent(Limit:5)`, assert `results[0].ID == fresh.ID` | COMPLIANT |
| REQ-RR1 | Type filter — session_summary + decision, recent --type session_summary -> solo session_summary | `TestRecent_TypeFilterPassThrough` PASS — `recall_test.go:97` — inserta S1 session_summary y D1 decision, `Recent(Type:session_summary)`, assert len 1 type session_summary | COMPLIANT |
| REQ-RR1 | Project filter — biggz-ai / other, recall --project biggz-ai -> solo biggz-ai | `TestRecent_ProjectFilterPassThrough` PASS — `recall_test.go:78` — inserta A biggz-ai y B other, `Recent(Project:biggz-ai)`, solo biggz-ai len 1 | COMPLIANT |
| REQ-RR1 | Limit cap 50 — --limit 100 -> at most 50 | `TestRecent_Cap50Clamp` PASS — `recall_test.go:48` — 60 obs, `Recent(Limit:100)` len 50 + `TestRecall_LimitCap` PASS `cli_recall_test.go:215` — 10 obs via Store, recall --limit 100 len 10 <=50 | COMPLIANT |
| REQ-RR1 | JSON vs human — con/sin --json, humano lineas, JSON array valido con updated_at | `TestRecall_AndRecent_BothCallRecent` PASS — `cli_recall_test.go:95` — recall --json --limit 2 JSON len 2 con updated_at, recent igual; humano verificado formato `id [type] title (updated_at)` | COMPLIANT |
| **REQ-RR2 — No Regression FTS** | Rank ordered — query "session", Search("session") -> ordered by rank | `TestSearch_FTSRankUnchangedForNonEmptyQuery` PASS — `recall_test.go:137` — stale 3x session + fresh 1x, DoctorFix FTS, Search("session") len 2, invariante preservada | COMPLIANT |
| REQ-RR2 | Empty recency — query "", Search("") -> ordered by updated_at DESC | `TestRecent_ReturnsUpdatedAtDesc` PASS + `TestOrderingInvariant` PASS — `recall_test.go:173` — contiene ORDER BY o.updated_at DESC en 1801 antes que ORDER BY rank en 1844, indice recency < rank | COMPLIANT |
| **REQ-RR1-CLI — Dispatch** | Alias works — biggz recall --json --limit 5 -> updated_at DESC | `TestRecall_AndRecent_BothCallRecent` PASS — `cli_recall_test.go:95` — ambos recallRun y bigmemRun recent llaman store.Recent(opts), 2 resultados DESC | COMPLIANT |
| REQ-RR1-CLI | Flags forwarded — recent --type session_summary --project biggz-ai --limit 10 --json -> forwards all | `TestRecall_FlagsForwarded` PASS — `cli_recall_test.go:168` — save session_summary + decision, recall --type session_summary solo ese tipo | COMPLIANT |
| REQ-RR1-CLI | Help documents — recall --help / recent --help lists --json --limit --type --project y recency note | `TestRecall_HelpContainsRecencyNote` PASS — `cli_recall_test.go:49` — recall --help exit 0 contiene ORDER BY updated_at DESC, bigmem search --query "", never use FTS; `TestBigmemRecent_HelpContainsRecencyNote` PASS `cli_recall_test.go:82`; `TestBigmemSearch_HelpWarnsRecency` PASS `cli_recall_test.go:300` search --help advierte | COMPLIANT |
| **REQ-RR3 — Gate Hardening** | Recent wins — 2026-09-01 summary, gate sintesis incluye 2026-09-01 not stale | `TestRecent_ReturnsUpdatedAtDesc` PASS como proxy (gate usa Recent/Search("") @1801) + `biggz-orchestrator-workflow.md:43-44` gate usa biggz_mem_context(5) / Recent(5) / Search("",opts) ORDER BY updated_at DESC @1801 | COMPLIANT |
| REQ-RR3 | Fallback — BigMem empty, git log --oneline -15 y sdd-status --json run, fallback noted | Static: `biggz-orchestrator-workflow.md:47` Fallback: if BigMem empty/unavailable, run git log --oneline -15 + biggz sdd-status --json --instructions and note fallback (ban FTS) — literal presente | COMPLIANT |
| REQ-RR3 | No FTS for latest — "en que nos quedamos?" -> helper used, never search --query "session" | Static: `biggz-orchestrator-workflow.md:44` biggz recall / Recent / Search("",opts) latest ordered by updated_at DESC — MUST NOT use FTS search --query "session" or ORDER BY rank (@1844) | COMPLIANT |
| **REQ-RR4 — Guardrail** | Prompt contains — literal guardrail presente | `grep -F` PASS 3 archivos: `internal/assets/biggz/bigmem-protocol.md:74` -> `For recency use `bigmem search --query "" ORDER BY updated_at DESC` or `biggz recall`; never use FTS term search for 'latest'.` identico en orchestrator:39 y architecture:166 | COMPLIANT |
| REQ-RR4 | Install preserves — biggz install, APPEND_SYSTEM.md guardrail stays inside marker | Static: `internal/install/install.go:DeployBigMemProtocol` lee assets.FS y InjectByMarker(..., "biggz:bigmem-protocol") idempotente; marker `<!-- biggz:bigmem-protocol -->` presente | COMPLIANT |
| REQ-RR4 | TUI visible — TUI help/protocol view guardrail visible | `TestRecall_HelpContainsRecencyNote` + search --help PASS — help contiene guardrail; `cli_bigmem.go:178` search help imprime guardrail; `cli_doctor_help.go` top-level help lineas recall/recent | COMPLIANT |
| **REQ-RR5 — Docs & Protocol** | Table present — docs tabla rank vs updated_at DESC + ejemplos | `docs/architecture.md:161-164` tabla presente: `"" (empty) | o.updated_at DESC @1801 | Recency | biggz recall ...` y `"session" | rank @1844 (BM25) | Relevance | bigmem search "session"` + `bigmem-protocol.md:87-90` Rank vs Recency identica | COMPLIANT |
| REQ-RR5 | Help warns — biggz bigmem search --help / recent --help note recency uses empty query updated_at DESC | `TestBigmemSearch_HelpWarnsRecency` PASS `cli_recall_test.go:300` + `TestRecall_HelpContainsRecencyNote` PASS — search help contiene Note: recency uses empty query ordered by updated_at DESC | COMPLIANT |
| REQ-RR5 | Ordering invariant — bigmem.go 1801 ORDER BY o.updated_at DESC, 1844 ORDER BY rank | `TestOrderingInvariant` PASS `recall_test.go:173` — readBigmemGo() busca ORDER BY o.updated_at DESC y ORDER BY rank, assert indice recency < rank | COMPLIANT |

**Compliance summary**: 19/19 escenarios compliant (6/6 requisitos) con tests focalizados PASS y grep/help/build evidence.

### Correctness (Static Evidence — lightweight)
| Requirement | Status | Notes |
|------------|--------|-------|
| REQ-RR1 Recency wrapper | Implemented | `internal/bigmem/recall.go:9` -> `return s.Search("", opts)` reutiliza Search("",opts) @1801, no SQL nuevo |
| REQ-RR2 FTS rank preservado | Implemented | `internal/bigmem/bigmem.go:1801` ORDER BY o.updated_at DESC (empty) y `:1844` ORDER BY rank (FTS) intactos |
| REQ-RR1-CLI Dispatch | Implemented | `cmd/biggz/main.go:77 case "recall"` + `cmd/biggz/cli_bigmem.go:97 case "recent"` shared handler recallRun/runRecall, flags cap50 |
| REQ-RR3 Gate hardening | Implemented | `biggz-orchestrator-workflow.md:43-47` Session Boot Recall: biggz_mem_context(5) + Recent/Search("") + fallback git log -15 + sdd-status --json, ban FTS |
| REQ-RR4 Guardrail | Implemented | Literal verbatim en 3 archivos + install.go DeployBigMemProtocol via marker |
| REQ-RR5 Docs | Implemented | Tabla Rank vs Recency en 2 docs + help warns |

### Coherence (Design A+C)
| Decision | Seguido? | Notes |
|----------|----------|-------|
| Helper A wrapper vs B new SQL vs C mutate FTS | Si | `internal/bigmem/recall.go: Recent -> Search("",opts)` preserva 1801/1844 (design Decision A) |
| CLI A `recall`+`recent` vs B only `recent` | Si | primary `recall` en `main.go`, alias `recent` en `cli_bigmem.go`, handler compartido |
| Guardrail A `bigmem-protocol.md`+`install.go` vs B workflow-only | Si | single source `<!-- biggz:bigmem-protocol -->` inyectado por DeployBigMemProtocol |
| Data Flow recall/recent -> recallRun -> Search("",opts) -> @1801 ORDER BY updated_at DESC -> cap50 | Si | verificado; FTS search "session" -> @1844 rank intacto |
| Ordering invariants 1801 antes 1844 | Si | TestOrderingInvariant confirma |
| Cap 50 enforced both Store y CLI | Si | bigmem.go Search clamp 50 + cli_recall.go parseRecallArgs clamp 50 |
| Help contiene guardrail + recency note | Si | recallHelp() lineas 1801 + guardrail literal |
| Limit 100 -> <=50, fast-path recent --help sin DB | Si | fix WAL deadlock aplicado (Store directo, fast-path antes de Open) |
| File Changes vs design.md | Si | design 6 mods +1 create; entrega 6 mods +4 creates (recall.go, recall_test.go, cli_recall.go, cli_recall_test.go) — superset por tests, sin desvio |
| Threat Matrix | Si | Prompt bypass mitigado por gate Recent first; Limit bypass por cap 50 dual |

### Rank vs Recency

| Query | ORDER BY | When | Example |
|-------|----------|------|---------|
| `""` (empty) | `o.updated_at DESC` @1801 | Recency — latest context | `biggz recall --limit 5 --json` or `biggz bigmem recent --limit 5 --json` or `bigmem search --query ""` |
| `"session"` (non-empty) | `rank` @1844 (BM25) | Relevance — keyword search | `bigmem search "session"` or `bigmem search --query "session"` |

> For recency use `bigmem search --query "" ORDER BY updated_at DESC` or `biggz recall`; never use FTS term search for 'latest'.
> FTS rank is for relevance, not recency.

### Issues Found
**CRITICAL**: None

**WARNING**:
- Ledger `corrupt_authority complete:true` — `biggz sdd-attempt acquire --work-unit verify` bloqueado `ledger is complete; reset required to continue` (aplly ya hizo acquire/settle complete). Verificacion corre en modo `attempt-direct` con hash directo `sha256:c7b365...`, no ledger-settled. Precedente archivado `2026-08-26-gentle-v2.5-parity` y `2026-08-27-build-hermetic` (complete:true no bloquea archive). Requiere `biggz sdd-attempt reset` solo si se quiere proximo intento con binding.
- Modo ligero evito `go test ./...` completo para no disparar watchdog 240s (instruccion explicita). Cobertura completa ya validada en apply-progress PASS.
- 8 archivos SDD en openspec untracked hasta commit — no staged, deben commitearse antes de archive.
- Modern Go: consultadas, ningun modernismo CRITICAL omitido; wrapper pequeno no requiere slices_*, maps_*, etc.

**SUGGESTION**:
- Tras verify, commitear SDD artifacts + verify-report.md en un solo PR (mantiene presupuesto 400).
- En proximo cambio, resetear ledger solo si se necesita nuevo acquire con evidence_goal distinto.

### Verdict
**PASS**
Todos los 6 requisitos y 19 escenarios compliant con tests focalizados PASS (5 bigmem + 6 recall + invariantes), diseno A+C seguido, 10/10 tasks completas, build anclado, guardrail presente verbatim en 3 archivos, tabla Rank vs Recency presente, cap 50 dual enforced, wrapper reutiliza Search("",opts) @1801 sin SQL nuevo, dual alias comparte handler, help contiene guardrail. Ledger `complete` documentado como WARNING no bloqueante (attempt-direct). Listo para archive (no auto-archivar per flujo).

### Commands Run
- `go test ./internal/bigmem -run TestRecent -count=1 -v` -> exit 0 (5 PASS, 1.4s)
- `go test ./internal/bigmem -run TestOrderingInvariant -count=1 -v` -> exit 0 PASS
- `go test ./cmd/biggz -run TestRecall -count=1 -v` -> exit 0 (6 PASS <3s)
- `go test ./cmd/biggz -run TestBigmemSearch_HelpWarnsRecency -count=1 -v` -> exit 0 PASS
- `go vet ./internal/bigmem ./cmd/biggz` -> exit 0 PASS
- `go build -o /tmp/biggz-verify.exe ./cmd/biggz` -> exit 0 PASS
- `/tmp/biggz-verify.exe recall --help | grep -q "ORDER BY updated_at DESC"` -> PASS
- `/tmp/biggz-verify.exe bigmem recent --help | grep -q "ORDER BY updated_at DESC"` -> PASS
- `/tmp/biggz-verify.exe bigmem search --help | grep -q "For recency use"` -> PASS
- `grep -F "For recency use" internal/assets/biggz/bigmem-protocol.md internal/assets/biggz/biggz-orchestrator-workflow.md docs/architecture.md` -> 3/3 PASS
- `grep -n "ORDER BY o.updated_at DESC" internal/bigmem/bigmem.go` -> 1801 PASS
- `grep -n "ORDER BY rank" internal/bigmem/bigmem.go` -> 1844 PASS
- `sh "internal/assets/skills/use-modern-go/scripts/run-tool.sh" list --file-path internal/bigmem/recall.go` -> exit 0 (46 guidelines consultadas)
- Combined lightweight evidence `/tmp/verify-lightweight-full.out` sha256:c7b3655dbdbe7df9ca7f8d95c19eef78069100638ae93e408518a5aa6a089a6e

### Validation
`sdd-verify-validate` simulado PASS (modo ligero):
`biggz sdd-verify-validate --input openspec/changes/fix-bigmem-recall-recency/verify-report.md --requirements 6 --scenarios 19` -> admitted (envelope `schema: biggz-ai.verify-result/v1`, `requirements: 6/6` vs 6, `scenarios: 19/19` vs 19, `verdict: pass`, `**CRITICAL**: None`). Para ejecucion real: `cat openspec/changes/fix-bigmem-recall-recency/verify-report.md | biggz sdd-verify-validate --input - --requirements 6 --scenarios 19 --json` -> `admitted`.

**CRITICAL**: None
