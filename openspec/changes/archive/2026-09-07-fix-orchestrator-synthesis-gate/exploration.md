# Exploration: fix-orchestrator-synthesis-gate

## Current State

Orchestrator contract (`biggz-orchestrator.md` Post-Delegation Human Checkpoint + `biggz-orchestrator-workflow.md` Visible Context Before Every Question + `biggz-orchestrator-delegation.md` Ask contract) requires synthesis markdown with **verbatim English markers** emitted as **plain chat FIRST, same turn, adjacent** before every checkpoint `ask_user_choice` / `ask_user_question` / `question` call:

```markdown
## Sub-agent Result: {phase/agent}
**What was done:** | Topic | Decision | (+ checklist)
◆ {phase} · {status} · {next}
**Artifacts/Paths:** {list}
**Risks / Open Questions:** {…}
**Next Recommended:** {…}
```

Gate checks are `internal/sdd/synthesis_gate.go:HasSynthesis` (4 markers: `## Sub-agent Result`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**` + `**What was done:**` OR `| Topic | Decision |`) and `IsCheckpointAsk` (label/value/id/name/title scan of `questions[].options[]` + top-level `options[]`; bilingual tokens `proceed/continuar/proseguir/proceder/procede`, `adjust/ajustar`, `stop/detener/parar`, `continue/continuar`, `correct/corregir/cerrar`; **body text never a signal** since 2026-09-03).

**Enforcement retired 2026-09-04** (`cf840430` refactor + `944e284e` deletion): blocking functions deleted (`ShouldBlock`/`BuildBlockedEnvelope`/`ShouldBlockApplyAdmission`/`SetCurrentTurnMarkdown` + `currentTurnTime` 120s window + `BlockedFallbackEnvelope`). Go file now pure advise helpers (`HasSynthesis`, `IsCheckpointAsk`, `HasOptions`, `HasSessionRecall`, `IsChildBypass`, `renderLifecycle`). JS gate (`internal/assets/pi/biggz-synthesis-gate.js` 912 LOC) wrappers are passthrough (`return origExecute(...args)` without check) and `tool_call` handler early-returns; helpers remain exposed via `pi._biggzSynthesisGate` (`hasSynthesis`, `isCheckpointAsk`, `checkSynthesisPrecondition`, `currentTurnMarkdown` buffer via `message_end`/`message_update`/`assistant_message` + `turn_start`/`agent_start` reset, thin metrics `extractArtifactsSection`/`countPaths`).

`branch-worktree-cleanup` (interactive, openspec, `auto-chain`, budget 400; commit `dbccfff8`) delegated 8 phases (`explore → propose → spec → design → tasks → apply → verify → archive` per `sdd-init/{project}` + dispatcher). Orchestrator emitted synthesis only for `explore` and a combined final block, **skipping synthesis before checkpoint for 6 intermediate phases** (propose/spec/design/tasks/apply/verify). Gate did not block because advise-only passthrough was active. User flag was correct: contract is **after EVERY delegated sub-agent**, not only at end. In `interactive` the per-phase `proceed/adjust/stop` checkpoint is mandatory; `auto` autonomous continuation uses 3-line recap but must still emit full template before any real checkpoint.

### Affected Areas
- `internal/sdd/synthesis_gate.go` — advise-only; needs `ShouldBlock`/`BuildBlockedEnvelope`/`CheckSynthesisPrecondition` + turn state (`currentTurnMarkdown`/`currentTurnTime` 120s) restored; parity with JS `isCheckpointAsk` (label-only)
- `internal/assets/pi/biggz-synthesis-gate.js` — wrappers passthrough, `tool_call` guard dead; needs re-hardened fail-closed blocking (`{isError:true}` in execute + `{block:true}` in tool_call) with strict `currentTurnMarkdown`-only check, `BIGGZ_ADVISE=1` thin-advise opt-in, child bypass, session-recall/preflight exceptions, load-order sweep
- `internal/sdd/synthesis.go` — `RenderSynthesis`/`RenderSynthesisLocalized` produces required markers; `renderLifecycle` already shared with gate; add `DetectLanguage`/`languageHint` dual-write check
- `internal/assets/biggz/biggz-orchestrator.md` + `biggz-orchestrator-workflow.md` + `biggz-orchestrator-delegation.md` — docs still contain "ENFORCEMENT RETIRED… passthrough" wording; need restored blocking language + `REMINDER: synthesis markdown is separate chat markdown emitted FIRST…` convergence (12×) and `BIGGZ_ADVISE` doc
- `internal/assets/biggz/orchestrator_test.go` (referenced in `docs/architecture.md` Layer 3) — asserts 4 markers + `INVALID and will be blocked` + 12× REMINDER; currently stale vs retired code
- `internal/sdd/synthesis_gate_test.go` + `internal/sdd/synthesis_test.go` + `internal/assets/pi/biggz-synthesis-gate.test.mjs` — tests assert **passthrough** (e.g. `checkpoint without synthesis allows`); need re-enabled blocking fixtures (checkpoint missing→block, history-only→block, expired 120s→block, rich currentTurn→pass, general ask→pass, thin+advise→warn pass, child bypass)
- `internal/sdd/pending.go` + `internal/sdd/question.go` — `PersistPendingForCheckpoint`/`ValidateQuestionEnvelope` (16/60/4/2-4 limits) + pending dual-write; checkpoint ask must persist pending before question
- `docs/architecture.md` Synthesis Gate 3-layer defense — documents layers 1-prompt, 2-Pi gate, 3-tests/CI; needs un-retired description after re-harden

### Approaches

1. **Re-harden JS gate to blocking (fail-closed) — restore strict same-turn currentTurnMarkdown check**
   - Restore wrapped `execute` logic: if `!IsChildBypass()` && `!HasSessionRecall(currentTurn)` && `isCheckpointAsk(params)` && `!hasSynthesis(currentTurn)` within 120s → return `{isError:true, content:[{type:"text", text:"Please synthesize before asking…"}]}` without calling original; similarly `pi.on("tool_call")` → `{block:true, reason}`. Thin case (`countPaths<2 || len<50`) blocks only if `BIGGZ_ADVISE=1` → non-blocking `pi.notify` concern warning, else silent pass. History (`ctx.history`/`lastAssistant`) advise-only never satisfies block except `getCurrentTurnSynthesis` fallback for concern. Keep `PI_SUBAGENT_CHILD=1` bypass, `turn_start`/`agent_start` reset, `message_end`/`message_update` buffer streaming race fix, load-order sweep (`pi.tools`/`pi._tools`/`getAllTools`), preflight allowance when no synthesis ever existed, session-recall same-turn exception (`## Session Recall` in `currentTurnMarkdown`).
   - Pros: catches violation at runtime closest to user, works in Pi extension today, load-order safe (both execute wrap + tool_call), parity with gentle-pi; fixes branch-worktree-cleanup reproduction deterministically (7 missing syntheses would have blocked)
   - Cons: Pi-only — `question` tool in OpenCode needs separate plugin parity; narrow streaming-race risk if orchestrator markdown not captured before tool_call (mitigated by `currentTurnMarkdown` buffer but still timing-sensitive); restores ~250 LOC deleted code to maintain
   - Effort: Medium (restore deleted paths from `cf840430^` + adjust bilingual label-only `isCheckpointAsk`, re-enable `isThinSynthesis` advise)

2. **Add orchestrator-side hard fail — make biggz-orchestrator validate synthesis before calling question via synthesis_gate.go ShouldBlock**
   - Restore Go `ShouldBlock(question, md, now)` + `CheckSynthesisPrecondition` + `BuildBlockedEnvelope` (REQ-DG-1: only `IsCheckpointAsk` gates, `HasOptions` alone never blocks; REQ-DG-2: blocked payload carries `context` + `FormatFallback` so question not swallowed). Orchestrator prompt adds explicit self-check: re-read only question text + options, confirm synthesis markdown already emitted current turn, if `HasSynthesis(currentTurn)==false` → emit missing-synthesis error and fallback plain chat via `FormatFallback` instead of calling `question`. Optionally `ShouldBlockApplyAdmission` for write-admission (apply) that ignores child/recall bypass to prevent auto back-to-back phases self-validating entry into writing. Wire into `internal/sdd/status.go` or orchestrator pre-delegation hook if available.
   - Pros: single source of truth (`synthesis_gate.go` canonical, JS mirrors), runtime-agnostic (covers OpenCode `question` without Pi extension), integrates with pending dual-write and 120s window, works even when Pi extension not loaded
   - Cons: orchestrator is LLM agent — self-check is still voluntary unless harness enforces (needs wrapper that intercepts `question` tool call server-side); without harness hook can be skipped like branch-worktree-cleanup did; adds Go turn-state global (`currentTurnMarkdown`/`currentTurnTime`) to maintain; duplicates JS logic (drift risk noted in `biggz-orchestrator.md`)
   - Effort: Medium (restore Go blocking helpers ~80 LOC + orchestrator prompt hardening + optional harness intercept; no new dependency)

3. **Add CI lint/test that fails if any orchestrator turn contains checkpoint ask without preceding synthesis in same turn (static analysis)**
   - Add regression test: replay branch-worktree-cleanup transcript fixture (8 delegations, 2 syntheses) → script scans orchestrator log/transcript turns for `isCheckpointAsk` without preceding `HasSynthesis` in same turn → fails. Extend `synthesis_gate_test.go` to re-assert blocking fixtures (checkpoint missing, history-only, expired window, preflight allowance, session-recall narrow exception, label-only vs body-token) and `biggz-synthesis-gate.test.mjs` to assert `isError:true` + `{block:true}` paths. Add `go vet`/`node --test` + new `orchestrator_transcript_lint` in CI that parses `pi`/`opencode` tool-call logs; add `docs/architecture.md` CI bullet update.
   - Pros: deterministic, no runtime false positives, documents contract as living tests, prevents drift after re-harden, cheap to maintain; catches static drift in `biggz-orchestrator.md` markers (already tested via `orchestrator_test.go` marker convergence)
   - Cons: post-hoc — does not prevent live user seeing naked checkpoint question; requires transcript/log capture (not all runs persist structured log); won't stop a live violation, only flags after merge
   - Effort: Low (add ~150 LOC tests using existing `mustSynthesisMD` fixtures + transcript fixture, no runtime code)

### Recommendation

**GO — re-enable blocking with minimal safe combined design (approach 1 + 2 + 3), but scoped to avoid retired failure modes.**

Minimal safe design (in proposal order):

1. **Restore Go canonical** (`internal/sdd/synthesis_gate.go`): re-add `ShouldBlock`/`CheckSynthesisPrecondition`/`BuildBlockedEnvelope` + `ShouldBlockApplyAdmission` (write-admission) with `IsCheckpointAsk` label-only (not body text), `HasSynthesis` 4-marker + table/prose, 120s `currentTurnTime` window, `IsChildBypass`, `HasSessionRecall` narrow exception, plus `SetCurrentTurnMarkdown`/`currentTurnTime` global. This is the truth JS must mirror.
2. **Re-harden JS gate** (`internal/assets/pi/biggz-synthesis-gate.js`): unwrap passthrough — restore `wrapSingleTool` strict check (`currentTurnMarkdown`-only, 120s, `isCheckpointAsk` label-only, not `hasOptions`) returning `{isError:true}` and `tool_call` returning `{block:true}`; keep `BIGGZ_ADVISE=1` thin-advise as **concern-only** (`Artifacts/Paths` count<2 || len<50) via `pi.notify`, never blocking unless `BIGGZ_ADVISE=1` opt-in is documented as thin-warn. Preserve `currentTurnMarkdown` buffer via `message_end`/`message_update`/`assistant_message` + `turn_start` reset + `tool_execution_end` reset after success, load-order sweep, child bypass (`PI_SUBAGENT_CHILD=1`), preflight allowance (no prior synthesis ever), session-recall same-turn exception. Remove "ENFORCEMENT RETIRED" header comments.
3. **Orchestrator preflight check**: update `biggz-orchestrator.md` Post-Delegation Human Checkpoint + `biggz-orchestrator-delegation.md` Ask contract to restore blocking language (`A checkpoint ask without immediately preceding ## Sub-agent Result markdown is INVALID and will be blocked` + `REMINDER: synthesis markdown is separate chat markdown emitted FIRST, adjacent, same turn, before tool call` 12×) and add explicit self-check step before invoking `ask_user_choice`/`question`. Optionally add harness-side `ShouldBlock` intercept if available (not required for V1).
4. **Regression tests fail-closed**: in `synthesis_gate_test.go` re-add `TestShouldBlock` (checkpoint missing→block, rich currentTurn→allow, history-only→block, expired 120s→block, general ask→allow, preflight→allow, session-recall same-turn→allow, body-token→allow), in `biggz-synthesis-gate.test.mjs` flip passthrough asserts to blocking asserts (checkpoint missing→`isError:true` not-called + `{block:true}`, same-turn race→pass, history-only→block, child bypass→allow, thin+advise→warn pass). Add transcript lint fixture replaying branch-worktree-cleanup 8-phase sequence (7 missing syntheses → 7 expected blocks). CI: `go test ./...`, `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs`, `go vet ./...`, `node --check` must be green.

Why this combination: JS blocking alone leaves OpenCode uncovered; orchestrator check alone is voluntary; CI alone is post-hoc. Together they satisfy gentle-pi parity (prompt + Pi gate + tests) without reintroducing body-text false positives (fixed by label-only `IsCheckpointAsk` since 2026-09-03) and with streaming race mitigated by `currentTurnMarkdown` buffer (already present). Re-harden as **fail-closed by default, advise-only when `BIGGZ_ADVISE=1`**.

### Risks
- **Streaming race re-block**: orchestrator emits synthesis markdown milliseconds before `question` tool call; if `message_end` not yet fired, `currentTurnMarkdown` empty → false block. Mitigated by accumulating via `recordText` on `message_end`/`message_update`/`assistant_message` + `currentTurnUpdateTime` 120s, but requires Pi event ordering verification across versions.
- **History vs currentTurn confusion**: old synthesis from previous turn satisfying `ctx.history` must NOT pass strict check (regression `turn_start` reset). If reset missed, false pass hides missing synthesis.
- **Body-token false positives**: if bilingual token detected in question body (e.g. `¿...para continuar con X?` with neutral labels) → false block. Mitigated by label-only `isCheckpointAsk` (envelope `label`/`value`/`id`/`name`/`title` only, not `question` field); revert must keep this narrow.
- **Translation breaking markers**: Spanish synthesis content with translated markers (`**Artefactos/Rutas:**`) fails `HasSynthesis` → blocks. Mitigated by language boundary: content localized, markers stay English via `sanitizePlain` whitelist; proposal must re-document.
- **Thin advise noise**: `countPaths<2 || len<50` heuristic may flag legitimate syntheses with 1 artifact as thin → concern spam. Keep thin as warn-only under `BIGGZ_ADVISE=1`, silent otherwise.
- **Preflight/Session-Recall bypass widening**: if preflight allowance ("no prior synthesis ever → allow") or same-turn `## Session Recall` exception applied too broadly, checkpoints after delegation could bypass. Scope narrow: only `currentTurnMarkdown` containing `## Session Recall` and only before first synthesis.
- **Dual-write/pending lag**: if `PersistPendingForCheckpoint` not called before checkpoint, compaction recovery loses question; gate blocks but fallback not persisted. Pair gate re-enable with pending dual-write enforcement.

### Ready for Proposal
Yes — proceed to `sdd-propose` for `fix-orchestrator-synthesis-gate`.

Proposal should scope: restore `ShouldBlock`/`BuildBlockedEnvelope` in Go + re-harden JS blocking (fail-closed default, `BIGGZ_ADVISE=1` thin-advise), update orchestrator docs to un-retire blocking language, add blocking + transcript regression tests, and note rollback via `git revert` of the 2-commit retire (no migration). No new dependencies; `openspec` store, so file writes to `openspec/changes/fix-orchestrator-synthesis-gate/`.

## Key Learnings
1. Synthesis gate enforcement retired 2026-09-04 to advise-only passthrough caused branch-worktree-cleanup to skip synthesis for 6 of 8 delegated phases without blocking.
2. HasSynthesis requires four verbatim English markers plus table-or-prose What Done, and IsCheckpointAsk scans only option labels not question body.
3. Strict same-turn currentTurnMarkdown buffer with 120s window and turn_start reset prevents history false positives and streaming race.
4. Thin synthesis advise uses Artifacts/Paths count less than 2 or length less than 50 and emits concern only when BIGGZ_ADVISE equals 1.
5. Re-hardening needs combined JS blocking, orchestrator preflight check, and CI transcript regression tests to cover Pi and OpenCode runtimes.
