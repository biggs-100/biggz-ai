# Apply Progress: add-odd-lane

## Slice 1 — ODD contract surfaces (PR 1, stacked-to-main)

- [x] 1.1 RED `internal/assets/odd_lane_contract_test.go`: embedded FS ships `skills/odd/SKILL.md`, frontmatter parses, delegation doc lists 7 ordered steps, no `odd/plans`.
- [x] 1.2 GREEN: create `internal/assets/skills/odd/SKILL.md` (frontmatter like `rdd-defect-workflow`; sections `Objective`/`Tasks`/`Evidence`/`Next`; no BigMem mirror, no `biggz odd-*`).
- [x] 1.3 Append 7-step protocol (track-before-write; read-only no artifact) to `internal/assets/biggz/biggz-orchestrator-delegation.md`; `biggz-orchestrator.md` untouched.
- [x] 1.4 Add single `odd` row to `AGENTS.md`.
- [x] 1.5 Close: tick boxes; `apply-progress.md` evidence.
- [x] 1.6 Budget ≤400 (est. 210–260).

## Files Changed (Slice 1)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/assets/odd_lane_contract_test.go` | Created | 106 lines, 4 subtests: embedded `skills/odd/SKILL.md`, YAML frontmatter (`name: odd`, `Trigger:`, license, metadata author/version), ordered 7-step protocol + `odd/tasks/<slug>.md` in the ODD section, no `odd/plans`. |
| `internal/assets/skills/odd/SKILL.md` | Created | 46 lines: frontmatter mirrors `rdd-defect-workflow`; hard rules + `Objective`/`Tasks`/`Evidence`/`Next` document contract + 7-step execution; no BigMem mirror, no `biggz odd-*`. |
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | Modified | +28 lines appended: `## ODD Lane — Organic Direct Development (non-SDD)` with the 7 ordered steps, `odd/tasks/<slug>.md` contract, read-only guard (REQ-ODD-003), non-SDD guarantee (REQ-ODD-004). |
| `AGENTS.md` | Modified | +1 line: `odd` skills-index row (after `judgment-day`). |
| `openspec/changes/add-odd-lane/tasks.md` | Modified | Ticked 1.1–1.6. |
| `openspec/changes/add-odd-lane/apply-progress.md` | Created | This evidence file. |

Unchanged by this slice: `internal/assets/biggz/biggz-orchestrator.md` (≤120-line guard stays green), all `internal/sdd/**` gates, `openspec/specs/**`, `openspec/config.yaml`, and no `odd/` directory was created in the repo.

## Work Unit Evidence (Slice 1)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/assets/ -run TestOddLaneContract -count=1` — **RED before implementation**: 4/4 subtests FAIL (`skills/odd/SKILL.md` file does not exist; ODD protocol section heading missing). **GREEN after**: `--- PASS: TestOddLaneContract (0.00s)` with all 4 subtests PASS, `ok github.com/biggs-100/biggz-ai/internal/assets 0.446s`, exit 0. |
| Runtime harness command/scenario and exact result | `go test ./internal/assets/biggz/ -run TestOrchestrator -count=1` — exit 0, `ok ... 0.804s` (thin orchestrator ≤120-line guard, untouched). Full suite: `go test ./internal/assets/... -count=1` — exit 0, both packages `ok`; `go test ./internal/assets/biggz/... -count=1` — exit 0, `ok ... 0.732s`. `go vet ./...` — exit 0, no output. `node scripts/check-skill-lint.mjs` — `OK internal/assets/skills/odd/SKILL.md (391 tokens)`, exit 0. |
| Rollback boundary | Revert the `odd` row in `AGENTS.md` + delete `internal/assets/skills/odd/SKILL.md` first (surfaces disable independently), then revert the delegation-doc append + delete `internal/assets/odd_lane_contract_test.go`. No `odd/tasks/*.md` artifacts were created or touched, so nothing durable needs reverting. |

## Diff Summary (authored, Slice 1)

```
AGENTS.md                                                    |  1 +
internal/assets/biggz/biggz-orchestrator-delegation.md       | 28 ++++++++++++++++++++++
2 files changed, 29 insertions(+)
untracked new: internal/assets/skills/odd/SKILL.md (46), internal/assets/odd_lane_contract_test.go (106)
```

181 authored additions, 0 deletions — within the 400-line review budget (est. 210–260). No commit was created; changes are staged only in the working tree.

## Slice 2 — Read-only ODD array (PR 2, stacked-to-main)

- [x] 2.1 RED `internal/sdd/odd_test.go` (122 lines): counts 5/3 and 2/0, `- [x]`/`- [X]` complete, deterministic path sort, RFC3339 UTC `lastTouched`, skips non-`.md`/directory-`*.md`/missing-dir/non-directory `odd/tasks`, zero-checkbox prose stays 0/0, never creates `odd/`.
- [x] 2.2 GREEN `internal/sdd/odd.go` (81 lines): `OddDocument`, `OddTaskProgress`, `ScanOddDocuments`, `RenderOddDocuments`; reuses `countTaskProgressText`/`taskCheckbox`; skip-only, never fatal, never writes.
- [x] 2.3 RED `cmd/biggz/odd_status_cli_test.go` (201 lines): JSON exact `odd[0]` keys, `odd: []` always present, human line format, malformed-doc isolation, change-less organic isolation, no synthetic dirs.
- [x] 2.4 GREEN `cmd/biggz/cli_sdd.go` (+21/−4): additive top-level `odd` in the JSON envelope (schema stays v2) and human section via `appendOddSection` on the three render paths (main, `renderStatusOnce`, watch loop). Never referenced by routing/gates/`active`/`nextRecommended`/`blockedReasons`.
- [x] 2.5 D4: change-less organic workspace keeps `active: []`, lists the temp doc, and creates no synthetic `openspec/changes/` entry; malformed + proposal-done keeps `nextRecommended=spec` with empty `blockedReasons`.
- [x] 2.6 Close: ticked Slice 2 boxes; this evidence.
- [x] 2.7 Budget: 429 authored additions+deletions (425 add / 4 del) — above the 400 single-PR line, inside the 380–430 estimate. Delivery recommendation: stacked `2a` (`odd.go` + `odd_test.go`, 203) → `2b` (`cli_sdd.go` + `odd_status_cli_test.go`, 226); files are cleanly separable, so the orchestrator may commit them as two work units, or accept `size:exception`.

## Files Changed (Slice 2)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/odd.go` | Created | 81 lines: `OddTaskProgress{total,completed}` (no `allComplete` gate signal), `OddDocument{path,taskProgress,lastTouched}`, `ScanOddDocuments` (regular `.md` only, sorted, non-nil empty result, UTC RFC3339 mtime), `RenderOddDocuments` (omitted when empty). |
| `internal/sdd/odd_test.go` | Created | 122 lines, 3 subtests + renderer test: counts/sort/UTC, skip rules (non-`.md`, directory named `*.md`, missing dir, file at `odd/tasks`), zero-checkbox 0/0, human line format. |
| `cmd/biggz/odd_status_cli_test.go` | Created | 201 lines, 2 tests: both surfaces with exact key set and line format; always-present `[]`, change-less organic isolation, malformed-doc isolation, no synthetic dirs, pure-SDD output unchanged. |
| `cmd/biggz/cli_sdd.go` | Modified | +21/−4: `Odd` field + scan in the JSON payload; `appendOddSection` helper appended on the main, `renderStatusOnce`, and watch-loop human paths. |
| `openspec/changes/add-odd-lane/tasks.md` | Modified | Ticked 2.1–2.7. |
| `openspec/changes/add-odd-lane/apply-progress.md` | Modified | This Slice 2 evidence (Slice 1 preserved above). |

Unchanged: `internal/sdd/status.go` (D1/D2/D5 avoided touching `TaskProgress` and the frozen projection), `openspec/specs/**`, `openspec/config.yaml`, `internal/assets/**`, `internal/review/**`, gates/dispatcher/attempt-ledger code, `AGENTS.md`. No commit, no push, no attempt-ledger action.

## Work Unit Evidence (Slice 2)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | RED before (final test files, implementation removed/restored to pre-change): `go test ./internal/sdd/ -run 'TestOdd' -count=1` → `undefined: OddTaskProgress`, `undefined: ScanOddDocuments`, `undefined: RenderOddDocuments`, `FAIL ... [build failed]`. `go test ./cmd/biggz/ -run 'TestOddStatus|TestSddStatus' -count=1` → `--- FAIL: TestOddStatusSurfaces ... odd key missing from envelope: {"active": null, "archived": null, "review_disabled": false}` and `--- FAIL: TestOddStatusEmptyAndIsolation ... odd key missing`; `TestSddStatus*` already passed. GREEN after: `ok github.com/biggs-100/biggz-ai/internal/sdd 0.132s`; `ok github.com/biggs-100/biggz-ai/cmd/biggz 4.001s`. TDD note: RED was first captured with the verbose draft tests, then the test files were compacted (same assertions) and RED/GREEN re-captured with the final files. |
| Runtime harness command/scenario and exact result | Real binary against a temp workspace: `go run ./cmd/biggz sdd-status --cwd <tmp> --json` → `"odd": [{"path": "odd/tasks/demo.md", "taskProgress": {"total": 3, "completed": 2}, "lastTouched": "2026-09-20T18:04:59Z"}, {"path": "odd/tasks/prose.md", "taskProgress": {"total": 0, "completed": 0}, ...}]` (`.sh` skipped, `active`/`archived` untouched); human run → `ODD tasks:` + `odd/tasks/demo.md — 2/3 tasks — 2026-09-20T18:04:59Z`. Post-run `find` shows no synthetic directories. PASS. |
| Rollback boundary | Delete `internal/sdd/odd.go`, `internal/sdd/odd_test.go`, `cmd/biggz/odd_status_cli_test.go` and revert the 4 `cmd/biggz/cli_sdd.go` hunks (JSON field, `appendOddSection`, `renderStatusOnce`, watch loop) — drops the `odd` array and restores pre-slice output. No durable data: the scanner never writes and no `odd/tasks/*.md` was created by this change. |

## Keep-Passing / Build / Vet (Slice 2)

| Check | Result |
|---|---|
| `go test ./internal/sdd/ -run 'TestStatus|TestRoute' -count=1` | `ok github.com/biggs-100/biggz-ai/internal/sdd 1.913s` |
| `go test ./cmd/biggz/ -run TestSddStatus -count=1` | `ok github.com/biggs-100/biggz-ai/cmd/biggz 1.424s` |
| `go build ./...` | exit 0, no output |
| `go vet ./internal/sdd ./cmd/biggz` | exit 0, no output |
| `gofmt -l` on the 4 touched files (go1.26.1) | exit 0, no files listed; no `gofmt -w` on pre-existing files; `internal/review/rdd_helpers.go` untouched |
| Full touched packages | `go test ./internal/sdd/ -count=1` and `go test ./cmd/biggz/ -count=1` fail ONLY on environment-dependent sub-agent ownership tests (`TestCheckCheckpointAsk`, `TestValidateCheckpointSubstance`, `TestSddAskCheckExitTable`, message `checkpoint asks may only be emitted by orchestrator, not sub-agent (ownership)`); all pass with `PI_SUBAGENT_CHILD=` cleared (`ok` both runs). Pre-existing environment limitation, not a regression. |
| Known flaky full suite | `go test ./...` not used per orchestration note (`internal/review` ~172s/180s budget under parallel load on this Windows box); focused packages used instead. |

## Diff Summary (authored, Slice 2)

```
cmd/biggz/cli_sdd.go               | 21 ++++++++++++++++++++++++----  (21 additions, 4 deletions)
new: internal/sdd/odd.go           | 81 lines
new: internal/sdd/odd_test.go      | 122 lines
new: cmd/biggz/odd_status_cli_test.go | 201 lines
```

425 authored additions + 4 deletions = 429 authored lines. Budget outcome: exceeds the 400-line single-PR threshold by 29; inside the slice estimate (380–430). Recommended delivery split: `2a` = `internal/sdd/odd.go` + `internal/sdd/odd_test.go` (203), `2b` = `cmd/biggz/cli_sdd.go` + `cmd/biggz/odd_status_cli_test.go` (226). No commit was created; changes remain in the working tree for the orchestrator-owned delivery.

## Slice 3 — Declared subroute (PR 3, stacked-to-main)

- [x] 3.1 RED `internal/sdd/subroute_test.go` (185 lines): declared `direct-inline`/`delegated-direct` → organic+subroute; undeclared omitted; invalid ignored (incl. mapping/malformed YAML); SDD suppresses; change-less → no route/subroute, no error, no synthetic dir.
- [x] 3.2 GREEN `internal/sdd/status.go` (+34/−1): `Subroute` field (`json:"subroute,omitempty"`); `validOrganicSubroutes` vocabulary + `declaredOrganicSubroute(route, changeDir)` reading the optional top-level `subroute` key from `state.yaml`; assigned in both derivation paths only when `route == organic`; invalid/absent/unreadable/malformed → "" (never an error, never a guess).
- [x] 3.3 `cmd/biggz/cli_sdd.go` (+5/−2): continue block prints `route: organic` + `subroute: <declared>` (omitted when undeclared) and `route: sdd`; `cmd/biggz/sdd_continue_picker_test.go` (+58): `TestSddContinue_RouteContext` (declared both values, undeclared, invalid ignored).
- [x] 3.4 Close: ticked Slice 3 boxes; this evidence.
- [x] 3.5 Budget: 285 authored lines (282 additions + 3 deletions) — 20 above the 185–265 slice estimate, 115 below the 400-line hard stop/review budget. No file reformatting; `gofmt -l` clean on all 4 touched files.

## Files Changed (Slice 3)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/subroute_test.go` | Created | 185 lines, 5 tests: declared organic (both values, field + JSON wire), undeclared omitted (no/without state key), invalid ignored (sdd/direct/wrong case/empty/mapping/malformed), SDD route suppresses, change-less workspace (0/0, no synthetic entries). |
| `internal/sdd/status.go` | Modified | +34/−1: `Subroute string `json:"subroute,omitempty"``; `validOrganicSubroutes = []string{"direct-inline", "delegated-direct"}` + `declaredOrganicSubroute` (exact-match, `slices.Contains`); `cs.Subroute = declaredOrganicSubroute(cs.Route, changeDir)` after `deriveRoute` in both derivation paths (~:520, ~:763); struct doc list updated. |
| `cmd/biggz/cli_sdd.go` | Modified | +5/−2: continue block emits `route: organic` (+`subroute: <value>` when declared) and `route: sdd`; parenthetical removed, `No SDD next` line unchanged. |
| `cmd/biggz/sdd_continue_picker_test.go` | Modified | +58: `TestSddContinue_RouteContext` table (declared direct-inline/delegated-direct, undeclared, invalid ignored) asserting exact line presence/absence; HOME/USERPROFILE isolated per subtest. |
| `openspec/changes/add-odd-lane/tasks.md` | Modified | Ticked 3.1–3.5. |
| `openspec/changes/add-odd-lane/apply-progress.md` | Modified | This Slice 3 evidence (Slices 1–2 preserved above). |

Unchanged: `internal/sdd/odd.go`, `openspec/specs/**`, `openspec/config.yaml`, `openspec/changes/add-odd-lane/state.yaml`, `internal/assets/**`, `internal/review/**`, gates/attempt-ledger code, `AGENTS.md`. No commit, no push, no `biggz sdd-attempt` action.

## Work Unit Evidence (Slice 3)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | RED before (tests only, implementation absent): `go test ./internal/sdd/ -run 'TestSubroute|TestRoute' -count=1` → `internal\sdd\subroute_test.go:90:10: cs.Subroute undefined (type ChangeStatus has no field or method Subroute)` (8 compile errors), `FAIL github.com/biggs-100/biggz-ai/internal/sdd [build failed]`. `go test ./cmd/biggz/ -run 'TestSddContinue|TestSddStatus' -count=1` → `--- FAIL: TestSddContinue_RouteContext` on all 4 subtests: stdout `Route: organic (direct inline or delegated direct)` (no `route: organic`, line 207) and no `subroute: direct-inline` (line 216); `FAIL github.com/biggs-100/biggz-ai/cmd/biggz 3.822s`. GREEN after: `ok github.com/biggs-100/biggz-ai/internal/sdd 0.488s` (verbose: `TestSubrouteOrganicDeclared`, `TestSubrouteUndeclaredOmitted`, `TestSubrouteInvalidIgnored`, `TestSubrouteSDDRouteSuppresses`, `TestSubrouteChangeLessWorkspace` all PASS); `ok github.com/biggs-100/biggz-ai/cmd/biggz 4.053s` (verbose: `TestSddContinue_RouteContext` + 4 subtests PASS). |
| Runtime harness command/scenario and exact result | Real binary (`go build -o /tmp/oddbin/biggz.exe ./cmd/biggz`) against temp workspaces with `openspec/changes/route-change/{proposal.md,state.yaml}`; `biggz sdd-continue route-change` (cd workspace) emitted exactly: declared direct-inline → `route: organic` + `subroute: direct-inline`; declared delegated-direct → `route: organic` + `subroute: delegated-direct`; undeclared → `route: organic` only; invalid (`subroute: direct`) → `route: organic` only, exit 0, no error; SDD (`tasks.md` + declared) → `route: sdd`, no subroute. `biggz sdd-status --json --cwd <ws>` mirrored it: `{'route': 'organic', 'subroute': 'direct-inline', 'subrouteKeyPresent': True}` / `'delegated-direct'` True / undeclared `subrouteKeyPresent: False` / invalid False / sdd `False`. Change-less workspace: `sdd-continue` → `No active changes.` exit 0; JSON `{'active': [], 'archived': [], 'odd': [], 'hasRouteKey': False, 'hasSubrouteKey': False}`; `find openspec` after both runs shows only `openspec/changes` (no synthetic entries). All exit 0. |
| Rollback boundary | Delete `internal/sdd/subroute_test.go`; drop `Subroute`, `validOrganicSubroutes`, `declaredOrganicSubroute`, and the two `cs.Subroute = ...` lines in `status.go`; revert the continue block in `cli_sdd.go` to the previous `Route:` lines; remove `TestSddContinue_RouteContext`. `route` derivation stays intact — Slice 3 removes only the declared-subroute surface. No durable data: nothing is written by status or continue. |

## Keep-Passing / Build / Vet (Slice 3)

| Check | Result |
|---|---|
| `go test ./internal/sdd/ -run 'TestOdd|TestStatus|TestRoute' -count=1` | `ok github.com/biggs-100/biggz-ai/internal/sdd 1.532s` |
| `go test ./cmd/biggz/ -run 'TestOddStatus|TestSddStatus' -count=1` | `ok github.com/biggs-100/biggz-ai/cmd/biggz 3.339s` |
| `PI_SUBAGENT_CHILD= go test ./internal/sdd/ -count=1` (full pkg) | `ok github.com/biggs-100/biggz-ai/internal/sdd 28.529s` |
| `PI_SUBAGENT_CHILD= go test ./cmd/biggz/ -count=1` (full pkg) | `ok github.com/biggs-100/biggz-ai/cmd/biggz 74.252s` |
| `go build ./...` | exit 0, no output |
| `go vet ./internal/sdd ./cmd/biggz` | exit 0, no output |
| `gofmt -l` on the 4 touched files (go1.26.1) | exit 0, no files listed; no `gofmt -w` on pre-existing files; `internal/review/rdd_helpers.go` untouched |
| Known flaky full suite | `go test ./...` not used per orchestration note (`internal/review` ~172s/180s budget on this Windows box). |

## Diff Summary (authored, Slice 3)

```
cmd/biggz/cli_sdd.go                  |  5 +++++--      (5 additions, 2 deletions)
cmd/biggz/sdd_continue_picker_test.go | 58 +++++++++++++++++++++++++++++++++++
internal/sdd/status.go                | 34 +++++++++++++++++++++++++++++++-    (34 additions, 1 deletion)
new: internal/sdd/subroute_test.go    | 185 lines
```

282 authored additions + 3 deletions = 285 authored lines. Budget outcome: 20 above the 185–265 slice estimate, well inside the 400-line review budget and hard stop. No commit was created; changes remain in the working tree for the orchestrator-owned delivery (stacked-to-main, PR 3).
