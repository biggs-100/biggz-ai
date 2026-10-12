# pi-native-mcp

## Objective

Make `biggz install --agent pi` provision Pi's **native** MCP support instead of the
third-party `pi-mcp-adapter` shim, so the bigmem MCP server is reached through the
built-in MCP client that Pi 1.1.0 ships. The adapter is not a compatibility layer for
the native client: while an extension registering `/mcp` is installed, Pi deliberately
does not read `mcp.json` and does not connect its own servers
(`docs/mcp.md` §"Replace the built-in MCP support"). Today biggz therefore *disables*
the native client it now should use, and the config it writes (`type: "local"`,
`imports`, `directTools`) is not native Pi vocabulary.

Outcome: one `~/.pi/agent/mcp.json` entry that Pi validates and connects on its own,
`pi remove npm:pi-mcp-adapter` reconciliation for existing installs, a doctor check that
proves the native path (and flags a still-shadowing adapter), and specs/docs that
describe the native contract.

## Evidence gathered (2026-09-26, Pi 1.1.0, isolated `PI_CODING_AGENT_DIR`)

| # | Fact | How it was observed |
|---|---|---|
| E1 | An installed extension registering `/mcp` (such as `pi-mcp-adapter`) replaces built-in MCP; Pi then does not read `mcp.json` | `docs/mcp.md` §"Replace the built-in MCP support" |
| E2 | `type: "local"` is rejected: `config error: server "bigmem" needs either "command" (stdio) or "url" (streamable HTTP)`. Valid types are `stdio`, `http`, `streamable-http`, or absent | `pi mcp list` against a temp `PI_CODING_AGENT_DIR` |
| E3 | Native shape is accepted and connected: `bigmem: connected, 20 tools (codemode, global)` | same run with `type` removed |
| E4 | Model-facing tool names are built by Pi: `mcp__<server>__<tool>` with non-alphanumerics replaced by `_`, plus a hash suffix only on collision or >64 chars (`createMcpToolName`, `dist/bundle/chunks/chunk-MXTIMZIDYW.js`) | Pi dist source |
| E5 | `exposure`/`toolExposure` (`direct`/`codemode`/`deferred`/`hidden`) are the native replacement for the adapter's `directTools`; `toolExposure` keys are the server-declared tool names | `docs/mcp.md` §"Control tool exposure", `dist/core/mcp-servers.js` validation |
| E6 | `pi remove <source>` (alias `pi uninstall`) is idempotent: on a non-installed package it prints "No matching package found" and exits 0 without touching `settings.json` | `pi remove npm:pi-mcp-adapter` in a temp agent dir |
| E7 | Native Pi reads MCP servers only from `~/.pi/agent/mcp.json` (and `.pi/mcp.json`); `settings.json` has no `mcpServers` key in the native schema (`packages`, `defaultTools`, `autoEnableCodemode` only) | `docs/settings.md`, `docs/mcp.md` |
| E8 | `biggz_mem_` appears 311 times outside `openspec/changes/archive` and `.git` (assets, docs, tests) | repo grep |

## Decisions

| # | Decision | Choice |
|---|---|---|
| D1 | Migration scope (human) | **Full native migration** — the adapter stops being installed, `imports`/`directTools` are removed, and the native shape is the only memory path |
| D2 | Tracking process (human) | **ODD** — this document plus work-unit commits; no SDD change |
| D3 | Branch base (human) | **New branch `feat/pi-native-mcp` from the HEAD of `feat/shell-visual-parity`**; the uncommitted shell-todo WIP and the untracked `openspec/changes/fix-archived-change-identity/` stay untouched |
| D4 | Tool-name prefix (human) | **Keep `--prefix=biggz`** explicitly. Native names become `mcp__bigmem__biggz_mem_save`; the 311 asset/doc references keep naming a real suffix of the callable name, and the anti-Engram guard in `bigmem-protocol.md` stays true. Passing the flag explicitly also defeats biggz-mcp's `init()` auto-detect, which otherwise picks `biggz` or `""` depending on whether an engram binary is on the machine. Exact-name coherence is follow-up F1, not this change |
| D5 | Servers that the adapter used to import (human) | **Generic migration** — convert every MCP server reachable today through the adapter (`settings.json` `mcpServers` plus `opencode.json` `mcp`/`mcpServers`) into `mcp.json`, idempotently and without overwriting an entry that already exists. Context7 is covered by this rule, not special-cased |
| D6 | `type` in the native entry (inferred from E2) | **Omit it.** Pi infers stdio from `command`; `"local"` (OpenCode vocabulary) is a hard config error, and `"stdio"` adds nothing |
| D7 | Adapter removal (inferred from E1/E6) | **Reconcile in the install pipeline** with an unconditional `pi remove npm:pi-mcp-adapter` (idempotent, exit 0), because leaving the extension installed keeps the native client switched off even with a perfect `mcp.json` |
| D8 | Exposure mapping (inferred from E5) | `exposure: "codemode"` on the `bigmem` server plus `toolExposure` marking the 11 `piDirectTools` names `direct`, reproducing today's 11-tool promotion without declaring all 20/22 tools to the model |
| D9 | How the migration resolves the OpenCode config path (inferred) | **Reuse `internal/agents/opencode` `Adapter.SettingsPath`** (the canonical `~/.config/opencode/opencode.json` resolver) instead of duplicating the path in the Pi package. The cost is that `internal/agents/pi` now imports a sibling adapter package whose `init()` registers the OpenCode adapter factory; verified harmless (the only importers of the Pi adapter are `cmd/biggz`, `internal/install` and `internal/tui`, and all of them already import the OpenCode package or never use the registry) |
| D10 | OpenCode `.jsonc` config variants (inferred) | **Not read in this change.** The migration reads the single `opencode.json` path, parsed with the JSONC-tolerant reader, and reports an unreadable source instead of failing. A user whose OpenCode config exists only as `opencode.jsonc` gets no migration — follow-up F5 |
| D11 | Exposing the adapter presence predicate (inferred) | **Export `install.PiMCPAdapterPresent`** so the external `install_test` package can cover the presence table, mirroring the existing exported `install.BinaryOnPath`. Accepted as a real API addition for testability rather than smuggling the check behind an unexported symbol with no coverage |
| D12 | Authority files that carry uncommitted WIP (inferred) | **Never stage them.** `internal/install/steps/pi_extensions.go` keeps its stale adapter comment until the human's shell-todo WIP is committed (follow-up F7), because staging the file would mix two unrelated units into one commit. Spec and doc files with no WIP are updated directly in unit 3.1 |
| D13 | Verification V1 finding: claimed versus real probe bounds (inferred) | **Make the code true instead of weakening the spec**, because a doctor that can hang is worse than a doctor that reports less: both probes get a real context deadline from `BIGGZ_MCP_TIMEOUT`, and the liveness rule becomes status-aware (`connected` plus `biggz_mem_*` names) rather than a substring that accepts `disabled` |
| D14 | Unit 2.3 implemented inline after the subagent runtime failed (inferred from the incident) | **Fix the parser inline rather than wait**, because the defect made every healthy install report WARNING and only a small, fully-tested parser change was needed; the independent re-verification stays a separate unit (4.3) for when the runtime recovers. Hard rule adopted: **test fixtures for external tool output must be verbatim captures**, never hand-written approximations — the fabricated `pi mcp list` fixture is exactly what let the regression through |

## Non-goals

- Rewriting the 311 `biggz_mem_*` references to exact native names (F1).
- Making `internal/install/steps/pi_extensions.go` (WIP file) part of this change beyond a
  comment correction, and never staging or reverting the shell-todo WIP.
- Rebuilding `biggz.exe`/`biggz-mcp.exe` release binaries (F3).
- Changing biggz-mcp's tool set, profiles, annotations or stdio protocol.

## Tasks

### Phase 1 — native provisioning

- [x] 1.1 Write the native `mcp.json` shape and prune adapter-era keys. *(910494b3)*
  Acceptance: `mergePiMCPFileBigMem` emits
  `mcpServers.bigmem {command: BiggzMCPPath(), args: ["--tools=agent","--prefix=biggz"],
  description: <one line>, exposure: "codemode", toolExposure: {<11 biggz_mem_* names>: "direct"}}`
  with no `type`, no `imports`, and no `directTools`; `mergePiSettingsBigMem` stops writing
  `mcpServers` into `settings.json`, keeps the `packages` reconcile, and prunes a
  biggz-owned `mcpServers.bigmem` plus `directTools` left there by earlier installs;
  `mcp.json` pruning also removes `imports` and `directTools` while preserving every other
  server and key; both writes stay atomic (`filemerge.WriteFileAtomic`) and idempotent
  (second run reports `changed=false`); `Pi DirectTools`-era tests are rewritten around the
  native shape.
  Evidence needed: `go test ./internal/agents/pi -run TestProvisionBigMemMCP -count=1 -v`
  and `go test ./internal/agents/pi -count=1`.
  Evidence observed (910494b3): RED 6 FAIL / 1 PASS on the adapter-era assertions,
  then `PI_CODING_AGENT_DIR= go test ./internal/agents/pi -run
  'TestProvisionBigMemMCP|TestMergePiMCPFileBigMem' -count=1` -> ok, full package -> ok,
  `go vet ./internal/agents/pi` exit 0 (re-observed by the orchestrator). `pi mcp list`
  against an isolated agent home: legacy `type:"local"` -> exit 1 config error; native
  shape -> exit 0 `bigmem: connected, 20 tools (codemode, global)`; leftover top-level
  `imports`/`directTools` are ignored, not rejected (prune is convergence, not acceptance).
  Incident: the first RED run was not hermetic - `PI_CODING_AGENT_DIR` is exported in this
  harness, so the pre-existing tests rewrote the live agent home
  `C:\Users\USER\.gentle-shell\agent` (`mcp.json`, `settings.json`). Handled as an
  incident: see the progress log.

- [x] 1.2 Migrate servers that the adapter used to import (D5). *(60fc9d96)*
  Acceptance: a single reconciling pass reads `opencode.json` (`mcp` and `mcpServers`
  entries) and `settings.json` `mcpServers`, converts each entry to Pi's native shape —
  `"type": "local"` with a `command` array splits into `command` + `args`, `environment`
  becomes `env`, `{env:NAME}` becomes `${NAME}`, remote entries become `url` — and adds it to
  `mcp.json` only when that server name is absent, preserving the whole existing file; the
  pass is idempotent, never overwrites a name that `mcp.json` already defines, and reports
  what it migrated; context7 is covered by the same rule.
  Evidence needed: `go test ./internal/agents/pi -run TestMigrateMCP -count=1 -v`.
  Evidence observed (60fc9d96): RED 3 FAIL / 1 vacuous PASS against the existing API
  (`mcpServers.context7/other/fresh missing`), then compile-level RED for the new report
  surface, then GREEN 8 tests / 15 scenarios including the conversion table, the
  absent-name guard, the settings `bigmem` skip and the byte-stable second run;
  `PI_CODING_AGENT_DIR= go test ./internal/agents/pi -count=1` ok, `go vet` exit 0,
  `go build ./internal/install ./cmd/biggz` ok (caller contract unchanged). Hermeticity
  proven by identical mtimes on the live harness home before and after the runs.

- [x] 1.3 Stop installing the adapter and reconcile its removal (D7). *(a3fa781a)*
  Acceptance: `InstallCommand` no longer contains `npm:pi-mcp-adapter@^2` and keeps the
  existing package order/dedupe behaviour for the remaining sources; the Pi install path
  runs `pi remove npm:pi-mcp-adapter` (idempotent, exit 0) before `pi install`, through the
  existing injectable command runner so tests stay hermetic; existing install assertions
  (`TestInstallCommand_ContainsAdapterBeforeSubagents`,
  `TestInstallCommand_CutoverDropsJ0k3r`) are retargeted rather than deleted.
  Evidence needed: `go test ./internal/agents/pi ./internal/install -count=1`.
  Evidence observed (a3fa781a): RED `InstallCommand must not install the retired
  pi-mcp-adapter extension`, then build-level RED for the absent presence predicate; GREEN
  `PI_CODING_AGENT_DIR= go test ./internal/agents/pi ./internal/install -count=1` -> both
  packages ok, `go vet` exit 0, live harness home mtimes identical before and after. The
  reconcile is a no-op that cannot fail the install, and it never runs under `--dry-run`.
  Decision D11: the presence predicate is exported as `install.PiMCPAdapterPresent` so the
  external `install_test` package can cover it, mirroring the existing `install.BinaryOnPath`.

### Phase 2 — doctor

- [x] 2.1 Replace `PiMCPAdapterCheck` with the native `pi-native-mcp` check and register it. *(bb2ae086)*
  Acceptance: `internal/doctor/pi_mcp_adapter.go` and its test are replaced by a check whose
  ID is `pi-native-mcp` that (a) skips when Pi is not installed; (b) warns when
  `pi-mcp-adapter` is still installed, naming the consequence (while an extension registers
  `/mcp`, Pi ignores `mcp.json` and its own servers) with the `pi remove npm:pi-mcp-adapter`
  remedy; (c) validates the native `bigmem` entry in `mcp.json` — present, `command`
  resolvable, no non-native `type`, `exposure`/`toolExposure` sane, no legacy
  `imports`/`directTools`; (d) verifies `settings.json` carries no biggz-owned
  `mcpServers.bigmem`/`directTools`; (e) keeps the bounded `tools/list` probe for
  `biggz_mem_*` behind `BIGGZ_MCP_TIMEOUT` plus the `pi mcp list` liveness signal; (f) stays
  read-only, panic-isolated, and keeps the injectable seams of the check it replaces; the
  `^2` version checks disappear. The check is registered in `internal/doctor/pi.go` with the
  `var _ Check` guard and in `cmd/biggz/cli_doctor_help.go` at the same position, and the
  existing test suite is retargeted rather than deleted (healthy passes, missing config warns
  with the `biggz install --agent pi` hint, adapter still installed warns with the removal
  remedy, non-native `type` warns, crash maps to `BIGGZ_MCP_TIMEOUT`, panic isolation,
  real-FS temp home, skip when Pi is missing).
  Evidence needed: `go test ./internal/doctor -run TestPiNativeMCP -count=1 -v`,
  `go vet ./internal/doctor ./cmd/biggz`, and `go run ./cmd/biggz doctor --json`.
  Evidence observed (bb2ae086): RED `undefined: NewPiNativeMCPCheckWithCustom /
  PiNativeMCPCheckID`; GREEN `PI_CODING_AGENT_DIR= go test ./internal/doctor -run
  TestPiNativeMCP -count=1` -> ok (14 tests / 20 subtests), full package -> ok; `go vet` and
  `go build ./cmd/biggz` exit 0; real `biggz doctor --json` against a live home reports
  `pi-native-mcp` status 0 with the native message and leaves both config files' mtimes
  untouched (read-only proven). Decision D12: the stale comment in the WIP file
  `internal/install/steps/pi_extensions.go` is deliberately left to F7, because staging that
  file would fold the human's uncommitted shell-todo WIP into this change.

### Phase 2b — close verification findings

- [x] 2.2 Make the native check's probes actually bounded and its liveness meaningful. *(dcfd5fee)*
  Verification V1 found that the rewritten doctor requirement claims guarantees the code does
  not provide: `exec.Command` runs with no context or deadline (so `BIGGZ_MCP_TIMEOUT` is only
  warning text and a hung `biggz-mcp --help` or `pi mcp list` blocks the doctor), the check
  never verifies `biggz_mem_*` through `tools/list` (it only runs `--help`), and the liveness
  test is a `strings.Contains(out, "bigmem")` that returns PASS for
  `bigmem: disabled` (`pi mcp list` exits 0 with `enabled: false`).
  Acceptance: both probes run through a context deadline derived from `BIGGZ_MCP_TIMEOUT`
  (still injectable for tests, with at least one test proving the deadline is applied); the
  liveness check requires the `bigmem: connected` status and the presence of the
  `biggz_mem_*` names in that output, and warns (instead of passing) for `disabled`, for an
  unknown status and for a connected server that lists no `biggz_mem_*` tool; the rewritten
  doctor spec states exactly those rules instead of claiming a bound that does not exist; and
  the stale `imports opencode` comment in `internal/install/install.go` (V1, line ~191) is
  corrected.
  Evidence needed: `PI_CODING_AGENT_DIR= go test ./internal/doctor -count=1 -v` (new
  disabled/unknown/connected-without-tools/deadline cases), `go vet ./internal/doctor
  ./cmd/biggz`, and a scoped re-verification of this unit.
  Evidence observed (dcfd5fee): RED showed `disabled`, an unknown status and a connected
  server without the promoted tools all reporting `Status=pass`, plus `both probes must carry
  a deadline, got 0 of 2`; GREEN `PI_CODING_AGENT_DIR= go test ./internal/doctor
  ./internal/agents/pi ./internal/install -count=1` ok for all three packages (re-observed by
  the orchestrator), `go vet` and `go build -o /dev/null ./cmd/biggz` clean, and the live
  harness home hashes unchanged (`mcp.json` md5 `8d86787d17486650e37c9a89b3aafc23`). The
  promoted names now come from `pi.PromotedBigMemTools()` (slices.Clone of `piDirectTools`)
  instead of a duplicated list, and the bounded-exec test kills a real hung child in 1s.

- [x] 2.3 Match the real `pi mcp list` rendering and scope the tool scan. *(35362580)*
  The scoped re-verification of 2.2 returned FAIL for that unit: `piMCPServerStatus` compared
  the raw status token, which Pi renders as `connected,` before the tool count, so the PASS
  branch was unreachable and every healthy install warned (the unit's tests hid it by
  fabricating `bigmem: connected (codemode, global)`). Two latent weaknesses rode along: only
  the first `bigmem:` line was read, and the promoted-tool scan covered the whole output.
  Acceptance: the status token is normalized before comparison; every `bigmem:` line is read
  and a non-connected one wins; the promoted-tool scan is scoped to the bigmem block; the
  test fixtures carry the format captured from Pi 1.1.0; the doctor requirement states the
  tolerated rendering, the multi-line rule and the block scope.
  Evidence observed (35362580): RED `reports bigmem as "connected,", not connected` on the
  real-format fixture, GREEN `PI_CODING_AGENT_DIR= go test ./internal/doctor -count=1` ok,
  `go vet`/`go build`/`gofmt` clean, and the E2E that the verifier used to fail now holds:
  a temp home with the native entry reports `pi-native-mcp` status 0, the same home with
  `enabled: false` reports status 1, and `pi mcp list` prints `bigmem: connected, 20 tools
  (codemode, global)`.

### Phase 2c — re-verification

- [x] 4.3 Scoped re-verification of unit 2.3. *(verified: PASS, no defect)*
  Acceptance: `gentle-ai-verify` reproduces the healthy real-format home as PASS, the
  `disabled` home as WARNING, the foreign-server tools case as WARNING, and confirms the
  deadline still bounds both probes.
  Why it was once blocked: the earlier session's subagent runtime stopped spawning `pi`
  (`could not start pi: spawn pi ENOENT`) after an unrelated concurrent gentle-shell install
  touched `C:\Users\USER\Desktop\herramientas\gentle-shell` and the global npm shims at
  16:58-16:59, so unit 2.3 was implemented inline by the orchestrator (decision D14). The
  re-verification ran in a later session once the runtime was healthy again.
  Evidence observed: **PASS**. Healthy real-format home: the verbatim `pi mcp list` capture
  (663 bytes, md5 `6704a68cf8dea6c99910e9e20bd9181e`, byte-checked with `cat -A`) reads
  `bigmem: connected, 20 tools (codemode, global)` plus the 11 `[direct]` marks, and on that
  temp home `biggz doctor --json` (2522 ms) returns
  `{"id":"pi-native-mcp","status":0,"severity":"INFO"}` with zero `pi-native-mcp` entries in
  `critical`/`warning` — so the trailing comma after `connected` that made 4.2 warn on a
  healthy install is genuinely closed (`pi_native_mcp.go:491` trims `,;` off the status
  token; the tool scan is block-scoped at `:504-539`). `disabled` home → WARNING, and
  foreign-server tools → WARNING naming 11 of 11 missing promoted tools. Both probes carry a
  deadline built per probe (`:404-417` via `:559-564`, `BIGGZ_MCP_TIMEOUT` ms at `:566-577`):
  at 1/60/150/500 ms the command still returned in 1.8-3.6 s with a probe warning instead of
  hanging, and 800/1200 ms were healthy. Live harness home unchanged (md5 `8d86787d…` /
  `a272ba9c…`), WIP and HEAD untouched. The first attempt at this unit died on a provider
  transport error after it had already produced the build/vet/unit-test/V-B/V-C evidence;
  both attempts ran under `gentle-ai-verify`, independently of the writer.

### Phase 3 — authority and docs

- [x] 3.1 Update the specs and docs that describe the adapter contract. *(03ca6d4c)*
  Acceptance: `openspec/specs/pi-integration/spec.md` describes the native provisioning
  contract (no adapter requirement, native shape, generic migration) with the adapter
  requirements replaced; `openspec/specs/agent-install/spec.md` no longer requires the
  adapter in `InstallCommand` and requires the removal reconcile instead;
  `openspec/specs/doctor/spec.md` describes `pi-native-mcp`;
  `openspec/specs/installer-pipeline/spec.md` keeps ordering but drops adapter wording;
  `docs/architecture.md` MCP/Pi lines match; `CHANGELOG.md` gains an entry naming the
  native migration and the adapter removal; the stale `pi-mcp-adapter@^2` comments in
  `internal/install/steps/pi_extensions.go` and `internal/assets/pi/biggz-shell/shell-bar.ts`
  are corrected without touching the shell-todo WIP.
  Evidence needed: readback of each edited section plus `grep -rn "pi-mcp-adapter"` showing
  only intentional mentions (CHANGELOG history, the removal reconcile, the doctor warning).
  Evidence observed (03ca6d4c): every edited section rewritten to the native contract, each
  remaining `pi-mcp-adapter` mention justified in the unit report (retired-adapter wording,
  removal path, doctor warning, negative test assertion, plus the historical changelog entry),
  `go build -o /dev/null ./cmd/biggz` and `go vet ./internal/doctor ./internal/agents/pi` exit
  0, the shell-bar asset suites pass (53 tests), and the live harness home mtimes stayed
  unchanged. No Go, test, JSON or behavior change.

### Phase 4 — verification

- [x] 4.1 Independent verification of the whole change. *(verified: PASS WITH RESERVATIONS)*
  Acceptance: targeted `go test` for `./internal/agents/pi`, `./internal/install`,
  `./internal/doctor`, `./cmd/biggz-mcp`; `go vet ./...`; `node --test` for the deployed
  Pi extensions; and one end-to-end run against a temp `PI_CODING_AGENT_DIR` where the
  provisioned `mcp.json` makes `pi mcp list` report `bigmem: connected` with the native
  shape and no `config error`. Any failure is reported as a blocker, never smoothed over.
  Evidence needed: verification report with exact commands and outputs, run by
  `gentle-ai-verify` (not by the implementing writer).
  Evidence observed: **PASS WITH RESERVATIONS** — `go test ./internal/agents/pi
  ./internal/install ./internal/doctor` and `./cmd/biggz-mcp` ok, `go vet ./...` and
  `go build ./cmd/biggz` clean, 30/30 `node --test` asset suites pass, E2E confirmed
  (`bigmem: connected` with the 11 `[direct]` marks for the native shape, the verbatim
  transport config error for `type: "local"`, leftover keys harmless), blast radius clean
  (no `biggz_mem_*` rename, zero commits touching the eight WIP paths or the untracked
  `openspec/changes/fix-archived-change-identity/`, live home hashes unchanged), F5/F6/F7
  confirmed. Three findings opened unit 2.2 (unbounded probes, no `biggz_mem_*`
  verification, `disabled` accepted as healthy); no blockers.

## Follow-ups (documented, not implemented here)

| # | Follow-up | Why deferred |
|---|---|---|
| F1 | Rename the tool-name surface to exact native names (`mcp__bigmem__mem_save`) and rewrite the 311 `biggz_mem_*` references across assets, docs and tests | Decision D4; it is a separate review-sized unit touching the deployed prompt surface |
| F2 | Decide whether `biggz install` should own a native Pi context7 entry once the generic migration (1.2) lands | The migration makes it appear; owning it as a managed entry is a separate product call |
| F3 | Rebuild `biggz.exe` after the change so the doctor reflects the new check | Release step, out of scope for a work-unit commit |
| F4 | Report a Pi `mcp.json` containing adapter-era keys that biggz does not own (`imports` outside bigmem) as a doctor warning | Needs the migration (1.2) proven first |
| F5 | Read `opencode.jsonc` (and any other OpenCode config variant) in the migration | Decision D10: the current pass reads only `opencode.json`, and the shape of the `.jsonc` variant is unverified |
| F6 | Surface the migration report's `Unreadable` sources and `Dropped` keys in the doctor output | `internal/install` discards `PiMCPProvisionReport.Migration`; only programmatic callers can see it today |
| F7 | Correct the stale `pi-mcp-adapter@^2` comment in `internal/install/steps/pi_extensions.go:346` | The file carries the human's uncommitted shell-todo WIP; unit 3.1 owns the comment correction |
| F8 | Investigate the `[bigmem] warning: both primary (…bigmem.db) and recovered (…bigmem_recovered…db) exist; merging by max(updated_at)` that `biggz doctor` prints on stderr | The check spawns `biggz-mcp`, whose startup merges the two BigMem databases, so a check documented as read-only writes outside the repo and the agent home. Observed during 4.3 (advisory A-1) with a temp agent home; not a defect in the `pi-native-mcp` check itself |

## Risks

| # | Risk | Mitigation |
|---|---|---|
| R1 | Existing user homes keep the adapter installed and therefore keep the native client off | D7 unconditional `pi remove` reconcile plus the doctor warning in 2.1 |
| R2 | Removing `imports` silently drops third-party servers the user got through the adapter | D5 generic migration in 1.2, non-destructive and idempotent, plus follow-up F4 |
| R3 | The native `type`/`exposure` schema drifts in a future Pi release | The doctor check validates against the native schema instead of trusting the written file |
| R4 | Test suites asserting the adapter strings break in ways that hide regressions | 1.3 retargets the assertions instead of deleting them; 4.1 verifies independently |
| R5 | Editing files that carry uncommitted shell-todo WIP mixes two units | D3: only single-line comment corrections, never `git add -A`, never a revert |

## Progress log

- 2026-09-26 — Feature opened. Exploration complete (evidence E1–E8), decisions D1–D8
  recorded, branch `feat/pi-native-mcp` created from `feat/shell-visual-parity` HEAD.
- 2026-09-26 — Unit 1.1 done and committed as `910494b3` (native shape, legacy prune on both
  files, dead adapter helpers deleted, provisioning tests made hermetic against an ambient
  `PI_CODING_AGENT_DIR`).
- 2026-09-26 — **Incident**: the unit's first RED run was not hermetic and rewrote the live
  harness agent home (`C:\Users\USER\.gentle-shell\agent`). Both files now carry the
  adapter-era `mcpServers.bigmem` with `type: "local"`, and `pi mcp list` reports
  `config error: ... server "bigmem" needs either "command" (stdio) or "url"`. The real
  `~/.pi/agent` was never touched. Repair is a human decision (see the conversation record)
  because that directory belongs to the running Gentle Shell harness, not to `~/.pi/agent`.
  Hardening that must not regress: the provisioning tests now clear the variable.
- 2026-09-26 — Unit 1.2 done and committed as `60fc9d96` (generic, non-destructive migration
  of `settings.json` `mcpServers` and the OpenCode config into native `mcp.json`, with the
  `PiMCPProvisionReport` carrier and decisions D9/D10). The same commit retargets
  `internal/install/bigmem_provision_test.go`'s `TestProvisionBigMemMCP_WritesBothFiles`,
  which unit 1.1 had left asserting the adapter-era shape in both files; the branch was
  green again (`go test ./internal/agents/pi`, `go test ./internal/install -run
  TestProvisionBigMemMCP`) and the live harness home mtime stayed unchanged, proving the
  hermeticity fix holds.
- 2026-09-26 — Unit 1.3 done and committed as `a3fa781a`: the adapter source is gone from
  `InstallCommand`, `install.Run` reconciles existing installs with
  `removeLegacyPiMCPAdapter` (best-effort, never fails, skipped on dry-run, present only when
  the npm directory or a `settings.json` `packages` entry says so), and the adapter
  expectations were retargeted instead of deleted. Decision D11 records the exported
  presence predicate; F7 records the one stale comment that unit 3.1 must fix.
- 2026-09-26 — Plan revision: doctor tasks 2.1 and 2.2 merged into one work unit, because a
  check replacement cannot compile without its registration in the same commit.
- 2026-09-26 — Unit 2.1 done and committed as `bb2ae086` (check `pi-native-mcp`, 14 tests /
  20 subtests retargeted, registration moved, `^2` version checks deleted, real
  `biggz doctor --json` E2E healthy and read-only). Decision D12 recorded.
- 2026-09-26 — Unit 3.1 done and committed as `03ca6d4c`: four openspec specs, the
  architecture doc, the changelog and one shell-bar comment now describe the native contract;
  no test asserted the changed text. Implementation phase complete (1.1, 1.2, 1.3, 2.1, 3.1);
  unit 4.1 runs the independent verification.
- 2026-09-26 — Unit 4.1 (independent verification by `gentle-ai-verify`) returned **PASS WITH
  RESERVATIONS**: every regression command, the three-case native E2E, all 30 asset suites
  and the blast-radius check passed; findings V1.1 (no exec deadline despite the
  `BIGGZ_MCP_TIMEOUT` claim), V1.2 (`--help` only, no `biggz_mem_*` verification) and V1.3
  (`disabled` accepted as healthy) opened unit 2.2. Also reported: exported
  `install.PiMCPAdapterPresent` has no spec line (D11 rationale stands), the doctor honours
  `PI_CODING_AGENT_DIR` with no spec mention (folded into 2.2's spec edit), and a stale
  `imports opencode` comment in `internal/install/install.go` (fixed in 2.2).
- 2026-09-26 — Unit 2.2 done and committed as `dcfd5fee`, closing V1.1-V1.3: the probes now
  carry a real context deadline from `BIGGZ_MCP_TIMEOUT`, liveness parses the `bigmem` status
  token and passes only for `connected` plus the promoted `biggz_mem_*` tools, the 11 names
  moved to `pi.PromotedBigMemTools()`, the doctor requirement was rewritten to the real
  guarantees, and the stale installer comment was corrected. Unit 4.2 re-verifies this unit.
- 2026-09-26 — Unit 4.2 (scoped re-verification) returned **FAIL for unit 2.2**: V1.1/V1.2/V1.3
  were genuinely closed, but the healthy E2E warned because the parser compared the raw
  `connected,` token, and the tests passed only against a fabricated fixture. The two latent
  weaknesses (first-line-only status, whole-output tool scan) were confirmed too.
- 2026-09-26 — **Runtime incident**: while dispatching unit 2.3, the subagent runner failed
  twice with `could not start pi: spawn pi ENOENT` and `orchestrator_list` reported session
  discovery not ready. Evidence points at an unrelated concurrent gentle-shell install
  (`.gentle-shell\agent\npm` mtime 15:07, `gentle-shell.cmd` created in the global npm dir at
  16:59:47, `Desktop\herramientas\gentle-shell` modified 16:58-16:59) that rewrote the npm
  shims my session's runner spawns. `pi` still resolves from a fresh shell (`pi.cmd`, `pi`,
  `pi.ps1` at `AppData\Roaming\npm`), and no repository file was affected.
- 2026-09-26 — Unit 2.3 done and committed as `35362580`, implemented inline by the
  orchestrator because the runtime could not spawn a writer (decision D14). Independent
  re-verification is unit 4.3 and remains open until the runtime recovers.
- 2026-09-26 — Live harness home repaired by human decision: `mcp.json` rewritten to the
  native `bigmem` shape (no `type`, `exposure: "codemode"`, 11 `toolExposure` names
  `direct`) and `settings.json` pruned of `mcpServers.bigmem`/`directTools`, with
  `*.bak-pre-native-repair-*` backups. Post-repair `pi mcp list` reports
  `bigmem: connected, 20 tools (codemode, global)` with the 11 `[direct]` marks and no
  config error — an end-to-end proof of the 1.1 shape against the real Pi binary.
- 2026-10-11 — Unit 4.3 (scoped re-verification of 2.3) returned **PASS with no defect**: the
  healthy real-format home passes with `status 0` / `INFO` instead of warning, `disabled` and
  foreign-server-tools warn as required, both probes stay bounded by `BIGGZ_MCP_TIMEOUT`, and
  the live harness home, the shell-todo WIP and HEAD were all unchanged. One advisory (A-1)
  became follow-up F8: `biggz doctor` makes `biggz-mcp` merge the primary and recovered BigMem
  databases. Bookkeeping note: the earlier entries above carry the `2026-09-26` label while
  `git log --format=%ci d937f82c..HEAD` dates all seven work-unit commits `2026-10-11`
  (15:02-17:19); recorded rather than rewritten, so the next reader checks git for dates.
