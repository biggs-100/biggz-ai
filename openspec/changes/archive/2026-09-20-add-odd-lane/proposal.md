# Proposal: add-odd-lane

## Intent

ODD is the non-SDD lane: direct work with durable evidence. The three-route ladder exists (`internal/assets/biggz/biggz-orchestrator-delegation.md:5-15`) but lacks a protocol, document convention, and visibility — direct work leaves no recoverable trace. Port gentle-ai's ODD lifecycle (`internal/components/agentguidance/routing.go:41-51`, `docs/usage.md:9-21,45-89`, `docs/trigger-rules.md:28-30`) without registering ODD as an SDD phase.

## Scope

### In Scope
- ODD protocol (Authorize → Explore → Resolve uncertainty → Classify → Track before first write → Implement → Close) added to `internal/assets/biggz/biggz-orchestrator-delegation.md`.
- New skill `internal/assets/skills/odd/SKILL.md` (artifact contract); root `AGENTS.md` gains the `odd` row.
- `odd/tasks/<slug>.md` at repo root (sibling of `openspec/`): one recoverable document, never archived/deleted, no BigMem mirror — single source of truth.
- `biggz sdd-status` read-only ODD array (`path`, `taskProgress`, `lastTouched`) in JSON and human output; outside `active`/`nextRecommended`/`blockedReasons`; no gate.
- Route vocabulary `organic|sdd` plus optional orchestrator-declared `subroute` (`direct-inline|delegated-direct`; CLI cannot infer it — `internal/sdd/status.go:1224`).
- Fix spec drift: `openspec/specs/orchestrator/spec.md:668-690` requires `direct-inline`/`delegated-direct`; `deriveRoute` returns `organic` (`internal/sdd/status.go:1225-1233`).

### Out of Scope (Non-Goals)
- No `biggz odd-new` CLI; orchestrator writes by convention.
- `sdd-ff` stays SDD with five gates intact (`openspec/specs/orchestrator/spec.md:771-810`).
- `internal/assets/biggz/orchestrator.md` stays ≤120 lines (`internal/assets/biggz/orchestrator_test.go:132-153`).
- Deferred status-authority defects (separate change; no requirements): capped archive set, no `YYYY-MM-DD-` strip (`internal/sdd/status.go:306,311`, twins `:429,434`); BigMem dedup (`engram_status.go:601-620`); `engram` skips filesystem (`status.go:330-341`); `sdd-new` scaffold passes the proposal gate; `sdd-new` creates `specs/none/`; checkpoint pending lacks CLI persistence; workflow cites missing `pending_question.languageHint`.

## Capabilities

### New Capabilities
- `odd-lane`: ODD routing protocol, `odd/tasks/` convention, read-only surfacing contract.

### Modified Capabilities
- `orchestrator`: REQ-OR-003 route vocabulary `organic|sdd` + optional `subroute`.
- `sdd-status`: read-only ODD-documents array, isolated from routing, gates, blockers.

## Approach

Port the protocol into the lazy routing doc, the artifact contract into the new skill; never port upstream code blindly (gentle-ai-relative anchors: `internal/components/agentguidance/inject.go:20-23`, `internal/components/sdd/odd_integration_test.go:32-61` assert installed output, including retiring `odd/plans/`). `odd/` stays at repo root, keeping it the non-SDD lane. Skills deploy by walking the embedded tree (`internal/install/steps/skills.go:26,61` — `fs.WalkDir(fsys, "skills", …)`); the `AGENTS.md` row is manual.

## Affected Areas

- `biggz-orchestrator-delegation.md`, root `AGENTS.md` (modified); `skills/odd/SKILL.md` (new); `internal/sdd/status.go`, `cmd/biggz/cli_sdd.go` (modified); specs `orchestrator`, `sdd-status` (delta).

## Risks

- High — route drift (`spec.md:668-690` vs `status.go:1225-1233`): fix spec; keep `subroute` optional so `organic` remains valid.
- Med — fast-lane clause (`spec.md:771-810`) must be amended without relaxing gates; delta preserves "gates MUST NOT be relaxed".
- Med — non-SDD store invisible to every existing gate by construction; array is observability-only and adds no gate.

## Rollback Plan

Revert per PR slice (auto-chain): drop the `sdd-status` array, then skill + `AGENTS.md` row, then the delegation-doc section. `odd/tasks/*.md` is durable evidence — never reverted or deleted. Spec deltas revert by removing the change folder before archive.

## Dependencies

- gentle-ai (`C:/Users/USER/Desktop/herramientas/gentle-ai`) as read-only design source.

## Success Criteria

- [ ] `biggz sdd-status --json` and human output list `odd/tasks/*.md` with `path`, `taskProgress`, `lastTouched`; never in `active`/`nextRecommended`/`blockedReasons`.
- [ ] `internal/assets/skills/odd/SKILL.md` exists, ships via the installer, and the `odd` row is in `AGENTS.md`.
- [ ] `biggz-orchestrator.md` ≤120-line test passes (`internal/assets/biggz/orchestrator_test.go:132-153`).
- [ ] Spec no longer requires `direct-inline`/`delegated-direct` as `route`; `subroute` optional.
- [ ] A biggz-ai-owned contract test asserts the embedded `skills` tree ships `skills/odd/SKILL.md` and that the ODD convention introduces no `odd/plans/` path. (Upstream `odd_integration_test.go:32-61` is a gentle-ai artifact and MUST NOT be cited as a biggz-ai criterion.)
