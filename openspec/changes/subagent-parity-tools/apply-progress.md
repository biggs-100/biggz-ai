# Apply Progress: subagent-parity-tools

**Mode**: Standard (strict_tdd: false in `openspec/config.yaml`)
**Work unit**: `parity-tools-impl` — attempt 1/2 — single PR slice (delivery_strategy `auto-chain`, forecast Low)
**Status**: 16/16 tasks complete — ready for sdd-verify

## Completed Tasks

- [x] 1.1 RED test: exact-j0k3r classification (`LEGACY_TOOL_RE` true for `subagent_run` / `subagent_list_running`, false for `subagent_list_tasks` / `subagent_send_message`)
- [x] 1.2 GREEN: `LEGACY_TOOL_RE = /^subagent_run$|^subagent_list_running$/` (runtime comment names the delta scenario)
- [x] 1.3 Test: scan returns the runtime's own 8-name surface ⇒ gate registers and all 8 tools register (no refusal)
- [x] 1.4 Test: genuine j0k3r names (`subagent_run` + `subagent_list_running`) ⇒ refusal kept (existing scan test retitled to name the scenario)
- [x] 2.1 Test: seeded ring settled `t1`, `t2` then running `t3` ⇒ rows `t3`, `t2`, `t1`
- [x] 2.2 Test: empty ring ⇒ `no subagent tasks this session`, non-error
- [x] 2.3 Test: `{ limit: 2 }` with 3 live tasks ⇒ at most 2 rows, newest first, ring-only read
- [x] 2.4 `listTasks` added after `agentsTool`: `registry.all().slice(-registry.limit).reverse()` via `formatTaskResult`; toolset returns 8 tools
- [x] 3.1 Test: background steer reaches the fake child (`fake_command`/`steer` echo) + `steered <id> · running` returns
- [x] 3.2 Test: unknown/expired id ⇒ `unknown or expired task "sub-gone"`, no throw
- [x] 3.3 Test: queued ⇒ `task "<id>" is not running (queued); steer not delivered`; settled ⇒ `… (completed) …`
- [x] 3.4 GREEN: `sendMessage` — `registry.get`, `running`/`spawning` gate, `task.steer(message)` false ⇒ bounded error (id ≤60, state ≤20)
- [x] 4.1 Surface assertions 6→8 at both call sites plus `six`→`eight` test title
- [x] 4.2 `node --check` both changed JS files ⇒ exit 0
- [x] 4.3 Focused trio (`runtime` + `width` + `extensions-factory`) ⇒ 75 tests, 75 pass, fail 0, exit 0
- [x] 4.4 `go vet ./...` ⇒ exit 0; `go test ./internal/install/... ./internal/assets/biggz -count=1` ⇒ `ok` ×3

## Files Changed

| File | Action | What Was Done | Lines |
|------|--------|---------------|-------|
| `internal/assets/pi/biggz-subagent-runtime.js` | Modified | Narrowed `LEGACY_TOOL_RE` to exact j0k3r names; added `subagent_list_tasks` and `subagent_send_message` after `agentsTool`; tool array 6→8 | +27 / -2 |
| `internal/assets/pi/biggz-subagent-runtime.test.mjs` | Modified | Legacy classification + own-surface gate tests; `subagent_list_tasks` describe (ordering/empty/cap); `subagent_send_message` describe (running/unknown/queued/settled); surface assertions 6→8 | +105 / -4 |
| `openspec/changes/subagent-parity-tools/tasks.md` | Modified | All 16 tasks marked `[x]` | checkbox edits |
| `openspec/changes/subagent-parity-tools/apply-progress.md` | Created | This report | — |

Code changed lines: **138** (132 additions + 6 deletions) — within the 400-line review budget.

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs` → `tests 50 / suites 13 / pass 50 / fail 0`, exit 0 (re-run after the final test-file edit) |
| Full pi suite | `node --test --test-force-exit internal/assets/pi/*.test.mjs` → `tests 193 / pass 193 / fail 0`, exit 0 |
| Task 4.3 trio | `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs internal/assets/pi/biggz-subagent-width.test.mjs internal/assets/pi/biggz-pi-extensions-factory.test.mjs` → `tests 75 / pass 75 / fail 0`, exit 0 |
| Syntax | `node --check` on both changed JS files ⇒ exit 0 |
| Go gates (4.4) | `go vet ./...` ⇒ exit 0; `go test ./internal/install/... ./internal/assets/biggz -count=1` ⇒ `ok internal/install 13.5s`, `ok internal/install/steps 6.2s`, `ok internal/assets/biggz 1.1s` |
| Runtime harness command/scenario and exact result | Real fake RPC child through the toolset (`FAKE_TICK_MS=30` + `FAKE_SETTLED=1`, cap 1): `subagent_send_message` → `steered sub-… · running`; child echo `{type:"fake_command",command:{type:"steer",message:"change of plan"}}` observed; queued/settled ids → bounded state errors. **Real pi, isolated iso** (`PI_CODING_AGENT_DIR`, agents/extensions/settings/auth.json copied, branch runtime hash `DA73556C…` overwritten): `pi -p --tools subagent_list_tasks "Call the subagent_list_tasks tool now and print its result."` → `no subagent tasks this session`, exit 0 (registration E2E proven); PONG smoke `pi -p --tools subagent …` → `subagent sub-… · completed · 8.6s · settled` + `PONG`, exit 0 (no regression). Captures: `%TEMP%\opencode\parity-tools-apply-evidence\probe-a-list-tasks.txt`, `probe-b-pong.txt` |
| Rollback boundary | Revert `biggz-subagent-runtime.js` (regex line + two tool definitions + return array) and the test-file hunks — no persisted state, no schema, no migration. Reinstall redeploys the previous asset bytes. |

### RED evidence (standard mode: tests authored with the change)

- Pre-narrowing regex check: `/^subagent_run$|^subagent_list_/` → `subagent_list_tasks=true subagent_send_message=false`; therefore the new classification assertion (`subagent_list_tasks` must be false) and the own-8-surface gate test FAIL on pre-change code.
- Pre-change `createSubagentToolset` exposed 6 tools, so `byName('subagent_list_tasks')` / `byName('subagent_send_message')` were `undefined` → every list/send test errors pre-change.

## Deviations from Design

None — implementation matches design (exact-name regex, ring slice newest-first, state gate + `task.steer`, 8 tools, bounded errors). Two test-harness choices differ from the tasks wording, for determinism only (runtime semantics untouched):

1. Queued/settled refusal cases let the fake children settle on their own (`FAKE_SETTLED=1`) instead of cancel-killing them: killing racing children leaked an `EPIPE` stream error after the test (`node:test` uncaughtException), so cancels were removed from those two paths. The original draft (cancel in `t.after`) passed assertions but failed the file-level hook.
2. Task 1.4's scenario maps onto the pre-existing refusal test (retitled to name `subagent_run` + `subagent_list_running`) plus the new classification test, avoiding a duplicate scan test.

## Issues Found

- **Test-harness only**: runtime `writeCommand` has no stdin `'error'` listener; a write racing a kill can surface `EPIPE` as an uncaughtException. Not triggered by production paths; the existing kill tests avoid the race. Out of this slice's scope (watchdog/kill semantics stay untouched) — follow-up candidate.
- Probe B logged `biggz-subagent-runtime: kill step taskkill /T failed …` — the pre-existing Windows kill-escalation path (`taskkill /T` then `/F`), untouched by this diff.

## Remaining Tasks

None — 16/16 complete.

## Workload / PR Boundary

- Mode: single PR (forecast Low; `auto-chain`, slice `parity-tools-impl`)
- Boundary: starts from master (6-tool runtime, blanket `subagent_list_*` legacy regex); ends at the 8-tool runtime + tests green + delta-spec consistency; no living-spec edits, no commits.
- Changed lines: 138 code (132+/6-) + this report + tasks.md checkboxes.
- Rollback: revert the two JS files; no persisted state.
