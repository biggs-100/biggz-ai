# Tasks: Enforce Session-Close Summary

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~220-290 (PR1 ~150-200, PR2 ~60-90) |
| 400-line budget risk | Low |
| Chained PRs recommended | Yes (Cut 1 before Cut 2) |
| Suggested split | PR1 Cut 1 (Go CLI) → PR2 Cut 2 (JS guard) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Cut 1: Go `session-close` CLI + tests | PR1 → main | `go test ./cmd/biggz/ -run TestSessionClose -count=1` | `go run ./cmd/biggz session-close --check-only --cwd <tmp>` | Revert verb case in `cmd/biggz/main.go`; CLI gone |
| 2 | Cut 2: JS `session_stop` guard + convergence | PR2 → main (stacked on PR1) | `node --test internal/assets/pi/biggz-session-stop.test.mjs` | `session_stop` with mocked CLI exits 0/1/timeout | Restore allow-only `session_stop`; no CLI change |

## Phase 1: Cut 1 — Go CLI session-close

- [x] 1.1 RED: create `cmd/biggz/cli_session_close_test.go` — flag conflict (exit 2), foreign-project allow, JSON shape, fallback-never-verifies, traversal `--change ../x` anchored
- [x] 1.2 GREEN: create `cmd/biggz/cli_session_close.go` — `sessionCloseRun()` manual flag parse, 10s ctx, delegate to `VerifySessionSummaryWithWorkspace` + `SaveSessionSummaryWithFallbackForChange`, exits 0/1/2
- [x] 1.3 Wire `case "session-close"` in `cmd/biggz/main.go` dispatch
- [x] 1.4 Add `session-close` help line in `cmd/biggz/cli_doctor_help.go` (flags + exits 0/1)
- [x] 1.5 APPLY-DECIDE Q1: keep `--change` default `session-close` unless active SDD change derivable; default to design value
- [x] 1.6 Verify: `go test ./cmd/biggz/ -run TestSessionClose -count=1` + `go vet`/`gofmt`

## Phase 2: Cut 2 — JS session_stop guard + convergence (after Phase 1)

- [x] 2.1 RED: create `internal/assets/pi/biggz-session-stop.test.mjs` (mock `execFileSync`, mirror `biggz-synthesis-gate.test.mjs`) — exit 0 allow, exit 1 block, timeout allow+warn, pending-first skips CLI, both-file parity
- [x] 2.2 APPLY-DECIDE Q2: measure `biggz` cold-start, confirm/finalize 250ms timeout; default 250ms → DECIDED 1000ms (measured 274–289ms cold-start win32 2026-09-07; 250ms would flap into degrade and lose enforcement)
- [x] 2.3 GREEN: export `checkSessionStop()` in `internal/assets/pi/biggz-tool-interception.js` — pending check → `execFileSync` argv array (no shell), never throws
- [x] 2.4 APPLY-DECIDE Q3: confirm ESM import route, then delegate `session_stop` in `internal/assets/pi/biggz-extension-api.js` to shared guard; delete inline logic → DECIDED static ESM import (precedent biggz-footer.js → biggz-extension-api.js, jiti resolves; one-way, no cycle)
- [x] 2.5 Verify: `node --test internal/assets/pi/biggz-session-stop.test.mjs` → 10/10 pass; full JS suite 49/49 pass

## Phase 3: Integration verification

- [x] 3.1 Run `go test ./... -count=1 -timeout 180s` (or scoped equivalent) + JS test suite; fix regressions → `go build ./...` + `go test ./cmd/biggz/ ./internal/sdd/ ./internal/extension/` all PASS + JS 49/49 PASS
- [x] 3.2 Walk spec scenarios: save→check round-trip, foreign-project no-write, `session-fallback.md` still blocks → foreign allow + no files (live), biggz-ai verify allow (live, real summary), stale-binary degrade (live + mocked), block/timeout parity (mocked); Go Cut-1 tests cover round-trip + fallback-never-verifies
