# Design: Synthesis Gate Hardening

## Technical Approach

Harden `b0d2fc1` so human always sees `## Sub-agent Result` before deciding. 3 layers (R1-R4): (1) prompt copy-pasteable + verifiable, (2) Pi JS gate blocks `ask/question` without 4 markers via same-turn buffer + thin advise, (3) unit+integration+CI lock drift. No SDD logic change.

## Architecture Decisions

### Prompt as machine-verifiable invariant

| Option | Tradeoff | Decision |
|---|---|---|
| Free-form instruction | Low friction, drift-prone | Rejected |
| Copy-paste block + `INVALID and will be blocked` + `REMINDER` per ask | Verifiable by gate+test, minor bloat | **Chosen** |
| Code-only rule | Strong enforcement, loses guidance | Rejected |

Rationale: `REMINDER: synthesis markdown is separate chat markdown emitted FIRST...` after every ask keeps prompt/gate convergent; 12 REMINDERs already present.

### JS gate source priority + blocking

| Option | Tradeoff | Decision |
|---|---|---|
| Check only `ctx.history` | Fails streaming race | Rejected |
| `currentTurnMarkdown` → `ctx.history` → `lastAssistant` (120s) | Covers race + lag | **Chosen** |
| `throw` on block | Aborts tool loop | Rejected |
| `{isError:true, text:"Please synthesize..."}` + no `original()` + `notify` | Pi-native error, tool not invoked | **Chosen** |

Rationale: `ask` with no markdown was the bypass; buffer fixes ms-race. 4-marker check blocks param-only synthesis. 120s balances false-block vs stale-pass.

### Thin heuristic + advise gating

| Option | Tradeoff | Decision |
|---|---|---|
| Block on thin | Noisy for small changes | Rejected |
| `countPaths<2 \|\| len<50` via `extractArtifactsSection` → `concern` only when `BIGGZ_ADVISE=1`, off default | Heuristic warn, no block, opt-in | **Chosen** |
| Allow `BIGGZ_ORCHESTRATOR` bypass | Reopens bypass | Rejected — only `PI_SUBAGENT_CHILD=1` |

Rationale: `countPaths` handles bullets/comma/slash; `extractArtifactsSection` cuts at `Risks`/`Next`/`## `. Advise is `pi.notify` warning only.

### Test layering

| Option | Tradeoff | Decision |
|---|---|---|
| Only JS unit | Misses prompt drift | Rejected |
| JS 4-scenario + Go template invariant + CI `go vet`/`go test`/`node --check`/`node --test` | Covers logic + convergence | **Chosen** |

## Data Flow

```
sub-agent {summary, artifacts, risks, next}
  → orchestrator emits markdown FIRST, same turn, adjacent
    ## Sub-agent Result: {phase/agent}
    **Artifacts/Paths:** a/b, c/d  (≥2, ≥50)
    **Risks / Open Questions:** ... **Next Recommended:** ...
  → recordText() → currentTurnMarkdown buffer
  → ask_user_question/question (same turn)
  → gate pi.registerTool wrapper:
      PI_SUBAGENT_CHILD=1 ? bypass
      : checkSynthesisPrecondition(currentTurn→history→lastAssistant 120s)
        false → block {isError:true} + notify, NO original()
        true  → isThin && BIGGZ_ADVISE=1 ? notify concern (warn) + allow
              : allow → original() → reset buffer
  → pi.on("tool_call") secondary guard (load-order safe)
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/assets/biggz/biggz-orchestrator.md` | Modify | Keep copy-paste `## Sub-agent Result` block, `INVALID and will be blocked` same-turn rule, `REMINDER: synthesis markdown is separate...` after every ask (verify 12× convergence) |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modify | 4-marker blocking, source `currentTurnMarkdown→history→lastAssistant` 120s, `isError:true` no-call, `countPaths`/`extractArtifactsSection`/`isThinSynthesis`, `BIGGZ_ADVISE=1` advise, only `PI_SUBAGENT_CHILD=1` bypass, expose `_biggzSynthesisGate` helpers |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Modify | 4 scenarios: missing→`isError:true` not-called, rich pass, thin+advise warn pass, thin silent without flag; + child bypass, race |
| `internal/assets/biggz/orchestrator.test.go` | Create | Reads `biggz-orchestrator.md`, asserts `## Sub-agent Result`, `**Artifacts/Paths:**`, `INVALID and will be blocked`, `REMINDER` |
| `docs/architecture.md` | Modify | Add `### Synthesis Gate (3-layer defense)` subsection |

## Interfaces / Contracts

```js
// block
{ content:[{type:"text", text:"Please synthesize before asking — missing ## Sub-agent Result block..."}],
  isError:true } // original() NOT called, pi.notify + ctx.ui.notify error
// thin advise (allow)
pi.notify("[biggz-synthesis-gate] concern: synthesis is thin (Artifacts/Paths count=N, len=M)...","warning")
```

`pi._biggzSynthesisGate`: `hasSynthesis`, `extractArtifactsSection`, `countPaths`, `getArtifactsMetrics`, `isThinSynthesis`, `isAdviseEnabled`, `checkSynthesisPrecondition`, `_test`. Invariants: 4 markers; thin=`count<2||len<50`; advise=`BIGGZ_ADVISE=1|true` or `pi.settings` advise; bypass only `PI_SUBAGENT_CHILD=1`.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | missing→`isError:true`; rich→pass; thin+advise→warn pass; thin no flag→silent; child bypass; race | `node --test .../biggz-synthesis-gate.test.mjs` fixtures only |
| Integration | template invariant guards drift | `go test ./...` asserts markers |
| CI | `go vet`/`go test -short`/`node --check`/`node --test` green | `ci.yml` format→vet→test; blocks merge |

## Threat Matrix

N/A — no routing/shell/subprocess/VCS/PR/executable classification that composes commands.

| Boundary | Applicability | Reason |
|---|---|---|
| Documentation-like paths | N/A | Only marker classification, no file execution |
| Git repository selection | N/A | No `git -C`/cwd logic |
| Commit state | N/A | No index interaction |
| Push state | N/A | No push/ref logic |
| PR commands | N/A | No `gh pr` composition |

Env reads (`BIGGZ_ADVISE`, `PI_SUBAGENT_CHILD`) are boolean gates, not injection surfaces.

## Migration / Rollout

No migration. Backward-compatible; blocking tightens, advise off preserves thin pass. Rollback `git revert <sha>`.

## Open Questions

- [ ] `orchestrator.test.go` package path `internal/assets/biggz` — verify testable or move to sibling.
- [ ] `docs/architecture.md` subsection placement.

