# Tasks: SDD Discipline Gates

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 250–350 |
| 400-line budget risk | Low |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 → PR 4 (stacked) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Go gate narrowing + unit tests | PR 1 | `go test ./internal/sdd/ -run 'Synthesis'` | N/A — unit gate only | `internal/sdd/synthesis_gate.go` + gate tests |
| 2 | JS mirror + node parity tests | PR 2 | `node --test` gate mirror tests | N/A — mirror parity only | `internal/assets/pi/biggz-synthesis-gate.js` + JS tests |
| 3 | HasExplicitPreflight + dispatcher admission + tests | PR 3 | `go test ./internal/sdd/ -run 'Preflight'` | `biggz sdd-status` without preflight → blocked | `internal/sdd/preflight.go`, `status.go`, `continue.go` + tests |
| 4 | Fallback envelope wiring | PR 4 | `go test ./internal/sdd/...` | Blocked checkpoint ask → same-turn fallback shown | Gate envelope fields only |
| 5 | Ledger multi-unit lifecycle + CLI hardening | PR 5 | `go test ./internal/sddattempt/ -run 'MultiUnit|UnknownFlag'` | `biggz sdd-attempt acquire/settle` two units, one final complete | `internal/sddattempt/sddattempt.go` settle path + `cmd/biggz/cli_sdd.go` flag loop + HelpText |

## Phase 1: Go gate narrowing (REQ-DG-1)

- [x] 1.1 Modify `ShouldBlock` in `internal/sdd/synthesis_gate.go` to require `IsCheckpointAsk`; `HasOptions` alone never blocks
- [x] 1.2 RED: free-text ask fixture returns false in `internal/sdd/synthesis_gate_test.go` (verify fail, then pass)
- [x] 1.3 RED: preflight option-ask fixture returns false in `internal/sdd/synthesis_gate_test.go`
- [x] 1.4 RED: checkpoint w/o synthesis blocks; valid synthesis passes (REQ-DG-4 regression)
- [x] 1.5 RED: gate-bypass fixtures (child/recall) keep prior behavior per threat matrix

## Phase 2: Fallback envelope + JS mirror (REQ-DG-2, REQ-DG-1 parity)

- [x] 2.1 Add `context`+`fallback` (via `FormatFallback`) to blocked envelope in `internal/sdd/synthesis_gate.go`
- [x] 2.2 RED: blocked checkpoint envelope contains full question verbatim in Go test
- [x] 2.3 Mirror `isCheckpointAsk` gate + `formatFallback` envelope in `internal/assets/pi/biggz-synthesis-gate.js`
- [x] 2.4 RED: JS parity test fails on verdict mismatch vs Go fixtures (checkpoint/free-text/preflight)

## Phase 3: Explicit-preflight admission (REQ-DG-3)

- [x] 3.1 Add `HasExplicitPreflight(cwd, home...) bool` in `internal/sdd/preflight.go` (cache > disk > false)
- [x] 3.2 RED: defaults-only returns false; empty→false; cache/disk→true in `internal/sdd/preflight_*_test.go`
- [x] 3.3 Gate phase entry in `internal/sdd/status.go` + `continue.go`: `blocked(preflight_missing)` + `resolve-blockers`, no launch
- [x] 3.4 RED: status-read without preflight succeeds; phase entry without preflight blocks (newly-blocked-flows row)

## Phase 4: Verification

- [x] 4.1 Run `go test ./internal/sdd/...` — all gate + admission tests pass
- [x] 4.2 Run JS `node --test` mirror suite — parity green
- [x] 4.3 Verify `git diff --stat` shows zero installer/TUI files (REQ-DG-4)

## Phase 5: Ledger multi-unit lifecycle + CLI hardening (Unit 5)

> Context: `Settle(passed)` sets `Complete=true` (`internal/sddattempt/sddattempt.go`), so `deriveAdmissionBlocked` refuses the next `Acquire` (`blocked(corrupt_authority)` — reset required). Multi-unit apply (Units 1-4) cannot run as acquire→settle-per-unit; it needed a maintainer reset between units. Also `sddAttemptRun` (`cmd/biggz/cli_sdd.go`) has no `default` in its flag loop, so unknown flags (e.g. `--cwd`, `--change`) are silently ignored — `change` is positional `args[1]`, cwd is always `os.Getwd()`. Budget: 285+/33- used of 400 — keep Unit 5 small.

- [x] 5.1 RED test in `internal/sddattempt/acquire_settle_test.go`: ONE acquire covering N work units with single final settle (intermediate settle must NOT complete ledger; next acquire admitted). Fix minimal coherent with `Begin/Finish` semantics: new non-terminal outcome value OR per-unit acquire without ledger-complete OR docs+guard message — record choice in `apply-progress.md`
- [x] 5.2 RED test + fail-closed fix in `cmd/biggz/cli_sdd.go` `sddAttemptRun` flag loop: unknown flag → `error: unknown flag <f>` exit 1 (mirror `sddStatusRun`); OR explicit documented decision not to, with reasoning in `apply-progress.md`
- [x] 5.3 Update `HelpText` in `internal/sddattempt/sddattempt.go`: state `<change>` is positional, cwd is always `os.Getwd()`, no `--cwd`/`--change` flags; verify `go test ./internal/sddattempt/ && go vet ./...` green and `git diff --stat` stays under 400-line budget
