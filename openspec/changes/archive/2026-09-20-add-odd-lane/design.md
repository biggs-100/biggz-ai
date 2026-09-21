# Design: add-odd-lane

## Technical Approach

ODD is docs + read-only observability: the protocol append, `odd` skill and contract test; a read-only `odd` array in `biggz sdd-status`; an optional orchestrator-declared `subroute` from `state.yaml` surfaced in status JSON and `sdd-continue`. The `orchestrator` delta is vocabulary-only.

## Architecture Decisions

| # | Choice (rejected alternative) | Rationale |
|---|---|---|
| D1 | `OddTaskProgress{Total, Completed}` → `{total, completed}` (rejected: full `TaskProgress`) | REQ-SS-ODD-001 names exactly both keys; `allComplete` is vacuously true for 0-checkbox docs and reads as a gate signal (forbidden); `status.go:69` stays untouched. Consumer `cli_sdd.go:1011` uses `Completed/Total`; no JSON ODD consumer exists. |
| D2 | Additive top-level `odd` in the v2 envelope; `schemaVersion` stays `2` (rejected: v3, array in `active`) | Envelope `cli_sdd.go:139-144` has no version field; strict check `status_v2.go:179` covers only `ChangeStatus` via `ProjectStatusV2` (test-only callers); `odd` is not in frozen `StatusV2Projection` (`status_v2.go:57`); `--contract` hard-rejects non-v2 (`cli_sdd.go:63,72`); no JS status consumer. |
| D3 | Optional top-level `subroute:` in `openspec/changes/{change}/state.yaml`, orchestrator-written; emitted only when `route == organic`; invalid/absent = undeclared (rejected: CLI flag, BigMem topic, `_meta.yaml`, new command) | `state.yaml` is already orchestrator-owned/dual-written (`pending.go:47-64`); status stays read-only and offline; zero new commands. |
| D4 | Subroute applies only to existing changes; change-less organic work gets no route and no synthetic dir (rejected: synthetic dir, repo-root file) | REQ-OR-003 says "on active changes"; REQ-ODD-004 bans `openspec/changes/` writes. Change-less subroute is unobservable by construction; the `odd` array is its visibility surface. |
| D5 | New `internal/sdd/odd.go`: `OddDocument`, `ScanOddDocuments`, `RenderOddDocuments` (rejected: extend `status.go`) | Isolates scan from a 1700+-line derivation file; CLI-only caller, never in next/blocker paths → cannot gate. Counting reuses `countTaskProgressText`/`taskCheckbox` (`edit_authority.go:31`): `- [ ]` pending, `- [x]`/`- [X]` done (`*`/`N.` markers also match); `lastTouched` = `ModTime().UTC().Format(time.RFC3339)`; skips missing/unreadable/non-regular/non-`.md` silently. |
| D6 | Spec-only (rejected: touch `deriveRoute`) | `deriveRoute` already returns `organic`/`sdd` (`status.go:1225-1233`); `sdd-route` uses unrelated `direct`/`ask-sdd` (`route.go:27-34`). Only D3 adds Go behavior under REQ-OR-003. |

## Data Flow

```
odd/tasks/*.md --ScanOddDocuments--> []OddDocument --> JSON `odd` + human section
state.yaml subroute --readChangeCtx--> ChangeStatus.Subroute --> status JSON + sdd-continue
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | Modify | Append ODD protocol (7 steps, `odd/tasks` contract, no SDD lifecycle). Thin orchestrator untouched → ≤120-line test (`orchestrator_test.go:132`) stays green. |
| `internal/assets/skills/odd/SKILL.md` | Create | ODD skill; frontmatter mirrors `rdd-defect-workflow/SKILL.md:1-8`; ships via `skills.go:26,61`. |
| `AGENTS.md` | Modify | Add `odd` skills-index row. |
| `internal/assets/odd_lane_contract_test.go` | Create | Embedded FS ships the skill, frontmatter parses, no `odd/plans` path/text. |
| `internal/sdd/odd.go` + `odd_test.go` | Create | Scanner, renderer, cases. |
| `internal/sdd/status.go` | Modify | `Subroute` field; read `state.yaml` in both derivation paths (~:517, ~:759); emit for organic only. |
| `internal/sdd/subroute_test.go` | Create | Declared/undeclared/invalid/SDD-suppressed. |
| `cmd/biggz/cli_sdd.go` | Modify | JSON `odd` (~:139); human append (~:154, ~:169, ~:228); continue subroute (~:1044-1053). |
| `cmd/biggz/odd_status_cli_test.go` | Create | JSON + human surfacing + isolation. |
| `cmd/biggz/sdd_continue_picker_test.go` | Modify | Continue route/subroute lines. |

No changes to gatekeeper/session_guard/workload_guard/edit_authority/verify behavior.

## Interfaces / Contracts

JSON: `{"active": [], "archived": [], "review_disabled": false, "odd": [{"path": "odd/tasks/x.md", "taskProgress": {"total": 5, "completed": 3}, "lastTouched": "2026-01-02T03:04:05Z"}]}` — `odd` always present, `[]` when none. `Subroute` emitted only when declared and `route == organic`. Human line: `<path> — <completed>/<total> tasks — <lastTouched>` (section omitted when empty). `sdd-continue` prints exactly `route: organic` and, when declared, `subroute: direct-inline`.

## Testing Strategy

| Slice | Files | RED assertions | Keep passing |
|---|---|---|---|
| 1 | `odd_lane_contract_test.go` | embedded skill readable; `name:`/`description:` parse; no `odd/plans` | `orchestrator_test.go` ≤120 + drift guards |
| 2 | `odd_test.go`, `odd_status_cli_test.go` | counts 5/3 and 2/0, `- [X]`, sorted; missing/unreadable/non-`.md`/dir skipped; UTC `lastTouched`; JSON `odd[0]` keys; human `3/5`; proposal-done change keeps `nextRecommended=spec`, empty `blockedReasons` | `status_test.go`, `status_v2_test.go`, `sdd_status_cli_test.go`, watch test |
| 3 | `subroute_test.go`, `sdd_continue_picker_test.go` | declared → organic+subroute; undeclared → omitted; invalid ignored; SDD suppresses; continue prints/omits subroute line | `route_test.go`, `status_v2_test.go` |

## Threat Matrix

| Boundary | Applicability | Response | RED tests |
|---|---|---|---|
| Documentation-like paths | Applicable — `odd/tasks/` listing rule (read-only, never executed) | Only regular `.md` files listed; dirs/non-`.md` skipped | `odd_test.go`: `.sh` and `x.md/` skipped, `.md` listed |
| Git repository selection | N/A — no git invocation | — | — |
| Commit state | N/A — read-only filesystem | — | — |
| Push state | N/A — no push automation | — | — |
| PR commands | N/A — no PR automation | — | — |

## Work-Unit Slices (auto-chain, 400-line budget)

1. ODD contract surfaces: delegation doc, skill, `AGENTS.md` row, contract test → rollback "skill + row, then delegation doc".
2. Read-only ODD array: `odd.go`, CLI JSON/human → rollback "drop the `sdd-status` array".
3. Declared subroute: `status.go`, continue lines → smallest, independent.

Each independently reviewable/revertible, chained to avoid `cli_sdd.go`/`status.go` conflicts. `odd/tasks/*.md` is durable — never reverted.

## Migration / Rollout

No migration. Malformed docs are skipped silently and can never block status or SDD transitions. Older consumers ignore unknown keys. Revert per slice.

## Open Questions

- [ ] None blocking. Recorded limitation: change-less organic subroute is unobservable by design (D4).
