# Proposal: Synthesis Gate Hardening

## Intent

Guarantee human sees audited synthesis before deciding. Fix bypass where orchestrator called `ask_user_question` twice without markdown, hiding `oh-my-pi` report. Gate `b0d2fc1` exists (blocking + thin advise `<2 paths`/`<50 chars`) but was skipped when no markdown emitted.

## Scope

### In Scope
- `internal/assets/biggz/biggz-orchestrator.md` — mandatory copy-paste synthesis example, `INVALID and will be blocked` same-turn rule
- `internal/assets/pi/biggz-synthesis-gate.js` — blocking default, no orchestrator bypass, keep `PI_SUBAGENT_CHILD=1` only, thin advise via `BIGGZ_ADVISE=1`
- `internal/assets/pi/biggz-synthesis-gate.test.mjs` — cover missing-block, thin-advise, thin-silent, rich-pass
- Orchestrator integration test — asserts synthesis before question
- `docs/architecture.md` — document 3-layer defense

### Out of Scope
- SDD phase business logic
- `oh-my-pi` learnings content
- Full gentle-ai port (reference only, keep Go simplicity)

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `orchestrator`: mandatory synthesis markdown separate from tool param, emitted FIRST adjacent same-turn before `ask_user_question`/`question`
- `pi-integration`: blocking gate `isError:true` when markers missing; advise `concern` on thin only when `BIGGZ_ADVISE=1`; no model call, no auto-fix

## Approach

3 layers:
1. **Prompt** — copy-paste block + `REMINDER` on every ask
2. **Gate** — check `currentTurnMarkdown`→`ctx.history`→`lastAssistant` (120s); thin via `countPaths`/len; warn via `pi.notify`
3. **Tests/CI** — unit + integration; `go vet` + `node --check`

Preflight: `interactive | openspec | auto-chain | 800`; chain_strategy deferred to `sdd-tasks`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/assets/biggz/biggz-orchestrator.md` | Modified | Example + tighter checkpoint |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | Blocking, thin/advise, same-turn buffer |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Modified | 4 scenarios |
| `internal/assets/biggz/orchestrator.test.go` | New | Synthesis-before-question check |
| `docs/architecture.md` | Modified | Defense docs |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| False block on valid synthesis | Low | 4 markers + 120s + history fallback |
| Thin heuristic noisy | Med | Heuristic-only, off by default |
| Prompt drift bypass | Med | Integration test on template |

## Rollback Plan

`git revert <sha>` or checkout previous. No migration. `PI_SUBAGENT_CHILD=1` still bypasses.

## Dependencies

- Gate `b0d2fc1` + checkpoint; `go test` + `node --test`; gentle-ai ref (optional)

## Success Criteria

- [ ] ask without `## Sub-agent Result` → `isError:true` + `Please synthesize…`, original not called
- [ ] ask with full synthesis (≥2 paths, ≥50 chars, Risks, Next) → pass, no warn
- [ ] ask thin (`-`/1 path) + `BIGGZ_ADVISE=1` → warn `concern: synthesis is thin` but pass; without flag → silent pass
- [ ] CI green: `go vet`, `go test ./...`, `node --test biggz-synthesis-gate.test.mjs`
