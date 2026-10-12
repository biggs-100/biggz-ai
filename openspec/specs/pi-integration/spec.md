# Pi Integration Specification

## Purpose

The Pi Integration domain covers biggz-ai runtime behavior when hosted inside the Pi coding agent (harness, synthesis gate, and inline watchdog). This spec defines the advisor dual-mode watchdog that complements the native RAR gatekeeper with non-blocking concern injection for thin orchestrator synthesis.

## Requirements

### Requirement: Advisor Inline Watchdog Advise Mode

`biggz-synthesis-gate.js` MUST block checkpoint `ask` only when strict `currentTurnMarkdown` lacks 4 markers; history ignored for block. Block MUST be `isError:true` without handler + notify. Thin (count<2 OR len<50) with markers MUST NOT block; MUST emit `concern: synthesis is thin` when `BIGGZ_ADVISE=1` (off default); advise MAY fallback `currentTurn→history→lastAssistant(120s)` for concern only. `PI_SUBAGENT_CHILD=1` bypasses; no orchestrator bypass. `hasSynthesis` stays pass. General without checkpoint tokens MUST bypass.
(Previously: block without same-turn strictness.)

#### Scenario: Missing blocks

- GIVEN lacks 4 markers
- WHEN checkpoint ask
- THEN MUST block `isError:true`

#### Scenario: Thin advises not blocks

- GIVEN `BIGGZ_ADVISE=1`, 4 markers count 1 len 10
- WHEN checkpoint ask
- THEN MUST allow + `concern: synthesis is thin`

#### Scenario: General bypasses

- GIVEN "¿por dónde empezamos?" no checkpoint tokens
- WHEN gate evaluates
- THEN MUST allow without synthesis

### Requirement: Synthesis Gate Verification and CI

Go `internal/sdd/synthesis_gate_test.go` + `transcript_lint_test.go` MUST cover the canonical gate, and `internal/assets/biggz/orchestrator_test.go` MUST cover the orchestrator prompt markers; `biggz-pi-extensions-factory.test.mjs` MUST cover the deployed pi extension list. CI MUST run `go vet`, `go test` green. (Previously: `biggz-synthesis-gate.test.mjs` covered the Pi-side gate; retired with the wrapper in `pi-wrapper-removal` — Pi-side enforcement no longer deployed, Go canonical is the enforced gate.)

#### Scenario: Gate tests pass

- GIVEN the remaining gate suites (`go test ./internal/sdd`, `go test ./internal/assets/biggz`, `node --test biggz-pi-extensions-factory.test.mjs`)
- WHEN fixtures run
- THEN MUST pass and Go block asserts `isError:true`

### Requirement: Question Envelope Validation

`validateQuestionEnvelope` MUST reject when header>16, label>60, questions>4, or options∉[2,4]; reject MUST be `isError:true` naming limit, NOT call handler, emit fallback. Valid MUST allow.

| Field | Limit |
|-------|-------|
| header | ≤16 |
| label | ≤60 |
| questions | ≤4 |
| options | 2–4 |

Rejection is enforced by the deployed asset: `internal/assets/pi/ask-user-choice.ts` MUST invoke the checkpoint-ask check command before presenting any question and MUST refuse to present it when the check decides a block, surfacing the check's message to the agent. The invocation MUST follow the deployed-asset contract (argv array, no shell; bounded timeout; injectable test seam — `biggz-session-guard.js` precedent) and MUST NOT depend on the non-deployed `biggz-synthesis-gate.js`. A check pass MUST NOT skip the envelope's own limits (table above). A check that cannot decide (timeout, missing/broken binary, crash) MUST present with a visible notice that enforcement was skipped — no silent pass for the synthesis precondition, no bricked ask. A node test MUST prove the deployed asset invokes the check (argv + bounded timeout) and the refuse path — asserting the call, not merely the check's existence.
(Previously: validation defined only on the undeployed side, with no production call site and no asset invocation.)

#### Scenario: Header too long

- GIVEN header 17 chars
- WHEN validated
- THEN MUST reject naming header 16

#### Scenario: Options range

- GIVEN question with 1 option
- WHEN validated
- THEN MUST reject and emit fallback

#### Scenario: Valid passes

- GIVEN header 12, 3 questions each 3 options <60
- WHEN validated
- THEN MUST allow native ask

#### Scenario: Deployed asset invokes the check

- GIVEN `ask-user-choice.ts` execute path with a question
- WHEN presenting
- THEN it MUST invoke the check via argv array (no shell) under a bounded timeout, asserted via the test seam

#### Scenario: Decided block refuses presentation

- GIVEN check exits non-zero for option description `yes, go ahead`
- WHEN the asset executes
- THEN it MUST NOT present the question and MUST surface the check's message to the agent

#### Scenario: Pass does not skip envelope limits

- GIVEN check exits 0 but header is 17 chars
- WHEN the asset validates
- THEN header MUST still be rejected naming limit 16

#### Scenario: Indeterminate check degrades visibly

- GIVEN check binary missing or timeout
- WHEN the asset executes
- THEN the question MUST present with a visible warning that enforcement was skipped

### Requirement: REQ-PS4 — Marker Invariant Gate (b0d2fc1)
Gate `hasSynthesis`/`HasSynthesis`/`ShouldBlock`/`isCheckpointAsk` MUST validate by verbatim English markers only, ignoring localized content. 120s same-turn window, Session Recall and thin-advise rules stay language-agnostic.
#### Scenario: Spanish content with English markers passes
- GIVEN markdown Spanish prose but English markers present
- WHEN `hasSynthesis` checks
- THEN it MUST return true and allow `continuar`/`proceed` ask
#### Scenario: Missing marker blocks regardless of language
- GIVEN Spanish synthesis missing `**Artifacts/Paths:**`
- WHEN checkpoint ask within 120s
- THEN gate MUST block `isError:true`/`block:true`
#### Scenario: Thin and Session Recall language-agnostic
- GIVEN thin Spanish synthesis `BIGGZ_ADVISE=1` or `## Session Recall` present or general `¿por dónde empezamos?` without checkpoint tokens
- WHEN gate evaluates
- THEN thin MUST not block (may emit concern), recall MUST allow, general MUST bypass

### Requirement: PRETTY-V2-PI-01 — Powerline Footer with Segment Contract and NerdFont Fallback

The system MUST render powerline footer via `internal/assets/pi/biggz-footer.js` and `extension-api.js` with segments `branch|change|lineage|lens 1/4|budget 1/1` in order. Segments MUST use powerline separators (``/``/`›`) when NerdFont detected, else fallback to ASCII `▕` and `/`. The system MUST register separators via extension-api separator registry and respect `BIGGZ_PRETTY=0` (footer off) and `TERM=dumb` (ASCII only). Slice MUST be <400 lines, stacked-to-main, revertible in one commit.

#### Scenario: Footer segment order
- GIVEN branch `main`, change `ui-pi-pretty-v2`, lineage `2`, lens `1/4`, budget `1/1`
- WHEN footer renders
- THEN output MUST contain `main ▕ ui-pi-pretty-v2 ▕ lineage 2 ▕ lens 1/4 ▕ budget 1/1` order left-to-right

#### Scenario: NerdFont fallback
- GIVEN NerdFont missing
- WHEN footer renders
- THEN separators MUST be `▕` and `/` with zero NerdFont glyphs and no garble

#### Scenario: Kill-switch disables footer
- GIVEN `BIGGZ_PRETTY=0`
- WHEN Pi harness initializes
- THEN footer MUST not render and extension-api MUST not inject powerline

### Requirement: PRETTY-V2-PI-02 — Pill Streaming via Extension API

The system MUST stream pill updates via `extension-api.js` to the Pi viewport using the same 16ms throttle and `isSyncSupported()` guard as TUI `syncOutput`. `biggz-tool-pills.js` MUST be collapsible (`>3 → +N`) and highlight-aware, updating incrementally without full re-render dump. The system MUST NOT stream pills when `BIGGZ_PRETTY=0` or `PI_SUBAGENT_CHILD=1`.

#### Scenario: Throttled incremental pill update
- GIVEN 2 pill updates within 16ms via extension-api
- WHEN viewport flushes
- THEN Pi MUST show coalesced pill state with single extension-api write

#### Scenario: Collapsible preserves order via API
- GIVEN 4 pills streamed via extension-api
- WHEN footer/pill area renders
- THEN display MUST be 3 pills plus `… +1 hidden` preserving input order

#### Scenario: Subagent child bypass
- GIVEN `PI_SUBAGENT_CHILD=1`
- WHEN extension-api pill streaming invoked
- THEN system MUST suppress pill injection and render plain fallback

### Requirement: PRETTY-V2-PI-03 — Opt-In Mouse Gating

The system MUST gate mouse support via `BIGGZ_MOUSE=1` opt-in using `enableMouse` in Pi harness. Default MUST be mouse off. When enabled, the system MUST still respect `BIGGZ_PRETTY=0` and `BIGGZ_NO_ANIMATION=1`/`TERM=dumb` (mouse off when pretty/animation disabled). Enable MUST be reversible in one revert and <400 lines.

#### Scenario: Default mouse off
- GIVEN `BIGGZ_MOUSE` unset
- WHEN Pi harness starts
- THEN `enableMouse` MUST NOT be called and mouse events MUST be ignored

#### Scenario: Opt-in enables mouse
- GIVEN `BIGGZ_MOUSE=1` and `BIGGZ_PRETTY!=0` and `TERM=xterm-256color`
- WHEN harness initializes
- THEN `enableMouse` MUST be invoked and mouse events MUST be handled

#### Scenario: Guard overrides opt-in
- GIVEN `BIGGZ_MOUSE=1` but `BIGGZ_PRETTY=0` or `BIGGZ_NO_ANIMATION=1` or `TERM=dumb`
- WHEN harness evaluates mouse
- THEN mouse MUST remain disabled and no `enableMouse` call MUST occur

### Requirement: Pi Native BigMem MCP Provisioning

`ProvisionBigMemMCP` MUST write `mcpServers.bigmem` into `~/.pi/agent/mcp.json` only, in Pi's native shape: `command=BiggzMCPPath()`, `args=["--tools=agent","--prefix=biggz"]`, a `description`, `exposure="codemode"`, and `toolExposure` mapping exactly the 11 allowlist names (`biggz_mem_save`, `biggz_mem_search`, `biggz_mem_get_observation`, `biggz_mem_context`, `biggz_mem_session_summary`, `biggz_mem_save_prompt`, `biggz_mem_update`, `biggz_mem_timeline`, `biggz_mem_review`, `biggz_mem_judge`, `biggz_mem_current_project`) to `direct`. The entry MUST carry no `type` (Pi infers stdio from `command`), no top-level `imports` and no top-level `directTools` (both pruned on reinstall). The write MUST be atomic via `filemerge.WriteFileAtomic` and MUST preserve foreign servers and foreign top-level keys; the merge MUST be allowlist-prune (drop the 9 removed BigMem names from `toolExposure` when present). `settings.json` MUST keep only the `packages` reconcile, and any adapter-era `mcpServers.bigmem` or top-level `directTools` there MUST be pruned. Project `.pi/mcp.json` overlays global but `bigmem` MUST stay authoritative; server `ProfileAgent` MUST stay at 20 tools.

#### Scenario: Fresh provision is the native 11-tool shape
- GIVEN no Pi MCP config exists
- WHEN `ProvisionBigMemMCP` executes
- THEN `mcp.json` `mcpServers.bigmem` MUST be native (`command`, `args`, `description`, `exposure="codemode"`, `toolExposure` = the 11 allowlist) with no `type`, and `settings.json` MUST carry no `mcpServers.bigmem` and no `directTools`

#### Scenario: Reinstall prunes stale 9
- GIVEN `mcp.json` whose `toolExposure` carries all 20 names
- WHEN reinstall merges
- THEN the 9 removed names MUST be dropped, 11 allowlist MUST remain

#### Scenario: Foreign entries preserved atomically
- GIVEN `mcp.json` with `mcpServers.other` plus foreign top-level keys
- WHEN merge runs
- THEN `other` and foreign keys MUST be preserved; failed write MUST leave target unchanged

#### Scenario: Settings.json reduced to packages
- GIVEN an earlier install left `mcpServers.bigmem` and `directTools` in `settings.json`
- WHEN reinstall reconciles
- THEN both adapter-era keys MUST be pruned and `packages` MUST remain

#### Scenario: Global vs project precedence
- GIVEN global and project `mcp.json` exist
- WHEN Pi resolves servers
- THEN project MUST overlay global but `bigmem` MUST win in both files

#### Scenario: Server stays at 20
- GIVEN `--tools=agent` server profile
- WHEN `tools/list` runs
- THEN `ProfileAgent` MUST still expose all 20 tools

### Requirement: Adapter MCP Server Migration

`ProvisionBigMemMCPReport` MUST fold, in the same atomic `mcp.json` write, every server the retired `pi-mcp-adapter` used to reach: the Pi `settings.json` `mcpServers` map (skipping the biggz-owned `bigmem`) plus the OpenCode config `mcp`/`mcpServers`. The conversion MUST follow the documented OpenCode-to-Pi shape: a `command` array splits into `command`+`args`, `environment` becomes `env`, `{env:NAME}` becomes `${NAME}`, a remote entry becomes `url`, and adapter-only keys are dropped and reported. A name `mcp.json` already defines MUST never be overwritten, the pass MUST be idempotent (a second run adds nothing and leaves the file byte-stable), and an unreadable source MUST be reported and left alone rather than failing the pass. Migrated, skipped and unreadable detail MUST be reported through `MCPMigrationReport`.

#### Scenario: Adapter-imported servers become native entries

- GIVEN `settings.json` `mcpServers.other` with a `command` array and `environment`, plus an OpenCode `mcp` entry using `{env:NAME}`
- WHEN `ProvisionBigMemMCPReport` runs
- THEN `mcp.json` MUST carry both in native shape (`command`+`args`, `env`, `${NAME}`) and `MCPMigrationReport.Migrated` MUST name them

#### Scenario: Existing names are never overwritten

- GIVEN `mcp.json` already defines `other`
- WHEN the migration pass runs
- THEN the existing `other` entry MUST remain byte-identical and the name MUST NOT be imported again

#### Scenario: Second run is a fixed point

- GIVEN a completed migration
- WHEN `ProvisionBigMemMCPReport` runs again
- THEN it MUST report no migration and leave `mcp.json` byte-stable

#### Scenario: Adapter-only keys dropped and unreadable source reported

- GIVEN an imported entry carrying `type` and a source file that is not valid JSON
- WHEN the pass runs
- THEN `type` MUST be dropped and reported in `Dropped`, and the file MUST appear in `Unreadable` without failing the pass

### Requirement: Slim APPEND_SYSTEM Generation

The system MUST generate `APPEND_SYSTEM.md` with a single REMINDER block, all `<!-- biggz:* -->` markers, and gate template intact, with zero semantic change (prose/example trim only). The background subagent policy is stated as rules in `biggz-orchestrator-delegation.md` (on-demand delegation contract, read at delegation time) and resolved live from `subagent` tool availability; no template token carries it, and `APPEND_SYSTEM.md` carries zero literal `{{...}}` tokens by design.

#### Scenario: Single REMINDER with markers intact
- GIVEN asset trim applied
- WHEN `APPEND_SYSTEM.md` is generated
- THEN exactly one REMINDER MUST exist and all markers/template MUST be present (background-policy token lives in the delegation asset, not `APPEND_SYSTEM.md`)

#### Scenario: Reinstall does not reduplicate REMINDER
- GIVEN existing slim `APPEND_SYSTEM.md`
- WHEN reinstall regenerates
- THEN REMINDER count MUST stay one

### Requirement: Reinstall Convergence and Rollback

The system MUST converge fresh and existing installs to the 11-name `toolExposure` allowlist plus slim prompt on every `biggz install --agent pi`; revert of sources plus reinstall MUST restore 20-tool promotion and full prompt with no migration.

#### Scenario: Existing install converges
- GIVEN deployed fat 20-name `mcp.json`
- WHEN `biggz install --agent pi` re-runs
- THEN `toolExposure` MUST equal the 11 allowlist

#### Scenario: Rollback restores fat state
- GIVEN slim sources reverted
- WHEN `biggz install --agent pi` re-runs
- THEN 20-tool promotion and full prompt MUST return

### Requirement: Native-Only Pi Memory Path

The system MUST serve `biggz_mem_*` through Pi's own native MCP client reading `mcp.json`, with no wrapper fallback and with no `pi-mcp-adapter`: while that retired extension is installed Pi ignores `mcp.json` and its own servers, so it MUST stay absent. Provisioning and annotations requirements stay unchanged.

#### Scenario: Fresh install serves tools without wrappers

- GIVEN fresh install with both wrappers excluded
- WHEN operator runs `pi mcp list` or `/mcp`
- THEN native `biggz_mem_*` (at least `biggz_mem_save`) MUST appear

#### Scenario: Doctor gate passes natively

- GIVEN provisioned native `mcp.json`
- WHEN `biggz doctor` runs the `pi-native-mcp` check
- THEN result MUST be PASS (no `pi-mcp-adapter`, native `bigmem` entry, `settings.json` carries no MCP state, `pi mcp list` reports `bigmem`)

#### Scenario: Native tool handle truthy in harness

- GIVEN Pi harness with its native MCP client loaded
- WHEN extension calls `pi.getTool("biggz_mem_save")`
- THEN result MUST be truthy and wrappers MUST stay no-op/absent

#### Scenario: Session-guard factory intact

- GIVEN wrappers removed from deploy list
- WHEN Pi starts and loads `biggz-session-guard.js` factory
- THEN Pi MUST start without `Extension does not export a valid factory function`

### Requirement: Phase-2 Stability Gate

The system MUST delete wrapper sources only when ALL criteria pass after one-release soak with zero fallback-firing reports.

| # | Criterion |
|---|-----------|
| 1 | `doctor pi-native-mcp` PASS |
| 2 | `pi.getTool("biggz_mem_save")` truthy, `pi mcp list` shows `bigmem` |
| 3 | `mcp.json` `bigmem` carries native `command`/`args`/`exposure`/`toolExposure` with no `type`; `settings.json` carries no MCP state |
| 4 | `go vet` + `go test` + `node --check` green |
| 5 | One-release soak, zero fallback reports |

#### Scenario: All criteria pass allows Phase 2

- GIVEN all five criteria verified after soak
- WHEN release manager approves source deletion
- THEN deletion of both JS sources MAY proceed

#### Scenario: Any criterion fails blocks Phase 2

- GIVEN any single criterion failing or soak incomplete
- WHEN Phase 2 deletion evaluated
- THEN sources MUST be retained and deploy-list exclusion stays

### Requirement: Single-Commit Rollback

The system MUST support single-commit revert restoring wrappers, plus reinstall and remedy paths.

#### Scenario: Revert restores wrappers

- GIVEN Phase 1 commit reverted
- WHEN `biggz install --agent pi` re-runs
- THEN both wrappers MUST redeploy and `pi list` MUST show them

#### Scenario: Remedy re-provisions native path

- GIVEN native MCP config missing or stale
- WHEN operator runs the remedy `pi remove npm:pi-mcp-adapter` then `biggz install --agent pi`
- THEN `doctor pi-native-mcp` MUST return to PASS

### Requirement: BigMem MCP Tool Annotations

`cmd/biggz-mcp:buildToolList` MUST set `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint` per BigMem semantics for Pi's tool-exposure filtering.

#### Scenario: Read-only marked
- GIVEN `tools/list` after build
- WHEN inspecting `biggz_mem_search`/`biggz_mem_get`
- THEN each MUST have `readOnlyHint:true`, `destructiveHint:false`

#### Scenario: Mutating not read-only
- GIVEN same list
- WHEN inspecting `biggz_mem_save`
- THEN it MUST have `readOnlyHint:false`
