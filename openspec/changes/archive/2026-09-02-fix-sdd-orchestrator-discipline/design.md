# Design: fix-sdd-orchestrator-discipline

## Context

Orchestrator allows inline writes, auto-continue without `proceed|adjust|stop`, delegates SDD via `general`, skips workflow/delegation reads, and verifies without RDD receipt. Specs 14 REQ (orch 4, sdd 4, rdd 3, review 3) require bilingual gate (120s), SD Authority, mandatory reads, ladder fail-closed, and review gate.

## Goals

- Blocking checkpoint: 4 markers + `HasSynthesis`/`IsCheckpointAsk` bilingual +120s.
- SD Authority: SDD only via `sdd-*`; `general`/`explore` blocked.
- Mandatory reads with evidence; unreadable blocks.
- RDD gate before `verify`: enabled requires `receiptValid`+chain; disabled → `unmanaged` no PASS.
- Ladder: size/count never selects SDD.

## Non-Goals

New phases/artifact types, review lens changes, BigMem migration, ledger `evidence_revision` changes.

## Technical Approach

Harden fail-closed; Go mirrored in JS. Extend `synthesis_gate.go` (bilingual, strict 4-marker, 120s). Add `authority.go`. Require reads. Wire RDD via `RDDStatus`+`Validate` → `rdd_*`.

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|---|---|---|---|
| Gate authority | JS-only vs Go+JS | JS drifts | Go `ShouldBlock` canonical, JS mirrors |
| Tokens | English vs bilingual | English breaks `es` UI | Bilingual `proceed\|adjust\|stop\|continue\|correct`+`continuar\|ajustar\|detener\|parar\|corregir\|proseguir\|cerrar` |
| Window | History vs strict same-turn | History allows stale `## Sub-agent Result` | Strict `currentTurnMarkdown` ≤120s; history only advise thin |
| Authority | Prompt vs code guard | Prompt ignored | Code guard `orchestrator/*` + fallback `synthesis_gate.go` |
| Reads | Implicit vs explicit | Implicit skipped | Require reads; unreadable blocks |
| RDD gate | Agent vs dispatcher+preflight | Agent bypassed via binary | Both preflight+dispatcher; `RDDStatus`+`Validate` |
| Ladder | Size vs explicit intent | Size auto-selects SDD | Explicit `biggz sdd-new`/`sdd-continue`/ask only |

## Data Flow

```
Human -> Orchestrator (reads workflow+delegation) -> SD Authority -> ShouldBlock(md,question,now)
 -> HasSynthesis? IsCheckpointAsk? HasSessionRecall? 120s -> block/allow -> ask_user_choice
 -> dispatcher sdd-status (nextRecommended+blockedReasons)
 -> verify preflight RDDStatus() -> enabled: require receipt else block; disabled: allow
 -> sdd-verify/sdd-apply
```



## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/sdd/synthesis_gate.go` | Modify | Bilingual tokens, 4-marker, 120s, `HasSessionRecall` |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modify | Mirror Go |
| `internal/orchestrator/authority.go` | Create | `GuardSDAgentAuthority` map `→sdd-*` |
| `internal/orchestrator/surfaces.go` | Modify | Wire authority |
| `internal/assets/biggz/biggz-orchestrator.md` | Modify | Checkpoint template |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modify | Reads + dispatcher |
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | Modify | Ladder + Authority |
| `internal/sdd/synthesis.go` | Modify | `DetectLanguage`, `RenderSynthesisLocalized` |
| `internal/sdd/verify.go` | Modify | RDD gate |
| `internal/sdd/status*.go` | Modify | Propagate `rdd_*` |
| `internal/review/*` | Modify | `RDDStatus`, `Validate`, `domainHash` |

## Interfaces / Contracts

```go
func HasSynthesis(md string) bool // 4 markers incl |Topic|Decision|
func IsCheckpointAsk(q string) bool
func ShouldBlock(q, md string, now time.Time) bool // !child && !recall && checkpoint && ≤120s && !HasSynthesis
func GuardSDAgentAuthority(phase, agent string) error // SDD → sdd-* else SD Agent Authority
func VerifyPreflight(change string) error // enabled→ receiptValid&&chain.Valid&&binding else rdd_*
type RDDStatusReport struct{ EffectiveMode RDDMode; Source, Revision string }
func (r PersistedReceipt) Validate() error
```

JS scans `questions[].options[].label/value/id/name/title` bilingual.

## Alternatives Considered

See Architecture Decisions; rejected: JS-only, English-only, history fallback, prompt-only, implicit reads, agent-only RDD, size heuristic.

## Risks

| Risk | Mitigation |
|------|------------|
| Gate blocks auto | Allow `auto` only when explicitly chosen; surface failures |
| Strict authority | Fail-closed with `sdd-*` suggestion |
| Review flakiness | Disabled → `unmanaged` not PASS |

## Threat Matrix

No shell/VCS/PR/executable routing beyond SDD agent routing; agent routing covered by authority tests.

| Boundary | Cases | Applicability | Response | RED tests |
|---|---|---|---|---|
| Docs paths | `requirements.txt`, `CMakeLists.txt`, MDX, `README.sh` | N/A: no classification | None | None |
| Git repo sel. | `git -C`, relative/absolute | N/A: no `git -C`/cwd change | Keep `detectGitDirs` | None |
| Commit state | staged, `commit -a`, empty index | N/A: no index handling | None | None |
| Push state | tracking/first push/refspec | N/A: no push logic | None | None |
| PR commands | `--head`, env prefix, composed | N/A: no `gh pr` composition | None | None |

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | 4-marker, bilingual, `ShouldBlock` 30s/121s, `HasSessionRecall`, reject `general` | `go test ./internal/sdd` + `biggz-synthesis-gate.test.mjs` |
| Integration | Missing read blocks; 12-file diff stays Simple Delegation | Stub reads; `sdd-status --json` |
| E2E | RDD enabled no receipt blocks, valid allows, disabled allows, tampered hash blocks | Temp repo + `biggz rdd` + receipt |

Suites: `synthesis_test.go`, `biggz-synthesis-gate.test.mjs`, `status_v2_test.go`.

## Migration / Rollout

No migration. Rollout: 1) gate+JS 2) authority+docs 3) RDD gate. Rollback: revert 3 commits; no data migration.

## Open Questions

- [ ] Keep `cerrar` token? Go parity needed.
- [ ] `auto` permissive vs fully blocking? Spec keeps permissive.
