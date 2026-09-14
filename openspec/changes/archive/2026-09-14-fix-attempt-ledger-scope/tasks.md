# Tasks: fix-attempt-ledger-scope

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~650–850 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | fields + seam, current semantics | PR 1 | `go test ./internal/sddattempt/ -run 'Legacy\|CAS' -count=1` | N/A — no visible change | revert seam; gen-1 bytes unstamped |
| 2 | advance + reset + cap + test migration | PR 2 | `go test ./internal/sddattempt/ -run 'Advance\|Reset' -count=1` | `acquire → settle passed → acquire --work-unit <other>` | revert admission hunks |
| 3 | assets + CLI + rebuild + dogfood | PR 3 | `go test ./... -count=1` | rebuilt `biggz.exe` multi-slice dogfood | revert assets/CLI |

## Phase 1: Foundation

- [x] 1.1 `sddattempt.go:44`: `Generation`, `Advances`, `RuntimeAttempt.ObjectiveGeneration`, all `omitempty`
- [x] 1.2 `deriveScopeAdmission` seam (`:448`) consumed by `Acquire:1312`, `Begin`, `StatusWithInstance:369`; `corrupt_authority` kept
- [x] 1.3 CAS contract: `omitempty`; gen-1 = field ABSENCE, never explicit `1`; `0≡1`/`Lifetime*` derived read-time, never canonical

## Phase 2: RED — advance/threat/refusal

- [x] 2.1 RED `advance_test.go` (gentle-ai `runtime_objective_advance_test.go`): Advance, Fresh budget `1`→`5`+`CumulativeChangedLines=0`, New/Kept generation, `LastAdvance`, lifetime, successor refund budget, Any field refused/Unchanged scope
- [x] 2.2 RED threat (Git repository selection): root/subdir advance → SAME `<git-common-dir>` store (record+`HEAD`); non-git scope → own machine dir
- [x] 2.3 RED: same-label acquire post-`passed` → `blocked(work_unit_complete)` naming successor; rewrites `acquire_settle_test.go:176-223` (Repeat refused/Ledger intact)

## Phase 3: GREEN — advance/refusal/guard

- [x] 3.1 GREEN: successor admission (`Complete`, no active, last `passed`, different `--work-unit`) opens its generation; budget from own request
- [x] 3.2 GREEN: `BlockedReasonWorkUnitComplete` + design message naming successor
- [x] 3.3 GREEN: drop duplicated guards `:1392-1410`; single 4-field guard → `invalid_continuation` when OPEN; unchanged scope admits

## Phase 4: GREEN — reset/refund cap

- [x] 4.1 GREEN: `Reset` (`:1020`) stops clearing `Attempts`, clears live objective only (Live state cleared/Attempts survive/lifetime survives)
- [x] 4.2 GREEN: refunds capped `2×MaxAttempts` of live generation (`:673`); successor not charged

## Phase 5: Tests — migration/legacy

- [x] 5.1 `legacy_compat_test.go`: PRE-CHANGE fixture re-verifies content address; frozen generation-1 bytes; Derived default shows 1, bytes untouched; Legacy advances
- [x] 5.2 Extend `cas_store_test.go:80` + `remediation_derive_test.go` (Corrective admitted after advance/Remediated refused)
- [x] 5.3 Unchanged: `parity_test.go`, `rescope_test.go`, `sddattempt_test.go:177`, `budget_refund_test.go` 1–3

## Phase 6: Assets/CLI/rebuild/dogfood

- [x] 6.1 Assets: `biggz-orchestrator-workflow.md:184-187`, `sdd-status-contract.md:86-93,229`, `opencode/commands/sdd-verify.md:22`, `prompts/sdd/sdd-verify.md:44,82`; mirror `skills/sdd-verify/SKILL.md`
- [x] 6.2 `cmd/biggz/cli_sdd.go`: print `Generation`/`Lifetime*`
- [x] 6.3 Guard: no `Rescope` (`:2043-2170`), remediation guards (`:1316-1345`, `:1629-1633`), `deriveSettleObligation` (`:415-446`), no new verb/flag, no `openspec/specs/**` edit
- [x] 6.4 Rebuild `go build -o biggz.exe ./cmd/biggz`; dogfood `acquire→settle passed→acquire --work-unit <other>→settle passed`, ZERO new `Resets`; refusal names successor
- [x] 6.5 Final `go test ./... -count=1` green
