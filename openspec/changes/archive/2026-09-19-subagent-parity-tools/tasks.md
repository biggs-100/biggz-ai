# Tasks: subagent-parity-tools

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~130–180 (runtime ≈35 + tests ≈100; 2 modified files) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | auto-chain |
| Chain strategy | pending (single PR) |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Narrow `LEGACY_TOOL_RE`; register `subagent_list_tasks` + `subagent_send_message` (8 tools); tests | PR 1 | `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs` | `toolsetFor` fake child: `FAKE_TICK_MS=30` steer echo; `fakePi` gate scan with own 8-name surface | Revert `biggz-subagent-runtime.js` hunks + test hunks; no persisted state |

## Phase 1: Gate reconciliation — `LEGACY_TOOL_RE` narrowing

- [x] 1.1 RED `internal/assets/pi/biggz-subagent-runtime.test.mjs`: `subagent_list_tasks`/`subagent_send_message` NOT legacy; `subagent_run`/`subagent_list_running` ARE (fails pre-narrowing)
- [x] 1.2 GREEN `internal/assets/pi/biggz-subagent-runtime.js:557`: `/^subagent_run$|^subagent_list_running$/`
- [x] 1.3 Test "Runtime's own list tool is not legacy": `fakePi({ tools: <own 8 names> })` + clean settings ⇒ gate registers, no refusal
- [x] 1.4 Test "Genuine legacy names still refused": scan `subagent_run` + `subagent_list_running` ⇒ refuse

## Phase 2: `subagent_list_tasks`

- [x] 2.1 RED: seeded `createTaskRegistry` + `createSubagentToolset({ registry })`; settled `t1`,`t2` then running `t3` ⇒ rows `t3`,`t2`,`t1`
- [x] 2.2 RED: empty ring ⇒ `"no subagent tasks this session"`, no `isError`
- [x] 2.3 RED: cap — registry `{ limit: 2 }` with 3 live tasks ⇒ at most 2 rows; ring-only read
- [x] 2.4 GREEN: add `listTasks` after `agentsTool` (:951-961) — `registry.all().slice(-registry.limit).reverse()` via `formatTaskResult`; return 8 tools (:962)

## Phase 3: `subagent_send_message`

- [x] 3.1 RED: `toolsetFor({ cap: 1 })` + `FAKE_TICK_MS=30`; background run ⇒ steer reaches child (`fake_command`/`steer` echo) and `steered <id> · <state>` returns
- [x] 3.2 RED: unknown/expired id ⇒ `unknown or expired task "<id>"`, no throw
- [x] 3.3 RED: queued (cap-1 second run) and settled (`FAKE_SETTLED=1`) ⇒ `task "<id>" is not running (<state>); steer not delivered`
- [x] 3.4 GREEN: add `sendMessage` — `registry.get`, gate `running`/`spawning`, `task.steer(message)` false ⇒ bounded error (id ≤60, state ≤20)

## Phase 4: Integration & evidence

- [x] 4.1 Update surface assertions 6→8 at `biggz-subagent-runtime.test.mjs:632` and `:672` (existing six + `subagent_list_tasks`, `subagent_send_message`)
- [x] 4.2 `node --check internal/assets/pi/biggz-subagent-runtime.js`
- [x] 4.3 `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs internal/assets/pi/biggz-subagent-width.test.mjs internal/assets/pi/biggz-pi-extensions-factory.test.mjs` — all green
- [x] 4.4 `go vet ./... && go test ./internal/install/... ./internal/assets/biggz -count=1` — exit 0
