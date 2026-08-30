# Design: 2026-08-30-ola2-guardrails-preflight-synthesis — Ola 2 Guardrails / Preflight / Synthesis Gate

## Technical Approach

Retroactive closure for `9f6c8be` (on `main`, `470` lines). No new prod diff; this design documents port decisions mapping gentle-pi `guardrails.ts`, `sdd-preflight.ts`, `synthesis-gate.ts` into three Go files. Single `openspec` slice (`auto-chain`, `stacked-to-main`, `400` budget `Medium`) with 7 requirements and 21 scenarios across `specs/policy` and `specs/sdd`. Verbatim pattern semantics where Go lacks lookahead, fail-closed safe defaults on malformed config, injectable `now` for `120s` window.

Spec deltas: `specs/policy` (4 req: deny + classify + config + sensitive) and `specs/sdd` (3 req: canonicalize + persist + gate).

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|----------|---------|----------|--------|
| **Deny `IsDenied`** | A: greedy regex B: 6-pattern slice | A blocks `git clean`/`git push` without flags (no lookahead) | **B** — 6 patterns with index `2` (`git clean` needs `-f`+`-d`) and `3` (`push` needs `--force`/`-f`) |
| **Classify** | A: guarded first B: denied first | A would `confirm` `push --force` | **B** — denied→`block` first; then 5 keys with `!auto→confirm`, else `guardedCommands` override, else defaults (`gitPush allow`, `npmPublish block`) |
| **Config parse/merge** | A: strict struct B: allowlist+merge | A rejects new keys | **B** — filter `validActions×validKeys`; env fast-path; read global→project; malformed→safe; merge project wins |
| **Sensitive path** | A: suffix B: 8 regexes normalized | A misses `~/.ssh`, `secrets/.env` | **B** — 8 regexes; `lower+~→HOME`; recurse `path/paths/file…`; guard `read/write/edit` only |
| **Preflight canonicalize** | A: case-sensitive B: alias folding | A splits `BigMem` | **B** — `both/hybrid/engram/bigmem→hybrid`, `none→""`; fill `interactive/400` |
| **Preflight persist** | A: repo file B: `GENTLE_PI_CONFIG_HOME`+cache | A pollutes repo | **B** — `home[0]`>env>`UserHomeDir/.pi/gentle-ai`; `MkdirAll 0755`/`0644`; `cache>disk>defaults` |
| **Synthesis gate** | A: immediate B: 120s+bypass | A blocks late/child | **B** — 4 markers; `!child&&!recall&&checkpoint&&≤120s&&!hasSynthesis` |

## Spec References

- `specs/policy/spec.md` — guardrails deny, classify, config merge, sensitive path (4 req)
- `specs/sdd/spec.md` — store canonicalize, disk persist/resolve, synthesis gate (3 req)

Requirements mirror `tasks.md` 11 tasks (Phases 1–4) with Given/When/Then scenarios (21 total).

## Data Flow

Guardrails: `IsDenied` 6 patterns (2/3 flag refinement) → `Classify` denied→block → autonomous branch → sensitive `Evaluate` collects→normalize→regex→`Block`. Config: env→global→project→malformed→safe merge. Preflight: `canonicalize`→`Write/Read`→`Resolve` cache>disk>defaults. Synthesis: `SetCurrentTurnMarkdown(Now)`→`ShouldBlock` child/recall/checkpoint/120s/synthesis.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/policy/guardrails.go` | Created | `deniedBashPatterns[6]`, `Guard*` consts, `guardedKeyPatterns[5]`, `autonomousDefaultActions`, `RuntimeGuardrailsConfig`, `IsDenied`, `ClassifyGuardedCommand`, `Parse/LoadRuntimeGuardrailsConfig`, `gentlePiConfigHome`, `sensitivePathPatterns[8]`, `isSensitivePath`, `collectPathInputs`, `EvaluateSensitivePathTool` |
| `internal/sdd/preflight.go` | Created | `PreflightPrefs`, `preflightCache`, `SddPreflightDiskPath`, `NormalizePreflightArtifactStore`, `canonicalizePrefs`, `Write/ReadSddPreflightToDisk`, `Set/Get/Clear/ResolvePreflightPrefs`, `ValidatePreflightQuestionEnvelope`, `SessionRecallMarkdown` |
| `internal/sdd/synthesis_gate.go` | Created | `synthesisMarkers[4]`, globals, `SetCurrentTurnMarkdown`, `HasSynthesis`, `HasSessionRecall`, `IsChildBypass`, `IsCheckpointAsk`, `ShouldBlock` (120s), `CheckSynthesisPrecondition` |
| `specs/policy/spec.md` | Created | Delta for policy — 4 req, 12 scenarios |
| `specs/sdd/spec.md` | Created | Delta for sdd — 3 req, 9 scenarios |
| `tasks.md` / `apply-progress.md` / `proposal.md` | Created | 11 tasks 4 phases, Forecast `~470` `Medium` single PR `stacked-to-main`, all `[x]` |

## Interfaces / Contracts

```go
func IsDenied(command string) bool
func ClassifyGuardedCommand(command string, cfg RuntimeGuardrailsConfig) string
func ParseGuardrailsConfigFile(raw string) (*RuntimeGuardrailsConfig, bool)
func LoadRuntimeGuardrailsConfig(cwd string, configHome ...string) RuntimeGuardrailsConfig
func EvaluateSensitivePathTool(toolName string, input any) *ToolCallDecision

func SddPreflightDiskPath(home ...string) string
func NormalizePreflightArtifactStore(s string) string
func WriteSddPreflightToDisk(p PreflightPrefs, home ...string) error
func ReadSddPreflightToDisk(home ...string) (PreflightPrefs, bool)
func ResolvePreflightPrefs(cwd string, home ...string) PreflightPrefs
func ValidatePreflightQuestionEnvelope(env PreflightQuestionEnvelope) bool
func HasSynthesis(md string) bool
func ShouldBlock(question string, md string, now time.Time) bool
func CheckSynthesisPrecondition(question string, md string) (bool, string)
```

## Testing Strategy

Unit deny (each pattern + flag refinement → true/false), classify (denied→block, auto defaults/custom, `!auto→confirm`, `not-guarded`), config merge (`TempDir` isolated `GENTLE_PI_CONFIG_HOME`, env fast-path, malformed→safe, merge), sensitive path (8 patterns, array/nested inputs, `exec` nil), preflight (alias folding, defaults, write→read round-trip `0644`, resolve precedence, envelope enums), synthesis (4 markers, bypasses, `120s` injectable `now`). Integration `go vet`, `gofmt -l`, `git show 9f6c8be --stat 470`, `sdd-status verify ready` (strict_tdd false defers full tables to verify).

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `rm -rf` over-block | Low | Restricted to `/(~|$HOME|..)` roots |
| Config drift on Windows | Low | `filepath.Join` + env override; `Normalize` lower |
| `120s` clock flake | Low | Injectable `now`; prod `time.Now` only wrapper |

## Migration / Rollout

No migration. `9f6c8be` on `main`; `git revert 9f6c8be` removes 3 files (470). Change is `openspec` filesystem only, `strict_tdd false`, no BigMem topics. Single PR `470` (`251+152+67`) `Medium` exceeds `400` by `70` accepted as `size:exception-ok`. Gates: `go vet` PASS, `gofmt` clean, `biggz sdd-status verify ready`.

## Open Questions

- [x] 470 vs 400: one commit already merged; document as `Medium` single PR.
- [x] `preflight` alias (`bigmem→hybrid`) vs store (`bigmem→engram`) divergence intentional.
- [ ] Future verify adds table-driven tests (9f6c8be has no `*_test.go`).

